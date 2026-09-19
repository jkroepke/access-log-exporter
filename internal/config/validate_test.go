package config_test

import (
	"testing"

	"github.com/jkroepke/access-log-exporter/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidate(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		conf config.Config
		err  string
	}{
		{
			config.Config{},
			"preset '' not found in configuration",
		},
	} {
		t.Run(tc.err, func(t *testing.T) {
			t.Parallel()

			err := config.Validate(tc.conf)
			if tc.err == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)

				if tc.err != "-" {
					assert.EqualError(t, err, tc.err)
				}
			}
		})
	}
}

func TestValidateTLS(t *testing.T) {
	t.Parallel()

	validConfig := func() config.Config {
		return config.Config{
			Preset:  "test",
			Presets: config.Presets{"test": {}},
		}
	}

	for _, tc := range []struct {
		name string
		conf config.Config
		err  string
	}{
		{
			name: "no TLS config",
			conf: validConfig(),
			err:  "",
		},
		{
			name: "both TLS flags set",
			conf: func() config.Config {
				c := validConfig()
				c.Web.TLSCertFile = "/path/to/cert.pem"
				c.Web.TLSKeyFile = "/path/to/key.pem"

				return c
			}(),
			err: "",
		},
		{
			name: "cert without key",
			conf: func() config.Config {
				c := validConfig()
				c.Web.TLSCertFile = "/path/to/cert.pem"

				return c
			}(),
			err: "both TLS certificate and key files must be set to enable TLS",
		},
		{
			name: "key without cert",
			conf: func() config.Config {
				c := validConfig()
				c.Web.TLSKeyFile = "/path/to/key.pem"

				return c
			}(),
			err: "both TLS certificate and key files must be set to enable TLS",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := config.Validate(tc.conf)
			if tc.err == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.EqualError(t, err, tc.err)
			}
		})
	}
}


func TestValidateMetric(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		cfg  config.Metric
		err  string
	}{
		{
			name: "valid counter",
			cfg: config.Metric{
				Name: "http_requests_total",
				Type: "counter",
			},
		},
		{
			name: "duplicate label",
			cfg: config.Metric{
				Name: "http_requests_total",
				Type: "counter",
				Labels: []config.Label{
					{Name: "method"},
					{Name: "method"},
				},
			},
			err: `metric label "method" is duplicated`,
		},
		{
			name: "upstream label collision",
			cfg: config.Metric{
				Name:       "http_request_duration_seconds",
				Type:       "histogram",
				ValueIndex: new(uint(1)),
				Labels: []config.Label{
					{Name: "upstream"},
				},
				Upstream: config.Upstream{
					Enabled: true,
					Label:   true,
				},
			},
			err: `metric label "upstream" conflicts with the upstream label`,
		},
		{
			name: "upstream settings without enable",
			cfg: config.Metric{
				Name: "http_requests_total",
				Type: "counter",
				Upstream: config.Upstream{
					Label: true,
				},
			},
			err: "upstream settings require upstream.enabled",
		},
		{
			name: "upstream counter without value",
			cfg: config.Metric{
				Name: "http_requests_total",
				Type: "counter",
				Upstream: config.Upstream{
					Enabled: true,
				},
			},
			err: "valueIndex must be set when upstream processing is enabled",
		},
		{
			name: "unordered histogram buckets",
			cfg: config.Metric{
				Name:       "http_request_duration_seconds",
				Type:       "histogram",
				ValueIndex: new(uint(1)),
				Buckets:    []float64{0.1, 0.1},
			},
			err: "histogram buckets must be in increasing order",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := config.ValidateMetric(tc.cfg)
			if tc.err == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.err)
			}
		})
	}
}

func TestValidateStaticEndpoints(t *testing.T) {
	t.Parallel()

	validConfig := func() config.Config {
		conf := config.Defaults
		conf.Preset = "test"
		conf.Presets = config.Presets{"test": {}}

		return conf
	}

	t.Run("invalid syslog scheme", func(t *testing.T) {
		t.Parallel()

		conf := validConfig()
		conf.Syslog.ListenAddress = "tcp://127.0.0.1:8514"

		require.ErrorContains(t, config.Validate(conf), "udp:// or unix://")
	})

	t.Run("invalid nginx scheme", func(t *testing.T) {
		t.Parallel()

		conf := validConfig()
		require.NoError(t, conf.Nginx.ScrapeURL.UnmarshalText([]byte("ftp://127.0.0.1/stub_status")))

		require.ErrorContains(t, config.Validate(conf), "unsupported scheme")
	})
}
