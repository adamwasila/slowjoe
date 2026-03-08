package slowjoe

import (
	"context"
	"errors"
	"io"
	"math/rand"
	"net"
	"sync"
	"time"

	_ "expvar"

	"github.com/google/uuid"
	"github.com/oklog/run"
	"github.com/sirupsen/logrus"

	"github.com/adamwasila/slowjoe/config"
)

type Proxy struct {
	version                     string
	bind, upstream              string
	delay                       time.Duration
	rate                        int
	adminPort                   int
	closeChance, throttleChance float64
	metricsEnabled              bool
	bindAddr                    *net.TCPAddr
	upstreamAddr                *net.TCPAddr
	instrumentations            Instrumentations
}

type connection struct {
	direction string
	inst      Instrumentation
	id        string
	alias     string
	typ       string
	throttle  bool
	rate      int
	delay     time.Duration
	r         *net.TCPConn
	w         *net.TCPConn
}

type proxyOption func(*Proxy) error

func Version(version string) proxyOption {
	return func(p *Proxy) error {
		p.version = version
		return nil
	}
}

func Config(cfg config.Config) proxyOption {
	return func(p *Proxy) error {
		if cfg.CloseChance < 0.0 {
			return errors.New("invalid config: close probability must be >= 0")
		}

		if cfg.CloseChance > 1.0 {
			return errors.New("invalid config: close probability must be <= 1.0")
		}

		if cfg.ThrottleChance < 0.0 {
			return errors.New("invalid config: throttle probability must be >= 0")
		}

		if cfg.ThrottleChance > 1.0 {
			return errors.New("invalid config: throttle probability must be <= 1.0")
		}

		if cfg.CloseChance+cfg.ThrottleChance > 1.0 {
			return errors.New("invalid config: sum of close and throttle probabilities must be <= 1.0")
		}

		if cfg.Rate < 0 && cfg.Rate != -1 {
			return errors.New("invalid config: rate must be >= 0 or -1 for unlimited")
		}

		p.closeChance = cfg.CloseChance
		p.throttleChance = cfg.CloseChance + cfg.ThrottleChance
		p.rate = cfg.Rate

		addr, err := net.ResolveTCPAddr("tcp", cfg.Bind)
		if err != nil {
			return err
		}
		p.bind = cfg.Bind
		p.bindAddr = addr

		addrUpstream, err := net.ResolveTCPAddr("tcp", cfg.Upstream)
		if err != nil {
			return err
		}
		p.upstream = cfg.Upstream
		p.upstreamAddr = addrUpstream

		p.delay = cfg.Delay
		p.metricsEnabled = cfg.MetricsEnabled
		p.adminPort = cfg.AdminPort

		return nil
	}
}

func Instrument(inst ...Instrumentation) proxyOption {
	return func(p *Proxy) error {
		p.instrumentations = append(p.instrumentations, inst...)
		return nil
	}
}

func New(options ...proxyOption) (*Proxy, error) {
	p := &Proxy{}
	for _, option := range options {
		err := option(p)
		if err != nil {
			return nil, err
		}
	}
	return p, nil
}

func (p *Proxy) Listen(ctx context.Context, g *run.Group) error {
	log := logrus.StandardLogger()
	log.WithFields(logrus.Fields{
		"bind":       p.bind,
		"upstream":   p.upstream,
		"rate":       p.rate,
		"admin-port": p.adminPort,
	}).Debugf("Config found")

	ln, err := net.ListenTCP("tcp", p.bindAddr)
	if err != nil {
		return err
	}
	log.WithField("bind", p.bind).Infof("Listen on TCP socket")

	ctx, cancel := context.WithCancel(ctx)

	g.Add(func() error {
		return p.accept(ctx, ln)
	}, func(error) {
		ln.Close()
		cancel()
	})
	return nil
}

