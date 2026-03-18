package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestHandleCanonicalAutoRegister_PVE(t *testing.T) {
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
		Type:       "pve",
		Host:       server.URL,
		ServerName: "test-node",
		TokenID:    "pulse-monitor@pve!pulse-test-node",
		TokenValue: "created-locally",
		Source:     "script",
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", nil)
	rec := httptest.NewRecorder()

	handler.handleCanonicalAutoRegister(rec, req, &reqBody, "127.0.0.1")

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
	if resp["type"] != "pve" {
		t.Fatalf("type = %v, want pve", resp["type"])
	}
	if resp["source"] != "script" {
		t.Fatalf("source = %v, want script", resp["source"])
	}
	if resp["host"] != server.URL {
		t.Fatalf("host = %v, want normalized host %q", resp["host"], server.URL)
	}
	if resp["nodeId"] != "test-node" {
		t.Fatalf("nodeId = %v, want canonical node identity", resp["nodeId"])
	}
	if resp["nodeName"] != "test-node" {
		t.Fatalf("nodeName = %v, want canonical node identity", resp["nodeName"])
	}
	if resp["tokenId"] != "pulse-monitor@pve!pulse-test-node" {
		t.Fatalf("tokenId = %v, want caller token id", resp["tokenId"])
	}
	if resp["action"] != "use_token" {
		t.Fatalf("action = %v, want use_token", resp["action"])
	}
	if resp["message"] != "Node test-node registered successfully at "+server.URL {
		t.Fatalf("message = %v, want success guidance", resp["message"])
	}
	if resp["tokenValue"] != "created-locally" {
		t.Fatalf("tokenValue = %v, want caller token secret", resp["tokenValue"])
	}

	if len(handler.defaultConfig.PVEInstances) != 1 {
		t.Fatalf("expected 1 PVE instance, got %d", len(handler.defaultConfig.PVEInstances))
	}
	instance := handler.defaultConfig.PVEInstances[0]
	if !strings.Contains(instance.Host, "https://") {
		t.Fatalf("expected normalized host, got %q", instance.Host)
	}
	if instance.TokenName != "pulse-monitor@pve!pulse-test-node" {
		t.Fatalf("expected stored token id, got %q", instance.TokenName)
	}
	if instance.TokenValue != "created-locally" {
		t.Fatalf("expected stored token secret, got %q", instance.TokenValue)
	}
}

func TestHandleCanonicalAutoRegister_PVEAcceptsCallerProvidedTokenWithoutCredentials(t *testing.T) {
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
		Type:       "pve",
		Host:       server.URL,
		ServerName: "test-node",
		TokenID:    "pulse-monitor@pve!pulse-server",
		TokenValue: "created-locally",
		Source:     "agent",
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", nil)
	rec := httptest.NewRecorder()

	handler.handleCanonicalAutoRegister(rec, req, &reqBody, "127.0.0.1")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["action"] != "use_token" {
		t.Fatalf("action = %v, want use_token", resp["action"])
	}
	if resp["type"] != "pve" {
		t.Fatalf("type = %v, want pve", resp["type"])
	}
	if resp["source"] != "agent" {
		t.Fatalf("source = %v, want agent", resp["source"])
	}
	if resp["host"] != server.URL {
		t.Fatalf("host = %v, want normalized host %q", resp["host"], server.URL)
	}
	if resp["nodeId"] != "test-node" || resp["nodeName"] != "test-node" {
		t.Fatalf("node identity = (%v, %v), want test-node", resp["nodeId"], resp["nodeName"])
	}
	if resp["tokenId"] != "pulse-monitor@pve!pulse-server" {
		t.Fatalf("tokenId = %v, want caller-supplied token id", resp["tokenId"])
	}
	if resp["tokenValue"] != "created-locally" {
		t.Fatalf("tokenValue = %v, want caller-supplied token value", resp["tokenValue"])
	}

	if len(handler.defaultConfig.PVEInstances) != 1 {
		t.Fatalf("expected 1 PVE instance, got %d", len(handler.defaultConfig.PVEInstances))
	}
	instance := handler.defaultConfig.PVEInstances[0]
	if instance.TokenName != "pulse-monitor@pve!pulse-server" {
		t.Fatalf("token name = %q, want caller-supplied token id", instance.TokenName)
	}
	if instance.TokenValue != "created-locally" {
		t.Fatalf("token value = %q, want caller-supplied token value", instance.TokenValue)
	}
	if instance.Source != "agent" {
		t.Fatalf("source = %q, want agent", instance.Source)
	}
}

