//go:build !windows

package installtests

// This file is a deliberately separate qualification packet. The live test is
// never part of ordinary `go test`: it mutates users, systemd units, and local
// container-runtime state and therefore requires an exact disposable-host
// marker plus an explicit opt-in. The host-side wrapper creates the isolated
// Ubuntu 24.04 systemd containers and never mounts the host runtime socket.

import (
	"context"
	"crypto/sha256"
	"debug/buildinfo"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

const (
	rootlessQualOptIn       = "PULSE_SECURE_RUNTIME_ROOTLESS_QUALIFICATION"
	rootlessQualOptInValue  = "disposable-v1"
	rootlessQualMarker      = "/etc/pulse-secure-runtime-rootless-qualification"
	rootlessQualFixture     = "pulse-rootless-qualification-fixture:v1"
	rootlessQualRunningName = "pulse-rootless-running"
	rootlessQualExitedName  = "pulse-rootless-exited"
)

var rootlessQualScenarioOrder = []string{
	"fresh_install",
	"legacy_migration",
	"collector_restart",
	"daemon_restart",
	"socket_loss_helper_fallback",
	"direct_recovery",
	"dual_socket_ambiguity_refusal",
	"exact_pin_recovery",
	"telemetry_parity",
	"authority_isolation",
	"cleanup",
}

type rootlessQualReceipt struct {
	SchemaVersion int                   `json:"schema_version"`
	Kind          string                `json:"kind"`
	Result        string                `json:"result"`
	SourceCommit  string                `json:"source_commit"`
	StartedAt     string                `json:"started_at"`
	CompletedAt   string                `json:"completed_at"`
	SourceHashes  map[string]string     `json:"source_hashes"`
	Artifacts     rootlessQualArtifacts `json:"artifacts"`
	Runs          []rootlessQualRun     `json:"runs"`
}

type rootlessQualHost struct {
	MachineID      string `json:"machine_id"`
	Architecture   string `json:"architecture"`
	Kernel         string `json:"kernel"`
	SystemdVersion string `json:"systemd_version"`
}

type rootlessQualArtifact struct {
	PathBasename string `json:"path_basename"`
	SHA256       string `json:"sha256"`
	Package      string `json:"package"`
	GoVersion    string `json:"go_version"`
	VCSRevision  string `json:"vcs_revision"`
	VCSModified  bool   `json:"vcs_modified"`
}

type rootlessQualInstallerArtifact struct {
	PathBasename string `json:"path_basename"`
	SHA256       string `json:"sha256"`
}

type rootlessQualArtifacts struct {
	QualificationTest rootlessQualArtifact          `json:"qualification_test"`
	Collector         rootlessQualArtifact          `json:"collector"`
	Helper            rootlessQualArtifact          `json:"helper"`
	Installer         rootlessQualInstallerArtifact `json:"installer"`
}

type rootlessQualRun struct {
	Host      rootlessQualHost       `json:"host"`
	Runtime   rootlessQualRuntime    `json:"runtime"`
	Scenarios []rootlessQualScenario `json:"scenarios"`
}

type rootlessQualRuntime struct {
	Runtime        string `json:"runtime"`
	RuntimeVersion string `json:"runtime_version"`
	DaemonID       string `json:"daemon_id"`
	CollectorUID   int    `json:"collector_uid"`
	SocketPath     string `json:"socket_path"`
	SocketUID      int    `json:"socket_uid"`
	SocketGID      int    `json:"socket_gid"`
	SocketMode     string `json:"socket_mode"`
	DaemonRootless bool   `json:"daemon_rootless"`
	SocketType     string `json:"socket_type"`
	SocketSymlink  bool   `json:"socket_symlink"`
}

type rootlessQualScenario struct {
	Name           string         `json:"name"`
	Result         string         `json:"result"`
	StartedAt      string         `json:"started_at"`
	CompletedAt    string         `json:"completed_at"`
	ReportSequence *uint64        `json:"report_sequence"`
	ReportStreamID *string        `json:"report_stream_id"`
	Evidence       map[string]any `json:"evidence"`
}

type rootlessQualDaemon struct {
	runtime      string
	rootlessUnit string
	rootfulUnit  string
	rootlessSock string
	rootfulSock  string
	uid          int
	home         string
}

func TestSecureRuntimeRootlessQualification(t *testing.T) {
	if os.Getenv(rootlessQualOptIn) != rootlessQualOptInValue {
		t.Skip("run through scripts/run-secure-runtime-rootless-qualification.sh inside its disposable systemd container")
	}
	runtimeKind := strings.TrimSpace(os.Getenv("PULSE_ROOTLESS_RUNTIME"))
	receiptPath := strings.TrimSpace(os.Getenv("PULSE_ROOTLESS_RECEIPT"))
	rootlessQualRequireDisposableHost(t, runtimeKind, receiptPath)

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
	server := httptest.NewServer(fixture)
	defer server.Close()
	collectorCredential := secureRuntimeLabToken

	daemon := rootlessQualPrepareDaemons(t, runtimeKind)
	defer rootlessQualBestEffortStop(daemon.rootlessUnit, daemon.rootfulUnit, rootlessQualOtherUnit(runtimeKind), fmt.Sprintf("user@%d.service", daemon.uid))
	rootlessQualStartRootful(t, daemon)
	rootlessQualStartRootless(t, daemon)
	identity := rootlessQualReadIdentityRecord(t, daemon)
	if err := os.Remove(daemon.rootlessSock + ".qualification-identity"); err != nil {
		t.Fatalf("remove transient runtime identity sidecar: %v", err)
	}
	rootlessQualCreateFixture(t, daemon, false)
	rootlessQualCreateFixture(t, daemon, true)
	daemonRootlessObserved := rootlessQualDaemonRootless(t, daemon)
	if !daemonRootlessObserved {
		t.Fatalf("%s daemon did not independently report rootless operation", runtimeKind)
	}
	rootfulBaseline := rootlessQualRuntimeBaseline(t, daemon, false)
	rootlessBaseline := rootlessQualRuntimeBaseline(t, daemon, true)
	if rootfulBaseline.SemanticDigest != rootlessBaseline.SemanticDigest || rootfulBaseline.Count != 2 {
		t.Fatalf("separate same-family baselines differ: rootful=%+v rootless=%+v", rootfulBaseline, rootlessBaseline)
	}

	var scenarios []rootlessQualScenario
	appendScenario := func(name string, began time.Time, report *agentsdocker.Report, evidence map[string]any) {
		scenario := rootlessQualScenario{Name: name, Result: "passed", StartedAt: began.Format(time.RFC3339Nano), CompletedAt: time.Now().UTC().Format(time.RFC3339Nano), Evidence: evidence}
		if report != nil {
			stream, sequence, ok := agentshost.ParseReportSequenceID(report.SequenceID)
			if !ok {
				t.Fatalf("scenario %s received invalid sequence ID %q", name, report.SequenceID)
			}
			scenario.ReportSequence = &sequence
			scenario.ReportStreamID = &stream
		}
		scenarios = append(scenarios, scenario)
	}

	// Prove a fresh safe-profile install first, then remove it before creating
	// the legacy root profile. Both paths therefore begin without installed
	// Pulse files; the surrounding outer container itself is also fresh.
	freshStarted := time.Now().UTC()
	secureRuntimeRunInstaller(t, installerPath, server.URL,
		"--least-privilege", "--enable-privileged-helper", "--enable-docker")
	freshReport := rootlessQualWaitDirect(t, fixture, freshStarted, runtimeKind, rootlessBaseline.SemanticDigest, 75*time.Second)
	secureRuntimeAssertSafeProfile(t)
	secureRuntimeAssertHelperProtocol(t)
	freshDigest := rootlessQualDigestReport(freshReport.Report)
	freshPID := secureRuntimeCollectorMainPID(t)
	daemonIDBefore := rootlessQualDaemonID(t, daemon, true)
	appendScenario("fresh_install", freshStarted, &freshReport.Report,
		rootlessQualDirectEvidence(daemon, identity, freshPID, daemonIDBefore, daemonRootlessObserved, freshDigest))
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
	legacyReport := rootlessQualWaitReport(t, fixture, migrationStarted, 75*time.Second, func(report agentsdocker.Report) bool {
		return rootlessQualComplete(report) && report.Host.CollectionMode == "" && report.Host.Runtime == runtimeKind
	})
	secureRuntimeAssertRootCommandProfile(t)
	legacyPID := secureRuntimeCollectorMainPID(t)
	legacyDigest := rootlessQualSemanticDigest(legacyReport.Report)
	if legacyDigest != rootfulBaseline.SemanticDigest {
		t.Fatalf("legacy root report differs from the separate rootful runtime baseline: report=%s baseline=%s", legacyDigest, rootfulBaseline.SemanticDigest)
	}
	applyStarted := time.Now().UTC()
	secureRuntimeRunInstallerWithCollectorCredential(t, installerPath, server.URL, collectorCredential, "--safe-profile-apply")
	migratedReport := rootlessQualWaitDirect(t, fixture, applyStarted, runtimeKind, rootlessBaseline.SemanticDigest, 75*time.Second)
	secureRuntimeAssertSafeProfile(t)
	secureRuntimeAssertHelperProtocol(t)
	migratedDigest := rootlessQualDigestReport(migratedReport.Report)
	if freshDigest.SemanticDigest != rootlessBaseline.SemanticDigest || migratedDigest.SemanticDigest != rootlessBaseline.SemanticDigest {
		t.Fatalf("direct report semantic parity failed: baseline=%s fresh=%s migration=%s", rootlessBaseline.SemanticDigest, freshDigest.SemanticDigest, migratedDigest.SemanticDigest)
	}
	migratedPID := secureRuntimeCollectorMainPID(t)
	if migratedPID == legacyPID {
		t.Fatalf("safe-profile migration did not replace legacy collector PID %d", legacyPID)
	}
	migrationEvidence := rootlessQualDirectEvidence(daemon, identity, migratedPID, daemonIDBefore, daemonRootlessObserved, migratedDigest)
	migrationEvidence["legacy_profile"] = "root-command-capable"
	migrationEvidence["target_profile"] = "typed-helper-monitoring-only"
	migrationEvidence["authority_reduced"] = true
	migrationEvidence["legacy_collector_pid"] = legacyPID
	appendScenario("legacy_migration", migrationStarted, &migratedReport.Report, migrationEvidence)

	restartStarted := time.Now().UTC()
	collectorPIDBefore := migratedPID
	streamBefore, _, _ := agentshost.ParseReportSequenceID(migratedReport.Report.SequenceID)
	secureRuntimeCommand(t, 20*time.Second, "systemctl", "restart", "pulse-agent.service")
	collectorPIDAfter := secureRuntimeCollectorMainPID(t)
	restartedCollectorReport := rootlessQualWaitDirect(t, fixture, restartStarted, runtimeKind, rootlessBaseline.SemanticDigest, 75*time.Second)
	streamAfter, _, _ := agentshost.ParseReportSequenceID(restartedCollectorReport.Report.SequenceID)
	restartedCollectorDigest := rootlessQualDigestReport(restartedCollectorReport.Report)
	if collectorPIDAfter == collectorPIDBefore || streamAfter == streamBefore || restartedCollectorDigest.SemanticDigest != rootlessBaseline.SemanticDigest {
		t.Fatalf("collector restart did not replace process/stream with parity: pid=%d/%d stream=%s/%s", collectorPIDBefore, collectorPIDAfter, streamBefore, streamAfter)
	}
	restartEvidence := rootlessQualDirectEvidence(daemon, identity, collectorPIDAfter, daemonIDBefore, daemonRootlessObserved, restartedCollectorDigest)
	restartEvidence["previous_collector_pid"] = collectorPIDBefore
	restartEvidence["previous_report_stream_id"] = streamBefore
	appendScenario("collector_restart", restartStarted, &restartedCollectorReport.Report, restartEvidence)

	daemonRestartStarted := time.Now().UTC()
	daemonPIDBefore, invocationBefore := rootlessQualUnitIdentity(t, daemon.rootlessUnit)
	rootlessQualStopUnit(t, daemon.rootlessUnit)
	rootlessQualStartRootless(t, daemon)
	daemonPIDAfter, invocationAfter := rootlessQualUnitIdentity(t, daemon.rootlessUnit)
	daemonIDAfter := rootlessQualDaemonID(t, daemon, true)
	daemonRootlessAfterRestart := rootlessQualDaemonRootless(t, daemon)
	daemonRestartReport := rootlessQualWaitDirect(t, fixture, daemonRestartStarted, runtimeKind, rootlessBaseline.SemanticDigest, 75*time.Second)
	daemonRestartDigest := rootlessQualDigestReport(daemonRestartReport.Report)
	if daemonPIDBefore == daemonPIDAfter || invocationBefore == invocationAfter || daemonIDBefore != daemonIDAfter || daemonRestartDigest.SemanticDigest != rootlessBaseline.SemanticDigest {
		t.Fatalf("daemon restart identity/parity mismatch: pid=%d/%d invocation=%s/%s daemon=%s/%s", daemonPIDBefore, daemonPIDAfter, invocationBefore, invocationAfter, daemonIDBefore, daemonIDAfter)
	}
	daemonEvidence := rootlessQualDirectEvidence(daemon, identity, collectorPIDAfter, daemonIDAfter, daemonRootlessAfterRestart, daemonRestartDigest)
	daemonEvidence["previous_daemon_pid"] = daemonPIDBefore
	daemonEvidence["daemon_pid"] = daemonPIDAfter
	daemonEvidence["previous_daemon_invocation_id"] = invocationBefore
	daemonEvidence["daemon_invocation_id"] = invocationAfter
	appendScenario("daemon_restart", daemonRestartStarted, &daemonRestartReport.Report, daemonEvidence)

	lossStarted := time.Now().UTC()
	rootlessQualStopUnit(t, daemon.rootlessUnit)
	helperReport := rootlessQualWaitReport(t, fixture, lossStarted, 90*time.Second, func(report agentsdocker.Report) bool {
		return rootlessQualComplete(report) && report.Host.CollectionMode == agentsdocker.CollectionModeTypedHelperSummary && report.Host.Runtime == runtimeKind
	})
	rootlessQualAssertHelperSummaryOnly(t, helperReport.Report, runtimeKind)
	helperDigest := rootlessQualDigestReport(helperReport.Report)
	if helperDigest.SemanticDigest != rootfulBaseline.SemanticDigest || secureRuntimeCollectorMainPID(t) != collectorPIDAfter {
		t.Fatalf("typed-helper fallback did not preserve rootful semantic baseline/collector PID")
	}
	appendScenario("socket_loss_helper_fallback", lossStarted, &helperReport.Report, map[string]any{
		"collector_pid": collectorPIDAfter, "collection_mode": agentsdocker.CollectionModeTypedHelperSummary,
		"direct_runtime_available": false, "helper_fallback": true, "inventory_complete": true,
		"inventory_count": helperDigest.Count, "rootful_baseline_inventory_count": rootfulBaseline.Count,
		"semantic_sha256": helperDigest.SemanticDigest, "rootful_baseline_semantic_sha256": rootfulBaseline.SemanticDigest,
		"full_fields_present": false, "stats_present": false, "secondary_structure_sha256": "",
		"container_actions_enabled": false, "container_updates_enabled": false, "collector_restart_count": 0,
	})

	recoveryStarted := time.Now().UTC()
	rootlessQualStartRootless(t, daemon)
	recoveryReport := rootlessQualWaitDirect(t, fixture, recoveryStarted, runtimeKind, rootlessBaseline.SemanticDigest, 90*time.Second)
	recoveryDigest := rootlessQualDigestReport(recoveryReport.Report)
	daemonRootlessAfterRecovery := rootlessQualDaemonRootless(t, daemon)
	if !rootlessQualStableDigestEqual(recoveryDigest, daemonRestartDigest) || secureRuntimeCollectorMainPID(t) != collectorPIDAfter || rootlessQualDaemonID(t, daemon, true) != daemonIDBefore {
		t.Fatalf("direct recovery did not restore the exact prior rootless telemetry/identity")
	}
	appendScenario("direct_recovery", recoveryStarted, &recoveryReport.Report,
		rootlessQualDirectEvidence(daemon, identity, collectorPIDAfter, daemonIDBefore, daemonRootlessAfterRecovery, recoveryDigest))

	ambiguityStarted := time.Now().UTC()
	otherUnit, otherSocket := rootlessQualStartOtherRootless(t, daemon)
	defer rootlessQualBestEffortStop(otherUnit)
	if _, err := os.Lstat(otherSocket); err != nil {
		t.Fatalf("second live rootless socket missing: %v", err)
	}
	ambiguityOutput := rootlessQualRunUnpinnedCollector(t, server.URL, collectorCredential, 6*time.Second)
	if !strings.Contains(strings.ToLower(ambiguityOutput), "ambiguous collector-owned rootless runtime endpoints") {
		t.Fatalf("unpinned collector did not fail closed on dual sockets:\n%s", ambiguityOutput)
	}
	appendScenario("dual_socket_ambiguity_refusal", ambiguityStarted, nil, map[string]any{
		"protected_collector_pid": collectorPIDAfter,
		"live_sockets":            rootlessQualDualSocketEvidence(t, daemon.uid),
		"probe_kind":              "separate-unpinned-collector",
		"admission_refused":       true, "fail_closed": true, "daemon_probe_count": 0,
		"container_actions_enabled": false, "collector_restart_count": 0,
	})
	rootlessQualStopUnit(t, otherUnit)

	pinStarted := time.Now().UTC()
	rootlessQualStopUnit(t, daemon.rootlessUnit)
	unitBefore := rootlessQualServicePin(t, runtimeKind)
	pinFallback := rootlessQualWaitReport(t, fixture, pinStarted, 90*time.Second, func(report agentsdocker.Report) bool {
		return rootlessQualComplete(report) && report.Host.CollectionMode == agentsdocker.CollectionModeTypedHelperSummary && report.Host.Runtime == runtimeKind
	})
	previousUpdateStream, _, _ := agentshost.ParseReportSequenceID(pinFallback.Report.SequenceID)
	updateStarted := time.Now().UTC()
	secureRuntimeRunInstallerWithCollectorCredential(t, installerPath, server.URL, collectorCredential, "--update")
	unitWhileAbsent := rootlessQualServicePin(t, runtimeKind)
	if unitBefore != daemon.rootlessSock || unitWhileAbsent != unitBefore {
		t.Fatalf("offline update lost exact rootless pin: before=%q after=%q", unitBefore, unitWhileAbsent)
	}
	collectorPIDAfterUpdate := secureRuntimeCollectorMainPID(t)
	postUpdateFallback := rootlessQualWaitReport(t, fixture, updateStarted, 90*time.Second, func(report agentsdocker.Report) bool {
		stream, _, ok := agentshost.ParseReportSequenceID(report.SequenceID)
		return ok && stream != previousUpdateStream && rootlessQualComplete(report) && report.Host.CollectionMode == agentsdocker.CollectionModeTypedHelperSummary && report.Host.Runtime == runtimeKind
	})
	rootlessQualStartRootless(t, daemon)
	pinReport := rootlessQualWaitDirect(t, fixture, postUpdateFallback.ReceivedAt, runtimeKind, rootlessBaseline.SemanticDigest, 90*time.Second)
	pinDigest := rootlessQualDigestReport(pinReport.Report)
	daemonRootlessAfterPinRecovery := rootlessQualDaemonRootless(t, daemon)
	if !rootlessQualStableDigestEqual(pinDigest, recoveryDigest) || rootlessQualServicePin(t, runtimeKind) != daemon.rootlessSock {
		t.Fatalf("exact pin recovery did not return to the same endpoint and telemetry")
	}
	pinEvidence := rootlessQualDirectEvidence(daemon, identity, collectorPIDAfterUpdate, daemonIDBefore, daemonRootlessAfterPinRecovery, pinDigest)
	_, pinFallbackSequence, _ := agentshost.ParseReportSequenceID(postUpdateFallback.Report.SequenceID)
	_, pinRecoverySequence, _ := agentshost.ParseReportSequenceID(pinReport.Report.SequenceID)
	pinEvidence["previous_collector_pid"] = collectorPIDAfter
	pinEvidence["previous_report_stream_id"] = previousUpdateStream
	pinEvidence["pin_source"] = "root-owned-systemd-unit"
	pinEvidence["pinned_socket_path"] = unitBefore
	pinEvidence["socket_absent_observed"] = true
	pinEvidence["fallback_report_sequence"] = pinFallbackSequence
	pinEvidence["recovery_report_sequence"] = pinRecoverySequence
	pinEvidence["recovered_socket_path"] = daemon.rootlessSock
	pinEvidence["selected_socket_path"] = rootlessQualServicePin(t, runtimeKind)
	pinEvidence["recovered_socket_uid"] = identity.SocketUID
	pinEvidence["recovered_socket_gid"] = identity.SocketGID
	pinEvidence["recovered_socket_mode"] = identity.SocketMode
	pinEvidence["recovered_socket_type"] = "unix"
	pinEvidence["recovered_socket_symlink"] = false
	pinEvidence["candidate_count"] = 1
	pinEvidence["daemon_probe_count"] = 1
	pinEvidence["collector_restart_count"] = 1
	appendScenario("exact_pin_recovery", pinStarted, &pinReport.Report, pinEvidence)

	parityStarted := time.Now().UTC()
	parityReport := rootlessQualWaitDirect(t, fixture, parityStarted, runtimeKind, rootlessBaseline.SemanticDigest, 75*time.Second)
	parityDigest := rootlessQualDigestReport(parityReport.Report)
	appendScenario("telemetry_parity", parityStarted, &parityReport.Report, map[string]any{
		"collector_pid": collectorPIDAfterUpdate, "baseline_kind": "root-client-same-rootless-daemon",
		"baseline_inventory_count": rootlessBaseline.Count, "collector_inventory_count": parityDigest.Count,
		"baseline_semantic_sha256": rootlessBaseline.SemanticDigest, "collector_semantic_sha256": parityDigest.SemanticDigest,
		"collector_full_fields_present": parityDigest.FullFieldsPresent, "collector_stats_present": parityDigest.StatsPresent,
		"collector_secondary_inventory_present": parityDigest.SecondaryInventoryPresent,
	})

	authorityStarted := time.Now().UTC()
	collectorUID := rootlessQualUID(t, "pulse-agent")
	rootfulDenied := rootlessQualRootfulAccessDenied(t, daemon)
	groups := strings.Fields(rootlessQualCommand(t, 10*time.Second, "id", "-nG", "pulse-agent"))
	for _, group := range groups {
		if group == "docker" || group == "podman" {
			t.Fatalf("collector retained daemon group %q", group)
		}
	}
	if !rootfulDenied || secureRuntimeCollectorProcessUID(t) != collectorUID || secureRuntimeCollectorHasArgument("--enable-commands") {
		t.Fatal("authority isolation did not remain exact")
	}
	helperNetworkDenied := rootlessQualAssertHelperNetworkDenied(t)
	commandSessionPresent := fixture.actionServer.IsAgentConnectedForOrganization(secureRuntimeLabOrgID, secureRuntimeLabAgentID)
	if !helperNetworkDenied || commandSessionPresent || fixture.authorityReductionCount() < 1 {
		t.Fatalf("authority evidence mismatch: helper_network_denied=%t command_session=%t reductions=%d", helperNetworkDenied, commandSessionPresent, fixture.authorityReductionCount())
	}
	appendScenario("authority_isolation", authorityStarted, nil, map[string]any{
		"collector_pid": collectorPIDAfterUpdate, "collector_uid": collectorUID, "effective_uid": collectorUID,
		"effective_root": false, "safe_profile_enabled": true, "commands_enabled": false,
		"privileged_helper_enabled": true, "reduction_request_observed": true,
		"collector_command_transport_present": false, "collector_command_session_present": false,
		"container_actions_enabled": false, "container_updates_enabled": false,
		"rootful_socket_access": false, "helper_network_access": false,
	})

	cleanupStarted := time.Now().UTC()
	rootlessQualUninstallPulse(t, installerPath, server.URL, collectorCredential)
	if registered, revoked, uninstalls := fixture.collectorLifecycleSnapshot(); registered || !revoked || uninstalls != 2 {
		t.Fatalf("final collector uninstall was not durably modeled: registered=%t revoked=%t uninstalls=%d", registered, revoked, uninstalls)
	}
	rootlessQualStopUnit(t, daemon.rootlessUnit)
	rootlessQualStopUnit(t, daemon.rootfulUnit)
	rootlessQualStopUserManager(t, daemon)
	rootlessQualRemoveRuntimeState(t, daemon)
	rootlessQualAssertPulseRemoved(t)
	for _, socket := range []string{daemon.rootlessSock, daemon.rootfulSock} {
		if _, err := os.Lstat(socket); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("cleanup left socket %s: %v", socket, err)
		}
	}
	userStateClean := rootlessQualUserStateClean(daemon)
	if !userStateClean {
		t.Fatal("cleanup left dedicated rootless runtime state")
	}
	appendScenario("cleanup", cleanupStarted, nil, map[string]any{
		"runtime_stopped": true, "socket_absent": true, "fixtures_removed": true, "user_state_clean": userStateClean,
	})

	artifacts := rootlessQualArtifactIdentities(t, installerPath)
	sourceHashes := rootlessQualSourceHashes(t)
	receipt := rootlessQualReceipt{
		SchemaVersion: 1, Kind: "pulse-secure-runtime-rootless-qualification", Result: "passed",
		SourceCommit: strings.TrimSpace(os.Getenv("PULSE_ROOTLESS_SOURCE_COMMIT")),
		StartedAt:    started.Format(time.RFC3339Nano), CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
		SourceHashes: sourceHashes, Artifacts: artifacts,
		Runs: []rootlessQualRun{{
			Host: rootlessQualHost{
				MachineID: strings.TrimSpace(string(rootlessQualReadFile(t, "/etc/machine-id"))), Architecture: runtime.GOARCH,
				Kernel:         rootlessQualCommand(t, 10*time.Second, "uname", "-srvmo"),
				SystemdVersion: strings.SplitN(rootlessQualCommand(t, 10*time.Second, "systemctl", "--version"), "\n", 2)[0],
			},
			Runtime: rootlessQualRuntime{
				Runtime: runtimeKind, RuntimeVersion: identity.RuntimeVersion, DaemonID: daemonIDBefore,
				CollectorUID: daemon.uid, SocketPath: daemon.rootlessSock, SocketUID: identity.SocketUID,
				SocketGID: identity.SocketGID, SocketMode: identity.SocketMode, DaemonRootless: daemonRootlessObserved,
				SocketType: "unix", SocketSymlink: false,
			},
			Scenarios: scenarios,
		}},
	}
	if err := rootlessQualValidateReceipt(receipt, 1); err != nil {
		t.Fatalf("generated receipt failed validation: %v", err)
	}
	rootlessQualWriteJSON(t, receiptPath, receipt)
}

