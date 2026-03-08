# Slow Joe - Project Summary

## Overview

**Slow Joe** is a simple TCP proxy written in Go designed to test services and applications under poor network conditions. It simulates various network degradation scenarios including latency, bandwidth throttling, and connection failures without requiring complex configuration or external dependencies.

**Key Philosophy**: Simplicity first. Users can download a single static binary or use Docker and start testing immediately by reading just a few command-line flags.

## Core Purpose

Slow Joe acts as a TCP proxy between clients and services, injecting realistic network problems to help developers test their applications' resilience to poor connectivity. It's particularly useful for:

- Testing mobile app behavior under slow networks
- Verifying timeout and retry logic
- Load testing with degraded network conditions
- Validating graceful degradation strategies

## Key Features

### Network Simulation

- **Bandwidth Throttling**: Limit connection throughput to simulate slow connections (configurable bytes/second)
- **Connection Delay**: Add initial delay before data transmission starts
- **Connection Closing**: Randomly close connections without sending data (simulate network failures)
- **Configurable Probabilities**: Mix multiple behaviors with configurable probability percentages

### Deployment & Simplicity

- **Single Binary Distribution**: No dependencies, just download and run
- **Docker Support**: Pre-built Docker images on Hub for containerized deployment
- **Minimal Configuration**: Command-line flags only, no complex config files or DSLs
- **Cross-platform**: Built with Go, runs on Linux, macOS, Windows

### Monitoring & Administration

- **Web Dashboard**: Real-time visualization of settings and active connections (port 6000 by default)
- **Metrics Collection**: Built-in metrics for monitoring proxy behavior
- **Connection Tracking**: View all currently open connections in real-time
- **Logging**: Configurable verbosity levels for debugging

### Operational

- **Graceful Shutdown**: Proper signal handling (SIGTERM, SIGINT) for clean exits
- **Request Flow**: Full bi-directional proxying of TCP connections
- **Connection Management**: Tracks active connections with UUIDs for monitoring

## Project Structure

```
slowjoe/
├── cmd/slowjoe/              # CLI entry point
│   └── slowjoe.go           # Main application setup and signal handling
├── admin/                    # Web dashboard and monitoring
│   ├── admin.go             # Dashboard service implementation
│   └── assets/              # Static assets
│       ├── data/            # JavaScript libraries and CSS
│       └── templates/       # HTML templates for dashboard
├── config/                  # Configuration handling
│   ├── config.go           # Config structure and parsing
│   └── magic.go            # Configuration utilities
├── proxy.go                # Core TCP proxy logic
├── executor.go             # Concurrent job executor pattern
├── metrics.go              # Metrics collection and tracking
├── logs.go                 # Logging infrastructure
├── instrumentation.go      # Instrumentation interface
├── gracefullquit.go        # Signal handling and graceful shutdown
├── names.go                # Connection naming utilities
├── Dockerfile              # Docker image definition
└── go.mod                  # Go module dependencies
```

## Core Components

### Proxy (`proxy.go`)

The heart of Slow Joe. Implements:

- TCP listener and connection acceptance
- Bi-directional data forwarding between client and upstream
- Random connection simulation (delays, throttling, closing)
- Data rate limiting based on configured bandwidth
- Connection metadata tracking (UUID, timestamps, bytes transferred)

### Configuration (`config/`)

Handles command-line configuration with these key parameters:

- `--upstream/-u`: Upstream service address (e.g., "httpbin.org:80")
- `--bind/-b`: Listen address (default "0.0.0.0:9998")
- `--throttle-chance/-t`: Probability of throttling (0.0-1.0)
- `--close-chance/-c`: Probability of closing connection (0.0-1.0)
- `--rate/-r`: Bandwidth limit in bytes/second
- `--delay/-d`: Initial delay before data transmission
- `--admin/-a`: Enable web dashboard
- `--admin-port/-p`: Dashboard port (default 6000)

### Admin Dashboard (`admin/`)

Web interface providing:

- Current proxy configuration display
- Real-time list of active connections
- Connection statistics (duration, bytes transferred, etc.)
- Settings viewer
- Dashboard templates and frontend libraries:
  - [HTMX](https://github.com/bigskysoftware/htmx) - Dynamic HTML updates
  - [surreal](https://github.com/gnat/surreal) - Lightweight DOM library
  - [pollinator](https://github.com/inspmoore/pollinator) - Polling utility
  - [Bootstrap 5.3.3](https://github.com/twbs/bootstrap) - Responsive CSS framework - Bootstrap 5.3.3 - Responsive CSS framework

### Executor (`executor.go`)

Concurrent job orchestration pattern:

- Runs independent jobs in parallel using goroutines
- Supports context-aware jobs for cancellation
- Provides panic recovery and completion handlers
- Used for managing proxy listeners and admin services

### Metrics (`metrics.go`)

Collects and exposes proxy metrics:

- Connection counts and statistics
- Data transfer rates and totals
- Latency measurements
- State tracking for dashboard

### Graceful Shutdown (`gracefullquit.go`)

Signal handling for clean termination:

- Captures SIGTERM and SIGINT signals
- Provides callback mechanisms for services
- Ensures all connections are properly closed before exit

## Dependencies

**Main Dependencies** (from go.mod):

- `github.com/sirupsen/logrus` - Structured logging
- `github.com/spf13/pflag` - CLI flag parsing
- `github.com/oklog/run` - Concurrent task management
- `github.com/zserge/metric` - Metrics collection
- `goji.io` - Web framework for dashboard
- `github.com/google/uuid` - Connection identification
- `github.com/dustin/go-humanize` - Human-readable formatting

**Go Version**: 1.22+

## Testing & Quality

- **Test Coverage**: Includes unit tests for core components
  - `executor_test.go` - Executor concurrency tests
  - `proxy_test.go` - Proxy functionality tests
  - `gracefullquit_test.go` - Signal handling tests
  - `names_test.go` - Name generation tests

- **Testing Framework**: GoConvey for BDD-style testing
- **Assertions**: Smarty assertions library for clear test failures

## Common Use Cases

1. **Mobile App Testing**: Simulate real-world cellular network conditions (slow 3G, poor WiFi)
2. **Timeout Testing**: Verify applications handle connection delays correctly
3. **Circuit Breaker Testing**: Test fallback mechanisms and retry logic
4. **Load Testing**: Combine with load generators to test under degraded conditions
5. **Chaos Engineering**: Introduce controlled failures for resilience testing
