//go:build !windows

package installtests

// This file is a standalone rootful-container qualification packet. The live
// test is opt-in and must run only inside the disposable Ubuntu/systemd hosts
// created by scripts/run-secure-runtime-rootful-qualification.sh.

import (
	"context"
	"debug/buildinfo"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agenthelper"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

const (
	rootfulQualOptIn       = "PULSE_SECURE_RUNTIME_ROOTFUL_QUALIFICATION"
	rootfulQualOptInValue  = "disposable-v1"
	rootfulQualMarker      = "/etc/pulse-secure-runtime-rootful-qualification"
	rootfulQualReceiptPath = "/opt/pulse/result/rootful-receipt.json"
	rootfulQualResultDir   = "/opt/pulse/result"
	rootfulQualFixture     = "pulse-rootful-qualification-fixture:v1"
	rootfulQualRunningName = "pulse-rootful-running"
	rootfulQualExitedName  = "pulse-rootful-exited"
	rootfulQualHelperSock  = "/run/pulse-agent/helper.sock"
	rootfulQualBoundProbe  = "/usr/local/libexec/pulse-rootful-qualification/dockeragent.test"
)

var (
	rootfulQualBaseImagePattern = regexp.MustCompile(`^ubuntu@sha256:[0-9a-f]{64}$`)
	rootfulQualScenarioOrder    = []string{
		"fresh_install",
		"legacy_migration",
		"collector_restart",
		"helper_restart",
		"helper_loss",
		"helper_recovery",
		"operation_bounds",
		"update_preservation",
		"authority_isolation",
		"cleanup",
	}
)

type rootfulQualReceipt struct {
	SchemaVersion int                   `json:"schema_version"`
	Kind          string                `json:"kind"`
	Result        string                `json:"result"`
	SourceCommit  string                `json:"source_commit"`
	BaseImage     string                `json:"base_image"`
	StartedAt     string                `json:"started_at"`
	CompletedAt   string                `json:"completed_at"`
	SourceHashes  map[string]string     `json:"source_hashes"`
	Artifacts     rootlessQualArtifacts `json:"artifacts"`
	Runs          []rootfulQualRun      `json:"runs"`
}

type rootfulQualRun struct {
	Host      rootlessQualHost       `json:"host"`
	Runtime   rootfulQualRuntime     `json:"runtime"`
	Scenarios []rootlessQualScenario `json:"scenarios"`
}

type rootfulQualRuntime struct {
	Runtime        string `json:"runtime"`
	RuntimeVersion string `json:"runtime_version"`
	DaemonID       string `json:"daemon_id"`
	DaemonRootless bool   `json:"daemon_rootless"`
	SocketPath     string `json:"socket_path"`
	SocketUID      int    `json:"socket_uid"`
	SocketGID      int    `json:"socket_gid"`
	SocketMode     string `json:"socket_mode"`
	SocketType     string `json:"socket_type"`
	SocketSymlink  bool   `json:"socket_symlink"`
}

type rootfulQualDaemon struct {
	runtime  string
	unit     string
	socket   string
	dataRoot string
	runRoot  string
	fixture  string
}

func TestSecureRuntimeRootfulQualification(t *testing.T) {
	if os.Getenv(rootfulQualOptIn) != rootfulQualOptInValue {
		t.Skip("run through scripts/run-secure-runtime-rootful-qualification.sh inside its disposable systemd container")
	}
	runtimeKind := strings.TrimSpace(os.Getenv("PULSE_ROOTFUL_RUNTIME"))
	receiptPath := strings.TrimSpace(os.Getenv("PULSE_ROOTFUL_RECEIPT"))
	rootfulQualRequireDisposableHost(t, runtimeKind, receiptPath)
	rootfulQualAssertSystemContainerdDisabled(t)

	collector := secureRuntimeReadArtifact(t, "PULSE_SECURE_RUNTIME_COLLECTOR")
	collectorSignature := secureRuntimeReadSignature(t, "PULSE_SECURE_RUNTIME_COLLECTOR_SIGNATURE")
	helper := secureRuntimeReadArtifact(t, "PULSE_SECURE_RUNTIME_HELPER")
	collectorVersion := secureRuntimeArtifactVersion(t, "PULSE_SECURE_RUNTIME_COLLECTOR")
	installerPath := strings.TrimSpace(os.Getenv("PULSE_SECURE_RUNTIME_INSTALLER"))
	if !filepath.IsAbs(installerPath) {
		t.Fatalf("PULSE_SECURE_RUNTIME_INSTALLER must be absolute: %q", installerPath)
	}

	started := time.Now().UTC()
	fixture := newSecureRuntimeLabFixture(collector, collectorSignature, helper, nil, collectorVersion)
	defer fixture.actionServer.Shutdown()
	server := httptestNewServer(t, fixture)
	defer server.Close()
	collectorCredential := secureRuntimeLabToken

	daemon := rootfulQualDaemonFor(runtimeKind)
	defer rootlessQualBestEffortStop(daemon.unit, rootfulQualHungUnit(runtimeKind))
	rootfulQualPrepareFixture(t, daemon)
	rootfulQualStartDaemon(t, daemon)
	rootfulQualCreateFixtures(t, daemon)
	baseline := rootfulQualRuntimeBaseline(t, daemon)
	if baseline.Count != 2 {
		t.Fatalf("rootful %s baseline count = %d, want 2", runtimeKind, baseline.Count)
	}
	runtimeVersion := rootfulQualRuntimeVersion(t, daemon)
	daemonID := rootfulQualDaemonID(t, daemon)
	socketUID, socketGID, socketMode := rootfulQualSocketIdentity(t, daemon.socket)
	if socketUID != 0 {
		t.Fatalf("rootful runtime socket UID = %d, want 0", socketUID)
	}

	var scenarios []rootlessQualScenario
	appendScenario := func(name string, began time.Time, report *agentsdocker.Report, evidence map[string]any) {
		scenario := rootlessQualScenario{
			Name: name, Result: "passed", StartedAt: began.Format(time.RFC3339Nano),
			CompletedAt: time.Now().UTC().Format(time.RFC3339Nano), Evidence: evidence,
		}
		if report != nil {
			stream, sequence, ok := agentshost.ParseReportSequenceID(report.SequenceID)
			if !ok {
				t.Fatalf("scenario %s received invalid sequence ID %q", name, report.SequenceID)
			}
			scenario.ReportStreamID = &stream
			scenario.ReportSequence = &sequence
		}
		scenarios = append(scenarios, scenario)
	}

	freshStarted := time.Now().UTC()
	secureRuntimeRunInstaller(t, installerPath, server.URL,
		"--least-privilege", "--enable-privileged-helper", "--enable-docker")
	fresh := rootfulQualWaitSummary(t, fixture, freshStarted, runtimeKind, baseline.SemanticDigest, 75*time.Second)
	secureRuntimeAssertSafeProfile(t)
	secureRuntimeAssertHelperProtocol(t)
	rootfulQualAssertCollectorSocketDenied(t, daemon.socket)
	freshPID := secureRuntimeCollectorMainPID(t)
	freshHelperPID, _ := rootfulQualUnitIdentity(t, "pulse-agent-helper.service")
	appendScenario("fresh_install", freshStarted, &fresh.Report,
		rootfulQualSummaryEvidence(freshPID, freshHelperPID, daemonID, fresh.Report))

	rootlessQualUninstallPulse(t, installerPath, server.URL, collectorCredential)
	rootlessQualAssertPulseRemoved(t)
	if registered, revoked, uninstalls := fixture.collectorLifecycleSnapshot(); registered || !revoked || uninstalls != 1 {
		t.Fatalf("fresh collector uninstall was not durably modeled: registered=%t revoked=%t uninstalls=%d", registered, revoked, uninstalls)
	}
	fixture.replaceCollectorCredential(secureRuntimeLabTokenV2, secureRuntimeCollectorBindingV2)
	collectorCredential = secureRuntimeLabTokenV2

	migrationStarted := time.Now().UTC()
	secureRuntimeRunInstallerWithCollectorCredential(t, installerPath, server.URL, collectorCredential,
		"--enable-commands", "--command-authority", "command-capable", "--enable-docker")
	legacy := rootlessQualWaitReport(t, fixture, migrationStarted, 75*time.Second, func(report agentsdocker.Report) bool {
		return rootlessQualComplete(report) && report.Host.CollectionMode == "" && report.Host.Runtime == runtimeKind && rootlessQualSemanticDigest(report) == baseline.SemanticDigest
	})
	secureRuntimeAssertRootCommandProfile(t)
	legacyPID := secureRuntimeCollectorMainPID(t)
	applyStarted := time.Now().UTC()
	secureRuntimeRunInstallerWithCollectorCredential(t, installerPath, server.URL, collectorCredential, "--safe-profile-apply")
	migrated := rootfulQualWaitSummary(t, fixture, applyStarted, runtimeKind, baseline.SemanticDigest, 75*time.Second)
	secureRuntimeAssertSafeProfile(t)
	secureRuntimeAssertHelperProtocol(t)
	rootfulQualAssertCollectorSocketDenied(t, daemon.socket)
	migratedPID := secureRuntimeCollectorMainPID(t)
	if legacyPID == migratedPID || fixture.authorityReductionCount() < 1 || rootlessQualSemanticDigest(legacy.Report) != rootlessQualSemanticDigest(migrated.Report) {
		t.Fatalf("rootful migration did not replace/reduce the collector with summary parity")
	}
	migratedHelperPID, _ := rootfulQualUnitIdentity(t, "pulse-agent-helper.service")
	migrationEvidence := rootfulQualSummaryEvidence(migratedPID, migratedHelperPID, daemonID, migrated.Report)
	migrationEvidence["legacy_profile"] = "root-command-capable"
	migrationEvidence["target_profile"] = "typed-helper-monitoring-only"
	migrationEvidence["authority_reduced"] = true
	migrationEvidence["legacy_collector_pid"] = legacyPID
	appendScenario("legacy_migration", migrationStarted, &migrated.Report, migrationEvidence)

	collectorRestartStarted := time.Now().UTC()
	previousStream, _, _ := agentshost.ParseReportSequenceID(migrated.Report.SequenceID)
	secureRuntimeCommand(t, 20*time.Second, "systemctl", "restart", "pulse-agent.service")
	collectorPID := secureRuntimeCollectorMainPID(t)
	collectorRestart := rootfulQualWaitSummary(t, fixture, collectorRestartStarted, runtimeKind, baseline.SemanticDigest, 75*time.Second)
	collectorStream, _, _ := agentshost.ParseReportSequenceID(collectorRestart.Report.SequenceID)
	if collectorPID == migratedPID || collectorStream == previousStream {
		t.Fatalf("collector restart did not replace PID/report stream: pid=%d/%d stream=%s/%s", migratedPID, collectorPID, previousStream, collectorStream)
	}
	helperPID, _ := rootfulQualUnitIdentity(t, "pulse-agent-helper.service")
	collectorRestartEvidence := rootfulQualSummaryEvidence(collectorPID, helperPID, daemonID, collectorRestart.Report)
	collectorRestartEvidence["previous_collector_pid"] = migratedPID
	collectorRestartEvidence["previous_report_stream_id"] = previousStream
	appendScenario("collector_restart", collectorRestartStarted, &collectorRestart.Report, collectorRestartEvidence)

	helperPIDBefore, helperInvocationBefore := rootfulQualUnitIdentity(t, "pulse-agent-helper.service")
	secureRuntimeCommand(t, 20*time.Second, "systemctl", "restart", "pulse-agent-helper.service")
	helperRestartStarted := time.Now().UTC()
	helperPIDAfter, helperInvocationAfter := rootfulQualUnitIdentity(t, "pulse-agent-helper.service")
	if helperPIDBefore == helperPIDAfter || helperInvocationBefore == helperInvocationAfter {
		t.Fatalf("helper restart did not replace exact service identity")
	}
	helperRestart := rootfulQualWaitSummary(t, fixture, helperRestartStarted, runtimeKind, baseline.SemanticDigest, 75*time.Second)
	helperRestartEvidence := rootfulQualSummaryEvidence(collectorPID, helperPIDAfter, daemonID, helperRestart.Report)
	helperRestartEvidence["previous_helper_pid"] = helperPIDBefore
	helperRestartEvidence["previous_helper_invocation_id"] = helperInvocationBefore
	helperRestartEvidence["helper_invocation_id"] = helperInvocationAfter
	appendScenario("helper_restart", helperRestartStarted, &helperRestart.Report, helperRestartEvidence)

	lossStarted := time.Now().UTC()
	secureRuntimeCommand(t, 20*time.Second, "systemctl", "stop", "pulse-agent-helper.socket", "pulse-agent-helper.service")
	loss := rootfulQualWaitStatusOnly(t, fixture, lossStarted, runtimeKind, 75*time.Second)
	lossStream, lossSequence, _ := agentshost.ParseReportSequenceID(loss.Report.SequenceID)
	if len(loss.Report.Containers) != 0 || secureRuntimeCollectorMainPID(t) != collectorPID {
		t.Fatal("helper loss emitted an authoritative empty inventory or replaced the collector")
	}
	appendScenario("helper_loss", lossStarted, &loss.Report, map[string]any{
		"collector_pid": collectorPID, "previous_helper_pid": helperPIDAfter,
		"collection_mode": "typed-helper-unavailable-status-only", "helper_available": false,
		"status_only": true, "inventory_complete": false, "inventory_present": false,
		"authoritative_inventory_replacement":    false,
		"previous_authoritative_inventory_count": baseline.Count,
		"previous_authoritative_semantic_sha256": baseline.SemanticDigest,
		"operation_status":                       "degraded", "operation": agenthelper.OperationContainerInventory,
		"container_updates_enabled": false, "container_actions_enabled": false, "direct_socket_access": false,
	})

	recoveryStarted := time.Now().UTC()
	secureRuntimeCommand(t, 20*time.Second, "systemctl", "start", "pulse-agent-helper.socket")
	recovered := rootfulQualWaitSummary(t, fixture, recoveryStarted, runtimeKind, baseline.SemanticDigest, 75*time.Second)
	secureRuntimeAssertHelperProtocol(t)
	helperRecoveryPID, _ := rootfulQualUnitIdentity(t, "pulse-agent-helper.service")
	recoveryStream, recoverySequence, _ := agentshost.ParseReportSequenceID(recovered.Report.SequenceID)
	if recoveryStream != lossStream || recoverySequence <= lossSequence {
		t.Fatalf("helper recovery did not advance the same report stream")
	}
	recoveryEvidence := rootfulQualSummaryEvidence(collectorPID, helperRecoveryPID, daemonID, recovered.Report)
	recoveryEvidence["previous_helper_pid"] = helperPIDAfter
	recoveryEvidence["previous_status_report_sequence"] = lossSequence
	appendScenario("helper_recovery", recoveryStarted, &recovered.Report, recoveryEvidence)

	boundStarted := time.Now().UTC()
	rootfulQualStopDaemon(t, daemon)
	rootfulQualStartHungDaemon(t, daemon)
	probeElapsed := rootfulQualRunBoundProbe(t, 2*time.Second)
	probeCompletedAt := time.Now().UTC()
	boundStatus := rootfulQualWaitStatusOnly(t, fixture, probeCompletedAt, runtimeKind, 45*time.Second)
	boundStatusStream, boundStatusSequence, _ := agentshost.ParseReportSequenceID(boundStatus.Report.SequenceID)
	rootlessQualBestEffortStop(rootfulQualHungUnit(runtimeKind))
	_ = os.Remove(daemon.socket)
	rootfulQualStartDaemon(t, daemon)
	boundRecoveryStarted := time.Now().UTC()
	boundRecovery := rootfulQualWaitSummary(t, fixture, boundRecoveryStarted, runtimeKind, baseline.SemanticDigest, 75*time.Second)
	boundRecoveryStream, boundRecoverySequence, _ := agentshost.ParseReportSequenceID(boundRecovery.Report.SequenceID)
	if boundRecoveryStream != boundStatusStream || boundRecoverySequence <= boundStatusSequence {
		t.Fatalf("bounded operation recovery did not advance the same report stream")
	}
	appendScenario("operation_bounds", boundStarted, &boundRecovery.Report, map[string]any{
		"collector_pid": collectorPID, "helper_pid": helperRecoveryPID,
		"operation": agenthelper.OperationContainerInventory, "failure_class": "bounded-timeout",
		"collection_mode":    agentsdocker.CollectionModeTypedHelperSummary,
		"inventory_complete": true, "full_fields_present": false, "stats_present": false,
		"secondary_structure_sha256": "",
		"deadline_ms":                2000, "elapsed_ms": probeElapsed.Milliseconds(), "bounded_failure_observed": true,
		"status_only_report_sequence": boundStatusSequence, "recovery_report_sequence": boundRecoverySequence,
		"previous_authoritative_inventory_count": baseline.Count,
		"previous_authoritative_semantic_sha256": baseline.SemanticDigest,
		"recovery_inventory_count":               baseline.Count, "recovery_semantic_sha256": rootlessQualSemanticDigest(boundRecovery.Report),
		"authoritative_empty_replacement": false, "collector_alive": true, "helper_alive": true,
		"container_updates_enabled": false, "container_actions_enabled": false, "direct_socket_access": false,
	})

	updateStarted := time.Now().UTC()
	preUpdatePID := secureRuntimeCollectorMainPID(t)
	preUpdateStream, _, _ := agentshost.ParseReportSequenceID(boundRecovery.Report.SequenceID)
	secureRuntimeRunInstallerWithCollectorCredential(t, installerPath, server.URL, collectorCredential, "--update")
	postUpdatePID := secureRuntimeCollectorMainPID(t)
	updated := rootfulQualWaitSummary(t, fixture, updateStarted, runtimeKind, baseline.SemanticDigest, 90*time.Second)
	postUpdateStream, _, _ := agentshost.ParseReportSequenceID(updated.Report.SequenceID)
	if postUpdatePID == preUpdatePID || postUpdateStream == preUpdateStream {
		t.Fatalf("ordinary update did not restart the safe collector: pid=%d/%d stream=%s/%s", preUpdatePID, postUpdatePID, preUpdateStream, postUpdateStream)
	}
	postUpdateHelperPID, _ := rootfulQualUnitIdentity(t, "pulse-agent-helper.service")
	if postUpdateHelperPID != helperRecoveryPID {
		t.Fatalf("ordinary collector update replaced the independent helper process: pid=%d/%d", helperRecoveryPID, postUpdateHelperPID)
	}
	updateEvidence := rootfulQualSummaryEvidence(postUpdatePID, postUpdateHelperPID, daemonID, updated.Report)
	updateEvidence["previous_collector_pid"] = preUpdatePID
	updateEvidence["previous_helper_pid"] = helperRecoveryPID
	updateEvidence["previous_report_stream_id"] = preUpdateStream
	updateEvidence["update_applied"] = true
	updateEvidence["collector_binary_sha256"] = secureRuntimeHash(secureRuntimeReadFile(t, "/usr/local/bin/pulse-agent"))
	updateEvidence["helper_binary_sha256"] = secureRuntimeHash(secureRuntimeReadFile(t, "/usr/local/lib/pulse-agent/pulse-agent-helper"))
	appendScenario("update_preservation", updateStarted, &updated.Report, updateEvidence)

	authorityStarted := time.Now().UTC()
	collectorUID := rootlessQualUID(t, "pulse-agent")
	groups := strings.Fields(rootlessQualCommand(t, 10*time.Second, "id", "-nG", "pulse-agent"))
	if slices.Contains(groups, "docker") || slices.Contains(groups, "podman") {
		t.Fatalf("safe collector retained a rootful daemon group: %v", groups)
	}
	rootfulQualAssertCollectorSocketDenied(t, daemon.socket)
	helperNetworkDenied := rootlessQualAssertHelperNetworkDenied(t)
	commandSessionPresent := fixture.actionServer.IsAgentConnectedForOrganization(secureRuntimeLabOrgID, secureRuntimeLabAgentID)
	if !helperNetworkDenied || commandSessionPresent || secureRuntimeCollectorHasArgument("--enable-commands") || secureRuntimeCollectorProcessUID(t) != collectorUID {
		t.Fatal("rootful authority isolation did not remain exact")
	}
	appendScenario("authority_isolation", authorityStarted, nil, map[string]any{
		"collector_pid": postUpdatePID, "collector_uid": collectorUID, "effective_uid": collectorUID,
		"effective_root": false, "safe_profile_enabled": true, "commands_enabled": false,
		"privileged_helper_enabled": true, "reduction_request_observed": true,
		"collector_command_transport_present": false, "collector_command_session_present": false,
		"container_actions_enabled": false, "container_updates_enabled": false,
		"rootful_socket_access": false, "direct_socket_access": false, "helper_network_access": false,
	})

	cleanupStarted := time.Now().UTC()
	rootlessQualUninstallPulse(t, installerPath, server.URL, collectorCredential)
	rootfulQualRemoveFixtures(t, daemon)
	rootfulQualStopDaemon(t, daemon)
	rootfulQualRemoveRuntimeState(t, daemon)
	rootlessQualAssertPulseRemoved(t)
	if registered, revoked, uninstalls := fixture.collectorLifecycleSnapshot(); registered || !revoked || uninstalls != 2 {
		t.Fatalf("final collector uninstall was not durably modeled: registered=%t revoked=%t uninstalls=%d", registered, revoked, uninstalls)
	}
	if _, err := os.Lstat(daemon.socket); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("cleanup left rootful runtime socket %s: %v", daemon.socket, err)
	}
	stateClean := rootfulQualRuntimeStateClean(daemon)
	if !stateClean {
		t.Fatal("cleanup left rootful runtime state")
	}
	appendScenario("cleanup", cleanupStarted, nil, map[string]any{
		"collector_stopped": true, "helper_stopped": true, "runtime_stopped": true,
		"socket_absent": true, "fixtures_removed": true, "state_clean": true,
	})

	receipt := rootfulQualReceipt{
		SchemaVersion: 1, Kind: "pulse-secure-runtime-rootful-qualification", Result: "passed",
		SourceCommit: strings.TrimSpace(os.Getenv("PULSE_ROOTFUL_SOURCE_COMMIT")),
		BaseImage:    strings.TrimSpace(os.Getenv("PULSE_ROOTFUL_UBUNTU_IMAGE")),
		StartedAt:    started.Format(time.RFC3339Nano), CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
		SourceHashes: rootfulQualSourceHashes(t), Artifacts: rootfulQualArtifactIdentities(t, installerPath),
		Runs: []rootfulQualRun{{
			Host: rootlessQualHost{
				MachineID: strings.TrimSpace(string(rootlessQualReadFile(t, "/etc/machine-id"))), Architecture: runtime.GOARCH,
				Kernel:         rootlessQualCommand(t, 10*time.Second, "uname", "-srvmo"),
				SystemdVersion: strings.SplitN(rootlessQualCommand(t, 10*time.Second, "systemctl", "--version"), "\n", 2)[0],
			},
			Runtime: rootfulQualRuntime{
				Runtime: runtimeKind, RuntimeVersion: runtimeVersion, DaemonID: daemonID, DaemonRootless: false,
				SocketPath: daemon.socket, SocketUID: socketUID, SocketGID: socketGID, SocketMode: socketMode,
				SocketType: "unix", SocketSymlink: false,
			},
			Scenarios: scenarios,
		}},
	}
	if err := rootfulQualValidateReceipt(receipt, 1); err != nil {
		t.Fatalf("generated rootful receipt failed validation: %v", err)
	}
	rootlessQualWriteJSON(t, receiptPath, receipt)
}

