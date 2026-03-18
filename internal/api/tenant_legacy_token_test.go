package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestTenantMiddlewareBlocksLegacyTokenAcrossOrgs(t *testing.T) {
	defer SetMultiTenantEnabled(false)
	SetMultiTenantEnabled(true)
	t.Setenv("PULSE_DEV", "true")

	rawToken := "legacy-org-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
	cfg := newTestConfigWithTokens(t, record)

	baseDir := cfg.DataPath
	for _, orgID := range []string{"org-a", "org-b"} {
		if err := os.MkdirAll(filepath.Join(baseDir, "orgs", orgID), 0o755); err != nil {
			t.Fatalf("create org dir: %v", err)
		}
	}

	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	req.Header.Set("X-Pulse-Org-ID", "org-b")
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for legacy token cross-org access, got %d", rec.Code)
	}
}