type rootlessQualBaseline struct {
	Count          int
	SemanticDigest string
}

type rootlessQualReportDigest struct {
	Count                     int
	InventoryDigest           string
	SemanticDigest            string
	StatsDigest               string
	StatsPresent              bool
	FullFieldsPresent         bool
	SecondaryDigest           string
	SecondaryInventoryPresent bool
}

type rootlessQualContainerSemantic struct {
	Name  string
	Image string
	State string
}

type rootlessQualIdentity struct {
	RuntimeVersion string `json:"runtime_version"`
	SocketUID      int    `json:"socket_uid"`
	SocketGID      int    `json:"socket_gid"`
	SocketMode     string `json:"socket_mode"`
}

func rootlessQualDirectEvidence(d rootlessQualDaemon, identity rootlessQualIdentity, collectorPID int, daemonID string, daemonRootless bool, digest rootlessQualReportDigest) map[string]any {
	return map[string]any{
		"collector_pid": collectorPID, "service_pid": collectorPID,
		"collection_path": "collector-owned-rootless-socket", "inventory_complete": true, "inventory_count": digest.Count,
		"semantic_sha256": digest.SemanticDigest, "full_fields_present": digest.FullFieldsPresent,
		"stats_present": digest.StatsPresent, "secondary_structure_sha256": digest.SecondaryDigest,
		"daemon_id": daemonID, "daemon_rootless": daemonRootless,
		"socket_path": d.rootlessSock, "socket_uid": identity.SocketUID, "socket_gid": identity.SocketGID,
		"socket_mode": identity.SocketMode, "socket_type": "unix", "socket_symlink": false,
	}
}

