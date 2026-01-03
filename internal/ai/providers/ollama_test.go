package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestOllamaClient_Chat_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/chat" {
			t.Errorf("Expected /api/chat, got %s", r.URL.Path)
		}

		// Decode request to verify it
		var req ollamaRequest
		json.NewDecoder(r.Body).Decode(&req)

		if req.Model != "llama2" {
			t.Errorf("Expected model llama2, got %s", req.Model)
		}

		// Return mock response
		resp := ollamaResponse{
			Model:     "llama2",
			CreatedAt: time.Now().Format(time.RFC3339),
			Message: ollamaMessageResp{
				Role:    "assistant",
				Content: "Hello! I'm Llama.",
			},
			Done:            true,
			DoneReason:      "stop",
			PromptEvalCount: 10,
			EvalCount:       15,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewOllamaClient("llama2", server.URL, 0)

	ctx := context.Background()
	resp, err := client.Chat(ctx, ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hello"}},
	})

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if resp.Content != "Hello! I'm Llama." {
		t.Errorf("Expected content 'Hello! I'm Llama.', got '%s'", resp.Content)
	}

	if resp.Model != "llama2" {
		t.Errorf("Expected model 'llama2', got '%s'", resp.Model)
	}

	if resp.InputTokens != 10 {
		t.Errorf("Expected 10 input tokens, got %d", resp.InputTokens)
	}

	if resp.OutputTokens != 15 {
		t.Errorf("Expected 15 output tokens, got %d", resp.OutputTokens)
	}
}

func TestOllamaClient_Chat_WithSystemPrompt(t *testing.T) {
	var receivedMessages []ollamaMessage

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ollamaRequest
		json.NewDecoder(r.Body).Decode(&req)
		receivedMessages = req.Messages

		resp := ollamaResponse{
			Model:   "llama2",
			Message: ollamaMessageResp{Role: "assistant", Content: "Response"},
			Done:    true,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewOllamaClient("llama2", server.URL, 0)

	ctx := context.Background()
	_, err := client.Chat(ctx, ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hello"}},
		System:   "You are a helpful assistant",
	})

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify system message was included
	if len(receivedMessages) < 2 {
		t.Fatalf("Expected at least 2 messages, got %d", len(receivedMessages))
	}

	if receivedMessages[0].Role != "system" {
		t.Errorf("Expected first message to be system, got %s", receivedMessages[0].Role)
	}

	if receivedMessages[0].Content != "You are a helpful assistant" {
		t.Errorf("Expected system content, got %s", receivedMessages[0].Content)
	}
}

func TestOllamaClient_Chat_WithOptions(t *testing.T) {
	var receivedRequest ollamaRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedRequest)

		resp := ollamaResponse{
			Model:   "llama2",
			Message: ollamaMessageResp{Role: "assistant", Content: "Response"},
			Done:    true,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewOllamaClient("llama2", server.URL, 0)

	ctx := context.Background()
	_, err := client.Chat(ctx, ChatRequest{
		Messages:    []Message{{Role: "user", Content: "Hello"}},
		MaxTokens:   500,
		Temperature: 0.7,
	})

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if receivedRequest.Options == nil {
		t.Fatal("Expected options to be set")
	}

	if receivedRequest.Options.NumPredict != 500 {
		t.Errorf("Expected num_predict 500, got %d", receivedRequest.Options.NumPredict)
	}

	if receivedRequest.Options.Temperature != 0.7 {
		t.Errorf("Expected temperature 0.7, got %f", receivedRequest.Options.Temperature)
	}
}

