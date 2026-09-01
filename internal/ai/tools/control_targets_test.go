package tools

// Regression coverage for GitHub issue #1782: an operator asks the Assistant
// to reboot Proxmox VMs, the model resolves them, and pulse_control must plan
// the advertised lifecycle action rather than manufacture a "discovery",
// "session binding", or guest-agent prerequisite. Three defects produced that
// behaviour and each is pinned here:
//
//  1. pulse_control handed the session-scoped ID (vm:<node>:<vmid>) to the
//     action lifecycle, whose registry keys on canonical unified IDs, so a
//     Proxmox guest plan could never resolve.
//  2. pulse_control gated the action on the legacy per-executor action list,
//     which never knew the canonical "reboot" capability Proxmox guests
//     advertise.
//  3. A reference absent from the session context was refused with a
//     "discovery is required" message even when the unified inventory
//     resolved it uniquely.

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/actionplanner"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func controlTestProxmoxVM(name string, vmid int, node string, running bool) unifiedresources.Resource {
	status := unifiedresources.StatusOnline
	operations := []string{"shutdown", "reboot", "stop"}
	if !running {
		status = unifiedresources.StatusOffline
		operations = []string{"start"}
	}
	capabilities := make([]unifiedresources.ResourceCapability, 0, len(operations))
	for _, operation := range operations {
		capabilities = append(capabilities, unifiedresources.ResourceCapability{
			Name:                 operation,
			Type:                 unifiedresources.CapabilityTypeCommon,
			Description:          "Proxmox VM lifecycle " + operation,
			MinimumApprovalLevel: unifiedresources.ApprovalAdmin,
			Platform:             "qemu",
			InternalHandler:      "proxmox.vm.lifecycle",
		})
	}
	return unifiedresources.Resource{
		ID:           fmt.Sprintf("vm-%s-%s", node, name),
		Type:         unifiedresources.ResourceTypeVM,
		Name:         name,
		Status:       status,
		ParentName:   node,
		Capabilities: capabilities,
		Proxmox: &unifiedresources.ProxmoxData{
			SourceID: fmt.Sprintf("%s:%s:%d", node, node, vmid),
			NodeName: node,
			Instance: node,
			VMID:     vmid,
		},
	}
}

// listingResolvedContext adds the optional enumeration surface the
// advertised-action gate consults.
type listingResolvedContext struct {
	*mockResolvedContext
}

func (l *listingResolvedContext) ListResolvedResources() []ResolvedResourceInfo {
	out := make([]ResolvedResourceInfo, 0, len(l.resources))
	for _, res := range l.resources {
		out = append(out, res)
	}
	return out
}

func newControlTestResolvedContext() *listingResolvedContext {
	return &listingResolvedContext{mockResolvedContext: &mockResolvedContext{
		resources:    make(map[string]ResolvedResourceInfo),
		aliases:      make(map[string]ResolvedResourceInfo),
		lastAccessed: make(map[string]time.Time),
	}}
}

type recordedPlan struct {
	requests []unifiedresources.ActionRequest
}

func (r *recordedPlan) planner(err error) typedActionPlannerFunc {
	return func(_ context.Context, _ string, req unifiedresources.ActionRequest) (*unifiedresources.ActionPlan, error) {
		r.requests = append(r.requests, req)
		if err != nil {
			return nil, err
		}
		return &unifiedresources.ActionPlan{
			ActionID:         "action-" + req.RequestID,
			RequestID:        req.RequestID,
			Allowed:          true,
			RequiresApproval: true,
			ApprovalPolicy:   unifiedresources.ApprovalAdmin,
			PlanHash:         "hash-1",
		}, nil
	}
}

func decodeControlPayload(t *testing.T, result CallToolResult) map[string]any {
	t.Helper()
	if len(result.Content) == 0 {
		t.Fatalf("expected result content, got %+v", result)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(result.Content[0].Text), &payload); err != nil {
		t.Fatalf("decode result payload %q: %v", result.Content[0].Text, err)
	}
	return payload
}

func decodeControlToolResponse(t *testing.T, result CallToolResult) ToolResponse {
	t.Helper()
	if len(result.Content) == 0 {
		t.Fatalf("expected result content, got %+v", result)
	}
	var response ToolResponse
	if err := json.Unmarshal([]byte(result.Content[0].Text), &response); err != nil {
		t.Fatalf("decode tool response %q: %v", result.Content[0].Text, err)
	}
	return response
}

