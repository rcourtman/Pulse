package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ssh/knownhosts"
	"github.com/rs/zerolog"
)

// TestPrivilegedMethodsCompleteness ensures all host-side RPC methods are in privilegedMethods
func TestPrivilegedMethodsCompleteness(t *testing.T) {
	// Define RPC methods that expose host-side effects
	hostSideEffects := map[string]string{
		RPCEnsureClusterKeys: "SSH key distribution to cluster nodes",
		RPCRegisterNodes:     "Node discovery and registration",
		RPCRequestCleanup:    "Cleanup operations on host",
	}

	// Verify each host-side effect RPC is in privilegedMethods
	for method, description := range hostSideEffects {
		if !privilegedMethods[method] {
			t.Errorf("SECURITY: %s (%s) is not in privilegedMethods - containers can call it!", method, description)
		}
	}

	// Verify read-only methods are NOT in privilegedMethods
	readOnlyMethods := map[string]string{
		RPCGetStatus:      "proxy status query",
		RPCGetTemperature: "temperature data query",
	}

	for method, description := range readOnlyMethods {
		if privilegedMethods[method] {
			t.Errorf("Read-only method %s (%s) should not be in privilegedMethods", method, description)
		}
	}
}

// TestPrivilegedMethodsBlocked ensures containers cannot call privileged methods
func TestPrivilegedMethodsBlocked(t *testing.T) {
	p := &Proxy{
		config:            &Config{AllowIDMappedRoot: true},
		allowedPeerUIDs:   map[uint32]struct{}{0: {}},
		allowedPeerGIDs:   map[uint32]struct{}{0: {}},
		peerCapabilities:  map[uint32]Capability{0: capabilityLegacyAll},
		idMappedUIDRanges: []idRange{{start: 100000, length: 65536}},
		idMappedGIDRanges: []idRange{{start: 100000, length: 65536}},
	}

	// Container credentials (ID-mapped root)
	containerCreds := &peerCredentials{
		uid: 101000, // Inside ID-mapped range
		gid: 101000,
		pid: 12345,
	}

	// Host credentials (real root)
	hostCreds := &peerCredentials{
		uid: 0,
		gid: 0,
		pid: 1,
	}

	// Test that containers ARE blocked from privileged methods
	t.Run("ContainerBlockedFromPrivilegedMethods", func(t *testing.T) {
		// Container should pass authentication
		caps, err := p.authorizePeer(containerCreds)
		if err != nil {
			t.Fatalf("Container should pass authentication, got: %v", err)
		}

		if caps.Has(CapabilityAdmin) {
			t.Fatal("Container should not have admin capability")
		}
	})

	// Test that host CAN call privileged methods
	t.Run("HostAllowedPrivilegedMethods", func(t *testing.T) {
		// Host should pass authentication
		caps, err := p.authorizePeer(hostCreds)
		if err != nil {
			t.Fatalf("Host should pass authentication, got: %v", err)
		}

		if !caps.Has(CapabilityAdmin) {
			t.Fatal("Host should have admin capability")
		}
	})
}

// TestIDMappedRootDetection tests container detection via ID mapping
func TestIDMappedRootDetection(t *testing.T) {
	p := &Proxy{
		config:            &Config{AllowIDMappedRoot: true},
		idMappedUIDRanges: []idRange{{start: 100000, length: 65536}},
		idMappedGIDRanges: []idRange{{start: 100000, length: 65536}},
	}

	tests := []struct {
		name       string
		cred       *peerCredentials
		isIDMapped bool
	}{
		{
			name:       "Container root (ID-mapped)",
			cred:       &peerCredentials{uid: 100000, gid: 100000},
			isIDMapped: true,
		},
		{
			name:       "Container user inside range",
			cred:       &peerCredentials{uid: 110000, gid: 110000},
			isIDMapped: true,
		},
		{
			name:       "Container at range boundary",
			cred:       &peerCredentials{uid: 165535, gid: 165535},
			isIDMapped: true,
		},
		{
			name:       "Host root",
			cred:       &peerCredentials{uid: 0, gid: 0},
			isIDMapped: false,
		},
		{
			name:       "Host user (low UID)",
			cred:       &peerCredentials{uid: 1000, gid: 1000},
			isIDMapped: false,
		},
		{
			name:       "Outside range (high)",
			cred:       &peerCredentials{uid: 200000, gid: 200000},
			isIDMapped: false,
		},
		{
			name:       "UID in range but GID not (should fail)",
			cred:       &peerCredentials{uid: 110000, gid: 50},
			isIDMapped: false,
		},
		{
			name:       "GID in range but UID not (should fail)",
			cred:       &peerCredentials{uid: 50, gid: 110000},
			isIDMapped: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.isIDMappedRoot(tt.cred)
			if got != tt.isIDMapped {
				t.Errorf("isIDMappedRoot() = %v, want %v for uid=%d gid=%d",
					got, tt.isIDMapped, tt.cred.uid, tt.cred.gid)
			}
		})
	}
}

// TestIDMappedRootWithoutRanges tests behavior when no ID ranges configured
func TestIDMappedRootWithoutRanges(t *testing.T) {
	p := &Proxy{
		config:            &Config{AllowIDMappedRoot: true},
		idMappedUIDRanges: []idRange{}, // Empty
		idMappedGIDRanges: []idRange{}, // Empty
	}

	// Should return false when no ranges are configured
	cred := &peerCredentials{uid: 110000, gid: 110000}
	if p.isIDMappedRoot(cred) {
		t.Error("isIDMappedRoot should return false when no ranges configured")
	}
}

// TestIDMappedRootDisabled tests when AllowIDMappedRoot is disabled
func TestIDMappedRootDisabled(t *testing.T) {
	p := &Proxy{
		config:            &Config{AllowIDMappedRoot: false},
		allowedPeerUIDs:   map[uint32]struct{}{0: {}},
		peerCapabilities:  map[uint32]Capability{0: capabilityLegacyAll},
		idMappedUIDRanges: []idRange{{start: 100000, length: 65536}},
		idMappedGIDRanges: []idRange{{start: 100000, length: 65536}},
	}

	// Container credentials
	cred := &peerCredentials{uid: 110000, gid: 110000}

	// Should fail authorization when AllowIDMappedRoot is false
	if _, err := p.authorizePeer(cred); err == nil {
		t.Error("authorizePeer should fail for ID-mapped root when AllowIDMappedRoot is false")
	}
}

