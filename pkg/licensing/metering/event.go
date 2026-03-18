package metering

import "time"

// EventType represents a metering event category.
type EventType string

const (
	EventAgentSeen  EventType = "agent_seen"
	EventRelayBytes EventType = "relay_bytes"
)

// Event represents a single metering event.
type Event struct {
	// Type is the event category.
	Type EventType

	// TenantID identifies the tenant that generated this event.
	TenantID string

	// Key is the specific entity being metered (e.g., agent ID, org ID).
	Key string

	// Value is the numeric value for the event (e.g., bytes transferred).
	// For counter events (agent_seen), this is typically 1.
	Value int64

	// Timestamp is when the event occurred.
	Timestamp time.Time

	// IdempotencyKey prevents duplicate event recording.
	// Events with the same IdempotencyKey within a window are deduplicated.
	IdempotencyKey string
}

// AggregatedBucket represents the aggregated metering data for a flush window.
type AggregatedBucket struct {
	// TenantID is the tenant this bucket belongs to.
	TenantID string

	// Type is the event type.
	Type EventType

	// Key is the specific entity.
	Key string

	// Count is the number of events in this bucket.
	Count int64

	// TotalValue is the sum of all event values in this bucket.
	TotalValue int64

	// WindowStart is the start of the aggregation window.
	WindowStart time.Time

	// WindowEnd is the end of the aggregation window.
	WindowEnd time.Time
}
