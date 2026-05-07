package approval

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestNewStore(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "approval-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(StoreConfig{
		DataDir:        tmpDir,
		DefaultTimeout: 1 * time.Minute,
		MaxApprovals:   10,
	})
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	if store == nil {
		t.Fatal("NewStore() returned nil store")
	}
}

func TestNewStoreEmptyDataDir(t *testing.T) {
	_, err := NewStore(StoreConfig{
		DataDir: "",
	})
	if err == nil {
		t.Fatal("NewStore() expected error for empty data dir")
	}
}

func TestNewStore_NonPositiveConfigUsesDefaults(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "approval-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(StoreConfig{
		DataDir:        tmpDir,
		DefaultTimeout: -1 * time.Minute,
		MaxApprovals:   -10,
	})
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	if store.defaultTimeout != 5*time.Minute {
		t.Fatalf("defaultTimeout = %v, want %v", store.defaultTimeout, 5*time.Minute)
	}
	if store.maxApprovals != 100 {
		t.Fatalf("maxApprovals = %d, want 100", store.maxApprovals)
	}

	req := &ApprovalRequest{Command: "echo ok"}
	if err := store.CreateApproval(req); err != nil {
		t.Fatalf("CreateApproval() error = %v", err)
	}
	if !req.ExpiresAt.After(req.RequestedAt) {
		t.Fatal("approval expiry should be after request time")
	}
}

func TestCreateApproval(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "approval-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, _ := NewStore(StoreConfig{
		DataDir:        tmpDir,
		DefaultTimeout: 1 * time.Minute,
	})

	req := &ApprovalRequest{
		ExecutionID: "exec-1",
		ToolID:      "tool-1",
		Command:     "systemctl restart nginx",
		TargetType:  "agent",
		TargetID:    "host-1",
		TargetName:  "webserver",
		Context:     "Service needs restart due to config change",
	}

	err = store.CreateApproval(req)
	if err != nil {
		t.Fatalf("CreateApproval() error = %v", err)
	}

	if req.ID == "" {
		t.Error("CreateApproval() did not set ID")
	}
	if req.Status != StatusPending {
		t.Errorf("CreateApproval() status = %v, want %v", req.Status, StatusPending)
	}
	if req.RequestedAt.IsZero() {
		t.Error("CreateApproval() did not set RequestedAt")
	}
	if req.ExpiresAt.IsZero() {
		t.Error("CreateApproval() did not set ExpiresAt")
	}
}

