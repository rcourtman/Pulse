package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

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

func TestPulseToolExecutor_ListTools_IncludesPulseControlForNativeAppProvider(t *testing.T) {
	provider := newTrueNASUnifiedQueryProvider(t)
	executor := NewPulseToolExecutor(ExecutorConfig{
		UnifiedResourceProvider:    provider,
		ReadState:                  provider.ResourceRegistry,
		AppContainerActionProvider: &stubAppContainerActionProvider{},
		ControlLevel:               ControlLevelControlled,
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
		t.Fatalf("expected pulse_control to be available with native app action provider, got %+v", tools)
	}
}

func TestPulseToolExecutor_ListTools_PulseControlDescriptionStaysCapabilityBounded(t *testing.T) {
	executor := NewPulseToolExecutor(ExecutorConfig{
		StateProvider: &mockStateProvider{},
		AgentServer:   &mockAgentServer{},
		ControlLevel:  ControlLevelControlled,
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
		if action := tool.InputSchema.Properties["action"].Description; !strings.Contains(action, "Advertised resource capability") {
			t.Fatalf("expected pulse_control action schema to describe shared action gating, got %q", action)
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
