package main

import (
	"io"
	"math/rand"
	"net"
	"os"
	"strings"
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
	var rate, adminPort int
	// var abortChance, delayInfChance,
	var throttleChance float64
	var verbose bool
	var veryVerbose bool

	pflag.StringVarP(&bind, "bind", "b", "127.0.0.1:9998", "Address to bind listening socket to")
	pflag.StringVarP(&upstream, "upstream", "u", "127.0.0.1:8000", "<host>[:port] of upstream service")
	pflag.IntVarP(&rate, "rate", "r", 0, "Maximum data rate of bytes per second if throttling applied (see --throttle-chance)")
	pflag.BoolVarP(&admin, "admin", "a", false, "Enable admin console service")
	pflag.IntVarP(&adminPort, "admin-port", "p", 6000, "Port for admin console service")
	pflag.BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output (debug logs)")
	pflag.BoolVarP(&veryVerbose, "very-verbose", "w", false, "Enable very verbose output (trace logs)")

	// pflag.Float64VarP(&abortChance, "abort-chance", "a", 0.0, "")
	// pflag.Float64VarP(&delayInfChance, "delay-chance", "d", 0.0, "")
	pflag.Float64VarP(&throttleChance, "throttle-chance", "t", 0.0, "Probability of throttling")

	// delayInfChance += abortChance
	// throttleChance += delayInfChance

	pflag.Parse()

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
	}).Infof("Config found")

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

	ln, err := net.ListenTCP("tcp", addr)
	if err != nil {
		logrus.Errorln("Could not listen", err)
		os.Exit(1)
	}

	logrus.WithField("bind", bind).Infof("Listening now")

	for {
		conn, err := ln.AcceptTCP()
		if err != nil {
			logrus.Errorln("Could not Accept", err)
			continue
		}

		chance := rand.Float64()

		// abort := chance < abortChance
		// delayInf := chance < delayInfChance
		throttle := chance < throttleChance

		name := nameGenerator.Generate()
		if throttle {
			name = strings.ToUpper(name)
		}

		logrus.WithField("name", name).Infof("Incoming connection")

		ups, err := net.DialTCP("tcp", nil, upstreamAddr)
		if err != nil {
			logrus.Errorf("Could not connect to upstream: [%s]", err)
			err := conn.Close()
			if err != nil {
				logrus.Warnf("Error while closing source connection: %s", err)
			}
			continue
		}

		// if abort {
		// 	err := conn.Close()
		// 	if err != nil {
		// 		logrus.Warnf("Error while closing source connection: %s", err)
		// 	}
		// 	err = ups.Close()
		// 	if err != nil {
		// 		logrus.Warnf("Error while closing upstream connection: %s", err)
		// 	}
		// 	continue
		// }

		m.activeConnectionAdd()

		once := sync.Once{}
		connectionCloser := func() {
			once.Do(func() {
				log := logrus.WithField("name", name)
				log.Warnf("Calling close")
				err := conn.Close()
				if err != nil {
					log.Warnf("Error while closing connection: %s", err)
				}
				err = ups.Close()
				if err != nil {
					log.Warnf("Error while closing connection: %s", err)
				}
				m.activeConnectionRemove()
			})
		}
		hookID := registerShutdownHook(connectionCloser)

		go func() {
			handleConnection(name, "to_upstream", throttle, rate, conn, ups)
			connectionCloser()
			unregisterShutdownHook(hookID)
		}()

		go func() {
			handleConnection(name, "from_upstream", throttle, rate, ups, conn)
			connectionCloser()
			unregisterShutdownHook(hookID)
		}()
	}
}

func handleConnection(name, direction string, throttle bool, rate int, r *net.TCPConn, w *net.TCPConn) {
	log := logrus.WithField("name", name).WithField("direction", direction)
	bufSize := rate
	if throttle {
		bufSize = bufSize >> 4
	}
	if bufSize > 16384 {
		bufSize = 16384
	}
	if bufSize < 128 {
		bufSize = 128
	}

	buf := make([]byte, bufSize)
	log.WithField("size", bufSize).Info("Buffer created")

	bytes := 0
	notClosed := true

	t0 := time.Now()

	for notClosed {
		t1 := time.Now()
		n, readErr := r.Read(buf)
		if readErr == io.EOF {
			readErr = nil
			err := r.CloseRead()
			if err != nil {
				log.Infof("Read returned EOF; closing on Read side")
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

		if throttle {
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
