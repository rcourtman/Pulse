package providers

import (
	"context"
	"testing"
	"time"
)

// Anthropic tests are limited because the client hardcodes API URLs.
// These tests verify context handling and basic construction.

func TestAnthropicClient_Name(t *testing.T) {
	client := NewAnthropicClient("test-key", "claude-3-5-sonnet")
	if client.Name() != "anthropic" {
		t.Errorf("Expected 'anthropic', got '%s'", client.Name())
	}
}

func TestAnthropicClient_Chat_ContextCanceled(t *testing.T) {
	client := NewAnthropicClient("test-key", "claude-3-5-sonnet")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := client.Chat(ctx, ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hello"}},
	})

	if err == nil {
		t.Error("Expected error for canceled context")
	}
}

func TestAnthropicClient_Chat_Timeout(t *testing.T) {
	client := NewAnthropicClient("test-key", "claude-3-5-sonnet")

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	_, err := client.Chat(ctx, ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hello"}},
	})

	if err == nil {
		t.Error("Expected timeout error")
	}
}
