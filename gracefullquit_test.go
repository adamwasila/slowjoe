package slowjoe

import (
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
			wait, _ := SetSignalCallback(cancel)
			go wait()

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
