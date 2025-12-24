package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

type rewriteToServerTransport struct {
	serverBase *url.URL
	rt         http.RoundTripper
}

func (t rewriteToServerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	cloned.URL.Scheme = t.serverBase.Scheme
	cloned.URL.Host = t.serverBase.Host
	cloned.Host = t.serverBase.Host
	return t.rt.RoundTrip(cloned)
}

// Mock OpenAI API response format
type mockOpenAIResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// Mock OpenAI models response
type mockOpenAIModelsResponse struct {
	Object string `json:"object"`
	Data   []struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int64  `json:"created"`
		OwnedBy string `json:"owned_by"`
	} `json:"data"`
}

func TestOpenAIClient_Chat_Success(t *testing.T) {
	// Create a mock server that returns a successful response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("Expected /v1/chat/completions, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-api-key" {
			t.Errorf("Expected Bearer token, got %s", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected JSON content type, got %s", r.Header.Get("Content-Type"))
		}

		// Return mock response
		resp := mockOpenAIResponse{
			ID:      "chatcmpl-123",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "gpt-4o",
			Choices: []struct {
				Index   int `json:"index"`
				Message struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				} `json:"message"`
				FinishReason string `json:"finish_reason"`
			}{
				{
					Index: 0,
					Message: struct {
						Role    string `json:"role"`
						Content string `json:"content"`
					}{
						Role:    "assistant",
						Content: "Hello! I'm here to help.",
					},
					FinishReason: "stop",
				},
			},
			Usage: struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			}{
				PromptTokens:     10,
				CompletionTokens: 20,
				TotalTokens:      30,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Create client with mock server URL
	client := NewOpenAIClient("test-api-key", "gpt-4o", server.URL+"/v1/chat/completions", 0)

	// Execute chat request
	ctx := context.Background()
	resp, err := client.Chat(ctx, ChatRequest{
		Messages: []Message{
			{Role: "user", Content: "Hello"},
		},
		Model: "gpt-4o",
	})

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if resp.Content != "Hello! I'm here to help." {
		t.Errorf("Expected content 'Hello! I'm here to help.', got '%s'", resp.Content)
	}

	if resp.Model != "gpt-4o" {
		t.Errorf("Expected model 'gpt-4o', got '%s'", resp.Model)
	}

	if resp.InputTokens != 10 {
		t.Errorf("Expected 10 input tokens, got %d", resp.InputTokens)
	}

	if resp.OutputTokens != 20 {
		t.Errorf("Expected 20 output tokens, got %d", resp.OutputTokens)
	}
}

func TestOpenAIClient_Chat_APIError(t *testing.T) {
	// Create a mock server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("Expected /v1/chat/completions, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": {"message": "Invalid API key", "type": "invalid_request_error"}}`))
	}))
	defer server.Close()

	client := NewOpenAIClient("invalid-key", "gpt-4o", server.URL+"/v1/chat/completions", 0)

	ctx := context.Background()
	_, err := client.Chat(ctx, ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hello"}},
	})

	if err == nil {
		t.Error("Expected error for invalid API key")
	}
}

func TestOpenAIClient_Chat_NetworkError(t *testing.T) {
	// Create client pointing to non-existent server
	client := NewOpenAIClient("test-key", "gpt-4o", "http://localhost:99999", 0)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := client.Chat(ctx, ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hello"}},
	})

	if err == nil {
		t.Error("Expected error for network failure")
	}
}

func TestOpenAIClient_Chat_ContextCanceled(t *testing.T) {
	// Create a slow mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
	}))
	defer server.Close()

	client := NewOpenAIClient("test-key", "gpt-4o", server.URL+"/v1/chat/completions", 0)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := client.Chat(ctx, ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hello"}},
	})

	if err == nil {
		t.Error("Expected error for canceled context")
	}
}

func TestOpenAIClient_Chat_WithSystemPrompt(t *testing.T) {
	var receivedRequest map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedRequest)

		resp := mockOpenAIResponse{
			ID:    "chatcmpl-123",
			Model: "gpt-4o",
			Choices: []struct {
				Index   int `json:"index"`
				Message struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				} `json:"message"`
				FinishReason string `json:"finish_reason"`
			}{
				{Message: struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				}{Role: "assistant", Content: "Response"}, FinishReason: "stop"},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewOpenAIClient("test-key", "gpt-4o", server.URL+"/v1/chat/completions", 0)

	ctx := context.Background()
	_, err := client.Chat(ctx, ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hello"}},
		System:   "You are a helpful assistant",
	})

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify system message was included
	messages, ok := receivedRequest["messages"].([]interface{})
	if !ok || len(messages) < 2 {
		t.Error("Expected at least 2 messages (system + user)")
	}
}

func TestOpenAIClient_ListModels_UsesConfiguredHostAndFilters(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/v1/models" {
			t.Fatalf("path = %s, want /v1/models", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-api-key" {
			t.Fatalf("Authorization = %q", r.Header.Get("Authorization"))
		}

		_ = json.NewEncoder(w).Encode(mockOpenAIModelsResponse{
			Object: "list",
			Data: []struct {
				ID      string `json:"id"`
				Object  string `json:"object"`
				Created int64  `json:"created"`
				OwnedBy string `json:"owned_by"`
			}{
				{ID: "gpt-4o", Object: "model", Created: 1, OwnedBy: "openai"},
				{ID: "text-embedding-3-small", Object: "model", Created: 2, OwnedBy: "openai"},
				{ID: "o1-mini", Object: "model", Created: 3, OwnedBy: "openai"},
				{ID: "nonmatching", Object: "model", Created: 4, OwnedBy: "openai"},
			},
		})
	}))
	defer server.Close()

	client := NewOpenAIClient("test-api-key", "gpt-4o", server.URL+"/v1/chat/completions", 0)
	models, err := client.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}

	if len(models) != 2 {
		t.Fatalf("models = %d, want 2", len(models))
	}
	if models[0].ID != "gpt-4o" || models[1].ID != "o1-mini" {
		t.Fatalf("unexpected models: %+v", models)
	}
}