func rootlessQualRequireDisposableHost(t *testing.T, runtimeKind, receiptPath string) {
	t.Helper()
	if os.Geteuid() != 0 || (runtimeKind != "docker" && runtimeKind != "podman") {
		t.Fatalf("qualification requires root and PULSE_ROOTLESS_RUNTIME=docker|podman")
	}
	marker, err := os.ReadFile(rootlessQualMarker)
	if err != nil || strings.TrimSpace(string(marker)) != rootlessQualOptInValue {
		t.Fatalf("disposable marker is absent or invalid: %v", err)
	}
	if !filepath.IsAbs(receiptPath) || filepath.Clean(receiptPath) != receiptPath {
		t.Fatalf("PULSE_ROOTLESS_RECEIPT must be an exact absolute path: %q", receiptPath)
	}
	osRelease := string(rootlessQualReadFile(t, "/etc/os-release"))
	if !strings.Contains(osRelease, "VERSION_ID=\"24.04\"") && !strings.Contains(osRelease, "VERSION_ID=24.04") {
		t.Fatal("qualification host is not Ubuntu 24.04")
	}
	if _, err := os.Stat("/run/systemd/system"); err != nil {
		t.Fatalf("qualification host is not booted under systemd: %v", err)
	}
	if routes := string(rootlessQualReadFile(t, "/proc/net/route")); rootlessQualHasDefaultRoute(routes) {
		t.Fatal("qualification workload must run with outer-container networking disabled")
	}
}

func rootlessQualHasDefaultRoute(routes string) bool {
	for _, line := range strings.Split(routes, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] == "00000000" {
			return true
		}
	}
	return false
}

