package configapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// A provider MSP client workspace runs in hosted mode with no local
// credential, API token, proxy secret or SSO provider: its users sign in
// through the control plane handoff. The install endpoints still have to mint
// a token, because the runtime enforces authentication and an agent without
// one cannot report. Before hosted mode counted as configured auth, the host
// flow answered 200 with an empty token and the Proxmox flow built a command
// with no token in it.
func TestHostedRuntimeMintsAgentInstallTokensWithoutLocalAuth(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
	}{
		{name: "host agent", body: `{"type":"host","name":"client-host"}`},
		{name: "proxmox agent", body: `{"type":"pve"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{
				DataPath:  t.TempDir(),
				PublicURL: "https://t-client.msp.example",
			}
			handler := newTestConfigHandlers(t, cfg)
			handler.hostedMode = true

			req := httptest.NewRequest(http.MethodPost, "/api/agent-install-command", strings.NewReader(tc.body))
			req.Host = "t-client.msp.example"
			rec := httptest.NewRecorder()
			handler.HandleAgentInstallCommand(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
			}

			var resp AgentInstallCommandResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if strings.TrimSpace(resp.Token) == "" {
				t.Fatal("hosted runtime returned an install response without a token")
			}
			if len(cfg.APITokens) != 1 {
				t.Fatalf("persisted API tokens = %d, want 1", len(cfg.APITokens))
			}
			if resp.Command != "" && !strings.Contains(resp.Command, resp.Token) {
				t.Fatalf("install command does not carry the minted token: %s", resp.Command)
			}
		})
	}
}

// Self-hosted runtimes keep the existing rule: with no authentication
// configured at all there is nothing for a token to authenticate against.
func TestSelfHostedRuntimeWithoutAuthStillOmitsInstallToken(t *testing.T) {
	cfg := &config.Config{DataPath: t.TempDir(), PublicURL: "http://pulse.local:7655"}
	handler := newTestConfigHandlers(t, cfg)

	req := httptest.NewRequest(http.MethodPost, "/api/agent-install-command", strings.NewReader(`{"type":"host","name":"open-host"}`))
	req.Host = "pulse.local:7655"
	rec := httptest.NewRecorder()
	handler.HandleAgentInstallCommand(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var resp AgentInstallCommandResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Token != "" || len(cfg.APITokens) != 0 {
		t.Fatalf("unauthenticated self-hosted runtime minted a token: %+v", resp)
	}
}
