package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/audit"
	authpkg "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

// Audit destinations and HTTP clients are independent I/O. Neither may retain
// the configuration lock after a token transaction has committed or rolled back.
type tokenMutationAuditProbe struct {
	auditCaptureLogger
	probe func()
}

func (l *tokenMutationAuditProbe) Record(event audit.Event) error {
	l.probe()
	return l.auditCaptureLogger.Record(event)
}

func installTokenMutationAuditProbe(t *testing.T, probe func()) *tokenMutationAuditProbe {
	t.Helper()
	previous, previousManager := audit.GetLogger(), GetTenantAuditManager()
	logger := &tokenMutationAuditProbe{probe: probe}
	audit.SetLogger(logger)
	SetTenantAuditManager(nil)
	t.Cleanup(func() {
		audit.SetLogger(previous)
		SetTenantAuditManager(previousManager)
	})
	return logger
}

func TestSecurityTokenMutationIOReleasesConfig(t *testing.T) {
	for _, operation := range []string{"update", "delete", "rotate", "create"} {
		scenarios := []string{"success", "missing", "hidden", "existing scope denied", "save failed"}
		if operation == "update" {
			scenarios = append(scenarios, "grant denied", "instance promotion denied")
		} else if operation == "create" {
			scenarios = []string{"success", "save failed"}
		}
		for _, scenario := range scenarios {
			t.Run(operation+"/"+scenario, func(t *testing.T) {
				f := newOrgTokenFixture(t)
				caller, target := f.def.Clone(), f.orgB.Clone()
				body := `{"name":"edited","scopes":["monitoring:read"]}`
				wantStatus, wantBody := http.StatusOK, ""
				switch scenario {
				case "missing":
					target.ID = "missing"
					wantStatus, wantBody = http.StatusNotFound, "Token not found\n"
				case "hidden":
					caller = f.orgA.Clone()
					wantStatus, wantBody = http.StatusNotFound, "Token not found\n"
				case "existing scope denied":
					caller.Scopes = []string{config.ScopeSettingsWrite}
					target = f.def.Clone()
					wantStatus = http.StatusForbidden
					verb := operation
					wantBody = "Cannot " + verb + " token with scope \"*\": your token does not have this scope\n"
				case "grant denied", "instance promotion denied":
					target = f.def.Clone()
					target.Scopes = []string{config.ScopeSettingsWrite}
					f.cfg.APITokens = []config.APITokenRecord{target}
					wantStatus = http.StatusForbidden
					if scenario == "grant denied" {
						caller.Scopes = []string{config.ScopeSettingsWrite}
						body = `{"scopes":["monitoring:write"]}`
						wantBody = "Cannot grant scope \"monitoring:write\": your token does not have this scope\n"
					} else {
						caller.OrgIDs = []string{"default", "org-a"}
						body = `{"scopes":["*"]}`
						wantBody = "Cannot grant scope \"*\" in the default organization: your token is not an instance-wide full-access token\n"
					}
				}
				f.cfg.SortAPITokens()
				before := make([]config.APITokenRecord, len(f.cfg.APITokens))
				for i := range f.cfg.APITokens {
					before[i] = f.cfg.APITokens[i].Clone()
				}
				if err := f.router.persistence.SaveAPITokens(before); err != nil {
					t.Fatal(err)
				}
				if scenario == "save failed" {
					// Initialize a real store, preserve its directory, then block
					// only that disposable store's path. Construction must succeed
					// before the write fault is introduced.
					blocker := filepath.Join(t.TempDir(), "state")
					f.router.persistence = config.NewConfigPersistence(blocker)
					if err := os.Rename(blocker, blocker+"-preserved"); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(blocker, []byte("blocker"), 0o600); err != nil {
						t.Fatal(err)
					}
					wantStatus = http.StatusInternalServerError
					switch operation {
					case "delete":
						wantBody = "Failed to save token deletion\n"
					case "rotate":
						wantBody = "Failed to save rotated token\n"
					default:
						wantBody = "Failed to save token\n"
					}
				}
				method, path, handler := http.MethodPatch, "/api/security/tokens/"+target.ID, f.router.handleUpdateAPIToken
				switch operation {
				case "delete":
					method, handler = http.MethodDelete, f.router.handleDeleteAPIToken
					if scenario == "success" {
						wantStatus = http.StatusNoContent
					}
				case "rotate":
					method, path, handler = http.MethodPost, path+"/rotate", f.router.handleRotateAPIToken
				case "create":
					method, path, handler = http.MethodPost, "/api/security/tokens", f.router.handleCreateAPIToken
				}
				probe := func(phase string) {
					// These direct handlers have no competing router workers.
					if !config.Mu.TryLock() {
						t.Errorf("token transaction holds config.Mu during %s I/O", phase)
						return
					}
					config.Mu.Unlock()
				}
				logger := installTokenMutationAuditProbe(t, func() { probe("audit") })
				writer := &tokenReadProbeWriter{ResponseRecorder: httptest.NewRecorder(), probe: func() { probe("response") }}
				req := httptest.NewRequest(method, path, strings.NewReader(body))
				attachAPITokenRecord(req, &caller)
				handler(writer, req)
				if !writer.probed || writer.Code != wantStatus {
					t.Fatalf("response probed=%v status=%d, want %d", writer.probed, writer.Code, wantStatus)
				}
				events, _ := logger.Query(audit.QueryFilter{})
				if len(events) == 0 {
					t.Fatal("token transaction omitted its audit event")
				}
				for _, event := range events {
					if event.Success != (scenario == "success") {
						t.Error("audit outcome differs from transaction outcome")
					}
				}
				if scenario != "success" {
					if writer.Body.String() != wantBody {
						t.Error("error response changed its existing status/body contract")
					}
					if !reflect.DeepEqual(before, f.cfg.APITokens) || f.cfg.APIToken != before[0].Hash {
						t.Error("failed transaction changed the original inventory or legacy primary")
					}
				} else {
					persisted, err := f.router.persistence.LoadAPITokens()
					// Persistence intentionally omits empty org bindings. Compare
					// every serialized field, not nil versus an empty OrgIDs slice.
					persistedJSON, marshalErr := json.Marshal(persisted)
					liveJSON, liveErr := json.Marshal(f.cfg.APITokens)
					if err != nil || marshalErr != nil || liveErr != nil || string(persistedJSON) != string(liveJSON) {
						t.Error("success response was not backed by the committed inventory")
					}
					if operation == "delete" {
						if writer.Body.Len() != 0 || len(persisted) != len(before)-1 {
							t.Error("deletion changed its bodyless success or inventory contract")
						}
					} else {
						var response struct {
							Token  string      `json:"token"`
							Record apiTokenDTO `json:"record"`
						}
						if err := json.Unmarshal(writer.Body.Bytes(), &response); err != nil {
							t.Fatal(err)
						}
						stored := findAPITokenByID(t, persisted, response.Record.ID)
						if !reflect.DeepEqual(toAPITokenDTO(stored), response.Record) {
							t.Error("response metadata differs from committed token")
						}
						if operation != "update" && (response.Token == "" || !f.cfg.IsValidAPIToken(response.Token)) {
							t.Error("create/rotate omitted the committed replacement credential")
						}
						if operation == "update" && (response.Token != "" || stored.Hash != target.Hash || stored.Name != "edited") {
							t.Error("metadata update changed credential identity or did not rename")
						}
					}
				}
				for _, record := range before {
					if strings.Contains(writer.Body.String(), record.Hash) {
						t.Error("token response disclosed a stored credential hash")
					}
				}
			})
		}
	}
}

