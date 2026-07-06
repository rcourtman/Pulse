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

func TestGeminiClient_ChatStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "streamGenerateContent") {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		events := []string{
			`{"candidates":[{"content":{"parts":[{"text":"Hello"}]}}],"usageMetadata":{"promptTokenCount":2,"candidatesTokenCount":3}}`,
			`{"candidates":[{"content":{"parts":[{"functionCall":{"name":"get_time","args":{"tz":"UTC"}}}]},"finishReason":"STOP"}]}`,
		}

		for _, event := range events {
			w.Write([]byte("data: " + event + "\n\n"))
			w.(http.Flusher).Flush()
		}
	}))
	defer server.Close()

	client := NewGeminiClient("test-key", "gemini-pro", server.URL, 0)

	var content string
	var done DoneEvent
	var doneCalled bool
	var toolStarts int

	err := client.ChatStream(context.Background(), ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hi"}},
	}, func(event StreamEvent) {
		switch event.Type {
		case "content":
			if data, ok := event.Data.(ContentEvent); ok {
				content += data.Text
			}
		case "tool_start":
			toolStarts++
		case "done":
			if data, ok := event.Data.(DoneEvent); ok {
				done = data
				doneCalled = true
			}
		}
	})

	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
	if content != "Hello" {
		t.Fatalf("content = %q", content)
	}
	if toolStarts != 1 {
		t.Fatalf("toolStarts = %d, want 1", toolStarts)
	}
	if !doneCalled {
		t.Fatalf("done event not called")
	}
	if done.StopReason != "tool_use" || len(done.ToolCalls) != 1 {
		t.Fatalf("unexpected done: %+v", done)
	}
}

