package main

import (
	"math/rand"
	"time"

	"github.com/adamwasila/slowjoe"
	"github.com/adamwasila/slowjoe/config"
	"github.com/sirupsen/logrus"
)

var version string = "0.0.0-snapshot"

func main() {
	var cfg config.Config
	cfg.Read()
	if cfg.Verbose {
		logrus.SetLevel(logrus.DebugLevel)
	}
	if cfg.VeryVerbose {
		logrus.SetLevel(logrus.TraceLevel)
	}

	rand.Seed(time.Now().UnixNano())

	sh := slowjoe.SignalShutdowner{}
	proxy := slowjoe.New(version, cfg, &sh)
	proxy.ListenAndLoop()
}
