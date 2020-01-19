package slowjoe

import (
	"errors"
	"io"
	"math"
	"math/rand"
	"net"
	"strings"
	"sync"
	"time"

	_ "expvar"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/adamwasila/slowjoe/admin"
	"github.com/adamwasila/slowjoe/config"
)

type Proxy struct {
	version      string
	cfg          config.Config
	bindAddr     *net.TCPAddr
	upstreamAddr *net.TCPAddr
	shutdowner   shutdowner
}

type connection struct {
	log             *logrus.Entry
	direction       string
	inst            instrumentation
	id              string
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
		cfg.ThrottleChance += cfg.CloseChance

		if cfg.ThrottleChance < 0.0 || cfg.CloseChance < 0.0 {
			return errors.New("Invalid config; chances must be >= 0")
		}

		if cfg.ThrottleChance > 1.0 || cfg.CloseChance > 1.0 {
			return errors.New("Invalid config; sum of all chances must be <= 1.0")
		}

		if cfg.Rate < -1 {
			return errors.New("Invalid config; rate must be >= 0 or -1 for unlimited")
		}
		p.cfg = cfg
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
	logrus.WithFields(logrus.Fields{
		"bind":       p.cfg.Bind,
		"upstream":   p.cfg.Upstream,
		"rate":       p.cfg.Rate,
		"admin-port": p.cfg.AdminPort,
	}).Debugf("Config found")

	ln, err := net.ListenTCP("tcp", p.bindAddr)
	if err != nil {
		return err
	}
	p.shutdowner.register(func() {
		ln.Close()
	})

	logrus.WithField("bind", p.cfg.Bind).Infof("Listen on TCP socket")

	var on instrumentation = &nopInstrumentation{}
	if p.cfg.MetricsEnabled {
		ad := admin.NewAdminData()
		ad.Version = p.version
		ad.Config = p.cfg
		m := metrics{}
		m.init(p.cfg.AdminPort, ad, p.shutdowner)
		on = composedInstrumentation([]instrumentation{ad, &m})
	}
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

		close := chance < p.cfg.CloseChance
		throttle := chance >= p.cfg.CloseChance && chance < p.cfg.ThrottleChance

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

		log := logrus.WithField("alias", name)
		if close {
			typ = "closing"
		}

		if throttle {
			typ = "throttling"
		}

		log = log.WithField("type", typ)

		ups, err := net.DialTCP("tcp", nil, p.upstreamAddr)
		if err != nil {
			logrus.WithError(err).Errorf("Could not connect to upstream")
			err := conn.Close()
			if err != nil {
				logrus.WithError(err).Warnf("Error closing source connection")
			}
			continue
		}
		log.Debugf("New connection")

		on.ConnectionOpened(id, name, typ)

		once := sync.Once{}
		connectionCloser := func() {
			once.Do(func() {
				log := logrus.WithField("alias", name)
				log.Tracef("Calling close")
				err := conn.Close()
				if err != nil {
					log.WithError(err).Debugf("Error while closing connection")
				}
				err = ups.Close()
				if err != nil {
					log.WithError(err).Warnf("Error while closing connection")
				}
				on.ConnectionClosed(id, time.Since(acceptedTimestamp))
			})
		}
		hookID := p.shutdowner.register(connectionCloser)

		if close {
			logrus.WithField("delay", p.cfg.Delay).Trace("Scheduling close")
			time.AfterFunc(p.cfg.Delay, func() {
				logrus.Trace("Delay triggered close")
				connectionCloser()
				p.shutdowner.unregister(hookID)
			})
			continue
		}

		go func() {
			Executor(
				Execute(
					func() {
						c := connection{log, config.DirUpstream, on, id, throttle, close, p.cfg.Rate, p.cfg.Delay, conn, ups}
						c.handleConnection()
						if p.cfg.Rate != 0 {
							c.closeSingleSide()
							on.ConnectionClosedUpstream(id)
						}
					},
					func() {
						c := connection{log, config.DirDownstream, on, id, throttle, close, p.cfg.Rate, p.cfg.Delay, ups, conn}
						c.handleConnection()
						if p.cfg.Rate != 0 {
							c.closeSingleSide()
							on.ConnectionClosedDownstream(id)
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
			).Run()
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

	log := c.log.WithField("direction", c.direction)

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
			c.inst.ConnectionProgressed(c.id, c.direction, m)

			if time.Since(t0) > c.delay && c.throttle && c.rate > 0 {
				waitTime := time.Duration(1000*float64(n)/float64(c.rate)) * time.Millisecond
				t2 := time.Since(t1)
				waitTime = waitTime - t2
				if waitTime < 0 {
					waitTime = 0
				}
				log.WithField("readbytes", n).WithField("writebytes", m).WithField("duration", waitTime.Seconds()).Trace("Sleeping")
				time.Sleep(waitTime)
			}
		}

		log.WithField("duration", time.Since(t0).Round(10*time.Millisecond)).
			WithField("rate", math.Round(float64(bytes)/time.Since(t0).Seconds())).
			WithField("bytes", bytes).
			Info("Completed")
	}

}

func (c *connection) closeSingleSide() {
	err := c.r.CloseRead()
	if err != nil {
		c.log.WithError(err).Debugf("Error closing reading")
	}
	err = c.w.CloseWrite()
	if err != nil {
		c.log.WithError(err).Debugf("Error closing writing")
	}
}
