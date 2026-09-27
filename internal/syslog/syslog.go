package syslog

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/url"
	"os"
	"strings"
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
	listenAddr string
}

func New(ctx context.Context, logger *slog.Logger, listenAddr string, msgCh chan<- Message) (Syslog, error) {
	syslogServer := Syslog{
		listenAddr: listenAddr,
		logger:     logger.With(slog.String("component", "syslog")),
		msgCh:      msgCh,
		done:       make(chan struct{}),
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

//nolint:cyclop
func (s *Syslog) Start() error {
	con := s.con
	msgCh := s.msgCh
	done := s.done

	buffer := make([]byte, maxDatagramSize)

	for {
		// The sender address is unused, so prefer Read over ReadFrom to avoid address allocation.
		n, err := con.Read(buffer)
		if err != nil {
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
			continue
		}

		receivedAtUnixNano := time.Now().UnixNano()

		// Ignore messages not starting with '<'
		if buffer[0] != '<' {
			continue
		}

		// Ignore trailing control characters and NULs
		//nolint:revive
		for ; (n > 0) && (buffer[n-1] < 32); n-- {
		}

		messageStart := syslogMessageStart(buffer[:n])
		if messageStart == -1 {
			continue
		}

		message := newMessage(buffer, messageStart, n, receivedAtUnixNano)

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


const rfc3164TimestampLength = len("Jan  1 00:00:00")

func syslogMessageStart(message []byte) int {
	priorityEnd := bytes.IndexByte(message, '>')
	if priorityEnd < 2 || priorityEnd+1 >= len(message) {
		return -1
	}

	header := message[priorityEnd+1:]
	if isRFC5424Header(header) {
		start := rfc5424MessageStart(header)
		if start == -1 {
			return -1
		}

		return priorityEnd + 1 + start
	}

	if len(header) <= rfc3164TimestampLength {
		return -1
	}

	tagEnd := bytes.IndexByte(header[rfc3164TimestampLength:], ':')
	if tagEnd == -1 {
		return -1
	}

	start := rfc3164TimestampLength + tagEnd + 1
	if start < len(header) && header[start] == ' ' {
		start++
	}

	return priorityEnd + 1 + start
}

func isRFC5424Header(header []byte) bool {
	versionEnd := bytes.IndexByte(header, ' ')
	if versionEnd <= 0 {
		return false
	}

	for _, char := range header[:versionEnd] {
		if char < '0' || char > '9' {
			return false
		}
	}

	return header[0] != '0'
}

func rfc5424MessageStart(header []byte) int {
	position := rfc5424StructuredDataStart(header)
	if position == -1 {
		return -1
	}

	switch header[position] {
	case '-':
		return skipMessageSeparator(header, position+1)
	case '[':
		position = rfc5424StructuredDataEnd(header, position)
		if position == -1 {
			return -1
		}

		return skipMessageSeparator(header, position)
	default:
		return -1
	}
}

func rfc5424StructuredDataStart(header []byte) int {
	position := 0

	// VERSION, TIMESTAMP, HOSTNAME, APP-NAME, PROCID and MSGID precede STRUCTURED-DATA.
	for range 6 {
		fieldEnd := bytes.IndexByte(header[position:], ' ')
		if fieldEnd == -1 {
			return -1
		}

		position += fieldEnd + 1
	}

	if position >= len(header) {
		return -1
	}

	return position
}

func rfc5424StructuredDataEnd(header []byte, position int) int {
	inQuotes := false
	escaped := false

	for position < len(header) {
		char := header[position]

		if escaped {
			escaped = false
			position++

			continue
		}

		if char == '\\' && inQuotes {
			escaped = true
			position++

			continue
		}

		if char == '"' {
			inQuotes = !inQuotes
			position++

			continue
		}

		if char == ']' && !inQuotes {
			position++
			if position < len(header) && header[position] == '[' {
				continue
			}

			return position
		}

		position++
	}

	return -1
}

func skipMessageSeparator(header []byte, position int) int {
	if position == len(header) {
		return position
	}

	if header[position] != ' ' {
		return -1
	}

	return position + 1
}
