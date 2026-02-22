package licensing

import (
	"fmt"
	"strings"
	"time"
)

// MeteringEvent is the canonical event shape expected by conversion recorders.
type MeteringEvent struct {
	Type           string
	TenantID       string
	Key            string
	Value          int64
	Timestamp      time.Time
	IdempotencyKey string
}

// MeteringBucket is the canonical snapshot bucket shape for conversion stats.
type MeteringBucket struct {
	TenantID    string
	Type        string
	Key         string
	Count       int64
	TotalValue  int64
	WindowStart time.Time
	WindowEnd   time.Time
}

// ConversionAggregator is the runtime aggregation dependency used by Recorder.
type ConversionAggregator interface {
	Record(event MeteringEvent) error
	Snapshot() []MeteringBucket
}

// Recorder records conversion events through an aggregator and optional durable store.
type Recorder struct {
	agg              ConversionAggregator
	store            *ConversionStore
	isDuplicateError func(error) bool
}

// NewRecorder creates a recorder backed by the given aggregator and store.
func NewRecorder(agg ConversionAggregator, store *ConversionStore, isDuplicateError func(error) bool) *Recorder {
	return &Recorder{
		agg:              agg,
		store:            store,
		isDuplicateError: isDuplicateError,
	}
}

// Record validates and records a conversion event as a metering event.
func (r *Recorder) Record(event ConversionEvent) error {
	if err := event.Validate(); err != nil {
		return fmt.Errorf("validate conversion event: %w", err)
	}
	if r == nil {
		return nil
	}
	if r.agg == nil && r.store == nil {
		return nil
	}

	orgID := strings.TrimSpace(event.OrgID)
	if orgID == "" {
		return fmt.Errorf("org_id is required")
	}

	if r.store != nil {
		if err := r.store.Record(StoredConversionEvent{
			OrgID:          orgID,
			EventType:      event.Type,
			Surface:        event.Surface,
			Capability:     event.Capability,
			IdempotencyKey: event.IdempotencyKey,
			CreatedAt:      time.UnixMilli(event.Timestamp).UTC(),
		}); err != nil {
			return fmt.Errorf("persist conversion event: %w", err)
		}
	}

	if r.agg != nil {
		err := r.agg.Record(MeteringEvent{
			Type:           event.Type,
			TenantID:       orgID,
			Key:            event.Surface + ":" + event.Capability,
			Value:          1,
			Timestamp:      time.UnixMilli(event.Timestamp),
			IdempotencyKey: event.IdempotencyKey,
		})
		if err != nil && r.isDuplicateError != nil && r.isDuplicateError(err) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("record metering conversion event: %w", err)
		}
	}

	return nil
}

// Snapshot returns a non-destructive copy of the current aggregation window buckets.
func (r *Recorder) Snapshot() []MeteringBucket {
	if r == nil || r.agg == nil {
		return []MeteringBucket{}
	}
	return r.agg.Snapshot()
}
