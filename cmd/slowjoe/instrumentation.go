package main

import "time"

type instrumentation interface {
	ConnectionOpened(id, alias, typ string)
	ConnectionProgressed(id, direction string, transferredBytes int)
	ConnectionClosedUpstream(id string)
	ConnectionClosedDownstream(id string)
	ConnectionClosed(id string, d time.Duration)
}

type nopInstrumentation struct{}

func (*nopInstrumentation) ConnectionOpened(id, alias, typ string)                          {}
func (*nopInstrumentation) ConnectionProgressed(id, direction string, transferredBytes int) {}
func (*nopInstrumentation) ConnectionClosedUpstream(id string)                              {}
func (*nopInstrumentation) ConnectionClosedDownstream(id string)                            {}
func (*nopInstrumentation) ConnectionClosed(id string, d time.Duration)                     {}

type composedInstrumentation []instrumentation

func (ci composedInstrumentation) ConnectionOpened(id, alias, typ string) {
	for _, i := range ci {
		i.ConnectionOpened(id, alias, typ)
	}
}

func (ci composedInstrumentation) ConnectionProgressed(id, direction string, transferredBytes int) {
	for _, i := range ci {
		i.ConnectionProgressed(id, direction, transferredBytes)
	}
}

func (ci composedInstrumentation) ConnectionClosedUpstream(id string) {
	for _, i := range ci {
		i.ConnectionClosedUpstream(id)
	}
}

func (ci composedInstrumentation) ConnectionClosedDownstream(id string) {
	for _, i := range ci {
		i.ConnectionClosedDownstream(id)
	}
}

func (ci composedInstrumentation) ConnectionClosed(id string, d time.Duration) {
	for _, i := range ci {
		i.ConnectionClosed(id, d)
	}
}
