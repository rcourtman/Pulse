package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenAIClient_ChatStream_Success(t *testing.T) {
	// Mock OpenAI SSE stream
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/chat/completions", r.URL.Path)
		assert.Equal(t, "Bearer sk-test", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Send SSE events
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		events := []string{
			`{"id":"chatcmpl-1","choices":[{"delta":{"content":"Hello"}}],"object":"chat.completion.chunk"}`,
			`{"id":"chatcmpl-1","choices":[{"delta":{"content":" World"}}],"object":"chat.completion.chunk"}`,
			`[DONE]`,
		}

		for _, event := range events {
			if event == "[DONE]" {
				fmt.Fprintf(w, "data: %s\n\n", event)
			} else {
				fmt.Fprintf(w, "data: %s\n\n", event)
			}
			w.(http.Flusher).Flush()
			time.Sleep(10 * time.Millisecond)
		}
	}))
	defer server.Close()

	client := NewOpenAIClient("sk-test", "gpt-4", server.URL, 0)

	var receivedContent string
	var doneCalled bool

	callback := func(event StreamEvent) {
		switch event.Type {
		case "content":
			if data, ok := event.Data.(ContentEvent); ok {
				receivedContent += data.Text
			}
		case "done":
			doneCalled = true
		}
	}

	req := ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hi"}},
	}

	err := client.ChatStream(context.Background(), req, callback)
	require.NoError(t, err)
	assert.Equal(t, "Hello World", receivedContent)
	assert.True(t, doneCalled)
}

func TestOpenAIClient_ChatStream_ToolCall(t *testing.T) {
	// Mock tool call stream
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")

		events := []string{
			`{"id":"chatcmpl-2","choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_123","type":"function","function":{"name":"get_weather","arguments":""}}]}}]}`,
			`{"id":"chatcmpl-2","choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"loc"}}]}}]}`,
			`{"id":"chatcmpl-2","choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"ation\":\"NYC\"}"}}]}}]}`,
			`[DONE]`,
		}

		for _, event := range events {
			fmt.Fprintf(w, "data: %s\n\n", event)
			w.(http.Flusher).Flush()
			time.Sleep(10 * time.Millisecond)
		}
	}))
	defer server.Close()

	client := NewOpenAIClient("sk-test", "gpt-4", server.URL, 0)

	var toolCalls []ToolCall
	var toolStartIndex int

	callback := func(event StreamEvent) {
		t.Logf("Received event type: %s", event.Type)
		if event.Type == "tool_start" {
			toolStartIndex++
		}
		if event.Type == "done" {
			if data, ok := event.Data.(DoneEvent); ok {
				t.Logf("Received DONE event with %d tool calls", len(data.ToolCalls))
				toolCalls = data.ToolCalls
			} else {
				t.Logf("Received DONE event but type assertion to DoneEvent failed. Actual type: %T", event.Data)
			}
		}
	}

	err := client.ChatStream(context.Background(), ChatRequest{Messages: []Message{{Role: "user"}}}, callback)
	require.NoError(t, err)

	// Check that we got a tool_start event
	assert.Equal(t, 1, toolStartIndex, "Should have received 1 tool_start event")

	// Check accumulated tool calls in done event
	require.Len(t, toolCalls, 1)
	assert.Equal(t, "call_123", toolCalls[0].ID)
	assert.Equal(t, "get_weather", toolCalls[0].Name)
	assert.Equal(t, map[string]interface{}{"location": "NYC"}, toolCalls[0].Input)
}

func TestOpenAIClient_ChatStream_Errors(t *testing.T) {
	t.Run("401 Unauthorized", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]string{"message": "Invalid API key"},
			})
		}))
		defer server.Close()

		client := NewOpenAIClient("bad-key", "gpt-4", server.URL, 0)
		err := client.ChatStream(context.Background(), ChatRequest{}, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "401")
	})

	t.Run("Invalid Context", func(t *testing.T) {
		// No server needed if context cancelled immediately
		client := NewOpenAIClient("sk", "gpt", "http://localhost:12345", 0)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := client.ChatStream(ctx, ChatRequest{}, nil)
		assert.Error(t, err)
	})
}

