package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestOllamaClient(t *testing.T, model, baseURL, username, password string, timeout time.Duration) *OllamaClient {
	t.Helper()
	client, err := NewOllamaClient(model, baseURL, username, password, timeout)
	require.NoError(t, err)
	return client
}

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
		// #1425: Pulse must pass keep_alive so the model unloads shortly
		// after the request burst ends instead of refreshing Ollama's
		// 5-minute default TTL on every call.
		assert.Equal(t, config.DefaultOllamaKeepAlive, req.KeepAlive)

		w.Header().Set("Content-Type", "application/x-ndjson")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockResponse))
	}))
	defer server.Close()

	client := newTestOllamaClient(t, "llama3", server.URL, "", "", 0)

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

func TestOllamaClient_ChatStream_Thinking(t *testing.T) {
	// Thinking models (e.g. qwen3) stream reasoning in message.thinking
	// before any content arrives — Pulse must surface it live.
	mockResponse := `{"model":"qwen3:8b","message":{"role":"assistant","content":"","thinking":"Let me"},"done":false}
{"model":"qwen3:8b","message":{"role":"assistant","content":"","thinking":" think."},"done":false}
{"model":"qwen3:8b","message":{"role":"assistant","content":"Answer"},"done":false}
{"model":"qwen3:8b","message":{"role":"assistant","content":""},"done":true,"done_reason":"stop","prompt_eval_count":10,"eval_count":5}
`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.Write([]byte(mockResponse))
	}))
	defer server.Close()

	client := newTestOllamaClient(t, "qwen3:8b", server.URL, "", "", 0)

	var thinking, content string
	var events []string

	err := client.ChatStream(context.Background(), ChatRequest{
		Model:    "qwen3:8b",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	}, func(event StreamEvent) {
		events = append(events, event.Type)
		switch event.Type {
		case "thinking":
			if data, ok := event.Data.(ThinkingEvent); ok {
				thinking += data.Text
			}
		case "content":
			if data, ok := event.Data.(ContentEvent); ok {
				content += data.Text
			}
		}
	})

	require.NoError(t, err)
	assert.Equal(t, "Let me think.", thinking)
	assert.Equal(t, "Answer", content)
	assert.Equal(t, []string{"thinking", "thinking", "content", "done"}, events)
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

	client := newTestOllamaClient(t, "llama3", server.URL, "", "", 0)

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

	client := newTestOllamaClient(t, "llama3", server.URL, "", "", 0)

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

	client := newTestOllamaClient(t, "llama3", server.URL, "", "", 0)
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
		client := newTestOllamaClient(t, "model", tt.input, "", "", 0)
		assert.Equal(t, tt.expect, client.baseURL)
	}
}

func TestNewOllamaClient_RejectsCredentialedBaseURL(t *testing.T) {
	_, err := NewOllamaClient("llama3", "http://user:pass@localhost:11434", "", "", 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "userinfo is not allowed")
}

func TestOllamaClient_Chat_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/chat", r.URL.Path)

		var req ollamaRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.False(t, req.Stream)
		assert.Equal(t, "llama3", req.Model)
		// #1425: keep_alive must be set on non-streaming Chat too.
		assert.Equal(t, config.DefaultOllamaKeepAlive, req.KeepAlive)
		require.Len(t, req.Tools, 1)
		assert.Equal(t, "function", req.Tools[0].Type)
		assert.Equal(t, "get_time", req.Tools[0].Function.Name)

		_ = json.NewEncoder(w).Encode(ollamaResponse{
			Model: "llama3",
			Message: ollamaMessageResp{
				Role:     "assistant",
				Content:  "Hello",
				Thinking: "pondering",
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

	client := newTestOllamaClient(t, "llama3", server.URL, "", "", 0)
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
	assert.Equal(t, "pondering", resp.ReasoningContent)
	assert.Equal(t, "tool_use", resp.StopReason)
	require.Len(t, resp.ToolCalls, 1)
	assert.Equal(t, "get_time", resp.ToolCalls[0].Name)
	assert.Equal(t, 2, resp.InputTokens)
	assert.Equal(t, 3, resp.OutputTokens)
}

func TestOllamaClient_Chat_UsesConfiguredKeepAlive(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ollamaRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, "24h", req.KeepAlive)

		_ = json.NewEncoder(w).Encode(ollamaResponse{
			Model:   "llama3",
			Message: ollamaMessageResp{Role: "assistant", Content: "Hello"},
		})
	}))
	defer server.Close()

	client, err := NewOllamaClientWithKeepAlive("llama3", server.URL, "", "", "24h", 0)
	require.NoError(t, err)

	_, err = client.Chat(context.Background(), ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	require.NoError(t, err)
}

func TestOllamaClient_Chat_OmitsKeepAliveForServerDefault(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var raw map[string]json.RawMessage
		require.NoError(t, json.NewDecoder(r.Body).Decode(&raw))
		assert.NotContains(t, raw, "keep_alive")

		_ = json.NewEncoder(w).Encode(ollamaResponse{
			Model:   "llama3",
			Message: ollamaMessageResp{Role: "assistant", Content: "Hello"},
		})
	}))
	defer server.Close()

	client, err := NewOllamaClientWithKeepAlive("llama3", server.URL, "", "", "", 0)
	require.NoError(t, err)

	_, err = client.Chat(context.Background(), ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	require.NoError(t, err)
}

func TestOllamaClient_Chat_EncodesNumericKeepAlive(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var raw map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&raw))
		assert.Equal(t, float64(0), raw["keep_alive"])

		_ = json.NewEncoder(w).Encode(ollamaResponse{
			Model:   "llama3",
			Message: ollamaMessageResp{Role: "assistant", Content: "Hello"},
		})
	}))
	defer server.Close()

	client, err := NewOllamaClientWithKeepAlive("llama3", server.URL, "", "", "0", 0)
	require.NoError(t, err)

	_, err = client.Chat(context.Background(), ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	require.NoError(t, err)
}

func TestOllamaClient_TestConnection(t *testing.T) {
	versionHits := 0
	tagsHits := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		assert.True(t, ok)
		assert.Equal(t, "unai", username)
		assert.Equal(t, "secret", password)
		switch r.URL.Path {
		case "/api/version":
			versionHits++
			_ = json.NewEncoder(w).Encode(map[string]any{"version": "0.1.0"})
		case "/api/tags":
			tagsHits++
			_ = json.NewEncoder(w).Encode(map[string]any{
				"models": []map[string]any{{"name": "llama3:latest"}},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newTestOllamaClient(t, "llama3", server.URL, "unai", "secret", 0)
	err := client.TestConnection(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, versionHits)
	assert.Equal(t, 1, tagsHits)
}

func TestOllamaClient_SupportsThinking(t *testing.T) {
	client := newTestOllamaClient(t, "qwen3:8b", "http://localhost:11434", "", "", 0)
	if !client.SupportsThinking("qwen3:8b") {
		t.Fatal("expected SupportsThinking to be true")
	}
}

func TestOllamaClient_TestConnection_BlocksMetadataServiceHost(t *testing.T) {
	client := newTestOllamaClient(t, "llama3", "http://169.254.169.254:11434", "", "", 0)

	err := client.TestConnection(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "metadata service address is not allowed")
}

func TestOllamaClient_TestConnection_ModelUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/version":
			_ = json.NewEncoder(w).Encode(map[string]any{"version": "0.1.0"})
		case "/api/tags":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"models": []map[string]any{{"name": "mistral:latest"}},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newTestOllamaClient(t, "llama3", server.URL, "", "", 0)
	err := client.TestConnection(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), `model "llama3" is not available`)
}
