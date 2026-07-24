package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// openAICompatibilityFixture models the minimum protocol shared by llama.cpp,
// LocalAI, LM Studio, and other OpenAI-compatible servers. Individual tests
// tighten or remove optional capabilities without changing the core fixture.
type openAICompatibilityFixture struct {
	mu       sync.Mutex
	requests []map[string]interface{}
	headers  []http.Header
}

func (f *openAICompatibilityFixture) capture(r *http.Request) (map[string]interface{}, error) {
	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, body)
	f.headers = append(f.headers, r.Header.Clone())
	return body, nil
}

func TestOpenAICompatibleKeylessFixtureListsOpaqueModelsAndUsesPortableRequestShape(t *testing.T) {
	fixture := &openAICompatibilityFixture{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Empty(t, r.Header.Get("Authorization"))
		switch r.URL.Path {
		case "/v1/models":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data": []map[string]interface{}{
					{"id": "HauhauCS/Qwen3.6-27B-Uncensored-HauhauCS-Balanced-Q5_K_P"},
					{"id": "local-model-without-known-prefix"},
				},
			})
		case "/v1/chat/completions":
			body, err := fixture.capture(r)
			require.NoError(t, err)
			require.Equal(t, float64(128), body["max_tokens"])
			require.NotContains(t, body, "max_completion_tokens")
			require.NotContains(t, body, "stream_options")
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"ok\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewOpenAICompatibleClient("openai", "", "local-model-without-known-prefix", server.URL, time.Second)
	models, err := client.ListModels(context.Background())
	require.NoError(t, err)
	require.Len(t, models, 2)
	require.Equal(t, "HauhauCS/Qwen3.6-27B-Uncensored-HauhauCS-Balanced-Q5_K_P", models[0].ID)
	require.Equal(t, "local-model-without-known-prefix", models[1].ID)

	var content string
	err = client.ChatStream(context.Background(), ChatRequest{
		Messages:  []Message{{Role: "user", Content: "hello"}},
		MaxTokens: 128,
	}, func(event StreamEvent) {
		if event.Type == "content" {
			content += event.Data.(ContentEvent).Text
		}
	})
	require.NoError(t, err)
	require.Equal(t, "ok", content)
}

func TestOpenAICompatibleStreamingUnsupportedFallsBackToValidatedBufferedTools(t *testing.T) {
	fixture := &openAICompatibilityFixture{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := fixture.capture(r)
		require.NoError(t, err)
		if body["stream"] == true {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"error":{"message":"streaming is not supported; stream must be false"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"model":"fixture-model",
			"choices":[{
				"message":{
					"role":"assistant",
					"reasoning_content":"checked schema",
					"tool_calls":[{
						"id":"call-1",
						"type":"function",
						"function":{"name":"inspect_resource","arguments":"{\"resource_id\":\"vm-101\"}"}
					}]
				},
				"finish_reason":"tool_calls"
			}],
			"usage":{"prompt_tokens":5,"completion_tokens":7}
		}`))
	}))
	defer server.Close()

	client := NewOpenAICompatibleClient("openai", "", "fixture-model", server.URL, time.Second)
	var eventTypes []string
	var done DoneEvent
	err := client.ChatStream(context.Background(), ChatRequest{
		Messages: []Message{{Role: "user", Content: "inspect"}},
		Tools: []Tool{{
			Name:        "inspect_resource",
			InputSchema: map[string]interface{}{"type": "object"},
		}},
	}, func(event StreamEvent) {
		eventTypes = append(eventTypes, event.Type)
		if event.Type == "done" {
			done = event.Data.(DoneEvent)
		}
	})
	require.NoError(t, err)
	require.Equal(t, []string{"thinking", "tool_start", "done"}, eventTypes)
	require.Len(t, done.ToolCalls, 1)
	require.Equal(t, "vm-101", done.ToolCalls[0].Input["resource_id"])
	require.Equal(t, 2, len(fixture.requests))
	require.Equal(t, true, fixture.requests[0]["stream"])
	require.NotContains(t, fixture.requests[1], "stream")
}

func TestOpenAICompatibleServerMayReturnBufferedJSONForStreamRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write([]byte(`{
			"model":"fixture-model",
			"choices":[{"message":{"role":"assistant","content":"buffered answer"},"finish_reason":"stop"}],
			"usage":{"prompt_tokens":2,"completion_tokens":3}
		}`))
	}))
	defer server.Close()

	client := NewOpenAICompatibleClient("openai", "", "fixture-model", server.URL, time.Second)
	var content string
	var done bool
	err := client.ChatStream(context.Background(), ChatRequest{
		Messages: []Message{{Role: "user", Content: "hello"}},
	}, func(event StreamEvent) {
		switch event.Type {
		case "content":
			content += event.Data.(ContentEvent).Text
		case "done":
			done = true
		}
	})
	require.NoError(t, err)
	require.Equal(t, "buffered answer", content)
	require.True(t, done)
}

func TestOpenAICompatibleMalformedBufferedResponseEmitsNothing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"fixture-model","choices":[]}`))
	}))
	defer server.Close()

	client := NewOpenAICompatibleClient("openai", "", "fixture-model", server.URL, time.Second)
	var eventCount int
	err := client.ChatStream(context.Background(), ChatRequest{
		Messages: []Message{{Role: "user", Content: "hello"}},
	}, func(StreamEvent) { eventCount++ })
	require.Error(t, err)
	require.Contains(t, err.Error(), "no response choices")
	require.Zero(t, eventCount)
}

func TestOpenAICompatibleRestrictedClientBlocksMetadataService(t *testing.T) {
	client := NewOpenAICompatibleClient("openai", "", "fixture-model", "http://169.254.169.254/v1", 100*time.Millisecond)
	_, err := client.ListModels(context.Background())
	require.Error(t, err)
	require.True(t,
		strings.Contains(err.Error(), "metadata service") || strings.Contains(err.Error(), "link-local"),
		err.Error(),
	)
}

func TestOpenAICompatibleConnectionKeepsTransportHealthSeparateFromSelectedModel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"id":"available-model"}]}`))
	}))
	defer server.Close()

	client := NewOpenAICompatibleClient("openai", "", "missing-model", server.URL, time.Second)
	err := client.TestConnection(context.Background())
	require.NoError(t, err)
}

func TestOpenAICompatibleMalformedBufferedToolArgumentsEmitNothing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"model":"fixture-model",
			"choices":[{
				"message":{"tool_calls":[{
					"id":"call-1",
					"type":"function",
					"function":{"name":"inspect_resource","arguments":"{\"resource_id\":"}
				}]},
				"finish_reason":"tool_calls"
			}]
		}`))
	}))
	defer server.Close()

	client := NewOpenAICompatibleClient("openai", "", "fixture-model", server.URL, time.Second)
	var eventCount int
	err := client.ChatStream(context.Background(), ChatRequest{
		Messages: []Message{{Role: "user", Content: "inspect"}},
		Tools: []Tool{{
			Name:        "inspect_resource",
			InputSchema: map[string]interface{}{"type": "object"},
		}},
	}, func(StreamEvent) { eventCount++ })
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid arguments")
	require.Zero(t, eventCount)
}
