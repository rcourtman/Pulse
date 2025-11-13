package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func setupTempWrapper(t *testing.T) (scriptPath, thermalFile, binDir, baseDir string) {
	t.Helper()

	baseDir = t.TempDir()

	thermalDir := filepath.Join(baseDir, "sys", "class", "thermal", "thermal_zone0")
	if err := os.MkdirAll(thermalDir, 0o755); err != nil {
		t.Fatalf("failed to create thermal zone directory: %v", err)
	}
	thermalFile = filepath.Join(thermalDir, "temp")

	scriptContent := strings.ReplaceAll(tempWrapperScript, "/sys/class/thermal/thermal_zone0/temp", thermalFile)
	scriptPath = filepath.Join(baseDir, "temp-wrapper.sh")
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0o755); err != nil {
		t.Fatalf("failed to write wrapper script: %v", err)
	}

	binDir = filepath.Join(baseDir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("failed to create bin directory: %v", err)
	}

	linkCommand := func(name string) {
		target, err := exec.LookPath(name)
		if err != nil {
			t.Fatalf("required command %q not found on host: %v", name, err)
		}
		content := fmt.Sprintf("#!/bin/sh\nexec %s \"$@\"\n", target)
		if err := os.WriteFile(filepath.Join(binDir, name), []byte(content), 0o755); err != nil {
			t.Fatalf("failed to create shim for %s: %v", name, err)
		}
	}

	linkCommand("awk")
	linkCommand("cat")

	return scriptPath, thermalFile, binDir, baseDir
}

func runTempWrapper(t *testing.T, scriptPath, binDir string, extraEnv ...string) []byte {
	t.Helper()
	cmd := exec.Command("sh", scriptPath)
	env := []string{"PATH=" + binDir}
	env = append(env, extraEnv...)
	cmd.Env = env
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("temp wrapper failed: %v (output: %s)", err, strings.TrimSpace(string(output)))
	}
	return output
}

func TestTempWrapperFallbackWhenSensorsMissing(t *testing.T) {
	scriptPath, thermalFile, binDir, _ := setupTempWrapper(t)

	if err := os.WriteFile(thermalFile, []byte("51234\n"), 0o644); err != nil {
		t.Fatalf("failed to write thermal zone temperature: %v", err)
	}

	output := runTempWrapper(t, scriptPath, binDir)

	var data map[string]map[string]map[string]float64
	if err := json.Unmarshal(output, &data); err != nil {
		t.Fatalf("failed to parse wrapper output as JSON: %v (output: %s)", err, strings.TrimSpace(string(output)))
	}

	temp1, ok := data["rpitemp-virtual"]["temp1"]["temp1_input"]
	if !ok {
		t.Fatalf("expected rpitemp-virtual temp1 reading in output: %v", data)
	}
	if temp1 != 51.23 {
		t.Fatalf("expected converted temperature 51.23, got %.2f", temp1)
	}
}

func TestTempWrapperFallbackWhenSensorsEmpty(t *testing.T) {
	scriptPath, thermalFile, binDir, _ := setupTempWrapper(t)

	sensorsStub := filepath.Join(binDir, "sensors")
	content := "#!/bin/sh\nexit 0\n"
	if err := os.WriteFile(sensorsStub, []byte(content), 0o755); err != nil {
		t.Fatalf("failed to create sensors stub: %v", err)
	}

	if err := os.WriteFile(thermalFile, []byte("47890\n"), 0o644); err != nil {
		t.Fatalf("failed to write thermal zone temperature: %v", err)
	}

	output := runTempWrapper(t, scriptPath, binDir)

	var data map[string]map[string]map[string]float64
	if err := json.Unmarshal(output, &data); err != nil {
		t.Fatalf("failed to parse wrapper output as JSON: %v (output: %s)", err, strings.TrimSpace(string(output)))
	}

	temp1, ok := data["rpitemp-virtual"]["temp1"]["temp1_input"]
	if !ok {
		t.Fatalf("expected rpitemp-virtual temp1 reading in output: %v", data)
	}
	if temp1 != 47.89 {
		t.Fatalf("expected converted temperature 47.89, got %.2f", temp1)
	}
}

