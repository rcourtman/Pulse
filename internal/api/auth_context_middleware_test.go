package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

func TestExtractAndStoreAuthContext_APITokenHeader(t *testing.T) {
	rawToken := "ctx-token-123.12345678"
	record, err := config.NewAPITokenRecord(rawToken, "ctx-token", []string{config.ScopeMonitoringRead})
	if err != nil {
		t.Fatalf("new token record: %v", err)
	}
	cfg := &config.Config{
		APITokens: []config.APITokenRecord{*record},
	}
	cfg.SortAPITokens()

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("X-API-Token", rawToken)

	req = extractAndStoreAuthContext(cfg, nil, req)

	user := internalauth.GetUser(req.Context())
	if user != "token:"+record.ID {
		t.Fatalf("expected user token:%s, got %q", record.ID, user)
	}
	ctxRecord, ok := internalauth.GetAPIToken(req.Context()).(*config.APITokenRecord)
	if !ok || ctxRecord == nil {
		t.Fatalf("expected API token record in context")
	}
	if ctxRecord.ID != record.ID {
		t.Fatalf("expected context token ID %q, got %q", record.ID, ctxRecord.ID)
	}
}

func TestExtractAndStoreAuthContext_BearerToken(t *testing.T) {
	rawToken := "ctx-bearer-123.12345678"
	record, err := config.NewAPITokenRecord(rawToken, "ctx-token", []string{config.ScopeMonitoringRead})
	if err != nil {
		t.Fatalf("new token record: %v", err)
	}
	cfg := &config.Config{
		APITokens: []config.APITokenRecord{*record},
	}
	cfg.SortAPITokens()

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Authorization", "Bearer "+rawToken)

	req = extractAndStoreAuthContext(cfg, nil, req)

	user := internalauth.GetUser(req.Context())
	if user != "token:"+record.ID {
		t.Fatalf("expected user token:%s, got %q", record.ID, user)
	}
}

func TestExtractAndStoreAuthContext_QueryTokenRequiresWebSocketUpgrade(t *testing.T) {
	rawToken := "ctx-query-123.12345678"
	record, err := config.NewAPITokenRecord(rawToken, "ctx-token", []string{config.ScopeMonitoringRead})
	if err != nil {
		t.Fatalf("new token record: %v", err)
	}
	cfg := &config.Config{
		APITokens: []config.APITokenRecord{*record},
	}
	cfg.SortAPITokens()

	req := httptest.NewRequest(http.MethodGet, "/api/test?token="+rawToken, nil)
	req = extractAndStoreAuthContext(cfg, nil, req)
	if internalauth.GetUser(req.Context()) != "" {
		t.Fatalf("expected no user context without WebSocket upgrade")
	}

	wsReq := httptest.NewRequest(http.MethodGet, "/api/test?token="+rawToken, nil)
	wsReq.Header.Set("Upgrade", "websocket")
	wsReq.Header.Set("Connection", "Upgrade")
	wsReq = extractAndStoreAuthContext(cfg, nil, wsReq)
	if internalauth.GetUser(wsReq.Context()) != "token:"+record.ID {
		t.Fatalf("expected user context for WebSocket query token")
	}
}

func TestExtractAndStoreAuthContext_SessionCookie(t *testing.T) {
	dir := t.TempDir()
	InitSessionStore(dir)

	sessionToken := generateSessionToken()
	GetSessionStore().CreateSession(sessionToken, time.Hour, "agent", "127.0.0.1", "alice")

	cfg := &config.Config{}

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.AddCookie(&http.Cookie{Name: "pulse_session", Value: sessionToken})
	req = extractAndStoreAuthContext(cfg, nil, req)

	if internalauth.GetUser(req.Context()) != "alice" {
		t.Fatalf("expected user alice, got %q", internalauth.GetUser(req.Context()))
	}
}

func TestExtractAndStoreAuthContext_ProxyAuth(t *testing.T) {
	cfg := &config.Config{
		ProxyAuthSecret:     "proxy-secret",
		ProxyAuthUserHeader: "X-Proxy-User",
	}

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("X-Proxy-Secret", "proxy-secret")
	req.Header.Set("X-Proxy-User", "proxyuser")

	req = extractAndStoreAuthContext(cfg, nil, req)

	if internalauth.GetUser(req.Context()) != "proxyuser" {
		t.Fatalf("expected proxy user context, got %q", internalauth.GetUser(req.Context()))
	}
}
