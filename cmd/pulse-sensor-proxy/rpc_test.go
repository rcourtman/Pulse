package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

type mockKnownHosts struct{}

func (m *mockKnownHosts) Ensure(ctx context.Context, host string) error                   { return nil }
func (m *mockKnownHosts) EnsureWithPort(ctx context.Context, host string, port int) error { return nil }
func (m *mockKnownHosts) EnsureWithEntries(ctx context.Context, host string, port int, entries [][]byte) error {
	return nil
}
func (m *mockKnownHosts) Path() string { return "/tmp/known_hosts" }

func TestHandleGetStatusV2(t *testing.T) {
	tmpDir := t.TempDir()
	pubKeyPath := filepath.Join(tmpDir, "id_ed25519.pub")
	os.WriteFile(pubKeyPath, []byte("ssh-ed25519 AAA..."), 0644)

	proxy := &Proxy{
		sshKeyPath: tmpDir,
	}

	req := &RPCRequest{}
	logger := zerolog.New(os.Stderr)

	resp, err := proxy.handleGetStatusV2(context.Background(), req, logger)
	if err != nil {
		t.Fatalf("handleGetStatusV2 failed: %v", err)
	}

	m, ok := resp.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map response, got %T", resp)
	}

	if m["version"] != Version {
		t.Errorf("expected version %s, got %v", Version, m["version"])
	}
	if m["public_key"] != "ssh-ed25519 AAA..." {
		t.Errorf("unexpected public key: %v", m["public_key"])
	}
}

func TestHandleGetTemperatureV2(t *testing.T) {
	// Mock execLookPath and execCommandFunc
	origLookPath := execLookPath
	origExec := execCommandFunc
	origExecCtx := execCommandContextFunc
	defer func() {
		execLookPath = origLookPath
		execCommandFunc = origExec
		execCommandContextFunc = origExecCtx
	}()

	execLookPath = func(file string) (string, error) {
		return "/usr/bin/" + file, nil
	}

	mockCmd := func(ctx context.Context, name string, arg ...string) *exec.Cmd {
		args := strings.Join(arg, " ")
		if name == "sh" && strings.Contains(args, "ssh") {
			// Mock SSH output with sensor JSON
			output := `{"cpu": {"temp1": {"temp1_input": 45.0}}}`
			return mockExecCommand(output)
		}
		// For local node
		if name == "sensors" {
			output := `{"cpu": {"temp1": {"temp1_input": 45.0}}}`
			return mockExecCommand(output)
		}
		return mockExecCommand("")
	}

	execCommandFunc = func(name string, arg ...string) *exec.Cmd {
		return mockCmd(context.Background(), name, arg...)
	}
	execCommandContextFunc = func(ctx context.Context, name string, arg ...string) *exec.Cmd {
		return mockCmd(ctx, name, arg...)
	}

	proxy := &Proxy{
		config: &Config{
			AllowedNodes: []string{"node1"},
		},
		metrics:    NewProxyMetrics("test"),
		nodeGate:   newNodeGate(),
		knownHosts: &mockKnownHosts{},
	}

	logger := zerolog.New(os.Stderr)

	// Success case
	req := &RPCRequest{
		Params: map[string]interface{}{
			"node": "node1",
		},
	}

	resp, err := proxy.handleGetTemperatureV2(context.Background(), req, logger)
	if err != nil {
		t.Fatalf("handleGetTemperatureV2 failed: %v", err)
	}

	m, ok := resp.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map response, got %T", resp)
	}
	respStr, ok := m["temperature"].(string)
	if !ok {
		t.Errorf("expected string temperature, got %T", m["temperature"])
	}
	if !strings.Contains(respStr, "45.0") {
		t.Errorf("expected 45.0 in response, got %s", respStr)
	}

	// Missing parameter
	reqMissing := &RPCRequest{Params: map[string]interface{}{}}
	_, err = proxy.handleGetTemperatureV2(context.Background(), reqMissing, logger)
	if err == nil {
		t.Error("expected error for missing node param")
	}

	// Invalid node (bad format)
	reqInvalid := &RPCRequest{
		Params: map[string]interface{}{
			"node": "invalid node!", // Space should make it invalid
		},
	}
	_, err = proxy.handleGetTemperatureV2(context.Background(), reqInvalid, logger)
	if err == nil {
		t.Error("expected error for invalid node")
	}
}

