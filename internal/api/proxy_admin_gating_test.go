package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestProxyAuthAdminGatesAdminEndpoints(t *testing.T) {
	t.Setenv("PULSE_DEV", "true")

	record := newTokenRecord(t, "proxy-admin-gate-token-123.12345678", []string{config.ScopeSettingsRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	cfg.ProxyAuthSecret = "proxy-secret"
	cfg.ProxyAuthUserHeader = "X-Remote-User"
	cfg.ProxyAuthRoleHeader = "X-Remote-Roles"
	cfg.ProxyAuthAdminRole = "admin"

	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	cases := []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodGet, path: "/api/logs/stream", body: ""},
		{method: http.MethodGet, path: "/api/logs/download", body: ""},
		{method: http.MethodGet, path: "/api/logs/level", body: ""},
		{method: http.MethodPost, path: "/api/logs/level", body: `{"level":"info"}`},
		{method: http.MethodGet, path: "/api/diagnostics", body: ""},
		{method: http.MethodPost, path: "/api/diagnostics/docker/prepare-token", body: `{}`},
		{method: http.MethodGet, path: "/api/system/settings", body: ""},
		{method: http.MethodPost, path: "/api/system/settings/update", body: `{}`},
		{method: http.MethodPost, path: "/api/security/oidc", body: `{}`},
		{method: http.MethodPost, path: "/api/agents/host/link", body: `{}`},
		{method: http.MethodPost, path: "/api/agents/host/unlink", body: `{}`},
		{method: http.MethodGet, path: "/api/admin/profiles/", body: ""},
	}

	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		req.Header.Set("X-Proxy-Secret", cfg.ProxyAuthSecret)
		req.Header.Set("X-Remote-User", "viewer-user")
		req.Header.Set("X-Remote-Roles", "viewer")
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for non-admin proxy user on %s %s, got %d", tc.method, tc.path, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "Admin") {
			t.Fatalf("expected admin privilege error for %s %s, got %q", tc.method, tc.path, rec.Body.String())
		}
	}
}
