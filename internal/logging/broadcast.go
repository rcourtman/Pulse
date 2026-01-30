package logging

import (
	"container/ring"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// DefaultBufferSize is the number of log lines to keep in memory.
const (
	DefaultBufferSize = 1000
)

var (
	broadcaster *LogBroadcaster
	broadcastMu sync.Once
)

// LogBroadcaster captures log writes, buffers them, and broadcasts them to subscribers.
type LogBroadcaster struct {
	mu          sync.RWMutex
	buffer      *ring.Ring
	subscribers map[string]chan string
}

// GetBroadcaster returns the singleton broadcaster instance.
func GetBroadcaster() *LogBroadcaster {
	broadcastMu.Do(func() {
		broadcaster = &LogBroadcaster{
			buffer:      ring.New(DefaultBufferSize),
			subscribers: make(map[string]chan string),
		}
	})
	return broadcaster
}

// Write implements io.Writer. It writes to the internal buffer and notifies subscribers.
func (b *LogBroadcaster) Write(p []byte) (n int, err error) {
	// Copy slice to avoid race conditions if p is reused
	msg := string(p)

	b.mu.Lock()
	defer b.mu.Unlock()

	// 1. Write to ring buffer
	b.buffer.Value = msg
	b.buffer = b.buffer.Next()

	// 2. Broadcast to subscribers
	for id, ch := range b.subscribers {
		select {
		case ch <- msg:
			// Sent successfully
		default:
			// Channel blocked, too slow consumer.
			// In a real production system we might drop the client,
			// here we just skip this message for them to avoid blocking writer.
			// Ideally we should warn or close their channel.
			fmt.Printf("logging: subscriber %s blocked, dropping message\n", id)
		}
	}

	return len(p), nil
}

// Subscribe adds a new subscriber and returns a channel of log lines
// plus a snapshot of the current history.
func (b *LogBroadcaster) Subscribe() (string, chan string, []string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	id := uuid.NewString()
	ch := make(chan string, 1000) // Large buffer to prevent blocking
	b.subscribers[id] = ch

	// Generate history snapshot
	history := make([]string, 0, DefaultBufferSize)
	b.buffer.Do(func(p interface{}) {
		if p != nil {
			history = append(history, p.(string))
		}
	})

	return id, ch, history
}

// Unsubscribe removes a subscriber.
func (b *LogBroadcaster) Unsubscribe(id string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if ch, ok := b.subscribers[id]; ok {
		close(ch)
		delete(b.subscribers, id)
	}
}

// GetHistory returns the current in-memory log log history.
func (b *LogBroadcaster) GetHistory() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	history := make([]string, 0, DefaultBufferSize)
	b.buffer.Do(func(p interface{}) {
		if p != nil {
			history = append(history, p.(string))
		}
	})
	return history
}

// SetGlobalLevel updates the global zerolog level at runtime.
func SetGlobalLevel(level string) {
	mu.Lock()
	defer mu.Unlock()
	zerolog.SetGlobalLevel(parseLevel(level))
}

// GetGlobalLevel returns the current global level string.
func GetGlobalLevel() string {
	return zerolog.GlobalLevel().String()
}
