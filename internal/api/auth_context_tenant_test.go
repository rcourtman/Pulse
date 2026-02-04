package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

func TestExtractAndStoreAuthContext_UsesTenantConfigForToken(t *testing.T) {
	t.Setenv("PULSE_MOCK_MODE", "true")
	mock.SetEnabled(true)
	t.Cleanup(func() { mock.SetEnabled(false) })

	baseCfg := &config.Config{
		DataPath:   t.TempDir(),
		ConfigPath: t.TempDir(),
	}

	mtPersistence := config.NewMultiTenantPersistence(t.TempDir())
	mtm := monitoring.NewMultiTenantMonitor(baseCfg, mtPersistence, nil)
	t.Cleanup(mtm.Stop)

	tenantID := "org-1"
	_, err := mtPersistence.GetPersistence(tenantID)
	if err != nil {
		t.Fatalf("tenant persistence: %v", err)
	}

	rawToken := "tenant-token-123.12345678"
	record, err := config.NewAPITokenRecord(rawToken, "tenant", []string{config.ScopeMonitoringRead})
	if err != nil {
		t.Fatalf("new token record: %v", err)
	}
	baseCfg.APITokens = []config.APITokenRecord{*record}
	baseCfg.SortAPITokens()

	// Ensure tenant monitor/config is initialized
	if _, err := mtm.GetMonitor(tenantID); err != nil {
		t.Fatalf("get tenant monitor: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("X-Pulse-Org-ID", tenantID)
	req.Header.Set("X-API-Token", rawToken)

	req = extractAndStoreAuthContext(baseCfg, mtm, req)

	user := internalauth.GetUser(req.Context())
	if user == "" {
		t.Fatalf("expected user context to be set")
	}
	if internalauth.GetAPIToken(req.Context()) == nil {
		t.Fatalf("expected API token record in context")
	}
}
