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

func TestNewGeminiClient(t *testing.T) {
	t.Parallel()

	client := NewGeminiClient("test-api-key", "gemini-pro", "", 0)
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if client.apiKey != "test-api-key" {
		t.Errorf("expected apiKey 'test-api-key', got %q", client.apiKey)
	}
	if client.model != "gemini-pro" {
		t.Errorf("expected model 'gemini-pro', got %q", client.model)
	}
	if client.baseURL != geminiAPIURL {
		t.Errorf("expected default baseURL, got %q", client.baseURL)
	}
}

func TestNewGeminiClient_StripPrefix(t *testing.T) {
	t.Parallel()

	client := NewGeminiClient("api-key", "gemini:gemini-1.5-pro", "", 0)
	if client.model != "gemini-1.5-pro" {
		t.Errorf("expected model with prefix stripped, got %q", client.model)
	}
}

func TestNewGeminiClient_CustomBaseURL(t *testing.T) {
	t.Parallel()

	client := NewGeminiClient("api-key", "gemini-pro", "https://custom.api.example.com", 0)
	if client.baseURL != "https://custom.api.example.com" {
		t.Errorf("expected custom baseURL, got %q", client.baseURL)
	}
}

func TestGeminiClient_Name(t *testing.T) {
	t.Parallel()

	client := NewGeminiClient("key", "model", "", 0)
	if client.Name() != "gemini" {
		t.Errorf("expected Name() to return 'gemini', got %q", client.Name())
	}
}

