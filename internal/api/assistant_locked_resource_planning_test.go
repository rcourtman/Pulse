package api

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// assistantPlanningProvider is the unified inventory the Assistant tool layer
// resolves pulse_control references against.
type assistantPlanningProvider struct{ resources []unified.Resource }

func (p assistantPlanningProvider) GetByType(t unified.ResourceType) []unified.Resource {
	var out []unified.Resource
	for _, resource := range p.resources {
		if resource.Type == t {
			out = append(out, resource)
		}
	}
	return out
}

// assistantTenantSeedProvider seeds one tenant with the VM and every other
// tenant with nothing.
type assistantTenantSeedProvider struct {
	orgID    string
	snapshot models.StateSnapshot
	vm       unified.Resource
}

func (p assistantTenantSeedProvider) GetStateForTenant(string) models.StateSnapshot {
	return p.snapshot
}

func (p assistantTenantSeedProvider) UnifiedReadStateForTenant(orgID string) unified.ReadState {
	return SnapshotReadState(p.GetStateForTenant(orgID))
}

func (p assistantTenantSeedProvider) UnifiedResourceSnapshotForTenant(orgID string) ([]unified.Resource, time.Time) {
	if orgID != p.orgID {
		return nil, p.snapshot.LastUpdate
	}
	return []unified.Resource{p.vm}, p.snapshot.LastUpdate
}

// newAssistantPlanningHarness wires the production Assistant planning chain
// for one tenant: the pulse_control tool executor, the assistantTypedActionPlanner,
// the real action lifecycle and the real Proxmox guest executor, with only the
// typed node runner faked. The seeded VM is named "guest" with VMID 160. The
// returned store is the tenant's lifecycle store, which is where the operator
// writes a lock and where dispatch reads it.
func newAssistantPlanningHarness(t *testing.T, orgID string) (*tools.PulseToolExecutor, unified.ResourceStore, unified.Resource) {
	t.Helper()
	now := time.Now().UTC()
	vm := proxmoxGuestActionResource("vm:160", unified.ResourceTypeVM, "running", now)
	h := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
	snapshot := models.StateSnapshot{LastUpdate: now}
	if orgID == "default" {
		h.SetStateProvider(resourceUnifiedSeedProvider{snapshot: snapshot, resources: []unified.Resource{vm}})
	} else {
		h.SetTenantStateProvider(assistantTenantSeedProvider{orgID: orgID, snapshot: snapshot, vm: vm})
	}
	h.SetActionExecutor(newRoutedActionExecutor(h, newProxmoxGuestActionExecutor(h, &fakeProxmoxActionAgentCommander{}, nil)))

	store, err := h.getStore(orgID)
	if err != nil {
		t.Fatalf("getStore(%q): %v", orgID, err)
	}
	exec := tools.NewPulseToolExecutor(tools.ExecutorConfig{})
	exec.SetOrgID(orgID)
	exec.SetControlLevel(tools.ControlLevelControlled)
	exec.SetUnifiedResourceProvider(assistantPlanningProvider{resources: []unified.Resource{vm}})
	exec.SetTypedActionPlanner(assistantTypedActionPlanner{resources: h})
	return exec, store, vm
}

func assistantRestart(t *testing.T, exec *tools.PulseToolExecutor, ref string) (result tools.CallToolResult, decoded map[string]any, raw string) {
	t.Helper()
	result, err := exec.ExecuteTool(context.Background(), "pulse_control", map[string]any{"type": "resource", "resource_id": ref, "action": "restart"})
	if err != nil {
		t.Fatalf("pulse_control: %v", err)
	}
	var text strings.Builder
	for _, content := range result.Content {
		text.WriteString(content.Text)
	}
	raw = text.String()
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatalf("pulse_control result is not JSON: %v: %s", err, raw)
	}
	return result, decoded, raw
}

func assertAssistantPlanned(t *testing.T, result tools.CallToolResult, decoded map[string]any, raw string) {
	t.Helper()
	if result.IsError || decoded["planned"] != true || decoded["requested_action"] != "restart" || decoded["capability"] != "reboot" || decoded["execution_requested"] != false {
		t.Fatalf("expected a saved, unexecuted plan, got isError=%v %s", result.IsError, raw)
	}
}

// assertAssistantLockRefusal checks the exact blocked-tool-response shape the
// agentic loop forwards to the model.
func assertAssistantLockRefusal(t *testing.T, result tools.CallToolResult, decoded map[string]any, raw string) {
	t.Helper()
	if !result.IsError || decoded["ok"] != false || decoded["planned"] != nil {
		t.Fatalf("a blocked resource must return an error result without a plan, got isError=%v %s", result.IsError, raw)
	}
	errObj, _ := decoded["error"].(map[string]any)
	details, _ := errObj["details"].(map[string]any)
	if errObj["code"] != "ACTION_NOT_ALLOWED" || errObj["blocked"] != true || details["reason_code"] != "resource_remediation_locked" || details["requested_action"] != "restart" {
		t.Fatalf("unexpected refusal envelope: %s", raw)
	}
	message, _ := errObj["message"].(string)
	boundary, _ := details["policy_boundary"].(string)
	for _, want := range []string{"Never auto-remediate", "Retired"} {
		if !strings.Contains(message, want) {
			t.Fatalf("refusal message must name both blocking states, missing %q: %s", want, message)
		}
	}
	for _, want := range []string{"Operator overrides", "turn off Never auto-remediate", "set it back to Active", "then save"} {
		if !strings.Contains(boundary, want) {
			t.Fatalf("policy boundary must say how to clear the block, missing %q: %s", want, boundary)
		}
	}
}

