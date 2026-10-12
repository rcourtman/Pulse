package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationreceipt"
	"github.com/rcourtman/pulse-go-rewrite/internal/relay"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

func TestAPTUpdateDetectorProposalApprovalDispatchAuditAndFindingReconciliation(t *testing.T) {
	now := time.Now().UTC()
	resource := hostUpdateActionResource(now)
	finding := detectSingleAPTWorkflowFinding(t, resource, now)
	if finding.ID != "apt:host-update:agent:host-1" || finding.Key != "apt-host-updates" {
		t.Fatalf("finding=%#v", finding)
	}
	assertBoundedAPTWorkflowEvidence(t, finding)

	resources := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
	resources.SetStateProvider(resourceUnifiedSeedProvider{snapshot: models.StateSnapshot{LastUpdate: now}, resources: []unified.Resource{resource}})
	agents := &fakeHostUpdateAgent{connected: true, result: &agentexec.HostUpdateResultPayload{
		Success: true, MutationStarted: true, ExecutionPhase: agentexec.HostUpdatePhaseComplete,
		Before:        agentexec.HostPackageUpdateSnapshot{Supported: true, Manager: "apt", InventoryHash: testHostPackageInventoryHash, PendingCount: 3, Packages: []agentexec.HostPackageUpdate{{Name: "private-package"}}, CheckedAt: now.Add(-time.Second)},
		After:         agentexec.HostPackageUpdateSnapshot{Supported: true, Manager: "apt", InventoryHash: testHostPackageEmptyInventoryHash, Packages: []agentexec.HostPackageUpdate{{Name: "private-package"}}, CheckedAt: now},
		HealthChecked: true, PackageManagerHealthy: true, Verification: agentexec.HostUpdateVerificationVerified,
		Error: "raw stderr token secret /private/cache/path",
	}}
	resources.SetActionExecutor(newRoutedActionExecutor(resources, newHostUpdateActionExecutor(resources, agents)))
	runAPTWorkflowFindingJourney(t, resources, finding, hostPackageUpdateCapability)
	if len(agents.requests) != 1 || agents.requests[0].Operation != agentexec.HostUpdateOperationInstall {
		t.Fatalf("typed update requests=%#v", agents.requests)
	}
}

