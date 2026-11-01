package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/api/configapi"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestConfigExportCurrentAdmission(t *testing.T) {
	for _, transport := range []string{"header", "bearer", "session"} {
		changes := []string{"expired", "scope", "org", "identity", "principal", "unrelated_scope"}
		if transport == "session" {
			changes = []string{"expired", "principal", "replaced", "recovery"}
		}
		for _, change := range changes {
			t.Run(transport+"/"+change, func(t *testing.T) {
				router, cfg, server, _ := newHeldConfigExportRouter(t)
				raw := logStreamToken
				if transport == "session" {
					raw = "synthetic-config-export-current-session"
					GetSessionStore().CreateSession(raw, time.Hour, "fixture", "127.0.0.1", cfg.AuthUser)
					t.Cleanup(func() { GetSessionStore().DeleteSession(raw) })
				}
				router.configHandlers.SetRuntimeDependencies(configapi.RuntimeDependencies{AuditEvent: func(_, event, _, _, _ string, success bool, _ string) {
					if event != "config_exported" || !success {
						t.Fatal("export failed before current-authority control")
					}
					if transport == "session" {
						mutateReadSession(raw, change)
						return
					}
					config.Mu.Lock()
					defer config.Mu.Unlock()
					switch change {
					case "expired":
						past := time.Now().Add(-time.Minute)
						cfg.APITokens[0].ExpiresAt = &past
					case "scope":
						cfg.APITokens[0].Scopes = []string{config.ScopeMonitoringRead}
					case "org":
						cfg.APITokens[0].OrgID = "another-organization"
					case "identity":
						cfg.APITokens[0].ID = "another-credential"
					case "principal":
						cfg.APITokens[0].Metadata = map[string]string{apiTokenMetadataOwnerUserID: "another-principal"}
					case "unrelated_scope":
						cfg.APITokens[0].Scopes = []string{config.ScopeSettingsRead}
					}
				}})
				req := configExportRequest(t, t.Context(), server, raw, transport)
				rec := httptest.NewRecorder()
				router.Handler().ServeHTTP(rec, req)
				if change == "unrelated_scope" {
					assertConfigExportArchive(t, configExportWire{rec.Code, rec.Header(), rec.Body.String()})
				} else if rec.Code != http.StatusUnauthorized || rec.Body.String() != "Authentication required\n" {
					t.Fatalf("changed credential admitted backup: status=%d", rec.Code)
				}
			})
		}
	}
}

func TestConfigTransferSessionAdmissionIsExplicit(t *testing.T) {
	router, cfg, server, _ := newHeldConfigExportRouter(t)
	const session = "synthetic-config-transfer-admission"
	GetSessionStore().CreateSession(session, time.Hour, "fixture", "127.0.0.1", cfg.AuthUser)
	t.Cleanup(func() { GetSessionStore().DeleteSession(session) })
	for _, transport := range []string{"session", "header", "invalid_header"} {
		t.Run(transport, func(t *testing.T) {
			req := configExportRequest(t, t.Context(), server, session, "session")
			if transport != "session" {
				token := logStreamToken
				if transport == "invalid_header" {
					token = "invalid-synthetic-credential"
				}
				req.Header.Set("X-API-Token", token)
			}
			req = extractAndStoreAuthContext(cfg, nil, req)
			rec := httptest.NewRecorder()
			allowed := router.authorizeConfigTransfer(rec, req, configTransferExport)
			if transport == "invalid_header" {
				if allowed || rec.Code != http.StatusUnauthorized || sessionReadRevalidator(req) != nil {
					t.Fatal("invalid explicit token fell back to ambient session")
				}
				return
			}
			if !allowed {
				t.Fatalf("admitted credential denied: %d", rec.Code)
			}
			recheck := sessionReadRevalidator(req)
			if transport == "session" {
				if recheck == nil || !recheck() {
					t.Fatal("route-local session admission was not retained")
				}
			} else if recheck != nil {
				t.Fatal("token admission borrowed ambient session authority")
			}
		})
	}
}
