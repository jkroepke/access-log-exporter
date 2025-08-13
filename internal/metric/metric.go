package metric

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/jkroepke/access-log-exporter/internal/config"
	"github.com/medama-io/go-useragent"
	"github.com/prometheus/client_golang/prometheus"
)

//nolint:cyclop
func New(cfg config.Metric) (*Metric, error) {
	var metric prometheus.Collector

	// Validate metric configuration
	if cfg.Name == "" {
		return nil, errors.New("metric name cannot be empty")
	}

	if cfg.ValueIndex == nil && cfg.Type != "counter" {
		return nil, errors.New("valueIndex must be set for non-counter metrics")
	}

	labelCount := len(cfg.Labels)
	if cfg.Upstream.Enabled && cfg.Upstream.Label {
		labelCount++ // Include upstream label if enabled
	}

	// Pre-allocate labelKeys with exact capacity
	labelKeys := make([]string, labelCount)

	var userAgent *useragent.Parser

	for i, label := range cfg.Labels {
		if label.Name == "" {
			return nil, errors.New("metric label name cannot be empty")
		}

		if label.UserAgent {
			userAgent = useragent.NewParser()
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
		return nil, fmt.Errorf("unsupported metric type: %q. Must be one of counter, gauge, or histogram", cfg.Type)
	}

	return &Metric{
		cfg:    cfg,
		metric: metric,
		ua:     userAgent,
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

//nolint:cyclop
func (m *Metric) Parse(line []string) error {
	lineLength := uint(len(line))

	if lineLength == 0 || line[0] == "" {
		return nil // Skip empty lines silently
	}

	var value string

	// Check bounds early for the value index if it exists.
	if m.cfg.ValueIndex != nil {
		if *m.cfg.ValueIndex >= lineLength {
			return fmt.Errorf("line index out of range for value index %d, line length is %d", *m.cfg.ValueIndex, lineLength)
		}

		value = line[*m.cfg.ValueIndex]
		if value == "" || value == "-" {
			return nil // Skip empty values silently
		}
	}

	// BCE (Bound Check Elimination)
	// https://go101.org/optimizations/5-bce.html
	_ = line[lineLength-1]

	// Calculate exact capacity including potential upstream label
	labelCapacity := len(m.cfg.Labels)
	if m.cfg.Upstream.Enabled && m.cfg.Upstream.Label {
		labelCapacity++
	}

	// Pre-allocate labels map with exact capacity
	labels := make(prometheus.Labels, labelCapacity)

	// Process labels first and validate line indices
	for _, label := range m.cfg.Labels {
		if label.LineIndex >= lineLength {
			return fmt.Errorf("line index out of range for label %s, line length is %d", label.Name, lineLength)
		}

		labelValue := line[label.LineIndex]

		if label.UserAgent {
			uaInfo := m.ua.Parse(labelValue)

			labelValue = uaInfo.Browser().String()
		}

		labelValue = m.labelValueReplacements(label.Replacements, labelValue)

		labels[label.Name] = labelValue
	}

	// Handle counter without value (increment by 1)
	if m.cfg.ValueIndex == nil {
		if counterVec, ok := m.metric.(*prometheus.CounterVec); ok {
			counterVec.With(labels).Inc()

			return nil
		}

		// This should never happen due to validation in New(), but be defensive
		return errors.New("valueIndex is nil but metric type is not counter")
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

//nolint:cyclop
func (m *Metric) setMetricWithUpstream(line []string, lineLength uint, value string, labels prometheus.Labels) error {
	var upstreams []string

	// Get upstreams if we need them for excludes or labels
	if len(m.cfg.Upstream.Excludes) != 0 || m.cfg.Upstream.Label {
		if m.cfg.Upstream.AddrLineIndex >= lineLength {
			return fmt.Errorf("line index out of range for upstream address index %d, line length is %d", m.cfg.Upstream.AddrLineIndex, lineLength)
		}

		upstreams = strings.Split(line[m.cfg.Upstream.AddrLineIndex], ",")

		// Trim whitespace from upstreams
		for i, upstream := range upstreams {
			upstreams[i] = strings.TrimSpace(upstream)
		}
	}

	valueIndex := 0
	for {
		var valueElement string
		var remaining string

		if comma := strings.IndexByte(value, ','); comma >= 0 {
			valueElement = strings.TrimSpace(value[:comma])
			remaining = value[comma+1:]
		} else {
			valueElement = strings.TrimSpace(value)
			remaining = ""
		}

		if valueElement == "-" {
			continue
		}

		// Create a copy of labels for this iteration with capacity for upstream label
		iterationLabels := make(prometheus.Labels, len(labels)+1)
		for k, v := range labels {
			iterationLabels[k] = v
		}

		// Handle upstream processing if we have upstreams
		if len(upstreams) > 0 {
			// If we have fewer upstreams than values, use the last upstream for remaining values
			upstreamIndex := valueIndex
			if upstreamIndex >= len(upstreams) {
				upstreamIndex = len(upstreams) - 1
			}

			upstream := upstreams[upstreamIndex]

			// Skip if upstream is in exclude list
			if len(m.cfg.Upstream.Excludes) != 0 && slices.Contains(m.cfg.Upstream.Excludes, upstream) {
				valueIndex++
				if remaining == "" {
					break
				}
				value = remaining

				continue
			}

			// Add upstream label if enabled
			if m.cfg.Upstream.Label {
				iterationLabels["upstream"] = upstream
			}
		}

		err := m.setMetric(valueElement, iterationLabels)
		if err != nil {
			return fmt.Errorf("failed to set metric %s with value %q: %w", m.cfg.Name, valueElement, err)
		}

		valueIndex++
		if remaining == "" {
			break
		}
		value = remaining
	}

	return nil
}

//nolint:cyclop
func (m *Metric) setMetric(value string, labels prometheus.Labels) error {
	// Early return for empty values before trimming
	if value == "" {
		return nil
	}

	// Handle special case for empty values after trimming
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
