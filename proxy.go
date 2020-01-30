package slowjoe

import (
	"context"
	"errors"
	"io"
	"math/rand"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	_ "expvar"

	"github.com/google/uuid"
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
	shutdowner                  shutdowner
	instrumentations            Instrumentations
}

type connection struct {
	direction       string
	inst            Instrumentation
	id              string
	alias           string
	typ             string
	throttle, close bool
	rate            int
	delay           time.Duration
	r               *net.TCPConn
	w               *net.TCPConn
}

type proxyOption func(*Proxy) error

func Version(version string) proxyOption {
	return func(p *Proxy) error {
		p.version = version
		return nil
	}
}

func Shutdowner(sh shutdowner) proxyOption {
	return func(p *Proxy) error {
		p.shutdowner = sh
		return nil
	}
}

func Bind(bindAddress string) proxyOption {
	return func(p *Proxy) error {
		addr, err := net.ResolveTCPAddr("tcp", bindAddress)
		p.bindAddr = addr
		return err
	}
}

func Upstream(upstreamAddress string) proxyOption {
	return func(p *Proxy) error {
		addr, err := net.ResolveTCPAddr("tcp", upstreamAddress)
		p.upstreamAddr = addr
		return err
	}
}

func Config(cfg config.Config) proxyOption {
	return func(p *Proxy) error {
		close := cfg.CloseChance
		throttle := close + cfg.ThrottleChance

		if throttle < 0.0 || close < 0.0 {
			return errors.New("Invalid config; chances must be >= 0")
		}

		if throttle > 1.0 || close > 1.0 {
			return errors.New("Invalid config; sum of all chances must be <= 1.0")
		}

		if cfg.Rate < -1 {
			return errors.New("Invalid config; rate must be >= 0 or -1 for unlimited")
		}

		p.throttleChance = throttle
		p.closeChance = close
		p.rate = cfg.Rate

		p.bind = cfg.Bind
		p.upstream = cfg.Upstream

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

func (p *Proxy) ListenAndLoop() error {
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
	p.shutdowner.start(os.Exit)
	p.shutdowner.register(func() {
		ln.Close()
	})

	log.WithField("bind", p.bind).Infof("Listen on TCP socket")

	on := p.instrumentations

	for {
		conn, err := ln.AcceptTCP()
		if err != nil {
			if strings.Contains(err.Error(), "use of closed network connection") {
				return nil
			}
			return err
		}
		acceptedTimestamp := time.Now()

		chance := rand.Float64()

		close := chance < p.closeChance
		throttle := chance >= p.closeChance && chance < p.throttleChance

		var id, name string

		id = uuid.New().String()

		switch {
		case close:
			name = randomC()
		case throttle:
			name = randomT()
		default:
			name = randomR()
		}

		typ := "regular"

		if close {
			typ = "closing"
		}

		if throttle {
			typ = "throttling"
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
				on.ConnectionClosed(id, name, time.Since(acceptedTimestamp))
			})
		}
		hookID := p.shutdowner.register(connectionCloser)

		if close {
			on.ConnectionScheduledClose(id, name, p.delay)

			time.AfterFunc(p.delay, func() {
				connectionCloser()
				p.shutdowner.unregister(hookID)
			})
			continue
		}

		go func() {
			Executor(
				Execute(
					func() {
						c := connection{config.DirUpstream, on, id, name, typ, throttle, close, p.rate, p.delay, conn, ups}
						c.handleConnection()
						if p.rate != 0 {
							c.closeSingleSide()
							on.ConnectionClosedUpstream(id, name)
						}
					},
					func() {
						c := connection{config.DirDownstream, on, id, name, typ, throttle, close, p.rate, p.delay, ups, conn}
						c.handleConnection()
						if p.rate != 0 {
							c.closeSingleSide()
							on.ConnectionClosedDownstream(id, name)
						}
					},
				),
				WhenAllFinished(
					func() {
						connectionCloser()
						p.shutdowner.unregister(hookID)
					},
				),
				WhenPanic(
					func(p interface{}) {
						logrus.WithField("panic", p).Fatalf("Unexpected panic")
					},
				),
			).Run(context.TODO())
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

func (c *connection) handleConnection() {
	bytes := 0
	notClosed := true

	t0 := time.Now()

	var buf []byte

	log := c.log()

	if c.rate != 0 {
		for notClosed {
			throttleAlready := c.throttle && (time.Since(t0) > c.delay)
			bufSize := c.calcBufSize(throttleAlready)

			if bufSize != len(buf) {
				buf = make([]byte, bufSize)
				log.WithField("size", bufSize).Trace("Buffer created")
			}

			t1 := time.Now()
			n, readErr := c.r.Read(buf)
			if readErr == io.EOF {
				log.Tracef("Read EOF")
				readErr = nil
				notClosed = false
			}
			m, writeErr := c.w.Write(buf[0:n])
			bytes += m

			if n != m {
				log.WithField("undeliveredBytes", n-m).Infof("Wrote less bytes than expected")
				notClosed = false
			}
			if readErr != nil {
				log.WithError(readErr).Warnf("Read returned error")
				notClosed = false
			}
			if writeErr != nil {
				log.WithError(writeErr).Warnf("Write returned error")
				notClosed = false
			}
			c.inst.ConnectionProgressed(c.id, c.alias, c.direction, m)

			if time.Since(t0) > c.delay && c.throttle && c.rate > 0 {
				waitTime := time.Duration(1000*float64(n)/float64(c.rate)) * time.Millisecond
				t2 := time.Since(t1)
				waitTime = waitTime - t2
				if waitTime < 0 {
					waitTime = 0
				}
				c.inst.ConnectionDelayed(c.id, c.alias, c.direction, waitTime)
				time.Sleep(waitTime)
			}
		}
		c.inst.ConnectionCompleted(c.id, c.alias, c.direction, bytes, time.Since(t0))
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
