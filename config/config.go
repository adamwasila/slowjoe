package config

import (
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

func (cfg *Config) Read() error {
	flags := pflag.NewFlagSet(os.Args[0], pflag.ContinueOnError)

	flags.StringVarP(&cfg.Bind, "bind", "b", "0.0.0.0:9998", "Address to bind listening socket to")
	flags.StringVarP(&cfg.Upstream, "upstream", "u", "127.0.0.1:8000", "<host>[:port] of upstream service")
	flags.IntVarP(&cfg.Rate, "rate", "r", -1, "Maximum data rate of bytes per second if throttling applied (see --throttle-chance)")
	flags.DurationVarP(&cfg.Delay, "delay", "d", 0, "Initial delay when connection starts to deteriorate")
	flags.BoolVarP(&cfg.MetricsEnabled, "admin", "a", false, "Enable admin console service")
	flags.IntVarP(&cfg.AdminPort, "admin-port", "p", 6000, "Port for admin console service")
	flags.BoolVarP(&cfg.Verbose, "verbose", "v", false, "Enable verbose output (debug logs)")
	flags.BoolVarP(&cfg.VeryVerbose, "very-verbose", "w", false, "Enable very verbose output (trace logs)")

	flags.Float64VarP(&cfg.CloseChance, "close-chance", "c", 0.0, "Probability of closing socket abruptly")
	flags.Float64VarP(&cfg.ThrottleChance, "throttle-chance", "t", 0.0, "Probability of throttling")

	helpMe := flags.BoolP("help", "h", false, "Print service usage and quit")

	err := flags.Parse(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n\n", err)
		printUsage(flags)
		return err
	}
	if *helpMe {
		printUsage(flags)
		return pflag.ErrHelp
	}
	return nil
}

func printUsage(flags *pflag.FlagSet) {
	fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
	flags.PrintDefaults()
}