// TestMultipleIDRanges tests handling of multiple ID mapping ranges
func TestMultipleIDRanges(t *testing.T) {
	p := &Proxy{
		config: &Config{AllowIDMappedRoot: true},
		idMappedUIDRanges: []idRange{
			{start: 100000, length: 65536},
			{start: 200000, length: 65536},
		},
		idMappedGIDRanges: []idRange{
			{start: 100000, length: 65536},
			{start: 200000, length: 65536},
		},
	}

	tests := []struct {
		name       string
		uid        uint32
		gid        uint32
		isIDMapped bool
	}{
		{"First range", 110000, 110000, true},
		{"Second range", 210000, 210000, true},
		{"Between ranges", 180000, 180000, false},
		{"Below ranges", 50000, 50000, false},
		{"Above ranges", 300000, 300000, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cred := &peerCredentials{uid: tt.uid, gid: tt.gid}
			got := p.isIDMappedRoot(cred)
			if got != tt.isIDMapped {
				t.Errorf("isIDMappedRoot() = %v, want %v for uid=%d gid=%d",
					got, tt.isIDMapped, tt.uid, tt.gid)
			}
		})
	}
}

func TestParseLogLevel(t *testing.T) {
	cases := []struct {
		input string
		want  zerolog.Level
	}{
		{"trace", zerolog.TraceLevel},
		{"debug", zerolog.DebugLevel},
		{"info", zerolog.InfoLevel},
		{"warn", zerolog.WarnLevel},
		{"warning", zerolog.WarnLevel},
		{"error", zerolog.ErrorLevel},
		{"fatal", zerolog.FatalLevel},
		{"panic", zerolog.PanicLevel},
		{"disabled", zerolog.Disabled},
		{"none", zerolog.Disabled},
		{"unknown", zerolog.InfoLevel},
	}
	for _, tc := range cases {
		if got := parseLogLevel(tc.input); got != tc.want {
			t.Errorf("parseLogLevel(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestHandleGetStatusV2(t *testing.T) {
	sshDir := t.TempDir()
	pubKeyPath := filepath.Join(sshDir, "id_ed25519.pub")
	os.WriteFile(pubKeyPath, []byte("test-key"), 0644)

	p := &Proxy{
		sshKeyPath: sshDir,
	}

	ctx := context.Background()
	ctx = withPeerCapabilities(ctx, CapabilityAdmin|CapabilityRead)

	resp, err := p.handleGetStatusV2(ctx, &RPCRequest{}, zerolog.Nop())
	if err != nil {
		t.Fatal(err)
	}

	data := resp.(map[string]interface{})
	if data["public_key"] != "test-key" {
		t.Errorf("expected test-key, got %v", data["public_key"])
	}
	if data["ssh_dir"] != sshDir {
		t.Errorf("expected ssh_dir %s, got %v", sshDir, data["ssh_dir"])
	}
	caps := data["capabilities"].([]string)
	found := false
	for _, c := range caps {
		if c == "admin" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected admin capability in response, got %v", caps)
	}
}

func TestHandleGetTemperatureV2_EdgeCases(t *testing.T) {
	p := &Proxy{
		nodeGate: newNodeGate(),
		metrics:  NewProxyMetrics("test"),
	}

	// Missing node
	_, err := p.handleGetTemperatureV2(context.Background(), &RPCRequest{
		Params: map[string]interface{}{},
	}, zerolog.Nop())
	if err == nil || !strings.Contains(err.Error(), "missing 'node'") {
		t.Errorf("expected missing node error, got %v", err)
	}

	// Node not a string
	_, err = p.handleGetTemperatureV2(context.Background(), &RPCRequest{
		Params: map[string]interface{}{"node": 123},
	}, zerolog.Nop())
	if err == nil || !strings.Contains(err.Error(), "must be a string") {
		t.Errorf("expected string type error, got %v", err)
	}

	// Invalid node name
	_, err = p.handleGetTemperatureV2(context.Background(), &RPCRequest{
		Params: map[string]interface{}{"node": "-invalid"},
	}, zerolog.Nop())
	if err == nil || !strings.Contains(err.Error(), "invalid node name") {
		t.Errorf("expected invalid node error, got %v", err)
	}
}

func TestHandleGetTemperatureV2_ValidationAndLock(t *testing.T) {
	v := &nodeValidator{
		hasAllowlist: true,
		allowHosts:   map[string]struct{}{"allowed": {}},
		resolver:     stubResolver{ips: []net.IP{net.ParseIP("10.0.0.1")}},
	}
	p := &Proxy{
		nodeGate:      newNodeGate(),
		nodeValidator: v,
		metrics:       NewProxyMetrics("test"),
	}

	// Validation fails
	_, err := p.handleGetTemperatureV2(context.Background(), &RPCRequest{
		Params: map[string]interface{}{"node": "denied"},
	}, zerolog.Nop())
	if err == nil || !strings.Contains(err.Error(), "rejected") {
		t.Errorf("expected validation error, got %v", err)
	}

	// Lock acquisition cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately
	_, err = p.handleGetTemperatureV2(ctx, &RPCRequest{
		Params: map[string]interface{}{"node": "allowed"},
	}, zerolog.Nop())
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

func TestIsProxmoxHost(t *testing.T) {
	// By default it might return false unless run on real Proxmox
	result := isProxmoxHost()
	t.Logf("isProxmoxHost() = %v", result)

	// Mock pvecm
	tmpDir := t.TempDir()
	pvecmPath := filepath.Join(tmpDir, "pvecm")
	os.WriteFile(pvecmPath, []byte("#!/bin/sh\nexit 0"), 0755)

	oldPath := os.Getenv("PATH")
	os.Setenv("PATH", tmpDir+string(os.PathListSeparator)+oldPath)
	defer os.Setenv("PATH", oldPath)

	if !isProxmoxHost() {
		t.Error("expected isProxmoxHost to be true when pvecm exists in PATH")
	}
}

func TestPeerCapabilitiesFromContext_Nil(t *testing.T) {
	if caps := peerCapabilitiesFromContext(context.TODO()); caps != 0 {
		t.Errorf("expected 0 caps for nil (TODO) context, got %v", caps)
	}
	if caps := peerCapabilitiesFromContext(context.Background()); caps != 0 {
		t.Errorf("expected 0 caps for empty context, got %v", caps)
	}
}

func TestSendResponse(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	p := &Proxy{writeTimeout: 1 * time.Second}
	resp := RPCResponse{CorrelationID: "test", Success: true}

	go p.sendResponse(c1, resp, 0)

	var received RPCResponse
	err := json.NewDecoder(c2).Decode(&received)
	if err != nil {
		t.Fatal(err)
	}
	if received.CorrelationID != "test" {
		t.Errorf("expected correlation ID test, got %s", received.CorrelationID)
	}
}

func TestSendErrorV2(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	p := &Proxy{writeTimeout: 1 * time.Second}
	go p.sendErrorV2(c1, "test error", "corr-123")

	var received RPCResponse
	err := json.NewDecoder(c2).Decode(&received)
	if err != nil {
		t.Fatal(err)
	}

	if received.CorrelationID != "corr-123" {
		t.Errorf("expected correlation ID 'corr-123', got %q", received.CorrelationID)
	}
	if received.Success {
		t.Error("expected Success=false")
	}
	if received.Error != "test error" {
		t.Errorf("expected error 'test error', got %q", received.Error)
	}
}

func TestEnsureSSHKeypair(t *testing.T) {
	tmpDir := t.TempDir()
	p := &Proxy{sshKeyPath: tmpDir}

	err := p.ensureSSHKeypair()
	if err != nil {
		t.Fatal(err)
	}

	privKey := filepath.Join(tmpDir, "id_ed25519")
	if _, err := os.Stat(privKey); err != nil {
		t.Error("private key not created")
	}

	// Test existing key
	err = p.ensureSSHKeypair()
	if err != nil {
		t.Fatal(err)
	}
}

func TestHandleEnsureClusterKeysV2(t *testing.T) {
	// Mock pvecm and ssh
	tmpDir := t.TempDir()

	pvecmPath := filepath.Join(tmpDir, "pvecm")
	script := "#!/bin/sh\necho \"0x00000001 1 10.0.0.1\"\n"
	os.WriteFile(pvecmPath, []byte(script), 0755)

	sshPath := filepath.Join(tmpDir, "ssh")
	os.WriteFile(sshPath, []byte("#!/bin/sh\necho 'test-key'\nexit 0"), 0755)

	sshKeygenPath := filepath.Join(tmpDir, "ssh-keygen")
	os.WriteFile(sshKeygenPath, []byte("#!/bin/sh\nexit 0"), 0755)

	oldPath := os.Getenv("PATH")
	os.Setenv("PATH", tmpDir+string(os.PathListSeparator)+oldPath)
	defer os.Setenv("PATH", oldPath)

	sshDir := t.TempDir()
	os.WriteFile(filepath.Join(sshDir, "id_ed25519.pub"), []byte("test-key"), 0644)
	os.WriteFile(filepath.Join(sshDir, "id_ed25519"), []byte("test-priv"), 0600)

	cfg := &Config{
		AllowedSourceSubnets: []string{"10.0.0.1/32"},
	}
	p := &Proxy{
		sshKeyPath: sshDir,
		config:     cfg,
		metrics:    NewProxyMetrics("test"),
	}

	_, err := p.handleEnsureClusterKeysV2(context.Background(), &RPCRequest{}, zerolog.Nop())
	if err != nil {
		t.Errorf("handleEnsureClusterKeysV2 failed: %v", err)
	}
}

func TestHandleRegisterNodesV2(t *testing.T) {
	// Mock pvecm and ssh
	tmpDir := t.TempDir()

	pvecmPath := filepath.Join(tmpDir, "pvecm")
	script := "#!/bin/sh\necho \"0x00000001 1 10.0.0.1\"\n"
	os.WriteFile(pvecmPath, []byte(script), 0755)

	sshPath := filepath.Join(tmpDir, "ssh")
	os.WriteFile(sshPath, []byte("#!/bin/sh\nexit 0"), 0755)

	oldPath := os.Getenv("PATH")
	os.Setenv("PATH", tmpDir+string(os.PathListSeparator)+oldPath)
	defer os.Setenv("PATH", oldPath)

	p := &Proxy{
		metrics: NewProxyMetrics("test"),
	}

	_, err := p.handleRegisterNodesV2(context.Background(), &RPCRequest{}, zerolog.Nop())
	if err != nil {
		t.Errorf("handleRegisterNodesV2 failed: %v", err)
	}
}

func TestResolveUserSpec(t *testing.T) {
	// Only test lookup failure for non-existent user on Linux
	if _, err := resolveUserSpec("non-existent-user-12345"); err == nil {
		t.Error("expected error for non-existent user")
	}

	// We can't easily test success without knowing a valid user on the system
	// But we can test the fallback if we mock /etc/passwd or similar,
	// however resolveUserSpec reads directly from system or file.
	// Let's create a temporary /etc/passwd file and use it?
	// lookupUserFromPasswd uses a hardcoded /etc/passwd.
	// So we can only test the failure case reliably across all envs.
}

func TestDropPrivileges(t *testing.T) {
	// Should return nil if username is empty
	if spec, err := dropPrivileges(""); err != nil || spec != nil {
		t.Errorf("expected nil spec and nil error for empty username, got %v, %v", spec, err)
	}

	// Should return nil if not root (assuming test is running as non-root)
	if os.Geteuid() != 0 {
		if spec, err := dropPrivileges("root"); err != nil || spec != nil {
			t.Errorf("expected nil spec and nil error when not root, got %v, %v", spec, err)
		}
	}
	// If running as root, this test might behave differently, but usually tests run as non-root.
}

func TestProxyStartStop(t *testing.T) {
	// Create temp dirs
	socketDir := t.TempDir()
	sshDir := t.TempDir()
	socketPath := filepath.Join(socketDir, "test.sock")

	// Ensure ssh keys exist so Start doesn't try to run ssh-keygen (which might be missing or fail)
	os.WriteFile(filepath.Join(sshDir, "id_ed25519"), []byte("priv"), 0600)
	os.WriteFile(filepath.Join(sshDir, "id_ed25519.pub"), []byte("pub"), 0644)
	os.WriteFile(filepath.Join(sshDir, "known_hosts"), []byte(""), 0644)

	metrics := NewProxyMetrics("test")
	p := &Proxy{
		socketPath: socketPath,
		sshKeyPath: sshDir,
		metrics:    metrics,
	}

	// Initialize known hosts
	km, err := knownhosts.NewManager(filepath.Join(sshDir, "known_hosts"))
	if err != nil {
		t.Fatal(err)
	}
	p.knownHosts = km

	if err := p.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Verify socket exists
	if _, err := os.Stat(socketPath); err != nil {
		t.Errorf("Socket file not created")
	}

	// Stop
	p.Stop()

	// Verify socket removed
	if _, err := os.Stat(socketPath); !os.IsNotExist(err) {
		t.Errorf("Socket file not removed")
	}
}

func TestVersionCmd(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	versionCmd.Run(versionCmd, []string{})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	if !strings.Contains(output, "pulse-sensor-proxy") {
		t.Errorf("expected version output to contain 'pulse-sensor-proxy', got %q", output)
	}
}

func TestKeysCmd(t *testing.T) {
	// Mock SSH dir env
	tmpDir := t.TempDir()
	os.Setenv("PULSE_SENSOR_PROXY_SSH_DIR", tmpDir)
	defer os.Unsetenv("PULSE_SENSOR_PROXY_SSH_DIR")

	// Write dummy key
	pubKeyPath := filepath.Join(tmpDir, "id_ed25519.pub")
	os.WriteFile(pubKeyPath, []byte("ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIKqy"), 0644)

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	keysCmd.Run(keysCmd, []string{})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	if !strings.Contains(output, "Proxy Public Key:") {
		t.Errorf("expected keys output to contain 'Proxy Public Key:', got %q", output)
	}
}

func TestHandleConnection(t *testing.T) {
	// Reduce timeouts for testing
	p := &Proxy{
		readTimeout:  1 * time.Second,
		writeTimeout: 1 * time.Second,
		metrics:      NewProxyMetrics("test"),
		router: map[string]handlerFunc{
			RPCGetStatus: func(ctx context.Context, req *RPCRequest, logger zerolog.Logger) (interface{}, error) {
				return map[string]string{"status": "ok"}, nil
			},
		},
		allowedPeerUIDs: map[uint32]struct{}{1000: {}},
		peerCapabilities: map[uint32]Capability{
			1000: capabilityLegacyAll,
		},
	}

	// Mock extractPeerCredentials
	origExtract := extractPeerCredentials
	defer func() { extractPeerCredentials = origExtract }()
	extractPeerCredentials = func(conn net.Conn) (*peerCredentials, error) {
		return &peerCredentials{uid: 1000, gid: 1000, pid: 123}, nil
	}

	p.rateLimiter = newRateLimiter(p.metrics, &RateLimitConfig{}, nil, nil)

	client, server := net.Pipe()
	defer client.Close()

	go p.handleConnection(server)

	// Send request
	req := RPCRequest{
		Method:        RPCGetStatus,
		CorrelationID: "123",
	}
	bytes, _ := json.Marshal(req)
	client.Write(bytes)
	client.Write([]byte("\n"))

	// Read response
	decoder := json.NewDecoder(client)
	var resp RPCResponse
	err := decoder.Decode(&resp)
	if err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Success {
		t.Errorf("expected success, got error: %s", resp.Error)
	}
}

func TestGetTemperatureViaSSH_Success(t *testing.T) {
	// Mock exec commands
	origExec := execCommandFunc
	origExecCtx := execCommandContextFunc
	defer func() {
		execCommandFunc = origExec
		execCommandContextFunc = origExecCtx
	}()

	execCommandFunc = func(name string, arg ...string) *exec.Cmd {
		// Mock hostname -f
		if name == "hostname" && len(arg) > 0 && arg[0] == "-f" {
			return errorExecCommand("unexpected command")
		}
		// Mock pvecm status
		if name == "pvecm" {
			return errorExecCommand("pvecm not found")
		}
		// Mock ip addr show
		if name == "ip" {
			return mockExecCommand("127.0.0.1/8")
		}
		return errorExecCommand("unexpected command: " + name)
	}

	execCommandContextFunc = func(ctx context.Context, name string, arg ...string) *exec.Cmd {
		args := strings.Join(arg, " ")
		if name == "sh" && strings.Contains(args, "ssh") {
			// Mock SSH successful output
			// We need to output valid JSON for sensors -j
			jsonOutput := `{"coretemp-isa-0000":{"Package id 0":{"temp1_input": 42.0}}}`
			return mockExecCommand(jsonOutput)
		}
		return errorExecCommand("unexpected command: " + name)
	}

	// Mock ssh key paths
	sshDir := t.TempDir()
	os.WriteFile(filepath.Join(sshDir, "id_ed25519"), []byte("priv"), 0600)
	os.WriteFile(filepath.Join(sshDir, "id_ed25519.pub"), []byte("pub"), 0644)
	os.WriteFile(filepath.Join(sshDir, "known_hosts"), []byte(""), 0644)

	// Mock keyscan to avoid calling real ssh-keyscan
	mockKeyscan := func(ctx context.Context, host string, port int, timeout time.Duration) ([]byte, error) {
		return []byte(fmt.Sprintf("%s ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIKqy\n", host)), nil
	}

	km, err := knownhosts.NewManager(filepath.Join(sshDir, "known_hosts"), knownhosts.WithKeyscanFunc(mockKeyscan))
	if err != nil {
		t.Fatalf("failed to create knownhosts manager: %v", err)
	}
	p := &Proxy{
		sshKeyPath:        sshDir,
		knownHosts:        km,
		metrics:           NewProxyMetrics("test"),
		maxSSHOutputBytes: 1024,
		config:            &Config{}, // Initialize config to avoid panic
	}

	// This should succeed because we mocked SSH output
	temp, err := p.getTemperatureViaSSH(context.Background(), "node1")
	if err != nil {
		t.Fatalf("getTemperatureViaSSH failed: %v", err)
	}

	if !strings.Contains(temp, "42.0") {
		t.Errorf("expected temp 42.0, got %s", temp)
	}
}

// Helper for mocking exec.Command
func mockExecCommand(output string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcess", "--", output}
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
	return cmd
}

func errorExecCommand(msg string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcess", "--", "ERROR:" + msg}
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
	return cmd
}

// TestHelperProcess isn't a real test. It's used to mock exec.Command
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	args := os.Args
	for len(args) > 0 {
		if args[0] == "--" {
			args = args[1:]
			break
		}
		args = args[1:]
	}
	if len(args) == 0 {
		os.Exit(0)
	}

	output := args[0]
	if strings.HasPrefix(output, "ERROR:") {
		fmt.Fprint(os.Stderr, output[6:])
		os.Exit(1)
	}

	fmt.Print(output)
	os.Exit(0)
}

func TestHandleConnection_Errors(t *testing.T) {
	// Setup proxy with mocks
	p := &Proxy{
		readTimeout:  1 * time.Second,
		writeTimeout: 1 * time.Second,
		metrics:      NewProxyMetrics("test"),
		router: map[string]handlerFunc{
			RPCGetStatus: func(ctx context.Context, req *RPCRequest, logger zerolog.Logger) (interface{}, error) {
				return map[string]string{"status": "ok"}, nil
			},
		},
		allowedPeerUIDs: map[uint32]struct{}{1000: {}},
		peerCapabilities: map[uint32]Capability{
			1000: capabilityLegacyAll,
		},
		config: &Config{},
	}
	p.rateLimiter = newRateLimiter(p.metrics, &RateLimitConfig{}, nil, nil)

	// Test 1: Credential Extraction Failure
	t.Run("CredFailure", func(t *testing.T) {
		origExtract := extractPeerCredentials
		defer func() { extractPeerCredentials = origExtract }()
		extractPeerCredentials = func(conn net.Conn) (*peerCredentials, error) {
			return nil, errors.New("extract failed")
		}

		client, server := net.Pipe()
		defer client.Close()
		go p.handleConnection(server)

		var resp RPCResponse
		json.NewDecoder(client).Decode(&resp)
		if resp.Success || resp.Error != "unauthorized" {
			t.Errorf("expected unauthorized error, got success=%v err=%s", resp.Success, resp.Error)
		}
	})

	// Test 2: Unauthorized Peer
	t.Run("UnauthorizedPeer", func(t *testing.T) {
		origExtract := extractPeerCredentials
		defer func() { extractPeerCredentials = origExtract }()
		extractPeerCredentials = func(conn net.Conn) (*peerCredentials, error) {
			return &peerCredentials{uid: 9999, gid: 9999}, nil // Unknown UID
		}

		client, server := net.Pipe()
		defer client.Close()
		go p.handleConnection(server)

		var resp RPCResponse
		json.NewDecoder(client).Decode(&resp)
		if resp.Success || resp.Error != "unauthorized" {
			t.Errorf("expected unauthorized error, got success=%v err=%s", resp.Success, resp.Error)
		}
	})

	// Test 3: Invalid JSON
	t.Run("InvalidJSON", func(t *testing.T) {
		origExtract := extractPeerCredentials
		defer func() { extractPeerCredentials = origExtract }()
		extractPeerCredentials = func(conn net.Conn) (*peerCredentials, error) {
			return &peerCredentials{uid: 1000, gid: 1000}, nil
		}

		client, server := net.Pipe()
		defer client.Close()
		go p.handleConnection(server)

		client.Write([]byte("invalid-json\n"))

		var resp RPCResponse
		json.NewDecoder(client).Decode(&resp)
		if resp.Success || resp.Error != "invalid request format" {
			t.Errorf("expected invalid format error, got %s", resp.Error)
		}
	})

	// Test 4: Unknown Method
	t.Run("UnknownMethod", func(t *testing.T) {
		origExtract := extractPeerCredentials
		defer func() { extractPeerCredentials = origExtract }()
		extractPeerCredentials = func(conn net.Conn) (*peerCredentials, error) {
			return &peerCredentials{uid: 1000, gid: 1000}, nil
		}

		client, server := net.Pipe()
		defer client.Close()
		go p.handleConnection(server)

		req := RPCRequest{Method: "unknown_method"}
		bytes, _ := json.Marshal(req)
		client.Write(bytes)
		client.Write([]byte("\n"))

		var resp RPCResponse
		json.NewDecoder(client).Decode(&resp)
		if resp.Success || resp.Error != "unknown method" {
			t.Errorf("expected unknown method error, got %s", resp.Error)
		}
	})

	// Test 5: Empty Request
	t.Run("EmptyRequest", func(t *testing.T) {
		origExtract := extractPeerCredentials
		defer func() { extractPeerCredentials = origExtract }()
		extractPeerCredentials = func(conn net.Conn) (*peerCredentials, error) {
			return &peerCredentials{uid: 1000, gid: 1000}, nil
		}

		client, server := net.Pipe()
		defer client.Close()
		go p.handleConnection(server)

		client.Write([]byte("\n")) // Just newline

		var resp RPCResponse
		json.NewDecoder(client).Decode(&resp)
		if resp.Success || resp.Error != "empty request" {
			t.Errorf("expected empty request error, got %s", resp.Error)
		}
	})
}

func TestGetTemperatureViaSSH_Failures(t *testing.T) {
	// Mock exec commands
	origExec := execCommandFunc
	origExecCtx := execCommandContextFunc
	defer func() {
		execCommandFunc = origExec
		execCommandContextFunc = origExecCtx
	}()

	// Test command failure
	execCommandContextFunc = func(ctx context.Context, name string, arg ...string) *exec.Cmd {
		args := strings.Join(arg, " ")
		if name == "sh" && strings.Contains(args, "ssh") {
			return errorExecCommand("ssh failed")
		}
		if name == "sensors" {
			return errorExecCommand("sensors failed")
		}
		return mockExecCommand("")
	}

	// Mock ssh key paths
	sshDir := t.TempDir()
	os.WriteFile(filepath.Join(sshDir, "id_ed25519"), []byte("priv"), 0600)
	os.WriteFile(filepath.Join(sshDir, "known_hosts"), []byte(""), 0644)

	mockKeyscan := func(ctx context.Context, host string, port int, timeout time.Duration) ([]byte, error) {
		return []byte(fmt.Sprintf("%s ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIKqy\n", host)), nil
	}
	km, _ := knownhosts.NewManager(filepath.Join(sshDir, "known_hosts"), knownhosts.WithKeyscanFunc(mockKeyscan))

	p := &Proxy{
		sshKeyPath:        sshDir,
		knownHosts:        km,
		metrics:           NewProxyMetrics("test"),
		maxSSHOutputBytes: 1024,
		config:            &Config{},
	}

	// Test SSH failure
	_, err := p.getTemperatureViaSSH(context.Background(), "remote-node")
	if err == nil {
		t.Error("expected error for ssh failure")
	}

	// Test Local sensors failure
	execCommandContextFunc = func(ctx context.Context, name string, arg ...string) *exec.Cmd {
		if name == "sensors" {
			return errorExecCommand("sensors failed")
		}
		return mockExecCommand("")
	}
	// isLocalNode depends on os.Hostname or netInterfaces
	// mock osHostname to match node name
	origHostname := osHostname
	defer func() { osHostname = origHostname }()
	osHostname = func() (string, error) { return "local-node", nil }

	// Should fallback to local sensors and fail
	// We need to allow ensureHostKey to succeed or skip it for local node?
	// ensureHostKey is called even for local node currently.
	// Assume ensureHostKey succeeds due to mockKeyscan.

	_, err = p.getTemperatureViaSSH(context.Background(), "local-node")
	// Since "sensors -j" fails, and fallback "sensors" fails, it returns "{}".
	// Wait, getTemperatureViaSSH returns "", error if SSH fails.
	// Ah, if localNode is true, and SSH fails, it attempts fallback.
	// And if fallback succeeds (returns non-empty), it returns string.
	// If fallback fails, it returns the SSH error.

	// If sensors fails, getTemperatureLocal returns "{}", nil.
	// So it returns "{}", nil if SSH fails and local fallback runs?
	// Let's check getTemperatureLocal code.
	// If "sensors -j" fails: cmd.Output() returns error.
	// It tries "sensors". If that fails, it returns error.
	// So getTemperatureLocal returns error.
	// If getTemperatureLocal returns error, getTemperatureViaSSH checks `localErr == nil`.
	// So if localErr != nil, it falls through to return SSH error.

	if err == nil {
		t.Errorf("expected error when both SSH and local sensors fail")
	}
}

func TestFetchAuthorizedNodes(t *testing.T) {
	// Mock HTTP server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/temperature-proxy/authorized-nodes" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if r.Header.Get("X-Proxy-Token") != "test-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"nodes": []map[string]string{
				{"name": "node1", "ip": "10.0.0.1"},
			},
			"hash":             "abc",
			"refresh_interval": 60,
		})
	}))
	defer ts.Close()

	// Use temp dir for config update
	tmpFile, err := os.CreateTemp("", "config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Close() // We just need the path, but empty file is fine as we are mocking writing usually?
	// Actually fetchAuthorizedNodes updates p.config.AllowedNodes.
	// No file writing happens in fetchAuthorizedNodes.

	p := &Proxy{
		controlPlaneCfg: &ControlPlaneConfig{
			URL: ts.URL,
		},
		controlPlaneToken: "test-token",
		config: &Config{
			AllowedNodes: []string{},
		},
		metrics: NewProxyMetrics("test"),
	}

	// Pre-req: nodeValidator must be set to update allowlist
	// But p.nodeValidator is private.
	// We can set it if we move it to be accessible or use a constructor.
	// However, fetchAuthorizedNodes calls p.nodeValidator.UpdateAllowlist if set.
	// Let's rely on it being nil or check if we can set it.
	// Oh, nodeValidator is a field in Proxy.
	// We can just set it.

	// Create a dummy nodeValidator
	validator, _ := newNodeValidator(&Config{}, p.metrics)
	p.nodeValidator = validator

	client := ts.Client()
	if err := p.fetchAuthorizedNodes(client); err != nil {
		t.Fatalf("fetchAuthorizedNodes failed: %v", err)
	}

	if len(p.config.AllowedNodes) != 1 || p.config.AllowedNodes[0] != "10.0.0.1" {
		t.Errorf("expected allowed nodes [10.0.0.1], got %v", p.config.AllowedNodes)
	}
}

