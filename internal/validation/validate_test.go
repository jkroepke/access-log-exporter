package validation_test

import (
	"testing"

	"github.com/jkroepke/access-log-exporter/internal/config"
	"github.com/jkroepke/access-log-exporter/internal/config/types"
	"github.com/jkroepke/access-log-exporter/internal/validation"
	"github.com/stretchr/testify/require"
)

func TestValidatePrometheusDescriptors(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		metrics []config.Metric
		err     string
	}{
		{
			name: "valid metric",
			metrics: []config.Metric{
				{
					Name: "http_requests_total",
					Type: "counter",
					Help: "Total HTTP requests",
				},
			},
		},
		{
			name: "invalid metric name",
			metrics: []config.Metric{
				{
					Name: "invalid metric",
					Type: "counter",
					Help: "Invalid metric",
				},
			},
			err: "invalid or conflicting Prometheus descriptors",
		},
		{
			name: "duplicate metric descriptor",
			metrics: []config.Metric{
				{
					Name: "http_requests_total",
					Type: "counter",
					Help: "Total HTTP requests",
				},
				{
					Name: "http_requests_total",
					Type: "counter",
					Help: "Total HTTP requests",
				},
			},
			err: "invalid or conflicting Prometheus descriptors",
		},
		{
			name: "conflicting metric descriptor",
			metrics: []config.Metric{
				{
					Name: "http_requests_total",
					Type: "counter",
					Help: "Total HTTP requests",
				},
				{
					Name: "http_requests_total",
					Type: "counter",
					Help: "Different help",
					Labels: []config.Label{
						{Name: "method"},
					},
				},
			},
			err: "invalid or conflicting Prometheus descriptors",
		},
		{
			name: "built-in metric conflict",
			metrics: []config.Metric{
				{
					Name: "log_parse_errors_total",
					Type: "counter",
					Help: "Custom metric",
				},
			},
			err: "invalid or conflicting Prometheus descriptors",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			conf := validConfig(tc.metrics)
			err := validation.Validate(conf)

			if tc.err == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.err)
			}
		})
	}
}

func TestValidateNginxMetricConflict(t *testing.T) {
	t.Parallel()

	conf := validConfig([]config.Metric{
		{
			Name: "nginx_up",
			Type: "gauge",
			Help: "Custom Nginx metric",
			ValueIndex: new(uint(0)),
		},
	})
	conf.Nginx.ScrapeURL = types.URL{}
	require.NoError(t, conf.Nginx.ScrapeURL.UnmarshalText([]byte("http://127.0.0.1/stub_status")))

	err := validation.Validate(conf)

	require.ErrorContains(t, err, "invalid or conflicting Prometheus descriptors")
}

func validConfig(metrics []config.Metric) config.Config {
	conf := config.Defaults
	conf.Preset = "test"
	conf.Presets = config.Presets{
		"test": {
			Metrics: metrics,
		},
	}

	return conf
}