func (p *Proxy) accept(ctx context.Context, ln *net.TCPListener) error {
	on := p.instrumentations

	for {
		conn, err := ln.AcceptTCP()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		acceptedTimestamp := time.Now()

		chance := rand.Float64()

		shouldClose := chance < p.closeChance
		throttle := chance >= p.closeChance && chance < p.throttleChance

		id := uuid.New().String()

		var name, typ string
		switch {
		case shouldClose:
			name = randomC()
			typ = "closing"
		case throttle:
			name = randomT()
			typ = "throttling"
		default:
			name = randomR()
			typ = "regular"
		}

		ups, err := net.DialTCP("tcp", nil, p.upstreamAddr)
		if err != nil {
			logrus.WithError(err).Errorf("Could not connect to upstream")
			err := conn.Close()
			if err != nil {
				logrus.WithError(err).Warnf("Error closing source connection")
			}
			continue
		}
		on.ConnectionOpened(id, name, typ)

		childCtx, ctxCancel := context.WithCancel(ctx)
		once := sync.Once{}
		connectionCloser := func() {
			once.Do(func() {
				log := logrus.WithField("alias", name)
				err := conn.Close()
				if err != nil {
					log.WithError(err).Debugf("Error while closing connection")
				}
				err = ups.Close()
				if err != nil {
					log.WithError(err).Warnf("Error while closing connection")
				}
				ctxCancel()
				on.ConnectionClosed(id, name, time.Since(acceptedTimestamp))
			})
		}

		if shouldClose {
			on.ConnectionScheduledClose(id, name, p.delay)

			time.AfterFunc(p.delay, connectionCloser)
			continue
		}

		go func() {
			Executor(
				ExecuteWithContext(
					func(ctx context.Context) {
						c := connection{
							direction: config.DirUpstream,
							inst:      on,
							id:        id,
							alias:     name,
							typ:       typ,
							throttle:  throttle,
							rate:      p.rate,
							delay:     p.delay,
							r:         conn,
							w:         ups,
						}
						c.handleConnection(ctx)
						if p.rate != 0 {
							c.closeSingleSide()
							on.ConnectionClosedUpstream(id, name)
						}
					},
					func(ctx context.Context) {
						c := connection{
							direction: config.DirDownstream,
							inst:      on,
							id:        id,
							alias:     name,
							typ:       typ,
							throttle:  throttle,
							rate:      p.rate,
							delay:     p.delay,
							r:         ups,
							w:         conn,
						}
						c.handleConnection(ctx)
						if p.rate != 0 {
							c.closeSingleSide()
							on.ConnectionClosedDownstream(id, name)
						}
					},
				),
				WhenAllFinished(
					func() {
						connectionCloser()
					},
				),
				WhenPanic(
					func(p interface{}) {
						logrus.WithField("panic", p).Fatalf("Unexpected panic")
					},
				),
			).Run(childCtx)
		}()
	}
}

func (c *connection) calcBufSize(inThrottle bool) int {
	bufSize := 16384
	if inThrottle && c.rate >= 0 {
		bufSize = c.rate >> 4
	}
	if bufSize > 16384 {
		bufSize = 16384
	}
	if bufSize < 1 {
		bufSize = 1
	}
	return bufSize
}

func (c *connection) handleConnection(ctx context.Context) {
	totalBytes := 0

	t0 := time.Now()

	var buf []byte

	log := c.log()

	if c.rate != 0 {
		for {
			throttleAlready := c.throttle && (time.Since(t0) > c.delay)
			bufSize := c.calcBufSize(throttleAlready)

			if bufSize != len(buf) {
				buf = make([]byte, bufSize)
				log.WithField("size", bufSize).Trace("Buffer created")
			}

			t1 := time.Now()
			n, readErr := c.r.Read(buf)
			eof := readErr == io.EOF
			if eof {
				log.Tracef("Read EOF")
				readErr = nil
			}
			m, writeErr := c.w.Write(buf[0:n])
			totalBytes += m

			if n != m {
				log.WithField("undeliveredBytes", n-m).Infof("Wrote less bytes than expected")
			}
			if readErr != nil {
				log.WithError(readErr).Warnf("Read returned error")
			}
			if writeErr != nil {
				log.WithError(writeErr).Warnf("Write returned error")
			}
			c.inst.ConnectionProgressed(c.id, c.alias, c.direction, m)

			ctxCancelled := ctx.Err() != nil
			if time.Since(t0) > c.delay && c.throttle && c.rate > 0 {
				waitTime := time.Duration(1000*float64(n)/float64(c.rate)) * time.Millisecond
				t2 := time.Since(t1)
				waitTime = waitTime - t2
				if waitTime < 0 {
					waitTime = 0
				}
				c.inst.ConnectionDelayed(c.id, c.alias, c.direction, waitTime)
				select {
				case <-ctx.Done():
					ctxCancelled = true
				case <-time.After(waitTime):
				}
			}

			if eof || n != m || readErr != nil || writeErr != nil || ctxCancelled {
				break
			}
		}
		c.inst.ConnectionCompleted(c.id, c.alias, c.direction, totalBytes, time.Since(t0))
	}
}

func (c *connection) log() *logrus.Entry {
	return logrus.WithField("alias", c.alias).WithField("type", c.typ).WithField("direction", c.direction)
}

func (c *connection) closeSingleSide() {
	err := c.r.CloseRead()
	if err != nil {
		c.log().WithError(err).Debugf("Error closing reading")
	}
	err = c.w.CloseWrite()
	if err != nil {
		c.log().WithError(err).Debugf("Error closing writing")
	}
}