func TestDropPrivileges_Root(t *testing.T) {
	// Mock osGeteuid to return 0 (root)
	origGeteuid := osGeteuid
	origSetgroups := unixSetgroups
	origSetgid := unixSetgid
	origSetuid := unixSetuid

	defer func() {
		osGeteuid = origGeteuid
		unixSetgroups = origSetgroups
		unixSetgid = origSetgid
		unixSetuid = origSetuid
	}()

	osGeteuid = func() int { return 0 }
	unixSetgroups = func(gids []int) error { return nil }
	unixSetgid = func(gid int) error { return nil }
	unixSetuid = func(uid int) error { return nil }

	mockSpec := &userSpec{
		name:   "testuser",
		uid:    1001,
		gid:    1001,
		groups: []int{1001},
		home:   "/home/testuser",
	}

	origResolve := resolveUserSpecFunc
	defer func() { resolveUserSpecFunc = origResolve }()
	resolveUserSpecFunc = func(username string) (*userSpec, error) {
		if username == "testuser" {
			return mockSpec, nil
		}
		return nil, errors.New("user not found")
	}

	// Test successful drop
	spec, err := dropPrivileges("testuser")
	if err != nil {
		t.Fatalf("dropPrivileges failed: %v", err)
	}
	if spec.uid != 1001 {
		t.Errorf("expected uid 1001, got %d", spec.uid)
	}

	// Test syscall failure
	unixSetuid = func(uid int) error { return errors.New("setuid failed") }
	_, err = dropPrivileges("testuser")
	if err == nil {
		t.Error("expected error for setuid failure")
	}
}