// TestSecureRuntimeRootfulBoundProbe is invoked as the installed collector UID
// by the live qualification test. It proves the real helper and provider honor
// a caller-supplied bounded deadline against an accepted but unresponsive
// rootful daemon connection.
func TestSecureRuntimeRootfulBoundProbe(t *testing.T) {
	if os.Getenv("PULSE_ROOTFUL_BOUND_PROBE") != "1" {
		t.Skip("internal rootful qualification subprocess")
	}
	deadlineMillis, err := strconv.Atoi(os.Getenv("PULSE_ROOTFUL_BOUND_DEADLINE_MS"))
	if err != nil || deadlineMillis < 1 {
		t.Fatalf("invalid bound-probe deadline: %v", err)
	}
	deadline := time.Duration(deadlineMillis) * time.Millisecond
	client, err := agenthelper.NewClient(agenthelper.ClientConfig{SocketPath: rootfulQualHelperSock, MaxDeadline: deadline})
	if err != nil {
		t.Fatal(err)
	}
	var response agenthelper.ContainerInventoryResult
	_, err = client.Call(context.Background(), agenthelper.OperationContainerInventory, agenthelper.OperationVersion1, deadline, struct{}{}, &response)
	var remote *agenthelper.RemoteError
	var networkError net.Error
	typedDeadline := errors.As(err, &remote) && remote.Code == agenthelper.ErrorDeadlineExceeded
	localDeadline := errors.As(err, &networkError) && networkError.Timeout()
	if !typedDeadline && !localDeadline {
		t.Fatalf("bounded helper operation error = %T %v", err, err)
	}
	fmt.Println("ROOTFUL_BOUND_RESULT=deadline_exceeded")
}