func TestOpenAIClient_Configuration(t *testing.T) {
	tests := []struct {
		name     string
		baseURL  string
		expected string
	}{
		{
			name:     "Default",
			baseURL:  "",
			expected: "https://api.openai.com/v1/chat/completions",
		},
		{
			name:     "Custom Base URL",
			baseURL:  "https://custom.api/v1",
			expected: "https://custom.api/v1/chat/completions",
		},
		{
			name:     "Custom Full URL",
			baseURL:  "https://custom.api/v1/chat/completions",
			expected: "https://custom.api/v1/chat/completions",
		},
		{
			name:     "OpenRouter Style",
			baseURL:  "https://openrouter.ai/api/v1",
			expected: "https://openrouter.ai/api/v1/chat/completions",
		},
		{
			name:     "Root URL",
			baseURL:  "https://my-local-llm:8080",
			expected: "https://my-local-llm:8080/v1/chat/completions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewOpenAIClient("key", "model", tt.baseURL, 0)
			assert.NotNil(t, client)
		})
	}
}

func TestOpenAIClient_ListModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{
				{"id": "gpt-4", "object": "model", "created": 1234567890, "owned_by": "openai"},
				{"id": "gpt-3.5-turbo", "object": "model", "created": 1234567890, "owned_by": "openai"},
				{"id": "claude-3", "object": "model", "created": 1234567890, "owned_by": "anthropic"},
			},
		})
	}))
	defer server.Close()

	client := NewOpenAIClient("sk-test", "gpt-4", server.URL, 0)

	models, err := client.ListModels(context.Background())
	require.NoError(t, err)

	assert.Len(t, models, 2)
	assert.Equal(t, "gpt-4", models[0].ID)
	assert.Equal(t, "gpt-3.5-turbo", models[1].ID)
}

func TestOpenAIClient_Chat_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/chat/completions", r.URL.Path)
		assert.Equal(t, "Bearer sk-test", r.Header.Get("Authorization"))

		var req openaiRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, "gpt-4", req.Model)
		assert.Equal(t, 123, req.MaxCompletionTokens)
		assert.Equal(t, 0.7, req.Temperature)
		require.Len(t, req.Tools, 1)
		assert.Equal(t, "function", req.Tools[0].Type)
		assert.Equal(t, "get_time", req.Tools[0].Function.Name)
		assert.Equal(t, "auto", req.ToolChoice)

		_ = json.NewEncoder(w).Encode(openaiResponse{
			ID:    "chatcmpl-1",
			Model: "gpt-4",
			Choices: []openaiChoice{
				{
					Message:      openaiRespMsg{Role: "assistant", Content: "Hello"},
					FinishReason: "stop",
				},
			},
			Usage: openaiUsage{PromptTokens: 2, CompletionTokens: 3},
		})
	}))
	defer server.Close()

	client := NewOpenAIClient("sk-test", "gpt-4", server.URL, 0)
	resp, err := client.Chat(context.Background(), ChatRequest{
		System:      "You are helpful",
		MaxTokens:   123,
		Temperature: 0.7,
		Messages: []Message{
			{Role: "user", Content: "Hi"},
		},
		Tools: []Tool{
			{
				Name:        "get_time",
				Description: "get time",
				InputSchema: map[string]interface{}{"type": "object"},
			},
			{
				Type: "web_search_20250305",
				Name: "web_search",
			},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "Hello", resp.Content)
	assert.Equal(t, 2, resp.InputTokens)
	assert.Equal(t, 3, resp.OutputTokens)
}

func TestOpenAIClient_HelperFlags(t *testing.T) {
	client := NewOpenAIClient("sk", "gpt-4", "https://api.openai.com", 0)
	assert.True(t, client.requiresMaxCompletionTokens("o1-mini"))
	assert.False(t, client.requiresMaxCompletionTokens("gpt-4"))
}

func TestOpenAIClient_SupportsThinking(t *testing.T) {
	client := NewOpenAIClient("sk", "deepseek-reasoner", "https://api.deepseek.com", 0)
	assert.True(t, client.SupportsThinking("deepseek-reasoner"))

	client = NewOpenAIClient("sk", "gpt-4", "https://api.openai.com", 0)
	assert.False(t, client.SupportsThinking("gpt-4"))
}

func TestOpenAIClient_TestConnection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/models", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{
				{"id": "gpt-4", "object": "model", "created": 123, "owned_by": "openai"},
			},
		})
	}))
	defer server.Close()

	client := NewOpenAIClient("sk-test", "gpt-4", server.URL, 0)
	err := client.TestConnection(context.Background())
	require.NoError(t, err)
}
