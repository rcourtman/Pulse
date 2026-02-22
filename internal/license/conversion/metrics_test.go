package conversion

import "testing"

func TestGetConversionMetricsSingleton(t *testing.T) {
	first := GetConversionMetrics()
	second := GetConversionMetrics()
	if first != second {
		t.Fatal("expected GetConversionMetrics to return singleton instance")
	}
}

func TestGetConversionMetricsRecordMethods(t *testing.T) {
	metrics := GetConversionMetrics()
	if metrics == nil {
		t.Fatal("GetConversionMetrics() returned nil")
	}

	// Compatibility shim should preserve behavior and remain no-panic for callers.
	metrics.RecordEvent("", "")
	metrics.RecordEvent("checkout_completed", "pricing_modal")
	metrics.RecordInvalid("")
	metrics.RecordInvalid("schema")
	metrics.RecordSkipped("")
	metrics.RecordSkipped("disabled")
}
