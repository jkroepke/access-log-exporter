package collector

import (
	"errors"
	"log/slog"
	"sync"

	"github.com/jkroepke/access-log-exporter/internal/config"
	"github.com/jkroepke/access-log-exporter/internal/metric"
	"github.com/prometheus/client_golang/prometheus"
)

var ErrNoSource = errors.New("no source data configured, cannot start collector")

type Collector struct {
	parseErrorMetric prometheus.Counter
	logger           *slog.Logger
	buffer           chan string
	wg               *sync.WaitGroup
	preset           config.Preset
	metrics          []*metric.Metric
}
