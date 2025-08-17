package metric

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/jkroepke/access-log-exporter/internal/config"
	"github.com/jkroepke/access-log-exporter/internal/useragent"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/ua-parser/uap-go/uaparser"
)

//nolint:cyclop
func New(cfg config.Metric) (*Metric, error) {
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

	var (
		uaParser         *uaparser.Parser
		userAgentEnabled bool
	)

	for i, label := range cfg.Labels {
		if label.Name == "" {
			return nil, errors.New("metric label name cannot be empty")
		}

		labelKeys[i] = label.Name

		if label.UserAgent {
			userAgentEnabled = true
		}
	}

	// Initialize user agent parser if needed
	if userAgentEnabled {
		uaParser = useragent.New()
	}

	// Add upstream label if enabled
	if cfg.Upstream.Enabled && cfg.Upstream.Label {
		labelKeys[len(cfg.Labels)] = "upstream"
	}

	var metric prometheus.Collector

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
		ua:     uaParser,
		labelsPool: &sync.Pool{
			New: func() interface{} {
				return make(prometheus.Labels, labelCount)
			},
		},
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

func (m *Metric) Name() string {
	return m.cfg.Name
}

// Parse processes a single line of input, extracting labels and values based on the metric configuration.
// It's guaranteed to be thread-safe and can be called concurrently.
func (m *Metric) Parse(line []string) error {
	// Validate and extract value from line
	value, skip, err := m.validateAndExtractValue(line)
	if err != nil {
		return err
	}

	if skip {
		return nil // Skip processing for empty/invalid lines
	}

	// Get labels map from pool and ensure cleanup
	labels := m.getLabelsFromPool()
	defer m.returnLabelsToPool(labels)

	// Process all labels from the line
	if err := m.processLabels(line, labels); err != nil {
		return err
	}

	// Handle metric value setting based on configuration
	return m.handleMetricValue(line, value, labels)
}

// validateAndExtractValue validates the input line and extracts the metric value if configured.
// Returns the value string, whether to skip processing, and any validation errors.
func (m *Metric) validateAndExtractValue(line []string) (string, bool, error) {
	lineLength := uint(len(line))

	if lineLength == 0 || line[0] == "" {
		return "", true, nil // Signal to skip processing
	}

	// BCE (Bound Check Elimination)
	// https://go101.org/optimizations/5-bce.html
	_ = line[lineLength-1]

	// If no value index is configured, this is a counter-only metric
	if m.cfg.ValueIndex == nil {
		return "", false, nil
	}

	// Validate value index bounds
	if *m.cfg.ValueIndex >= lineLength {
		return "", false, fmt.Errorf("line index out of range for value index %d, line length is %d", *m.cfg.ValueIndex, lineLength)
	}

	value := line[*m.cfg.ValueIndex]
	if value == "" || value == "-" {
		return "", true, nil // Signal to skip processing
	}

	if m.cfg.Replacements != nil {
		value = m.valueReplacements(m.cfg.Replacements, value)
	}

	return value, false, nil
}

// getLabelsFromPool retrieves a labels map from the sync.Pool for thread-safe reuse.
func (m *Metric) getLabelsFromPool() prometheus.Labels {
	labels, ok := m.labelsPool.Get().(prometheus.Labels)
	if !ok {
		// If the type assertion fails, create a new map
		labels = make(prometheus.Labels, len(m.cfg.Labels))
	}

	return labels
}

// returnLabelsToPool clears the labels map and returns it to the pool for reuse.
func (m *Metric) returnLabelsToPool(labels prometheus.Labels) {
	// Clear the map and return it to the pool
	for k := range labels {
		delete(labels, k)
	}

	m.labelsPool.Put(labels)
}

// processLabels extracts and processes all configured labels from the log line.
func (m *Metric) processLabels(line []string, labels prometheus.Labels) error {
	lineLength := uint(len(line))

	for _, label := range m.cfg.Labels {
		if label.LineIndex >= lineLength {
			return fmt.Errorf("line index out of range for label %s, line length is %d", label.Name, lineLength)
		}

		labelValue := line[label.LineIndex]

		// Apply user agent parsing if configured
		if label.UserAgent {
			uaInfo := m.ua.Parse(labelValue)
			labelValue = uaInfo.UserAgent.Family
		}

		// Apply regex replacements if configured
		labelValue = m.valueReplacements(label.Replacements, labelValue)

		labels[label.Name] = labelValue
	}

	return nil
}

// handleMetricValue handles setting the metric value based on the configuration type.
func (m *Metric) handleMetricValue(line []string, value string, labels prometheus.Labels) error {
	// Handle counter without value (increment by 1)
	if m.cfg.ValueIndex == nil {
		return m.handleCounterIncrement(labels)
	}

	// Skip processing if value is empty (validated earlier)
	if value == "" {
		return nil
	}

	// Handle upstream processing if enabled
	if m.cfg.Upstream.Enabled {
		return m.setMetricWithUpstream(line, uint(len(line)), value, labels)
	}

	// Handle standard metric setting
	if err := m.setMetric(value, labels); err != nil {
		return fmt.Errorf("failed to set metric %s with value %q: %w", m.cfg.Name, value, err)
	}

	return nil
}

// handleCounterIncrement handles counter metrics that increment by 1 (no value configured).
func (m *Metric) handleCounterIncrement(labels prometheus.Labels) error {
	counterVec, ok := m.metric.(*prometheus.CounterVec)
	if !ok {
		// This should never happen due to validation in New(), but be defensive
		return errors.New("valueIndex is nil but metric type is not counter")
	}

	counterVec.With(labels).Inc()

	return nil
}

// setMetricWithUpstream processes comma-separated metric values with corresponding upstream servers.
//
// This function handles the upstream feature where multiple metric values can be associated
// with different upstream servers. It parses both comma-separated values and upstream addresses,
// applies exclusion rules, and sets metrics with appropriate upstream labels.
//
// Parameters:
//   - line: The complete log line as string array
//   - lineLength: Length of the line array (for bounds checking)
//   - value: Comma-separated string of metric values
//   - labels: Base labels map (will be modified to include upstream labels)
//
// Returns:
//   - error: Returns an error if upstream parsing fails or metric setting fails
//
// Thread Safety:
// This function is thread-safe when called with unique labels maps per goroutine.
//
// Behavior:
//   - Splits comma-separated values and processes each one
//   - Maps values to upstream servers (reuses last upstream if fewer upstreams than values)
//   - Skips values associated with excluded upstream servers
//   - Adds "upstream" label when upstream labeling is enabled
func (m *Metric) setMetricWithUpstream(line []string, lineLength uint, value string, labels prometheus.Labels) error {
	upstreams, err := m.parseUpstreams(line, lineLength)
	if err != nil {
		return err
	}

	return m.processCommaDelimitedValues(value, upstreams, labels)
}

// parseUpstreams extracts and processes upstream server addresses from the log line.
func (m *Metric) parseUpstreams(line []string, lineLength uint) ([]string, error) {
	// Only parse upstreams if we need them for excludes or labels
	if len(m.cfg.Upstream.Excludes) == 0 && !m.cfg.Upstream.Label {
		return nil, nil
	}

	if m.cfg.Upstream.AddrLineIndex >= lineLength {
		return nil, fmt.Errorf("line index out of range for upstream address index %d, line length is %d", m.cfg.Upstream.AddrLineIndex, lineLength)
	}

	upstreams := strings.Split(line[m.cfg.Upstream.AddrLineIndex], ",")

	// Trim whitespace from upstreams
	for i, upstream := range upstreams {
		upstreams[i] = strings.TrimSpace(upstream)
	}

	return upstreams, nil
}

// processCommaDelimitedValues processes comma-separated metric values with upstream mapping.
func (m *Metric) processCommaDelimitedValues(value string, upstreams []string, labels prometheus.Labels) error {
	valueIndex := 0

	for {
		valueElement, remaining := m.extractNextValue(value)

		if valueElement != "-" {
			if err := m.processValueWithUpstream(valueElement, upstreams, valueIndex, labels); err != nil {
				return err
			}
		}

		valueIndex++

		if remaining == "" {
			break
		}

		value = remaining
	}

	return nil
}

// extractNextValue extracts the next comma-separated value from the input string.
func (m *Metric) extractNextValue(value string) (string, string) {
	if comma := strings.IndexByte(value, ','); comma >= 0 {
		return strings.TrimSpace(value[:comma]), value[comma+1:]
	}

	return strings.TrimSpace(value), ""
}

// processValueWithUpstream processes a single metric value with its associated upstream.
func (m *Metric) processValueWithUpstream(valueElement string, upstreams []string, valueIndex int, labels prometheus.Labels) error {
	if len(upstreams) == 0 {
		return m.setMetric(valueElement, labels)
	}

	upstream := m.getUpstreamForValue(upstreams, valueIndex)

	// Skip if upstream is in exclude list
	if m.isUpstreamExcluded(upstream) {
		return nil
	}

	// Add upstream label if enabled
	if m.cfg.Upstream.Label {
		labels["upstream"] = upstream
	}

	return m.setMetric(valueElement, labels)
}

// getUpstreamForValue returns the appropriate upstream for the given value index.
// If there are fewer upstreams than values, it reuses the last upstream.
func (m *Metric) getUpstreamForValue(upstreams []string, valueIndex int) string {
	upstreamIndex := valueIndex
	if upstreamIndex >= len(upstreams) {
		upstreamIndex = len(upstreams) - 1
	}

	return upstreams[upstreamIndex]
}

// isUpstreamExcluded checks if the upstream server is in the exclusion list.
func (m *Metric) isUpstreamExcluded(upstream string) bool {
	return len(m.cfg.Upstream.Excludes) != 0 && slices.Contains(m.cfg.Upstream.Excludes, upstream)
}

// setMetric processes a metric value string and sets it on the appropriate Prometheus metric type.
//
// The function performs the following operations:
// 1. Trims whitespace from the value and skips empty values
// 2. Parses the value as a float64
// 3. Applies any configured math transformations (multiplication/division)
// 4. Sets the value on the appropriate metric type (counter, gauge, or histogram)
//
// Parameters:
//   - value: The string representation of the metric value to be processed
//   - labels: Prometheus labels map to identify the specific metric instance
//
// Returns:
//   - error: Returns an error if value parsing fails, counter receives negative value,
//     or if the metric type is unsupported
//
// Thread Safety:
// This function is thread-safe and can be called concurrently by multiple goroutines.
// The labels map should be unique per goroutine (handled by sync.Pool in Parse()).
//
// Examples:
//   - Counter: Adds the parsed value to the counter (must be non-negative)
//   - Gauge: Sets the gauge to the parsed value
//   - Histogram: Observes the parsed value as a sample
func (m *Metric) setMetric(value string, labels prometheus.Labels) error {
	// Handle empty values early
	value = strings.TrimSpace(value)
	if value == "" {
		return nil // Skip empty values silently
	}

	valueFloat, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fmt.Errorf("failed to parse value %q: %w", value, err)
	}

	// Apply math transformations if configured
	valueFloat = m.applyMathTransformations(valueFloat)

	// Set the metric value based on type
	return m.setMetricValue(valueFloat, labels)
}

