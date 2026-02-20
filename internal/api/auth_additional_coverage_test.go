package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestCheckAuth_APIOnlyModeRequiresToken(t *testing.T) {
	record, err := config.NewAPITokenRecord("token-required-123.12345678", "api", []string{config.ScopeMonitoringRead})
	if err != nil {
		t.Fatalf("NewAPITokenRecord: %v", err)
	}
	cfg := &config.Config{
		APITokens: []config.APITokenRecord{*record},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	rr := httptest.NewRecorder()

	if CheckAuth(cfg, rr, req) {
		t.Fatalf("expected CheckAuth to fail without token in API-only mode")
	}
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "API token required") {
		t.Fatalf("expected API token required message, got %q", rr.Body.String())
	}
	if rr.Header().Get("WWW-Authenticate") == "" {
		t.Fatalf("expected WWW-Authenticate header to be set")
	}
}

func TestCheckAuth_APIOnlyModeRejectsInvalidToken(t *testing.T) {
	record, err := config.NewAPITokenRecord("token-valid-123.12345678", "api", []string{config.ScopeMonitoringRead})
	if err != nil {
		t.Fatalf("NewAPITokenRecord: %v", err)
	}
	cfg := &config.Config{
		APITokens: []config.APITokenRecord{*record},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("X-API-Token", "token-invalid-123.12345678")
	rr := httptest.NewRecorder()

	if CheckAuth(cfg, rr, req) {
		t.Fatalf("expected CheckAuth to fail with invalid token")
	}
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestCheckAuth_APIOnlyModeAcceptsQueryToken(t *testing.T) {
	rawToken := "token-query-123.12345678"
	record, err := config.NewAPITokenRecord(rawToken, "api", []string{config.ScopeMonitoringRead})
	if err != nil {
		t.Fatalf("NewAPITokenRecord: %v", err)
	}
	cfg := &config.Config{
		APITokens: []config.APITokenRecord{*record},
	}

	// Query-string tokens are rejected on regular HTTP to prevent URL-based leakage.
	req := httptest.NewRequest(http.MethodGet, "/api/test?token="+rawToken, nil)
	rr := httptest.NewRecorder()

	if CheckAuth(cfg, rr, req) {
		t.Fatalf("expected CheckAuth to reject query token on regular HTTP request")
	}

	// Query-string tokens are accepted on WebSocket upgrade requests.
	wsReq := httptest.NewRequest(http.MethodGet, "/api/test?token="+rawToken, nil)
	wsReq.Header.Set("Upgrade", "websocket")
	wsReq.Header.Set("Connection", "Upgrade")
	wsRR := httptest.NewRecorder()

	if !CheckAuth(cfg, wsRR, wsReq) {
		t.Fatalf("expected CheckAuth to succeed with query token on WebSocket upgrade")
	}
	if wsRR.Header().Get("X-Auth-Method") != "api_token" {
		t.Fatalf("expected X-Auth-Method api_token, got %q", wsRR.Header().Get("X-Auth-Method"))
	}
}

func TestCheckAuth_AcceptsBearerToken(t *testing.T) {
	rawToken := "token-bearer-123.12345678"
	record, err := config.NewAPITokenRecord(rawToken, "api", []string{config.ScopeMonitoringRead})
	if err != nil {
		t.Fatalf("NewAPITokenRecord: %v", err)
	}
	cfg := &config.Config{
		APITokens: []config.APITokenRecord{*record},
		AuthUser:  "admin",
		AuthPass:  "$2a$10$dummy",
	}

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Authorization", "Bearer "+rawToken)
	rr := httptest.NewRecorder()

	if !CheckAuth(cfg, rr, req) {
		t.Fatalf("expected CheckAuth to succeed with bearer token")
	}
	if rr.Header().Get("X-Auth-Method") != "api_token" {
		t.Fatalf("expected X-Auth-Method api_token, got %q", rr.Header().Get("X-Auth-Method"))
	}
}

func TestCheckAuth_NilConfigFailsClosed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	rr := httptest.NewRecorder()

	if CheckAuth(nil, rr, req) {
		t.Fatalf("expected CheckAuth to fail when config is nil")
	}
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Authentication unavailable") {
		t.Fatalf("expected authentication unavailable message, got %q", rr.Body.String())
	}
}
