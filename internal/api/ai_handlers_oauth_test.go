package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
)

func resetOAuthSessions() {
	oauthSessionsMu.Lock()
	oauthSessions = make(map[string]*providers.OAuthSession)
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
