package syslog

const maxDatagramSize = 64 * 1024

type Message struct {
	Line string
}

func newMessage(buffer []byte, start, end int) Message {
	return Message{
		Line: string(buffer[start:end]),
	}
}

// Release is kept for callers that explicitly release messages.
// Message data no longer retains the reusable receive buffer.
func (Message) Release() {}
