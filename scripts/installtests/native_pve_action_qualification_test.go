package installtests

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativePVEQualificationEvidenceVerifierFailsClosedWithPythonOptimize(t *testing.T) {
	scriptBytes, err := os.ReadFile(repoFile("scripts", "run-native-pve-action-qualification.sh"))
	if err != nil {
		t.Fatalf("read native PVE qualification wrapper: %v", err)
	}
	startMarker := "import hashlib\n"
	endMarker := "\nPY\nthen\n"
	start := strings.Index(string(scriptBytes), startMarker)
	end := strings.Index(string(scriptBytes), endMarker)
	if start < 0 || end <= start {
		t.Fatal("native PVE evidence verifier heredoc was not found")
	}
	verifier := string(scriptBytes)[start:end]
	if strings.Contains(verifier, "assert ") {
		t.Fatal("native PVE evidence verifier must not depend on optimizable Python assertions")
	}

	paths, args := writeNativePVEEvidenceFixtures(t)
	if output, err := runNativePVEEvidenceVerifier(verifier, args); err != nil {
		t.Fatalf("valid evidence was rejected with PYTHONOPTIMIZE=1: %v: %s", err, output)
	}

	receiptBytes, err := os.ReadFile(paths[2])
	if err != nil {
		t.Fatal(err)
	}
	var receipt map[string]any
	if err := json.Unmarshal(receiptBytes, &receipt); err != nil {
		t.Fatal(err)
	}
	receipt["guests"].([]any)[0].(map[string]any)["operations"].([]any)[0].(map[string]any)["result"].(map[string]any)["execution_phase"] = "mutate"
	writeNativePVEEvidenceJSON(t, paths[2], receipt)
	if output, err := runNativePVEEvidenceVerifier(verifier, args); err == nil {
		t.Fatalf("malformed production result was accepted with PYTHONOPTIMIZE=1: %s", output)
	}
}

func writeNativePVEEvidenceFixtures(t *testing.T) ([3]string, []string) {
	t.Helper()
	dir := t.TempDir()
	paths := [3]string{filepath.Join(dir, "manifest.json"), filepath.Join(dir, "cleanup.json"), filepath.Join(dir, "receipt.json")}
	sourceCommit := strings.Repeat("a", 40)
	machineID := strings.Repeat("b", 32)
	node := "pve-a"
	clusterID := "standalone:pve-a"
	runnerHash := strings.Repeat("c", 64)
	testHash := strings.Repeat("d", 64)
	supervisorUnit := "pulse-pve-qualification-aaaaaaaa-101-102.service"
	supervisorInvocationID := strings.Repeat("e", 32)
	runnerInvocationID := strings.Repeat("f", 32)
	guests := []any{
		map[string]any{"kind": "vm", "vmid": 101, "node": node, "config_digest": strings.Repeat("1", 40), "bridges": []string{"vmbr0"}, "networks": map[string]string{"net0": "vmbr0"}},
		map[string]any{"kind": "ct", "vmid": 102, "node": node, "config_digest": strings.Repeat("2", 40), "bridges": []string{"vmbr0"}, "networks": map[string]string{"net0": "vmbr0"}},
	}
	common := map[string]any{
		"source_commit": sourceCommit, "machine_id": machineID, "node": node, "cluster_id": clusterID,
		"supervisor_unit": supervisorUnit, "supervisor_invocation_id": supervisorInvocationID,
		"runner_sha256": runnerHash, "test_sha256": testHash,
	}
	manifest := cloneNativePVEEvidenceMap(common)
	manifest["schema_version"] = 1
	manifest["runner_anchor_invocation_id"] = runnerInvocationID
	manifest["guests"] = guests
	writeNativePVEEvidenceJSON(t, paths[0], manifest)
	manifestBytes, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	manifestHash := fmt.Sprintf("%x", sha256.Sum256(manifestBytes))

	cleanup := cloneNativePVEEvidenceMap(common)
	cleanup["schema_version"] = 1
	cleanup["result"] = "passed"
	cleanup["manifest_sha256"] = manifestHash
	cleanup["runner_anchor_invocation_id"] = runnerInvocationID
	cleanup["anchor_stopped"] = true
	cleanup["action_units_gone"] = true
	cleanup["sideband_empty"] = true
	cleanup["guests"] = []any{
		map[string]any{"identity": guests[0], "stopped": true},
		map[string]any{"identity": guests[1], "stopped": true},
	}
	writeNativePVEEvidenceJSON(t, paths[1], cleanup)

	receipt := cloneNativePVEEvidenceMap(common)
	receipt["schema_version"] = 1
	receipt["result"] = "passed"
	receipt["manifest_sha256"] = manifestHash
	receipt["runner_anchor_invocation_id"] = runnerInvocationID
	receipt["guests"] = []any{
		nativePVEFixtureGuest(guests[0].(map[string]any)),
		nativePVEFixtureGuest(guests[1].(map[string]any)),
	}
	writeNativePVEEvidenceJSON(t, paths[2], receipt)
	args := []string{paths[0], paths[1], paths[2], sourceCommit, machineID, node, clusterID, runnerHash, testHash, supervisorUnit, supervisorInvocationID, "101", "102"}
	return paths, args
}

func nativePVEFixtureGuest(identity map[string]any) map[string]any {
	operations := []string{"start", "reboot", "shutdown", "start", "stop"}
	before := []string{"stopped", "running", "running", "stopped", "running"}
	after := []string{"running", "running", "stopped", "running", "stopped"}
	observations := make([]any, 0, len(operations))
	for index, operation := range operations {
		result := map[string]any{
			"operation": operation, "guest_kind": identity["kind"], "vmid": identity["vmid"],
			"execution_phase": "complete", "mutation_started": true, "mutation_completed": true, "readback_ran": true,
			"before": map[string]any{"status": before[index]}, "after": map[string]any{"status": after[index]},
		}
		observation := map[string]any{"operation": operation, "result": result, "action_units_gone": true, "sideband_empty": true}
		if after[index] == "running" {
			vmid := int(identity["vmid"].(int))
			if identity["kind"] == "vm" {
				observation["cgroup"] = fmt.Sprintf("0::/qemu.slice/qemu-%d.scope", vmid)
			} else {
				observation["cgroup"] = fmt.Sprintf("0::/lxc.payload.%d", vmid)
			}
			observation["link_paths"] = map[string]any{"net0": []string{fmt.Sprintf("guest-%d-net0", vmid), "vmbr0"}}
		}
		observations = append(observations, observation)
	}
	return map[string]any{"identity": identity, "operations": observations, "final_state": "stopped", "emergency_cleanup": false}
}

func runNativePVEEvidenceVerifier(verifier string, args []string) (string, error) {
	commandArgs := append([]string{"-I", "-"}, args...)
	command := exec.Command("python3", commandArgs...)
	command.Env = append(os.Environ(), "PYTHONOPTIMIZE=1")
	command.Stdin = strings.NewReader(verifier)
	output, err := command.CombinedOutput()
	return strings.TrimSpace(string(output)), err
}

func writeNativePVEEvidenceJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}

func cloneNativePVEEvidenceMap(source map[string]any) map[string]any {
	result := make(map[string]any, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}
