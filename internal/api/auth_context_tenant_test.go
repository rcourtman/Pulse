package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

func TestCheckAuthTenantConcurrentValidation(t *testing.T) {
	const orgID = "tenant-a"
	const raw = "synthetic-concurrent-tenant-token.12345678"
	for _, inventory := range []string{"tenant", "global"} {
		t.Run(inventory, func(t *testing.T) {
			record, err := config.NewAPITokenRecord(raw, "concurrent", []string{config.ScopeMonitoringRead})
			if err != nil {
				t.Fatal(err)
			}
			record.OrgID = orgID
			record.Metadata = map[string]string{apiTokenMetadataOwnerUserID: "tenant-owner"}
			base, tenant := &config.Config{}, &config.Config{}
			tokenConfig := tenant
			if inventory == "global" {
				tokenConfig = base
			}
			tokenConfig.APITokens = []config.APITokenRecord{*record}
			before := record.Clone()
			mtp := config.NewMultiTenantPersistence(t.TempDir())
			if _, err := mtp.GetPersistence(orgID); err != nil {
				t.Fatal(err)
			}
			manager := monitoring.NewMultiTenantMonitor(base, mtp, nil)
			t.Cleanup(manager.Stop)
			// A cached fake owner exercises auth without starting a runtime,
			// opening a database or contacting a monitored node.
			monitor := &monitoring.Monitor{}
			setUnexportedField(t, monitor, "config", tenant)
			setUnexportedField(t, manager, "monitors", map[string]*monitoring.Monitor{orgID: monitor})
			newRequest := func() *http.Request {
				r := httptest.NewRequest(http.MethodGet, "/api/test", nil)
				r.Header.Set("X-Pulse-Org-ID", orgID)
				r.Header.Set("X-API-Token", raw)
				return r.WithContext(context.WithValue(r.Context(), OrgIDContextKey, orgID))
			}
			for _, stage := range []string{"route revalidation", "middleware and route"} {
				t.Run(stage, func(t *testing.T) {
					seed := extractAndStoreAuthContext(base, manager, newRequest())
					started := time.Now().UTC()
					start := make(chan struct{})
					var callers sync.WaitGroup
					for range 4 {
						callers.Go(func() {
							<-start
							for range 32 {
								r := seed.Clone(seed.Context())
								if stage == "middleware and route" {
									r = extractAndStoreAuthContext(base, manager, newRequest())
								}
								w := httptest.NewRecorder()
								if !checkAuth(base, w, r, false) {
									t.Error("valid concurrent credential was rejected")
									return
								}
								got := getAPITokenRecordFromRequest(r)
								if got == nil || got.ID != record.ID || got.OrgID != orgID || apiTokenAuthenticatedUser(got) != "tenant-owner" || got.LastUsedAt == nil || got.LastUsedAt.Before(started) {
									t.Error("route auth lost token identity, organisation, owner or usage metadata")
									return
								}
								if !ensureScope(w, r, config.ScopeMonitoringRead) || ensureScope(httptest.NewRecorder(), r, config.ScopeSettingsWrite) {
									t.Error("concurrent validation changed token scope enforcement")
									return
								}
							}
						})
					}
					close(start)
					callers.Wait()
				})
			}
			config.Mu.RLock()
			after := tokenConfig.APITokens[0].Clone()
			config.Mu.RUnlock()
			if after.LastUsedAt == nil {
				t.Error("successful validation did not update the live inventory")
			}
			after.LastUsedAt = nil
			if !reflect.DeepEqual(after, before) {
				t.Error("validation changed token authority or identity")
			}
		})
	}
}

