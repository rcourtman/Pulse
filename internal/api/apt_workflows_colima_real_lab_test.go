package api

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationreceipt"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

// TestAPTWorkflowsColimaRealLabDebianAndUbuntu is the mutation-gated RG-09
// certification. It runs only from an exact-SHA archive prepared by the
// dedicated Colima wrapper. The disposable agents use a local one-package APT
// repository; all default distribution sources are removed before the agent
// starts, so the fixed production upgrade catalog cannot select another
// package.
func TestAPTWorkflowsColimaRealLabDebianAndUbuntu(t *testing.T) {
	runID := strings.TrimSpace(os.Getenv("PULSE_INTELLIGENCE_RG09_RUN_ID"))
	agentBinary := strings.TrimSpace(os.Getenv("PULSE_INTELLIGENCE_RG09_AGENT_BINARY"))
	sourceDir := strings.TrimSpace(os.Getenv("PULSE_INTELLIGENCE_RG09_SOURCE_DIR"))
	artifactDir := strings.TrimSpace(os.Getenv("PULSE_INTELLIGENCE_RG09_ARTIFACT_DIR"))
	scratchDir := strings.TrimSpace(os.Getenv("PULSE_INTELLIGENCE_RG09_SCRATCH_DIR"))
	gitSHA := strings.TrimSpace(os.Getenv("PULSE_INTELLIGENCE_RG09_GIT_SHA"))
	images := map[string]string{
		"debian": strings.TrimSpace(os.Getenv("PULSE_INTELLIGENCE_RG09_DEBIAN_IMAGE")),
		"ubuntu": strings.TrimSpace(os.Getenv("PULSE_INTELLIGENCE_RG09_UBUNTU_IMAGE")),
	}
	if runID == "" || agentBinary == "" || sourceDir == "" || artifactDir == "" || scratchDir == "" || gitSHA == "" || images["debian"] == "" || images["ubuntu"] == "" {
		t.Skip("released RG-09 Colima lab environment is not configured")
	}
	if os.Getenv("DOCKER_CONTEXT") != "colima" {
		t.Fatal("RG-09 real lab requires DOCKER_CONTEXT=colima")
	}
	workingDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Clean(workingDir) != filepath.Clean(filepath.Join(sourceDir, "internal", "api")) {
		t.Fatalf("RG-09 proof must execute from pinned archive source=%q cwd=%q", sourceDir, workingDir)
	}
	bindingBytes, err := os.ReadFile(filepath.Join(sourceDir, ".pulse-rg09-source-binding.json"))
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

	results := map[string]any{}
	for _, distro := range []string{"debian", "ubuntu"} {
		distro := distro
		t.Run(distro, func(t *testing.T) {
			results[distro] = runRG09Distro(t, runID, distro, images[distro], agentBinary, scratchDir, sourceBinding)
		})
	}
	artifact := map[string]any{
		"gate":           "RG-09",
		"git_sha":        gitSHA,
		"run_id":         runID,
		"source_binding": sourceBinding,
		"distros":        results,
		"evidence_boundary": map[string]any{
			"action_result_v2":     "agent_attested",
			"independent_readback": "colima_control_plane",
		},
	}
	encoded, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(artifactDir, "canonical-journey.json"), append(encoded, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}

type rg09Server struct {
	*agentexec.Server
	mu           sync.Mutex
	updateCalls  int
	cleanupCalls int
}

func (s *rg09Server) ExecuteHostUpdate(ctx context.Context, agentID string, req agentexec.HostUpdatePayload) (*agentexec.HostUpdateResultPayload, error) {
	s.mu.Lock()
	s.updateCalls++
	s.mu.Unlock()
	return s.Server.ExecuteHostUpdate(ctx, agentID, req)
}

func (s *rg09Server) ExecuteHostStorageCleanup(ctx context.Context, agentID string, req agentexec.HostStorageCleanupPayload) (*agentexec.HostStorageCleanupResultPayload, error) {
	s.mu.Lock()
	s.cleanupCalls++
	s.mu.Unlock()
	return s.Server.ExecuteHostStorageCleanup(ctx, agentID, req)
}

func (s *rg09Server) counts() (updates, cleanups int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.updateCalls, s.cleanupCalls
}

type rg09WorkflowProof struct {
	ActionID       string                      `json:"action_id"`
	AttemptID      string                      `json:"attempt_id"`
	DispatchCount  int                         `json:"dispatch_count"`
	EvidenceClass  unified.ActionEvidenceClass `json:"evidence_class"`
	FindingID      string                      `json:"finding_id"`
	FindingOutcome string                      `json:"finding_outcome"`
	Resolved       bool                        `json:"resolved"`
}

func runRG09Distro(t *testing.T, runID, distro, image, agentBinary, scratchDir string, sourceBinding any) map[string]any {
	t.Helper()
	agentID := fmt.Sprintf("rg09-%s-%s", distro, runID)
	server := &rg09Server{Server: agentexec.NewServer(func(token, id, host string) bool {
		return token == "rg09-local-lab" && id == agentID
	})}
	reportCh := make(chan agentshost.Report, 64)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/agents/agent/report", rg09ReportHandler(agentID, reportCh))
	mux.HandleFunc("/", server.HandleWebSocket)
	ws := httptest.NewUnstartedServer(mux)
	listener, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		t.Fatal(err)
	}
	ws.Listener = listener
	ws.Start()
	defer ws.Close()
	containerID := startRG09Agent(t, runID, distro, agentID, image, agentBinary, scratchDir, ws.Listener.Addr().(*net.TCPAddr).Port)
	defer func() {
		if t.Failed() {
			t.Logf("RG-09 %s agent failure diagnostic (redacted):\n%s", distro, rg09AgentFailureDiagnostic(containerID))
		}
		stopRG09Agent(containerID)
	}()
	waitForRG06Agent(t, server.Server, agentID)

	initialReport := waitForRG09Report(t, reportCh, func(report agentshost.Report) bool {
		updates := report.Host.PackageUpdates
		cleanup := report.Host.StorageCleanup
		return report.Agent.CommandsEnabled && report.Agent.OperationReceiptVersion == operationreceipt.ProtocolVersion &&
			updates != nil && updates.Supported && updates.PendingCount == 1 &&
			cleanup != nil && cleanup.Supported && cleanup.ReclaimableBytes >= unified.HostStorageCleanupMinReclaimableBytes
	})
	if got := strings.ToLower(strings.TrimSpace(initialReport.Host.Platform)); got != distro {
		t.Fatalf("agent platform=%q want=%q", got, distro)
	}
	resource := rg09ResourceFromReport(t, initialReport, agentID, filepath.Join(scratchDir, distro, "monitor-initial"))
	findings := ai.DetectAPTWorkflowFindings(rg09Registry(resource), time.Now().UTC())
	updateFinding := rg09FindingByKey(t, findings, "apt-host-updates")
	cleanupFinding := rg09FindingByKey(t, findings, "apt-package-cache-pressure")

	barriers := []map[string]any{
		rg09UpdateDriftBarrier(t, server, agentID, containerID, initialReport.Host.PackageUpdates.InventoryHash, distro),
		rg09RefreshFailureBarrier(t, server, agentID, containerID, initialReport.Host.PackageUpdates.InventoryHash, distro),
	}
	updateProof := runRG09PositiveWorkflow(t, filepath.Join(scratchDir, distro, "update-state"), resource, updateFinding, hostPackageUpdateCapability, func(resources *ResourceHandlers) ActionExecutor {
		return newHostUpdateActionExecutor(resources, server)
	})

	restartRG09Agent(t, server.Server, agentID, containerID, reportCh)
	postUpdateReport := waitForRG09Report(t, reportCh, func(report agentshost.Report) bool {
		updates := report.Host.PackageUpdates
		cleanup := report.Host.StorageCleanup
		return updates != nil && updates.Supported && updates.PendingCount == 0 && cleanup != nil && cleanup.Supported && cleanup.ReclaimableBytes > 0
	})
	postUpdateResource := rg09ResourceFromReport(t, postUpdateReport, agentID, filepath.Join(scratchDir, distro, "monitor-post-update"))
	postUpdateFindings := ai.DetectAPTWorkflowFindings(rg09Registry(postUpdateResource), time.Now().UTC())
	cleanupFinding = rg09FindingByKey(t, postUpdateFindings, "apt-package-cache-pressure")
	barriers = append(barriers, rg09CleanupDriftBarrier(t, server, agentID, containerID, postUpdateReport.Host.StorageCleanup.Fingerprint, distro))
	cleanupProof := runRG09PositiveWorkflow(t, filepath.Join(scratchDir, distro, "cleanup-state"), postUpdateResource, cleanupFinding, hostStorageCleanupCapability, func(resources *ResourceHandlers) ActionExecutor {
		return newHostStorageCleanupActionExecutor(resources, server)
	})

	readback := rg09IndependentReadback(t, containerID)
	if readback.InstalledVersion != "2" || readback.PendingFixtureUpgrade || readback.CacheArchiveBytes != 0 {
		t.Fatalf("independent readback=%#v", readback)
	}
	updates, cleanups := server.counts()
	if updates != 3 || cleanups != 2 {
		t.Fatalf("typed command counts update=%d cleanup=%d", updates, cleanups)
	}
	encodedProof, err := json.Marshal(struct {
		Update  rg09WorkflowProof `json:"update"`
		Cleanup rg09WorkflowProof `json:"cleanup"`
	}{updateProof, cleanupProof})
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"pulse-rg09-fixture", "/var/cache/apt", "apt-get", "stderr"} {
		if strings.Contains(strings.ToLower(string(encodedProof)), strings.ToLower(forbidden)) {
			t.Fatalf("raw APT detail %q escaped canonical audit/finding projection: %s", forbidden, encodedProof)
		}
	}
	return map[string]any{
		"distro": distro, "image": image, "agent_id": agentID, "resource_id": resource.ID,
		"source_binding":       sourceBinding,
		"detected_findings":    []string{updateFinding.ID, cleanupFinding.ID},
		"positive":             map[string]any{"update": updateProof, "cleanup": cleanupProof},
		"physical_barriers":    barriers,
		"typed_command_calls":  map[string]int{"host_update": updates, "host_storage_cleanup": cleanups},
		"independent_readback": readback,
	}
}

