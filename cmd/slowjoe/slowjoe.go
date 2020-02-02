package main

import (
	"context"
	"math/rand"
	"time"

	"github.com/adamwasila/slowjoe"
	"github.com/adamwasila/slowjoe/admin"
	"github.com/adamwasila/slowjoe/config"
	"github.com/sirupsen/logrus"
)

var version string = "0.0.0-snapshot"

func main() {
	defer slowjoe.SafeQuit()
	rand.Seed(time.Now().UnixNano())

	var cfg config.Config
	err := cfg.Read()
	if err != nil {
		return
	}

	initLogger(cfg.Verbose, cfg.VeryVerbose)

	var insts slowjoe.Instrumentations
	insts = append(insts, slowjoe.DefaultLogs())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	slowjoe.SetSignalCallback(cancel)

	if cfg.MetricsEnabled {
		ad := admin.NewAdminData()
		ad.Version = version
		ad.Config = cfg
		m := slowjoe.Metrics{}
		m.Init(ctx, cfg.AdminPort, ad)
		insts = append(insts, ad, &m)
	}

	proxy, err := slowjoe.New(
		slowjoe.Version(version),
		slowjoe.Config(cfg),
		slowjoe.Bind(cfg.Bind),
		slowjoe.Upstream(cfg.Upstream),
		slowjoe.Instrument(
			insts,
		),
	)
	if err != nil {
		logrus.Fatal(err.Error())
		return
	}

	err = proxy.ListenAndLoop(ctx)
	if err != nil {
		logrus.WithError(err).Infof("Main loop break. Service will quit shortly")
	}

}

func initLogger(verbose, veryVerbose bool) {
	if verbose {
		logrus.SetLevel(logrus.DebugLevel)
	}
	if veryVerbose {
		logrus.SetLevel(logrus.TraceLevel)
	}
	// do not ignore automatically upon fatal log; lets app logic decide
	// if it should quit immediately or do some cleanup before
	logrus.StandardLogger().ExitFunc = func(int) {}
}
