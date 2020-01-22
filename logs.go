package slowjoe

import (
	"math"
	"time"

	"github.com/sirupsen/logrus"
)

type Logs struct {
	Log *logrus.Logger
}

func (l *Logs) ConnectionOpened(id, alias, typ string) {
	l.Log.WithField("alias", alias).WithField("alias", alias).Debugf("New connection")
}

func (l *Logs) ConnectionProgressed(id, alias, direction string, transferredBytes int) {
	l.Log.WithField("alias", alias).WithField("direction", direction).WithField("bytes", transferredBytes).Trace("Transferred")
}

func (l *Logs) ConnectionDelayed(id, alias, direction string, delay time.Duration) {
	l.Log.WithField("alias", alias).WithField("direction", direction).WithField("duration", delay.Seconds()).Trace("Sleeping")
}

func (l *Logs) ConnectionCompleted(id, alias, direction string, transferredBytes int, duration time.Duration) {
	l.Log.WithField("alias", alias).
		WithField("direction", direction).
		WithField("duration", duration.Round(10*time.Millisecond)).
		WithField("rate", math.Round(float64(transferredBytes)/duration.Seconds())).
		WithField("bytes", transferredBytes).
		Info("Completed")

}

func (l *Logs) ConnectionScheduledClose(id, alias string, delay time.Duration) {
	l.Log.WithField("alias", alias).WithField("delay", delay).Trace("Scheduling close")
}

func (l *Logs) ConnectionClosedUpstream(id, alias string) {
}

func (l *Logs) ConnectionClosedDownstream(id, alias string) {
}

func (l *Logs) ConnectionClosed(id, alias string, d time.Duration) {
	l.Log.WithField("alias", alias).Tracef("Closed")
}
