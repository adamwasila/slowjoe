package main

import (
	"context"
	"expvar"
	"fmt"
	"net/http"

	"github.com/sirupsen/logrus"
	"github.com/zserge/metric"
)

type metrics struct {
	connectionsOpenedMetric           metric.Metric
	connectionsClosedMetric           metric.Metric
	connectionsClosedUpstreamMetric   metric.Metric
	connectionsClosedDownstreamMetric metric.Metric
}

func (m *metrics) ConnectionOpened(id string) {
	m.connectionsOpenedMetric.Add(float64(1))
}

func (m *metrics) ConnectionClosedUpstream(id string) {
	m.connectionsClosedUpstreamMetric.Add(float64(1))
}

func (m *metrics) ConnectionClosedDownstream(id string) {
	m.connectionsClosedDownstreamMetric.Add(float64(1))
}

func (m *metrics) ConnectionClosed(id string) {
	m.connectionsClosedMetric.Add(float64(1))
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
	m.connectionsOpenedMetric = metric.NewCounter("30m10s")
	m.connectionsClosedMetric = metric.NewCounter("30m10s")
	m.connectionsClosedUpstreamMetric = metric.NewCounter("30m10s")
	m.connectionsClosedDownstreamMetric = metric.NewCounter("30m10s")
	expvar.Publish("conn.opened", m.connectionsOpenedMetric)
	expvar.Publish("conn.closed", m.connectionsClosedMetric)
	expvar.Publish("conn.closed.upstream", m.connectionsClosedUpstreamMetric)
	expvar.Publish("conn.closed.downstream", m.connectionsClosedDownstreamMetric)
}
