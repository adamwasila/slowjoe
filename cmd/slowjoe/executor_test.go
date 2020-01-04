package main

import (
	"strings"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func TestSimpleConcurrentExecution(t *testing.T) {
	Convey("Given executor with concurrent operations", t, func() {
		var response chan rune = make(chan rune, 6)
		e := Executor(
			Execute(func() {
				time.Sleep(10 * time.Millisecond)
				response <- 'A'
				time.Sleep(50 * time.Millisecond)
				response <- 'a'
			}),
			Execute(func() {
				time.Sleep(20 * time.Millisecond)
				response <- 'B'
				time.Sleep(30 * time.Millisecond)
				response <- 'b'
			}),
			Execute(func() {
				time.Sleep(30 * time.Millisecond)
				response <- 'C'
				time.Sleep(10 * time.Millisecond)
				response <- 'c'
			}),
			Execute(func() {
				time.Sleep(100 * time.Millisecond)
				close(response)
			}),
		)
		Convey("When executor runs", func() {
			e.Run()
			sb := strings.Builder{}
			for r := range response {
				sb.WriteRune(r)
			}

			Convey("Then all operations should be executed concurrently in correct order determined by internal delays", func() {
				So(sb.String(), ShouldEqual, "ABCcba")
			})
		})
	})
}
