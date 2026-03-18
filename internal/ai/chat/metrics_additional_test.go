package chat

import (
	"testing"
	"time"
)

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
	m.RecordExploreRun("success", "provider:model", 150*time.Millisecond, 12, 34)
	m.RecordExploreRun("skipped_no_model", "", 0, 0, 0)

	cb := NewAIMetricsTelemetryCallback()
	cb.RecordStrictResolutionBlock("validateResolvedResource", "start")
	cb.RecordAutoRecoveryAttempt("ERR", "tool")
	cb.RecordAutoRecoverySuccess("ERR", "tool")
	cb.RecordRoutingMismatchBlock("pulse_control", "node", "vm")
}
