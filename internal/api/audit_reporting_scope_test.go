package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestAuditEndpointsRequireSettingsReadScope(t *testing.T) {
	t.Setenv("PULSE_DEV", "true")

	rawToken := "audit-scope-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0", nil)

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
			t.Fatalf("expected 403 for missing settings:read scope on %s, got %d", path, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), config.ScopeSettingsRead) {
			t.Fatalf("expected missing scope response to mention %q, got %q", config.ScopeSettingsRead, rec.Body.String())
		}
	}
}

func TestReportingEndpointsRequireSettingsReadScope(t *testing.T) {
	t.Setenv("PULSE_DEV", "true")

	rawToken := "reports-scope-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0", nil)

	paths := []string{
		"/api/admin/reports/generate",
		"/api/admin/reports/generate-multi",
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

func TestAuditWebhooksRequireSettingsScopes(t *testing.T) {
	t.Setenv("PULSE_DEV", "true")

	t.Run("read scope required", func(t *testing.T) {
		rawToken := "audit-webhooks-read-scope-token-123.12345678"
		record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
		cfg := newTestConfigWithTokens(t, record)
		router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0", nil)

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
		router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0", nil)

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