func TestAPTUpdateEmptyExecutionPhaseCannotVerifyResolveOrReplay(t *testing.T) {
	now := time.Now().UTC()
	resource := hostUpdateActionResource(now)
	finding := detectSingleAPTWorkflowFinding(t, resource, now)
	resources := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
	resources.SetStateProvider(resourceUnifiedSeedProvider{snapshot: models.StateSnapshot{LastUpdate: now}, resources: []unified.Resource{resource}})
	agents := &fakeHostUpdateAgent{connected: true, result: &agentexec.HostUpdateResultPayload{
		Success: true,
		Before: agentexec.HostPackageUpdateSnapshot{
			Supported: true, Manager: "apt", InventoryHash: testHostPackageInventoryHash, PendingCount: 3, CheckedAt: now.Add(-time.Second),
		},
		After: agentexec.HostPackageUpdateSnapshot{
			Supported: true, Manager: "apt", InventoryHash: testHostPackageEmptyInventoryHash, CheckedAt: now,
		},
		HealthChecked: true, PackageManagerHealthy: true, Verification: agentexec.HostUpdateVerificationVerified,
	}}
	resources.SetActionExecutor(newRoutedActionExecutor(resources, newHostUpdateActionExecutor(resources, agents)))

	aiHandler, patrol, _, _ := setupAIHandlerWithPatrol(t)
	notifications := make(chan relay.PushNotificationPayload, 2)
	patrol.SetPushNotifyCallback(func(payload relay.PushNotificationPayload) { notifications <- payload })
	if !patrol.GetFindings().Add(finding) {
		t.Fatal("deterministic finding was not admitted")
	}
	investigations := newTestInvestigationStore()
	investigation := investigations.Create(finding.ID, "invalid-result-session")
	aiHandler.investigationStores = map[string]aicontracts.InvestigationStore{"default": investigations}
	aiHandler.SetResourceStoreProvider(resources.getStore)
	resources.SetActionTransitionPublisher(aiHandler.ReconcilePatrolActionTransition)
	store, err := resources.getStore("default")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetResourceOperatorState(unified.ResourceOperatorState{
		CanonicalID: resource.ID,
		AutoRemediationPolicy: unified.AutoRemediationPolicy{
			Enabled: true, CapabilityNames: []string{hostPackageUpdateCapability},
		},
		SetAt: now, SetBy: "operator@example.com",
	}); err != nil {
		t.Fatal(err)
	}

	proposal := aicontracts.ActionProposal{
		ProposalID: "proposal-invalid-host-update", FindingID: finding.ID, InvestigationID: investigation.ID,
		ResourceID: finding.ResourceID, CapabilityName: hostPackageUpdateCapability, Params: map[string]any{},
		Reason: "Reject a malformed typed host update result without replay.",
	}
	broker := NewPatrolActionBroker("default", resources, func(context.Context, string) (PatrolActionPolicySnapshot, error) {
		return PatrolActionPolicySnapshot{EffectiveAutonomyLevel: "full", FullModeUnlocked: true}, nil
	})
	disposition, err := broker.Submit(context.Background(), proposal)
	if err == nil || !strings.Contains(err.Error(), `unsupported host update execution phase ""`) {
		t.Fatalf("Submit disposition=%#v err=%v", disposition, err)
	}
	duplicate, duplicateErr := broker.Submit(context.Background(), proposal)
	if duplicateErr != nil {
		t.Fatalf("duplicate Submit disposition=%#v err=%v", duplicate, duplicateErr)
	}
	if duplicate.State != string(unified.ActionStateExecuting) || duplicate.VerificationStatus == string(unified.VerificationVerified) || len(agents.requests) != 1 || len(agents.queries) != 1 {
		t.Fatalf("duplicate disposition=%#v requests=%d queries=%d", duplicate, len(agents.requests), len(agents.queries))
	}
	reader, ok := store.(unified.ActionAuditOriginReader)
	if !ok {
		t.Fatal("action store does not expose canonical origin lookup")
	}
	audit, found, err := reader.GetLatestActionAuditByOrigin(patrolActionOriginSurface, investigation.ID)
	if err != nil || !found || audit.State != unified.ActionStateExecuting || audit.VerificationOutcome.Status == unified.VerificationVerified {
		t.Fatalf("malformed result audit found=%v err=%v audit=%#v", found, err, audit)
	}
	reconciled := patrol.GetFindings().Get(finding.ID)
	if reconciled == nil || reconciled.ResolvedAt != nil || reconciled.InvestigationOutcome == string(aicontracts.OutcomeFixVerified) {
		t.Fatalf("malformed result reconciled finding=%#v", reconciled)
	}
	select {
	case notification := <-notifications:
		t.Fatalf("malformed result emitted terminal notification=%#v", notification)
	default:
	}
}

