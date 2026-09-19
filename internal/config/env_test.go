package config //nolint:testpackage

import (
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadFromFlagAndEnvironment(t *testing.T) {
	t.Setenv("CONFIG_DEBUG_ENABLE", "true")
	t.Setenv("CONFIG_NGINX_SCRAPE__TIMEOUT", "17s")
	t.Setenv("CONFIG_BUFFER__SIZE", "42")
	t.Setenv("CONFIG_WORKER", "4")
	t.Setenv("CONFIG_LOG_LEVEL", "debug")

	conf := Defaults
	require.NoError(t, conf.ReadFromFlagAndEnvironment([]string{"access-log-exporter"}, io.Discard))

	assert.True(t, conf.Debug.Enable)
	assert.Equal(t, 17*time.Second, conf.Nginx.ScrapeTimeout)
	assert.Equal(t, uint(42), conf.BufferSize)
	assert.Equal(t, 4, conf.WorkerCount)
	assert.Equal(t, "DEBUG", conf.Log.Level.String())
}

func TestReadFromFlagAndEnvironmentRejectsNonBooleanEnvironmentValues(t *testing.T) {
	for _, value := range []string{"", "0", "1", "FALSE", "TRUE", "f", "t", "yes"} {
		t.Run(value, func(t *testing.T) {
			const environmentVariable = "CONFIG_DEBUG_ENABLE"

			t.Setenv(environmentVariable, value)

			conf := Defaults
			err := conf.ReadFromFlagAndEnvironment([]string{"access-log-exporter"}, io.Discard)

			require.Error(t, err)
			require.ErrorContains(t, err, environmentVariable)
			require.ErrorContains(t, err, "true or false")
		})
	}
}

func TestReadFromFlagAndEnvironmentRejectsInvalidEnvironmentValues(t *testing.T) {
	for _, tc := range []struct {
		name                string
		environmentVariable string
		value               string
	}{
		{
			name:                "duration",
			environmentVariable: "CONFIG_NGINX_SCRAPE__TIMEOUT",
			value:               "eventually",
		},
		{
			name:                "log level",
			environmentVariable: "CONFIG_LOG_LEVEL",
			value:               "verbose",
		},
		{
			name:                "unsigned integer",
			environmentVariable: "CONFIG_BUFFER__SIZE",
			value:               "many",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(tc.environmentVariable, tc.value)

			conf := Defaults
			err := conf.ReadFromFlagAndEnvironment([]string{"access-log-exporter"}, io.Discard)

			require.Error(t, err)
			require.ErrorContains(t, err, tc.environmentVariable)
		})
	}
}

func TestReadFromFlagAndEnvironmentCLIOverridesEnvironment(t *testing.T) {
	t.Setenv("CONFIG_WORKER", "4")

	conf := Defaults
	require.NoError(t, conf.ReadFromFlagAndEnvironment(
		[]string{"access-log-exporter", "--worker", "8"},
		io.Discard,
	))

	assert.Equal(t, 8, conf.WorkerCount)
}

func TestReadFromFlagAndEnvironmentIgnoresUnknownConfigEnvironmentVariables(t *testing.T) {
	t.Setenv("CONFIG_SOMETHING_ELSE", "value")

	conf := Defaults
	require.NoError(t, conf.ReadFromFlagAndEnvironment([]string{"access-log-exporter"}, io.Discard))
	assert.Equal(t, Defaults, conf)
}

func TestReadFromFlagAndEnvironmentSupportsLegacyBufferSizeVariable(t *testing.T) {
	t.Setenv("CONFIG_BUFFER_SIZE", "23")

	conf := Defaults
	require.NoError(t, conf.ReadFromFlagAndEnvironment([]string{"access-log-exporter"}, io.Discard))
	assert.Equal(t, uint(23), conf.BufferSize)
}

func TestReadFromFlagAndEnvironmentPrefersCanonicalBufferSizeVariable(t *testing.T) {
	t.Setenv("CONFIG_BUFFER_SIZE", "23")
	t.Setenv("CONFIG_BUFFER__SIZE", "42")

	conf := Defaults
	require.NoError(t, conf.ReadFromFlagAndEnvironment([]string{"access-log-exporter"}, io.Discard))
	assert.Equal(t, uint(42), conf.BufferSize)
}

func TestGetEnvironmentVariableByFlagName(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "CONFIG_FILE", getEnvironmentVariableByFlagName("config"))
	assert.Equal(t, "CONFIG_BUFFER__SIZE", getEnvironmentVariableByFlagName("buffer-size"))
	assert.Equal(t, "CONFIG_NGINX_SCRAPE__TIMEOUT", getEnvironmentVariableByFlagName("nginx.scrape-timeout"))
}
