package nginx

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

const templateMetrics string = `Active connections: %d
server accepts handled requests
%d %d %d
Reading: %d Writing: %d Waiting: %d
`

type Collector struct {
	upMetric            prometheus.Gauge
	connectionsAccepted *prometheus.Desc
	connectionsActive   *prometheus.Desc
	connectionsHandled  *prometheus.Desc
	connectionsReading  *prometheus.Desc
	connectionsWaiting  *prometheus.Desc
	connectionsWriting  *prometheus.Desc
	logger              *slog.Logger
	scrapeURL           string
	mu                  sync.Mutex
}

// StubStats represents NGINX stub_status metrics.
type StubStats struct {
	Connections StubConnections
	Requests    int64
}

// StubConnections represents connections related metrics.
type StubConnections struct {
	Active   int64
	Accepted int64
	Handled  int64
	Reading  int64
	Writing  int64
	Waiting  int64
}

func New(logger *slog.Logger, scrapeURL string) *Collector {
	return &Collector{
		scrapeURL: scrapeURL,
		logger:    logger.With(slog.String("component", "nginx_collector")),
		upMetric: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "nginx_up",
			Help: "Whether the NGINX server is up (1) or down (0). 1 means the server is up and metrics are being collected, 0 means the server is down or unreachable.",
		}),
		connectionsAccepted: prometheus.NewDesc(
			"nginx_connections_accepted_total",
			"Accepted client connections.",
			nil, nil,
		),
		connectionsActive: prometheus.NewDesc(
			"nginx_connections_active",
			"Active client connections.",
			nil, nil,
		),
		connectionsHandled: prometheus.NewDesc(
			"nginx_connections_handled_total",
			"Handled client connections.",
			nil, nil,
		),
		connectionsReading: prometheus.NewDesc(
			"nginx_connections_reading",
			"Connections where NGINX is reading the request header.",
			nil, nil,
		),
		connectionsWaiting: prometheus.NewDesc(
			"nginx_connections_waiting",
			"Idle client connections.",
			nil, nil,
		),
		connectionsWriting: prometheus.NewDesc(
			"nginx_connections_writing",
			"Connections where NGINX is writing the response back to the client.",
			nil, nil,
		),
	}
}

func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.upMetric.Desc()

	ch <- c.connectionsAccepted

	ch <- c.connectionsActive

	ch <- c.connectionsHandled

	ch <- c.connectionsReading

	ch <- c.connectionsWaiting

	ch <- c.connectionsWriting
}

func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	c.logger.Error("hit Collect method")

	c.mu.Lock() // To protect metrics from concurrent collects
	defer c.mu.Unlock()

	//nolint:noctx
	resp, err := http.Get(c.scrapeURL)
	if err != nil {
		c.upMetric.Set(0)
		c.logger.Error("Failed to scrape NGINX metrics",
			slog.String("url", c.scrapeURL),
			slog.Any("error", err),
		)

		ch <- c.upMetric

		return
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		c.upMetric.Set(0)

		c.logger.Error("NGINX metrics endpoint returned non-200 status code",
			slog.String("url", c.scrapeURL),
			slog.Int("status_code", resp.StatusCode),
		)

		ch <- c.upMetric

		return
	}

	stats, err := parseStubStats(resp.Body)
	if err != nil {
		c.upMetric.Set(0)
		c.logger.Error("Failed to parse NGINX metrics",
			slog.String("url", c.scrapeURL),
			slog.Any("error", err),
		)

		ch <- c.upMetric

		return
	}

	c.upMetric.Set(1)

	ch <- c.upMetric

	ch <- prometheus.MustNewConstMetric(c.connectionsActive,
		prometheus.GaugeValue, float64(stats.Connections.Active))

	ch <- prometheus.MustNewConstMetric(c.connectionsAccepted,
		prometheus.CounterValue, float64(stats.Connections.Accepted))

	ch <- prometheus.MustNewConstMetric(c.connectionsHandled,
		prometheus.CounterValue, float64(stats.Connections.Handled))

	ch <- prometheus.MustNewConstMetric(c.connectionsReading,
		prometheus.GaugeValue, float64(stats.Connections.Reading))

	ch <- prometheus.MustNewConstMetric(c.connectionsWriting,
		prometheus.GaugeValue, float64(stats.Connections.Writing))

	ch <- prometheus.MustNewConstMetric(c.connectionsWaiting,
		prometheus.GaugeValue, float64(stats.Connections.Waiting))
}

func parseStubStats(r io.Reader) (*StubStats, error) {
	var stubStats StubStats
	if _, err := fmt.Fscanf(r, templateMetrics,
		&stubStats.Connections.Active,
		&stubStats.Connections.Accepted,
		&stubStats.Connections.Handled,
		&stubStats.Requests,
		&stubStats.Connections.Reading,
		&stubStats.Connections.Writing,
		&stubStats.Connections.Waiting); err != nil {
		return nil, fmt.Errorf("failed to scan template metrics: %w", err)
	}

	return &stubStats, nil
}
