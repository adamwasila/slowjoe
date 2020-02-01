package slowjoe

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/sirupsen/logrus"
)

func CallForSignals(cancel func()) {
	var gracefulStop = make(chan os.Signal, 1)
	signal.Notify(gracefulStop, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		sig := <-gracefulStop
		logrus.Infof("Caught signal: %+v", sig)
		cancel()
	}()
}
