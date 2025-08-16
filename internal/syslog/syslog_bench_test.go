package syslog_test

import (
	"log/slog"
	"net"
	"testing"

	"github.com/jkroepke/access-log-exporter/internal/syslog"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/nettest"
)

func Benchmark_Syslog(b *testing.B) {
	unixSocket, err := nettest.LocalPath()
	require.NoError(b, err)

	logBuffer := make(chan string, 1)

	server, err := syslog.New(b.Context(), slog.New(slog.DiscardHandler), "unix://"+unixSocket, logBuffer)
	require.NoError(b, err)

	b.Cleanup(func() {
		require.NoError(b, server.Close(b.Context()))
	})

	var dial net.Dialer

	syslogClient, err := dial.DialContext(b.Context(), "unixgram", unixSocket)
	require.NoError(b, err)

	logMessageBytes := []byte("<190>Aug 15 20:16:01 nginx: localhost:8080\tGET\t404\t0.000\t767\t710")

	var serverErr error

	go func() {
		serverErr = server.Start()
	}()

	b.Cleanup(func() {
		require.NoError(b, serverErr)
	})

	// Benchmark the syslog server by sending a message
	b.ResetTimer()

	for b.Loop() {
		_, _ = syslogClient.Write(logMessageBytes)

		<-logBuffer
	}

	b.ReportAllocs()
}
