package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// Issue #1644: the Settings > Infrastructure installer mints generic host
// install tokens, while install.sh auto-detects Proxmox on the target machine
// and presents type "pve"/"pbs" to /api/auto-register. The bootstrap grant
// must accept a host-issued install token for a canonical Proxmox type, while
// staying one-shot and hostname-bound.

func mintHostInstallToken(t *testing.T, handler *ConfigHandlers) string {
	t.Helper()
	installReq := httptest.NewRequest(
		http.MethodPost,
		"/api/agent-install-command",
		strings.NewReader(`{"type":"host","name":"issue-1644-host"}`),
	)
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
	if strings.TrimSpace(install.Token) == "" {
		t.Fatal("host install token mint omitted runtime token")
	}
	return install.Token
}

func TestIssue1644HostInstallTokenBootstrapsDetectedProxmoxSource(t *testing.T) {
	stubAutoRegisterNetworkDeps(t)

	cfg := &config.Config{
		DataPath: t.TempDir(),
		AuthUser: "admin",
		AuthPass: "hashed-password",
	}
	handler := newTestConfigHandlers(t, cfg)
	rawToken := mintHostInstallToken(t, handler)
	if len(cfg.APITokens) != 1 {
		t.Fatalf("API tokens = %d, want 1", len(cfg.APITokens))
	}
	if got := cfg.APITokens[0].Metadata["install_type"]; got != agentInstallTypeHost {
		t.Fatalf("minted install_type = %q, want %q", got, agentInstallTypeHost)
	}

	// The agent's pre-registration check must report canRegister=true so
	// runForType proceeds instead of aborting with a blocked registration.
	checkRec := runAgentAutoRegister(t, handler, rawToken, AutoRegisterRequest{
		Type:              "pve",
		Host:              "https://pve-host.local:8006",
		ServerName:        "pve-host",
		Source:            "agent",
		CheckRegistration: true,
	})
	if checkRec.Code != http.StatusOK {
		t.Fatalf("registration check status = %d, body=%s", checkRec.Code, checkRec.Body.String())
	}
	var check autoRegisterCheckResponse
	if err := json.Unmarshal(checkRec.Body.Bytes(), &check); err != nil {
		t.Fatalf("decode registration check: %v", err)
	}
	if !check.CanRegister {
		t.Fatalf("host install token presenting pve reported canRegister=%v, want true", check.CanRegister)
	}

	registerRec := runAgentAutoRegister(t, handler, rawToken, AutoRegisterRequest{
		Type:       "pve",
		Host:       "https://pve-host.local:8006",
		TokenID:    "pulse-monitor@pve!pulse-pve-host",
		TokenValue: "proxmox-secret",
		ServerName: "pve-host",
		Source:     "agent",
	})
	if registerRec.Code != http.StatusOK {
		t.Fatalf("host-token pve registration status = %d, body=%s", registerRec.Code, registerRec.Body.String())
	}
	if len(cfg.PVEInstances) != 1 {
		t.Fatalf("PVE instances = %d, want 1", len(cfg.PVEInstances))
	}
	if !strings.EqualFold(cfg.APITokens[0].Metadata[proxmoxInstallRegistrationCompletedKey], "true") {
		t.Fatal("host-token pve registration did not consume the install grant")
	}

	// The grant stays one-shot: after the pve bootstrap completes, the same
	// token cannot bootstrap another source of any type.
	reuseRec := runAgentAutoRegister(t, handler, rawToken, AutoRegisterRequest{
		Type:       "pbs",
		Host:       "https://pbs-host.local:8007",
		TokenID:    "pulse-monitor@pbs!pulse-pbs-host",
		TokenValue: "other-proxmox-secret",
		ServerName: "pve-host",
		Source:     "agent",
	})
	if reuseRec.Code != http.StatusForbidden {
		t.Fatalf("consumed host-token grant reuse status = %d, want 403; body=%s", reuseRec.Code, reuseRec.Body.String())
	}
	if len(cfg.PBSInstances) != 0 {
		t.Fatalf("PBS instances = %d, want 0 after consumed grant", len(cfg.PBSInstances))
	}
}