func TestOllamaClient_Chat_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "Model not found"}`))
	}))
	defer server.Close()

	client := NewOllamaClient("nonexistent", server.URL, 0)

	ctx := context.Background()
	_, err := client.Chat(ctx, ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hello"}},
	})

	if err == nil {
		t.Error("Expected error for API failure")
	}
}

func TestOllamaClient_Chat_NetworkError(t *testing.T) {
	client := NewOllamaClient("llama2", "http://localhost:99999", 0)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := client.Chat(ctx, ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hello"}},
	})

	if err == nil {
		t.Error("Expected error for network failure")
	}
}

func TestOllamaClient_Chat_ModelFallback(t *testing.T) {
	var receivedModel string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ollamaRequest
		json.NewDecoder(r.Body).Decode(&req)
		receivedModel = req.Model

		resp := ollamaResponse{
			Model:   req.Model,
			Message: ollamaMessageResp{Role: "assistant", Content: "Response"},
			Done:    true,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Client with no default model
	client := NewOllamaClient("", server.URL, 0)

	ctx := context.Background()
	_, err := client.Chat(ctx, ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hello"}},
		// No model specified in request either
	})

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should fallback to llama3
	if receivedModel != "llama3" {
		t.Errorf("Expected fallback to llama3, got %s", receivedModel)
	}
}

func TestOllamaClient_Chat_StripModelPrefix(t *testing.T) {
	var receivedModel string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ollamaRequest
		json.NewDecoder(r.Body).Decode(&req)
		receivedModel = req.Model

		resp := ollamaResponse{
			Model:   req.Model,
			Message: ollamaMessageResp{Role: "assistant", Content: "Response"},
			Done:    true,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewOllamaClient("default", server.URL, 0)

	ctx := context.Background()
	_, err := client.Chat(ctx, ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hello"}},
		Model:    "ollama:llama2", // With prefix
	})

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should strip the prefix
	if receivedModel != "llama2" {
		t.Errorf("Expected model 'llama2' (prefix stripped), got %s", receivedModel)
	}
}

func TestOllamaClient_TestConnection_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/version" {
			t.Errorf("Expected /api/version, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"version": "0.1.0"}`))
	}))
	defer server.Close()

	client := NewOllamaClient("llama2", server.URL, 0)

	ctx := context.Background()
	err := client.TestConnection(ctx)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestOllamaClient_TestConnection_Failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := NewOllamaClient("llama2", server.URL, 0)

	ctx := context.Background()
	err := client.TestConnection(ctx)

	if err == nil {
		t.Error("Expected error for failed connection test")
	}
}

func TestOllamaClient_ListModels_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			t.Errorf("Expected /api/tags, got %s", r.URL.Path)
		}

		resp := struct {
			Models []struct {
				Name       string `json:"name"`
				ModifiedAt string `json:"modified_at"`
				Size       int64  `json:"size"`
			} `json:"models"`
		}{
			Models: []struct {
				Name       string `json:"name"`
				ModifiedAt string `json:"modified_at"`
				Size       int64  `json:"size"`
			}{
				{Name: "llama2:latest", ModifiedAt: "2024-01-01T00:00:00Z", Size: 1000000},
				{Name: "mistral:latest", ModifiedAt: "2024-01-01T00:00:00Z", Size: 2000000},
			},
		}

		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewOllamaClient("llama2", server.URL, 0)

	ctx := context.Background()
	models, err := client.ListModels(ctx)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(models) != 2 {
		t.Errorf("Expected 2 models, got %d", len(models))
	}

	if models[0].ID != "llama2:latest" {
		t.Errorf("Expected first model 'llama2:latest', got '%s'", models[0].ID)
	}
}

func TestOllamaClient_ListModels_Failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal error"))
	}))
	defer server.Close()

	client := NewOllamaClient("llama2", server.URL, 0)

	ctx := context.Background()
	_, err := client.ListModels(ctx)

	if err == nil {
		t.Error("Expected error for failed list models")
	}
}

func TestNewOllamaClient_NormalizesBaseURL(t *testing.T) {
	tests := []struct {
		in       string
		expected string
	}{
		{"", "http://localhost:11434"},
		{"http://example:11434", "http://example:11434"},
		{"http://example:11434/", "http://example:11434"},
		{"http://example:11434/api", "http://example:11434"},
		{"http://example:11434/api/", "http://example:11434"},
	}

	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			client := NewOllamaClient("llama3", tc.in, 0)
			if client.baseURL != tc.expected {
				t.Fatalf("baseURL = %q, want %q", client.baseURL, tc.expected)
			}
		})
	}
}

