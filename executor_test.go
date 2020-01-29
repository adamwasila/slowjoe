package slowjoe

import (
	"context"
	"strings"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func TestEmptyExecutor(t *testing.T) {
	Convey("Given empty executor", t, func() {
		e := Executor()
		Convey("When empty executor runs it should not panic", func() {
			So(func() {
				e.Run(context.Background())
			}, ShouldNotPanic)
		})
	})
}

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
			}),
		)
		Convey("When executor runs", func() {
			e.Run(context.Background())
			result := readChannel(response)

			Convey("Then all operations should be executed concurrently in correct order determined by internal delays", func() {
				So(result, ShouldEqual, "ABCcba")
			})
		})
	})
}

func TestExecutionFinalizer(t *testing.T) {
	Convey("Given executor single operation and finalizer", t, func() {
		var response chan rune = make(chan rune, 6)
		e := Executor(
			Execute(func() {
				response <- 'A'
			}),
			WhenAllFinished(func() {
				response <- 'F'
			}),
		)
		Convey("When executor runs", func() {
			e.Run(context.Background())
			result := readChannel(response)

			Convey("Then operation should run then finalizer", func() {
				So(result, ShouldEqual, "AF")
			})
		})
	})
}

func TestExecutionWithTwoFinalizers(t *testing.T) {
	Convey("Given executor single operation and two finalizers", t, func() {
		var response chan rune = make(chan rune, 6)
		e := Executor(
			Execute(func() {
				response <- 'A'
			}),
			Execute(func() {
				time.Sleep(100 * time.Millisecond)
				response <- 'B'
			}),
			WhenAllFinished(func() {
				response <- 'X'
			}),
			WhenAllFinished(func() {
				response <- 'F'
			}),
		)
		Convey("When executor runs", func() {
			e.Run(context.Background())
			result := readChannel(response)

			Convey("Then operations should run and only last defined finalizer", func() {
				So(result, ShouldEqual, "ABF")
			})
		})
	})
}

func TestExecutionFinalizerFirst(t *testing.T) {
	Convey("Given executor with finalizer defined before operation", t, func() {
		var response chan rune = make(chan rune, 6)
		e := Executor(
			WhenAllFinished(func() {
				response <- 'F'
			}),
			Execute(func() {
				response <- 'A'
			}),
		)
		Convey("When executor runs", func() {
			e.Run(context.Background())
			result := readChannel(response)

			Convey("Then all operation should still be executed before finalizer", func() {
				So(result, ShouldEqual, "AF")
			})
		})
	})
}

func TestExecutionOnlyFinalizer(t *testing.T) {
	Convey("Given executor with no operation", t, func() {
		var response chan rune = make(chan rune, 6)
		e := Executor(
			WhenAllFinished(func() {
				response <- 'F'
			}),
		)
		Convey("When executor runs", func() {
			e.Run(context.Background())
			result := readChannel(response)

			Convey("Then all operations should be executed concurrently in correct order determined by internal delays", func() {
				So(result, ShouldEqual, "F")
			})
		})
	})
}

func TestExecutionCatchingPanic(t *testing.T) {
	Convey("Given executor with panic handler", t, func() {
		var response chan rune = make(chan rune, 6)
		e := Executor(
			Execute(func() {
				response <- 'A'
				panic('p')
			}),
			WhenPanic(func(p interface{}) {
				response <- 'P'
				response <- p.(rune)
			}),
		)
		Convey("When executor runs it should not panic", func() {
			e.Run(context.Background())
			result := readChannel(response)

			Convey("Then panic is succesfuly recovered", func() {
				So(result, ShouldEqual, "APp")
			})
		})
	})
}

func TestNilContextJobExecution(t *testing.T) {
	Convey("Given executor that has single job", t, func() {
		var receivedCtx context.Context = context.Background()
		e := Executor(
			ExecuteWithContext(func(ctx context.Context) {
				receivedCtx = ctx
			}),
		)
		Convey("When executor runs with nil context", func() {
			e.Run(nil)

			Convey("Then run should execute with no panic and context received in job should be nil as well", func() {
				So(receivedCtx, ShouldBeNil)
			})
		})
	})
}

func TestInterruptedJobExecution(t *testing.T) {
	Convey("Given executor that executes single job and checks for interrupt inbetween", t, func() {
		var response chan rune = make(chan rune, 6)
		e := Executor(
			ExecuteWithContext(func(ctx context.Context) {
				response <- 'A'

				time.Sleep(100 * time.Millisecond)

				if ctx.Err() != nil {
					response <- 'Q'
				}
				response <- 'B'

				time.Sleep(200 * time.Millisecond)

				if ctx.Err() != nil {
					response <- 'Q'
				}

				time.Sleep(100 * time.Millisecond)

				response <- 'C'

				if ctx.Err() != nil {
					response <- 'Q'
					return
				}
			}),
		)
		Convey("When executor runs but is interrupted in the middle", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer cancel()
			e.Run(ctx)
			result := readChannel(response)

			Convey("Then interrupt check should return true only after interrupt was called", func() {
				So(result, ShouldEqual, "ABQCQ")
			})
		})
	})
}

func TestInterruptedJobUsingChannelExecution(t *testing.T) {
	Convey("Given executor that executes single job and checks for interrupt inbetween using channel", t, func() {
		var response chan rune = make(chan rune, 6)
		e := Executor(
			ExecuteWithContext(func(ctx context.Context) {
				response <- 'A'

				select {
				case _, ok := <-ctx.Done():
					response <- 'Q'
					if ok {
						response <- 'O'
					}
				case <-time.After(100 * time.Millisecond):
					response <- 'T'
				}

				response <- 'B'

				select {
				case <-ctx.Done():
					response <- 'Q'
				case <-time.After(500 * time.Millisecond):
					response <- 'T'
				}
			}),
		)
		Convey("When executor runs but is interrupted in the middle", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer cancel()
			e.Run(ctx)
			time.Sleep(1 * time.Second)
			result := readChannel(response)

			Convey("Then channel should be closed only after executor job is interrupted", func() {
				So(result, ShouldEqual, "ATBQ")
			})
		})
	})
}

func readChannel(ch chan rune) string {
	close(ch)
	sb := strings.Builder{}
	for r := range ch {
		sb.WriteRune(r)
	}
	return sb.String()
}
