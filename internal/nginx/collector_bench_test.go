package nginx_test

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jkroepke/access-log-exporter/internal/nginx"
	"github.com/prometheus/client_golang/prometheus"
)

func BenchmarkCollectorCollect(b *testing.B) {
	const stubStatus = "Active connections: 1\nserver accepts handled requests\n10 10 10\nReading: 0 Writing: 1 Waiting: 0\n"

	stubServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Add("Server", "nginx/1.29.0")
		w.WriteHeader(http.StatusOK)

		_, _ = w.Write([]byte(stubStatus))
	}))
	b.Cleanup(stubServer.Close)

	col := nginx.New(slog.New(slog.DiscardHandler), stubServer.URL)
	metrics := make(chan prometheus.Metric, 7)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		col.Collect(metrics)

		for range 7 {
			<-metrics
		}
	}
}