func TestOpenAIClient_TestConnection_CallsListModels(t *testing.T) {
	var called int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("path = %s, want /v1/models", r.URL.Path)
		}
		called++
		_ = json.NewEncoder(w).Encode(mockOpenAIModelsResponse{
			Object: "list",
			Data:   nil,
		})
	}))
	defer server.Close()

	client := NewOpenAIClient("test-api-key", "gpt-4o", server.URL+"/v1/chat/completions", 0)
	if err := client.TestConnection(context.Background()); err != nil {
		t.Fatalf("TestConnection: %v", err)
	}
	if called != 1 {
		t.Fatalf("ListModels calls = %d, want 1", called)
	}
}

func TestOpenAIClient_Chat_UsesMaxCompletionTokensForOpenAI(t *testing.T) {
	var got map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		_ = json.NewEncoder(w).Encode(mockOpenAIResponse{
			ID:    "chatcmpl-123",
			Model: "gpt-4o",
			Choices: []struct {
				Index   int `json:"index"`
				Message struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				} `json:"message"`
				FinishReason string `json:"finish_reason"`
			}{
				{Message: struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				}{Role: "assistant", Content: "ok"}, FinishReason: "stop"},
			},
		})
	}))
	defer server.Close()

	client := NewOpenAIClient("test-api-key", "gpt-4o", server.URL+"/v1/chat/completions", 0)
	_, err := client.Chat(context.Background(), ChatRequest{
		Messages:  []Message{{Role: "user", Content: "Hello"}},
		MaxTokens: 123,
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}

	if _, ok := got["max_completion_tokens"]; !ok {
		t.Fatalf("expected max_completion_tokens to be set, got: %+v", got)
	}
	if _, ok := got["max_tokens"]; ok {
		t.Fatalf("did not expect max_tokens for OpenAI, got: %+v", got)
	}
}

func TestOpenAIClient_Chat_GPT52NonChat_UsesCompletionsEndpointAndPrompt(t *testing.T) {
	var got map[string]any
	var gotPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&got)
		_ = json.NewEncoder(w).Encode(openaiResponse{
			Model: "gpt-5.2-pro",
			Choices: []openaiChoice{
				{Text: "ok", FinishReason: "stop"},
			},
		})
	}))
	defer server.Close()

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}

	client := NewOpenAIClient("test-api-key", "gpt-5.2-pro", "https://api.openai.com/v1/chat/completions", 0)
	client.client.Transport = rewriteToServerTransport{serverBase: serverURL, rt: http.DefaultTransport}

	_, err = client.Chat(context.Background(), ChatRequest{
		System:    "System prompt",
		Messages:  []Message{{Role: "user", Content: "Hello"}},
		MaxTokens: 50,
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}

	if gotPath != "/v1/completions" {
		t.Fatalf("path = %q, want /v1/completions", gotPath)
	}
	if _, ok := got["prompt"]; !ok {
		t.Fatalf("expected prompt in completions request, got: %+v", got)
	}
	if _, ok := got["messages"]; ok {
		t.Fatalf("did not expect messages in completions request, got: %+v", got)
	}
}

func TestOpenAIClient_ListModels_DeepSeekUsesModelsEndpoint(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewEncoder(w).Encode(struct {
			Data []struct {
				ID      string `json:"id"`
				Object  string `json:"object"`
				Created int64  `json:"created"`
				OwnedBy string `json:"owned_by"`
			} `json:"data"`
		}{})
	}))
	defer server.Close()

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}

	client := NewOpenAIClient("test-api-key", "deepseek-chat", "https://api.deepseek.com/v1/chat/completions", 0)
	client.client.Transport = rewriteToServerTransport{serverBase: serverURL, rt: http.DefaultTransport}

	_, err = client.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if gotPath != "/models" {
		t.Fatalf("path = %q, want /models", gotPath)
	}
}

func TestOpenAIClient_Chat_O1OmitsTemperature(t *testing.T) {
	var got map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		_ = json.NewEncoder(w).Encode(mockOpenAIResponse{
			ID:    "chatcmpl-123",
			Model: "o1-mini",
			Choices: []struct {
				Index   int `json:"index"`
				Message struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				} `json:"message"`
				FinishReason string `json:"finish_reason"`
			}{
				{Message: struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				}{Role: "assistant", Content: "ok"}, FinishReason: "stop"},
			},
		})
	}))
	defer server.Close()

	client := NewOpenAIClient("test-api-key", "o1-mini", server.URL+"/v1/chat/completions", 0)
	_, err := client.Chat(context.Background(), ChatRequest{
		Messages:    []Message{{Role: "user", Content: "Hello"}},
		Model:       "o1-mini",
		Temperature: 0.7,
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if _, ok := got["temperature"]; ok {
		t.Fatalf("did not expect temperature for o1 models, got: %+v", got)
	}
}

// to derive the models URL from the baseURL, similar to how Chat works.
//
// For now, these tests would require actual API keys to run E2E tests,
// which should be done in a separate integration test suite.
