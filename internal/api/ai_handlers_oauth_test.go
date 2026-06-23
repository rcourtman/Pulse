package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestHandleOAuthStart_Unsupported(t *testing.T) {
	handler := &AISettingsHandler{}
	req := httptest.NewRequest(http.MethodGet, "/api/ai/oauth/start", nil)
	rr := httptest.NewRecorder()

	handler.HandleOAuthStart(rr, req)

	if rr.Code != http.StatusNotImplemented {
		t.Fatalf("expected 501, got %d body=%q", rr.Code, rr.Body.String())
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["error"] != "unsupported_anthropic_oauth" {
		t.Fatalf("expected unsupported error, got %v", resp["error"])
	}
}

func TestHandleOAuthStart_MethodNotAllowed(t *testing.T) {
	handler := &AISettingsHandler{}
	req := httptest.NewRequest(http.MethodPut, "/api/ai/oauth/start", nil)
	rr := httptest.NewRecorder()

	handler.HandleOAuthStart(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
}

func TestHandleOAuthExchange_Unsupported(t *testing.T) {
	handler := &AISettingsHandler{}
	req := httptest.NewRequest(http.MethodPost, "/api/ai/oauth/exchange", strings.NewReader(`{"code":"code","state":"state"}`))
	rr := httptest.NewRecorder()

	handler.HandleOAuthExchange(rr, req)

	if rr.Code != http.StatusNotImplemented {
		t.Fatalf("expected 501, got %d body=%q", rr.Code, rr.Body.String())
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["error"] != "unsupported_anthropic_oauth" {
		t.Fatalf("expected unsupported error, got %v", resp["error"])
	}
}

func TestHandleOAuthExchange_MethodNotAllowed(t *testing.T) {
	handler := &AISettingsHandler{}
	req := httptest.NewRequest(http.MethodGet, "/api/ai/oauth/exchange", nil)
	rr := httptest.NewRecorder()

	handler.HandleOAuthExchange(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
}

func TestHandleOAuthCallback_Unsupported(t *testing.T) {
	handler := &AISettingsHandler{}
	req := httptest.NewRequest(http.MethodGet, "/api/ai/oauth/callback?code=abc&state=state", nil)
	rr := httptest.NewRecorder()

	handler.HandleOAuthCallback(rr, req)

	if rr.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected 307, got %d", rr.Code)
	}
	location := rr.Header().Get("Location")
	if !strings.Contains(location, "ai_oauth_error=unsupported") {
		t.Fatalf("expected unsupported redirect, got %q", location)
	}
}

func TestHandleOAuthCallback_ErrorParamURLEncoded(t *testing.T) {
	handler := &AISettingsHandler{}
	req := httptest.NewRequest(http.MethodGet, "/api/ai/oauth/callback?error=bad%26inject%3Dfoo&error_description=test", nil)
	rr := httptest.NewRecorder()

	handler.HandleOAuthCallback(rr, req)
	if rr.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected 307, got %d", rr.Code)
	}
	location := rr.Header().Get("Location")
	parsed, err := url.Parse(location)
	if err != nil {
		t.Fatalf("failed to parse redirect URL %q: %v", location, err)
	}
	qvals := parsed.Query()
	if len(qvals) != 1 {
		t.Fatalf("expected exactly 1 query key, got %d: %v", len(qvals), qvals)
	}
	got := qvals.Get("ai_oauth_error")
	if got != "bad&inject=foo" {
		t.Fatalf("expected decoded error value %q, got %q", "bad&inject=foo", got)
	}
}

func TestHandleOAuthCallback_MethodNotAllowed(t *testing.T) {
	handler := &AISettingsHandler{}
	req := httptest.NewRequest(http.MethodPost, "/api/ai/oauth/callback", nil)
	rr := httptest.NewRecorder()

	handler.HandleOAuthCallback(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
}

func TestHandleOAuthDisconnect_MethodNotAllowed(t *testing.T) {
	handler := &AISettingsHandler{}
	req := httptest.NewRequest(http.MethodGet, "/api/ai/oauth/disconnect", nil)
	rr := httptest.NewRecorder()

	handler.HandleOAuthDisconnect(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
}

func TestHandleOAuthDisconnect_AuthFailure(t *testing.T) {
	handler := &AISettingsHandler{}
	req := httptest.NewRequest(http.MethodPost, "/api/ai/oauth/disconnect", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rr := httptest.NewRecorder()

	handler.HandleOAuthDisconnect(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 for nil config auth failure, got %d", rr.Code)
	}
}

func TestHandleOAuthDisconnect_Success(t *testing.T) {
	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	p, err := mtp.GetPersistence("default")
	if err != nil {
		t.Fatalf("create default persistence: %v", err)
	}

	aiCfg := config.NewDefaultAIConfig()
	aiCfg.AuthMethod = config.AuthMethodOAuth
	aiCfg.OAuthAccessToken = "access-token-123"
	aiCfg.OAuthRefreshToken = "refresh-token-456"
	aiCfg.OAuthExpiresAt = time.Now().Add(time.Hour)
	if err := p.SaveAIConfig(*aiCfg); err != nil {
		t.Fatalf("seed AI config: %v", err)
	}

	handler := NewAISettingsHandler(mtp, nil, nil)
	handler.SetConfig(&config.Config{})

	req := httptest.NewRequest(http.MethodPost, "/api/ai/oauth/disconnect", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rr := httptest.NewRecorder()

	handler.HandleOAuthDisconnect(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%q", rr.Code, rr.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["success"] != true {
		t.Fatalf("expected success=true, got %v", resp["success"])
	}

	saved, err := p.LoadAIConfig()
	if err != nil {
		t.Fatalf("load AI config after disconnect: %v", err)
	}
	if saved.AuthMethod != config.AuthMethodAPIKey {
		t.Fatalf("expected auth method %q, got %q", config.AuthMethodAPIKey, saved.AuthMethod)
	}
	if saved.OAuthAccessToken != "" {
		t.Fatalf("expected empty access token, got %q", saved.OAuthAccessToken)
	}
	if saved.OAuthRefreshToken != "" {
		t.Fatalf("expected empty refresh token, got %q", saved.OAuthRefreshToken)
	}
	if !saved.OAuthExpiresAt.IsZero() {
		t.Fatalf("expected zero expiry, got %v", saved.OAuthExpiresAt)
	}
}

func TestHandleOAuthDisconnect_NilPersistence(t *testing.T) {
	handler := NewAISettingsHandler(nil, nil, nil)
	handler.SetConfig(&config.Config{})

	req := httptest.NewRequest(http.MethodPost, "/api/ai/oauth/disconnect", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rr := httptest.NewRecorder()

	handler.HandleOAuthDisconnect(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for nil persistence, got %d body=%q", rr.Code, rr.Body.String())
	}
}

func TestHandleOAuthDisconnect_TenantIsolation(t *testing.T) {
	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)

	pA, err := mtp.GetPersistence("tenant-a")
	if err != nil {
		t.Fatalf("create tenant-a persistence: %v", err)
	}
	pB, err := mtp.GetPersistence("tenant-b")
	if err != nil {
		t.Fatalf("create tenant-b persistence: %v", err)
	}

	seedOAuth := func(p *config.ConfigPersistence, token, refresh string) {
		aiCfg := config.NewDefaultAIConfig()
		aiCfg.AuthMethod = config.AuthMethodOAuth
		aiCfg.OAuthAccessToken = token
		aiCfg.OAuthRefreshToken = refresh
		aiCfg.OAuthExpiresAt = time.Now().Add(time.Hour)
		if err := p.SaveAIConfig(*aiCfg); err != nil {
			t.Fatalf("seed AI config: %v", err)
		}
	}
	seedOAuth(pA, "access-a", "refresh-a")
	seedOAuth(pB, "access-b", "refresh-b")

	handler := NewAISettingsHandler(mtp, nil, nil)
	handler.SetConfig(&config.Config{})

	req := httptest.NewRequest(http.MethodPost, "/api/ai/oauth/disconnect", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, "tenant-a"))
	rr := httptest.NewRecorder()

	handler.HandleOAuthDisconnect(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%q", rr.Code, rr.Body.String())
	}

	cfgA, err := pA.LoadAIConfig()
	if err != nil {
		t.Fatalf("load tenant-a config: %v", err)
	}
	if cfgA.OAuthAccessToken != "" || cfgA.OAuthRefreshToken != "" || cfgA.AuthMethod != config.AuthMethodAPIKey {
		t.Fatalf("expected tenant-a OAuth cleared, got auth=%q token=%q refresh=%q", cfgA.AuthMethod, cfgA.OAuthAccessToken, cfgA.OAuthRefreshToken)
	}

	cfgB, err := pB.LoadAIConfig()
	if err != nil {
		t.Fatalf("load tenant-b config: %v", err)
	}
	if cfgB.AuthMethod != config.AuthMethodOAuth {
		t.Fatalf("expected tenant-b auth method %q, got %q", config.AuthMethodOAuth, cfgB.AuthMethod)
	}
	if cfgB.OAuthAccessToken != "access-b" {
		t.Fatalf("expected tenant-b access token %q, got %q", "access-b", cfgB.OAuthAccessToken)
	}
	if cfgB.OAuthRefreshToken != "refresh-b" {
		t.Fatalf("expected tenant-b refresh token %q, got %q", "refresh-b", cfgB.OAuthRefreshToken)
	}
}
