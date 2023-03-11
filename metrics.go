package slowjoe

import (
	"context"
	"expvar"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/adamwasila/slowjoe/admin"
	"github.com/oklog/run"
	"github.com/sirupsen/logrus"
	"github.com/zserge/metric"
	"goji.io"
	"goji.io/pat"
)

type Metrics struct {
	connectionsOpenedMetric           metric.Metric
	connectionsClosedMetric           metric.Metric
	connectionsClosedUpstreamMetric   metric.Metric
	connectionsClosedDownstreamMetric metric.Metric
	connectionsTimeMetric             metric.Metric
	connectionsTransferredBytes       metric.Metric
}

func (m *Metrics) ConnectionOpened(id, alias, typ string) {
	m.connectionsOpenedMetric.Add(float64(1))
}

func (m *Metrics) ConnectionProgressed(id, alias, direction string, transferredBytes int) {
	m.connectionsTransferredBytes.Add(float64(transferredBytes))
}

func (m *Metrics) ConnectionDelayed(id, alias, direction string, delay time.Duration) {
}

func (m *Metrics) ConnectionCompleted(id, alias, direction string, transferredBytes int, duration time.Duration) {
}

func (m *Metrics) ConnectionScheduledClose(id, alias string, delay time.Duration) {
}

func (m *Metrics) ConnectionClosedUpstream(id, alias string) {
	m.connectionsClosedUpstreamMetric.Add(float64(1))
}

func (m *Metrics) ConnectionClosedDownstream(id, alias string) {
	m.connectionsClosedDownstreamMetric.Add(float64(1))
}

func (m *Metrics) ConnectionClosed(id, alias string, d time.Duration) {
	m.connectionsClosedMetric.Add(float64(1))
	m.connectionsTimeMetric.Add(d.Seconds())
}

func (m *Metrics) Init(ctx context.Context, g *run.Group, adminPort int, data *admin.AdminData) {
	mux := goji.NewMux()
	mux.Handle(pat.Get("/debug/metrics"), metric.Handler(metric.Exposed))
	admin.AddRoutes(mux, data)
	server := &http.Server{Addr: fmt.Sprintf(":%d", adminPort), BaseContext: func(net.Listener) context.Context { return ctx }, Handler: mux}

	g.Add(func() error {
		logrus.WithField("port", adminPort).Infof("Start admin console")

		err := server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			logrus.Infof("HTTP handler closed with error: %s", err)
		}
		return nil
	}, func(error) {
		shutdownContext, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		err := server.Shutdown(shutdownContext)
		if err != nil {
			logrus.Errorf("Shutdown of admin console unclean: [%s]", err)
		}
	})

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
