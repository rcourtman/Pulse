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

type fakeProxmoxActionAgentCommander struct {
	calls          []agentexec.ProxmoxGuestLifecyclePayload
	callAgents     []string
	connected      map[string]bool
	agentByHost    map[string]string
	afterStatus    string
	mutateResult   func(*agentexec.ProxmoxGuestLifecycleResultPayload)
	receiptVersion *int
	organizationID string
}

func (f *fakeProxmoxActionAgentCommander) ExecuteCommand(context.Context, string, agentexec.ExecuteCommandPayload) (*agentexec.CommandResultPayload, error) {
	panic("Proxmox actions must not dispatch generic commands")
}
func (f *fakeProxmoxActionAgentCommander) GetAgentForHost(string) (string, bool) {
	panic("Proxmox actions must not discover generic command sessions")
}
func (f *fakeProxmoxActionAgentCommander) IsAgentConnected(string) bool {
	panic("generic connectivity is not typed runner authority")
}
func (f *fakeProxmoxActionAgentCommander) GetActionRunnerForHostForOrganization(org, hostname string) (string, bool) {
	f.organizationID = org
	id := "node-agent-1"
	if f.agentByHost != nil {
		id = f.agentByHost[hostname]
	}
	return id, id != "" && (f.connected == nil || f.connected[id])
}
func (f *fakeProxmoxActionAgentCommander) AgentOperationReceiptVersion(string) int {
	if f.receiptVersion != nil {
		return *f.receiptVersion
	}
	return 1
}
func (f *fakeProxmoxActionAgentCommander) ExecuteProxmoxGuestLifecycle(_ context.Context, agentID string, req agentexec.ProxmoxGuestLifecyclePayload) (*agentexec.ProxmoxGuestLifecycleResultPayload, error) {
	f.calls = append(f.calls, req)
	f.callAgents = append(f.callAgents, agentID)
	status := "running"
	if req.Operation == "stop" || req.Operation == "shutdown" {
		status = "stopped"
	}
	if f.afterStatus != "" {
		status = f.afterStatus
	}
	now := time.Now().UTC()
	result := &agentexec.ProxmoxGuestLifecycleResultPayload{
		RequestID: req.RequestID, ActionID: req.ActionID, Operation: req.Operation,
		OperationVersion: req.OperationVersion, RequestDigest: req.RequestDigest,
		GuestKind: req.GuestKind, VMID: req.VMID, ExecutionPhase: agentexec.ProxmoxGuestPhaseComplete,
		MutationStarted: true, MutationCompleted: true, ReadbackRan: true,
		Before: agentexec.ProxmoxGuestLifecycleSnapshot{Status: req.ExpectedStatus, ObservedAt: now},
		After:  agentexec.ProxmoxGuestLifecycleSnapshot{Status: status, ObservedAt: now},
	}
	if f.mutateResult != nil {
		f.mutateResult(result)
	}
	return result, nil
}

func TestProxmoxGuestPlanningRejectsLegacySessionAndMissingReceipts(t *testing.T) {
	noReceipts := 0
	for _, tc := range []struct {
		name   string
		agents actionAgentCommander
		code   string
	}{
		{"legacy connected", &fakeDockerActionAgentCommander{}, "typed_operation_unavailable"},
		{"no admitted runner", &fakeProxmoxActionAgentCommander{connected: map[string]bool{}}, "action_runner_unavailable"},
		{"receipts unavailable", &fakeProxmoxActionAgentCommander{receiptVersion: &noReceipts}, "operation_receipt_unsupported"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Now().UTC()
			h := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
			h.SetStateProvider(resourceUnifiedSeedProvider{snapshot: models.StateSnapshot{LastUpdate: now}, resources: []unified.Resource{proxmoxGuestActionResource("vm:160", unified.ResourceTypeVM, "running", now)}})
			h.SetActionExecutor(newRoutedActionExecutor(h, newProxmoxGuestActionExecutor(h, tc.agents, nil)))
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/actions/plan", bytes.NewBufferString(`{"requestId":"runner-check","resourceId":"vm:160","capabilityName":"shutdown","reason":"operator requested shutdown","requestedBy":"operator"}`))
			h.HandlePlanAction(rec, actionHandlerTestRequest(req, ""))
			if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), `"reasonCode":"`+tc.code+`"`) {
				t.Fatalf("plan response = %d %s", rec.Code, rec.Body.String())
			}
			store, err := h.getStore("default")
			if err != nil {
				t.Fatal(err)
			}
			audits, err := store.GetActionAudits("vm:160", time.Time{}, 10)
			if err != nil || len(audits) != 0 {
				t.Fatalf("refused plan wrote audit: %#v, %v", audits, err)
			}
		})
	}
}

