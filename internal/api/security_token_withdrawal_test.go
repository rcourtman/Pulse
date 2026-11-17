package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/pkg/audit"
)

// A complete, bounded JSON document may still arrive after its caller's
// credential changes. The real route must not turn that old admission into a
// new credential, or edit a credential after withdrawal has committed.
type tokenWithdrawalBody struct {
	io.Reader
	started chan struct{}
	resume  chan struct{}
	read    bool
}

func (b *tokenWithdrawalBody) Read(p []byte) (int, error) {
	if !b.read {
		b.read = true
		close(b.started)
		<-b.resume
	}
	return b.Reader.Read(p)
}
func (b *tokenWithdrawalBody) Close() error { return nil }

func tokenWithdrawalFixture(t *testing.T) (*Router, *config.Config, config.APITokenRecord, config.APITokenRecord) {
	t.Helper()
	caller := newTokenRecord(t, "synthetic-withdrawal-caller.12345678", []string{config.ScopeWildcard}, nil)
	caller.ID, caller.Name, caller.OrgID = "fixture-caller", "Automation manager", "default"
	target := newTokenRecord(t, "synthetic-withdrawal-target.12345678", []string{config.ScopeMonitoringRead}, nil)
	target.ID, target.Name, target.OrgID = "fixture-target", "Saved collector", "default"
	target.Prefix, target.Suffix = "test", "1234"
	target.CreatedAt = time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	admin := newTokenRecord(t, "synthetic-withdrawal-admin.12345678", []string{config.ScopeWildcard}, nil)
	admin.ID, admin.Name, admin.OrgID = "fixture-admin", "Independent admin", "default"
	cfg := newTestConfigWithTokens(t, caller, target, admin)
	router := newTestRouter(t, cfg, nil, nil, nil, nil, "test")
	return router, cfg, caller, target
}

