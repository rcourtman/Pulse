package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAdminBypassDoesNotAllowAdminEndpointsByDefault(t *testing.T) {
	// Ensure bypass is not enabled
	t.Setenv("ALLOW_ADMIN_BYPASS", "")
	t.Setenv("PULSE_DEV", "")
	t.Setenv("NODE_ENV", "")
	resetAdminBypassState()

	record := newTokenRecord(t, "admin-bypass-test-token-123.12345678", nil, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/system/settings", nil)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth, got %d", rec.Code)
	}
}

func TestAdminBypassAllowsAdminEndpointInDevMode(t *testing.T) {
	// Enable admin bypass in dev mode
	t.Setenv("ALLOW_ADMIN_BYPASS", "1")
	t.Setenv("PULSE_DEV", "true")
	t.Setenv("NODE_ENV", "")
	resetAdminBypassState()

	record := newTokenRecord(t, "admin-bypass-dev-token-123.12345678", nil, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/system/settings", nil)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with admin bypass enabled, got %d", rec.Code)
	}
}

func TestAdminBypassRequiresExplicitFlag(t *testing.T) {
	// Dev mode alone should not enable bypass
	t.Setenv("ALLOW_ADMIN_BYPASS", "")
	t.Setenv("PULSE_DEV", "true")
	t.Setenv("NODE_ENV", "")
	resetAdminBypassState()

	record := newTokenRecord(t, "admin-bypass-flag-token-123.12345678", nil, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/system/settings", nil)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without bypass flag, got %d", rec.Code)
	}
}

func TestAdminBypassDeclinedOutsideDevMode(t *testing.T) {
	// ALLOW_ADMIN_BYPASS without dev mode should not bypass
	t.Setenv("ALLOW_ADMIN_BYPASS", "1")
	t.Setenv("PULSE_DEV", "")
	t.Setenv("NODE_ENV", "production")
	resetAdminBypassState()

	record := newTokenRecord(t, "admin-bypass-prod-token-123.12345678", nil, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/system/settings", nil)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when bypass declined, got %d", rec.Code)
	}
}
