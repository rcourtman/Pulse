package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/hostagent"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/relay"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	"github.com/rs/zerolog"
)

func TestDockerRestartColimaRealLabCanonicalJourney(t *testing.T) {
	runID := strings.TrimSpace(os.Getenv("PULSE_INTELLIGENCE_LAB_RUN_ID"))
	containerID := strings.ToLower(strings.TrimSpace(os.Getenv("PULSE_INTELLIGENCE_LAB_CONTAINER_ID")))
	artifactDir := strings.TrimSpace(os.Getenv("PULSE_INTELLIGENCE_LAB_ARTIFACT_DIR"))
	scratchDir := strings.TrimSpace(os.Getenv("PULSE_INTELLIGENCE_LAB_SCRATCH_DIR"))
	if runID == "" || containerID == "" || artifactDir == "" || scratchDir == "" {
		t.Skip("released Colima lab environment is not configured")
	}
	if os.Getenv("DOCKER_CONTEXT") != "colima" {
		t.Fatal("real lab requires DOCKER_CONTEXT=colima")
	}
	before, err := observeColimaContainer(context.Background(), containerID)
	if err != nil {
		t.Fatal(err)
	}

	agentServer := agentexec.NewServer(func(token, agent, host string) bool {
		return token == "task06-local-lab" && agent == "task06-colima-agent"
	})
	wsServer := httptest.NewServer(http.HandlerFunc(agentServer.HandleWebSocket))
	defer wsServer.Close()
	logger := zerolog.Nop()
	client := hostagent.NewCommandClient(hostagent.Config{PulseURL: wsServer.URL, APIToken: "task06-local-lab", StateDir: filepath.Join(scratchDir, "agent-state"), Logger: &logger}, "task06-colima-agent", "task06-colima", "darwin", "task06-current-build")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- client.Run(ctx) }()
	defer func() { cancel(); _ = client.Close(); <-done }()
	deadline := time.Now().Add(5 * time.Second)
	for !agentServer.IsAgentConnected("task06-colima-agent") && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !agentServer.IsAgentConnected("task06-colima-agent") {
		t.Fatal("real Unified Agent command client did not connect")
	}

	resource := dockerContainerActionResource("app-container:"+runID, "docker", before.State, before.ObservedAt)
	resource.Name = "pulse-task06-" + runID
	resource.Docker.AgentID = "task06-colima-agent"
	resource.Docker.ContainerID = containerID
	resource.Docker.StartedAt = &before.StartedAt
	resource.Docker.RestartCount = before.RestartCount
	resources := newActionTestResourceHandlers(t, &config.Config{DataPath: filepath.Join(scratchDir, "pulse-state")})
	resources.SetStateProvider(resourceUnifiedSeedProvider{snapshot: models.StateSnapshot{LastUpdate: before.ObservedAt}, resources: []unified.Resource{resource}})
	executor := dockerContainerActionExecutor{resources: resources, agents: agentServer, observer: colimaDirectObserver{}}
	resources.SetActionExecutor(newRoutedActionExecutor(resources, executor))

	aiHandler, patrol, _, _ := setupAIHandlerWithPatrol(t)
	notifications := make(chan relay.PushNotificationPayload, 1)
	patrol.SetPushNotifyCallback(func(payload relay.PushNotificationPayload) { notifications <- payload })
	finding := &ai.Finding{ID: "docker-restart-loop:" + runID, Key: "docker-container-restart-loop", ResourceID: resource.ID, ResourceName: resource.Name, ResourceType: "docker", Title: "Container restart loop", Description: "Fresh Colima daemon evidence reported repeated restarts.", Evidence: fmt.Sprintf("restartCount=%d; state=%s; observedAt=%s", before.RestartCount, before.State, before.ObservedAt.Format(time.RFC3339Nano)), Severity: ai.FindingSeverityWarning, Category: ai.FindingCategoryReliability, DetectedAt: before.ObservedAt}
	if !patrol.GetFindings().Add(finding) {
		t.Fatal("fresh Docker finding was not admitted")
	}
	investigations := newTestInvestigationStore()
	investigation := investigations.Create(finding.ID, "task06-colima-bounded-investigation")
	aiHandler.investigationStores = map[string]aicontracts.InvestigationStore{"default": investigations}
	aiHandler.SetResourceStoreProvider(resources.getStore)
	resources.SetActionTransitionPublisher(aiHandler.ReconcilePatrolActionTransition)

	proposal := aicontracts.ActionProposal{ProposalID: "proposal-" + runID, FindingID: finding.ID, InvestigationID: investigation.ID, ResourceID: resource.ID, CapabilityName: "restart", Params: map[string]any{}, Reason: "Restart the exact fresh disposable Colima fixture after explicit approval.", EvidenceIDs: []string{"finding-evidence:" + finding.ID}}
	disposition, err := NewPatrolActionBroker("default", resources).Submit(context.Background(), proposal)
	if err != nil || disposition.State != string(unified.ActionStatePending) || !disposition.Plan.RequiresApproval {
		t.Fatalf("proposal disposition=%#v err=%v", disposition, err)
	}
	decision := httptest.NewRecorder()
	decisionReq := httptest.NewRequest(http.MethodPost, "/api/actions/"+disposition.ActionID+"/decision", bytes.NewBufferString(`{"outcome":"approved","reason":"authorized disposable Colima lab"}`))
	decisionReq.SetPathValue("id", disposition.ActionID)
	decisionReq = decisionReq.WithContext(auth.WithUser(decisionReq.Context(), "operator@example.com"))
	resources.HandleDecideAction(decision, actionHandlerTestRequest(decisionReq, ""))
	if decision.Code != http.StatusOK {
		t.Fatalf("decision status=%d body=%s", decision.Code, decision.Body.String())
	}
	execution := httptest.NewRecorder()
	executionReq := httptest.NewRequest(http.MethodPost, "/api/actions/"+disposition.ActionID+"/execute", bytes.NewBufferString(`{"reason":"execute one approved typed Docker restart"}`))
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
	attempt, attemptFound, attemptErr := store.GetActionDispatchAttempt(disposition.ActionID)
	receipt, receiptFound, receiptErr := store.GetActionDispatchReceipt(attempt.ID)
	if err != nil || !found || attemptErr != nil || !attemptFound || receiptErr != nil || !receiptFound || audit.State != unified.ActionStateCompleted || audit.Result == nil || audit.Result.ActionResultV2 == nil {
		t.Fatalf("terminal audit/attempt audit=%#v found=%v err=%v attempt=%#v attemptFound=%v attemptErr=%v receipt=%#v receiptFound=%v receiptErr=%v", audit, found, err, attempt, attemptFound, attemptErr, receipt, receiptFound, receiptErr)
	}
	truth := audit.Result.ActionResultV2
	if truth.Execution.Status != unified.ActionExecutionSucceeded || truth.Verification.Status != unified.ActionVerificationConfirmed || truth.Verification.EvidenceClass != unified.ActionEvidenceIndependent || len(truth.Verification.Evidence) != 1 {
		t.Fatalf("terminal truth=%#v", truth)
	}
	if attempt.ID == "" || attempt.RequestDigest == "" || attempt.State != unified.ActionDispatchReceiptRecorded {
		t.Fatalf("durable attempt=%#v", attempt)
	}
	reconciled := patrol.GetFindings().Get(finding.ID)
	if reconciled == nil || reconciled.ResolvedAt == nil || reconciled.InvestigationOutcome != string(aicontracts.OutcomeFixVerified) {
		t.Fatalf("reconciled finding=%#v", reconciled)
	}
	var notification relay.PushNotificationPayload
	select {
	case notification = <-notifications:
	default:
		t.Fatal("terminal desktop/mobile notification projection was not published")
	}
	after, err := observeColimaContainer(context.Background(), containerID)
	if err != nil || !after.StartedAt.After(before.StartedAt) {
		t.Fatalf("direct postcondition before=%s after=%s err=%v", before.StartedAt, after.StartedAt, err)
	}
	artifact := dockerRestartLabArtifact{
		RunID:         runID,
		Proposal:      dockerRestartLabProposal{ProposalID: proposal.ProposalID, FindingID: proposal.FindingID, InvestigationID: proposal.InvestigationID, ResourceID: proposal.ResourceID, CapabilityName: proposal.CapabilityName, Params: proposal.Params, EvidenceIDs: proposal.EvidenceIDs},
		Investigation: dockerRestartLabInvestigation{ID: investigation.ID, FindingID: finding.ID},
		Action: dockerRestartLabAction{
			ID: audit.ID, CreatedAt: audit.CreatedAt, UpdatedAt: audit.UpdatedAt, State: audit.State, DecisionRevision: audit.DecisionRevision,
			Request: dockerRestartLabActionRequest{RequestID: audit.Request.RequestID, ResourceID: audit.Request.ResourceID, CapabilityName: audit.Request.CapabilityName, Params: audit.Request.Params, Reason: audit.Request.Reason, RequestedBy: audit.Request.RequestedBy},
			Plan:    audit.Plan, Origin: audit.Origin,
			Result: dockerRestartLabActionResult{Success: audit.Result.Success, ActionResultV2: *truth}, VerificationOutcome: audit.VerificationOutcome,
		},
		Attempt:      dockerRestartLabAttempt{ID: attempt.ID, ActionID: attempt.ActionID, State: attempt.State, CreatedAt: attempt.CreatedAt, UpdatedAt: attempt.UpdatedAt, DispatchCount: attempt.DispatchCount, OperationKind: attempt.OperationKind, OperationVersion: attempt.OperationVersion, RequestDigest: attempt.RequestDigest, AgentID: attempt.AgentID},
		Receipt:      receipt,
		Finding:      dockerRestartLabFinding{ID: reconciled.ID, Outcome: reconciled.InvestigationOutcome, ResolvedAt: reconciled.ResolvedAt},
		Notification: dockerRestartLabNotification{Kind: notification.Type, Outcome: string(truth.Verification.Status), ActionType: notification.ActionType, ActionID: notification.ActionID, Category: notification.Category, Severity: notification.Severity},
		Before:       before, After: after, Evidence: truth.Verification.Evidence[0],
	}
	encoded, _ := json.MarshalIndent(artifact, "", "  ")
	if err := os.WriteFile(filepath.Join(artifactDir, "canonical-journey.json"), encoded, 0o600); err != nil {
		t.Fatal(err)
	}
}

