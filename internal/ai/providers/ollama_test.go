package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

	client := NewOllamaClient("llama2", server.URL)

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

	client := NewOllamaClient("llama2", server.URL)

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

	client := NewOllamaClient("llama2", server.URL)

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

	client := NewOllamaClient("nonexistent", server.URL)

	ctx := context.Background()
	_, err := client.Chat(ctx, ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hello"}},
	})

	if err == nil {
		t.Error("Expected error for API failure")
	}
}

func TestOllamaClient_Chat_NetworkError(t *testing.T) {
	client := NewOllamaClient("llama2", "http://localhost:99999")

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
	client := NewOllamaClient("", server.URL)

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

	client := NewOllamaClient("default", server.URL)

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

	client := NewOllamaClient("llama2", server.URL)

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

	client := NewOllamaClient("llama2", server.URL)

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

	client := NewOllamaClient("llama2", server.URL)

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

	client := NewOllamaClient("llama2", server.URL)

	ctx := context.Background()
	_, err := client.ListModels(ctx)

	if err == nil {
		t.Error("Expected error for failed list models")
	}
}
