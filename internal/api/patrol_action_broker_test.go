package api

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
)

func newPatrolBrokerTestHandlers(t *testing.T, minimumApproval unified.ActionApprovalLevel) (*ResourceHandlers, *stubActionExecutor) {
	t.Helper()
	now := time.Now().UTC()
	h := NewResourceHandlers(&config.Config{DataPath: t.TempDir()})
	h.SetStateProvider(resourceUnifiedSeedProvider{
		snapshot: models.StateSnapshot{LastUpdate: now},
		resources: []unified.Resource{
			{
				ID:        "vm:42",
				Type:      unified.ResourceTypeVM,
				Name:      "web-42",
				Status:    unified.StatusWarning,
				LastSeen:  now,
				UpdatedAt: now,
				Sources:   []unified.DataSource{unified.SourceProxmox},
				Capabilities: []unified.ResourceCapability{
					{
						Name:                 "restart",
						Type:                 unified.CapabilityTypeCommon,
						Description:          "Restart the VM",
						MinimumApprovalLevel: minimumApproval,
						InternalHandler:      "proxmox.vm.restart",
						Params: []unified.CapabilityParam{
							{Name: "mode", Type: "string", Required: true, Enum: []string{"graceful", "force"}},
						},
					},
					{
						Name:                 "join_cluster",
						Type:                 unified.CapabilityTypeCommon,
						Description:          "Join an authenticated cluster",
						MinimumApprovalLevel: unified.ApprovalAdmin,
						InternalHandler:      "proxmox.vm.join",
						Params: []unified.CapabilityParam{
							{Name: "join_token", Type: "string", Required: true, IsSensitive: true},
						},
					},
				},
			},
		},
	})
	executor := &stubActionExecutor{result: &unified.ExecutionResult{Success: true}}
	h.SetActionExecutor(executor)
	return h, executor
}

func patrolTestProposal() aicontracts.ActionProposal {
	return aicontracts.ActionProposal{
		ProposalID:      "prop-1",
		FindingID:       "finding-1",
		InvestigationID: "inv-1",
		ResourceID:      "vm:42",
		CapabilityName:  "restart",
		Params:          map[string]any{"mode": "graceful"},
		Reason:          "Recover after confirmed outage",
	}
}

func TestPatrolActionBrokerSubmitPlansThroughCanonicalLifecycle(t *testing.T) {
	h, executor := newPatrolBrokerTestHandlers(t, unified.ApprovalAdmin)
	broker := NewPatrolActionBroker("default", h)

	disposition, err := broker.Submit(context.Background(), patrolTestProposal())
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if disposition.ActionID == "" {
		t.Fatal("disposition has no action id")
	}
	if disposition.State != string(unified.ActionStatePending) {
		t.Fatalf("state = %q, want pending_approval", disposition.State)
	}
	if !disposition.Plan.RequiresApproval || disposition.Plan.ApprovalPolicy != string(unified.ApprovalAdmin) {
		t.Fatalf("plan projection = %#v", disposition.Plan)
	}
	if executor.calls != 0 {
		t.Fatalf("submit must never execute, executor calls = %d", executor.calls)
	}

	store, err := h.getStore("default")
	if err != nil {
		t.Fatalf("getStore: %v", err)
	}
	record, ok, err := store.GetActionAudit(disposition.ActionID)
	if err != nil || !ok {
		t.Fatalf("GetActionAudit: ok=%v err=%v", ok, err)
	}
	if record.Request.RequestedBy != patrolActionBrokerActor {
		t.Fatalf("requestedBy = %q, want %q", record.Request.RequestedBy, patrolActionBrokerActor)
	}
	if record.Origin == nil ||
		record.Origin.Surface != patrolActionOriginSurface ||
		record.Origin.FindingID != "finding-1" ||
		record.Origin.InvestigationID != "inv-1" ||
		record.Origin.ProposalID != "prop-1" {
		t.Fatalf("audit origin = %#v", record.Origin)
	}
}

func TestPatrolActionBrokerSubmitIsPlanOnlyForApprovalNone(t *testing.T) {
	h, executor := newPatrolBrokerTestHandlers(t, unified.ApprovalNone)
	broker := NewPatrolActionBroker("default", h)

	disposition, err := broker.Submit(context.Background(), patrolTestProposal())
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if disposition.State != string(unified.ActionStatePlanned) {
		t.Fatalf("state = %q, want planned", disposition.State)
	}
	if disposition.Plan.RequiresApproval {
		t.Fatal("ApprovalNone capability should not require approval")
	}
	if executor.calls != 0 {
		t.Fatalf("submit must never auto-execute, executor calls = %d", executor.calls)
	}
}

func TestPatrolActionBrokerRejectsSensitiveParams(t *testing.T) {
	h, executor := newPatrolBrokerTestHandlers(t, unified.ApprovalAdmin)
	broker := NewPatrolActionBroker("default", h)

	proposal := patrolTestProposal()
	proposal.CapabilityName = "join_cluster"
	proposal.Params = map[string]any{"join_token": "sekret-token-value"}
	_, err := broker.Submit(context.Background(), proposal)
	if !errors.Is(err, aicontracts.ErrSensitiveParamsRequireOperator) {
		t.Fatalf("error = %v, want ErrSensitiveParamsRequireOperator", err)
	}
	if executor.calls != 0 {
		t.Fatalf("executor calls = %d, want 0", executor.calls)
	}

	store, err := h.getStore("default")
	if err != nil {
		t.Fatalf("getStore: %v", err)
	}
	audits, err := store.GetActionAudits("vm:42", time.Time{}, 10)
	if err != nil {
		t.Fatalf("GetActionAudits: %v", err)
	}
	if len(audits) != 0 {
		t.Fatalf("sensitive-param proposals must not persist audits, got %d", len(audits))
	}
}

func TestPatrolActionBrokerCapabilitiesCatalog(t *testing.T) {
	h, _ := newPatrolBrokerTestHandlers(t, unified.ApprovalAdmin)
	broker := NewPatrolActionBroker("default", h)

	catalog, err := broker.Capabilities(context.Background(), "vm:42")
	if err != nil {
		t.Fatalf("Capabilities: %v", err)
	}
	if catalog.ResourceID != "vm:42" || len(catalog.Capabilities) != 2 {
		t.Fatalf("catalog = %#v", catalog)
	}
	byName := map[string]aicontracts.ActionCapabilityInfo{}
	for _, capability := range catalog.Capabilities {
		byName[capability.Name] = capability
	}
	restart := byName["restart"]
	if restart.MinimumApprovalLevel != string(unified.ApprovalAdmin) || len(restart.Params) != 1 || restart.Params[0].Sensitive {
		t.Fatalf("restart capability = %#v", restart)
	}
	join := byName["join_cluster"]
	if len(join.Params) != 1 || !join.Params[0].Sensitive {
		t.Fatalf("join_cluster capability must mark its token sensitive: %#v", join)
	}

	if _, err := broker.Capabilities(context.Background(), "vm:404"); err == nil {
		t.Fatal("unknown resource must error")
	}
}