func TestCreateApproval_PreservesActionPlanAndContextConfidence(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "approval-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, _ := NewStore(StoreConfig{
		DataDir:            tmpDir,
		DefaultTimeout:     1 * time.Minute,
		DisablePersistence: true,
	})

	req := &ApprovalRequest{
		ID:         "approval-1",
		Command:    "systemctl restart nginx",
		TargetType: "agent",
		TargetID:   "agent-1",
		TargetName: "web1",
		Context:    "Restart web service",
		Plan: &unifiedresources.ActionPlan{
			ActionID:         "action-1",
			Allowed:          true,
			RequiresApproval: true,
			ApprovalPolicy:   unifiedresources.ApprovalAdmin,
			Message:          "Restart web service",
			PlanHash:         "hash-1",
		},
		ContextConfidence: &ContextConfidence{
			Level:    ContextConfidenceVerified,
			Summary:  "Target was resolved to a concrete resource before approval.",
			Evidence: []string{"Target identifier bound to agent-1."},
		},
		Preflight: &ActionPreflight{
			Target:            "agent:web1 (agent-1)",
			CurrentState:      "Resolved approval target: agent:web1 (agent-1).",
			IntendedChange:    "Restart web service",
			DryRunAvailable:   false,
			DryRunSummary:     "No provider-supported dry run is available for this action.",
			SafetyChecks:      []string{"Approval is scoped to this organization."},
			VerificationSteps: []string{"Read back the target state after execution."},
			GeneratedAt:       time.Now().UTC(),
		},
	}

	if err := store.CreateApproval(req); err != nil {
		t.Fatalf("CreateApproval() error = %v", err)
	}

	got, ok := store.GetApproval("approval-1")
	if !ok {
		t.Fatal("approval not found")
	}
	if got.Plan == nil {
		t.Fatal("plan was not preserved")
	}
	if got.Plan.ActionID != "action-1" {
		t.Fatalf("plan action id = %q, want action-1", got.Plan.ActionID)
	}
	if got.Plan.RequestID != "approval-1" {
		t.Fatalf("plan request id = %q, want approval-1", got.Plan.RequestID)
	}
	if got.Plan.ExpiresAt.IsZero() {
		t.Fatal("plan expiry should be populated")
	}
	if got.ContextConfidence == nil || got.ContextConfidence.Level != ContextConfidenceVerified {
		t.Fatalf("unexpected context confidence: %+v", got.ContextConfidence)
	}
	if got.Preflight == nil {
		t.Fatal("preflight was not preserved")
	}
	if got.Plan.Preflight == nil {
		t.Fatal("plan preflight was not populated from approval preflight")
	}
	if got.Preflight.Target != "agent:web1 (agent-1)" {
		t.Fatalf("preflight target = %q, want agent:web1 (agent-1)", got.Preflight.Target)
	}
	if got.Plan.Preflight.Target != got.Preflight.Target {
		t.Fatalf("plan preflight target = %q, want %q", got.Plan.Preflight.Target, got.Preflight.Target)
	}
	if got.Preflight.DryRunAvailable {
		t.Fatal("preflight dry run should remain false")
	}
	if len(got.Preflight.SafetyChecks) != 1 {
		t.Fatalf("preflight safety checks = %+v, want one entry", got.Preflight.SafetyChecks)
	}
}

func TestCreateApproval_PopulatesActionPlanPreflightWhenMissing(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "approval-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, _ := NewStore(StoreConfig{
		DataDir:        tmpDir,
		DefaultTimeout: 5 * time.Minute,
	})

	req := &ApprovalRequest{
		ID:         "approval-plan-preflight",
		Command:    "systemctl restart nginx",
		TargetType: "agent",
		TargetID:   "agent-1",
		TargetName: "web1",
		Context:    "Restart web service",
		Plan: &unifiedresources.ActionPlan{
			ActionID:         "action-plan-preflight",
			Allowed:          true,
			RequiresApproval: true,
			ApprovalPolicy:   unifiedresources.ApprovalAdmin,
			Message:          "Restart web service",
			PlanHash:         "hash-plan-preflight",
		},
	}

	if err := store.CreateApproval(req); err != nil {
		t.Fatalf("CreateApproval() error = %v", err)
	}

	got, ok := store.GetApproval("approval-plan-preflight")
	if !ok {
		t.Fatal("approval not found")
	}
	if got.Plan == nil || got.Plan.Preflight == nil {
		t.Fatalf("expected plan preflight to be populated: %+v", got.Plan)
	}
	if got.Preflight != got.Plan.Preflight {
		t.Fatal("approval preflight should share the normalized plan preflight")
	}
	if got.Plan.Preflight.Target != "agent:agent-1" {
		t.Fatalf("preflight target = %q, want agent:agent-1", got.Plan.Preflight.Target)
	}
	if got.Plan.Preflight.DryRunAvailable {
		t.Fatal("generated preflight should explicitly mark dry-run unavailable")
	}
}

func TestCreateApproval_RejectsUnsupportedHostTargetType(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "approval-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, _ := NewStore(StoreConfig{
		DataDir:        tmpDir,
		DefaultTimeout: 1 * time.Minute,
	})

	req := &ApprovalRequest{
		Command:    "uptime",
		TargetType: "host",
		TargetID:   "host-1",
	}

	if err := store.CreateApproval(req); err == nil {
		t.Fatal("expected unsupported host target type to be rejected")
	}
}

