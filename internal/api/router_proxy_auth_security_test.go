package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestProxyAuthNonAdminCannotResetLockout(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())

	cfg := &config.Config{
		ProxyAuthSecret:     "proxy-secret",
		ProxyAuthUserHeader: "X-Proxy-User",
		ProxyAuthRoleHeader: "X-Proxy-Roles",
		ProxyAuthAdminRole:  "admin",
	}

	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	body, err := json.Marshal(map[string]string{"identifier": "user1"})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/security/reset-lockout", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Proxy-Secret", "proxy-secret")
	req.Header.Set("X-Proxy-User", "viewer")
	req.Header.Set("X-Proxy-Roles", "viewer|user")
	rec := httptest.NewRecorder()

	router.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d (%s)", http.StatusForbidden, rec.Code, rec.Body.String())
	}
}

func TestProxyAuthAdminCanResetLockoutWithSettingsScope(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())

	rawToken := "proxy-admin-token-123.12345678"
	record, err := config.NewAPITokenRecord(rawToken, "admin-token", []string{config.ScopeSettingsWrite})
	if err != nil {
		t.Fatalf("new token record: %v", err)
	}

	cfg := &config.Config{
		ProxyAuthSecret:     "proxy-secret",
		ProxyAuthUserHeader: "X-Proxy-User",
		ProxyAuthRoleHeader: "X-Proxy-Roles",
		ProxyAuthAdminRole:  "admin",
		APITokens:           []config.APITokenRecord{*record},
	}
	cfg.SortAPITokens()

	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	body, err := json.Marshal(map[string]string{"identifier": "user1"})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/security/reset-lockout", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Proxy-Secret", "proxy-secret")
	req.Header.Set("X-Proxy-User", "adminuser")
	req.Header.Set("X-Proxy-Roles", "viewer|admin")
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()

	router.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d (%s)", http.StatusOK, rec.Code, rec.Body.String())
	}
}

func TestSecurityStatusAccessibleWithProxyAuth(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())

	cfg := &config.Config{
		ProxyAuthSecret:     "proxy-secret",
		ProxyAuthUserHeader: "X-Proxy-User",
		ProxyAuthRoleHeader: "X-Proxy-Roles",
		ProxyAuthAdminRole:  "admin",
	}

	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/security/status", nil)
	req.Header.Set("X-Proxy-Secret", "proxy-secret")
	req.Header.Set("X-Proxy-User", "viewer")
	req.Header.Set("X-Proxy-Roles", "viewer|user")
	rec := httptest.NewRecorder()

	router.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d (%s)", http.StatusOK, rec.Code, rec.Body.String())
	}
}
