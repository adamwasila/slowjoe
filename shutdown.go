package slowjoe

import (
	"os"
	"os/signal"
	"sync"
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

var safeGroup sync.WaitGroup

type safeBlock struct{}

// SafeBlock marks start of code block that should be guarded
func SafeBlock() safeBlock {
	safeGroup.Add(1)
	return safeBlock{}
}

// StopBlocking marks end of code block that should be guarded
func StopBlocking(safeBlock) {
	safeGroup.Done()
}

// SafeQuit won't return if there is any guarded block of code
// that is still executed.
func SafeQuit() {
	safeGroup.Wait()
}