func httptestNewServer(t *testing.T, handler *secureRuntimeLabFixture) *httptest.Server {
	t.Helper()
	return httptest.NewServer(handler)
}

func rootfulQualRequireDisposableHost(t *testing.T, runtimeKind, receiptPath string) {
	t.Helper()
	if os.Geteuid() != 0 || (runtimeKind != "docker" && runtimeKind != "podman") {
		t.Fatalf("qualification requires root and PULSE_ROOTFUL_RUNTIME=docker|podman")
	}
	marker, err := os.ReadFile(rootfulQualMarker)
	if err != nil || strings.TrimSpace(string(marker)) != rootfulQualOptInValue {
		t.Fatalf("disposable marker is absent or invalid: %v", err)
	}
	if receiptPath != rootfulQualReceiptPath {
		t.Fatalf("PULSE_ROOTFUL_RECEIPT must use %q: %q", rootfulQualReceiptPath, receiptPath)
	}
	info, err := os.Lstat(rootfulQualResultDir)
	if err != nil {
		t.Fatal(err)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0o700 || stat.Uid != 0 {
		t.Fatalf("rootful result directory must be root-owned mode 0700: mode=%s stat=%#v", info.Mode(), info.Sys())
	}
	osRelease := string(rootlessQualReadFile(t, "/etc/os-release"))
	if !strings.Contains(osRelease, "VERSION_ID=\"24.04\"") && !strings.Contains(osRelease, "VERSION_ID=24.04") {
		t.Fatal("qualification host is not Ubuntu 24.04")
	}
	if _, err := os.Stat("/run/systemd/system"); err != nil {
		t.Fatalf("qualification host is not booted under systemd: %v", err)
	}
	if rootlessQualHasDefaultRoute(string(rootlessQualReadFile(t, "/proc/net/route"))) {
		t.Fatal("qualification workload must run with outer-container networking disabled")
	}
}

func rootfulQualDaemonFor(runtimeKind string) rootfulQualDaemon {
	if runtimeKind == "docker" {
		return rootfulQualDaemon{runtime: runtimeKind, unit: "pulse-rootful-docker", socket: "/var/run/docker.sock", dataRoot: "/var/lib/pulse-rootful-docker", runRoot: "/run/pulse-rootful-docker", fixture: "/opt/pulse/rootful-fixture"}
	}
	return rootfulQualDaemon{runtime: runtimeKind, unit: "pulse-rootful-podman", socket: "/run/podman/podman.sock", dataRoot: "/var/lib/pulse-rootful-podman", runRoot: "/run/pulse-rootful-podman", fixture: "/opt/pulse/rootful-fixture"}
}

func rootfulQualPrepareFixture(t *testing.T, daemon rootfulQualDaemon) {
	t.Helper()
	rootlessQualCommand(t, 10*time.Second, "install", "-d", "-o", "root", "-g", "root", "-m", "0700", daemon.fixture)
	containerfile := "FROM scratch\nCOPY busybox /busybox\nENTRYPOINT [\"/busybox\"]\n"
	if err := os.WriteFile(filepath.Join(daemon.fixture, "Containerfile"), []byte(containerfile), 0o600); err != nil {
		t.Fatal(err)
	}
	busybox, err := os.ReadFile("/bin/busybox")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(daemon.fixture, "busybox"), busybox, 0o700); err != nil {
		t.Fatal(err)
	}
}