func TestProxmoxTypedMutationAndVerificationRemainSeparate(t *testing.T) {
	for _, tc := range []struct {
		name, operation string
		mutate          func(*agentexec.ProxmoxGuestLifecycleResultPayload)
	}{
		{"reboot status cannot prove restart", "reboot", nil},
		{"completed mutation with failed readback", "shutdown", func(r *agentexec.ProxmoxGuestLifecycleResultPayload) {
			r.ExecutionPhase = agentexec.ProxmoxGuestPhaseVerify
			r.ReadbackRan = false
			r.After = agentexec.ProxmoxGuestLifecycleSnapshot{}
			r.Error = "status read timed out"
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Now().UTC()
			h := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
			h.SetStateProvider(resourceUnifiedSeedProvider{snapshot: models.StateSnapshot{LastUpdate: now}, resources: []unified.Resource{proxmoxGuestActionResource("vm:160", unified.ResourceTypeVM, "running", now)}})
			agents := &fakeProxmoxActionAgentCommander{mutateResult: tc.mutate}
			result, err := newProxmoxGuestActionExecutor(h, agents, nil).ExecuteAction(actionDispatchTestContext(t, "act_vm"), proxmoxGuestActionRecord("act_vm", "vm:160", tc.operation))
			if err != nil {
				t.Fatal(err)
			}
			if result.ActionResultV2.Execution.Status != unified.ActionExecutionSucceeded || result.ActionResultV2.Verification.Status != unified.ActionVerificationInconclusive {
				t.Fatalf("execution and verification were conflated: %#v", result)
			}
		})
	}
}

func TestProxmoxGuestActionExecutorDispatchesVMShutdownAndVerification(t *testing.T) {
	now := time.Now().UTC()
	h := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
	h.SetStateProvider(resourceUnifiedSeedProvider{
		snapshot: models.StateSnapshot{LastUpdate: now},
		resources: []unified.Resource{
			proxmoxGuestActionResource("vm:160", unified.ResourceTypeVM, "running", now),
		},
	})
	agents := &fakeProxmoxActionAgentCommander{}
	executor := newProxmoxGuestActionExecutor(h, agents, nil)

	result, err := executor.ExecuteAction(actionDispatchTestContext(t, "act_vm"), proxmoxGuestActionRecord("act_vm", "vm:160", "shutdown"))
	if err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}
	if result == nil || !result.Success || result.Verification == nil || !result.Verification.Success {
		t.Fatalf("result = %#v, want successful execution and verification", result)
	}
	if len(agents.calls) != 1 {
		t.Fatalf("typed calls = %d, want one dispatch with runner-owned readback", len(agents.calls))
	}
	call := agents.calls[0]
	if call.GuestKind != "vm" || call.Operation != "shutdown" || call.VMID != 160 || call.ExpectedStatus != "running" || call.ActionID != "act_vm" || call.Timeout != 180 || call.RequestID != "act_vm.dispatch.1" {
		t.Fatalf("typed dispatch = %#v", call)
	}
	if err := agentexec.ValidateProxmoxGuestLifecyclePayload(&call); err != nil {
		t.Fatalf("unbound dispatch: %v", err)
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
	agents := &fakeProxmoxActionAgentCommander{}
	executor := newProxmoxGuestActionExecutor(h, agents, nil)

	result, err := executor.ExecuteAction(actionDispatchTestContext(t, "act_ct"), proxmoxGuestActionRecord("act_ct", "system-container:101", "start"))
	if err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}
	if result == nil || !result.Success || result.Verification == nil || !result.Verification.Success {
		t.Fatalf("result = %#v, want successful execution and verification", result)
	}
	if len(agents.calls) != 1 || agents.calls[0].GuestKind != "ct" || agents.calls[0].VMID != 101 || agents.calls[0].Operation != "start" || agents.calls[0].ExpectedStatus != "stopped" {
		t.Fatalf("typed LXC dispatch = %#v", agents.calls)
	}
}

