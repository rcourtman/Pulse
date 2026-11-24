package configapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/api/apicontext"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
)

// Authentication and organization selection accept a query-string token only
// when the Upgrade header is exactly "websocket" in any case. This copy must
// not accept one they did not see: strings.TrimSpace also removes U+00A0 and
// U+0085, which net/http leaves in a header value.
func TestExplicitAPITokenFromRequestQueryTokenNeedsExactUpgrade(t *testing.T) {
	cases := []struct {
		upgrade string
		want    bool
	}{
		{"websocket", true},
		{"WebSocket", true},
		{"", false},
		{"websocket\u00a0", false},
		{"\u0085websocket", false},
		{" websocket ", false},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodPost, "/api/auto-register?token=secret", nil)
		if tc.upgrade != "" {
			req.Header.Set("Upgrade", tc.upgrade)
		}
		token, ok := explicitAPITokenFromRequest(req)
		if ok != tc.want || (ok && token != "secret") {
			t.Errorf("Upgrade %q: token = %q, %v, want found = %v", tc.upgrade, token, ok, tc.want)
		}
	}
}

// The default organization's inventory is the instance inventory, which also
// holds the install tokens of every tenant organization. A token bound to a
// tenant must not register a Proxmox source in the default organization, even
// when the request reaches the handler without tenant selection having seen
// the token.
func TestTenantInstallTokenCannotAutoRegisterInDefaultOrganization(t *testing.T) {
	stubAutoRegisterNetworkDeps(t)

	const orgID = "t-CLIENT0003"
	dataDir := t.TempDir()
	cfg := &config.Config{DataPath: dataDir, ConfigPath: dataDir, PublicURL: "https://pulse.example"}
	mtp := config.NewMultiTenantPersistence(dataDir)
	if _, err := mtp.GetPersistence(orgID); err != nil {
		t.Fatalf("create tenant persistence: %v", err)
	}
	mtm := monitoring.NewMultiTenantMonitor(cfg, mtp, nil)
	t.Cleanup(mtm.Stop)
	handler := NewConfigHandlers(mtp, mtm, func() error { return nil }, nil, nil, func() {}, true)
	handler.SetConfig(cfg)
	orgCtx := context.WithValue(context.Background(), apicontext.OrgIDContextKey, orgID)

	installReq := httptest.NewRequest(http.MethodPost, "/api/agent-install-command", strings.NewReader(`{"type":"pve"}`))
	installReq.Host = "pulse.example"
	installRec := httptest.NewRecorder()
	handler.HandleAgentInstallCommand(installRec, installReq.WithContext(orgCtx))
	if installRec.Code != http.StatusOK {
		t.Fatalf("install command status = %d, body=%s", installRec.Code, installRec.Body.String())
	}
	var install AgentInstallCommandResponse
	if err := json.Unmarshal(installRec.Body.Bytes(), &install); err != nil {
		t.Fatalf("decode install command: %v", err)
	}
	if len(cfg.APITokens) != 1 || cfg.APITokens[0].OrgID != orgID {
		t.Fatalf("instance inventory = %+v, want one token bound to %s", cfg.APITokens, orgID)
	}

	body, err := json.Marshal(AutoRegisterRequest{
		Type:       "pve",
		Host:       "https://pve-client.local:8006",
		TokenID:    "pulse-monitor@pve!pulse-client",
		TokenValue: "proxmox-secret",
		ServerName: "pve-client",
		Source:     "agent",
	})
	if err != nil {
		t.Fatalf("marshal auto-register: %v", err)
	}
	for _, ctx := range []context.Context{
		context.Background(),
		context.WithValue(context.Background(), apicontext.OrgIDContextKey, "default"),
	} {
		req := httptest.NewRequest(http.MethodPost, "/api/auto-register", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-API-Token", install.Token)
		rec := httptest.NewRecorder()
		handler.HandleAutoRegister(rec, req.WithContext(ctx))
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("default-org auto-register with a tenant token: status = %d, want 401, body=%s", rec.Code, rec.Body.String())
		}
	}
	if len(cfg.PVEInstances) != 0 {
		t.Fatalf("default organization gained %d Proxmox connection(s)", len(cfg.PVEInstances))
	}
	if got := cfg.APITokens[0].Metadata[proxmoxInstallRegistrationCompletedKey]; got != "" {
		t.Fatalf("tenant token grant = %q, want unspent", got)
	}
}
