package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	u "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestQueryActionPreservesIndependentOutcomeWithoutInventory(t *testing.T) {
	store := u.NewMemoryStore()
	now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	truth, err := u.NormalizeActionResultV2(u.ActionResultV2{
		Version:   u.ActionResultV2Version,
		Execution: u.ActionExecutionTruth{Status: u.ActionExecutionSucceeded},
		Verification: u.ActionVerificationTruth{Status: u.ActionVerificationConfirmed, EvidenceClass: u.ActionEvidenceIndependent, Evidence: []u.ActionEvidence{{
			Version: u.ActionEvidenceVersion, ID: "proof-1", ObserverID: "proxmox-api", ObserverKind: "provider",
			ObserverTrustDomain: "provider:proxmox", ExecutorTrustDomain: "agent:node",
			Method: "resource_status", SubjectID: "vm-110", ObservedAt: now, ReceivedAt: now.Add(time.Second), Summary: "status=running",
		}}},
		Compensation: u.ActionCompensationTruth{Support: u.ActionCompensationUnavailable, Status: u.ActionCompensationNotAvailable},
	})
	if err != nil {
		t.Fatal(err)
	}
	record := u.ActionAuditRecord{
		ID: "act-proof", CreatedAt: now, UpdatedAt: now.Add(time.Second), State: u.ActionStateCompleted,
		Request: u.ActionRequest{ResourceID: "vm-110", CapabilityName: "start", Actor: u.ActionActor{SubjectID: "operator", Kind: u.ActionActorUser, OrgID: "tenant-a"}, Params: map[string]any{"private": "private-parameter"}},
		Plan:    u.ActionPlan{ActionID: "act-proof", ApprovalPolicy: u.ApprovalAdmin, PredictedBlastRadius: []string{"vm-110", "node-1"}, Preflight: &u.ActionPreflight{CurrentState: "offline"}},
		Result:  &u.ExecutionResult{Success: true, Output: "private-driver-output", ActionResultV2: &truth},
	}
	if err := store.RecordActionAudit(record); err != nil {
		t.Fatal(err)
	}
	executor := NewPulseToolExecutor(ExecutorConfig{ActionAuditStore: store, OrgID: "tenant-a"})
	result, err := executor.executeQuery(context.Background(), map[string]interface{}{"action": "action", "action_id": record.ID})
	if err != nil || result.IsError {
		t.Fatalf("query failed: %+v %v", result, err)
	}
	var got struct {
		State  u.ActionState    `json:"state"`
		Plan   u.ActionPlan     `json:"plan"`
		Result u.ActionResultV2 `json:"action_result_v2"`
	}
	text := result.Content[0].Text
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatal(err)
	}
	if got.State != u.ActionStateCompleted || got.Result.Execution.Status != u.ActionExecutionSucceeded || got.Result.Verification.EvidenceClass != u.ActionEvidenceIndependent || got.Result.Verification.Status != u.ActionVerificationConfirmed {
		t.Fatalf("canonical outcome lost: %+v", got)
	}
	if got.Plan.Preflight.CurrentState != "offline" || len(got.Plan.PredictedBlastRadius) != 2 || got.Plan.ApprovalPolicy != u.ApprovalAdmin {
		t.Fatalf("canonical planning context lost: %+v", got.Plan)
	}
	if len(got.Result.Verification.Evidence) != 1 || !got.Result.Verification.Evidence[0].ObservedAt.Equal(now) {
		t.Fatalf("observation provenance lost: %+v", got.Result.Verification)
	}
	for _, private := range []string{"private-parameter", "private-driver-output"} {
		if strings.Contains(text, private) {
			t.Fatalf("private detail exposed: %s", private)
		}
	}
	executor.SetOrgID("tenant-b")
	result, err = executor.executeQuery(context.Background(), map[string]interface{}{"action": "action", "action_id": record.ID})
	if err != nil || !result.IsError {
		t.Fatal("cross-tenant action should not be visible")
	}
}

func TestQueryActionMissingRecordIsNotNoExecutionProof(t *testing.T) {
	executor := NewPulseToolExecutor(ExecutorConfig{ActionAuditStore: u.NewMemoryStore()})
	for _, id := range []string{"", "act-missing"} {
		result, err := executor.executeQuery(context.Background(), map[string]interface{}{"action": "action", "action_id": id})
		if err != nil || !result.IsError {
			t.Fatalf("missing record must be unavailable, not a synthesized outcome: %+v %v", result, err)
		}
	}
}
