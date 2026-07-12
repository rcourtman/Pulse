package api

import (
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/actionlifecycle"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationreceipt"
	"github.com/rcourtman/pulse-go-rewrite/internal/relay"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
)

// TestPatrolAutonomyColimaRealLabCanonicalJourney is the RG-06 mutation proof.
// It is environment-gated and must be run only by the disposable Colima
// runner. The fixture is a real Linux Unified Agent with the production
// clean_package_cache capability; this test never changes capability policy.
func TestPatrolAutonomyColimaRealLabCanonicalJourney(t *testing.T) {
	runID := strings.TrimSpace(os.Getenv("PULSE_INTELLIGENCE_RG06_RUN_ID"))
	agentID := strings.TrimSpace(os.Getenv("PULSE_INTELLIGENCE_RG06_AGENT_ID"))
	agentImage := strings.TrimSpace(os.Getenv("PULSE_INTELLIGENCE_RG06_AGENT_IMAGE"))
	agentBinary := strings.TrimSpace(os.Getenv("PULSE_INTELLIGENCE_RG06_AGENT_BINARY"))
	sourceDir := strings.TrimSpace(os.Getenv("PULSE_INTELLIGENCE_RG06_SOURCE_DIR"))
	artifactDir := strings.TrimSpace(os.Getenv("PULSE_INTELLIGENCE_RG06_ARTIFACT_DIR"))
	scratchDir := strings.TrimSpace(os.Getenv("PULSE_INTELLIGENCE_RG06_SCRATCH_DIR"))
	gitSHA := strings.TrimSpace(os.Getenv("PULSE_INTELLIGENCE_RG06_GIT_SHA"))
	if runID == "" || agentID == "" || agentImage == "" || agentBinary == "" || sourceDir == "" || artifactDir == "" || scratchDir == "" || gitSHA == "" {
		t.Skip("released RG-06 Colima lab environment is not configured")
	}
	if os.Getenv("DOCKER_CONTEXT") != "colima" {
		t.Fatal("RG-06 real lab requires DOCKER_CONTEXT=colima")
	}
	workingDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	expectedWorkingDir := filepath.Join(sourceDir, "internal", "api")
	if filepath.Clean(workingDir) != filepath.Clean(expectedWorkingDir) {
		t.Fatalf("RG-06 proof must execute from pinned archive source=%q cwd=%q", sourceDir, workingDir)
	}
	bindingBytes, err := os.ReadFile(filepath.Join(sourceDir, ".pulse-rg06-source-binding.json"))
	if err != nil {
		t.Fatal(err)
	}
	var sourceBinding struct {
		GitSHA        string `json:"git_sha"`
		ArchiveSHA256 string `json:"archive_sha256"`
	}
	if err := json.Unmarshal(bindingBytes, &sourceBinding); err != nil || sourceBinding.GitSHA != gitSHA || len(sourceBinding.ArchiveSHA256) != 64 {
		t.Fatalf("pinned archive binding mismatch sha=%q requested=%q err=%v", sourceBinding.GitSHA, gitSHA, err)
	}
	if err := os.MkdirAll(artifactDir, 0o700); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	orgID := "rg06-" + runID

	// The acknowledgement and activation are written and reopened before any
	// action is planned. Their digest is a measured authority baseline.
	configDir := filepath.Join(scratchDir, "config")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	persistence := config.NewConfigPersistence(configDir)
	InitSessionStore(scratchDir)
	initial := config.NewDefaultAIConfig()
	initial.PatrolAutonomyLevel = config.PatrolAutonomyApproval
	if err := persistence.SaveAIConfig(*initial); err != nil {
		t.Fatal(err)
	}
	policyNow := now
	settings := newTestAISettingsHandler(&config.Config{DataPath: configDir}, persistence, nil)
	settings.defaultAIService.SetOrgID(orgID)
	settings.SetPatrolAutopilotServerPolicyProvider(func() unified.PatrolAutopilotServerPolicy {
		return unified.CurrentPatrolAutopilotServerPolicy(policyNow)
	})
	ack := createPatrolAutopilotAcknowledgement(t, settings, orgID, "rg06-operator", "rg06-session-"+runID, "ack-"+runID)
	if ack.Code != 201 {
		t.Fatalf("acknowledgement status=%d body=%s", ack.Code, ack.Body.String())
	}
	activationReq := patrolAutopilotSessionRequest(t, "PUT", "/api/ai/patrol/autonomy", fmt.Sprintf(`{"autonomy_level":"full","acknowledgement_id":"ack-%s","investigation_budget":10,"investigation_timeout_sec":120}`, runID), orgID, "rg06-operator", "rg06-session-"+runID)
	activationRec := httptest.NewRecorder()
	handlePatrolAutonomyUpdateForTest(settings, activationRec, activationReq)
	if activationRec.Code != http.StatusOK {
		t.Fatalf("activation status=%d body=%s", activationRec.Code, activationRec.Body.String())
	}
	stored, err := persistence.LoadAIConfig()
	if err != nil {
		t.Fatal(err)
	}
	effective, status := stored.GetEffectivePatrolAutonomyWithPolicy(orgID, unified.CurrentPatrolAutopilotServerPolicy(policyNow))
	if effective != config.PatrolAutonomyFull || !status.Active || stored.PatrolAutopilotActivation == nil {
		t.Fatalf("reopened full-mode evidence effective=%q status=%#v", effective, status)
	}
	configDigest := digestJSON(t, stored)

	// Bind a real agent execution server to a non-loopback listener so the
	// disposable Linux agent can connect through host.docker.internal.
	server := &countingRG06AgentServer{Server: agentexec.NewServer(func(token, id, host string) bool {
		return token == "rg06-local-lab" && id == agentID
	})}
	reportCh := make(chan agentshost.Report, 32)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/agents/agent/report", func(w http.ResponseWriter, r *http.Request) {
		var body io.Reader = r.Body
		if r.Header.Get("Content-Encoding") == "gzip" {
			gz, gzipErr := gzip.NewReader(r.Body)
			if gzipErr != nil {
				http.Error(w, "invalid gzip", http.StatusBadRequest)
				return
			}
			defer gz.Close()
			body = gz
		}
		var report agentshost.Report
		if decodeErr := json.NewDecoder(body).Decode(&report); decodeErr != nil {
			http.Error(w, "invalid report", http.StatusBadRequest)
			return
		}
		select {
		case reportCh <- report:
		default:
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, fmt.Sprintf(`{"success":true,"agentId":%q}`, agentID))
	})
	mux.HandleFunc("/", server.HandleWebSocket)
	ws := httptest.NewUnstartedServer(mux)
	listener, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		t.Fatal(err)
	}
	ws.Listener = listener
	ws.Start()
	defer ws.Close()
	containerID := startRG06Agent(t, runID, agentID, agentImage, agentBinary, ws.Listener.Addr().(*net.TCPAddr).Port)
	defer stopRG06Agent(containerID)
	waitForRG06Agent(t, server.Server, agentID)
	var report agentshost.Report
	var lastReport agentshost.Report
	reportCount := 0
	reportDeadline := time.After(30 * time.Second)
	for {
		select {
		case candidate := <-reportCh:
			reportCount++
			lastReport = candidate
			cleanup := candidate.Host.StorageCleanup
			if candidate.Agent.CommandsEnabled && candidate.Agent.OperationReceiptVersion == operationreceipt.ProtocolVersion && cleanup != nil && cleanup.Supported && cleanup.ReclaimableBytes >= unified.HostStorageCleanupMinReclaimableBytes {
				report = candidate
				goto reportReady
			}
		case <-reportDeadline:
			t.Fatalf("production Linux agent did not publish an eligible storage-cleanup report: reports=%d commands=%v receipt=%d cleanup=%#v", reportCount, lastReport.Agent.CommandsEnabled, lastReport.Agent.OperationReceiptVersion, lastReport.Host.StorageCleanup)
		}
	}