func TestGetApproval(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "approval-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, _ := NewStore(StoreConfig{
		DataDir:        tmpDir,
		DefaultTimeout: 1 * time.Minute,
	})

	req := &ApprovalRequest{
		Command: "apt update",
	}
	_ = store.CreateApproval(req)

	got, found := store.GetApproval(req.ID)
	if !found {
		t.Fatal("GetApproval() not found")
	}
	if got.Command != "apt update" {
		t.Errorf("GetApproval() command = %v, want %v", got.Command, "apt update")
	}

	_, found = store.GetApproval("nonexistent")
	if found {
		t.Error("GetApproval() found nonexistent approval")
	}
}

func TestGetPendingApprovals(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "approval-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, _ := NewStore(StoreConfig{
		DataDir:        tmpDir,
		DefaultTimeout: 1 * time.Minute,
	})

	// Create multiple approvals
	for i := 0; i < 3; i++ {
		_ = store.CreateApproval(&ApprovalRequest{
			Command: "test command",
		})
	}

	pending := store.GetPendingApprovals()
	if len(pending) != 3 {
		t.Errorf("GetPendingApprovals() count = %v, want %v", len(pending), 3)
	}
}

func TestGetPendingApprovalsReturnsOperationalPriorityOrder(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "approval-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, _ := NewStore(StoreConfig{
		DataDir:            tmpDir,
		DefaultTimeout:     1 * time.Minute,
		DisablePersistence: true,
	})

	base := time.Now().UTC().Add(30 * time.Minute).Truncate(time.Second)
	create := func(id string, expiresAt time.Time, risk RiskLevel, requestedAt time.Time) {
		req := &ApprovalRequest{
			ID:        id,
			Command:   "test command " + id,
			RiskLevel: risk,
			ExpiresAt: expiresAt,
		}
		if err := store.CreateApproval(req); err != nil {
			t.Fatalf("CreateApproval(%s) error = %v", id, err)
		}
		req.ExpiresAt = expiresAt
		req.RiskLevel = risk
		req.RequestedAt = requestedAt
	}

	create("later-critical", base.Add(3*time.Minute), RiskLevel("critical"), base)
	create("same-expiry-medium", base.Add(2*time.Minute), RiskMedium, base.Add(-2*time.Minute))
	create("same-expiry-high-b", base.Add(2*time.Minute), RiskHigh, base)
	create("same-expiry-high-a", base.Add(2*time.Minute), RiskHigh, base)
	create("same-expiry-high-newer", base.Add(2*time.Minute), RiskHigh, base.Add(time.Minute))
	create("soon-low", base.Add(time.Minute), RiskLow, base)

	assertApprovalOrder(t, store.GetPendingApprovals(), []string{
		"soon-low",
		"same-expiry-high-a",
		"same-expiry-high-b",
		"same-expiry-high-newer",
		"same-expiry-medium",
		"later-critical",
	})
}

func TestNormalizeOrgID(t *testing.T) {
	if got := NormalizeOrgID(""); got != DefaultOrgID {
		t.Fatalf("NormalizeOrgID(\"\") = %q, want %q", got, DefaultOrgID)
	}
	if got := NormalizeOrgID("  org-a  "); got != "org-a" {
		t.Fatalf("NormalizeOrgID trims input, got %q", got)
	}
}

func TestBelongsToOrg(t *testing.T) {
	if BelongsToOrg(nil, "default") {
		t.Fatal("BelongsToOrg should return false for nil request")
	}

	legacy := &ApprovalRequest{ID: "legacy"}
	if !BelongsToOrg(legacy, "default") {
		t.Fatal("legacy approvals should belong to default org")
	}
	if BelongsToOrg(legacy, "org-a") {
		t.Fatal("legacy approvals should not belong to non-default org")
	}

	scoped := &ApprovalRequest{ID: "scoped", OrgID: "org-a"}
	if !BelongsToOrg(scoped, "org-a") {
		t.Fatal("expected approval to belong to matching org")
	}
	if BelongsToOrg(scoped, "Org-A") {
		t.Fatal("org comparison should be case-sensitive")
	}
}

