package metrics

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

func TestRecordAlertFired(t *testing.T) {
	alert := &alerts.Alert{
		ID:        "test-alert-1",
		Level:     alerts.AlertLevelWarning,
		Type:      "container_cpu",
		StartTime: time.Now(),
	}

	// Should not panic
	RecordAlertFired(alert)
}

func TestRecordAlertResolved(t *testing.T) {
	now := time.Now()
	alert := &alerts.Alert{
		ID:        "test-alert-2",
		Level:     alerts.AlertLevelWarning,
		Type:      "container_memory",
		StartTime: now.Add(-5 * time.Minute),
		LastSeen:  now,
	}

	// Should not panic
	RecordAlertResolved(alert)
}

func TestRecordAlertAcknowledged(t *testing.T) {
	// Should not panic
	RecordAlertAcknowledged()
}

func TestRecordAlertSuppressed(t *testing.T) {
	// Should not panic with various reasons
	RecordAlertSuppressed("quiet_hours")
	RecordAlertSuppressed("rate_limit")
	RecordAlertSuppressed("duplicate")
}

func TestMetricVectors_NotNil(t *testing.T) {
	// Verify that metric vectors are properly initialized
	if AlertsActive == nil {
		t.Error("AlertsActive should not be nil")
	}
	if AlertsFiredTotal == nil {
		t.Error("AlertsFiredTotal should not be nil")
	}
	if AlertsResolvedTotal == nil {
		t.Error("AlertsResolvedTotal should not be nil")
	}
	if AlertsAcknowledgedTotal == nil {
		t.Error("AlertsAcknowledgedTotal should not be nil")
	}
	if AlertDurationSeconds == nil {
		t.Error("AlertDurationSeconds should not be nil")
	}
	if AlertsSuppressedTotal == nil {
		t.Error("AlertsSuppressedTotal should not be nil")
	}
	if AlertsRateLimitedTotal == nil {
		t.Error("AlertsRateLimitedTotal should not be nil")
	}
}