reportReady:

	// ApplyHostReport and HostIngestRecord are the production projection path.
	// Capability eligibility is therefore derived from the actual agent report,
	// never manufactured or changed by this proof.
	monitor, err := monitoring.New(&config.Config{DataPath: filepath.Join(scratchDir, "monitor")})
	if err != nil {
		t.Fatal(err)
	}
	host, err := monitor.ApplyHostReport(report, nil)
	if err != nil {
		t.Fatal(err)
	}
	reportAt := host.LastSeen
	if reportAt.IsZero() {
		reportAt = time.Now().UTC()
	}
	resource := unified.HostIngestRecord(host).Resource
	if _, ok := resourceCapabilityByName(resource.Capabilities, hostStorageCleanupCapability); !ok {
		t.Fatalf("production host adapter did not advertise %q: %#v", hostStorageCleanupCapability, resource.Capabilities)
	}
	resources := newActionTestResourceHandlers(t, &config.Config{DataPath: filepath.Join(scratchDir, "pulse-state")})
	resourceProvider := &mutableResourceUnifiedSeedProvider{snapshot: models.StateSnapshot{LastUpdate: reportAt}, resources: []unified.Resource{resource}, freshness: reportAt}
	resources.SetStateProvider(resourceProvider)
	store, err := resources.getStore(orgID)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetResourceOperatorState(unified.ResourceOperatorState{CanonicalID: resource.ID, AutoRemediationPolicy: unified.AutoRemediationPolicy{Enabled: true, CapabilityNames: []string{hostStorageCleanupCapability}}, SetAt: reportAt, SetBy: "rg06-operator"}); err != nil {
		t.Fatal(err)
	}
	resources.SetActionExecutor(newRoutedActionExecutor(resources, newHostStorageCleanupActionExecutor(resources, server)))
	aiHandler, patrol, _, _ := setupAIHandlerWithPatrol(t)
	patrol.SetPushNotifyCallback(func(relay.PushNotificationPayload) {})
	resources.SetActionTransitionPublisher(aiHandler.ReconcilePatrolActionTransition)
	investigations := newTestInvestigationStore()
	aiHandler.investigationStores = map[string]aicontracts.InvestigationStore{orgID: investigations}
	aiHandler.SetResourceStoreProvider(resources.getStore)

	var policyOverride atomic.Value
	policyProvider := func(context.Context, string) (PatrolActionPolicySnapshot, error) {
		current, loadErr := persistence.LoadAIConfig()
		if loadErr != nil {
			return PatrolActionPolicySnapshot{}, loadErr
		}
		level, currentStatus := current.GetEffectivePatrolAutonomyWithPolicy(orgID, unified.CurrentPatrolAutopilotServerPolicy(policyNow))
		if value, _ := policyOverride.Load().(string); value == "downgrade" {
			level, currentStatus.Active = config.PatrolAutonomyApproval, false
		}
		return PatrolActionPolicySnapshot{EffectiveAutonomyLevel: level, FullModeUnlocked: level == config.PatrolAutonomyFull && currentStatus.Active, EmergencyStop: current.PatrolActionEmergencyStop, PolicyVersion: fmt.Sprintf("rg06:%d", unified.CurrentPatrolAutopilotServerPolicy(policyNow).CurrentVersion)}, nil
	}
	broker := NewPatrolActionBroker(orgID, resources, policyProvider).(*patrolActionBroker)

	// Barriers are evaluated before the successful cleanup so each starts with
	// real pressure. The helper measures all five zero dimensions and records
	// lifecycle refusal separately from the triple-zero claim.
	barriers := []map[string]any{
		rg06BarrierMeasured(t, "emergency_stop", "action_emergency_stop", persistence, store, patrol, investigations, resource, func() error {
			return mutateRG06Config(persistence, func(cfg *config.AIConfig) { cfg.PatrolActionEmergencyStop = true })
		}, func() {
			_ = mutateRG06Config(persistence, func(cfg *config.AIConfig) { cfg.PatrolActionEmergencyStop = false })
		}, func() { policyOverride.Store("") }, func(findingID, investigationID string) (aicontracts.ActionDisposition, error) {
			return rg06SubmitBarrier(context.Background(), broker, rg06BarrierProposalFor(runID, "emergency", resource.ID, findingID, investigationID))
		}, server, containerID),
		rg06BarrierMeasured(t, "policy_downgrade", "policy_authorization_revoked", persistence, store, patrol, investigations, resource, func() error { policyOverride.Store("downgrade"); return nil }, func() {}, func() { policyOverride.Store("") }, func(findingID, investigationID string) (aicontracts.ActionDisposition, error) {
			return rg06SubmitBarrier(context.Background(), broker, rg06BarrierProposalFor(runID, "downgrade", resource.ID, findingID, investigationID))
		}, server, containerID),
		rg06BarrierMeasured(t, "stale_resource", "policy_authorization_revoked", persistence, store, patrol, investigations, resource, func() error {
			resourceProvider.freshness = time.Now().UTC().Add(-2 * time.Hour)
			resourceProvider.resources[0].SourceStatus[unified.SourceAgent] = unified.SourceStatus{Status: "stale", LastSeen: time.Now().UTC().Add(-2 * time.Hour)}
			return nil
		}, func() {}, func() {
			resourceProvider.freshness = reportAt
			resourceProvider.resources[0].SourceStatus[unified.SourceAgent] = unified.SourceStatus{Status: "online", LastSeen: reportAt}
		}, func(findingID, investigationID string) (aicontracts.ActionDisposition, error) {
			return rg06SubmitBarrier(context.Background(), broker, rg06BarrierProposalFor(runID, "stale", resource.ID, findingID, investigationID))
		}, server, containerID),
		rg06BarrierMeasured(t, "never_auto_remediate", "policy_authorization_revoked", persistence, store, patrol, investigations, resource, func() error {
			return store.SetResourceOperatorState(unified.ResourceOperatorState{CanonicalID: resource.ID, NeverAutoRemediate: true, AutoRemediationPolicy: unified.AutoRemediationPolicy{Enabled: true, CapabilityNames: []string{hostStorageCleanupCapability}}, SetAt: time.Now().UTC(), SetBy: "rg06-operator"})
		}, func() {}, func() {
			_ = store.SetResourceOperatorState(unified.ResourceOperatorState{CanonicalID: resource.ID, AutoRemediationPolicy: unified.AutoRemediationPolicy{Enabled: true, CapabilityNames: []string{hostStorageCleanupCapability}}, SetAt: time.Now().UTC(), SetBy: "rg06-operator"})
		}, func(findingID, investigationID string) (aicontracts.ActionDisposition, error) {
			return rg06SubmitBarrier(context.Background(), broker, rg06BarrierProposalFor(runID, "never", resource.ID, findingID, investigationID))
		}, server, containerID),
	}
	_ = barriers

	cacheBeforePositive := rg06CacheState(t, containerID)
	if !cacheBeforePositive.Exists || cacheBeforePositive.Bytes <= 0 {
		t.Fatal("disposable APT cache fixture was not present before authorized cleanup")
	}
	positiveFinding := addPatrolFindingForResource(t, patrol, "finding-"+runID, reportAt, resource.ID, "Disposable APT cache pressure")
	positiveInvestigation := investigations.Create(positiveFinding.ID, "investigation-"+runID)
	positive, err := broker.Submit(context.Background(), rg06Proposal(runID, positiveFinding.ID, positiveInvestigation.ID, resource.ID))
	if err != nil || positive.State != string(unified.ActionStateCompleted) {
		t.Fatalf("unattended positive disposition=%#v err=%v", positive, err)
	}
	audit, found, err := store.GetActionAudit(positive.ActionID)
	if err != nil || !found || audit.Result == nil || audit.Result.ActionResultV2 == nil || audit.State != unified.ActionStateCompleted {
		t.Fatalf("positive terminal audit=%#v found=%v err=%v", audit, found, err)
	}
	attempt, attemptFound, err := store.GetActionDispatchAttempt(positive.ActionID)
	if err != nil || !attemptFound || attempt.DispatchCount != 1 || server.CleanupCalls() != 1 {
		t.Fatalf("positive dispatch attempt=%#v found=%v calls=%d err=%v", attempt, attemptFound, server.CleanupCalls(), err)
	}
	receipt, receiptFound, err := store.GetActionDispatchReceipt(attempt.ID)
	if err != nil || !receiptFound || receipt.TransportRequestID != attempt.ID {
		t.Fatalf("positive receipt=%#v found=%v err=%v", receipt, receiptFound, err)
	}
	positiveEvents, err := store.GetActionLifecycleEvents(positive.ActionID, time.Time{}, 1000)
	if err != nil || len(positiveEvents) == 0 {
		t.Fatalf("positive lifecycle audit events=%#v err=%v", positiveEvents, err)
	}
	truth := audit.Result.ActionResultV2
	if truth.Execution.Status != unified.ActionExecutionSucceeded || truth.Verification.Status != unified.ActionVerificationConfirmed || truth.Verification.EvidenceClass != unified.ActionEvidenceIndependent {
		t.Fatalf("positive result truth=%#v", truth)
	}
	if audit.Request.Actor.OrgID != orgID || audit.Request.Actor.Kind != unified.ActionActorService || audit.Request.ResourceID != resource.ID || audit.Request.CapabilityName != hostStorageCleanupCapability || audit.Plan.PolicyVersion == "" || audit.Plan.PolicyDecision.Scope.OrgID != orgID || audit.Plan.PolicyDecision.Scope.ResourceID != resource.ID || audit.Plan.PolicyDecision.Scope.CapabilityName != hostStorageCleanupCapability {
		t.Fatalf("positive authority binding audit=%#v", audit)
	}
	reconciled := patrol.GetFindings().Get(positiveFinding.ID)
	if reconciled == nil || reconciled.ResolvedAt == nil || reconciled.InvestigationOutcome != string(aicontracts.OutcomeFixVerified) {
		t.Fatalf("positive finding reconciliation=%#v", reconciled)
	}
	cacheAfterPositive := rg06CacheState(t, containerID)
	if cacheAfterPositive.Exists || cacheAfterPositive.Bytes != 0 || cacheAfterPositive.Fingerprint == cacheBeforePositive.Fingerprint || cacheAfterPositive.ContainerID != cacheBeforePositive.ContainerID || !cacheAfterPositive.Running || cacheAfterPositive.StartedAt != cacheBeforePositive.StartedAt {
		t.Fatalf("independent cache readback did not show bounded reclamation before=%#v after=%#v", cacheBeforePositive, cacheAfterPositive)
	}

	// Revocation is planned while current, then revoked before the dispatch
	// policy boundary. This negative records actual command/attempt/receipt
	// deltas, not merely the refusal error.
	revocationFinding := addPatrolFindingForResource(t, patrol, "finding-"+runID+"-revoked", reportAt, resource.ID, "Revocation barrier")
	revocationInvestigation := investigations.Create(revocationFinding.ID, "investigation-"+runID+"-revoked")
	proposal := rg06BarrierProposal(runID, "revoked", resource.ID)
	factors, autoAuthorized := broker.planPolicyFactors(context.Background(), proposal, policyNow)
	if !autoAuthorized {
		t.Fatal("revocation precondition was not auto-eligible before revocation")
	}
	plan, err := broker.lifecycle().PlanWithOptions(context.Background(), orgID, unified.ActionRequest{RequestID: proposal.ProposalID, ResourceID: resource.ID, CapabilityName: hostStorageCleanupCapability, Reason: proposal.Reason, RequestedBy: patrolActionBrokerActor}, actionlifecycle.PlanOptions{Actor: unified.ActionActor{SubjectID: patrolActionBrokerActor, Kind: unified.ActionActorService, CredentialID: "service:patrol-action-broker", OrgID: orgID}, Origin: &unified.ActionOrigin{Surface: patrolActionOriginSurface, FindingID: revocationFinding.ID, InvestigationID: revocationInvestigation.ID, ProposalID: proposal.ProposalID}, PolicyFactors: factors})
	if err != nil {
		t.Fatal(err)
	}
	if err := mutateRG06Config(persistence, func(cfg *config.AIConfig) {
		revocation, created, revokeErr := unified.RevokePatrolAutopilotAcknowledgement(cfg.PatrolAutopilotAcknowledgements, cfg.PatrolAutopilotRevocations, stored.PatrolAutopilotAcknowledgements[0].ID, stored.PatrolAutopilotAcknowledgements[0].Actor, "RG-06 dispatch barrier", unified.CurrentPatrolAutopilotServerPolicy(policyNow))
		if revokeErr != nil || !created {
			t.Fatalf("revoke acknowledgement created=%v err=%v", created, revokeErr)
		}
		cfg.PatrolAutopilotRevocations = append(cfg.PatrolAutopilotRevocations, revocation)
		cfg.PatrolFullModeUnlocked = false
	}); err != nil {
		t.Fatal(err)
	}
	beforeBarrier := rg06MeasuredState(t, persistence, store, server, containerID, revocationFinding)
	revokedRecord, barrierErr := broker.lifecycle().ExecuteUnderPolicy(context.Background(), orgID, plan.ActionID, patrolActionPolicyActor, func(ctx context.Context, current unified.ActionAuditRecord, at time.Time) (unified.ActionPolicyAuthorizationLease, string, error) {
		return broker.policyAuthorizationLease(ctx, proposal, current, at)
	})
	afterBarrier := rg06MeasuredState(t, persistence, store, server, containerID, revocationFinding)
	if barrierErr == nil || revokedRecord.State == unified.ActionStateCompleted || !strings.Contains(strings.ToLower(barrierErr.Error()), "policy_authorization_revoked") || beforeBarrier.Cache != afterBarrier.Cache || beforeBarrier.CleanupCalls != afterBarrier.CleanupCalls || beforeBarrier.ConfigDigest != afterBarrier.ConfigDigest || afterBarrier.AttemptRecordCount != beforeBarrier.AttemptRecordCount || afterBarrier.DispatchCount != beforeBarrier.DispatchCount || afterBarrier.DispatchReceipts != beforeBarrier.DispatchReceipts || beforeBarrier.FindingResolved != afterBarrier.FindingResolved {
		t.Fatalf("revocation barrier violated measured triple-zero before=%#v after=%#v err=%v", beforeBarrier, afterBarrier, barrierErr)
	}
	barriers = append(barriers, map[string]any{"name": "revoked_acknowledgement", "error": barrierErr.Error(), "before": beforeBarrier, "after": afterBarrier, "audit_event_delta": afterBarrier.AuditEvents - beforeBarrier.AuditEvents, "triple_zero": map[string]int{"unauthorized_mutations": boolInt(beforeBarrier.Cache != afterBarrier.Cache), "transport_dispatches": afterBarrier.CleanupCalls - beforeBarrier.CleanupCalls, "authority_writes": boolInt(beforeBarrier.ConfigDigest != afterBarrier.ConfigDigest)}, "lifecycle_refusal": true})

	artifact := map[string]any{"git_sha": gitSHA, "source_dir": sourceDir, "source_binding": sourceBinding, "run_id": runID, "org_id": orgID, "config_digest_before_barriers": configDigest, "resource_id": resource.ID, "capability": hostStorageCleanupCapability, "dispatch": map[string]any{"attempt_id": attempt.ID, "attempt_record_count": 1, "dispatch_count": attempt.DispatchCount, "receipt_record_count": 1, "receipt_transport_request_id": receipt.TransportRequestID, "cleanup_calls": server.CleanupCalls(), "audit_events": len(positiveEvents)}, "readback": map[string]any{"before": cacheBeforePositive, "after": cacheAfterPositive, "evidence": truth.Verification}, "finding": map[string]any{"id": reconciled.ID, "outcome": reconciled.InvestigationOutcome, "resolved": reconciled.ResolvedAt != nil}, "authorized_dispatches": 1, "barriers": barriers, "triple_zero_note": "Negative barriers measure cache/daemon mutation, transport dispatch, and authority/config writes independently; expected lifecycle refusal events are recorded separately and are not counted as unauthorized mutation."}
	encoded, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(artifactDir, "canonical-journey.json"), encoded, 0o600); err != nil {
		t.Fatal(err)
	}
}

