package collector

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/jkroepke/access-log-exporter/internal/config"
	"github.com/jkroepke/access-log-exporter/internal/metric"
	"github.com/jkroepke/access-log-exporter/internal/syslog"
	"github.com/prometheus/client_golang/prometheus"
)

func New(ctx context.Context, logger *slog.Logger, preset config.Preset, workerCount int, messageCh <-chan syslog.Message) (*Collector, error) {
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
		wg:        &sync.WaitGroup{},
		metrics:   metrics,
		messageCh: messageCh,
		metricLogReceived: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "log_messages_received_total",
			Help: "Total number of log messages taken from the buffer for processing",
		}),
		metricLogParseError: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "log_parse_errors_total",
			Help: "Total number of parse errors",
		}),
		metricLogLastReceived: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "log_last_received_timestamp_seconds",
			Help: "Timestamp of the last received log message in seconds since epoch",
		}),
		metricBufferSize: prometheus.NewDesc(
			"log_buffer_size",
			"Capacity of the syslog message buffer. Once the buffer is full, the syslog receiver blocks and the kernel drops datagrams.",
			nil, nil,
		),
		metricBufferLength: prometheus.NewDesc(
			"log_buffer_length",
			"Number of log messages currently waiting in the syslog message buffer",
			nil, nil,
		),
	}

	collector.lineHandlerWorkers(ctx, logger, workerCount, messageCh)

	return collector, nil
}

// Describe implements the prometheus.Collector interface.
func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	c.metricLogReceived.Describe(ch)
	c.metricLogParseError.Describe(ch)
	c.metricLogLastReceived.Describe(ch)

	ch <- c.metricBufferSize

	ch <- c.metricBufferLength

	for _, met := range c.metrics {
		met.Describe(ch)
	}
}

// Collect implements the prometheus.Collector interface.
func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	c.metricLogReceived.Collect(ch)
	c.metricLogParseError.Collect(ch)
	c.metricLogLastReceived.Collect(ch)

	ch <- prometheus.MustNewConstMetric(c.metricBufferSize, prometheus.GaugeValue, float64(cap(c.messageCh)))

	ch <- prometheus.MustNewConstMetric(c.metricBufferLength, prometheus.GaugeValue, float64(len(c.messageCh)))

	for _, met := range c.metrics {
		met.Collect(ch)
	}
}

// Close stops the collector and waits for all workers to finish.
func (c *Collector) Close() {
	c.wg.Wait()
}
