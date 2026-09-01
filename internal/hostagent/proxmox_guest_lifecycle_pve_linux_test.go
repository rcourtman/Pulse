//go:build linux

package hostagent

import (
	"context"
	"crypto/sha256"
	"debug/buildinfo"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
)

const nativePVEQualificationPrefix = "I_HAVE_VERIFIED_THIS_DISPOSABLE_PVE_TARGET"

var nativePVEHexDigest = regexp.MustCompile(`^[0-9a-f]{40,64}$`)
var nativePVESystemdInvocationID = regexp.MustCompile(`^[0-9a-f]{32}$`)
var nativePVEQualificationUnit = regexp.MustCompile(`^pulse-pve-qualification-[0-9a-f]{8}-[1-9][0-9]{0,8}-[1-9][0-9]{0,8}\.service$`)

type nativePVEGuestIdentity struct {
	Kind         string            `json:"kind"`
	VMID         int               `json:"vmid"`
	Node         string            `json:"node"`
	ConfigDigest string            `json:"config_digest"`
	Bridges      []string          `json:"bridges"`
	Networks     map[string]string `json:"networks"`
}

type nativePVEOperationObservation struct {
	Operation       string                                       `json:"operation"`
	Result          agentexec.ProxmoxGuestLifecycleResultPayload `json:"result"`
	Cgroup          string                                       `json:"cgroup,omitempty"`
	LinkPaths       map[string][]string                          `json:"link_paths,omitempty"`
	ActionUnitsGone bool                                         `json:"action_units_gone"`
	SidebandEmpty   bool                                         `json:"sideband_empty"`
}

type nativePVEGuestObservation struct {
	Identity   nativePVEGuestIdentity          `json:"identity"`
	Operations []nativePVEOperationObservation `json:"operations"`
	FinalState string                          `json:"final_state"`
	Emergency  bool                            `json:"emergency_cleanup"`
}

type nativePVEQualificationReceipt struct {
	SchemaVersion            int                         `json:"schema_version"`
	SourceCommit             string                      `json:"source_commit"`
	MachineID                string                      `json:"machine_id"`
	Node                     string                      `json:"node"`
	ClusterID                string                      `json:"cluster_id"`
	SupervisorUnit           string                      `json:"supervisor_unit"`
	SupervisorInvocationID   string                      `json:"supervisor_invocation_id"`
	PVEVersion               string                      `json:"pve_version"`
	KernelVersion            string                      `json:"kernel_version"`
	SystemdVersion           string                      `json:"systemd_version"`
	RunnerAnchorInvocationID string                      `json:"runner_anchor_invocation_id"`
	RunnerSHA256             string                      `json:"runner_sha256"`
	TestSHA256               string                      `json:"test_sha256"`
	ManifestSHA256           string                      `json:"manifest_sha256"`
	RunnerBuildInfo          string                      `json:"runner_build_info"`
	TestBuildInfo            string                      `json:"test_build_info"`
	StartedAt                string                      `json:"started_at"`
	CompletedAt              string                      `json:"completed_at"`
	Result                   string                      `json:"result"`
	Guests                   []nativePVEGuestObservation `json:"guests"`
}

type nativePVEQualificationManifest struct {
	SchemaVersion            int                      `json:"schema_version"`
	SourceCommit             string                   `json:"source_commit"`
	MachineID                string                   `json:"machine_id"`
	Node                     string                   `json:"node"`
	ClusterID                string                   `json:"cluster_id"`
	SupervisorUnit           string                   `json:"supervisor_unit"`
	SupervisorInvocationID   string                   `json:"supervisor_invocation_id"`
	RunnerAnchorInvocationID string                   `json:"runner_anchor_invocation_id"`
	RunnerSHA256             string                   `json:"runner_sha256"`
	TestSHA256               string                   `json:"test_sha256"`
	CreatedAt                string                   `json:"created_at"`
	Guests                   []nativePVEGuestIdentity `json:"guests"`
}

type nativePVECleanupGuest struct {
	Identity  nativePVEGuestIdentity `json:"identity"`
	Stopped   bool                   `json:"stopped"`
	Emergency bool                   `json:"emergency_cleanup"`
	Error     string                 `json:"error,omitempty"`
}

type nativePVECleanupReceipt struct {
	SchemaVersion            int                     `json:"schema_version"`
	SourceCommit             string                  `json:"source_commit"`
	MachineID                string                  `json:"machine_id"`
	Node                     string                  `json:"node"`
	ClusterID                string                  `json:"cluster_id"`
	SupervisorUnit           string                  `json:"supervisor_unit"`
	SupervisorInvocationID   string                  `json:"supervisor_invocation_id"`
	RunnerAnchorInvocationID string                  `json:"runner_anchor_invocation_id"`
	RunnerSHA256             string                  `json:"runner_sha256"`
	TestSHA256               string                  `json:"test_sha256"`
	ManifestSHA256           string                  `json:"manifest_sha256"`
	CompletedAt              string                  `json:"completed_at"`
	Result                   string                  `json:"result"`
	ActionUnits              []string                `json:"stopped_action_units"`
	ActionUnitsGone          bool                    `json:"action_units_gone"`
	SidebandEmpty            bool                    `json:"sideband_empty"`
	AnchorStopped            bool                    `json:"anchor_stopped"`
	Guests                   []nativePVECleanupGuest `json:"guests"`
}

