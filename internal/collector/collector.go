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

func New(ctx context.Context, logger *slog.Logger, conf config.Config) (*Collector, error) {
	var err error

	preset, ok := conf.Presets[conf.Preset]
	if !ok {
		return nil, fmt.Errorf("preset '%s' not found in configuration", conf.Preset)
	}

	metrics := make([]*metric.Metric, len(preset.Metrics))
	for i, metricConfig := range preset.Metrics {
		metrics[i], err = metric.New(metricConfig)
		if err != nil {
			return nil, fmt.Errorf("could not create metric '%s': %w", metricConfig.Name, err)
		}
	}

	collector := &Collector{
		preset:  preset,
		logger:  logger,
		buffer:  make(chan string, conf.BufferSize),
		wg:      &sync.WaitGroup{},
		metrics: metrics,
		parseErrorMetric: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "nginxlog_parse_errors_total",
			Help: "Total number of parse errors",
		}),
	}

	err = collector.startPump(ctx, conf.Syslog)
	if err != nil {
		return nil, fmt.Errorf("could not start syslog pump: %w", err)
	}

	err = collector.lineHandler(ctx, conf.WorkerCount)
	if err != nil {
		return nil, fmt.Errorf("could not start syslog pump: %w", err)
	}

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

func (c *Collector) Close() {
	c.wg.Wait()

	close(c.buffer)
}
