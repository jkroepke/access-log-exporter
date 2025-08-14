package syslog_test

import (
	"fmt"
	"log/slog"
	syslogclient "log/syslog"
	"testing"

	"github.com/jkroepke/access-log-exporter/internal/syslog"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/nettest"
)

func TestSyslogServer(t *testing.T) {
	t.Parallel()

	unixSocket, err := nettest.LocalPath()
	require.NoError(t, err)

	logBuffer := make(chan string, 1)

	server, err := syslog.New(slog.New(slog.DiscardHandler), "unix://"+unixSocket, logBuffer)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, server.Shutdown(t.Context()))
	})

	syslogClient, err := syslogclient.Dial("unixgram", unixSocket, syslogclient.LOG_LOCAL7, "")
	require.NoError(t, err)

	logMessage := "Test message"

	_, err = fmt.Fprint(syslogClient, logMessage)
	require.NoError(t, err)

	require.Equal(t, logMessage, <-logBuffer)
}

func TestSyslogInvalidListenAddr(t *testing.T) {
	t.Parallel()

	for _, tc := range []string{
		"://address",
		"invalid://address",
		"tcp://invalid:1234",
		"udp://invalid:1234",
		"udp://0.0.0.1:1000000",
	} {
		t.Run(tc, func(t *testing.T) {
			t.Parallel()

			_, err := syslog.New(slog.New(slog.DiscardHandler), tc, nil)
			require.Error(t, err)
		})
	}
}