func TestLookupUserFromPasswd(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "passwd")
	content := "root:x:0:0:root:/root:/bin/bash\ntestuser:x:1001:1001:Test User:/home/testuser:/bin/sh\n"
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	origPath := passwdPath
	defer func() { passwdPath = origPath }()
	passwdPath = tmpFile

	// Test success
	spec, err := lookupUserFromPasswd("testuser")
	if err != nil {
		t.Fatalf("lookupUserFromPasswd failed: %v", err)
	}
	if spec.uid != 1001 || spec.gid != 1001 || spec.home != "/home/testuser" {
		t.Errorf("unexpected spec: %+v", spec)
	}

	// Test not found
	_, err = lookupUserFromPasswd("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent user")
	}

	// Test malformed line
	os.WriteFile(tmpFile, []byte("malformed\n"), 0644)
	_, err = lookupUserFromPasswd("testuser")
	if err == nil {
		t.Error("expected error/fail for malformed file")
	}
}

func TestStartControlPlaneSync(t *testing.T) {
	// Create a token file
	tokenFile := filepath.Join(t.TempDir(), "token")
	os.WriteFile(tokenFile, []byte("my-token"), 0600)

	p := &Proxy{
		controlPlaneCfg: &ControlPlaneConfig{
			URL:       "http://example.com",
			TokenFile: tokenFile,
		},
	}

	// We want to verify it starts the loop.
	// startControlPlaneSync calls `go p.controlPlaneLoop(ctx)`
	// We can't inspect the goroutine, but we can check if `controlPlaneCancel` is set.

	if p.controlPlaneCancel != nil {
		t.Error("expected cancel to be nil initially")
	}

	p.startControlPlaneSync()

	if p.controlPlaneCancel == nil {
		t.Error("expected cancel to be set after start")
	}
	// Clean up
	if p.controlPlaneCancel != nil {
		p.controlPlaneCancel()
	}
}

