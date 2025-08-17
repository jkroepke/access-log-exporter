package main

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHelpFlag(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}

	rt := run(t.Context(), []string{"access-log-exporter", "--help"}, stdout, nil)
	require.Equal(t, ReturnCodeOK, rt, stdout)
	require.Contains(t, stdout.String(), "Documentation available at")
}

func TestVersionFlag(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}

	rt := run(t.Context(), []string{"access-log-exporter", "--version"}, stdout, nil)
	require.Equal(t, ReturnCodeOK, rt, stdout)
	require.Contains(t, stdout.String(), "version")
}

func TestConfigFileNotFound(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}

	rt := run(t.Context(), []string{"access-log-exporter", "--config=invalid"}, stdout, nil)
	require.Equal(t, ReturnCodeError, rt, stdout)
	require.Contains(t, stdout.String(), "error opening config file invalid")
}

func TestDefaultConfigFile(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}

	rt := run(t.Context(), []string{"access-log-exporter"}, stdout, nil)
	require.Equal(t, ReturnCodeError, rt, stdout)
	require.Contains(t, stdout.String(), "error opening config file config.yaml")
}

func TestEmptyConfigFile(t *testing.T) {
	stdout := &bytes.Buffer{}

	createTemp, err := os.CreateTemp(t.TempDir(), "config.yaml")
	require.NoError(t, err)

	t.Setenv("CONFIG_FILE", createTemp.Name())

	t.Cleanup(func() {
		require.NoError(t, createTemp.Close())
	})

	rt := run(t.Context(), []string{"access-log-exporter"}, stdout, nil)
	require.Equal(t, ReturnCodeError, rt, stdout)
	require.Contains(t, stdout.String(), "configuration file is empty")
}

func TestInvalidPreset(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}

	wd, err := os.Getwd()
	require.NoError(t, err)

	moduleRoot, err := findModuleRoot(wd)
	require.NoError(t, err)

	rt := run(t.Context(), []string{"access-log-exporter", "--config=" + moduleRoot + "/packaging/etc/access-log-exporter/config.yaml", "--preset", "invalid"}, stdout, nil)
	require.Equal(t, ReturnCodeError, rt, stdout)
	require.Contains(t, stdout.String(), "preset 'invalid' not found in configuration")
}

func TestVerifyConfig(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}

	wd, err := os.Getwd()
	require.NoError(t, err)

	moduleRoot, err := findModuleRoot(wd)
	require.NoError(t, err)

	rt := run(t.Context(), []string{
		"access-log-exporter",
		"--config=" + moduleRoot + "/packaging/etc/access-log-exporter/config.yaml",
		"--log.format=json",
		"--verify-config",
	}, stdout, nil)
	require.Equal(t, ReturnCodeOK, rt, stdout)
}
