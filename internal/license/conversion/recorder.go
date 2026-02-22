package conversion

import (
	"errors"

	"github.com/rcourtman/pulse-go-rewrite/internal/license/metering"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

// Recorder records conversion events through the metering aggregator.
type Recorder struct {
	inner *pkglicensing.Recorder
}

func NewRecorder(agg *metering.WindowedAggregator, store *ConversionStore) *Recorder {
	var wrapped pkglicensing.ConversionAggregator
	if agg != nil {
		wrapped = &meteringAggregatorAdapter{agg: agg}
	}

	return &Recorder{
		inner: pkglicensing.NewRecorder(
			wrapped,
			store,
			func(err error) bool { return errors.Is(err, metering.ErrDuplicateEvent) },
		),
	}
}

// Record validates and records a conversion event as a metering event.
func (r *Recorder) Record(event ConversionEvent) error {
	if r == nil {
		var inner *pkglicensing.Recorder
		return inner.Record(event)
	}
	if r.inner == nil {
		return nil
	}
	return r.inner.Record(event)
}

// Snapshot returns a non-destructive copy of the current aggregation window buckets.
func (r *Recorder) Snapshot() []metering.AggregatedBucket {
	if r == nil || r.inner == nil {
		return []metering.AggregatedBucket{}
	}

	snapshot := r.inner.Snapshot()
	out := make([]metering.AggregatedBucket, 0, len(snapshot))
	for _, bucket := range snapshot {
		out = append(out, metering.AggregatedBucket{
			TenantID:    bucket.TenantID,
			Type:        metering.EventType(bucket.Type),
			Key:         bucket.Key,
			Count:       bucket.Count,
			TotalValue:  bucket.TotalValue,
			WindowStart: bucket.WindowStart,
			WindowEnd:   bucket.WindowEnd,
		})
	}
	return out
}

type meteringAggregatorAdapter struct {
	agg *metering.WindowedAggregator
}

func (a *meteringAggregatorAdapter) Record(event pkglicensing.MeteringEvent) error {
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

func (a *meteringAggregatorAdapter) Snapshot() []pkglicensing.MeteringBucket {
	if a == nil || a.agg == nil {
		return []pkglicensing.MeteringBucket{}
	}

	buckets := a.agg.Snapshot()
	out := make([]pkglicensing.MeteringBucket, 0, len(buckets))
	for _, bucket := range buckets {
		out = append(out, pkglicensing.MeteringBucket{
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
