package metering

import (
	"errors"
	"sync"
	"time"
)

// MaxCardinalityPerTenant is the maximum number of unique keys per tenant per event type.
// Prevents attribute explosion attacks.
const MaxCardinalityPerTenant = 1000

// Errors
var (
	ErrCardinalityExceeded = errors.New("cardinality limit exceeded for tenant")
	ErrDuplicateEvent      = errors.New("duplicate event (idempotency key already seen)")
)

// bucketKey uniquely identifies an aggregation bucket.
type bucketKey struct {
	TenantID string
	Type     EventType
	Key      string
}

// WindowedAggregator aggregates metering events in memory with hourly flush windows.
type WindowedAggregator struct {
	mu sync.Mutex

	// counters holds the current window's aggregation buckets.
	// map[bucketKey]*bucket
	counters map[bucketKey]*bucket

	// seenIdempotencyKeys tracks idempotency keys for dedup within the current window.
	seenIdempotencyKeys map[string]bool

	// cardinalityCounts tracks unique keys per tenant per event type.
	// map[tenantID]map[EventType]int
	cardinalityCounts map[string]map[EventType]int

	// windowStart is when the current aggregation window began.
	windowStart time.Time
}

type bucket struct {
	count      int64
	totalValue int64
}

// NewWindowedAggregator creates a new aggregator starting now.
func NewWindowedAggregator() *WindowedAggregator {
	return &WindowedAggregator{
		counters:            make(map[bucketKey]*bucket),
		seenIdempotencyKeys: make(map[string]bool),
		cardinalityCounts:   make(map[string]map[EventType]int),
		windowStart:         time.Now(),
	}
}

// Record records a metering event into the current window.
// Returns ErrDuplicateEvent if the IdempotencyKey was already seen.
// Returns ErrCardinalityExceeded if the tenant has too many unique keys for this event type.
func (w *WindowedAggregator) Record(event Event) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if event.IdempotencyKey != "" && w.seenIdempotencyKeys[event.IdempotencyKey] {
		return ErrDuplicateEvent
	}

	key := bucketKey{
		TenantID: event.TenantID,
		Type:     event.Type,
		Key:      event.Key,
	}

	currentBucket, exists := w.counters[key]
	if !exists {
		tenantCardinality, ok := w.cardinalityCounts[event.TenantID]
		if !ok {
			tenantCardinality = make(map[EventType]int)
			w.cardinalityCounts[event.TenantID] = tenantCardinality
		}

		if tenantCardinality[event.Type] >= MaxCardinalityPerTenant {
			return ErrCardinalityExceeded
		}

		tenantCardinality[event.Type]++
		currentBucket = &bucket{}
		w.counters[key] = currentBucket
	}

	currentBucket.count++
	currentBucket.totalValue += event.Value

	if event.IdempotencyKey != "" {
		w.seenIdempotencyKeys[event.IdempotencyKey] = true
	}

	return nil
}

// Flush returns all aggregated buckets for the current window and resets.
// The returned buckets include the window start/end times.
// After flush, counters, idempotency keys, and cardinality counts are all reset.
func (w *WindowedAggregator) Flush() []AggregatedBucket {
	w.mu.Lock()
	defer w.mu.Unlock()

	windowEnd := time.Now()
	out := make([]AggregatedBucket, 0, len(w.counters))
	for key, b := range w.counters {
		out = append(out, AggregatedBucket{
			TenantID:    key.TenantID,
			Type:        key.Type,
			Key:         key.Key,
			Count:       b.count,
			TotalValue:  b.totalValue,
			WindowStart: w.windowStart,
			WindowEnd:   windowEnd,
		})
	}

	w.counters = make(map[bucketKey]*bucket)
	w.seenIdempotencyKeys = make(map[string]bool)
	w.cardinalityCounts = make(map[string]map[EventType]int)
	w.windowStart = windowEnd

	return out
}

// BucketCount returns the number of active aggregation buckets (for testing/monitoring).
func (w *WindowedAggregator) BucketCount() int {
	w.mu.Lock()
	defer w.mu.Unlock()

	return len(w.counters)
}