func TestAPTUpdateCallbackLossServerRestartReconcilesTerminalAuditAndFindingWithoutResend(t *testing.T) {
	now := time.Now().UTC()
	resource := hostUpdateActionResource(now)
	finding := detectSingleAPTWorkflowFinding(t, resource, now)
	agents := &fakeHostUpdateAgent{connected: true, err: errors.New("controlled callback loss")}
	runAPTWorkflowReceiptRecoveryJourney(t, resource, finding, hostPackageUpdateCapability,
		func(resources *ResourceHandlers) ActionExecutor {
			return newHostUpdateActionExecutor(resources, agents)
		},
		func(attempt unified.ActionDispatchAttempt) {
			payload := agentexec.HostUpdateResultPayload{
				RequestID: attempt.ID, ActionID: attempt.ActionID, Success: true, MutationStarted: true, ExecutionPhase: agentexec.HostUpdatePhaseComplete,
				Before:        agentexec.HostPackageUpdateSnapshot{Supported: true, Manager: "apt", InventoryHash: testHostPackageInventoryHash, PendingCount: 3, CheckedAt: now.Add(-2 * time.Second)},
				After:         agentexec.HostPackageUpdateSnapshot{Supported: true, Manager: "apt", InventoryHash: testHostPackageEmptyInventoryHash, CheckedAt: now.Add(-time.Second)},
				HealthChecked: true, PackageManagerHealthy: true, Verification: agentexec.HostUpdateVerificationVerified,
			}
			raw, _ := json.Marshal(payload)
			identity := operationreceipt.Identity{AttemptID: attempt.ID, ActionID: attempt.ActionID, OperationKind: attempt.OperationKind, OperationVersion: attempt.OperationVersion, RequestDigest: attempt.RequestDigest, AgentID: attempt.AgentID}
			agents.queryResult = terminalAPTQuery(identity, agentexec.HostUpdateReceiptKind, raw, now)
		},
		func() (int, int) { return len(agents.requests), len(agents.queries) })
}

func TestAPTCacheCleanupCallbackLossServerRestartReconcilesTerminalAuditAndFindingWithoutResend(t *testing.T) {
	now := time.Now().UTC()
	resource := hostStorageCleanupActionResource(now)
	finding := detectSingleAPTWorkflowFinding(t, resource, now)
	agents := &fakeHostStorageCleanupAgent{connected: true, err: errors.New("controlled callback loss")}
	runAPTWorkflowReceiptRecoveryJourney(t, resource, finding, hostStorageCleanupCapability,
		func(resources *ResourceHandlers) ActionExecutor {
			return newHostStorageCleanupActionExecutor(resources, agents)
		},
		func(attempt unified.ActionDispatchAttempt) {
			payload := agentexec.HostStorageCleanupResultPayload{
				RequestID: attempt.ID, ActionID: attempt.ActionID, Success: true, MutationStarted: true, ExecutionPhase: agentexec.HostStorageCleanupPhaseComplete,
				Before:         agentexec.HostStorageCleanupSnapshot{Supported: true, Provider: "apt-package-cache", Fingerprint: testHostStorageCleanupFingerprint, ReclaimableBytes: 512 * 1024 * 1024, CheckedAt: now.Add(-2 * time.Second)},
				After:          agentexec.HostStorageCleanupSnapshot{Supported: true, Provider: "apt-package-cache", Fingerprint: testHostStorageCleanupAfterFingerprint, ReclaimableBytes: 8 * 1024 * 1024, CheckedAt: now.Add(-time.Second)},
				ReclaimedBytes: 504 * 1024 * 1024, Verification: agentexec.HostStorageCleanupVerificationVerified,
			}
			raw, _ := json.Marshal(payload)
			identity := operationreceipt.Identity{AttemptID: attempt.ID, ActionID: attempt.ActionID, OperationKind: attempt.OperationKind, OperationVersion: attempt.OperationVersion, RequestDigest: attempt.RequestDigest, AgentID: attempt.AgentID}
			agents.queryResult = terminalAPTQuery(identity, agentexec.HostStorageCleanupReceiptKind, raw, now)
		},
		func() (int, int) { return len(agents.requests), len(agents.queries) })
}

func terminalAPTQuery(identity operationreceipt.Identity, kind string, payload []byte, terminalAt time.Time) operationreceipt.QueryResult {
	return operationreceipt.QueryResult{Version: operationreceipt.ProtocolVersion, Status: operationreceipt.QueryFoundTerminal, Record: &operationreceipt.Record{
		Identity: identity, State: operationreceipt.StateTerminal, AcceptedAt: terminalAt.Add(-4 * time.Second), StartedAt: terminalAt.Add(-3 * time.Second), TerminalAt: terminalAt,
		ResultKind: kind, ResultVersion: agentexec.HostAPTReceiptVersion, Result: payload,
	}}
}

