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
func (c *Collector) lineHandlerWorkers(ctx context.Context, logger *slog.Logger, workerCount int, messageCh <-chan string) {
	if workerCount <= 0 {
		workerCount = runtime.NumCPU()
	}

	for range workerCount {
		c.wg.Go(func() {
			c.lineHandlerWorker(ctx, logger, messageCh)
		})
	}

	logger.InfoContext(ctx, "line handler started", slog.Int("workers", runtime.NumCPU()))
}

// lineHandlerWorker is a worker that will read messages from the message channel
// and call the lineHandler method to process them.
// It will log any errors that occur during parsing and increment the metricLogParseError.
// The worker will stop when the context is done or when the message channel is closed.
func (c *Collector) lineHandlerWorker(ctx context.Context, logger *slog.Logger, messageCh <-chan string) {
	var err error

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-messageCh:
			if !ok {
				return
			}

			c.metricLogLastReceived.SetToCurrentTime()

			err = c.lineHandler(strings.Split(msg, "\t"))
			if err != nil {
				logger.LogAttrs(ctx, slog.LevelDebug, "error parsing metric",
					slog.Any("err", err),
					slog.String("line", msg),
				)

				c.metricLogParseError.Inc()
			}
		}
	}
}

// lineHandler processes a single line of log data.
func (c *Collector) lineHandler(line []string) error {
	errs := make([]error, 0)

	for _, met := range c.metrics {
		err := met.Parse(line)
		if err != nil {
			errs = append(errs, fmt.Errorf("metric %s: %w", met.Name(), err))
		}
	}

	if len(errs) != 0 {
		return errors.Join(errs...)
	}

	return nil
}
