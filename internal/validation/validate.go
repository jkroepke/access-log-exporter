package validation

import (
	"fmt"
	"log/slog"
	"sort"

	"github.com/jkroepke/access-log-exporter/internal/config"
	"github.com/jkroepke/access-log-exporter/internal/metric"
	"github.com/jkroepke/access-log-exporter/internal/nginx"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	versioncollector "github.com/prometheus/client_golang/prometheus/collectors/version"
)

// Validate verifies configuration semantics and Prometheus descriptor compatibility
// without opening listeners or starting workers.
func Validate(conf config.Config) error {
	if err := config.Validate(conf); err != nil {
		return fmt.Errorf("%w", err)
	}

	presetNames := make([]string, 0, len(conf.Presets))
	for presetName := range conf.Presets {
		presetNames = append(presetNames, presetName)
	}

	sort.Strings(presetNames)

	for _, presetName := range presetNames {
		if err := validatePreset(conf, presetName, conf.Presets[presetName]); err != nil {
			return err
		}
	}

	return nil
}

func validatePreset(conf config.Config, presetName string, preset config.Preset) error {
	registry := prometheus.NewRegistry()

	baseCollectors := []prometheus.Collector{
		collectors.NewGoCollector(),
		collectors.NewBuildInfoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		versioncollector.NewCollector("access_log_exporter"),
		prometheus.NewCounter(prometheus.CounterOpts{
			Name: "log_parse_errors_total",
			Help: "Total number of parse errors",
		}),
		prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "log_last_received_timestamp_seconds",
			Help: "Timestamp of the last received log message in seconds since epoch",
		}),
	}

	if !conf.Nginx.ScrapeURL.IsEmpty() {
		baseCollectors = append(baseCollectors, nginx.New(
			slog.New(slog.DiscardHandler),
			conf.Nginx.ScrapeURL.String(),
			nginx.WithTimeout(conf.Nginx.ScrapeTimeout),
		))
	}

	for _, collector := range baseCollectors {
		if err := registry.Register(collector); err != nil {
			return fmt.Errorf("could not validate built-in Prometheus collector: %w", err)
		}
	}

	for metricIndex, metricConfig := range preset.Metrics {
		met, err := metric.New(metricConfig)
		if err != nil {
			return fmt.Errorf(
				"preset %q metric %d (%q): %w",
				presetName,
				metricIndex,
				metricConfig.Name,
				err,
			)
		}

		if err = registry.Register(met); err != nil {
			return fmt.Errorf(
				"preset %q metric %d (%q) has invalid or conflicting Prometheus descriptors: %w",
				presetName,
				metricIndex,
				metricConfig.Name,
				err,
			)
		}
	}

	return nil
}
