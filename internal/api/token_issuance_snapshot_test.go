package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/api/agenttokens"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// Join the real issuers to native HTTP auth/scope consumption without starting
// a monitor, database, external agent or new product surface. Returned issuance
// records are snapshots, while each request must still use live revocation.
func TestIssuedTokenSnapshotsKeepHTTPAdmissionOnLiveInventory(t *testing.T) {
	for _, runner := range []bool{false, true} {
		name, allowedScope := "collector", config.ScopeAgentReport
		if runner {
			name, allowedScope = "action-runner", config.ScopeAgentExec
		}
		t.Run(name, func(t *testing.T) {
			cfg := &config.Config{DataPath: t.TempDir(), AuthUser: "fixture-admin", AuthPass: "fixture-password-hash"}
			persistence := config.NewConfigPersistence(cfg.DataPath)
			var raw string
			var issued *config.APITokenRecord
			var err error
			if runner {
				raw, issued, err = agenttokens.IssueActionRunnerAndPersist(cfg, persistence, agenttokens.ActionRunnerIssueOptions{
					OrgID: "default", AgentID: "fixture-host", Hostname: "fixture-host.example", OwnerUserID: "fixture-owner",
				})
			} else {
				raw, issued, err = agenttokens.IssueAndPersist(cfg, persistence, agenttokens.IssueOptions{
					OrgID: "default", OwnerUserID: "fixture-owner",
				})
			}
			if err != nil || issued == nil {
				t.Fatalf("issue failed: %v", err)
			}
			id, admittedOwner := issued.ID, apiTokenOwnerUserID(*issued)
			issued.Metadata[agenttokens.RuntimeRoleMetadataKey] = "snapshot-only-role"
			issued.Metadata[agenttokens.OwnerUserIDMetadataKey] = "snapshot-only-owner"
			issued.Scopes[0] = config.ScopeSettingsWrite
			if issued.ExpiresAt != nil {
				*issued.ExpiresAt = time.Now().UTC().Add(-time.Hour)
			}

			newRequest := func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/api/synthetic-token-consumer", nil)
				req.Header.Set("Authorization", "Bearer "+raw)
				return req
			}
			consumer := func(w http.ResponseWriter, req *http.Request) {
				record := getAPITokenRecordFromRequest(req)
				if record == nil || record.ID != id || record.OrgID != "default" || apiTokenOwnerUserID(*record) != admittedOwner {
					t.Error("HTTP authentication lost the admitted inventory identity or owner")
				}
				w.WriteHeader(http.StatusNoContent)
			}
			allowed := RequireAuth(cfg, RequireScope(allowedScope, consumer))
			rec := httptest.NewRecorder()
			allowed(rec, newRequest())
			if rec.Code != http.StatusNoContent {
				t.Fatalf("editing issuance result broke valid HTTP auth: status=%d", rec.Code)
			}
			rec = httptest.NewRecorder()
			RequireAuth(cfg, RequireScope(config.ScopeSettingsWrite, consumer))(rec, newRequest())
			if rec.Code != http.StatusForbidden {
				t.Fatalf("editing issuance result granted settings access: status=%d", rec.Code)
			}

			// Even a previously authenticated context must not keep a revoked
			// credential alive merely because its issuance snapshot is detached.
			admittedRequest := extractAndStoreAuthContext(cfg, nil, newRequest())
			config.Mu.Lock()
			cfg.RemoveAPIToken(id)
			cfg.SortAPITokens()
			if err := persistence.SaveAPITokens(cfg.APITokens); err != nil {
				config.Mu.Unlock()
				t.Fatal(err)
			}
			config.Mu.Unlock()
			for _, req := range []*http.Request{admittedRequest, newRequest()} {
				rec = httptest.NewRecorder()
				allowed(rec, req)
				if rec.Code != http.StatusUnauthorized {
					t.Fatalf("live revocation failed for issuance snapshot: status=%d", rec.Code)
				}
			}
		})
	}
}
