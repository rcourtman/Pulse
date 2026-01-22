package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOllamaClient_ChatStream_Success(t *testing.T) {
	// Mock Ollama Stream (NDJSON)
	mockResponse := `{"model":"llama3","created_at":"2023-08-04T08:52:19.385406455-07:00","message":{"role":"assistant","content":"Hello"},"done":false}
{"model":"llama3","created_at":"2023-08-04T08:52:19.385406455-07:00","message":{"role":"assistant","content":" world"},"done":false}
{"model":"llama3","created_at":"2023-08-04T08:52:19.385406455-07:00","message":{"role":"assistant","content":"!"},"done":true,"total_duration":123,"load_duration":1,"prompt_eval_count":10,"eval_count":5}
`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/chat", r.URL.Path)
		assert.Equal(t, "POST", r.Method)

		var req ollamaRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)

		assert.True(t, req.Stream)
		assert.Equal(t, "llama3", req.Model)
		assert.NotEmpty(t, req.Messages)

		w.Header().Set("Content-Type", "application/x-ndjson")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockResponse))
	}))
	defer server.Close()

	client := NewOllamaClient("llama3", server.URL, 0)

	var content string
	var done bool

	err := client.ChatStream(context.Background(), ChatRequest{
		Model: "llama3",
		Messages: []Message{
			{Role: "user", Content: "Hi"},
		},
	}, func(event StreamEvent) {
		switch event.Type {
		case "content":
			if data, ok := event.Data.(ContentEvent); ok {
				content += data.Text
			}
		case "done":
			done = true
			if data, ok := event.Data.(DoneEvent); ok {
				assert.Equal(t, 10, data.InputTokens)
				assert.Equal(t, 5, data.OutputTokens)
			}
		}
	})

	require.NoError(t, err)
	assert.Equal(t, "Hello world!", content)
	assert.True(t, done)
}

func TestOllamaClient_ChatStream_ToolCall(t *testing.T) {
	// Mock Ollama Tool Call Streaming (NDJSON)
	// Note: Ollama sends tool calls in the message object, often in one chunk or accumulated.
	mockResponse := `{"model":"llama3","message":{"role":"assistant","tool_calls":[{"function":{"name":"get_weather","arguments":{"location":"London"}}}]},"done":false}
{"model":"llama3","done":true,"done_reason":"stop"}
`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.Write([]byte(mockResponse))
	}))
	defer server.Close()

	client := NewOllamaClient("llama3", server.URL, 0)

	var toolsFound []string

	err := client.ChatStream(context.Background(), ChatRequest{
		Model:    "llama3",
		Messages: []Message{{Role: "user", Content: "Weather?"}},
	}, func(event StreamEvent) {
		if event.Type == "tool_start" {
			if data, ok := event.Data.(ToolStartEvent); ok {
				toolsFound = append(toolsFound, data.Name)
			}
		}
		if event.Type == "done" {
			if data, ok := event.Data.(DoneEvent); ok {
				assert.Equal(t, "tool_use", data.StopReason)
				assert.Len(t, data.ToolCalls, 1)
				assert.Equal(t, "get_weather", data.ToolCalls[0].Name)
			}
		}
	})

	require.NoError(t, err)
	assert.Contains(t, toolsFound, "get_weather")
}

func TestOllamaClient_ChatStream_Errors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal error"}`))
	}))
	defer server.Close()

	client := NewOllamaClient("llama3", server.URL, 0)

	err := client.ChatStream(context.Background(), ChatRequest{Messages: []Message{{Role: "user"}}}, func(e StreamEvent) {})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestOllamaClient_ListModels(t *testing.T) {
	mockResponse := `{
		"models": [
			{"name": "llama3:latest", "modified_at": "2023-11-04T00:00:00Z", "size": 4000000000},
			{"name": "mistral:7b", "modified_at": "2023-10-01T00:00:00Z", "size": 7000000000}
		]
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/tags", r.URL.Path)
		w.Write([]byte(mockResponse))
	}))
	defer server.Close()

	client := NewOllamaClient("llama3", server.URL, 0)
	models, err := client.ListModels(context.Background())

	require.NoError(t, err)
	assert.Len(t, models, 2)
	assert.Equal(t, "llama3:latest", models[0].Name)
	assert.True(t, models[0].Notable)
}

func TestNewOllamaClient_Normalization(t *testing.T) {
	tests := []struct {
		input  string
		expect string
	}{
		{"http://localhost:11434", "http://localhost:11434"},
		{"http://localhost:11434/", "http://localhost:11434"},
		{"http://localhost:11434/api", "http://localhost:11434"},
		{"http://localhost:11434/api/", "http://localhost:11434"},
	}

	for _, tt := range tests {
		client := NewOllamaClient("model", tt.input, 0)
		assert.Equal(t, tt.expect, client.baseURL)
	}
}

func TestOllamaClient_Chat_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/chat", r.URL.Path)

		var req ollamaRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.False(t, req.Stream)
		assert.Equal(t, "llama3", req.Model)
		require.Len(t, req.Tools, 1)
		assert.Equal(t, "function", req.Tools[0].Type)
		assert.Equal(t, "get_time", req.Tools[0].Function.Name)

		_ = json.NewEncoder(w).Encode(ollamaResponse{
			Model: "llama3",
			Message: ollamaMessageResp{
				Role:    "assistant",
				Content: "Hello",
				ToolCalls: []ollamaToolCall{
					{Function: ollamaFunctionCall{Name: "get_time", Arguments: map[string]interface{}{"tz": "UTC"}}},
				},
			},
			DoneReason:      "stop",
			PromptEvalCount: 2,
			EvalCount:       3,
		})
	}))
	defer server.Close()

	client := NewOllamaClient("llama3", server.URL, 0)
	resp, err := client.Chat(context.Background(), ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hi"}},
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
	assert.Equal(t, "tool_use", resp.StopReason)
	require.Len(t, resp.ToolCalls, 1)
	assert.Equal(t, "get_time", resp.ToolCalls[0].Name)
	assert.Equal(t, 2, resp.InputTokens)
	assert.Equal(t, 3, resp.OutputTokens)
}

func TestOllamaClient_TestConnection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/version", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewOllamaClient("llama3", server.URL, 0)
	err := client.TestConnection(context.Background())
	require.NoError(t, err)
}

func TestOllamaClient_SupportsThinking(t *testing.T) {
	client := NewOllamaClient("llama3", "http://localhost:11434", 0)
	if client.SupportsThinking("llama3") {
		t.Fatal("expected SupportsThinking to be false")
	}
}