func TestNativePVEProxmoxGuestLifecycleQualification(t *testing.T) {
	if strings.TrimSpace(os.Getenv("PULSE_TEST_PVE_CONFIRM")) == "" {
		t.Skip("native PVE qualification requires an exact target-bound confirmation")
	}
	if os.Geteuid() != 0 {
		t.Fatal("native PVE qualification must run as root on the target PVE node")
	}
	if info, err := os.Stat("/etc/pve"); err != nil || !info.IsDir() {
		t.Fatal("native PVE qualification requires a real /etc/pve filesystem")
	}

	lock := acquireNativePVEQualificationLock(t)
	defer lock.Close()
	requireNoInstalledPulseRunner(t)
	requireNoNativePVETypedActionUnits(t)

	machineID := strings.TrimSpace(nativePVEReadFile(t, "/etc/machine-id"))
	node := nativePVELocalNode(t)
	clusterID := nativePVEClusterID(t, node)
	commit := strings.TrimSpace(os.Getenv("PULSE_TEST_SOURCE_COMMIT"))
	if !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(commit) {
		t.Fatal("PULSE_TEST_SOURCE_COMMIT must be the exact 40-character source revision")
	}
	vmid := requireNativePVEGuestID(t, "PULSE_TEST_PVE_VM_ID")
	ctid := requireNativePVEGuestID(t, "PULSE_TEST_PVE_CT_ID")
	if vmid == ctid {
		t.Fatal("the disposable VM and container must use distinct IDs")
	}
	supervisorUnit := strings.TrimSpace(os.Getenv("PULSE_TEST_PVE_SUPERVISOR_UNIT"))
	wantSupervisorUnit := fmt.Sprintf("pulse-pve-qualification-%s-%d-%d.service", commit[:8], vmid, ctid)
	supervisorInvocationID := strings.TrimSpace(os.Getenv("INVOCATION_ID"))
	if supervisorUnit != wantSupervisorUnit || !nativePVEQualificationUnit.MatchString(supervisorUnit) || !nativePVESystemdInvocationID.MatchString(supervisorInvocationID) {
		t.Fatal("native PVE qualification requires the exact systemd supervisor identity")
	}
	wantConfirmation := nativePVEQualificationConfirmation(machineID, node, clusterID, commit, vmid, ctid)
	if os.Getenv("PULSE_TEST_PVE_CONFIRM") != wantConfirmation {
		t.Fatal("native PVE qualification confirmation did not match the exact node, commit, VM, and container identity")
	}

	runnerPath := requireNativePVERunnerBinary(t, os.Getenv("PULSE_TEST_ACTION_RUNNER_BINARY"))
	receiptPath := requireNativePVEReceiptPath(t, os.Getenv("PULSE_TEST_PVE_RECEIPT_PATH"))
	if stateDir := filepath.Clean(strings.TrimSpace(os.Getenv("PULSE_AGENT_RUNNER_STATE_DIR"))); stateDir != filepath.Dir(receiptPath) {
		t.Fatal("qualification runner state directory must equal the private receipt directory")
	}
	testPath, err := os.Executable()
	if err != nil {
		t.Fatalf("resolve qualification test executable: %v", err)
	}
	runnerSHA256 := requireNativePVEArtifactHash(t, runnerPath, "PULSE_TEST_ACTION_RUNNER_SHA256")
	testSHA256 := requireNativePVEArtifactHash(t, testPath, "PULSE_TEST_HOSTAGENT_SHA256")
	receipt := nativePVEQualificationReceipt{
		SchemaVersion:          1,
		SourceCommit:           commit,
		MachineID:              machineID,
		Node:                   node,
		ClusterID:              clusterID,
		SupervisorUnit:         supervisorUnit,
		SupervisorInvocationID: supervisorInvocationID,
		PVEVersion:             nativePVECommandOutput(t, 30*time.Second, "/usr/bin/pveversion", "--verbose"),
		KernelVersion:          nativePVECommandOutput(t, 10*time.Second, "/usr/bin/uname", "-a"),
		SystemdVersion:         nativePVECommandOutput(t, 10*time.Second, "/usr/bin/systemctl", "--version"),
		StartedAt:              time.Now().UTC().Format(time.RFC3339Nano),
		Result:                 "failed",
		RunnerSHA256:           runnerSHA256,
		TestSHA256:             testSHA256,
		RunnerBuildInfo:        nativePVEBuildInfo(t, runnerPath),
		TestBuildInfo:          nativePVEBuildInfo(t, testPath),
	}
	t.Cleanup(func() {
		receipt.CompletedAt = time.Now().UTC().Format(time.RFC3339Nano)
		if !t.Failed() {
			receipt.Result = "passed"
		}
		if err := writeNativePVEJSON(receiptPath, receipt); err != nil {
			t.Errorf("write native PVE qualification receipt: %v", err)
		}
	})

	identities := []nativePVEGuestIdentity{
		fetchNativePVEGuestIdentity(t, node, "vm", vmid),
		fetchNativePVEGuestIdentity(t, node, "ct", ctid),
	}
	manifestPath := filepath.Join(filepath.Dir(receiptPath), "manifest.json")
	manifest := nativePVEQualificationManifest{
		SchemaVersion: 1, SourceCommit: commit, MachineID: machineID, Node: node,
		ClusterID: clusterID, SupervisorUnit: supervisorUnit, SupervisorInvocationID: supervisorInvocationID,
		RunnerSHA256: runnerSHA256, TestSHA256: testSHA256,
		CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), Guests: identities,
	}
	if err := writeNativePVEJSON(manifestPath, manifest); err != nil {
		t.Fatalf("write durable pre-anchor native PVE run manifest: %v", err)
	}
	receipt.RunnerAnchorInvocationID = startNativePVERunnerAnchor(t, supervisorUnit, supervisorInvocationID)
	manifest.RunnerAnchorInvocationID = receipt.RunnerAnchorInvocationID
	if err := writeNativePVEJSON(manifestPath, manifest); err != nil {
		t.Fatalf("bind runner anchor to durable native PVE run manifest: %v", err)
	}
	receipt.ManifestSHA256 = nativePVESHA256Bytes([]byte(nativePVEReadFile(t, manifestPath)))
	if receipt.ManifestSHA256 == "" {
		t.Fatal("durable native PVE run manifest did not hash")
	}
	previousExecutable := typedActionCurrentExecutable
	typedActionCurrentExecutable = func() (string, error) { return runnerPath, nil }
	t.Cleanup(func() { typedActionCurrentExecutable = previousExecutable })

	for _, identity := range identities {
		observation := nativePVEGuestObservation{Identity: identity}
		receipt.Guests = append(receipt.Guests, observation)
		index := len(receipt.Guests) - 1
		t.Cleanup(func() {
			emergency, finalState, err := cleanupNativePVEGuest(identity)
			receipt.Guests[index].Emergency = emergency
			receipt.Guests[index].FinalState = finalState
			if err != nil {
				t.Errorf("cleanup disposable %s %d: %v", identity.Kind, identity.VMID, err)
			}
			if emergency {
				t.Errorf("qualification required emergency cleanup for %s %d", identity.Kind, identity.VMID)
			}
		})
		receipt.Guests[index].Operations = exerciseNativePVEGuestLifecycle(t, identity)
		receipt.Guests[index].FinalState = "stopped"
	}

	requireNoNativePVETypedActionUnits(t)
	requireNativePVESidebandEmpty(t)
}

