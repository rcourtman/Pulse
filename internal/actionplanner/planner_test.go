package actionplanner

import (
	"errors"
	"strings"
	"testing"
	"time"

	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestPlannerBuildsDeterministicGovernedPlan(t *testing.T) {
	now := time.Date(2026, 5, 3, 9, 30, 0, 0, time.UTC)
	parentID := "agent:node-1"
	resource := unified.Resource{
		ID:        "vm:42",
		Type:      unified.ResourceTypeVM,
		Name:      "web-42",
		Status:    unified.StatusWarning,
		LastSeen:  now.Add(-time.Minute),
		UpdatedAt: now.Add(-30 * time.Second),
		ParentID:  &parentID,
		Capabilities: []unified.ResourceCapability{
			{
				Name:                 "restart",
				Type:                 unified.CapabilityTypeCommon,
				Description:          "Restart the VM",
				MinimumApprovalLevel: unified.ApprovalAdmin,
				InternalHandler:      "proxmox.vm.restart",
				Params: []unified.CapabilityParam{
					{Name: "mode", Type: "string", Required: true, Enum: []string{"graceful", "force"}},
				},
			},
		},
		Relationships: []unified.ResourceRelationship{
			{
				SourceID: "vm:42",
				TargetID: "service:web",
				Type:     unified.RelDependsOn,
				Active:   true,
			},
		},
	}
	req := unified.ActionRequest{
		RequestID:      "agent-run-123",
		ResourceID:     " vm:42 ",
		CapabilityName: "restart",
		Params:         map[string]any{"mode": "graceful"},
		Reason:         "Recover after confirmed outage",
		RequestedBy:    "agent:oncall-helper",
	}

	planner := Planner{Now: func() time.Time { return now }}
	plan, err := planner.Plan(req, resource)
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	second, err := planner.Plan(req, resource)
	if err != nil {
		t.Fatalf("Plan() second error = %v", err)
	}

	if plan.ActionID == "" || plan.ActionID != second.ActionID {
		t.Fatalf("action id is not deterministic: first=%q second=%q", plan.ActionID, second.ActionID)
	}
	if plan.PlanHash == "" || plan.PlanHash != second.PlanHash {
		t.Fatalf("plan hash is not deterministic: first=%q second=%q", plan.PlanHash, second.PlanHash)
	}
	if !plan.Allowed {
		t.Fatalf("Allowed = false, want true")
	}
	if !plan.RequiresApproval {
		t.Fatalf("RequiresApproval = false, want true")
	}
	if plan.ApprovalPolicy != unified.ApprovalAdmin {
		t.Fatalf("ApprovalPolicy = %q, want %q", plan.ApprovalPolicy, unified.ApprovalAdmin)
	}
	if !plan.PlannedAt.Equal(now) {
		t.Fatalf("PlannedAt = %s, want %s", plan.PlannedAt, now)
	}
	if !plan.ExpiresAt.Equal(now.Add(DefaultPlanTTL)) {
		t.Fatalf("ExpiresAt = %s, want %s", plan.ExpiresAt, now.Add(DefaultPlanTTL))
	}
	if len(plan.PredictedBlastRadius) != 3 ||
		plan.PredictedBlastRadius[0] != "vm:42" ||
		plan.PredictedBlastRadius[1] != "agent:node-1" ||
		plan.PredictedBlastRadius[2] != "service:web" {
		t.Fatalf("PredictedBlastRadius = %#v", plan.PredictedBlastRadius)
	}
	if plan.Preflight == nil {
		t.Fatalf("Preflight is nil")
	}
	if plan.Preflight.Target != "vm:42" {
		t.Fatalf("Preflight.Target = %q, want vm:42", plan.Preflight.Target)
	}
	if plan.Preflight.DryRunAvailable {
		t.Fatalf("DryRunAvailable = true, want false without provider dry-run contract")
	}
}

func TestPlannerBuildsDryRunOnlyPlanWithoutExecutionApproval(t *testing.T) {
	now := time.Date(2026, 5, 5, 9, 0, 0, 0, time.UTC)
	resource := unified.Resource{
		ID:     "vm:42",
		Type:   unified.ResourceTypeVM,
		Name:   "web-42",
		Status: unified.StatusOnline,
		Capabilities: []unified.ResourceCapability{
			{
				Name:                 "restart",
				Type:                 unified.CapabilityTypeCommon,
				Description:          "Restart the VM",
				MinimumApprovalLevel: unified.ApprovalDryRun,
			},
		},
	}
	req := unified.ActionRequest{
		RequestID:      "agent-run-dry-run",
		ResourceID:     "vm:42",
		CapabilityName: "restart",
		Reason:         "Validate restart path without execution",
		RequestedBy:    "agent:oncall-helper",
	}

	plan, err := (Planner{Now: func() time.Time { return now }}).Plan(req, resource)
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if plan.ApprovalPolicy != unified.ApprovalDryRun {
		t.Fatalf("ApprovalPolicy = %q, want %q", plan.ApprovalPolicy, unified.ApprovalDryRun)
	}
	if plan.RequiresApproval {
		t.Fatalf("RequiresApproval = true, want false because dry-run-only plans cannot be executed")
	}
	if plan.Preflight == nil || !strings.Contains(strings.Join(plan.Preflight.SafetyChecks, " "), "dry-run-only") {
		t.Fatalf("dry-run-only safety checks missing: %#v", plan.Preflight)
	}
	if !strings.Contains(plan.Message, "dry-run only") {
		t.Fatalf("plan message = %q", plan.Message)
	}
}

func TestPlannerRejectsUndeclaredParams(t *testing.T) {
	resource := unified.Resource{
		ID:   "vm:42",
		Type: unified.ResourceTypeVM,
		Capabilities: []unified.ResourceCapability{
			{Name: "restart", Type: unified.CapabilityTypeCommon, MinimumApprovalLevel: unified.ApprovalAdmin},
		},
	}
	req := unified.ActionRequest{
		RequestID:      "agent-run-123",
		ResourceID:     "vm:42",
		CapabilityName: "restart",
		Params:         map[string]any{"force": true},
		Reason:         "Recover after confirmed outage",
		RequestedBy:    "agent:oncall-helper",
	}

	_, err := Planner{}.Plan(req, resource)
	validationErr, ok := AsValidationError(err)
	if !ok {
		t.Fatalf("Plan() error = %v, want validation error", err)
	}
	if validationErr.Field != "params.force" {
		t.Fatalf("validation field = %q, want params.force", validationErr.Field)
	}
}

func TestResourceVersionIgnoresObservationOnlyTimestamps(t *testing.T) {
	base := unified.Resource{
		ID:        "vm:42",
		Type:      unified.ResourceTypeVM,
		Name:      "web-42",
		Status:    unified.StatusOnline,
		LastSeen:  time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC),
	}
	refreshed := base
	refreshed.LastSeen = base.LastSeen.Add(time.Minute)
	refreshed.UpdatedAt = base.UpdatedAt.Add(time.Minute)

	if got, want := ResourceVersion(refreshed), ResourceVersion(base); got != want {
		t.Fatalf("ResourceVersion changed for observation-only timestamp drift: got %q want %q", got, want)
	}

	changed := base
	changed.Status = unified.StatusWarning
	if got, unchanged := ResourceVersion(changed), ResourceVersion(base); got == unchanged {
		t.Fatalf("ResourceVersion did not change for status drift: %q", got)
	}
}

func TestPlannerReturnsCapabilityNotFound(t *testing.T) {
	resource := unified.Resource{ID: "vm:42", Type: unified.ResourceTypeVM}
	req := unified.ActionRequest{
		RequestID:      "agent-run-123",
		ResourceID:     "vm:42",
		CapabilityName: "restart",
		Reason:         "Recover after confirmed outage",
		RequestedBy:    "agent:oncall-helper",
	}

	_, err := Planner{}.Plan(req, resource)
	if !errors.Is(err, ErrCapabilityNotFound) {
		t.Fatalf("Plan() error = %v, want ErrCapabilityNotFound", err)
	}
}
