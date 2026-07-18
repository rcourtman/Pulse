package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func decodeHostAgentInstallResponse(t *testing.T, rec *httptest.ResponseRecorder) AgentInstallCommandResponse {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status OK, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp AgentInstallCommandResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	return resp
}

func TestHandleAgentInstallCommand_HostWithCommands(t *testing.T) {
	cfg := &config.Config{DataPath: t.TempDir(), AuthUser: "admin", AuthPass: "hashed-password"}
	handler := newTestConfigHandlers(t, cfg)

	body := []byte(`{"type":"host","enableCommands":true,"name":"nuc-agent"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/agent-install-command", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleAgentInstallCommand(rec, req)

	resp := decodeHostAgentInstallResponse(t, rec)
	if resp.Token == "" {
		t.Fatalf("expected token in response")
	}
	if resp.Record == nil {
		t.Fatalf("expected token record in response")
	}
	if resp.Record.Name != "nuc-agent" {
		t.Fatalf("expected requested token name, got %q", resp.Record.Name)
	}
	if len(cfg.APITokens) != 1 {
		t.Fatalf("expected API token to be persisted")
	}
	record := cfg.APITokens[0]
	if !record.HasScope(config.ScopeAgentExec) {
		t.Fatalf("expected exec scope on commands-enabled host install token, got %v", record.Scopes)
	}
	if !record.HasScope(config.ScopeDockerReport) || !record.HasScope(config.ScopeAgentReport) {
		t.Fatalf("expected reporting scopes on host install token, got %v", record.Scopes)
	}
	if got := record.Metadata["install_type"]; got != agentInstallTypeHost {
		t.Fatalf("install_type metadata = %q, want %q", got, agentInstallTypeHost)
	}
	if got := record.Metadata["issued_via"]; got != agentInstallIssuedViaConfig {
		t.Fatalf("issued_via metadata = %q, want %q", got, agentInstallIssuedViaConfig)
	}
	if !canBindAgentInstallExecToken(&record, "agent-nuc", "nuc") {
		t.Fatalf("expected host install token to be eligible for first-use exec binding")
	}
}

func TestHandleAgentInstallCommand_HostWithoutCommands(t *testing.T) {
	cfg := &config.Config{DataPath: t.TempDir(), AuthUser: "admin", AuthPass: "hashed-password"}
	handler := newTestConfigHandlers(t, cfg)

	body := []byte(`{"type":"host","enableCommands":false}`)
	req := httptest.NewRequest(http.MethodPost, "/api/agent-install-command", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleAgentInstallCommand(rec, req)

	resp := decodeHostAgentInstallResponse(t, rec)
	if resp.Token == "" {
		t.Fatalf("expected token in response")
	}
	if len(cfg.APITokens) != 1 {
		t.Fatalf("expected API token to be persisted")
	}
	record := cfg.APITokens[0]
	if record.HasScope(config.ScopeAgentExec) {
		t.Fatalf("expected no exec scope without enableCommands, got %v", record.Scopes)
	}
}

func TestHandleAgentInstallCommand_HostOmitsTokenWhenAuthOptional(t *testing.T) {
	cfg := &config.Config{DataPath: t.TempDir()}
	handler := newTestConfigHandlers(t, cfg)

	body := []byte(`{"type":"host","enableCommands":true}`)
	req := httptest.NewRequest(http.MethodPost, "/api/agent-install-command", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleAgentInstallCommand(rec, req)

	resp := decodeHostAgentInstallResponse(t, rec)
	if resp.Token != "" {
		t.Fatalf("expected optional-auth host install response to omit token, got %q", resp.Token)
	}
	if len(cfg.APITokens) != 0 {
		t.Fatalf("expected optional-auth host install to avoid persisting API tokens")
	}
}
