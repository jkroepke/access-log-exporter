package syslog

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

type packetReader interface {
	net.PacketConn
	io.Reader
}

type Syslog struct {
	logger     *slog.Logger
	con        packetReader
	msgCh      chan<- Message
	done       chan struct{}
	bufferPool *sync.Pool
	listenAddr string
}

func New(ctx context.Context, logger *slog.Logger, listenAddr string, msgCh chan<- Message) (Syslog, error) {
	syslogServer := Syslog{
		listenAddr: listenAddr,
		logger:     logger.With(slog.String("component", "syslog")),
		msgCh:      msgCh,
		done:       make(chan struct{}),
		bufferPool: &sync.Pool{
			New: func() any {
				return new(packetBuffer)
			},
		},
	}

	uri, err := url.Parse(listenAddr)
	if err != nil {
		return Syslog{}, fmt.Errorf("could not parse syslog listen address '%s': %w", listenAddr, err)
	}

	var (
		listenConf net.ListenConfig
		listener   net.PacketConn
	)

	switch uri.Scheme {
	case "udp":
		listener, err = listenConf.ListenPacket(ctx, "udp", uri.Host)
	case "unix":
		listener, err = listenConf.ListenPacket(ctx, "unixgram", uri.Host+uri.Path)
	default:
		err = errors.New("syslog listen address must be start with udp:// or unix://")
	}

	if err != nil {
		return Syslog{}, fmt.Errorf("could not listen syslog server on '%s': %w", listenAddr, err)
	}

	conn, ok := listener.(packetReader)
	if !ok {
		_ = listener.Close()

		return Syslog{}, fmt.Errorf("syslog listener for '%s' does not support address-less reads", listenAddr)
	}

	syslogServer.con = conn

	return syslogServer, nil
}

//nolint:gocognit,cyclop
func (s *Syslog) Start() error {
	con := s.con
	msgCh := s.msgCh
	done := s.done

	for {
		buffer, _ := s.bufferPool.Get().(*packetBuffer)
		msg := buffer[:]

		// The sender address is unused, so prefer Read over ReadFrom to avoid address allocation.
		n, err := con.Read(msg)
		if err != nil {
			s.bufferPool.Put(buffer)

			select {
			case <-done:
				return nil
			default:
			}

			// there has been an error. Either the server has been killed
			// or may be getting a transitory error due to (e.g.) the
			// interface being shutdown in which case sleep() to avoid busy wait.
			var opError *net.OpError

			ok := errors.As(err, &opError)
			if ok && !opError.Temporary() && !opError.Timeout() {
				return fmt.Errorf("syslog server stopped: %w", err)
			}

			time.Sleep(10 * time.Millisecond)

			continue
		}

		if n <= 0 {
			// Ignore empty messages
			s.bufferPool.Put(buffer)

			continue
		}

		// Ignore messages not starting with '<'
		if msg[0] != '<' {
			s.bufferPool.Put(buffer)

			continue
		}

		// Ignore trailing control characters and NULs
		//nolint:revive
		for ; (n > 0) && (msg[n-1] < 32); n-- {
		}

		// msg may contain a syslog message with a header like "<34>Oct 11 22:14:15 nginx: "
		// We need to find the first occurrence of ": " to extract the actual message.
		// Find the index after the third occurrence of ':' (optionally followed by a space).
		colonCount := 0
		messageStart := -1

		for i, b := range msg[:n] {
			if b == ':' {
				colonCount++
				if colonCount == 3 {
					messageStart = i + 1
					// Optionally, check for a space after the colon
					if messageStart < n && msg[messageStart] == ' ' {
						messageStart++
					}

					break
				}
			}
		}

		if messageStart == -1 {
			s.bufferPool.Put(buffer)

			continue // fewer than 4 colons found
		}

		// Now msg[messageStart:n] contains the message after the third colon (and space, if present).
		message := newMessage(buffer, messageStart, n, s.bufferPool)

		select {
		case msgCh <- message:
		case <-done:
			message.Release()

			return nil
		}
	}
}

func (s *Syslog) Close(ctx context.Context) error {
	if s.con == nil {
		return errors.New("syslog server is not initialized")
	}

	close(s.done)

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
