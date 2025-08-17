package metric_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/jkroepke/access-log-exporter/internal/config"
	"github.com/jkroepke/access-log-exporter/internal/config/types"
	"github.com/jkroepke/access-log-exporter/internal/metric"
	"github.com/stretchr/testify/require"
)

func BenchmarkMetricParseSimple(b *testing.B) {
	met, err := metric.New(config.Metric{
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
	})

	require.NoError(b, err)

	logLine := strings.Split("example.com\tGET\t200", "\t")

	for b.Loop() {
		_ = met.Parse(logLine)
	}

	b.ReportAllocs()
}

func BenchmarkMetricParseUserAgent(b *testing.B) {
	met, err := metric.New(config.Metric{
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
			{
				Name:      "user_agent",
				LineIndex: 3,
				UserAgent: true,
			},
		},
	})

	require.NoError(b, err)

	logLine := strings.Split("example.com\tGET\t200\tMozilla/5.0 (iPhone; CPU iPhone OS 14_7_1 like Mac OS X) AppleWebKit/605.1.15", "\t")

	for b.Loop() {
		_ = met.Parse(logLine)
	}

	b.ReportAllocs()
}

func BenchmarkMetricParseReplacement(b *testing.B) {
	met, err := metric.New(config.Metric{
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
			{
				Name:      "path",
				LineIndex: 3,
				Replacements: []config.Replacement{
					{
						Regexp:      regexp.MustCompile(`/api/v1/resource\?id=\d+.+`),
						Replacement: "/api/v1/resource?id=:id",
					},
				},
			},
		},
	})

	require.NoError(b, err)

	logLine := strings.Split("example.com\tGET\t200\t/api/v1/resource?id=12345&name=test", "\t")

	for b.Loop() {
		_ = met.Parse(logLine)
	}

	b.ReportAllocs()
}

func BenchmarkMetricParseUpstream(b *testing.B) {
	met, err := metric.New(config.Metric{
		Name:       "http_upstream_connect_duration_seconds",
		Type:       "histogram",
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
	})

	require.NoError(b, err)

	logLine := strings.Split("web.example.org\tPOST\t502\t2.150\t2048\t512\t10.0.1.10:8080, 10.0.1.11:8080, 10.0.1.12:8080\t0.005, 0.004, -\t0.120, 0.115, -\t0.800, 0.900, -", "\t")

	for b.Loop() {
		_ = met.Parse(logLine)
	}

	b.ReportAllocs()
}
