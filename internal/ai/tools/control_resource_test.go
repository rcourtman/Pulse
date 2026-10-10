package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

type stubAppContainerActionProvider struct {
	calls  []AppContainerActionRequest
	result *AppContainerActionResult
	err    error
}

func (s *stubAppContainerActionProvider) ExecuteAction(_ context.Context, req AppContainerActionRequest) (*AppContainerActionResult, error) {
	s.calls = append(s.calls, req)
	if s.err != nil {
		return nil, s.err
	}
	if s.result == nil {
		return &AppContainerActionResult{
			ResourceID:  req.ResourceID,
			ProviderUID: req.ProviderUID,
			Name:        req.Name,
			Host:        req.Host,
			Platform:    req.Platform,
			Action:      req.Action,
			Status:      "running",
			Output:      "ok",
		}, nil
	}
	result := *s.result
	return &result, nil
}

func TestPulseToolExecutor_ListTools_IncludesPulseControlForNativeAppPlanner(t *testing.T) {
	provider := newTrueNASUnifiedQueryProvider(t)
	executor := NewPulseToolExecutor(ExecutorConfig{
		UnifiedResourceProvider: provider,
		ReadState:               provider.ResourceRegistry,
		TypedActionPlanner: typedActionPlannerFunc(func(context.Context, string, unifiedresources.ActionRequest) (*unifiedresources.ActionPlan, error) {
			return nil, nil
		}),
		ControlLevel: ControlLevelControlled,
	})

	tools := executor.ListTools()
	found := false
	for _, tool := range tools {
		if tool.Name == "pulse_control" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected pulse_control to be available with native app planner, got %+v", tools)
	}
}

func TestPulseToolExecutor_ListTools_PulseControlDescriptionStaysCapabilityBounded(t *testing.T) {
	executor := NewPulseToolExecutor(ExecutorConfig{
		StateProvider: &mockStateProvider{},
		TypedActionPlanner: typedActionPlannerFunc(func(context.Context, string, unifiedresources.ActionRequest) (*unifiedresources.ActionPlan, error) {
			return nil, nil
		}),
		ControlLevel: ControlLevelControlled,
	})

	tools := executor.ListTools()
	for _, tool := range tools {
		if tool.Name != "pulse_control" {
			continue
		}
		if !strings.Contains(tool.Description, "explicitly advertises the requested capability") {
			t.Fatalf("expected pulse_control description to stay capability-bounded, got %q", tool.Description)
		}
		if !strings.Contains(tool.Description, "never executes commands") {
			t.Fatalf("expected pulse_control description to name the command-free boundary, got %q", tool.Description)
		}
		if !strings.Contains(tool.Description, "Proxmox VM and LXC lifecycle actions do not require the QEMU guest agent") {
			t.Fatalf("expected pulse_control description to distinguish hypervisor lifecycle from guest-agent operations, got %q", tool.Description)
		}
		if action := tool.InputSchema.Properties["action"].Description; !strings.Contains(action, "Advertised resource capability") {
			t.Fatalf("expected pulse_control action schema to describe shared action gating, got %q", action)
		}
		for _, retired := range []string{"guest_id", "command", "target_host", "run_on_host", "force"} {
			if _, exists := tool.InputSchema.Properties[retired]; exists {
				t.Fatalf("pulse_control schema still exposes retired %q input", retired)
			}
		}
		return
	}
	t.Fatalf("expected pulse_control to be available, got %+v", tools)
}