func rg09ReportHandler(agentID string, reports chan<- agentshost.Report) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body io.Reader = r.Body
		if r.Header.Get("Content-Encoding") == "gzip" {
			gz, err := gzip.NewReader(r.Body)
			if err != nil {
				http.Error(w, "invalid gzip", http.StatusBadRequest)
				return
			}
			defer gz.Close()
			body = gz
		}
		var report agentshost.Report
		if err := json.NewDecoder(body).Decode(&report); err != nil {
			http.Error(w, "invalid report", http.StatusBadRequest)
			return
		}
		select {
		case reports <- report:
		default:
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, fmt.Sprintf(`{"success":true,"agentId":%q}`, agentID))
	}
}

func waitForRG09Report(t *testing.T, reports <-chan agentshost.Report, ready func(agentshost.Report) bool) agentshost.Report {
	t.Helper()
	deadline := time.After(60 * time.Second)
	var last agentshost.Report
	for {
		select {
		case candidate := <-reports:
			last = candidate
			if ready(candidate) {
				return candidate
			}
		case <-deadline:
			t.Fatalf("timed out waiting for RG-09 agent telemetry: platform=%q updates=%#v cleanup=%#v", last.Host.Platform, last.Host.PackageUpdates, last.Host.StorageCleanup)
		}
	}
}

