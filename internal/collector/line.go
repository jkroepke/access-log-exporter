package collector

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"strings"
)

// lineHandlerWorkers starts several workers that will handle incoming
// messages from the message channel.
// Each worker will parse the incoming message and call the lineHandler method to process it.
// The amount workers can be specified, and if less than or equal to zero, it defaults to the amount CPU cores available.
func (c *Collector) lineHandlerWorkers(ctx context.Context, workerCount int, messageCh <-chan string) {
	if workerCount <= 0 {
		workerCount = runtime.NumCPU()
	}

	for range workerCount {
		c.wg.Add(1)

		go func() {
			defer c.wg.Done()

			c.lineHandlerWorker(ctx, messageCh)
		}()
	}

	c.logger.InfoContext(ctx, "line handler started", slog.Int("workers", runtime.NumCPU()))
}

// lineHandlerWorker is a worker that will read messages from the message channel
// and call the lineHandler method to process them.
// It will log any errors that occur during parsing and increment the parseErrorMetric.
// The worker will stop when the context is done or when the message channel is closed.
func (c *Collector) lineHandlerWorker(ctx context.Context, messageCh <-chan string) {
	var err error

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-messageCh:
			if !ok {
				return
			}

			err = c.lineHandler(strings.Split(msg, "\t"))
			if err != nil {
				c.logger.DebugContext(ctx, "error parsing metric",
					slog.Any("err", err),
					slog.String("line", msg),
				)

				c.parseErrorMetric.Inc()
			}
		}
	}
}

// lineHandler processes a single line of log data.
func (c *Collector) lineHandler(line []string) error {
	errs := make([]error, 0)

	for i, met := range c.metrics {
		err := met.Parse(line)
		if err != nil {
			var metricName string

			if i < len(c.preset.Metrics) {
				metricName = c.preset.Metrics[i].Name
			}

			errs = append(errs, fmt.Errorf("metric %q: %w", metricName, err))
		}
	}

	if len(errs) != 0 {
		return errors.Join(errs...)
	}

	return nil
}
