package metrics

import "testing"

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