func tokenWithdrawalInventory(t *testing.T, router *Router) []byte {
	t.Helper()
	config.Mu.RLock()
	defer config.Mu.RUnlock()
	b, err := json.Marshal(router.config.APITokens)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestSecurityTokenMutationRoutesFollowCallerWithdrawal(t *testing.T) {
	t.Setenv("ALLOW_ADMIN_BYPASS", "")
	resetAdminBypassState()
	t.Cleanup(resetAdminBypassState)
	for _, operation := range []string{"create", "rename", "scope"} {
		for _, change := range []string{"revoked", "rotated", "scope removed", "expired", "rebound", "owner changed"} {
			t.Run(operation+"/"+change, func(t *testing.T) {
				router, cfg, caller, target := tokenWithdrawalFixture(t)
				method, path, body := http.MethodPatch, "/api/security/tokens/"+target.ID, `{"name":"Current collector"}`
				if operation == "create" {
					method, path, body = http.MethodPost, "/api/security/tokens", `{"name":"New collector","scopes":["monitoring:read"]}`
				} else if operation == "scope" {
					body = `{"scopes":["settings:read"]}`
				}
				paused := &tokenWithdrawalBody{Reader: strings.NewReader(body), started: make(chan struct{}), resume: make(chan struct{})}
				req := httptest.NewRequest(method, path, paused)
				req.Header.Set("X-API-Token", "synthetic-withdrawal-caller.12345678")
				rec := httptest.NewRecorder()
				done := make(chan struct{})
				go func() { defer close(done); router.Handler().ServeHTTP(rec, req) }()
				// Release and join on every assertion path; no detached request may keep
				// writing its test's state after cleanup.
				released := false
				defer func() {
					if !released {
						close(paused.resume)
					}
					<-done
				}()
				select {
				case <-paused.started:
				case <-time.After(5 * time.Second):
					t.Fatal("registered route did not reach its body reader")
				}
				wantStatus := http.StatusUnauthorized
				switch change {
				case "revoked", "rotated", "scope removed":
					adminMethod, adminPath, adminBody := http.MethodDelete, "/api/security/tokens/"+caller.ID, ""
					adminStatus := http.StatusNoContent
					if change == "rotated" {
						adminMethod, adminPath, adminStatus = http.MethodPost, adminPath+"/rotate", http.StatusOK
					}
					if change == "scope removed" {
						adminMethod, adminBody, adminStatus = http.MethodPatch, `{"scopes":["settings:read","monitoring:read"]}`, http.StatusOK
						wantStatus = http.StatusForbidden
					}
					adminReq := httptest.NewRequest(adminMethod, adminPath, strings.NewReader(adminBody))
					adminReq.Header.Set("X-API-Token", "synthetic-withdrawal-admin.12345678")
					adminRec := httptest.NewRecorder()
					router.Handler().ServeHTTP(adminRec, adminReq)
					if adminRec.Code != adminStatus {
						t.Fatalf("independent withdrawal status=%d, want %d", adminRec.Code, adminStatus)
					}
				default:
					config.Mu.Lock()
					for i := range cfg.APITokens {
						if cfg.APITokens[i].ID != caller.ID {
							continue
						}
						switch change {
						case "expired":
							expires := time.Now().Add(-time.Second)
							cfg.APITokens[i].ExpiresAt = &expires
						case "rebound":
							cfg.APITokens[i].OrgID = "different-estate"
							wantStatus = http.StatusForbidden
						case "owner changed":
							cfg.APITokens[i].Metadata = map[string]string{apiTokenMetadataOwnerUserID: "different-owner"}
						}
					}
					cfg.SortAPITokens()
					err := router.persistence.SaveAPITokens(cfg.APITokens)
					config.Mu.Unlock()
					if err != nil {
						t.Fatal(err)
					}
				}
				before := tokenWithdrawalInventory(t, router)
				durableBefore, err := router.persistence.LoadAPITokens()
				if err != nil {
					t.Fatal(err)
				}
				close(paused.resume)
				released = true
				select {
				case <-done:
				case <-time.After(5 * time.Second):
					t.Fatal("request did not finish after its body resumed")
				}
				// Native producer bytes for the consuming browser proof. Only the
				// credential-free rename and scope responses are emitted; no raw issued
				// secret/hash is ever logged.
				if operation != "create" && (change == "scope removed" || change == "revoked") {
					artifact := map[string]any{"operation": operation, "change": change, "status": rec.Code, "content_type": rec.Header().Get("Content-Type"), "body": rec.Body.String(), "initial": toAPITokenDTO(target)}
					encoded, marshalErr := json.Marshal(artifact)
					if marshalErr != nil {
						t.Fatal(marshalErr)
					}
					fmt.Printf("TOKEN_WITHDRAWAL_PRODUCER %s\n", encoded)
				}
				if rec.Code != wantStatus {
					t.Errorf("withdrawn caller mutation status=%d, want %d", rec.Code, wantStatus)
				}
				if strings.Contains(rec.Body.String(), `"token":`) {
					t.Error("withdrawn caller received a new credential")
				}
				if got := tokenWithdrawalInventory(t, router); string(got) != string(before) {
					t.Error("withdrawn caller changed the post-withdrawal live inventory")
				}
				durableAfter, err := router.persistence.LoadAPITokens()
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(durableBefore, durableAfter) {
					t.Error("withdrawn caller changed the durable inventory")
				}
			})
		}
	}
}

// These handlers do not read a body. Capture a genuine credential admission,
// then withdraw it before entering the transaction, as can happen while the
// admitted request waits for configuration ownership.
func TestSecurityTokenMutationTransactionsFollowCallerWithdrawal(t *testing.T) {
	for _, operation := range []string{"delete", "rotate"} {
		for _, change := range []string{"revoked", "hash replaced", "scope removed", "expired", "rebound", "owner changed", "invalid runtime role"} {
			t.Run(operation+"/"+change, func(t *testing.T) {
				router, cfg, caller, target := tokenWithdrawalFixture(t)
				method, path, handler := http.MethodDelete, "/api/security/tokens/"+target.ID, router.handleDeleteAPIToken
				if operation == "rotate" {
					method, path, handler = http.MethodPost, path+"/rotate", router.handleRotateAPIToken
				}
				req := httptest.NewRequest(method, path, nil)
				req.Header.Set("X-API-Token", "synthetic-withdrawal-caller.12345678")
				if !CheckAuth(cfg, httptest.NewRecorder(), req) {
					t.Fatal("original caller was not admitted")
				}
				if change == "invalid runtime role" {
					// Persistence already refuses invalid roles. Keep that valid
					// saved baseline, then exercise an in-memory authority fault.
					config.Mu.Lock()
					err := router.persistence.SaveAPITokens(cfg.APITokens)
					config.Mu.Unlock()
					if err != nil {
						t.Fatal(err)
					}
				}
				config.Mu.Lock()
				wantStatus := http.StatusUnauthorized
				for i := range cfg.APITokens {
					current := &cfg.APITokens[i]
					if current.ID != caller.ID {
						continue
					}
					switch change {
					case "revoked":
						cfg.RemoveAPIToken(caller.ID)
					case "hash replaced":
						current.Hash = "different-credential-hash"
					case "scope removed":
						current.Scopes = []string{config.ScopeSettingsRead}
						wantStatus = http.StatusForbidden
					case "expired":
						expires := time.Now().Add(-time.Second)
						current.ExpiresAt = &expires
					case "rebound":
						current.OrgIDs = []string{"default", "different-estate"}
						wantStatus = http.StatusForbidden
					case "owner changed":
						current.Metadata = map[string]string{apiTokenMetadataOwnerUserID: "different-owner"}
					case "invalid runtime role":
						current.Metadata = map[string]string{"runtime_role": "unknown-role"}
					}
					break
				}
				cfg.SortAPITokens()
				var err error
				if change != "invalid runtime role" {
					err = router.persistence.SaveAPITokens(cfg.APITokens)
				}
				config.Mu.Unlock()
				if err != nil {
					t.Fatal(err)
				}
				before := tokenWithdrawalInventory(t, router)
				durableBefore, err := router.persistence.LoadAPITokens()
				if err != nil {
					t.Fatal(err)
				}
				logger := installTokenMutationAuditProbe(t, func() {
					if !config.Mu.TryLock() {
						t.Error("withdrawal denial retained configuration ownership during audit")
						return
					}
					config.Mu.Unlock()
				})
				rec := httptest.NewRecorder()
				handler(rec, req)
				if rec.Code != wantStatus {
					t.Errorf("status=%d, want %d", rec.Code, wantStatus)
				}
				if strings.Contains(rec.Body.String(), `"token":`) {
					t.Error("withdrawn caller received a replacement secret")
				}
				if string(tokenWithdrawalInventory(t, router)) != string(before) {
					t.Error("withdrawn transaction changed live credentials")
				}
				durableAfter, err := router.persistence.LoadAPITokens()
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(durableBefore, durableAfter) {
					t.Error("withdrawn transaction changed saved credentials")
				}
				events, err := logger.Query(audit.QueryFilter{})
				if err != nil || len(events) == 0 {
					t.Fatal("withdrawal denial omitted its audit result")
				}
				for _, event := range events {
					if event.Success {
						t.Error("withdrawn transaction audited a successful mutation")
					}
				}
			})
		}
	}
}

func TestSecurityTokenMutationUsesCurrentGrantScopes(t *testing.T) {
	for _, operation := range []string{"create", "scope", "delete", "rotate"} {
		for _, allowed := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/allowed=%v", operation, allowed), func(t *testing.T) {
				router, cfg, caller, target := tokenWithdrawalFixture(t)
				method, path, body, handler := http.MethodPatch, "/api/security/tokens/"+target.ID, `{"scopes":["monitoring:read","monitoring:write"]}`, router.handleUpdateAPIToken
				if operation == "create" {
					method, path, body, handler = http.MethodPost, "/api/security/tokens", `{"name":"New collector","scopes":["monitoring:read","monitoring:write"]}`, router.handleCreateAPIToken
				}
				if operation == "delete" {
					method, body, handler = http.MethodDelete, "", router.handleDeleteAPIToken
				}
				if operation == "rotate" {
					method, path, body, handler = http.MethodPost, path+"/rotate", "", router.handleRotateAPIToken
				}
				req := httptest.NewRequest(method, path, strings.NewReader(body))
				req.Header.Set("X-API-Token", "synthetic-withdrawal-caller.12345678")
				if !CheckAuth(cfg, httptest.NewRecorder(), req) {
					t.Fatal("original caller was not admitted")
				}
				config.Mu.Lock()
				for i := range cfg.APITokens {
					if cfg.APITokens[i].ID == caller.ID {
						cfg.APITokens[i].Scopes = []string{config.ScopeSettingsWrite}
						if allowed {
							cfg.APITokens[i].Scopes = append(cfg.APITokens[i].Scopes, config.ScopeMonitoringRead, config.ScopeMonitoringWrite)
						}
					}
				}
				cfg.SortAPITokens()
				config.Mu.Unlock()
				rec := httptest.NewRecorder()
				handler(rec, req)
				want := http.StatusForbidden
				if allowed {
					want = http.StatusOK
					if operation == "delete" {
						want = http.StatusNoContent
					}
				}
				if rec.Code != want {
					t.Errorf("status=%d, want %d", rec.Code, want)
				}
				if !allowed && strings.Contains(rec.Body.String(), `"token":`) {
					t.Error("old grant scope issued a new secret")
				}
			})
		}
	}
}

