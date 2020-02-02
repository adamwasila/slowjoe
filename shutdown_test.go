package slowjoe

import (
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func TestSignalsCatch(t *testing.T) {
	Convey("Given shutdowner default instance and cancel function instance", t, func() {
		cancelCalled := make(chan bool)
		cancel := func() {
			close(cancelCalled)
		}

		Convey("When hook is registered and gracefull stop routine is started", func() {
			CallForSignals(cancel)

			Convey("And when SIGINT signal is sent to the process", func() {

				err := syscall.Kill(syscall.Getpid(), syscall.SIGINT)
				So(err, ShouldBeNil)

				Convey("Then cancel function will be called shortly", func() {
					result := false
					select {
					case <-time.After(3 * time.Second):
						result = false
					case <-cancelCalled:
						result = true

					}
					So(result, ShouldBeTrue)
				})
			})

			Convey("And when SIGINT signal is NOT sent to the process", func() {

				Convey("Then cancel function will not be called", func() {
					result := false
					select {
					case <-time.After(100 * time.Millisecond):
						result = false
					case <-cancelCalled:
						result = true

					}
					So(result, ShouldBeFalse)
				})
			})

		})

	})
}

func TestSafeQuit(t *testing.T) {
	Convey("Given two types of safeguarded functions with parametrized sleep time", t, func() {
		var counter int32
		f := func(i int) {
			defer atomic.AddInt32(&counter, 1)
			defer StopBlocking(SafeBlock())
			time.Sleep(time.Duration(i) * time.Millisecond)
		}

		Convey("When 100 goroutines runs calling that function with duration from 1 to 100ms", func() {
			t0 := time.Now()

			for i := 1; i <= 100; i++ {
				go f(i)
			}

			SafeQuit()

			duration := time.Since(t0)

			Convey("SafeQuit won't return in less than 100ms and all goroutines completes until that time", func() {
				So(counter, ShouldEqual, 100)
				So(duration, ShouldBeGreaterThan, 100*time.Millisecond)

			})
		})

	})
}
