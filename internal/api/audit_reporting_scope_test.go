package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestAuditEndpointsRequireAuditReadScope(t *testing.T) {
	t.Setenv("PULSE_DEV", "true")

	rawToken := "audit-scope-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := newTestRouter(t, cfg, nil, nil, nil, nil, "1.0.0")

	paths := []string{
		"/api/audit",
		"/api/audit/event-1/verify",
		"/api/audit/export",
		"/api/audit/summary",
	}

	for _, path := range paths {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("X-API-Token", rawToken)
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for missing audit:read scope on %s, got %d", path, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), config.ScopeAuditRead) {
			t.Fatalf("expected missing scope response to mention %q, got %q", config.ScopeAuditRead, rec.Body.String())
		}
	}
}

func TestRBACEndpointsRequireFullAccessScope(t *testing.T) {
	t.Setenv("PULSE_DEV", "true")

	cases := []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodGet, path: "/api/admin/roles", body: ""},
		{method: http.MethodPost, path: "/api/admin/roles", body: `{"id":"role-1","name":"Role 1"}`},
		{method: http.MethodPut, path: "/api/admin/roles/role-1", body: `{"id":"role-1","name":"Role 1"}`},
		{method: http.MethodDelete, path: "/api/admin/roles/role-1", body: ""},
		{method: http.MethodGet, path: "/api/admin/users", body: ""},
		{method: http.MethodGet, path: "/api/admin/users/alice/permissions", body: ""},
		{method: http.MethodPut, path: "/api/admin/users/alice/roles", body: `{"roleIds":["admin"]}`},
		{method: http.MethodDelete, path: "/api/admin/users/alice", body: ""},
		{method: http.MethodGet, path: "/api/admin/rbac/integrity", body: ""},
		{method: http.MethodPost, path: "/api/admin/rbac/reset-admin", body: `{}`},
	}

	// An agent install token and the widest settings token both stop short of
	// full access, so neither may read or change who administers the instance.
	for name, scopes := range map[string][]string{
		"agent":    {config.ScopeAgentReport, config.ScopeAgentConfigRead, config.ScopeDockerReport, config.ScopeKubernetesReport},
		"settings": {config.ScopeSettingsRead, config.ScopeSettingsWrite},
	} {
		t.Run(name, func(t *testing.T) {
			rawToken := "rbac-scope-" + name + "-token-123.12345678"
			record := newTokenRecord(t, rawToken, scopes, nil)
			cfg := newTestConfigWithTokens(t, record)
			router := newTestRouter(t, cfg, nil, nil, nil, nil, "1.0.0")

			for _, tc := range cases {
				req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
				req.Header.Set("X-API-Token", rawToken)
				rec := httptest.NewRecorder()
				router.Handler().ServeHTTP(rec, req)
				if rec.Code != http.StatusForbidden {
					t.Fatalf("expected 403 for missing full-access scope on %s %s, got %d", tc.method, tc.path, rec.Code)
				}
				if !strings.Contains(rec.Body.String(), `"requiredScope":"`+config.ScopeWildcard+`"`) {
					t.Fatalf("expected missing scope response to name %q on %s %s, got %q", config.ScopeWildcard, tc.method, tc.path, rec.Body.String())
				}
			}
		})
	}

	t.Run("full access", func(t *testing.T) {
		rawToken := "rbac-scope-full-access-token-123.12345678"
		record := newTokenRecord(t, rawToken, []string{config.ScopeWildcard}, nil)
		cfg := newTestConfigWithTokens(t, record)
		router := newTestRouter(t, cfg, nil, nil, nil, nil, "1.0.0")

		for _, path := range []string{"/api/admin/roles", "/api/admin/users"} {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.Header.Set("X-API-Token", rawToken)
			rec := httptest.NewRecorder()
			router.Handler().ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200 for a full-access token on %s, got %d %q", path, rec.Code, rec.Body.String())
			}
		}
	})
}

func TestReportingEndpointsRequireSettingsReadScope(t *testing.T) {
	t.Setenv("PULSE_DEV", "true")

	rawToken := "reports-scope-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := newTestRouter(t, cfg, nil, nil, nil, nil, "1.0.0")

	paths := []string{
		"/api/admin/reports/catalog",
		"/api/admin/reports/generate",
		"/api/admin/reports/generate-multi",
		"/api/admin/reports/inventory/vms/export",
		"/api/admin/reports/schedules",
	}

	for _, path := range paths {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("X-API-Token", rawToken)
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for missing settings:read scope on %s, got %d", path, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), config.ScopeSettingsRead) {
			t.Fatalf("expected missing scope response to mention %q, got %q", config.ScopeSettingsRead, rec.Body.String())
		}
	}
}

func TestReportScheduleMutationEndpointsRequireSettingsWriteScope(t *testing.T) {
	t.Setenv("PULSE_DEV", "true")

	rawToken := "reports-write-scope-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := newTestRouter(t, cfg, nil, nil, nil, nil, "1.0.0")

	cases := []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodPost, path: "/api/admin/reports/schedules", body: `{}`},
		{method: http.MethodPut, path: "/api/admin/reports/schedules/schedule-1", body: `{}`},
		{method: http.MethodDelete, path: "/api/admin/reports/schedules/schedule-1", body: ""},
		{method: http.MethodPost, path: "/api/admin/reports/schedules/schedule-1/run", body: ""},
	}

	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		req.Header.Set("X-API-Token", rawToken)
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for missing settings:write scope on %s %s, got %d", tc.method, tc.path, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), config.ScopeSettingsWrite) {
			t.Fatalf("expected missing scope response to mention %q, got %q", config.ScopeSettingsWrite, rec.Body.String())
		}
	}
}

func TestAuditWebhooksRequireSettingsScopes(t *testing.T) {
	t.Setenv("PULSE_DEV", "true")

	t.Run("read scope required", func(t *testing.T) {
		rawToken := "audit-webhooks-read-scope-token-123.12345678"
		record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
		cfg := newTestConfigWithTokens(t, record)
		router := newTestRouter(t, cfg, nil, nil, nil, nil, "1.0.0")

		req := httptest.NewRequest(http.MethodGet, "/api/admin/webhooks/audit", nil)
		req.Header.Set("X-API-Token", rawToken)
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for missing settings:read scope, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), config.ScopeSettingsRead) {
			t.Fatalf("expected missing scope response to mention %q, got %q", config.ScopeSettingsRead, rec.Body.String())
		}
	})

	t.Run("write scope required", func(t *testing.T) {
		rawToken := "audit-webhooks-write-scope-token-123.12345678"
		record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsRead}, nil)
		cfg := newTestConfigWithTokens(t, record)
		router := newTestRouter(t, cfg, nil, nil, nil, nil, "1.0.0")

		req := httptest.NewRequest(http.MethodPost, "/api/admin/webhooks/audit", strings.NewReader(`{"urls":[]}`))
		req.Header.Set("X-API-Token", rawToken)
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for missing settings:write scope, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), config.ScopeSettingsWrite) {
			t.Fatalf("expected missing scope response to mention %q, got %q", config.ScopeSettingsWrite, rec.Body.String())
		}
	})
}