func rootfulQualStartDaemon(t *testing.T, daemon rootfulQualDaemon) {
	t.Helper()
	rootlessQualBestEffortStop(daemon.unit)
	_ = os.Remove(daemon.socket)
	rootlessQualCommand(t, 10*time.Second, "install", "-d", "-o", "root", "-g", "root", "-m", "0755", filepath.Dir(daemon.socket))
	if daemon.runtime == "docker" {
		rootlessQualCommand(t, 20*time.Second, "systemd-run", "--quiet", "--collect", "--unit", daemon.unit, "--property=Type=exec", "--",
			"/usr/bin/dockerd", "--host=unix://"+daemon.socket, "--data-root="+daemon.dataRoot,
			"--exec-root=/run/pulse-rootful-docker", "--pidfile=/run/pulse-rootful-docker.pid", "--storage-driver=vfs", "--iptables=false", "--bridge=none")
	} else {
		rootlessQualCommand(t, 20*time.Second, "systemd-run", "--quiet", "--collect", "--unit", daemon.unit, "--property=Type=exec", "--",
			"/usr/bin/podman", "--storage-driver=vfs", "--root="+daemon.dataRoot, "--runroot="+daemon.runRoot,
			"system", "service", "--time=0", "unix://"+daemon.socket)
	}
	rootlessQualWaitSocket(t, daemon.socket)
	rootlessQualCommand(t, 10*time.Second, "chmod", "0660", daemon.socket)
	rootfulQualRuntimeCommand(t, daemon, 30*time.Second, "info")
	if driver := rootfulQualRuntimeStorageDriver(t, daemon); driver != "vfs" {
		t.Fatalf("rootful %s storage driver = %q, want vfs", daemon.runtime, driver)
	}
}

func rootfulQualRuntimeStorageDriver(t *testing.T, daemon rootfulQualDaemon) string {
	t.Helper()
	if daemon.runtime == "docker" {
		return rootfulQualRuntimeCommand(t, daemon, 30*time.Second, "info", "--format", "{{.Driver}}")
	}
	return rootfulQualRuntimeCommand(t, daemon, 30*time.Second, "info", "--format", "{{.Store.GraphDriverName}}")
}

func rootfulQualStopDaemon(t *testing.T, daemon rootfulQualDaemon) {
	t.Helper()
	rootlessQualStopUnit(t, daemon.unit)
	_ = os.Remove(daemon.socket)
	if daemon.runtime == "docker" {
		rootfulQualWaitNoContainerd(t, 20*time.Second)
	}
}

func rootfulQualAssertSystemContainerdDisabled(t *testing.T) {
	t.Helper()
	unitState := rootlessQualCommand(t, 10*time.Second, "systemctl", "show", "containerd.service", "--property=UnitFileState", "--value")
	activeState := rootlessQualCommand(t, 10*time.Second, "systemctl", "show", "containerd.service", "--property=ActiveState", "--value")
	if unitState != "masked" || activeState != "inactive" {
		t.Fatalf("distro containerd service must be masked and inactive: UnitFileState=%q ActiveState=%q", unitState, activeState)
	}
	rootfulQualWaitNoContainerd(t, 5*time.Second)
}

