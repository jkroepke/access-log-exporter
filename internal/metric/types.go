package metric

import (
	"github.com/jkroepke/access-log-exporter/internal/config"
	"github.com/prometheus/client_golang/prometheus"
)

type Metric struct {
	metric prometheus.Collector

	cfg config.Metric
}
