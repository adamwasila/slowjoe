package slowjoe

import (
	"os"
	"os/signal"
	"sync"
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

var safeGroup sync.WaitGroup

type safeBlock struct{}

// SafeBlock marks start of code block that should be guarded
func SafeBlock() safeBlock {
	safeGroup.Add(1)
	logrus.Trace("Entering safeguarded block")
	return safeBlock{}
}

// StopBlocking marks end of code block that should be guarded
func StopBlocking(safeBlock) {
	logrus.Trace("Leaving safeguarded block")
	safeGroup.Done()
}

// SafeQuit won't return if there is any guarded block of code
// that is still executed.
func SafeQuit() {
	logrus.Trace("Waiting for all safeguarded blocks to be leaved")
	safeGroup.Wait()
	logrus.Trace("It's now safe to turn off your application")
}