func TestSecurityTokenMutationKeepsLiveSelfWithdrawal(t *testing.T) {
	for _, operation := range []string{"delete", "rotate", "narrow"} {
		t.Run(operation, func(t *testing.T) {
			router, cfg, caller, _ := tokenWithdrawalFixture(t)
			method, path, body, want := http.MethodDelete, "/api/security/tokens/"+caller.ID, "", http.StatusNoContent
			if operation == "rotate" {
				method, path, want = http.MethodPost, path+"/rotate", http.StatusOK
			}
			if operation == "narrow" {
				method, body, want = http.MethodPatch, `{"scopes":["monitoring:read"]}`, http.StatusOK
			}
			req := httptest.NewRequest(method, path, strings.NewReader(body))
			req.Header.Set("X-API-Token", "synthetic-withdrawal-caller.12345678")
			rec := httptest.NewRecorder()
			router.Handler().ServeHTTP(rec, req)
			if rec.Code != want {
				t.Fatalf("live self-change status=%d, want %d", rec.Code, want)
			}
			if operation == "rotate" {
				var response struct {
					Token string `json:"token"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
					t.Fatal(err)
				}
				config.Mu.RLock()
				valid := cfg.IsValidAPIToken(response.Token)
				config.Mu.RUnlock()
				if response.Token == "" || !valid {
					t.Error("self-rotation lost the replacement credential")
				}
			}
			checkReq := httptest.NewRequest(http.MethodGet, "/api/security/tokens", nil)
			checkReq.Header.Set("X-API-Token", "synthetic-withdrawal-caller.12345678")
			check := httptest.NewRecorder()
			router.Handler().ServeHTTP(check, checkReq)
			denied := http.StatusUnauthorized
			if operation == "narrow" {
				denied = http.StatusForbidden
			}
			if check.Code != denied {
				t.Errorf("old authority remains usable: status=%d, want %d", check.Code, denied)
			}
		})
	}
}

func TestSecurityTokenMutationRejectsShadowedCredential(t *testing.T) {
	router, cfg, caller, target := tokenWithdrawalFixture(t)
	req := httptest.NewRequest(http.MethodPost, "/api/security/tokens/"+target.ID+"/rotate", nil)
	req.Header.Set("X-API-Token", "synthetic-withdrawal-caller.12345678")
	if !CheckAuth(cfg, httptest.NewRecorder(), req) {
		t.Fatal("original credential was not admitted")
	}
	shadow := caller.Clone()
	shadow.ID = "different-first-matching-record"
	shadow.CreatedAt = time.Now().Add(time.Hour)
	config.Mu.Lock()
	cfg.UpsertAPIToken(shadow)
	config.Mu.Unlock()
	before := tokenWithdrawalInventory(t, router)
	rec := httptest.NewRecorder()
	router.handleRotateAPIToken(rec, req)
	if rec.Code != http.StatusUnauthorized || strings.Contains(rec.Body.String(), `"token":`) {
		t.Error("old context chose authority behind the first matching credential")
	}
	if string(tokenWithdrawalInventory(t, router)) != string(before) {
		t.Error("shadowed caller changed the inventory")
	}
}

func TestSecurityTokenMutationSharedIssuerKeepsTenantInventoryOwner(t *testing.T) {
	for _, change := range []string{"live", "revoked", "owner removed", "owner replaced"} {
		t.Run(change, func(t *testing.T) {
			const orgID, raw = "tenant-a", "synthetic-tenant-mutation.12345678"
			record := newTokenRecord(t, raw, []string{config.ScopeSettingsWrite, config.ScopeRelayMobileAccess}, nil)
			record.OrgID = orgID
			base := newTestConfigWithTokens(t, record.Clone())
			tenant := &config.Config{APITokens: []config.APITokenRecord{record.Clone()}}
			persistence := config.NewMultiTenantPersistence(t.TempDir())
			if _, err := persistence.GetPersistence(orgID); err != nil {
				t.Fatal(err)
			}
			makeManager := func(cfg *config.Config) *monitoring.MultiTenantMonitor {
				t.Helper()
				manager := monitoring.NewMultiTenantMonitor(base, persistence, nil)
				// Cached owners suffice: no monitor, database, appliance or Mobile
				// runtime is started to exercise the shared credential transaction.
				owner := &monitoring.Monitor{}
				setUnexportedField(t, owner, "config", cfg)
				setUnexportedField(t, manager, "monitors", map[string]*monitoring.Monitor{orgID: owner})
				t.Cleanup(manager.Stop)
				return manager
			}
			manager := makeManager(tenant)
			req := httptest.NewRequest(http.MethodPost, "/api/security/tokens/relay-mobile", nil)
			req.Header.Set("X-Pulse-Org-ID", orgID)
			req.Header.Set("X-API-Token", raw)
			req = extractAndStoreAuthContext(base, manager, req)
			req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, orgID))
			if getAPITokenRecordFromRequest(req) == nil {
				t.Fatal("tenant credential was not admitted")
			}
			liveManager := manager
			source, ok := req.Context().Value(tenantTokenSourceContextKey).(tenantTokenSource)
			if !ok {
				t.Fatal("tenant credential lost its owning inventory")
			}
			source.resolveManager = func() *monitoring.MultiTenantMonitor { return liveManager }
			req = req.WithContext(context.WithValue(req.Context(), tenantTokenSourceContextKey, source))
			switch change {
			case "revoked":
				config.Mu.Lock()
				tenant.RemoveAPIToken(record.ID)
				config.Mu.Unlock()
			case "owner removed":
				liveManager = nil
			case "owner replaced":
				liveManager = makeManager(&config.Config{})
			}
			router := &Router{config: base, persistence: config.NewConfigPersistence(base.DataPath)}
			before := tokenWithdrawalInventory(t, router)
			token, created, err := router.createAPITokenRecord(req, "Mobile access", relayMobileAccessTokenScopes(), nil, nil)
			if change == "live" {
				if err != nil || token == "" || created == nil || created.OrgID != orgID || !created.HasScope(config.ScopeRelayMobileAccess) || created.HasScope(config.ScopeSettingsWrite) {
					t.Fatal("live shared issuer lost its existing org-scoped compatibility")
				}
			} else {
				failure, ok := err.(*tokenMutationFailure)
				if !ok || failure.status != http.StatusUnauthorized || token != "" || created != nil {
					t.Error("shared issuer fell back from withdrawn tenant owner to a global copy")
				}
				if string(tokenWithdrawalInventory(t, router)) != string(before) {
					t.Error("withdrawn tenant caller changed the global inventory")
				}
			}
		})
	}
}
