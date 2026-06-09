package chat

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
)

func TestAgenticLoop_Setters(t *testing.T) {
	loop := &AgenticLoop{}
	loop.SetMaxTurns(7)
	if loop.maxTurns != 7 {
		t.Fatalf("expected maxTurns=7, got %d", loop.maxTurns)
	}

	loop.SetProviderInfo("provider", "model")
	if loop.providerName != "provider" || loop.modelName != "model" {
		t.Fatalf("expected provider/model to be set")
	}

	called := false
	loop.SetBudgetChecker(func() error {
		called = true
		return nil
	})
	if loop.budgetChecker == nil {
		t.Fatalf("expected budgetChecker to be set")
	}
	_ = loop.budgetChecker()
	if !called {
		t.Fatalf("expected budgetChecker to be invoked")
	}
}

func TestPruneMessagesForModel_Stateless(t *testing.T) {
	prev := StatelessContext
	StatelessContext = true
	defer func() { StatelessContext = prev }()

	messages := []Message{
		{Role: "user", Content: "first"},
		{Role: "assistant", Content: "ok"},
		{Role: "user", Content: "second"},
		{Role: "assistant", Content: "done"},
	}

	pruned := pruneMessagesForModel(messages)
	if len(pruned) != 1 {
		t.Fatalf("expected 1 message, got %d", len(pruned))
	}
	if pruned[0].Content != "second" {
		t.Fatalf("expected last user message to be kept")
	}
}

func TestPruneMessagesForModel_SkipsOrphanedToolResults(t *testing.T) {
	// Build a message list longer than MaxContextMessagesLimit so pruning occurs.
	messages := make([]Message, 0, MaxContextMessagesLimit+2)
	messages = append(messages, Message{Role: "user", Content: "a"})
	messages = append(messages, Message{Role: "assistant", Content: "b"})
	// This tool result should be dropped if it becomes the first item after pruning.
	messages = append(messages, Message{Role: "assistant", ToolResult: &ToolResult{Content: "tool"}})
	for i := 0; i < MaxContextMessagesLimit; i++ {
		messages = append(messages, Message{Role: "user", Content: "msg"})
	}

	pruned := pruneMessagesForModel(messages)
	if len(pruned) == 0 {
		t.Fatalf("expected pruned messages")
	}
	if pruned[0].ToolResult != nil {
		t.Fatalf("expected leading tool result to be pruned")
	}
}

func TestPruneMessagesForModel_SkipsAssistantWithToolCalls(t *testing.T) {
	// Ensure assistant tool calls at the start of the pruned window are skipped.
	messages := make([]Message, 0, MaxContextMessagesLimit+3)
	messages = append(messages, Message{Role: "user", Content: "seed"})
	messages = append(messages, Message{Role: "assistant", Content: "seed"})
	messages = append(messages, Message{Role: "assistant", ToolCalls: []ToolCall{{Name: "pulse_query"}}})
	messages = append(messages, Message{Role: "assistant", ToolResult: &ToolResult{Content: "result"}})
	for i := 0; i < MaxContextMessagesLimit; i++ {
		messages = append(messages, Message{Role: "user", Content: "msg"})
	}

	pruned := pruneMessagesForModel(messages)
	if len(pruned) == 0 {
		t.Fatalf("expected pruned messages")
	}
	if pruned[0].Role == "assistant" && len(pruned[0].ToolCalls) > 0 {
		t.Fatalf("expected assistant tool-call message to be pruned")
	}
	if pruned[0].ToolResult != nil {
		t.Fatalf("expected tool result following pruned tool call to be removed")
	}
}

type stubStreamingProvider struct {
	lastRequest providers.ChatRequest
	chatStream  func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error
}

func (s *stubStreamingProvider) Chat(ctx context.Context, req providers.ChatRequest) (*providers.ChatResponse, error) {
	return &providers.ChatResponse{Content: "ok"}, nil
}

func (s *stubStreamingProvider) ChatStream(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
	s.lastRequest = req
	if s.chatStream != nil {
		return s.chatStream(ctx, req, callback)
	}
	callback(providers.StreamEvent{Type: "content", Data: providers.ContentEvent{Text: "summary"}})
	callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{InputTokens: 1, OutputTokens: 1}})
	return nil
}

