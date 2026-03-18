package aicontracts

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestEmptyOrchestratorMessage_UsesCanonicalEmptyCollections(t *testing.T) {
	payload, err := json.Marshal(EmptyOrchestratorMessage())
	if err != nil {
		t.Fatalf("marshal empty orchestrator message: %v", err)
	}
	if !strings.Contains(string(payload), `"tool_calls":[]`) {
		t.Fatalf("expected empty orchestrator message to retain tool_calls array, got %s", payload)
	}
}

func TestEmptyOrchestratorToolCallInfo_UsesCanonicalEmptyCollections(t *testing.T) {
	payload, err := json.Marshal(EmptyOrchestratorToolCallInfo())
	if err != nil {
		t.Fatalf("marshal empty orchestrator tool call info: %v", err)
	}
	if !strings.Contains(string(payload), `"input":{}`) {
		t.Fatalf("expected empty orchestrator tool call info to retain input object, got %s", payload)
	}

	payload, err = json.Marshal(OrchestratorMessage{
		ToolCalls: []OrchestratorToolCallInfo{{
			ID:   "call-1",
			Name: "diagnose",
		}},
	}.NormalizeCollections())
	if err != nil {
		t.Fatalf("marshal normalized orchestrator message with tool call: %v", err)
	}
	if !strings.Contains(string(payload), `"input":{}`) {
		t.Fatalf("expected normalized orchestrator tool call to retain input object, got %s", payload)
	}
}

func TestEmptyInvestigationSession_UsesCanonicalEmptyCollections(t *testing.T) {
	payload, err := json.Marshal(EmptyInvestigationSession())
	if err != nil {
		t.Fatalf("marshal empty investigation session: %v", err)
	}
	if !strings.Contains(string(payload), `"tools_available":[]`) {
		t.Fatalf("expected empty investigation session to retain tools_available array, got %s", payload)
	}
	if !strings.Contains(string(payload), `"tools_used":[]`) {
		t.Fatalf("expected empty investigation session to retain tools_used array, got %s", payload)
	}
	if !strings.Contains(string(payload), `"evidence_ids":[]`) {
		t.Fatalf("expected empty investigation session to retain evidence_ids array, got %s", payload)
	}
}

func TestEmptyFix_UsesCanonicalEmptyCollections(t *testing.T) {
	payload, err := json.Marshal(EmptyFix())
	if err != nil {
		t.Fatalf("marshal empty fix: %v", err)
	}
	if !strings.Contains(string(payload), `"commands":[]`) {
		t.Fatalf("expected empty fix to retain commands array, got %s", payload)
	}
}
