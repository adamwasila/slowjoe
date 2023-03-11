package slowjoe

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/sirupsen/logrus"
)

// SetSignalCallback registers callback function to be called
// when one of SIGTERM, SIGINT signals is received
func SetSignalCallback(callOnSignal func()) {
	var signalChan = make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		sig := <-signalChan
		logrus.Infof("Caught signal: %+v", sig)
		callOnSignal()
	}()
}
