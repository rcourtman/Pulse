package chat

import (
	"context"
	"errors"
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
		[]Message{{Role: "assistant", Content: ""}},
		[]providers.Message{{Role: "assistant", Content: ""}},
		func(event StreamEvent) {},
	)
	if len(result2) != 1 {
		t.Fatalf("expected no summary message when provider errors")
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