func TestGeminiClient_Chat_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "generateContent") {
			t.Errorf("expected generateContent in path, got %s", r.URL.Path)
		}

		resp := geminiResponse{
			Candidates: []geminiCandidate{
				{
					Content: geminiContent{
						Parts: []geminiPart{
							{Text: "Hello! I'm Gemini."},
						},
					},
					FinishReason: "STOP",
				},
			},
			UsageMetadata: &geminiUsageMetadata{
				PromptTokenCount:     10,
				CandidatesTokenCount: 20,
				TotalTokenCount:      30,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewGeminiClient("test-key", "gemini-pro", server.URL, 0)

	ctx := context.Background()
	resp, err := client.Chat(ctx, ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hello"}},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Content != "Hello! I'm Gemini." {
		t.Errorf("expected content 'Hello! I'm Gemini.', got %q", resp.Content)
	}

	if resp.InputTokens != 10 {
		t.Errorf("expected 10 input tokens, got %d", resp.InputTokens)
	}

	if resp.OutputTokens != 20 {
		t.Errorf("expected 20 output tokens, got %d", resp.OutputTokens)
	}

	if resp.StopReason != "end_turn" {
		t.Errorf("expected stop reason 'end_turn', got %q", resp.StopReason)
	}
}

func TestGeminiClient_Chat_WithSystemPrompt(t *testing.T) {
	var receivedReq geminiRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedReq)

		resp := geminiResponse{
			Candidates: []geminiCandidate{
				{
					Content: geminiContent{
						Parts: []geminiPart{{Text: "Response"}},
					},
					FinishReason: "STOP",
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewGeminiClient("test-key", "gemini-pro", server.URL, 0)

	ctx := context.Background()
	_, err := client.Chat(ctx, ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hello"}},
		System:   "You are a helpful assistant",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedReq.SystemInstruction == nil {
		t.Fatal("expected system instruction to be set")
	}
	if len(receivedReq.SystemInstruction.Parts) == 0 {
		t.Fatal("expected system instruction parts")
	}
	if receivedReq.SystemInstruction.Parts[0].Text != "You are a helpful assistant" {
		t.Errorf("expected system instruction text, got %q", receivedReq.SystemInstruction.Parts[0].Text)
	}
}

func TestGeminiClient_Chat_WithMaxTokens(t *testing.T) {
	var receivedReq geminiRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedReq)

		resp := geminiResponse{
			Candidates: []geminiCandidate{
				{
					Content: geminiContent{
						Parts: []geminiPart{{Text: "Response"}},
					},
					FinishReason: "STOP",
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewGeminiClient("test-key", "gemini-pro", server.URL, 0)

	ctx := context.Background()
	_, err := client.Chat(ctx, ChatRequest{
		Messages:  []Message{{Role: "user", Content: "Hello"}},
		MaxTokens: 1024,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedReq.GenerationConfig == nil {
		t.Fatal("expected generation config to be set")
	}
	if receivedReq.GenerationConfig.MaxOutputTokens != 1024 {
		t.Errorf("expected max tokens 1024, got %d", receivedReq.GenerationConfig.MaxOutputTokens)
	}
}

func TestGeminiClient_Chat_ToolCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := geminiResponse{
			Candidates: []geminiCandidate{
				{
					Content: geminiContent{
						Parts: []geminiPart{
							{
								FunctionCall: &geminiFunctionCall{
									Name: "get_weather",
									Args: map[string]interface{}{"location": "NYC"},
								},
							},
						},
					},
					FinishReason: "STOP",
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewGeminiClient("test-key", "gemini-pro", server.URL, 0)

	ctx := context.Background()
	resp, err := client.Chat(ctx, ChatRequest{
		Messages: []Message{{Role: "user", Content: "What's the weather in NYC?"}},
		Tools: []Tool{
			{
				Type:        "function",
				Name:        "get_weather",
				Description: "Get weather info",
			},
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StopReason != "tool_use" {
		t.Errorf("expected stop reason 'tool_use', got %q", resp.StopReason)
	}

	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}

	if resp.ToolCalls[0].Name != "get_weather" {
		t.Errorf("expected tool name 'get_weather', got %q", resp.ToolCalls[0].Name)
	}
}

func TestGeminiClient_Chat_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(geminiError{
			Error: struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
				Status  string `json:"status"`
			}{
				Code:    401,
				Message: "Invalid API key",
				Status:  "UNAUTHENTICATED",
			},
		})
	}))
	defer server.Close()

	client := NewGeminiClient("invalid-key", "gemini-pro", server.URL, 0)

	ctx := context.Background()
	_, err := client.Chat(ctx, ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hello"}},
	})

	if err == nil {
		t.Error("expected error for invalid API key")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("expected error to contain status code, got %v", err)
	}
}

func TestGeminiClient_Chat_NoCandidates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := geminiResponse{
			Candidates: []geminiCandidate{},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewGeminiClient("test-key", "gemini-pro", server.URL, 0)

	ctx := context.Background()
	_, err := client.Chat(ctx, ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hello"}},
	})

	if err == nil {
		t.Error("expected error for no candidates")
	}
	if !strings.Contains(err.Error(), "no response candidates") {
		t.Errorf("expected 'no response candidates' error, got %v", err)
	}
}

func TestGeminiClient_Chat_SafetyBlocked(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := geminiResponse{
			Candidates: []geminiCandidate{
				{
					Content:      geminiContent{Parts: []geminiPart{}},
					FinishReason: "SAFETY",
					SafetyRatings: []geminySafety{
						{Category: "HARM_CATEGORY_DANGEROUS", Probability: "HIGH", Blocked: true},
					},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewGeminiClient("test-key", "gemini-pro", server.URL, 0)

	ctx := context.Background()
	_, err := client.Chat(ctx, ChatRequest{
		Messages: []Message{{Role: "user", Content: "Something dangerous"}},
	})

	if err == nil {
		t.Error("expected error for safety blocked content")
	}
	if !strings.Contains(err.Error(), "safety") {
		t.Errorf("expected 'safety' in error, got %v", err)
	}
}

func TestGeminiClient_Chat_PromptBlocked(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := geminiResponse{
			PromptFeedback: &geminiPromptFeedback{
				BlockReason: "SAFETY",
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewGeminiClient("test-key", "gemini-pro", server.URL, 0)

	ctx := context.Background()
	_, err := client.Chat(ctx, ChatRequest{
		Messages: []Message{{Role: "user", Content: "Blocked prompt"}},
	})

	if err == nil {
		t.Error("expected error for blocked prompt")
	}
	if !strings.Contains(err.Error(), "blocked by Gemini") {
		t.Errorf("expected 'blocked by Gemini' error, got %v", err)
	}
}

func TestGeminiClient_ListModels_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/models") {
			t.Errorf("expected /models in path, got %s", r.URL.Path)
		}

		resp := struct {
			Models []struct {
				Name                       string   `json:"name"`
				DisplayName                string   `json:"displayName"`
				Description                string   `json:"description"`
				SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
			} `json:"models"`
		}{
			Models: []struct {
				Name                       string   `json:"name"`
				DisplayName                string   `json:"displayName"`
				Description                string   `json:"description"`
				SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
			}{
				{
					Name:                       "models/gemini-1.5-pro",
					DisplayName:                "Gemini 1.5 Pro",
					Description:                "Advanced language model",
					SupportedGenerationMethods: []string{"generateContent"},
				},
				{
					Name:                       "models/gemini-1.5-flash",
					DisplayName:                "Gemini 1.5 Flash",
					Description:                "Fast model",
					SupportedGenerationMethods: []string{"generateContent"},
				},
				{
					Name:                       "models/text-embedding-004", // Should be filtered
					DisplayName:                "Text Embedding",
					Description:                "Embedding model",
					SupportedGenerationMethods: []string{"embedContent"},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewGeminiClient("test-key", "gemini-pro", server.URL, 0)

	ctx := context.Background()
	models, err := client.ListModels(ctx)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should only include gemini-1.5-* models, not embedding
	if len(models) != 2 {
		t.Errorf("expected 2 models, got %d", len(models))
	}
}

func TestGeminiClient_ListModels_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	client := NewGeminiClient("test-key", "gemini-pro", server.URL, 0)

	ctx := context.Background()
	_, err := client.ListModels(ctx)

	if err == nil {
		t.Error("expected error for server error")
	}
}

func TestGeminiClient_TestConnection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := struct {
			Models []interface{} `json:"models"`
		}{Models: []interface{}{}}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewGeminiClient("test-key", "gemini-pro", server.URL, 0)

	ctx := context.Background()
	err := client.TestConnection(ctx)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGeminiClient_Chat_NetworkError(t *testing.T) {
	client := NewGeminiClient("test-key", "gemini-pro", "http://localhost:99999", 0)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := client.Chat(ctx, ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hello"}},
	})

	if err == nil {
		t.Error("expected error for network failure")
	}
}

func TestGeminiClient_Chat_RoleConversion(t *testing.T) {
	var receivedReq geminiRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedReq)

		resp := geminiResponse{
			Candidates: []geminiCandidate{
				{
					Content:      geminiContent{Parts: []geminiPart{{Text: "Ok"}}},
					FinishReason: "STOP",
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewGeminiClient("test-key", "gemini-pro", server.URL, 0)

	ctx := context.Background()
	_, err := client.Chat(ctx, ChatRequest{
		Messages: []Message{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi there"},
			{Role: "user", Content: "How are you?"},
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify role conversion: assistant -> model
	if len(receivedReq.Contents) != 3 {
		t.Fatalf("expected 3 contents, got %d", len(receivedReq.Contents))
	}

	if receivedReq.Contents[0].Role != "user" {
		t.Errorf("expected first role 'user', got %q", receivedReq.Contents[0].Role)
	}
	if receivedReq.Contents[1].Role != "model" {
		t.Errorf("expected second role 'model', got %q", receivedReq.Contents[1].Role)
	}
	if receivedReq.Contents[2].Role != "user" {
		t.Errorf("expected third role 'user', got %q", receivedReq.Contents[2].Role)
	}
}