func (s *stubStreamingProvider) SupportsThinking(model string) bool       { return false }
func (s *stubStreamingProvider) TestConnection(ctx context.Context) error { return nil }
func (s *stubStreamingProvider) Name() string                             { return "stub" }
func (s *stubStreamingProvider) ListModels(ctx context.Context) ([]providers.ModelInfo, error) {
	return nil, nil
}

func TestEnsureFinalTextResponse(t *testing.T) {
	provider := &stubStreamingProvider{}
	loop := &AgenticLoop{provider: provider, baseSystemPrompt: "prompt"}

	result := loop.ensureFinalTextResponse(
		context.Background(),
		"session-1",
		[]Message{{Role: "assistant", Content: ""}},
		[]providers.Message{{Role: "assistant", Content: ""}},
		func(event StreamEvent) {},
	)
	if len(result) != 2 {
		t.Fatalf("expected summary message to be appended")
	}
	if provider.lastRequest.ToolChoice != nil || len(provider.lastRequest.Tools) != 0 {
		t.Fatalf("expected summary call to omit tools and tool_choice, got tools=%d choice=%+v", len(provider.lastRequest.Tools), provider.lastRequest.ToolChoice)
	}

	provider.chatStream = func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
		return errors.New("boom")
	}
	result2 := loop.ensureFinalTextResponse(
		context.Background(),
		"session-2",
		[]Message{
			{Role: "assistant", Content: ""},
			{Role: "user", ToolResult: &ToolResult{ToolUseID: "pulse_query_0", Content: "{\"nodes\":1}", IsError: false}},
		},
		[]providers.Message{{Role: "assistant", Content: ""}},
		func(event StreamEvent) {},
	)
	if len(result2) != 3 {
		t.Fatalf("expected fallback summary message when provider errors")
	}
	if !strings.Contains(result2[len(result2)-1].Content, "didn't return a written summary") {
		t.Fatalf("expected deterministic fallback summary, got %q", result2[len(result2)-1].Content)
	}

	provider.chatStream = func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
		callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{InputTokens: 1, OutputTokens: 1}})
		return nil
	}
	result3 := loop.ensureFinalTextResponse(
		context.Background(),
		"session-3",
		[]Message{
			{Role: "assistant", Content: ""},
			{Role: "user", ToolResult: &ToolResult{ToolUseID: "pulse_metrics_1", Content: "cpu ok", IsError: false}},
		},
		[]providers.Message{{Role: "assistant", Content: ""}},
		func(event StreamEvent) {},
	)
	if len(result3) != 3 {
		t.Fatalf("expected fallback summary when provider returns empty content")
	}
	fallback3 := result3[len(result3)-1].Content
	if !strings.Contains(fallback3, "didn't return a written summary") {
		t.Fatalf("expected clean fallback summary, got %q", fallback3)
	}
	// The fallback must not dump raw tool output / result snippets into the chat.
	if strings.Contains(fallback3, "cpu ok") || strings.Contains(fallback3, "result snippet") {
		t.Fatalf("fallback summary must not include raw tool output, got %q", fallback3)
	}
}

func TestEnsureFinalTextResponseRequiresAssistantTextAfterLatestToolResult(t *testing.T) {
	provider := &stubStreamingProvider{}
	loop := &AgenticLoop{provider: provider, baseSystemPrompt: "prompt"}

	var emitted strings.Builder
	result := loop.ensureFinalTextResponse(
		context.Background(),
		"session-pre-tool-preamble",
		[]Message{
			{Role: "user", Content: "how many devices are in this?"},
			{
				Role:    "assistant",
				Content: "I'll check the device nodes.",
				ToolCalls: []ToolCall{
					{ID: "call-1", Name: "pulse_read", Input: map[string]interface{}{"command": "ls /dev | wc -l"}},
				},
			},
			{
				Role:       "user",
				ToolResult: &ToolResult{ToolUseID: "pulse_read_0", Content: "4358", IsError: false},
			},
			{Role: "assistant", Content: ""},
		},
		[]providers.Message{
			{Role: "user", Content: "how many devices are in this?"},
			{
				Role:    "assistant",
				Content: "I'll check the device nodes.",
				ToolCalls: []providers.ToolCall{
					{ID: "call-1", Name: "pulse_read", Input: map[string]interface{}{"command": "ls /dev | wc -l"}},
				},
			},
			{Role: "tool", ToolResult: &providers.ToolResult{ToolUseID: "call-1", Content: "4358"}},
			{Role: "assistant", Content: ""},
		},
		func(event StreamEvent) {
			if event.Type != "content" {
				return
			}
			var data ContentData
			if err := json.Unmarshal(event.Data, &data); err == nil {
				emitted.WriteString(data.Text)
			}
		},
	)

	if len(result) != 5 {
		t.Fatalf("expected final summary to be appended after tool result, got %d messages", len(result))
	}
	if result[len(result)-1].Content != "summary" {
		t.Fatalf("expected provider summary to be appended, got %q", result[len(result)-1].Content)
	}
	if emitted.String() != "summary" {
		t.Fatalf("expected summary to be streamed, got %q", emitted.String())
	}
}

