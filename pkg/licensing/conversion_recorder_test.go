package licensing

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestRecorderRecordValidEvent(t *testing.T) {
	agg := &fakeConversionAggregator{}
	recorder := NewRecorder(agg, nil, nil)

	err := recorder.Record(ConversionEvent{
		Type:           EventPaywallViewed,
		OrgID:          "default",
		Capability:     "long_term_metrics",
		Surface:        "history_chart",
		Timestamp:      time.Now().UnixMilli(),
		IdempotencyKey: "paywall:history_chart:long_term_metrics:1",
	})
	if err != nil {
		t.Fatalf("Record() error = %v, want nil", err)
	}

	if len(agg.recorded) != 1 {
		t.Fatalf("len(recorded) = %d, want 1", len(agg.recorded))
	}
	got := agg.recorded[0]
	if got.Type != EventPaywallViewed {
		t.Fatalf("recorded type = %q, want %q", got.Type, EventPaywallViewed)
	}
	if got.TenantID != "default" {
		t.Fatalf("recorded tenant = %q, want default", got.TenantID)
	}
	if got.Key != "history_chart:long_term_metrics" {
		t.Fatalf("recorded key = %q, want history_chart:long_term_metrics", got.Key)
	}
	if got.Value != 1 {
		t.Fatalf("recorded value = %d, want 1", got.Value)
	}
}

func TestRecorderRecordIdempotencyPredicate(t *testing.T) {
	duplicateErr := errors.New("duplicate")
	agg := &fakeConversionAggregator{recordErr: duplicateErr}
	recorder := NewRecorder(agg, nil, func(err error) bool { return errors.Is(err, duplicateErr) })

	err := recorder.Record(ConversionEvent{
		Type:           EventUpgradeClicked,
		OrgID:          "default",
		Capability:     "ai_autofix",
		Surface:        "ai_intelligence",
		Timestamp:      time.Now().UnixMilli(),
		IdempotencyKey: "upgrade_clicked:ai_intelligence:ai_autofix:42",
	})
	if err != nil {
		t.Fatalf("Record() error = %v, want nil for duplicate event", err)
	}
}

func TestRecorderRecordValidationRejection(t *testing.T) {
	agg := &fakeConversionAggregator{}
	recorder := NewRecorder(agg, nil, nil)

	err := recorder.Record(ConversionEvent{
		Type:      EventPaywallViewed,
		OrgID:     "default",
		Surface:   "settings_tab",
		Timestamp: time.Now().UnixMilli(),
		// missing capability for paywall_viewed
		IdempotencyKey: "paywall:settings_tab::1",
	})
	if err == nil {
		t.Fatal("Record() error = nil, want validation error")
	}
}

func TestRecorderRecordUnexpectedAggregatorError(t *testing.T) {
	agg := &fakeConversionAggregator{recordErr: errors.New("write failed")}
	recorder := NewRecorder(agg, nil, nil)

	err := recorder.Record(ConversionEvent{
		Type:           EventTrialStarted,
		OrgID:          "default",
		Surface:        "license_panel",
		Timestamp:      time.Now().UnixMilli(),
		IdempotencyKey: "trial_started:license_panel::1",
	})
	if err == nil {
		t.Fatal("Record() error = nil, want wrapped error")
	}
	if !strings.Contains(err.Error(), "record metering conversion event") {
		t.Fatalf("Record() error = %v, want wrapped metering error", err)
	}
}

func TestRecorderSnapshotNilAggregator(t *testing.T) {
	recorder := NewRecorder(nil, nil, nil)
	buckets := recorder.Snapshot()
	if len(buckets) != 0 {
		t.Fatalf("len(Snapshot()) = %d, want 0", len(buckets))
	}
}

type fakeConversionAggregator struct {
	recorded  []MeteringEvent
	snapshot  []MeteringBucket
	recordErr error
}

func (f *fakeConversionAggregator) Record(event MeteringEvent) error {
	f.recorded = append(f.recorded, event)
	return f.recordErr
}

func (f *fakeConversionAggregator) Snapshot() []MeteringBucket {
	if f.snapshot == nil {
		return []MeteringBucket{}
	}
	out := make([]MeteringBucket, len(f.snapshot))
	copy(out, f.snapshot)
	return out
}
