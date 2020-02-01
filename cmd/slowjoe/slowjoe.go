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
	var cfg config.Config
	cfg.Read()
	if cfg.Verbose {
		logrus.SetLevel(logrus.DebugLevel)
	}
	if cfg.VeryVerbose {
		logrus.SetLevel(logrus.TraceLevel)
	}

	rand.Seed(time.Now().UnixNano())

	var insts slowjoe.Instrumentations
	insts = append(insts, &slowjoe.Logs{logrus.StandardLogger()})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	slowjoe.CallForSignals(cancel)

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
	}

	err = proxy.ListenAndLoop(ctx)
	if err != nil {
		logrus.WithError(err).Infof("Main loop break. Service will quit shortly")
	}
}
