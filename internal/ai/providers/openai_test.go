package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return fn(r)
}

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

// eofReader wraps content in a reader that returns n>0 and io.EOF simultaneously
// on the final Read, simulating servers that close the connection with buffered data.
type eofReader struct {
	data []byte
	pos  int
}

func (r *eofReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	if r.pos >= len(r.data) {
		// Return data AND EOF in same call, per io.Reader contract
		return n, io.EOF
	}
	return n, nil
}

func (r *eofReader) Close() error { return nil }

func TestOpenAIClient_ChatStream_ToolCallWithSimultaneousEOF(t *testing.T) {
	// Simulate a server that returns all SSE data and EOF in the same Read call.
	// This tests that the parser processes pendingData before breaking on EOF.
	ssePayload := "data: {\"id\":\"1\",\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call_eof\",\"type\":\"function\",\"function\":{\"name\":\"test_tool\",\"arguments\":\"{\\\"key\\\":\\\"value\\\"}\"}}]}}]}\n\n" +
		"data: {\"id\":\"1\",\"choices\":[{\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\n" +
		"data: [DONE]\n\n"

	client := &OpenAIClient{
		apiKey:  "sk-test",
		model:   "test-model",
		baseURL: "http://unused",
		client: &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: 200,
					Body:       &eofReader{data: []byte(ssePayload)},
					Header:     http.Header{"Content-Type": {"text/event-stream"}},
				}, nil
			}),
		},
	}

	var toolCalls []ToolCall
	callback := func(event StreamEvent) {
		if event.Type == "done" {
			if data, ok := event.Data.(DoneEvent); ok {
				toolCalls = data.ToolCalls
			}
		}
	}

	err := client.ChatStream(context.Background(), ChatRequest{Messages: []Message{{Role: "user", Content: "test"}}}, callback)
	require.NoError(t, err)

	require.Len(t, toolCalls, 1, "Should have 1 tool call even when EOF arrives with data")
	assert.Equal(t, "call_eof", toolCalls[0].ID)
	assert.Equal(t, "test_tool", toolCalls[0].Name)
	assert.Equal(t, map[string]interface{}{"key": "value"}, toolCalls[0].Input)
}

func TestOpenAIClient_ChatStream_ToolCallWithoutDONE(t *testing.T) {
	// Simulate a server that sends tool call deltas but closes the connection
	// without sending [DONE]. The fallback should still emit accumulated tool calls.
	ssePayload := "data: {\"id\":\"1\",\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call_nodone\",\"type\":\"function\",\"function\":{\"name\":\"my_tool\",\"arguments\":\"\"}}]}}]}\n\n" +
		"data: {\"id\":\"1\",\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"arguments\":\"{\\\"a\\\":1}\"}}]}}]}\n\n" +
		"data: {\"id\":\"1\",\"choices\":[{\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\n"
	// Note: no [DONE] event

	client := &OpenAIClient{
		apiKey:  "sk-test",
		model:   "test-model",
		baseURL: "http://unused",
		client: &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: 200,
					Body:       &eofReader{data: []byte(ssePayload)},
					Header:     http.Header{"Content-Type": {"text/event-stream"}},
				}, nil
			}),
		},
	}

	var toolCalls []ToolCall
	var stopReason string
	callback := func(event StreamEvent) {
		if event.Type == "done" {
			if data, ok := event.Data.(DoneEvent); ok {
				toolCalls = data.ToolCalls
				stopReason = data.StopReason
			}
		}
	}

	err := client.ChatStream(context.Background(), ChatRequest{Messages: []Message{{Role: "user", Content: "test"}}}, callback)
	require.NoError(t, err)

	require.Len(t, toolCalls, 1, "Should have 1 tool call even without [DONE]")
	assert.Equal(t, "call_nodone", toolCalls[0].ID)
	assert.Equal(t, "my_tool", toolCalls[0].Name)
	assert.Equal(t, map[string]interface{}{"a": float64(1)}, toolCalls[0].Input)
	assert.Equal(t, "tool_use", stopReason)
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

	assert.Len(t, models, 3)
	assert.Equal(t, "gpt-4", models[0].ID)
	assert.Equal(t, "gpt-3.5-turbo", models[1].ID)
	assert.Equal(t, "claude-3", models[2].ID)
}

func TestOpenAIClient_ListModels_OfficialEndpointStillFiltersNonChatModels(t *testing.T) {
	client := NewOpenAIClient("sk-test", "gpt-4", "", 0)
	client.client = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			assert.Equal(t, "https", r.URL.Scheme)
			assert.Equal(t, "api.openai.com", r.URL.Host)
			assert.Equal(t, "/v1/models", r.URL.Path)

			rec := httptest.NewRecorder()
			rec.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(rec).Encode(map[string]interface{}{
				"data": []map[string]interface{}{
					{"id": "gpt-4", "object": "model", "created": 1234567890, "owned_by": "openai"},
					{"id": "gpt-3.5-turbo", "object": "model", "created": 1234567890, "owned_by": "openai"},
					{"id": "claude-3", "object": "model", "created": 1234567890, "owned_by": "anthropic"},
				},
			})
			return rec.Result(), nil
		}),
	}

	models, err := client.ListModels(context.Background())
	require.NoError(t, err)

	assert.Len(t, models, 2)
	assert.Equal(t, "gpt-4", models[0].ID)
	assert.Equal(t, "gpt-3.5-turbo", models[1].ID)
}

func TestOpenAIClient_ListModels_CustomEndpointIncludesNonOpenAIModelNames(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{
				{"id": "llama3-8b", "object": "model", "created": 1234567890, "owned_by": "localai"},
				{"id": "qwen3.5-27b", "object": "model", "created": 1234567891, "owned_by": "localai"},
				{"id": "gemma-3-4b", "object": "model", "created": 1234567892, "owned_by": "localai"},
			},
		})
	}))
	defer server.Close()

	client := NewOpenAIClient("sk-test", "llama3-8b", server.URL+"/custom-openai", 0)

	models, err := client.ListModels(context.Background())
	require.NoError(t, err)

	assert.Len(t, models, 3)
	assert.Equal(t, "llama3-8b", models[0].ID)
	assert.Equal(t, "qwen3.5-27b", models[1].ID)
	assert.Equal(t, "gemma-3-4b", models[2].ID)
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