type countingRG06AgentServer struct {
	*agentexec.Server
	cleanupCalls atomic.Int64
}

func (s *countingRG06AgentServer) ExecuteHostStorageCleanup(ctx context.Context, agentID string, req agentexec.HostStorageCleanupPayload) (*agentexec.HostStorageCleanupResultPayload, error) {
	s.cleanupCalls.Add(1)
	return s.Server.ExecuteHostStorageCleanup(ctx, agentID, req)
}
func (s *countingRG06AgentServer) CleanupCalls() int { return int(s.cleanupCalls.Load()) }

func startRG06Agent(t *testing.T, runID, agentID, image, binary string, port int) string {
	t.Helper()
	name := "pulse-rg06-agent-" + runID
	args := []string{"--context", "colima", "create", "--name", name, "--label", "com.pulse.intelligence-lab.run=" + runID, "--label", "com.pulse.intelligence-lab.gate=rg-06", "--add-host", "host.docker.internal:host-gateway", "--tmpfs", "/var/cache/apt/archives:size=72m", "-e", fmt.Sprintf("PULSE_URL=http://host.docker.internal:%d", port), "-e", "PULSE_TOKEN=rg06-local-lab", "-e", "PULSE_AGENT_ID=" + agentID, "-e", "PULSE_HOSTNAME=rg06-agent-" + runID, "-e", "PULSE_ENABLE_HOST=true", "-e", "PULSE_ENABLE_COMMANDS=true", "-e", "PULSE_INTERVAL=2s", "-e", "PULSE_HEALTH_ADDR=off", image, "/bin/sh", "-c", "dd if=/dev/zero of=/var/cache/apt/archives/rg06-fixture.deb bs=1M count=65 status=none && touch -d @0 /var/cache/apt/archives/rg06-fixture.deb && exec /usr/local/bin/pulse-agent"}
	cmd := exec.Command("docker", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("create RG-06 agent: %v: %s", err, strings.TrimSpace(string(out)))
	}
	containerID := strings.TrimSpace(string(out))
	copyOutput, err := exec.Command("docker", "--context", "colima", "cp", binary, containerID+":/usr/local/bin/pulse-agent").CombinedOutput()
	if err != nil {
		stopRG06Agent(containerID)
		t.Fatalf("copy pinned agent into RG-06 container: %v: %s", err, strings.TrimSpace(string(copyOutput)))
	}
	startOutput, err := exec.Command("docker", "--context", "colima", "start", containerID).CombinedOutput()
	if err != nil {
		stopRG06Agent(containerID)
		t.Fatalf("start RG-06 agent: %v: %s", err, strings.TrimSpace(string(startOutput)))
	}
	return containerID
}