func rootlessQualPrepareDaemons(t *testing.T, runtimeKind string) rootlessQualDaemon {
	t.Helper()
	if _, err := exec.Command("id", "-u", "pulse-agent").Output(); err != nil {
		rootlessQualCommand(t, 10*time.Second, "useradd", "--system", "--create-home", "--home-dir", "/var/lib/pulse-rootless", "--shell", "/usr/sbin/nologin", "pulse-agent")
	}
	uid := rootlessQualUID(t, "pulse-agent")
	home := "/var/lib/pulse-rootless"
	rootlessQualWriteSubID(t, "/etc/subuid", "pulse-agent:100000:65536")
	rootlessQualWriteSubID(t, "/etc/subgid", "pulse-agent:100000:65536")
	paths := []string{filepath.Join("/run/user", strconv.Itoa(uid)), filepath.Join(home, "fixture"), filepath.Join(home, "docker"), filepath.Join(home, "podman")}
	for _, path := range paths {
		rootlessQualCommand(t, 10*time.Second, "install", "-d", "-o", "pulse-agent", "-g", "pulse-agent", "-m", "0700", path)
	}
	rootlessQualCommand(t, 10*time.Second, "loginctl", "enable-linger", "pulse-agent")
	userUnit := fmt.Sprintf("user@%d.service", uid)
	rootlessQualCommand(t, 20*time.Second, "systemctl", "start", userUnit)
	active := rootlessQualCommand(t, 10*time.Second, "systemctl", "show", userUnit, "--property=ActiveState", "--value")
	delegated := rootlessQualCommand(t, 10*time.Second, "systemctl", "show", userUnit, "--property=Delegate", "--value")
	controlGroup := rootlessQualCommand(t, 10*time.Second, "systemctl", "show", userUnit, "--property=ControlGroup", "--value")
	expectedControlGroup := fmt.Sprintf("/user.slice/user-%d.slice/user@%d.service", uid, uid)
	if active != "active" || delegated != "yes" || controlGroup != expectedControlGroup {
		t.Fatalf("rootless user manager is not exactly delegated: active=%q delegate=%q control_group=%q", active, delegated, controlGroup)
	}
	containerfile := "FROM scratch\nCOPY busybox /busybox\nENTRYPOINT [\"/busybox\"]\n"
	if err := os.WriteFile(filepath.Join(home, "fixture", "Containerfile"), []byte(containerfile), 0o600); err != nil {
		t.Fatal(err)
	}
	busybox, err := os.ReadFile("/bin/busybox")
	if err != nil {
		t.Fatalf("read offline busybox fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(home, "fixture", "busybox"), busybox, 0o755); err != nil {
		t.Fatal(err)
	}
	rootlessQualCommand(t, 10*time.Second, "chown", "-R", "pulse-agent:pulse-agent", home)
	d := rootlessQualDaemon{runtime: runtimeKind, uid: uid, home: home, rootlessUnit: "pulse-rootless-" + runtimeKind, rootfulUnit: "pulse-rootful-" + runtimeKind}
	if runtimeKind == "docker" {
		d.rootlessSock = filepath.Join("/run/user", strconv.Itoa(uid), "docker.sock")
		d.rootfulSock = "/var/run/docker.sock"
	} else {
		d.rootlessSock = filepath.Join("/run/user", strconv.Itoa(uid), "podman", "podman.sock")
		d.rootfulSock = "/run/podman/podman.sock"
	}
	return d
}

func rootlessQualStartRootless(t *testing.T, d rootlessQualDaemon) {
	t.Helper()
	rootlessQualBestEffortStop(d.rootlessUnit)
	if d.runtime == "docker" {
		rootlessQualCommand(t, 20*time.Second, "systemd-run", rootlessQualDockerStartArgs(d)...)
	} else {
		rootlessQualCommand(t, 20*time.Second, "install", "-d", "-o", "pulse-agent", "-g", "pulse-agent", "-m", "0700", filepath.Dir(d.rootlessSock))
		rootlessQualCommand(t, 20*time.Second, "systemd-run", "--quiet", "--collect", "--unit", d.rootlessUnit, "--property=Type=exec", "--property=User=pulse-agent", "--property=Group=pulse-agent", "--",
			"/usr/bin/env", "HOME="+d.home, "XDG_RUNTIME_DIR="+filepath.Join("/run/user", strconv.Itoa(d.uid)),
			"/usr/bin/podman", "system", "service", "--time=0", "unix://"+d.rootlessSock)
	}
	rootlessQualWaitSocket(t, d.rootlessSock)
	rootlessQualRuntimePing(t, d, true)
	rootlessQualCaptureIdentity(t, d)
}

func rootlessQualDockerStartArgs(d rootlessQualDaemon) []string {
	return []string{"--quiet", "--collect", "--unit", d.rootlessUnit, "--property=Type=exec", "--property=User=pulse-agent", "--property=Group=pulse-agent", "--",
		"/usr/bin/env", "HOME=" + d.home, "XDG_RUNTIME_DIR=" + filepath.Dir(d.rootlessSock), "DOCKERD_ROOTLESS_ROOTLESSKIT_NET=slirp4netns", "DOCKERD_ROOTLESS_ROOTLESSKIT_PORT_DRIVER=none",
		"/usr/bin/dockerd-rootless.sh", "--host=unix://" + d.rootlessSock, "--data-root=" + filepath.Join(d.home, "docker", "data"), "--exec-root=" + filepath.Join(d.home, "docker", "exec"), "--pidfile=" + filepath.Join(d.home, "docker", "dockerd.pid"), "--storage-driver=vfs", "--iptables=false", "--bridge=none"}
}

func rootlessQualStartRootful(t *testing.T, d rootlessQualDaemon) {
	t.Helper()
	rootlessQualBestEffortStop(d.rootfulUnit)
	rootlessQualCommand(t, 10*time.Second, "install", "-d", "-o", "root", "-g", "root", "-m", "0755", filepath.Dir(d.rootfulSock))
	if d.runtime == "docker" {
		rootlessQualCommand(t, 20*time.Second, "systemd-run", "--quiet", "--collect", "--unit", d.rootfulUnit, "--property=Type=exec", "--",
			"/usr/bin/dockerd", "--host=unix://"+d.rootfulSock, "--data-root=/var/lib/pulse-rootful-docker", "--exec-root=/run/pulse-rootful-docker", "--pidfile=/run/pulse-rootful-docker.pid", "--storage-driver=vfs", "--iptables=false", "--bridge=none")
	} else {
		rootlessQualCommand(t, 20*time.Second, "systemd-run", "--quiet", "--collect", "--unit", d.rootfulUnit, "--property=Type=exec", "--",
			"/usr/bin/podman", "system", "service", "--time=0", "unix://"+d.rootfulSock)
	}
	rootlessQualWaitSocket(t, d.rootfulSock)
	rootlessQualCommand(t, 10*time.Second, "chmod", "0660", d.rootfulSock)
	rootlessQualRuntimePing(t, d, false)
}

func rootlessQualStartOtherRootless(t *testing.T, d rootlessQualDaemon) (string, string) {
	t.Helper()
	other := d
	if d.runtime == "docker" {
		other.runtime, other.rootlessUnit = "podman", "pulse-rootless-podman-ambiguity"
		other.rootlessSock = filepath.Join("/run/user", strconv.Itoa(d.uid), "podman", "podman.sock")
	} else {
		other.runtime, other.rootlessUnit = "docker", "pulse-rootless-docker-ambiguity"
		other.rootlessSock = filepath.Join("/run/user", strconv.Itoa(d.uid), "docker.sock")
	}
	rootlessQualStartRootless(t, other)
	return other.rootlessUnit, other.rootlessSock
}

func rootlessQualOtherUnit(runtimeKind string) string {
	if runtimeKind == "docker" {
		return "pulse-rootless-podman-ambiguity"
	}
	return "pulse-rootless-docker-ambiguity"
}

func rootlessQualCreateFixture(t *testing.T, d rootlessQualDaemon, rootless bool) {
	t.Helper()
	cli := rootlessQualCLI(d, rootless)
	rootlessQualCommand(t, 2*time.Minute, cli[0], append(cli[1:], "build", "--network=none", "-t", rootlessQualFixture, "-f", filepath.Join(d.home, "fixture", "Containerfile"), filepath.Join(d.home, "fixture"))...)
	rootlessQualCommand(t, 30*time.Second, cli[0], append(cli[1:], rootlessQualRunningFixtureArgs()...)...)
	if out, err := rootlessQualCommandError(30*time.Second, cli[0], append(cli[1:], "run", "--name", rootlessQualExitedName, rootlessQualFixture, "true")...); err != nil {
		t.Fatalf("create exited fixture: %v\n%s", err, out)
	}
}

func rootlessQualRunningFixtureArgs() []string {
	return []string{"run", "-d", "--restart=always", "--name", rootlessQualRunningName, rootlessQualFixture, "sleep", "3600"}
}

func rootlessQualCLI(d rootlessQualDaemon, rootless bool) []string {
	if d.runtime == "docker" {
		host := d.rootfulSock
		if rootless {
			host = d.rootlessSock
			return []string{"runuser", "-u", "pulse-agent", "--", "env", "HOME=" + d.home, "XDG_RUNTIME_DIR=" + filepath.Dir(host), "DOCKER_HOST=unix://" + host, "docker"}
		}
		return []string{"docker", "--host", "unix://" + host}
	}
	host := d.rootfulSock
	if rootless {
		host = d.rootlessSock
		return []string{"runuser", "-u", "pulse-agent", "--", "env", "HOME=" + d.home, "XDG_RUNTIME_DIR=" + filepath.Join("/run/user", strconv.Itoa(d.uid)), "podman", "--url", "unix://" + host}
	}
	return []string{"podman", "--url", "unix://" + host}
}

func rootlessQualRuntimePing(t *testing.T, d rootlessQualDaemon, rootless bool) {
	t.Helper()
	cli := rootlessQualCLI(d, rootless)
	rootlessQualCommand(t, 30*time.Second, cli[0], append(cli[1:], "info")...)
}

func rootlessQualRuntimeBaseline(t *testing.T, d rootlessQualDaemon, rootless bool) rootlessQualBaseline {
	t.Helper()
	cli := rootlessQualCLI(d, rootless)
	format := "{{.Names}}|{{.Image}}|{{.State}}"
	out := rootlessQualCommand(t, 30*time.Second, cli[0], append(cli[1:], "ps", "-a", "--format", format)...)
	return rootlessQualBaselineFromPSOutput(out)
}

func rootlessQualBaselineFromPSOutput(out string) rootlessQualBaseline {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	var normalized []rootlessQualContainerSemantic
	for _, line := range lines {
		parts := strings.SplitN(strings.TrimSpace(line), "|", 3)
		if len(parts) == 3 {
			normalized = append(normalized, rootlessQualContainerSemantic{
				Name:  strings.TrimSpace(strings.TrimPrefix(parts[0], "/")),
				Image: strings.TrimSpace(parts[1]),
				State: strings.TrimSpace(parts[2]),
			})
		}
	}
	sort.Slice(normalized, func(i, j int) bool { return normalized[i].Name < normalized[j].Name })
	return rootlessQualBaseline{Count: len(normalized), SemanticDigest: rootlessQualHashJSON(normalized)}
}

func rootlessQualWaitDirect(t *testing.T, fixture *secureRuntimeLabFixture, after time.Time, runtimeKind, semanticDigest string, timeout time.Duration) secureRuntimeDockerReport {
	t.Helper()
	return rootlessQualWaitReport(t, fixture, after, timeout, func(report agentsdocker.Report) bool {
		return rootlessQualComplete(report) && report.Host.CollectionMode != agentsdocker.CollectionModeTypedHelperSummary &&
			report.Host.Runtime == runtimeKind && len(report.Containers) == 2 && rootlessQualSemanticDigest(report) == semanticDigest
	})
}

func rootlessQualWaitReport(t *testing.T, fixture *secureRuntimeLabFixture, after time.Time, timeout time.Duration, predicate func(agentsdocker.Report) bool) secureRuntimeDockerReport {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		for _, candidate := range fixture.dockerSnapshot() {
			if candidate.ReceivedAt.After(after) && predicate(candidate.Report) {
				return candidate
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for container report after %s", after.Format(time.RFC3339Nano))
	return secureRuntimeDockerReport{}
}

func rootlessQualComplete(report agentsdocker.Report) bool {
	return report.InventoryComplete == nil || *report.InventoryComplete
}

func rootlessQualAssertHelperSummaryOnly(t *testing.T, report agentsdocker.Report, runtimeKind string) {
	t.Helper()
	if report.InventoryComplete == nil || !*report.InventoryComplete {
		t.Fatal("typed-helper runtime summary did not declare complete inventory")
	}
	if report.Host.Runtime != runtimeKind || report.Host.CollectionMode != agentsdocker.CollectionModeTypedHelperSummary {
		t.Fatalf("typed-helper collection posture = runtime:%q mode:%q", report.Host.Runtime, report.Host.CollectionMode)
	}
	if len(report.Images) != 0 || len(report.Volumes) != 0 || len(report.Networks) != 0 || len(report.Services) != 0 || len(report.Tasks) != 0 || len(report.Nodes) != 0 || len(report.Secrets) != 0 || len(report.Configs) != 0 || report.StorageUsage != nil {
		t.Fatal("typed-helper runtime summary fabricated unsupported secondary inventories")
	}
	for _, container := range report.Containers {
		if container.ImageDigest != "" || container.Health != "" || len(container.HealthcheckTargets) != 0 || container.CPUPercent != 0 || container.MemoryUsageBytes != 0 || container.MemoryLimitBytes != 0 || container.MemoryPercent != 0 || container.UptimeSeconds != 0 || container.RestartCount != 0 || container.ExitCode != 0 || container.OOMKilled != nil || container.StartedAt != nil || container.FinishedAt != nil || len(container.Ports) != 0 || len(container.Labels) != 0 || len(container.Env) != 0 || len(container.Networks) != 0 || container.NetworkRXBytes != 0 || container.NetworkTXBytes != 0 || container.WritableLayerBytes != 0 || container.RootFilesystemBytes != 0 || container.BlockIO != nil || len(container.Mounts) != 0 || container.Podman != nil || container.UpdateStatus != nil {
			t.Fatalf("typed-helper container %q escaped the summary-only boundary: %+v", container.ID, container)
		}
	}
}

func rootlessQualDigestReport(report agentsdocker.Report) rootlessQualReportDigest {
	type stats struct {
		Name           string
		MemoryLimited  bool
		OOMKnown       bool
		RuntimeDetails bool
	}
	semanticRows := make([]rootlessQualContainerSemantic, 0, len(report.Containers))
	statsRows := make([]stats, 0, len(report.Containers))
	fullFieldsPresent := len(report.Containers) > 0
	runningStatsPresent := false
	for _, item := range report.Containers {
		semanticRows = append(semanticRows, rootlessQualContainerSemantic{Name: item.Name, Image: item.Image, State: item.State})
		statsRows = append(statsRows, stats{Name: item.Name, MemoryLimited: item.MemoryLimitBytes > 0, OOMKnown: item.OOMKilled != nil, RuntimeDetails: item.StartedAt != nil || item.FinishedAt != nil})
		if item.CreatedAt.IsZero() || item.Status == "" || item.OOMKilled == nil || (item.StartedAt == nil && item.FinishedAt == nil) {
			fullFieldsPresent = false
		}
		if strings.EqualFold(item.State, "running") && (item.MemoryUsageBytes > 0 || item.MemoryLimitBytes > 0 || item.CPUPercent != 0 || item.NetworkRXBytes > 0 || item.NetworkTXBytes > 0 || item.BlockIO != nil) {
			runningStatsPresent = true
		}
	}
	sort.Slice(semanticRows, func(i, j int) bool { return semanticRows[i].Name < semanticRows[j].Name })
	sort.Slice(statsRows, func(i, j int) bool { return statsRows[i].Name < statsRows[j].Name })
	imageNames := make([]string, 0, len(report.Images))
	for _, image := range report.Images {
		imageNames = append(imageNames, strings.Join(image.RepoTags, ","))
	}
	volumeNames := make([]string, 0, len(report.Volumes))
	for _, volume := range report.Volumes {
		volumeNames = append(volumeNames, volume.Name)
	}
	networkNames := make([]string, 0, len(report.Networks))
	for _, network := range report.Networks {
		networkNames = append(networkNames, network.Name)
	}
	sort.Strings(imageNames)
	sort.Strings(volumeNames)
	sort.Strings(networkNames)
	secondary := map[string]any{"images": imageNames, "volumes": volumeNames, "networks": networkNames}
	inventory := map[string]any{"containers": semanticRows, "runtime": report.Host.Runtime, "mode": report.Host.CollectionMode}
	return rootlessQualReportDigest{
		Count: len(report.Containers), InventoryDigest: rootlessQualHashJSON(inventory), SemanticDigest: rootlessQualHashJSON(semanticRows),
		StatsDigest: rootlessQualHashJSON(statsRows), StatsPresent: runningStatsPresent,
		FullFieldsPresent: fullFieldsPresent, SecondaryDigest: rootlessQualHashJSON(secondary),
		SecondaryInventoryPresent: len(report.Images) > 0 && len(report.Networks) > 0,
	}
}

func rootlessQualSemanticDigest(report agentsdocker.Report) string {
	return rootlessQualDigestReport(report).SemanticDigest
}

func rootlessQualStableDigestEqual(left, right rootlessQualReportDigest) bool {
	return left.Count == right.Count && left.InventoryDigest == right.InventoryDigest &&
		left.SemanticDigest == right.SemanticDigest && left.SecondaryDigest == right.SecondaryDigest &&
		left.StatsPresent && right.StatsPresent
}

func rootlessQualHashJSON(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func rootlessQualDaemonID(t *testing.T, d rootlessQualDaemon, rootless bool) string {
	t.Helper()
	cli := rootlessQualCLI(d, rootless)
	if d.runtime == "docker" {
		return rootlessQualCommand(t, 30*time.Second, cli[0], append(cli[1:], "info", "--format", "{{.ID}}")...)
	}
	info := rootlessQualCommand(t, 30*time.Second, cli[0], append(cli[1:], "info", "--format", "json")...)
	var decoded map[string]any
	if err := json.Unmarshal([]byte(info), &decoded); err != nil {
		t.Fatalf("decode podman info: %v", err)
	}
	store, _ := decoded["store"].(map[string]any)
	identity := map[string]any{"graphRoot": store["graphRoot"], "runRoot": store["runRoot"], "runtime": d.runtime}
	return rootlessQualHashJSON(identity)
}

func rootlessQualDaemonRootless(t *testing.T, d rootlessQualDaemon) bool {
	t.Helper()
	cli := rootlessQualCLI(d, true)
	if d.runtime == "docker" {
		securityOptions := rootlessQualCommand(t, 30*time.Second, cli[0], append(cli[1:], "info", "--format", "{{json .SecurityOptions}}")...)
		return strings.Contains(strings.ToLower(securityOptions), "rootless")
	}
	rootless := rootlessQualCommand(t, 30*time.Second, cli[0], append(cli[1:], "info", "--format", "{{.Host.Security.Rootless}}")...)
	return strings.EqualFold(strings.TrimSpace(rootless), "true")
}

func rootlessQualUnitIdentity(t *testing.T, unit string) (int, string) {
	t.Helper()
	pidText := rootlessQualCommand(t, 10*time.Second, "systemctl", "show", unit, "--property=MainPID", "--value")
	pid, err := strconv.Atoi(pidText)
	if err != nil || pid <= 1 {
		t.Fatalf("unit %s returned invalid MainPID %q", unit, pidText)
	}
	invocation := rootlessQualCommand(t, 10*time.Second, "systemctl", "show", unit, "--property=InvocationID", "--value")
	if len(invocation) != 32 {
		t.Fatalf("unit %s returned invalid InvocationID %q", unit, invocation)
	}
	return pid, invocation
}

func rootlessQualRunUnpinnedCollector(t *testing.T, serverURL, collectorCredential string, duration time.Duration) string {
	t.Helper()
	protectedPID := secureRuntimeCollectorMainPID(t)
	stateDir, err := os.MkdirTemp("/tmp", "pulse-unpinned-probe-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(stateDir)
	collectorUID := rootlessQualUID(t, "pulse-agent")
	collectorGID := rootlessQualGID(t, "pulse-agent")
	if err := os.Chown(stateDir, collectorUID, collectorGID); err != nil {
		t.Fatalf("chown ambiguity probe state: %v", err)
	}
	if err := os.Chmod(stateDir, 0o700); err != nil {
		t.Fatalf("chmod ambiguity probe state: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()
	cmd := rootlessQualProcessGroupCommand(ctx, "runuser", "-u", "pulse-agent", "--", "env", "-u", "DOCKER_HOST", "-u", "PODMAN_HOST", "-u", "CONTAINER_HOST",
		"PULSE_URL="+serverURL, "PULSE_TOKEN="+collectorCredential, "PULSE_INTERVAL=1s", "PULSE_AGENT_ID="+secureRuntimeLabAgentID,
		"PULSE_HOSTNAME="+secureRuntimeLabHostname, "PULSE_ENABLE_HOST=false", "PULSE_ENABLE_DOCKER=true", "PULSE_ENABLE_COMMANDS=false",
		"PULSE_AGENT_ALLOW_PLAINTEXT_HTTP=true", "/usr/local/bin/pulse-agent", "--state-dir", stateDir, "--health-addr", "")
	out, err := cmd.CombinedOutput()
	if err == nil || !errors.Is(ctx.Err(), context.DeadlineExceeded) {
		t.Fatalf("unpinned ambiguity probe did not remain alive until bounded cancellation: err=%v\n%s", err, out)
	}
	if afterPID := secureRuntimeCollectorMainPID(t); afterPID != protectedPID {
		t.Fatalf("separate ambiguity probe disturbed protected collector: before=%d after=%d", protectedPID, afterPID)
	}
	return string(out)
}

func rootlessQualProcessGroupCommand(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	// The ambiguity probe crosses runuser before starting the collector. Killing
	// only runuser leaves the collector holding CombinedOutput's pipe open.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil {
			if errors.Is(err, syscall.ESRCH) {
				return os.ErrProcessDone
			}
			return err
		}
		return nil
	}
	return cmd
}

func TestRootlessQualificationCancellationKillsProbeProcessGroup(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	started := time.Now()
	cmd := rootlessQualProcessGroupCommand(ctx, "sh", "-c", "sleep 30 & child=$!; echo $child; wait")
	out, err := cmd.CombinedOutput()
	if err == nil || !errors.Is(ctx.Err(), context.DeadlineExceeded) {
		t.Fatalf("process-group probe did not reach bounded cancellation: err=%v output=%q", err, out)
	}
	if elapsed := time.Since(started); elapsed > 3*time.Second {
		t.Fatalf("process-group cancellation waited for an inherited output pipe: elapsed=%s output=%q", elapsed, out)
	}
	childPID, parseErr := strconv.Atoi(strings.TrimSpace(string(out)))
	if parseErr != nil || childPID <= 1 {
		t.Fatalf("process-group probe returned invalid child PID %q: %v", out, parseErr)
	}
	deadline := time.Now().Add(time.Second)
	for {
		err = syscall.Kill(childPID, 0)
		if errors.Is(err, syscall.ESRCH) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("probe child %d survived process-group cancellation: %v", childPID, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func rootlessQualServicePin(t *testing.T, runtimeKind string) string {
	t.Helper()
	environment := secureRuntimeSystemdProperty(t, "Environment")
	key := "DOCKER_HOST=unix://"
	if runtimeKind == "podman" {
		key = "CONTAINER_HOST=unix://"
	}
	for _, field := range strings.Fields(environment) {
		field = strings.Trim(field, `"`)
		if strings.HasPrefix(field, key) {
			return strings.TrimPrefix(field, key)
		}
	}
	t.Fatalf("collector unit lacks %s pin: %s", key, environment)
	return ""
}

func rootlessQualRootfulAccessDenied(t *testing.T, d rootlessQualDaemon) bool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "runuser", "-u", "pulse-agent", "--", "curl", "-fsS", "--max-time", "2", "--unix-socket", d.rootfulSock, "http://runtime/_ping")
	return cmd.Run() != nil
}

func rootlessQualDualSocketEvidence(t *testing.T, uid int) []map[string]any {
	t.Helper()
	result := make([]map[string]any, 0, 2)
	for _, item := range []struct{ runtime, path string }{
		{"docker", filepath.Join("/run/user", strconv.Itoa(uid), "docker.sock")},
		{"podman", filepath.Join("/run/user", strconv.Itoa(uid), "podman", "podman.sock")},
	} {
		socketUID, socketGID, mode := rootlessQualSocketIdentity(t, item.path)
		result = append(result, map[string]any{
			"runtime": item.runtime, "path": item.path, "uid": socketUID, "gid": socketGID,
			"mode": mode, "type": "unix", "symlink": false,
		})
		_ = os.Remove(item.path + ".qualification-identity")
	}
	return result
}

func rootlessQualAssertHelperNetworkDenied(t *testing.T) bool {
	t.Helper()
	rootlessQualCommand(t, 10*time.Second, "ip", "link", "add", "pulse-rootless-canary", "type", "dummy")
	t.Cleanup(func() { _ = exec.Command("ip", "link", "del", "pulse-rootless-canary").Run() })
	rootlessQualCommand(t, 10*time.Second, "ip", "address", "add", "192.0.2.1/32", "dev", "pulse-rootless-canary")
	rootlessQualCommand(t, 10*time.Second, "ip", "link", "set", "pulse-rootless-canary", "up")
	observations := secureRuntimeAssertHelperOutboundNetworkDenied(t)
	return observations["helper_namespace_connection"] == "denied" && observations["host_canary_reachable"] == true
}

func rootlessQualCaptureIdentity(t *testing.T, d rootlessQualDaemon) {
	t.Helper()
	uid, gid, mode := rootlessQualSocketIdentity(t, d.rootlessSock)
	cli := rootlessQualCLI(d, true)
	version := rootlessQualCommand(t, 30*time.Second, cli[0], append(cli[1:], "version", "--format", "{{.Server.Version}}")...)
	if d.runtime == "podman" {
		version = rootlessQualCommand(t, 30*time.Second, cli[0], append(cli[1:], "version", "--format", "{{.Server.Version}}")...)
	}
	rootlessQualWriteJSON(t, d.rootlessSock+".qualification-identity", rootlessQualIdentity{RuntimeVersion: version, SocketUID: uid, SocketGID: gid, SocketMode: mode})
}

func rootlessQualReadIdentityRecord(t *testing.T, d rootlessQualDaemon) rootlessQualIdentity {
	t.Helper()
	path := d.rootlessSock + ".qualification-identity"
	raw := rootlessQualReadFile(t, path)
	var identity rootlessQualIdentity
	if err := json.Unmarshal(raw, &identity); err != nil {
		t.Fatalf("decode runtime identity: %v", err)
	}
	return identity
}

func rootlessQualSocketIdentity(t *testing.T, path string) (int, int, string) {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("inspect socket identity %s: %v", path, err)
	}
	if info.Mode()&os.ModeSocket == 0 || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0o600 != 0o600 || info.Mode().Perm()&0o006 != 0 {
		t.Fatalf("unsafe rootless socket mode at %s: %s", path, info.Mode())
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatalf("socket %s lacks Unix stat identity", path)
	}
	return int(stat.Uid), int(stat.Gid), fmt.Sprintf("%04o", info.Mode().Perm())
}

func rootlessQualArtifactIdentities(t *testing.T, installerPath string) rootlessQualArtifacts {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	collectorPath := strings.TrimSpace(os.Getenv("PULSE_SECURE_RUNTIME_COLLECTOR"))
	helperPath := strings.TrimSpace(os.Getenv("PULSE_SECURE_RUNTIME_HELPER"))
	return rootlessQualArtifacts{
		QualificationTest: rootlessQualGoArtifact(t, executable, "dockeragent.test"),
		Collector:         rootlessQualGoArtifact(t, collectorPath, "pulse-agent"),
		Helper:            rootlessQualGoArtifact(t, helperPath, "pulse-agent-helper"),
		Installer: rootlessQualInstallerArtifact{
			PathBasename: filepath.Base(installerPath), SHA256: secureRuntimeHash(rootlessQualReadFile(t, installerPath)),
		},
	}
}

func rootlessQualGoArtifact(t *testing.T, path, basename string) rootlessQualArtifact {
	t.Helper()
	info, err := buildinfo.ReadFile(path)
	if err != nil {
		t.Fatalf("read Go build metadata for %s: %v", path, err)
	}
	artifact := rootlessQualArtifact{
		PathBasename: filepath.Base(path), SHA256: secureRuntimeHash(rootlessQualReadFile(t, path)),
		Package: info.Path, GoVersion: info.GoVersion,
	}
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			artifact.VCSRevision = setting.Value
		case "vcs.modified":
			artifact.VCSModified = setting.Value == "true"
		}
	}
	wantCommit := strings.TrimSpace(os.Getenv("PULSE_ROOTLESS_SOURCE_COMMIT"))
	if artifact.PathBasename != basename || artifact.Package == "" || artifact.VCSRevision != wantCommit || artifact.VCSModified {
		t.Fatalf("qualification artifact is not an exact clean source build: %+v", artifact)
	}
	return artifact
}

func rootlessQualSourceHashes(t *testing.T) map[string]string {
	t.Helper()
	path := strings.TrimSpace(os.Getenv("PULSE_ROOTLESS_SOURCE_HASHES"))
	if !filepath.IsAbs(path) {
		t.Fatalf("PULSE_ROOTLESS_SOURCE_HASHES must be absolute: %q", path)
	}
	var hashes map[string]string
	if err := json.Unmarshal(rootlessQualReadFile(t, path), &hashes); err != nil {
		t.Fatalf("decode source hashes: %v", err)
	}
	if len(hashes) == 0 {
		t.Fatal("source hash map is empty")
	}
	return hashes
}

func rootlessQualUninstallPulse(t *testing.T, installerPath, serverURL, collectorCredential string) {
	t.Helper()
	secureRuntimeRunInstallerWithCollectorCredential(t, installerPath, serverURL, collectorCredential, "--uninstall")
}

func rootlessQualAssertPulseRemoved(t *testing.T) {
	t.Helper()
	for _, path := range secureRuntimeInstalledPaths {
		if _, err := os.Lstat(path); err == nil {
			t.Fatalf("Pulse cleanup left %s", path)
		} else if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("inspect cleanup path %s: %v", path, err)
		}
	}
}

func rootlessQualRemoveRuntimeState(t *testing.T, d rootlessQualDaemon) {
	t.Helper()
	for _, path := range []string{d.home, filepath.Join("/run/user", strconv.Itoa(d.uid)), "/var/lib/pulse-rootful-docker", "/var/lib/containers/storage"} {
		if err := os.RemoveAll(path); err != nil {
			t.Fatalf("remove runtime state %s: %v", path, err)
		}
	}
}

func rootlessQualStopUserManager(t *testing.T, d rootlessQualDaemon) {
	t.Helper()
	rootlessQualCommand(t, 10*time.Second, "loginctl", "disable-linger", "pulse-agent")
	rootlessQualStopUnit(t, fmt.Sprintf("user@%d.service", d.uid))
}

func rootlessQualUserStateClean(d rootlessQualDaemon) bool {
	for _, path := range []string{d.home, filepath.Join("/run/user", strconv.Itoa(d.uid)), "/var/lib/pulse-rootful-docker", "/var/lib/containers/storage", "/var/lib/systemd/linger/pulse-agent"} {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			return false
		}
	}
	return true
}

func rootlessQualStopUnit(t *testing.T, unit string) {
	t.Helper()
	rootlessQualCommand(t, 30*time.Second, "systemctl", "stop", unit)
	resetOutput, resetErr := rootlessQualCommandError(30*time.Second, "systemctl", "reset-failed", unit)
	if resetErr == nil {
		return
	}
	loadState, stateErr := rootlessQualCommandError(30*time.Second, "systemctl", "show", "--property=LoadState", "--value", unit)
	if stateErr == nil && strings.TrimSpace(loadState) == "not-found" {
		return
	}
	t.Fatalf("systemctl reset-failed %s: %v\n%s", unit, resetErr, resetOutput)
}

func rootlessQualBestEffortStop(units ...string) {
	for _, unit := range units {
		if unit != "" {
			_ = exec.Command("systemctl", "stop", unit).Run()
			_ = exec.Command("systemctl", "reset-failed", unit).Run()
		}
	}
}

func rootlessQualWaitSocket(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSocket != 0 && info.Mode()&os.ModeSymlink == 0 {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("runtime socket did not appear: %s", path)
}

func rootlessQualUID(t *testing.T, user string) int {
	t.Helper()
	uidText := rootlessQualCommand(t, 10*time.Second, "id", "-u", user)
	uid, err := strconv.Atoi(uidText)
	if err != nil {
		t.Fatalf("parse UID %q: %v", uidText, err)
	}
	return uid
}

func rootlessQualGID(t *testing.T, user string) int {
	t.Helper()
	gidText := rootlessQualCommand(t, 10*time.Second, "id", "-g", user)
	gid, err := strconv.Atoi(gidText)
	if err != nil {
		t.Fatalf("parse GID %q: %v", gidText, err)
	}
	return gid
}

func rootlessQualWriteSubID(t *testing.T, path, line string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), strings.SplitN(line, ":", 2)[0]+":") {
		return
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := fmt.Fprintln(f, line); err != nil {
		t.Fatal(err)
	}
}

func rootlessQualCommand(t *testing.T, timeout time.Duration, name string, args ...string) string {
	t.Helper()
	out, err := rootlessQualCommandError(timeout, name, args...)
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(out)
}

func rootlessQualCommandError(timeout time.Duration, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if ctx.Err() != nil {
		return string(out), fmt.Errorf("command timed out: %w", ctx.Err())
	}
	return string(out), err
}

func rootlessQualReadFile(t *testing.T, path string) []byte {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, 64<<20))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func rootlessQualWriteJSON(t *testing.T, path string, value any) {
	t.Helper()
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(raw, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmp, path); err != nil {
		t.Fatal(err)
	}
}

