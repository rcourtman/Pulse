package providers

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestProviderContracts_UseCanonicalEmptyCollections(t *testing.T) {
	payload, err := json.Marshal(EmptyMessage())
	if err != nil {
		t.Fatalf("marshal empty provider message: %v", err)
	}
	if !strings.Contains(string(payload), `"tool_calls":[]`) {
		t.Fatalf("expected empty provider message to retain tool_calls array, got %s", payload)
	}

	payload, err = json.Marshal(EmptyToolCall())
	if err != nil {
		t.Fatalf("marshal empty provider tool call: %v", err)
	}
	if !strings.Contains(string(payload), `"input":{}`) {
		t.Fatalf("expected empty provider tool call to retain input object, got %s", payload)
	}

	payload, err = json.Marshal(ChatResponse{}.NormalizeCollections())
	if err != nil {
		t.Fatalf("marshal empty provider chat response: %v", err)
	}
	if !strings.Contains(string(payload), `"tool_calls":[]`) {
		t.Fatalf("expected empty provider chat response to retain tool_calls array, got %s", payload)
	}

	payload, err = json.Marshal(DoneEvent{}.NormalizeCollections())
	if err != nil {
		t.Fatalf("marshal empty provider done event: %v", err)
	}
	if !strings.Contains(string(payload), `"tool_calls":[]`) {
		t.Fatalf("expected empty provider done event to retain tool_calls array, got %s", payload)
	}

	payload, err = json.Marshal(ToolStartEvent{}.NormalizeCollections())
	if err != nil {
		t.Fatalf("marshal empty provider tool start event: %v", err)
	}
	if !strings.Contains(string(payload), `"input":{}`) {
		t.Fatalf("expected empty provider tool start event to retain input object, got %s", payload)
	}

	payload, err = json.Marshal(Message{
		ToolCalls: []ToolCall{{
			ID:   "call-1",
			Name: "diagnose",
		}},
	}.NormalizeCollections())
	if err != nil {
		t.Fatalf("marshal normalized provider message with tool call: %v", err)
	}
	if !strings.Contains(string(payload), `"input":{}`) {
		t.Fatalf("expected normalized provider message tool call to retain input object, got %s", payload)
	}

	req := ChatRequest{
		Messages: []Message{{Role: "user", Content: "hi"}},
		Tools: []Tool{{
			Name: "diagnose",
		}},
	}.NormalizeCollections()
	if req.Tools[0].InputSchema == nil {
		t.Fatal("expected normalized provider tool input_schema to be initialized")
	}

	payload, err = json.Marshal(EmptyTool())
	if err != nil {
		t.Fatalf("marshal empty provider tool: %v", err)
	}
	if !strings.Contains(string(payload), `"input_schema":{}`) {
		t.Fatalf("expected empty provider tool to retain input_schema object, got %s", payload)
	}
}
