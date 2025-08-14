package metric_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/jkroepke/access-log-exporter/internal/config"
	"github.com/jkroepke/access-log-exporter/internal/config/types"
	"github.com/jkroepke/access-log-exporter/internal/metric"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/require"
)

func TestMetrics(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name      string
		cfg       config.Metric
		logLines  []string
		metrics   string
		metricErr string
		parseErr  string
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
			parseErr: "line index out of range for label status, line length is 2",
		},
		{
			name: "simple metric with out of range value index",
			cfg: config.Metric{
				Name:       "http_requests_total",
				Type:       "counter",
				Help:       "The total number of client requests.",
				ValueIndex: ptr(uint(4)),
			},
			logLines: []string{
				"example.com\tGET",
			},
			parseErr: "line index out of range for value index 4, line length is 2",
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
			parseErr: "",
		},
		{
			name:      "metric without name",
			cfg:       config.Metric{},
			logLines:  make([]string, 0),
			metricErr: "metric name cannot be empty",
		},
		{
			name: "metric without type",
			cfg: config.Metric{
				Name:       "http_requests_total",
				ValueIndex: ptr(uint(0)),
			},
			logLines:  make([]string, 0),
			metricErr: `unsupported metric type: "". Must be one of counter, gauge, or histogram`,
		},
		{
			name: "metric with empty label name",
			cfg: config.Metric{
				Name:       "http_requests_total",
				ValueIndex: ptr(uint(0)),
				Labels: []config.Label{
					{},
				},
			},
			logLines:  make([]string, 0),
			metricErr: `metric label name cannot be empty`,
		},
		{
			name: "metric with invalid type",
			cfg: config.Metric{
				Name:       "http_requests_total",
				Type:       "info",
				ValueIndex: ptr(uint(0)),
			},
			logLines:  make([]string, 0),
			metricErr: `unsupported metric type: "info". Must be one of counter, gauge, or histogram`,
		},
		{
			name: "non-counter metrics without valueIndex",
			cfg: config.Metric{
				Name: "http_requests_total",
				Type: "gauge",
			},
			logLines:  make([]string, 0),
			metricErr: "valueIndex must be set for non-counter metrics",
		},
		{
			name: "gauge metrics",
			cfg: config.Metric{
				Name:       "http_requests_total",
				Help:       "The total number of client requests.",
				Type:       "gauge",
				ValueIndex: ptr(uint(2)),
			},
			logLines: []string{
				"example.com\tGET\t200",
			},
			metrics: `
# HELP http_requests_total The total number of client requests.
# TYPE http_requests_total gauge
http_requests_total 200
`,
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
		{
			name: "metric with empty value",
			cfg: config.Metric{
				Name:       "http_response_duration_seconds",
				Type:       "counter",
				Help:       "The time spent on receiving the response from the upstream server",
				ValueIndex: ptr(uint(3)),
			},
			logLines: []string{
				"app.example.net\tPUT\t500\t-\t4096\t512",
			},
			metrics: ``,
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
		{
			name: "counter metric all preset",
			cfg: config.Metric{
				Name: "http_requests_total",
				Help: "The total number of client requests.",
				Type: "counter",
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
					{
						Name:      "remote_user",
						LineIndex: 11,
					},
					{
						Name:      "ssl",
						LineIndex: 12,
						Replacements: []config.Replacement{
							{
								Regexp:      regexp.MustCompile("^$"),
								Replacement: "off",
							},
						},
					},
					{
						Name:      "ssl_protocol",
						LineIndex: 13,
					},
					{
						Name:      "user_agent",
						LineIndex: 14,
						UserAgent: true,
					},
				},
			},
			logLines: []string{
				"metrics.example.com\tGET\t200\t2.567\t128\t8192\t10.0.1.8:6000\t0.025\t0.500\t2.540\tMISS\tmonitoruser\ton\tHTTP/2.0\tPrometheus/2.30.0",
				"example.com\tGET\t200\t0.045\t1024\t5432\t192.168.1.10:8080\t0.005\t0.020\t0.040\tMISS\t-\ton\tHTTP/2.0\tMozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
				"api.mysite.com\tPOST\t201\t0.123\t2048\t1234\t10.0.1.5:3000\t0.008\t0.045\t0.115\tBYPASS\tjohnuser\t\tHTTP/1.1\tcurl/7.68.0",
				"blog.example.org\tGET\t404\t0.012\t512\t404\t-\t-\t-\t-\t-\t-\t\tHTTP/1.1\tMozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15",
				"cdn.static.com\tGET\t304\t0.008\t0\t0\t192.168.1.15:9000\t0.002\t0.003\t0.005\tHIT\t-\ton\tHTTP/2.0\tMozilla/5.0 (iPhone; CPU iPhone OS 14_7_1 like Mac OS X) AppleWebKit/605.1.15",
				"app.example.net\tPUT\t500\t1.234\t4096\t512\t172.16.0.20:8000\t0.050\t0.200\t1.180\tBYPASS\tadminuser\ton\tHTTP/1.1\tPython-urllib/3.9",
				"www.example.com\tHEAD\t200\t0.003\t0\t0\t192.168.1.10:8080\t0.001\t0.001\t0.001\tMISS\t-\t\tHTTP/1.1\tMozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36",
				"api.service.io\tDELETE\t204\t0.067\t256\t0\t10.0.1.7:4000\t0.010\t0.025\t0.057\tBYPASS\tapiuser\ton\tHTTP/1.1\tPostman/7.36.5",
				"shop.example.com\tGET\t301\t0.015\t768\t301\t-\t-\t-\t-\t-\t-\t\tHTTP/1.1\tGooglebot/2.1",
				"auth.example.com\tPOST\t401\t0.089\t1536\t256\t192.168.1.25:5000\t0.015\t0.030\t0.074\tBYPASS\t-\ton\tHTTP/1.1\tMozilla/5.0 (Windows NT 10.0; Win64; x64; rv:91.0) Gecko/20100101",
				"metrics.example.com\tGET\t200\t2.567\t128\t8192\t10.0.1.8:6000\t0.025\t0.500\t2.540\tMISS\tmonitoruser\ton\tHTTP/2.0\tPrometheus/2.30.0",
			},
			metrics: `
# HELP http_requests_total The total number of client requests.
# TYPE http_requests_total counter
http_requests_total{host="api.mysite.com",method="POST",remote_user="johnuser",ssl="off",ssl_protocol="HTTP/1.1",status="201",user_agent="curl"} 1
http_requests_total{host="api.service.io",method="DELETE",remote_user="apiuser",ssl="on",ssl_protocol="HTTP/1.1",status="204",user_agent="Other"} 1
http_requests_total{host="app.example.net",method="PUT",remote_user="adminuser",ssl="on",ssl_protocol="HTTP/1.1",status="500",user_agent="Python-urllib"} 1
http_requests_total{host="auth.example.com",method="POST",remote_user="-",ssl="on",ssl_protocol="HTTP/1.1",status="401",user_agent="Other"} 1
http_requests_total{host="blog.example.org",method="GET",remote_user="-",ssl="off",ssl_protocol="HTTP/1.1",status="404",user_agent="Apple Mail"} 1
http_requests_total{host="cdn.static.com",method="GET",remote_user="-",ssl="on",ssl_protocol="HTTP/2.0",status="304",user_agent="Mobile Safari UI/WKWebView"} 1
http_requests_total{host="example.com",method="GET",remote_user="-",ssl="on",ssl_protocol="HTTP/2.0",status="200",user_agent="Other"} 1
http_requests_total{host="metrics.example.com",method="GET",remote_user="monitoruser",ssl="on",ssl_protocol="HTTP/2.0",status="200",user_agent="Other"} 2
http_requests_total{host="shop.example.com",method="GET",remote_user="-",ssl="off",ssl_protocol="HTTP/1.1",status="301",user_agent="Googlebot"} 1
http_requests_total{host="www.example.com",method="HEAD",remote_user="-",ssl="off",ssl_protocol="HTTP/1.1",status="200",user_agent="Other"} 1
`,
		},
		{
			name: "metric with upstream connect duration",
			cfg: config.Metric{
				Name:       "http_upstream_connect_duration_seconds",
				Type:       "counter",
				Help:       "The time spent on establishing a connection with the upstream server",
				ValueIndex: ptr(uint(7)),
				Buckets:    types.Float64Slice{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
				Math: config.Math{
					Enabled: true,
					Div:     1000,
					Mul:     0, // default value
				},
				Upstream: config.Upstream{
					Enabled:       true,
					AddrLineIndex: 6,
					Excludes:      make([]string, 0),
					Label:         false, // default value
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
				"api.example.com\tGET\t200\t0.125\t1536\t4096\t10.0.1.5:8080\t0.003\t0.045\t0.120",
				"web.example.org\tPOST\t502\t2.150\t2048\t512\t10.0.1.10:8080, 10.0.1.11:8080, 10.0.1.12:8080\t0.005, 0.004, -\t0.120, 0.115, -\t0.800, 0.900, -",
			},
			metrics: `
# HELP http_upstream_connect_duration_seconds The time spent on establishing a connection with the upstream server
# TYPE http_upstream_connect_duration_seconds counter
http_upstream_connect_duration_seconds{host="api.example.com",method="GET",status="200"} 3e-06
http_upstream_connect_duration_seconds{host="web.example.org",method="POST",status="502"} 9e-06
`,
		},
		{
			name: "metric with excluded upstream connect duration",
			cfg: config.Metric{
				Name:       "http_upstream_connect_duration_seconds",
				Type:       "counter",
				Help:       "The time spent on establishing a connection with the upstream server",
				ValueIndex: ptr(uint(7)),
				Buckets:    types.Float64Slice{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
				Math: config.Math{
					Enabled: true,
					Div:     1000,
					Mul:     0, // default value
				},
				Upstream: config.Upstream{
					Enabled:       true,
					AddrLineIndex: 6,
					Excludes:      []string{"10.0.1.11:8080"},
					Label:         false, // default value
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
				"api.example.com\tGET\t200\t0.125\t1536\t4096\t10.0.1.5:8080\t0.003\t0.045\t0.120",
				"web.example.org\tPOST\t502\t2.150\t2048\t512\t10.0.1.10:8080, 10.0.1.11:8080, 10.0.1.12:8080\t0.005, 0.004, -\t0.120, 0.115, -\t0.800, 0.900, -",
			},
			metrics: `
# HELP http_upstream_connect_duration_seconds The time spent on establishing a connection with the upstream server
# TYPE http_upstream_connect_duration_seconds counter
http_upstream_connect_duration_seconds{host="api.example.com",method="GET",status="200"} 3e-06
http_upstream_connect_duration_seconds{host="web.example.org",method="POST",status="502"} 5e-06
`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			met, err := metric.New(tc.cfg)
			if err != nil {
				if tc.metricErr != "" {
					require.EqualError(t, err, tc.metricErr)
				} else {
					require.NoError(t, err)
				}

				return
			}

			for _, line := range tc.logLines {
				err := met.Parse(strings.Split(line, "\t"))
				if err != nil {
					if tc.parseErr != "" {
						require.EqualError(t, err, tc.parseErr)
					} else {
						require.NoError(t, err)
					}

					return
				}
			}

			allMetrics, err := MetricsToText(t, met)
			require.NoError(t, err)

			require.Equal(t, strings.TrimSpace(tc.metrics), allMetrics)
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
