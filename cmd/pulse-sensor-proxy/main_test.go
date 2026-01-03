package main

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	if caps := peerCapabilitiesFromContext(nil); caps != 0 {
		t.Errorf("expected 0 caps for nil context, got %v", caps)
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