func TestAPTCacheCleanupDetectorProposalApprovalDispatchAuditAndFindingReconciliation(t *testing.T) {
	now := time.Now().UTC()
	resource := hostStorageCleanupActionResource(now)
	finding := detectSingleAPTWorkflowFinding(t, resource, now)
	if finding.ID != "apt:cache-cleanup:agent:host-cleanup" || finding.Key != "apt-package-cache-pressure" {
		t.Fatalf("finding=%#v", finding)
	}
	assertBoundedAPTWorkflowEvidence(t, finding)

	resources := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
	resources.SetStateProvider(resourceUnifiedSeedProvider{snapshot: models.StateSnapshot{LastUpdate: now}, resources: []unified.Resource{resource}})
	agents := &fakeHostStorageCleanupAgent{connected: true, result: &agentexec.HostStorageCleanupResultPayload{
		Success: true, MutationStarted: true, ExecutionPhase: agentexec.HostStorageCleanupPhaseComplete,
		Before:         agentexec.HostStorageCleanupSnapshot{Supported: true, Provider: "apt-package-cache", Fingerprint: testHostStorageCleanupFingerprint, ReclaimableBytes: 512 * 1024 * 1024, CheckedAt: now.Add(-time.Second)},
		After:          agentexec.HostStorageCleanupSnapshot{Supported: true, Provider: "apt-package-cache", Fingerprint: testHostStorageCleanupAfterFingerprint, ReclaimableBytes: 8 * 1024 * 1024, CheckedAt: now},
		ReclaimedBytes: 504 * 1024 * 1024, Verification: agentexec.HostStorageCleanupVerificationVerified,
	}}
	resources.SetActionExecutor(newRoutedActionExecutor(resources, newHostStorageCleanupActionExecutor(resources, agents)))
	runAPTWorkflowFindingJourney(t, resources, finding, hostStorageCleanupCapability)
	if len(agents.requests) != 1 || agents.requests[0].Operation != agentexec.HostStorageCleanupOperationPackageCache {
		t.Fatalf("typed cleanup requests=%#v", agents.requests)
	}
}

func detectSingleAPTWorkflowFinding(t *testing.T, resource unified.Resource, now time.Time) *ai.Finding {
	t.Helper()
	registry := unified.NewRegistry(nil)
	registry.IngestResources([]unified.Resource{resource})
	findings := ai.DetectAPTWorkflowFindings(registry, now)
	if len(findings) != 1 {
		t.Fatalf("detected findings=%#v", findings)
	}
	return findings[0]
}

func assertBoundedAPTWorkflowEvidence(t *testing.T, finding *ai.Finding) {
	t.Helper()
	if finding == nil || finding.Evidence == "" || len(finding.Evidence) > 512 {
		t.Fatalf("unbounded finding evidence=%#v", finding)
	}
	for _, forbidden := range []string{"apt-get", "/var/cache", "package=", "stderr", "--no-remove", "reboot "} {
		if strings.Contains(strings.ToLower(finding.Evidence), forbidden) {
			t.Fatalf("finding evidence exposes command/path/package/reboot authority: %q", finding.Evidence)
		}
	}
}