func TestTempWrapperPrefersSensorsOutput(t *testing.T) {
	scriptPath, thermalFile, binDir, _ := setupTempWrapper(t)

	jsonOutput := `{"cpu_thermal-virtual-0":{"temp1":{"temp1_input":42.5}}}`
	sensorsStub := filepath.Join(binDir, "sensors")
	content := fmt.Sprintf("#!/bin/sh\nprintf '%s'\n", jsonOutput)
	if err := os.WriteFile(sensorsStub, []byte(content), 0o755); err != nil {
		t.Fatalf("failed to create sensors stub: %v", err)
	}

	// Ensure thermal zone file exists but should be ignored
	if err := os.WriteFile(thermalFile, []byte("40000\n"), 0o644); err != nil {
		t.Fatalf("failed to write thermal zone temperature: %v", err)
	}

	output := runTempWrapper(t, scriptPath, binDir)

	trimmed := strings.TrimSpace(string(output))
	if trimmed != jsonOutput {
		t.Fatalf("expected wrapper to return sensors output %s, got %s", jsonOutput, trimmed)
	}
}

func TestReadAllWithLimit(t *testing.T) {
	reader := bytes.NewBufferString("abcdefg")
	data, exceeded, err := readAllWithLimit(reader, 4)
	if err != nil {
		t.Fatalf("readAllWithLimit returned error: %v", err)
	}
	if string(data) != "abcd" {
		t.Fatalf("expected truncated output 'abcd', got %q", string(data))
	}
	if !exceeded {
		t.Fatalf("expected exceeded flag when data exceeds limit")
	}

	reader2 := bytes.NewBufferString("xyz")
	data, exceeded, err = readAllWithLimit(reader2, 10)
	if err != nil {
		t.Fatalf("readAllWithLimit returned error: %v", err)
	}
	if string(data) != "xyz" {
		t.Fatalf("expected full output 'xyz', got %q", string(data))
	}
	if exceeded {
		t.Fatalf("did not expect exceeded flag for small output")
	}

	reader3 := bytes.NewBufferString("12345")
	data, exceeded, err = readAllWithLimit(reader3, 0)
	if err != nil {
		t.Fatalf("readAllWithLimit returned error: %v", err)
	}
	if string(data) != "12345" || exceeded {
		t.Fatalf("expected unlimited read to return full data without exceeding")
	}
}

func TestDiscoverLocalHostAddresses(t *testing.T) {
	// This test verifies that discoverLocalHostAddresses returns valid addresses
	// It will vary by host but should always return at least hostname or IP addresses
	addresses, err := discoverLocalHostAddresses()
	if err != nil {
		t.Fatalf("discoverLocalHostAddresses failed: %v", err)
	}

	if len(addresses) == 0 {
		t.Fatal("expected at least one address from discoverLocalHostAddresses")
	}

	// Verify addresses are non-empty and don't contain loopback
	for _, addr := range addresses {
		if addr == "" {
			t.Error("got empty address in results")
		}
		if addr == "127.0.0.1" || addr == "::1" {
			t.Errorf("discoverLocalHostAddresses should not return loopback address: %s", addr)
		}
		if strings.HasPrefix(addr, "127.") {
			t.Errorf("discoverLocalHostAddresses should not return loopback range: %s", addr)
		}
		if strings.HasPrefix(addr, "fe80:") {
			t.Errorf("discoverLocalHostAddresses should not return link-local IPv6: %s", addr)
		}
	}

	t.Logf("Discovered %d local addresses: %v", len(addresses), addresses)
}

