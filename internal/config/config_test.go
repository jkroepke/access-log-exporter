package config_test

import (
	"bytes"
	"flag"
	"io"
	"os"
	"testing"

	"github.com/jkroepke/access-log-exporter/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name       string
		configFile string
		conf       config.Config
		err        error
	}{
		{
			"empty file",
			"",
			config.Defaults,
			nil,
		},
		{
			"minimal file",
			// language=yaml
			`
web:
  listen-address: ":9000"
`,
			func() config.Config {
				conf := config.Defaults
				conf.Web.ListenAddress = ":9000"

				return conf
			}(),
			nil,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer

			_ = io.Writer(&buf)

			file, err := os.CreateTemp(t.TempDir(), "access-log-exporter-*")
			require.NoError(t, err)

			// close and remove the temporary file at the end of the program.
			t.Cleanup(func() {
				require.NoError(t, file.Close())
				require.NoError(t, os.Remove(file.Name()))
			})

			_, err = file.WriteString(tc.configFile)
			require.NoError(t, err)

			conf, err := config.New([]string{"access-log-exporter", "--config", file.Name()}, &buf)
			if tc.err != nil {
				require.Error(t, err)
				assert.Equal(t, tc.err.Error(), err.Error())
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.conf, conf)
			}
		})
	}
}

func TestConfigHelpFlag(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	_ = io.Writer(&buf)

	_, err := config.New([]string{"access-log-exporter", "--help"}, &buf)

	require.ErrorIs(t, err, flag.ErrHelp)
}

func TestConfigVersionFlag(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	_ = io.Writer(&buf)

	_, err := config.New([]string{"access-log-exporter", "--version"}, &buf)

	require.ErrorIs(t, err, config.ErrVersion)
}
