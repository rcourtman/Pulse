// +build integration

package providers_test

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
)

// Integration tests for Ollama provider
// Run with: go test -tags=integration ./internal/ai/providers/...
//
// These tests require a running Ollama instance.
// Set OLLAMA_URL environment variable or default to http://192.168.0.124:11434

func getOllamaURL() string {
	if url := os.Getenv("OLLAMA_URL"); url != "" {
		return url
	}
	return "http://192.168.0.124:11434"
}

func TestIntegration_Ollama_TestConnection(t *testing.T) {
	client := providers.NewOllamaClient("tinyllama", getOllamaURL(, 0))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := client.TestConnection(ctx)
	if err != nil {
		t.Fatalf("TestConnection failed: %v", err)
	}
	t.Log("✓ Ollama connection successful")
}

func TestIntegration_Ollama_ListModels(t *testing.T) {
	client := providers.NewOllamaClient("tinyllama", getOllamaURL(, 0))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	models, err := client.ListModels(ctx)
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}

	if len(models) == 0 {
		t.Error("Expected at least one model")
	}

	t.Logf("✓ Found %d models:", len(models))
	for _, m := range models {
		t.Logf("  - %s", m.ID)
	}
}

func TestIntegration_Ollama_SimpleChat(t *testing.T) {
	client := providers.NewOllamaClient("tinyllama", getOllamaURL(, 0))

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	resp, err := client.Chat(ctx, providers.ChatRequest{
		Messages: []providers.Message{
			{Role: "user", Content: "Say 'hello' and nothing else."},
		},
		MaxTokens: 10,
	})

	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	if resp.Content == "" {
		t.Error("Expected non-empty response")
	}

	t.Logf("✓ Response: %s", resp.Content)
	t.Logf("  Input tokens: %d, Output tokens: %d", resp.InputTokens, resp.OutputTokens)
}

func TestIntegration_Ollama_SystemPrompt(t *testing.T) {
	client := providers.NewOllamaClient("tinyllama", getOllamaURL(, 0))

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	resp, err := client.Chat(ctx, providers.ChatRequest{
		Messages: []providers.Message{
			{Role: "user", Content: "What is 2+2?"},
		},
		System:    "You are a math tutor. Always answer with just the number.",
		MaxTokens: 10,
	})

	if err != nil {
		t.Fatalf("Chat with system prompt failed: %v", err)
	}

	if resp.Content == "" {
		t.Error("Expected non-empty response")
	}

	// Should contain "4" somewhere
	if !strings.Contains(resp.Content, "4") {
		t.Logf("Warning: Expected '4' in response, got: %s", resp.Content)
	}

	t.Logf("✓ Response with system prompt: %s", resp.Content)
}

func TestIntegration_Ollama_MultiTurnConversation(t *testing.T) {
	client := providers.NewOllamaClient("tinyllama", getOllamaURL(, 0))

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// First turn
	resp1, err := client.Chat(ctx, providers.ChatRequest{
		Messages: []providers.Message{
			{Role: "user", Content: "My name is Alice."},
		},
		MaxTokens: 50,
	})
	if err != nil {
		t.Fatalf("First turn failed: %v", err)
	}
	t.Logf("Turn 1 response: %s", resp1.Content)

	// Second turn - should remember the name
	resp2, err := client.Chat(ctx, providers.ChatRequest{
		Messages: []providers.Message{
			{Role: "user", Content: "My name is Alice."},
			{Role: "assistant", Content: resp1.Content},
			{Role: "user", Content: "What is my name?"},
		},
		MaxTokens: 20,
	})
	if err != nil {
		t.Fatalf("Second turn failed: %v", err)
	}

	t.Logf("✓ Turn 2 response: %s", resp2.Content)

	// Should mention Alice
	if !strings.Contains(strings.ToLower(resp2.Content), "alice") {
		t.Logf("Warning: Expected 'Alice' in response, got: %s", resp2.Content)
	}
}

func TestIntegration_Ollama_TokenCounting(t *testing.T) {
	client := providers.NewOllamaClient("tinyllama", getOllamaURL(, 0))

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	resp, err := client.Chat(ctx, providers.ChatRequest{
		Messages: []providers.Message{
			{Role: "user", Content: "Count to 5."},
		},
		MaxTokens: 50,
	})

	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	t.Logf("✓ Token usage - Input: %d, Output: %d", resp.InputTokens, resp.OutputTokens)

	// Ollama should return token counts
	if resp.InputTokens == 0 {
		t.Log("Note: Input tokens not reported (may be Ollama version dependent)")
	}
	if resp.OutputTokens == 0 {
		t.Log("Note: Output tokens not reported (may be Ollama version dependent)")
	}
}