func TestGetPendingApprovalsForOrg(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "approval-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, _ := NewStore(StoreConfig{
		DataDir:            tmpDir,
		DefaultTimeout:     1 * time.Minute,
		DisablePersistence: true,
	})

	_ = store.CreateApproval(&ApprovalRequest{OrgID: "org-a", Command: "pending-a"})
	_ = store.CreateApproval(&ApprovalRequest{OrgID: "org-b", Command: "pending-b"})

	expired := &ApprovalRequest{
		OrgID:     "org-a",
		Command:   "expired-a",
		ExpiresAt: time.Now().Add(-1 * time.Minute),
	}
	_ = store.CreateApproval(expired)

	pendingOrgA := store.GetPendingApprovalsForOrg("org-a")
	if len(pendingOrgA) != 1 {
		t.Fatalf("GetPendingApprovalsForOrg(org-a) count = %d, want 1", len(pendingOrgA))
	}
	if pendingOrgA[0].OrgID != "org-a" {
		t.Fatalf("expected org-a approval, got %q", pendingOrgA[0].OrgID)
	}
}

func TestGetPendingApprovalsForOrgReturnsOperationalPriorityOrder(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "approval-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, _ := NewStore(StoreConfig{
		DataDir:            tmpDir,
		DefaultTimeout:     1 * time.Minute,
		DisablePersistence: true,
	})

	base := time.Now().UTC().Add(30 * time.Minute).Truncate(time.Second)
	create := func(id, orgID string, expiresAt time.Time, risk RiskLevel) {
		req := &ApprovalRequest{
			ID:        id,
			OrgID:     orgID,
			Command:   "test command " + id,
			RiskLevel: risk,
			ExpiresAt: expiresAt,
		}
		if err := store.CreateApproval(req); err != nil {
			t.Fatalf("CreateApproval(%s) error = %v", id, err)
		}
		req.ExpiresAt = expiresAt
		req.RiskLevel = risk
	}

	create("org-b-sooner", "org-b", base.Add(time.Minute), RiskHigh)
	create("org-a-later-critical", "org-a", base.Add(3*time.Minute), RiskLevel("critical"))
	create("org-a-sooner-low", "org-a", base.Add(time.Minute), RiskLow)
	create("org-a-same-expiry-high", "org-a", base.Add(2*time.Minute), RiskHigh)

	assertApprovalOrder(t, store.GetPendingApprovalsForOrg("org-a"), []string{
		"org-a-sooner-low",
		"org-a-same-expiry-high",
		"org-a-later-critical",
	})
}

func TestGetApprovalsByExecution(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "approval-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, _ := NewStore(StoreConfig{
		DataDir:        tmpDir,
		DefaultTimeout: 1 * time.Minute,
	})

	_ = store.CreateApproval(&ApprovalRequest{ExecutionID: "exec-1", Command: "cmd-1"})
	_ = store.CreateApproval(&ApprovalRequest{ExecutionID: "exec-1", Command: "cmd-2"})
	_ = store.CreateApproval(&ApprovalRequest{ExecutionID: "exec-2", Command: "cmd-3"})

	results := store.GetApprovalsByExecution("exec-1")
	if len(results) != 2 {
		t.Fatalf("GetApprovalsByExecution() count = %v, want %v", len(results), 2)
	}
	for _, req := range results {
		if req.ExecutionID != "exec-1" {
			t.Fatalf("GetApprovalsByExecution() returned wrong execution ID: %v", req.ExecutionID)
		}
	}
}

func TestApprove(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "approval-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, _ := NewStore(StoreConfig{
		DataDir:        tmpDir,
		DefaultTimeout: 1 * time.Minute,
	})

	req := &ApprovalRequest{
		Command: "systemctl restart nginx",
	}
	_ = store.CreateApproval(req)

	got, err := store.Approve(req.ID, "admin")
	if err != nil {
		t.Fatalf("Approve() error = %v", err)
	}

	if got.Status != StatusApproved {
		t.Errorf("Approve() status = %v, want %v", got.Status, StatusApproved)
	}
	if got.DecidedBy != "admin" {
		t.Errorf("Approve() DecidedBy = %v, want %v", got.DecidedBy, "admin")
	}
	if got.DecidedAt == nil {
		t.Error("Approve() did not set DecidedAt")
	}
}