func TestResolveUserSpec_Fallback(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "passwd")
	// Use a user name that is unlikely to exist on the system to force fallback
	fallbackUser := "fallbackuser_9999"
	content := fmt.Sprintf("%s:x:2000:2000:Fallback User:/home/%s:/bin/sh\n", fallbackUser, fallbackUser)
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	origPath := passwdPath
	defer func() { passwdPath = origPath }()
	passwdPath = tmpFile

	spec, err := resolveUserSpec(fallbackUser)
	if err != nil {
		t.Fatalf("resolveUserSpec failed: %v", err)
	}
	if spec.name != fallbackUser || spec.uid != 2000 {
		t.Errorf("expected spec for %s (uid 2000), got %+v", fallbackUser, spec)
	}
}

func TestProxyStart(t *testing.T) {
	origListen := netListen
	origExec := execCommandFunc
	defer func() {
		netListen = origListen
		execCommandFunc = origExec
	}()

	// Mock successful listener
	mockListener := &mockListener{
		addr:   &net.UnixAddr{Name: "socket", Net: "unix"},
		closed: make(chan struct{}),
	}
	netListen = func(network, address string) (net.Listener, error) {
		return mockListener, nil
	}

	// Mock ssh-keygen success
	execCommandFunc = func(name string, arg ...string) *exec.Cmd {
		return mockExecCommand("")
	}

	tmpDir := t.TempDir()
	p := &Proxy{
		sshKeyPath: filepath.Join(tmpDir, "ssh"),
		socketPath: filepath.Join(tmpDir, "socket"),
		metrics:    NewProxyMetrics("test"),
	}

	if err := p.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if p.listener == nil {
		t.Error("expected listener to be set")
	}

	// Test Listen failure
	netListen = func(network, address string) (net.Listener, error) {
		return nil, errors.New("listen failed")
	}
	if err := p.Start(); err == nil {
		t.Error("expected error for listen failure")
	}
}

