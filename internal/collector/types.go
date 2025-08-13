package collector

import (
	"log/slog"
	"sync"

	"github.com/jkroepke/access-log-exporter/internal/config"
	"github.com/jkroepke/access-log-exporter/internal/metric"
	"github.com/prometheus/client_golang/prometheus"
)

type Collector struct {
	parseErrorMetric prometheus.Counter
	logger           *slog.Logger
	wg               *sync.WaitGroup
	preset           config.Preset
	metrics          []*metric.Metric
}