func TestHasFinalAssistantText(t *testing.T) {
	tests := []struct {
		name     string
		messages []Message
		want     bool
	}{
		{
			name:     "assistant text without tool results is final",
			messages: []Message{{Role: "assistant", Content: "done"}},
			want:     true,
		},
		{
			name: "assistant text before latest user prompt is not final",
			messages: []Message{
				{Role: "assistant", Content: "previous answer"},
				{Role: "user", Content: "new question"},
			},
			want: false,
		},
		{
			name: "pre tool text before latest tool result is not final",
			messages: []Message{
				{Role: "user", Content: "check it"},
				{Role: "assistant", Content: "I'll check", ToolCalls: []ToolCall{{Name: "pulse_query"}}},
				{Role: "user", ToolResult: &ToolResult{ToolUseID: "pulse_query_0", Content: "ok"}},
			},
			want: false,
		},
		{
			name: "assistant text after latest tool result is final",
			messages: []Message{
				{Role: "user", Content: "check it"},
				{Role: "assistant", Content: "I'll check", ToolCalls: []ToolCall{{Name: "pulse_query"}}},
				{Role: "user", ToolResult: &ToolResult{ToolUseID: "pulse_query_0", Content: "ok"}},
				{Role: "assistant", Content: "The result is ok."},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasFinalAssistantText(tt.messages); got != tt.want {
				t.Fatalf("hasFinalAssistantText() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEnsureFinalTextResponseAppliesRequestSanitizer(t *testing.T) {
	provider := &stubStreamingProvider{}
	loop := &AgenticLoop{provider: provider, baseSystemPrompt: "raw-host"}
	loop.SetRequestSanitizer(func(req providers.ChatRequest) providers.ChatRequest {
		req.System = strings.ReplaceAll(req.System, "raw-host", "[redacted]")
		req.Messages = append([]providers.Message(nil), req.Messages...)
		for i := range req.Messages {
			req.Messages[i].Content = strings.ReplaceAll(req.Messages[i].Content, "raw-host", "[redacted]")
		}
		return req
	})

	loop.ensureFinalTextResponse(
		context.Background(),
		"session-sanitized",
		[]Message{{Role: "assistant", Content: ""}},
		[]providers.Message{{Role: "user", Content: "check raw-host"}},
		func(event StreamEvent) {},
	)

	if strings.Contains(provider.lastRequest.System, "raw-host") {
		t.Fatalf("summary system prompt was not sanitized: %q", provider.lastRequest.System)
	}
	if strings.Contains(provider.lastRequest.Messages[0].Content, "raw-host") {
		t.Fatalf("summary message was not sanitized: %q", provider.lastRequest.Messages[0].Content)
	}
}

func TestBuildAutomaticFallbackSummary(t *testing.T) {
	summary := buildAutomaticFallbackSummary([]Message{
		{Role: "user", ToolResult: &ToolResult{ToolUseID: "pulse_query_0", Content: "nodes ok", IsError: false}},
		{Role: "user", ToolResult: &ToolResult{ToolUseID: "pulse_query_1", Content: "containers ok", IsError: false}},
		{Role: "user", ToolResult: &ToolResult{ToolUseID: "pulse_read_0", Content: "read failed", IsError: true}},
	})
	if !strings.Contains(summary, "I ran 2 checks") {
		t.Fatalf("unexpected fallback summary: %q", summary)
	}
	// Tool name is surfaced operator-facing (pulse_ prefix stripped), never the
	// raw provider call id.
	if !strings.Contains(summary, "query") {
		t.Fatalf("expected tool name in fallback summary, got %q", summary)
	}
	if !strings.Contains(summary, "1 tool error") {
		t.Fatalf("expected error count in fallback summary, got %q", summary)
	}
}

func TestBuildAutomaticFallbackSummary_UsesToolNamesNotCallIDsOrRawOutput(t *testing.T) {
	// Reproduces the real OpenRouter shape that produced the ugly chat dump:
	// tool results carry opaque provider call ids, and the real tool name lives
	// on the assistant tool call. The fallback must name the tools, not leak the
	// call ids, and must not dump raw tool output into chat-visible text.
	summary := buildAutomaticFallbackSummary([]Message{
		{Role: "assistant", ToolCalls: []ToolCall{
			{ID: "call_27f0f389aba4652a1e292dc", Name: "pulse_query"},
			{ID: "call_66a4659a104b4ee7807201cb", Name: "pulse_metrics"},
		}},
		{Role: "user", ToolResult: &ToolResult{ToolUseID: "call_27f0f389aba4652a1e292dc", Content: `{"nodes":2}`, IsError: false}},
		{Role: "user", ToolResult: &ToolResult{ToolUseID: "call_66a4659a104b4ee7807201cb", Content: `{"cpuPct":12}`, IsError: false}},
	})
	if strings.Contains(summary, "call_") {
		t.Fatalf("fallback summary must not leak provider call ids, got %q", summary)
	}
	if !strings.Contains(summary, "query") || !strings.Contains(summary, "metrics") {
		t.Fatalf("expected real tool names in fallback summary, got %q", summary)
	}
	if strings.Contains(summary, "nodes") || strings.Contains(summary, "cpuPct") || strings.Contains(summary, "{") {
		t.Fatalf("fallback summary must not dump raw tool output, got %q", summary)
	}
}

func TestExecuteToolSafely_RecoversPanic(t *testing.T) {
	exec := tools.NewPulseToolExecutor(tools.ExecutorConfig{})
	exec.RegisterTool(tools.RegisteredTool{
		Definition: tools.Tool{
			Name: "panic_tool",
			InputSchema: tools.InputSchema{
				Type:       "object",
				Properties: map[string]tools.PropertySchema{},
			},
		},
		Handler: func(_ context.Context, _ *tools.PulseToolExecutor, _ map[string]interface{}) (tools.CallToolResult, error) {
			panic("boom")
		},
	})

	loop := &AgenticLoop{executor: exec}
	result, err := loop.executeToolSafely(context.Background(), "panic_tool", map[string]interface{}{})
	if err == nil {
		t.Fatalf("expected panic recovery error")
	}
	if !result.IsError {
		t.Fatalf("expected error result after panic recovery")
	}
	if len(result.Content) == 0 || !strings.Contains(result.Content[0].Text, "tool panic in panic_tool") {
		t.Fatalf("unexpected panic recovery result: %+v", result)
	}
}

func TestAgenticLoop_RetriesProviderStreamBeforeEvents(t *testing.T) {
	provider := &stubStreamingProvider{}
	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{})
	loop := NewAgenticLoop(provider, executor, "prompt")

	callCount := 0
	provider.chatStream = func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
		callCount++
		if callCount == 1 {
			return errors.New("connection reset by peer")
		}
		callback(providers.StreamEvent{Type: "content", Data: providers.ContentEvent{Text: "hello"}})
		callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{}})
		return nil
	}

	var workflowStates []WorkflowStateData
	results, err := loop.Execute(context.Background(), "retry-before-events", []Message{{Role: "user", Content: "hi"}}, func(event StreamEvent) {
		if event.Type != "workflow_state" {
			return
		}
		var data WorkflowStateData
		if err := json.Unmarshal(event.Data, &data); err != nil {
			t.Fatalf("failed to decode workflow_state: %v", err)
		}
		workflowStates = append(workflowStates, data)
	})
	if err != nil {
		t.Fatalf("expected retry to recover stream failure, got error: %v", err)
	}
	if callCount != 2 {
		t.Fatalf("expected 2 provider attempts, got %d", callCount)
	}
	if len(workflowStates) != 1 {
		t.Fatalf("expected one retry workflow state, got %#v", workflowStates)
	}
	retry := workflowStates[0]
	if retry.Phase != "provider_retry" {
		t.Fatalf("retry phase = %q, want provider_retry", retry.Phase)
	}
	if retry.Attempt != 2 || retry.MaxAttempts != 2 {
		t.Fatalf("retry attempts = %d/%d, want 2/2", retry.Attempt, retry.MaxAttempts)
	}
	if retry.RetryAfterMS != 200 {
		t.Fatalf("retry backoff = %dms, want 200ms", retry.RetryAfterMS)
	}
	if retry.Message != "Selected route connection failed before any output; retrying." {
		t.Fatalf("retry message = %q", retry.Message)
	}
	if len(results) != 1 || results[0].Content != "hello" {
		t.Fatalf("unexpected results: %+v", results)
	}
}

func TestAgenticLoop_RetriesProviderStreamAfterOnlySuppressedArtifacts(t *testing.T) {
	provider := &stubStreamingProvider{}
	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{})
	loop := NewAgenticLoop(provider, executor, "prompt")

	callCount := 0
	provider.chatStream = func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
		callCount++
		if callCount == 1 {
			callback(providers.StreamEvent{
				Type: "content",
				Data: providers.ContentEvent{
					Text: "I'llcheckthedevicenodesinsidethecontainertoanswerthat.Letmecounttheentriesin/devandlisttheblockdevices.",
				},
			})
			callback(providers.StreamEvent{
				Type: "content",
				Data: providers.ContentEvent{Text: `pulse_read(target_host="current_resource", command="ls /dev | wc -l")`},
			})
			return errors.New("connection reset by peer")
		}
		callback(providers.StreamEvent{Type: "content", Data: providers.ContentEvent{Text: "There are 4,358 entries under /dev."}})
		callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{}})
		return nil
	}

	var emittedContent strings.Builder
	var workflowStates []WorkflowStateData
	results, err := loop.Execute(context.Background(), "retry-after-suppressed-artifacts", []Message{{Role: "user", Content: "how many devices in this"}}, func(event StreamEvent) {
		switch event.Type {
		case "content":
			var data ContentData
			if err := json.Unmarshal(event.Data, &data); err != nil {
				t.Fatalf("failed to decode content event: %v", err)
			}
			emittedContent.WriteString(data.Text)
		case "workflow_state":
			var data WorkflowStateData
			if err := json.Unmarshal(event.Data, &data); err != nil {
				t.Fatalf("failed to decode workflow_state: %v", err)
			}
			workflowStates = append(workflowStates, data)
		}
	})
	if err != nil {
		t.Fatalf("expected retry to recover suppressed artifact stream failure, got error: %v", err)
	}
	if callCount != 2 {
		t.Fatalf("expected 2 provider attempts, got %d", callCount)
	}
	if emittedContent.String() != "There are 4,358 entries under /dev." {
		t.Fatalf("unexpected streamed content: %q", emittedContent.String())
	}
	if strings.Contains(emittedContent.String(), "pulse_read") || strings.Contains(emittedContent.String(), "I'llcheck") {
		t.Fatalf("streamed content leaked suppressed artifact text: %q", emittedContent.String())
	}
	if len(workflowStates) != 1 || workflowStates[0].Phase != "provider_retry" {
		t.Fatalf("expected one provider_retry workflow state, got %#v", workflowStates)
	}
	if len(results) != 1 || results[0].Content != "There are 4,358 entries under /dev." {
		t.Fatalf("unexpected results: %+v", results)
	}
}

