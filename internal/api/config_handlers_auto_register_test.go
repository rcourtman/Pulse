package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

func newTestConfigHandlers(t *testing.T, cfg *config.Config) *ConfigHandlers {
	t.Helper()

	if cfg == nil {
		cfg = &config.Config{}
	}
	if cfg.DataPath == "" {
		cfg.DataPath = t.TempDir()
	}
	h := NewConfigHandlers(nil, nil, func() error { return nil }, nil, nil, func() {})
	h.legacyConfig = cfg
	h.legacyPersistence = config.NewConfigPersistence(cfg.DataPath)

	return h
}

func TestHandleAutoRegisterRejectsWithoutAuth(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}

	handler := newTestConfigHandlers(t, cfg)

	reqBody := AutoRegisterRequest{
		Type:       "pve",
		Host:       "https://pve.local:8006",
		TokenID:    "pulse-monitor@pam!token",
		TokenValue: "secret-token",
		ServerName: "pve.local",
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.HandleAutoRegister(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleAutoRegisterAcceptsWithSetupToken(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}

	handler := newTestConfigHandlers(t, cfg)

	const tokenValue = "TEMP-TOKEN"
	tokenHash := internalauth.HashAPIToken(tokenValue)
	handler.codeMutex.Lock()
	handler.setupCodes[tokenHash] = &SetupCode{
		ExpiresAt: time.Now().Add(5 * time.Minute),
		NodeType:  "pve",
	}
	handler.codeMutex.Unlock()

	reqBody := AutoRegisterRequest{
		Type:       "pve",
		Host:       "https://pve.local:8006",
		TokenID:    "pulse-monitor@pam!token",
		TokenValue: "secret-token",
		ServerName: "pve.local",
		AuthToken:  tokenValue,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.HandleAutoRegister(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body=%s", rec.Code, rec.Body.String())
	}

	if len(cfg.PVEInstances) != 1 {
		t.Fatalf("expected 1 PVE instance stored, got %d", len(cfg.PVEInstances))
	}
}

// TestDisambiguateNodeName verifies that duplicate hostnames get disambiguated
// with their IP address appended. Issue #891.
func TestDisambiguateNodeName(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
		PVEInstances: []config.PVEInstance{
			{Name: "px1", Host: "https://10.0.1.100:8006"},
		},
	}

	handler := newTestConfigHandlers(t, cfg)

	tests := []struct {
		name     string
		nodeName string
		host     string
		nodeType string
		want     string
	}{
		{
			name:     "unique name unchanged",
			nodeName: "px2",
			host:     "https://10.0.2.200:8006",
			nodeType: "pve",
			want:     "px2", // No disambiguation needed
		},
		{
			name:     "duplicate name gets IP appended",
			nodeName: "px1",
			host:     "https://10.0.2.224:8006",
			nodeType: "pve",
			want:     "px1 (10.0.2.224)", // Disambiguated with IP
		},
		{
			name:     "same host same name is not duplicate",
			nodeName: "px1",
			host:     "https://10.0.1.100:8006", // Same host as existing
			nodeType: "pve",
			want:     "px1", // Same host = same node, no disambiguation
		},
		{
			name:     "empty name unchanged",
			nodeName: "",
			host:     "https://10.0.3.100:8006",
			nodeType: "pve",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := handler.disambiguateNodeName(context.Background(), tt.nodeName, tt.host, tt.nodeType)
			if got != tt.want {
				t.Errorf("disambiguateNodeName() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestAutoRegisterDuplicateHostnameSeparateNodes verifies that two Proxmox hosts
// with the same hostname but different IPs are stored as separate nodes.
// This is a regression test for Issue #891.
func TestAutoRegisterDuplicateHostnameSeparateNodes(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	// Mock detectPVECluster to avoid network calls
	originalDetectPVECluster := detectPVECluster
	detectPVECluster = func(clientConfig proxmox.ClientConfig, nodeName string, existingEndpoints []config.ClusterEndpoint) (bool, string, []config.ClusterEndpoint) {
		return false, "", nil
	}
	defer func() { detectPVECluster = originalDetectPVECluster }()

	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}

	handler := newTestConfigHandlers(t, cfg)

	// Create setup token
	const tokenValue = "TEMP-TOKEN"
	tokenHash := internalauth.HashAPIToken(tokenValue)

	// Register first node "px1" at 10.0.1.100
	handler.codeMutex.Lock()
	handler.setupCodes[tokenHash] = &SetupCode{
		ExpiresAt: time.Now().Add(5 * time.Minute),
		NodeType:  "pve",
	}
	handler.codeMutex.Unlock()

	reqBody1 := AutoRegisterRequest{
		Type:       "pve",
		Host:       "https://10.0.1.100:8006",
		TokenID:    "pulse-monitor@pam!token1",
		TokenValue: "secret-token-1",
		ServerName: "px1",
		AuthToken:  tokenValue,
	}

	body1, _ := json.Marshal(reqBody1)
	req1 := httptest.NewRequest(http.MethodPost, "/api/auto-register", bytes.NewReader(body1))
	rec1 := httptest.NewRecorder()
	handler.HandleAutoRegister(rec1, req1)

	if rec1.Code != http.StatusOK {
		t.Fatalf("first registration failed: status=%d, body=%s", rec1.Code, rec1.Body.String())
	}

	// Register second node "px1" at 10.0.2.224 (same hostname, different host)
	handler.codeMutex.Lock()
	handler.setupCodes[tokenHash] = &SetupCode{
		ExpiresAt: time.Now().Add(5 * time.Minute),
		NodeType:  "pve",
	}
	handler.codeMutex.Unlock()

	reqBody2 := AutoRegisterRequest{
		Type:       "pve",
		Host:       "https://10.0.2.224:8006",
		TokenID:    "pulse-monitor@pam!token2",
		TokenValue: "secret-token-2",
		ServerName: "px1", // Same hostname as first node
		AuthToken:  tokenValue,
	}

	body2, _ := json.Marshal(reqBody2)
	req2 := httptest.NewRequest(http.MethodPost, "/api/auto-register", bytes.NewReader(body2))
	rec2 := httptest.NewRecorder()
	handler.HandleAutoRegister(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("second registration failed: status=%d, body=%s", rec2.Code, rec2.Body.String())
	}

	// Verify we have TWO separate nodes (regression test for Issue #891)
	if len(cfg.PVEInstances) != 2 {
		t.Fatalf("expected 2 PVE instances, got %d (duplicate hostnames were incorrectly merged!)", len(cfg.PVEInstances))
	}

	// Verify the names are disambiguated
	node1 := cfg.PVEInstances[0]
	node2 := cfg.PVEInstances[1]

	if node1.Host == node2.Host {
		t.Error("both nodes have the same host - they should be different!")
	}

	// First node should keep original name, second should be disambiguated
	if node1.Name != "px1" {
		t.Errorf("first node name = %q, want %q", node1.Name, "px1")
	}

	if node2.Name != "px1 (10.0.2.224)" {
		t.Errorf("second node name = %q, want %q (should be disambiguated)", node2.Name, "px1 (10.0.2.224)")
	}
}

// TestExtractHostIP verifies the IP extraction from host URLs.
func TestExtractHostIP(t *testing.T) {
	tests := []struct {
		name     string
		hostURL  string
		expected string
	}{
		{
			name:     "IP-based URL",
			hostURL:  "https://192.168.1.100:8006",
			expected: "192.168.1.100",
		},
		{
			name:     "hostname URL returns empty",
			hostURL:  "https://pve.local:8006",
			expected: "",
		},
		{
			name:     "IPv6 URL",
			hostURL:  "https://[::1]:8006",
			expected: "::1",
		},
		{
			name:     "empty URL",
			hostURL:  "",
			expected: "",
		},
		{
			name:     "invalid URL",
			hostURL:  "not-a-url",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractHostIP(tt.hostURL)
			if got != tt.expected {
				t.Errorf("extractHostIP(%q) = %q, want %q", tt.hostURL, got, tt.expected)
			}
		})
	}
}

func TestIsPulseAgentToken(t *testing.T) {
	tests := []struct {
		token string
		want  bool
	}{
		{"pulse-monitor@pam!pulse-1234567890", true},
		{"pulse-monitor@pbs!pulse-1234567890", true},
		{"pulse-monitor@pam!pulse-", true},
		{"pulse-monitor@pam!token1", false},
		{"root@pam!mytoken", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := isPulseAgentToken(tt.token); got != tt.want {
			t.Errorf("isPulseAgentToken(%q) = %v, want %v", tt.token, got, tt.want)
		}
	}
}

// TestAutoRegisterAgentReRegistrationMerges verifies that an agent re-registering
// with a new Pulse token (e.g., after an update) merges with the existing entry
// instead of creating a duplicate. Regression test for Issue #1245.
func TestAutoRegisterAgentReRegistrationMerges(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	originalDetectPVECluster := detectPVECluster
	detectPVECluster = func(clientConfig proxmox.ClientConfig, nodeName string, existingEndpoints []config.ClusterEndpoint) (bool, string, []config.ClusterEndpoint) {
		return false, "", nil
	}
	defer func() { detectPVECluster = originalDetectPVECluster }()

	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
		// Pre-existing agent-registered node with an old Pulse token
		PVEInstances: []config.PVEInstance{
			{
				Name:       "pve",
				Host:       "https://10.0.1.100:8006",
				TokenName:  "pulse-monitor@pam!pulse-1700000000",
				TokenValue: "old-secret",
				Source:     "agent",
			},
		},
	}

	handler := newTestConfigHandlers(t, cfg)

	const tokenValue = "TEMP-TOKEN"
	tokenHash := internalauth.HashAPIToken(tokenValue)
	handler.codeMutex.Lock()
	handler.setupCodes[tokenHash] = &SetupCode{
		ExpiresAt: time.Now().Add(5 * time.Minute),
		NodeType:  "pve",
	}
	handler.codeMutex.Unlock()

	// Agent re-registers with a NEW Pulse token (different timestamp) and possibly different IP
	reqBody := AutoRegisterRequest{
		Type:       "pve",
		Host:       "https://10.0.1.200:8006", // IP may have changed
		TokenID:    "pulse-monitor@pam!pulse-1700099999",
		TokenValue: "new-secret",
		ServerName: "pve",
		Source:     "agent",
		AuthToken:  tokenValue,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleAutoRegister(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("re-registration failed: status=%d, body=%s", rec.Code, rec.Body.String())
	}

	// Should still be exactly 1 node (merged, not duplicated)
	if len(cfg.PVEInstances) != 1 {
		t.Fatalf("expected 1 PVE instance after re-registration, got %d (duplicate created!)", len(cfg.PVEInstances))
	}

	// Token should be updated to the new one
	node := cfg.PVEInstances[0]
	if node.TokenName != "pulse-monitor@pam!pulse-1700099999" {
		t.Errorf("token not updated: got %q, want %q", node.TokenName, "pulse-monitor@pam!pulse-1700099999")
	}
}

// TestAutoRegisterPreservesUserConfiguredHostname verifies that when an agent
// re-registers with the same token but sends a local IP, a user-configured
// hostname (public URL) is preserved instead of being overwritten.
// Regression test for Issue #1283.
func TestAutoRegisterPreservesUserConfiguredHostname(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	originalDetectPVECluster := detectPVECluster
	detectPVECluster = func(clientConfig proxmox.ClientConfig, nodeName string, existingEndpoints []config.ClusterEndpoint) (bool, string, []config.ClusterEndpoint) {
		return false, "", nil
	}
	defer func() { detectPVECluster = originalDetectPVECluster }()

	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
		// Agent originally registered with local IP, user later edited to public hostname
		PVEInstances: []config.PVEInstance{
			{
				Name:       "pve01",
				Host:       "https://pve.example.com:8006", // User-configured hostname
				TokenName:  "pulse-monitor@pam!pulse-1700000000",
				TokenValue: "old-secret",
				Source:     "agent",
			},
		},
	}

	handler := newTestConfigHandlers(t, cfg)

	const tokenValue = "TEMP-TOKEN"
	tokenHash := internalauth.HashAPIToken(tokenValue)
	handler.codeMutex.Lock()
	handler.setupCodes[tokenHash] = &SetupCode{
		ExpiresAt: time.Now().Add(5 * time.Minute),
		NodeType:  "pve",
	}
	handler.codeMutex.Unlock()

	// Agent re-registers with same token but sends local IP
	reqBody := AutoRegisterRequest{
		Type:       "pve",
		Host:       "https://192.168.1.100:8006", // Local IP from agent
		TokenID:    "pulse-monitor@pam!pulse-1700000000",
		TokenValue: "new-secret",
		ServerName: "pve01",
		Source:     "agent",
		AuthToken:  tokenValue,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleAutoRegister(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("re-registration failed: status=%d, body=%s", rec.Code, rec.Body.String())
	}

	// Should still be exactly 1 node
	if len(cfg.PVEInstances) != 1 {
		t.Fatalf("expected 1 PVE instance, got %d", len(cfg.PVEInstances))
	}

	node := cfg.PVEInstances[0]

	// Host should still be the user-configured hostname, NOT overwritten with local IP
	if node.Host != "https://pve.example.com:8006" {
		t.Errorf("user-configured hostname was overwritten: got %q, want %q",
			node.Host, "https://pve.example.com:8006")
	}

	// Token value should still be updated
	if node.TokenValue != "new-secret" {
		t.Errorf("token value not updated: got %q, want %q", node.TokenValue, "new-secret")
	}
}

// TestAutoRegisterPreservesUserConfiguredPublicIP verifies that when an agent
// re-registers with a local/private IP, a user-configured public IP is preserved.
// This is the exact scenario from #1283: Host URL was a public IP, agent sends
// its local 192.168.x.x IP, and the public IP gets overwritten.
func TestAutoRegisterPreservesUserConfiguredPublicIP(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	originalDetectPVECluster := detectPVECluster
	detectPVECluster = func(clientConfig proxmox.ClientConfig, nodeName string, existingEndpoints []config.ClusterEndpoint) (bool, string, []config.ClusterEndpoint) {
		return false, "", nil
	}
	defer func() { detectPVECluster = originalDetectPVECluster }()

	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
		PVEInstances: []config.PVEInstance{
			{
				Name:       "pve01",
				Host:       "https://203.0.113.52:8006", // User-configured public IP
				TokenName:  "pulse-monitor@pam!pulse-1700000000",
				TokenValue: "old-secret",
				Source:     "agent",
			},
		},
	}

	handler := newTestConfigHandlers(t, cfg)

	const tokenValue = "TEMP-TOKEN"
	tokenHash := internalauth.HashAPIToken(tokenValue)
	handler.codeMutex.Lock()
	handler.setupCodes[tokenHash] = &SetupCode{
		ExpiresAt: time.Now().Add(5 * time.Minute),
		NodeType:  "pve",
	}
	handler.codeMutex.Unlock()

	// Agent re-registers with same token but sends local/private IP
	reqBody := AutoRegisterRequest{
		Type:       "pve",
		Host:       "https://192.168.0.250:8006", // Local IP from agent
		TokenID:    "pulse-monitor@pam!pulse-1700000000",
		TokenValue: "new-secret",
		ServerName: "pve01",
		Source:     "agent",
		AuthToken:  tokenValue,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleAutoRegister(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("re-registration failed: status=%d, body=%s", rec.Code, rec.Body.String())
	}

	if len(cfg.PVEInstances) != 1 {
		t.Fatalf("expected 1 PVE instance, got %d", len(cfg.PVEInstances))
	}

	node := cfg.PVEInstances[0]

	// Public IP should be preserved, NOT overwritten with agent's local IP
	if node.Host != "https://203.0.113.52:8006" {
		t.Errorf("user-configured public IP was overwritten: got %q, want %q",
			node.Host, "https://203.0.113.52:8006")
	}

	if node.TokenValue != "new-secret" {
		t.Errorf("token value not updated: got %q, want %q", node.TokenValue, "new-secret")
	}
}

// TestAutoRegisterAgentDHCPPreservesHost verifies that when an agent re-registers
// with a different IP (DHCP), the existing host is preserved. The entry still
// merges (preventing duplicates), but the host is not overwritten since the user
// may have configured it. DHCP on servers is rare; the user can update manually.
func TestAutoRegisterAgentDHCPPreservesHost(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	originalDetectPVECluster := detectPVECluster
	detectPVECluster = func(clientConfig proxmox.ClientConfig, nodeName string, existingEndpoints []config.ClusterEndpoint) (bool, string, []config.ClusterEndpoint) {
		return false, "", nil
	}
	defer func() { detectPVECluster = originalDetectPVECluster }()

	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
		PVEInstances: []config.PVEInstance{
			{
				Name:       "pve01",
				Host:       "https://10.0.1.100:8006",
				TokenName:  "pulse-monitor@pam!pulse-1700000000",
				TokenValue: "secret",
				Source:     "agent",
			},
		},
	}

	handler := newTestConfigHandlers(t, cfg)

	const tokenValue = "TEMP-TOKEN"
	tokenHash := internalauth.HashAPIToken(tokenValue)
	handler.codeMutex.Lock()
	handler.setupCodes[tokenHash] = &SetupCode{
		ExpiresAt: time.Now().Add(5 * time.Minute),
		NodeType:  "pve",
	}
	handler.codeMutex.Unlock()

	reqBody := AutoRegisterRequest{
		Type:       "pve",
		Host:       "https://10.0.1.200:8006", // New IP from DHCP
		TokenID:    "pulse-monitor@pam!pulse-1700000000",
		TokenValue: "secret",
		ServerName: "pve01",
		Source:     "agent",
		AuthToken:  tokenValue,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleAutoRegister(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("re-registration failed: status=%d, body=%s", rec.Code, rec.Body.String())
	}

	// Still 1 entry (merged, not duplicated)
	if len(cfg.PVEInstances) != 1 {
		t.Fatalf("expected 1 PVE instance, got %d", len(cfg.PVEInstances))
	}

	node := cfg.PVEInstances[0]

	// Host should be preserved (agent source always preserves to protect user edits)
	if node.Host != "https://10.0.1.100:8006" {
		t.Errorf("host should be preserved for agent source: got %q, want %q",
			node.Host, "https://10.0.1.100:8006")
	}
}

// TestAutoRegisterNonAgentDHCPUpdatesHost verifies that DHCP IP changes from
// non-agent sources (e.g., script-based registration) still update the host.
func TestAutoRegisterNonAgentDHCPUpdatesHost(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	originalDetectPVECluster := detectPVECluster
	detectPVECluster = func(clientConfig proxmox.ClientConfig, nodeName string, existingEndpoints []config.ClusterEndpoint) (bool, string, []config.ClusterEndpoint) {
		return false, "", nil
	}
	defer func() { detectPVECluster = originalDetectPVECluster }()

	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
		PVEInstances: []config.PVEInstance{
			{
				Name:       "pve01",
				Host:       "https://10.0.1.100:8006",
				TokenName:  "pulse-monitor@pam!pulse-1700000000",
				TokenValue: "secret",
				Source:     "script",
			},
		},
	}

	handler := newTestConfigHandlers(t, cfg)

	const tokenValue = "TEMP-TOKEN"
	tokenHash := internalauth.HashAPIToken(tokenValue)
	handler.codeMutex.Lock()
	handler.setupCodes[tokenHash] = &SetupCode{
		ExpiresAt: time.Now().Add(5 * time.Minute),
		NodeType:  "pve",
	}
	handler.codeMutex.Unlock()

	reqBody := AutoRegisterRequest{
		Type:       "pve",
		Host:       "https://10.0.1.200:8006",
		TokenID:    "pulse-monitor@pam!pulse-1700000000",
		TokenValue: "secret",
		ServerName: "pve01",
		Source:     "script", // Non-agent source
		AuthToken:  tokenValue,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleAutoRegister(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("re-registration failed: status=%d, body=%s", rec.Code, rec.Body.String())
	}

	if len(cfg.PVEInstances) != 1 {
		t.Fatalf("expected 1 PVE instance, got %d", len(cfg.PVEInstances))
	}

	node := cfg.PVEInstances[0]

	// Host should be updated for non-agent DHCP
	if node.Host != "https://10.0.1.200:8006" {
		t.Errorf("non-agent DHCP host update failed: got %q, want %q",
			node.Host, "https://10.0.1.200:8006")
	}
}

// TestAutoRegisterPBSPreservesUserConfiguredHostname verifies the same
// hostname-preservation logic works for PBS instances. Regression test for #1283.
func TestAutoRegisterPBSPreservesUserConfiguredHostname(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
		PBSInstances: []config.PBSInstance{
			{
				Name:       "pbs01",
				Host:       "https://pbs.example.com:8007",
				TokenName:  "pulse-monitor@pbs!pulse-1700000000",
				TokenValue: "old-secret",
				Source:     "agent",
			},
		},
	}

	handler := newTestConfigHandlers(t, cfg)

	const tokenValue = "TEMP-TOKEN"
	tokenHash := internalauth.HashAPIToken(tokenValue)
	handler.codeMutex.Lock()
	handler.setupCodes[tokenHash] = &SetupCode{
		ExpiresAt: time.Now().Add(5 * time.Minute),
		NodeType:  "pbs",
	}
	handler.codeMutex.Unlock()

	reqBody := AutoRegisterRequest{
		Type:       "pbs",
		Host:       "https://192.168.1.50:8007",
		TokenID:    "pulse-monitor@pbs!pulse-1700000000",
		TokenValue: "new-secret",
		ServerName: "pbs01",
		Source:     "agent",
		AuthToken:  tokenValue,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleAutoRegister(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("re-registration failed: status=%d, body=%s", rec.Code, rec.Body.String())
	}

	if len(cfg.PBSInstances) != 1 {
		t.Fatalf("expected 1 PBS instance, got %d", len(cfg.PBSInstances))
	}

	node := cfg.PBSInstances[0]
	if node.Host != "https://pbs.example.com:8007" {
		t.Errorf("PBS user-configured hostname was overwritten: got %q, want %q",
			node.Host, "https://pbs.example.com:8007")
	}
	if node.TokenValue != "new-secret" {
		t.Errorf("PBS token value not updated: got %q, want %q", node.TokenValue, "new-secret")
	}
}

// TestAutoRegisterAgentDifferentHostsSameNameStaySeparate verifies that two
// genuinely different hosts with the same name but non-Pulse tokens (e.g.,
// manually created tokens) are NOT merged. Regression test for Issue #891.
func TestAutoRegisterAgentDifferentHostsSameNameStaySeparate(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	originalDetectPVECluster := detectPVECluster
	detectPVECluster = func(clientConfig proxmox.ClientConfig, nodeName string, existingEndpoints []config.ClusterEndpoint) (bool, string, []config.ClusterEndpoint) {
		return false, "", nil
	}
	defer func() { detectPVECluster = originalDetectPVECluster }()

	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
		// Existing node with a manually-created (non-Pulse) token
		PVEInstances: []config.PVEInstance{
			{
				Name:       "pve",
				Host:       "https://10.0.1.100:8006",
				TokenName:  "root@pam!my-manual-token",
				TokenValue: "manual-secret",
				Source:     "agent",
			},
		},
	}

	handler := newTestConfigHandlers(t, cfg)

	const tokenValue = "TEMP-TOKEN"
	tokenHash := internalauth.HashAPIToken(tokenValue)
	handler.codeMutex.Lock()
	handler.setupCodes[tokenHash] = &SetupCode{
		ExpiresAt: time.Now().Add(5 * time.Minute),
		NodeType:  "pve",
	}
	handler.codeMutex.Unlock()

	// Different physical host registers with same name but Pulse agent token
	reqBody := AutoRegisterRequest{
		Type:       "pve",
		Host:       "https://10.0.2.200:8006",
		TokenID:    "pulse-monitor@pam!pulse-1700099999",
		TokenValue: "new-secret",
		ServerName: "pve",
		Source:     "agent",
		AuthToken:  tokenValue,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleAutoRegister(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("registration failed: status=%d, body=%s", rec.Code, rec.Body.String())
	}

	// Should be 2 separate nodes (one has manual token, other has Pulse token)
	if len(cfg.PVEInstances) != 2 {
		t.Fatalf("expected 2 PVE instances (different hosts), got %d", len(cfg.PVEInstances))
	}
}
