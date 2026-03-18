// Package audit provides audit logging functionality for Pulse.
//
// This package defines the AuditLogger interface which can be implemented
// by different backends. The OSS version uses ConsoleAuditLogger (logs to zerolog),
// while the enterprise version can provide a more sophisticated implementation
// with persistent storage, signing, and webhook delivery.
//
// This package is in pkg/ so it can be imported by external modules (pulse-enterprise).
package audit

import (
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// Event represents a single audit log entry.
type Event struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	EventType string    `json:"event"` // "login", "logout", "config_change", etc.
	User      string    `json:"user,omitempty"`
	IP        string    `json:"ip"`
	Path      string    `json:"path,omitempty"`
	Success   bool      `json:"success"`
	Details   string    `json:"details,omitempty"`
	Signature string    `json:"signature,omitempty"` // Empty for OSS, HMAC for enterprise
}

// QueryFilter defines filters for querying audit events.
type QueryFilter struct {
	ID        string
	StartTime *time.Time
	EndTime   *time.Time
	EventType string
	User      string
	Success   *bool
	Limit     int
	Offset    int
}

// Logger defines the interface for audit logging backends.
// The OSS version uses ConsoleLogger which outputs to zerolog.
// Enterprise implementations can provide persistent storage with signing.
type Logger interface {
	// Log records an audit event
	Log(event Event) error

	// Query retrieves audit events matching the filter (optional, may return empty for console logger)
	Query(filter QueryFilter) ([]Event, error)

	// Count returns the number of audit events matching the filter
	Count(filter QueryFilter) (int, error)

	// Webhook Management (Optional, may return empty/not implemented for console logger)
	GetWebhookURLs() []string
	UpdateWebhookURLs(urls []string) error

	// Close releases any resources held by the logger
	Close() error
}

// Global logger instance with thread-safe access
var (
	globalLogger Logger
	loggerMu     sync.RWMutex
	loggerOnce   sync.Once
)

// SetLogger sets the global audit logger.
// This should be called during application initialization.
// If called multiple times, subsequent calls replace the previous logger.
func SetLogger(l Logger) {
	loggerMu.Lock()
	defer loggerMu.Unlock()
	globalLogger = l
}

// GetLogger returns the current global audit logger.
// If no logger has been set, it returns a ConsoleLogger.
func GetLogger() Logger {
	loggerMu.RLock()
	l := globalLogger
	loggerMu.RUnlock()

	if l != nil {
		return l
	}

	// Initialize default console logger on first access
	loggerOnce.Do(func() {
		loggerMu.Lock()
		defer loggerMu.Unlock()
		if globalLogger == nil {
			globalLogger = NewConsoleLogger()
		}
	})

	loggerMu.RLock()
	defer loggerMu.RUnlock()
	return globalLogger
}

// Close closes the global audit logger if it implements Close.
func Close() error {
	loggerMu.RLock()
	l := globalLogger
	loggerMu.RUnlock()
	if l == nil {
		return nil
	}
	if closer, ok := l.(interface{ Close() error }); ok {
		return closer.Close()
	}
	return nil
}

// Log is a convenience function that logs an event using the global logger.
func Log(eventType, user, ip, path string, success bool, details string) {
	event := Event{
		ID:        uuid.NewString(),
		Timestamp: time.Now(),
		EventType: eventType,
		User:      user,
		IP:        ip,
		Path:      path,
		Success:   success,
		Details:   details,
	}

	if err := GetLogger().Log(event); err != nil {
		log.Error().Err(err).Str("event", eventType).Msg("Failed to log audit event")
	}
}

// ConsoleLogger implements Logger by writing to zerolog.
// This is the default implementation used by the OSS version.
type ConsoleLogger struct{}

// NewConsoleLogger creates a new console-based audit logger.
func NewConsoleLogger() *ConsoleLogger {
	return &ConsoleLogger{}
}

// Log writes an audit event to zerolog.
func (c *ConsoleLogger) Log(event Event) error {
	logEvent := log.With().
		Str("audit_id", event.ID).
		Str("event", event.EventType).
		Str("user", event.User).
		Str("ip", event.IP).
		Str("path", event.Path).
		Time("timestamp", event.Timestamp).
		Str("details", event.Details).
		Logger()

	if event.Success {
		logEvent.Info().Msg("Audit event")
	} else {
		logEvent.Warn().Msg("Audit event - FAILED")
	}

	return nil
}

// Query returns an empty slice for the console logger.
// Console logs are not queryable - use enterprise version for persistent storage.
func (c *ConsoleLogger) Query(filter QueryFilter) ([]Event, error) {
	return []Event{}, nil
}

// Count returns zero for the console logger.
func (c *ConsoleLogger) Count(filter QueryFilter) (int, error) {
	return 0, nil
}

// GetWebhookURLs returns an empty slice for the console logger.
func (c *ConsoleLogger) GetWebhookURLs() []string {
	return []string{}
}

// UpdateWebhookURLs returns an error for the console logger.
func (c *ConsoleLogger) UpdateWebhookURLs(urls []string) error {
	return nil // Or return an error saying it's not supported
}

// Close is a no-op for the console logger.
func (c *ConsoleLogger) Close() error {
	return nil
}