func rg09ResourceFromReport(t *testing.T, report agentshost.Report, agentID, monitorPath string) unified.Resource {
	t.Helper()
	monitor, err := monitoring.New(&config.Config{DataPath: monitorPath})
	if err != nil {
		t.Fatal(err)
	}
	host, err := monitor.ApplyHostReport(report, &config.APITokenRecord{ID: agentID})
	if err != nil {
		t.Fatal(err)
	}
	registry := unified.NewRegistry(nil)
	registry.IngestRecords(unified.SourceAgent, []unified.IngestRecord{unified.HostIngestRecord(host)})
	resources := registry.List()
	if len(resources) != 1 {
		t.Fatalf("host report projected %d resources", len(resources))
	}
	return resources[0]
}

func rg09Registry(resource unified.Resource) *unified.ResourceRegistry {
	registry := unified.NewRegistry(nil)
	registry.IngestResources([]unified.Resource{resource})
	return registry
}

func rg09FindingByKey(t *testing.T, findings []*ai.Finding, key string) *ai.Finding {
	t.Helper()
	for _, finding := range findings {
		if finding != nil && finding.Key == key {
			return finding
		}
	}
	t.Fatalf("finding %q missing from %#v", key, findings)
	return nil
}

func runRG09PositiveWorkflow(t *testing.T, dataPath string, resource unified.Resource, finding *ai.Finding, capability string, executorFactory func(*ResourceHandlers) ActionExecutor) rg09WorkflowProof {
	t.Helper()
	resources := newActionTestResourceHandlers(t, &config.Config{DataPath: dataPath})
	resources.SetStateProvider(resourceUnifiedSeedProvider{snapshot: models.StateSnapshot{LastUpdate: time.Now().UTC()}, resources: []unified.Resource{resource}})
	resources.SetActionExecutor(newRoutedActionExecutor(resources, executorFactory(resources)))
	aiHandler, patrol, _, _ := setupAIHandlerWithPatrol(t)
	if !patrol.GetFindings().Add(finding) {
		t.Fatal("deterministic APT finding was not admitted")
	}
	investigations := newTestInvestigationStore()
	investigation := investigations.Create(finding.ID, "rg09-"+capability)
	aiHandler.investigationStores = map[string]aicontracts.InvestigationStore{"default": investigations}
	aiHandler.SetResourceStoreProvider(resources.getStore)
	resources.SetActionTransitionPublisher(aiHandler.ReconcilePatrolActionTransition)
	proposal := aicontracts.ActionProposal{
		ProposalID: "rg09-proposal-" + finding.ID, FindingID: finding.ID, InvestigationID: investigation.ID,
		ResourceID: finding.ResourceID, CapabilityName: capability, Params: map[string]any{},
		Reason:      "Execute the exact typed APT workflow certified by the disposable RG-09 lab.",
		EvidenceIDs: []string{"finding-evidence:" + finding.ID},
	}
	disposition, err := NewPatrolActionBroker("default", resources).Submit(context.Background(), proposal)
	if err != nil || disposition.State != string(unified.ActionStatePending) || !disposition.Plan.RequiresApproval {
		t.Fatalf("planned disposition=%#v err=%v", disposition, err)
	}
	decision := httptest.NewRecorder()
	decisionReq := httptest.NewRequest(http.MethodPost, "/api/actions/"+disposition.ActionID+"/decision", bytes.NewBufferString(`{"outcome":"approved","reason":"RG-09 disposable lab authorization"}`))
	decisionReq.SetPathValue("id", disposition.ActionID)
	decisionReq = decisionReq.WithContext(auth.WithUser(decisionReq.Context(), "rg09-operator"))
	resources.HandleDecideAction(decision, actionHandlerTestRequest(decisionReq, ""))
	if decision.Code != http.StatusOK {
		t.Fatalf("decision status=%d body=%s", decision.Code, decision.Body.String())
	}
	execution := httptest.NewRecorder()
	executionReq := httptest.NewRequest(http.MethodPost, "/api/actions/"+disposition.ActionID+"/execute", bytes.NewBufferString(`{"reason":"execute authorized RG-09 typed action"}`))
	executionReq.SetPathValue("id", disposition.ActionID)
	executionReq = executionReq.WithContext(auth.WithUser(executionReq.Context(), "rg09-operator"))
	resources.HandleExecuteAction(execution, actionHandlerTestRequest(executionReq, ""))
	if execution.Code != http.StatusOK {
		t.Fatalf("execution status=%d body=%s", execution.Code, execution.Body.String())
	}
	store, err := resources.getStore("default")
	if err != nil {
		t.Fatal(err)
	}
	audit, found, err := store.GetActionAudit(disposition.ActionID)
	if err != nil || !found || audit.State != unified.ActionStateCompleted || audit.Result == nil || audit.Result.ActionResultV2 == nil {
		t.Fatalf("terminal audit found=%v err=%v audit=%#v", found, err, audit)
	}
	truth := audit.Result.ActionResultV2
	if truth.Execution.Status != unified.ActionExecutionSucceeded || truth.Verification.Status != unified.ActionVerificationConfirmed || truth.Verification.EvidenceClass != unified.ActionEvidenceAgentAttested {
		t.Fatalf("terminal action truth=%#v", truth)
	}
	attempt, found, err := store.GetActionDispatchAttempt(disposition.ActionID)
	if err != nil || !found || attempt.DispatchCount != 1 {
		t.Fatalf("dispatch attempt found=%v err=%v attempt=%#v", found, err, attempt)
	}
	receipt, found, err := store.GetActionDispatchReceipt(attempt.ID)
	if err != nil || !found || receipt.TransportRequestID != attempt.ID {
		t.Fatalf("dispatch receipt found=%v err=%v receipt=%#v", found, err, receipt)
	}
	reconciled := patrol.GetFindings().Get(finding.ID)
	if reconciled == nil || reconciled.ResolvedAt == nil || reconciled.InvestigationOutcome != string(aicontracts.OutcomeFixVerified) {
		t.Fatalf("finding reconciliation=%#v", reconciled)
	}
	projection, err := json.Marshal(struct {
		Audit   unified.ActionAuditRecord `json:"audit"`
		Finding *ai.Finding               `json:"finding"`
	}{Audit: audit, Finding: reconciled})
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"pulse-rg09-fixture", "/var/cache/apt", "apt-get", "stderr"} {
		if strings.Contains(strings.ToLower(string(projection)), strings.ToLower(forbidden)) {
			t.Fatalf("raw APT detail %q escaped audit/finding projection: %s", forbidden, projection)
		}
	}
	return rg09WorkflowProof{ActionID: audit.ID, AttemptID: attempt.ID, DispatchCount: attempt.DispatchCount, EvidenceClass: truth.Verification.EvidenceClass, FindingID: finding.ID, FindingOutcome: reconciled.InvestigationOutcome, Resolved: true}
}