func TestAgenticLoop_DoesNotRetryAfterPartialEvents(t *testing.T) {
	provider := &stubStreamingProvider{}
	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{})
	loop := NewAgenticLoop(provider, executor, "prompt")

	callCount := 0
	provider.chatStream = func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
		callCount++
		callback(providers.StreamEvent{Type: "content", Data: providers.ContentEvent{Text: "partial"}})
		return errors.New("connection reset by peer")
	}

	_, err := loop.Execute(context.Background(), "no-retry-partial", []Message{{Role: "user", Content: "hi"}}, func(event StreamEvent) {})
	if err == nil {
		t.Fatalf("expected provider error when stream fails after partial output")
	}
	if callCount != 1 {
		t.Fatalf("expected no retry after partial output, got %d attempts", callCount)
	}
}

func TestAgenticLoop_EmitsFallbackErrorEventOnTransportFailure(t *testing.T) {
	provider := &stubStreamingProvider{}
	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{})
	loop := NewAgenticLoop(provider, executor, "prompt")

	provider.chatStream = func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
		callback(providers.StreamEvent{Type: "content", Data: providers.ContentEvent{Text: "partial"}})
		return errors.New("connection reset by peer")
	}

	var events []StreamEvent
	_, err := loop.Execute(context.Background(), "emit-fallback-error", []Message{{Role: "user", Content: "hi"}}, func(event StreamEvent) {
		events = append(events, event)
	})
	if err == nil {
		t.Fatalf("expected provider error when stream fails after partial output")
	}

	var foundError bool
	for _, event := range events {
		if event.Type != "error" {
			continue
		}
		foundError = true
		var data ErrorData
		if decodeErr := json.Unmarshal(event.Data, &data); decodeErr != nil {
			t.Fatalf("failed to decode error event payload: %v", decodeErr)
		}
		if strings.TrimSpace(data.Message) == "" {
			t.Fatalf("expected non-empty fallback error message")
		}
	}
	if !foundError {
		t.Fatalf("expected fallback error event to be emitted")
	}
}

