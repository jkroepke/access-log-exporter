package collector

import (
	"sync"

	"github.com/jkroepke/access-log-exporter/internal/metric"
	"github.com/jkroepke/access-log-exporter/internal/syslog"
	"github.com/prometheus/client_golang/prometheus"
)

type Collector struct {
	metricLogReceived     prometheus.Counter
	metricLogParseError   prometheus.Counter
	metricLogLastReceived prometheus.Gauge
	metricBufferSize      *prometheus.Desc
	metricBufferLength    *prometheus.Desc
	wg                    *sync.WaitGroup
	messageCh             <-chan syslog.Message
	metrics               []*metric.Metric
}
