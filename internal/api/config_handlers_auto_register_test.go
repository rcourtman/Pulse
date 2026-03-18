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
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
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
	h.defaultConfig = cfg
	h.defaultPersistence = config.NewConfigPersistence(cfg.DataPath)

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
		TokenID:    "pulse-monitor@pve!pulse-pve-local",
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
	if body := rec.Body.String(); body != "Pulse setup token required\n" {
		t.Fatalf("body = %q, want missing-auth guidance", body)
	}
}

func TestHandleAutoRegisterRejectsInvalidSetupToken(t *testing.T) {
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
		TokenID:    "pulse-monitor@pve!pulse-pve-local",
		TokenValue: "secret-token",
		ServerName: "pve.local",
		AuthToken:  "invalid-setup-token",
		Source:     "script",
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
	if body := rec.Body.String(); body != "Invalid or expired setup token\n" {
		t.Fatalf("body = %q, want invalid-setup-token guidance", body)
	}
}

func TestHandleAutoRegisterAcceptsWithSetupToken(t *testing.T) {
	stubAutoRegisterNetworkDeps(t)

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
	handler.setupTokens[tokenHash] = &SetupTokenRecord{
		ExpiresAt: time.Now().Add(5 * time.Minute),
		NodeType:  "pve",
	}
	handler.codeMutex.Unlock()

	reqBody := AutoRegisterRequest{
		Type:       "pve",
		Host:       "https://pve.local:8006",
		TokenID:    "pulse-monitor@pve!pulse-pve-local",
		TokenValue: "secret-token",
		ServerName: "pve.local",
		AuthToken:  tokenValue,
		Source:     "script",
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
	if cfg.PVEInstances[0].Source != "script" {
		t.Fatalf("source = %q, want script", cfg.PVEInstances[0].Source)
	}
}

func TestHandleAutoRegisterRejectsMissingSource(t *testing.T) {
	stubAutoRegisterNetworkDeps(t)

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
	handler.setupTokens[tokenHash] = &SetupTokenRecord{
		ExpiresAt: time.Now().Add(5 * time.Minute),
		NodeType:  "pve",
	}
	handler.codeMutex.Unlock()

	reqBody := AutoRegisterRequest{
		Type:       "pve",
		Host:       "https://pve.local:8006",
		TokenID:    "pulse-monitor@pve!pulse-pve-local",
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

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d, body=%s", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); body != "source is required\n" {
		t.Fatalf("body = %q, want canonical missing-source guidance", body)
	}
	if len(cfg.PVEInstances) != 0 {
		t.Fatalf("expected missing-source request to persist nothing, got %d nodes", len(cfg.PVEInstances))
	}
}

func TestHandleAutoRegisterRejectsMissingServerName(t *testing.T) {
	stubAutoRegisterNetworkDeps(t)

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
	handler.setupTokens[tokenHash] = &SetupTokenRecord{
		ExpiresAt: time.Now().Add(5 * time.Minute),
		NodeType:  "pve",
	}
	handler.codeMutex.Unlock()

	reqBody := AutoRegisterRequest{
		Type:       "pve",
		Host:       "https://pve.local:8006",
		TokenID:    "pulse-monitor@pve!pulse-pve-local",
		TokenValue: "secret-token",
		AuthToken:  tokenValue,
		Source:     "script",
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.HandleAutoRegister(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d, body=%s", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); body != "Missing required canonical auto-register fields: serverName\n" {
		t.Fatalf("body = %q, want canonical missing-serverName guidance", body)
	}
	if len(cfg.PVEInstances) != 0 {
		t.Fatalf("expected missing-serverName request to persist nothing, got %d nodes", len(cfg.PVEInstances))
	}
}

func TestHandleAutoRegisterRejectsMismatchedCompletionPayload(t *testing.T) {
	stubAutoRegisterNetworkDeps(t)

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
	handler.setupTokens[tokenHash] = &SetupTokenRecord{
		ExpiresAt: time.Now().Add(5 * time.Minute),
		NodeType:  "pve",
	}
	handler.codeMutex.Unlock()

	reqBody := AutoRegisterRequest{
		Type:       "pve",
		Host:       "https://pve.local:8006",
		TokenID:    "pulse-monitor@pve!pulse-pve-local",
		ServerName: "pve.local",
		AuthToken:  tokenValue,
		Source:     "script",
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.HandleAutoRegister(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d, body=%s", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); body != "tokenId and tokenValue must be provided together\n" {
		t.Fatalf("body = %q, want canonical completion-payload guidance", body)
	}
}

func TestHandleAutoRegisterRejectsUnknownSource(t *testing.T) {
	stubAutoRegisterNetworkDeps(t)

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
	handler.setupTokens[tokenHash] = &SetupTokenRecord{
		ExpiresAt: time.Now().Add(5 * time.Minute),
		NodeType:  "pve",
	}
	handler.codeMutex.Unlock()

	reqBody := AutoRegisterRequest{
		Type:       "pve",
		Host:       "https://pve.local:8006",
		TokenID:    "pulse-monitor@pve!token",
		TokenValue: "secret-token",
		ServerName: "pve.local",
		AuthToken:  tokenValue,
		Source:     "manual",
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.HandleAutoRegister(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d, body=%s", rec.Code, rec.Body.String())
	}
	if len(cfg.PVEInstances) != 0 {
		t.Fatalf("expected unknown-source request to persist nothing, got %d nodes", len(cfg.PVEInstances))
	}
}

func TestHandleAutoRegisterRejectsUnknownType(t *testing.T) {
	stubAutoRegisterNetworkDeps(t)

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
	handler.setupTokens[tokenHash] = &SetupTokenRecord{
		ExpiresAt: time.Now().Add(5 * time.Minute),
		NodeType:  "pmg",
	}
	handler.codeMutex.Unlock()

	reqBody := AutoRegisterRequest{
		Type:       "pmg",
		Host:       "https://pmg.local:8006",
		TokenID:    "pulse-monitor@pmg!token",
		TokenValue: "secret-token",
		ServerName: "pmg.local",
		AuthToken:  tokenValue,
		Source:     "script",
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.HandleAutoRegister(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d, body=%s", rec.Code, rec.Body.String())
	}
	if len(cfg.PVEInstances) != 0 || len(cfg.PBSInstances) != 0 {
		t.Fatalf("expected unknown-type request to persist nothing, got %d PVE and %d PBS nodes", len(cfg.PVEInstances), len(cfg.PBSInstances))
	}
}

func TestHandleAutoRegisterAcceptsSecureCallerProvidedTokenWithoutCredentials(t *testing.T) {
	stubAutoRegisterNetworkDeps(t)

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
	handler.setupTokens[tokenHash] = &SetupTokenRecord{
		ExpiresAt: time.Now().Add(5 * time.Minute),
		NodeType:  "pve",
	}
	handler.codeMutex.Unlock()

	reqBody := AutoRegisterRequest{
		Type:       "pve",
		Host:       "https://pve.local:8006",
		TokenID:    "pulse-monitor@pve!pulse-server",
		TokenValue: "created-locally",
		ServerName: "pve.local",
		AuthToken:  tokenValue,
		Source:     "agent",
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
	if cfg.PVEInstances[0].TokenName != "pulse-monitor@pve!pulse-server" {
		t.Fatalf("tokenName = %q, want caller-provided token id", cfg.PVEInstances[0].TokenName)
	}
	if cfg.PVEInstances[0].TokenValue != "created-locally" {
		t.Fatalf("tokenValue = %q, want caller-provided token value", cfg.PVEInstances[0].TokenValue)
	}
	if cfg.PVEInstances[0].Source != "agent" {
		t.Fatalf("source = %q, want agent", cfg.PVEInstances[0].Source)
	}
}

func TestHandleAutoRegisterReturnsCanonicalNodeIdentity(t *testing.T) {
	stubAutoRegisterNetworkDeps(t)

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
	handler.setupTokens[tokenHash] = &SetupTokenRecord{
		ExpiresAt: time.Now().Add(5 * time.Minute),
		NodeType:  "pve",
	}
	handler.codeMutex.Unlock()

	reqBody := AutoRegisterRequest{
		Type:       "pve",
		Host:       "https://192.0.2.10:8006",
		TokenID:    "pulse-monitor@pve!pulse-node-1",
		TokenValue: "secret-token",
		ServerName: "pve-node-1",
		AuthToken:  tokenValue,
		Source:     "script",
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

	var response struct {
		Status     string `json:"status"`
		Message    string `json:"message"`
		Type       string `json:"type"`
		Source     string `json:"source"`
		Host       string `json:"host"`
		NodeID     string `json:"nodeId"`
		NodeName   string `json:"nodeName"`
		TokenID    string `json:"tokenId"`
		TokenValue string `json:"tokenValue"`
		Action     string `json:"action"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if response.Type != "pve" {
		t.Fatalf("type = %q, want pve", response.Type)
	}
	if response.Source != "script" {
		t.Fatalf("source = %q, want script", response.Source)
	}
	if response.Host != "https://192.0.2.10:8006" {
		t.Fatalf("host = %q, want normalized host", response.Host)
	}
	if response.NodeID != "pve-node-1" {
		t.Fatalf("nodeId = %q, want canonical node identity", response.NodeID)
	}
	if response.NodeName != "pve-node-1" {
		t.Fatalf("nodeName = %q, want canonical node identity", response.NodeName)
	}
	if response.Message != "Node pve-node-1 registered successfully at https://192.0.2.10:8006" {
		t.Fatalf("message = %q", response.Message)
	}
	if response.Action != "use_token" {
		t.Fatalf("action = %q, want use_token", response.Action)
	}
	if response.TokenID != "pulse-monitor@pve!pulse-node-1" {
		t.Fatalf("tokenId = %q", response.TokenID)
	}
	if response.TokenValue != "secret-token" {
		t.Fatalf("tokenValue = %q", response.TokenValue)
	}
}

func TestBuildAutoRegisterEventDataUsesCanonicalIdentity(t *testing.T) {
	req := &AutoRegisterRequest{
		Type:       "pve",
		Host:       "https://192.0.2.10:8006",
		TokenID:    "pulse-monitor@pve!pulse-node-1",
		ServerName: "pve-node-1",
	}

	event := buildAutoRegisterEventData(req, "https://pve.local:8006", "pve-node-1 (2)", "pulse-monitor@pve!pulse-node-1")

	if got := event["host"]; got != "https://pve.local:8006" {
		t.Fatalf("host = %#v, want normalized stored host", got)
	}
	if got := event["name"]; got != "pve-node-1 (2)" {
		t.Fatalf("name = %#v, want canonical stored node name", got)
	}
	if got := event["nodeId"]; got != "pve-node-1 (2)" {
		t.Fatalf("nodeId = %#v, want canonical stored node identity", got)
	}
	if got := event["nodeName"]; got != "pve-node-1 (2)" {
		t.Fatalf("nodeName = %#v, want canonical stored node identity", got)
	}
	if got := event["tokenId"]; got != "pulse-monitor@pve!pulse-node-1" {
		t.Fatalf("tokenId = %#v, want explicit token id", got)
	}
	if got := event["verifySSL"]; got != true {
		t.Fatalf("verifySSL = %#v, want true", got)
	}
}

func TestHandleAutoRegisterPreservesExplicitAgentSource(t *testing.T) {
	stubAutoRegisterNetworkDeps(t)

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
	handler.setupTokens[tokenHash] = &SetupTokenRecord{
		ExpiresAt: time.Now().Add(5 * time.Minute),
		NodeType:  "pve",
	}
	handler.codeMutex.Unlock()

	reqBody := AutoRegisterRequest{
		Type:       "pve",
		Host:       "https://pve.local:8006",
		TokenID:    "pulse-monitor@pve!pulse-pve-local",
		TokenValue: "secret-token",
		ServerName: "pve.local",
		AuthToken:  tokenValue,
		Source:     "agent",
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
	if cfg.PVEInstances[0].Source != "agent" {
		t.Fatalf("source = %q, want explicit agent source preserved", cfg.PVEInstances[0].Source)
	}
}

func TestHandleAutoRegister_BlocksNewCountedSystemAtLimit(t *testing.T) {
	stubAutoRegisterNetworkDeps(t)

	setMaxMonitoredSystemsLicenseForTests(t, 1)

	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
		PVEInstances: []config.PVEInstance{
			{
				Name:   "existing-node",
				Host:   "https://pve-existing.local:8006",
				Source: "manual",
			},
		},
	}
	handler := newTestConfigHandlers(t, cfg)
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestRecords(unifiedresources.SourceAgent, []unifiedresources.IngestRecord{
		{
			SourceID: "host-1",
			Resource: unifiedresources.Resource{
				ID:     "host-1",
				Type:   unifiedresources.ResourceTypeAgent,
				Name:   "existing-node",
				Status: unifiedresources.StatusOnline,
				Agent: &unifiedresources.AgentData{
					AgentID:   "agent-1",
					Hostname:  "existing-node",
					MachineID: "machine-1",
				},
				Identity: unifiedresources.ResourceIdentity{
					MachineID: "machine-1",
					Hostnames: []string{"existing-node"},
				},
			},
		},
	})
	monitor := &monitoring.Monitor{}
	monitor.SetResourceStore(unifiedresources.NewMonitorAdapter(registry))
	handler.defaultMonitor = monitor

	const tokenValue = "LIMIT-TOKEN"
	tokenHash := internalauth.HashAPIToken(tokenValue)
	handler.codeMutex.Lock()
	handler.setupTokens[tokenHash] = &SetupTokenRecord{
		ExpiresAt: time.Now().Add(5 * time.Minute),
		NodeType:  "pve",
	}
	handler.codeMutex.Unlock()

	reqBody := AutoRegisterRequest{
		Type:       "pve",
		Host:       "https://pve-new.local:8006",
		TokenID:    "pulse-monitor@pve!pulse-pve-local",
		TokenValue: "secret-token",
		ServerName: "pve-new.local",
		AuthToken:  tokenValue,
		Source:     "script",
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleAutoRegister(rec, req)

	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("expected 402 once monitored-system cap is full, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleAutoRegisterRejectsRemovedBodyTokenContract(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	record, err := config.NewAPITokenRecord("org-bound-body-token-123.12345678", "org-bound", []string{config.ScopeAgentReport})
	if err != nil {
		t.Fatalf("new api token record: %v", err)
	}
	record.OrgID = "org-a"

	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
		APITokens:  []config.APITokenRecord{*record},
	}

	handler := newTestConfigHandlers(t, cfg)

	reqBody := AutoRegisterRequest{
		Type:       "pve",
		Host:       "https://pve.local:8006",
		TokenID:    "pulse-monitor@pve!pulse-pve-local",
		TokenValue: "secret-token",
		ServerName: "pve.local",
		AuthToken:  "org-bound-body-token-123.12345678",
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", bytes.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, "org-b"))
	rec := httptest.NewRecorder()

	handler.HandleAutoRegister(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d, body=%s", rec.Code, rec.Body.String())
	}
	if len(cfg.PVEInstances) != 0 {
		t.Fatalf("expected no PVE instances stored on removed body-token contract, got %d", len(cfg.PVEInstances))
	}
}

func TestHandleAutoRegisterRejectsRemovedHeaderTokenContract(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	record, err := config.NewAPITokenRecord("org-bound-header-token-123.12345678", "org-bound", []string{config.ScopeAgentReport})
	if err != nil {
		t.Fatalf("new api token record: %v", err)
	}
	record.OrgID = "org-a"

	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
		APITokens:  []config.APITokenRecord{*record},
	}

	handler := newTestConfigHandlers(t, cfg)

	reqBody := AutoRegisterRequest{
		Type:       "pve",
		Host:       "https://pve.local:8006",
		TokenID:    "pulse-monitor@pve!token",
		TokenValue: "secret-token",
		ServerName: "pve.local",
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", bytes.NewReader(body))
	req.Header.Set("X-API-Token", "org-bound-header-token-123.12345678")
	req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, "org-b"))
	rec := httptest.NewRecorder()

	handler.HandleAutoRegister(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d, body=%s", rec.Code, rec.Body.String())
	}
	if len(cfg.PVEInstances) != 0 {
		t.Fatalf("expected no PVE instances stored on org mismatch, got %d", len(cfg.PVEInstances))
	}
}

func TestHandleAutoRegisterRejectsNonCanonicalTokenID(t *testing.T) {
	stubAutoRegisterNetworkDeps(t)

	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
		PVEInstances: []config.PVEInstance{
			{
				Name:        "homelab",
				ClusterName: "cluster-A",
				IsCluster:   true,
				ClusterEndpoints: []config.ClusterEndpoint{
					{NodeName: "minipc", Host: "https://10.0.0.5:8006"},
				},
			},
		},
	}
	handler := newTestConfigHandlers(t, cfg)

	const tokenValue = "TEMP-TOKEN"
	tokenHash := internalauth.HashAPIToken(tokenValue)
	handler.codeMutex.Lock()
	handler.setupTokens[tokenHash] = &SetupTokenRecord{
		ExpiresAt: time.Now().Add(5 * time.Minute),
		NodeType:  "pve",
	}
	handler.codeMutex.Unlock()

	reqBody := AutoRegisterRequest{
		Type:       "pve",
		Host:       "https://10.0.0.5:8006",
		TokenID:    "pulse@pve!token",
		TokenValue: "secret",
		ServerName: "minipc",
		AuthToken:  tokenValue,
		Source:     "script",
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleAutoRegister(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d, body=%s", rec.Code, rec.Body.String())
	}
	if len(cfg.PVEInstances) != 1 {
		t.Fatalf("expected canonical cluster instance to remain unchanged, got %d", len(cfg.PVEInstances))
	}
	cluster := cfg.PVEInstances[0]
	if got := cluster.TokenName; got != "" {
		t.Fatalf("TokenName = %q, want empty unchanged token id", got)
	}
	if got := cluster.TokenValue; got != "" {
		t.Fatalf("TokenValue = %q, want empty unchanged token value", got)
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
	stubAutoRegisterNetworkDeps(t)

	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

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
	handler.setupTokens[tokenHash] = &SetupTokenRecord{
		ExpiresAt: time.Now().Add(5 * time.Minute),
		NodeType:  "pve",
	}
	handler.codeMutex.Unlock()

	reqBody1 := AutoRegisterRequest{
		Type:       "pve",
		Host:       "https://10.0.1.100:8006",
		TokenID:    "pulse-monitor@pve!pulse-px1-a",
		TokenValue: "secret-token-1",
		ServerName: "px1",
		AuthToken:  tokenValue,
		Source:     "script",
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
	handler.setupTokens[tokenHash] = &SetupTokenRecord{
		ExpiresAt: time.Now().Add(5 * time.Minute),
		NodeType:  "pve",
	}
	handler.codeMutex.Unlock()

	reqBody2 := AutoRegisterRequest{
		Type:       "pve",
		Host:       "https://10.0.2.224:8006",
		TokenID:    "pulse-monitor@pve!pulse-px1-b",
		TokenValue: "secret-token-2",
		ServerName: "px1", // Same hostname as first node
		AuthToken:  tokenValue,
		Source:     "script",
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
