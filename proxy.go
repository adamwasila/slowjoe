package slowjoe

import (
	"io"
	"math"
	"math/rand"
	"net"
	"os"
	"sync"
	"time"

	_ "expvar"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/adamwasila/slowjoe/admin"
	"github.com/adamwasila/slowjoe/config"
)

func Proxy(version string) {
	setupGracefulStop()
	rand.Seed(time.Now().UnixNano())

	var cfg config.Config
	cfg.Read()

	var on instrumentation = &nopInstrumentation{}
	if cfg.MetricsEnabled {
		ad := admin.NewAdminData()
		ad.Version = version
		ad.Config = cfg
		m := metrics{}
		m.init(cfg.AdminPort, ad)
		on = composedInstrumentation([]instrumentation{ad, &m})
	}

	cfg.ThrottleChance += cfg.CloseChance

	if cfg.ThrottleChance < 0.0 || cfg.CloseChance < 0.0 {
		logrus.Fatal("Invalid config; chances must be >= 0")
	}

	if cfg.ThrottleChance > 1.0 || cfg.CloseChance > 1.0 {
		logrus.Fatal("Invalid config; sum of all chances must be <= 1.0")
	}

	if cfg.Rate < -1 {
		logrus.Fatal("Invalid config; rate must be >= 0 or -1 for unlimited")
	}

	if cfg.Verbose {
		logrus.SetLevel(logrus.DebugLevel)
	}
	if cfg.VeryVerbose {
		logrus.SetLevel(logrus.TraceLevel)
	}

	logrus.WithFields(logrus.Fields{
		"bind":       cfg.Bind,
		"upstream":   cfg.Upstream,
		"rate":       cfg.Rate,
		"admin-port": cfg.AdminPort,
	}).Debugf("Config found")

	addr, err := net.ResolveTCPAddr("tcp", cfg.Bind)
	if err != nil {
		logrus.Errorln("Could not resolve", err)
		os.Exit(1)
	}

	upstreamAddr, err := net.ResolveTCPAddr("tcp", cfg.Upstream)
	if err != nil {
		logrus.Errorln("Could not resolve", err)
		os.Exit(1)
	}
	logrus.WithField("address", cfg.Upstream).Infof("Upstream set")

	ln, err := net.ListenTCP("tcp", addr)
	if err != nil {
		logrus.Errorln("Could not listen", err)
		os.Exit(1)
	}

	logrus.WithField("bind", cfg.Bind).Infof("Listen on TCP socket")

	for {
		conn, err := ln.AcceptTCP()
		if err != nil {
			logrus.Errorln("Could not Accept", err)
			continue
		}
		acceptedTimestamp := time.Now()

		chance := rand.Float64()

		close := chance < cfg.CloseChance
		throttle := chance >= cfg.CloseChance && chance < cfg.ThrottleChance

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

		ups, err := net.DialTCP("tcp", nil, upstreamAddr)
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
		hookID := registerShutdownHook(connectionCloser)

		if close {
			logrus.WithField("delay", cfg.Delay).Trace("Scheduling close")
			time.AfterFunc(cfg.Delay, func() {
				logrus.Trace("Delay triggered close")
				connectionCloser()
				unregisterShutdownHook(hookID)
			})
			continue
		}

		go func() {
			Executor(
				Execute(
					func() {
						handleConnection(log, config.DirUpstream, on, id, throttle, close, cfg.Rate, cfg.Delay, conn, ups)
						if cfg.Rate != 0 {
							closeSingleSide(log, conn, ups)
							on.ConnectionClosedUpstream(id)
						}
					},
					func() {
						handleConnection(log, config.DirDownstream, on, id, throttle, close, cfg.Rate, cfg.Delay, ups, conn)
						if cfg.Rate != 0 {
							closeSingleSide(log, ups, conn)
							on.ConnectionClosedDownstream(id)
						}
					},
				),
				WhenAllFinished(
					func() {
						connectionCloser()
						unregisterShutdownHook(hookID)
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

func calcBufSize(throttle bool, rate int) int {
	bufSize := 16384
	if throttle && rate >= 0 {
		bufSize = rate >> 4
	}
	if bufSize > 16384 {
		bufSize = 16384
	}
	if bufSize < 1 {
		bufSize = 1
	}
	return bufSize
}

func handleConnection(log *logrus.Entry, direction string, inst instrumentation, id string, throttle, close bool, rate int, delay time.Duration, r *net.TCPConn, w *net.TCPConn) {
	bytes := 0
	notClosed := true

	t0 := time.Now()

	var buf []byte

	log = log.WithField("direction", direction)

	if rate != 0 {
		for notClosed {
			throttleAlready := throttle && (time.Since(t0) > delay)
			bufSize := calcBufSize(throttleAlready, rate)

			if bufSize != len(buf) {
				buf = make([]byte, bufSize)
				log.WithField("size", bufSize).Trace("Buffer created")
			}

			t1 := time.Now()
			n, readErr := r.Read(buf)
			if readErr == io.EOF {
				log.Tracef("Read EOF")
				readErr = nil
				notClosed = false
			}
			m, writeErr := w.Write(buf[0:n])
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
			inst.ConnectionProgressed(id, direction, m)

			if time.Since(t0) > delay && throttle && rate > 0 {
				waitTime := time.Duration(1000*float64(n)/float64(rate)) * time.Millisecond
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

func closeSingleSide(log *logrus.Entry, r *net.TCPConn, w *net.TCPConn) {
	err := r.CloseRead()
	if err != nil {
		log.WithError(err).Debugf("Error closing reading")
	}
	err = w.CloseWrite()
	if err != nil {
		log.WithError(err).Debugf("Error closing writing")
	}
}
