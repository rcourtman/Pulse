package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestHandleOIDCLogin_DisabledGetRedirect(t *testing.T) {
	router := &Router{config: &config.Config{OIDC: &config.OIDCConfig{Enabled: false}}}

	req := httptest.NewRequest(http.MethodGet, "/api/oidc/login", nil)
	rec := httptest.NewRecorder()

	router.handleOIDCLogin(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", rec.Code)
	}
	location := rec.Header().Get("Location")
	if !strings.Contains(location, "oidc=error") || !strings.Contains(location, "oidc_error=oidc_disabled") {
		t.Fatalf("unexpected redirect location: %q", location)
	}
}

func TestHandleOIDCLogin_DisabledPost(t *testing.T) {
	router := &Router{config: &config.Config{OIDC: &config.OIDCConfig{Enabled: false}}}

	req := httptest.NewRequest(http.MethodPost, "/api/oidc/login", nil)
	rec := httptest.NewRecorder()

	router.handleOIDCLogin(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}

	var payload struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Code != "oidc_disabled" {
		t.Fatalf("code = %q, want oidc_disabled", payload.Code)
	}
}

func TestHandleOIDCCallback_Disabled(t *testing.T) {
	router := &Router{config: &config.Config{OIDC: &config.OIDCConfig{Enabled: false}}}

	req := httptest.NewRequest(http.MethodGet, "/api/oidc/callback", nil)
	rec := httptest.NewRecorder()

	router.handleOIDCCallback(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestGetOIDCService_Disabled(t *testing.T) {
	router := &Router{config: &config.Config{OIDC: &config.OIDCConfig{Enabled: false}}}

	if _, err := router.getOIDCService(context.Background(), "https://example.com/callback"); err == nil {
		t.Fatalf("expected error when oidc disabled")
	}
}

func TestRedirectOIDCError(t *testing.T) {
	router := &Router{config: &config.Config{}}

	req := httptest.NewRequest(http.MethodGet, "/api/oidc/callback", nil)
	rec := httptest.NewRecorder()

	router.redirectOIDCError(rec, req, "/login?foo=bar", "bad")
	location := rec.Header().Get("Location")
	if !strings.Contains(location, "oidc=error") || !strings.Contains(location, "oidc_error=bad") {
		t.Fatalf("unexpected redirect location: %q", location)
	}
}

func TestEnsureOIDCConfig_Defaults(t *testing.T) {
	cfg := &config.Config{PublicURL: "https://pulse.example.com"}
	router := &Router{config: cfg}

	oidcCfg := router.ensureOIDCConfig()
	if oidcCfg == nil {
		t.Fatalf("expected oidc config to be initialized")
	}
	if oidcCfg.RedirectURL != "https://pulse.example.com"+config.DefaultOIDCCallbackPath {
		t.Fatalf("redirect url = %q, want default", oidcCfg.RedirectURL)
	}
}
