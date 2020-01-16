package slowjoe

import (
	"sync/atomic"
	"syscall"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestShutdowner(t *testing.T) {
	Convey("Given shutdowner default instance and shutdown hook", t, func() {
		s := SignalShutdowner{}
		var hookCounter int32 = 0
		hook := func() {
			atomic.AddInt32(&hookCounter, 1)
		}

		Convey("When hook is registered and shutdown is called", func() {
			s.register(hook)
			s.callShutdownHooks()

			Convey("Hook will be called on shutdown", func() {
				So(hookCounter, ShouldEqual, 1)
			})
		})

		Convey("When hook is registered and deregistered and shutdown is called", func() {
			id := s.register(hook)
			s.unregister(id)
			s.callShutdownHooks()

			Convey("Hook will not be called on shutdown", func() {
				So(hookCounter, ShouldEqual, 0)
			})
		})

		Convey("When hook is registered and tryExit is called twice", func() {
			s.register(hook)

			s.TryExit(func(int) {})
			s.TryExit(func(int) {})

			Convey("Shutdown hooks will be called once", func() {
				So(hookCounter, ShouldEqual, 1)
			})
		})

		Convey("When hook is registered and gracefull stop routine is started", func() {
			s.register(hook)
			s.start(func(int) {})

			Convey("And when SIGINT signal is sent to the process and tryExit will be used to wait", func() {

				syscall.Kill(syscall.Getpid(), syscall.SIGINT)
				s.TryExit(func(int) {})

				Convey("Shutdown hooks will be called once", func() {
					So(hookCounter, ShouldEqual, 1)
				})
			})

			Convey("And when SIGTERM signal is sent to the process and tryExit will be used to wait", func() {

				syscall.Kill(syscall.Getpid(), syscall.SIGINT)
				s.TryExit(func(int) {})

				Convey("Shutdown hooks will be called once", func() {
					So(hookCounter, ShouldEqual, 1)
				})
			})

		})

	})
}