func rg09UpdateDriftBarrier(t *testing.T, server *rg09Server, agentID, containerID, expectedHash, distro string) map[string]any {
	t.Helper()
	rg09DockerExec(t, containerID, "/usr/local/bin/rg09-set-repo", "3")
	defer rg09DockerExec(t, containerID, "/usr/local/bin/rg09-set-repo", "2")
	req := agentexec.HostUpdatePayload{RequestID: "rg09-" + distro + "-inventory-drift.dispatch.1", ActionID: "rg09-" + distro + "-inventory-drift", Operation: agentexec.HostUpdateOperationInstall, ExpectedInventoryHash: expectedHash, Timeout: 60}
	if err := agentexec.BindHostUpdatePayload(&req); err != nil {
		t.Fatal(err)
	}
	result, err := server.ExecuteHostUpdate(context.Background(), agentID, req)
	if err != nil || result == nil || result.Success || result.MutationStarted || result.Verification != agentexec.HostUpdateVerificationInconclusive || rg09InstalledVersion(t, containerID) != "1" {
		t.Fatalf("inventory drift result=%#v err=%v", result, err)
	}
	return map[string]any{"name": "inventory_drift", "mutation_started": result.MutationStarted, "verification": result.Verification, "installed_version": "1", "physical_state_unchanged": true}
}

