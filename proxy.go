package main

import (
	"io"
	"math"
	"math/rand"
	"net"
	"os"
	"sync"
	"time"

	_ "expvar"

	"github.com/sirupsen/logrus"
)

func main() {
	setupGracefulStop()
	rand.Seed(time.Now().UnixNano())

	var cfg config
	cfg.read()

	var on instrumentation = &nopInstrumentation{}
	if cfg.metricsEnabled {
		m := metrics{}
		m.init(cfg.adminPort)
		on = &m
	}

	cfg.throttleChance += cfg.closeChance

	if cfg.throttleChance < 0.0 || cfg.closeChance < 0.0 {
		logrus.Fatal("Invalid config; chances must be >= 0")
	}

	if cfg.throttleChance > 1.0 || cfg.closeChance > 1.0 {
		logrus.Fatal("Invalid config; sum of all chances must be <= 1.0")
	}

	if cfg.rate < -1 {
		logrus.Fatal("Invalid config; rate must be >= 0 or -1 for unlimited")
	}

	if cfg.verbose {
		logrus.SetLevel(logrus.DebugLevel)
	}
	if cfg.veryVerbose {
		logrus.SetLevel(logrus.TraceLevel)
	}

	logrus.WithFields(logrus.Fields{
		"bind":       cfg.bind,
		"upstream":   cfg.upstream,
		"rate":       cfg.rate,
		"admin-port": cfg.adminPort,
	}).Debugf("Config found")

	addr, err := net.ResolveTCPAddr("tcp", cfg.bind)
	if err != nil {
		logrus.Errorln("Could not resolve", err)
		os.Exit(1)
	}

	upstreamAddr, err := net.ResolveTCPAddr("tcp", cfg.upstream)
	if err != nil {
		logrus.Errorln("Could not resolve", err)
		os.Exit(1)
	}
	logrus.WithField("address", cfg.upstream).Infof("Upstream set")

	ln, err := net.ListenTCP("tcp", addr)
	if err != nil {
		logrus.Errorln("Could not listen", err)
		os.Exit(1)
	}

	logrus.WithField("bind", cfg.bind).Infof("Listen on TCP socket")

	for {
		conn, err := ln.AcceptTCP()
		if err != nil {
			logrus.Errorln("Could not Accept", err)
			continue
		}
		acceptedTimestamp := time.Now()

		chance := rand.Float64()

		close := chance < cfg.closeChance
		throttle := chance >= cfg.closeChance && chance < cfg.throttleChance

		var name string

		switch {
		case close:
			name = randomC()
		case throttle:
			name = randomT()
		default:
			name = randomR()
		}

		log := logrus.WithField("alias", name)
		if close {
			log = log.WithField("type", "closing")
		}

		if throttle {
			log = log.WithField("type", "throttling")
		}

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

		on.ConnectionOpened()

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
				on.ConnectionClosed(time.Since(acceptedTimestamp))
			})
		}
		hookID := registerShutdownHook(connectionCloser)

		if close {
			logrus.WithField("delay", cfg.delay).Trace("Scheduling close")
			time.AfterFunc(cfg.delay, func() {
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
						log = log.WithField("direction", "upstream")
						handleConnection(log, on, throttle, close, cfg.rate, cfg.delay, conn, ups)
						if cfg.rate != 0 {
							closeSingleSide(log, conn, ups)
							on.ConnectionClosedUpstream()
						}
					},
					func() {
						log = log.WithField("direction", "downstream")
						handleConnection(log, on, throttle, close, cfg.rate, cfg.delay, ups, conn)
						if cfg.rate != 0 {
							closeSingleSide(log, ups, conn)
							on.ConnectionClosedDownstream()
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

func handleConnection(log *logrus.Entry, inst instrumentation, throttle, close bool, rate int, delay time.Duration, r *net.TCPConn, w *net.TCPConn) {
	bytes := 0
	notClosed := true

	t0 := time.Now()

	var buf []byte

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
			inst.ConnectionProgressed(m)

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