func TestSSHConnection(t *testing.T) {
	origExec := execCommandFunc
	defer func() { execCommandFunc = origExec }()

	// Mock ssh success
	execCommandFunc = func(name string, arg ...string) *exec.Cmd {
		args := strings.Join(arg, " ")
		if name == "sh" && strings.Contains(args, "ssh") {
			return mockExecCommand("")
		}
		return errorExecCommand("unexpected")
	}

	tmpDir := t.TempDir()
	km, _ := knownhosts.NewManager(filepath.Join(tmpDir, "known_hosts"), knownhosts.WithKeyscanFunc(func(ctx context.Context, host string, port int, timeout time.Duration) ([]byte, error) {
		return []byte("host ssh-ed25519 KEY"), nil
	}))

	p := &Proxy{
		sshKeyPath:        tmpDir,
		knownHosts:        km,
		metrics:           NewProxyMetrics("test"),
		maxSSHOutputBytes: 1024,
		config:            &Config{}, // Initialize config
	}

	// Ensure dummy key exists
	os.WriteFile(filepath.Join(tmpDir, "id_ed25519"), []byte("priv"), 0600)

	if err := p.testSSHConnection("host"); err != nil {
		t.Errorf("testSSHConnection failed: %v", err)
	}

	// Test failure
	execCommandFunc = func(name string, arg ...string) *exec.Cmd {
		args := strings.Join(arg, " ")
		if name == "sh" && strings.Contains(args, "ssh") {
			return errorExecCommand("ssh failed")
		}
		return mockExecCommand("")
	}

	if err := p.testSSHConnection("host"); err == nil {
		t.Error("expected error for ssh failure")
	}
}