func TestProxmoxGuestActionExecutorResolvesTypedRunnerByNodeHostname(t *testing.T) {
	now := time.Now().UTC()
	resource := proxmoxGuestActionResource("vm:160", unified.ResourceTypeVM, "running", now)
	resource.Proxmox.LinkedAgentID = "stale-agent"
	h := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
	h.SetStateProvider(resourceUnifiedSeedProvider{
		snapshot:  models.StateSnapshot{LastUpdate: now},
		resources: []unified.Resource{resource},
	})
	agents := &fakeProxmoxActionAgentCommander{
		connected: map[string]bool{
			"stale-agent":     false,
			"command-agent-1": true,
		},
		agentByHost: map[string]string{"delly": "command-agent-1"},
	}
	executor := newProxmoxGuestActionExecutor(h, agents, nil).(proxmoxGuestActionExecutor)

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

func TestProxmoxGuestActionExecutorVerificationContradictionDoesNotRewriteExecution(t *testing.T) {
	now := time.Now().UTC()
	h := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
	h.SetStateProvider(resourceUnifiedSeedProvider{
		snapshot: models.StateSnapshot{LastUpdate: now},
		resources: []unified.Resource{
			proxmoxGuestActionResource("vm:160", unified.ResourceTypeVM, "running", now),
		},
	})
	agents := &fakeProxmoxActionAgentCommander{afterStatus: "running"}
	executor := newProxmoxGuestActionExecutor(h, agents, nil)

	result, err := executor.ExecuteAction(actionDispatchTestContext(t, "act_vm"), proxmoxGuestActionRecord("act_vm", "vm:160", "shutdown"))
	if err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}
	if result == nil || !result.Success || result.Verification == nil || result.Verification.Success {
		t.Fatalf("result = %#v, want succeeded execution with contradicted verification", result)
	}
	if result.ActionResultV2 == nil || result.ActionResultV2.Execution.Status != unified.ActionExecutionSucceeded || result.ActionResultV2.Verification.Status != unified.ActionVerificationContradicted || result.ActionResultV2.Verification.EvidenceClass != unified.ActionEvidenceAgentAttested {
		t.Fatalf("canonical truth = %#v, want independent execution and verification axes", result.ActionResultV2)
	}
	if result.ErrorMessage != "" {
		t.Fatalf("error = %q, want no execution error from verification contradiction", result.ErrorMessage)
	}
}

func TestProxmoxGuestActionExecutorUsesIndependentControlPlaneVerification(t *testing.T) {
	now := time.Now().UTC()
	resource := proxmoxGuestActionResource("vm:160", unified.ResourceTypeVM, "running", now)
	resource.Proxmox.Uptime = 3600
	h := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
	h.SetStateProvider(resourceUnifiedSeedProvider{
		snapshot:  models.StateSnapshot{LastUpdate: now},
		resources: []unified.Resource{resource},
	})
	agents := &fakeProxmoxActionAgentCommander{}
	observer := &fakeProxmoxGuestPostconditionObserver{observations: []proxmoxGuestPostconditionObservation{
		proxmoxGuestActionObservation(now.Add(-time.Second), "running", 3600, "proxmox-control-plane:default:homelab"),
		// The after observation must postdate actionStartedAt, which is stamped
		// inside ExecuteAction after handler setup; a one-second offset loses
		// that race on a loaded runner, so use a generous margin.
		proxmoxGuestActionObservation(now.Add(time.Minute), "stopped", 0, "proxmox-control-plane:default:homelab"),
	}}
	executor := newProxmoxGuestActionExecutor(h, agents, observer)

	result, err := executor.ExecuteAction(actionDispatchTestContext(t, "act_vm"), proxmoxGuestActionRecord("act_vm", "vm:160", "shutdown"))
	if err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}
	if result == nil || result.ActionResultV2 == nil {
		t.Fatalf("result = %#v, want canonical truth", result)
	}
	truth := result.ActionResultV2.Verification
	if truth.Status != unified.ActionVerificationConfirmed || truth.EvidenceClass != unified.ActionEvidenceIndependent || len(truth.Evidence) != 1 {
		t.Fatalf("verification truth = %#v, want independent confirmation", truth)
	}
	if truth.Evidence[0].ObserverKind != "proxmox_control_plane" || truth.Evidence[0].ObserverTrustDomain == truth.Evidence[0].ExecutorTrustDomain {
		t.Fatalf("independent evidence = %#v", truth.Evidence[0])
	}
}