func runAPTWorkflowFindingJourney(t *testing.T, resources *ResourceHandlers, finding *ai.Finding, capability string) {
	t.Helper()
	aiHandler, patrol, _, _ := setupAIHandlerWithPatrol(t)
	notifications := make(chan relay.PushNotificationPayload, 1)
	patrol.SetPushNotifyCallback(func(payload relay.PushNotificationPayload) { notifications <- payload })
	if !patrol.GetFindings().Add(finding) {
		t.Fatal("deterministic finding was not admitted")
	}
	investigations := newTestInvestigationStore()
	investigation := investigations.Create(finding.ID, "bounded-investigation-session")
	aiHandler.investigationStores = map[string]aicontracts.InvestigationStore{"default": investigations}
	aiHandler.SetResourceStoreProvider(resources.getStore)
	resources.SetActionTransitionPublisher(aiHandler.ReconcilePatrolActionTransition)

	proposal := aicontracts.ActionProposal{
		ProposalID: "proposal-" + finding.ID, FindingID: finding.ID, InvestigationID: investigation.ID,
		ResourceID: finding.ResourceID, CapabilityName: capability, Params: map[string]any{},
		Reason:      "Apply the exact typed action supported by the bounded APT finding evidence.",
		EvidenceIDs: []string{"finding-evidence:" + finding.ID},
	}
	if len(proposal.Params) != 0 {
		t.Fatalf("APT proposal params=%#v, want exact empty object", proposal.Params)
	}
	disposition, err := NewPatrolActionBroker("default", resources).Submit(context.Background(), proposal)
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if disposition.State != string(unified.ActionStatePending) || !disposition.Plan.RequiresApproval {
		t.Fatalf("planned disposition=%#v", disposition)
	}

	decision := httptest.NewRecorder()
	decisionReq := httptest.NewRequest(http.MethodPost, "/api/actions/"+disposition.ActionID+"/decision", bytes.NewBufferString(`{"outcome":"approved","reason":"controlled fake-only proof"}`))
	decisionReq.SetPathValue("id", disposition.ActionID)
	decisionReq = decisionReq.WithContext(auth.WithUser(decisionReq.Context(), "operator@example.com"))
	resources.HandleDecideAction(decision, actionHandlerTestRequest(decisionReq, ""))
	if decision.Code != http.StatusOK {
		t.Fatalf("decision status=%d body=%s", decision.Code, decision.Body.String())
	}

	execution := httptest.NewRecorder()
	executionReq := httptest.NewRequest(http.MethodPost, "/api/actions/"+disposition.ActionID+"/execute", bytes.NewBufferString(`{"reason":"execute approved typed APT action"}`))
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
	if audit.Result == nil || audit.Result.ActionResultV2 == nil || audit.Result.ActionResultV2.Verification.EvidenceClass != unified.ActionEvidenceAgentAttested {
		t.Fatalf("terminal action truth=%#v", audit.Result)
	}
	reconciled := patrol.GetFindings().Get(finding.ID)
	if reconciled == nil || reconciled.ResolvedAt != nil || reconciled.InvestigationOutcome != string(aicontracts.OutcomeFixVerificationUnknown) {
		t.Fatalf("reconciled finding=%#v", reconciled)
	}
	var notification relay.PushNotificationPayload
	select {
	case notification = <-notifications:
	default:
		t.Fatal("terminal finding notification was not published")
	}
	encoded, err := json.Marshal(struct {
		Audit        unified.ActionAuditRecord     `json:"audit"`
		Finding      *ai.Finding                   `json:"finding"`
		Notification relay.PushNotificationPayload `json:"notification"`
	}{Audit: audit, Finding: reconciled, Notification: notification})
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"private-package", "raw stderr", "token secret", "/private/cache/path"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("raw APT detail %q escaped terminal projections: %s", forbidden, encoded)
		}
	}
}

