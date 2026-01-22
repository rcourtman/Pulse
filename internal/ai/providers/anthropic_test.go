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

func TestAnthropicClient_New(t *testing.T) {
	c := NewAnthropicClient("test-key", "claude-3", 0)
	if c.baseURL != anthropicAPIURL {
		t.Errorf("Expected default baseURL, got %s", c.baseURL)
	}
	if c.client.Timeout != 300*time.Second {
		t.Errorf("Expected default timeout 300s, got %v", c.client.Timeout)
	}

	c2 := NewAnthropicClientWithBaseURL("test-key", "claude-3", "", -1)
	if c2.baseURL != anthropicAPIURL {
		t.Errorf("Expected default baseURL for empty, got %s", c2.baseURL)
	}
	if c2.client.Timeout != 300*time.Second {
		t.Errorf("Expected default timeout for negative, got %v", c2.client.Timeout)
	}
}

func TestAnthropicClient_Name(t *testing.T) {
	client := NewAnthropicClient("test-key", "claude-3-5-sonnet", 0)
	if client.Name() != "anthropic" {
		t.Errorf("Expected 'anthropic', got '%s'", client.Name())
	}
}

func TestAnthropicClient_Chat_ContextCanceled(t *testing.T) {
	client := NewAnthropicClient("test-key", "claude-3-5-sonnet", 0)

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
	client := NewAnthropicClient("test-key", "claude-3-5-sonnet", 0)

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

	client := NewAnthropicClientWithBaseURL("test-key", "claude-3-5-sonnet", server.URL+"/v1/messages", 0)
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

	client := NewAnthropicClientWithBaseURL("test-key", "claude-3-5-sonnet", server.URL+"/v1/messages", 0)
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

	client := NewAnthropicClientWithBaseURL("test-key", "claude-3-5-sonnet", server.URL+"/v1/messages", 0)
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

	client := NewAnthropicClientWithBaseURL("test-key", "claude-3-5-sonnet", server.URL+"/v1/messages", 0)
	if err := client.TestConnection(context.Background()); err != nil {
		t.Fatalf("TestConnection: %v", err)
	}
	if called != 1 {
		t.Fatalf("ListModels calls = %d, want 1", called)
	}
}
func TestAnthropicClient_Chat_AssistantToolCallsInRequest(t *testing.T) {
	var got map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		_ = json.NewEncoder(w).Encode(anthropicResponse{
			ID: "msg_123", Type: "message", Role: "assistant", Model: "claude-3",
			StopReason: "end_turn", Content: []anthropicContent{{Type: "text", Text: "ok"}},
		})
	}))
	defer server.Close()

	client := NewAnthropicClientWithBaseURL("test-key", "claude-3", server.URL, 0)
	_, err := client.Chat(context.Background(), ChatRequest{
		Messages: []Message{
			{Role: "assistant", Content: "I will use a tool.", ToolCalls: []ToolCall{{ID: "tc1", Name: "tool1", Input: map[string]any{"arg1": "val1"}}}},
		},
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}

	msgs := got["messages"].([]any)
	asstMsg := msgs[0].(map[string]any)
	content := asstMsg["content"].([]any)
	if len(content) != 2 {
		t.Fatalf("Expected 2 content blocks, got %d", len(content))
	}
}

func TestAnthropicClient_Chat_StripPrefixAndDefaultMaxTokens(t *testing.T) {
	var got map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		_ = json.NewEncoder(w).Encode(anthropicResponse{
			ID: "msg_123", Type: "message", Role: "assistant", Model: "claude-3",
			StopReason: "end_turn", Content: []anthropicContent{{Type: "text", Text: "ok"}},
		})
	}))
	defer server.Close()

	client := NewAnthropicClientWithBaseURL("test-key", "claude-3", server.URL, 0)
	_, err := client.Chat(context.Background(), ChatRequest{
		Model:    "anthropic:claude-3-opus",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}

	if got["model"] != "claude-3-opus" {
		t.Errorf("Expected model 'claude-3-opus', got %s", got["model"])
	}
	if got["max_tokens"].(float64) != 4096 {
		t.Errorf("Expected default max_tokens 4096, got %v", got["max_tokens"])
	}
}

