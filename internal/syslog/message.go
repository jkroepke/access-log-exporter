package syslog

import "sync"

const bufferSize = 4096

type packetBuffer [bufferSize]byte

type Message struct {
	buffer *packetBuffer
	pool   *sync.Pool
	Line   string
}

func newMessage(buffer *packetBuffer, start, end int, pool *sync.Pool) Message {
	return Message{
		Line:   string(buffer[start:end]),
		buffer: buffer,
		pool:   pool,
	}
}

func (m Message) Release() {
	if m.pool != nil {
		m.pool.Put(m.buffer)
	}
}
