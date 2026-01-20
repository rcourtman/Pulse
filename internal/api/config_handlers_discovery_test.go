package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
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
