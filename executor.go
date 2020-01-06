package slowjoe

import (
	"sync"
)

type executor struct {
	closeHandler func()
	panicHandler func(interface{})
	jobs         []func()
}

// Runner is the interface that wraps basic, argumentless Run method
type Runner interface {
	Run()
}

// Executor returns new instance of concurrent jobs executor
func Executor(ops ...func(*executor)) Runner {
	e := &executor{}
	for _, op := range ops {
		op(e)
	}
	return e
}

// Execute wraps list of plain, argumentless funtions to be independent jobs
// to be executed concurrently
func Execute(jobs ...func()) func(*executor) {
	return func(e *executor) {
		e.jobs = append(e.jobs, jobs...)
	}
}

// WhenAllFinished adds handler that is called when all jobs finishes. It will
// be called only once and only when all jobs quit no matter of the result
// or panic they raise.
func WhenAllFinished(closeHandler func()) func(*executor) {
	return func(e *executor) {
		e.closeHandler = closeHandler
	}
}

// WhenPanic adds handler called for any job that panics. Note that handler
// must be reentrant as may be called multiple times by different goroutines
func WhenPanic(panicHandler func(interface{})) func(*executor) {
	return func(e *executor) {
		e.panicHandler = panicHandler
	}
}

// Run executes all jobs concurrently, call handlers - if needed and provided, then returns
func (e *executor) Run() {
	var wg sync.WaitGroup
	for _, job := range e.jobs {
		wg.Add(1)
		go wrappedJob(&wg, e.panicHandler, job)
	}
	wg.Wait()
	if e.closeHandler != nil {
		e.closeHandler()
	}
}

func wrappedJob(wg *sync.WaitGroup, onPanic func(interface{}), f func()) {
	defer wg.Done()
	if onPanic != nil {
		defer func() {
			p := recover()
			if p != nil {
				onPanic(p)
			}
		}()
	}
	f()
}