type dockerRestartLabArtifact struct {
	RunID         string                                     `json:"run_id"`
	Proposal      dockerRestartLabProposal                   `json:"proposal"`
	Investigation dockerRestartLabInvestigation              `json:"investigation"`
	Action        dockerRestartLabAction                     `json:"action"`
	Attempt       dockerRestartLabAttempt                    `json:"attempt"`
	Receipt       unified.ActionDispatchReceipt              `json:"receipt"`
	Finding       dockerRestartLabFinding                    `json:"finding"`
	Notification  dockerRestartLabNotification               `json:"notification"`
	Before        agentexec.DockerContainerLifecycleSnapshot `json:"before"`
	After         agentexec.DockerContainerLifecycleSnapshot `json:"after"`
	Evidence      unified.ActionEvidence                     `json:"evidence"`
}

type dockerRestartLabProposal struct {
	ProposalID      string         `json:"proposal_id"`
	FindingID       string         `json:"finding_id"`
	InvestigationID string         `json:"investigation_id"`
	ResourceID      string         `json:"resource_id"`
	CapabilityName  string         `json:"capability_name"`
	Params          map[string]any `json:"params"`
	EvidenceIDs     []string       `json:"evidence_ids"`
}

type dockerRestartLabInvestigation struct {
	ID        string `json:"id"`
	FindingID string `json:"finding_id"`
}

