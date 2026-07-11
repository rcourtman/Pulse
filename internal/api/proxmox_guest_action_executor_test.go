package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestProxmoxGuestActionExecutorDispatchesVMShutdownAndVerification(t *testing.T) {
	now := time.Now().UTC()
	h := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
	h.SetStateProvider(resourceUnifiedSeedProvider{
		snapshot: models.StateSnapshot{LastUpdate: now},
		resources: []unified.Resource{
			proxmoxGuestActionResource("vm:160", unified.ResourceTypeVM, "running", now),
		},
	})
	agents := &fakeDockerActionAgentCommander{results: []*agentexec.CommandResultPayload{
		{RequestID: "act_vm", Success: true, ExitCode: 0, Stdout: "shutdown requested"},
		{RequestID: "act_vm-verify-1", Success: true, ExitCode: 0, Stdout: "status: stopped"},
	}}
	executor := newProxmoxGuestActionExecutor(h, agents)

	result, err := executor.ExecuteAction(actionDispatchTestContext(t, "act_vm"), proxmoxGuestActionRecord("act_vm", "vm:160", "shutdown"))
	if err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}
	if result == nil || !result.Success || result.Verification == nil || !result.Verification.Success {
		t.Fatalf("result = %#v, want successful execution and verification", result)
	}
	if len(agents.calls) != 2 {
		t.Fatalf("agent calls = %d, want dispatch and verification", len(agents.calls))
	}
	if got := agents.calls[0].Command; got != "qm shutdown 160" {
		t.Fatalf("dispatch command = %q", got)
	}
	if agents.calls[0].ApprovalID != "act_vm" || !agents.calls[0].Trusted || agents.calls[0].Timeout != 180 {
		t.Fatalf("dispatch approval/trust/timeout = %q/%v/%d", agents.calls[0].ApprovalID, agents.calls[0].Trusted, agents.calls[0].Timeout)
	}
	if agents.calls[0].RequestID != "act_vm.dispatch.1" {
		t.Fatalf("dispatch request identity = %q", agents.calls[0].RequestID)
	}
	if got := agents.calls[1].Command; got != "qm status 160" {
		t.Fatalf("verification command = %q", got)
	}
	for _, agentID := range agents.callAgents {
		if agentID != "node-agent-1" {
			t.Fatalf("called agent %q, want node-agent-1; all calls %#v", agentID, agents.callAgents)
		}
	}
}

func TestProxmoxGuestActionExecutorDispatchesLXCStartAndVerification(t *testing.T) {
	now := time.Now().UTC()
	h := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
	h.SetStateProvider(resourceUnifiedSeedProvider{
		snapshot: models.StateSnapshot{LastUpdate: now},
		resources: []unified.Resource{
			proxmoxGuestActionResource("system-container:101", unified.ResourceTypeSystemContainer, "stopped", now),
		},
	})
	agents := &fakeDockerActionAgentCommander{results: []*agentexec.CommandResultPayload{
		{RequestID: "act_ct", Success: true, ExitCode: 0, Stdout: "start requested"},
		{RequestID: "act_ct-verify-1", Success: true, ExitCode: 0, Stdout: "status: running"},
	}}
	executor := newProxmoxGuestActionExecutor(h, agents)

	result, err := executor.ExecuteAction(actionDispatchTestContext(t, "act_ct"), proxmoxGuestActionRecord("act_ct", "system-container:101", "start"))
	if err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}
	if result == nil || !result.Success || result.Verification == nil || !result.Verification.Success {
		t.Fatalf("result = %#v, want successful execution and verification", result)
	}
	if got := agents.calls[0].Command; got != "pct start 101" {
		t.Fatalf("dispatch command = %q", got)
	}
	if got := agents.calls[1].Command; got != "pct status 101" {
		t.Fatalf("verification command = %q", got)
	}
}

func TestProxmoxGuestActionExecutorResolvesCommandAgentByNodeHostname(t *testing.T) {
	now := time.Now().UTC()
	resource := proxmoxGuestActionResource("vm:160", unified.ResourceTypeVM, "running", now)
	resource.Proxmox.LinkedAgentID = "stale-agent"
	h := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
	h.SetStateProvider(resourceUnifiedSeedProvider{
		snapshot:  models.StateSnapshot{LastUpdate: now},
		resources: []unified.Resource{resource},
	})
	agents := &fakeDockerActionAgentCommander{
		results: []*agentexec.CommandResultPayload{
			{RequestID: "act_vm", Success: true, ExitCode: 0, Stdout: "reboot requested"},
			{RequestID: "act_vm-verify-1", Success: true, ExitCode: 0, Stdout: "status: running"},
		},
		connected: map[string]bool{
			"stale-agent":     false,
			"command-agent-1": true,
		},
		agentByHost: map[string]string{"delly": "command-agent-1"},
	}
	executor := newProxmoxGuestActionExecutor(h, agents).(proxmoxGuestActionExecutor)

	readiness := executor.CheckActionAvailable(context.Background(), unified.ActionRequest{
		RequestID:      "req-availability",
		ResourceID:     "vm:160",
		CapabilityName: "reboot",
		Reason:         "operator requested reboot",
		RequestedBy:    "operator",
	}, resource)
	if !readiness.Available {
		t.Fatalf("CheckActionAvailable readiness = %#v, want available through node hostname fallback", readiness)
	}

	result, err := executor.ExecuteAction(actionDispatchTestContext(t, "act_vm"), proxmoxGuestActionRecord("act_vm", "vm:160", "reboot"))
	if err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}
	if result == nil || !result.Success {
		t.Fatalf("result = %#v, want successful execution", result)
	}
	for _, agentID := range agents.callAgents {
		if agentID != "command-agent-1" {
			t.Fatalf("called agent %q, want command-agent-1; all calls %#v", agentID, agents.callAgents)
		}
	}
}

