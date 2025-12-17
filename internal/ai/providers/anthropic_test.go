package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Anthropic tests focus on request/response correctness and endpoint behavior.

func TestAnthropicClient_Name(t *testing.T) {
	client := NewAnthropicClient("test-key", "claude-3-5-sonnet")
	if client.Name() != "anthropic" {
		t.Errorf("Expected 'anthropic', got '%s'", client.Name())
	}
}

func TestAnthropicClient_Chat_ContextCanceled(t *testing.T) {
	client := NewAnthropicClient("test-key", "claude-3-5-sonnet")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := client.Chat(ctx, ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hello"}},
	})

	if err == nil {
		t.Error("Expected error for canceled context")
	}
}

func TestAnthropicClient_Chat_Timeout(t *testing.T) {
	client := NewAnthropicClient("test-key", "claude-3-5-sonnet")

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	_, err := client.Chat(ctx, ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hello"}},
	})

	if err == nil {
		t.Error("Expected timeout error")
	}
}

func TestAnthropicClient_Chat_Success_TextAndToolCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/v1/messages" {
			t.Fatalf("path = %s, want /v1/messages", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "test-key" {
			t.Fatalf("x-api-key = %q", r.Header.Get("x-api-key"))
		}
		if r.Header.Get("anthropic-version") != anthropicAPIVersion {
			t.Fatalf("anthropic-version = %q", r.Header.Get("anthropic-version"))
		}

		var got anthropicRequest
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		if got.Model != "claude-3-5-sonnet" {
			t.Fatalf("Model = %q", got.Model)
		}
		if got.System != "You are helpful" {
			t.Fatalf("System = %q", got.System)
		}
		if got.MaxTokens != 123 {
			t.Fatalf("MaxTokens = %d", got.MaxTokens)
		}
		if len(got.Messages) != 1 || got.Messages[0].Role != "user" {
			t.Fatalf("unexpected messages: %+v", got.Messages)
		}
		if got.Messages[0].Content != "Hello" {
			t.Fatalf("unexpected message content: %+v", got.Messages[0])
		}
		if len(got.Tools) != 2 {
			t.Fatalf("tools = %d, want 2", len(got.Tools))
		}
		if got.Tools[0].Name != "get_time" || got.Tools[0].Type != "" {
			t.Fatalf("unexpected function tool: %+v", got.Tools[0])
		}
		if got.Tools[1].Type != "web_search_20250305" || got.Tools[1].Name != "web_search" || got.Tools[1].MaxUses != 2 {
			t.Fatalf("unexpected web search tool: %+v", got.Tools[1])
		}

		_ = json.NewEncoder(w).Encode(anthropicResponse{
			ID:         "msg_123",
			Type:       "message",
			Role:       "assistant",
			Model:      "claude-3-5-sonnet",
			StopReason: "tool_use",
			Content: []anthropicContent{
				{Type: "text", Text: "Hi"},
				{Type: "tool_use", ID: "tool_1", Name: "get_time", Input: map[string]any{"tz": "UTC"}},
			},
			Usage: anthropicUsage{InputTokens: 10, OutputTokens: 20},
		})
	}))
	defer server.Close()

	client := NewAnthropicClientWithBaseURL("test-key", "claude-3-5-sonnet", server.URL+"/v1/messages")
	out, err := client.Chat(context.Background(), ChatRequest{
		System:    "You are helpful",
		Messages:  []Message{{Role: "user", Content: "Hello"}},
		MaxTokens: 123,
		Tools: []Tool{
			{
				Name:        "get_time",
				Description: "get time",
				InputSchema: map[string]any{"type": "object"},
			},
			{
				Type:    "web_search_20250305",
				Name:    "web_search",
				MaxUses: 2,
			},
		},
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}

	if out.Content != "Hi" {
		t.Fatalf("Content = %q, want Hi", out.Content)
	}
	if out.StopReason != "tool_use" {
		t.Fatalf("StopReason = %q, want tool_use", out.StopReason)
	}
	if len(out.ToolCalls) != 1 || out.ToolCalls[0].ID != "tool_1" || out.ToolCalls[0].Name != "get_time" {
		t.Fatalf("unexpected ToolCalls: %+v", out.ToolCalls)
	}
	if out.ToolCalls[0].Input["tz"] != "UTC" {
		t.Fatalf("unexpected ToolCall input: %+v", out.ToolCalls[0].Input)
	}
	if out.InputTokens != 10 || out.OutputTokens != 20 {
		t.Fatalf("unexpected usage: %+v", out)
	}
}

