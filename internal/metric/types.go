package metric

import (
	"sync"

	"github.com/jkroepke/access-log-exporter/internal/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/ua-parser/uap-go/uaparser"
)

type Metric struct {
	metric     prometheus.Collector
	ua         *uaparser.Parser
	labelsPool *sync.Pool // Pool for reusing label maps in a thread-safe way

	cfg config.Metric
}
