package main

import (
	"context"
	"expvar"
	"fmt"
	"net/http"
	"sync/atomic"

	"github.com/sirupsen/logrus"
	"github.com/zserge/metric"
)

type metrics struct {
	activeConnections       int64
	activeConnectionsMetric metric.Metric
}

func (m *metrics) ConnectionOpened(id string) {
	val := atomic.AddInt64(&m.activeConnections, 1)
	m.activeConnectionsMetric.Add(float64(val))
}

func (m *metrics) ConnectionClosedUpstream(id string) {
}

func (m *metrics) ConnectionClosedDownstream(id string) {

}

func (m *metrics) ConnectionClosed(id string) {
	val := atomic.AddInt64(&m.activeConnections, -1)
	m.activeConnectionsMetric.Add(float64(val))
}

func (m *metrics) init(adminPort int) {
	go func() {
		http.Handle("/", MainPageHandler())
		http.Handle("/debug/metrics", metric.Handler(metric.Exposed))
		logrus.WithField("port", adminPort).Infof("Start admin console")
		server := &http.Server{Addr: fmt.Sprintf(":%d", adminPort), Handler: http.DefaultServeMux}
		registerShutdownHook(func() {
			err := server.Shutdown(context.Background())
			if err != nil {
				logrus.Errorf("Shutdown of admin console unclean: [%s]", err)
			}
		})
		err := server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			logrus.Infof("HTTP handler closed with error: %s", err)
		}
	}()
	m.activeConnectionsMetric = metric.NewGauge("15m1m")
	expvar.Publish("connections:active", m.activeConnectionsMetric)
}
