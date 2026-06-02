package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	"github.com/stretchr/testify/require"
)

func TestHostedTenantAgentInstallCommand_GeneratesOrgBoundTokenAndCommand(t *testing.T) {
	setMockModeForTest(t, true)

	dataDir := t.TempDir()
	cfg := &config.Config{
		DataPath:  dataDir,
		PublicURL: "https://cloud.example.com",
	}

	router := &Router{
		config:      cfg,
		persistence: config.NewConfigPersistence(dataDir),
		multiTenant: config.NewMultiTenantPersistence(dataDir),
		hostedMode:  true,
	}

	orgID := "acme"
	// Ensure org directory exists so OrgExists() passes.
	_, err := router.multiTenant.GetPersistence(orgID)
	require.NoError(t, err)

	// Initialize tenant monitor to verify we update already-loaded tenant configs.
	router.mtMonitor = monitoring.NewMultiTenantMonitor(cfg, router.multiTenant, nil)
	t.Cleanup(router.mtMonitor.Stop)
	tenantMonitor, err := router.mtMonitor.GetMonitor(orgID)
	require.NoError(t, err)
	require.NotNil(t, tenantMonitor)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/admin/orgs/{id}/agent-install-command", router.handleHostedTenantAgentInstallCommand)

	req := httptest.NewRequest(http.MethodPost, "/api/admin/orgs/"+orgID+"/agent-install-command", strings.NewReader(`{"type":"pbs"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	var resp hostedTenantAgentInstallCommandResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))

	require.Equal(t, orgID, resp.OrgID)
	require.Contains(t, resp.Command, "https://cloud.example.com/install.sh")
	require.Contains(t, resp.Command, "printf %s "+posixShellQuote(resp.Token)+` > "$token_file"`)
	require.Contains(t, resp.Command, `--token-file "$token_file"`)
	require.Contains(t, resp.Command, "--proxmox-type "+posixShellQuote("pbs"))

	require.Regexp(t, regexp.MustCompile(`^[a-f0-9]{64}$`), resp.Token)

	// Token is persisted to global token store with org binding.
	tokens, err := router.persistence.LoadAPITokens()
	require.NoError(t, err)
	require.NotEmpty(t, tokens)

	found := false
	for _, tok := range tokens {
		if tok.OrgID == orgID && tok.Name != "" && tok.Hash != "" {
			// Validate raw token matches this record.
			if _, ok := (&config.Config{APITokens: []config.APITokenRecord{tok}}).ValidateAPIToken(resp.Token); ok {
				require.Equal(t, "pbs", tok.Metadata["install_type"])
				require.Equal(t, "hosted_agent_install_command", tok.Metadata["issued_via"])
				found = true
				break
			}
		}
	}
	require.True(t, found, "expected persisted org-bound token record matching response token")

	// Existing tenant monitor config sees the token immediately.
	require.NotNil(t, tenantMonitor.GetConfig())
	_, ok := tenantMonitor.GetConfig().ValidateAPIToken(resp.Token)
	require.True(t, ok, "expected tenant monitor config to validate newly issued token")
}

func TestGenerateHostedTenantAgentInstallCommandEnforcesHostedOrgBoundary(t *testing.T) {
	setMockModeForTest(t, true)

	dataDir := t.TempDir()
	cfg := &config.Config{
		DataPath:  dataDir,
		PublicURL: "https://cloud.example.com",
	}
	persistence := config.NewConfigPersistence(dataDir)
	multiTenant := config.NewMultiTenantPersistence(dataDir)

	_, err := GenerateHostedTenantAgentInstallCommand(HostedTenantAgentInstallCommandOptions{
		Config:      cfg,
		Persistence: persistence,
		MultiTenant: multiTenant,
		HostedMode:  false,
		OrgID:       "acme",
		InstallType: "pve",
		BaseURL:     "https://acme.cloud.example.com",
	})
	require.ErrorIs(t, err, ErrHostedTenantInstallRequiresHostedMode)

	_, err = GenerateHostedTenantAgentInstallCommand(HostedTenantAgentInstallCommandOptions{
		Config:      cfg,
		Persistence: persistence,
		MultiTenant: multiTenant,
		HostedMode:  true,
		OrgID:       "missing",
		InstallType: "pve",
		BaseURL:     "https://missing.cloud.example.com",
	})
	require.ErrorIs(t, err, ErrHostedTenantInstallInvalidOrg)

	orgID := "acme"
	_, err = multiTenant.GetPersistence(orgID)
	require.NoError(t, err)

	result, err := GenerateHostedTenantAgentInstallCommand(HostedTenantAgentInstallCommandOptions{
		Config:      cfg,
		Persistence: persistence,
		MultiTenant: multiTenant,
		HostedMode:  true,
		OrgID:       orgID,
		InstallType: "pbs",
		OwnerUserID: "owner-1",
		BaseURL:     "https://acme.cloud.example.com/",
	})
	require.NoError(t, err)
	require.Equal(t, orgID, result.OrgID)
	require.Equal(t, "pbs", result.InstallType)
	require.NotEmpty(t, result.Command)
	require.NotEmpty(t, result.Token)
	require.NotEmpty(t, result.TokenID)
	require.Contains(t, result.Command, "https://acme.cloud.example.com/install.sh")
	require.Contains(t, result.Command, "--proxmox-type "+posixShellQuote("pbs"))

	tokens, err := persistence.LoadAPITokens()
	require.NoError(t, err)
	require.Len(t, tokens, 1)
	require.Equal(t, orgID, tokens[0].OrgID)
	require.Equal(t, "owner-1", tokens[0].Metadata[apiTokenMetadataOwnerUserID])
	require.Equal(t, "pbs", tokens[0].Metadata["install_type"])
	require.Equal(t, "hosted_agent_install_command", tokens[0].Metadata["issued_via"])
	_, ok := (&config.Config{APITokens: tokens}).ValidateAPIToken(result.Token)
	require.True(t, ok)
}

func TestHostedTenantAgentInstallTokenCannotReportToOtherTenant(t *testing.T) {
	defer SetMultiTenantEnabled(false)
	SetMultiTenantEnabled(true)
	t.Setenv("PULSE_DEV", "true")
	setMockModeForTest(t, true)

	dataDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   dataDir,
		ConfigPath: dataDir,
		PublicURL:  "https://msp.example.com",
	}
	persistence := config.NewConfigPersistence(dataDir)
	multiTenant := config.NewMultiTenantPersistence(dataDir)
	for _, orgID := range []string{"client-a", "client-b"} {
		_, err := multiTenant.GetPersistence(orgID)
		require.NoError(t, err)
	}

	mtm := monitoring.NewMultiTenantMonitor(cfg, multiTenant, nil)
	t.Cleanup(mtm.Stop)
	_, err := mtm.GetMonitor("client-a")
	require.NoError(t, err)

	install, err := GenerateHostedTenantAgentInstallCommand(HostedTenantAgentInstallCommandOptions{
		Config:        cfg,
		Persistence:   persistence,
		MultiTenant:   multiTenant,
		TenantMonitor: mtm,
		HostedMode:    true,
		OrgID:         "client-a",
		InstallType:   "pve",
		OwnerUserID:   "owner-1",
		BaseURL:       "https://client-a.msp.example.com",
	})
	require.NoError(t, err)

	router := NewRouter(cfg, nil, mtm, nil, nil, "1.0.0")
	report := agentshost.Report{
		Agent: agentshost.AgentInfo{
			ID:      "agent-client-a",
			Version: "1.0.0",
			Type:    "unified",
		},
		Host: agentshost.HostInfo{
			ID:        "machine-client-a",
			MachineID: "machine-client-a",
			Hostname:  "pve1",
			Platform:  "linux",
		},
		Timestamp: time.Now().UTC(),
	}
	body, err := json.Marshal(report)
	require.NoError(t, err)

	allowedReq := httptest.NewRequest(http.MethodPost, "/api/agents/agent/report", bytes.NewReader(body))
	allowedReq.Header.Set("X-API-Token", install.Token)
	allowedRec := httptest.NewRecorder()
	router.Handler().ServeHTTP(allowedRec, allowedReq)
	require.Equal(t, http.StatusOK, allowedRec.Code, allowedRec.Body.String())

	retargetReq := httptest.NewRequest(http.MethodPost, "/api/agents/agent/report", bytes.NewReader(body))
	retargetReq.Header.Set("X-API-Token", install.Token)
	retargetReq.Header.Set("X-Pulse-Org-ID", "client-b")
	retargetRec := httptest.NewRecorder()
	router.Handler().ServeHTTP(retargetRec, retargetReq)
	require.Equal(t, http.StatusForbidden, retargetRec.Code, retargetRec.Body.String())

	monitorA, ok := mtm.PeekMonitor("client-a")
	require.True(t, ok)
	require.Len(t, monitorA.GetLiveHostsSnapshot(), 1)

	monitorB, ok := mtm.PeekMonitor("client-b")
	require.True(t, ok)
	require.Empty(t, monitorB.GetLiveHostsSnapshot(), "retargeted client-a token must not write into client-b")
}
