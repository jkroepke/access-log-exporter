package collector

import (
	"context"
	"log/slog"
	"runtime"
	"strings"
)

func (c *Collector) lineHandler(ctx context.Context, workerCount uint) error {
	for range workerCount {
		c.wg.Add(1)

		go func() {
			defer c.wg.Done()

			c.lineHandlerWorker(ctx)
		}()
	}

	c.logger.InfoContext(ctx, "line handler started", slog.Int("workers", runtime.NumCPU()))

	return nil
}

func (c *Collector) lineHandlerWorker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-c.buffer:
			if !ok {
				return
			}

			line := strings.Split(msg, "\t")

			for i, met := range c.metrics {
				err := met.Parse(line)
				if err != nil {
					var metricName string
					if i < len(c.preset.Metrics) {
						metricName = c.preset.Metrics[i].Name
					}

					c.logger.DebugContext(ctx, "error parsing metric",
						slog.String("metric", metricName),
						slog.Any("error", err),
						slog.String("line", msg),
					)

					c.parseErrorMetric.Inc()
				}
			}
		}
	}
}