func TestDeny(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "approval-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, _ := NewStore(StoreConfig{
		DataDir:        tmpDir,
		DefaultTimeout: 1 * time.Minute,
	})

	req := &ApprovalRequest{
		Command: "rm -rf /tmp/data",
	}
	_ = store.CreateApproval(req)

	got, err := store.Deny(req.ID, "admin", "Too dangerous")
	if err != nil {
		t.Fatalf("Deny() error = %v", err)
	}

	if got.Status != StatusDenied {
		t.Errorf("Deny() status = %v, want %v", got.Status, StatusDenied)
	}
	if got.DenyReason != "Too dangerous" {
		t.Errorf("Deny() DenyReason = %v, want %v", got.DenyReason, "Too dangerous")
	}
}

func TestConsumeApproval_LegacyWithoutCommandHash_MatchingTuple(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "approval-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, _ := NewStore(StoreConfig{
		DataDir:        tmpDir,
		DefaultTimeout: 1 * time.Minute,
	})

	req := &ApprovalRequest{
		Command:    "docker restart web",
		TargetType: "docker",
		TargetID:   "host-1:web",
	}
	_ = store.CreateApproval(req)
	req.CommandHash = "" // Simulate legacy persisted approval without hash
	_, _ = store.Approve(req.ID, "admin")

	consumed, err := store.ConsumeApproval(req.ID, "docker restart web", "docker", "host-1:web")
	if err != nil {
		t.Fatalf("ConsumeApproval() error = %v", err)
	}
	if !consumed.Consumed {
		t.Fatal("expected approval to be consumed")
	}
	if consumed.CommandHash == "" {
		t.Fatal("expected legacy command hash to be backfilled on consume")
	}
}

func TestConsumeApproval_LegacyWithoutCommandHash_MismatchRejected(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "approval-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, _ := NewStore(StoreConfig{
		DataDir:        tmpDir,
		DefaultTimeout: 1 * time.Minute,
	})

	req := &ApprovalRequest{
		Command:    "docker restart web",
		TargetType: "docker",
		TargetID:   "host-1:web",
	}
	_ = store.CreateApproval(req)
	req.CommandHash = "" // Simulate legacy persisted approval without hash
	_, _ = store.Approve(req.ID, "admin")

	if _, err := store.ConsumeApproval(req.ID, "docker restart db", "docker", "host-1:db"); err == nil {
		t.Fatal("expected mismatch for legacy approval without command hash")
	}

	stored, ok := store.GetApproval(req.ID)
	if !ok {
		t.Fatal("expected approval to exist")
	}
	if stored.Consumed {
		t.Fatal("expected approval to remain unconsumed after mismatch")
	}
}

func TestConsumeApproval_RejectsUnsupportedHostTargetTypeInput(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "approval-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, _ := NewStore(StoreConfig{
		DataDir:        tmpDir,
		DefaultTimeout: 1 * time.Minute,
	})

	req := &ApprovalRequest{
		Command:    "uptime",
		TargetType: "agent",
		TargetID:   "node-1",
	}
	_ = store.CreateApproval(req)
	_, _ = store.Approve(req.ID, "admin")

	if _, err := store.ConsumeApproval(req.ID, "uptime", "host", "node-1"); err == nil {
		t.Fatal("expected error for unsupported host target type input")
	}
}

func TestGetStats(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "approval-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, _ := NewStore(StoreConfig{
		DataDir:        tmpDir,
		DefaultTimeout: 1 * time.Minute,
	})

	pending := &ApprovalRequest{Command: "pending"}
	_ = store.CreateApproval(pending)

	approved := &ApprovalRequest{Command: "approved"}
	_ = store.CreateApproval(approved)
	if _, err := store.Approve(approved.ID, "admin"); err != nil {
		t.Fatalf("Approve() error = %v", err)
	}

	denied := &ApprovalRequest{Command: "denied"}
	_ = store.CreateApproval(denied)
	if _, err := store.Deny(denied.ID, "admin", "no"); err != nil {
		t.Fatalf("Deny() error = %v", err)
	}

	expired := &ApprovalRequest{
		Command:   "expired",
		ExpiresAt: time.Now().Add(-time.Minute),
	}
	_ = store.CreateApproval(expired)
	store.CleanupExpired()

	if err := store.StoreExecution(&ExecutionState{ID: "exec-1"}); err != nil {
		t.Fatalf("StoreExecution() error = %v", err)
	}

	stats := store.GetStats()
	if stats["pending"] != 1 || stats["approved"] != 1 || stats["denied"] != 1 || stats["expired"] != 1 {
		t.Fatalf("GetStats() unexpected approval counts: %+v", stats)
	}
	if stats["executions"] != 1 {
		t.Fatalf("GetStats() executions = %v, want %v", stats["executions"], 1)
	}
}

