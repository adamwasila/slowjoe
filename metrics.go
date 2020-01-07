package slowjoe

import (
	"context"
	"expvar"
	"fmt"
	"net/http"
	"time"

	"github.com/adamwasila/slowjoe/admin"
	"github.com/sirupsen/logrus"
	"github.com/zserge/metric"
	"goji.io"
	"goji.io/pat"
)

type metrics struct {
	connectionsOpenedMetric           metric.Metric
	connectionsClosedMetric           metric.Metric
	connectionsClosedUpstreamMetric   metric.Metric
	connectionsClosedDownstreamMetric metric.Metric
	connectionsTimeMetric             metric.Metric
	connectionsTransferredBytes       metric.Metric
}

func (m *metrics) ConnectionOpened(id, alias, typ string) {
	m.connectionsOpenedMetric.Add(float64(1))
}

func (m *metrics) ConnectionProgressed(id, direction string, transferredBytes int) {
	m.connectionsTransferredBytes.Add(float64(transferredBytes))
}

func (m *metrics) ConnectionClosedUpstream(id string) {
	m.connectionsClosedUpstreamMetric.Add(float64(1))
}

func (m *metrics) ConnectionClosedDownstream(id string) {
	m.connectionsClosedDownstreamMetric.Add(float64(1))
}

func (m *metrics) ConnectionClosed(id string, d time.Duration) {
	m.connectionsClosedMetric.Add(float64(1))
	m.connectionsTimeMetric.Add(d.Seconds())
}

func (m *metrics) init(adminPort int, data *admin.AdminData, sh shutdowner) {
	go func() {
		mux := goji.NewMux()

		mux.Handle(pat.Get("/debug/metrics"), metric.Handler(metric.Exposed))

		admin.AddRoutes(mux, data)

		logrus.WithField("port", adminPort).Infof("Start admin console")
		server := &http.Server{Addr: fmt.Sprintf(":%d", adminPort), Handler: mux}
		sh.register(func() {
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
	m.connectionsTimeMetric = metric.NewHistogram("30m10s")
	m.connectionsTransferredBytes = metric.NewCounter("30m10s")
	expvar.Publish("conn.opened", m.connectionsOpenedMetric)
	expvar.Publish("conn.closed", m.connectionsClosedMetric)
	expvar.Publish("conn.closed.upstream", m.connectionsClosedUpstreamMetric)
	expvar.Publish("conn.closed.downstream", m.connectionsClosedDownstreamMetric)
	expvar.Publish("conn.time", m.connectionsTimeMetric)
	expvar.Publish("conn.bytes", m.connectionsTransferredBytes)
}
