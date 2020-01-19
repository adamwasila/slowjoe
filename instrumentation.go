package slowjoe

import "time"

type instrumentation interface {
	ConnectionOpened(id, alias, typ string)
	ConnectionProgressed(id, direction string, transferredBytes int)
	ConnectionDelayed(id, direction string, delay time.Duration)
	ConnectionClosedUpstream(id string)
	ConnectionClosedDownstream(id string)
	ConnectionClosed(id string, d time.Duration)
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

func (ci instrumentations) ConnectionProgressed(id, direction string, transferredBytes int) {
	for _, i := range ci {
		i.ConnectionProgressed(id, direction, transferredBytes)
	}
}

func (ci instrumentations) ConnectionDelayed(id, direction string, delay time.Duration) {
	for _, i := range ci {
		i.ConnectionDelayed(id, direction, delay)
	}
}

func (ci instrumentations) ConnectionClosedUpstream(id string) {
	for _, i := range ci {
		i.ConnectionClosedUpstream(id)
	}
}

func (ci instrumentations) ConnectionClosedDownstream(id string) {
	for _, i := range ci {
		i.ConnectionClosedDownstream(id)
	}
}

func (ci instrumentations) ConnectionClosed(id string, d time.Duration) {
	for _, i := range ci {
		i.ConnectionClosed(id, d)
	}
}