func TestProxmoxGuestActionExecutorVerificationFailureFailsAction(t *testing.T) {
	now := time.Now().UTC()
	h := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
	h.SetStateProvider(resourceUnifiedSeedProvider{
		snapshot: models.StateSnapshot{LastUpdate: now},
		resources: []unified.Resource{
			proxmoxGuestActionResource("vm:160", unified.ResourceTypeVM, "running", now),
		},
	})
	agents := &fakeDockerActionAgentCommander{results: []*agentexec.CommandResultPayload{
		{RequestID: "act_vm", Success: true, ExitCode: 0, Stdout: "shutdown requested"},
		{RequestID: "act_vm-verify-1", Success: true, ExitCode: 0, Stdout: "status: running"},
		{RequestID: "act_vm-verify-2", Success: true, ExitCode: 0, Stdout: "status: running"},
		{RequestID: "act_vm-verify-3", Success: true, ExitCode: 0, Stdout: "status: running"},
		{RequestID: "act_vm-verify-4", Success: true, ExitCode: 0, Stdout: "status: running"},
		{RequestID: "act_vm-verify-5", Success: true, ExitCode: 0, Stdout: "status: running"},
	}}
	executor := newProxmoxGuestActionExecutor(h, agents)

	result, err := executor.ExecuteAction(actionDispatchTestContext(t, "act_vm"), proxmoxGuestActionRecord("act_vm", "vm:160", "shutdown"))
	if err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}
	if result == nil || result.Success || result.Verification == nil || result.Verification.Success {
		t.Fatalf("result = %#v, want failed verification to fail action", result)
	}
	if !strings.Contains(result.ErrorMessage, "verification did not confirm") {
		t.Fatalf("error = %q, want verification failure", result.ErrorMessage)
	}
}

func TestHandlePlanActionRejectsDisconnectedProxmoxNodeCommandAgent(t *testing.T) {
	now := time.Now().UTC()
	h := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
	h.SetStateProvider(resourceUnifiedSeedProvider{
		snapshot: models.StateSnapshot{LastUpdate: now},
		resources: []unified.Resource{
			proxmoxGuestActionResource("vm:160", unified.ResourceTypeVM, "running", now),
		},
	})
	h.SetActionExecutor(newRoutedActionExecutor(
		h,
		newProxmoxGuestActionExecutor(h, &fakeDockerActionAgentCommander{
			connected: map[string]bool{"node-agent-1": false},
		}),
	))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/actions/plan", bytes.NewBufferString(`{
		"requestId":"req-disconnected-proxmox-agent",
		"resourceId":"vm:160",
		"capabilityName":"reboot",
		"reason":"operator requested reboot",
		"requestedBy":"operator"
	}`))
	h.HandlePlanAction(rec, actionHandlerTestRequest(req, ""))

	if rec.Code != http.StatusConflict {
		t.Fatalf("plan status = %d, want %d, body=%s", rec.Code, http.StatusConflict, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"error":"action_execution_unavailable"`) ||
		!strings.Contains(rec.Body.String(), `"reason":"Proxmox node command agent is not connected."`) ||
		!strings.Contains(rec.Body.String(), `"reasonCode":"command_agent_disconnected"`) {
		t.Fatalf("unexpected response body: %s", rec.Body.String())
	}
	store, err := h.getStore("default")
	if err != nil {
		t.Fatalf("get store: %v", err)
	}
	audits, err := store.GetActionAudits("vm:160", time.Time{}, 10)
	if err != nil {
		t.Fatalf("GetActionAudits: %v", err)
	}
	if len(audits) != 0 {
		t.Fatalf("audits = %#v, want none for refused plan", audits)
	}
}

