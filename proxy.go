package main

import (
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net"
	"os"
	"sync"
	"time"

	_ "expvar"

	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
)

type instrumentation interface {
	ConnectionOpened(id string)
	ConnectionClosedUpstream(id string)
	ConnectionClosedDownstream(id string)
	ConnectionClosed(id string)
}

type nopInstrumentation struct{}

func (*nopInstrumentation) ConnectionOpened(id string)           {}
func (*nopInstrumentation) ConnectionClosedUpstream(id string)   {}
func (*nopInstrumentation) ConnectionClosedDownstream(id string) {}
func (*nopInstrumentation) ConnectionClosed(id string)           {}

func main() {
	setupGracefulStop()

	rand.Seed(time.Now().UnixNano())

	var bind, upstream string
	var delay time.Duration
	var rate, adminPort int
	var closeChance, throttleChance float64
	var verbose bool
	var veryVerbose bool
	var metricsEnabled bool

	pflag.StringVarP(&bind, "bind", "b", "127.0.0.1:9998", "Address to bind listening socket to")
	pflag.StringVarP(&upstream, "upstream", "u", "127.0.0.1:8000", "<host>[:port] of upstream service")
	pflag.IntVarP(&rate, "rate", "r", -1, "Maximum data rate of bytes per second if throttling applied (see --throttle-chance)")
	pflag.DurationVarP(&delay, "delay", "d", 0, "Initial delay when connection starts to deteriorate")
	pflag.BoolVarP(&metricsEnabled, "admin", "a", false, "Enable admin console service")
	pflag.IntVarP(&adminPort, "admin-port", "p", 6000, "Port for admin console service")
	pflag.BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output (debug logs)")
	pflag.BoolVarP(&veryVerbose, "very-verbose", "w", false, "Enable very verbose output (trace logs)")

	pflag.Float64VarP(&closeChance, "close-chance", "c", 0.0, "Probability of closing socket abruptly")
	pflag.Float64VarP(&throttleChance, "throttle-chance", "t", 0.0, "Probability of throttling")

	pflag.ErrHelp = errors.New("")
	pflag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n\n", os.Args[0])
		pflag.PrintDefaults()
	}

	pflag.Parse()

	var i instrumentation = &nopInstrumentation{}
	if metricsEnabled {
		m := metrics{}
		m.init(adminPort)
		i = &m
	}

	throttleChance += closeChance

	if throttleChance < 0.0 || closeChance < 0.0 {
		logrus.Fatal("Invalid config; chances must be >= 0")
	}

	if throttleChance > 1.0 || closeChance > 1.0 {
		logrus.Fatal("Invalid config; sum of all chances must be <= 1.0")
	}

	if rate < -1 {
		logrus.Fatal("Invalid config; rate must be >= 0 or -1 for unlimited")
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
		"admin-port": adminPort,
	}).Debugf("Config found")

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

		i.ConnectionOpened(name)

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
				i.ConnectionClosed(name)
			})
		}
		hookID := registerShutdownHook(connectionCloser)

		if close {
			logrus.WithField("delay", delay).Trace("Scheduling close")
			time.AfterFunc(delay, func() {
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
						handleConnection(log, throttle, close, rate, delay, conn, ups)
						if rate != 0 {
							closeSingleSide(log, conn, ups)
							i.ConnectionClosedUpstream(name)
						}
					},
					func() {
						log = log.WithField("direction", "downstream")
						handleConnection(log, throttle, close, rate, delay, ups, conn)
						if rate != 0 {
							closeSingleSide(log, ups, conn)
							i.ConnectionClosedDownstream(name)
						}
					},
				),
				WhenAllFinished(
					func() {
						connectionCloser()
						unregisterShutdownHook(hookID)
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

func handleConnection(log *logrus.Entry, throttle, close bool, rate int, delay time.Duration, r *net.TCPConn, w *net.TCPConn) {
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
