package conversion

import (
	"errors"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/license/metering"
)

// Recorder records conversion events through the metering aggregator.
type Recorder struct {
	agg *metering.WindowedAggregator
}

func NewRecorder(agg *metering.WindowedAggregator) *Recorder {
	return &Recorder{agg: agg}
}

// Record validates and records a conversion event as a metering event.
func (r *Recorder) Record(event ConversionEvent) error {
	if err := event.Validate(); err != nil {
		return err
	}
	if r == nil || r.agg == nil {
		return nil
	}

	err := r.agg.Record(metering.Event{
		Type:           metering.EventType(event.Type),
		TenantID:       "default",
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

// Snapshot returns a non-destructive copy of the current aggregation window buckets.
func (r *Recorder) Snapshot() []metering.AggregatedBucket {
	if r == nil || r.agg == nil {
		return []metering.AggregatedBucket{}
	}
	return r.agg.Snapshot()
}
