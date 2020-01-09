package slowjoe

import (
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
)

type shutdowner interface {
	start()
	register(hook func()) int64
	unregister(ID int64)
}

type SignalShutdowner struct {
	shutdownHookID  int64
	shutdownHooks   map[int64]func()
	hooksLock       sync.Mutex
	shutdownCleanup sync.Once
}

func (s *SignalShutdowner) register(hook func()) int64 {
	s.hooksLock.Lock()
	defer s.hooksLock.Unlock()
	if s.shutdownHooks == nil {
		s.shutdownHooks = make(map[int64]func())
	}
	ID := s.shutdownHookID
	s.shutdownHookID++
	s.shutdownHooks[ID] = hook
	return ID
}

func (s *SignalShutdowner) unregister(ID int64) {
	s.hooksLock.Lock()
	defer s.hooksLock.Unlock()
	if s.shutdownHooks == nil {
		return
	}
	logrus.WithField("hookid", ID).Tracef("Unregistering hook")
	delete(s.shutdownHooks, ID)
}

func (s *SignalShutdowner) callShutdownHooks() {
	s.hooksLock.Lock()
	defer s.hooksLock.Unlock()
	if s.shutdownHooks == nil {
		return
	}
	logrus.WithField("hooks", len(s.shutdownHooks)).Tracef("Calling shutdown hooks")
	for ID, hook := range s.shutdownHooks {
		logrus.WithField("hookid", ID).Tracef("Before hook")
		hook()
	}
}

func (s *SignalShutdowner) start() {
	var gracefulStop = make(chan os.Signal)
	signal.Notify(gracefulStop, syscall.SIGTERM)
	signal.Notify(gracefulStop, syscall.SIGINT)
	go func() {
		sig := <-gracefulStop
		logrus.Infof("Caught signal: %+v", sig)
		s.TryExit()
	}()
}

func (s *SignalShutdowner) TryExit() {
	s.shutdownCleanup.Do(func() {
		s.callShutdownHooks()
		logrus.Info("Wait for 2 second to finish processing")
		time.Sleep(2 * time.Second)
		os.Exit(0)
	})
}
