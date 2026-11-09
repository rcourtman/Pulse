package api

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/approval"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func installCommandAuthorizationStore(t *testing.T, req *approval.ApprovalRequest) *approval.Store {
	t.Helper()
	dir := t.TempDir()
	if req != nil {
		// Final command authorization reads saved authority. Load a complete
		// approved record, including an already-expired fixture where needed,
		// instead of editing a detached creation result to change store state.
		record := *req
		record.Status = approval.StatusApproved
		record.CommandHash = approval.ComputeCommandHash(record.Command, record.TargetType, record.TargetID)
		if record.ExpiresAt.IsZero() {
			record.ExpiresAt = time.Now().Add(time.Hour)
		}
		body, err := json.Marshal([]*approval.ApprovalRequest{&record})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "ai_approvals.json"), body, 0600); err != nil {
			t.Fatal(err)
		}
	}
	store, err := approval.NewStore(approval.StoreConfig{DataDir: dir})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(store.Flush)
	previous := approval.GetStore()
	approval.SetStore(store)
	t.Cleanup(func() { approval.SetStore(previous) })
	return store
}

func commandAuthorizationApproval(id string) *approval.ApprovalRequest {
	return &approval.ApprovalRequest{
		ID: id, OrgID: "org-1", Command: "echo ok", TargetType: "agent", TargetID: "agent-1",
		Plan: &unifiedresources.ActionPlan{ActionID: "action-1", RequestID: id, Allowed: true, RequiresApproval: true},
	}
}

func expiredCommandAuthorizationApproval() *approval.ApprovalRequest {
	req := commandAuthorizationApproval("expired")
	req.ExpiresAt = time.Now().Add(-time.Minute)
	return req
}

func TestVerifyAndConsumeCommandAuthorizationRejectsInvalidApprovalsWithoutNewConsumption(t *testing.T) {
	tests := []struct {
		name      string
		approval  *approval.ApprovalRequest
		prepare   func(*approval.Store, *approval.ApprovalRequest)
		request   agentexec.CommandAuthorizationRequest
		wantError string
	}{
		{name: "nonexistent", request: agentexec.CommandAuthorizationRequest{ApprovalID: "missing", OrgID: "org-1", ActionID: "action-1", Command: "echo ok", TargetType: "agent", TargetID: "agent-1"}, wantError: "approval not found"},
		{name: "wrong org", approval: commandAuthorizationApproval("wrong-org"), request: agentexec.CommandAuthorizationRequest{ApprovalID: "wrong-org", OrgID: "org-2", ActionID: "action-1", Command: "echo ok", TargetType: "agent", TargetID: "agent-1"}, wantError: "another org"},
		{name: "wrong action", approval: commandAuthorizationApproval("wrong-action"), request: agentexec.CommandAuthorizationRequest{ApprovalID: "wrong-action", OrgID: "org-1", ActionID: "action-2", Command: "echo ok", TargetType: "agent", TargetID: "agent-1"}, wantError: "action does not match"},
		{name: "expired", approval: expiredCommandAuthorizationApproval(), request: agentexec.CommandAuthorizationRequest{ApprovalID: "expired", OrgID: "org-1", ActionID: "action-1", Command: "echo ok", TargetType: "agent", TargetID: "agent-1"}, wantError: "expired"},
		{name: "consumed", approval: commandAuthorizationApproval("consumed"), prepare: func(store *approval.Store, _ *approval.ApprovalRequest) {
			if _, err := store.ConsumeApproval("consumed", "echo ok", "agent", "agent-1"); err != nil {
				t.Fatalf("pre-consume: %v", err)
			}
		}, request: agentexec.CommandAuthorizationRequest{ApprovalID: "consumed", OrgID: "org-1", ActionID: "action-1", Command: "echo ok", TargetType: "agent", TargetID: "agent-1"}, wantError: "already been consumed"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := installCommandAuthorizationStore(t, tc.approval)
			if tc.prepare != nil {
				tc.prepare(store, tc.approval)
			}
			before, foundBefore := store.GetApproval(tc.request.ApprovalID)
			err := verifyAndConsumeCommandAuthorization(tc.request)
			if err == nil || !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("error = %v, want %q", err, tc.wantError)
			}
			after, foundAfter := store.GetApproval(tc.request.ApprovalID)
			if foundBefore != foundAfter || !reflect.DeepEqual(before, after) {
				t.Fatal("invalid authorization mutated the saved approval")
			}
		})
	}
}

func TestVerifyAndConsumeCommandAuthorizationConsumesExactBoundApprovalOnce(t *testing.T) {
	req := commandAuthorizationApproval("valid")
	store := installCommandAuthorizationStore(t, req)
	auth := agentexec.CommandAuthorizationRequest{ApprovalID: "valid", OrgID: "org-1", ActionID: "action-1", AgentID: "agent-1", Command: "echo ok", TargetType: "agent", TargetID: "agent-1"}
	if err := verifyAndConsumeCommandAuthorization(auth); err != nil {
		t.Fatalf("first authorization: %v", err)
	}
	stored, found := store.GetApproval(req.ID)
	if !found || !stored.Consumed {
		t.Fatal("valid authorization was not consumed")
	}
	if err := verifyAndConsumeCommandAuthorization(auth); err == nil || !strings.Contains(err.Error(), "already been consumed") {
		t.Fatalf("replay error = %v, want already consumed", err)
	}
}
