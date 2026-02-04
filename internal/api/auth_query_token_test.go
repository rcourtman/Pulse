package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestIsWebSocketUpgrade(t *testing.T) {
	tests := []struct {
		name     string
		upgrade  string
		expected bool
	}{
		{"lowercase websocket", "websocket", true},
		{"uppercase WebSocket", "WebSocket", true},
		{"mixed case WEBSOCKET", "WEBSOCKET", true},
		{"empty header", "", false},
		{"other value", "h2c", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.upgrade != "" {
				req.Header.Set("Upgrade", tt.upgrade)
			}
			if got := isWebSocketUpgrade(req); got != tt.expected {
				t.Errorf("isWebSocketUpgrade() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCheckAuth_QueryTokenRejectedWithoutWebSocketUpgrade(t *testing.T) {
	// API-only mode: only API tokens configured, no user/password.
	const rawToken = "test-api-only-query-token-12345"

	record, err := config.NewAPITokenRecord(rawToken, "test", nil)
	if err != nil {
		t.Fatalf("create token record: %v", err)
	}

	cfg := &config.Config{
		APITokens: []config.APITokenRecord{*record},
	}
	cfg.SortAPITokens()

	// Regular HTTP request with query-string token should be rejected.
	req := httptest.NewRequest(http.MethodGet, "/api/state?token="+rawToken, nil)
	rr := httptest.NewRecorder()

	if CheckAuth(cfg, rr, req) {
		t.Fatal("expected CheckAuth to reject query-string token on regular HTTP request (API-only mode)")
	}

	// Same request with WebSocket Upgrade header should be accepted.
	wsReq := httptest.NewRequest(http.MethodGet, "/api/state?token="+rawToken, nil)
	wsReq.Header.Set("Upgrade", "websocket")
	wsReq.Header.Set("Connection", "Upgrade")
	wsRR := httptest.NewRecorder()

	if !CheckAuth(cfg, wsRR, wsReq) {
		t.Fatal("expected CheckAuth to accept query-string token on WebSocket upgrade (API-only mode)")
	}
}

func TestCheckAuth_QueryTokenRejectedStandardMode(t *testing.T) {
	// Standard mode: both password auth and API tokens configured.
	const rawToken = "test-standard-query-token-12345"

	record, err := config.NewAPITokenRecord(rawToken, "test", nil)
	if err != nil {
		t.Fatalf("create token record: %v", err)
	}

	cfg := &config.Config{
		AuthUser:  "admin",
		AuthPass:  "$2a$10$invalidhashfortesting1234567890123456789012",
		APITokens: []config.APITokenRecord{*record},
	}
	cfg.SortAPITokens()

	// Regular HTTP request with query-string token should be rejected.
	req := httptest.NewRequest(http.MethodGet, "/api/state?token="+rawToken, nil)
	rr := httptest.NewRecorder()

	if CheckAuth(cfg, rr, req) {
		t.Fatal("expected CheckAuth to reject query-string token on regular HTTP request (standard mode)")
	}

	// WebSocket upgrade with query-string token should be accepted.
	wsReq := httptest.NewRequest(http.MethodGet, "/api/state?token="+rawToken, nil)
	wsReq.Header.Set("Upgrade", "websocket")
	wsReq.Header.Set("Connection", "Upgrade")
	wsRR := httptest.NewRecorder()

	if !CheckAuth(cfg, wsRR, wsReq) {
		t.Fatal("expected CheckAuth to accept query-string token on WebSocket upgrade (standard mode)")
	}

	// Header-based token should still work for regular requests.
	headerReq := httptest.NewRequest(http.MethodGet, "/api/state", nil)
	headerReq.Header.Set("X-API-Token", rawToken)
	headerRR := httptest.NewRecorder()

	if !CheckAuth(cfg, headerRR, headerReq) {
		t.Fatal("expected CheckAuth to accept header token on regular HTTP request")
	}
}
