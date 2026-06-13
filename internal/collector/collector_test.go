package collector_test

import (
	"log/slog"
	"testing"
	"time"

	"github.com/jkroepke/access-log-exporter/internal/collector"
	"github.com/jkroepke/access-log-exporter/internal/config"
	"github.com/jkroepke/access-log-exporter/internal/syslog"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
)

func TestCollectorExposesLastReceivedMetric(t *testing.T) {
	t.Parallel()

	messageCh := make(chan syslog.Message)

	col, err := collector.New(t.Context(), slog.New(slog.DiscardHandler), newTestPreset(), 1, messageCh)
	require.NoError(t, err)

	t.Cleanup(func() {
		close(messageCh)
		col.Close()
	})

	messageCh <- syslog.Message{Line: "example.com\tGET\t200"}

	require.Eventually(t, func() bool {
		return testutil.CollectAndCount(col, "log_last_received_timestamp_seconds") == 1
	}, time.Second, 10*time.Millisecond)
}

func newTestPreset() config.Preset {
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