func rootfulQualWaitNoContainerd(t *testing.T, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		output, err := rootlessQualCommandError(3*time.Second, "pgrep", "-x", "containerd")
		if err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
				return
			}
			t.Fatalf("inspect containerd processes: %v\n%s", err, output)
		}
		if time.Now().After(deadline) {
			t.Fatalf("containerd process remained after explicit Docker daemon stop: %s", strings.TrimSpace(output))
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func rootfulQualRuntimeCommand(t *testing.T, daemon rootfulQualDaemon, timeout time.Duration, args ...string) string {
	t.Helper()
	if daemon.runtime == "docker" {
		return rootlessQualCommand(t, timeout, "docker", append([]string{"--host", "unix://" + daemon.socket}, args...)...)
	}
	return rootlessQualCommand(t, timeout, "podman", append([]string{"--url", "unix://" + daemon.socket}, args...)...)
}

func rootfulQualCreateFixtures(t *testing.T, daemon rootfulQualDaemon) {
	t.Helper()
	rootfulQualRuntimeCommand(t, daemon, 2*time.Minute, "build", "--network=none", "-t", rootfulQualFixture, "-f", filepath.Join(daemon.fixture, "Containerfile"), daemon.fixture)
	rootfulQualRuntimeCommand(t, daemon, 30*time.Second, "run", "-d", "--restart=always", "--name", rootfulQualRunningName, rootfulQualFixture, "sleep", "3600")
	if out, err := rootfulQualRuntimeCommandError(daemon, 30*time.Second, "run", "--name", rootfulQualExitedName, rootfulQualFixture, "true"); err != nil {
		t.Fatalf("create exited fixture: %v\n%s", err, out)
	}
}

func rootfulQualRemoveFixtures(t *testing.T, daemon rootfulQualDaemon) {
	t.Helper()
	for _, name := range []string{rootfulQualRunningName, rootfulQualExitedName} {
		_, _ = rootfulQualRuntimeCommandError(daemon, 30*time.Second, "rm", "-f", name)
	}
	_, _ = rootfulQualRuntimeCommandError(daemon, 30*time.Second, "rmi", "-f", rootfulQualFixture)
}

func rootfulQualRuntimeCommandError(daemon rootfulQualDaemon, timeout time.Duration, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	name := "podman"
	prefix := []string{"--url", "unix://" + daemon.socket}
	if daemon.runtime == "docker" {
		name = "docker"
		prefix = []string{"--host", "unix://" + daemon.socket}
	}
	output, err := exec.CommandContext(ctx, name, append(prefix, args...)...).CombinedOutput()
	return strings.TrimSpace(string(output)), err
}

func rootfulQualRuntimeBaseline(t *testing.T, daemon rootfulQualDaemon) rootlessQualBaseline {
	t.Helper()
	return rootlessQualBaselineFromPSOutput(rootfulQualRuntimeCommand(t, daemon, 30*time.Second, "ps", "-a", "--format", "{{.Names}}|{{.Image}}|{{.State}}"))
}

func rootfulQualRuntimeVersion(t *testing.T, daemon rootfulQualDaemon) string {
	t.Helper()
	return rootfulQualRuntimeCommand(t, daemon, 30*time.Second, "version", "--format", "{{.Server.Version}}")
}

func rootfulQualDaemonID(t *testing.T, daemon rootfulQualDaemon) string {
	t.Helper()
	d := rootlessQualDaemon{runtime: daemon.runtime, rootfulSock: daemon.socket}
	return rootlessQualDaemonID(t, d, false)
}

func rootfulQualSocketIdentity(t *testing.T, path string) (int, int, string) {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSocket == 0 || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0o660 {
		t.Fatalf("unsafe rootful runtime socket %s mode=%s", path, info.Mode())
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatalf("socket %s lacks Unix stat identity", path)
	}
	return int(stat.Uid), int(stat.Gid), fmt.Sprintf("%04o", info.Mode().Perm())
}

func rootfulQualWaitSummary(t *testing.T, fixture *secureRuntimeLabFixture, after time.Time, runtimeKind, digest string, timeout time.Duration) secureRuntimeDockerReport {
	t.Helper()
	report := rootlessQualWaitReport(t, fixture, after, timeout, func(report agentsdocker.Report) bool {
		return rootlessQualComplete(report) && report.InventoryComplete != nil && *report.InventoryComplete &&
			report.Host.CollectionMode == agentsdocker.CollectionModeTypedHelperSummary && report.Host.Runtime == runtimeKind &&
			rootlessQualSemanticDigest(report) == digest && len(report.Containers) > 0
	})
	rootlessQualAssertHelperSummaryOnly(t, report.Report, runtimeKind)
	return report
}

func rootfulQualWaitStatusOnly(t *testing.T, fixture *secureRuntimeLabFixture, after time.Time, runtimeKind string, timeout time.Duration) secureRuntimeDockerReport {
	t.Helper()
	return rootlessQualWaitReport(t, fixture, after, timeout, func(report agentsdocker.Report) bool {
		return report.InventoryComplete != nil && !*report.InventoryComplete && report.Host.Runtime == runtimeKind &&
			report.Host.CollectionMode == agentsdocker.CollectionModeTypedHelperSummary && len(report.Containers) == 0 &&
			secureRuntimeDockerHelperModuleState(report) == "degraded"
	})
}

func rootfulQualSummaryEvidence(collectorPID, helperPID int, daemonID string, report agentsdocker.Report) map[string]any {
	digest := rootlessQualDigestReport(report)
	return map[string]any{
		"collector_pid": collectorPID, "helper_pid": helperPID,
		"collection_mode":    agentsdocker.CollectionModeTypedHelperSummary,
		"inventory_complete": true, "inventory_count": digest.Count,
		"semantic_sha256": digest.SemanticDigest, "full_fields_present": false,
		"stats_present": false, "secondary_structure_sha256": "",
		"container_updates_enabled": false, "container_actions_enabled": false,
		"direct_socket_access": false, "daemon_id": daemonID, "daemon_rootless": false,
	}
}

func rootfulQualUnitIdentity(t *testing.T, unit string) (int, string) {
	t.Helper()
	pidText := rootlessQualCommand(t, 10*time.Second, "systemctl", "show", unit, "--property=MainPID", "--value")
	pid, err := strconv.Atoi(pidText)
	if err != nil || pid <= 0 {
		t.Fatalf("invalid %s MainPID %q", unit, pidText)
	}
	invocation := rootlessQualCommand(t, 10*time.Second, "systemctl", "show", unit, "--property=InvocationID", "--value")
	if len(invocation) != 32 {
		t.Fatalf("invalid %s InvocationID %q", unit, invocation)
	}
	return pid, invocation
}

func rootfulQualAssertCollectorSocketDenied(t *testing.T, socket string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "runuser", "-u", "pulse-agent", "--", "curl", "-fsS", "--max-time", "2", "--unix-socket", socket, "http://runtime/_ping")
	if output, err := command.CombinedOutput(); err == nil {
		t.Fatalf("safe collector unexpectedly reached rootful socket %s: %s", socket, strings.TrimSpace(string(output)))
	}
}

func rootfulQualHungUnit(runtimeKind string) string { return "pulse-rootful-" + runtimeKind + "-hung" }