type mockListener struct {
	addr   net.Addr
	closed chan struct{}
}

func (m *mockListener) Accept() (net.Conn, error) {
	if m.closed != nil {
		<-m.closed
	} else {
		select {} // Block forever if nil
	}
	return nil, errors.New("listener closed")
}
func (m *mockListener) Close() error {
	if m.closed != nil {
		select {
		case <-m.closed:
		default:
			close(m.closed)
		}
	}
	return nil
}
func (m *mockListener) Addr() net.Addr { return m.addr }

func TestEnsureHostKeyFromProxmox(t *testing.T) {
	origExec := execCommandFunc
	origPath := proxmoxClusterKnownHostsPath
	origLookPath := execLookPath
	defer func() {
		execCommandFunc = origExec
		proxmoxClusterKnownHostsPath = origPath
		execLookPath = origLookPath
	}()

	// Mock isProxmoxHost -> true
	execLookPath = func(file string) (string, error) {
		if file == "pvecm" {
			return "/usr/sbin/pvecm", nil
		}
		return "", fmt.Errorf("not found")
	}

	// Create a dummy known_hosts file simulating /etc/pve/priv/known_hosts
	tmpDir := t.TempDir()
	knownHostsFile := filepath.Join(tmpDir, "pve_known_hosts")
	// Format: host key
	if err := os.WriteFile(knownHostsFile, []byte("node1 ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIKqy\n"), 0644); err != nil {
		t.Fatal(err)
	}
	proxmoxClusterKnownHostsPath = knownHostsFile

	execCommandFunc = func(name string, arg ...string) *exec.Cmd {
		return mockExecCommand("ok")
	}

	proxy := &Proxy{
		knownHosts: &mockKnownHostsManager{},
		metrics:    NewProxyMetrics("test"),
	}

	// Test success
	if err := proxy.ensureHostKeyFromProxmox(context.Background(), "node1"); err != nil {
		t.Errorf("ensureHostKeyFromProxmox failed: %v", err)
	}

	// Test failure - not proxmox
	execCommandFunc = func(name string, arg ...string) *exec.Cmd {
		return errorExecCommand("fail")
	}
	if err := proxy.ensureHostKeyFromProxmox(context.Background(), "node1"); err == nil || err.Error() != "not running on Proxmox host" {
		t.Errorf("expected not running on Proxmox host error, got %v", err)
	}
}