func TestGetStatsForOrg(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "approval-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, _ := NewStore(StoreConfig{
		DataDir:            tmpDir,
		DefaultTimeout:     1 * time.Minute,
		DisablePersistence: true,
	})

	pendingA := &ApprovalRequest{OrgID: "org-a", Command: "pending-a"}
	_ = store.CreateApproval(pendingA)

	approvedA := &ApprovalRequest{OrgID: "org-a", Command: "approved-a"}
	_ = store.CreateApproval(approvedA)
	if _, err := store.Approve(approvedA.ID, "admin"); err != nil {
		t.Fatalf("Approve() error = %v", err)
	}

	deniedA := &ApprovalRequest{OrgID: "org-a", Command: "denied-a"}
	_ = store.CreateApproval(deniedA)
	if _, err := store.Deny(deniedA.ID, "admin", "no"); err != nil {
		t.Fatalf("Deny() error = %v", err)
	}

	expiredA := &ApprovalRequest{
		OrgID:     "org-a",
		Command:   "expired-a",
		ExpiresAt: time.Now().Add(-1 * time.Minute),
	}
	_ = store.CreateApproval(expiredA)

	otherOrg := &ApprovalRequest{OrgID: "org-b", Command: "pending-b"}
	_ = store.CreateApproval(otherOrg)

	store.CleanupExpired()

	stats := store.GetStatsForOrg("org-a")
	if stats["pending"] != 1 || stats["approved"] != 1 || stats["denied"] != 1 || stats["expired"] != 1 {
		t.Fatalf("GetStatsForOrg() unexpected counts: %+v", stats)
	}
	if stats["executions"] != 0 {
		t.Fatalf("GetStatsForOrg() executions = %d, want 0", stats["executions"])
	}
}

func TestApproveNonexistent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "approval-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, _ := NewStore(StoreConfig{
		DataDir:        tmpDir,
		DefaultTimeout: 1 * time.Minute,
	})

	_, err = store.Approve("nonexistent", "admin")
	if err == nil {
		t.Error("Approve() expected error for nonexistent approval")
	}
}

func TestApproveAlreadyDecided(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "approval-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, _ := NewStore(StoreConfig{
		DataDir:        tmpDir,
		DefaultTimeout: 1 * time.Minute,
	})

	req := &ApprovalRequest{
		Command: "test",
	}
	_ = store.CreateApproval(req)
	_, _ = store.Deny(req.ID, "admin", "reason")

	_, err = store.Approve(req.ID, "admin2")
	if err == nil {
		t.Error("Approve() expected error for already decided approval")
	}
}

func TestExecutionState(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "approval-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, _ := NewStore(StoreConfig{
		DataDir:        tmpDir,
		DefaultTimeout: 1 * time.Minute,
	})

	state := &ExecutionState{
		ID: "state-1",
		OriginalRequest: map[string]interface{}{
			"message": "test",
		},
		Messages: []map[string]interface{}{
			{"role": "user", "content": "test"},
		},
	}

	err = store.StoreExecution(state)
	if err != nil {
		t.Fatalf("StoreExecution() error = %v", err)
	}

	got, found := store.GetExecution(state.ID)
	if !found {
		t.Fatal("GetExecution() not found")
	}
	if got.ID != state.ID {
		t.Errorf("GetExecution() ID = %v, want %v", got.ID, state.ID)
	}
	if got.PendingToolCall == nil {
		t.Fatal("GetExecution() should normalize pending tool call map")
	}

	_, found = store.GetExecution("nonexistent")
	if found {
		t.Error("GetExecution() found nonexistent state")
	}
}