func TestProxmoxGuestActionExecutorRequiresUptimeResetToVerifyReboot(t *testing.T) {
	now := time.Now().UTC()
	resource := proxmoxGuestActionResource("vm:160", unified.ResourceTypeVM, "running", now)
	resource.Proxmox.Uptime = 7200
	h := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
	h.SetStateProvider(resourceUnifiedSeedProvider{snapshot: models.StateSnapshot{LastUpdate: now}, resources: []unified.Resource{resource}})
	agents := &fakeProxmoxActionAgentCommander{}
	observer := &fakeProxmoxGuestPostconditionObserver{observations: []proxmoxGuestPostconditionObservation{
		proxmoxGuestActionObservation(now.Add(-time.Second), "running", 7200, "proxmox-control-plane:default:homelab"),
		proxmoxGuestActionObservation(now.Add(time.Minute), "running", 4, "proxmox-control-plane:default:homelab"),
	}}
	executor := newProxmoxGuestActionExecutor(h, agents, observer)

	result, err := executor.ExecuteAction(actionDispatchTestContext(t, "act_vm"), proxmoxGuestActionRecord("act_vm", "vm:160", "reboot"))
	if err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}
	if result == nil || result.ActionResultV2 == nil || result.ActionResultV2.Verification.Status != unified.ActionVerificationConfirmed || result.ActionResultV2.Verification.EvidenceClass != unified.ActionEvidenceIndependent {
		t.Fatalf("result = %#v, want independently verified reboot", result)
	}
	if len(agents.calls) != 1 {
		t.Fatalf("agent calls = %d, want mutation only because status-only agent read cannot prove reboot", len(agents.calls))
	}
}

func TestProxmoxGuestActionExecutorKeepsIndependentContradictionSeparateFromExecution(t *testing.T) {
	now := time.Now().UTC()
	resource := proxmoxGuestActionResource("vm:160", unified.ResourceTypeVM, "running", now)
	h := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
	h.SetStateProvider(resourceUnifiedSeedProvider{snapshot: models.StateSnapshot{LastUpdate: now}, resources: []unified.Resource{resource}})
	agents := &fakeProxmoxActionAgentCommander{}
	observer := &fakeProxmoxGuestPostconditionObserver{observations: []proxmoxGuestPostconditionObservation{
		proxmoxGuestActionObservation(now.Add(-time.Second), "running", 7200, "proxmox-control-plane:default:homelab"),
		proxmoxGuestActionObservation(now.Add(time.Minute), "running", 7201, "proxmox-control-plane:default:homelab"),
	}}
	executor := newProxmoxGuestActionExecutor(h, agents, observer)
	ctx, cancel := context.WithTimeout(actionDispatchTestContext(t, "act_vm"), 100*time.Millisecond)
	defer cancel()

	result, err := executor.ExecuteAction(ctx, proxmoxGuestActionRecord("act_vm", "vm:160", "reboot"))
	if err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}
	if result == nil || result.ActionResultV2 == nil || !result.Success {
		t.Fatalf("result = %#v, want successful command execution", result)
	}
	truth := result.ActionResultV2.Verification
	if truth.Status != unified.ActionVerificationContradicted || truth.EvidenceClass != unified.ActionEvidenceIndependent {
		t.Fatalf("verification truth = %#v, want independent contradiction", truth)
	}
}

func TestProxmoxGuestActionExecutorRejectsSameDomainIndependentEvidence(t *testing.T) {
	now := time.Now().UTC()
	resource := proxmoxGuestActionResource("vm:160", unified.ResourceTypeVM, "running", now)
	h := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
	h.SetStateProvider(resourceUnifiedSeedProvider{snapshot: models.StateSnapshot{LastUpdate: now}, resources: []unified.Resource{resource}})
	agents := &fakeProxmoxActionAgentCommander{}
	observer := &fakeProxmoxGuestPostconditionObserver{observations: []proxmoxGuestPostconditionObservation{
		proxmoxGuestActionObservation(now.Add(-time.Second), "running", 100, "agent:node-agent-1"),
		proxmoxGuestActionObservation(now.Add(time.Second), "stopped", 0, "agent:node-agent-1"),
	}}
	executor := newProxmoxGuestActionExecutor(h, agents, observer)

	result, err := executor.ExecuteAction(actionDispatchTestContext(t, "act_vm"), proxmoxGuestActionRecord("act_vm", "vm:160", "shutdown"))
	if err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}
	if result == nil || result.ActionResultV2 == nil || result.ActionResultV2.Verification.Status != unified.ActionVerificationConfirmed || result.ActionResultV2.Verification.EvidenceClass != unified.ActionEvidenceAgentAttested {
		t.Fatalf("result = %#v, want same-domain observation rejected in favor of agent-attested truth", result)
	}
}

