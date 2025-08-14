package collector

import (
	"sync"

	"github.com/jkroepke/access-log-exporter/internal/metric"
	"github.com/prometheus/client_golang/prometheus"
)

type Collector struct {
	parseErrorMetric prometheus.Counter
	wg               *sync.WaitGroup
	metrics          []*metric.Metric
}