func TestAgenticLoop_IgnoresErrorAfterDoneEvent(t *testing.T) {
	provider := &stubStreamingProvider{}
	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{})
	loop := NewAgenticLoop(provider, executor, "prompt")

	callCount := 0
	provider.chatStream = func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
		callCount++
		callback(providers.StreamEvent{Type: "content", Data: providers.ContentEvent{Text: "complete"}})
		callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{}})
		return errors.New("EOF")
	}

	results, err := loop.Execute(context.Background(), "ignore-after-done", []Message{{Role: "user", Content: "hi"}}, func(event StreamEvent) {})
	if err != nil {
		t.Fatalf("expected post-done error to be ignored, got: %v", err)
	}
	if callCount != 1 {
		t.Fatalf("expected single provider attempt, got %d", callCount)
	}
	if len(results) != 1 || results[0].Content != "complete" {
		t.Fatalf("unexpected results: %+v", results)
	}
}

func TestAgenticLoop_RetriesOnErrorEventBeforeVisibleOutput(t *testing.T) {
	provider := &stubStreamingProvider{}
	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{})
	loop := NewAgenticLoop(provider, executor, "prompt")

	callCount := 0
	provider.chatStream = func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
		callCount++
		if callCount == 1 {
			callback(providers.StreamEvent{Type: "error", Data: providers.ErrorEvent{Message: "connection reset by peer"}})
			return nil
		}
		callback(providers.StreamEvent{Type: "content", Data: providers.ContentEvent{Text: "recovered"}})
		callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{}})
		return nil
	}

	results, err := loop.Execute(context.Background(), "retry-error-event", []Message{{Role: "user", Content: "hi"}}, func(event StreamEvent) {})
	if err != nil {
		t.Fatalf("expected recovery after transient error event, got: %v", err)
	}
	if callCount != 2 {
		t.Fatalf("expected 2 provider attempts, got %d", callCount)
	}
	if len(results) != 1 || results[0].Content != "recovered" {
		t.Fatalf("unexpected results: %+v", results)
	}
}

