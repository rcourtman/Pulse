package chat

import (
	"context"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

type stubServiceProvider struct {
	streamFn func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error
}

func (s *stubServiceProvider) Chat(ctx context.Context, req providers.ChatRequest) (*providers.ChatResponse, error) {
	return &providers.ChatResponse{Content: "ok", Model: req.Model}, nil
}

func (s *stubServiceProvider) TestConnection(ctx context.Context) error { return nil }

func (s *stubServiceProvider) Name() string { return "stub" }

func (s *stubServiceProvider) ListModels(ctx context.Context) ([]providers.ModelInfo, error) {
	return nil, nil
}

func (s *stubServiceProvider) ChatStream(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
	if s.streamFn != nil {
		return s.streamFn(ctx, req, callback)
	}
	callback(providers.StreamEvent{
		Type: "content",
		Data: providers.ContentEvent{Text: "hello"},
	})
	callback(providers.StreamEvent{
		Type: "done",
		Data: providers.DoneEvent{InputTokens: 1, OutputTokens: 1},
	})
	return nil
}

func (s *stubServiceProvider) SupportsThinking(model string) bool { return false }

func TestService_ExecuteStream_Success(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}

	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{})
	provider := &stubServiceProvider{}
	loop := NewAgenticLoop(provider, executor, "system")

	svc := &Service{
		cfg:         &config.AIConfig{ChatModel: "openai:test"},
		sessions:    store,
		executor:    executor,
		agenticLoop: loop,
		provider:    provider, // Required: per-request loops need a provider to create new instances
		started:     true,
	}

	var doneCount int
	callback := func(event StreamEvent) {
		if event.Type == "done" {
			doneCount++
		}
	}

	req := ExecuteRequest{
		SessionID: "sess-1",
		Prompt:    "hello",
	}
	if err := svc.ExecuteStream(context.Background(), req, callback); err != nil {
		t.Fatalf("ExecuteStream failed: %v", err)
	}
	if doneCount != 1 {
		t.Fatalf("expected done callback once, got %d", doneCount)
	}

	messages, err := store.GetMessages("sess-1")
	if err != nil {
		t.Fatalf("GetMessages failed: %v", err)
	}
	if len(messages) < 2 {
		t.Fatalf("expected at least 2 messages, got %d", len(messages))
	}
}

func TestService_ExecuteStream_PrefetchMentionsAndOverrideModel(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}

	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{})
	provider := &stubServiceProvider{}
	loop := NewAgenticLoop(provider, executor, "system")

	state := models.StateSnapshot{
		VMs: []models.VM{
			{ID: "vm:node1:101", VMID: 101, Name: "vm1", Node: "node1", Status: "running"},
		},
	}

	var capturedModel string
	svc := &Service{
		cfg:               &config.AIConfig{ChatModel: "openai:primary"},
		sessions:          store,
		executor:          executor,
		agenticLoop:       loop,
		contextPrefetcher: NewContextPrefetcher(&mockStateProvider{state: state}, nil),
		started:           true,
		providerFactory: func(modelStr string) (providers.StreamingProvider, error) {
			capturedModel = modelStr
			return provider, nil
		},
	}

	autonomous := true
	req := ExecuteRequest{
		SessionID:      "sess-2",
		Prompt:         "check @vm1",
		Model:          "openai:override",
		Mentions:       []StructuredMention{{ID: "vm:node1:101", Name: "vm1", Type: "vm", Node: "node1"}},
		MaxTurns:       1,
		AutonomousMode: &autonomous,
	}

	if err := svc.ExecuteStream(context.Background(), req, func(StreamEvent) {}); err != nil {
		t.Fatalf("ExecuteStream failed: %v", err)
	}
	if capturedModel != "openai:override" {
		t.Fatalf("expected override model to be used, got %q", capturedModel)
	}

	resolved := store.GetResolvedContext("sess-2")
	if !resolved.WasRecentlyAccessed("vm:node1:101", time.Minute) {
		t.Fatal("expected explicit access to be recorded for structured mention")
	}
}