func TestHandleCanonicalAutoRegisterRejectsUnknownSource(t *testing.T) {
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
		Type:       "pve",
		Host:       server.URL,
		ServerName: "test-node",
		TokenID:    "pulse-monitor@pve!pulse-server",
		TokenValue: "created-locally",
		Source:     "manual",
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", nil)
	rec := httptest.NewRecorder()

	handler.handleCanonicalAutoRegister(rec, req, &reqBody, "127.0.0.1")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); body != "source must be 'agent' or 'script'\n" {
		t.Fatalf("body = %q, want canonical unknown-source guidance", body)
	}
	if len(handler.defaultConfig.PVEInstances) != 0 {
		t.Fatalf("expected no stored PVE instances, got %d", len(handler.defaultConfig.PVEInstances))
	}
}

func TestHandleCanonicalAutoRegisterRejectsMissingServerName(t *testing.T) {
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
		Type:       "pve",
		Host:       server.URL,
		TokenID:    "pulse-monitor@pve!pulse-server",
		TokenValue: "created-locally",
		Source:     "agent",
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", nil)
	rec := httptest.NewRecorder()

	handler.handleCanonicalAutoRegister(rec, req, &reqBody, "127.0.0.1")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); body != "Missing required canonical auto-register fields: serverName\n" {
		t.Fatalf("body = %q, want canonical missing-serverName guidance", body)
	}
	if len(handler.defaultConfig.PVEInstances) != 0 {
		t.Fatalf("expected no stored PVE instances, got %d", len(handler.defaultConfig.PVEInstances))
	}
}

func TestHandleCanonicalAutoRegisterRejectsMissingSource(t *testing.T) {
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
		Type:       "pve",
		Host:       server.URL,
		ServerName: "test-node",
		TokenID:    "pulse-monitor@pve!pulse-server",
		TokenValue: "created-locally",
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", nil)
	rec := httptest.NewRecorder()

	handler.handleCanonicalAutoRegister(rec, req, &reqBody, "127.0.0.1")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); body != "source is required\n" {
		t.Fatalf("body = %q, want canonical missing-source guidance", body)
	}
}

func TestHandleCanonicalAutoRegisterRejectsUnknownType(t *testing.T) {
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
		Type:       "pmg",
		Host:       server.URL,
		ServerName: "mail-gateway",
		TokenID:    "pulse-monitor@pmg!pulse-server",
		TokenValue: "created-locally",
		Source:     "script",
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", nil)
	rec := httptest.NewRecorder()

	handler.handleCanonicalAutoRegister(rec, req, &reqBody, "127.0.0.1")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); body != "type must be 'pve' or 'pbs'\n" {
		t.Fatalf("body = %q, want canonical unknown-type guidance", body)
	}
	if len(handler.defaultConfig.PVEInstances) != 0 || len(handler.defaultConfig.PBSInstances) != 0 {
		t.Fatalf("expected no stored nodes, got %d PVE and %d PBS", len(handler.defaultConfig.PVEInstances), len(handler.defaultConfig.PBSInstances))
	}
}

func TestHandleCanonicalAutoRegisterRejectsNonCanonicalTokenID(t *testing.T) {
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
		Type:       "pve",
		Host:       server.URL,
		ServerName: "test-node",
		TokenID:    "pulse@pve!token",
		TokenValue: "created-locally",
		Source:     "script",
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", nil)
	rec := httptest.NewRecorder()

	handler.handleCanonicalAutoRegister(rec, req, &reqBody, "127.0.0.1")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); body != "tokenId must be a canonical Pulse-managed token id\n" {
		t.Fatalf("body = %q, want canonical tokenId guidance", body)
	}
	if len(handler.defaultConfig.PVEInstances) != 0 {
		t.Fatalf("expected no stored PVE instances, got %d", len(handler.defaultConfig.PVEInstances))
	}
}

func TestHandleCanonicalAutoRegisterRejectsMismatchedCompletionPayload(t *testing.T) {
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
		Type:       "pve",
		Host:       server.URL,
		ServerName: "test-node",
		TokenID:    "pulse-monitor@pve!pulse-server",
		Source:     "script",
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", nil)
	rec := httptest.NewRecorder()

	handler.handleCanonicalAutoRegister(rec, req, &reqBody, "127.0.0.1")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); body != "tokenId and tokenValue must be provided together\n" {
		t.Fatalf("body = %q, want canonical completion-payload guidance", body)
	}
}

