package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestAdminEndpointsRequireAuthWhenOIDCEnabled(t *testing.T) {
	t.Setenv("ALLOW_ADMIN_BYPASS", "")
	t.Setenv("PULSE_DEV", "")
	t.Setenv("NODE_ENV", "")
	resetAdminBypassState()

	dataDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   dataDir,
		ConfigPath: dataDir,
		OIDC: &config.OIDCConfig{
			Enabled: true,
		},
	}

	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	paths := []string{
		"/api/system/settings",
		"/api/diagnostics",
	}

	for _, path := range paths {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 without auth for %s, got %d", path, rec.Code)
		}
	}
}
