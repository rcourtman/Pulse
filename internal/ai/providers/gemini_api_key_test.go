package providers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

const geminiTestAPIKey = "AIzaSy-test-secret-key"

func TestGeminiClient_APIKeySentInHeaderNotURL(t *testing.T) {
	var mu sync.Mutex
	seen := map[string]bool{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.String(), geminiTestAPIKey) || r.URL.Query().Has("key") {
			t.Errorf("API key must not be in the request URL: %s", r.URL.String())
		}
		if got := r.Header.Get("x-goog-api-key"); got != geminiTestAPIKey {
			t.Errorf("x-goog-api-key = %q, want the configured key", got)
		}
		mu.Lock()
		seen[r.URL.Path] = true
		mu.Unlock()
		switch {
		case strings.HasSuffix(r.URL.Path, ":streamGenerateContent"):
			if r.URL.Query().Get("alt") != "sse" {
				t.Errorf("expected alt=sse, got %q", r.URL.RawQuery)
			}
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte("data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"ok\"}]},\"finishReason\":\"STOP\"}]}\n\n"))
		case strings.HasSuffix(r.URL.Path, ":generateContent"):
			_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"ok"}]},"finishReason":"STOP"}]}`))
		default:
			_, _ = w.Write([]byte(`{"models":[]}`))
		}
	}))
	defer server.Close()

	client := NewGeminiClient(geminiTestAPIKey, "gemini-pro", server.URL, 0)
	ctx := context.Background()
	req := ChatRequest{Messages: []Message{{Role: "user", Content: "hi"}}}

	if _, err := client.ListModels(ctx); err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if _, err := client.Chat(ctx, req); err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if err := client.ChatStream(ctx, req, func(StreamEvent) {}); err != nil {
		t.Fatalf("ChatStream: %v", err)
	}

	for _, path := range []string{"/models", "/models/gemini-pro:generateContent", "/models/gemini-pro:streamGenerateContent"} {
		if !seen[path] {
			t.Errorf("expected a request to %s, saw %v", path, seen)
		}
	}
}

func TestGeminiClient_ErrorsDoNotContainAPIKey(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	unreachable := server.URL
	server.Close()

	ctx := context.Background()
	// An unparseable model name fails while building the request URL.
	badModel := ChatRequest{Model: "gemini:%zz", Messages: []Message{{Role: "user", Content: "hi"}}}

	client := NewGeminiClient(geminiTestAPIKey, "gemini-pro", unreachable, 0)
	_, listErr := client.ListModels(ctx)
	_, chatErr := client.Chat(ctx, badModel)
	streamErr := client.ChatStream(ctx, badModel, func(StreamEvent) {})

	for name, err := range map[string]error{"ListModels": listErr, "Chat": chatErr, "ChatStream": streamErr} {
		if err == nil {
			t.Errorf("%s: expected an error", name)
			continue
		}
		if strings.Contains(err.Error(), geminiTestAPIKey) {
			t.Errorf("%s: error contains the API key: %v", name, err)
		}
	}
}

func TestGeminiClient_RedirectDoesNotForwardAPIKeyOffOrigin(t *testing.T) {
	var leaked bool
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		leaked = true
		_, _ = w.Write([]byte(`{"models":[]}`))
	}))
	defer other.Close()

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/moved/models" {
			_, _ = w.Write([]byte(`{"models":[]}`))
			return
		}
		if r.URL.Path == "/same/models" {
			http.Redirect(w, r, "/moved/models", http.StatusTemporaryRedirect)
			return
		}
		http.Redirect(w, r, other.URL+"/models", http.StatusTemporaryRedirect)
	}))
	defer origin.Close()

	ctx := context.Background()
	if _, err := NewGeminiClient(geminiTestAPIKey, "gemini-pro", origin.URL+"/same", 0).ListModels(ctx); err != nil {
		t.Errorf("same-origin redirect should be followed: %v", err)
	}

	_, err := NewGeminiClient(geminiTestAPIKey, "gemini-pro", origin.URL, 0).ListModels(ctx)
	if err == nil || !strings.Contains(err.Error(), "redirects must stay on the same origin") {
		t.Errorf("expected cross-origin redirect to be refused, got %v", err)
	}
	if leaked {
		t.Error("cross-origin redirect target received a request")
	}
}