func runAPTWorkflowReceiptRecoveryJourney(t *testing.T, resource unified.Resource, finding *ai.Finding, capability string, executorFactory func(*ResourceHandlers) ActionExecutor, setTerminal func(unified.ActionDispatchAttempt), counts func() (requests, queries int)) {
	t.Helper()
	dataPath := t.TempDir()
	cfg := &config.Config{DataPath: dataPath}
	resources := newActionTestResourceHandlers(t, cfg)
	resources.SetStateProvider(resourceUnifiedSeedProvider{snapshot: models.StateSnapshot{LastUpdate: time.Now().UTC()}, resources: []unified.Resource{resource}})
	resources.SetActionExecutor(newRoutedActionExecutor(resources, executorFactory(resources)))
	aiHandler, patrol, _, _ := setupAIHandlerWithPatrol(t)
	if !patrol.GetFindings().Add(finding) {
		t.Fatal("deterministic finding was not admitted")
	}
	investigations := newTestInvestigationStore()
	investigation := investigations.Create(finding.ID, "receipt-recovery-session")
	aiHandler.investigationStores = map[string]aicontracts.InvestigationStore{"default": investigations}
	aiHandler.SetResourceStoreProvider(resources.getStore)
	resources.SetActionTransitionPublisher(aiHandler.ReconcilePatrolActionTransition)
	disposition, err := NewPatrolActionBroker("default", resources).Submit(context.Background(), aicontracts.ActionProposal{
		ProposalID: "recovery-" + finding.ID, FindingID: finding.ID, InvestigationID: investigation.ID, ResourceID: finding.ResourceID, CapabilityName: capability, Params: map[string]any{}, Reason: "Exercise durable typed receipt recovery.",
	})
	if err != nil {
		t.Fatal(err)
	}
	decisionReq := httptest.NewRequest(http.MethodPost, "/api/actions/"+disposition.ActionID+"/decision", bytes.NewBufferString(`{"outcome":"approved","reason":"controlled recovery proof"}`))
	decisionReq.SetPathValue("id", disposition.ActionID)
	decisionReq = actionHandlerTestRequest(decisionReq, "operator@example.com")
	decision := httptest.NewRecorder()
	resources.HandleDecideAction(decision, decisionReq)
	if decision.Code != http.StatusOK {
		t.Fatalf("decision status=%d body=%s", decision.Code, decision.Body.String())
	}
	executionReq := httptest.NewRequest(http.MethodPost, "/api/actions/"+disposition.ActionID+"/execute", bytes.NewBufferString(`{"reason":"controlled callback loss"}`))
	executionReq.SetPathValue("id", disposition.ActionID)
	executionReq = actionHandlerTestRequest(executionReq, "operator@example.com")
	resources.HandleExecuteAction(httptest.NewRecorder(), executionReq)
	store, err := resources.getStore("default")
	if err != nil {
		t.Fatal(err)
	}
	attempt, found, err := store.GetActionDispatchAttempt(disposition.ActionID)
	if err != nil || !found || attempt.State != unified.ActionDispatchReceiptPending {
		t.Fatalf("pending attempt found=%v err=%v attempt=%#v", found, err, attempt)
	}
	setTerminal(attempt)

	restarted := NewResourceHandlers(cfg)
	restarted.SetStateProvider(resourceUnifiedSeedProvider{snapshot: models.StateSnapshot{LastUpdate: time.Now().UTC()}, resources: []unified.Resource{resource}})
	restarted.SetActionExecutor(newRoutedActionExecutor(restarted, executorFactory(restarted)))
	restarted.SetActionTransitionPublisher(aiHandler.ReconcilePatrolActionTransition)
	aiHandler.SetResourceStoreProvider(restarted.getStore)
	recovered, err := restarted.ActionLifecycle().RecoverExecutingActions(context.Background(), "default", "system:apt-receipt-recovery", 10)
	if err != nil || len(recovered) != 1 || recovered[0].State != unified.ActionStateCompleted {
		t.Fatalf("recovered=%#v err=%v", recovered, err)
	}
	requests, queries := counts()
	if requests != 1 || queries != 1 {
		t.Fatalf("requests=%d queries=%d; recovery must query once without resend", requests, queries)
	}
	reconciled := patrol.GetFindings().Get(finding.ID)
	if reconciled == nil || reconciled.ResolvedAt != nil || reconciled.InvestigationOutcome != string(aicontracts.OutcomeFixVerificationUnknown) {
		t.Fatalf("reconciled finding=%#v", reconciled)
	}
}