func rg09RefreshFailureBarrier(t *testing.T, server *rg09Server, agentID, containerID, expectedHash, distro string) map[string]any {
	t.Helper()
	rg09DockerExec(t, containerID, "/usr/local/bin/rg09-break-repo")
	defer rg09DockerExec(t, containerID, "/usr/local/bin/rg09-set-repo", "2")
	req := agentexec.HostUpdatePayload{RequestID: "rg09-" + distro + "-refresh-failure.dispatch.1", ActionID: "rg09-" + distro + "-refresh-failure", Operation: agentexec.HostUpdateOperationInstall, ExpectedInventoryHash: expectedHash, Timeout: 60}
	if err := agentexec.BindHostUpdatePayload(&req); err != nil {
		t.Fatal(err)
	}
	result, err := server.ExecuteHostUpdate(context.Background(), agentID, req)
	if err != nil || result == nil || result.Success || result.MutationStarted || result.ExecutionPhase != agentexec.HostUpdatePhaseRefresh || result.Verification != agentexec.HostUpdateVerificationInconclusive || rg09InstalledVersion(t, containerID) != "1" {
		t.Fatalf("refresh failure result=%#v err=%v", result, err)
	}
	return map[string]any{"name": "refresh_failure", "phase": result.ExecutionPhase, "mutation_started": result.MutationStarted, "verification": result.Verification, "installed_version": "1", "physical_state_unchanged": true}
}

