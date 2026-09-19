package config

import (
	"errors"
	"fmt"
	"math"
	"net/url"
	"sort"
)

// Validate validates the complete static configuration.
func Validate(conf Config) error {
	if _, ok := conf.Presets[conf.Preset]; !ok {
		return fmt.Errorf("preset '%s' not found in configuration", conf.Preset)
	}

	if err := validateTLS(conf); err != nil {
		return err
	}

	if err := validateLog(conf); err != nil {
		return err
	}

	if err := validateSyslog(conf); err != nil {
		return err
	}

	if err := validateNginx(conf); err != nil {
		return err
	}

	return validatePresets(conf.Presets)
}

func validateLog(conf Config) error {
	switch conf.Log.Format {
	case "json", "console":
		return nil
	default:
		return fmt.Errorf("unknown log format: %s", conf.Log.Format)
	}
}

// validateTLS validates TLS configuration.
func validateTLS(conf Config) error {
	certSet := conf.Web.TLSCertFile != ""
	keySet := conf.Web.TLSKeyFile != ""

	if certSet != keySet {
		return errors.New("both TLS certificate and key files must be set to enable TLS")
	}

	return nil
}

func validateSyslog(conf Config) error {
	uri, err := url.Parse(conf.Syslog.ListenAddress)
	if err != nil {
		return fmt.Errorf("could not parse syslog listen address %q: %w", conf.Syslog.ListenAddress, err)
	}

	switch uri.Scheme {
	case "udp", "unix":
		return nil
	default:
		return errors.New("syslog listen address must start with udp:// or unix://")
	}
}

func validateNginx(conf Config) error {
	if conf.Nginx.ScrapeURL.IsEmpty() {
		return nil
	}

	switch conf.Nginx.ScrapeURL.Scheme {
	case "http", "https":
		if conf.Nginx.ScrapeURL.Host == "" {
			return errors.New("nginx.scrape-url must include a host for HTTP or HTTPS")
		}
	case "unix":
		if conf.Nginx.ScrapeURL.Path == "" {
			return errors.New("nginx.scrape-url must include a socket path for unix")
		}
	default:
		return fmt.Errorf("nginx.scrape-url has unsupported scheme %q", conf.Nginx.ScrapeURL.Scheme)
	}

	return nil
}

func validatePresets(presets Presets) error {
	presetNames := make([]string, 0, len(presets))
	for presetName := range presets {
		presetNames = append(presetNames, presetName)
	}

	sort.Strings(presetNames)

	for _, presetName := range presetNames {
		preset := presets[presetName]

		for metricIndex, metricConfig := range preset.Metrics {
			if err := ValidateMetric(metricConfig); err != nil {
				return fmt.Errorf("preset %q metric %d (%q): %w", presetName, metricIndex, metricConfig.Name, err)
			}
		}
	}

	return nil
}

// ValidateMetric validates a metric configuration without constructing the Prometheus collector.
func ValidateMetric(cfg Metric) error {
	if cfg.Name == "" {
		return errors.New("metric name cannot be empty")
	}

	if cfg.ValueIndex == nil && cfg.Type != "counter" {
		return errors.New("valueIndex must be set for non-counter metrics")
	}

	labelNames := make(map[string]struct{}, len(cfg.Labels)+1)
	for _, label := range cfg.Labels {
		if label.Name == "" {
			return errors.New("metric label name cannot be empty")
		}

		if _, exists := labelNames[label.Name]; exists {
			return fmt.Errorf("metric label %q is duplicated", label.Name)
		}

		labelNames[label.Name] = struct{}{}
	}

	if !cfg.Upstream.Enabled {
		if cfg.Upstream.Label || len(cfg.Upstream.Excludes) != 0 || cfg.Upstream.AddrLineIndex != 0 {
			return errors.New("upstream settings require upstream.enabled")
		}
	} else {
		if cfg.ValueIndex == nil {
			return errors.New("valueIndex must be set when upstream processing is enabled")
		}

		if cfg.Upstream.Label {
			if _, exists := labelNames["upstream"]; exists {
				return errors.New("metric label \"upstream\" conflicts with the upstream label")
			}
		}
	}

	switch cfg.Type {
	case "counter", "gauge":
		return nil
	case "histogram":
		return validateHistogramBuckets(cfg.Buckets)
	default:
		return fmt.Errorf("unsupported metric type: %q. Must be one of counter, gauge, or histogram", cfg.Type)
	}
}

func validateHistogramBuckets(buckets []float64) error {
	for i, bucket := range buckets {
		if math.IsNaN(bucket) {
			return errors.New("histogram bucket cannot be NaN")
		}

		if i == 0 {
			continue
		}

		if buckets[i-1] >= bucket {
			return fmt.Errorf(
				"histogram buckets must be in increasing order: %f >= %f",
				buckets[i-1],
				bucket,
			)
		}
	}

	return nil
}