func TestHandleCanonicalAutoRegisterConsolidatesStandaloneOverlapIntoClusterInMemory(t *testing.T) {
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
		Type:       "pve",
		Host:       server.URL,
		ServerName: "minipc",
		TokenID:    "pulse-monitor@pve!pulse-cluster-node",
		TokenValue: "cluster-token",
		Source:     "script",
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", nil)
	rec := httptest.NewRecorder()

	handler.handleCanonicalAutoRegister(rec, req, &reqBody, "127.0.0.1")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if len(handler.defaultConfig.PVEInstances) != 1 {
		t.Fatalf("expected 1 PVE instance after consolidation, got %d", len(handler.defaultConfig.PVEInstances))
	}
	cluster := handler.defaultConfig.PVEInstances[0]
	if cluster.TokenName != "pulse-monitor@pve!pulse-cluster-node" {
		t.Fatalf("expected token id to be promoted onto surviving cluster, got %q", cluster.TokenName)
	}
	if cluster.TokenValue != "cluster-token" {
		t.Fatalf("expected token secret to be promoted onto surviving cluster, got %q", cluster.TokenValue)
	}
	if cluster.Source != "script" {
		t.Fatalf("source = %q, want script", cluster.Source)
	}
}

func TestHandleCanonicalAutoRegister_PVEPreservesExistingHostnameWhenRequestUsesSameIP(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	server := newIPv4TLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	hostnameURL := strings.Replace(server.URL, "127.0.0.1", "localhost", 1)
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
		PVEInstances: []config.PVEInstance{
			{
				Name:       "existing-node",
				Host:       hostnameURL,
				TokenName:  "pulse-monitor@pve!pulse-existing-node",
				TokenValue: "existing-token",
				Source:     "script",
			},
		},
	}
	handler := newTestConfigHandlers(t, cfg)

	reqBody := AutoRegisterRequest{
		Type:       "pve",
		Host:       server.URL,
		ServerName: "existing-node",
		TokenID:    "pulse-monitor@pve!pulse-existing-node",
		TokenValue: "existing-token",
		Source:     "script",
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", nil)
	rec := httptest.NewRecorder()

	handler.handleCanonicalAutoRegister(rec, req, &reqBody, "127.0.0.1")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["action"] != "use_token" {
		t.Fatalf("action = %v, want use_token for reused canonical /api/auto-register token", resp["action"])
	}

	if len(handler.defaultConfig.PVEInstances) != 1 {
		t.Fatalf("expected 1 PVE instance after resolved-host dedupe, got %d", len(handler.defaultConfig.PVEInstances))
	}

	instance := handler.defaultConfig.PVEInstances[0]
	if instance.Host != hostnameURL {
		t.Fatalf("host = %q, want existing hostname-preserving host %q", instance.Host, hostnameURL)
	}
	if instance.TokenName != "pulse-monitor@pve!pulse-existing-node" {
		t.Fatalf("token name = %q, want reused Pulse-managed token", instance.TokenName)
	}
	if instance.TokenValue != "existing-token" {
		t.Fatalf("token value = %q, want reused Pulse-managed token", instance.TokenValue)
	}
	if instance.Source != "script" {
		t.Fatalf("source = %q, want script", instance.Source)
	}
}

func TestHandleCanonicalAutoRegisterReturnsCanonicalStoredNodeIdentity(t *testing.T) {
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
				Name:       "test-node",
				Host:       "https://existing.local:8006",
				TokenName:  "pulse-monitor@pve!pulse-existing-node",
				TokenValue: "existing-token",
				Source:     "script",
			},
		},
	}
	handler := newTestConfigHandlers(t, cfg)

	reqBody := AutoRegisterRequest{
		Type:       "pve",
		Host:       server.URL,
		ServerName: "test-node",
		TokenID:    "pulse-monitor@pve!pulse-new-node",
		TokenValue: "new-token",
		Source:     "script",
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", nil)
	rec := httptest.NewRecorder()

	handler.handleCanonicalAutoRegister(rec, req, &reqBody, "127.0.0.1")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(handler.defaultConfig.PVEInstances) != 2 {
		t.Fatalf("expected 2 PVE instances, got %d", len(handler.defaultConfig.PVEInstances))
	}
	storedName := handler.defaultConfig.PVEInstances[1].Name
	if storedName == "test-node" {
		t.Fatalf("stored name = %q, want disambiguated canonical node name", storedName)
	}
	if resp["nodeId"] != storedName {
		t.Fatalf("nodeId = %v, want canonical stored node identity %q", resp["nodeId"], storedName)
	}
}

