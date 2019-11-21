package main

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/pflag"
)

type config struct {
	bind, upstream              string
	delay                       time.Duration
	rate, adminPort             int
	closeChance, throttleChance float64
	verbose                     bool
	veryVerbose                 bool
	metricsEnabled              bool
}

func (cfg *config) read() {
	pflag.StringVarP(&cfg.bind, "bind", "b", "0.0.0.0:9998", "Address to bind listening socket to")
	pflag.StringVarP(&cfg.upstream, "upstream", "u", "127.0.0.1:8000", "<host>[:port] of upstream service")
	pflag.IntVarP(&cfg.rate, "rate", "r", -1, "Maximum data rate of bytes per second if throttling applied (see --throttle-chance)")
	pflag.DurationVarP(&cfg.delay, "delay", "d", 0, "Initial delay when connection starts to deteriorate")
	pflag.BoolVarP(&cfg.metricsEnabled, "admin", "a", false, "Enable admin console service")
	pflag.IntVarP(&cfg.adminPort, "admin-port", "p", 6000, "Port for admin console service")
	pflag.BoolVarP(&cfg.verbose, "verbose", "v", false, "Enable verbose output (debug logs)")
	pflag.BoolVarP(&cfg.veryVerbose, "very-verbose", "w", false, "Enable very verbose output (trace logs)")

	pflag.Float64VarP(&cfg.closeChance, "close-chance", "c", 0.0, "Probability of closing socket abruptly")
	pflag.Float64VarP(&cfg.throttleChance, "throttle-chance", "t", 0.0, "Probability of throttling")

	pflag.ErrHelp = errors.New("")
	pflag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n\n", os.Args[0])
		pflag.PrintDefaults()
	}

	pflag.Parse()
}
