package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestQuickstartProxyURL_Default(t *testing.T) {
	t.Setenv("PULSE_AI_QUICKSTART_PROXY_URL", "")
	if got := quickstartProxyURL(); got != defaultQuickstartProxyURL {
		t.Fatalf("quickstartProxyURL()=%q want %q", got, defaultQuickstartProxyURL)
	}
}

func TestQuickstartClientChat_UsesOverrideProxyURL(t *testing.T) {
	var seenLicenseID string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req quickstartRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		seenLicenseID = req.LicenseID
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(quickstartResponse{
			Content:    "hello",
			Model:      quickstartModel,
			StopReason: "end_turn",
		}); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
	defer server.Close()

	t.Setenv("PULSE_AI_QUICKSTART_PROXY_URL", server.URL)

	client := NewQuickstartClient("lic_test")
	resp, err := client.Chat(context.Background(), ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("Chat(): %v", err)
	}
	if seenLicenseID != "lic_test" {
		t.Fatalf("license_id=%q want lic_test", seenLicenseID)
	}
	if resp.Content != "hello" {
		t.Fatalf("content=%q want hello", resp.Content)
	}
}

func TestQuickstartClientWithToken_UsesBearerAuthAndSyncsServerState(t *testing.T) {
	var seenAuthorization string
	var seenLicenseID string
	var seenExecutionID string
	var synced QuickstartServerState
	var syncCalls int
	remaining := 16
	total := 25

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req quickstartRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		seenAuthorization = r.Header.Get("Authorization")
		seenLicenseID = req.LicenseID
		seenExecutionID = req.ExecutionID
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(quickstartResponse{
			Content:                  "hello",
			Model:                    quickstartModel,
			StopReason:               "end_turn",
			CreditsRemaining:         &remaining,
			CreditsTotal:             &total,
			QuickstartTokenExpiresAt: "2026-04-02T12:00:00Z",
		}); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
	defer server.Close()

	t.Setenv("PULSE_AI_QUICKSTART_PROXY_URL", server.URL)

	client := NewQuickstartClientWithToken("qst_live_test", func(state QuickstartServerState) {
		synced = state
		syncCalls++
	}, nil)
	resp, err := client.Chat(context.Background(), ChatRequest{
		Messages:    []Message{{Role: "user", Content: "Hi"}},
		ExecutionID: "patrol-run-123",
	})
	if err != nil {
		t.Fatalf("Chat(): %v", err)
	}
	if seenAuthorization != "Bearer qst_live_test" {
		t.Fatalf("Authorization=%q want Bearer qst_live_test", seenAuthorization)
	}
	if seenLicenseID != "" {
		t.Fatalf("license_id=%q want empty for bearer-token quickstart", seenLicenseID)
	}
	if seenExecutionID != "patrol-run-123" {
		t.Fatalf("execution_id=%q want patrol-run-123", seenExecutionID)
	}
	if resp.Content != "hello" {
		t.Fatalf("content=%q want hello", resp.Content)
	}
	if syncCalls != 1 {
		t.Fatalf("syncCalls=%d want 1", syncCalls)
	}
	if synced.CreditsRemaining == nil || synced.CreditsTotal == nil {
		t.Fatalf("expected synced credits, got %v/%v", synced.CreditsRemaining, synced.CreditsTotal)
	}
	if *synced.CreditsRemaining != 16 || *synced.CreditsTotal != 25 {
		t.Fatalf("synced credits=%d/%d want 16/25", *synced.CreditsRemaining, *synced.CreditsTotal)
	}
	if synced.TokenExpiresAt == nil {
		t.Fatal("expected token expiry sync")
	}
	if got := synced.TokenExpiresAt.UTC().Format(time.RFC3339); got != "2026-04-02T12:00:00Z" {
		t.Fatalf("TokenExpiresAt=%q want 2026-04-02T12:00:00Z", got)
	}
}

func TestQuickstartClientWithToken_AcceptsTokenExpiresAtAlias(t *testing.T) {
	var synced QuickstartServerState

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		remaining := 24
		total := 25
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(quickstartResponse{
			Content:          "hello",
			Model:            quickstartModel,
			StopReason:       "end_turn",
			CreditsRemaining: &remaining,
			CreditsTotal:     &total,
			TokenExpiresAt:   "2026-04-03T19:00:00Z",
		}); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
	defer server.Close()

	t.Setenv("PULSE_AI_QUICKSTART_PROXY_URL", server.URL)

	client := NewQuickstartClientWithToken("qst_live_test", func(state QuickstartServerState) {
		synced = state
	}, nil)
	if _, err := client.Chat(context.Background(), ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hi"}},
	}); err != nil {
		t.Fatalf("Chat(): %v", err)
	}
	if synced.TokenExpiresAt == nil {
		t.Fatal("expected token expiry sync")
	}
	if got := synced.TokenExpiresAt.UTC().Format(time.RFC3339); got != "2026-04-03T19:00:00Z" {
		t.Fatalf("TokenExpiresAt=%q want 2026-04-03T19:00:00Z", got)
	}
}

func TestQuickstartClientWithToken_ExhaustionSyncsStateAndReturnsTypedError(t *testing.T) {
	var synced QuickstartServerState
	var syncCalls int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusPaymentRequired)
		w.Header().Set("Content-Type", "application/json")
		remaining := 0
		total := 25
		if err := json.NewEncoder(w).Encode(quickstartResponse{
			Error:            "credits exhausted",
			Code:             "quickstart_credits_exhausted",
			CreditsRemaining: &remaining,
			CreditsTotal:     &total,
		}); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
	defer server.Close()

	t.Setenv("PULSE_AI_QUICKSTART_PROXY_URL", server.URL)

	client := NewQuickstartClientWithToken("qst_live_test", func(state QuickstartServerState) {
		synced = state
		syncCalls++
	}, nil)
	_, err := client.Chat(context.Background(), ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err == nil {
		t.Fatal("expected quickstart exhaustion error")
	}
	if !IsQuickstartCreditsExhausted(err) {
		t.Fatalf("expected typed quickstart exhaustion, got %v", err)
	}
	if syncCalls != 1 {
		t.Fatalf("syncCalls=%d want 1", syncCalls)
	}
	if synced.CreditsRemaining == nil || synced.CreditsTotal == nil {
		t.Fatalf("expected synced credits, got %v/%v", synced.CreditsRemaining, synced.CreditsTotal)
	}
	if *synced.CreditsRemaining != 0 || *synced.CreditsTotal != 25 {
		t.Fatalf("synced credits=%d/%d want 0/25", *synced.CreditsRemaining, *synced.CreditsTotal)
	}
}

func TestQuickstartClientWithToken_DoesNotZeroCreditsWhenErrorOmitsInventory(t *testing.T) {
	var syncCalls int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(quickstartResponse{
			Error: "temporary upstream failure",
		}); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
	defer server.Close()

	t.Setenv("PULSE_AI_QUICKSTART_PROXY_URL", server.URL)

	client := NewQuickstartClientWithToken("qst_live_test", func(state QuickstartServerState) {
		syncCalls++
	}, nil)
	_, err := client.Chat(context.Background(), ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err == nil {
		t.Fatal("expected quickstart error")
	}
	if syncCalls != 0 {
		t.Fatalf("syncCalls=%d want 0 when error omits inventory", syncCalls)
	}
}
