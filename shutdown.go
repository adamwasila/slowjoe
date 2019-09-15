package main

import (
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
)

var (
	shutdownHookID int64
	shutdownHooks  = make(map[int64]func())
	hooksLock      = sync.RWMutex{}
)

func registerShutdownHook(hook func()) int64 {
	hooksLock.Lock()
	defer hooksLock.Unlock()
	ID := shutdownHookID
	shutdownHookID++
	shutdownHooks[ID] = hook
	return ID
}

func unregisterShutdownHook(ID int64) {
	hooksLock.Lock()
	defer hooksLock.Unlock()
	logrus.WithField("hookid", ID).Debugf("Unregistering hook")
	delete(shutdownHooks, ID)
}

func callShutdownHooks() {
	hooksLock.Lock()
	defer hooksLock.Unlock()
	logrus.WithField("hooks", len(shutdownHooks)).Debugf("Calling shutdown hooks")
	for ID, hook := range shutdownHooks {
		logrus.WithField("hookid", ID).Tracef("Before hook")
		hook()
	}
}

func setupGracefulStop() {
	var gracefulStop = make(chan os.Signal)
	signal.Notify(gracefulStop, syscall.SIGTERM)
	signal.Notify(gracefulStop, syscall.SIGINT)
	go func() {
		sig := <-gracefulStop
		logrus.Debugf("Caught signal: %+v", sig)
		callShutdownHooks()
		logrus.Info("Wait for 2 second to finish processing")
		time.Sleep(2 * time.Second)
		os.Exit(0)
	}()
}
