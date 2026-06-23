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
	for _, path := range []string{"openai.go", "anthropic.go"} {
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		text := string(source)
		for _, fragment := range []string{
			"agentcapabilities.ParseProviderToolInput(",
			"agentcapabilities.ProviderToolInputOrRaw(",
		} {
			if !strings.Contains(text, fragment) {
				t.Fatalf("%s must use shared provider tool input parsing; missing %s", path, fragment)
			}
		}
		for _, forbidden := range []string{
			`map[string]interface{}{"raw":`,
			`json.Unmarshal([]byte(currentToolInput.String())`,
			`json.Unmarshal([]byte(builder.args.String())`,
		} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("%s must not own provider tool input fallback parsing; found %s", path, forbidden)
			}
		}
	}

	providerSource, err := os.ReadFile("provider.go")
	if err != nil {
		t.Fatalf("read provider.go: %v", err)
	}
	if strings.Contains(string(providerSource), "func parseStreamToolInput(") {
		t.Fatal("provider package must not keep a local streamed tool-input parser")
	}

	parsed, ok := agentcapabilities.ParseProviderToolInput(`{"host":"nas01"}`)
	if !ok || parsed["host"] != "nas01" {
		t.Fatalf("shared parser returned %#v, %v", parsed, ok)
	}
}

func TestProviderContracts_AnthropicStreamingUsesSharedMessageProjection(t *testing.T) {
	source, err := os.ReadFile("anthropic.go")
	if err != nil {
		t.Fatalf("read anthropic.go: %v", err)
	}
	text := string(source)

	if strings.Count(text, "convertMessagesToAnthropic(req.Messages)") != 2 {
		t.Fatal("Anthropic Chat and ChatStream must both use convertMessagesToAnthropic for provider message projection")
	}

	streamStart := strings.Index(text, "func (c *AnthropicClient) ChatStream(")
	if streamStart < 0 {
		t.Fatal("missing Anthropic ChatStream")
	}
	streamText := text[streamStart:]
	for _, forbidden := range []string{
		`json.Marshal(m.ToolResult.Content)`,
		`Type:      "tool_result"`,
		`Type:  "tool_use"`,
		`Content: m.Content`,
	} {
		if strings.Contains(streamText, forbidden) {
			t.Fatalf("Anthropic ChatStream must not duplicate provider message projection; found %s", forbidden)
		}
	}
}

func TestProviderContracts_OpenAIStreamingUsesSharedMessageProjection(t *testing.T) {
	source, err := os.ReadFile("openai.go")
	if err != nil {
		t.Fatalf("read openai.go: %v", err)
	}
	text := string(source)

	if strings.Count(text, "convertMessagesToOpenAI(req, c.shouldSendReasoningContent())") != 2 {
		t.Fatal("OpenAI Chat and ChatStream must both use convertMessagesToOpenAI for provider message projection")
	}

	streamStart := strings.Index(text, "func (c *OpenAIClient) ChatStream(")
	if streamStart < 0 {
		t.Fatal("missing OpenAI ChatStream")
	}
	streamText := text[streamStart:]
	modelStart := strings.Index(streamText, "model := req.Model")
	if modelStart < 0 {
		t.Fatal("missing OpenAI ChatStream model setup")
	}
	streamProjectionText := streamText[:modelStart]
	for _, forbidden := range []string{
		`json.Marshal(tc.Input)`,
		`m.ToolResult`,
		`m.ToolCalls`,
		`m.ReasoningContent`,
		`m.Content`,
	} {
		if strings.Contains(streamProjectionText, forbidden) {
			t.Fatalf("OpenAI ChatStream must not duplicate provider message projection; found %s", forbidden)
		}
	}
}

func TestProviderContracts_GeminiStreamingUsesSharedMessageProjection(t *testing.T) {
	source, err := os.ReadFile("gemini.go")
	if err != nil {
		t.Fatalf("read gemini.go: %v", err)
	}
	text := string(source)

	if strings.Count(text, "convertMessagesToGemini(req.Messages)") != 2 {
		t.Fatal("Gemini Chat and ChatStream must both use convertMessagesToGemini for provider message projection")
	}

	streamStart := strings.Index(text, "func (c *GeminiClient) ChatStream(")
	if streamStart < 0 {
		t.Fatal("missing Gemini ChatStream")
	}
	streamText := text[streamStart:]
	modelStart := strings.Index(streamText, "model := req.Model")
	if modelStart < 0 {
		t.Fatal("missing Gemini ChatStream model setup")
	}
	streamProjectionText := streamText[:modelStart]
	for _, forbidden := range []string{
		`m.ToolResult`,
		`m.ToolCalls`,
		`m.Content`,
		`FunctionResponse:`,
		`FunctionCall:`,
	} {
		if strings.Contains(streamProjectionText, forbidden) {
			t.Fatalf("Gemini ChatStream must not duplicate provider message projection; found %s", forbidden)
		}
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
