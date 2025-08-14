package collector

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/jkroepke/access-log-exporter/internal/config"
	"github.com/jkroepke/access-log-exporter/internal/metric"
	"github.com/prometheus/client_golang/prometheus"
)

func New(ctx context.Context, logger *slog.Logger, preset config.Preset, workerCount int, messageCh <-chan string) (*Collector, error) {
	var (
		err       error
		userAgent bool
	)

	metrics := make([]*metric.Metric, len(preset.Metrics))
	for i, metricConfig := range preset.Metrics {
		metrics[i], err = metric.New(metricConfig)
		if err != nil {
			return nil, fmt.Errorf("could not create metric '%s': %w", metricConfig.Name, err)
		}

		for _, label := range metricConfig.Labels {
			if label.UserAgent {
				userAgent = true
			}
		}
	}

	if userAgent {
		logger.WarnContext(ctx, "The user agent parser is currently experimental and changed in the future or may not work as expected. "+
			"Please report any issues you encounter.")
	}

	collector := &Collector{
		wg:      &sync.WaitGroup{},
		metrics: metrics,
		parseErrorMetric: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "log_parse_errors_total",
			Help: "Total number of parse errors",
		}),
	}

	collector.lineHandlerWorkers(ctx, logger, workerCount, messageCh)

	return collector, nil
}

// Describe implements the prometheus.Collector interface.
func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	c.parseErrorMetric.Describe(ch)

	for _, met := range c.metrics {
		met.Describe(ch)
	}
}

// Collect implements the prometheus.Collector interface.
func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	c.parseErrorMetric.Collect(ch)

	for _, met := range c.metrics {
		met.Collect(ch)
	}
}

// Close stops the collector and waits for all workers to finish.
func (c *Collector) Close() {
	c.wg.Wait()
}
