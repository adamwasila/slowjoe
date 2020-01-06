package main

import (
	"github.com/adamwasila/slowjoe"
)

var version string = "0.0.0-snapshot"

func main() {
	slowjoe.Proxy(version)
}
