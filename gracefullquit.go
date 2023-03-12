package slowjoe

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/sirupsen/logrus"
)

// SetSignalCallback registers callback function to be called
// when one of SIGTERM, SIGINT signals is received.
// Returns two functions:
// first one should be called from separate goroutine as it is returing only
// if signal is received; it calls callback before returning
// second one is cancelling waiting for signal; callback won't be called.
func SetSignalCallback(callOnSignal func()) (wait, cancel func()) {
	cancelChan := make(chan struct{})

	wait = func() {
		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, syscall.SIGTERM, syscall.SIGINT)
		select {
		case sig := <-signalChan:
			logrus.Infof("Caught signal: %+v", sig)
			callOnSignal()
		case <-cancelChan:
			logrus.Trace("No longer catching signals")
		}
	}
	cancel = func() {
		close(cancelChan)
	}
	return wait, cancel
}