// TestStandaloneNodeErrorPatterns verifies that the proxy correctly identifies
// all known error patterns from standalone nodes and LXC containers
func TestStandaloneNodeErrorPatterns(t *testing.T) {
	// These are real error messages users reported in GitHub issues
	standalonePatterns := []struct {
		name    string
		stderr  string
		stdout  string
		issueNo string // GitHub issue reference
	}{
		{
			name:    "classic standalone node",
			stderr:  "Error: Corosync config '/etc/pve/corosync.conf' does not exist - is this node part of a cluster?\n",
			stdout:  "",
			issueNo: "common",
		},
		{
			name:    "LXC ipcc_send_rec errors (issue #571)",
			stderr:  "ipcc_send_rec[1] failed: Unknown error -1\nipcc_send_rec[2] failed: Unknown error -1\nipcc_send_rec[3] failed: Unknown error -1\nUnable to load access control list: Unknown error -1\n",
			stdout:  "",
			issueNo: "#571",
		},
		{
			name:    "unknown error -1 only",
			stderr:  "Unknown error -1\n",
			stdout:  "",
			issueNo: "#571",
		},
		{
			name:    "access control list error",
			stderr:  "Unable to load access control list: Unknown error -1\n",
			stdout:  "",
			issueNo: "#571",
		},
		{
			name:    "not part of cluster message",
			stdout:  "This node is not part of a cluster\n",
			stderr:  "",
			issueNo: "common",
		},
		{
			name:    "mixed stdout/stderr (some PVE versions)",
			stdout:  "ipcc_send_rec failed: Unknown error -1\n",
			stderr:  "",
			issueNo: "#571",
		},
	}

	for _, tc := range standalonePatterns {
		t.Run(tc.name, func(t *testing.T) {
			combinedOutput := tc.stderr + tc.stdout

			// Check each detection pattern we added
			isStandalone := strings.Contains(combinedOutput, "does not exist") ||
				strings.Contains(combinedOutput, "not part of a cluster") ||
				strings.Contains(combinedOutput, "ipcc_send_rec") ||
				strings.Contains(combinedOutput, "Unknown error -1") ||
				strings.Contains(combinedOutput, "Unable to load access control list")

			if !isStandalone {
				t.Errorf("Failed to detect standalone/LXC pattern from %s:\n  stderr: %q\n  stdout: %q",
					tc.issueNo, tc.stderr, tc.stdout)
			} else {
				t.Logf("✓ Correctly identified standalone/LXC pattern from %s", tc.issueNo)
			}
		})
	}
}

// TestNonStandaloneErrors verifies that genuine errors are NOT misidentified as standalone nodes
func TestNonStandaloneErrors(t *testing.T) {
	genuineErrors := []struct {
		name   string
		stderr string
		stdout string
	}{
		{
			name:   "network timeout",
			stderr: "Connection timed out\n",
			stdout: "",
		},
		{
			name:   "permission denied",
			stderr: "Permission denied (publickey)\n",
			stdout: "",
		},
		{
			name:   "command not found",
			stderr: "bash: pvecm: command not found\n",
			stdout: "",
		},
	}

	for _, tc := range genuineErrors {
		t.Run(tc.name, func(t *testing.T) {
			combinedOutput := tc.stderr + tc.stdout

			// These should NOT match our standalone patterns
			isStandalone := strings.Contains(combinedOutput, "does not exist") ||
				strings.Contains(combinedOutput, "not part of a cluster") ||
				strings.Contains(combinedOutput, "ipcc_send_rec") ||
				strings.Contains(combinedOutput, "Unknown error -1") ||
				strings.Contains(combinedOutput, "Unable to load access control list")

			if isStandalone {
				t.Errorf("False positive: genuine error misidentified as standalone:\n  stderr: %q\n  stdout: %q",
					tc.stderr, tc.stdout)
			} else {
				t.Logf("✓ Correctly identified genuine error (not standalone)")
			}
		})
	}
}
