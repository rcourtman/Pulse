package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	pkgdiscovery "github.com/rcourtman/pulse-go-rewrite/pkg/discovery"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

func TestHandleDiscoverServers_Manual(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	cfg := &config.Config{
		DataPath:         tempDir,
		ConfigPath:       tempDir,
		DiscoveryEnabled: true,
		Discovery:        config.DiscoveryConfig{
			// Fields like SubnetBlocklist etc. can optionally be set here if needed
		},
	}

	handler := newTestConfigHandlers(t, cfg)

	// Valid manual scan request
	reqBody := map[string]interface{}{
		"subnet":    "127.0.0.1/32",
		"use_cache": false,
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/discover", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.HandleDiscoverServers(rec, req)

	// Should return 200 OK even if nothing found
	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d. Body: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should contain standard fields
	if _, ok := resp["servers"]; !ok {
		t.Error("response missing 'servers' field")
	}
	if cached, ok := resp["cached"].(bool); !ok || cached {
		t.Error("expected cached=false")
	}
}

func TestWriteCachedDiscoveryResponse_UsesStructuredOwnerAndLegacyCompatibility(t *testing.T) {
	handler := newTestConfigHandlers(t, &config.Config{DataPath: t.TempDir(), ConfigPath: t.TempDir()})
	rec := httptest.NewRecorder()

	result := &pkgdiscovery.DiscoveryResult{
		Servers: []pkgdiscovery.DiscoveredServer{
			{IP: "10.0.0.1", Port: 8006, Type: "pve"},
		},
		StructuredErrors: []pkgdiscovery.DiscoveryError{
			{
				Phase:     "docker_bridge_network",
				ErrorType: "timeout",
				Message:   "request timed out",
				IP:        "10.0.0.2",
				Port:      8007,
				Timestamp: time.Unix(1700000000, 0).UTC(),
			},
		},
	}

	handler.writeCachedDiscoveryResponse(rec, result, time.Unix(1700000010, 0).UTC())

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp struct {
		Errors           []string                      `json:"errors"`
		StructuredErrors []pkgdiscovery.DiscoveryError `json:"structured_errors"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(resp.StructuredErrors) != 1 {
		t.Fatalf("structured_errors len = %d, want 1", len(resp.StructuredErrors))
	}
	if len(resp.Errors) != 1 {
		t.Fatalf("errors len = %d, want 1", len(resp.Errors))
	}
	if resp.Errors[0] != "Docker bridge network [10.0.0.2:8007]: request timed out" {
		t.Fatalf("legacy errors[0] = %q", resp.Errors[0])
	}
}

func TestValidateNodeAPI(t *testing.T) {
	// This function is internal (private), but accessible because we are in package api

	// Case 1: Trying to connect to a non-existent port should fail immediately
	// Use a port that is likely closed or invalid
	clientConfig := proxmox.ClientConfig{
		Host:      "https://127.0.0.1:54321", // Random port
		User:      "root@pam",
		Password:  "invalid",
		VerifySSL: false,
	}

	// Create dummy cluster status
	clusterNode := proxmox.ClusterStatus{
		Name:   "test-node",
		ID:     "node/test-node",
		Online: 1,
		IP:     "127.0.0.1",
	}

	isValid, fingerprint := validateNodeAPI(clusterNode, clientConfig)

	if isValid {
		t.Error("expected isValid=false for invalid connection, got true")
	}

	// Note: We cannot easily test success case without a real Proxmox server or complex mocking
	// of proxmox.NewClient which is a static function in pkg/proxmox.
	// However, we verify that the function handles failure gracefully.

	// Case 2: Invalid host url
	configInvalid := proxmox.ClientConfig{
		Host: "::invalid-url::",
	}
	isValid2, _ := validateNodeAPI(clusterNode, configInvalid)
	if isValid2 {
		t.Error("expected isValid=false for invalid url")
	}

	// Test Fingerprint capture logic if we could mock connection.
	if fingerprint != "" {
		// Should be empty on connection failure
		t.Error("expected empty fingerprint on failure")
	}
}