func rootfulQualStartHungDaemon(t *testing.T, daemon rootfulQualDaemon) {
	t.Helper()
	scriptPath := filepath.Join(daemon.fixture, "hung-runtime.py")
	script := `import os, socket, threading, time, sys
path = sys.argv[1]
try: os.unlink(path)
except FileNotFoundError: pass
s = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
s.bind(path)
os.chmod(path, 0o660)
s.listen(64)
def hold(c):
    try: time.sleep(120)
    finally: c.close()
while True:
    c, _ = s.accept()
    threading.Thread(target=hold, args=(c,), daemon=True).start()
`
	if err := os.WriteFile(scriptPath, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	rootlessQualCommand(t, 20*time.Second, "systemd-run", "--quiet", "--collect", "--unit", rootfulQualHungUnit(daemon.runtime), "--property=Type=exec", "--",
		"/usr/bin/python3", scriptPath, daemon.socket)
	rootlessQualWaitSocket(t, daemon.socket)
}

func rootfulQualRunBoundProbe(t *testing.T, deadline time.Duration) time.Duration {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	boundProbe := strings.TrimSpace(os.Getenv("PULSE_ROOTFUL_BOUND_PROBE_BINARY"))
	if boundProbe != rootfulQualBoundProbe {
		t.Fatalf("PULSE_ROOTFUL_BOUND_PROBE_BINARY must use %q: %q", rootfulQualBoundProbe, boundProbe)
	}
	if secureRuntimeHash(rootlessQualReadFile(t, boundProbe)) != secureRuntimeHash(rootlessQualReadFile(t, executable)) {
		t.Fatal("collector-executable bound probe differs from the qualification binary")
	}
	for _, path := range []string{filepath.Dir(boundProbe), boundProbe} {
		info, statErr := os.Lstat(path)
		if statErr != nil {
			t.Fatalf("stat bound-probe path %s: %v", path, statErr)
		}
		statInfo, ok := info.Sys().(*syscall.Stat_t)
		isProbe := path == boundProbe
		if !ok || statInfo.Uid != 0 || info.Mode().Perm() != 0o755 || isProbe && !info.Mode().IsRegular() || !isProbe && !info.IsDir() {
			t.Fatalf("bound-probe path is not root-owned mode 0755 with a regular executable: %s %+v", path, info)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), deadline+3*time.Second)
	defer cancel()
	started := time.Now()
	cmd := exec.CommandContext(ctx, "runuser", "-u", "pulse-agent", "--", "env",
		"PULSE_ROOTFUL_BOUND_PROBE=1", fmt.Sprintf("PULSE_ROOTFUL_BOUND_DEADLINE_MS=%d", deadline.Milliseconds()),
		boundProbe, "-test.run", "^TestSecureRuntimeRootfulBoundProbe$", "-test.count=1", "-test.v", "-test.timeout=10s")
	output, err := cmd.CombinedOutput()
	elapsed := time.Since(started)
	if err != nil || !strings.Contains(string(output), "ROOTFUL_BOUND_RESULT=deadline_exceeded") {
		t.Fatalf("bounded helper probe failed after %s: %v\n%s", elapsed, err, output)
	}
	if elapsed < deadline/2 || elapsed > deadline+time.Second {
		t.Fatalf("bounded helper probe elapsed %s outside expected interval", elapsed)
	}
	return elapsed
}

func rootfulQualRemoveRuntimeState(t *testing.T, daemon rootfulQualDaemon) {
	t.Helper()
	roots := []string{daemon.dataRoot, daemon.runRoot}
	deadline := time.Now().Add(30 * time.Second)
	for {
		mountInfo := string(rootlessQualReadFile(t, "/proc/self/mountinfo"))
		remaining, err := rootlessQualMountPointsBelow(mountInfo, roots)
		if err != nil {
			t.Fatalf("inspect disposable runtime mounts: %v", err)
		}
		if len(remaining) == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("runtime mounts remain after service shutdown: %q", remaining)
		}
		time.Sleep(100 * time.Millisecond)
	}
	for _, path := range []string{daemon.dataRoot, daemon.runRoot, daemon.fixture} {
		if err := os.RemoveAll(path); err != nil {
			t.Fatalf("remove disposable runtime path %s: %v", path, err)
		}
	}
	if daemon.runtime == "podman" {
		if err := os.Remove(filepath.Dir(daemon.socket)); err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("remove disposable Podman socket directory: %v", err)
		}
	}
}

func rootfulQualRuntimeStateClean(daemon rootfulQualDaemon) bool {
	paths := []string{daemon.socket, daemon.dataRoot, daemon.runRoot, daemon.fixture}
	if daemon.runtime == "podman" {
		paths = append(paths, filepath.Dir(daemon.socket))
	}
	for _, path := range paths {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			return false
		}
	}
	return true
}

func rootfulQualArtifactIdentities(t *testing.T, installerPath string) rootlessQualArtifacts {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return rootlessQualArtifacts{
		QualificationTest: rootfulQualGoArtifact(t, executable, "dockeragent.test"),
		Collector:         rootfulQualGoArtifact(t, strings.TrimSpace(os.Getenv("PULSE_SECURE_RUNTIME_COLLECTOR")), "pulse-agent"),
		Helper:            rootfulQualGoArtifact(t, strings.TrimSpace(os.Getenv("PULSE_SECURE_RUNTIME_HELPER")), "pulse-agent-helper"),
		Installer:         rootlessQualInstallerArtifact{PathBasename: filepath.Base(installerPath), SHA256: secureRuntimeHash(rootlessQualReadFile(t, installerPath))},
	}
}

func rootfulQualGoArtifact(t *testing.T, path, basename string) rootlessQualArtifact {
	t.Helper()
	info, err := buildinfo.ReadFile(path)
	if err != nil {
		t.Fatalf("read Go build metadata for %s: %v", path, err)
	}
	artifact := rootlessQualArtifact{PathBasename: filepath.Base(path), SHA256: secureRuntimeHash(rootlessQualReadFile(t, path)), Package: info.Path, GoVersion: info.GoVersion}
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			artifact.VCSRevision = setting.Value
		case "vcs.modified":
			artifact.VCSModified = setting.Value == "true"
		}
	}
	wantCommit := strings.TrimSpace(os.Getenv("PULSE_ROOTFUL_SOURCE_COMMIT"))
	if artifact.PathBasename != basename || artifact.Package == "" || artifact.VCSRevision != wantCommit || artifact.VCSModified {
		t.Fatalf("rootful qualification artifact is not an exact clean source build: %+v", artifact)
	}
	return artifact
}

func rootfulQualSourceHashes(t *testing.T) map[string]string {
	t.Helper()
	path := strings.TrimSpace(os.Getenv("PULSE_ROOTFUL_SOURCE_HASHES"))
	if !filepath.IsAbs(path) {
		t.Fatalf("PULSE_ROOTFUL_SOURCE_HASHES must be absolute: %q", path)
	}
	var hashes map[string]string
	if err := json.Unmarshal(rootlessQualReadFile(t, path), &hashes); err != nil {
		t.Fatal(err)
	}
	if len(hashes) == 0 {
		t.Fatal("rootful source hash map is empty")
	}
	return hashes
}

func rootfulQualValidateReceipt(receipt rootfulQualReceipt, expectedRuns int) error {
	if receipt.SchemaVersion != 1 || receipt.Kind != "pulse-secure-runtime-rootful-qualification" || receipt.Result != "passed" {
		return errors.New("invalid rootful qualification identity")
	}
	if len(receipt.SourceCommit) != 40 || !rootfulQualBaseImagePattern.MatchString(receipt.BaseImage) || receipt.StartedAt == "" || receipt.CompletedAt == "" || len(receipt.SourceHashes) == 0 || len(receipt.Runs) != expectedRuns {
		return errors.New("incomplete rootful qualification envelope")
	}
	for _, run := range receipt.Runs {
		if run.Runtime.Runtime != "docker" && run.Runtime.Runtime != "podman" {
			return fmt.Errorf("unsupported runtime %q", run.Runtime.Runtime)
		}
		if run.Runtime.DaemonRootless || run.Runtime.DaemonID == "" || run.Runtime.SocketUID != 0 || run.Runtime.SocketPath == "" || run.Runtime.SocketMode != "0660" || run.Runtime.SocketType != "unix" || run.Runtime.SocketSymlink {
			return errors.New("invalid rootful runtime identity")
		}
		if run.Host.MachineID == "" || len(run.Scenarios) != len(rootfulQualScenarioOrder) {
			return errors.New("incomplete rootful host/scenario evidence")
		}
		for index, scenario := range run.Scenarios {
			if scenario.Name != rootfulQualScenarioOrder[index] || scenario.Result != "passed" || scenario.StartedAt == "" || scenario.CompletedAt == "" || scenario.Evidence == nil {
				return fmt.Errorf("invalid scenario %d", index)
			}
			isReporting := scenario.Name != "authority_isolation" && scenario.Name != "cleanup"
			if isReporting != (scenario.ReportSequence != nil && scenario.ReportStreamID != nil) {
				return fmt.Errorf("scenario %s report binding mismatch", scenario.Name)
			}
		}
	}
	return nil
}