func TestExecuteControlResource_TrueNASAppUsesNativeActionProvider(t *testing.T) {
	provider := newTrueNASUnifiedQueryProvider(t)
	resolved := &mockResolvedContext{
		resources: make(map[string]ResolvedResourceInfo),
		aliases:   make(map[string]ResolvedResourceInfo),
	}
	actionProvider := &stubAppContainerActionProvider{
		result: &AppContainerActionResult{
			ResourceID:  "app-container:truenas-main:nextcloud",
			ProviderUID: "nextcloud",
			Name:        "Nextcloud",
			Host:        "truenas-main",
			Platform:    "truenas",
			Action:      "restart",
			Status:      "running",
			Output:      "restart app Nextcloud on truenas-main; current state=running",
		},
	}
	store := unifiedresources.NewMemoryStore()
	executor := NewPulseToolExecutor(ExecutorConfig{
		UnifiedResourceProvider:    provider,
		ReadState:                  provider.ResourceRegistry,
		AppContainerActionProvider: actionProvider,
		ActionAuditStore:           store,
		TypedActionPlanner: typedActionPlannerFunc(func(_ context.Context, _ string, req unifiedresources.ActionRequest) (*unifiedresources.ActionPlan, error) {
			return &unifiedresources.ActionPlan{
				ActionID:         "action-1",
				RequestID:        req.RequestID,
				Allowed:          true,
				RequiresApproval: true,
				ApprovalPolicy:   unifiedresources.ApprovalAdmin,
				PlanHash:         "hash-1",
			}, nil
		}),
	})
	executor.SetResolvedContext(resolved)

	if _, err := executor.executeGetResource(context.Background(), map[string]interface{}{
		"resource_type": "app-container",
		"resource_id":   "nextcloud",
	}); err != nil {
		t.Fatalf("seed resolved context: unexpected error: %v", err)
	}

	result, err := executor.executeControl(context.Background(), map[string]interface{}{
		"type":        "resource",
		"resource_id": "Nextcloud",
		"action":      "restart",
	})
	if err != nil {
		t.Fatalf("executeControl(type=resource): unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success result, got %+v", result)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(result.Content[0].Text), &payload); err != nil {
		t.Fatalf("decode result payload: %v", err)
	}
	if payload["action_id"] != "action-1" || payload["capability"] != "restart" {
		t.Fatalf("expected canonical action plan, got %+v", payload)
	}

	if len(actionProvider.calls) != 0 {
		t.Fatalf("model planning must not call the native provider, got %+v", actionProvider.calls)
	}
}

type typedActionPlannerFunc func(context.Context, string, unifiedresources.ActionRequest) (*unifiedresources.ActionPlan, error)

func (f typedActionPlannerFunc) PlanTypedAction(ctx context.Context, orgID string, req unifiedresources.ActionRequest) (*unifiedresources.ActionPlan, error) {
	return f(ctx, orgID, req)
}

// Tool exposure follows the plan-only lifecycle boundary, not an execution
// transport. Read-only policy must still hide it even when planning is wired.
func TestPulseControlAvailabilityUsesPlanner(t *testing.T) {
	planner := typedActionPlannerFunc(func(context.Context, string, unifiedresources.ActionRequest) (*unifiedresources.ActionPlan, error) {
		t.Fatal("listing tools must not plan or execute")
		return nil, nil
	})
	for _, tc := range []struct {
		name string
		cfg  ExecutorConfig
		want bool
	}{
		{"planner", ExecutorConfig{StateProvider: &mockStateProvider{}, TypedActionPlanner: planner, ControlLevel: ControlLevelControlled}, true},
		{"read_only", ExecutorConfig{StateProvider: &mockStateProvider{}, TypedActionPlanner: planner, ControlLevel: ControlLevelReadOnly}, false},
		{"no_state", ExecutorConfig{TypedActionPlanner: planner, ControlLevel: ControlLevelControlled}, false},
		{"agent_only", ExecutorConfig{StateProvider: &mockStateProvider{}, AgentServer: &mockAgentServer{}, ControlLevel: ControlLevelControlled}, false},
		{"app_executor_only", ExecutorConfig{StateProvider: &mockStateProvider{}, AppContainerActionProvider: &stubAppContainerActionProvider{}, ControlLevel: ControlLevelControlled}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := NewPulseToolExecutor(tc.cfg)
			offered, governed := false, false
			for _, tool := range e.ListTools() {
				offered = offered || tool.Name == "pulse_control"
			}
			for _, tool := range e.ListToolGovernance() {
				governed = governed || tool.Name == "pulse_control"
			}
			if offered != tc.want || governed != tc.want {
				t.Fatalf("offered=%v governed=%v want=%v", offered, governed, tc.want)
			}
		})
	}
}

