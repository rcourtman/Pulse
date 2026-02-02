package api

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

func resetConfigSigningStateForTests() {
	configSigningState = struct {
		once sync.Once
		key  ed25519.PrivateKey
		err  error
	}{}
}

func generateSigningKey(t *testing.T) string {
	t.Helper()

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	return base64.StdEncoding.EncodeToString(priv)
}

func decodeErrorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()

	var resp struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	return resp.Code
}

func TestHostAgentHandlers_HandleReportMethodNotAllowed(t *testing.T) {
	handler := newHostAgentHandlerForTests(t)

	req := httptest.NewRequest(http.MethodGet, "/api/agents/host/report", nil)
	rec := httptest.NewRecorder()

	handler.HandleReport(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHostAgentHandlers_HandleReportInvalidJSON(t *testing.T) {
	handler := newHostAgentHandlerForTests(t)

	req := httptest.NewRequest(http.MethodPost, "/api/agents/host/report", bytes.NewBufferString("{"))
	rec := httptest.NewRecorder()

	handler.HandleReport(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if code := decodeErrorCode(t, rec); code != "invalid_json" {
		t.Fatalf("expected error code %q, got %q", "invalid_json", code)
	}
}

func TestHostAgentHandlers_HandleReportInvalidReport(t *testing.T) {
	handler := newHostAgentHandlerForTests(t)

	report := agentshost.Report{
		Agent: agentshost.AgentInfo{ID: "agent-err"},
		Host:  agentshost.HostInfo{Hostname: ""},
	}
	body, _ := json.Marshal(report)

	req := httptest.NewRequest(http.MethodPost, "/api/agents/host/report", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.HandleReport(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if code := decodeErrorCode(t, rec); code != "invalid_report" {
		t.Fatalf("expected error code %q, got %q", "invalid_report", code)
	}
}

func TestHostAgentHandlers_HandleReportIncludesConfigOverride(t *testing.T) {
	handler, monitor := newHostAgentHandlers(t, nil)

	hostID := "machine-override"
	enabled := true
	if err := monitor.UpdateHostAgentConfig(hostID, &enabled); err != nil {
		t.Fatalf("UpdateHostAgentConfig: %v", err)
	}

	report := agentshost.Report{
		Agent: agentshost.AgentInfo{ID: "agent-override"},
		Host: agentshost.HostInfo{
			ID:       hostID,
			Hostname: "host-override.local",
			Platform: "linux",
		},
	}
	body, _ := json.Marshal(report)

	req := httptest.NewRequest(http.MethodPost, "/api/agents/host/report", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.HandleReport(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["hostId"] != hostID {
		t.Fatalf("expected hostId %q, got %#v", hostID, resp["hostId"])
	}
	cfg, ok := resp["config"].(map[string]any)
	if !ok {
		t.Fatalf("expected config override in response")
	}
	if val, ok := cfg["commandsEnabled"].(bool); !ok || !val {
		t.Fatalf("expected commandsEnabled=true, got %#v", cfg["commandsEnabled"])
	}
}

func TestHostAgentHandlers_HandleDeleteHostErrors(t *testing.T) {
	handler := newHostAgentHandlerForTests(t)

	req := httptest.NewRequest(http.MethodGet, "/api/agents/host/host-1", nil)
	rec := httptest.NewRecorder()
	handler.HandleDeleteHost(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/agents/host/", nil)
	rec = httptest.NewRecorder()
	handler.HandleDeleteHost(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if code := decodeErrorCode(t, rec); code != "missing_host_id" {
		t.Fatalf("expected error code %q, got %q", "missing_host_id", code)
	}
}

func TestHostAgentHandlers_HandleConfigErrors(t *testing.T) {
	handler := newHostAgentHandlerForTests(t, models.Host{
		ID:      "host-1",
		TokenID: "token-1",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/agents/host/host-1/config", nil)
	rec := httptest.NewRecorder()
	handler.HandleConfig(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/agents/host//config", nil)
	rec = httptest.NewRecorder()
	handler.HandleConfig(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if code := decodeErrorCode(t, rec); code != "missing_host_id" {
		t.Fatalf("expected error code %q, got %q", "missing_host_id", code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/agents/host/host-1/config", nil)
	attachAPITokenRecord(req, &config.APITokenRecord{
		ID:     "token-2",
		Scopes: []string{config.ScopeHostConfigRead},
	})
	rec = httptest.NewRecorder()
	handler.HandleConfig(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
	if code := decodeErrorCode(t, rec); code != "host_not_found" {
		t.Fatalf("expected error code %q, got %q", "host_not_found", code)
	}
}

func TestHostAgentHandlers_HandleConfigPatchErrors(t *testing.T) {
	handler := newHostAgentHandlerForTests(t)

	req := httptest.NewRequest(http.MethodPatch, "/api/agents/host/host-1/config", bytes.NewBufferString("{"))
	rec := httptest.NewRecorder()
	handler.HandleConfig(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if code := decodeErrorCode(t, rec); code != "invalid_json" {
		t.Fatalf("expected error code %q, got %q", "invalid_json", code)
	}

	req = httptest.NewRequest(http.MethodPatch, "/api/agents/host/host-1/config", bytes.NewBufferString(`{"commandsEnabled":true}`))
	rec = httptest.NewRecorder()
	handler.HandleConfig(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}
	if code := decodeErrorCode(t, rec); code != "update_failed" {
		t.Fatalf("expected error code %q, got %q", "update_failed", code)
	}
}

func TestHostAgentHandlers_EnsureHostTokenMatchScopes(t *testing.T) {
	handler := newHostAgentHandlerForTests(t)

	req := httptest.NewRequest(http.MethodGet, "/api/agents/host/host-1/config", nil)
	attachAPITokenRecord(req, &config.APITokenRecord{
		ID:     "token-1",
		Scopes: []string{config.ScopeWildcard},
	})
	rec := httptest.NewRecorder()
	if ok := handler.ensureHostTokenMatch(rec, req, "missing"); !ok {
		t.Fatalf("expected wildcard scope to allow access")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/agents/host/host-1/config", nil)
	attachAPITokenRecord(req, &config.APITokenRecord{
		ID:     "token-1",
		Scopes: []string{config.ScopeHostConfigRead},
	})
	rec = httptest.NewRecorder()
	if ok := handler.ensureHostTokenMatch(rec, req, "missing"); ok {
		t.Fatalf("expected missing host to fail")
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestHostAgentHandlers_HandleConfigSigningRequiredMissingKey(t *testing.T) {
	handler := newHostAgentHandlerForTests(t, models.Host{ID: "host-1"})

	t.Setenv("PULSE_AGENT_CONFIG_SIGNATURE_REQUIRED", "true")
	t.Setenv("PULSE_AGENT_CONFIG_SIGNING_KEY", "")
	resetConfigSigningStateForTests()
	t.Cleanup(resetConfigSigningStateForTests)

	req := httptest.NewRequest(http.MethodGet, "/api/agents/host/host-1/config", nil)
	rec := httptest.NewRecorder()
	handler.HandleConfig(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}
	if code := decodeErrorCode(t, rec); code != "config_signing_failed" {
		t.Fatalf("expected error code %q, got %q", "config_signing_failed", code)
	}
}

func TestHostAgentHandlers_HandleConfigSigningSuccess(t *testing.T) {
	handler := newHostAgentHandlerForTests(t, models.Host{ID: "host-1"})

	t.Setenv("PULSE_AGENT_CONFIG_SIGNATURE_REQUIRED", "true")
	t.Setenv("PULSE_AGENT_CONFIG_SIGNING_KEY", generateSigningKey(t))
	resetConfigSigningStateForTests()
	t.Cleanup(resetConfigSigningStateForTests)

	req := httptest.NewRequest(http.MethodGet, "/api/agents/host/host-1/config", nil)
	rec := httptest.NewRecorder()
	handler.HandleConfig(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp struct {
		Success bool                       `json:"success"`
		HostID  string                     `json:"hostId"`
		Config  monitoring.HostAgentConfig `json:"config"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.HostID != "host-1" {
		t.Fatalf("expected hostId %q, got %q", "host-1", resp.HostID)
	}
	if resp.Config.Signature == "" {
		t.Fatalf("expected config signature to be set")
	}
	if resp.Config.IssuedAt == nil || resp.Config.ExpiresAt == nil {
		t.Fatalf("expected issuedAt/expiresAt to be set")
	}
	if !resp.Config.ExpiresAt.After(*resp.Config.IssuedAt) {
		t.Fatalf("expected expiresAt after issuedAt")
	}
}

func TestHostAgentHandlers_HandleConfigInvalidKeyAllowed(t *testing.T) {
	handler := newHostAgentHandlerForTests(t, models.Host{ID: "host-1"})

	t.Setenv("PULSE_AGENT_CONFIG_SIGNATURE_REQUIRED", "false")
	t.Setenv("PULSE_AGENT_CONFIG_SIGNING_KEY", "not-base64")
	resetConfigSigningStateForTests()
	t.Cleanup(resetConfigSigningStateForTests)

	req := httptest.NewRequest(http.MethodGet, "/api/agents/host/host-1/config", nil)
	rec := httptest.NewRecorder()
	handler.HandleConfig(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp struct {
		Config monitoring.HostAgentConfig `json:"config"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Config.Signature != "" {
		t.Fatalf("expected no signature when signing key is invalid")
	}
	if resp.Config.IssuedAt != nil || resp.Config.ExpiresAt != nil {
		t.Fatalf("expected no issuedAt/expiresAt when signing key is invalid")
	}
}

func TestHostAgentHandlers_HandleUninstallErrors(t *testing.T) {
	handler := newHostAgentHandlerForTests(t)

	req := httptest.NewRequest(http.MethodGet, "/api/agents/host/uninstall", nil)
	rec := httptest.NewRecorder()
	handler.HandleUninstall(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/agents/host/uninstall", bytes.NewBufferString("{"))
	rec = httptest.NewRecorder()
	handler.HandleUninstall(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if code := decodeErrorCode(t, rec); code != "invalid_json" {
		t.Fatalf("expected error code %q, got %q", "invalid_json", code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/agents/host/uninstall", bytes.NewBufferString(`{"hostId":""}`))
	rec = httptest.NewRecorder()
	handler.HandleUninstall(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if code := decodeErrorCode(t, rec); code != "missing_host_id" {
		t.Fatalf("expected error code %q, got %q", "missing_host_id", code)
	}
}

func TestHostAgentHandlers_HandleUninstallMissingHostStillSucceeds(t *testing.T) {
	handler, _ := newHostAgentHandlers(t, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/agents/host/uninstall", bytes.NewBufferString(`{"hostId":"missing"}`))
	rec := httptest.NewRecorder()
	handler.HandleUninstall(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestHostAgentHandlers_HandleLinkUnlinkErrors(t *testing.T) {
	handler := newHostAgentHandlerForTests(t)

	req := httptest.NewRequest(http.MethodGet, "/api/agents/host/link", nil)
	rec := httptest.NewRecorder()
	handler.HandleLink(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/agents/host/link", bytes.NewBufferString("{"))
	rec = httptest.NewRecorder()
	handler.HandleLink(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if code := decodeErrorCode(t, rec); code != "invalid_json" {
		t.Fatalf("expected error code %q, got %q", "invalid_json", code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/agents/host/link", bytes.NewBufferString(`{"hostId":"","nodeId":"node-1"}`))
	rec = httptest.NewRecorder()
	handler.HandleLink(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if code := decodeErrorCode(t, rec); code != "missing_host_id" {
		t.Fatalf("expected error code %q, got %q", "missing_host_id", code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/agents/host/link", bytes.NewBufferString(`{"hostId":"host-1","nodeId":""}`))
	rec = httptest.NewRecorder()
	handler.HandleLink(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if code := decodeErrorCode(t, rec); code != "missing_node_id" {
		t.Fatalf("expected error code %q, got %q", "missing_node_id", code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/agents/host/link", bytes.NewBufferString(`{"hostId":"host-1","nodeId":"node-1"}`))
	rec = httptest.NewRecorder()
	handler.HandleLink(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if code := decodeErrorCode(t, rec); code != "link_failed" {
		t.Fatalf("expected error code %q, got %q", "link_failed", code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/agents/host/unlink", nil)
	rec = httptest.NewRecorder()
	handler.HandleUnlink(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/agents/host/unlink", bytes.NewBufferString("{"))
	rec = httptest.NewRecorder()
	handler.HandleUnlink(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if code := decodeErrorCode(t, rec); code != "invalid_json" {
		t.Fatalf("expected error code %q, got %q", "invalid_json", code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/agents/host/unlink", bytes.NewBufferString(`{"hostId":""}`))
	rec = httptest.NewRecorder()
	handler.HandleUnlink(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if code := decodeErrorCode(t, rec); code != "missing_host_id" {
		t.Fatalf("expected error code %q, got %q", "missing_host_id", code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/agents/host/unlink", bytes.NewBufferString(`{"hostId":"missing"}`))
	rec = httptest.NewRecorder()
	handler.HandleUnlink(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
	if code := decodeErrorCode(t, rec); code != "unlink_failed" {
		t.Fatalf("expected error code %q, got %q", "unlink_failed", code)
	}
}