func TestOllamaClient_Chat_ToolCallsResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ollamaResponse{
			Model: "llama3",
			Message: ollamaMessageResp{
				Role:    "assistant",
				Content: "",
				ToolCalls: []ollamaToolCall{
					{
						ID: "call_1",
						Function: ollamaFunctionCall{
							Name:      "get_time",
							Arguments: map[string]interface{}{"tz": "UTC"},
						},
					},
				},
			},
			Done:       true,
			DoneReason: "stop",
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewOllamaClient("llama3", server.URL, 0)
	out, err := client.Chat(context.Background(), ChatRequest{
		Messages: []Message{{Role: "user", Content: "What time is it?"}},
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if out.StopReason != "tool_use" {
		t.Fatalf("StopReason = %q, want tool_use", out.StopReason)
	}
	if len(out.ToolCalls) != 1 {
		t.Fatalf("ToolCalls = %d, want 1", len(out.ToolCalls))
	}
	if out.ToolCalls[0].ID != "call_1" || out.ToolCalls[0].Name != "get_time" {
		t.Fatalf("unexpected tool call: %+v", out.ToolCalls[0])
	}
	if out.ToolCalls[0].Input["tz"] != "UTC" {
		t.Fatalf("unexpected tool call input: %+v", out.ToolCalls[0].Input)
	}
}

func TestOllamaClient_Chat_ToolCallsAndToolResultsInRequest(t *testing.T) {
	var got ollamaRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		_ = json.NewEncoder(w).Encode(ollamaResponse{
			Model:   got.Model,
			Message: ollamaMessageResp{Role: "assistant", Content: "ok"},
			Done:    true,
		})
	}))
	defer server.Close()

	client := NewOllamaClient("llama3", server.URL, 0)
	_, err := client.Chat(context.Background(), ChatRequest{
		System: "system prompt",
		Messages: []Message{
			{
				Role:    "assistant",
				Content: "calling tool",
				ToolCalls: []ToolCall{
					{Name: "get_time", Input: map[string]any{"tz": "UTC"}},
				},
			},
			{
				Role:       "assistant",
				ToolResult: &ToolResult{Content: "{\"time\":\"00:00\"}"},
			},
		},
		Tools: []Tool{
			{
				Type:        "function",
				Name:        "get_time",
				Description: "get time",
				InputSchema: map[string]any{"type": "object"},
			},
			{
				Type: "web_search",
				Name: "search",
			},
		},
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}

	if got.Messages[0].Role != "system" || got.Messages[0].Content != "system prompt" {
		t.Fatalf("expected system message first, got: %+v", got.Messages[0])
	}

	var sawAssistantToolCall bool
	var sawToolResult bool
	for _, m := range got.Messages {
		if m.Role == "assistant" && len(m.ToolCalls) == 1 && m.ToolCalls[0].Function.Name == "get_time" {
			if m.ToolCalls[0].Function.Arguments["tz"] != "UTC" {
				t.Fatalf("unexpected tool call args: %+v", m.ToolCalls[0].Function.Arguments)
			}
			sawAssistantToolCall = true
		}
		if m.Role == "tool" && strings.Contains(m.Content, "00:00") {
			sawToolResult = true
		}
	}
	if !sawAssistantToolCall {
		t.Fatalf("expected assistant tool call message in request, got: %+v", got.Messages)
	}
	if !sawToolResult {
		t.Fatalf("expected tool result message in request, got: %+v", got.Messages)
	}

	if len(got.Tools) != 1 || got.Tools[0].Function.Name != "get_time" {
		t.Fatalf("expected only function tools to be included, got: %+v", got.Tools)
	}
}
func TestOllamaClient_Chat_ToolCallIDGeneration(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ollamaResponse{
			Model: "llama3",
			Message: ollamaMessageResp{
				Role:    "assistant",
				Content: "",
				ToolCalls: []ollamaToolCall{
					{
						// NO ID provided by Ollama
						Function: ollamaFunctionCall{
							Name:      "get_time",
							Arguments: map[string]interface{}{"tz": "UTC"},
						},
					},
				},
			},
			Done:       true,
			DoneReason: "stop",
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewOllamaClient("llama3", server.URL, 0)
	out, err := client.Chat(context.Background(), ChatRequest{
		Messages: []Message{{Role: "user", Content: "What time is it?"}},
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if len(out.ToolCalls) != 1 {
		t.Fatalf("ToolCalls = %d, want 1", len(out.ToolCalls))
	}
	if !strings.HasPrefix(out.ToolCalls[0].ID, "ollama_get_time_") {
		t.Errorf("Expected generated ID to start with ollama_get_time_, got %s", out.ToolCalls[0].ID)
	}
}

func TestOllamaClient_Chat_StatusError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Not Found"))
	}))
	defer server.Close()

	client := NewOllamaClient("llama3", server.URL, 0)
	_, err := client.Chat(context.Background(), ChatRequest{
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err == nil {
		t.Fatal("Expected error for 404 status")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("Expected error to contain 404, got %v", err)
	}
}

func TestNewOllamaClient_Defaults(t *testing.T) {
	c := NewOllamaClient("llama3", "", -1)
	if c.baseURL != "http://localhost:11434" {
		t.Errorf("Expected default baseURL, got %s", c.baseURL)
	}
	if c.client.Timeout != 300*time.Second {
		t.Errorf("Expected default timeout 300s, got %v", c.client.Timeout)
	}
}