func TestBuildAutoRegisterEventDataUsesCanonicalAutoRegisterTokenIdentity(t *testing.T) {
	req := &AutoRegisterRequest{
		Type:       "pbs",
		ServerName: "backup-node",
	}

	event := buildAutoRegisterEventData(req, "https://pbs.local:8007", "backup-node (2)", "pulse-monitor@pbs!pulse-backup")

	if got := event["name"]; got != "backup-node (2)" {
		t.Fatalf("name = %#v, want canonical stored node identity", got)
	}
	if got := event["nodeId"]; got != "backup-node (2)" {
		t.Fatalf("nodeId = %#v, want canonical stored node identity", got)
	}
	if got := event["tokenId"]; got != "pulse-monitor@pbs!pulse-backup" {
		t.Fatalf("tokenId = %#v, want canonical token id", got)
	}
}

func TestHandleCanonicalAutoRegister_PVERotatesStoredPulseTokenOnCallerReplacement(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	server := newIPv4TLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	hostnameURL := strings.Replace(server.URL, "127.0.0.1", "localhost", 1)
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
		PVEInstances: []config.PVEInstance{
			{
				Name:       "existing-node",
				Host:       hostnameURL,
				TokenName:  "pulse-monitor@pve!pulse-existing-node",
				TokenValue: "existing-token",
				Source:     "script",
			},
		},
	}
	handler := newTestConfigHandlers(t, cfg)

	reqBody := AutoRegisterRequest{
		Type:       "pve",
		Host:       server.URL,
		ServerName: "existing-node",
		TokenID:    "pulse-monitor@pve!pulse-existing-node",
		TokenValue: "rotated-token",
		Source:     "script",
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", nil)
	rec := httptest.NewRecorder()

	handler.handleCanonicalAutoRegister(rec, req, &reqBody, "127.0.0.1")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["action"] != "use_token" {
		t.Fatalf("action = %v, want use_token for canonical /api/auto-register token replacement", resp["action"])
	}
	if resp["message"] != "Node existing-node registered successfully at "+server.URL {
		t.Fatalf("message = %v, want success guidance", resp["message"])
	}
	if resp["tokenValue"] != "rotated-token" {
		t.Fatalf("tokenValue = %v, want rotated token secret", resp["tokenValue"])
	}

	if len(handler.defaultConfig.PVEInstances) != 1 {
		t.Fatalf("expected 1 PVE instance after resolved-host dedupe, got %d", len(handler.defaultConfig.PVEInstances))
	}

	instance := handler.defaultConfig.PVEInstances[0]
	if instance.Host != hostnameURL {
		t.Fatalf("host = %q, want existing hostname-preserving host %q", instance.Host, hostnameURL)
	}
	if instance.TokenValue != "rotated-token" {
		t.Fatalf("token value = %q, want rotated token secret", instance.TokenValue)
	}
	if instance.Source != "script" {
		t.Fatalf("source = %q, want script after caller-supplied token replacement", instance.Source)
	}
	if instance.TokenValue == "existing-token" {
		t.Fatalf("stored token value = %q, want caller replacement to rotate stale token secret", instance.TokenValue)
	}
}

func TestHandleCanonicalAutoRegister_PVEUpdatesExistingNodeOnDHCPHostChange(t *testing.T) {
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
				Name:       "existing-node",
				Host:       "https://10.0.0.10:8006",
				TokenName:  buildPulseMonitorTokenName("pulse.example.com"),
				TokenValue: "existing-token",
				Source:     "script",
			},
		},
	}
	cfg.PVEInstances[0].TokenName = "pulse-monitor@pve!" + cfg.PVEInstances[0].TokenName
	handler := newTestConfigHandlers(t, cfg)

	reqBody := AutoRegisterRequest{
		Type:       "pve",
		Host:       server.URL,
		ServerName: "existing-node",
		TokenID:    "pulse-monitor@pve!" + buildPulseMonitorTokenName("pulse.example.com"),
		TokenValue: "existing-token",
		Source:     "script",
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", nil)
	req.Host = "pulse.example.com"
	rec := httptest.NewRecorder()

	handler.handleCanonicalAutoRegister(rec, req, &reqBody, "127.0.0.1")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["action"] != "use_token" {
		t.Fatalf("action = %v, want use_token for DHCP continuity rerun", resp["action"])
	}

	if len(handler.defaultConfig.PVEInstances) != 1 {
		t.Fatalf("expected 1 PVE instance after DHCP continuity dedupe, got %d", len(handler.defaultConfig.PVEInstances))
	}

	instance := handler.defaultConfig.PVEInstances[0]
	if instance.Host != server.URL {
		t.Fatalf("host = %q, want updated host %q", instance.Host, server.URL)
	}
	if instance.TokenName != "pulse-monitor@pve!"+buildPulseMonitorTokenName("pulse.example.com") {
		t.Fatalf("token name = %q, want reused Pulse-managed token", instance.TokenName)
	}
	if instance.TokenValue != "existing-token" {
		t.Fatalf("token value = %q, want reused Pulse-managed token", instance.TokenValue)
	}
	if instance.Source != "script" {
		t.Fatalf("source = %q, want script", instance.Source)
	}
}