func TestResourceResponsesFilterDisconnectedProxmoxLifecycleCapabilities(t *testing.T) {
	now := time.Now().UTC()
	h := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
	h.SetStateProvider(resourceUnifiedSeedProvider{
		snapshot: models.StateSnapshot{LastUpdate: now},
		resources: []unified.Resource{
			proxmoxGuestActionResource("vm:160", unified.ResourceTypeVM, "running", now),
		},
	})
	h.SetActionExecutor(newRoutedActionExecutor(
		h,
		newProxmoxGuestActionExecutor(h, &fakeDockerActionAgentCommander{
			connected: map[string]bool{"node-agent-1": false},
		}),
	))

	listRec := httptest.NewRecorder()
	listReq := httptest.NewRequest(http.MethodGet, "/api/resources?type=vm", nil)
	h.HandleListResources(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, body=%s", listRec.Code, listRec.Body.String())
	}
	var list ResourcesResponse
	if err := json.Unmarshal(listRec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list.Data) != 1 {
		t.Fatalf("list data len = %d, want 1", len(list.Data))
	}
	if got := dockerActionCapabilityNames(list.Data[0].Capabilities); len(got) != 0 {
		t.Fatalf("list capabilities = %#v, want none", got)
	}
	readiness, ok := dockerActionReadinessByName(list.Data[0].ActionReadiness, "reboot")
	if !ok || readiness.Available || readiness.ReasonCode != "command_agent_disconnected" {
		t.Fatalf("list action readiness = %#v, ok=%v; want disconnected reboot", list.Data[0].ActionReadiness, ok)
	}
}

func proxmoxGuestActionResource(id string, typ unified.ResourceType, state string, now time.Time) unified.Resource {
	handler := proxmoxVMLifecycleHandler
	technology := "qemu"
	vmid := 160
	capabilities := []unified.ResourceCapability{
		{
			Name:                 "shutdown",
			Type:                 unified.CapabilityTypeCommon,
			Description:          "Gracefully shut down this Proxmox VM",
			MinimumApprovalLevel: unified.ApprovalAdmin,
			Platform:             "qemu",
			InternalHandler:      proxmoxVMLifecycleHandler,
		},
		{
			Name:                 "reboot",
			Type:                 unified.CapabilityTypeCommon,
			Description:          "Reboot this Proxmox VM",
			MinimumApprovalLevel: unified.ApprovalAdmin,
			Platform:             "qemu",
			InternalHandler:      proxmoxVMLifecycleHandler,
		},
		{
			Name:                 "stop",
			Type:                 unified.CapabilityTypeCommon,
			Description:          "Hard stop this Proxmox VM",
			MinimumApprovalLevel: unified.ApprovalAdmin,
			Platform:             "qemu",
			InternalHandler:      proxmoxVMLifecycleHandler,
		},
	}
	if typ == unified.ResourceTypeSystemContainer {
		handler = proxmoxCTLifecycleHandler
		technology = "lxc"
		vmid = 101
		capabilities = []unified.ResourceCapability{
			{
				Name:                 "start",
				Type:                 unified.CapabilityTypeCommon,
				Description:          "Start this Proxmox LXC",
				MinimumApprovalLevel: unified.ApprovalAdmin,
				Platform:             "lxc",
				InternalHandler:      handler,
			},
		}
	}
	return unified.Resource{
		ID:         id,
		Type:       typ,
		Technology: technology,
		Name:       "guest",
		Status:     proxmoxGuestResourceStatus(state),
		LastSeen:   now,
		UpdatedAt:  now,
		Sources:    []unified.DataSource{unified.SourceProxmox},
		SourceStatus: map[unified.DataSource]unified.SourceStatus{
			unified.SourceProxmox: {Status: "online", LastSeen: now},
		},
		Proxmox: &unified.ProxmoxData{
			SourceID:      "homelab:delly:" + strings.TrimPrefix(id, "vm:"),
			NodeName:      "delly",
			Instance:      "homelab",
			ClusterName:   "homelab",
			VMID:          vmid,
			LinkedAgentID: "node-agent-1",
		},
		Capabilities: capabilities,
	}
}

func proxmoxGuestResourceStatus(state string) unified.ResourceStatus {
	switch state {
	case "running":
		return unified.StatusOnline
	case "stopped":
		return unified.StatusOffline
	default:
		return unified.StatusUnknown
	}
}

func proxmoxGuestActionRecord(actionID, resourceID, operation string) unified.ActionAuditRecord {
	return unified.ActionAuditRecord{
		ID:        actionID,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
		State:     unified.ActionStateExecuting,
		Request: unified.ActionRequest{
			RequestID:      "req-" + actionID,
			ResourceID:     resourceID,
			CapabilityName: operation,
			Reason:         "test execution",
			RequestedBy:    "agent:oncall-helper",
			Params:         map[string]any{},
		},
		Plan: unified.ActionPlan{
			ActionID:         actionID,
			RequestID:        "req-" + actionID,
			Allowed:          true,
			RequiresApproval: true,
			ApprovalPolicy:   unified.ApprovalAdmin,
			PlannedAt:        time.Now().UTC().Add(-time.Minute),
			ExpiresAt:        time.Now().UTC().Add(time.Minute),
			ResourceVersion:  "resource:sha256:test",
			PolicyVersion:    "policy:sha256:test",
			PlanHash:         "sha256:test",
		},
	}
}
