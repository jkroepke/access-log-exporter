package collector

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"

	"github.com/jkroepke/access-log-exporter/internal/config"
	"gopkg.in/mcuadros/go-syslog.v2"
	"gopkg.in/mcuadros/go-syslog.v2/format"
)

func (c *Collector) startPump(ctx context.Context, conf config.Syslog) error {
	if conf.ListenAddress == "" {
		return ErrNoSource
	}

	err := c.pumpSyslog(ctx, conf)
	if err != nil {
		return err
	}

	return nil
}

func (c *Collector) pumpSyslog(ctx context.Context, conf config.Syslog) error {
	server := syslog.NewServer()
	server.SetFormat(syslog.RFC3164)
	server.SetHandler(c)

	uri, err := url.Parse(conf.ListenAddress)
	if err != nil {
		return fmt.Errorf("could not parse syslog listen address '%s': %w", conf.ListenAddress, err)
	}

	switch uri.Scheme {
	case "tcp":
		err = server.ListenTCP(uri.Host)
	case "udp":
		err = server.ListenUDP(uri.Host)
	case "unix":
		err = server.ListenUnixgram(uri.Host + uri.Path)
	default:
		return errors.New("syslog server should be in format unix/tcp/udp://127.0.0.1:5533")
	}

	if err != nil {
		return fmt.Errorf("could not start syslog server on '%s': %w", conf.ListenAddress, err)
	}

	c.logger.InfoContext(ctx, "syslog server listening", slog.String("address", conf.ListenAddress))

	err = server.Boot()
	if err != nil {
		return fmt.Errorf("could not boot syslog server: %w", err)
	}

	c.logger.InfoContext(ctx, "syslog server booted", slog.String("address", conf.ListenAddress))
	c.wg.Add(1)

	go func() {
		defer c.wg.Done()

		<-ctx.Done()

		c.logger.InfoContext(ctx, "shutting down syslog server", slog.String("address", conf.ListenAddress))

		_ = server.Kill()

		server.Wait()

		if uri.Scheme == "unix" {
			_ = os.Remove(uri.Host + uri.Path)
		}

		c.logger.InfoContext(ctx, "syslog server shutdown complete", slog.String("address", conf.ListenAddress))
	}()

	return nil
}

func (c *Collector) Handle(log format.LogParts, _ int64, err error) {
	if err != nil {
		c.logger.Error("error while handling syslog message", slog.Any("error", err))

		return
	}

	msg, ok := log["content"].(string)
	if !ok {
		c.logger.Error("could not convert syslog message to string", slog.Any("log", log))

		return
	}

	c.buffer <- msg
}
