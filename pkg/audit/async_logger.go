package audit

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
)

// AsyncLoggerConfig configures the async audit logger.
type AsyncLoggerConfig struct {
	BufferSize int
}

// AsyncLogger wraps a Logger and writes events asynchronously.
type AsyncLogger struct {
	backend Logger
	queue   chan Event
	stop    chan struct{}
	wg      sync.WaitGroup
	closed  atomic.Bool
}

// NewAsyncLogger wraps the provided logger with an async worker.
func NewAsyncLogger(backend Logger, cfg AsyncLoggerConfig) *AsyncLogger {
	if backend == nil {
		backend = NewConsoleLogger()
	}
	if cfg.BufferSize <= 0 {
		cfg.BufferSize = 4096
	}

	l := &AsyncLogger{
		backend: backend,
		queue:   make(chan Event, cfg.BufferSize),
		stop:    make(chan struct{}),
	}

	l.wg.Add(1)
	go l.run()

	return l
}

// Log enqueues the event for async processing. If the queue is full, it falls back to sync logging.
func (l *AsyncLogger) Log(event Event) error {
	if l == nil {
		return nil
	}
	if l.closed.Load() {
		return l.backend.Log(event)
	}

	select {
	case l.queue <- event:
		return nil
	default:
		// Queue full; fall back to synchronous logging to avoid dropping events.
		return l.backend.Log(event)
	}
}

// Query delegates to the backend logger.
func (l *AsyncLogger) Query(filter QueryFilter) ([]Event, error) {
	return l.backend.Query(filter)
}

// Count delegates to the backend logger.
func (l *AsyncLogger) Count(filter QueryFilter) (int, error) {
	return l.backend.Count(filter)
}

// GetWebhookURLs delegates to the backend logger.
func (l *AsyncLogger) GetWebhookURLs() []string {
	return l.backend.GetWebhookURLs()
}

// UpdateWebhookURLs delegates to the backend logger.
func (l *AsyncLogger) UpdateWebhookURLs(urls []string) error {
	return l.backend.UpdateWebhookURLs(urls)
}

// Close drains queued events, stops the worker, and closes the backend logger.
func (l *AsyncLogger) Close() error {
	if l == nil {
		return nil
	}
	if l.closed.Swap(true) {
		return nil
	}

	close(l.stop)
	l.wg.Wait()
	return l.backend.Close()
}

func (l *AsyncLogger) run() {
	defer l.wg.Done()
	for {
		select {
		case event := <-l.queue:
			l.logEvent(event)
		case <-l.stop:
			l.drain()
			return
		}
	}
}

func (l *AsyncLogger) drain() {
	for {
		select {
		case event := <-l.queue:
			l.logEvent(event)
		default:
			return
		}
	}
}

func (l *AsyncLogger) logEvent(event Event) {
	start := time.Now()
	if err := l.backend.Log(event); err != nil {
		log.Error().Err(err).Str("event", event.EventType).Msg("Failed to log audit event")
		return
	}
	if time.Since(start) > 250*time.Millisecond {
		log.Warn().
			Str("event", event.EventType).
			Dur("duration", time.Since(start)).
			Msg("Audit log write slow")
	}
}

// EnableAsyncLogging wraps the current global logger with an AsyncLogger.
// It is safe to call multiple times.
func EnableAsyncLogging(cfg AsyncLoggerConfig) {
	loggerMu.Lock()
	defer loggerMu.Unlock()

	if globalLogger == nil {
		globalLogger = NewConsoleLogger()
	}
	if _, ok := globalLogger.(*AsyncLogger); ok {
		return
	}
	globalLogger = NewAsyncLogger(globalLogger, cfg)
}
