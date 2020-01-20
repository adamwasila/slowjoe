package slowjoe

import (
	"math"
	"time"

	"github.com/sirupsen/logrus"
)

type logs struct {
	log *logrus.Logger
}

func (l *logs) ConnectionOpened(id, alias, typ string) {
	l.log.WithField("alias", alias).WithField("alias", alias).Debugf("New connection")
}

func (l *logs) ConnectionProgressed(id, alias, direction string, transferredBytes int) {
	l.log.WithField("alias", alias).WithField("direction", direction).WithField("bytes", transferredBytes).Trace("Transferred")
}

func (l *logs) ConnectionDelayed(id, alias, direction string, delay time.Duration) {
	l.log.WithField("alias", alias).WithField("direction", direction).WithField("duration", delay.Seconds()).Trace("Sleeping")
}

func (l *logs) ConnectionCompleted(id, alias, direction string, transferredBytes int, duration time.Duration) {
	l.log.WithField("alias", alias).
		WithField("direction", direction).
		WithField("duration", duration.Round(10*time.Millisecond)).
		WithField("rate", math.Round(float64(transferredBytes)/duration.Seconds())).
		WithField("bytes", transferredBytes).
		Info("Completed")

}

func (l *logs) ConnectionClosedUpstream(id, alias string) {
}

func (l *logs) ConnectionClosedDownstream(id, alias string) {
}

func (l *logs) ConnectionClosed(id, alias string, d time.Duration) {
	l.log.WithField("alias", alias).Tracef("Closed")
}