func stopRG06Agent(containerID string) {
	if strings.TrimSpace(containerID) != "" {
		_ = exec.Command("docker", "--context", "colima", "rm", "-f", containerID).Run()
	}
}

func waitForRG06Agent(t *testing.T, server *agentexec.Server, agentID string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for !server.IsAgentConnected(agentID) && time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
	}
	if !server.IsAgentConnected(agentID) {
		t.Fatal("production Linux agent did not connect through agent WebSocket")
	}
}

type rg06CacheSnapshot struct {
	Exists           bool   `json:"exists"`
	Bytes            int64  `json:"bytes"`
	ReclaimableBytes int64  `json:"reclaimable_bytes"`
	Fingerprint      string `json:"fingerprint"`
	ContainerID      string `json:"container_id"`
	Running          bool   `json:"running"`
	StartedAt        string `json:"started_at"`
}

func rg06CacheState(t *testing.T, containerID string) rg06CacheSnapshot {
	t.Helper()
	inspect := exec.Command("docker", "--context", "colima", "inspect", "--format", "{{.Id}}|{{.State.Running}}|{{.State.StartedAt}}", containerID)
	inspectOutput, err := inspect.Output()
	if err != nil {
		t.Fatalf("inspect RG-06 agent: %v", err)
	}
	parts := strings.Split(strings.TrimSpace(string(inspectOutput)), "|")
	if len(parts) != 3 {
		t.Fatalf("unexpected RG-06 inspect output %q", inspectOutput)
	}
	stat := exec.Command("docker", "--context", "colima", "exec", containerID, "sh", "-c", "if [ -f /var/cache/apt/archives/rg06-fixture.deb ]; then stat -c '%s %Y' /var/cache/apt/archives/rg06-fixture.deb; else echo '0 0'; fi")
	statOutput, err := stat.Output()
	if err != nil {
		t.Fatalf("read RG-06 cache state: %v", err)
	}
	values := strings.Fields(string(statOutput))
	if len(values) != 2 {
		t.Fatalf("unexpected RG-06 cache stat %q", statOutput)
	}
	bytes, err := strconv.ParseInt(values[0], 10, 64)
	if err != nil {
		t.Fatalf("parse RG-06 cache bytes: %v", err)
	}
	seconds, err := strconv.ParseInt(values[1], 10, 64)
	if err != nil {
		t.Fatalf("parse RG-06 cache mtime: %v", err)
	}
	hashInput := []byte{}
	if bytes > 0 {
		hashInput = []byte(fmt.Sprintf("rg06-fixture.deb\x00%d\x00%d\n", bytes, seconds*int64(time.Second)))
	}
	digest := sha256.Sum256(hashInput)
	return rg06CacheSnapshot{Exists: bytes > 0, Bytes: bytes, ReclaimableBytes: bytes, Fingerprint: "sha256:" + hex.EncodeToString(digest[:]), ContainerID: parts[0], Running: parts[1] == "true", StartedAt: parts[2]}
}

