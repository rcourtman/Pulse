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

func TestNewOpenAIClient(t *testing.T) {
	// Test default baseURL and timeout
	c1 := NewOpenAIClient("key", "gpt-4o", "", 0)
	if c1.baseURL != openaiAPIURL {
		t.Errorf("Expected default baseURL, got %s", c1.baseURL)
	}
	if c1.client.Timeout != 300*time.Second {
		t.Errorf("Expected default timeout 300s, got %v", c1.client.Timeout)
	}

	// Test model prefix stripping
	c2 := NewOpenAIClient("key", "openai:gpt-4o", "", 10*time.Second)
	if c2.model != "gpt-4o" {
		t.Errorf("Expected model 'gpt-4o', got %s", c2.model)
	}
	if c2.client.Timeout != 10*time.Second {
		t.Errorf("Expected timeout 10s, got %v", c2.client.Timeout)
	}

	c3 := NewOpenAIClient("key", "deepseek:deepseek-chat", "", 0)
	if c3.model != "deepseek-chat" {
		t.Errorf("Expected model 'deepseek-chat', got %s", c3.model)
	}
}

func TestOpenAIClient_Name(t *testing.T) {
	c := &OpenAIClient{}
	if c.Name() != "openai" {
		t.Errorf("Expected Name() to be 'openai', got %s", c.Name())
	}
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

func TestOpenAIClient_Chat_DeepSeekUsesMaxTokens(t *testing.T) {
	var got map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		_ = json.NewEncoder(w).Encode(mockOpenAIResponse{
			ID:    "chatcmpl-123",
			Model: "deepseek-chat",
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

	client := NewOpenAIClient("test-api-key", "deepseek-chat", "https://api.deepseek.com/chat/completions", 0)
	serverURL, _ := url.Parse(server.URL)
	client.client.Transport = rewriteToServerTransport{serverBase: serverURL, rt: http.DefaultTransport}

	_, err := client.Chat(context.Background(), ChatRequest{
		Messages:  []Message{{Role: "user", Content: "Hello"}},
		MaxTokens: 100,
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}

	if val, ok := got["max_tokens"]; !ok || val.(float64) != 100 {
		t.Fatalf("expected max_tokens=100 for DeepSeek, got: %+v", got)
	}
	if _, ok := got["max_completion_tokens"]; ok {
		t.Fatal("did not expect max_completion_tokens for DeepSeek")
	}
}

func TestOpenAIClient_Chat_ToolCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"id":    "chatcmpl-123",
			"model": "gpt-4o",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": nil,
						"tool_calls": []map[string]interface{}{
							{
								"id":   "call_123",
								"type": "function",
								"function": map[string]interface{}{
									"name":      "get_weather",
									"arguments": `{"location":"San Francisco"}`,
								},
							},
						},
					},
					"finish_reason": "tool_calls",
				},
			},
			"usage": map[string]interface{}{"prompt_tokens": 10, "completion_tokens": 20},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewOpenAIClient("test-key", "gpt-4o", server.URL, 0)
	resp, err := client.Chat(context.Background(), ChatRequest{
		Messages: []Message{{Role: "user", Content: "weather?"}},
		Tools: []Tool{
			{
				Name:        "get_weather",
				Description: "Get weather",
				InputSchema: map[string]interface{}{"type": "object"},
			},
			{
				Type: "other", // Should be skipped
				Name: "other",
			},
		},
	})

	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if len(resp.ToolCalls) != 1 || resp.ToolCalls[0].Name != "get_weather" {
		t.Errorf("Expected 1 tool call 'get_weather', got %+v", resp.ToolCalls)
	}
	if resp.StopReason != "tool_use" {
		t.Errorf("Expected stop reason 'tool_use', got %s", resp.StopReason)
	}
}

func TestOpenAIClient_Chat_DeepSeekReasoner(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"id":    "chatcmpl-123",
			"model": "deepseek-reasoner",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":              "assistant",
						"content":           "The answer is 42.",
						"reasoning_content": "Let me think...",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]interface{}{"prompt_tokens": 10, "completion_tokens": 20},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewOpenAIClient("test-key", "deepseek-reasoner", "https://api.deepseek.com/chat/completions", 0)
	// Mock transport to point to our test server
	serverURL, _ := url.Parse(server.URL)
	client.client.Transport = rewriteToServerTransport{serverBase: serverURL, rt: http.DefaultTransport}

	resp, err := client.Chat(context.Background(), ChatRequest{
		Messages: []Message{
			{Role: "user", Content: "What is the answer?"},
			{Role: "assistant", Content: "Previous answer", ReasoningContent: "Previous reasoning", ToolCalls: []ToolCall{{ID: "tc1", Name: "tool", Input: map[string]any{"x": 1}}}},
		},
	})

	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.ReasoningContent != "Let me think..." {
		t.Errorf("Expected reasoning content, got %s", resp.ReasoningContent)
	}
}

func TestOpenAIClient_Chat_Retry(t *testing.T) {
	var count int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		if count == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":{"message":"Rate limited"}}`))
			return
		}
		json.NewEncoder(w).Encode(mockOpenAIResponse{
			ID: "chatcmpl-123",
			Choices: []struct {
				Index   int `json:"index"`
				Message struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				} `json:"message"`
				FinishReason string `json:"finish_reason"`
			}{{Message: struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			}{Role: "assistant", Content: "ok"}, FinishReason: "stop"}},
		})
	}))
	defer server.Close()

	// Shorten backoff for tests if we could, but we can't easily without modifying the code.
	// But it only retries once in our test case.
	client := NewOpenAIClient("test-key", "gpt-4o", server.URL, 0)
	ctx := context.Background()
	_, err := client.Chat(ctx, ChatRequest{Messages: []Message{{Role: "user", Content: "hi"}}})
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 attempts, got %d", count)
	}
}

func TestOpenAIClient_ModelsEndpoint_Invalid(t *testing.T) {
	client := &OpenAIClient{baseURL: "invalid-url"}
	endpoint := client.modelsEndpoint()
	if endpoint != "https://api.openai.com/v1/models" {
		t.Errorf("Expected default models endpoint for invalid baseURL, got %s", endpoint)
	}

	client = &OpenAIClient{baseURL: "https://api.deepseek.com/chat/completions"}
	endpoint = client.modelsEndpoint()
	if endpoint != "https://api.deepseek.com/models" {
		t.Errorf("Expected DeepSeek models endpoint, got %s", endpoint)
	}
}

func TestOpenAIClient_ListModels_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error"))
	}))
	defer server.Close()

	client := NewOpenAIClient("key", "model", server.URL, 0)

	if _, err := client.ListModels(context.Background()); err == nil {
		t.Error("Expected error for 500 status")
	}
}

// to derive the models URL from the baseURL, similar to how Chat works.
//
// For now, these tests would require actual API keys to run E2E tests,
// which should be done in a separate integration test suite.
