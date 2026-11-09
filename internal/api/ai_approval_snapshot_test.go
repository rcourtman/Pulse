package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/approval"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/stretchr/testify/require"
)

// Load fixed saved records through the production store, so native before/after
// response bytes can be compared without an exported fixture-mutation API.
func approvalSnapshotHTTPFixture(t *testing.T) (*AISettingsHandler, *approval.Store, []*approval.ApprovalRequest) {
	t.Helper()
	items := make([]*approval.ApprovalRequest, 0, 3)
	for i, org := range []string{"default", "default", "fixture-org"} {
		req := &approval.ApprovalRequest{
			ID: fmt.Sprintf("fixture-%d", i), OrgID: org, ExecutionID: "fixture-execution",
			Command: "printf fixture", TargetType: "agent", TargetID: "fixture-host", TargetName: "Fixture host",
			Context: "Fixture review", RequestedBy: approval.RequesterPulseAssistant,
			RiskLevel: approval.RiskLow, Status: approval.StatusPending,
			RequestedAt:       time.Date(2026, 1, 1, 0, 0, i, 0, time.UTC),
			ExpiresAt:         time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC),
			ContextConfidence: &approval.ContextConfidence{Level: approval.ContextConfidenceVerified, Evidence: []string{"fixture evidence"}},
			Preflight:         &approval.ActionPreflight{Target: "agent:fixture-host", SafetyChecks: []string{"fixture check"}},
			Plan: &unifiedresources.ActionPlan{
				ActionID: fmt.Sprintf("fixture-action-%d", i), RequiresApproval: true,
				PredictedBlastRadius: []string{"fixture-host"},
			},
		}
		if i == 1 {
			req.Plan.PolicyDecision.Authorities = []unifiedresources.ActionPolicyAuthorityFactor{}
		}
		req.CommandHash = approval.ComputeCommandHash(req.Command, req.TargetType, req.TargetID)
		items = append(items, req)
	}
	dir := t.TempDir()
	body, err := json.Marshal(items)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ai_approvals.json"), body, 0600))
	store, err := approval.NewStore(approval.StoreConfig{DataDir: dir})
	require.NoError(t, err)
	t.Cleanup(store.Flush)
	prior := approval.GetStore()
	approval.SetStore(store)
	t.Cleanup(func() { approval.SetStore(prior) })
	return &AISettingsHandler{defaultConfig: &config.Config{}}, store, items
}

func TestAIApprovalSnapshotWireCompatibility(t *testing.T) {
	handler, store, items := approvalSnapshotHTTPFixture(t)
	for _, org := range []string{"default", "fixture-org"} {
		req := newLoopbackRequest(http.MethodGet, "/api/ai/approvals", nil)
		req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, org))
		rec := httptest.NewRecorder()
		handler.HandleListApprovals(rec, req)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		want, err := json.Marshal(map[string]interface{}{
			"approvals": store.GetPendingApprovalsForOrg(org), "stats": store.GetStatsForOrg(org),
		})
		require.NoError(t, err)
		require.JSONEq(t, string(want), rec.Body.String())
		t.Logf("APPROVAL_SNAPSHOT_WIRE list/%s %s", org, base64.StdEncoding.EncodeToString(rec.Body.Bytes()))
	}
	for _, item := range items {
		req := newLoopbackRequest(http.MethodGet, "/api/ai/approvals/"+item.ID, nil)
		req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, item.OrgID))
		rec := httptest.NewRecorder()
		handler.HandleGetApproval(rec, req)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		want, err := json.Marshal(item)
		require.NoError(t, err)
		require.JSONEq(t, string(want), rec.Body.String())
		t.Logf("APPROVAL_SNAPSHOT_WIRE detail/%s %s", item.ID, base64.StdEncoding.EncodeToString(rec.Body.Bytes()))
	}
}

func TestAIApprovalSnapshotConcurrentHTTPReadsAndDecision(t *testing.T) {
	handler, store, _ := approvalSnapshotHTTPFixture(t)
	var wg sync.WaitGroup
	wg.Add(2)
	start := make(chan struct{})
	go func() {
		defer wg.Done()
		close(start)
		for i := 0; i < 80; i++ {
			req := newLoopbackRequest(http.MethodGet, "/api/ai/approvals/fixture-0", nil)
			rec := httptest.NewRecorder()
			handler.HandleGetApproval(rec, req)
			if rec.Code != http.StatusOK || !json.Valid(rec.Body.Bytes()) {
				t.Error("concurrent detail read failed")
			}
			req = newLoopbackRequest(http.MethodGet, "/api/ai/approvals", nil)
			rec = httptest.NewRecorder()
			handler.HandleListApprovals(rec, req)
			if rec.Code != http.StatusOK || !json.Valid(rec.Body.Bytes()) {
				t.Error("concurrent list read failed")
			}
		}
	}()
	go func() {
		defer wg.Done()
		<-start
		if _, err := store.Approve("fixture-0", "fixture-reviewer"); err != nil {
			t.Error(err)
		}
		if _, err := store.ConsumeApproval("fixture-0", "printf fixture", "agent", "fixture-host"); err != nil {
			t.Error(err)
		}
	}()
	wg.Wait()
	got, found := store.GetApproval("fixture-0")
	require.True(t, found)
	require.Equal(t, approval.StatusApproved, got.Status)
	require.True(t, got.Consumed)
}
