package metric

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/jkroepke/access-log-exporter/internal/config"
	"github.com/prometheus/client_golang/prometheus"
)

func New(cfg config.Metric) (Metric, error) {
	var metric prometheus.Collector

	// Validate metric configuration
	if cfg.Name == "" {
		return Metric{}, errors.New("metric name cannot be empty")
	}

	if cfg.ValueIndex == nil && cfg.Type != "counter" {
		return Metric{}, errors.New("valueIndex must be set for non-counter metrics")
	}

	labelCount := len(cfg.Labels)
	if cfg.Upstream.Enabled && cfg.Upstream.Label {
		labelCount++ // Include upstream label if enabled
	}

	// Pre-allocate labelKeys with exact capacity
	labelKeys := make([]string, labelCount)

	for i, label := range cfg.Labels {
		if label.Name == "" {
			return Metric{}, errors.New("metric label name cannot be empty")
		}
		labelKeys[i] = label.Name
	}

	// Add upstream label if enabled
	if cfg.Upstream.Enabled && cfg.Upstream.Label {
		labelKeys[len(cfg.Labels)] = "upstream"
	}

	switch cfg.Type {
	case "counter":
		metric = prometheus.NewCounterVec(prometheus.CounterOpts{
			Name:        cfg.Name,
			Help:        cfg.Help,
			ConstLabels: cfg.ConstLabels,
		}, labelKeys)
	case "gauge":
		metric = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name:        cfg.Name,
			Help:        cfg.Help,
			ConstLabels: cfg.ConstLabels,
		}, labelKeys)
	case "histogram":
		buckets := cfg.Buckets
		if len(buckets) == 0 {
			buckets = prometheus.DefBuckets
		}

		metric = prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:        cfg.Name,
			Help:        cfg.Help,
			ConstLabels: cfg.ConstLabels,
			Buckets:     buckets,
		}, labelKeys)
	default:
		return Metric{}, fmt.Errorf("unsupported metric type: %s", cfg.Type)
	}

	return Metric{
		cfg:    cfg,
		metric: metric,
	}, nil
}

func (m *Metric) Describe(ch chan<- *prometheus.Desc) {
	if m.metric != nil {
		m.metric.Describe(ch)
	}
}

func (m *Metric) Collect(ch chan<- prometheus.Metric) {
	if m.metric != nil {
		m.metric.Collect(ch)
	}
}

func (m *Metric) Parse(line []string) error {
	lineLength := uint(len(line))

	// Check bounds early for the value index if it exists.
	if m.cfg.ValueIndex != nil && *m.cfg.ValueIndex >= lineLength {
		return fmt.Errorf("line index out of range for value index %d, line length is %d", *m.cfg.ValueIndex, lineLength)
	}

	// BCE (Bound Check Elimination)
	//https://go101.org/optimizations/5-bce.html
	_ = line[lineLength-1]

	// Pre-allocate labels map with exact capacity
	labels := make(prometheus.Labels, len(m.cfg.Labels))

	// Process labels first and validate line indices
	for _, label := range m.cfg.Labels {
		if label.LineIndex >= lineLength {
			return fmt.Errorf("line index out of range for label %s, line length is %d", label.Name, lineLength)
		}

		labelValue := m.labelValueReplacements(label.Replacements, line[label.LineIndex])
		labels[label.Name] = labelValue
	}

	// Handle counter without value (increment by 1)
	if m.cfg.ValueIndex == nil {
		if counterVec, ok := m.metric.(*prometheus.CounterVec); ok {
			counterVec.With(labels).Inc()

			return nil
		}

		// This should never happen due to validation in New(), but be defensive
		return fmt.Errorf("valueIndex is nil but metric type is not counter")
	}

	value := line[*m.cfg.ValueIndex]
	if value == "" {
		return nil // Skip empty values silently
	}

	if m.cfg.Upstream.Enabled {
		err := m.setMetricWithUpstream(line, lineLength, value, labels)
		if err != nil {
			return err
		}

		return nil
	}

	if err := m.setMetric(value, labels); err != nil {
		return fmt.Errorf("failed to set metric %s with value %q: %w", m.cfg.Name, value, err)
	}

	return nil
}

func (m *Metric) setMetricWithUpstream(line []string, lineLength uint, value string, labels prometheus.Labels) error {
	valueElements := strings.Split(value, ",")
	var upstreams []string

	// Get upstreams if we need them for excludes or labels
	if len(m.cfg.Upstream.Excludes) != 0 || m.cfg.Upstream.Label {
		if m.cfg.Upstream.AddrLineIndex >= lineLength {
			return fmt.Errorf("line index out of range for upstream address index %d, line length is %d", m.cfg.Upstream.AddrLineIndex, lineLength)
		}

		upstreams = strings.Split(line[m.cfg.Upstream.AddrLineIndex], ",")
	}

	// Process each value element
	for i, valueElement := range valueElements {
		// Handle upstream processing if we have upstreams
		if len(upstreams) > 0 {
			// If we have fewer upstreams than values, use the last upstream for remaining values
			upstreamIndex := i
			if upstreamIndex >= len(upstreams) {
				upstreamIndex = len(upstreams) - 1
			}

			upstream := strings.TrimSpace(upstreams[upstreamIndex])

			// Skip if upstream is in exclude list
			if len(m.cfg.Upstream.Excludes) != 0 && slices.Contains(m.cfg.Upstream.Excludes, upstream) {
				continue
			}

			// Add upstream label if enabled
			if m.cfg.Upstream.Label {
				labels["upstream"] = upstream
			}
		}

		err := m.setMetric(valueElement, labels)
		if err != nil {
			return fmt.Errorf("failed to set metric %s with value %q: %w", m.cfg.Name, valueElement, err)
		}
	}

	return nil
}

func (m *Metric) setMetric(value string, labels prometheus.Labels) error {
	// Handle special case for empty values after trimming whitespace
	value = strings.TrimSpace(value)
	if value == "" {
		return nil // Skip empty values silently
	}

	valueFloat, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fmt.Errorf("failed to parse value %q: %w", value, err)
	}

	if m.cfg.Math.Enabled {
		if m.cfg.Math.Div != 0 {
			valueFloat /= m.cfg.Math.Div
		}

		if m.cfg.Math.Mul != 0 {
			valueFloat *= m.cfg.Math.Mul
		}
	}

	// Apply value to appropriate metric type
	switch metric := m.metric.(type) {
	case *prometheus.CounterVec:
		if valueFloat < 0 {
			return fmt.Errorf("counter value cannot be negative: %f", valueFloat)
		}
		metric.With(labels).Add(valueFloat)
	case *prometheus.GaugeVec:
		metric.With(labels).Set(valueFloat)
	case *prometheus.HistogramVec:
		metric.With(labels).Observe(valueFloat)
	default:
		return fmt.Errorf("unsupported metric type %s", m.cfg.Type)
	}

	return nil
}

func (m *Metric) labelValueReplacements(replacements []config.Replacement, labelValue string) string {
	if len(replacements) == 0 {
		return labelValue
	}

	for _, replacement := range replacements {
		if replacement.Regexp.MatchString(labelValue) {
			return replacement.Replacement
		}
	}

	return labelValue
}