func rg09CleanupDriftBarrier(t *testing.T, server *rg09Server, agentID, containerID, expectedFingerprint, distro string) map[string]any {
	t.Helper()
	before := rg09IndependentReadback(t, containerID)
	rg09DockerExec(t, containerID, "sh", "-c", "dd if=/dev/zero of=/var/cache/apt/archives/rg09-drift.deb bs=1M count=1 status=none && touch -d @1 /var/cache/apt/archives/rg09-drift.deb")
	defer rg09DockerExec(t, containerID, "rm", "-f", "/var/cache/apt/archives/rg09-drift.deb")
	req := agentexec.HostStorageCleanupPayload{RequestID: "rg09-" + distro + "-cache-drift.dispatch.1", ActionID: "rg09-" + distro + "-cache-drift", Operation: agentexec.HostStorageCleanupOperationPackageCache, ExpectedFingerprint: expectedFingerprint, Timeout: 60}
	if err := agentexec.BindHostStorageCleanupPayload(&req); err != nil {
		t.Fatal(err)
	}
	result, err := server.ExecuteHostStorageCleanup(context.Background(), agentID, req)
	after := rg09IndependentReadback(t, containerID)
	if err != nil || result == nil || result.Success || result.MutationStarted || result.Verification != agentexec.HostStorageCleanupVerificationInconclusive || after.CacheArchiveBytes <= before.CacheArchiveBytes {
		t.Fatalf("cleanup drift result=%#v before=%#v after=%#v err=%v", result, before, after, err)
	}
	return map[string]any{"name": "cache_fingerprint_drift", "mutation_started": result.MutationStarted, "verification": result.Verification, "deliberate_fixture_drift_observed": true, "action_mutation": false, "preexisting_cache_preserved": true}
}

type rg09Readback struct {
	InstalledVersion      string `json:"installed_version"`
	PendingFixtureUpgrade bool   `json:"pending_fixture_upgrade"`
	CacheArchiveBytes     int64  `json:"cache_archive_bytes"`
}

func rg09IndependentReadback(t *testing.T, containerID string) rg09Readback {
	t.Helper()
	version := rg09InstalledVersion(t, containerID)
	simulation := rg09DockerExec(t, containerID, "apt-get", "-s", "-o", "Debug::NoLocking=1", "upgrade")
	bytesText := rg09DockerExec(t, containerID, "sh", "-c", "find /var/cache/apt/archives -maxdepth 1 -type f -name '*.deb' -printf '%s\\n' | awk '{s+=$1} END {print s+0}'")
	var archiveBytes int64
	if _, err := fmt.Sscan(strings.TrimSpace(bytesText), &archiveBytes); err != nil {
		t.Fatal(err)
	}
	return rg09Readback{InstalledVersion: version, PendingFixtureUpgrade: strings.Contains(simulation, "Inst pulse-rg09-fixture"), CacheArchiveBytes: archiveBytes}
}

func rg09InstalledVersion(t *testing.T, containerID string) string {
	t.Helper()
	return strings.TrimSpace(rg09DockerExec(t, containerID, "dpkg-query", "-W", "-f=${Version}", "pulse-rg09-fixture"))
}

func rg09DockerExec(t *testing.T, containerID string, args ...string) string {
	t.Helper()
	commandArgs := append([]string{"--context", "colima", "exec", containerID}, args...)
	output, err := exec.Command("docker", commandArgs...).CombinedOutput()
	if err != nil {
		t.Fatalf("docker exec %q: %v: %s", args, err, strings.TrimSpace(string(output)))
	}
	return string(output)
}

