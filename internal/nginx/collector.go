package nginx

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// The requests value from the "server accepts handled requests" line is parsed only to match the stub_status format.
const templateMetrics string = `Active connections: %d
server accepts handled requests
%d %d %d
Reading: %d Writing: %d Waiting: %d
`

const (
	defaultScrapeTimeout = time.Second
	defaultServerVersion = "N/A"
	userAgent            = "jkroepke/access-log-exporter"
)

type Collector struct {
	upMetric            *prometheus.Desc
	connectionsAccepted *prometheus.Desc
	connectionsActive   *prometheus.Desc
	connectionsHandled  *prometheus.Desc
	connectionsReading  *prometheus.Desc
	connectionsWaiting  *prometheus.Desc
	connectionsWriting  *prometheus.Desc
	logger              *slog.Logger
	client              *http.Client
	scrapeURL           string
	timeout             time.Duration
}

// StubStats represents NGINX stub_status metrics.
type StubStats struct {
	Connections StubConnections
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

type Option func(*Collector)

func WithHTTPClient(client *http.Client) Option {
	return func(c *Collector) {
		if client != nil {
			c.client = client
		}
	}
}

func WithTimeout(timeout time.Duration) Option {
	return func(c *Collector) {
		if timeout > 0 {
			c.timeout = timeout
		}
	}
}

func New(logger *slog.Logger, scrapeURL string, opts ...Option) *Collector {
	collector := &Collector{
		scrapeURL: scrapeURL,
		logger:    logger.With(slog.String("component", "nginx_collector")),
		client:    http.DefaultClient,
		timeout:   defaultScrapeTimeout,
		upMetric: prometheus.NewDesc(
			"nginx_up",
			"Whether the NGINX server is up (1) or down (0). 1 means the server is up and metrics are being collected, 0 means the server is down or unreachable.",
			[]string{"version"}, nil,
		),
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

	if client, ok := newUnixHTTPClient(scrapeURL); ok {
		collector.client = client
		collector.scrapeURL = "http://unix/"
	}

	for _, opt := range opts {
		opt(collector)
	}

	return collector
}

func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.upMetric

	ch <- c.connectionsAccepted

	ch <- c.connectionsActive

	ch <- c.connectionsHandled

	ch <- c.connectionsReading

	ch <- c.connectionsWaiting

	ch <- c.connectionsWriting
}

func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	serverVersion := defaultServerVersion

	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.scrapeURL, nil)
	if err != nil {
		c.logger.Error(
			"Failed to create HTTP request for NGINX metrics",
			slog.String("url", c.scrapeURL),
			slog.Any("error", err),
		)

		c.collectUp(ch, 0, serverVersion)

		return
	}

	req.Header.Set("User-Agent", userAgent)

	resp, err := c.client.Do(req)
	if err != nil {
		c.logger.Error(
			"Failed to scrape NGINX metrics",
			slog.String("url", c.scrapeURL),
			slog.Any("error", err),
		)

		c.collectUp(ch, 0, serverVersion)

		return
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		c.logger.Error(
			"NGINX metrics endpoint returned non-200 status code",
			slog.String("url", c.scrapeURL),
			slog.Int("status_code", resp.StatusCode),
		)

		c.collectUp(ch, 0, serverVersion)

		return
	}

	// Attempt to read the server version from the response header
	if version := resp.Header.Get("Server"); strings.HasPrefix(version, "nginx/") {
		serverVersion = strings.TrimPrefix(version, "nginx/")
	}

	stats, err := parseStubStats(resp.Body)
	if err != nil {
		c.logger.Error(
			"Failed to parse NGINX metrics",
			slog.String("url", c.scrapeURL),
			slog.Any("error", err),
		)

		c.collectUp(ch, 0, serverVersion)

		return
	}

	c.collectUp(ch, 1, serverVersion)

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

func (c *Collector) collectUp(ch chan<- prometheus.Metric, value float64, serverVersion string) {
	ch <- prometheus.MustNewConstMetric(c.upMetric,
		prometheus.GaugeValue, value, serverVersion)
}

func newUnixHTTPClient(scrapeURL string) (*http.Client, bool) {
	parsedURL, err := url.Parse(scrapeURL)
	if err != nil || parsedURL.Scheme != "unix" || parsedURL.Path == "" {
		return nil, false
	}

	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, false
	}

	unixTransport := transport.Clone()
	unixTransport.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
		var dialer net.Dialer

		return dialer.DialContext(ctx, "unix", parsedURL.Path)
	}

	return &http.Client{Transport: unixTransport}, true
}

func parseStubStats(reader io.Reader) (StubStats, error) {
	var (
		stubStats       StubStats
		ignoredRequests int64
	)

	if _, err := fmt.Fscanf(reader, templateMetrics,
		&stubStats.Connections.Active,
		&stubStats.Connections.Accepted,
		&stubStats.Connections.Handled,
		&ignoredRequests,
		&stubStats.Connections.Reading,
		&stubStats.Connections.Writing,
		&stubStats.Connections.Waiting); err != nil {
		return StubStats{}, fmt.Errorf("failed to scan template metrics: %w", err)
	}

	return stubStats, nil
}
