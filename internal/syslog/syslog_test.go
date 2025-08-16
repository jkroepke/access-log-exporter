package syslog_test

import (
	"fmt"
	"log/slog"
	syslogclient "log/syslog"
	"net"
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

	server, err := syslog.New(t.Context(), slog.New(slog.DiscardHandler), "unix://"+unixSocket, logBuffer)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, server.Close(t.Context()))
	})

	var serverErr error

	go func() {
		serverErr = server.Start()
	}()

	t.Cleanup(func() {
		require.NoError(t, serverErr)
	})

	syslogClient, err := syslogclient.Dial("unixgram", unixSocket, syslogclient.LOG_LOCAL7, "")
	require.NoError(t, err)

	logMessage := "Test message"

	_, err = fmt.Fprint(syslogClient, logMessage)
	require.NoError(t, err)

	require.Equal(t, logMessage, <-logBuffer)
}

func TestSyslogServerRawMessage(t *testing.T) {
	t.Parallel()

	unixSocket, err := nettest.LocalPath()
	require.NoError(t, err)

	logBuffer := make(chan string, 1)

	server, err := syslog.New(t.Context(), slog.New(slog.DiscardHandler), "unix://"+unixSocket, logBuffer)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, server.Close(t.Context()))
	})

	var serverErr error

	go func() {
		serverErr = server.Start()
	}()

	t.Cleanup(func() {
		require.NoError(t, serverErr)
	})

	var dial net.Dialer

	syslogClient, err := dial.DialContext(t.Context(), "unixgram", unixSocket)
	require.NoError(t, err)

	_, err = syslogClient.Write([]byte("<190>Aug 15 20:16:01 nginx: localhost:8080\tGET\t404\t0.000\t767\t710"))
	require.NoError(t, err)

	logMessage := "localhost:8080\tGET\t404\t0.000\t767\t710"

	_, err = fmt.Fprint(syslogClient, logMessage)
	require.NoError(t, err)

	require.Equal(t, logMessage, <-logBuffer)
}

func TestSyslogServerWithInvalidMessages(t *testing.T) {
	t.Parallel()

	unixSocket, err := nettest.LocalPath()
	require.NoError(t, err)

	logBuffer := make(chan string, 1)

	server, err := syslog.New(t.Context(), slog.New(slog.DiscardHandler), "unix://"+unixSocket, logBuffer)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, server.Close(t.Context()))
	})

	var serverErr error

	go func() {
		serverErr = server.Start()
	}()

	t.Cleanup(func() {
		require.NoError(t, serverErr)
	})

	var dial net.Dialer

	syslogClient, err := dial.DialContext(t.Context(), "unixgram", unixSocket)
	require.NoError(t, err)

	logMessage := "<34>Oct 11 22:14:15"

	_, err = fmt.Fprint(syslogClient, logMessage)
	require.NoError(t, err)

	require.Empty(t, logBuffer)
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

			_, err := syslog.New(t.Context(), slog.New(slog.DiscardHandler), tc, nil)
			require.Error(t, err)
		})
	}
}