func TestGeminiClient_SupportsThinking(t *testing.T) {
	client := NewGeminiClient("key", "gemini-pro", "", 0)
	if client.SupportsThinking("gemini-pro") {
		t.Fatal("expected SupportsThinking to be false")
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
		_ = json.NewDecoder(r.Body).Decode(&receivedReq)

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
		_ = json.NewDecoder(r.Body).Decode(&receivedReq)

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
									ID:   "gemini-call-weather",
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
	if resp.ToolCalls[0].ID != "gemini-call-weather" {
		t.Errorf("expected provider tool id to be preserved, got %q", resp.ToolCalls[0].ID)
	}
}

func TestGeminiClient_Chat_ToolChoiceRequiredUsesAnyAndSanitizesSchema(t *testing.T) {
	var receivedReq geminiRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&receivedReq)
		resp := geminiResponse{
			Candidates: []geminiCandidate{
				{
					Content: geminiContent{
						Parts: []geminiPart{{
							FunctionCall: &geminiFunctionCall{
								ID:   "call-verify-1",
								Name: "verify_pulse_patrol",
								Args: map[string]interface{}{"ok": true},
							},
						}},
					},
					FinishReason: "STOP",
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewGeminiClient("test-key", "gemini-pro", server.URL, 0)

	resp, err := client.Chat(context.Background(), ChatRequest{
		Messages: []Message{{Role: "user", Content: "Run the self-test"}},
		Tools: []Tool{{
			Name:        "verify_pulse_patrol",
			Description: "Verify Patrol tool calling",
			InputSchema: map[string]interface{}{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]interface{}{
					"ok": map[string]interface{}{
						"type":                 "boolean",
						"description":          "Always true.",
						"additionalProperties": false,
					},
				},
				"required": []string{"ok"},
			},
		}},
		ToolChoice: &ToolChoice{Type: ToolChoiceRequired},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedReq.ToolConfig == nil || receivedReq.ToolConfig.FunctionCallingConfig == nil {
		t.Fatalf("expected Gemini toolConfig in required mode, got %+v", receivedReq.ToolConfig)
	}
	if receivedReq.ToolConfig.FunctionCallingConfig.Mode != "ANY" {
		t.Fatalf("Gemini function calling mode = %q, want ANY", receivedReq.ToolConfig.FunctionCallingConfig.Mode)
	}
	if len(receivedReq.Tools) != 1 || len(receivedReq.Tools[0].FunctionDeclarations) != 1 {
		t.Fatalf("expected one Gemini function declaration, got %+v", receivedReq.Tools)
	}
	params := receivedReq.Tools[0].FunctionDeclarations[0].Parameters
	if hasMapKeyDeep(params, "additionalProperties") {
		t.Fatalf("Gemini function parameters leaked unsupported additionalProperties: %#v", params)
	}
	if len(resp.ToolCalls) != 1 || resp.ToolCalls[0].ID != "call-verify-1" {
		t.Fatalf("expected provider function-call id to survive response parsing, got %+v", resp.ToolCalls)
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
		_ = json.NewDecoder(r.Body).Decode(&receivedReq)

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
func TestGeminiClient_Chat_ToolResultsAndAssistantToolCalls(t *testing.T) {
	var got geminiRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		resp := geminiResponse{
			Candidates: []geminiCandidate{{Content: geminiContent{Parts: []geminiPart{{Text: "Ok"}}}, FinishReason: "STOP"}},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewGeminiClient("test-key", "gemini-pro", server.URL, 0)
	_, err := client.Chat(context.Background(), ChatRequest{
		Messages: []Message{
			{Role: "assistant", Content: "Calling tool", ToolCalls: []ToolCall{{ID: "tc1", Name: "get_time", Input: map[string]any{"tz": "UTC"}}}},
			{Role: "user", ToolResult: &ToolResult{ToolUseID: "tc1", Content: "{\"time\":\"00:00\"}"}},
		},
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}

	if len(got.Contents) != 2 {
		t.Fatalf("Expected 2 contents, got %d", len(got.Contents))
	}
	// Check assistant tool call
	if got.Contents[0].Role != "model" || got.Contents[0].Parts[1].FunctionCall == nil {
		t.Errorf("Expected model role with function call, got %+v", got.Contents[0])
	}
	if got.Contents[0].Parts[1].FunctionCall.ID != "tc1" {
		t.Errorf("Expected model function call id tc1, got %+v", got.Contents[0].Parts[1].FunctionCall)
	}
	// Check tool result
	if got.Contents[1].Role != "user" || got.Contents[1].Parts[0].FunctionResponse == nil {
		t.Errorf("Expected user role with function response, got %+v", got.Contents[1])
	}
	if got.Contents[1].Parts[0].FunctionResponse.ID != "tc1" {
		t.Errorf("Expected function response id tc1, got %+v", got.Contents[1].Parts[0].FunctionResponse)
	}
	if got.Contents[1].Parts[0].FunctionResponse.Name != "get_time" {
		t.Errorf("Expected function response name get_time, got %+v", got.Contents[1].Parts[0].FunctionResponse)
	}
}

func TestGeminiClient_Chat_ResolvesAndGroupsToolResults(t *testing.T) {
	var got geminiRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		_ = json.NewEncoder(w).Encode(geminiResponse{
			Candidates: []geminiCandidate{{Content: geminiContent{Parts: []geminiPart{{Text: "Ok"}}}, FinishReason: "STOP"}},
		})
	}))
	defer server.Close()

	client := NewGeminiClient("test-key", "gemini-pro", server.URL, 0)
	_, err := client.Chat(context.Background(), ChatRequest{
		Messages: []Message{
			{
				Role: "assistant",
				ToolCalls: []ToolCall{
					{ID: "call-time", Name: "get_time", Input: map[string]any{"tz": "UTC"}},
					{ID: "call-weather", Name: "get_weather", Input: map[string]any{"location": "NYC"}},
				},
			},
			{Role: "user", ToolResult: &ToolResult{ToolUseID: "call-time", Content: "{\"time\":\"00:00\"}"}},
			{Role: "user", ToolResult: &ToolResult{ToolUseID: "call-weather", Content: "{\"temp\":72}"}},
		},
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}

	if len(got.Contents) != 2 {
		t.Fatalf("contents length = %d, want grouped assistant call plus tool result response", len(got.Contents))
	}
	if got.Contents[0].Role != "model" || len(got.Contents[0].Parts) != 2 {
		t.Fatalf("expected model content with two function calls, got %+v", got.Contents[0])
	}
	resultParts := got.Contents[1].Parts
	if got.Contents[1].Role != "user" || len(resultParts) != 2 {
		t.Fatalf("expected one grouped user content with two function responses, got %+v", got.Contents[1])
	}
	if resultParts[0].FunctionResponse == nil || resultParts[0].FunctionResponse.Name != "get_time" {
		t.Fatalf("first function response = %+v, want get_time", resultParts[0].FunctionResponse)
	}
	if resultParts[0].FunctionResponse.ID != "call-time" {
		t.Fatalf("first function response id = %q, want call-time", resultParts[0].FunctionResponse.ID)
	}
	if resultParts[1].FunctionResponse == nil || resultParts[1].FunctionResponse.Name != "get_weather" {
		t.Fatalf("second function response = %+v, want get_weather", resultParts[1].FunctionResponse)
	}
	if resultParts[1].FunctionResponse.ID != "call-weather" {
		t.Fatalf("second function response id = %q, want call-weather", resultParts[1].FunctionResponse.ID)
	}
}

func hasMapKeyDeep(value interface{}, key string) bool {
	switch v := value.(type) {
	case map[string]interface{}:
		for k, item := range v {
			if k == key || hasMapKeyDeep(item, key) {
				return true
			}
		}
	case []interface{}:
		for _, item := range v {
			if hasMapKeyDeep(item, key) {
				return true
			}
		}
	}
	return false
}

func TestGeminiClient_Chat_Retry(t *testing.T) {
	var count int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		if count == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":{"message":"Quota exceeded"}}`))
			return
		}
		resp := geminiResponse{
			Candidates: []geminiCandidate{{Content: geminiContent{Parts: []geminiPart{{Text: "Ok"}}}, FinishReason: "STOP"}},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewGeminiClient("test-key", "gemini-pro", server.URL, 0)
	_, err := client.Chat(context.Background(), ChatRequest{Messages: []Message{{Role: "user", Content: "hi"}}})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 attempts, got %d", count)
	}
}

func TestGeminiClient_Chat_DefaultMaxTokensAndStripPrefix(t *testing.T) {
	var got geminiRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		resp := geminiResponse{
			Candidates: []geminiCandidate{{Content: geminiContent{Parts: []geminiPart{{Text: "Ok"}}}, FinishReason: "STOP"}},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewGeminiClient("test-key", "gemini-pro", server.URL, 0)
	_, err := client.Chat(context.Background(), ChatRequest{
		Model:    "gemini:gemini-1.5-flash",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}

	if got.GenerationConfig.MaxOutputTokens != 8192 {
		t.Errorf("Expected default max tokens 8192, got %d", got.GenerationConfig.MaxOutputTokens)
	}
}
