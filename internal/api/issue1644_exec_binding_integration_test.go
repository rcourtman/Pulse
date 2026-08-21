package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// Issue #1644 follow-up: the auto-register path writes bound_hostname for host
// install tokens without bound_agent_id or a binding version. The first
// command-channel enrollment must take the clean first-use bind.
func TestIssue1644AutoRegisteredHostTokenBindsExecOnFirstUse(t *testing.T) {
	stubAutoRegisterNetworkDeps(t)

	dataPath := t.TempDir()
	cfg := &config.Config{DataPath: dataPath, AuthUser: "admin", AuthPass: "hashed-password"}
	handler := newTestConfigHandlers(t, cfg)

	installReq := httptest.NewRequest(http.MethodPost, "/api/agent-install-command", strings.NewReader(`{"type":"host","name":"issue-1644-exec","enableCommands":true}`))
	installReq.Host = "pulse.example:7655"
	installRec := httptest.NewRecorder()
	handler.HandleAgentInstallCommand(installRec, installReq)
	if installRec.Code != http.StatusOK {
		t.Fatalf("host install token mint status = %d, body=%s", installRec.Code, installRec.Body.String())
	}
	var install AgentInstallCommandResponse
	if err := json.Unmarshal(installRec.Body.Bytes(), &install); err != nil {
		t.Fatalf("decode host install token response: %v", err)
	}
	rawToken := strings.TrimSpace(install.Token)
	if rawToken == "" {
		t.Fatal("host install token mint omitted runtime token")
	}
	if !cfg.APITokens[0].HasScope(config.ScopeAgentExec) {
		t.Fatalf("commands-enabled host install token is missing %s: %v", config.ScopeAgentExec, cfg.APITokens[0].Scopes)
	}

	registerRec := runAgentAutoRegister(t, handler, rawToken, AutoRegisterRequest{
		Type: "pve", Host: "https://pve-exec.local:8006", TokenID: "pulse-monitor@pve!pulse-pve-exec",
		TokenValue: "proxmox-secret", ServerName: "pve-exec", Source: "agent",
	})
	if registerRec.Code != http.StatusOK {
		t.Fatalf("host-token pve registration status = %d, body=%s", registerRec.Code, registerRec.Body.String())
	}
	if got := cfg.APITokens[0].Metadata["bound_hostname"]; got != "pve-exec" {
		t.Fatalf("auto-register bound hostname = %q, want %q", got, "pve-exec")
	}
	if got := strings.TrimSpace(cfg.APITokens[0].Metadata["bound_agent_id"]); got != "" {
		t.Fatalf("auto-register wrote bound_agent_id = %q; the exec first-use path assumes it is empty", got)
	}

	decision := evaluateAgentExecBinding(&cfg.APITokens[0], "agent-machine-id", "pve-exec.lan")
	if !decision.admit || !decision.firstBind {
		t.Fatalf("auto-registered install token exec decision = %+v, want a clean first bind", decision)
	}
	if decision.legacyMigrate {
		t.Fatal("auto-registered install token was admitted through the legacy migration branch")
	}

	router := &Router{config: cfg, persistence: config.NewConfigPersistence(dataPath)}
	admission, ok := router.admitAgentExecToken(rawToken, "agent-machine-id", "pve-exec.lan")
	if !ok {
		t.Fatal("auto-registered host install token was rejected on the command channel")
	}
	if admission.AgentID != "agent-machine-id" {
		t.Fatalf("admitted agent id = %q, want %q", admission.AgentID, "agent-machine-id")
	}

	config.Mu.RLock()
	defer config.Mu.RUnlock()
	bound := cfg.APITokens[0].Metadata
	if got := bound["bound_agent_id"]; got != "agent-machine-id" {
		t.Fatalf("bound_agent_id = %q, want %q", got, "agent-machine-id")
	}
	if got := bound[agentExecBindingVersionKey]; got != agentExecBindingVersion {
		t.Fatalf("%s = %q, want %q", agentExecBindingVersionKey, got, agentExecBindingVersion)
	}
	if got := bound["bound_hostname"]; got != "pve-exec" {
		t.Fatalf("bound_hostname after exec bind = %q, want the auto-registered %q", got, "pve-exec")
	}
}