type dockerRestartLabActionRequest struct {
	RequestID      string         `json:"requestId"`
	ResourceID     string         `json:"resourceId"`
	CapabilityName string         `json:"capabilityName"`
	Params         map[string]any `json:"params"`
	Reason         string         `json:"reason"`
	RequestedBy    string         `json:"requestedBy"`
}

type dockerRestartLabActionResult struct {
	Success        bool                   `json:"success"`
	ActionResultV2 unified.ActionResultV2 `json:"actionResultV2"`
}

type dockerRestartLabAction struct {
	ID                  string                        `json:"id"`
	CreatedAt           time.Time                     `json:"createdAt"`
	UpdatedAt           time.Time                     `json:"updatedAt"`
	State               unified.ActionState           `json:"state"`
	DecisionRevision    uint64                        `json:"decisionRevision"`
	Request             dockerRestartLabActionRequest `json:"request"`
	Plan                unified.ActionPlan            `json:"plan"`
	Origin              *unified.ActionOrigin         `json:"origin,omitempty"`
	Result              dockerRestartLabActionResult  `json:"result"`
	VerificationOutcome unified.VerificationOutcome   `json:"verificationOutcome"`
}

type dockerRestartLabAttempt struct {
	ID               string                      `json:"id"`
	ActionID         string                      `json:"actionId"`
	State            unified.ActionDispatchState `json:"state"`
	CreatedAt        time.Time                   `json:"createdAt"`
	UpdatedAt        time.Time                   `json:"updatedAt"`
	DispatchCount    int                         `json:"dispatchCount"`
	OperationKind    string                      `json:"operationKind"`
	OperationVersion int                         `json:"operationVersion"`
	RequestDigest    string                      `json:"requestDigest"`
	AgentID          string                      `json:"agentId"`
}