func TestCheckAuthTenantSourceCannotChangeDuringValidation(t *testing.T) {
	const orgID = "tenant-a"
	const raw = "synthetic-replaced-during-validation.12345678"
	record, err := config.NewAPITokenRecord(raw, "old-owner", []string{config.ScopeMonitoringRead})
	if err != nil {
		t.Fatal(err)
	}
	base := &config.Config{}
	mtp := config.NewMultiTenantPersistence(t.TempDir())
	if _, err := mtp.GetPersistence(orgID); err != nil {
		t.Fatal(err)
	}
	makeManager := func(cfg *config.Config) *monitoring.MultiTenantMonitor {
		t.Helper()
		monitor := &monitoring.Monitor{}
		setUnexportedField(t, monitor, "config", cfg)
		manager := monitoring.NewMultiTenantMonitor(base, mtp, nil)
		setUnexportedField(t, manager, "monitors", map[string]*monitoring.Monitor{orgID: monitor})
		t.Cleanup(manager.Stop)
		return manager
	}
	oldManager := makeManager(&config.Config{APITokens: []config.APITokenRecord{*record}})
	newManager := makeManager(&config.Config{})
	r := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	r.Header.Set("X-Pulse-Org-ID", orgID)
	r.Header.Set("X-API-Token", raw)
	r = extractAndStoreAuthContext(base, oldManager, r)
	r = r.WithContext(context.WithValue(r.Context(), OrgIDContextKey, orgID))
	source := r.Context().Value(tenantTokenSourceContextKey).(tenantTokenSource)
	resolutions := 0
	source.resolveManager = func() *monitoring.MultiTenantMonitor {
		resolutions++
		if resolutions == 1 {
			return oldManager
		}
		return newManager
	}
	r = r.WithContext(context.WithValue(r.Context(), tenantTokenSourceContextKey, source))
	if checkAuth(base, httptest.NewRecorder(), r, false) {
		t.Fatal("route auth accepted the owner replaced across metadata validation")
	}
	if resolutions < 2 {
		t.Fatal("route auth did not recheck its live owner after the lock transition")
	}
}

func TestValidateAPITokenAgainstConfigsLockedReturnsDetachedSnapshot(t *testing.T) {
	const raw = "synthetic-detached-token-snapshot.12345678"
	for _, inventory := range []string{"tenant", "global"} {
		t.Run(inventory, func(t *testing.T) {
			record, err := config.NewAPITokenRecord(raw, "snapshot", []string{config.ScopeMonitoringRead})
			if err != nil {
				t.Fatal(err)
			}
			record.OrgIDs = []string{"tenant-a"}
			record.Metadata = map[string]string{apiTokenMetadataOwnerUserID: "tenant-owner"}
			base, tenant := &config.Config{}, &config.Config{}
			live := tenant
			if inventory == "global" {
				live = base
			}
			live.APITokens = []config.APITokenRecord{*record}
			config.Mu.RLock()
			got, ok := validateAPITokenAgainstConfigsLocked(base, tenant, raw)
			// The helper must restore the caller's read ownership.
			if config.Mu.TryLock() {
				config.Mu.Unlock()
				t.Error("token validation lost the caller's config read lock")
			}
			config.Mu.RUnlock()
			if !ok || got == nil || got.LastUsedAt == nil {
				t.Fatal("valid credential returned no usage snapshot")
			}
			got.Scopes[0] = config.ScopeSettingsWrite
			got.OrgIDs[0] = "tenant-b"
			got.Metadata[apiTokenMetadataOwnerUserID] = "changed-owner"
			*got.LastUsedAt = time.Time{}
			config.Mu.RLock()
			stored := live.APITokens[0].Clone()
			config.Mu.RUnlock()
			if !reflect.DeepEqual(stored.Scopes, []string{config.ScopeMonitoringRead}) || !reflect.DeepEqual(stored.OrgIDs, []string{"tenant-a"}) || stored.Metadata[apiTokenMetadataOwnerUserID] != "tenant-owner" || (stored.LastUsedAt == nil || stored.LastUsedAt.IsZero()) {
				t.Error("returned auth snapshot aliases live authority or usage metadata")
			}
		})
	}
}

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
			// TenantMiddleware resolves this context before route authentication.
			req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, tenantID))

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
	req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, "tenant-a"))
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
	req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, "tenant-a"))
	if !checkAuth(base, httptest.NewRecorder(), req, false) {
		t.Fatal("valid tenant credential was not usable")
	}
	req.Header.Set("X-Pulse-Org-ID", "tenant-b")
	req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, "tenant-b"))
	if checkAuth(base, httptest.NewRecorder(), req, false) {
		t.Fatal("validated tenant source followed a changed organisation")
	}
	req.Header.Set("X-Pulse-Org-ID", "tenant-a")
	req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, "tenant-a"))
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
		r = r.WithContext(context.WithValue(r.Context(), OrgIDContextKey, orgID))
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