func TestExecuteControlResource_PlansAdvertisedRebootAgainstCanonicalID(t *testing.T) {
	vm := controlTestProxmoxVM("win-01", 101, "pve", true)
	provider := &stubUnifiedResourceProvider{resources: []unifiedresources.Resource{vm}}
	resolved := newControlTestResolvedContext()
	reg, ok := canonicalGuestRegistration("vm", vm)
	if !ok {
		t.Fatal("expected canonical guest registration")
	}
	resolved.AddResolvedResource(reg)

	plans := &recordedPlan{}
	executor := NewPulseToolExecutor(ExecutorConfig{
		UnifiedResourceProvider: provider,
		ControlLevel:            ControlLevelControlled,
		TypedActionPlanner:      plans.planner(nil),
	})
	executor.SetResolvedContext(resolved)

	result, err := executor.executeControl(context.Background(), map[string]interface{}{
		"type":        "resource",
		"resource_id": "win-01",
		"action":      "reboot",
	})
	if err != nil {
		t.Fatalf("executeControl: unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected planned result, got error %+v", result)
	}
	if len(plans.requests) != 1 {
		t.Fatalf("expected one plan request, got %d", len(plans.requests))
	}
	if plans.requests[0].ResourceID != vm.ID {
		t.Fatalf("plan ResourceID = %q, want canonical %q (session-scoped ids never resolve in the lifecycle registry)", plans.requests[0].ResourceID, vm.ID)
	}
	if plans.requests[0].CapabilityName != "reboot" {
		t.Fatalf("plan CapabilityName = %q, want reboot", plans.requests[0].CapabilityName)
	}
	payload := decodeControlPayload(t, result)
	if payload["planned"] != true || payload["requires_approval"] != true {
		t.Fatalf("expected planned action awaiting approval, got %+v", payload)
	}
	if payload["resource_id"] != vm.ID || payload["capability"] != "reboot" || payload["resource_name"] != "win-01" {
		t.Fatalf("unexpected plan payload %+v", payload)
	}
	if message, _ := payload["message"].(string); strings.Contains(strings.ToLower(message), "discover") {
		t.Fatalf("plan message must not mention discovery: %q", message)
	}
}

func TestExecuteControlResource_RestartMapsToAdvertisedRebootInPayload(t *testing.T) {
	vm := controlTestProxmoxVM("win-02", 102, "pve", true)
	provider := &stubUnifiedResourceProvider{resources: []unifiedresources.Resource{vm}}
	resolved := newControlTestResolvedContext()
	reg, _ := canonicalGuestRegistration("vm", vm)
	resolved.AddResolvedResource(reg)

	plans := &recordedPlan{}
	executor := NewPulseToolExecutor(ExecutorConfig{
		UnifiedResourceProvider: provider,
		ControlLevel:            ControlLevelControlled,
		TypedActionPlanner:      plans.planner(nil),
	})
	executor.SetResolvedContext(resolved)

	result, err := executor.executeControl(context.Background(), map[string]interface{}{
		"type":        "resource",
		"resource_id": vm.ID,
		"action":      "restart",
	})
	if err != nil || result.IsError {
		t.Fatalf("expected planned result, got err=%v result=%+v", err, result)
	}
	payload := decodeControlPayload(t, result)
	if payload["requested_action"] != "restart" || payload["capability"] != "reboot" {
		t.Fatalf("expected restart to surface the advertised reboot capability, got %+v", payload)
	}
}

