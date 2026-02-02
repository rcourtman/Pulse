package chat

import "testing"

func TestAIMetricsRecording(t *testing.T) {
	m := GetAIMetrics()
	if sanitizeLabel("") != "unknown" {
		t.Fatalf("expected empty label to be sanitized")
	}
	if sanitizeLabel("has space") != "has_space" {
		t.Fatalf("expected spaces to be replaced")
	}

	m.RecordFSMToolBlock(StateReading, "pulse_query", ToolKindResolve)
	m.RecordFSMFinalBlock(StateResolving)
	m.RecordStrictResolutionBlock("validateResolvedResource", "restart")
	m.RecordRoutingMismatchBlock("pulse_control", "node", "vm")
	m.RecordPhantomDetected("provider", "model")
	m.RecordAutoRecoveryAttempt("STRICT_RESOLUTION", "pulse_query")
	m.RecordAutoRecoverySuccess("STRICT_RESOLUTION", "pulse_query")
	m.RecordAgenticIteration("provider", "model")

	cb := NewAIMetricsTelemetryCallback()
	cb.RecordStrictResolutionBlock("validateResolvedResource", "start")
	cb.RecordAutoRecoveryAttempt("ERR", "tool")
	cb.RecordAutoRecoverySuccess("ERR", "tool")
	cb.RecordRoutingMismatchBlock("pulse_control", "node", "vm")
}
