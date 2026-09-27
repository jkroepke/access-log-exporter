package collector_test

import (
	"log/slog"
	"strings"
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

	messageCh <- syslog.Message{
		Line:               "example.com\tGET\t200",
		ReceivedAtUnixNano: int64(42 * time.Second),
	}

	require.Eventually(t, func() bool {
		expected := `# HELP log_last_received_timestamp_seconds Timestamp of the last received log message in seconds since epoch
# TYPE log_last_received_timestamp_seconds gauge
log_last_received_timestamp_seconds 42
`

		return testutil.CollectAndCompare(
			col,
			strings.NewReader(expected),
			"log_last_received_timestamp_seconds",
		) == nil
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
