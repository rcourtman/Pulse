package conversion

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/license/metering"
)

// Recorder records conversion events through the metering aggregator.
type Recorder struct {
	agg   *metering.WindowedAggregator
	store *ConversionStore
}

func NewRecorder(agg *metering.WindowedAggregator, store *ConversionStore) *Recorder {
	return &Recorder{agg: agg, store: store}
}

// Record validates and records a conversion event as a metering event.
func (r *Recorder) Record(event ConversionEvent) error {
	if err := event.Validate(); err != nil {
		return err
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
			return err
		}
	}

	if r.agg != nil {
		err := r.agg.Record(metering.Event{
			Type:           metering.EventType(event.Type),
			TenantID:       orgID,
			Key:            event.Surface + ":" + event.Capability,
			Value:          1,
			Timestamp:      time.UnixMilli(event.Timestamp),
			IdempotencyKey: event.IdempotencyKey,
		})
		if errors.Is(err, metering.ErrDuplicateEvent) {
			return nil
		}
		return err
	}

	return nil
}

// Snapshot returns a non-destructive copy of the current aggregation window buckets.
func (r *Recorder) Snapshot() []metering.AggregatedBucket {
	if r == nil || r.agg == nil {
		return []metering.AggregatedBucket{}
	}
	return r.agg.Snapshot()
}