// applyMathTransformations applies division and multiplication if configured.
func (m *Metric) applyMathTransformations(value float64) float64 {
	if !m.cfg.Math.Enabled {
		return value
	}

	if m.cfg.Math.Div != 0 {
		value /= m.cfg.Math.Div
	}

	if m.cfg.Math.Mul != 0 {
		value *= m.cfg.Math.Mul
	}

	return value
}

// setMetricValue sets the value on the appropriate metric type.
func (m *Metric) setMetricValue(value float64, labels prometheus.Labels) error {
	switch metric := m.metric.(type) {
	case *prometheus.CounterVec:
		if value < 0 {
			return fmt.Errorf("counter value cannot be negative: %f", value)
		}

		metric.With(labels).Add(value)
	case *prometheus.GaugeVec:
		metric.With(labels).Set(value)
	case *prometheus.HistogramVec:
		metric.With(labels).Observe(value)
	default:
		return fmt.Errorf("unsupported metric type %s", m.cfg.Type)
	}

	return nil
}

func (m *Metric) valueReplacements(replacements []config.Replacement, labelValue string) string {
	if len(replacements) == 0 {
		return labelValue
	}

	for _, replacement := range replacements {
		if replacement.StringReplacer != nil && strings.Contains(labelValue, *replacement.String) {
			return replacement.StringReplacer.Replace(labelValue)
		}

		if replacement.Regexp != nil && replacement.Regexp.MatchString(labelValue) {
			return replacement.Regexp.ReplaceAllString(labelValue, replacement.Replacement)
		}
	}

	return labelValue
}
