package providers

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
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

func TestProviderContracts_StreamToolInputParsingUsesSharedAgentCapabilities(t *testing.T) {
	parsed, ok := agentcapabilities.ParseProviderToolInput(`{"host":"nas01"}`)
	if !ok || parsed["host"] != "nas01" {
		t.Fatalf("shared parser returned %#v, %v", parsed, ok)
	}
}

func TestProviderToolUsesSharedAgentCapabilitiesShape(t *testing.T) {
	var shared agentcapabilities.ProviderTool = Tool{
		Name:        "diagnose",
		Description: "Diagnose infrastructure",
		InputSchema: map[string]interface{}{
			"type": "object",
		},
	}
	if shared.Name != "diagnose" {
		t.Fatalf("shared provider tool name = %q", shared.Name)
	}

	tool := Tool{Name: "diagnose"}.NormalizeCollections()
	if tool.InputSchema == nil {
		t.Fatal("provider tool alias must retain shared NormalizeCollections behavior")
	}
}

func TestProviderContracts_PulseProviderToolMetadataStaysOutOfUpstreamEncoders(t *testing.T) {
	for _, path := range []string{"openai.go", "anthropic.go", "gemini.go", "ollama.go"} {
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		text := string(source)
		for _, forbidden := range []string{
			"BehaviorHints",
			"PulseGovernance",
			"behavior_hints",
			"pulse_governance",
		} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("%s must not forward Pulse-local provider tool metadata to upstream provider payloads; found %s", path, forbidden)
			}
		}
	}
}

func TestProviderToolCallAndResultUseSharedAgentCapabilitiesShape(t *testing.T) {
	var sharedCall agentcapabilities.ProviderToolCall = ToolCall{
		ID:    "call-1",
		Name:  "diagnose",
		Input: map[string]interface{}{"resource_id": "vm/100"},
	}
	if sharedCall.Name != "diagnose" {
		t.Fatalf("shared provider tool call name = %q", sharedCall.Name)
	}

	toolCall := ToolCall{Name: "diagnose"}.NormalizeCollections()
	if toolCall.Input == nil {
		t.Fatal("provider tool call alias must retain shared NormalizeCollections behavior")
	}

	input := map[string]interface{}{"resource_id": "vm/100"}
	toolCall = ToolCall{Name: "diagnose", Input: input}.NormalizeCollections()
	toolCall.Input["resource_id"] = "vm/101"
	if input["resource_id"] != "vm/100" {
		t.Fatalf("provider tool call alias must retain shared no-alias input normalization: source=%#v normalized=%#v", input, toolCall.Input)
	}

	emptyToolCall := EmptyToolCall()
	if emptyToolCall.Input == nil {
		t.Fatal("EmptyToolCall must return the shared initialized provider tool-call shape")
	}

	var sharedResult agentcapabilities.ProviderToolResult = ToolResult{
		ToolUseID: "call-1",
		Content:   "done",
		IsError:   true,
	}
	if sharedResult.ToolUseID != "call-1" {
		t.Fatalf("shared provider tool result id = %q", sharedResult.ToolUseID)
	}
}