func TestHandleCanonicalAutoRegister_PBSPreservesExistingHostnameWhenRequestUsesSameIP(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	server := newIPv4TLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	hostnameURL := strings.Replace(server.URL, "127.0.0.1", "localhost", 1)
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
		PBSInstances: []config.PBSInstance{
			{
				Name:       "existing-backup",
				Host:       hostnameURL,
				TokenName:  "pulse-monitor@pbs!pulse-existing-backup",
				TokenValue: "existing-token",
			},
		},
	}
	handler := newTestConfigHandlers(t, cfg)

	reqBody := AutoRegisterRequest{
		Type:       "pbs",
		Host:       server.URL,
		ServerName: "existing-backup",
		TokenID:    "pulse-monitor@pbs!pulse-existing-backup",
		TokenValue: "existing-token",
		Source:     "script",
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", nil)
	rec := httptest.NewRecorder()

	handler.handleCanonicalAutoRegister(rec, req, &reqBody, "127.0.0.1")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if len(handler.defaultConfig.PBSInstances) != 1 {
		t.Fatalf("expected 1 PBS instance after resolved-host dedupe, got %d", len(handler.defaultConfig.PBSInstances))
	}

	instance := handler.defaultConfig.PBSInstances[0]
	if instance.Host != hostnameURL {
		t.Fatalf("host = %q, want existing hostname-preserving host %q", instance.Host, hostnameURL)
	}
	if instance.TokenName != "pulse-monitor@pbs!pulse-existing-backup" {
		t.Fatalf("token name = %q, want reused Pulse-managed token", instance.TokenName)
	}
	if instance.TokenValue != "existing-token" {
		t.Fatalf("token value = %q, want reused Pulse-managed token", instance.TokenValue)
	}
}

func TestHandleCanonicalAutoRegister_PBSUpdatesExistingNodeOnDHCPHostChange(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	server := newIPv4TLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
		PBSInstances: []config.PBSInstance{
			{
				Name:       "existing-backup",
				Host:       "https://10.0.0.20:8007",
				TokenName:  "pulse-monitor@pbs!" + buildPulseMonitorTokenName("pulse.example.com"),
				TokenValue: "existing-token",
			},
		},
	}
	handler := newTestConfigHandlers(t, cfg)

	reqBody := AutoRegisterRequest{
		Type:       "pbs",
		Host:       server.URL,
		ServerName: "existing-backup",
		TokenID:    "pulse-monitor@pbs!" + buildPulseMonitorTokenName("pulse.example.com"),
		TokenValue: "existing-token",
		Source:     "script",
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", nil)
	req.Host = "pulse.example.com"
	rec := httptest.NewRecorder()

	handler.handleCanonicalAutoRegister(rec, req, &reqBody, "127.0.0.1")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["action"] != "use_token" {
		t.Fatalf("action = %v, want use_token for DHCP continuity rerun", resp["action"])
	}

	if len(handler.defaultConfig.PBSInstances) != 1 {
		t.Fatalf("expected 1 PBS instance after DHCP continuity dedupe, got %d", len(handler.defaultConfig.PBSInstances))
	}

	instance := handler.defaultConfig.PBSInstances[0]
	if instance.Host != server.URL {
		t.Fatalf("host = %q, want updated host %q", instance.Host, server.URL)
	}
	if instance.TokenName != "pulse-monitor@pbs!"+buildPulseMonitorTokenName("pulse.example.com") {
		t.Fatalf("token name = %q, want reused Pulse-managed token", instance.TokenName)
	}
	if instance.TokenValue != "existing-token" {
		t.Fatalf("token value = %q, want reused Pulse-managed token", instance.TokenValue)
	}
}