func TestAnthropicClient_Chat_ToolResultInRequest(t *testing.T) {
	var got map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		_ = json.NewEncoder(w).Encode(anthropicResponse{
			ID:         "msg_123",
			Type:       "message",
			Role:       "assistant",
			Model:      "claude-3-5-sonnet",
			StopReason: "end_turn",
			Content:    []anthropicContent{{Type: "text", Text: "ok"}},
		})
	}))
	defer server.Close()

	client := NewAnthropicClientWithBaseURL("test-key", "claude-3-5-sonnet", server.URL+"/v1/messages")
	_, err := client.Chat(context.Background(), ChatRequest{
		Messages: []Message{
			{
				ToolResult: &ToolResult{
					ToolUseID: "tool_1",
					Content:   "{\"time\":\"00:00\"}",
					IsError:   true,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}

	msgs, ok := got["messages"].([]any)
	if !ok || len(msgs) != 1 {
		t.Fatalf("unexpected messages: %+v", got["messages"])
	}
	first, ok := msgs[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected message type: %T", msgs[0])
	}
	if first["role"] != "user" {
		t.Fatalf("role = %v, want user", first["role"])
	}
	contentArr, ok := first["content"].([]any)
	if !ok || len(contentArr) != 1 {
		t.Fatalf("unexpected content: %+v", first["content"])
	}
	block, ok := contentArr[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected content block type: %T", contentArr[0])
	}
	if block["type"] != "tool_result" || block["tool_use_id"] != "tool_1" || block["is_error"] != true {
		t.Fatalf("unexpected tool_result block: %+v", block)
	}
	if _, ok := block["content"].(string); !ok {
		t.Fatalf("expected tool_result content to be a string, got: %+v", block["content"])
	}
}

func TestAnthropicClient_ListModels_UsesConfiguredHost(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/v1/models" {
			t.Fatalf("path = %s, want /v1/models", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "test-key" {
			t.Fatalf("x-api-key = %q", r.Header.Get("x-api-key"))
		}
		if r.Header.Get("anthropic-version") != anthropicAPIVersion {
			t.Fatalf("anthropic-version = %q", r.Header.Get("anthropic-version"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"id": "claude-3-5-sonnet", "display_name": "Claude 3.5 Sonnet", "created_at": "2024-01-01T00:00:00Z"},
			},
		})
	}))
	defer server.Close()

	client := NewAnthropicClientWithBaseURL("test-key", "claude-3-5-sonnet", server.URL+"/v1/messages")
	models, err := client.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if len(models) != 1 || models[0].ID != "claude-3-5-sonnet" || models[0].Name != "Claude 3.5 Sonnet" {
		t.Fatalf("unexpected models: %+v", models)
	}
}

func TestAnthropicClient_TestConnection_CallsListModels(t *testing.T) {
	var called int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("path = %s, want /v1/models", r.URL.Path)
		}
		called++
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
	}))
	defer server.Close()

	client := NewAnthropicClientWithBaseURL("test-key", "claude-3-5-sonnet", server.URL+"/v1/messages")
	if err := client.TestConnection(context.Background()); err != nil {
		t.Fatalf("TestConnection: %v", err)
	}
	if called != 1 {
		t.Fatalf("ListModels calls = %d, want 1", called)
	}
}
