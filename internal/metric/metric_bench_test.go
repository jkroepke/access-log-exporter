package metric_test

import (
	"strings"
	"testing"

	"github.com/jkroepke/access-log-exporter/internal/config"
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
