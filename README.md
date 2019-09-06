# Slow Joe

Small TCP proxy to test your services for reconnection & timeout procedures.

## Install

Go 1.12+ should be installed in the system. While it was not tested with earlier versions of go it should work as long as you provide all required dependencies.

Clone this repository or download latest version. There are no official, versioned releases yet so take latest commit from the master.

Enter repository and issue command:

```bash
go build
```

## Quick start

First, start listening on default port:

```bash
slowjoe -a -u "httpbin.org:80" -t 0.9
```

Test if proxy replies correctly:

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

Less boring stuff: kill `slowjoe` and run it again with 100% throttling chance and 1024b/s maximum rate:

```bash
slowjoe -a -u "httpbin.org:80" -t 1.0 -r 1024
```

Now, open following link in the browser:

<http://localhost:9998/image/png>

It should take few seconds for image to load which proves proxy is delaying data transfer as expected.

## Configuration

Help flag `-h` allows to see full configuration options:

```bash
slowjoe -h

Usage of ./slowjoe:
  -a, --admin                   Enable admin console service
  -p, --admin-port int          Port for admin console service (default 6000)
  -b, --bind string             Address to bind listening socket to (default "127.0.0.1:9998")
  -r, --rate int                Maximum data rate of bytes per second if throttling applied (see --throttle-chance)
  -t, --throttle-chance float   Probability of throttling
  -u, --upstream string         <host>[:port] of upstream service (default "127.0.0.1:8000")
  -v, --verbose                 Enable verbose output (debug logs)
  -w, --very-verbose            Enable very verbose output (trace logs)
```