func TestIntegration_Ollama_ErrorHandling_BadModel(t *testing.T) {
	client := providers.NewOllamaClient("nonexistent-model-12345", getOllamaURL(, 0))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := client.Chat(ctx, providers.ChatRequest{
		Messages: []providers.Message{
			{Role: "user", Content: "Hello"},
		},
	})

	if err == nil {
		t.Error("Expected error for non-existent model")
	} else {
		t.Logf("✓ Got expected error for bad model: %v", err)
	}
}

func TestIntegration_Ollama_Timeout(t *testing.T) {
	client := providers.NewOllamaClient("tinyllama", getOllamaURL(, 0))

	// Very short timeout - should fail
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	_, err := client.Chat(ctx, providers.ChatRequest{
		Messages: []providers.Message{
			{Role: "user", Content: "Write a long essay about the history of computing."},
		},
		MaxTokens: 1000,
	})

	if err == nil {
		t.Error("Expected timeout error")
	} else {
		t.Logf("✓ Got expected timeout error: %v", err)
	}
}

// --- More useful tests below ---

func TestIntegration_Ollama_JSONOutput(t *testing.T) {
	client := providers.NewOllamaClient("tinyllama", getOllamaURL(, 0))

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	resp, err := client.Chat(ctx, providers.ChatRequest{
		Messages: []providers.Message{
			{Role: "user", Content: `Respond with only valid JSON, no other text. The JSON should have keys "status" and "count". Example: {"status": "ok", "count": 5}`},
		},
		System:    "You are a JSON-only bot. Only output valid JSON, nothing else.",
		MaxTokens: 50,
	})

	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	// Try to extract JSON from response
	content := strings.TrimSpace(resp.Content)
	t.Logf("Raw response: %s", content)

	// Check if it looks like JSON
	if !strings.Contains(content, "{") || !strings.Contains(content, "}") {
		t.Logf("Warning: Response doesn't look like JSON: %s", content)
	}

	// Try to parse it
	var result map[string]interface{}
	// Find JSON in response (model might add text around it)
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start >= 0 && end > start {
		jsonStr := content[start : end+1]
		if err := json.Unmarshal([]byte(jsonStr), &result); err == nil {
			t.Logf("✓ Successfully parsed JSON: %v", result)
		} else {
			t.Logf("Warning: Found JSON-like content but couldn't parse: %s", jsonStr)
		}
	}
}

func TestIntegration_Ollama_LongResponse(t *testing.T) {
	client := providers.NewOllamaClient("tinyllama", getOllamaURL(, 0))

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	resp, err := client.Chat(ctx, providers.ChatRequest{
		Messages: []providers.Message{
			{Role: "user", Content: "List the numbers 1 through 20, one per line."},
		},
		MaxTokens: 200,
	})

	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	lines := strings.Split(resp.Content, "\n")
	t.Logf("✓ Got %d lines, %d tokens output", len(lines), resp.OutputTokens)

	if resp.OutputTokens < 20 {
		t.Logf("Note: Expected ~40+ tokens for counting 1-20, got %d", resp.OutputTokens)
	}
}

func TestIntegration_Ollama_EmptyMessage(t *testing.T) {
	client := providers.NewOllamaClient("tinyllama", getOllamaURL(, 0))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.Chat(ctx, providers.ChatRequest{
		Messages: []providers.Message{
			{Role: "user", Content: ""},
		},
		MaxTokens: 10,
	})

	// Should either error or return something
	if err != nil {
		t.Logf("✓ Empty message returned error (acceptable): %v", err)
	} else {
		t.Logf("✓ Empty message returned response: %s", resp.Content)
	}
}

func TestIntegration_Ollama_SpecialCharacters(t *testing.T) {
	client := providers.NewOllamaClient("tinyllama", getOllamaURL(, 0))

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Test with special chars, unicode, newlines
	testMessage := "Repeat back: Hello 世界! Line1\nLine2\t<tag>&entity;\"quotes\""

	resp, err := client.Chat(ctx, providers.ChatRequest{
		Messages: []providers.Message{
			{Role: "user", Content: testMessage},
		},
		MaxTokens: 100,
	})

	if err != nil {
		t.Fatalf("Chat with special chars failed: %v", err)
	}

	t.Logf("✓ Handled special characters. Response: %s", resp.Content)
}