func TestAgenticLoop_DoesNotRetryErrorEventAfterVisibleOutput(t *testing.T) {
	provider := &stubStreamingProvider{}
	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{})
	loop := NewAgenticLoop(provider, executor, "prompt")

	callCount := 0
	provider.chatStream = func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
		callCount++
		callback(providers.StreamEvent{Type: "content", Data: providers.ContentEvent{Text: "partial"}})
		callback(providers.StreamEvent{Type: "error", Data: providers.ErrorEvent{Message: "connection reset by peer"}})
		return nil
	}

	_, err := loop.Execute(context.Background(), "no-retry-error-after-content", []Message{{Role: "user", Content: "hi"}}, func(event StreamEvent) {})
	if err == nil {
		t.Fatalf("expected error when stream emits error after visible output")
	}
	if callCount != 1 {
		t.Fatalf("expected no retry after visible output, got %d attempts", callCount)
	}
}

func TestCommandExtraction(t *testing.T) {
	if cmd := getCommandFromInput(map[string]interface{}{"command": "ls"}); cmd != "ls" {
		t.Fatalf("expected command to be extracted")
	}
	if cmd := getCommandFromInput(map[string]interface{}{}); cmd != "<unknown>" {
		t.Fatalf("expected fallback command string")
	}
}

func TestAgenticLoop_DoesNotAutoRecoverStructuredToolCall(t *testing.T) {
	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{})
	failCalls := 0
	recoveryCalls := 0

	executor.RegisterTool(tools.RegisteredTool{
		Definition: tools.Tool{
			Name: "fail_tool",
			InputSchema: tools.InputSchema{
				Type:       "object",
				Properties: map[string]tools.PropertySchema{},
			},
		},
		Handler: func(ctx context.Context, exec *tools.PulseToolExecutor, args map[string]interface{}) (tools.CallToolResult, error) {
			failCalls++
			return tools.NewToolResponseResult(tools.NewToolBlockedError(
				tools.ErrCodeActionNotAllowed,
				"blocked",
				map[string]interface{}{
					"policy_boundary": "resource requires model-selected follow-up context",
				},
			)), nil
		},
	})
	executor.RegisterTool(tools.RegisteredTool{
		Definition: tools.Tool{
			Name: "recovery_tool",
			InputSchema: tools.InputSchema{
				Type: "object",
				Properties: map[string]tools.PropertySchema{
					"value": {Type: "string"},
				},
			},
		},
		Handler: func(ctx context.Context, exec *tools.PulseToolExecutor, args map[string]interface{}) (tools.CallToolResult, error) {
			recoveryCalls++
			value, _ := args["value"].(string)
			return tools.NewTextResult(value), nil
		},
	})

	provider := &stubStreamingProvider{}
	loop := NewAgenticLoop(provider, executor, "prompt")
	callCount := 0
	provider.chatStream = func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
		callCount++
		switch callCount {
		case 1:
			callback(providers.StreamEvent{
				Type: "tool_start",
				Data: providers.ToolStartEvent{ID: "call_1", Name: "fail_tool"},
			})
			callback(providers.StreamEvent{
				Type: "done",
				Data: providers.DoneEvent{
					ToolCalls: []providers.ToolCall{{
						ID:    "call_1",
						Name:  "fail_tool",
						Input: map[string]interface{}{},
					}},
				},
			})
		case 2:
			if len(req.Messages) != 3 {
				t.Fatalf("expected assistant tool call plus tool result, got %d messages", len(req.Messages))
			}
			if req.Messages[2].ToolResult == nil || !req.Messages[2].ToolResult.IsError {
				t.Fatalf("expected original blocked tool result, got %+v", req.Messages[2].ToolResult)
			}
			if strings.Contains(req.Messages[2].ToolResult.Content, "recovered through query") {
				t.Fatalf("blocked result should not be replaced by auto-recovery output: %+v", req.Messages[2].ToolResult)
			}
			callback(providers.StreamEvent{
				Type: "content",
				Data: providers.ContentEvent{Text: "The tool was blocked; I need to decide the next step."},
			})
			callback(providers.StreamEvent{
				Type: "done",
				Data: providers.DoneEvent{},
			})
		default:
			t.Fatalf("unexpected provider call %d", callCount)
		}
		return nil
	}

	results, err := loop.Execute(context.Background(), "structured-model-owned-recovery", []Message{{Role: "user", Content: "help"}}, func(event StreamEvent) {})
	if err != nil {
		t.Fatalf("expected model-owned recovery turn to continue, got %v", err)
	}
	if failCalls != 1 || recoveryCalls != 0 {
		t.Fatalf("expected only the model-selected failing call to execute, got fail=%d recovery=%d", failCalls, recoveryCalls)
	}
	if len(results) != 3 {
		t.Fatalf("expected assistant tool call, blocked tool result, and final response, got %+v", results)
	}
	if results[1].ToolResult == nil || !results[1].ToolResult.IsError {
		t.Fatalf("expected blocked tool result in transcript, got %+v", results[1])
	}
	if results[2].Content != "The tool was blocked; I need to decide the next step." {
		t.Fatalf("unexpected final response: %+v", results[2])
	}
}

