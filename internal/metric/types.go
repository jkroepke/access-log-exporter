package metric

import (
	"github.com/jkroepke/access-log-exporter/internal/config"
	"github.com/medama-io/go-useragent"
	"github.com/prometheus/client_golang/prometheus"
)

type Metric struct {
	metric prometheus.Collector
	ua     *useragent.Parser

	cfg config.Metric
}
