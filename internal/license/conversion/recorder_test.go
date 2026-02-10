package conversion

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/license/metering"
)

func TestRecorderRecordValidEvent(t *testing.T) {
	agg := metering.NewWindowedAggregator()
	recorder := NewRecorder(agg, nil)

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

	buckets := agg.Flush()
	if len(buckets) != 1 {
		t.Fatalf("len(Flush()) = %d, want 1", len(buckets))
	}

	got := buckets[0]
	if got.Type != metering.EventType(EventPaywallViewed) {
		t.Fatalf("bucket type = %q, want %q", got.Type, EventPaywallViewed)
	}
	if got.TenantID != "default" {
		t.Fatalf("bucket tenant = %q, want default", got.TenantID)
	}
	if got.Key != "history_chart:long_term_metrics" {
		t.Fatalf("bucket key = %q, want history_chart:long_term_metrics", got.Key)
	}
	if got.Count != 1 {
		t.Fatalf("bucket count = %d, want 1", got.Count)
	}
}

func TestRecorderRecordIdempotency(t *testing.T) {
	agg := metering.NewWindowedAggregator()
	recorder := NewRecorder(agg, nil)

	event := ConversionEvent{
		Type:           EventUpgradeClicked,
		OrgID:          "default",
		Capability:     "ai_autofix",
		Surface:        "ai_intelligence",
		Timestamp:      time.Now().UnixMilli(),
		IdempotencyKey: "upgrade_clicked:ai_intelligence:ai_autofix:42",
	}

	if err := recorder.Record(event); err != nil {
		t.Fatalf("first Record() error = %v, want nil", err)
	}
	if err := recorder.Record(event); err != nil {
		t.Fatalf("second Record() error = %v, want nil (dedup accepted)", err)
	}

	buckets := agg.Flush()
	if len(buckets) != 1 {
		t.Fatalf("len(Flush()) = %d, want 1", len(buckets))
	}
	if buckets[0].Count != 1 {
		t.Fatalf("bucket count = %d, want 1", buckets[0].Count)
	}
}

func TestRecorderRecordValidationRejection(t *testing.T) {
	agg := metering.NewWindowedAggregator()
	recorder := NewRecorder(agg, nil)

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

func TestRecorderRecordNilAggregator(t *testing.T) {
	recorder := NewRecorder(nil, nil)

	err := recorder.Record(ConversionEvent{
		Type:           EventTrialStarted,
		Surface:        "license_panel",
		Timestamp:      time.Now().UnixMilli(),
		IdempotencyKey: "trial_started:license_panel::1",
	})
	if err != nil {
		t.Fatalf("Record() error = %v, want nil", err)
	}
}