func TestAgenticLoop_CancelsUnavailableCurrentResourcePendingToolCall(t *testing.T) {
	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{})
	provider := &stubStreamingProvider{}
	loop := NewAgenticLoop(provider, executor, "prompt")

	var events []StreamEvent
	callCount := 0
	provider.chatStream = func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
		callCount++
		switch callCount {
		case 1:
			callback(providers.StreamEvent{
				Type: "tool_start",
				Data: providers.ToolStartEvent{
					ID:   "call-read-dev",
					Name: "pulse_read",
				},
			})
			callback(providers.StreamEvent{
				Type: "done",
				Data: providers.DoneEvent{
					ToolCalls: []providers.ToolCall{{
						ID:   "call-read-dev",
						Name: "pulse_read",
						Input: map[string]interface{}{
							"action":      "exec",
							"target_host": "current_resource",
							"command":     "ls /dev | wc -l",
						},
					}},
				},
			})
		case 2:
			if len(req.Messages) != 3 {
				t.Fatalf("expected user, assistant tool call, and hidden tool result, got %d messages", len(req.Messages))
			}
			toolResult := req.Messages[2].ToolResult
			if toolResult == nil || !toolResult.IsError {
				t.Fatalf("expected hidden current_resource tool result error, got %+v", toolResult)
			}
			if !strings.Contains(toolResult.Content, "CURRENT_RESOURCE_UNAVAILABLE") {
				t.Fatalf("expected current resource block marker, got %q", toolResult.Content)
			}
			if !strings.Contains(toolResult.Content, "Ask which host, VM, container, app, or storage resource") {
				t.Fatalf("expected model-facing recovery instruction, got %q", toolResult.Content)
			}
			callback(providers.StreamEvent{
				Type: "content",
				Data: providers.ContentEvent{Text: "Which host, VM, container, app, or storage resource should I check?"},
			})
			callback(providers.StreamEvent{
				Type: "done",
				Data: providers.DoneEvent{},
			})
		default:
			t.Fatalf("unexpected provider call %d", callCount)
		}
		return nil
	}

	results, err := loop.Execute(context.Background(), "current-resource-hidden-block", []Message{{Role: "user", Content: "how many devices in this"}}, func(event StreamEvent) {
		events = append(events, event)
	})
	if err != nil {
		t.Fatalf("expected hidden current_resource recovery turn, got %v", err)
	}
	if callCount != 2 {
		t.Fatalf("expected recovery turn after hidden current_resource block, got %d provider calls", callCount)
	}
	toolStarts := 0
	toolCancels := 0
	for _, event := range events {
		switch event.Type {
		case "tool_start":
			toolStarts++
		case "tool_cancel":
			toolCancels++
			var data ToolCancelData
			if err := json.Unmarshal(event.Data, &data); err != nil {
				t.Fatalf("unmarshal tool_cancel event: %v", err)
			}
			if data.ID != "call-read-dev" || data.Name != "pulse_read" {
				t.Fatalf("unexpected tool_cancel payload: %+v", data)
			}
		case "tool_end":
			t.Fatalf("current_resource placeholder block should cancel the pending tool, not complete it: %+v", event)
		}
	}
	if toolStarts != 1 || toolCancels != 1 {
		t.Fatalf("expected one early tool_start and one tool_cancel, got starts=%d cancels=%d events=%+v", toolStarts, toolCancels, events)
	}
	if len(results) != 1 {
		t.Fatalf("expected only final assistant message to be session-visible, got %d: %+v", len(results), results)
	}
	if len(results[0].ToolCalls) != 0 || results[0].ToolResult != nil {
		t.Fatalf("expected hidden placeholder call to be absent from session-visible messages, got %+v", results[0])
	}
	if !strings.Contains(results[0].Content, "Which host") {
		t.Fatalf("expected assistant to ask for target, got %q", results[0].Content)
	}
}
