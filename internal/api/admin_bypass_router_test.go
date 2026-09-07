package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestAdminBypassPreservesExplicitTokenAuthority(t *testing.T) {
	t.Setenv("ALLOW_ADMIN_BYPASS", "1")
	t.Setenv("PULSE_DEV", "true")
	resetAdminBypassState()
	t.Cleanup(resetAdminBypassState)
	const raw = "scoped-runner-auth-test.12345678"
	record := newTokenRecord(t, raw, []string{config.ScopeAgentExec}, nil)
	record.OrgID = "default"
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "test")
	t.Cleanup(router.shutdownBackgroundWorkers)
	t.Cleanup(router.ShutdownResourceStores)
	t.Cleanup(router.ShutdownRBAC)
	router.mux.HandleFunc("/api/qualification-runner-token", RequireAuth(cfg, RequireScope(config.ScopeAgentExec, func(w http.ResponseWriter, req *http.Request) {
		actual := getAPITokenRecordFromRequest(req)
		if actual == nil || actual.ID != record.ID || isAdminBypassRequest(req.Context()) {
			t.Error("explicit runner identity was replaced by development authority")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})))
	router.mux.HandleFunc("/api/qualification-runner-admin", RequireAuth(cfg, RequireScope(config.ScopeSettingsWrite, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})))
	for _, tc := range []struct {
		name, header, token, path string
		status                    int
	}{
		{"bearer identity", "Authorization", "Bearer " + raw, "/api/qualification-runner-token", http.StatusNoContent},
		{"header identity", "X-API-Token", raw, "/api/qualification-runner-token", http.StatusNoContent},
		{"invalid bearer", "Authorization", "Bearer invalid", "/api/qualification-runner-token", http.StatusUnauthorized},
		{"empty explicit token", "X-API-Token", "", "/api/qualification-runner-token", http.StatusUnauthorized},
		{"scope is retained", "Authorization", "Bearer " + raw, "/api/qualification-runner-admin", http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPatch, tc.path, nil)
			req.Header.Set(tc.header, tc.token)
			rec := httptest.NewRecorder()
			router.Handler().ServeHTTP(rec, req)
			if rec.Code != tc.status {
				t.Fatalf("response = %d %s, want %d", rec.Code, rec.Body.String(), tc.status)
			}
		})
	}
}

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
