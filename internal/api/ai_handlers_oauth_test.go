package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
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
