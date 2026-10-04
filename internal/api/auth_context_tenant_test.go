package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

func TestExtractAndStoreAuthContext_UsesTenantConfigForToken(t *testing.T) {
	const globalToken = "global-token-123.12345678"
	const tenantToken = "tenant-token-123.12345678"
	for _, tc := range []struct {
		name       string
		global     bool
		tenant     bool
		provided   string
		wantRecord string
	}{
		{"global fallback with empty tenant tokens", true, false, globalToken, "global"},
		{"tenant token with different global tokens", true, true, tenantToken, "tenant"},
		{"unknown token is rejected", true, true, "unknown-token-123.12345678", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			baseCfg := &config.Config{}
			tenantCfg := &config.Config{}
			for _, token := range []struct {
				enabled bool
				cfg     *config.Config
				raw     string
				name    string
			}{
				{tc.global, baseCfg, globalToken, "global"},
				{tc.tenant, tenantCfg, tenantToken, "tenant"},
			} {
				if !token.enabled {
					continue
				}
				record, err := config.NewAPITokenRecord(token.raw, token.name, []string{config.ScopeMonitoringRead})
				if err != nil {
					t.Fatalf("new %s token record: %v", token.name, err)
				}
				token.cfg.APITokens = []config.APITokenRecord{*record}
				token.cfg.SortAPITokens()
			}

			const tenantID = "org-1"
			mtPersistence := config.NewMultiTenantPersistence(t.TempDir())
			if _, err := mtPersistence.GetPersistence(tenantID); err != nil {
				t.Fatalf("tenant persistence: %v", err)
			}
			mtm := monitoring.NewMultiTenantMonitor(baseCfg, mtPersistence, nil)
			t.Cleanup(mtm.Stop)

			// Authentication needs a cached tenant configuration, not a running
			// demo monitor whose background writers can outlive fixture cleanup.
			tenantMonitor := &monitoring.Monitor{}
			setUnexportedField(t, tenantMonitor, "config", tenantCfg)
			setUnexportedField(t, mtm, "monitors", map[string]*monitoring.Monitor{tenantID: tenantMonitor})

			req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
			req.Header.Set("X-Pulse-Org-ID", tenantID)
			req.Header.Set("X-API-Token", tc.provided)
			req = extractAndStoreAuthContext(baseCfg, mtm, req)

			user := internalauth.GetUser(req.Context())
			token := internalauth.GetAPIToken(req.Context())
			if tc.wantRecord == "" {
				if user != "" || token != nil {
					t.Fatal("unknown token must not receive user or API token context")
				}
				return
			}
			if user == "" {
				t.Fatal("expected authenticated user context")
			}
			record, ok := token.(*config.APITokenRecord)
			if !ok || record.Name != tc.wantRecord || !record.HasScope(config.ScopeMonitoringRead) {
				t.Fatalf("expected %s monitoring token in context, got %#v", tc.wantRecord, token)
			}
		})
	}
}
