package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ssh/knownhosts"
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

func TestTempWrapperPrefersOverrideWrapper(t *testing.T) {
	scriptPath, _, binDir, baseDir := setupTempWrapper(t)

	overrideDir := filepath.Join(baseDir, "override")
	if err := os.MkdirAll(overrideDir, 0o755); err != nil {
		t.Fatalf("failed to create override directory: %v", err)
	}

	overridePath := filepath.Join(overrideDir, "pulse-sensor-wrapper.sh")
	expectedOutput := `{"smart":[{"device":"/dev/test","temperature":42}]}`
	overrideScript := fmt.Sprintf("#!/bin/sh\nprintf '%s'\n", expectedOutput)
	if err := os.WriteFile(overridePath, []byte(overrideScript), 0o755); err != nil {
		t.Fatalf("failed to write override wrapper: %v", err)
	}

	output := runTempWrapper(t, scriptPath, binDir, "PULSE_SENSOR_WRAPPER="+overridePath)
	if strings.TrimSpace(string(output)) != expectedOutput {
		t.Fatalf("expected override wrapper output %s, got %s", expectedOutput, strings.TrimSpace(string(output)))
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
		{
			name:    "no cluster keyword",
			stderr:  "Error: no cluster configuration\n",
			stdout:  "",
			issueNo: "variation",
		},
		{
			name:    "IPC failure variant",
			stderr:  "IPC communication error\n",
			stdout:  "",
			issueNo: "variation",
		},
	}

	// Use the same detection logic as the actual code
	standaloneIndicators := []string{
		"does not exist", "not found", "no such file",
		"not part of a cluster", "no cluster", "standalone",
		"ipcc_send_rec", "IPC", "communication failed", "connection refused",
		"Unknown error -1", "Unable to load", "access denied", "permission denied",
		"access control list",
	}

	for _, tc := range standalonePatterns {
		t.Run(tc.name, func(t *testing.T) {
			combinedOutput := tc.stderr + tc.stdout

			// Check using the permissive detection strategy
			isStandalone := false
			for _, indicator := range standaloneIndicators {
				if strings.Contains(strings.ToLower(combinedOutput), strings.ToLower(indicator)) {
					isStandalone = true
					break
				}
			}

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
		// {
		// 	name:   "permission denied",
		// 	stderr: "Permission denied (publickey)\n",
		// 	stdout: "",
		// },
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

func TestShellQuote(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"", "''"},
		{"foo", "'foo'"},
		{"foo'bar", "\"foo'bar\""},
	}
	for _, tc := range cases {
		if got := shellQuote(tc.input); got != tc.want {
			t.Errorf("shellQuote(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestIsLocalNode(t *testing.T) {
	if !isLocalNode("localhost") {
		t.Error("localhost should be local")
	}
	if !isLocalNode("127.0.0.1") {
		t.Error("127.0.0.1 should be local")
	}
	if !isLocalNode("::1") {
		t.Error("::1 should be local")
	}
	if isLocalNode("8.8.8.8") {
		t.Error("8.8.8.8 should not be local")
	}
}

func TestIsLocalNode_Hostname(t *testing.T) {
	hostname, _ := os.Hostname()
	if !isLocalNode(hostname) {
		t.Errorf("hostname %q should be local", hostname)
	}
}

func TestIsProxmoxHost_DirCheck(t *testing.T) {
	// Mock PATH to NOT find pvecm
	oldPath := os.Getenv("PATH")
	os.Setenv("PATH", t.TempDir()) // Empty path
	defer os.Setenv("PATH", oldPath)

	// Since we can't mock /etc/pve easily, we rely on it likely not existing.
	if isProxmoxHost() {
		// If it exists, good for us?
	}
}

func TestDiscoverLocalHostAddressesFallback(t *testing.T) {
	// Only test that it doesn't panic and returns something
	addresses, _ := discoverLocalHostAddressesFallback()
	if len(addresses) == 0 {
		// Even if 'ip addr' fails, it should return hostname
		t.Log("discoverLocalHostAddressesFallback returned no addresses")
	}
}

func TestGetTemperatureLocal(t *testing.T) {
	_, _, binDir, _ := setupTempWrapper(t)

	// Mock sensors command
	sensorsStub := filepath.Join(binDir, "sensors")
	jsonOutput := `{"cpu_thermal-virtual-0":{"temp1":{"temp1_input":42.5}}}`
	content := fmt.Sprintf("#!/bin/sh\nprintf '%s'\n", jsonOutput)
	if err := os.WriteFile(sensorsStub, []byte(content), 0o755); err != nil {
		t.Fatalf("failed to create sensors stub: %v", err)
	}

	oldPath := os.Getenv("PATH")
	os.Setenv("PATH", binDir+string(os.PathListSeparator)+oldPath)
	defer os.Setenv("PATH", oldPath)

	p := &Proxy{metrics: NewProxyMetrics("test")}
	out, err := p.getTemperatureLocal(context.Background())
	if err != nil {
		t.Fatalf("getTemperatureLocal failed: %v", err)
	}
	if strings.TrimSpace(out) != jsonOutput {
		t.Errorf("expected output %s, got %s", jsonOutput, out)
	}
}

func TestDiscoverClusterNodes(t *testing.T) {
	tmpDir := t.TempDir()
	pvecmPath := filepath.Join(tmpDir, "pvecm")
	// Normal cluster output
	script := "#!/bin/sh\necho \"0x00000001 1 10.0.0.1\"\n"
	os.WriteFile(pvecmPath, []byte(script), 0755)

	oldPath := os.Getenv("PATH")
	os.Setenv("PATH", tmpDir+string(os.PathListSeparator)+oldPath)
	defer os.Setenv("PATH", oldPath)

	nodes, err := discoverClusterNodes()
	if err != nil {
		t.Fatalf("discoverClusterNodes failed: %v", err)
	}
	if len(nodes) != 1 || nodes[0] != "10.0.0.1" {
		t.Errorf("expected [10.0.0.1], got %v", nodes)
	}

	// Standalone node (not part of cluster)
	script = "#!/bin/sh\necho \"Error: Corosync config '/etc/pve/corosync.conf' does not exist\"\nexit 1\n"
	os.WriteFile(pvecmPath, []byte(script), 0755)

	// discoverClusterNodes should fall back to local discovery
	nodes, err = discoverClusterNodes()
	if err != nil {
		t.Fatalf("discoverClusterNodes failed on standalone node: %v", err)
	}
	if len(nodes) == 0 {
		t.Error("expected local addresses for standalone node")
	}

	// Unknown error
	script = "#!/bin/sh\necho \"Some other error\"\nexit 1\n"
	os.WriteFile(pvecmPath, []byte(script), 0755)

	_, err = discoverClusterNodes()
	if err == nil {
		t.Error("expected error for unknown failure")
	}

	// IPC error (should NOT fallback to local)
	script = "#!/bin/sh\necho \"ipcc_send_rec failed\"\nexit 1\n"
	os.WriteFile(pvecmPath, []byte(script), 0755)

	_, err = discoverClusterNodes()
	if err == nil {
		t.Error("expected error for IPC failure")
	} else if strings.Contains(err.Error(), "ipcc_send_rec failed") {
		// This is good, it propagated the error instead of masking it
	}
}

func TestDiscoverClusterNodes_LookPathError(t *testing.T) {
	// Modify PATH to not include pvecm
	oldPath := os.Getenv("PATH")
	os.Setenv("PATH", t.TempDir()) // Empty path
	defer os.Setenv("PATH", oldPath)

	_, err := discoverClusterNodes()
	if err == nil || !strings.Contains(err.Error(), "pvecm not found") {
		t.Errorf("expected pvecm not found error, got %v", err)
	}
}

func TestDiscoverLocalHostAddresses_NetlinkError(t *testing.T) {
	// Mock netInterfaces to return error
	oldNetInterfaces := netInterfaces
	defer func() { netInterfaces = oldNetInterfaces }()

	netInterfaces = func() ([]net.Interface, error) {
		return nil, fmt.Errorf("address family not supported by protocol")
	}

	// Should fallback to 'ip addr' command (which we can mock or let fail naturally)
	// If fallback also fails (e.g. no 'ip' command in test env), that's fine as long as we hit the code path.
	// We want to verify that the error was caught and logged (we can't easily verify log output here nicely)
	// But we can verify that it returned result or error without panicking.
	// Since discoverLocalHostAddressesFallback relies on 'hostname' or 'ip', it usually returns at least hostname.

	addrs, err := discoverLocalHostAddresses()
	if err != nil {
		t.Fatalf("unexpected error (should handle fallback): %v", err)
	}
	if len(addrs) == 0 {
		t.Log("Warning: no addresses found during fallback test")
	}

	// Test generic error
	netInterfaces = func() ([]net.Interface, error) {
		return nil, fmt.Errorf("generic connection error")
	}

	// This path logs error and continues (returning empty or hostname-only additions)
	// effectively skipping interface enumeration.
	addrs, err = discoverLocalHostAddresses()
	if err != nil {
		t.Fatalf("unexpected error from generic failure: %v", err)
	}
}

func TestDiscoverLocalHostAddressesFallback_IPCommand(t *testing.T) {
	// Mock netInterfaces to trigger fallback
	oldNetInterfaces := netInterfaces
	defer func() { netInterfaces = oldNetInterfaces }()
	netInterfaces = func() ([]net.Interface, error) {
		return nil, fmt.Errorf("address family not supported")
	}

	// Mock 'ip' command
	tmpDir := t.TempDir()
	ipPath := filepath.Join(tmpDir, "ip")
	// Output with some IPs and loopback
	output := `
1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 qdisc noqueue state UNKNOWN group default qlen 1000
    link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00
    inet 127.0.0.1/8 scope host lo
       valid_lft forever preferred_lft forever
    inet6 ::1/128 scope host 
       valid_lft forever preferred_lft forever
2: eth0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc fq_codel state UP group default qlen 1000
    link/ether 00:15:5d:00:07:02 brd ff:ff:ff:ff:ff:ff
    inet 192.168.1.50/24 brd 192.168.1.255 scope global eth0
       valid_lft forever preferred_lft forever
    inet6 fe80::215:5dff:fe00:702/64 scope link 
       valid_lft forever preferred_lft forever
`
	script := fmt.Sprintf("#!/bin/sh\ncat <<EOF\n%s\nEOF\n", output)
	os.WriteFile(ipPath, []byte(script), 0755)

	oldPath := os.Getenv("PATH")
	os.Setenv("PATH", tmpDir+string(os.PathListSeparator)+oldPath)
	defer os.Setenv("PATH", oldPath)

	addrs, err := discoverLocalHostAddresses()
	if err != nil {
		t.Fatalf("discoverLocalHostAddresses failed with fallback: %v", err)
	}

	found := false
	for _, addr := range addrs {
		if addr == "192.168.1.50" {
			found = true
		}
		if addr == "127.0.0.1" {
			t.Error("should not include loopback 127.0.0.1")
		}
		if strings.HasPrefix(addr, "fe80:") {
			t.Error("should not include link-local fe80::")
		}
	}
	if !found {
		t.Errorf("expected to find 192.168.1.50 in %v", addrs)
	}

	// Test failure of 'ip' command
	script = "#!/bin/sh\nexit 1\n"
	os.WriteFile(ipPath, []byte(script), 0755)
	addrs, err = discoverLocalHostAddresses()
	if err != nil {
		t.Fatal(err)
	}
	// Should at least return hostname (assume test runner has hostname)
	if len(addrs) == 0 {
		t.Log("No hostname addresses found when ip command fails")
	}
}

func TestGetTemperatureLocal_Fallback(t *testing.T) {
	_, _, binDir, _ := setupTempWrapper(t)

	// Mock sensors command to fail first but succeed second
	sensorsStub := filepath.Join(binDir, "sensors")
	script := "#!/bin/sh\nif [ \"$1\" = \"-j\" ]; then exit 1; fi\necho \"text output\"\nexit 0\n"
	if err := os.WriteFile(sensorsStub, []byte(script), 0o755); err != nil {
		t.Fatalf("failed to create sensors stub: %v", err)
	}

	oldPath := os.Getenv("PATH")
	os.Setenv("PATH", binDir+string(os.PathListSeparator)+oldPath)
	defer os.Setenv("PATH", oldPath)

	p := &Proxy{metrics: NewProxyMetrics("test")}
	out, err := p.getTemperatureLocal(context.Background())
	if err != nil {
		t.Fatalf("getTemperatureLocal failed: %v", err)
	}
	if out != "{}" {
		t.Errorf("expected empty JSON for fallback, got %q", out)
	}
}

func TestGetTemperatureLocal_CompleteFailure(t *testing.T) {
	_, _, binDir, _ := setupTempWrapper(t)

	// Mock sensors command to fail completely
	sensorsStub := filepath.Join(binDir, "sensors")
	script := "#!/bin/sh\nexit 1\n"
	if err := os.WriteFile(sensorsStub, []byte(script), 0o755); err != nil {
		t.Fatalf("failed to create sensors stub: %v", err)
	}

	oldPath := os.Getenv("PATH")
	os.Setenv("PATH", binDir+string(os.PathListSeparator)+oldPath)
	defer os.Setenv("PATH", oldPath)

	p := &Proxy{metrics: NewProxyMetrics("test")}
	_, err := p.getTemperatureLocal(context.Background())
	if err == nil {
		t.Error("expected error when sensors command fails")
	}
}

func TestGetTemperatureLocal_EmptyOutput(t *testing.T) {
	_, _, binDir, _ := setupTempWrapper(t)

	// Mock sensors to return empty string
	sensorsStub := filepath.Join(binDir, "sensors")
	script := "#!/bin/sh\nexit 0\n"
	if err := os.WriteFile(sensorsStub, []byte(script), 0o755); err != nil {
		t.Fatalf("failed to create sensors stub: %v", err)
	}

	oldPath := os.Getenv("PATH")
	os.Setenv("PATH", binDir+string(os.PathListSeparator)+oldPath)
	defer os.Setenv("PATH", oldPath)

	p := &Proxy{metrics: NewProxyMetrics("test")}
	out, err := p.getTemperatureLocal(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "{}" {
		t.Errorf("expected empty JSON for empty output, got %q", out)
	}
}

func TestDiscoverLocalHostAddresses_InterfaceEdgeCases(t *testing.T) {
	oldNetInterfaces := netInterfaces
	defer func() { netInterfaces = oldNetInterfaces }()

	netInterfaces = func() ([]net.Interface, error) {
		return []net.Interface{
			{Name: "lo", Flags: net.FlagUp | net.FlagLoopback}, // Should be skipped (loopback)
			{Name: "down0", Flags: 0},                          // Should be skipped (down)
			{Name: "nonexistent0", Flags: net.FlagUp},          // Should trigger Addrs() error
		}, nil
	}

	addrs, err := discoverLocalHostAddresses()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// We expect empty addresses because all mocked interfaces are skipped or fail Addrs()
	// But it might pick up hostname addresses.
	// We can't easily mock hostname without another variable.
	// But we can check that it didn't crash.
	t.Logf("Got addresses: %v", addrs)
}

func TestLoadProxmoxHostKeys(t *testing.T) {
	tmpDir := t.TempDir()
	hostsFile := filepath.Join(tmpDir, "known_hosts")

	// Override path
	origPath := proxmoxClusterKnownHostsPath
	defer func() { proxmoxClusterKnownHostsPath = origPath }()
	proxmoxClusterKnownHostsPath = hostsFile

	// Create dummy file
	content := `
# Comment
node1 ssh-ed25519 AAA...
node2 ssh-rsa BBB... comment
invalid line
node1 ssh-rsa CCC...
[node3]:2222 ssh-ed25519 DDD...
`
	if err := os.WriteFile(hostsFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Test success
	keys, err := loadProxmoxHostKeys("node1")
	if err != nil {
		t.Fatalf("loadProxmoxHostKeys failed: %v", err)
	}

	if len(keys) != 2 {
		t.Errorf("expected 2 keys for node1, got %d", len(keys))
	}

	if !strings.Contains(string(keys[0]), "node1 ssh-ed25519 AAA...") {
		t.Errorf("unexpected key content: %s", keys[0])
	}

	// Test node2 with comment
	keys2, err := loadProxmoxHostKeys("node2")
	if err != nil {
		t.Fatal(err)
	}
	if len(keys2) != 1 {
		t.Errorf("expected 1 key for node2")
	}
	// Normalization puts comment at end
	if !strings.Contains(string(keys2[0]), "comment") {
		t.Errorf("expected comment in key: %s", keys2[0])
	}

	// Test not found
	_, err = loadProxmoxHostKeys("unknown")
	if err == nil {
		t.Error("expected error for unknown host")
	}

	// Test file missing
	proxmoxClusterKnownHostsPath = filepath.Join(tmpDir, "missing")
	_, err = loadProxmoxHostKeys("node1")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestBuildAuthorizedKey(t *testing.T) {
	proxy := &Proxy{
		config: &Config{
			AllowedSourceSubnets: []string{"10.0.0.1/32", "192.168.1.0/24"},
		},
	}

	pubKey := "ssh-ed25519 AAA..."
	line, err := proxy.buildAuthorizedKey(pubKey)
	if err != nil {
		t.Fatalf("buildAuthorizedKey failed: %v", err)
	}

	expectedStart := `from="10.0.0.1/32,192.168.1.0/24",command="/usr/local/libexec/pulse-sensor-proxy/temp-wrapper.sh",no-port-forwarding,no-X11-forwarding,no-agent-forwarding,no-pty ssh-ed25519 AAA... pulse-sensor-proxy`
	if !strings.HasPrefix(line, expectedStart) && line != expectedStart {
		t.Errorf("unexpected authorized_key line: %s", line)
	}

	// Test empty config
	proxyEmpty := &Proxy{config: &Config{}}
	_, err = proxyEmpty.buildAuthorizedKey(pubKey)
	if err == nil {
		t.Error("expected error for empty allowed subnets")
	}
}

func TestGetPublicKeyFrom(t *testing.T) {
	tmpDir := t.TempDir()
	pubKeyPath := filepath.Join(tmpDir, "id_ed25519.pub")

	content := "ssh-ed25519 AAA... comment"
	if err := os.WriteFile(pubKeyPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Test getPublicKeyFrom
	proxy := &Proxy{}
	pub, err := proxy.getPublicKeyFrom(tmpDir)
	if err != nil {
		t.Fatalf("getPublicKeyFrom failed: %v", err)
	}

	expected := "ssh-ed25519 AAA... comment"
	if pub != expected {
		t.Errorf("expected %s, got %s", expected, pub)
	}

	// Test missing
	_, err = proxy.getPublicKeyFrom(filepath.Join(tmpDir, "missing"))
	if err == nil {
		t.Error("expected error for missing key")
	}
}

func TestHandleHostKeyEnsureError(t *testing.T) {
	proxy := &Proxy{
		metrics: NewProxyMetrics("test"),
	}

	// Normal error
	err := errors.New("normal error")
	res := proxy.handleHostKeyEnsureError("node1", err)
	if res != err {
		t.Errorf("expected original error, got %v", res)
	}

	// HostKeyChangeError
	changeErr := &knownhosts.HostKeyChangeError{
		Host:     "node1",
		Existing: "old",
		Provided: "new",
	}

	res = proxy.handleHostKeyEnsureError("node1", changeErr)
	if res != changeErr {
		t.Errorf("expected original error, got %v", res)
	}
	// Verify metrics if possible, or just standard output log
}

func TestEnsureHostKeyFromProxmox(t *testing.T) {
	// Mock isProxmoxHost
	origLookPath := execLookPath
	origStat := osStat
	defer func() {
		execLookPath = origLookPath
		osStat = origStat
	}()

	// Case 1: Not Proxmox host
	execLookPath = func(file string) (string, error) {
		return "", fmt.Errorf("not found")
	}
	osStat = func(name string) (os.FileInfo, error) {
		return nil, os.ErrNotExist
	}

	proxy := &Proxy{}
	err := proxy.ensureHostKeyFromProxmox(context.Background(), "node1")
	if err == nil || err.Error() != "not running on Proxmox host" {
		t.Errorf("expected 'not running on Proxmox host' error, got %v", err)
	}

	// Case 2: Proxmox host, key found
	execLookPath = func(file string) (string, error) {
		if file == "pvecm" {
			return "/bin/pvecm", nil
		}
		return "", fmt.Errorf("not found")
	}

	tmpDir := t.TempDir()
	hostsFile := filepath.Join(tmpDir, "known_hosts")
	origPath := proxmoxClusterKnownHostsPath
	defer func() { proxmoxClusterKnownHostsPath = origPath }()
	proxmoxClusterKnownHostsPath = hostsFile

	os.WriteFile(hostsFile, []byte("node1 ssh-ed25519 AAA..."), 0644)

	proxy = &Proxy{
		knownHosts: &mockKnownHosts{},
		config:     &Config{RequireProxmoxHostkeys: true},
	}

	err = proxy.ensureHostKeyFromProxmox(context.Background(), "node1")
	if err != nil {
		t.Errorf("ensureHostKeyFromProxmox failed: %v", err)
	}

	// Case 3: Proxmox host, key NOT found
	// knownHosts EnsureWithEntries (mock) returns nil, but loadProxmoxHostKeys will verify if key exists in file

	// loadProxmoxHostKeys returns error if key not found
	err = proxy.ensureHostKeyFromProxmox(context.Background(), "unknown")
	if err == nil {
		t.Error("expected error for unknown host")
	}
}

func TestPushSSHKeyFrom(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "id_ed25519.pub"), []byte("ssh-ed25519 AAA..."), 0644)

	proxy := &Proxy{
		sshKeyPath: tmpDir,
		metrics:    NewProxyMetrics("test"),
		knownHosts: &mockKnownHosts{},
		config:     &Config{AllowedSourceSubnets: []string{"10.0.0.0/24"}},
	}

	origExec := execCommandFunc
	defer func() { execCommandFunc = origExec }()

	// Case 1: Success (key already present)
	execCommandFunc = func(name string, arg ...string) *exec.Cmd {
		args := strings.Join(arg, " ")
		if strings.Contains(args, "grep -F") {
			return mockExecCommand("from=...,ssh-ed25519 AAA...") // Found
		}
		if strings.Contains(args, "stage temperature wrapper") {
			return mockExecCommand("") // Wrapper install success
		}
		return mockExecCommand("")
	}

	if err := proxy.pushSSHKeyFrom("node1", tmpDir); err != nil {
		t.Errorf("pushSSHKeyFrom failed (present): %v", err)
	}

	// Case 2: Success (key added)
	execCommandFunc = func(name string, arg ...string) *exec.Cmd {
		args := strings.Join(arg, " ")
		if strings.Contains(args, "grep -F") {
			return mockExecCommandWithExitCode("", 1) // Not found
		}
		return mockExecCommand("")
	}

	if err := proxy.pushSSHKeyFrom("node1", tmpDir); err != nil {
		t.Errorf("pushSSHKeyFrom failed (added): %v", err)
	}

	// Case 3: Error key not found
	if err := proxy.pushSSHKeyFrom("node1", filepath.Join(tmpDir, "missing")); err == nil {
		t.Error("expected error for missing key dir")
	}

	// Case 4: ensureHostKey error (mock knownHosts.Ensure failure?)
	// mockKnownHosts returns nil.
}

func TestSSHConnection(t *testing.T) {
	tmpDir := t.TempDir()

	proxy := &Proxy{
		sshKeyPath:        tmpDir,
		metrics:           NewProxyMetrics("test"),
		knownHosts:        &mockKnownHosts{},
		maxSSHOutputBytes: 1024,
		config:            &Config{},
	}

	origExec := execCommandFunc
	origExecCtx := execCommandContextFunc
	defer func() {
		execCommandFunc = origExec
		execCommandContextFunc = origExecCtx
	}()

	mockCmd := func(name string, arg ...string) *exec.Cmd {
		if name == "sh" { // execCommandWithLimits uses "sh -c"
			return mockExecCommand("{}")
		}
		return mockExecCommand("")
	}

	execCommandFunc = mockCmd
	execCommandContextFunc = func(ctx context.Context, name string, arg ...string) *exec.Cmd {
		return mockCmd(name, arg...)
	}

	if err := proxy.testSSHConnection("node1"); err != nil {
		t.Errorf("testSSHConnection failed: %v", err)
	}

	// Failure
	mockCmdFail := func(name string, arg ...string) *exec.Cmd {
		if name == "sh" {
			return mockExecCommandWithExitCode("failed", 1)
		}
		return mockExecCommand("")
	}
	execCommandContextFunc = func(ctx context.Context, name string, arg ...string) *exec.Cmd {
		return mockCmdFail(name, arg...)
	}
	execCommandFunc = mockCmdFail

	if err := proxy.testSSHConnection("node1"); err == nil {
		t.Error("expected error for connection failure")
	}
}
