package syslog

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

type Syslog struct {
	con        net.PacketConn
	logger     *slog.Logger
	msgCh      chan<- string
	poolBuffer *sync.Pool
	listenAddr string
}

func New(ctx context.Context, logger *slog.Logger, listenAddr string, msgCh chan<- string) (Syslog, error) {
	syslogServer := Syslog{
		listenAddr: listenAddr,
		logger:     logger.With(slog.String("component", "syslog")),
		msgCh:      msgCh,
		poolBuffer: &sync.Pool{
			New: func() any {
				buf := make([]byte, 4096)

				return &buf
			},
		},
	}

	uri, err := url.Parse(listenAddr)
	if err != nil {
		return Syslog{}, fmt.Errorf("could not parse syslog listen address '%s': %w", listenAddr, err)
	}

	var listenConf net.ListenConfig

	switch uri.Scheme {
	case "udp":
		syslogServer.con, err = listenConf.ListenPacket(ctx, "udp", uri.Host)
	case "unix":
		syslogServer.con, err = listenConf.ListenPacket(ctx, "unixgram", uri.Host+uri.Path)
	default:
		err = errors.New("syslog listen address must be start with udp:// or unix://")
	}

	if err != nil {
		return Syslog{}, fmt.Errorf("could not listen syslog server on '%s': %w", listenAddr, err)
	}

	return syslogServer, nil
}

//nolint:gocognit,cyclop
func (s *Syslog) Start() error {
	for {
		buf, _ := s.poolBuffer.Get().(*[]byte)
		msg := *buf

		n, _, err := s.con.ReadFrom(msg)
		if err != nil {
			// there has been an error. Either the server has been killed
			// or may be getting a transitory error due to (e.g.) the
			// interface being shutdown in which case sleep() to avoid busy wait.
			var opError *net.OpError

			ok := errors.As(err, &opError)
			if (ok) && !opError.Temporary() && !opError.Timeout() {
				return fmt.Errorf("syslog server stopped: %w", err)
			}

			time.Sleep(10 * time.Millisecond)

			s.poolBuffer.Put(buf)

			continue
		}

		if n <= 0 {
			// Ignore empty messages
			s.poolBuffer.Put(buf)

			continue
		}

		// Ignore messages not starting with '<'
		if !bytes.HasPrefix(msg, []byte("<")) {
			s.poolBuffer.Put(buf)

			continue
		}

		// Ignore trailing control characters and NULs
		//nolint:revive
		for ; (n > 0) && (msg[n-1] < 32); n-- {
		}

		// buf may contain a syslog message with a header like "<34>Oct 11 22:14:15 nginx: "
		// We need to find the first occurrence of ": " to extract the actual message.
		// Find the index after the 3th occurrence of ':' (optionally followed by a space)
		colonCount := 0
		idx := -1

		for i := range n {
			if msg[i] == ':' {
				colonCount++
				if colonCount == 3 {
					idx = i
					// Optionally, check for a space after the colon
					if i+1 < n && msg[i+1] == ' ' {
						idx = i + 1 // include the space
					}

					break
				}
			}
		}

		if idx == -1 {
			s.poolBuffer.Put(buf)

			continue // fewer than 4 colons found
		}

		// Now buf[idx+1:n] contains the message after the 3th colon (and space, if present)
		s.msgCh <- string(msg[idx+1 : n])

		s.poolBuffer.Put(buf)
	}
}

func (s *Syslog) Close(ctx context.Context) error {
	if s.con == nil {
		return errors.New("syslog server is not initialized")
	}

	err := s.con.Close()
	if err != nil {
		return fmt.Errorf("could not stop syslog server: %w", err)
	}

	if unixSocketPath, ok := strings.CutPrefix(s.listenAddr, "unix://"); ok {
		_ = os.Remove(unixSocketPath)
	}

	s.logger.InfoContext(ctx, "syslog server shutdown complete")

	return nil
}
