package syslog

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strings"

	"gopkg.in/mcuadros/go-syslog.v2"
	"gopkg.in/mcuadros/go-syslog.v2/format"
)

type Syslog struct {
	server     *syslog.Server
	logger     *slog.Logger
	msgCh      chan<- string
	listenAddr string
}

func New(logger *slog.Logger, listenAddr string, msgCh chan<- string) (Syslog, error) {
	syslogServer := Syslog{
		listenAddr: listenAddr,
		logger:     logger.With(slog.String("component", "syslog")),
		msgCh:      msgCh,
	}

	syslogServer.server = syslog.NewServer()
	syslogServer.server.SetFormat(syslog.RFC3164)
	syslogServer.server.SetHandler(syslogServer)

	uri, err := url.Parse(listenAddr)
	if err != nil {
		return Syslog{}, fmt.Errorf("could not parse syslog listen address '%s': %w", listenAddr, err)
	}

	switch uri.Scheme {
	case "tcp":
		err = syslogServer.server.ListenTCP(uri.Host)
	case "udp":
		err = syslogServer.server.ListenUDP(uri.Host)
	case "unix":
		err = syslogServer.server.ListenUnixgram(uri.Host + uri.Path)
	default:
		err = errors.New("syslog listen address must be start with tcp://, udp:// or unix://")
	}

	if err != nil {
		return Syslog{}, fmt.Errorf("could not start syslog server on '%s': %w", listenAddr, err)
	}

	err = syslogServer.server.Boot()
	if err != nil {
		return Syslog{}, fmt.Errorf("could not boot syslog server: %w", err)
	}

	return syslogServer, nil
}

func (s Syslog) Handle(parts format.LogParts, _ int64, err error) {
	if err != nil {
		s.logger.Error("error while handling syslog message", slog.Any("error", err))

		return
	}

	msg, ok := parts["content"].(string)
	if !ok {
		s.logger.Error("could not convert syslog message to string", slog.Any("log", parts))

		return
	}

	s.msgCh <- msg
}

func (s Syslog) Shutdown(ctx context.Context) error {
	if s.server == nil {
		return errors.New("syslog server is not initialized")
	}

	if err := s.server.Kill(); err != nil {
		return fmt.Errorf("could not stop syslog server: %w", err)
	}

	s.server.Wait()

	if strings.HasPrefix(s.listenAddr, "unix://") {
		_ = os.Remove(strings.TrimPrefix(s.listenAddr, "unix://"))
	}

	s.logger.InfoContext(ctx, "syslog server shutdown complete")

	return nil
}
