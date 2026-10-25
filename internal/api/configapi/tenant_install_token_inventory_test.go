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

// A Proxmox install token minted in a tenant organization lives only in the
// instance inventory, bound to that org. Auto-register for the org must find
// it there and record the one-shot grant on that record, where report-time
// identity binding reads it and where it survives a tenant reload. Before,
// the grant was written to whichever copy the tenant config happened to hold.
func TestTenantProxmoxInstallTokenGrantLivesOnInstanceRecord(t *testing.T) {
	stubAutoRegisterNetworkDeps(t)

	const orgID = "t-CLIENT0002"
	dataDir := t.TempDir()
	cfg := &config.Config{DataPath: dataDir, ConfigPath: dataDir, PublicURL: "https://t-client0002.msp.example"}
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
	installReq.Host = "t-client0002.msp.example"
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
	tenantMonitor, err := mtm.GetMonitor(orgID)
	if err != nil {
		t.Fatalf("tenant monitor: %v", err)
	}
	if n := len(tenantMonitor.GetConfig().APITokens); n != 0 {
		t.Fatalf("tenant config copy holds %d tokens; the instance record must be the only copy", n)
	}

	register := func(server string) *httptest.ResponseRecorder {
		body, err := json.Marshal(AutoRegisterRequest{
			Type:       "pve",
			Host:       "https://" + server + ".local:8006",
			TokenID:    "pulse-monitor@pve!pulse-" + server,
			TokenValue: "proxmox-secret",
			ServerName: server,
			Source:     "agent",
		})
		if err != nil {
			t.Fatalf("marshal auto-register: %v", err)
		}
		req := httptest.NewRequest(http.MethodPost, "/api/auto-register", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-API-Token", install.Token)
		rec := httptest.NewRecorder()
		handler.HandleAutoRegister(rec, req.WithContext(orgCtx))
		return rec
	}

	if rec := register("pve-client"); rec.Code != http.StatusOK {
		t.Fatalf("tenant auto-register status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if got := cfg.APITokens[0].Metadata[proxmoxInstallRegistrationCompletedKey]; got != "true" {
		t.Fatalf("instance record grant = %q, want consumed", got)
	}
	persisted, err := config.NewConfigPersistence(dataDir).LoadAPITokens()
	if err != nil || len(persisted) != 1 || persisted[0].Metadata[proxmoxInstallRegistrationCompletedKey] != "true" {
		t.Fatalf("persisted instance inventory = %+v (err %v), want the consumed grant", persisted, err)
	}

	// A reloaded tenant runtime still sees the consumed grant.
	if _, err := mtm.ReloadTenant(orgID); err != nil {
		t.Fatalf("reload tenant: %v", err)
	}
	if rec := register("pve-other"); rec.Code != http.StatusForbidden {
		t.Fatalf("second bootstrap after reload status = %d, want 403; body=%s", rec.Code, rec.Body.String())
	}
}
