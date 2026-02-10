package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/stretchr/testify/require"
)

func TestHostedTenantAgentInstallCommand_GeneratesOrgBoundTokenAndCommand(t *testing.T) {
	t.Setenv("PULSE_MOCK_MODE", "true")
	mock.SetEnabled(true)
	t.Cleanup(func() { mock.SetEnabled(false) })

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
	require.Contains(t, resp.Command, "--token "+resp.Token)
	require.Contains(t, resp.Command, "--proxmox-type pbs")

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
