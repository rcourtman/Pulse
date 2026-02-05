package metrics

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
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

func TestAlertMetricsIncrements(t *testing.T) {
	alert := &alerts.Alert{
		ID:        "metrics-test",
		Level:     alerts.AlertLevelCritical,
		Type:      "unit_test_alert",
		StartTime: time.Now().Add(-time.Minute),
		LastSeen:  time.Now(),
	}

	fired := AlertsFiredTotal.WithLabelValues(string(alert.Level), alert.Type)
	active := AlertsActive.WithLabelValues(string(alert.Level), alert.Type)
	resolved := AlertsResolvedTotal.WithLabelValues(alert.Type)

	firedBefore := testutil.ToFloat64(fired)
	activeBefore := testutil.ToFloat64(active)
	resolvedBefore := testutil.ToFloat64(resolved)

	RecordAlertFired(alert)

	if testutil.ToFloat64(fired) != firedBefore+1 {
		t.Fatalf("expected fired counter increment")
	}
	if testutil.ToFloat64(active) != activeBefore+1 {
		t.Fatalf("expected active gauge increment")
	}

	RecordAlertResolved(alert)

	if testutil.ToFloat64(resolved) != resolvedBefore+1 {
		t.Fatalf("expected resolved counter increment")
	}
	if testutil.ToFloat64(active) != activeBefore {
		t.Fatalf("expected active gauge to return to baseline")
	}

	ackBefore := testutil.ToFloat64(AlertsAcknowledgedTotal)
	RecordAlertAcknowledged()
	if testutil.ToFloat64(AlertsAcknowledgedTotal) != ackBefore+1 {
		t.Fatalf("expected acknowledged counter increment")
	}

	suppressed := AlertsSuppressedTotal.WithLabelValues("unit_test_reason")
	suppressedBefore := testutil.ToFloat64(suppressed)
	RecordAlertSuppressed("unit_test_reason")
	if testutil.ToFloat64(suppressed) != suppressedBefore+1 {
		t.Fatalf("expected suppressed counter increment")
	}
}