func TestNativePVEQualificationSupervisorCleanup(t *testing.T) {
	rawManifestPath := strings.TrimSpace(os.Getenv("PULSE_TEST_PVE_CLEANUP_MANIFEST"))
	rawReceiptPath := strings.TrimSpace(os.Getenv("PULSE_TEST_PVE_CLEANUP_RECEIPT"))
	if rawManifestPath == "" && rawReceiptPath == "" {
		t.Skip("native PVE supervisor cleanup requires its exact durable manifest and receipt paths")
	}
	manifestPath := filepath.Clean(rawManifestPath)
	receiptPath := filepath.Clean(rawReceiptPath)
	if !filepath.IsAbs(manifestPath) || filepath.Base(manifestPath) != "manifest.json" || !filepath.IsAbs(receiptPath) || filepath.Base(receiptPath) != "cleanup.json" || filepath.Dir(manifestPath) != filepath.Dir(receiptPath) {
		t.Fatal("cleanup requires co-located absolute manifest.json and cleanup.json paths")
	}
	if stateDir := filepath.Clean(strings.TrimSpace(os.Getenv("PULSE_AGENT_RUNNER_STATE_DIR"))); stateDir != filepath.Dir(manifestPath) {
		t.Fatal("cleanup runner state directory must equal the durable manifest directory")
	}
	receipt := nativePVECleanupReceipt{SchemaVersion: 1, Result: "failed"}
	t.Cleanup(func() {
		receipt.CompletedAt = time.Now().UTC().Format(time.RFC3339Nano)
		if !t.Failed() {
			receipt.Result = "passed"
		}
		if err := writeNativePVEJSON(receiptPath, receipt); err != nil {
			t.Errorf("write supervisor cleanup receipt: %v", err)
		}
	})
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read qualification manifest: %v", err)
	}
	var manifest nativePVEQualificationManifest
	if err := json.Unmarshal(data, &manifest); err != nil || manifest.SchemaVersion != 1 || len(manifest.Guests) != 2 || (manifest.RunnerAnchorInvocationID != "" && !nativePVESystemdInvocationID.MatchString(manifest.RunnerAnchorInvocationID)) || !nativePVEQualificationUnit.MatchString(manifest.SupervisorUnit) || !nativePVESystemdInvocationID.MatchString(manifest.SupervisorInvocationID) || !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(manifest.SourceCommit) || !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(manifest.RunnerSHA256) || !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(manifest.TestSHA256) {
		t.Fatalf("qualification manifest was invalid: %v", err)
	}
	if strings.TrimSpace(os.Getenv("INVOCATION_ID")) != manifest.SupervisorInvocationID {
		t.Fatal("qualification cleanup supervisor identity did not match the durable manifest")
	}
	if strings.TrimSpace(nativePVEReadFile(t, "/etc/machine-id")) != manifest.MachineID || nativePVELocalNode(t) != manifest.Node || nativePVEClusterID(t, manifest.Node) != manifest.ClusterID {
		t.Fatal("qualification cleanup target identity changed")
	}
	receipt.SourceCommit = manifest.SourceCommit
	receipt.MachineID = manifest.MachineID
	receipt.Node = manifest.Node
	receipt.ClusterID = manifest.ClusterID
	receipt.SupervisorUnit = manifest.SupervisorUnit
	receipt.SupervisorInvocationID = manifest.SupervisorInvocationID
	receipt.RunnerAnchorInvocationID = manifest.RunnerAnchorInvocationID
	receipt.RunnerSHA256 = manifest.RunnerSHA256
	receipt.TestSHA256 = manifest.TestSHA256
	receipt.ManifestSHA256 = nativePVESHA256Bytes(data)

	receipt.ActionUnits = stopNativePVEActionUnitsForCleanup(t)
	for _, identity := range manifest.Guests {
		guest := nativePVECleanupGuest{Identity: identity}
		emergency, state, cleanupErr := cleanupNativePVEGuest(identity)
		guest.Emergency = emergency
		guest.Stopped = state == "stopped"
		if cleanupErr != nil {
			guest.Error = cleanupErr.Error()
		}
		receipt.Guests = append(receipt.Guests, guest)
		if cleanupErr != nil || !guest.Stopped {
			t.Errorf("supervisor cleanup could not prove %s %d stopped: %v", identity.Kind, identity.VMID, cleanupErr)
		}
	}
	receipt.AnchorStopped = stopNativePVEAnchorForCleanup(t, manifest.RunnerAnchorInvocationID)
	receipt.ActionUnitsGone = nativePVETypedActionUnitOutput() == ""
	receipt.SidebandEmpty = nativePVESidebandEmpty()
	if !receipt.AnchorStopped || !receipt.ActionUnitsGone || !receipt.SidebandEmpty {
		t.Fatal("supervisor cleanup did not remove the qualification runtime")
	}
}

func TestNativePVEQualificationIdentityValidationFailsClosed(t *testing.T) {
	identity := nativePVEGuestIdentity{Kind: "vm", VMID: 101, Node: "pve-a", ConfigDigest: strings.Repeat("a", 40), Bridges: []string{"vmbr0"}}
	for name, mutate := range map[string]func(*nativePVEGuestIdentity){
		"kind":   func(v *nativePVEGuestIdentity) { v.Kind = "ct" },
		"vmid":   func(v *nativePVEGuestIdentity) { v.VMID++ },
		"owner":  func(v *nativePVEGuestIdentity) { v.Node = "pve-b" },
		"digest": func(v *nativePVEGuestIdentity) { v.ConfigDigest = strings.Repeat("b", 40) },
		"bridge": func(v *nativePVEGuestIdentity) { v.Bridges = []string{"vmbr1"} },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := identity
			candidate.Bridges = append([]string(nil), identity.Bridges...)
			mutate(&candidate)
			if reflect.DeepEqual(candidate, identity) {
				t.Fatal("identity drift was not detected")
			}
		})
	}
	if nativePVEConfigEligible(map[string]any{"digest": strings.Repeat("a", 40), "tags": "pulse-disposable", "template": 1.0, "net0": "virtio=00:00:00:00:00:01,bridge=vmbr0"}) == nil {
		t.Fatal("template config was accepted")
	}
	if nativePVEConfigEligible(map[string]any{"digest": strings.Repeat("a", 40), "tags": "other", "template": 0.0, "net0": "virtio=00:00:00:00:00:01,bridge=vmbr0"}) == nil {
		t.Fatal("wrong tag was accepted")
	}
	if nativePVEConfigEligible(map[string]any{"digest": strings.Repeat("a", 40), "tags": "pulse-disposable", "template": 0.0}) == nil {
		t.Fatal("missing bridge was accepted")
	}
	if nativePVEConfigEligible(map[string]any{"digest": strings.Repeat("a", 40), "tags": "pulse-disposable", "template": 0.0, "onboot": true, "net0": "virtio=00:00:00:00:00:01,bridge=vmbr0"}) == nil {
		t.Fatal("onboot guest was accepted")
	}
}

func TestNativePVEQualificationNetworkAndCgroupEvidenceFailsClosed(t *testing.T) {
	identity := nativePVEGuestIdentity{Kind: "vm", VMID: 101, Networks: map[string]string{"net0": "vmbr0", "net1": "vmbr1"}}
	links := []nativePVELink{
		{IfIndex: 1, IfName: "vmbr0", Flags: []string{"UP"}},
		{IfIndex: 2, IfName: "vmbr1", Flags: []string{"UP"}},
		{IfIndex: 3, IfName: "tap101i0", Master: "vmbr0", Flags: []string{"UP"}},
		{IfIndex: 4, IfName: "tap101i1", Master: "vmbr1", Flags: []string{"UP"}},
	}
	if _, ok := nativePVEBridgePathFromLinks(identity, "net0", "vmbr0", links); !ok {
		t.Fatal("exact net0 bridge path was not accepted")
	}
	if _, ok := nativePVEBridgePathFromLinks(identity, "net1", "vmbr1", links); !ok {
		t.Fatal("exact net1 bridge path was not accepted")
	}
	links[1].Flags = nil
	if _, ok := nativePVEBridgePathFromLinks(identity, "net1", "vmbr1", links); ok {
		t.Fatal("down bridge path was accepted")
	}
	if got := nativePVEGuestLinkNames(identity, links); !reflect.DeepEqual(got, []string{"tap101i0", "tap101i1"}) {
		t.Fatalf("stale target links were not independently visible: %v", got)
	}
	if nativePVECgroupMatches(identity, "0::/qemu.slice/qemu-999.scope") {
		t.Fatal("wrong VMID in a generic qemu slice was accepted")
	}
	if !nativePVECgroupMatches(identity, "0::/qemu.slice/qemu-101.scope") {
		t.Fatal("exact VMID native scope was rejected")
	}
}

