package api

import (
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/approval"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func installCommandAuthorizationStore(t *testing.T, req *approval.ApprovalRequest) *approval.Store {
	t.Helper()
	store, err := approval.NewStore(approval.StoreConfig{DataDir: t.TempDir(), DisablePersistence: true})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	previous := approval.GetStore()
	approval.SetStore(store)
	t.Cleanup(func() { approval.SetStore(previous) })
	if req != nil {
		if err := store.CreateApproval(req); err != nil {
			t.Fatalf("CreateApproval: %v", err)
		}
		if _, err := store.Approve(req.ID, "operator@example.com"); err != nil {
			t.Fatalf("Approve: %v", err)
		}
	}
	return store
}

func commandAuthorizationApproval(id string) *approval.ApprovalRequest {
	return &approval.ApprovalRequest{
		ID: id, OrgID: "org-1", Command: "echo ok", TargetType: "agent", TargetID: "agent-1",
		Plan: &unifiedresources.ActionPlan{ActionID: "action-1", RequestID: id, Allowed: true, RequiresApproval: true},
	}
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
		{name: "expired", approval: commandAuthorizationApproval("expired"), prepare: func(_ *approval.Store, req *approval.ApprovalRequest) { req.ExpiresAt = time.Now().Add(-time.Minute) }, request: agentexec.CommandAuthorizationRequest{ApprovalID: "expired", OrgID: "org-1", ActionID: "action-1", Command: "echo ok", TargetType: "agent", TargetID: "agent-1"}, wantError: "expired"},
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
			wasConsumed := tc.approval != nil && tc.approval.Consumed
			wasStatus := approval.ApprovalStatus("")
			if tc.approval != nil {
				wasStatus = tc.approval.Status
			}
			err := verifyAndConsumeCommandAuthorization(tc.request)
			if err == nil || !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("error = %v, want %q", err, tc.wantError)
			}
			if tc.approval != nil && (tc.approval.Consumed != wasConsumed || tc.approval.Status != wasStatus) {
				t.Fatalf("invalid authorization mutated approval to status=%s consumed=%v, want status=%s consumed=%v", tc.approval.Status, tc.approval.Consumed, wasStatus, wasConsumed)
			}
		})
	}
}

func TestVerifyAndConsumeCommandAuthorizationConsumesExactBoundApprovalOnce(t *testing.T) {
	req := commandAuthorizationApproval("valid")
	installCommandAuthorizationStore(t, req)
	auth := agentexec.CommandAuthorizationRequest{ApprovalID: "valid", OrgID: "org-1", ActionID: "action-1", AgentID: "agent-1", Command: "echo ok", TargetType: "agent", TargetID: "agent-1"}
	if err := verifyAndConsumeCommandAuthorization(auth); err != nil {
		t.Fatalf("first authorization: %v", err)
	}
	if !req.Consumed {
		t.Fatal("valid authorization was not consumed")
	}
	if err := verifyAndConsumeCommandAuthorization(auth); err == nil || !strings.Contains(err.Error(), "already been consumed") {
		t.Fatalf("replay error = %v, want already consumed", err)
	}
}
