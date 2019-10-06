package main

import (
	"sync"
)

type executor struct {
	closeHandler func()
	panicHandler func(interface{})
	jobs         []func()
}

func Executor(ops ...func(*executor) error) *executor {
	e := &executor{}
	for _, op := range ops {
		op(e)
	}
	return e
}

func Execute(jobs ...func()) func(*executor) error {
	return func(e *executor) error {
		e.jobs = append(e.jobs, jobs...)
		return nil
	}
}

func WhenAllFinished(closeHandler func()) func(*executor) error {
	return func(e *executor) error {
		e.closeHandler = closeHandler
		return nil
	}
}

func WhenPanic(panicHandler func(interface{})) func(*executor) error {
	return func(e *executor) error {
		e.panicHandler = panicHandler
		return nil
	}
}

func (e *executor) Run() {
	var wg sync.WaitGroup
	for _, job := range e.jobs {
		wg.Add(1)
		go wrappedJob(&wg, e.panicHandler, job)
	}
	wg.Wait()
	e.closeHandler()
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