func TestNativePVEQualificationPreAnchorCleanupFailsClosed(t *testing.T) {
	if err := nativePVEValidateAnchorCleanupIdentity("", ""); err != nil {
		t.Fatalf("absent pre-anchor runtime should be safe: %v", err)
	}
	if err := nativePVEValidateAnchorCleanupIdentity("", strings.Repeat("a", 32)); err == nil {
		t.Fatal("an unrecorded runner anchor was accepted for cleanup")
	}
	if err := nativePVEValidateAnchorCleanupIdentity(strings.Repeat("a", 32), strings.Repeat("b", 32)); err == nil {
		t.Fatal("a mismatched runner anchor was accepted for cleanup")
	}
	if err := nativePVEValidateAnchorCleanupIdentity(strings.Repeat("a", 32), strings.Repeat("a", 32)); err != nil {
		t.Fatalf("the exact recorded runner anchor was rejected: %v", err)
	}
}

func acquireNativePVEQualificationLock(t *testing.T) *os.File {
	t.Helper()
	file, err := os.OpenFile("/run/pulse-pve-qualification.lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatalf("open qualification lock: %v", err)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		t.Fatalf("another native PVE qualification is active: %v", err)
	}
	return file
}

func requireNoInstalledPulseRunner(t *testing.T) {
	t.Helper()
	loadState := strings.TrimSpace(nativePVECommandOutput(t, 10*time.Second, "/usr/bin/systemctl", "show", "--property=LoadState", "--value", "pulse-agent-runner.service"))
	if loadState != "not-found" {
		t.Fatalf("refusing to overlap an installed or loaded pulse-agent-runner.service: LoadState=%s", loadState)
	}
	for _, path := range []string{
		"/etc/systemd/system/pulse-agent-runner.service", "/run/systemd/system/pulse-agent-runner.service",
		"/usr/lib/systemd/system/pulse-agent-runner.service", "/lib/systemd/system/pulse-agent-runner.service",
		"/etc/systemd/system/pulse-agent-runner.service.d", "/run/systemd/system/pulse-agent-runner.service.d",
		"/usr/lib/systemd/system/pulse-agent-runner.service.d", "/lib/systemd/system/pulse-agent-runner.service.d",
	} {
		if _, err := os.Lstat(path); err == nil || !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("refusing to overlap runner unit path %s", path)
		}
	}
}

func startNativePVERunnerAnchor(t *testing.T, supervisorUnit, supervisorInvocationID string) string {
	t.Helper()
	if !nativePVEQualificationUnit.MatchString(supervisorUnit) || !nativePVESystemdInvocationID.MatchString(supervisorInvocationID) {
		t.Fatal("runner anchor requires the exact supervisor unit and invocation identity")
	}
	supervisorProperties, err := nativePVECombinedOutput(10*time.Second, "/usr/bin/systemctl", "show", "--property=ActiveState", "--property=InvocationID", supervisorUnit)
	if err != nil || !strings.Contains(supervisorProperties, "ActiveState=active") || !strings.Contains(supervisorProperties, "InvocationID="+supervisorInvocationID) {
		t.Fatalf("runner anchor supervisor was not active with the expected identity: %v: %s", err, supervisorProperties)
	}
	output, err := nativePVECombinedOutput(15*time.Second, "/usr/bin/systemd-run", "--no-ask-password", "--quiet", "--collect", "--service-type=exec", "--unit=pulse-agent-runner.service", "--property=User=root", "--property=Group=root", "--property=BindsTo="+supervisorUnit, "--property=After="+supervisorUnit, "--property=PartOf="+supervisorUnit, "--", "/usr/bin/sleep", "infinity")
	if err != nil {
		t.Fatalf("start isolated runner anchor: %v: %s", err, strings.TrimSpace(string(output)))
	}
	invocationOutput, invocationErr := nativePVECombinedOutput(10*time.Second, "/usr/bin/systemctl", "show", "--property=InvocationID", "--value", "pulse-agent-runner.service")
	invocationID := strings.TrimSpace(invocationOutput)
	if !nativePVESystemdInvocationID.MatchString(invocationID) {
		_, _ = nativePVECombinedOutput(15*time.Second, "/usr/bin/systemctl", "--no-ask-password", "stop", "pulse-agent-runner.service")
		t.Fatalf("isolated runner anchor had invalid InvocationID %q: %v", invocationID, invocationErr)
	}
	t.Cleanup(func() {
		current, err := nativePVECombinedOutput(10*time.Second, "/usr/bin/systemctl", "show", "--property=LoadState", "--property=InvocationID", "pulse-agent-runner.service")
		if err != nil {
			t.Errorf("inspect runner anchor during cleanup: %v: %s", err, current)
			return
		}
		currentProperties := nativePVESystemdProperties(current)
		currentInvocationID := currentProperties["InvocationID"]
		if currentProperties["LoadState"] == "not-found" && currentInvocationID == "" {
			return
		}
		if currentInvocationID == "" {
			t.Errorf("runner anchor remained loaded without an InvocationID")
			return
		}
		if currentInvocationID != invocationID {
			t.Errorf("runner anchor identity changed; refusing to stop InvocationID %q", currentInvocationID)
			return
		}
		_, _ = nativePVECombinedOutput(15*time.Second, "/usr/bin/systemctl", "--no-ask-password", "stop", "pulse-agent-runner.service")
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			out, _ := nativePVECombinedOutput(5*time.Second, "/usr/bin/systemctl", "show", "--property=LoadState", "--value", "pulse-agent-runner.service")
			if strings.TrimSpace(out) == "not-found" {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
		t.Errorf("isolated runner anchor did not disappear")
	})
	if state := strings.TrimSpace(nativePVECommandOutput(t, 10*time.Second, "/usr/bin/systemctl", "is-active", "pulse-agent-runner.service")); state != "active" {
		t.Fatalf("isolated runner anchor state=%s", state)
	}
	properties := nativePVECommandOutput(t, 10*time.Second, "/usr/bin/systemctl", "show", "--property=FragmentPath", "--property=DropInPaths", "--property=User", "--property=Group", "--property=ExecStart", "--property=BindsTo", "--property=After", "--property=PartOf", "pulse-agent-runner.service")
	effective := nativePVESystemdProperties(properties)
	if effective["FragmentPath"] != "/run/systemd/transient/pulse-agent-runner.service" || effective["DropInPaths"] != "" || effective["User"] != "root" || effective["Group"] != "root" || !strings.Contains(effective["ExecStart"], "/usr/bin/sleep infinity") || !nativePVESystemdUnitSetContains(effective["BindsTo"], supervisorUnit) || !nativePVESystemdUnitSetContains(effective["After"], supervisorUnit) || !nativePVESystemdUnitSetContains(effective["PartOf"], supervisorUnit) {
		t.Fatalf("isolated runner anchor effective state was not exact: %s", properties)
	}
	return invocationID
}

func nativePVESystemdUnitSetContains(value, want string) bool {
	for _, unit := range strings.Fields(value) {
		if unit == want {
			return true
		}
	}
	return false
}

func nativePVESystemdProperties(output string) map[string]string {
	properties := map[string]string{}
	for _, line := range strings.Split(output, "\n") {
		key, value, ok := strings.Cut(line, "=")
		if ok {
			properties[key] = value
		}
	}
	return properties
}

func nativePVEQualificationConfirmation(machineID, node, clusterID, commit string, vmid, ctid int) string {
	machineID = strings.TrimSpace(machineID)
	if len(machineID) > 12 {
		machineID = machineID[:12]
	}
	clusterDigest := sha256.Sum256([]byte(clusterID))
	return fmt.Sprintf("%s_MACHINE_%s_NODE_%s_CLUSTER_%x_COMMIT_%s_VM_%d_CT_%d", nativePVEQualificationPrefix, machineID, node, clusterDigest[:6], commit, vmid, ctid)
}

func requireNativePVERunnerBinary(t *testing.T, rawPath string) string {
	t.Helper()
	path := filepath.Clean(strings.TrimSpace(rawPath))
	if !filepath.IsAbs(path) {
		t.Fatal("PULSE_TEST_ACTION_RUNNER_BINARY must be an absolute path")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&0o077 != 0 || info.Mode()&0o100 == 0 {
		t.Fatalf("qualification runner must be a root-private executable regular file: %v", err)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != 0 {
		t.Fatal("qualification runner must be owned by root")
	}
	return path
}

func requireNativePVEArtifactHash(t *testing.T, path, envName string) string {
	t.Helper()
	want := strings.TrimSpace(os.Getenv(envName))
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(want) {
		t.Fatalf("%s must contain the exact SHA-256 artifact digest", envName)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open qualification artifact: %v", err)
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		t.Fatalf("hash qualification artifact: %v", err)
	}
	got := fmt.Sprintf("%x", hash.Sum(nil))
	if got != want {
		t.Fatalf("qualification artifact hash mismatch for %s", filepath.Base(path))
	}
	return got
}

func nativePVEBuildInfo(t *testing.T, path string) string {
	t.Helper()
	info, err := buildinfo.ReadFile(path)
	if err != nil {
		t.Fatalf("read qualification artifact build info: %v", err)
	}
	parts := []string{info.GoVersion, info.Path, info.Main.Path + "@" + info.Main.Version}
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs", "vcs.revision", "vcs.modified", "GOOS", "GOARCH":
			parts = append(parts, setting.Key+"="+setting.Value)
		}
	}
	return strings.Join(parts, ";")
}

func requireNativePVEReceiptPath(t *testing.T, rawPath string) string {
	t.Helper()
	path := filepath.Clean(strings.TrimSpace(rawPath))
	if !filepath.IsAbs(path) || filepath.Base(path) != "receipt.json" {
		t.Fatal("PULSE_TEST_PVE_RECEIPT_PATH must be an absolute receipt.json path")
	}
	info, err := os.Stat(filepath.Dir(path))
	if err != nil || !info.IsDir() || info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("qualification receipt directory must already be private: %v", err)
	}
	return path
}

