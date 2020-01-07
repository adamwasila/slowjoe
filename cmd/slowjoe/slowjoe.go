package main

import (
	"github.com/adamwasila/slowjoe"
	"github.com/adamwasila/slowjoe/config"
)

var version string = "0.0.0-snapshot"

func main() {
	var cfg config.Config
	cfg.Read()

	sh := slowjoe.SignalShutdowner{}
	proxy := slowjoe.New(version, cfg, &sh)
	proxy.Loop()
}