func startRG09Agent(t *testing.T, runID, distro, agentID, image, binary, scratchDir string, port int) string {
	t.Helper()
	name := fmt.Sprintf("pulse-rg09-%s-%s", distro, runID)
	args := []string{"--context", "colima", "create", "--name", name,
		"--label", "com.pulse.intelligence-lab.run=" + runID,
		"--label", "com.pulse.intelligence-lab.gate=rg-09",
		"--privileged", "--add-host", "host.docker.internal:host-gateway",
		"-e", fmt.Sprintf("PULSE_URL=http://host.docker.internal:%d", port),
		"-e", "PULSE_TOKEN=rg09-local-lab", "-e", "PULSE_AGENT_ID=" + agentID,
		"-e", "PULSE_HOSTNAME=" + name, "-e", "PULSE_ENABLE_HOST=true",
		"-e", "PULSE_ENABLE_COMMANDS=true", "-e", "PULSE_INTERVAL=2s", "-e", "PULSE_HEALTH_ADDR=off",
		image, "/bin/sh", "/usr/local/bin/rg09-entrypoint.sh"}
	output, err := exec.Command("docker", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("create RG-09 agent: %v: %s", err, strings.TrimSpace(string(output)))
	}
	containerID := strings.TrimSpace(string(output))
	scriptPath := filepath.Join(scratchDir, "rg09-entrypoint-"+distro+".sh")
	if err := os.WriteFile(scriptPath, []byte(rg09EntrypointScript), 0o700); err != nil {
		stopRG09Agent(containerID)
		t.Fatal(err)
	}
	for source, target := range map[string]string{binary: "/usr/local/bin/pulse-agent", scriptPath: "/usr/local/bin/rg09-entrypoint.sh"} {
		copyOutput, copyErr := exec.Command("docker", "--context", "colima", "cp", source, containerID+":"+target).CombinedOutput()
		if copyErr != nil {
			stopRG09Agent(containerID)
			t.Fatalf("copy RG-09 fixture: %v: %s", copyErr, strings.TrimSpace(string(copyOutput)))
		}
	}
	startOutput, err := exec.Command("docker", "--context", "colima", "start", containerID).CombinedOutput()
	if err != nil {
		logs, _ := exec.Command("docker", "--context", "colima", "logs", containerID).CombinedOutput()
		stopRG09Agent(containerID)
		t.Fatalf("start RG-09 agent: %v: %s logs=%s", err, strings.TrimSpace(string(startOutput)), strings.TrimSpace(string(logs)))
	}
	return containerID
}

func stopRG09Agent(containerID string) {
	if strings.TrimSpace(containerID) != "" {
		_ = exec.Command("docker", "--context", "colima", "rm", "-f", containerID).Run()
	}
}

var rg09SensitiveLogValue = regexp.MustCompile(`(?i)((?:authorization|api[_-]?token|password|secret)["' ]*[:=]["' ]*)[^"'\s,}]+`)

func rg09AgentFailureDiagnostic(containerID string) string {
	output, err := exec.Command("docker", "--context", "colima", "logs", "--tail", "200", containerID).CombinedOutput()
	diagnostic := string(output)
	if err != nil {
		diagnostic += "\n[container log read failed]"
	}
	return rg09RedactAgentDiagnostic(diagnostic)
}

func rg09RedactAgentDiagnostic(value string) string {
	const maxDiagnosticBytes = 16 << 10
	if len(value) > maxDiagnosticBytes {
		value = value[len(value)-maxDiagnosticBytes:]
	}
	value = rg09SensitiveLogValue.ReplaceAllString(value, `${1}[REDACTED]`)
	for old, replacement := range map[string]string{
		"rg09-local-lab":     "[REDACTED]",
		"pulse-rg09-fixture": "[fixture-package]",
		"/var/cache/apt":     "[package-cache-path]",
		"apt-get":            "[package-manager-command]",
	} {
		value = strings.ReplaceAll(value, old, replacement)
	}
	if len(value) > maxDiagnosticBytes {
		value = value[len(value)-maxDiagnosticBytes:]
	}
	return strings.TrimSpace(value)
}

func TestRG09AgentFailureDiagnosticIsBoundedAndRedacted(t *testing.T) {
	raw := strings.Repeat("x", 17<<10) + ` authorization: Bearer-value api_token=token-value rg09-local-lab pulse-rg09-fixture /var/cache/apt apt-get`
	got := rg09RedactAgentDiagnostic(raw)
	if len(got) > 16<<10 || strings.Contains(got, "Bearer-value") || strings.Contains(got, "token-value") || strings.Contains(got, "rg09-local-lab") || strings.Contains(got, "pulse-rg09-fixture") || strings.Contains(got, "/var/cache/apt") || strings.Contains(got, "apt-get") {
		t.Fatalf("failure diagnostic was not bounded and redacted: %q", got)
	}
}

