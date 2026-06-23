package chat

import (
	"os"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
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
	m.RecordAutoRecoveryAttempt(agentcapabilities.ErrCodeStrictResolution, "pulse_query")
	m.RecordAutoRecoverySuccess(agentcapabilities.ErrCodeStrictResolution, "pulse_query")
	m.RecordAgenticIteration("provider", "model")

	cb := NewAIMetricsTelemetryCallback()
	cb.RecordStrictResolutionBlock("validateResolvedResource", "start")
	cb.RecordAutoRecoveryAttempt("ERR", "tool")
	cb.RecordAutoRecoverySuccess("ERR", "tool")
	cb.RecordRoutingMismatchBlock("pulse_control", "node", "vm")
}

func TestAIMetricsTelemetryUsesSharedToolResponseErrorCodes(t *testing.T) {
	src, err := os.ReadFile("metrics.go")
	if err != nil {
		t.Fatalf("read metrics.go: %v", err)
	}
	text := string(src)
	for _, fragment := range []string{
		"agentcapabilities.ErrCodeStrictResolution",
		"agentcapabilities.ErrCodeRoutingMismatch",
	} {
		if !strings.Contains(text, fragment) {
			t.Fatalf("metrics telemetry must use shared error-code vocabulary; missing %s", fragment)
		}
	}
	for _, literal := range []string{
		`RecordAutoRecoveryAttempt("STRICT_RESOLUTION"`,
		`RecordAutoRecoveryAttempt("ROUTING_MISMATCH"`,
	} {
		if strings.Contains(text, literal) {
			t.Fatalf("metrics telemetry must not hardcode shared tool-response error codes; found %s", literal)
		}
	}
}
