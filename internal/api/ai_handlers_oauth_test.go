package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func resetOAuthSessions() {
	oauthSessionsMu.Lock()
	oauthSessions = make(map[string]*oauthSessionBinding)
	oauthSessionsMu.Unlock()
}

func TestHandleOAuthStart(t *testing.T) {
	resetOAuthSessions()
	handler := &AISettingsHandler{}
	req := httptest.NewRequest(http.MethodGet, "/api/ai/oauth/start", nil)
	rr := httptest.NewRecorder()

	handler.HandleOAuthStart(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["auth_url"] == "" || resp["state"] == "" {
		t.Fatalf("expected auth_url and state in response")
	}
	if !strings.Contains(resp["auth_url"], "claude.ai/oauth/authorize") {
		t.Fatalf("expected auth_url to contain authorize endpoint")
	}

	oauthSessionsMu.Lock()
	delete(oauthSessions, resp["state"])
	oauthSessionsMu.Unlock()
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

func TestHandleOAuthExchange_MethodNotAllowed(t *testing.T) {
	handler := &AISettingsHandler{}
	req := httptest.NewRequest(http.MethodGet, "/api/ai/oauth/exchange", nil)
	rr := httptest.NewRecorder()

	handler.HandleOAuthExchange(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
}

func TestHandleOAuthExchange_InvalidBody(t *testing.T) {
	handler := &AISettingsHandler{}
	req := httptest.NewRequest(http.MethodPost, "/api/ai/oauth/exchange", strings.NewReader("{"))
	rr := httptest.NewRecorder()

	handler.HandleOAuthExchange(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestHandleOAuthExchange_MissingFields(t *testing.T) {
	handler := &AISettingsHandler{}
	body := []byte(`{"code":"","state":""}`)
	req := httptest.NewRequest(http.MethodPost, "/api/ai/oauth/exchange", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	handler.HandleOAuthExchange(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestHandleOAuthExchange_UnknownState(t *testing.T) {
	resetOAuthSessions()
	handler := &AISettingsHandler{}
	body := []byte(`{"code":"code123","state":"missing"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/ai/oauth/exchange", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	handler.HandleOAuthExchange(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestHandleOAuthExchange_BodySizeLimit(t *testing.T) {
	handler := &AISettingsHandler{}
	// Build valid JSON that exceeds the 4 KB limit so failure is specifically
	// from MaxBytesReader, not from JSON syntax errors.
	longCode := strings.Repeat("a", 8*1024)
	oversized := []byte(`{"code":"` + longCode + `","state":"s"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/ai/oauth/exchange", bytes.NewReader(oversized))
	rr := httptest.NewRecorder()

	handler.HandleOAuthExchange(rr, req)
	// MaxBytesReader truncates the stream → json.Decode fails → 400.
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for oversized body, got %d", rr.Code)
	}
}

func TestHandleOAuthExchange_ExpiredSession(t *testing.T) {
	resetOAuthSessions()

	oauthSessionsMu.Lock()
	oauthSessions["expired-state"] = &oauthSessionBinding{
		session: &providers.OAuthSession{
			State:        "expired-state",
			CodeVerifier: "verifier",
			CreatedAt:    time.Now().Add(-oauthSessionTTL - time.Minute),
		},
		orgID: "default",
	}
	oauthSessionsMu.Unlock()

	handler := &AISettingsHandler{}
	body := []byte(`{"code":"code123","state":"expired-state"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/ai/oauth/exchange", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	handler.HandleOAuthExchange(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for expired session, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "expired") {
		t.Fatalf("expected 'expired' in body, got %q", rr.Body.String())
	}
}

func TestHandleOAuthExchange_StripsHashFromCode(t *testing.T) {
	resetOAuthSessions()

	var capturedCode string
	oldExchange := exchangeOAuthCodeForTokens
	exchangeOAuthCodeForTokens = func(ctx context.Context, code string, session *providers.OAuthSession) (*providers.OAuthTokens, error) {
		capturedCode = code
		return &providers.OAuthTokens{
			AccessToken:  "at",
			RefreshToken: "rt",
			ExpiresAt:    time.Now().Add(time.Hour),
		}, nil
	}
	oldCreateAPIKey := createAPIKeyFromOAuth
	createAPIKeyFromOAuth = func(ctx context.Context, accessToken string) (string, error) {
		return "", errors.New("403 org:create_api_key")
	}
	defer func() {
		exchangeOAuthCodeForTokens = oldExchange
		createAPIKeyFromOAuth = oldCreateAPIKey
	}()

	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	if _, err := mtp.GetPersistence("default"); err != nil {
		t.Fatalf("create default persistence: %v", err)
	}
	handler := NewAISettingsHandler(mtp, nil, nil)

	oauthSessionsMu.Lock()
	oauthSessions["state-hash"] = &oauthSessionBinding{
		session: &providers.OAuthSession{
			State:        "state-hash",
			CodeVerifier: "verifier",
			CreatedAt:    time.Now(),
		},
		orgID: "default",
	}
	oauthSessionsMu.Unlock()

	// Anthropic displays code as "code#state" — handler should strip #state.
	body := []byte(`{"code":"  realcode#extrapart  ","state":"state-hash"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/ai/oauth/exchange", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	handler.HandleOAuthExchange(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%q", rr.Code, rr.Body.String())
	}
	if capturedCode != "realcode" {
		t.Fatalf("expected code='realcode' after hash stripping, got %q", capturedCode)
	}
}

func TestHandleOAuthExchange_TokenExchangeFailure(t *testing.T) {
	resetOAuthSessions()

	oldExchange := exchangeOAuthCodeForTokens
	exchangeOAuthCodeForTokens = func(ctx context.Context, code string, session *providers.OAuthSession) (*providers.OAuthTokens, error) {
		return nil, errors.New("token exchange failed: invalid_grant")
	}
	defer func() { exchangeOAuthCodeForTokens = oldExchange }()

	oauthSessionsMu.Lock()
	oauthSessions["state-fail"] = &oauthSessionBinding{
		session: &providers.OAuthSession{
			State:        "state-fail",
			CodeVerifier: "verifier",
			CreatedAt:    time.Now(),
		},
		orgID: "default",
	}
	oauthSessionsMu.Unlock()

	handler := &AISettingsHandler{}
	body := []byte(`{"code":"badcode","state":"state-fail"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/ai/oauth/exchange", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	handler.HandleOAuthExchange(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for token exchange failure, got %d", rr.Code)
	}
}

func TestHandleOAuthExchange_APIKeyCreatedSuccessfully(t *testing.T) {
	resetOAuthSessions()

	oldExchange := exchangeOAuthCodeForTokens
	exchangeOAuthCodeForTokens = func(ctx context.Context, code string, session *providers.OAuthSession) (*providers.OAuthTokens, error) {
		return &providers.OAuthTokens{
			AccessToken:  "access-token",
			RefreshToken: "refresh-token",
			ExpiresAt:    time.Now().Add(time.Hour),
		}, nil
	}
	oldCreateAPIKey := createAPIKeyFromOAuth
	createAPIKeyFromOAuth = func(ctx context.Context, accessToken string) (string, error) {
		return "sk-ant-generated-key", nil
	}
	defer func() {
		exchangeOAuthCodeForTokens = oldExchange
		createAPIKeyFromOAuth = oldCreateAPIKey
	}()

	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	if _, err := mtp.GetPersistence("default"); err != nil {
		t.Fatalf("create default persistence: %v", err)
	}
	handler := NewAISettingsHandler(mtp, nil, nil)

	oauthSessionsMu.Lock()
	oauthSessions["state-apikey"] = &oauthSessionBinding{
		session: &providers.OAuthSession{
			State:        "state-apikey",
			CodeVerifier: "verifier",
			CreatedAt:    time.Now(),
		},
		orgID: "default",
	}
	oauthSessionsMu.Unlock()

	body := []byte(`{"code":"goodcode","state":"state-apikey"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/ai/oauth/exchange", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	handler.HandleOAuthExchange(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%q", rr.Code, rr.Body.String())
	}

	// Verify the API key was saved.
	p, _ := mtp.GetPersistence("default")
	cfg, err := p.LoadAIConfig()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.APIKey != "sk-ant-generated-key" {
		t.Fatalf("expected API key 'sk-ant-generated-key', got %q", cfg.APIKey)
	}
	if cfg.AuthMethod != config.AuthMethodOAuth {
		t.Fatalf("expected auth method OAuth, got %q", cfg.AuthMethod)
	}
}

func TestHandleOAuthCallback_ErrorParam(t *testing.T) {
	handler := &AISettingsHandler{}
	req := httptest.NewRequest(http.MethodGet, "/api/ai/oauth/callback?error=access_denied&error_description=no", nil)
	rr := httptest.NewRecorder()

	handler.HandleOAuthCallback(rr, req)
	if rr.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected 307, got %d", rr.Code)
	}
	location := rr.Header().Get("Location")
	if !strings.Contains(location, "ai_oauth_error=access_denied") {
		t.Fatalf("expected redirect to include error, got %q", location)
	}
}

func TestHandleOAuthCallback_MissingParams(t *testing.T) {
	handler := &AISettingsHandler{}
	req := httptest.NewRequest(http.MethodGet, "/api/ai/oauth/callback?code=abc", nil)
	rr := httptest.NewRecorder()

	handler.HandleOAuthCallback(rr, req)
	if rr.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected 307, got %d", rr.Code)
	}
	location := rr.Header().Get("Location")
	if !strings.Contains(location, "ai_oauth_error=missing_params") {
		t.Fatalf("expected missing_params redirect, got %q", location)
	}
}

func TestHandleOAuthCallback_InvalidState(t *testing.T) {
	resetOAuthSessions()
	handler := &AISettingsHandler{}
	req := httptest.NewRequest(http.MethodGet, "/api/ai/oauth/callback?code=abc&state=missing", nil)
	rr := httptest.NewRecorder()

	handler.HandleOAuthCallback(rr, req)
	if rr.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected 307, got %d", rr.Code)
	}
	location := rr.Header().Get("Location")
	if !strings.Contains(location, "ai_oauth_error=invalid_state") {
		t.Fatalf("expected invalid_state redirect, got %q", location)
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
	// With nil config, CheckAuth returns 503 Service Unavailable.
	handler := &AISettingsHandler{}
	req := httptest.NewRequest(http.MethodPost, "/api/ai/oauth/disconnect", nil)
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

	// Pre-populate with OAuth tokens so we can verify they get cleared.
	aiCfg := config.NewDefaultAIConfig()
	aiCfg.AuthMethod = config.AuthMethodOAuth
	aiCfg.OAuthAccessToken = "access-token-123"
	aiCfg.OAuthRefreshToken = "refresh-token-456"
	aiCfg.OAuthExpiresAt = time.Now().Add(time.Hour)
	if err := p.SaveAIConfig(*aiCfg); err != nil {
		t.Fatalf("seed AI config: %v", err)
	}

	handler := NewAISettingsHandler(mtp, nil, nil)
	// Set a config with no auth credentials so CheckAuth passes as anonymous.
	handler.SetConfig(&config.Config{})

	req := httptest.NewRequest(http.MethodPost, "/api/ai/oauth/disconnect", nil)
	rr := httptest.NewRecorder()

	handler.HandleOAuthDisconnect(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%q", rr.Code, rr.Body.String())
	}

	// Verify JSON response.
	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["success"] != true {
		t.Fatalf("expected success=true, got %v", resp["success"])
	}

	// Verify OAuth tokens were cleared and auth method reverted to API key.
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
	// Handler with nil persistence should return 500 when loading config fails.
	handler := NewAISettingsHandler(nil, nil, nil)
	handler.SetConfig(&config.Config{})

	req := httptest.NewRequest(http.MethodPost, "/api/ai/oauth/disconnect", nil)
	rr := httptest.NewRecorder()

	handler.HandleOAuthDisconnect(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for nil persistence, got %d body=%q", rr.Code, rr.Body.String())
	}
}

func TestHandleOAuthDisconnect_TenantIsolation(t *testing.T) {
	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)

	// Set up two tenants with OAuth tokens.
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

	// Disconnect tenant-a's OAuth.
	req := httptest.NewRequest(http.MethodPost, "/api/ai/oauth/disconnect", nil)
	req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, "tenant-a"))
	rr := httptest.NewRecorder()

	handler.HandleOAuthDisconnect(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%q", rr.Code, rr.Body.String())
	}

	// Verify tenant-a tokens are cleared.
	cfgA, err := pA.LoadAIConfig()
	if err != nil {
		t.Fatalf("load tenant-a config: %v", err)
	}
	if cfgA.OAuthAccessToken != "" || cfgA.OAuthRefreshToken != "" || cfgA.AuthMethod != config.AuthMethodAPIKey {
		t.Fatalf("expected tenant-a OAuth cleared, got auth=%q token=%q refresh=%q", cfgA.AuthMethod, cfgA.OAuthAccessToken, cfgA.OAuthRefreshToken)
	}

	// Verify tenant-b tokens are completely untouched (exact values preserved).
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

func TestHandleOAuthStart_BindsSessionToOrgFromContext(t *testing.T) {
	resetOAuthSessions()
	handler := &AISettingsHandler{}

	req := httptest.NewRequest(http.MethodGet, "/api/ai/oauth/start", nil)
	req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, "tenant-a"))
	rr := httptest.NewRecorder()

	handler.HandleOAuthStart(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	state := strings.TrimSpace(resp["state"])
	if state == "" {
		t.Fatalf("expected state in response")
	}

	oauthSessionsMu.Lock()
	binding, ok := oauthSessions[state]
	oauthSessionsMu.Unlock()
	if !ok || binding == nil {
		t.Fatalf("expected OAuth session binding for state %q", state)
	}
	if binding.orgID != "tenant-a" {
		t.Fatalf("expected bound org tenant-a, got %q", binding.orgID)
	}
}

func TestConsumeOAuthSession_RejectsExpiredSession(t *testing.T) {
	resetOAuthSessions()

	oauthSessionsMu.Lock()
	oauthSessions["expired-state"] = &oauthSessionBinding{
		session: &providers.OAuthSession{
			State:        "expired-state",
			CodeVerifier: "verifier",
			CreatedAt:    time.Now().Add(-oauthSessionTTL - time.Minute),
		},
		orgID: "default",
	}
	oauthSessionsMu.Unlock()

	if binding, ok := consumeOAuthSession("expired-state"); ok || binding != nil {
		t.Fatalf("expected expired session to be rejected")
	}
}

func TestHandleOAuthCallback_UsesSessionBoundOrgForSave(t *testing.T) {
	resetOAuthSessions()

	oldExchange := exchangeOAuthCodeForTokens
	exchangeOAuthCodeForTokens = func(ctx context.Context, code string, session *providers.OAuthSession) (*providers.OAuthTokens, error) {
		return &providers.OAuthTokens{
			AccessToken:  "access-token-a",
			RefreshToken: "refresh-token-a",
			ExpiresAt:    time.Now().Add(time.Hour),
		}, nil
	}
	defer func() { exchangeOAuthCodeForTokens = oldExchange }()

	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	if _, err := mtp.GetPersistence("tenant-a"); err != nil {
		t.Fatalf("create tenant-a persistence: %v", err)
	}
	if _, err := mtp.GetPersistence("tenant-b"); err != nil {
		t.Fatalf("create tenant-b persistence: %v", err)
	}
	handler := NewAISettingsHandler(mtp, nil, nil)

	oauthSessionsMu.Lock()
	oauthSessions["state-a"] = &oauthSessionBinding{
		session: &providers.OAuthSession{
			State:        "state-a",
			CodeVerifier: "verifier-a",
			CreatedAt:    time.Now(),
		},
		orgID: "tenant-a",
	}
	oauthSessionsMu.Unlock()

	req := httptest.NewRequest(http.MethodGet, "/api/ai/oauth/callback?code=code-a&state=state-a", nil)
	req.AddCookie(&http.Cookie{Name: "pulse_org_id", Value: "tenant-b"})
	rr := httptest.NewRecorder()

	handler.HandleOAuthCallback(rr, req)

	if rr.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected 307, got %d", rr.Code)
	}
	if got := rr.Header().Get("Location"); !strings.Contains(got, "ai_oauth_success=true") {
		t.Fatalf("expected success redirect, got %q", got)
	}

	pA, _ := mtp.GetPersistence("tenant-a")
	cfgA, err := pA.LoadAIConfig()
	if err != nil {
		t.Fatalf("load tenant-a config: %v", err)
	}
	if cfgA.AuthMethod != config.AuthMethodOAuth || cfgA.OAuthAccessToken != "access-token-a" {
		t.Fatalf("expected tenant-a OAuth config to be updated, got auth=%q token=%q", cfgA.AuthMethod, cfgA.OAuthAccessToken)
	}

	pB, _ := mtp.GetPersistence("tenant-b")
	cfgB, err := pB.LoadAIConfig()
	if err != nil {
		t.Fatalf("load tenant-b config: %v", err)
	}
	if cfgB.OAuthAccessToken != "" || cfgB.AuthMethod != config.AuthMethodAPIKey {
		t.Fatalf("expected tenant-b OAuth config to remain unchanged, got auth=%q token=%q", cfgB.AuthMethod, cfgB.OAuthAccessToken)
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

func TestHandleOAuthCallback_ExpiredSession(t *testing.T) {
	resetOAuthSessions()

	oauthSessionsMu.Lock()
	oauthSessions["expired-cb"] = &oauthSessionBinding{
		session: &providers.OAuthSession{
			State:        "expired-cb",
			CodeVerifier: "verifier",
			CreatedAt:    time.Now().Add(-oauthSessionTTL - time.Minute),
		},
		orgID: "default",
	}
	oauthSessionsMu.Unlock()

	handler := &AISettingsHandler{}
	req := httptest.NewRequest(http.MethodGet, "/api/ai/oauth/callback?code=abc&state=expired-cb", nil)
	rr := httptest.NewRecorder()

	handler.HandleOAuthCallback(rr, req)
	if rr.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected 307, got %d", rr.Code)
	}
	location := rr.Header().Get("Location")
	if !strings.Contains(location, "ai_oauth_error=invalid_state") {
		t.Fatalf("expected invalid_state redirect for expired session, got %q", location)
	}
}

func TestHandleOAuthCallback_TokenExchangeFailure(t *testing.T) {
	resetOAuthSessions()

	oldExchange := exchangeOAuthCodeForTokens
	exchangeOAuthCodeForTokens = func(ctx context.Context, code string, session *providers.OAuthSession) (*providers.OAuthTokens, error) {
		return nil, errors.New("token exchange failed")
	}
	defer func() { exchangeOAuthCodeForTokens = oldExchange }()

	oauthSessionsMu.Lock()
	oauthSessions["state-fail-cb"] = &oauthSessionBinding{
		session: &providers.OAuthSession{
			State:        "state-fail-cb",
			CodeVerifier: "verifier",
			CreatedAt:    time.Now(),
		},
		orgID: "default",
	}
	oauthSessionsMu.Unlock()

	handler := &AISettingsHandler{}
	req := httptest.NewRequest(http.MethodGet, "/api/ai/oauth/callback?code=badcode&state=state-fail-cb", nil)
	rr := httptest.NewRecorder()

	handler.HandleOAuthCallback(rr, req)
	if rr.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected 307, got %d", rr.Code)
	}
	location := rr.Header().Get("Location")
	if !strings.Contains(location, "ai_oauth_error=token_exchange_failed") {
		t.Fatalf("expected token_exchange_failed redirect, got %q", location)
	}
}

func TestHandleOAuthCallback_ErrorParamURLEncoded(t *testing.T) {
	handler := &AISettingsHandler{}
	// Inject special characters in the error param to verify URL-encoding
	req := httptest.NewRequest(http.MethodGet, "/api/ai/oauth/callback?error=bad%26inject%3Dfoo&error_description=test", nil)
	rr := httptest.NewRecorder()

	handler.HandleOAuthCallback(rr, req)
	if rr.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected 307, got %d", rr.Code)
	}
	location := rr.Header().Get("Location")
	// Parse the redirect URL and verify the error value round-trips correctly.
	parsed, err := url.Parse(location)
	if err != nil {
		t.Fatalf("failed to parse redirect URL %q: %v", location, err)
	}
	qvals := parsed.Query()
	// Only one query key should be present — injection would add extra keys.
	if len(qvals) != 1 {
		t.Fatalf("expected exactly 1 query key, got %d: %v", len(qvals), qvals)
	}
	got := qvals.Get("ai_oauth_error")
	if got != "bad&inject=foo" {
		t.Fatalf("expected decoded error value %q, got %q", "bad&inject=foo", got)
	}
}

func TestHandleOAuthCallback_SessionConsumedOnce(t *testing.T) {
	resetOAuthSessions()

	oldExchange := exchangeOAuthCodeForTokens
	exchangeOAuthCodeForTokens = func(ctx context.Context, code string, session *providers.OAuthSession) (*providers.OAuthTokens, error) {
		return &providers.OAuthTokens{
			AccessToken:  "at",
			RefreshToken: "rt",
			ExpiresAt:    time.Now().Add(time.Hour),
		}, nil
	}
	defer func() { exchangeOAuthCodeForTokens = oldExchange }()

	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	if _, err := mtp.GetPersistence("default"); err != nil {
		t.Fatalf("create default persistence: %v", err)
	}
	handler := NewAISettingsHandler(mtp, nil, nil)

	oauthSessionsMu.Lock()
	oauthSessions["state-once"] = &oauthSessionBinding{
		session: &providers.OAuthSession{
			State:        "state-once",
			CodeVerifier: "verifier",
			CreatedAt:    time.Now(),
		},
		orgID: "default",
	}
	oauthSessionsMu.Unlock()

	// First call should succeed
	req1 := httptest.NewRequest(http.MethodGet, "/api/ai/oauth/callback?code=abc&state=state-once", nil)
	rr1 := httptest.NewRecorder()
	handler.HandleOAuthCallback(rr1, req1)
	if !strings.Contains(rr1.Header().Get("Location"), "ai_oauth_success=true") {
		t.Fatalf("first call should succeed, got %q", rr1.Header().Get("Location"))
	}

	// Second call with same state should fail (session consumed)
	req2 := httptest.NewRequest(http.MethodGet, "/api/ai/oauth/callback?code=abc&state=state-once", nil)
	rr2 := httptest.NewRecorder()
	handler.HandleOAuthCallback(rr2, req2)
	if !strings.Contains(rr2.Header().Get("Location"), "ai_oauth_error=invalid_state") {
		t.Fatalf("second call should fail with invalid_state, got %q", rr2.Header().Get("Location"))
	}
}

func TestHandleOAuthExchange_UsesSessionBoundOrgForSave(t *testing.T) {
	resetOAuthSessions()

	oldExchange := exchangeOAuthCodeForTokens
	oldCreateAPIKey := createAPIKeyFromOAuth
	exchangeOAuthCodeForTokens = func(ctx context.Context, code string, session *providers.OAuthSession) (*providers.OAuthTokens, error) {
		return &providers.OAuthTokens{
			AccessToken:  "access-token-a",
			RefreshToken: "refresh-token-a",
			ExpiresAt:    time.Now().Add(time.Hour),
		}, nil
	}
	createAPIKeyFromOAuth = func(ctx context.Context, accessToken string) (string, error) {
		return "", errors.New("403 org:create_api_key")
	}
	defer func() {
		exchangeOAuthCodeForTokens = oldExchange
		createAPIKeyFromOAuth = oldCreateAPIKey
	}()

	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	if _, err := mtp.GetPersistence("tenant-a"); err != nil {
		t.Fatalf("create tenant-a persistence: %v", err)
	}
	if _, err := mtp.GetPersistence("tenant-b"); err != nil {
		t.Fatalf("create tenant-b persistence: %v", err)
	}
	handler := NewAISettingsHandler(mtp, nil, nil)

	oauthSessionsMu.Lock()
	oauthSessions["state-a"] = &oauthSessionBinding{
		session: &providers.OAuthSession{
			State:        "state-a",
			CodeVerifier: "verifier-a",
			CreatedAt:    time.Now(),
		},
		orgID: "tenant-a",
	}
	oauthSessionsMu.Unlock()

	body := []byte(`{"code":"code-a","state":"state-a"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/ai/oauth/exchange", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "pulse_org_id", Value: "tenant-b"})
	rr := httptest.NewRecorder()

	handler.HandleOAuthExchange(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%q", rr.Code, rr.Body.String())
	}

	pA, _ := mtp.GetPersistence("tenant-a")
	cfgA, err := pA.LoadAIConfig()
	if err != nil {
		t.Fatalf("load tenant-a config: %v", err)
	}
	if cfgA.AuthMethod != config.AuthMethodOAuth || cfgA.OAuthAccessToken != "access-token-a" {
		t.Fatalf("expected tenant-a OAuth config to be updated, got auth=%q token=%q", cfgA.AuthMethod, cfgA.OAuthAccessToken)
	}

	pB, _ := mtp.GetPersistence("tenant-b")
	cfgB, err := pB.LoadAIConfig()
	if err != nil {
		t.Fatalf("load tenant-b config: %v", err)
	}
	if cfgB.OAuthAccessToken != "" || cfgB.AuthMethod != config.AuthMethodAPIKey {
		t.Fatalf("expected tenant-b OAuth config to remain unchanged, got auth=%q token=%q", cfgB.AuthMethod, cfgB.OAuthAccessToken)
	}
}
