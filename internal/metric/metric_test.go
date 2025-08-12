package metric_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jkroepke/access-log-exporter/internal/config"
	"github.com/jkroepke/access-log-exporter/internal/metric"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/require"
)

func TestMetrics(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		cfg      config.Metric
		logLines []string
		metrics  string
		err      string
	}{
		{
			name: "simple metric",
			cfg: config.Metric{
				Name: "http_requests_total",
				Type: "counter",
				Help: "The total number of client requests.",
				Labels: []config.Label{
					{
						Name:      "host",
						LineIndex: 0,
					},
					{
						Name:      "method",
						LineIndex: 1,
					},
					{
						Name:      "status",
						LineIndex: 2,
					},
				},
			},
			logLines: []string{
				"example.com\tGET\t200",
			},
			metrics: `
# HELP http_requests_total The total number of client requests.
# TYPE http_requests_total counter
http_requests_total{host="example.com",method="GET",status="200"} 1`,
		},
		{
			name: "simple metric test math",
			cfg: config.Metric{
				Name: "http_requests_total",
				Type: "counter",
				Help: "The total number of client requests.",
				Labels: []config.Label{
					{
						Name:      "host",
						LineIndex: 0,
					},
					{
						Name:      "method",
						LineIndex: 1,
					},
					{
						Name:      "status",
						LineIndex: 2,
					},
				},
				Math: config.Math{
					Enabled: true,
					Mul:     4.0,
					Div:     4.0,
				},
			},
			logLines: []string{
				"example.com\tGET\t200",
			},
			metrics: `
# HELP http_requests_total The total number of client requests.
# TYPE http_requests_total counter
http_requests_total{host="example.com",method="GET",status="200"} 1`,
		},
		{
			name: "simple metric with incomplete log line",
			cfg: config.Metric{
				Name: "http_requests_total",
				Type: "counter",
				Help: "The total number of client requests.",
				Labels: []config.Label{
					{
						Name:      "host",
						LineIndex: 0,
					},
					{
						Name:      "method",
						LineIndex: 1,
					},
					{
						Name:      "status",
						LineIndex: 2,
					},
				},
			},
			logLines: []string{
				"example.com\tGET",
			},
			err: "line index out of range for label status, line length is 2",
		},
		{
			name: "simple metric with empty log line",
			cfg: config.Metric{
				Name: "http_requests_total",
				Type: "counter",
				Help: "The total number of client requests.",
				Labels: []config.Label{
					{
						Name:      "host",
						LineIndex: 0,
					},
					{
						Name:      "method",
						LineIndex: 1,
					},
					{
						Name:      "status",
						LineIndex: 2,
					},
				},
			},
			logLines: []string{
				"",
			},
			err: "",
		},
		{
			name: "simple preset",
			cfg: config.Metric{
				Name:       "http_response_duration_seconds",
				Type:       "histogram",
				Help:       "The time spent on receiving the response from the upstream server",
				ValueIndex: ptr(uint(3)),
				Buckets:    []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
				Math: config.Math{
					Enabled: true,
					Div:     1000,
				},
				Labels: []config.Label{
					{
						Name:      "host",
						LineIndex: 0,
					},
					{
						Name:      "method",
						LineIndex: 1,
					},
					{
						Name:      "status",
						LineIndex: 2,
					},
				},
			},
			logLines: []string{
				"app.example.net\tPUT\t500\t1.234\t4096\t512",
			},
			metrics: `
# HELP http_response_duration_seconds The time spent on receiving the response from the upstream server
# TYPE http_response_duration_seconds histogram
http_response_duration_seconds_bucket{host="app.example.net",method="PUT",status="500",le="0.005"} 1
http_response_duration_seconds_bucket{host="app.example.net",method="PUT",status="500",le="0.01"} 1
http_response_duration_seconds_bucket{host="app.example.net",method="PUT",status="500",le="0.025"} 1
http_response_duration_seconds_bucket{host="app.example.net",method="PUT",status="500",le="0.05"} 1
http_response_duration_seconds_bucket{host="app.example.net",method="PUT",status="500",le="0.1"} 1
http_response_duration_seconds_bucket{host="app.example.net",method="PUT",status="500",le="0.25"} 1
http_response_duration_seconds_bucket{host="app.example.net",method="PUT",status="500",le="0.5"} 1
http_response_duration_seconds_bucket{host="app.example.net",method="PUT",status="500",le="1"} 1
http_response_duration_seconds_bucket{host="app.example.net",method="PUT",status="500",le="2.5"} 1
http_response_duration_seconds_bucket{host="app.example.net",method="PUT",status="500",le="5"} 1
http_response_duration_seconds_bucket{host="app.example.net",method="PUT",status="500",le="10"} 1
http_response_duration_seconds_bucket{host="app.example.net",method="PUT",status="500",le="+Inf"} 1
http_response_duration_seconds_sum{host="app.example.net",method="PUT",status="500"} 0.001234
http_response_duration_seconds_count{host="app.example.net",method="PUT",status="500"} 1`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			met, err := metric.New(tc.cfg)
			require.NoError(t, err)

			for _, line := range tc.logLines {
				err := met.Parse(strings.Split(line, "\t"))
				if err != nil {
					if tc.err != "" {
						require.EqualError(t, err, tc.err)
					} else {
						require.NoError(t, err)
					}
				}
			}

			allMetrics, err := MetricsToText(t, met)
			require.NoError(t, err)

			require.Equal(t, allMetrics, strings.TrimSpace(tc.metrics))
		})
	}
}

func MetricsToText(tb testing.TB, met prometheus.Collector) (string, error) {
	tb.Helper()

	reg := prometheus.NewRegistry()
	err := reg.Register(met)
	require.NoError(tb, err)

	request, err := http.NewRequestWithContext(tb.Context(), http.MethodGet, "/", nil)
	require.NoError(tb, err)

	request.Header.Add("Accept", "text/plain")

	writer := httptest.NewRecorder()

	regHandler := promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
	regHandler.ServeHTTP(writer, request)

	require.Equal(tb, http.StatusOK, writer.Code)

	allMetrics, err := io.ReadAll(writer.Body)
	if err != nil {
		return "", fmt.Errorf("error reading writer body: %w", err)
	}

	return strings.TrimSpace(string(allMetrics)), nil
}

func ptr[T any](v T) *T {
	return &v
}
