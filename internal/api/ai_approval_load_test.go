package api

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/approval"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const approvalLoadGoodFixture = `[{"id":"load-fixture","orgId":"fixture-org","executionId":"fixture-execution","toolId":"investigation_fix","command":"printf fixture","targetType":"investigation","targetId":"fixture-finding","targetName":"Fixture host","context":"Review fixture service health","requestedBy":"pulse_patrol","riskLevel":"low","status":"pending","requestedAt":"2026-11-09T10:00:00Z","expiresAt":"2099-01-01T00:00:00Z","commandHash":"fixture-nonlegacy-hash","plan":{"actionId":"fixture-action","requestId":"load-fixture","requiresApproval":true}}]`

func approvalLoadWire(t *testing.T, h *AISettingsHandler, label, org string, want int) *httptest.ResponseRecorder {
	t.Helper()
	req := newLoopbackRequest(http.MethodGet, "/api/ai/approvals", nil)
	req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, org))
	rec := httptest.NewRecorder()
	h.HandleListApprovals(rec, req)
	t.Logf("APPROVAL_LOAD_WIRE %s %d %s", label, rec.Code, base64.StdEncoding.EncodeToString(rec.Body.Bytes()))
	assert.Equal(t, want, rec.Code, label)
	return rec
}

func TestAIApprovalLoadFailureAndRecovery(t *testing.T) {
	for _, filename := range []string{"ai_approvals.json", "ai_executions.json"} {
		t.Run(filename, func(t *testing.T) {
			prior := approval.GetStore()
			h := &AIHandler{}
			settings := &AISettingsHandler{defaultConfig: &config.Config{}}
			t.Cleanup(func() { h.clearApprovalStore(); approval.SetStore(prior) })
			h.clearApprovalStore()
			approvalLoadWire(t, settings, filename+"/disabled", "fixture-org", http.StatusOK)
			dir := t.TempDir()
			originals := map[string][]byte{"ai_approvals.json": []byte(approvalLoadGoodFixture), "ai_executions.json": []byte("[]\n")}
			originals[filename] = []byte(`[{"id":`)
			for name, body := range originals {
				require.NoError(t, os.WriteFile(filepath.Join(dir, name), body, 0600))
			}
			h.ensureApprovalStore(dir)
			assert.Nil(t, approval.GetStore(), "failed load must not publish an empty mutable store")
			assert.Nil(t, h.approvalStoreStop, "failed load must not start cleanup")
			assert.ErrorContains(t, verifyAndConsumeCommandAuthorization(agentexec.CommandAuthorizationRequest{ApprovalID: "load-fixture", OrgID: "fixture-org", Command: "printf fixture", TargetType: "agent", TargetID: "fixture-host"}), "unavailable")
			rec := approvalLoadWire(t, settings, filename+"/unavailable", "fixture-org", http.StatusServiceUnavailable)
			assert.NotContains(t, rec.Body.String(), dir)
			assert.NotContains(t, rec.Body.String(), "printf fixture")
			approvalLoadWire(t, settings, filename+"/unavailable-default", "default", http.StatusServiceUnavailable)
			// Unknown tenant context still has its own earlier rejection.
			approvalLoadWire(t, settings, filename+"/unknown-org", " ", http.StatusServiceUnavailable)
			if approval.GetStore() == nil {
				for _, op := range []string{"detail", "approve", "deny"} {
					method, path := http.MethodPost, "/api/ai/approvals/load-fixture/"+op
					if op == "detail" {
						method = http.MethodGet
						path = "/api/ai/approvals/load-fixture"
					}
					req := newLoopbackRequest(method, path, nil)
					req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, "fixture-org"))
					rec := httptest.NewRecorder()
					switch op {
					case "detail":
						settings.HandleGetApproval(rec, req)
					case "approve":
						settings.HandleApproveCommand(rec, req)
					case "deny":
						settings.HandleDenyCommand(rec, req)
					}
					assert.Equal(t, http.StatusServiceUnavailable, rec.Code, op)
				}
			}
			for name, body := range originals {
				got, err := os.ReadFile(filepath.Join(dir, name))
				require.NoError(t, err)
				assert.Equal(t, body, got, name)
			}
			// Correct only the synthetic invalid source, then use normal initialization.
			// Recovery must reveal the same saved request, without token/role recreation.
			good := []byte(approvalLoadGoodFixture)
			if filename == "ai_executions.json" {
				good = []byte("[]\n")
			}
			require.NoError(t, os.WriteFile(filepath.Join(dir, filename), good, 0600))
			h.ensureApprovalStore(dir)
			require.NotNil(t, approval.GetStore())
			rec = approvalLoadWire(t, settings, filename+"/recovered", "fixture-org", http.StatusOK)
			assert.Contains(t, rec.Body.String(), "load-fixture")
			rec = approvalLoadWire(t, settings, filename+"/isolated-default", "default", http.StatusOK)
			assert.NotContains(t, rec.Body.String(), "load-fixture")
			h.clearApprovalStore()
			approvalLoadWire(t, settings, filename+"/disabled-again", "fixture-org", http.StatusOK)
		})
	}
}
