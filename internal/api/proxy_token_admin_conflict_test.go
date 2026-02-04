package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestProxyAuthNonAdminCannotEscalateWithToken(t *testing.T) {
	record := newTokenRecord(t, "proxy-token-conflict-123.12345678", []string{config.ScopeSettingsRead, config.ScopeSettingsWrite}, nil)
	cfg := newTestConfigWithTokens(t, record)
	cfg.ProxyAuthSecret = "proxy-secret"
	cfg.ProxyAuthUserHeader = "X-Remote-User"
	cfg.ProxyAuthRoleHeader = "X-Remote-Roles"
	cfg.ProxyAuthAdminRole = "admin"

	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/system/settings", nil)
	req.Header.Set("X-Proxy-Secret", cfg.ProxyAuthSecret)
	req.Header.Set("X-Remote-User", "viewer-user")
	req.Header.Set("X-Remote-Roles", "viewer")
	req.Header.Set("X-API-Token", "proxy-token-conflict-123.12345678")
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin proxy user, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Admin") {
		t.Fatalf("expected admin privilege error, got %q", rec.Body.String())
	}
}
