package chat

import (
	"context"
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
	if provider.lastRequest.ToolChoice == nil || provider.lastRequest.ToolChoice.Type != providers.ToolChoiceNone {
		t.Fatalf("expected summary call to enforce text-only tool choice")
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
	if !strings.Contains(result2[len(result2)-1].Content, "automatic summary") {
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
	if !strings.Contains(result3[len(result3)-1].Content, "Latest successful result snippet") {
		t.Fatalf("expected fallback summary to include tool snippet")
	}
}

func TestBuildAutomaticFallbackSummary(t *testing.T) {
	summary := buildAutomaticFallbackSummary([]Message{
		{Role: "user", ToolResult: &ToolResult{ToolUseID: "pulse_query_0", Content: "nodes ok", IsError: false}},
		{Role: "user", ToolResult: &ToolResult{ToolUseID: "pulse_query_1", Content: "containers ok", IsError: false}},
		{Role: "user", ToolResult: &ToolResult{ToolUseID: "pulse_read_0", Content: "read failed", IsError: true}},
	})
	if !strings.Contains(summary, "2 successful check(s)") {
		t.Fatalf("unexpected fallback summary: %q", summary)
	}
	if !strings.Contains(summary, "pulse_query") {
		t.Fatalf("expected tool name in fallback summary")
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

	results, err := loop.Execute(context.Background(), "retry-before-events", []Message{{Role: "user", Content: "hi"}}, func(event StreamEvent) {})
	if err != nil {
		t.Fatalf("expected retry to recover stream failure, got error: %v", err)
	}
	if callCount != 2 {
		t.Fatalf("expected 2 provider attempts, got %d", callCount)
	}
	if len(results) != 1 || results[0].Content != "hello" {
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

func TestTryAutoRecoveryAndCommandExtraction(t *testing.T) {
	result := tools.CallToolResult{
		Content: []tools.Content{{Type: "text", Text: `{"error":{"details":{"auto_recoverable":true,"suggested_rewrite":"pulse_query action=get","category":"strict"}}}`}},
		IsError: true,
	}
	rewrite, attempted := tryAutoRecovery(result, providers.ToolCall{Input: map[string]interface{}{}}, nil, context.Background())
	if attempted || rewrite == "" {
		t.Fatalf("expected suggested rewrite and no prior attempt")
	}

	alt := tools.CallToolResult{
		Content: []tools.Content{{Type: "text", Text: `{"error":5,"auto_recoverable":true,"suggested_rewrite":"pulse_query action=list"}`}},
		IsError: true,
	}
	rewrite, _ = tryAutoRecovery(alt, providers.ToolCall{Input: map[string]interface{}{}}, nil, context.Background())
	if rewrite == "" {
		t.Fatalf("expected suggested rewrite from alternate format")
	}

	rewrite, attempted = tryAutoRecovery(result, providers.ToolCall{Input: map[string]interface{}{"_auto_recovery_attempt": true}}, nil, context.Background())
	if !attempted || rewrite != "" {
		t.Fatalf("expected auto recovery to be skipped when already attempted")
	}

	if cmd := getCommandFromInput(map[string]interface{}{"command": "ls"}); cmd != "ls" {
		t.Fatalf("expected command to be extracted")
	}
	if cmd := getCommandFromInput(map[string]interface{}{}); cmd != "<unknown>" {
		t.Fatalf("expected fallback command string")
	}
}
