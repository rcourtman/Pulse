package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestHandleSecureAutoRegister_PVE(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}
	handler := newTestConfigHandlers(t, cfg)

	server := newIPv4TLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	reqBody := AutoRegisterRequest{
		Type:         "pve",
		Host:         server.URL,
		ServerName:   "test-node",
		RequestToken: true,
		Username:     "root@pam",
		Password:     "secret",
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auto-register/secure", nil)
	rec := httptest.NewRecorder()

	handler.handleSecureAutoRegister(rec, req, &reqBody, "127.0.0.1")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["status"] != "success" {
		t.Fatalf("status = %v, want success", resp["status"])
	}
	if resp["tokenId"] == "" || resp["tokenValue"] == "" {
		t.Fatalf("expected token details in response")
	}
	if resp["action"] != "create_token" {
		t.Fatalf("action = %v, want create_token", resp["action"])
	}

	if len(handler.defaultConfig.PVEInstances) != 1 {
		t.Fatalf("expected 1 PVE instance, got %d", len(handler.defaultConfig.PVEInstances))
	}
	instance := handler.defaultConfig.PVEInstances[0]
	if !strings.Contains(instance.Host, "https://") {
		t.Fatalf("expected normalized host, got %q", instance.Host)
	}
	if instance.TokenName == "" || instance.TokenValue == "" {
		t.Fatalf("expected stored token values")
	}
}

func TestHandleSecureAutoRegisterConsolidatesStandaloneOverlapIntoClusterInMemory(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	server := newIPv4TLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
		PVEInstances: []config.PVEInstance{
			{
				Name:        "homelab",
				ClusterName: "cluster-A",
				IsCluster:   true,
				ClusterEndpoints: []config.ClusterEndpoint{
					{NodeName: "minipc", Host: server.URL},
				},
			},
		},
	}
	handler := newTestConfigHandlers(t, cfg)

	reqBody := AutoRegisterRequest{
		Type:         "pve",
		Host:         server.URL,
		ServerName:   "minipc",
		RequestToken: true,
		Username:     "root@pam",
		Password:     "secret",
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auto-register/secure", nil)
	rec := httptest.NewRecorder()

	handler.handleSecureAutoRegister(rec, req, &reqBody, "127.0.0.1")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if len(handler.defaultConfig.PVEInstances) != 1 {
		t.Fatalf("expected 1 PVE instance after consolidation, got %d", len(handler.defaultConfig.PVEInstances))
	}
	cluster := handler.defaultConfig.PVEInstances[0]
	if cluster.TokenName == "" || cluster.TokenValue == "" {
		t.Fatalf("expected token values to be promoted onto surviving cluster")
	}
}
