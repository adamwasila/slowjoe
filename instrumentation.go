package slowjoe

import "time"

type Instrumentation interface {
	ConnectionOpened(id, alias, typ string)
	ConnectionProgressed(id, alias, direction string, transferredBytes int)
	ConnectionDelayed(id, alias, direction string, delay time.Duration)
	ConnectionCompleted(id, alias, direction string, transferredBytes int, duration time.Duration)
	ConnectionScheduledClose(id, alias string, delay time.Duration)
	ConnectionClosedUpstream(id, alias string)
	ConnectionClosedDownstream(id, alias string)
	ConnectionClosed(id, alias string, duration time.Duration)
}

type Instrumentations []Instrumentation

func (ci Instrumentations) Add(i Instrumentation) Instrumentations {
	ci = append(ci, i)
	return ci
}

func (ci Instrumentations) ConnectionOpened(id, alias, typ string) {
	for _, i := range ci {
		i.ConnectionOpened(id, alias, typ)
	}
}

func (ci Instrumentations) ConnectionProgressed(id, alias, direction string, transferredBytes int) {
	for _, i := range ci {
		i.ConnectionProgressed(id, alias, direction, transferredBytes)
	}
}

func (ci Instrumentations) ConnectionCompleted(id, alias, direction string, transferredBytes int, duration time.Duration) {
	for _, i := range ci {
		i.ConnectionCompleted(id, alias, direction, transferredBytes, duration)
	}
}

func (ci Instrumentations) ConnectionScheduledClose(id, alias string, delay time.Duration) {
	for _, i := range ci {
		i.ConnectionScheduledClose(id, alias, delay)
	}
}

func (ci Instrumentations) ConnectionDelayed(id, alias, direction string, delay time.Duration) {
	for _, i := range ci {
		i.ConnectionDelayed(id, alias, direction, delay)
	}
}

func (ci Instrumentations) ConnectionClosedUpstream(id, alias string) {
	for _, i := range ci {
		i.ConnectionClosedUpstream(id, alias)
	}
}

func (ci Instrumentations) ConnectionClosedDownstream(id, alias string) {
	for _, i := range ci {
		i.ConnectionClosedDownstream(id, alias)
	}
}

func (ci Instrumentations) ConnectionClosed(id, alias string, d time.Duration) {
	for _, i := range ci {
		i.ConnectionClosed(id, alias, d)
	}
}
