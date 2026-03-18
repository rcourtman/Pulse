package licensing

import (
	"errors"

	"github.com/rcourtman/pulse-go-rewrite/pkg/licensing/metering"
)

// NewRecorderFromWindowedAggregator creates a conversion recorder backed by
// metering.WindowedAggregator and optional durable conversion store.
func NewRecorderFromWindowedAggregator(agg *metering.WindowedAggregator, store *ConversionStore) *Recorder {
	var wrapped ConversionAggregator
	if agg != nil {
		wrapped = &windowedAggregatorAdapter{agg: agg}
	}

	return NewRecorder(
		wrapped,
		store,
		func(err error) bool { return errors.Is(err, metering.ErrDuplicateEvent) },
	)
}

type windowedAggregatorAdapter struct {
	agg *metering.WindowedAggregator
}

func (a *windowedAggregatorAdapter) Record(event MeteringEvent) error {
	if a == nil || a.agg == nil {
		return nil
	}
	return a.agg.Record(metering.Event{
		Type:           metering.EventType(event.Type),
		TenantID:       event.TenantID,
		Key:            event.Key,
		Value:          event.Value,
		Timestamp:      event.Timestamp,
		IdempotencyKey: event.IdempotencyKey,
	})
}

func (a *windowedAggregatorAdapter) Snapshot() []MeteringBucket {
	if a == nil || a.agg == nil {
		return []MeteringBucket{}
	}

	buckets := a.agg.Snapshot()
	out := make([]MeteringBucket, 0, len(buckets))
	for _, bucket := range buckets {
		out = append(out, MeteringBucket{
			TenantID:    bucket.TenantID,
			Type:        string(bucket.Type),
			Key:         bucket.Key,
			Count:       bucket.Count,
			TotalValue:  bucket.TotalValue,
			WindowStart: bucket.WindowStart,
			WindowEnd:   bucket.WindowEnd,
		})
	}
	return out
}
