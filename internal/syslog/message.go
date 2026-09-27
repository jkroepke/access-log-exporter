package syslog

const maxDatagramSize = 64 * 1024

type Message struct {
	Line               string
	ReceivedAtUnixNano int64
}

func newMessage(buffer []byte, start, end int, receivedAtUnixNano int64) Message {
	return Message{
		Line:               string(buffer[start:end]),
		ReceivedAtUnixNano: receivedAtUnixNano,
	}
}

// Release is kept for callers that explicitly release messages.
// Message data no longer retains the reusable receive buffer.
func (Message) Release() {}
