package metric

import (
	"github.com/jkroepke/access-log-exporter/internal/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/ua-parser/uap-go/uaparser"
)

type Metric struct {
	metric prometheus.Collector
	ua     *uaparser.Parser

	cfg config.Metric
}