func TestSecurityTokenMutationRoutesAllowAuthenticationDuringOutput(t *testing.T) {
	t.Setenv("ALLOW_ADMIN_BYPASS", "")
	resetAdminBypassState()
	t.Cleanup(resetAdminBypassState)
	for _, operation := range []string{"update", "delete", "rotate"} {
		t.Run(operation, func(t *testing.T) {
			const callerRaw = "synthetic-mutation-admin.12345678"
			target := newTokenRecord(t, "synthetic-mutation-target.12345678", []string{config.ScopeMonitoringRead}, nil)
			cfg := newTestConfigWithTokens(t, newTokenRecord(t, callerRaw, []string{config.ScopeWildcard}, nil), target)
			router := newTestRouter(t, cfg, nil, nil, nil, nil, "test")
			method, path := http.MethodPatch, "/api/security/tokens/"+target.ID
			wantStatus := http.StatusOK
			if operation == "delete" {
				method, wantStatus = http.MethodDelete, http.StatusNoContent
			} else if operation == "rotate" {
				method, path = http.MethodPost, path+"/rotate"
			}
			req := httptest.NewRequest(method, path, strings.NewReader(`{"name":"edited"}`))
			req.Header.Set("X-API-Token", callerRaw)
			var completed chan bool
			writer := &tokenReadProbeWriter{ResponseRecorder: httptest.NewRecorder()}
			writer.probe = func() {
				completed = make(chan bool, 1)
				go func() {
					authReq := httptest.NewRequest(http.MethodGet, "/api/health", nil)
					authReq.Header.Set("X-API-Token", callerRaw)
					completed <- CheckAuth(cfg, httptest.NewRecorder(), authReq)
				}()
				select {
				case ok := <-completed:
					if !ok {
						t.Error("independent token authentication was denied")
					}
					completed = nil
				case <-time.After(2 * time.Second):
					t.Error("independent authentication stalled behind token mutation output")
				}
			}
			router.Handler().ServeHTTP(writer, req)
			if completed != nil {
				select {
				case <-completed:
				case <-time.After(2 * time.Second):
					t.Fatal("diagnostic authentication did not finish after output released")
				}
			}
			if !writer.probed || writer.Code != wantStatus {
				t.Errorf("route did not reach expected output: probed=%v status=%d", writer.probed, writer.Code)
			}
		})
	}
}