func TestAnthropicClient_Chat_ServerToolAndWebSearchResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(anthropicResponse{
			ID:         "msg_123",
			Type:       "message",
			Role:       "assistant",
			Model:      "claude-3",
			StopReason: "end_turn",
			Content: []anthropicContent{
				{Type: "text", Text: "I found this information."},
				{Type: "server_tool_use", ID: "st1", Name: "web_search"},
				{Type: "web_search_tool_result", ID: "ws1"},
			},
		})
	}))
	defer server.Close()

	client := NewAnthropicClientWithBaseURL("test-key", "claude-3", server.URL, 0)
	resp, err := client.Chat(context.Background(), ChatRequest{
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.Content != "I found this information." {
		t.Errorf("Expected content, got %s", resp.Content)
	}
}

func TestAnthropicClient_Chat_Retry(t *testing.T) {
	var count int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		if count == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":{"message":"Overloaded"}}`))
			return
		}
		_ = json.NewEncoder(w).Encode(anthropicResponse{
			ID: "msg_123", Type: "message", Role: "assistant",
			StopReason: "end_turn", Content: []anthropicContent{{Type: "text", Text: "ok"}},
		})
	}))
	defer server.Close()

	client := NewAnthropicClientWithBaseURL("test-key", "claude-3", server.URL, 0)
	_, err := client.Chat(context.Background(), ChatRequest{Messages: []Message{{Role: "user", Content: "hi"}}})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 attempts, got %d", count)
	}
}

func TestAnthropicClient_ModelsEndpoint_Invalid(t *testing.T) {
	client := &AnthropicClient{baseURL: "invalid-url"}
	endpoint := client.modelsEndpoint()
	if endpoint != "https://api.anthropic.com/v1/models" {
		t.Errorf("Expected default models endpoint for invalid baseURL, got %s", endpoint)
	}
}

func TestAnthropicClient_ChatStream_ToolUse(t *testing.T) {
	stream := []string{
		`{"type":"message_start","message":{"usage":{"input_tokens":5}}}`,
		`{"type":"content_block_start","content_block":{"type":"text"}}`,
		`{"type":"content_block_delta","delta":{"type":"text_delta","text":"Hi "}}`,
		`{"type":"content_block_stop"}`,
		`{"type":"content_block_start","content_block":{"type":"tool_use","id":"tool_1","name":"get_time"}}`,
		`{"type":"content_block_delta","delta":{"type":"input_json_delta","partial_json":"{\"tz\":\"UTC\"}"}}`,
		`{"type":"content_block_stop"}`,
		`{"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":7}}`,
		`{"type":"message_stop"}`,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		for _, event := range stream {
			_, _ = w.Write([]byte("data: " + event + "\n\n"))
			w.(http.Flusher).Flush()
		}
	}))
	defer server.Close()

	client := NewAnthropicClientWithBaseURL("test-key", "claude-3-5-sonnet", server.URL, 0)

	var content string
	var done DoneEvent
	var doneCalled bool
	var toolStarts int

	err := client.ChatStream(context.Background(), ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hi"}},
	}, func(event StreamEvent) {
		switch event.Type {
		case "content":
			if data, ok := event.Data.(ContentEvent); ok {
				content += data.Text
			}
		case "tool_start":
			toolStarts++
		case "done":
			if data, ok := event.Data.(DoneEvent); ok {
				done = data
				doneCalled = true
			}
		}
	})
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}

	if content != "Hi " {
		t.Fatalf("content = %q", content)
	}
	if toolStarts != 1 {
		t.Fatalf("toolStarts = %d, want 1", toolStarts)
	}
	if !doneCalled {
		t.Fatalf("done event not called")
	}
	if done.StopReason != "tool_use" || len(done.ToolCalls) != 1 {
		t.Fatalf("unexpected done: %+v", done)
	}
	if done.ToolCalls[0].Name != "get_time" || done.InputTokens != 5 || done.OutputTokens != 7 {
		t.Fatalf("unexpected tool call or usage: %+v", done)
	}
}

func TestAnthropicClient_SupportsThinking(t *testing.T) {
	client := NewAnthropicClient("test-key", "claude-3-5-sonnet", 0)
	if client.SupportsThinking("claude-3-5-sonnet") {
		t.Fatal("expected SupportsThinking to be false")
	}
}