func TestRootfulQualificationReceiptContract(t *testing.T) {
	stream := "stream"
	sequence := uint64(1)
	scenarios := make([]rootlessQualScenario, 0, len(rootfulQualScenarioOrder))
	for _, name := range rootfulQualScenarioOrder {
		scenario := rootlessQualScenario{Name: name, Result: "passed", StartedAt: time.Now().UTC().Format(time.RFC3339Nano), CompletedAt: time.Now().UTC().Format(time.RFC3339Nano), Evidence: map[string]any{"observed": true}}
		if name != "authority_isolation" && name != "cleanup" {
			scenario.ReportStreamID = &stream
			scenario.ReportSequence = &sequence
		}
		scenarios = append(scenarios, scenario)
	}
	receipt := rootfulQualReceipt{
		SchemaVersion: 1, Kind: "pulse-secure-runtime-rootful-qualification", Result: "passed",
		SourceCommit: strings.Repeat("a", 40), StartedAt: time.Now().UTC().Format(time.RFC3339Nano), CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
		BaseImage:    "ubuntu@sha256:" + strings.Repeat("c", 64),
		SourceHashes: map[string]string{"go.mod": strings.Repeat("b", 64)},
		Runs:         []rootfulQualRun{{Host: rootlessQualHost{MachineID: strings.Repeat("1", 32)}, Runtime: rootfulQualRuntime{Runtime: "docker", RuntimeVersion: "1", DaemonID: "daemon", SocketPath: "/var/run/docker.sock", SocketUID: 0, SocketGID: 999, SocketMode: "0660", SocketType: "unix"}, Scenarios: scenarios}},
	}
	if err := rootfulQualValidateReceipt(receipt, 1); err != nil {
		t.Fatal(err)
	}
	receipt.Runs[0].Scenarios[3], receipt.Runs[0].Scenarios[4] = receipt.Runs[0].Scenarios[4], receipt.Runs[0].Scenarios[3]
	if err := rootfulQualValidateReceipt(receipt, 1); err == nil {
		t.Fatal("validator accepted reordered rootful scenarios")
	}
}

func TestRootfulQualificationGoSchemaPassesPythonValidator(t *testing.T) {
	commit := strings.Repeat("a", 40)
	digest := strings.Repeat("b", 64)
	started := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	receipt := rootfulQualReceipt{
		SchemaVersion: 1, Kind: "pulse-secure-runtime-rootful-qualification", Result: "passed",
		SourceCommit: commit, StartedAt: started.Format(time.RFC3339Nano),
		BaseImage:    "ubuntu@sha256:" + strings.Repeat("c", 64),
		CompletedAt:  started.Add(2 * time.Minute).Format(time.RFC3339Nano),
		SourceHashes: map[string]string{"internal/agenthelper/container_inventory.go": digest, "scripts/install.sh": digest},
		Artifacts: rootlessQualArtifacts{
			QualificationTest: rootlessQualArtifact{PathBasename: "dockeragent.test", SHA256: digest, Package: "github.com/rcourtman/pulse-go-rewrite/scripts/installtests.test", GoVersion: "go1.25.0", VCSRevision: commit},
			Collector:         rootlessQualArtifact{PathBasename: "pulse-agent", SHA256: digest, Package: "github.com/rcourtman/pulse-go-rewrite/cmd/pulse-agent", GoVersion: "go1.25.0", VCSRevision: commit},
			Helper:            rootlessQualArtifact{PathBasename: "pulse-agent-helper", SHA256: digest, Package: "github.com/rcourtman/pulse-go-rewrite/cmd/pulse-agent-helper", GoVersion: "go1.25.0", VCSRevision: commit},
			Installer:         rootlessQualInstallerArtifact{PathBasename: "install.sh", SHA256: digest},
		},
	}
	for index, runtimeKind := range []string{"docker", "podman"} {
		receipt.Runs = append(receipt.Runs, rootfulQualValidatorFixtureRun(runtimeKind, index, started.Add(time.Duration(index)*30*time.Second), digest))
	}
	path := filepath.Join(t.TempDir(), "receipt.json")
	rootlessQualWriteJSON(t, path, receipt)
	validator := repoFile("scripts", "release_control", "secure_runtime_rootful_attestation_v1.py")
	program := `import importlib.util, pathlib, sys
path=pathlib.Path(sys.argv[1]).resolve()
sys.path.insert(0, str(path.parent))
spec=importlib.util.spec_from_file_location("validator", path)
module=importlib.util.module_from_spec(spec); spec.loader.exec_module(module)
module.parse_receipt_bytes(pathlib.Path(sys.argv[2]).read_bytes())
`
	cmd := exec.Command("python3", "-I", "-c", program, validator, path)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Go rootful receipt schema failed the real Python validator: %v\n%s", err, output)
	}
}

func rootfulQualValidatorFixtureRun(runtimeKind string, index int, began time.Time, digest string) rootfulQualRun {
	base := index * 100
	daemonID := runtimeKind + "-daemon"
	socketPath := "/var/run/docker.sock"
	socketGID := 999
	if runtimeKind == "podman" {
		socketPath = "/run/podman/podman.sock"
		socketGID = 0
	}
	stream := func(value string) *string { return &value }
	sequence := func(value uint64) *uint64 { return &value }
	makeScenario := func(offset int, name string, streamID *string, seq *uint64, evidence map[string]any) rootlessQualScenario {
		start := began.Add(time.Duration(offset) * time.Second)
		return rootlessQualScenario{
			Name: name, Result: "passed", StartedAt: start.Format(time.RFC3339Nano),
			CompletedAt:    start.Add(time.Second).Format(time.RFC3339Nano),
			ReportStreamID: streamID, ReportSequence: seq, Evidence: evidence,
		}
	}
	summary := func(collectorPID, helperPID int) map[string]any {
		return map[string]any{
			"collector_pid": collectorPID, "helper_pid": helperPID,
			"collection_mode": "typed-helper-summary", "inventory_complete": true,
			"inventory_count": 2, "semantic_sha256": digest, "full_fields_present": false,
			"stats_present": false, "secondary_structure_sha256": "",
			"container_updates_enabled": false, "container_actions_enabled": false,
			"direct_socket_access": false, "daemon_id": daemonID, "daemon_rootless": false,
		}
	}

	fresh := summary(base+100, base+200)
	migration := summary(base+110, base+210)
	migration["legacy_profile"] = "root-command-capable"
	migration["target_profile"] = "typed-helper-monitoring-only"
	migration["authority_reduced"] = true
	migration["legacy_collector_pid"] = base + 90
	collectorRestart := summary(base+120, base+210)
	collectorRestart["previous_collector_pid"] = base + 110
	collectorRestart["previous_report_stream_id"] = runtimeKind + "-migration"
	helperRestart := summary(base+120, base+220)
	helperRestart["previous_helper_pid"] = base + 210
	helperRestart["previous_helper_invocation_id"] = runtimeKind + "-helper-old"
	helperRestart["helper_invocation_id"] = runtimeKind + "-helper-new"
	loss := map[string]any{
		"collector_pid": base + 120, "previous_helper_pid": base + 220,
		"collection_mode": "typed-helper-unavailable-status-only", "helper_available": false,
		"status_only": true, "inventory_complete": false, "inventory_present": false,
		"authoritative_inventory_replacement":    false,
		"previous_authoritative_inventory_count": 2, "previous_authoritative_semantic_sha256": digest,
		"operation_status": "degraded", "operation": "container.inventory",
		"container_updates_enabled": false, "container_actions_enabled": false, "direct_socket_access": false,
	}
	recovery := summary(base+120, base+230)
	recovery["previous_helper_pid"] = base + 220
	recovery["previous_status_report_sequence"] = uint64(3)
	bounds := map[string]any{
		"collector_pid": base + 120, "helper_pid": base + 230,
		"operation": "container.inventory", "failure_class": "bounded-timeout",
		"deadline_ms": 2000, "elapsed_ms": 2000, "bounded_failure_observed": true,
		"status_only_report_sequence": uint64(5), "recovery_report_sequence": uint64(6),
		"collection_mode": "typed-helper-summary", "inventory_complete": true,
		"previous_authoritative_inventory_count": 2, "previous_authoritative_semantic_sha256": digest,
		"recovery_inventory_count": 2, "recovery_semantic_sha256": digest,
		"full_fields_present": false, "stats_present": false, "secondary_structure_sha256": "",
		"authoritative_empty_replacement": false, "collector_alive": true, "helper_alive": true,
		"container_updates_enabled": false, "container_actions_enabled": false, "direct_socket_access": false,
	}
	update := summary(base+130, base+230)
	update["previous_collector_pid"] = base + 120
	update["previous_helper_pid"] = base + 230
	update["previous_report_stream_id"] = runtimeKind + "-steady"
	update["update_applied"] = true
	update["collector_binary_sha256"] = digest
	update["helper_binary_sha256"] = digest
	authority := map[string]any{
		"collector_pid": base + 130, "collector_uid": 1000 + index, "effective_uid": 1000 + index,
		"effective_root": false, "safe_profile_enabled": true, "commands_enabled": false,
		"privileged_helper_enabled": true, "reduction_request_observed": true,
		"collector_command_transport_present": false, "collector_command_session_present": false,
		"container_actions_enabled": false, "container_updates_enabled": false,
		"rootful_socket_access": false, "direct_socket_access": false, "helper_network_access": false,
	}

	return rootfulQualRun{
		Host: rootlessQualHost{
			MachineID: "machine-" + runtimeKind, Architecture: "amd64",
			Kernel: "Linux fixture", SystemdVersion: "systemd 255",
		},
		Runtime: rootfulQualRuntime{
			Runtime: runtimeKind, RuntimeVersion: "1.0.0", DaemonID: daemonID,
			DaemonRootless: false, SocketPath: socketPath, SocketUID: 0, SocketGID: socketGID,
			SocketMode: "0660", SocketType: "unix", SocketSymlink: false,
		},
		Scenarios: []rootlessQualScenario{
			makeScenario(0, "fresh_install", stream(runtimeKind+"-fresh"), sequence(1), fresh),
			makeScenario(2, "legacy_migration", stream(runtimeKind+"-migration"), sequence(1), migration),
			makeScenario(4, "collector_restart", stream(runtimeKind+"-steady"), sequence(1), collectorRestart),
			makeScenario(6, "helper_restart", stream(runtimeKind+"-steady"), sequence(2), helperRestart),
			makeScenario(8, "helper_loss", stream(runtimeKind+"-steady"), sequence(3), loss),
			makeScenario(10, "helper_recovery", stream(runtimeKind+"-steady"), sequence(4), recovery),
			makeScenario(12, "operation_bounds", stream(runtimeKind+"-steady"), sequence(6), bounds),
			makeScenario(14, "update_preservation", stream(runtimeKind+"-update"), sequence(1), update),
			makeScenario(16, "authority_isolation", nil, nil, authority),
			makeScenario(18, "cleanup", nil, nil, map[string]any{
				"collector_stopped": true, "helper_stopped": true, "runtime_stopped": true,
				"socket_absent": true, "fixtures_removed": true, "state_clean": true,
			}),
		},
	}
}

