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
		{"tenant token without global tokens", false, true, tenantToken, "tenant"},
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
			if !checkAuth(baseCfg, httptest.NewRecorder(), req, false) {
				t.Fatal("route authentication rejected the middleware-validated credential")
			}
			if tc.wantRecord == "tenant" {
				// A stale context record is not authority after revocation.
				tenantCfg.APITokens = nil
				if checkAuth(baseCfg, httptest.NewRecorder(), req, false) {
					t.Fatal("route authentication accepted a revoked tenant credential")
				}
			}
		})
	}
}

func TestCheckAuthRejectsUnvalidatedTenantTokenContext(t *testing.T) {
	cfg := &config.Config{}
	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("X-Pulse-Org-ID", "tenant-a")
	req.Header.Set("X-API-Token", "unvalidated-tenant-token.12345678")
	record, err := config.NewAPITokenRecord("unvalidated-tenant-token.12345678", "forged-context", []string{config.ScopeWildcard})
	if err != nil {
		t.Fatal(err)
	}
	req = req.WithContext(internalauth.WithAPIToken(req.Context(), record))
	if checkAuth(cfg, httptest.NewRecorder(), req, false) {
		t.Fatal("caller-supplied token context authenticated without a live validated inventory")
	}
}

func TestCheckAuthTenantSourceCannotChangeOrganisation(t *testing.T) {
	const raw = "synthetic-live-tenant-token.12345678"
	record, err := config.NewAPITokenRecord(raw, "tenant", []string{config.ScopeMonitoringRead})
	if err != nil {
		t.Fatal(err)
	}
	base := &config.Config{}
	tenantCfg := &config.Config{APITokens: []config.APITokenRecord{*record}}
	mtp := config.NewMultiTenantPersistence(t.TempDir())
	if _, err := mtp.GetPersistence("tenant-a"); err != nil {
		t.Fatal(err)
	}
	mtm := monitoring.NewMultiTenantMonitor(base, mtp, nil)
	t.Cleanup(mtm.Stop)
	m := &monitoring.Monitor{}
	setUnexportedField(t, m, "config", tenantCfg)
	setUnexportedField(t, mtm, "monitors", map[string]*monitoring.Monitor{"tenant-a": m})
	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("X-Pulse-Org-ID", "tenant-a")
	req.Header.Set("X-API-Token", raw)
	req = extractAndStoreAuthContext(base, mtm, req)
	if !checkAuth(base, httptest.NewRecorder(), req, false) {
		t.Fatal("valid tenant credential was not usable")
	}
	req.Header.Set("X-Pulse-Org-ID", "tenant-b")
	if checkAuth(base, httptest.NewRecorder(), req, false) {
		t.Fatal("validated tenant source followed a changed organisation")
	}
	req.Header.Set("X-Pulse-Org-ID", "tenant-a")
	// Replace the cached owner without changing the old config: revalidation
	// must consult the replacement, not the earlier authenticated pointer.
	replacement := &monitoring.Monitor{}
	setUnexportedField(t, replacement, "config", &config.Config{})
	setUnexportedField(t, mtm, "monitors", map[string]*monitoring.Monitor{"tenant-a": replacement})
	if checkAuth(base, httptest.NewRecorder(), req, false) {
		t.Fatal("stale tenant config authenticated after runtime replacement")
	}
}

func TestAuthContextMiddlewareUsesCurrentTenantManager(t *testing.T) {
	const orgID = "tenant-a"
	const oldRaw = "synthetic-before-reload-token.12345678"
	const newRaw = "synthetic-after-reload-token.12345678"
	base := &config.Config{}
	mtp := config.NewMultiTenantPersistence(t.TempDir())
	if _, err := mtp.GetPersistence(orgID); err != nil {
		t.Fatal(err)
	}
	makeManager := func(raw string) *monitoring.MultiTenantMonitor {
		t.Helper()
		record, err := config.NewAPITokenRecord(raw, "synthetic", []string{config.ScopeMonitoringRead})
		if err != nil {
			t.Fatal(err)
		}
		cfg := &config.Config{APITokens: []config.APITokenRecord{*record}}
		monitor := &monitoring.Monitor{}
		setUnexportedField(t, monitor, "config", cfg)
		manager := monitoring.NewMultiTenantMonitor(base, mtp, nil)
		setUnexportedField(t, manager, "monitors", map[string]*monitoring.Monitor{orgID: monitor})
		t.Cleanup(manager.Stop)
		return manager
	}
	oldManager, newManager := makeManager(oldRaw), makeManager(newRaw)
	current := oldManager
	replaceBetweenChecks := false
	handler := authContextMiddlewareWithMonitorResolver(base, func() *monitoring.MultiTenantMonitor { return current }, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if replaceBetweenChecks {
			current = newManager
		}
		if !checkAuth(base, w, r, false) {
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	for _, tc := range []struct {
		name, raw string
		replace   bool
		status    int
	}{
		{"old owner", oldRaw, false, http.StatusOK},
		{"owner changes before enforcement", oldRaw, true, http.StatusUnauthorized},
		{"new owner", newRaw, false, http.StatusOK},
		{"old token after replacement", oldRaw, false, http.StatusUnauthorized},
	} {
		t.Run(tc.name, func(t *testing.T) {
			replaceBetweenChecks = tc.replace
			req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
			req.Header.Set("X-Pulse-Org-ID", orgID)
			req.Header.Set("X-API-Token", tc.raw)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d", rec.Code, tc.status)
			}
		})
	}
}
