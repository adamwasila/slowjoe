package main

import (
	"io"
	"math/rand"
	"net"
	"os"
	"sync"
	"time"

	_ "expvar"

	"github.com/goombaio/namegenerator"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
)

func main() {
	setupGracefulStop()

	seed := time.Now().UTC().UnixNano()
	nameGenerator := namegenerator.NewNameGenerator(seed)

	var bind, upstream string
	var admin bool
	var delay time.Duration
	var rate, adminPort int
	var closeChance, throttleChance float64
	var verbose bool
	var veryVerbose bool

	pflag.StringVarP(&bind, "bind", "b", "127.0.0.1:9998", "Address to bind listening socket to")
	pflag.StringVarP(&upstream, "upstream", "u", "127.0.0.1:8000", "<host>[:port] of upstream service")
	pflag.IntVarP(&rate, "rate", "r", 0, "Maximum data rate of bytes per second if throttling applied (see --throttle-chance)")
	pflag.DurationVarP(&delay, "delay", "d", 0, "Initial delay when connection starts to deteriorate")
	pflag.BoolVarP(&admin, "admin", "a", false, "Enable admin console service")
	pflag.IntVarP(&adminPort, "admin-port", "p", 6000, "Port for admin console service")
	pflag.BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output (debug logs)")
	pflag.BoolVarP(&veryVerbose, "very-verbose", "w", false, "Enable very verbose output (trace logs)")

	pflag.Float64VarP(&closeChance, "close-chance", "c", 0.0, "Probability of closing socket abruptly")
	pflag.Float64VarP(&throttleChance, "throttle-chance", "t", 0.0, "Probability of throttling")

	pflag.Parse()

	throttleChance += closeChance

	if throttleChance < 0.0 || closeChance < 0.0 {
		logrus.Fatal("Invalid config; chances must be >= 0")
	}

	if throttleChance > 1.0 || closeChance > 1.0 {
		logrus.Fatal("Invalid config; sum of all chances must be <= 1.0")
	}

	if rate < 0 {
		logrus.Fatal("Invalid config; rate must be >= 0")
	}

	if verbose {
		logrus.SetLevel(logrus.DebugLevel)
	}
	if veryVerbose {
		logrus.SetLevel(logrus.TraceLevel)
	}

	logrus.WithFields(logrus.Fields{
		"bind":       bind,
		"upstream":   upstream,
		"rate":       rate,
		"admin":      admin,
		"admin-port": adminPort,
	}).Debugf("Config found")

	var m metrics
	m.init(admin, adminPort)

	addr, err := net.ResolveTCPAddr("tcp", bind)
	if err != nil {
		logrus.Errorln("Could not resolve", err)
		os.Exit(1)
	}

	upstreamAddr, err := net.ResolveTCPAddr("tcp", upstream)
	if err != nil {
		logrus.Errorln("Could not resolve", err)
		os.Exit(1)
	}
	logrus.WithField("address", upstream).Infof("Upstream set")

	ln, err := net.ListenTCP("tcp", addr)
	if err != nil {
		logrus.Errorln("Could not listen", err)
		os.Exit(1)
	}

	logrus.WithField("bind", bind).Infof("Listen on TCP socket")

	for {
		conn, err := ln.AcceptTCP()
		if err != nil {
			logrus.Errorln("Could not Accept", err)
			continue
		}

		chance := rand.Float64()

		close := chance < closeChance
		throttle := chance >= closeChance && chance < throttleChance

		name := nameGenerator.Generate()
		log := logrus.WithField("alias", name)
		if close {
			log = log.WithField("type", "closing")
		}

		if throttle {
			log = log.WithField("type", "throttling")
		}

		log.Infof("Accepting connection")

		ups, err := net.DialTCP("tcp", nil, upstreamAddr)
		if err != nil {
			logrus.WithError(err).Errorf("Could not connect to upstream")
			err := conn.Close()
			if err != nil {
				logrus.WithError(err).Warnf("Error closing source connection")
			}
			continue
		}

		m.activeConnectionAdd()

		once := sync.Once{}
		connectionCloser := func() {
			once.Do(func() {
				log := logrus.WithField("alias", name)
				log.Debugf("Calling close")
				err := conn.Close()
				if err != nil {
					log.WithError(err).Warnf("Error while closing connection")
				}
				err = ups.Close()
				if err != nil {
					log.WithError(err).Warnf("Error while closing connection")
				}
				m.activeConnectionRemove()
			})
		}
		hookID := registerShutdownHook(connectionCloser)

		if close {
			logrus.WithField("delay", delay).Debug("Scheduling close")
			time.AfterFunc(delay, func() {
				logrus.Debug("Delay triggered close")
				connectionCloser()
				unregisterShutdownHook(hookID)
			})
		}

		go func() {
			handleConnection(log.WithField("direction", "upstream"), throttle, close, rate, delay, conn, ups)
			if rate > 0 {
				connectionCloser()
				unregisterShutdownHook(hookID)
			}
		}()

		go func() {
			handleConnection(log.WithField("direction", "downstream"), throttle, close, rate, delay, ups, conn)
			if rate > 0 {
				connectionCloser()
				unregisterShutdownHook(hookID)
			}
		}()
	}
}

func handleConnection(log *logrus.Entry, throttle, close bool, rate int, delay time.Duration, r *net.TCPConn, w *net.TCPConn) {
	bufSize := 16384
	if throttle {
		bufSize = rate >> 4
	}
	if bufSize > 16384 {
		bufSize = 16384
	}
	if bufSize < 1 {
		bufSize = 1
	}

	buf := make([]byte, bufSize)
	log.WithField("size", bufSize).Info("Buffer created")

	bytes := 0
	notClosed := true

	t0 := time.Now()

	if rate > 0 {
		for notClosed {
			t1 := time.Now()
			n, readErr := r.Read(buf)
			if readErr == io.EOF {
				readErr = nil
				err := r.CloseRead()
				log.Debugf("Read EOF")
				if err != nil {
					log.WithError(err).Errorf("Error closing on read side")
				}
				notClosed = false
			}
			m, writeErr := w.Write(buf[0:n])
			if n != m {
				log.WithField("undeliveredBytes", m-n).Infof("Closing connection: wrote less bytes than expected")
				notClosed = false
			}
			if readErr != nil || writeErr != nil {
				log.WithField("readErr", readErr).WithField("writeErr", writeErr).Warnf("Closing connection upon error")
				notClosed = false
			}
			bytes += m

			if time.Since(t0) > delay && throttle && rate > 0 {
				waitTime := time.Duration(1000*float64(n)/float64(rate)) * time.Millisecond
				t2 := time.Since(t1)
				waitTime = waitTime - t2
				if waitTime < 0 {
					waitTime = 0
				}
				log.WithField("readbytes", n).WithField("writebytes", m).WithField("duration", waitTime.Seconds()).Debug("Sleeping")
				time.Sleep(waitTime)
			}
		}
		log.WithField("time", time.Since(t0).Seconds()).WithField("rate", float64(bytes)/time.Since(t0).Seconds()).WithField("bytes", bytes).Infof("Closing")
	}

}