func TestIntegration_Ollama_ConcurrentRequests(t *testing.T) {
	client := providers.NewOllamaClient("tinyllama", getOllamaURL(, 0))

	const numRequests = 3
	results := make(chan error, numRequests)

	for i := 0; i < numRequests; i++ {
		go func(id int) {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			_, err := client.Chat(ctx, providers.ChatRequest{
				Messages: []providers.Message{
					{Role: "user", Content: "Say 'ok'"},
				},
				MaxTokens: 5,
			})
			results <- err
		}(i)
	}

	successes := 0
	for i := 0; i < numRequests; i++ {
		if err := <-results; err == nil {
			successes++
		}
	}

	t.Logf("✓ Concurrent requests: %d/%d succeeded", successes, numRequests)

	if successes < numRequests {
		t.Logf("Note: Some concurrent requests failed (Ollama might be queuing)")
	}
}

func TestIntegration_Ollama_InfrastructureAnalysis(t *testing.T) {
	// This simulates what Pulse actually does - send infrastructure context
	client := providers.NewOllamaClient("tinyllama", getOllamaURL(, 0))

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// Simulated infrastructure context like Pulse would send
	infraContext := `## Infrastructure Overview
- 3 nodes: pve1, pve2, pve3
- 15 VMs running
- 8 containers

## Current Alerts
- VM 101 (web-server): CPU at 95%
- Container 203 (redis): Memory at 88%

## Question
What should I investigate first?`

	resp, err := client.Chat(ctx, providers.ChatRequest{
		Messages: []providers.Message{
			{Role: "user", Content: infraContext},
		},
		System:    "You are an infrastructure analyst. Prioritize issues by severity.",
		MaxTokens: 150,
	})

	if err != nil {
		t.Fatalf("Infrastructure analysis failed: %v", err)
	}

	t.Logf("✓ Infrastructure analysis response (%d tokens):", resp.OutputTokens)
	t.Logf("  %s", resp.Content)

	// Check if it mentions any of our issues
	lower := strings.ToLower(resp.Content)
	if strings.Contains(lower, "cpu") || strings.Contains(lower, "95") {
		t.Log("  ✓ Mentions CPU issue")
	}
	if strings.Contains(lower, "memory") || strings.Contains(lower, "88") {
		t.Log("  ✓ Mentions memory issue")
	}
}

func TestIntegration_Ollama_ModelName_Preserved(t *testing.T) {
	client := providers.NewOllamaClient("tinyllama", getOllamaURL(, 0))

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	resp, err := client.Chat(ctx, providers.ChatRequest{
		Messages: []providers.Message{
			{Role: "user", Content: "Hi"},
		},
		MaxTokens: 5,
	})

	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	// Model name should be in response
	if resp.Model == "" {
		t.Error("Model name not returned in response")
	} else {
		t.Logf("✓ Model in response: %s", resp.Model)
	}
}

func TestIntegration_Ollama_StopReason(t *testing.T) {
	client := providers.NewOllamaClient("tinyllama", getOllamaURL(, 0))

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	resp, err := client.Chat(ctx, providers.ChatRequest{
		Messages: []providers.Message{
			{Role: "user", Content: "Say hello."},
		},
		MaxTokens: 100, // Enough to complete naturally
	})

	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	t.Logf("✓ Stop reason: %s", resp.StopReason)
}

func TestIntegration_Ollama_VeryLongInput(t *testing.T) {
	client := providers.NewOllamaClient("tinyllama", getOllamaURL(, 0))

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Create a long input (similar to what Pulse sends with full infrastructure)
	longInput := strings.Repeat("The quick brown fox jumps over the lazy dog. ", 100)
	longInput += "\n\nSummarize the above text in one word."

	resp, err := client.Chat(ctx, providers.ChatRequest{
		Messages: []providers.Message{
			{Role: "user", Content: longInput},
		},
		MaxTokens: 20,
	})

	if err != nil {
		t.Fatalf("Long input failed: %v", err)
	}

	t.Logf("✓ Handled long input (%d chars). Response: %s", len(longInput), resp.Content)
	t.Logf("  Input tokens: %d", resp.InputTokens)
}

func TestIntegration_Ollama_RapidFireRequests(t *testing.T) {
	client := providers.NewOllamaClient("tinyllama", getOllamaURL(, 0))

	// Send 5 requests in rapid succession
	for i := 0; i < 5; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

		resp, err := client.Chat(ctx, providers.ChatRequest{
			Messages: []providers.Message{
				{Role: "user", Content: "1+1="},
			},
			MaxTokens: 3,
		})
		cancel()

		if err != nil {
			t.Logf("Request %d failed: %v", i+1, err)
		} else {
			t.Logf("Request %d: %s", i+1, strings.TrimSpace(resp.Content))
		}
	}
	t.Log("✓ Completed rapid-fire requests")
}