type mockKnownHostsManager struct {
	ensureErr error
}

func (m *mockKnownHostsManager) Ensure(ctx context.Context, host string) error { return m.ensureErr }
func (m *mockKnownHostsManager) EnsureWithPort(ctx context.Context, host string, port int) error {
	return m.ensureErr
}
func (m *mockKnownHostsManager) EnsureWithEntries(ctx context.Context, host string, port int, entries [][]byte) error {
	return m.ensureErr
}
func (m *mockKnownHostsManager) Path() string { return "" }

func TestPushSSHKey(t *testing.T) {
	origExec := execCommandFunc
	defer func() { execCommandFunc = origExec }()

	tmpDir := t.TempDir()
	proxy := &Proxy{
		sshKeyPath: tmpDir,
		config: &Config{
			AllowedSourceSubnets: []string{"192.168.1.0/24"},
		},
		knownHosts: &mockKnownHostsManager{},
		metrics:    NewProxyMetrics("test"),
	}
	os.WriteFile(filepath.Join(tmpDir, "id_ed25519.pub"), []byte("pubkey"), 0644)

	// Mock successful copy
	execCommandFunc = func(name string, arg ...string) *exec.Cmd {
		// Mock ssh-copy-id or manual command
		return mockExecCommand("")
	}

	if err := proxy.pushSSHKeyFrom("remote-node", tmpDir); err != nil {
		t.Errorf("pushSSHKeyFrom failed: %v", err)
	}

	// Test failure
	execCommandFunc = func(name string, arg ...string) *exec.Cmd {
		return errorExecCommand("fail")
	}
	if err := proxy.pushSSHKeyFrom("remote-node", tmpDir); err == nil {
		t.Error("expected error for failure")
	}
}

func TestLoadProxmoxHostKeys(t *testing.T) {
	tmpDir := t.TempDir()
	knownHostsFile := filepath.Join(tmpDir, "pve_known_hosts")
	// node1 matches. node2 does not match requested host.
	content := "node1 ssh-ed25519 AAAKEY1\nnode2 ssh-ed25519 AAAKEY2\n# comment\n"
	if err := os.WriteFile(knownHostsFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	origPath := proxmoxClusterKnownHostsPath
	defer func() { proxmoxClusterKnownHostsPath = origPath }()
	proxmoxClusterKnownHostsPath = knownHostsFile

	entries, err := loadProxmoxHostKeys("node1")
	if err != nil {
		t.Fatalf("loadProxmoxHostKeys failed: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(entries))
	}

	// Test file open error
	proxmoxClusterKnownHostsPath = filepath.Join(tmpDir, "nonexistent")
	_, err = loadProxmoxHostKeys("node1")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestHandleHostKeyEnsureError(t *testing.T) {
	proxy := &Proxy{
		metrics: NewProxyMetrics("test"),
	}

	// Test generic error
	err := errors.New("generic error")
	if ret := proxy.handleHostKeyEnsureError("node1", err); ret != err {
		t.Errorf("expected same error, got %v", ret)
	}

	// Test HostKeyChangeError
	// We need to construct a HostKeyChangeError. It's from knownhosts package.
	// But knownhosts.HostKeyChangeError might be exported.
	// If it's not easy to construct, we might need a mock knownhosts manager to return it.
	// Let's assume we can mock the error type check or just ensure code path is covered if possible.
	// Without referencing the internal/ssh/knownhosts type directly if it's internal?
	// It is exported `github.com/rcourtman/pulse-go-rewrite/internal/ssh/knownhosts`.
	// Since I imported it as `knownhosts`, I can use it.

	changeErr := &knownhosts.HostKeyChangeError{
		Host:     "node1",
		Existing: "ssh-ed25519 AAA...",
		Provided: "ssh-ed25519 BBB...",
	}

	// This should log and record metric, then return the error.
	if ret := proxy.handleHostKeyEnsureError("node1", changeErr); ret != changeErr {
		t.Errorf("expected same error, got %v", ret)
	}
}

func TestDefaultExtractPeerCredentials(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Skipping peer creds test on non-linux")
	}

	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")

	l, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := l.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		creds, err := defaultExtractPeerCredentials(conn)
		if err != nil {
			t.Errorf("defaultExtractPeerCredentials failed: %v", err)
			return
		}
		if creds.uid != uint32(os.Geteuid()) {
			t.Errorf("expected uid %d, got %d", os.Geteuid(), creds.uid)
		}
		if creds.gid != uint32(os.Getgid()) {
			t.Errorf("expected gid %d, got %d", creds.gid, os.Getgid())
		}
		if creds.pid <= 0 {
			t.Errorf("expected value pid > 0, got %d", creds.pid)
		}
	}()

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	conn.Close()
	<-done
}
