package nginx_test

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jkroepke/access-log-exporter/internal/nginx"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/nettest"
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

			require.NoError(t, testutil.CollectAndCompare(col, strings.NewReader(strings.TrimSpace(tc.metrics)+"\n")))
		})
	}
}

func TestCollector_NoServer(t *testing.T) {
	t.Parallel()

	col := nginx.New(slog.New(slog.DiscardHandler), "http://nonexistent-server")

	expected := `# HELP nginx_up Whether the NGINX server is up (1) or down (0). 1 means the server is up and metrics are being collected, 0 means the server is down or unreachable.
# TYPE nginx_up gauge
nginx_up{version="N/A"} 0`

	require.NoError(t, testutil.CollectAndCompare(col, strings.NewReader(strings.TrimSpace(expected)+"\n")))
}

func TestCollector_UnixSocket(t *testing.T) {
	t.Parallel()

	listener, err := nettest.NewLocalListener("unix")
	require.NoError(t, err)

	stubServer := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Add("Server", "nginx/1.29.0")
		w.WriteHeader(http.StatusOK)

		_, err := w.Write([]byte("Active connections: 2\nserver accepts handled requests\n11 11 12\nReading: 0 Writing: 1 Waiting: 1\n"))
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	}))
	stubServer.Listener = listener
	stubServer.Start()
	t.Cleanup(stubServer.Close)

	col := nginx.New(slog.New(slog.DiscardHandler), "unix://"+listener.Addr().String())

	expected := `# HELP nginx_connections_accepted_total Accepted client connections.
# TYPE nginx_connections_accepted_total counter
nginx_connections_accepted_total 11
# HELP nginx_connections_active Active client connections.
# TYPE nginx_connections_active gauge
nginx_connections_active 2
# HELP nginx_connections_handled_total Handled client connections.
# TYPE nginx_connections_handled_total counter
nginx_connections_handled_total 11
# HELP nginx_connections_reading Connections where NGINX is reading the request header.
# TYPE nginx_connections_reading gauge
nginx_connections_reading 0
# HELP nginx_connections_waiting Idle client connections.
# TYPE nginx_connections_waiting gauge
nginx_connections_waiting 1
# HELP nginx_connections_writing Connections where NGINX is writing the response back to the client.
# TYPE nginx_connections_writing gauge
nginx_connections_writing 1
# HELP nginx_up Whether the NGINX server is up (1) or down (0). 1 means the server is up and metrics are being collected, 0 means the server is down or unreachable.
# TYPE nginx_up gauge
nginx_up{version="1.29.0"} 1`

	require.NoError(t, testutil.CollectAndCompare(col, strings.NewReader(strings.TrimSpace(expected)+"\n")))
}