type rg06Measured struct {
	Cache              rg06CacheSnapshot `json:"cache"`
	CleanupCalls       int               `json:"cleanup_calls"`
	ConfigDigest       string            `json:"config_digest"`
	DispatchReceipts   int               `json:"dispatch_receipts"`
	AttemptRecordCount int               `json:"attempt_record_count"`
	DispatchCount      int               `json:"dispatch_count"`
	AuditEvents        int               `json:"audit_events"`
	FindingResolved    bool              `json:"finding_resolved"`
}

func rg06MeasuredState(t *testing.T, persistence *config.ConfigPersistence, store unified.ResourceStore, server *countingRG06AgentServer, containerID string, finding *ai.Finding) rg06Measured {
	t.Helper()
	cfg, err := persistence.LoadAIConfig()
	if err != nil {
		t.Fatal(err)
	}
	audits, err := store.GetActionAudits("", time.Time{}, 1000)
	if err != nil {
		t.Fatal(err)
	}
	measured := rg06Measured{Cache: rg06CacheState(t, containerID), CleanupCalls: server.CleanupCalls(), ConfigDigest: digestJSON(t, cfg), FindingResolved: finding != nil && finding.ResolvedAt != nil}
	for _, audit := range audits {
		events, eventErr := store.GetActionLifecycleEvents(audit.ID, time.Time{}, 1000)
		if eventErr != nil {
			t.Fatal(eventErr)
		}
		measured.AuditEvents += len(events)
		attempt, ok, attemptErr := store.GetActionDispatchAttempt(audit.ID)
		if attemptErr != nil {
			t.Fatal(attemptErr)
		}
		if !ok {
			continue
		}
		measured.AttemptRecordCount++
		measured.DispatchCount += attempt.DispatchCount
		_, receiptOK, receiptErr := store.GetActionDispatchReceipt(attempt.ID)
		if receiptErr != nil {
			t.Fatal(receiptErr)
		}
		if receiptOK {
			measured.DispatchReceipts++
		}
	}
	return measured
}