func rootlessQualValidateReceipt(receipt rootlessQualReceipt, wantRuns int) error {
	if receipt.SchemaVersion != 1 || receipt.Kind != "pulse-secure-runtime-rootless-qualification" || receipt.Result != "passed" {
		return errors.New("invalid top-level qualification identity")
	}
	if len(receipt.SourceCommit) != 40 || len(receipt.Runs) != wantRuns || receipt.Artifacts.QualificationTest.PathBasename != "dockeragent.test" || receipt.Artifacts.QualificationTest.VCSModified || len(receipt.SourceHashes) == 0 {
		return errors.New("invalid source, artifact, or runtime binding")
	}
	seen := map[string]bool{}
	for _, run := range receipt.Runs {
		rt := run.Runtime
		if seen[rt.Runtime] || (rt.Runtime != "docker" && rt.Runtime != "podman") || !rt.DaemonRootless || rt.CollectorUID <= 0 || rt.SocketUID != rt.CollectorUID || rt.SocketPath == "" || rt.DaemonID == "" || rt.SocketType != "unix" || rt.SocketSymlink {
			return fmt.Errorf("invalid runtime identity for %q", rt.Runtime)
		}
		seen[rt.Runtime] = true
		if rt.SocketMode != "0600" && rt.SocketMode != "0660" {
			return fmt.Errorf("runtime %s socket mode is not owner-private", rt.Runtime)
		}
		if len(run.Scenarios) != len(rootlessQualScenarioOrder) {
			return fmt.Errorf("runtime %s scenario count = %d", rt.Runtime, len(run.Scenarios))
		}
		for i, scenario := range run.Scenarios {
			if scenario.Name != rootlessQualScenarioOrder[i] || scenario.Result != "passed" || scenario.StartedAt == "" || scenario.CompletedAt == "" || len(scenario.Evidence) == 0 {
				return fmt.Errorf("runtime %s scenario %d is invalid", rt.Runtime, i)
			}
			reportExpected := scenario.Name != "dual_socket_ambiguity_refusal" && scenario.Name != "authority_isolation" && scenario.Name != "cleanup"
			if reportExpected != (scenario.ReportSequence != nil && scenario.ReportStreamID != nil && *scenario.ReportStreamID != "") {
				return fmt.Errorf("runtime %s scenario %s report binding is invalid", rt.Runtime, scenario.Name)
			}
		}
	}
	return nil
}