func TestHandleRegisterNodesV2(t *testing.T) {
	// Mock pvecm
	origLookPath := execLookPath
	origExec := execCommandFunc
	origExecCtx := execCommandContextFunc
	defer func() {
		execLookPath = origLookPath
		execCommandFunc = origExec
		execCommandContextFunc = origExecCtx
	}()

	execLookPath = func(file string) (string, error) {
		if file == "pvecm" {
			return "/usr/sbin/pvecm", nil
		}
		return "", fmt.Errorf("not found")
	}

	mockCmd := func(ctx context.Context, name string, arg ...string) *exec.Cmd {
		args := strings.Join(arg, " ")
		if name == "pvecm" && args == "status" {
			// Mock pvecm status output
			output := "\nCluster information\n-------------------\nName: cluster\nConfig Version: 1\nTransport: knet\nSecure auth: on\n\nQuorum information\n------------------\nDate: Sun Jan 1 00:00:00 2023\nQuorum provider: corosync_votequorum\nNodes: 1\nNode ID: 0x00000001\nRing ID: 1.1\nQuorate: Yes\n\nVotequorum information\n----------------------\nExpected votes: 1\nHighest expected: 1\nTotal votes: 1\nQuorum: 1\nFlags: Quorate \n\nMembership information\n----------------------\n    Nodeid      Votes Name\n0x00000001          1 10.0.0.1 (local)\n"
			return mockExecCommand(output)
		}

		// Mock SSH connection test
		if name == "sh" && strings.Contains(args, "ssh") {
			// Success
			return mockExecCommand("")
		}
		return mockExecCommand("")
	}

	execCommandFunc = func(name string, arg ...string) *exec.Cmd {
		return mockCmd(context.Background(), name, arg...)
	}
	execCommandContextFunc = func(ctx context.Context, name string, arg ...string) *exec.Cmd {
		return mockCmd(ctx, name, arg...)
	}

	tmpDir := t.TempDir()
	proxy := &Proxy{
		sshKeyPath: tmpDir,
		metrics:    NewProxyMetrics("test"),
		knownHosts: &mockKnownHosts{},
		config:     &Config{},
	}

	logger := zerolog.New(os.Stderr)
	req := &RPCRequest{}

	resp, err := proxy.handleRegisterNodesV2(context.Background(), req, logger)
	if err != nil {
		t.Fatalf("handleRegisterNodesV2 failed: %v", err)
	}

	m, ok := resp.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map response, got %T", resp)
	}

	nodes, ok := m["nodes"].([]map[string]interface{})
	if !ok {
		t.Fatalf("expected nodes list, got %T", m["nodes"])
	}

	if len(nodes) != 1 {
		t.Errorf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0]["name"] != "10.0.0.1" {
		t.Errorf("expected node name 10.0.0.1, got %v", nodes[0]["name"])
	}
	if nodes[0]["ssh_ready"] != true {
		t.Errorf("expected ssh_ready true, got %v, error: %v", nodes[0]["ssh_ready"], nodes[0]["error"])
	}
}

