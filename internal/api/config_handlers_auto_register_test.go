package api

import (
	"bytes"
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

	h := &ConfigHandlers{
		config:               cfg,
		persistence:          config.NewConfigPersistence(cfg.DataPath),
		setupCodes:           make(map[string]*SetupCode),
		recentSetupTokens:    make(map[string]time.Time),
		lastClusterDetection: make(map[string]time.Time),
		recentAutoRegistered: make(map[string]time.Time),
	}

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
			got := handler.disambiguateNodeName(tt.nodeName, tt.host, tt.nodeType)
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