func TestStoreExecution_NormalizesEmptyCollections(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "approval-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, _ := NewStore(StoreConfig{
		DataDir:        tmpDir,
		DefaultTimeout: 1 * time.Minute,
	})

	state := &ExecutionState{ID: "state-normalized"}
	if err := store.StoreExecution(state); err != nil {
		t.Fatalf("StoreExecution() error = %v", err)
	}

	if state.OriginalRequest == nil {
		t.Fatal("expected original request map to be normalized")
	}
	if state.Messages == nil {
		t.Fatal("expected messages slice to be normalized")
	}
	if state.PendingToolCall == nil {
		t.Fatal("expected pending tool call map to be normalized")
	}
}

func TestExecutionStatePersistence_NormalizesLoadedCollections(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "approval-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	now := time.Now().UTC()
	raw := fmt.Sprintf(
		`[{"id":"exec-1","createdAt":"%s","expiresAt":"%s"}]`,
		now.Add(-1*time.Minute).Format(time.RFC3339),
		now.Add(1*time.Hour).Format(time.RFC3339),
	)
	if err := os.WriteFile(filepath.Join(tmpDir, "ai_executions.json"), []byte(raw), 0o600); err != nil {
		t.Fatalf("write executions file: %v", err)
	}

	store, _ := NewStore(StoreConfig{
		DataDir:        tmpDir,
		DefaultTimeout: 1 * time.Minute,
	})

	state, found := store.GetExecution("exec-1")
	if !found {
		t.Fatal("expected execution to load")
	}
	if state.OriginalRequest == nil {
		t.Fatal("expected original request map to be normalized after load")
	}
	if state.Messages == nil {
		t.Fatal("expected messages slice to be normalized after load")
	}
	if state.PendingToolCall == nil {
		t.Fatal("expected pending tool call map to be normalized after load")
	}
}

func TestDeleteExecution(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "approval-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, _ := NewStore(StoreConfig{
		DataDir:        tmpDir,
		DefaultTimeout: 1 * time.Minute,
	})

	state := &ExecutionState{
		ID: "state-1",
	}
	_ = store.StoreExecution(state)

	store.DeleteExecution(state.ID)

	_, found := store.GetExecution(state.ID)
	if found {
		t.Error("DeleteExecution() did not delete state")
	}
}

func TestCleanupExpired(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "approval-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, _ := NewStore(StoreConfig{
		DataDir:        tmpDir,
		DefaultTimeout: 1 * time.Millisecond, // Very short timeout
	})

	// Create an approval that will expire immediately
	req := &ApprovalRequest{
		Command: "test",
	}
	_ = store.CreateApproval(req)

	// Wait for expiration
	time.Sleep(10 * time.Millisecond)

	cleaned := store.CleanupExpired()
	if cleaned != 1 {
		t.Errorf("CleanupExpired() cleaned = %v, want %v", cleaned, 1)
	}

	got, found := store.GetApproval(req.ID)
	if !found {
		t.Fatal("Approval should still exist after cleanup")
	}
	if got.Status != StatusExpired {
		t.Errorf("CleanupExpired() status = %v, want %v", got.Status, StatusExpired)
	}
}

func TestStartCleanup(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "approval-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, _ := NewStore(StoreConfig{
		DataDir:        tmpDir,
		DefaultTimeout: 1 * time.Minute,
	})

	ctx, cancel := context.WithCancel(context.Background())
	store.StartCleanup(ctx)

	// Give cleanup goroutine a moment to start
	time.Sleep(10 * time.Millisecond)

	// Cancel should stop the cleanup loop
	cancel()
	time.Sleep(10 * time.Millisecond)
}