func restartRG09Agent(t *testing.T, server *agentexec.Server, agentID, containerID string, reports <-chan agentshost.Report) {
	t.Helper()
	output, err := exec.Command("docker", "--context", "colima", "restart", containerID).CombinedOutput()
	if err != nil {
		t.Fatalf("restart RG-09 agent: %v: %s", err, strings.TrimSpace(string(output)))
	}
	for {
		select {
		case <-reports:
		default:
			waitForRG06Agent(t, server, agentID)
			return
		}
	}
}

const rg09EntrypointScript = `#!/bin/sh
set -eu
state=/var/lib/pulse-rg09
repo=/opt/pulse-rg09-repo
if [ ! -f "$state/ready" ]; then
  mkdir -p "$state" "$repo" /tmp/rg09-packages /var/cache/apt/archives
  printf '%s\n' "$PULSE_AGENT_ID" > /etc/machine-id
  rm -f /etc/apt/sources.list
  rm -f /etc/apt/sources.list.d/*
  make_package() {
    version="$1"
    root="/tmp/rg09-packages/root-$version"
    rm -rf "$root"
    mkdir -p "$root/DEBIAN" "$root/usr/share/pulse-rg09"
    cat > "$root/DEBIAN/control" <<EOF
Package: pulse-rg09-fixture
Version: $version
Section: misc
Priority: optional
Architecture: all
Maintainer: Pulse RG-09 Lab
Description: inert bounded package used only by the disposable RG-09 lab
EOF
    printf '%s\n' "$version" > "$root/usr/share/pulse-rg09/version"
    dpkg-deb --build "$root" "/tmp/rg09-packages/pulse-rg09-fixture_${version}_all.deb" >/dev/null
  }
  make_package 1
  make_package 2
  make_package 3
  dpkg -i /tmp/rg09-packages/pulse-rg09-fixture_1_all.deb >/dev/null
  cp /tmp/rg09-packages/pulse-rg09-fixture_2_all.deb /tmp/rg09-packages/pulse-rg09-fixture_3_all.deb "$repo/"
  cat > /usr/local/bin/rg09-set-repo <<'SCRIPT'
#!/bin/sh
set -eu
version="$1"
repo=/opt/pulse-rg09-repo
package="$repo/pulse-rg09-fixture_${version}_all.deb"
test -f "$package"
size=$(stat -c %s "$package")
md5=$(md5sum "$package" | awk '{print $1}')
sha=$(sha256sum "$package" | awk '{print $1}')
cat > "$repo/Packages" <<EOF
Package: pulse-rg09-fixture
Version: $version
Architecture: all
Maintainer: Pulse RG-09 Lab
Filename: pulse-rg09-fixture_${version}_all.deb
Size: $size
MD5sum: $md5
SHA256: $sha
Description: inert bounded package used only by the disposable RG-09 lab

EOF
printf 'deb [trusted=yes] file:%s ./\n' "$repo" > /etc/apt/sources.list.d/pulse-rg09.list
apt-get update -o Acquire::Languages=none >/dev/null
SCRIPT
  cat > /usr/local/bin/rg09-break-repo <<'SCRIPT'
#!/bin/sh
set -eu
printf 'deb [trusted=yes] file:/opt/pulse-rg09-missing ./\n' > /etc/apt/sources.list.d/pulse-rg09.list
SCRIPT
  chmod 0755 /usr/local/bin/rg09-set-repo /usr/local/bin/rg09-break-repo /usr/local/bin/pulse-agent
  /usr/local/bin/rg09-set-repo 2
  dd if=/dev/zero of="$state/cache.img" bs=1M count=96 status=none
  mkfs.ext4 -q -F "$state/cache.img"
  mount -o loop "$state/cache.img" /var/cache/apt/archives
  dd if=/dev/zero of=/var/cache/apt/archives/rg09-cache-fixture.deb bs=1M count=80 status=none
  touch -d @0 /var/cache/apt/archives/rg09-cache-fixture.deb
  touch "$state/ready"
fi
grep -qs ' /var/cache/apt/archives ' /proc/mounts || mount -o loop "$state/cache.img" /var/cache/apt/archives
exec /usr/local/bin/pulse-agent
`