func TestRootlessQualificationReceiptContract(t *testing.T) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	sequence := uint64(1)
	scenarios := make([]rootlessQualScenario, 0, len(rootlessQualScenarioOrder))
	for _, name := range rootlessQualScenarioOrder {
		scenario := rootlessQualScenario{Name: name, Result: "passed", StartedAt: now, CompletedAt: now, Evidence: map[string]any{"observed": true}}
		if name != "dual_socket_ambiguity_refusal" && name != "authority_isolation" && name != "cleanup" {
			scenario.ReportSequence = &sequence
			stream := "stream"
			scenario.ReportStreamID = &stream
		}
		scenarios = append(scenarios, scenario)
	}
	receipt := rootlessQualReceipt{
		SchemaVersion: 1, Kind: "pulse-secure-runtime-rootless-qualification", Result: "passed", SourceCommit: strings.Repeat("a", 40),
		SourceHashes: map[string]string{"source.go": strings.Repeat("b", 64)},
		Artifacts:    rootlessQualArtifacts{QualificationTest: rootlessQualArtifact{PathBasename: "dockeragent.test", VCSRevision: strings.Repeat("a", 40)}},
		Runs:         []rootlessQualRun{{Runtime: rootlessQualRuntime{Runtime: "docker", DaemonID: "daemon", CollectorUID: 1000, SocketUID: 1000, SocketMode: "0600", SocketPath: "/run/user/1000/docker.sock", SocketType: "unix", DaemonRootless: true}, Scenarios: scenarios}},
	}
	if err := rootlessQualValidateReceipt(receipt, 1); err != nil {
		t.Fatal(err)
	}
	receipt.Runs[0].Scenarios[3].ReportSequence = nil
	if err := rootlessQualValidateReceipt(receipt, 1); err == nil {
		t.Fatal("receipt validator accepted a report-producing scenario without a sequence")
	}
}

func TestRootlessQualificationBaselineUsesReportSemanticShape(t *testing.T) {
	baseline := rootlessQualBaselineFromPSOutput(strings.Join([]string{
		"/pulse-rootless-running|pulse-rootless-qualification:v1|running",
		"pulse-rootless-exited|pulse-rootless-qualification:v1|exited",
	}, "\n"))
	report := agentsdocker.Report{Containers: []agentsdocker.Container{
		{Name: "pulse-rootless-exited", Image: "pulse-rootless-qualification:v1", State: "exited"},
		{Name: "pulse-rootless-running", Image: "pulse-rootless-qualification:v1", State: "running"},
	}}
	digest := rootlessQualDigestReport(report)
	if baseline.Count != digest.Count || baseline.SemanticDigest != digest.SemanticDigest {
		t.Fatalf("runtime baseline and report semantics diverged: baseline=%+v report=%+v", baseline, digest)
	}
}

