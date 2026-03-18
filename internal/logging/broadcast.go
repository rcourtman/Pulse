package logging

import (
	"container/ring"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// DefaultBufferSize is the number of log lines to keep in memory.
const (
	DefaultBufferSize = 1000
	// Cap each broadcasted log line to bound per-entry memory usage.
	maxBroadcastMessageBytes = 16 * 1024
	broadcastTruncationTag   = "...[truncated]"
)

var (
	broadcaster         *LogBroadcaster
	broadcastMu         sync.Once
	broadcastWarnWriter io.Writer = os.Stderr
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
	// Copy at most a bounded prefix so oversized log lines cannot inflate memory.
	msg := string(p)
	if len(p) > maxBroadcastMessageBytes {
		keep := maxBroadcastMessageBytes - len(broadcastTruncationTag)
		if keep < 0 {
			keep = 0
		}
		msg = string(p[:keep]) + broadcastTruncationTag
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	// 1. Write to ring buffer
	b.buffer.Value = msg
	b.buffer = b.buffer.Next()

	// 2. Broadcast to subscribers
	for subscriberID, ch := range b.subscribers {
		select {
		case ch <- msg:
			// Sent successfully
		default:
			// Channel blocked, too slow consumer.
			// In a real production system we might drop the client,
			// here we just skip this message for them to avoid blocking writer.
			// Ideally we should warn or close their channel.
			fmt.Fprintf(
				broadcastWarnWriter,
				"logging: subscriber_blocked subscriber_id=%s action=drop_message\n",
				subscriberID,
			)
		}
	}

	return len(p), nil
}

// Subscribe adds a new subscriber and returns a channel of log lines
// plus a snapshot of the current history.
func (b *LogBroadcaster) Subscribe() (string, chan string, []string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	subscriberID := uuid.NewString()
	ch := make(chan string, 1000) // Large buffer to prevent blocking
	b.subscribers[subscriberID] = ch

	return subscriberID, ch, ringHistorySnapshot(b.buffer)
}

// Unsubscribe removes a subscriber.
func (b *LogBroadcaster) Unsubscribe(subscriberID string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if ch, ok := b.subscribers[subscriberID]; ok {
		close(ch)
		delete(b.subscribers, subscriberID)
	}
}

// Shutdown closes all subscriber channels and clears the subscriber set.
func (b *LogBroadcaster) Shutdown() {
	b.mu.Lock()
	defer b.mu.Unlock()

	for id, ch := range b.subscribers {
		close(ch)
		delete(b.subscribers, id)
	}
}

// GetHistory returns the current in-memory log log history.
func (b *LogBroadcaster) GetHistory() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return ringHistorySnapshot(b.buffer)
}

func ringHistorySnapshot(buffer *ring.Ring) []string {
	history := make([]string, 0, DefaultBufferSize)
	buffer.Do(func(value any) {
		msg, ok := value.(string)
		if !ok {
			return
		}
		history = append(history, msg)
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