func TestExecuteControlResource_ResolvesCanonicalTargetWithoutSessionDiscovery(t *testing.T) {
	vm := controlTestProxmoxVM("win-03", 103, "pve", true)
	provider := &stubUnifiedResourceProvider{resources: []unifiedresources.Resource{vm}}
	resolved := newControlTestResolvedContext()

	plans := &recordedPlan{}
	executor := NewPulseToolExecutor(ExecutorConfig{
		UnifiedResourceProvider: provider,
		ControlLevel:            ControlLevelControlled,
		TypedActionPlanner:      plans.planner(nil),
	})
	executor.SetResolvedContext(resolved)

	result, err := executor.executeControl(context.Background(), map[string]interface{}{
		"type":        "resource",
		"resource_id": "win-03",
		"action":      "reboot",
	})
	if err != nil {
		t.Fatalf("executeControl: unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("a reference that resolves uniquely in the unified inventory must plan, got %+v", result)
	}
	if len(plans.requests) != 1 || plans.requests[0].ResourceID != vm.ID {
		t.Fatalf("expected one plan against %q, got %+v", vm.ID, plans.requests)
	}
	if _, ok := resolved.GetResolvedResourceByAlias("win-03"); !ok {
		t.Fatal("expected the canonical target to be registered in session context after planning")
	}
}

func TestExecuteControlResource_UnknownReferenceNamesTheRecoveryCall(t *testing.T) {
	provider := &stubUnifiedResourceProvider{resources: []unifiedresources.Resource{controlTestProxmoxVM("win-04", 104, "pve", true)}}
	plans := &recordedPlan{}
	executor := NewPulseToolExecutor(ExecutorConfig{
		UnifiedResourceProvider: provider,
		ControlLevel:            ControlLevelControlled,
		TypedActionPlanner:      plans.planner(nil),
	})
	executor.SetResolvedContext(newControlTestResolvedContext())

	result, err := executor.executeControl(context.Background(), map[string]interface{}{
		"type":        "resource",
		"resource_id": "no-such-guest",
		"action":      "reboot",
	})
	if err != nil {
		t.Fatalf("executeControl: unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected a blocked result for an unknown reference, got %+v", result)
	}
	if len(plans.requests) != 0 {
		t.Fatalf("an unresolved reference must not reach the planner, got %+v", plans.requests)
	}
	response := decodeControlToolResponse(t, result)
	if response.Error == nil || response.Error.Code != agentcapabilities.ErrCodeNotFound {
		t.Fatalf("expected %s, got %+v", agentcapabilities.ErrCodeNotFound, response.Error)
	}
	if !strings.Contains(response.Error.Message, "pulse_query action=search") {
		t.Fatalf("lookup miss must name the exact recovery call, got %q", response.Error.Message)
	}
	lower := strings.ToLower(response.Error.Message + fmt.Sprint(response.Error.Details["policy_boundary"]))
	if strings.Contains(lower, "discovery is required") || strings.Contains(lower, "has not been discovered") {
		t.Fatalf("lookup miss must not read as a discovery prerequisite: %q", lower)
	}
}

func TestExecuteControlResource_RefusesAmbiguousNameWithCandidates(t *testing.T) {
	provider := &stubUnifiedResourceProvider{resources: []unifiedresources.Resource{
		controlTestProxmoxVM("win-05", 105, "pve-a", true),
		controlTestProxmoxVM("win-05", 205, "pve-b", true),
	}}
	plans := &recordedPlan{}
	executor := NewPulseToolExecutor(ExecutorConfig{
		UnifiedResourceProvider: provider,
		ControlLevel:            ControlLevelControlled,
		TypedActionPlanner:      plans.planner(nil),
	})
	executor.SetResolvedContext(newControlTestResolvedContext())

	result, err := executor.executeControl(context.Background(), map[string]interface{}{
		"type":        "resource",
		"resource_id": "win-05",
		"action":      "reboot",
	})
	if err != nil {
		t.Fatalf("executeControl: unexpected error: %v", err)
	}
	if !result.IsError || len(plans.requests) != 0 {
		t.Fatalf("an ambiguous name must not plan, got result=%+v plans=%+v", result, plans.requests)
	}
	response := decodeControlToolResponse(t, result)
	if response.Error == nil || response.Error.Code != agentcapabilities.ErrCodeInvalidInput {
		t.Fatalf("expected %s, got %+v", agentcapabilities.ErrCodeInvalidInput, response.Error)
	}
	candidates, _ := response.Error.Details["candidates"].([]any)
	if len(candidates) != 2 {
		t.Fatalf("expected both canonical candidates to be listed, got %+v", response.Error.Details)
	}
}

func TestExecuteControlResource_UnadvertisedCapabilityIsToolEvidence(t *testing.T) {
	vm := controlTestProxmoxVM("win-06", 106, "pve", false) // stopped: advertises start only
	provider := &stubUnifiedResourceProvider{resources: []unifiedresources.Resource{vm}}
	resolved := newControlTestResolvedContext()
	reg, _ := canonicalGuestRegistration("vm", vm)
	resolved.AddResolvedResource(reg)

	plans := &recordedPlan{}
	executor := NewPulseToolExecutor(ExecutorConfig{
		UnifiedResourceProvider: provider,
		ControlLevel:            ControlLevelControlled,
		TypedActionPlanner:      plans.planner(fmt.Errorf("plan: %w", actionplanner.ErrCapabilityNotFound)),
	})
	executor.SetResolvedContext(resolved)

	result, err := executor.executeControl(context.Background(), map[string]interface{}{
		"type":        "resource",
		"resource_id": "win-06",
		"action":      "reboot",
	})
	if err != nil {
		t.Fatalf("executeControl: unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected a boundary result, got %+v", result)
	}
	response := decodeControlToolResponse(t, result)
	if response.Error == nil || response.Error.Code != agentcapabilities.ErrCodeActionNotAllowed {
		t.Fatalf("expected %s, got %+v", agentcapabilities.ErrCodeActionNotAllowed, response.Error)
	}
	if !strings.Contains(response.Error.Message, "start") || !strings.Contains(response.Error.Message, "reboot") {
		t.Fatalf("boundary must name the requested and advertised capabilities, got %q", response.Error.Message)
	}
	advertised, _ := response.Error.Details["advertised_capabilities"].([]any)
	if len(advertised) != 1 || advertised[0] != "start" {
		t.Fatalf("expected advertised_capabilities [start], got %+v", response.Error.Details)
	}
}

func TestSessionTargetsAdvertisingAction_UsesCanonicalCapabilities(t *testing.T) {
	running := controlTestProxmoxVM("win-07", 107, "pve", true)
	stopped := controlTestProxmoxVM("win-08", 108, "pve", false)
	provider := &stubUnifiedResourceProvider{resources: []unifiedresources.Resource{running, stopped}}
	resolved := newControlTestResolvedContext()
	for _, vm := range []unifiedresources.Resource{running, stopped} {
		reg, ok := canonicalGuestRegistration("vm", vm)
		if !ok {
			t.Fatalf("expected registration for %s", vm.Name)
		}
		resolved.AddResolvedResource(reg)
	}
	executor := NewPulseToolExecutor(ExecutorConfig{
		UnifiedResourceProvider: provider,
		ControlLevel:            ControlLevelControlled,
	})
	executor.SetResolvedContext(resolved)

	reboot := executor.SessionTargetsAdvertisingAction("reboot")
	if len(reboot) != 1 || reboot[0].CanonicalID != running.ID || reboot[0].Capability != "reboot" || reboot[0].Kind != "vm" {
		t.Fatalf("expected only the running VM to advertise reboot, got %+v", reboot)
	}
	// The legacy executor action list never carried "reboot"; the canonical
	// capability list is the only source of truth.
	if targets := executor.SessionTargetsAdvertisingAction("restart"); len(targets) != 1 || targets[0].Capability != "reboot" {
		t.Fatalf("restart must resolve to the advertised reboot synonym, got %+v", targets)
	}
	start := executor.SessionTargetsAdvertisingAction("start")
	if len(start) != 1 || start[0].CanonicalID != stopped.ID {
		t.Fatalf("expected only the stopped VM to advertise start, got %+v", start)
	}
	if targets := executor.SessionTargetsAdvertisingAction("delete"); len(targets) != 0 {
		t.Fatalf("expected no targets for an unadvertised action, got %+v", targets)
	}
}

func TestSessionTargetsAdvertisingAction_IgnoresContextsWithoutEnumeration(t *testing.T) {
	vm := controlTestProxmoxVM("win-09", 109, "pve", true)
	provider := &stubUnifiedResourceProvider{resources: []unifiedresources.Resource{vm}}
	resolved := &mockResolvedContext{
		resources: make(map[string]ResolvedResourceInfo),
		aliases:   make(map[string]ResolvedResourceInfo),
	}
	reg, _ := canonicalGuestRegistration("vm", vm)
	resolved.AddResolvedResource(reg)
	executor := NewPulseToolExecutor(ExecutorConfig{UnifiedResourceProvider: provider, ControlLevel: ControlLevelControlled})
	executor.SetResolvedContext(resolved)
	if targets := executor.SessionTargetsAdvertisingAction("reboot"); len(targets) != 0 {
		t.Fatalf("a context that cannot enumerate must yield no gate evidence, got %+v", targets)
	}
}