func TestRootlessQualificationStopAcceptsCollectedTransientUnit(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "systemctl.log")
	systemctlPath := filepath.Join(tempDir, "systemctl")
	script := `#!/bin/sh
printf '%s\n' "$*" >>"$ROOTLESS_QUAL_SYSTEMCTL_LOG"
case "$1" in
  stop) exit 0 ;;
  reset-failed) echo 'Unit is not loaded.' >&2; exit 1 ;;
  show) echo 'not-found'; exit 0 ;;
esac
exit 2
`
	if err := os.WriteFile(systemctlPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ROOTLESS_QUAL_SYSTEMCTL_LOG", logPath)
	t.Setenv("PATH", tempDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	rootlessQualStopUnit(t, "pulse-rootless-docker")
	logText := string(rootlessQualReadFile(t, logPath))
	for _, command := range []string{
		"stop pulse-rootless-docker",
		"reset-failed pulse-rootless-docker",
		"show --property=LoadState --value pulse-rootless-docker",
	} {
		if !strings.Contains(logText, command) {
			t.Fatalf("systemctl lifecycle did not execute %q: %s", command, logText)
		}
	}
}

func TestRootlessQualificationRunningFixtureRestartsWithDaemon(t *testing.T) {
	args := rootlessQualRunningFixtureArgs()
	if !slices.Contains(args, "--restart=always") {
		t.Fatalf("running qualification fixture lacks daemon-restart policy: %q", args)
	}
}

func TestRootlessQualificationGoSchemaPassesPythonValidator(t *testing.T) {
	commit := strings.Repeat("a", 40)
	digest := strings.Repeat("b", 64)
	started := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	receipt := rootlessQualReceipt{
		SchemaVersion: 1, Kind: "pulse-secure-runtime-rootless-qualification", Result: "passed",
		SourceCommit: commit, StartedAt: started.Format(time.RFC3339Nano),
		CompletedAt:  started.Add(2 * time.Minute).Format(time.RFC3339Nano),
		SourceHashes: map[string]string{"internal/dockeragent/agent.go": digest, "scripts/install.sh": digest},
		Artifacts: rootlessQualArtifacts{
			QualificationTest: rootlessQualArtifact{PathBasename: "dockeragent.test", SHA256: digest, Package: "github.com/rcourtman/pulse-go-rewrite/scripts/installtests.test", GoVersion: "go1.25.0", VCSRevision: commit},
			Collector:         rootlessQualArtifact{PathBasename: "pulse-agent", SHA256: digest, Package: "github.com/rcourtman/pulse-go-rewrite/cmd/pulse-agent", GoVersion: "go1.25.0", VCSRevision: commit},
			Helper:            rootlessQualArtifact{PathBasename: "pulse-agent-helper", SHA256: digest, Package: "github.com/rcourtman/pulse-go-rewrite/cmd/pulse-agent-helper", GoVersion: "go1.25.0", VCSRevision: commit},
			Installer:         rootlessQualInstallerArtifact{PathBasename: "install.sh", SHA256: digest},
		},
	}
	for index, runtimeKind := range []string{"docker", "podman"} {
		receipt.Runs = append(receipt.Runs, rootlessQualValidatorFixtureRun(runtimeKind, 1000, index, started.Add(time.Duration(index)*time.Minute), digest))
	}
	path := filepath.Join(t.TempDir(), "receipt.json")
	rootlessQualWriteJSON(t, path, receipt)
	validator := repoFile("scripts", "release_control", "secure_runtime_rootless_attestation_v1.py")
	program := `import importlib.util, pathlib, sys
spec=importlib.util.spec_from_file_location("validator", pathlib.Path(sys.argv[1]))
module=importlib.util.module_from_spec(spec); spec.loader.exec_module(module)
module.parse_receipt_bytes(pathlib.Path(sys.argv[2]).read_bytes())
`
	cmd := exec.Command("python3", "-I", "-c", program, validator, path)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Go receipt schema failed the real Python validator: %v\n%s", err, output)
	}
}

func rootlessQualValidatorFixtureRun(runtimeKind string, uid, index int, began time.Time, digest string) rootlessQualRun {
	socketPath := filepath.Join("/run/user", strconv.Itoa(uid), "docker.sock")
	if runtimeKind == "podman" {
		socketPath = filepath.Join("/run/user", strconv.Itoa(uid), "podman", "podman.sock")
	}
	daemonID := runtimeKind + "-daemon"
	direct := func(pid int) map[string]any {
		return map[string]any{
			"collector_pid": pid, "service_pid": pid, "collection_path": "collector-owned-rootless-socket",
			"inventory_complete": true, "inventory_count": 2, "semantic_sha256": digest,
			"full_fields_present": true, "stats_present": true, "secondary_structure_sha256": digest,
			"daemon_id": daemonID, "daemon_rootless": true, "socket_path": socketPath,
			"socket_uid": uid, "socket_gid": uid, "socket_mode": "0600", "socket_type": "unix", "socket_symlink": false,
		}
	}
	stream := func(value string) *string { return &value }
	sequence := func(value uint64) *uint64 { return &value }
	makeScenario := func(offset int, name string, streamID *string, seq *uint64, evidence map[string]any) rootlessQualScenario {
		start := began.Add(time.Duration(offset) * time.Second)
		return rootlessQualScenario{Name: name, Result: "passed", StartedAt: start.Format(time.RFC3339Nano), CompletedAt: start.Add(time.Second).Format(time.RFC3339Nano), ReportStreamID: streamID, ReportSequence: seq, Evidence: evidence}
	}
	fresh := direct(100 + index*10)
	migration := direct(101 + index*10)
	migration["legacy_profile"] = "root-command-capable"
	migration["target_profile"] = "typed-helper-monitoring-only"
	migration["authority_reduced"] = true
	migration["legacy_collector_pid"] = 150 + index*10
	restart := direct(102 + index*10)
	restart["previous_collector_pid"] = 101 + index*10
	restart["previous_report_stream_id"] = fmt.Sprintf("%s-migration-%d", runtimeKind, index)
	daemon := direct(102 + index*10)
	daemon["previous_daemon_pid"] = 200 + index*10
	daemon["daemon_pid"] = 201 + index*10
	daemon["previous_daemon_invocation_id"] = fmt.Sprintf("%s-old-invocation", runtimeKind)
	daemon["daemon_invocation_id"] = fmt.Sprintf("%s-new-invocation", runtimeKind)
	fallback := map[string]any{
		"collector_pid": 102 + index*10, "collection_mode": "typed-helper-summary", "direct_runtime_available": false,
		"helper_fallback": true, "inventory_complete": true, "inventory_count": 2, "rootful_baseline_inventory_count": 2,
		"semantic_sha256": digest, "rootful_baseline_semantic_sha256": digest, "full_fields_present": false,
		"stats_present": false, "secondary_structure_sha256": "", "container_actions_enabled": false,
		"container_updates_enabled": false, "collector_restart_count": 0,
	}
	liveSockets := []map[string]any{
		{"runtime": "docker", "path": filepath.Join("/run/user", strconv.Itoa(uid), "docker.sock"), "uid": uid, "gid": uid, "mode": "0600", "type": "unix", "symlink": false},
		{"runtime": "podman", "path": filepath.Join("/run/user", strconv.Itoa(uid), "podman", "podman.sock"), "uid": uid, "gid": uid, "mode": "0600", "type": "unix", "symlink": false},
	}
	ambiguity := map[string]any{
		"protected_collector_pid": 102 + index*10, "live_sockets": liveSockets,
		"probe_kind": "separate-unpinned-collector", "admission_refused": true, "fail_closed": true,
		"daemon_probe_count": 0, "container_actions_enabled": false, "collector_restart_count": 0,
	}
	pin := direct(103 + index*10)
	pin["previous_collector_pid"] = 102 + index*10
	pin["previous_report_stream_id"] = fmt.Sprintf("%s-restart-%d", runtimeKind, index)
	pin["pin_source"] = "root-owned-systemd-unit"
	pin["pinned_socket_path"] = socketPath
	pin["socket_absent_observed"] = true
	pin["fallback_report_sequence"] = 1
	pin["recovery_report_sequence"] = 2
	pin["recovered_socket_path"] = socketPath
	pin["selected_socket_path"] = socketPath
	pin["recovered_socket_uid"] = uid
	pin["recovered_socket_gid"] = uid
	pin["recovered_socket_mode"] = "0600"
	pin["recovered_socket_type"] = "unix"
	pin["recovered_socket_symlink"] = false
	pin["candidate_count"] = 1
	pin["daemon_probe_count"] = 1
	pin["collector_restart_count"] = 1
	parity := map[string]any{
		"collector_pid": 103 + index*10, "baseline_kind": "root-client-same-rootless-daemon",
		"baseline_inventory_count": 2, "collector_inventory_count": 2,
		"baseline_semantic_sha256": digest, "collector_semantic_sha256": digest,
		"collector_full_fields_present": true, "collector_stats_present": true,
		"collector_secondary_inventory_present": true,
	}
	authority := map[string]any{
		"collector_pid": 103 + index*10, "collector_uid": uid, "effective_uid": uid, "effective_root": false,
		"safe_profile_enabled": true, "commands_enabled": false, "privileged_helper_enabled": true,
		"reduction_request_observed": true, "collector_command_transport_present": false,
		"collector_command_session_present": false, "container_actions_enabled": false, "container_updates_enabled": false,
		"rootful_socket_access": false, "helper_network_access": false,
	}
	run := rootlessQualRun{
		Host:    rootlessQualHost{MachineID: fmt.Sprintf("machine-%s", runtimeKind), Architecture: "amd64", Kernel: "Linux fixture", SystemdVersion: "systemd 255"},
		Runtime: rootlessQualRuntime{Runtime: runtimeKind, RuntimeVersion: "1.0.0", DaemonID: daemonID, CollectorUID: uid, SocketPath: socketPath, SocketUID: uid, SocketGID: uid, SocketMode: "0600", SocketType: "unix", DaemonRootless: true},
	}
	run.Scenarios = []rootlessQualScenario{
		makeScenario(0, "fresh_install", stream(fmt.Sprintf("%s-fresh-%d", runtimeKind, index)), sequence(1), fresh),
		makeScenario(2, "legacy_migration", stream(fmt.Sprintf("%s-migration-%d", runtimeKind, index)), sequence(1), migration),
		makeScenario(4, "collector_restart", stream(fmt.Sprintf("%s-restart-%d", runtimeKind, index)), sequence(1), restart),
		makeScenario(6, "daemon_restart", stream(fmt.Sprintf("%s-restart-%d", runtimeKind, index)), sequence(2), daemon),
		makeScenario(8, "socket_loss_helper_fallback", stream(fmt.Sprintf("%s-restart-%d", runtimeKind, index)), sequence(3), fallback),
		makeScenario(10, "direct_recovery", stream(fmt.Sprintf("%s-restart-%d", runtimeKind, index)), sequence(4), direct(102+index*10)),
		makeScenario(12, "dual_socket_ambiguity_refusal", nil, nil, ambiguity),
		makeScenario(14, "exact_pin_recovery", stream(fmt.Sprintf("%s-update-%d", runtimeKind, index)), sequence(2), pin),
		makeScenario(16, "telemetry_parity", stream(fmt.Sprintf("%s-update-%d", runtimeKind, index)), sequence(3), parity),
		makeScenario(18, "authority_isolation", nil, nil, authority),
		makeScenario(20, "cleanup", nil, nil, map[string]any{"runtime_stopped": true, "socket_absent": true, "fixtures_removed": true, "user_state_clean": true}),
	}
	return run
}

