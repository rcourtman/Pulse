package api

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/chat"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/stretchr/testify/require"
)

type stubStreamingProvider struct{}

func (p *stubStreamingProvider) Chat(ctx context.Context, req providers.ChatRequest) (*providers.ChatResponse, error) {
	return &providers.ChatResponse{Content: "ok", Model: req.Model}, nil
}

func (p *stubStreamingProvider) TestConnection(ctx context.Context) error { return nil }

func (p *stubStreamingProvider) Name() string { return "stub" }

func (p *stubStreamingProvider) ListModels(ctx context.Context) ([]providers.ModelInfo, error) {
	return nil, nil
}

func (p *stubStreamingProvider) SupportsThinking(model string) bool { return false }

func (p *stubStreamingProvider) ChatStream(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
	callback(providers.StreamEvent{Type: "content", Data: providers.ContentEvent{Text: "hello"}})
	callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{StopReason: "end_turn"}})
	return nil
}

func TestChatServiceAdapter_ExecuteStreamAndPatrol(t *testing.T) {
	cfg := chat.Config{
		DataDir: t.TempDir(),
		AIConfig: &config.AIConfig{
			Enabled:     true,
			ChatModel:   "stub:model",
			PatrolModel: "stub:model",
		},
	}
	service := chat.NewService(cfg)
	setUnexportedField(t, service, "providerFactory", func(string) (providers.StreamingProvider, error) {
		return &stubStreamingProvider{}, nil
	})
	require.NoError(t, service.Start(context.Background()))
	t.Cleanup(func() { _ = service.Stop(context.Background()) })

	adapter := &chatServiceAdapter{svc: service}

	var events []ai.ChatStreamEvent
	err := adapter.ExecuteStream(context.Background(), ai.ChatExecuteRequest{Prompt: "hi"}, func(ev ai.ChatStreamEvent) {
		events = append(events, ev)
	})
	require.NoError(t, err)
	if len(events) == 0 {
		t.Fatalf("expected stream events")
	}

	var sawContent bool
	for _, ev := range events {
		if ev.Type == "content" {
			sawContent = true
			var data chat.ContentData
			if err := json.Unmarshal(ev.Data, &data); err != nil {
				t.Fatalf("unmarshal content event: %v", err)
			}
			if data.Text != "hello" {
				t.Fatalf("unexpected content: %q", data.Text)
			}
		}
	}
	if !sawContent {
		t.Fatalf("expected content event")
	}

	resp, err := adapter.ExecutePatrolStream(context.Background(), ai.PatrolExecuteRequest{
		Prompt: "patrol",
	}, func(ai.ChatStreamEvent) {})
	require.NoError(t, err)
	if resp.Content == "" {
		t.Fatalf("expected patrol response content")
	}
}