func TestIssue1644HostInstallTokenGrantStaysHostnameBound(t *testing.T) {
	stubAutoRegisterNetworkDeps(t)

	rawToken := "issue-1644-host-bound.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeAgentReport}, map[string]string{
		"install_type": agentInstallTypeHost,
		"issued_via":   agentInstallIssuedViaConfig,
	})
	cfg := &config.Config{
		DataPath:  t.TempDir(),
		APITokens: []config.APITokenRecord{record},
	}
	handler := newTestConfigHandlers(t, cfg)

	firstRec := runAgentAutoRegister(t, handler, rawToken, AutoRegisterRequest{
		Type:       "pve",
		Host:       "https://pve-bound.local:8006",
		TokenID:    "pulse-monitor@pve!pulse-pve-bound",
		TokenValue: "proxmox-secret",
		ServerName: "pve-bound",
		Source:     "agent",
	})
	if firstRec.Code != http.StatusOK {
		t.Fatalf("bound host-token registration status = %d, body=%s", firstRec.Code, firstRec.Body.String())
	}
	if got := cfg.APITokens[0].Metadata["bound_hostname"]; got != "pve-bound" {
		t.Fatalf("bound hostname = %q, want %q", got, "pve-bound")
	}

	otherHostRec := runAgentAutoRegister(t, handler, rawToken, AutoRegisterRequest{
		Type:       "pve",
		Host:       "https://pve-other.local:8006",
		TokenID:    "pulse-monitor@pve!pulse-pve-other",
		TokenValue: "proxmox-secret",
		ServerName: "pve-other",
		Source:     "agent",
	})
	if otherHostRec.Code != http.StatusForbidden {
		t.Fatalf("cross-host host-token registration status = %d, want 403; body=%s", otherHostRec.Code, otherHostRec.Body.String())
	}
	if len(cfg.PVEInstances) != 1 {
		t.Fatalf("PVE instances = %d, want 1 after cross-host rejection", len(cfg.PVEInstances))
	}
}

func TestIssue1644HostInstallTokenRejectsNonCanonicalType(t *testing.T) {
	rawToken := "issue-1644-host-bad-type.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeAgentReport}, map[string]string{
		"install_type": agentInstallTypeHost,
		"issued_via":   agentInstallIssuedViaConfig,
	})
	req := &AutoRegisterRequest{
		Type:       "host",
		ServerName: "some-host",
		Source:     "agent",
	}
	if canBootstrapProxmoxInstallRegistration(&record, req) {
		t.Fatal("host install token must not hold a bootstrap grant for non-Proxmox types")
	}
}

func runIssue1644AutoRegisterRaw(t *testing.T, handler *ConfigHandlers, rawToken string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	handler.HandleAutoRegister(rec, req)
	return rec
}

func TestIssue1644TypedInstallTokenStaysPinnedToItsType(t *testing.T) {
	rawToken := "issue-1644-typed-pinned.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeAgentReport}, map[string]string{
		"install_type": "pve",
		"issued_via":   agentInstallIssuedViaConfig,
	})
	cfg := &config.Config{
		DataPath:  t.TempDir(),
		APITokens: []config.APITokenRecord{record},
	}
	handler := newTestConfigHandlers(t, cfg)

	payload, err := json.Marshal(AutoRegisterRequest{
		Type:       "pbs",
		Host:       "https://pbs-pinned.local:8007",
		TokenID:    "pulse-monitor@pbs!pulse-pbs-pinned",
		TokenValue: "proxmox-secret",
		ServerName: "pbs-pinned",
		Source:     "agent",
	})
	if err != nil {
		t.Fatalf("marshal pinned-type payload: %v", err)
	}
	rec := runIssue1644AutoRegisterRaw(t, handler, rawToken, payload)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("pve-typed token presenting pbs status = %d, want 403; body=%s", rec.Code, rec.Body.String())
	}
}
