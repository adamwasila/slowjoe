package slowjoe

import "time"

type instrumentation interface {
	ConnectionOpened(id, alias, typ string)
	ConnectionProgressed(id, alias, direction string, transferredBytes int)
	ConnectionDelayed(id, alias, direction string, delay time.Duration)
	ConnectionCompleted(id, alias, direction string, transferredBytes int, duration time.Duration)
	ConnectionClosedUpstream(id, alias string)
	ConnectionClosedDownstream(id, alias string)
	ConnectionClosed(id, alias string, duration time.Duration)
}

type instrumentations []instrumentation

func (ci instrumentations) Add(i instrumentation) instrumentations {
	ci = append(ci, i)
	return ci
}

func (ci instrumentations) ConnectionOpened(id, alias, typ string) {
	for _, i := range ci {
		i.ConnectionOpened(id, alias, typ)
	}
}

func (ci instrumentations) ConnectionProgressed(id, alias, direction string, transferredBytes int) {
	for _, i := range ci {
		i.ConnectionProgressed(id, alias, direction, transferredBytes)
	}
}

func (ci instrumentations) ConnectionCompleted(id, alias, direction string, transferredBytes int, duration time.Duration) {
	for _, i := range ci {
		i.ConnectionCompleted(id, alias, direction, transferredBytes, duration)
	}
}

func (ci instrumentations) ConnectionDelayed(id, alias, direction string, delay time.Duration) {
	for _, i := range ci {
		i.ConnectionDelayed(id, alias, direction, delay)
	}
}

func (ci instrumentations) ConnectionClosedUpstream(id, alias string) {
	for _, i := range ci {
		i.ConnectionClosedUpstream(id, alias)
	}
}

func (ci instrumentations) ConnectionClosedDownstream(id, alias string) {
	for _, i := range ci {
		i.ConnectionClosedDownstream(id, alias)
	}
}

func (ci instrumentations) ConnectionClosed(id, alias string, d time.Duration) {
	for _, i := range ci {
		i.ConnectionClosed(id, alias, d)
	}
}