func rg06BarrierMeasured(t *testing.T, name, expectedReason string, persistence *config.ConfigPersistence, store unified.ResourceStore, patrol *ai.PatrolService, investigations aicontracts.InvestigationStore, resource unified.Resource, setup func() error, teardown func(), reset func(), submit func(string, string) (aicontracts.ActionDisposition, error), server *countingRG06AgentServer, containerID string) map[string]any {
	t.Helper()
	finding := addPatrolFindingForResource(t, patrol, "finding-barrier-"+name, time.Now().UTC(), resource.ID, "RG-06 "+name+" barrier")
	investigation := investigations.Create(finding.ID, "investigation-barrier-"+name)
	before := rg06MeasuredState(t, persistence, store, server, containerID, finding)
	if err := setup(); err != nil {
		t.Fatal(err)
	}
	barrierBefore := rg06MeasuredState(t, persistence, store, server, containerID, finding)
	disposition, err := submit(finding.ID, investigation.ID)
	teardown()
	barrierAfter := rg06MeasuredState(t, persistence, store, server, containerID, finding)
	reset()
	after := rg06MeasuredState(t, persistence, store, server, containerID, finding)
	if disposition.State == string(unified.ActionStateCompleted) || err == nil || !strings.Contains(strings.ToLower(errorString(err)), strings.ToLower(expectedReason)) {
		t.Fatalf("barrier %q was not refused with expected reason %q: disposition=%#v err=%v", name, expectedReason, disposition, err)
	}
	if barrierBefore.Cache != barrierAfter.Cache || barrierBefore.CleanupCalls != barrierAfter.CleanupCalls || barrierBefore.ConfigDigest != barrierAfter.ConfigDigest || barrierBefore.AttemptRecordCount != barrierAfter.AttemptRecordCount || barrierBefore.DispatchCount != barrierAfter.DispatchCount || barrierBefore.DispatchReceipts != barrierAfter.DispatchReceipts || barrierBefore.FindingResolved != barrierAfter.FindingResolved {
		t.Fatalf("barrier %q violated measured triple-zero before=%#v after=%#v", name, barrierBefore, barrierAfter)
	}
	return map[string]any{"name": name, "state": disposition.State, "error": errorString(err), "before": before, "barrier_before": barrierBefore, "barrier_after": barrierAfter, "after": after, "audit_event_delta": barrierAfter.AuditEvents - barrierBefore.AuditEvents, "triple_zero": map[string]int{"unauthorized_mutations": boolInt(barrierBefore.Cache != barrierAfter.Cache), "transport_dispatches": barrierAfter.CleanupCalls - barrierBefore.CleanupCalls, "authority_writes": boolInt(barrierBefore.ConfigDigest != barrierAfter.ConfigDigest)}, "lifecycle_refusal": true}
}

