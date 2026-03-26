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