func TestAssistantProjectedModesStillOnlyPlanAndRespectExecuteAuthority(t *testing.T) {
	for _, raw := range []string{config.ControlLevelReadOnly, config.ControlLevelControlled, config.ControlLevelAutonomous} {
		for _, authority := range []bool{false, true} {
			t.Run(raw+map[bool]string{true: "/authorised", false: "/unauthorised"}[authority], func(t *testing.T) {
				provider := newTrueNASUnifiedQueryProvider(t)
				canonicalID := ""
				for _, resource := range provider.GetByType(unifiedresources.ResourceTypeAppContainer) {
					if strings.EqualFold(resource.Name, "Nextcloud") {
						if canonicalID != "" {
							t.Fatal("fixture has ambiguous Nextcloud resources")
						}
						canonicalID = resource.ID
					}
				}
				if canonicalID == "" {
					t.Fatal("fixture has no canonical Nextcloud resource")
				}
				native := &stubAppContainerActionProvider{}
				planned := 0
				executor := NewPulseToolExecutor(ExecutorConfig{
					UnifiedResourceProvider: provider, ReadState: provider.ResourceRegistry,
					AppContainerActionProvider: native,
					ControlLevel:               ControlLevel(config.AssistantControlLevel(raw)),
					TypedActionPlanner: typedActionPlannerFunc(func(_ context.Context, org string, req unifiedresources.ActionRequest) (*unifiedresources.ActionPlan, error) {
						planned++
						if org != "lab" || req.ResourceID != canonicalID || req.CapabilityName != "restart" {
							t.Fatalf("plan lost scope: %q/%s", org, req.ResourceID)
						}
						return &unifiedresources.ActionPlan{ActionID: "synthetic-action", Allowed: true, RequiresApproval: true, ApprovalPolicy: unifiedresources.ApprovalAdmin, PlanHash: "synthetic-plan"}, nil
					}),
				})
				executor.SetOrgID("lab")
				executor.ApplyExecutionProfile(ProfileInteractiveAssistant)
				executor.SetExecuteAuthority(authority)
				result, err := executor.ExecuteTool(context.Background(), "pulse_control", map[string]interface{}{"type": "resource", "resource_id": "Nextcloud", "action": "restart"})
				wantPlan := raw != config.ControlLevelReadOnly && authority
				if wantPlan {
					if err != nil || result.IsError {
						t.Fatalf("expected plan: %v / %+v", err, result)
					}
					var payload map[string]any
					if err := json.Unmarshal([]byte(result.Content[0].Text), &payload); err != nil {
						t.Fatal(err)
					}
					if payload["execution_requested"] != false || payload["requires_approval"] != true || payload["approval_policy"] != string(unifiedresources.ApprovalAdmin) {
						t.Fatalf("plan granted execution: %+v", payload)
					}
					if planned != 1 {
						t.Fatalf("plans=%d", planned)
					}
				} else {
					// Read-only returns the existing informational disabled-mode reply,
					// not a failed native action. Unauthorised planning is an error.
					if raw == config.ControlLevelReadOnly {
						if err != nil || len(result.Content) != 1 || !strings.Contains(result.Content[0].Text, "Control tools are disabled") {
							t.Fatalf("missing read-only refusal: %v / %+v", err, result)
						}
					} else if err == nil && !result.IsError {
						t.Fatalf("unauthorised/read-only plan accepted: %+v", result)
					}
					if planned != 0 {
						t.Fatalf("unauthorised plans=%d", planned)
					}
				}
				if len(native.calls) != 0 {
					t.Fatal("Assistant plan executed against native provider")
				}
			})
		}
	}
}
