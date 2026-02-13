package nginx_test

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jkroepke/access-log-exporter/internal/nginx"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/require"
)

func TestCollector(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		handler http.HandlerFunc
		metrics string
	}{
		{
			name: "valid URL",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Add("Server", "nginx")
				w.WriteHeader(http.StatusOK)

				_, err := w.Write([]byte("Active connections: 1\nserver accepts handled requests\n10 10 10\nReading: 0 Writing: 1 Waiting: 0\n"))
				if err != nil {
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				}
			},
			metrics: `# HELP nginx_connections_accepted_total Accepted client connections.
# TYPE nginx_connections_accepted_total counter
nginx_connections_accepted_total 10
# HELP nginx_connections_active Active client connections.
# TYPE nginx_connections_active gauge
nginx_connections_active 1
# HELP nginx_connections_handled_total Handled client connections.
# TYPE nginx_connections_handled_total counter
nginx_connections_handled_total 10
# HELP nginx_connections_reading Connections where NGINX is reading the request header.
# TYPE nginx_connections_reading gauge
nginx_connections_reading 0
# HELP nginx_connections_waiting Idle client connections.
# TYPE nginx_connections_waiting gauge
nginx_connections_waiting 0
# HELP nginx_connections_writing Connections where NGINX is writing the response back to the client.
# TYPE nginx_connections_writing gauge
nginx_connections_writing 1
# HELP nginx_up Whether the NGINX server is up (1) or down (0). 1 means the server is up and metrics are being collected, 0 means the server is down or unreachable.
# TYPE nginx_up gauge
nginx_up{version="N/A"} 1`,
		},
		{
			name: "valid URL with version",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Add("Server", "nginx/1.23.1")
				w.WriteHeader(http.StatusOK)

				_, err := w.Write([]byte("Active connections: 1\nserver accepts handled requests\n10 10 10\nReading: 0 Writing: 1 Waiting: 0\n"))
				if err != nil {
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				}
			},
			metrics: `# HELP nginx_connections_accepted_total Accepted client connections.
# TYPE nginx_connections_accepted_total counter
nginx_connections_accepted_total 10
# HELP nginx_connections_active Active client connections.
# TYPE nginx_connections_active gauge
nginx_connections_active 1
# HELP nginx_connections_handled_total Handled client connections.
# TYPE nginx_connections_handled_total counter
nginx_connections_handled_total 10
# HELP nginx_connections_reading Connections where NGINX is reading the request header.
# TYPE nginx_connections_reading gauge
nginx_connections_reading 0
# HELP nginx_connections_waiting Idle client connections.
# TYPE nginx_connections_waiting gauge
nginx_connections_waiting 0
# HELP nginx_connections_writing Connections where NGINX is writing the response back to the client.
# TYPE nginx_connections_writing gauge
nginx_connections_writing 1
# HELP nginx_up Whether the NGINX server is up (1) or down (0). 1 means the server is up and metrics are being collected, 0 means the server is down or unreachable.
# TYPE nginx_up gauge
nginx_up{version="1.23.1"} 1`,
		},
		{
			name: "invalid format",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)

				_, err := w.Write([]byte("Invalid format"))
				if err != nil {
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				}
			},
			metrics: `# HELP nginx_up Whether the NGINX server is up (1) or down (0). 1 means the server is up and metrics are being collected, 0 means the server is down or unreachable.
# TYPE nginx_up gauge
nginx_up{version="N/A"} 0`,
		},
		{
			name: "empty response",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)

				_, err := w.Write([]byte(""))
				if err != nil {
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				}
			},
			metrics: `# HELP nginx_up Whether the NGINX server is up (1) or down (0). 1 means the server is up and metrics are being collected, 0 means the server is down or unreachable.
# TYPE nginx_up gauge
nginx_up{version="N/A"} 0`,
		},
		{
			name: "access denied",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusForbidden)
			},
			metrics: `# HELP nginx_up Whether the NGINX server is up (1) or down (0). 1 means the server is up and metrics are being collected, 0 means the server is down or unreachable.
# TYPE nginx_up gauge
nginx_up{version="N/A"} 0`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			stubServer := httptest.NewServer(tc.handler)
			t.Cleanup(stubServer.Close)

			col := nginx.New(slog.New(slog.DiscardHandler), stubServer.URL)

			metrics, err := MetricsToText(t, col)
			require.NoError(t, err)
			require.Equal(t, tc.metrics, metrics)
		})
	}
}

func TestCollector_NoServer(t *testing.T) {
	t.Parallel()

	col := nginx.New(slog.New(slog.DiscardHandler), "http://nonexistent-server")

	metrics, err := MetricsToText(t, col)
	require.NoError(t, err)

	expected := `# HELP nginx_up Whether the NGINX server is up (1) or down (0). 1 means the server is up and metrics are being collected, 0 means the server is down or unreachable.
# TYPE nginx_up gauge
nginx_up{version="N/A"} 0`

	require.Equal(t, expected, metrics)
}

func MetricsToText(tb testing.TB, met prometheus.Collector) (string, error) {
	tb.Helper()

	reg := prometheus.NewRegistry()
	err := reg.Register(met)
	require.NoError(tb, err)

	request, err := http.NewRequestWithContext(tb.Context(), http.MethodGet, "/", nil)
	require.NoError(tb, err)

	request.Header.Add("Accept", "text/plain")

	writer := httptest.NewRecorder()

	regHandler := promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
	regHandler.ServeHTTP(writer, request)

	require.Equal(tb, http.StatusOK, writer.Code)

	allMetrics, err := io.ReadAll(writer.Body)
	if err != nil {
		return "", fmt.Errorf("error reading writer body: %w", err)
	}

	return strings.TrimSpace(string(allMetrics)), nil
}