func requireNativePVEGuestID(t *testing.T, name string) int {
	t.Helper()
	raw := strings.TrimSpace(os.Getenv(name))
	if !regexp.MustCompile(`^[1-9][0-9]{0,8}$`).MatchString(raw) {
		t.Fatalf("%s must be an exact positive decimal Proxmox guest ID", name)
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	return value
}

func nativePVELocalNode(t *testing.T) string {
	t.Helper()
	path, err := filepath.EvalSymlinks("/etc/pve/local")
	if err != nil {
		t.Fatalf("resolve local PVE node: %v", err)
	}
	node := filepath.Base(path)
	if !regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.-]*$`).MatchString(node) {
		t.Fatalf("invalid local PVE node identity %q", node)
	}
	return node
}

func nativePVEClusterID(t *testing.T, node string) string {
	t.Helper()
	data, err := os.ReadFile("/etc/pve/corosync.conf")
	if errors.Is(err, os.ErrNotExist) {
		return "standalone:" + node
	}
	if err != nil {
		t.Fatalf("read PVE cluster identity: %v", err)
	}
	digest := sha256.Sum256(data)
	return fmt.Sprintf("corosync-sha256:%x", digest[:])
}

func fetchNativePVEGuestIdentity(t *testing.T, localNode, kind string, vmid int) nativePVEGuestIdentity {
	t.Helper()
	resources := nativePVEPveshArray(t, "/cluster/resources", "--type", "vm")
	wantType := "qemu"
	if kind == "ct" {
		wantType = "lxc"
	}
	found := false
	for _, resource := range resources {
		if nativePVEInt(resource["vmid"]) != vmid {
			continue
		}
		if nativePVEString(resource["type"]) != wantType || nativePVEString(resource["node"]) != localNode {
			t.Fatalf("%s %d is not a local %s guest on node %s", kind, vmid, wantType, localNode)
		}
		if nativePVEString(resource["status"]) != "stopped" {
			t.Fatalf("%s %d must initially be stopped", kind, vmid)
		}
		found = true
		break
	}
	if !found {
		t.Fatalf("%s %d was not present in the PVE resource inventory", kind, vmid)
	}
	for _, resource := range nativePVEPveshArray(t, "/cluster/ha/resources") {
		sid := nativePVEString(resource["sid"])
		if sid == "vm:"+strconv.Itoa(vmid) || sid == "ct:"+strconv.Itoa(vmid) {
			t.Fatalf("refusing HA-managed disposable guest %s", sid)
		}
	}

	endpoint := fmt.Sprintf("/nodes/%s/%s/%d/config", localNode, wantType, vmid)
	config := nativePVEPveshObject(t, endpoint)
	if err := nativePVEConfigEligible(config); err != nil {
		t.Fatalf("%s %d is not eligible for destructive qualification: %v", kind, vmid, err)
	}
	bridges := nativePVEConfigBridges(config)
	return nativePVEGuestIdentity{Kind: kind, VMID: vmid, Node: localNode, ConfigDigest: nativePVEString(config["digest"]), Bridges: bridges, Networks: nativePVEConfigNetworks(config)}
}

func nativePVEConfigEligible(config map[string]any) error {
	digest := nativePVEString(config["digest"])
	if !nativePVEHexDigest.MatchString(digest) {
		return errors.New("missing or malformed config digest")
	}
	if nativePVEInt(config["template"]) != 0 {
		return errors.New("templates are never mutable qualification targets")
	}
	if nativePVEInt(config["onboot"]) != 0 {
		return errors.New("onboot guests are never mutable qualification targets")
	}
	hasTag := false
	for _, tag := range strings.Split(nativePVEString(config["tags"]), ";") {
		if strings.TrimSpace(tag) == "pulse-disposable" {
			hasTag = true
		}
	}
	if !hasTag {
		return errors.New("exact pulse-disposable tag is required")
	}
	if len(nativePVEConfigBridges(config)) == 0 {
		return errors.New("at least one bridge-backed network is required")
	}
	return nil
}

func nativePVEConfigBridges(config map[string]any) []string {
	networks := nativePVEConfigNetworks(config)
	set := map[string]struct{}{}
	for _, bridge := range networks {
		set[bridge] = struct{}{}
	}
	values := make([]string, 0, len(set))
	for value := range set {
		values = append(values, value)
	}
	sort.Strings(values)
	return values
}

func nativePVEConfigNetworks(config map[string]any) map[string]string {
	result := map[string]string{}
	for key, raw := range config {
		if !regexp.MustCompile(`^net[0-9]+$`).MatchString(key) {
			continue
		}
		for _, field := range strings.Split(nativePVEString(raw), ",") {
			name, value, ok := strings.Cut(strings.TrimSpace(field), "=")
			if ok && name == "bridge" && regexp.MustCompile(`^[A-Za-z0-9_.:-]+$`).MatchString(value) {
				result[key] = value
			}
		}
	}
	return result
}

func revalidateNativePVEGuestIdentity(identity nativePVEGuestIdentity) error {
	resources, err := nativePVEPveshArrayE("/cluster/resources", "--type", "vm")
	if err != nil {
		return err
	}
	wantType := "qemu"
	if identity.Kind == "ct" {
		wantType = "lxc"
	}
	found := false
	for _, resource := range resources {
		if nativePVEInt(resource["vmid"]) == identity.VMID {
			if nativePVEString(resource["type"]) != wantType || nativePVEString(resource["node"]) != identity.Node {
				return errors.New("guest kind or owner changed")
			}
			found = true
			break
		}
	}
	if !found {
		return errors.New("guest disappeared")
	}
	ha, err := nativePVEPveshArrayE("/cluster/ha/resources")
	if err != nil {
		return err
	}
	for _, resource := range ha {
		sid := nativePVEString(resource["sid"])
		if sid == "vm:"+strconv.Itoa(identity.VMID) || sid == "ct:"+strconv.Itoa(identity.VMID) {
			return errors.New("guest entered HA management")
		}
	}
	config, err := nativePVEPveshObjectE(fmt.Sprintf("/nodes/%s/%s/%d/config", identity.Node, wantType, identity.VMID))
	if err != nil {
		return err
	}
	if err := nativePVEConfigEligible(config); err != nil {
		return err
	}
	current := nativePVEGuestIdentity{Kind: identity.Kind, VMID: identity.VMID, Node: identity.Node, ConfigDigest: nativePVEString(config["digest"]), Bridges: nativePVEConfigBridges(config), Networks: nativePVEConfigNetworks(config)}
	if !reflect.DeepEqual(current, identity) {
		return errors.New("guest config identity changed")
	}
	return nil
}

func exerciseNativePVEGuestLifecycle(t *testing.T, identity nativePVEGuestIdentity) []nativePVEOperationObservation {
	t.Helper()
	operations := []struct{ name, before, after string }{
		{name: "start", before: "stopped", after: "running"},
		{name: "reboot", before: "running", after: "running"},
		{name: "shutdown", before: "running", after: "stopped"},
		{name: "start", before: "stopped", after: "running"},
		{name: "stop", before: "running", after: "stopped"},
	}
	manager := newProxmoxGuestLifecycleManager()
	observations := make([]nativePVEOperationObservation, 0, len(operations))
	for index, operation := range operations {
		if err := revalidateNativePVEGuestIdentity(identity); err != nil {
			t.Fatalf("refusing %s for %s %d after identity drift: %v", operation.name, identity.Kind, identity.VMID, err)
		}
		payload := agentexec.ProxmoxGuestLifecyclePayload{
			RequestID: fmt.Sprintf("pve-qualification-%s-%d-%d", identity.Kind, identity.VMID, index),
			ActionID:  fmt.Sprintf("pve-qualification-%s-%d-%s-%d", identity.Kind, identity.VMID, operation.name, index),
			Operation: operation.name, GuestKind: identity.Kind, VMID: identity.VMID,
			ExpectedStatus: operation.before, Timeout: 300,
		}
		if err := agentexec.BindProxmoxGuestLifecyclePayload(&payload); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		result := manager.Apply(ctx, payload)
		cancel()
		if err := agentexec.ValidateProxmoxGuestLifecycleResultForRequest(payload, result); err != nil {
			t.Fatalf("%s result binding failed: %v: %+v", operation.name, err, result)
		}
		if result.ExecutionPhase != agentexec.ProxmoxGuestPhaseComplete || !result.MutationStarted || !result.MutationCompleted || !result.ReadbackRan || result.Before.Status != operation.before || result.After.Status != operation.after || result.Error != "" || result.ReasonCode != "" || result.Before.ObservedAt.Location() != time.UTC || result.After.ObservedAt.Location() != time.UTC {
			t.Fatalf("%s %s %d did not produce the exact successful postcondition: %+v", operation.name, identity.Kind, identity.VMID, result)
		}
		waitForNativePVEContainmentCleanup(t)
		observation := nativePVEOperationObservation{Operation: operation.name, Result: result, ActionUnitsGone: true, SidebandEmpty: true}
		if operation.after == "running" {
			pid := waitForNativePVEGuestPID(t, identity)
			observation.Cgroup = requireNativePVEGuestCgroup(t, identity, pid)
			observation.LinkPaths = map[string][]string{}
			for network, bridge := range identity.Networks {
				observation.LinkPaths[network] = waitForNativePVEBridgePath(t, identity, network, bridge)
			}
		} else {
			waitForNoNativePVEGuestLinks(t, identity)
		}
		observations = append(observations, observation)
	}
	return observations
}

func cleanupNativePVEGuest(identity nativePVEGuestIdentity) (bool, string, error) {
	if err := revalidateNativePVEGuestIdentity(identity); err != nil {
		return false, "unknown", fmt.Errorf("identity drifted; refusing cleanup mutation: %w", err)
	}
	status, err := nativePVEGuestStatus(identity)
	if err != nil {
		return false, "unknown", err
	}
	if status == "stopped" {
		return false, status, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	output, err := exec.CommandContext(ctx, nativePVETool(identity.Kind), "stop", strconv.Itoa(identity.VMID)).CombinedOutput()
	if err != nil {
		return true, "unknown", fmt.Errorf("emergency stop failed: %v: %s", err, strings.TrimSpace(string(output)))
	}
	status, err = nativePVEGuestStatus(identity)
	if err != nil || status != "stopped" {
		return true, status, fmt.Errorf("emergency cleanup final state=%q: %w", status, err)
	}
	return true, status, nil
}

func stopNativePVEActionUnitsForCleanup(t *testing.T) []string {
	t.Helper()
	output := nativePVETypedActionUnitOutput()
	if output == "" {
		return nil
	}
	if strings.HasPrefix(output, "query-error:") {
		t.Fatalf("query qualification action units: %s", output)
	}
	units := []string{}
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || !validTypedActionUnitName(fields[0]) {
			t.Fatalf("unexpected unit in qualification cleanup: %q", line)
		}
		units = append(units, fields[0])
		if stopOutput, err := nativePVECombinedOutput(15*time.Second, "/usr/bin/systemctl", "--no-ask-password", "stop", fields[0]); err != nil {
			t.Errorf("stop qualification action unit %s: %v: %s", fields[0], err, stopOutput)
		}
	}
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if nativePVETypedActionUnitOutput() == "" {
			return units
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("qualification action units did not stop")
	return nil
}

func stopNativePVEAnchorForCleanup(t *testing.T, wantInvocationID string) bool {
	t.Helper()
	output, err := nativePVECombinedOutput(10*time.Second, "/usr/bin/systemctl", "show", "--property=LoadState", "--property=InvocationID", "pulse-agent-runner.service")
	if err != nil {
		t.Errorf("inspect runner anchor for supervisor cleanup: %v: %s", err, output)
		return false
	}
	properties := nativePVESystemdProperties(output)
	current := properties["InvocationID"]
	if properties["LoadState"] == "not-found" && current == "" {
		return true
	}
	if current == "" {
		t.Error("runner anchor remained loaded without an InvocationID")
		return false
	}
	if err := nativePVEValidateAnchorCleanupIdentity(wantInvocationID, current); err != nil {
		t.Fatal(err)
	}
	if stopOutput, err := nativePVECombinedOutput(15*time.Second, "/usr/bin/systemctl", "--no-ask-password", "stop", "pulse-agent-runner.service"); err != nil {
		t.Errorf("stop runner anchor: %v: %s", err, stopOutput)
		return false
	}
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		out, _ := nativePVECombinedOutput(5*time.Second, "/usr/bin/systemctl", "show", "--property=LoadState", "--value", "pulse-agent-runner.service")
		if strings.TrimSpace(out) == "not-found" {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

func nativePVEValidateAnchorCleanupIdentity(wantInvocationID, currentInvocationID string) error {
	if currentInvocationID == "" {
		return nil
	}
	if wantInvocationID == "" {
		return fmt.Errorf("runner anchor exists but the pre-anchor manifest has no InvocationID; refusing cleanup of InvocationID %q", currentInvocationID)
	}
	if currentInvocationID != wantInvocationID {
		return fmt.Errorf("runner anchor identity changed; refusing cleanup of InvocationID %q", currentInvocationID)
	}
	return nil
}

func nativePVEGuestStatus(identity nativePVEGuestIdentity) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, nativePVETool(identity.Kind), "status", strconv.Itoa(identity.VMID)).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("read guest status: %v: %s", err, strings.TrimSpace(string(output)))
	}
	fields := strings.Fields(strings.ToLower(strings.TrimSpace(string(output))))
	if len(fields) != 2 || fields[0] != "status:" || (fields[1] != "running" && fields[1] != "stopped") {
		return "", errors.New("malformed guest status")
	}
	return fields[1], nil
}

func nativePVETool(kind string) string {
	if kind == "ct" {
		return "/usr/sbin/pct"
	}
	return "/usr/sbin/qm"
}

func waitForNativePVEContainmentCleanup(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if nativePVETypedActionUnitOutput() == "" && nativePVESidebandEmpty() {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("typed-action unit or sideband remained after production operation: units=%q", nativePVETypedActionUnitOutput())
}

func requireNoNativePVETypedActionUnits(t *testing.T) {
	t.Helper()
	if output := nativePVETypedActionUnitOutput(); output != "" {
		t.Fatalf("refusing qualification with existing typed-action units: %s", output)
	}
}

func nativePVETypedActionUnitOutput() string {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "/usr/bin/systemctl", "--no-ask-password", "list-units", "--all", "--full", "--plain", "--no-legend", typedActionUnitPrefix+"*"+typedActionUnitSuffix).CombinedOutput()
	if err != nil {
		return "query-error:" + strings.TrimSpace(string(output))
	}
	return strings.TrimSpace(string(output))
}

func requireNativePVESidebandEmpty(t *testing.T) {
	t.Helper()
	if !nativePVESidebandEmpty() {
		t.Fatal("typed-action sideband directory was not empty")
	}
}

func nativePVESidebandEmpty() bool {
	dir := filepath.Join(strings.TrimSpace(os.Getenv("PULSE_AGENT_RUNNER_STATE_DIR")), "typed-actions")
	entries, err := os.ReadDir(dir)
	return errors.Is(err, os.ErrNotExist) || (err == nil && len(entries) == 0)
}

func waitForNativePVEGuestPID(t *testing.T, identity nativePVEGuestIdentity) int {
	t.Helper()
	wantType := "qemu"
	if identity.Kind == "ct" {
		wantType = "lxc"
	}
	endpoint := fmt.Sprintf("/nodes/%s/%s/%d/status/current", identity.Node, wantType, identity.VMID)
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		status, err := nativePVEPveshObjectE(endpoint)
		if err == nil && nativePVEString(status["status"]) == "running" {
			pid := nativePVEInt(status["pid"])
			if pid > 1 {
				if _, statErr := os.Stat(filepath.Join("/proc", strconv.Itoa(pid))); statErr == nil {
					return pid
				}
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatalf("timed out resolving PVE-owned process for running %s %d", identity.Kind, identity.VMID)
	return 0
}

func requireNativePVEGuestCgroup(t *testing.T, identity nativePVEGuestIdentity, pid int) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "cgroup"))
	if err != nil {
		t.Fatalf("read native %s %d cgroup: %v", identity.Kind, identity.VMID, err)
	}
	cgroup := strings.TrimSpace(string(data))
	if strings.Contains(cgroup, typedActionUnitPrefix) {
		t.Fatalf("%s %d process %d remained inside a Pulse typed-action cgroup: %s", identity.Kind, identity.VMID, pid, cgroup)
	}
	if !nativePVECgroupMatches(identity, cgroup) {
		t.Fatalf("%s %d process %d did not enter a recognizable native PVE cgroup: %s", identity.Kind, identity.VMID, pid, cgroup)
	}
	return strings.ReplaceAll(cgroup, "\n", "|")
}

func nativePVECgroupMatches(identity nativePVEGuestIdentity, cgroup string) bool {
	markers := []string{"qemu-" + strconv.Itoa(identity.VMID) + ".scope", "/" + strconv.Itoa(identity.VMID) + ".scope"}
	if identity.Kind == "ct" {
		markers = []string{"lxc.payload." + strconv.Itoa(identity.VMID), "/lxc/" + strconv.Itoa(identity.VMID) + "/", "machine-lxc\\x2d" + strconv.Itoa(identity.VMID) + ".scope"}
	}
	for _, marker := range markers {
		if strings.Contains(cgroup, marker) {
			return true
		}
	}
	return false
}

type nativePVELink struct {
	IfIndex   int      `json:"ifindex"`
	IfName    string   `json:"ifname"`
	Master    string   `json:"master"`
	LinkIndex int      `json:"link_index"`
	Flags     []string `json:"flags"`
}

func waitForNativePVEBridgePath(t *testing.T, identity nativePVEGuestIdentity, network, bridge string) []string {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		path, present, err := nativePVEBridgePath(identity, network, bridge)
		if err == nil && present {
			return path
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatalf("live bridged network path for %s=%s was not observed for %s %d", network, bridge, identity.Kind, identity.VMID)
	return nil
}

func waitForNoNativePVEGuestLinks(t *testing.T, identity nativePVEGuestIdentity) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		links, err := nativePVELinks()
		if err == nil && len(nativePVEGuestLinkNames(identity, links)) == 0 {
			return
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatalf("target-specific network interfaces remained after stopping %s %d", identity.Kind, identity.VMID)
}

func nativePVEBridgePath(identity nativePVEGuestIdentity, network, bridge string) ([]string, bool, error) {
	links, err := nativePVELinks()
	if err != nil {
		return nil, false, err
	}
	path, ok := nativePVEBridgePathFromLinks(identity, network, bridge, links)
	return path, ok, nil
}

func nativePVEBridgePathFromLinks(identity nativePVEGuestIdentity, network, bridge string, links []nativePVELink) ([]string, bool) {
	index := strings.TrimPrefix(network, "net")
	if index == network || !regexp.MustCompile(`^[0-9]+$`).MatchString(index) {
		return nil, false
	}
	byName := map[string]nativePVELink{}
	byIndex := map[int]string{}
	graph := map[string][]string{}
	for _, link := range links {
		byName[link.IfName] = link
		byIndex[link.IfIndex] = link.IfName
	}
	for _, link := range links {
		if link.Master != "" {
			graph[link.IfName] = append(graph[link.IfName], link.Master)
			graph[link.Master] = append(graph[link.Master], link.IfName)
		}
		if peer := byIndex[link.LinkIndex]; peer != "" {
			graph[link.IfName] = append(graph[link.IfName], peer)
			graph[peer] = append(graph[peer], link.IfName)
		}
	}
	id := strconv.Itoa(identity.VMID)
	prefixes := []string{"tap" + id + "i" + index, "fwbr" + id + "i" + index, "fwpr" + id + "p" + index, "fwln" + id + "i" + index}
	if identity.Kind == "ct" {
		prefixes = append(prefixes, "veth"+id+"i"+index)
	}
	starts := []string{}
	for name, link := range byName {
		for _, prefix := range prefixes {
			if name == prefix && nativePVELinkUp(link) {
				starts = append(starts, name)
			}
		}
	}
	if len(starts) == 0 || !nativePVELinkUp(byName[bridge]) {
		return nil, false
	}
	for _, start := range starts {
		queue := [][]string{{start}}
		seen := map[string]bool{start: true}
		for len(queue) > 0 {
			path := queue[0]
			queue = queue[1:]
			last := path[len(path)-1]
			if last == bridge {
				return path, true
			}
			for _, next := range graph[last] {
				if !seen[next] && nativePVELinkUp(byName[next]) {
					seen[next] = true
					queue = append(queue, append(append([]string(nil), path...), next))
				}
			}
		}
	}
	return nil, false
}

func nativePVELinks() ([]nativePVELink, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "/usr/sbin/ip", "-json", "-details", "link", "show").Output()
	if err != nil {
		return nil, err
	}
	var links []nativePVELink
	if err := json.Unmarshal(output, &links); err != nil {
		return nil, err
	}
	return links, nil
}

func nativePVEGuestLinkNames(identity nativePVEGuestIdentity, links []nativePVELink) []string {
	id := strconv.Itoa(identity.VMID)
	prefixes := []string{"tap" + id + "i", "fwbr" + id + "i", "fwpr" + id + "p", "fwln" + id + "i"}
	if identity.Kind == "ct" {
		prefixes = append(prefixes, "veth"+id+"i")
	}
	result := []string{}
	for _, link := range links {
		for _, prefix := range prefixes {
			if strings.HasPrefix(link.IfName, prefix) {
				result = append(result, link.IfName)
			}
		}
	}
	sort.Strings(result)
	return result
}

func nativePVELinkUp(link nativePVELink) bool {
	for _, flag := range link.Flags {
		if flag == "UP" {
			return true
		}
	}
	return false
}

func nativePVEPveshArray(t *testing.T, endpoint string, args ...string) []map[string]any {
	t.Helper()
	value, err := nativePVEPveshArrayE(endpoint, args...)
	if err != nil {
		t.Fatalf("pvesh %s: %v", endpoint, err)
	}
	return value
}

func nativePVEPveshArrayE(endpoint string, args ...string) ([]map[string]any, error) {
	var value []map[string]any
	err := nativePVEPveshJSON(endpoint, &value, args...)
	return value, err
}

func nativePVEPveshObject(t *testing.T, endpoint string) map[string]any {
	t.Helper()
	value, err := nativePVEPveshObjectE(endpoint)
	if err != nil {
		t.Fatalf("pvesh %s: %v", endpoint, err)
	}
	return value
}

func nativePVEPveshObjectE(endpoint string) (map[string]any, error) {
	var value map[string]any
	err := nativePVEPveshJSON(endpoint, &value)
	return value, err
}

func nativePVEPveshJSON(endpoint string, target any, args ...string) error {
	commandArgs := append([]string{"get", endpoint, "--output-format", "json"}, args...)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "/usr/bin/pvesh", commandArgs...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(output)))
	}
	if err := json.Unmarshal(output, target); err != nil {
		return fmt.Errorf("decode JSON: %w", err)
	}
	return nil
}

func nativePVEString(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func nativePVEInt(value any) int {
	switch typed := value.(type) {
	case bool:
		if typed {
			return 1
		}
		return 0
	case float64:
		return int(typed)
	case json.Number:
		parsed, _ := strconv.Atoi(typed.String())
		return parsed
	case string:
		parsed, _ := strconv.Atoi(strings.TrimSpace(typed))
		return parsed
	default:
		return 0
	}
}

func nativePVECommandOutput(t *testing.T, timeout time.Duration, name string, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	output, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v failed: %v: %s", name, args, err, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output))
}

func nativePVECombinedOutput(timeout time.Duration, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	output, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return strings.TrimSpace(string(output)), fmt.Errorf("command timed out after %s", timeout)
	}
	return strings.TrimSpace(string(output)), err
}

func nativePVESHA256Bytes(data []byte) string {
	digest := sha256.Sum256(data)
	return fmt.Sprintf("%x", digest[:])
}

func nativePVEReadFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func writeNativePVEJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	dir := filepath.Dir(path)
	tmp, err := os.OpenFile(filepath.Join(dir, "."+filepath.Base(path)+".tmp"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	ok := false
	defer func() {
		tmp.Close()
		if !ok {
			os.Remove(tmpPath)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	dirHandle, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer dirHandle.Close()
	if err := dirHandle.Sync(); err != nil {
		return err
	}
	ok = true
	return nil
}
