package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/relay"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

func TestProxmoxStoppedDetectorProposalApprovalDispatchIndependentVerificationAndFindingReconciliation(t *testing.T) {
	now := time.Now().UTC()
	running := proxmoxPatrolVMResource(unified.StatusOnline, now)
	stopped := proxmoxPatrolVMResource(unified.StatusOffline, now.Add(10*time.Second))
	findings := ai.DetectProxmoxGuestLifecycleFindings(
		proxmoxPatrolReadState(running),
		proxmoxPatrolReadState(stopped),
		now.Add(10*time.Second),
	)
	if len(findings) != 1 {
		t.Fatalf("detected findings=%#v", findings)
	}
	finding := findings[0]
	if finding.ID != "proxmox:guest:stopped:vm:160" || finding.Key != "proxmox-guest-stopped" {
		t.Fatalf("finding=%#v", finding)
	}

	resources := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
	resources.SetStateProvider(resourceUnifiedSeedProvider{
		snapshot:  models.StateSnapshot{LastUpdate: stopped.LastSeen},
		resources: []unified.Resource{stopped},
	})
	agents := &fakeDockerActionAgentCommander{results: []*agentexec.CommandResultPayload{
		{RequestID: "dispatch", Success: true, ExitCode: 0, Stdout: "start requested"},
		{RequestID: "verify", Success: true, ExitCode: 0, Stdout: "status: running"},
	}}
	observer := &fakeProxmoxGuestPostconditionObserver{observations: []proxmoxGuestPostconditionObservation{
		proxmoxGuestActionObservation(now.Add(-time.Second), "stopped", 0, "proxmox-control-plane:default:homelab"),
		proxmoxGuestActionObservation(now.Add(time.Minute), "running", 5, "proxmox-control-plane:default:homelab"),
	}}
	resources.SetActionExecutor(newRoutedActionExecutor(resources, newProxmoxGuestActionExecutor(resources, agents, observer)))

	aiHandler, patrol, _, _ := setupAIHandlerWithPatrol(t)
	notifications := make(chan relay.PushNotificationPayload, 1)
	patrol.SetPushNotifyCallback(func(payload relay.PushNotificationPayload) { notifications <- payload })
	if !patrol.GetFindings().Add(finding) {
		t.Fatal("deterministic Proxmox finding was not admitted")
	}
	investigations := newTestInvestigationStore()
	investigation := investigations.Create(finding.ID, "proxmox-lifecycle-session")
	aiHandler.investigationStores = map[string]aicontracts.InvestigationStore{"default": investigations}
	aiHandler.SetResourceStoreProvider(resources.getStore)
	resources.SetActionTransitionPublisher(aiHandler.ReconcilePatrolActionTransition)

	proposal := aicontracts.ActionProposal{
		ProposalID: "proposal-" + finding.ID, FindingID: finding.ID, InvestigationID: investigation.ID,
		ResourceID: finding.ResourceID, CapabilityName: "start", Params: map[string]any{},
		Reason:      "Restore the stopped Proxmox guest through its exact governed lifecycle capability.",
		EvidenceIDs: []string{"finding-evidence:" + finding.ID},
	}
	disposition, err := NewPatrolActionBroker("default", resources).Submit(context.Background(), proposal)
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if disposition.State != string(unified.ActionStatePending) || !disposition.Plan.RequiresApproval {
		t.Fatalf("planned disposition=%#v", disposition)
	}

	decision := httptest.NewRecorder()
	decisionReq := httptest.NewRequest(http.MethodPost, "/api/actions/"+disposition.ActionID+"/decision", bytes.NewBufferString(`{"outcome":"approved","reason":"restore service"}`))
	decisionReq.SetPathValue("id", disposition.ActionID)
	decisionReq = decisionReq.WithContext(auth.WithUser(decisionReq.Context(), "operator@example.com"))
	resources.HandleDecideAction(decision, actionHandlerTestRequest(decisionReq, ""))
	if decision.Code != http.StatusOK {
		t.Fatalf("decision status=%d body=%s", decision.Code, decision.Body.String())
	}

	execution := httptest.NewRecorder()
	executionReq := httptest.NewRequest(http.MethodPost, "/api/actions/"+disposition.ActionID+"/execute", bytes.NewBufferString(`{"reason":"execute approved Proxmox start"}`))
	executionReq.SetPathValue("id", disposition.ActionID)
	executionReq = executionReq.WithContext(auth.WithUser(executionReq.Context(), "operator@example.com"))
	resources.HandleExecuteAction(execution, actionHandlerTestRequest(executionReq, ""))
	if execution.Code != http.StatusOK {
		t.Fatalf("execution status=%d body=%s", execution.Code, execution.Body.String())
	}

	store, err := resources.getStore("default")
	if err != nil {
		t.Fatal(err)
	}
	audit, found, err := store.GetActionAudit(disposition.ActionID)
	if err != nil || !found || audit.State != unified.ActionStateCompleted || audit.Origin == nil || audit.Origin.FindingID != finding.ID || len(audit.Request.Params) != 0 {
		t.Fatalf("terminal audit found=%v err=%v audit=%#v", found, err, audit)
	}
	truth := unified.CanonicalActionResultV2(audit)
	if truth.Execution.Status != unified.ActionExecutionSucceeded || truth.Verification.Status != unified.ActionVerificationConfirmed || truth.Verification.EvidenceClass != unified.ActionEvidenceIndependent || len(truth.Verification.Evidence) != 1 {
		t.Fatalf("terminal action truth=%#v", truth)
	}
	if truth.Verification.Evidence[0].ObserverKind != "proxmox_control_plane" || truth.Verification.Evidence[0].ObserverTrustDomain == truth.Verification.Evidence[0].ExecutorTrustDomain {
		t.Fatalf("independent evidence=%#v", truth.Verification.Evidence[0])
	}
	if len(agents.calls) != 2 || agents.calls[0].Command != "qm start 160" || agents.calls[1].Command != "qm status 160" {
		t.Fatalf("typed Proxmox calls=%#v", agents.calls)
	}

	reconciled := patrol.GetFindings().Get(finding.ID)
	if reconciled == nil || reconciled.ResolvedAt == nil || reconciled.InvestigationOutcome != string(aicontracts.OutcomeFixVerified) {
		t.Fatalf("reconciled finding=%#v", reconciled)
	}
	select {
	case notification := <-notifications:
		if notification.Type != "fix_completed" || notification.ActionType != "view_fix_result" {
			t.Fatalf("terminal notification=%#v", notification)
		}
	default:
		t.Fatal("verified Proxmox finding notification was not published")
	}
}

func proxmoxPatrolReadState(resource unified.Resource) unified.ReadState {
	registry := unified.NewRegistry(nil)
	registry.IngestResources([]unified.Resource{resource})
	return registry
}

func proxmoxPatrolVMResource(status unified.ResourceStatus, observedAt time.Time) unified.Resource {
	resource := proxmoxGuestActionResource("vm:160", unified.ResourceTypeVM, "running", observedAt)
	resource.Status = status
	resource.LastSeen = observedAt
	resource.UpdatedAt = observedAt
	resource.SourceStatus[unified.SourceProxmox] = unified.SourceStatus{Status: "online", LastSeen: observedAt}
	if status == unified.StatusOffline {
		resource.Capabilities = []unified.ResourceCapability{{
			Name: "start", Type: unified.CapabilityTypeCommon, Description: "Start this Proxmox VM",
			MinimumApprovalLevel: unified.ApprovalAdmin, Platform: "qemu", InternalHandler: proxmoxVMLifecycleHandler,
		}}
	}
	return resource
}
