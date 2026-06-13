package collector_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/jkroepke/access-log-exporter/internal/collector"
	"github.com/jkroepke/access-log-exporter/internal/config"
	"github.com/jkroepke/access-log-exporter/internal/syslog"
	"github.com/stretchr/testify/require"
)

func BenchmarkCollectorSimple(b *testing.B) {
	ctx, cancel := context.WithCancel(b.Context())
	defer cancel()

	messageCh := make(chan syslog.Message)
	col, err := collector.New(ctx, slog.New(slog.DiscardHandler), newBenchmarkPreset(), 1, messageCh)
	require.NoError(b, err)

	logLine := "example.com\tGET\t200"

	b.Cleanup(func() {
		close(messageCh)
		col.Close()
	})

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		messageCh <- syslog.Message{Line: logLine}
	}
}

func newBenchmarkPreset() config.Preset {
	return config.Preset{
		Metrics: []config.Metric{
			{
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
		},
	}
}
