package config

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/pflag"
)

type Config struct {
	Bind, Upstream              string
	Delay                       time.Duration
	Rate, AdminPort             int
	CloseChance, ThrottleChance float64
	Verbose                     bool
	VeryVerbose                 bool
	MetricsEnabled              bool
}

func (cfg *Config) Read() {
	pflag.StringVarP(&cfg.Bind, "bind", "b", "0.0.0.0:9998", "Address to bind listening socket to")
	pflag.StringVarP(&cfg.Upstream, "upstream", "u", "127.0.0.1:8000", "<host>[:port] of upstream service")
	pflag.IntVarP(&cfg.Rate, "rate", "r", -1, "Maximum data rate of bytes per second if throttling applied (see --throttle-chance)")
	pflag.DurationVarP(&cfg.Delay, "delay", "d", 0, "Initial delay when connection starts to deteriorate")
	pflag.BoolVarP(&cfg.MetricsEnabled, "admin", "a", false, "Enable admin console service")
	pflag.IntVarP(&cfg.AdminPort, "admin-port", "p", 6000, "Port for admin console service")
	pflag.BoolVarP(&cfg.Verbose, "verbose", "v", false, "Enable verbose output (debug logs)")
	pflag.BoolVarP(&cfg.VeryVerbose, "very-verbose", "w", false, "Enable very verbose output (trace logs)")

	pflag.Float64VarP(&cfg.CloseChance, "close-chance", "c", 0.0, "Probability of closing socket abruptly")
	pflag.Float64VarP(&cfg.ThrottleChance, "throttle-chance", "t", 0.0, "Probability of throttling")

	pflag.ErrHelp = errors.New("")
	pflag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n\n", os.Args[0])
		pflag.PrintDefaults()
	}

	pflag.Parse()
}