func TestSecurityTokenMutationCreateRecordDetached(t *testing.T) {
	f := newOrgTokenFixture(t)
	expiresIn := "1h"
	req := httptest.NewRequest(http.MethodPost, "/api/security/tokens", nil)
	req = req.WithContext(authpkg.WithUser(req.Context(), "original-owner"))
	_, snapshot, err := f.router.createAPITokenRecord(req, "original", []string{config.ScopeMonitoringRead}, &expiresIn, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := snapshot.Clone()
	config.Mu.Lock()
	for i := range f.cfg.APITokens {
		live := &f.cfg.APITokens[i]
		if live.ID == snapshot.ID {
			live.Scopes[0] = config.ScopeWildcard
			live.Metadata[apiTokenMetadataOwnerUserID] = "replacement-owner"
			*live.ExpiresAt = live.ExpiresAt.Add(time.Hour)
		}
	}
	config.Mu.Unlock()
	if !reflect.DeepEqual(*snapshot, want) {
		t.Error("minting helper returned mutable authority owned by the live inventory")
	}
}

func TestSecurityTokenMutationResponsesDetachedBeforeAudit(t *testing.T) {
	for _, operation := range []string{"update", "rotate", "create"} {
		t.Run(operation, func(t *testing.T) {
			f := newOrgTokenFixture(t)
			mutated := false
			installTokenMutationAuditProbe(t, func() {
				if mutated {
					return
				}
				if !config.Mu.TryLock() {
					t.Error("audit still owns the global configuration lock")
					return
				}
				defer config.Mu.Unlock()
				mutated = true
				for i := range f.cfg.APITokens {
					live := &f.cfg.APITokens[i]
					live.Name = "concurrent rename"
					live.Scopes[0] = config.ScopeSettingsWrite
					live.Metadata = map[string]string{apiTokenMetadataOwnerUserID: "concurrent owner"}
				}
			})
			method, path, handler := http.MethodPatch, "/api/security/tokens/"+f.orgB.ID, f.router.handleUpdateAPIToken
			wantName := "edited"
			if operation == "rotate" {
				method, path, handler = http.MethodPost, path+"/rotate", f.router.handleRotateAPIToken
				wantName = f.orgB.Name
			} else if operation == "create" {
				method, path, handler = http.MethodPost, "/api/security/tokens", f.router.handleCreateAPIToken
			}
			req := httptest.NewRequest(method, path, strings.NewReader(`{"name":"edited","scopes":["monitoring:read"]}`))
			attachAPITokenRecord(req, &f.def)
			rec := httptest.NewRecorder()
			handler(rec, req)
			var response struct {
				Record apiTokenDTO `json:"record"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if !mutated || rec.Code != http.StatusOK || response.Record.Name != wantName ||
				len(response.Record.Scopes) != 1 || response.Record.Scopes[0] != config.ScopeMonitoringRead || response.Record.OwnerUserID != "" {
				t.Error("audit-time live edit changed the already-committed response snapshot")
			}
		})
	}
}