func TestAssessRiskLevel(t *testing.T) {
	tests := []struct {
		name       string
		command    string
		targetType string
		want       RiskLevel
	}{
		{
			name:    "high risk rm -rf",
			command: "rm -rf /var/log",
			want:    RiskHigh,
		},
		{
			name:    "high risk dd",
			command: "dd if=/dev/zero of=/dev/sda",
			want:    RiskHigh,
		},
		{
			name:    "high risk apt purge",
			command: "apt purge nginx",
			want:    RiskHigh,
		},
		{
			name:    "medium risk service restart",
			command: "systemctl restart nginx",
			want:    RiskMedium,
		},
		{
			name:    "medium risk docker restart",
			command: "docker restart mycontainer",
			want:    RiskMedium,
		},
		{
			name:    "medium risk apt install",
			command: "apt install htop",
			want:    RiskMedium,
		},
		{
			name:    "low risk diagnostic",
			command: "df -h",
			want:    RiskLow,
		},
		{
			name:    "low risk logs",
			command: "journalctl -u nginx",
			want:    RiskLow,
		},
		{
			name:    "low risk status check",
			command: "systemctl status nginx",
			want:    RiskLow,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AssessRiskLevel(tt.command, tt.targetType)
			if got != tt.want {
				t.Errorf("AssessRiskLevel(%q) = %v, want %v", tt.command, got, tt.want)
			}
		})
	}
}

func TestStorePersistence(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "approval-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create store and add data
	store1, _ := NewStore(StoreConfig{
		DataDir:        tmpDir,
		DefaultTimeout: 1 * time.Hour,
	})

	req := &ApprovalRequest{
		Command:    "test command",
		TargetID:   "host-1",
		TargetName: "webserver",
	}
	_ = store1.CreateApproval(req)
	approvalID := req.ID

	state := &ExecutionState{
		ID: "exec-1",
		OriginalRequest: map[string]interface{}{
			"message": "test",
		},
	}
	_ = store1.StoreExecution(state)

	// Flush debounced writes to disk immediately
	store1.Flush()

	// Create new store from same directory
	store2, _ := NewStore(StoreConfig{
		DataDir:        tmpDir,
		DefaultTimeout: 1 * time.Hour,
	})

	// Verify data was loaded
	loadedApproval, found := store2.GetApproval(approvalID)
	if !found {
		t.Fatal("Approval not persisted")
	}
	if loadedApproval.Command != "test command" {
		t.Errorf("Persisted approval command = %v, want %v", loadedApproval.Command, "test command")
	}

	loadedState, found := store2.GetExecution("exec-1")
	if !found {
		t.Fatal("Execution state not persisted")
	}
	if loadedState.ID != "exec-1" {
		t.Errorf("Persisted state ID = %v, want %v", loadedState.ID, "exec-1")
	}
}

func TestMaxApprovals(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "approval-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, _ := NewStore(StoreConfig{
		DataDir:        tmpDir,
		DefaultTimeout: 1 * time.Hour,
		MaxApprovals:   2,
	})

	// Create first two approvals - should succeed
	_ = store.CreateApproval(&ApprovalRequest{Command: "test1"})
	_ = store.CreateApproval(&ApprovalRequest{Command: "test2"})

	// Third should fail
	err = store.CreateApproval(&ApprovalRequest{Command: "test3"})
	if err == nil {
		t.Error("CreateApproval() expected error when max approvals reached")
	}
}

func TestGlobalStore(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "approval-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, _ := NewStore(StoreConfig{
		DataDir:        tmpDir,
		DefaultTimeout: 1 * time.Minute,
	})

	SetStore(store)
	got := GetStore()

	if got != store {
		t.Error("GetStore() did not return the set store")
	}

	// Reset global store
	SetStore(nil)
}

func assertApprovalOrder(t *testing.T, approvals []*ApprovalRequest, want []string) {
	t.Helper()
	if len(approvals) != len(want) {
		t.Fatalf("approval count = %d, want %d", len(approvals), len(want))
	}
	for i, req := range approvals {
		if req == nil {
			t.Fatalf("approval[%d] = nil, want %q", i, want[i])
		}
		if req.ID != want[i] {
			t.Fatalf("approval[%d].ID = %q, want %q", i, req.ID, want[i])
		}
	}
}