func TestRootfulQualificationWrapperInvariants(t *testing.T) {
	raw, err := os.ReadFile(repoFile("scripts", "run-secure-runtime-rootful-qualification.sh"))
	if err != nil {
		t.Fatal(err)
	}
	script := string(raw)
	for _, required := range []string{
		"pulse-secure-runtime-rootful-qualification", "PULSE_ROOTFUL_QUALIFICATION_CONFIRM",
		"--network none", "--cgroupns=private", "docker-receipt.json", "podman-receipt.json",
		"secure_runtime_rootful_attestation_v1.py", "qualification output directory must have exact mode 0700",
		"capture_qualification_container_diagnostics", "journalctl --no-pager -n 2000",
		"org.pulse.rootful-qualification.run", "-buildvcs=true",
		"github.com/rcourtman/pulse-go-rewrite/scripts/installtests.test",
		"https://github.com/rcourtman/Pulse.git", "refs/remotes/origin/main", "refs/heads/main",
		"PULSE_ROOTFUL_UBUNTU_IMAGE", "PULSE_ROOTFUL_BOUND_PROBE_BINARY",
		"rootful_qualification_systemd_readiness",
		"containerd.service",
		"podman-auto-update.service", "podman-auto-update.timer",
		"podman-clean-transient.service", "podman-restart.service",
		rootfulQualBoundProbe,
	} {
		if !strings.Contains(script, required) {
			t.Fatalf("rootful qualification wrapper missing %q", required)
		}
	}
	if count := strings.Count(script, "-buildvcs=true"); count != 3 {
		t.Fatalf("rootful wrapper must require VCS metadata for exactly three Go artifacts: got %d", count)
	}
	for _, forbidden := range []string{"/var/run/docker.sock:/", "/run/docker.sock:/", "/run/podman/podman.sock:/", "--pid=host", "--cgroupns=host"} {
		if strings.Contains(script, forbidden) {
			t.Fatalf("rootful qualification wrapper contains forbidden host boundary %q", forbidden)
		}
	}
}

func TestRootfulQualificationSystemdReadiness(t *testing.T) {
	runtimeScript := repoFile("scripts", "secure-runtime-rootful-runtime.sh")
	binDir := t.TempDir()
	fakeDocker := filepath.Join(binDir, "docker")
	fake := `#!/bin/sh
case "$*" in
  *"systemctl is-system-running"*)
    printf '%s\n' "${FAKE_SYSTEMD_MANAGER_STATE:-starting}"
    [ "${FAKE_SYSTEMD_MANAGER_STATE:-starting}" = running ]
    ;;
  *"systemctl show --property=ActiveState --value multi-user.target"*)
    printf '%s\n' "${FAKE_SYSTEMD_TARGET_STATE:-inactive}"
    ;;
  *"systemctl list-units --state=failed --no-legend --no-pager --plain"*)
    printf '%s' "${FAKE_SYSTEMD_FAILED_UNITS:-}"
    ;;
  *)
    printf 'unexpected docker arguments: %s\n' "$*" >&2
    exit 99
    ;;
esac
`
	if err := os.WriteFile(fakeDocker, []byte(fake), 0o700); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name         string
		managerState string
		targetState  string
		failedUnits  string
		wantExit     int
		wantOutput   string
	}{
		{name: "manager starting", managerState: "starting", targetState: "active", wantExit: 1},
		{name: "target inactive", managerState: "running", targetState: "inactive", wantExit: 1},
		{name: "clean running manager", managerState: "running", targetState: "active", wantExit: 0},
		{name: "degraded manager", managerState: "degraded", targetState: "active", wantExit: 2, wantOutput: "degraded"},
		{name: "maintenance manager", managerState: "maintenance", targetState: "active", wantExit: 2, wantOutput: "maintenance"},
		{name: "failed unit", managerState: "running", targetState: "active", failedUnits: "podman-restart.service loaded failed failed", wantExit: 2, wantOutput: "podman-restart.service"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := exec.Command("bash", "-c", `source "$1"; rootful_qualification_systemd_readiness fixture`, "bash", runtimeScript)
			cmd.Env = append(os.Environ(),
				"PATH="+binDir+":"+os.Getenv("PATH"),
				"FAKE_SYSTEMD_MANAGER_STATE="+test.managerState,
				"FAKE_SYSTEMD_TARGET_STATE="+test.targetState,
				"FAKE_SYSTEMD_FAILED_UNITS="+test.failedUnits,
			)
			output, err := cmd.CombinedOutput()
			gotExit := 0
			if err != nil {
				var exitErr *exec.ExitError
				if !errors.As(err, &exitErr) {
					t.Fatalf("readiness helper failed without an exit status: %v", err)
				}
				gotExit = exitErr.ExitCode()
			}
			if gotExit != test.wantExit {
				t.Fatalf("readiness exit = %d, want %d\n%s", gotExit, test.wantExit, output)
			}
			if test.wantOutput != "" && !strings.Contains(string(output), test.wantOutput) {
				t.Fatalf("readiness output missing %q: %s", test.wantOutput, output)
			}
		})
	}
}