func TestHandleConnection_InvalidJSON(t *testing.T) {
	// Mock extractPeerCredentials
	origExtract := extractPeerCredentials
	defer func() { extractPeerCredentials = origExtract }()
	extractPeerCredentials = func(conn net.Conn) (*peerCredentials, error) {
		return &peerCredentials{uid: 1000, gid: 1000, pid: 123}, nil
	}

	metrics := NewProxyMetrics("test")
	proxy := &Proxy{
		metrics: metrics,
		config: &Config{
			AllowedPeerUIDs: []uint32{1000},
			RateLimit: &RateLimitConfig{
				PerPeerIntervalMs: 1,
				PerPeerBurst:      10,
			},
		},
		allowedPeerUIDs: map[uint32]struct{}{1000: {}},
		peerCapabilities: map[uint32]Capability{
			1000: 3, // LegacyAll
		},
		rateLimiter: newRateLimiter(metrics, &RateLimitConfig{
			PerPeerIntervalMs: 1,
			PerPeerBurst:      10,
		}, nil, nil),
	}

	client, server := net.Pipe()

	go func() {
		defer client.Close()
		client.Write([]byte("invalid json\n"))
	}()

	// handleConnection handles the connection and returns
	proxy.handleConnection(server)
	server.Close()
}

func TestHandleEnsureClusterKeysV2(t *testing.T) {
	// Mock pvecm and ssh
	origLookPath := execLookPath
	origExec := execCommandFunc
	defer func() {
		execLookPath = origLookPath
		execCommandFunc = origExec
	}()

	execLookPath = func(file string) (string, error) {
		return "/usr/bin/" + file, nil
	}

	execCommandFunc = func(name string, arg ...string) *exec.Cmd {
		args := strings.Join(arg, " ")
		if name == "pvecm" && args == "status" {
			output := "\nCluster information\n-------------------\nName: cluster\nConfig Version: 1\nTransport: knet\nSecure auth: on\n\nQuorum information\n------------------\nDate: Sun Jan 1 00:00:00 2023\nQuorum provider: corosync_votequorum\nNodes: 1\nNode ID: 0x00000001\nRing ID: 1.1\nQuorate: Yes\n\nVotequorum information\n----------------------\nExpected votes: 1\nHighest expected: 1\nTotal votes: 1\nQuorum: 1\nFlags: Quorate \n\nMembership information\n----------------------\n    Nodeid      Votes Name\n0x00000001          1 10.0.0.1 (local)\n"
			return mockExecCommand(output)
		}

		if name == "sh" && strings.Contains(args, "ssh") {
			// Check if key exists (grep) - return empty (not found) to trigger adding
			if strings.Contains(args, "grep -F") {
				return mockExecCommandWithExitCode("", 1) // 1 = not found
			}
			// ensureTempWrapper or key push
			return mockExecCommand("")
		}
		return mockExecCommand("")
	}

	tmpDir := t.TempDir()
	// Create SSH keys in tmpDir (for push)
	os.WriteFile(filepath.Join(tmpDir, "id_ed25519.pub"), []byte("ssh-ed25519 KEY"), 0644)

	proxy := &Proxy{
		sshKeyPath: tmpDir,
		metrics:    NewProxyMetrics("test"),
		knownHosts: &mockKnownHosts{},
		config: &Config{
			AllowedSourceSubnets: []string{"10.0.0.0/24"},
		},
	}

	logger := zerolog.New(os.Stderr)
	req := &RPCRequest{}

	resp, err := proxy.handleEnsureClusterKeysV2(context.Background(), req, logger)
	if err != nil {
		t.Fatalf("handleEnsureClusterKeysV2 failed: %v", err)
	}

	m, ok := resp.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map response, got %T", resp)
	}

	if m["success_count"] != 1 {
		t.Errorf("expected 1 success, got %v", m["success_count"])
	}
}

// mockExecCommandWithExitCode helper needed for grep fail
func mockExecCommandWithExitCode(output string, exitCode int) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcess", "--", output}
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{
		"GO_WANT_HELPER_PROCESS=1",
		"GO_HELPER_OUTPUT=" + output,
		fmt.Sprintf("GO_HELPER_EXIT_CODE=%d", exitCode),
	}
	return cmd
}
