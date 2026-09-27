package collector

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"strings"

	"github.com/jkroepke/access-log-exporter/internal/syslog"
)

// lineHandlerWorkers starts several workers that will handle incoming
// messages from the message channel.
// Each worker will parse the incoming message and call the lineHandler method to process it.
// The amount workers can be specified, and if less than or equal to zero, it defaults to the amount CPU cores available.
func (c *Collector) lineHandlerWorkers(ctx context.Context, logger *slog.Logger, workerCount int, messageCh <-chan syslog.Message) {
	if workerCount <= 0 {
		workerCount = runtime.NumCPU()
	}

	for range workerCount {
		c.wg.Go(func() {
			c.lineHandlerWorker(ctx, logger, messageCh)
		})
	}

	logger.InfoContext(ctx, "line handler started", slog.Int("workers", workerCount))
}

// lineHandlerWorker is a worker that will read messages from the message channel
// and call the lineHandler method to process them.
// It will log any errors that occur during parsing and increment the metricLogParseError.
// The worker will stop when the context is done or when the message channel is closed.
func (c *Collector) lineHandlerWorker(ctx context.Context, logger *slog.Logger, messageCh <-chan syslog.Message) {
	var err error

	fields := make([]string, 0, 16)

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-messageCh:
			if !ok {
				return
			}

			c.metricLogLastReceived.SetToCurrentTime()

			fields = splitLineFields(fields, msg.Line)

			err = c.lineHandler(fields)
			if err != nil {
				logger.LogAttrs(
					ctx, slog.LevelDebug, "error parsing metric",
					slog.Any("err", err),
					slog.String("line", msg.Line),
				)

				c.metricLogParseError.Inc()
			}

			msg.Release()
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

func splitLineFields(fields []string, line string) []string {
	fields = fields[:0]

	for {
		index := strings.IndexByte(line, '\t')
		if index == -1 {
			return append(fields, line)
		}

		fields = append(fields, line[:index])
		line = line[index+1:]
	}
}