type fakeProxmoxGuestPostconditionObserver struct {
	observations []proxmoxGuestPostconditionObservation
	next         int
}

func (o *fakeProxmoxGuestPostconditionObserver) ObserveProxmoxGuest(context.Context, string, string, string, int, proxmoxGuestKind) (proxmoxGuestPostconditionObservation, error) {
	if o.next >= len(o.observations) {
		return proxmoxGuestPostconditionObservation{}, context.DeadlineExceeded
	}
	observation := o.observations[o.next]
	o.next++
	return observation, nil
}

func proxmoxGuestActionObservation(observedAt time.Time, status string, uptime uint64, trustDomain string) proxmoxGuestPostconditionObservation {
	return proxmoxGuestPostconditionObservation{
		ObserverID:  "proxmox-api:default:homelab",
		TrustDomain: trustDomain,
		Method:      "proxmox_api_guest_status_current",
		Snapshot: proxmoxGuestLifecycleSnapshot{
			Instance:   "homelab",
			Node:       "delly",
			VMID:       160,
			Kind:       proxmoxGuestVM,
			Status:     status,
			Uptime:     uptime,
			ObservedAt: observedAt,
		},
		ReceivedAt: observedAt,
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
		newProxmoxGuestActionExecutor(h, &fakeProxmoxActionAgentCommander{
			connected: map[string]bool{"node-agent-1": false},
		}, nil),
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
		!strings.Contains(rec.Body.String(), `"reason":"Connect a typed action runner on this Proxmox node before planning a guest action."`) ||
		!strings.Contains(rec.Body.String(), `"reasonCode":"action_runner_unavailable"`) {
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
		newProxmoxGuestActionExecutor(h, &fakeProxmoxActionAgentCommander{
			connected: map[string]bool{"node-agent-1": false},
		}, nil),
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
	if !ok || readiness.Available || readiness.ReasonCode != "action_runner_unavailable" {
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

func TestProxmoxTypedReadbackClockSkewPreservesExecutionAndEvidence(t *testing.T) {
	for _, tc := range []struct {
		name     string
		skew     time.Duration
		verified bool
	}{
		{"ahead within bound", 2 * time.Second, true},
		{"behind within bound", -2 * time.Second, true},
		{"excessively ahead", 6 * time.Minute, false},
		{"stale", -16 * time.Minute, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Now().UTC()
			observedAt := now.Add(tc.skew)
			h := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
			h.SetStateProvider(resourceUnifiedSeedProvider{snapshot: models.StateSnapshot{LastUpdate: now}, resources: []unified.Resource{proxmoxGuestActionResource("vm:160", unified.ResourceTypeVM, "running", now)}})
			agents := &fakeProxmoxActionAgentCommander{mutateResult: func(r *agentexec.ProxmoxGuestLifecycleResultPayload) {
				r.After.ObservedAt = observedAt
			}}
			result, err := newProxmoxGuestActionExecutor(h, agents, nil).ExecuteAction(actionDispatchTestContext(t, "act_clock"), proxmoxGuestActionRecord("act_clock", "vm:160", "shutdown"))
			if err != nil {
				t.Fatalf("clock skew discarded execution result: %v", err)
			}
			truth := result.ActionResultV2
			if truth.Execution.Status != unified.ActionExecutionSucceeded {
				t.Fatalf("readback clock rewrote completed execution: %#v", truth)
			}
			if tc.verified {
				if truth.Verification.Status != unified.ActionVerificationConfirmed || truth.Verification.EvidenceClass != unified.ActionEvidenceAgentAttested || len(truth.Verification.Evidence) != 1 {
					t.Fatalf("bounded skew lost readback: %#v", truth)
				}
				e := truth.Verification.Evidence[0]
				if !e.ObservedAt.Equal(observedAt) || e.ReceivedAt.Before(now) || e.ReceivedAt.After(time.Now().UTC()) {
					t.Fatalf("observation/receipt clocks were rewritten: %#v", e)
				}
			} else if truth.Verification.Status != unified.ActionVerificationInconclusive || truth.Verification.ReasonCode != "stale_agent_readback" || len(truth.Verification.Evidence) != 0 {
				t.Fatalf("unusable clock established verification: %#v", truth)
			}
		})
	}
}
