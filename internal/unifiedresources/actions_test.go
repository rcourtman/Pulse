package unifiedresources

import (
	"strings"
	"testing"
	"time"
)

func TestNormalizeActionAuditRecordPopulatesGovernedPlanPreflight(t *testing.T) {
	now := time.Date(2026, 4, 25, 22, 30, 0, 0, time.UTC)
	record, err := NormalizeActionAuditRecord(ActionAuditRecord{
		ID:        " action-1 ",
		CreatedAt: now,
		State:     ActionStateExecuting,
		Request: ActionRequest{
			ResourceID:     " vm:42 ",
			CapabilityName: " pulse_control ",
			Reason:         "Restart workload",
			RequestedBy:    " pulse_assistant ",
		},
	})
	if err != nil {
		t.Fatalf("NormalizeActionAuditRecord() error = %v", err)
	}

	if record.ID != "action-1" || record.Plan.ActionID != "action-1" {
		t.Fatalf("action id normalization failed: %#v", record)
	}
	if record.Request.RequestID != "action-1" || record.Plan.RequestID != "action-1" {
		t.Fatalf("request id normalization failed: request=%q plan=%q", record.Request.RequestID, record.Plan.RequestID)
	}
	if record.Request.ResourceID != "vm:42" || record.Request.CapabilityName != "pulse_control" || record.Request.RequestedBy != "pulse_assistant" {
		t.Fatalf("request normalization failed: %#v", record.Request)
	}
	if record.Plan.ApprovalPolicy != ApprovalNone {
		t.Fatalf("approval policy = %q, want %q", record.Plan.ApprovalPolicy, ApprovalNone)
	}
	if record.Plan.Preflight == nil {
		t.Fatal("expected preflight to be populated")
	}
	if record.Plan.Preflight.Target != "vm:42" {
		t.Fatalf("preflight target = %q, want vm:42", record.Plan.Preflight.Target)
	}
	if record.Plan.Preflight.DryRunAvailable {
		t.Fatal("default preflight must explicitly mark dry-run unavailable")
	}
	if !strings.Contains(record.Plan.Preflight.DryRunSummary, "No provider-supported dry run") {
		t.Fatalf("unexpected dry-run summary: %q", record.Plan.Preflight.DryRunSummary)
	}
	if len(record.Plan.Preflight.SafetyChecks) == 0 || len(record.Plan.Preflight.VerificationSteps) == 0 {
		t.Fatalf("preflight should carry safety and verification checks: %#v", record.Plan.Preflight)
	}
}

func TestNormalizeActionAuditRecordRejectsUngovernedRecords(t *testing.T) {
	_, err := NormalizeActionAuditRecord(ActionAuditRecord{
		ID:    "action-1",
		State: ActionStateCompleted,
		Request: ActionRequest{
			ResourceID:  "vm:42",
			RequestedBy: "pulse_assistant",
		},
	})
	if err == nil {
		t.Fatal("expected missing capability to be rejected")
	}

	_, err = NormalizeActionAuditRecord(ActionAuditRecord{
		ID:    "action-1",
		State: ActionState("unknown"),
		Request: ActionRequest{
			ResourceID:     "vm:42",
			CapabilityName: "pulse_control",
			RequestedBy:    "pulse_assistant",
		},
	})
	if err == nil {
		t.Fatal("expected invalid state to be rejected")
	}
}

func TestNormalizeActionLifecycleEventRejectsInvalidEvents(t *testing.T) {
	if _, err := NormalizeActionLifecycleEvent(ActionLifecycleEvent{State: ActionStatePlanned}); err == nil {
		t.Fatal("expected missing action id to be rejected")
	}
	if _, err := NormalizeActionLifecycleEvent(ActionLifecycleEvent{ActionID: "action-1", State: ActionState("paused")}); err == nil {
		t.Fatal("expected invalid lifecycle state to be rejected")
	}
}
