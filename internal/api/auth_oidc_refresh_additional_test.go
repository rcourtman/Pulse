package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func newOIDCTestServer(t *testing.T, tokenStatus int, tokenBody map[string]interface{}) *httptest.Server {
	t.Helper()

	return newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		baseURL := scheme + "://" + r.Host

		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"issuer":                 baseURL,
				"authorization_endpoint": baseURL + "/auth",
				"token_endpoint":         baseURL + "/token",
				"jwks_uri":               baseURL + "/jwks",
			})
		case "/jwks":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"keys": []interface{}{}})
		case "/token":
			if r.Method != http.MethodPost {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(tokenStatus)
			if tokenBody != nil {
				_ = json.NewEncoder(w).Encode(tokenBody)
			}
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestRefreshOIDCSessionTokens_Success(t *testing.T) {
	InitSessionStore(t.TempDir())
	store := GetSessionStore()

	tokenResp := map[string]interface{}{
		"access_token":  "new-access",
		"refresh_token": "new-refresh",
		"expires_in":    3600,
		"token_type":    "Bearer",
	}
	server := newOIDCTestServer(t, http.StatusOK, tokenResp)
	defer server.Close()

	cfg := &config.Config{
		OIDC: &config.OIDCConfig{
			Enabled:      true,
			IssuerURL:    server.URL,
			ClientID:     "client",
			ClientSecret: "secret",
			RedirectURL:  "http://localhost/callback",
			Scopes:       []string{"openid"},
		},
	}

	sessionToken := "oidc-session-success"
	store.CreateOIDCSession(sessionToken, time.Hour, "agent", "127.0.0.1", "user", &OIDCTokenInfo{
		RefreshToken:   "old-refresh",
		AccessTokenExp: time.Now().Add(-time.Minute),
		Issuer:         server.URL,
		ClientID:       "client",
	})

	session := store.GetSession(sessionToken)
	if session == nil {
		t.Fatalf("expected session to exist")
	}

	refreshOIDCSessionTokens(cfg, sessionToken, session)

	updated := store.GetSession(sessionToken)
	if updated == nil {
		t.Fatalf("expected session to remain after refresh")
	}
	if updated.OIDCRefreshToken != "new-refresh" {
		t.Fatalf("expected refresh token to update, got %q", updated.OIDCRefreshToken)
	}
	if time.Until(updated.OIDCAccessTokenExp) <= 0 {
		t.Fatalf("expected access token expiry to be in the future")
	}
}

func TestRefreshOIDCSessionTokens_IssuerMismatchSkipsRefresh(t *testing.T) {
	InitSessionStore(t.TempDir())
	store := GetSessionStore()

	cfg := &config.Config{
		OIDC: &config.OIDCConfig{
			Enabled:   true,
			IssuerURL: "https://issuer.example",
		},
	}

	sessionToken := "oidc-session-mismatch"
	store.CreateOIDCSession(sessionToken, time.Hour, "agent", "127.0.0.1", "user", &OIDCTokenInfo{
		RefreshToken:   "refresh",
		AccessTokenExp: time.Now().Add(-time.Minute),
		Issuer:         "https://different-issuer",
		ClientID:       "client",
	})

	session := store.GetSession(sessionToken)
	if session == nil {
		t.Fatalf("expected session to exist")
	}

	refreshOIDCSessionTokens(cfg, sessionToken, session)

	// Session should survive — the issuer mismatch means this is likely an SSO
	// OIDC session managed by a different provider. Skip refresh, don't invalidate.
	if store.GetSession(sessionToken) == nil {
		t.Fatalf("expected session to remain when issuer does not match legacy OIDC config")
	}
}

func TestRefreshOIDCSessionTokens_RefreshFailureInvalidates(t *testing.T) {
	InitSessionStore(t.TempDir())
	store := GetSessionStore()

	tokenResp := map[string]interface{}{
		"error":             "invalid_grant",
		"error_description": "refresh token expired",
	}
	server := newOIDCTestServer(t, http.StatusBadRequest, tokenResp)
	defer server.Close()

	cfg := &config.Config{
		OIDC: &config.OIDCConfig{
			Enabled:      true,
			IssuerURL:    server.URL,
			ClientID:     "client",
			ClientSecret: "secret",
			RedirectURL:  "http://localhost/callback",
			Scopes:       []string{"openid"},
		},
	}

	sessionToken := "oidc-session-failure"
	store.CreateOIDCSession(sessionToken, time.Hour, "agent", "127.0.0.1", "user", &OIDCTokenInfo{
		RefreshToken:   "refresh",
		AccessTokenExp: time.Now().Add(-time.Minute),
		Issuer:         server.URL,
		ClientID:       "client",
	})

	session := store.GetSession(sessionToken)
	if session == nil {
		t.Fatalf("expected session to exist")
	}

	refreshOIDCSessionTokens(cfg, sessionToken, session)

	if store.GetSession(sessionToken) != nil {
		t.Fatalf("expected session to be invalidated after refresh failure")
	}
}

func TestRefreshOIDCSessionTokens_OIDCDisabledDoesNotInvalidate(t *testing.T) {
	InitSessionStore(t.TempDir())
	store := GetSessionStore()

	cfg := &config.Config{
		OIDC: &config.OIDCConfig{
			Enabled: false,
		},
	}

	sessionToken := "oidc-session-disabled"
	store.CreateOIDCSession(sessionToken, time.Hour, "agent", "127.0.0.1", "user", &OIDCTokenInfo{
		RefreshToken:   "refresh",
		AccessTokenExp: time.Now().Add(-time.Minute),
		Issuer:         "https://issuer.example",
		ClientID:       "client",
	})

	session := store.GetSession(sessionToken)
	if session == nil {
		t.Fatalf("expected session to exist")
	}

	refreshOIDCSessionTokens(cfg, sessionToken, session)

	if store.GetSession(sessionToken) == nil {
		t.Fatalf("expected session to remain when OIDC is disabled")
	}
}

func TestNewOIDCTestServer_IssuerFieldUsesURL(t *testing.T) {
	server := newOIDCTestServer(t, http.StatusOK, nil)
	defer server.Close()

	resp, err := http.Get(server.URL + "/.well-known/openid-configuration")
	if err != nil {
		t.Fatalf("failed to fetch discovery doc: %v", err)
	}
	defer resp.Body.Close()

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode discovery doc: %v", err)
	}

	issuer := body["issuer"]
	if issuer == "" {
		t.Fatalf("expected issuer in discovery doc")
	}
	if !strings.Contains(issuer, "http://") {
		t.Fatalf("expected issuer to include scheme, got %q", issuer)
	}
	if _, err := url.Parse(issuer); err != nil {
		t.Fatalf("expected issuer to parse as URL: %v", err)
	}
}
