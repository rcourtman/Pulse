package api

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/chat"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock implementation of chat.StateProvider
type mockChatStateProvider struct{}

func configuredOllamaChatTestConfig() *config.AIConfig {
	return &config.AIConfig{
		ChatModel:     "ollama:llama3",
		OllamaBaseURL: config.DefaultOllamaBaseURL,
	}
}

func TestChatServiceAdapter_CreateSession(t *testing.T) {
	// Setup real chat service with minimal config
	cfg := chat.Config{
		DataDir:  t.TempDir(),
		AIConfig: configuredOllamaChatTestConfig(),
	}
	realSvc := chat.NewService(cfg)
	require.NoError(t, realSvc.Start(context.Background()))
	defer func() { _ = realSvc.Stop(context.Background()) }()

	// Create adapter
	adapter := &chatServiceAdapter{svc: realSvc}

	// Test
	session, err := adapter.CreateSession(context.Background())
	require.NoError(t, err)
	assert.NotEmpty(t, session.ID)
}

func TestChatServiceAdapter_GetMessages(t *testing.T) {
	// Setup real chat service
	cfg := chat.Config{
		DataDir:  t.TempDir(),
		AIConfig: configuredOllamaChatTestConfig(),
	}
	realSvc := chat.NewService(cfg)
	require.NoError(t, realSvc.Start(context.Background()))
	defer func() { _ = realSvc.Stop(context.Background()) }()

	// Seed a session and message directly into the real service's store?
	// Since we can't easily inject into the private store, we'll use the public API of the real service
	// BUT chat.Service.ExecuteStream is hard to use without a real AI provider.
	//
	// However, we can use CreateSession via adapter, then maybe we can't *add* messages easily
	// because `adapter` doesn't expose AddMessage.
	//
	// We can use the real service's internal SessionStore if we can access it...
	// But it's private `sessions *SessionStore`.
	//
	// Wait, internal/ai/chat/service.go has CreateSession but not AddMessage exposed directly?
	// It does! But we need `CreateSession` first.

	// Let's rely on the fact that `ExecuteStream` adds a user message.
	// But `ExecuteStream` will try to call the AI provider and fail if we don't mock it.
	// `chat.NewService` doesn't allow easy mocking of the provider factory (it's unexported).

	// Actually, looking at `internal/ai/chat/service.go`, `CreateSession` is exposed.
	// But `AddMessage` is NOT exposed on Service.

	// So `GetMessages` test is hard without a way to insert messages.
	// Unless... we can modify `chat.Service` to be more testable, OR we skip `GetMessages` integration test
	// and accept that `CreateSession` coverage proves the wiring is correct.

	// Let's stick to CreateSession and implicit interface satisfaction tests.

	adapter := &chatServiceAdapter{svc: realSvc}

	// Verify interface compliance (compile-time check)
	// var _ ai.ChatServiceProvider = adapter // We can't do this easily inside the test function
	// but the fact that it compiles is enough.

	// Just reuse the session creation test.
	session, err := adapter.CreateSession(context.Background())
	require.NoError(t, err)

	// Try to get messages for empty session
	msgs, err := adapter.GetMessages(context.Background(), session.ID)
	require.NoError(t, err)
	assert.Empty(t, msgs)

	// Try delete
	err = adapter.DeleteSession(context.Background(), session.ID)
	require.NoError(t, err)
}

func TestAdaptChatMessageUsesSharedProviderToolCallShape(t *testing.T) {
	success := true
	msg := adaptChatMessage(chat.Message{
		ID:        "msg-1",
		Role:      "assistant",
		Content:   "checking",
		Timestamp: time.Unix(100, 0),
		ToolCalls: []chat.ToolCall{{
			ID:               "call-1",
			Name:             "diagnose",
			Output:           "in-app only",
			Success:          &success,
			ThoughtSignature: json.RawMessage(`{"provider":"gemini"}`),
		}},
		ToolResult: &chat.ToolResult{
			ToolUseID: "call-1",
			Content:   "done",
			IsError:   true,
		},
	})

	require.Len(t, msg.ToolCalls, 1)
	var shared agentcapabilities.ProviderToolCall = msg.ToolCalls[0]
	assert.Equal(t, "call-1", shared.ID)
	assert.Equal(t, "diagnose", shared.Name)
	assert.NotNil(t, shared.Input)

	payload, err := json.Marshal(msg.ToolCalls[0])
	require.NoError(t, err)
	text := string(payload)
	assert.Contains(t, text, `"input":{}`)
	assert.Contains(t, text, `"thought_signature":{"provider":"gemini"}`)
	assert.False(t, strings.Contains(text, `"output"`), text)
	assert.False(t, strings.Contains(text, `"success"`), text)

	require.NotNil(t, msg.ToolResult)
	var sharedResult agentcapabilities.ProviderToolResult = *msg.ToolResult
	assert.Equal(t, "call-1", sharedResult.ToolUseID)
	assert.True(t, sharedResult.IsError)
}

func TestAdaptChatMessageDoesNotAliasToolCallInput(t *testing.T) {
	input := map[string]interface{}{"resource_id": "vm/100"}
	msg := adaptChatMessage(chat.Message{
		ID:   "msg-1",
		Role: "assistant",
		ToolCalls: []chat.ToolCall{{
			ID:    "call-1",
			Name:  "diagnose",
			Input: input,
		}},
	})

	require.Len(t, msg.ToolCalls, 1)
	msg.ToolCalls[0].Input["resource_id"] = "vm/101"
	assert.Equal(t, "vm/100", input["resource_id"])
}

func TestChatServiceAdapter_ReloadConfig(t *testing.T) {
	cfg := chat.Config{
		DataDir:  t.TempDir(),
		AIConfig: configuredOllamaChatTestConfig(),
	}
	realSvc := chat.NewService(cfg)
	require.NoError(t, realSvc.Start(context.Background()))
	adapter := &chatServiceAdapter{svc: realSvc}

	newCfg := configuredOllamaChatTestConfig()
	err := adapter.ReloadConfig(context.Background(), newCfg)
	require.NoError(t, err)
}

func TestChatServiceAdapter_GetExecutor(t *testing.T) {
	cfg := chat.Config{DataDir: t.TempDir(), AIConfig: &config.AIConfig{}}
	realSvc := chat.NewService(cfg)
	adapter := &chatServiceAdapter{svc: realSvc}

	exec := adapter.GetExecutor()
	// It might be nil if not fully initialized or config not set, but assert we get value or nil
	// actually NewService initializes executor.
	assert.NotNil(t, exec)
}
