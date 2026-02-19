package api

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/stretchr/testify/require"
)

func TestNormalizeProxmoxInstallType(t *testing.T) {
	installType, err := normalizeProxmoxInstallType(" PBS ")
	require.NoError(t, err)
	require.Equal(t, "pbs", installType)

	_, err = normalizeProxmoxInstallType("vmware")
	require.Error(t, err)
	require.Equal(t, "Type must be 'pve' or 'pbs'", err.Error())
}

func TestBuildProxmoxAgentInstallCommand(t *testing.T) {
	command := buildProxmoxAgentInstallCommand(agentInstallCommandOptions{
		BaseURL:            "https://pulse.example.com/",
		Token:              "token-123",
		InstallType:        "pbs",
		IncludeInstallType: true,
	})
	require.Contains(t, command, posixShellQuote("https://pulse.example.com/install.sh"))
	require.Contains(t, command, "--token "+posixShellQuote("token-123"))
	require.Contains(t, command, "--proxmox-type "+posixShellQuote("pbs"))
}

func TestBuildProxmoxAgentInstallCommand_ShellEscapesArguments(t *testing.T) {
	baseURL := "https://pulse.example.com' && touch /tmp/pwned #"
	token := "tok'en"
	command := buildProxmoxAgentInstallCommand(agentInstallCommandOptions{
		BaseURL:            baseURL,
		Token:              token,
		InstallType:        "pve",
		IncludeInstallType: true,
	})

	require.Contains(t, command, posixShellQuote(baseURL+"/install.sh"))
	require.Contains(t, command, "--url "+posixShellQuote(baseURL))
	require.Contains(t, command, "--token "+posixShellQuote(token))
	require.Contains(t, command, "--proxmox-type "+posixShellQuote("pve"))
}

func TestResolveConfigAgentInstallBaseURL(t *testing.T) {
	cfg := &config.Config{
		AgentConnectURL: "https://agents.example.com/",
		PublicURL:       "https://public.example.com",
		FrontendPort:    7655,
	}
	req := httptest.NewRequest("POST", "/api/agent-install-command", nil)
	req.Host = "127.0.0.1:7655"
	req.Header.Set("X-Forwarded-Proto", "https")

	require.Equal(t, "https://agents.example.com", resolveConfigAgentInstallBaseURL(req, cfg))

	cfg.AgentConnectURL = ""
	require.Equal(t, "https://public.example.com", resolveConfigAgentInstallBaseURL(req, cfg))
}

func TestIssueAndPersistAgentInstallToken(t *testing.T) {
	dataDir := t.TempDir()
	cfg := &config.Config{DataPath: dataDir}
	persistence := config.NewConfigPersistence(dataDir)

	rawToken, record, err := issueAndPersistAgentInstallToken(cfg, persistence, issueAgentInstallTokenOptions{
		TokenName: "test-install-token",
		OrgID:     "acme",
		Metadata: map[string]string{
			"install_type": "pbs",
			"issued_via":   "test",
		},
	})
	require.NoError(t, err)
	require.NotEmpty(t, rawToken)
	require.NotNil(t, record)
	require.Equal(t, "acme", record.OrgID)
	require.Equal(t, "pbs", record.Metadata["install_type"])
	require.Equal(t, "test", record.Metadata["issued_via"])

	require.Len(t, cfg.APITokens, 1)
	savedRecord, ok := cfg.ValidateAPIToken(rawToken)
	require.True(t, ok)
	require.Equal(t, "acme", savedRecord.OrgID)

	tokens, err := persistence.LoadAPITokens()
	require.NoError(t, err)
	require.Len(t, tokens, 1)
	require.True(t, strings.HasPrefix(tokens[0].Name, "test-install-token"))
}