func assistantActionAuditCount(t *testing.T, store unified.ResourceStore, resourceID string) int {
	t.Helper()
	audits, err := store.GetActionAudits(resourceID, time.Time{}, 10)
	if err != nil {
		t.Fatalf("GetActionAudits: %v", err)
	}
	return len(audits)
}

func TestAssistantPlansRestartForUnlockedGuest(t *testing.T) {
	exec, store, vm := newAssistantPlanningHarness(t, "default")

	result, decoded, raw := assistantRestart(t, exec, "guest")
	assertAssistantPlanned(t, result, decoded, raw)
	if got := assistantActionAuditCount(t, store, vm.ID); got != 1 {
		t.Fatalf("planned action audits = %d, want 1", got)
	}
}

// The operator lock ("Never auto-remediate") and a retired lifecycle refuse
// every dispatch for the resource, approved or not. The Assistant must say so
// at planning time rather than leave a plan in Actions that can never run.
func TestAssistantDeclinesToPlanAgainstBlockedGuest(t *testing.T) {
	for _, tc := range []struct {
		name  string
		state unified.ResourceOperatorState
	}{
		{"never auto-remediate", unified.ResourceOperatorState{NeverAutoRemediate: true}},
		{"retired lifecycle", unified.ResourceOperatorState{LifecycleState: unified.LifecycleStateRetired}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			exec, store, vm := newAssistantPlanningHarness(t, "default")
			state := tc.state
			state.CanonicalID, state.SetAt, state.SetBy = vm.ID, time.Now().UTC(), "operator:test"
			if err := store.SetResourceOperatorState(state); err != nil {
				t.Fatalf("SetResourceOperatorState: %v", err)
			}

			result, decoded, raw := assistantRestart(t, exec, "guest")
			assertAssistantLockRefusal(t, result, decoded, raw)
			if got := assistantActionAuditCount(t, store, vm.ID); got != 0 {
				t.Fatalf("a refused plan wrote %d action audit(s)", got)
			}

			if err := store.ClearResourceOperatorState(vm.ID); err != nil {
				t.Fatalf("ClearResourceOperatorState: %v", err)
			}
			result, decoded, raw = assistantRestart(t, exec, "guest")
			assertAssistantPlanned(t, result, decoded, raw)
		})
	}
}

// Only NeverAutoRemediate and a retired lifecycle block remediation. The other
// operator overrides change alerting and Patrol attention, not whether the
// Assistant may propose an action.
func TestAssistantStillPlansWhenOnlyNonBlockingOverridesAreSet(t *testing.T) {
	for _, tc := range []struct {
		name  string
		state unified.ResourceOperatorState
	}{
		{"intentionally offline", unified.ResourceOperatorState{IntentionallyOffline: true}},
		{"muted", unified.ResourceOperatorState{MonitoringMode: unified.MonitoringModeMuted}},
		{"high criticality", unified.ResourceOperatorState{Criticality: unified.CriticalityHigh}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			exec, store, vm := newAssistantPlanningHarness(t, "default")
			state := tc.state
			state.CanonicalID, state.SetAt, state.SetBy = vm.ID, time.Now().UTC(), "operator:test"
			if err := store.SetResourceOperatorState(state); err != nil {
				t.Fatalf("SetResourceOperatorState: %v", err)
			}

			result, decoded, raw := assistantRestart(t, exec, "guest")
			assertAssistantPlanned(t, result, decoded, raw)
		})
	}
}

// A non-default tenant keeps its operator state in the lifecycle store under its
// own org. The planner must read that store, not the default tenant's.
func TestAssistantReadsTheTenantsOwnLockState(t *testing.T) {
	const orgID = "org-b"
	exec, store, vm := newAssistantPlanningHarness(t, orgID)

	if err := store.SetResourceOperatorState(unified.ResourceOperatorState{
		CanonicalID: vm.ID, NeverAutoRemediate: true, SetAt: time.Now().UTC(), SetBy: "operator:test",
	}); err != nil {
		t.Fatalf("SetResourceOperatorState: %v", err)
	}

	result, decoded, raw := assistantRestart(t, exec, "guest")
	assertAssistantLockRefusal(t, result, decoded, raw)
	if got := assistantActionAuditCount(t, store, vm.ID); got != 0 {
		t.Fatalf("a refused tenant plan wrote %d action audit(s)", got)
	}

	if err := store.ClearResourceOperatorState(vm.ID); err != nil {
		t.Fatalf("ClearResourceOperatorState: %v", err)
	}
	result, decoded, raw = assistantRestart(t, exec, "guest")
	assertAssistantPlanned(t, result, decoded, raw)
}