type dockerRestartLabFinding struct {
	ID         string     `json:"id"`
	Outcome    string     `json:"outcome"`
	ResolvedAt *time.Time `json:"resolved_at"`
}

type dockerRestartLabNotification struct {
	Kind       string `json:"kind"`
	Outcome    string `json:"outcome"`
	ActionType string `json:"action_type"`
	ActionID   string `json:"action_id"`
	Category   string `json:"category"`
	Severity   string `json:"severity"`
}

type colimaDirectObserver struct{}

func (colimaDirectObserver) ObserveDockerContainer(ctx context.Context, actionID, containerID string) (dockerContainerPostconditionObservation, error) {
	snapshot, err := observeColimaContainer(ctx, containerID)
	return dockerContainerPostconditionObservation{ObserverID: "task06-colima-direct", TrustDomain: "daemon:colima-direct", Method: "docker_context_colima_inspect", Snapshot: snapshot, ReceivedAt: time.Now().UTC()}, err
}

func observeColimaContainer(ctx context.Context, containerID string) (agentexec.DockerContainerLifecycleSnapshot, error) {
	raw, err := exec.CommandContext(ctx, "docker", "--context", "colima", "inspect", "--format", `{{json .State}}`, containerID).CombinedOutput()
	if err != nil {
		return agentexec.DockerContainerLifecycleSnapshot{}, fmt.Errorf("direct Colima inspect: %w", err)
	}
	var state struct {
		Status    string `json:"Status"`
		Running   bool   `json:"Running"`
		StartedAt string `json:"StartedAt"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(raw), &state); err != nil {
		return agentexec.DockerContainerLifecycleSnapshot{}, err
	}
	startedAt, err := time.Parse(time.RFC3339Nano, state.StartedAt)
	if err != nil {
		return agentexec.DockerContainerLifecycleSnapshot{}, err
	}
	restartRaw, err := exec.CommandContext(ctx, "docker", "--context", "colima", "inspect", "--format", `{{.RestartCount}}`, containerID).CombinedOutput()
	if err != nil {
		return agentexec.DockerContainerLifecycleSnapshot{}, err
	}
	restartCount, err := strconv.Atoi(strings.TrimSpace(string(restartRaw)))
	if err != nil {
		return agentexec.DockerContainerLifecycleSnapshot{}, err
	}
	return agentexec.DockerContainerLifecycleSnapshot{ContainerID: containerID, State: strings.ToLower(state.Status), Running: state.Running, StartedAt: startedAt.UTC(), RestartCount: restartCount, ObservedAt: time.Now().UTC()}, nil
}