func TestRootlessQualificationGuardAndWrapperInvariants(t *testing.T) {
	if rootlessQualHasDefaultRoute("Iface Destination Gateway\neth0 00000000 0100007F") != true || rootlessQualHasDefaultRoute("Iface Destination Gateway\nlo 0000007F 00000000") {
		t.Fatal("default-route refusal parser drifted")
	}
	raw, err := os.ReadFile(repoFile("scripts", "run-secure-runtime-rootless-qualification.sh"))
	if err != nil {
		t.Fatal(err)
	}
	script := string(raw)
	for _, required := range []string{
		`PULSE_ROOTLESS_UBUNTU_IMAGE`, `^ubuntu@sha256:`, `--network none`, `--cgroupns=private`, `--tmpfs /run`,
		`PULSE_SECURE_RUNTIME_ROOTLESS_QUALIFICATION=disposable-v1`, `dockeragent.test`,
		`run_runtime docker`, `run_runtime podman`, `PULSE_ROOTLESS_RUNTIME=${runtime_name}`, `--privileged`,
		`pulse-secure-runtime-rootless-qualification`, `qualification result != \"passed\"`,
		`openssl pkeyutl -sign -rawin -inkey`, `qualification output directory must have exact mode 0700`,
		`install -d -m 0700 /opt/pulse/packet`,
		`capture_qualification_container_diagnostics`, `journalctl --no-pager -n 2000`,
		`302a300506032b6570032100`, `len(spki) != len(prefix) + 32`,
	} {
		if !strings.Contains(script, required) {
			t.Fatalf("rootless qualification wrapper missing %q", required)
		}
	}
	for _, forbidden := range []string{"/var/run/docker.sock:", "/run/podman/podman.sock:", "/sys/fs/cgroup:/sys/fs/cgroup", "--cgroupns=host", "-v $", "--volume"} {
		if strings.Contains(script, forbidden) {
			t.Fatalf("rootless qualification wrapper contains forbidden host-runtime mount marker %q", forbidden)
		}
	}
	if strings.Contains(script, "--private-key") || strings.Contains(script, "update_seed=") {
		t.Fatal("rootless wrapper must not put its ephemeral signing key in a process argument")
	}
	createIndex := strings.Index(script, `container_id="$(docker create`)
	trackIndex := strings.Index(script, `CONTAINER_IDS+=("${container_id}")`)
	if createIndex < 0 || trackIndex < 0 || trackIndex < createIndex {
		t.Fatal("rootless wrapper must track the exact container ID only after docker create succeeds")
	}
	packetDirectoryIndex := strings.Index(script, `install -d -m 0700 /opt/pulse/packet`)
	packetCopyIndex := strings.Index(script, `docker cp "${PACKET_DIR}/." "${container_id}:/opt/pulse/packet"`)
	if packetDirectoryIndex < 0 || packetCopyIndex < 0 || packetCopyIndex < packetDirectoryIndex {
		t.Fatal("rootless wrapper must create the private packet destination in the image before artifact injection")
	}
}

func TestRootlessQualificationDockerCommandUsesSupportedNetworkDriver(t *testing.T) {
	d := rootlessQualDaemon{
		rootlessUnit: "pulse-rootless-docker",
		rootlessSock: "/run/user/996/docker.sock",
		home:         "/var/lib/pulse-rootless",
	}
	command := strings.Join(rootlessQualDockerStartArgs(d), "\x00")
	if !strings.Contains(command, "DOCKERD_ROOTLESS_ROOTLESSKIT_NET=slirp4netns") || !strings.Contains(command, "DOCKERD_ROOTLESS_ROOTLESSKIT_PORT_DRIVER=none") {
		t.Fatalf("rootless Docker command does not select the supported contained drivers: %q", command)
	}
	if strings.Contains(command, "DOCKERD_ROOTLESS_ROOTLESSKIT_NET=host") {
		t.Fatalf("rootless Docker command selected the unsupported host network driver: %q", command)
	}
}

func runRootlessQualificationWithFakeDocker(t *testing.T, mode string) (string, string, error) {
	t.Helper()
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 is required for the wrapper ownership regression")
	}
	temporary := t.TempDir()
	fakeBin := filepath.Join(temporary, "bin")
	outputDir := filepath.Join(temporary, "output")
	if err := os.MkdirAll(fakeBin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(outputDir, 0o700); err != nil {
		t.Fatal(err)
	}
	writeExecutable := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(fakeBin, name), []byte(body), 0o755); err != nil {
			t.Fatalf("write fake %s: %v", name, err)
		}
	}
	writeExecutable("git", `#!/bin/sh
case " $* " in
  *" branch --show-current "*) printf '%s\n' main ;;
  *" status --porcelain "*) : ;;
  *" rev-parse HEAD "*) printf '%040d\n' 0 | tr 0 a ;;
  *) printf 'unexpected fake git invocation: %s\n' "$*" >&2; exit 64 ;;
esac
`)
	writeExecutable("go", `#!/bin/sh
if [ "${1:-}" = env ] && [ "${2:-}" = GOARCH ]; then
  printf '%s\n' amd64
  exit 0
fi
output=''
while [ "$#" -gt 0 ]; do
  if [ "$1" = -o ]; then
    output="$2"
    break
  fi
  shift
done
[ -n "$output" ] || exit 65
printf '%s\n' fake-go-artifact >"$output"
chmod 0700 "$output"
`)
	writeExecutable("openssl", `#!/bin/sh
command_name="${1:-}"
shift || true
case "$command_name" in
  rand)
    printf '%032d\n' 0 | tr 0 a
    ;;
  genpkey)
    while [ "$#" -gt 0 ]; do
      if [ "$1" = -out ]; then printf '%s\n' fake-private-key >"$2"; exit 0; fi
      shift
    done
    exit 66
    ;;
  pkey)
    while [ "$#" -gt 0 ]; do
      if [ "$1" = -out ]; then
        python3 - "$2" <<'PY'
from pathlib import Path
import sys
Path(sys.argv[1]).write_bytes(bytes.fromhex("302a300506032b6570032100") + bytes(32))
PY
        exit 0
      fi
      shift
    done
    exit 67
    ;;
  pkeyutl)
    printf '%s' fake-signature
    ;;
  base64)
    python3 -c 'import base64,sys; sys.stdout.write(base64.b64encode(sys.stdin.buffer.read()).decode())'
    ;;
  *)
    exit 68
    ;;
esac
`)
	dockerLog := filepath.Join(temporary, "docker.log")
	dockerState := filepath.Join(temporary, "docker.state")
	dockerLabel := filepath.Join(temporary, "docker.label")
	writeExecutable("docker", `#!/bin/sh
printf '%s\n' "$*" >>"$PULSE_FAKE_DOCKER_LOG"
container_id='cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc'
case "${1:-}" in
  info) exit 0 ;;
  build) printf '%s\n' fake-build ; exit 0 ;;
  image) printf '%s\n' '[{}]' ; exit 0 ;;
  create)
    if [ "$PULSE_FAKE_DOCKER_MODE" = name-conflict ]; then
      printf '%s\n' 'Conflict. The container name is already in use.' >&2
      exit 125
    fi
    previous=''
    for argument in "$@"; do
      if [ "$previous" = --label ]; then
        printf '%s\n' "${argument#*=}" >"$PULSE_FAKE_DOCKER_LABEL"
      fi
      previous="$argument"
    done
    : >"$PULSE_FAKE_DOCKER_STATE"
    printf '%s\n' "$container_id"
    ;;
  cp)
    case "${2:-}" in
      *:*) printf '%s\n' '{}' >"$3" ;;
    esac
    ;;
  start) exit 0 ;;
  exec)
    case " $* " in
      *" ip route "*) exit 1 ;;
      *) exit 0 ;;
    esac
    ;;
  inspect)
    case " $* " in
      *".Mounts"*) exit 0 ;;
      *".Config.Labels"*)
        [ "$PULSE_FAKE_DOCKER_MODE" != inspect-failure ] || exit 1
        cat "$PULSE_FAKE_DOCKER_LABEL"
        ;;
      *) exit 70 ;;
    esac
    ;;
  rm)
    [ "$PULSE_FAKE_DOCKER_MODE" != rm-failure ] || exit 1
    rm -f "$PULSE_FAKE_DOCKER_STATE"
    ;;
  ps)
    [ ! -f "$PULSE_FAKE_DOCKER_STATE" ] || printf '%s\n' "$container_id"
    ;;
  logs) exit 0 ;;
  *) printf 'unexpected fake docker invocation: %s\n' "$*" >&2; exit 69 ;;
esac
`)
	commit := strings.Repeat("a", 40)
	command := exec.Command("bash", repoFile("scripts", "run-secure-runtime-rootless-qualification.sh"))
	command.Env = append(os.Environ(),
		"PATH="+fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"PULSE_FAKE_DOCKER_LOG="+dockerLog,
		"PULSE_FAKE_DOCKER_STATE="+dockerState,
		"PULSE_FAKE_DOCKER_LABEL="+dockerLabel,
		"PULSE_FAKE_DOCKER_MODE="+mode,
		"PULSE_ROOTLESS_UBUNTU_IMAGE=ubuntu@sha256:"+strings.Repeat("b", 64),
		"PULSE_ROOTLESS_QUALIFICATION_OUTPUT_DIR="+outputDir,
		"PULSE_ROOTLESS_QUALIFICATION_CONFIRM=I_HAVE_VERIFIED_THESE_ARE_DISPOSABLE_ROOTLESS_SYSTEMD_CONTAINERS_COMMIT_"+commit,
	)
	output, err := command.CombinedOutput()
	logBytes, readErr := os.ReadFile(dockerLog)
	if readErr != nil {
		t.Fatal(readErr)
	}
	return string(output), string(logBytes), err
}

func TestRootlessQualificationNameConflictDoesNotRemoveContainer(t *testing.T) {
	output, logText, err := runRootlessQualificationWithFakeDocker(t, "name-conflict")
	if err == nil {
		t.Fatalf("wrapper unexpectedly succeeded after Docker name conflict: %s", output)
	}
	if !strings.Contains(logText, "create --name pulse-rootless-qual-docker-") {
		t.Fatalf("fake Docker did not reach the name-conflict create; docker log=%s wrapper output=%s", logText, output)
	}
	for _, line := range strings.Split(logText, "\n") {
		if strings.HasPrefix(line, "rm ") {
			t.Fatalf("name-conflict cleanup attempted to remove an unowned container: %s", logText)
		}
	}
}

func TestRootlessQualificationCleanupFailureCannotEmitPassingPacket(t *testing.T) {
	for _, mode := range []string{"inspect-failure", "rm-failure"} {
		t.Run(mode, func(t *testing.T) {
			output, logText, err := runRootlessQualificationWithFakeDocker(t, mode)
			if err == nil {
				t.Fatalf("wrapper unexpectedly succeeded after %s: %s", mode, output)
			}
			if strings.Contains(output, "Rootless qualification passed:") {
				t.Fatalf("wrapper emitted a passing packet after %s: %s", mode, output)
			}
			if !strings.Contains(logText, ".Config.Labels") {
				t.Fatalf("wrapper did not reach strict ownership verification after %s: %s", mode, logText)
			}
			if mode == "rm-failure" && !strings.Contains(logText, "rm -f cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc") {
				t.Fatalf("wrapper did not exercise strict removal failure: %s", logText)
			}
		})
	}
}
