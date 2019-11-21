package main

import "time"

type instrumentation interface {
	ConnectionOpened()
	ConnectionProgressed(transferredBytes int)
	ConnectionClosedUpstream()
	ConnectionClosedDownstream()
	ConnectionClosed(d time.Duration)
}

type nopInstrumentation struct{}

func (*nopInstrumentation) ConnectionOpened()                         {}
func (*nopInstrumentation) ConnectionProgressed(transferredBytes int) {}
func (*nopInstrumentation) ConnectionClosedUpstream()                 {}
func (*nopInstrumentation) ConnectionClosedDownstream()               {}
func (*nopInstrumentation) ConnectionClosed(d time.Duration)          {}