func rg06SubmitBarrier(ctx context.Context, broker *patrolActionBroker, proposal aicontracts.ActionProposal) (aicontracts.ActionDisposition, error) {
	disposition, err := broker.Submit(ctx, proposal)
	if err != nil {
		return disposition, err
	}
	if disposition.State != string(unified.ActionStatePending) {
		return disposition, fmt.Errorf("barrier action was not pending before policy admission: state=%s", disposition.State)
	}
	record, executeErr := broker.lifecycle().ExecuteUnderPolicy(ctx, broker.orgID, disposition.ActionID, patrolActionPolicyActor, func(ctx context.Context, current unified.ActionAuditRecord, at time.Time) (unified.ActionPolicyAuthorizationLease, string, error) {
		return broker.policyAuthorizationLease(ctx, proposal, current, at)
	})
	return dispositionFromRecord(record), executeErr
}

func rg06Proposal(runID, findingID, investigationID, resourceID string) aicontracts.ActionProposal {
	return aicontracts.ActionProposal{ProposalID: "proposal-" + runID + "-" + findingID, FindingID: findingID, InvestigationID: investigationID, ResourceID: resourceID, CapabilityName: hostStorageCleanupCapability, Params: map[string]any{}, Reason: "Reclaim bounded APT package-cache pressure through the production host-storage-cleanup executor.", EvidenceIDs: []string{"finding-evidence:" + findingID}}
}
func rg06BarrierProposal(runID, name, resourceID string) aicontracts.ActionProposal {
	return rg06Proposal(runID+"-"+name, "finding-barrier-"+name, "investigation-barrier-"+name, resourceID)
}
func rg06BarrierProposalFor(runID, name, resourceID, findingID, investigationID string) aicontracts.ActionProposal {
	return rg06Proposal(runID+"-"+name, findingID, investigationID, resourceID)
}
func mutateRG06Config(persistence *config.ConfigPersistence, mutate func(*config.AIConfig)) error {
	cfg, err := persistence.LoadAIConfig()
	if err != nil {
		return err
	}
	mutate(cfg)
	return persistence.SaveAIConfig(*cfg)
}
func digestJSON(t *testing.T, value any) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}
func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
