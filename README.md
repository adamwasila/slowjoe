![slowjoe](slowjoe-logo.png "Slow Joe")

# Slow Joe

[![Go Report Card](https://goreportcard.com/badge/adamwasila/slowjoe)](https://goreportcard.com/report/adamwasila/slowjoe) [![Build Status](https://github.com/adamwasila/slowjoe/actions/workflows/main.yml/badge.svg)](https://github.com/adamwasila/slowjoe/actions/workflows/main.yml) [![Coverage Status](https://coveralls.io/repos/github/adamwasila/slowjoe/badge.svg?branch=master)](https://coveralls.io/github/adamwasila/slowjoe?branch=master) ![GitHub release (latest by date)](https://img.shields.io/github/v/release/adamwasila/slowjoe) ![GitHub](https://img.shields.io/github/license/adamwasila/slowjoe)

Simple TCP proxy to test your services for poor network conditions.

Simplicity is the most important aspect. No one wants to spend hours looking for dependencies, then learning yet another DSL or the quirks of a config file, right? Downloading a single, static binary (see [releases](../releases/latest)) and reading the description of a [few flags](#configuration) is all you need to start. Being a Docker aficionado makes things even simpler: all you need is to run `docker run`, as the [service image is available on Docker Hub](https://hub.docker.com/r/adamwasila/slowjoe).

> **WARNING**: unstable product. API, configuration and behaviour may and will change without a warning.

## Quick start

If docker is available it is by far the easiest way to download and run service:

```console
docker run --rm -it adamwasila/slowjoe:latest

INFO[0000] Upstream set                                  address="127.0.0.1:8000"
INFO[0000] Listen on TCP socket                          bind="127.0.0.1:9998"
```

Otherwise, go to [releases subpage](../releases/latest) and download latest version, unpack and put in your `PATH`.

Last option is to build on your own. See [Install](#install) section below for details.

With the binary in your `PATH`, you may try the following examples. First, run a less trivial example where `slowjoe` works as a proxy to the [httpbin service](httpbin.org). Let's assume that 10% of requests should have throughput limited to 1kb/s.

```bash
slowjoe -a -u "httpbin.org:80" -t 0.1 -r 1024
```

Use `curl` to check proxy response:

```bash
curl http://localhost:9998/headers

{
  "headers": {
    "Accept": "*/*", 
    "Host": "localhost", 
    "User-Agent": "curl/7.64.0"
  }
}
```

Now hit the following link in your browser <http://localhost:9998/image/jpeg> and experience modem-like connection speed. Welcome back to the Internet of the 90s.

Next test closing immediately behaviour:

```bash
slowjoe -u "httpbin.org:80" -c 1.0
```

Now, all requests should be closed without sending any data.

Both behaviours may be mixed together. Here, half of the connections are closed immediately and half are throttled to 1000 bytes per second:

```bash
slowjoe -u "httpbin.org:80" -c 0.5 -t 0.5 -r 1000
```

Finally, point your browser to <http://localhost:6000> to see current settings and a list of currently open connections:

![webdashboard](dashboard.png "Simple dashboard")

> Note: `-a` option must be set to enable web dashboard. Then `-p` may be used to set port other than default `6000`.

## Install

Go 1.26 should be installed on the system. While it should compile succesfully using older versions it is recommended to use version 1.26 or newer.

Clone project repository to your machine:

```bash
git clone https://github.com/adamwasila/slowjoe.git
```

To build, enter repository and run:

```bash
go build -o slowjoe ./cmd/slowjoe
```

## Configuration

Help flag `-h` allows to see full configuration options:

```console
slowjoe -h

Usage of slowjoe:
  -a, --admin                   Enable admin console service
  -p, --admin-port int          Port for admin console service (default 6000)
  -b, --bind string             Address to bind listening socket to (default "0.0.0.0:9998")
  -c, --close-chance float      Probability of closing socket abruptly
  -d, --delay duration          Initial delay when connection starts to deteriorate
  -r, --rate int                Maximum data rate of bytes per second if throttling applied (see --throttle-chance)
  -t, --throttle-chance float   Probability of throttling
  -u, --upstream string         <host>[:port] of upstream service (default "127.0.0.1:8000")
  -v, --verbose                 Enable verbose output (debug logs)
  -w, --very-verbose            Enable very verbose output (trace logs)
```
