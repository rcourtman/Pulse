package tools

import (
	"context"
	"encoding/json"
	"testing"
	"time"

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
	if payload["platform"] != "truenas" {
		t.Fatalf("expected truenas platform, got %+v", payload)
	}
	if payload["action"] != "restart" || payload["status"] != "running" {
		t.Fatalf("unexpected action payload: %+v", payload)
	}

	if len(actionProvider.calls) != 1 {
		t.Fatalf("expected one native app action call, got %+v", actionProvider.calls)
	}
	call := actionProvider.calls[0]
	if call.OrgID != "default" || call.ProviderUID != "nextcloud" || call.Host != "truenas-main" || call.Action != "restart" {
		t.Fatalf("unexpected native app action request: %+v", call)
	}

	audits, err := store.GetActionAudits("", time.Time{}, 10)
	if err != nil {
		t.Fatalf("GetActionAudits() error = %v", err)
	}
	if len(audits) != 1 {
		t.Fatalf("expected one action audit, got %+v", audits)
	}
	if audits[0].Request.CapabilityName != "pulse_control" {
		t.Fatalf("unexpected capability name: %+v", audits[0].Request)
	}
	if got := audits[0].Request.Params["action"]; got != "restart" {
		t.Fatalf("expected audited action restart, got %+v", audits[0].Request.Params)
	}
	if got := audits[0].Request.Params["platform"]; got != "truenas" {
		t.Fatalf("expected audited platform truenas, got %+v", audits[0].Request.Params)
	}
}
