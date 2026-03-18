package api

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

func TestBuildProxmoxAgentInstallCommand_IncludesInsecureForPlainHTTP(t *testing.T) {
	command := buildProxmoxAgentInstallCommand(agentInstallCommandOptions{
		BaseURL:            "http://pulse.example.com:7655/",
		Token:              "token-123",
		InstallType:        "pve",
		IncludeInstallType: true,
	})

	require.Contains(t, command, posixShellQuote("http://pulse.example.com:7655/install.sh"))
	require.Contains(t, command, "--url "+posixShellQuote("http://pulse.example.com:7655"))
	require.Contains(t, command, "--insecure")
}

func TestBuildProxmoxAgentInstallCommand_UsesPrivilegeEscalationWrapper(t *testing.T) {
	command := buildProxmoxAgentInstallCommand(agentInstallCommandOptions{
		BaseURL:            "https://pulse.example.com/",
		Token:              "token-123",
		InstallType:        "pve",
		IncludeInstallType: true,
	})

	require.Contains(t, command, `| { if [ "$(id -u)" -eq 0 ]; then bash -s --`)
	require.Contains(t, command, `elif command -v sudo >/dev/null 2>&1; then sudo bash -s --`)
	require.Contains(t, command, `else echo "Root privileges required. Run as root (su -) and retry." >&2; exit 1; fi; }`)
	require.NotContains(t, command, "| bash -s -- --url")
}

func TestBuildProxmoxAgentInstallCommand_OmitsTokenWhenNotProvided(t *testing.T) {
	command := buildProxmoxAgentInstallCommand(agentInstallCommandOptions{
		BaseURL:            "https://pulse.example.com/",
		Token:              "",
		InstallType:        "pbs",
		IncludeInstallType: true,
	})

	require.Contains(t, command, "--url "+posixShellQuote("https://pulse.example.com"))
	require.Contains(t, command, "--proxmox-type "+posixShellQuote("pbs"))
	require.NotContains(t, command, "--token")
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

func TestBuildProxmoxAgentInstallCommand_NormalizesTrailingSlashes(t *testing.T) {
	command := buildProxmoxAgentInstallCommand(agentInstallCommandOptions{
		BaseURL:            "https://pulse.example.com/base///",
		Token:              "token-123",
		InstallType:        "pve",
		IncludeInstallType: true,
	})

	require.Contains(t, command, posixShellQuote("https://pulse.example.com/base/install.sh"))
	require.Contains(t, command, "--url "+posixShellQuote("https://pulse.example.com/base"))
	require.NotContains(t, command, "//install.sh")
	require.NotContains(t, command, "--url "+posixShellQuote("https://pulse.example.com/base/"))
}

func TestBuildContainerRuntimeAgentInstallCommand_UsesLifecycleTransport(t *testing.T) {
	command := buildContainerRuntimeAgentInstallCommand("https://pulse.example.com/base", "token-123")

	require.Contains(t, command, posixShellQuote("https://pulse.example.com/base/install.sh"))
	require.Contains(t, command, "--url "+posixShellQuote("https://pulse.example.com/base"))
	require.Contains(t, command, "--token "+posixShellQuote("token-123"))
	require.Contains(t, command, "--enable-docker")
	require.Contains(t, command, "--enable-host=false")
	require.Contains(t, command, "--interval 30s")
	require.Contains(t, command, `| { if [ "$(id -u)" -eq 0 ]; then bash -s --`)
	require.Contains(t, command, `elif command -v sudo >/dev/null 2>&1; then sudo bash -s --`)
	require.NotContains(t, command, "| sudo bash -s -- --url")
}

func TestBuildContainerRuntimeAgentInstallCommand_OmitsTokenAndAddsInsecureForHTTP(t *testing.T) {
	command := buildContainerRuntimeAgentInstallCommand("http://pulse.example.com:7655/", "")

	require.Contains(t, command, posixShellQuote("http://pulse.example.com:7655/install.sh"))
	require.Contains(t, command, "--url "+posixShellQuote("http://pulse.example.com:7655"))
	require.NotContains(t, command, "--token")
	require.Contains(t, command, "--insecure")
}

func TestBuildSetupScriptCommand_UsesFailFastQuotedTransport(t *testing.T) {
	command := buildSetupScriptCommand("https://pulse.example.com/api/setup-script?type=pve&host=pve1.local", "token-123")

	require.Contains(t, command, "curl -fsSL "+posixShellQuote("https://pulse.example.com/api/setup-script?type=pve&host=pve1.local")+" | ")
	require.Contains(t, command, `if [ "$(id -u)" -eq 0 ]; then PULSE_SETUP_TOKEN=`+posixShellQuote("token-123")+` bash`)
	require.Contains(t, command, `elif command -v sudo >/dev/null 2>&1; then sudo env PULSE_SETUP_TOKEN=`+posixShellQuote("token-123")+` bash`)
	require.Contains(t, command, `else echo "Root privileges required. Run as root (su -) and retry." >&2; exit 1; fi; }`)
}

func TestBuildSetupScriptCommand_OmitsTokenWhenNotProvided(t *testing.T) {
	command := buildSetupScriptCommand("https://pulse.example.com/api/setup-script?type=pbs", "")

	require.Contains(t, command, "curl -fsSL "+posixShellQuote("https://pulse.example.com/api/setup-script?type=pbs")+" | ")
	require.Contains(t, command, `if [ "$(id -u)" -eq 0 ]; then bash`)
	require.Contains(t, command, `elif command -v sudo >/dev/null 2>&1; then sudo bash`)
	require.NotContains(t, command, "PULSE_SETUP_TOKEN=")
}

func TestBuildSetupScriptURL_PreservesEncodedHostAndBackupPerms(t *testing.T) {
	scriptURL := buildSetupScriptURL(
		"https://pulse.example.com:7656/",
		"pve",
		"https://[2001:db8::1]:8006",
		"https://pulse.example.com:7656",
		true,
	)

	require.Equal(
		t,
		"https://pulse.example.com:7656/api/setup-script?backup_perms=true&host=https%3A%2F%2F%5B2001%3Adb8%3A%3A1%5D%3A8006&pulse_url=https%3A%2F%2Fpulse.example.com%3A7656&type=pve",
		scriptURL,
	)
}

func TestBuildSetupScriptInstallArtifact_UsesSharedBackendShape(t *testing.T) {
	expiresAt := time.Unix(1900000000, 0).UTC().Unix()

	artifact := buildSetupScriptInstallArtifact(
		"https://pulse.example",
		"pve",
		"https://pve1.local:8006",
		"https://pulse.example",
		true,
		"setup-token-123",
		expiresAt,
	)

	require.Equal(t, "pve", artifact.Type)
	require.Equal(t, "https://pve1.local:8006", artifact.Host)
	require.Contains(t, artifact.URL, "/api/setup-script?")
	require.Contains(t, artifact.DownloadURL, "setup_token=setup-token-123")
	require.Equal(t, "pulse-setup-pve.sh", artifact.ScriptFileName)
	require.Equal(t, artifact.Command, artifact.CommandWithEnv)
	require.Contains(t, artifact.CommandWithEnv, "PULSE_SETUP_TOKEN='setup-token-123'")
	require.NotContains(t, artifact.CommandWithoutEnv, "PULSE_SETUP_TOKEN=")
	require.Equal(t, expiresAt, artifact.Expires)
	require.Equal(t, "setup-token-123", artifact.SetupToken)
	require.Equal(t, "set…123", artifact.TokenHint)
}

func TestRenderSetupScript_UsesSharedInstallArtifactShape(t *testing.T) {
	artifact := buildSetupScriptInstallArtifact(
		"https://pulse.example",
		"pve",
		"https://pve1.local:8006",
		"https://pulse.example",
		true,
		"setup-token-123",
		0,
	)

	script := renderSetupScript("pve", setupScriptRenderContext{
		ServerName:       deriveSetupScriptServerName("https://pve1.local:8006"),
		PulseURL:         "https://pulse.example",
		ServerHost:       "https://pve1.local:8006",
		SetupToken:       "setup-token-123",
		TokenName:        buildPulseMonitorTokenName("https://pulse.example"),
		TokenMatchPrefix: buildPulseMonitorTokenName("https://pulse.example"),
		StoragePerms:     "\npveum aclmod /storage -user pulse-monitor@pve -role PVEDatastoreAdmin",
		SensorsPublicKey: "ssh-ed25519 AAAATEST pulse@test",
		Artifact:         artifact,
	})

	require.Contains(t, script, `SETUP_SCRIPT_URL="https://pulse.example/api/setup-script?backup_perms=true&host=https%3A%2F%2Fpve1.local%3A8006&pulse_url=https%3A%2F%2Fpulse.example&type=pve"`)
	require.Contains(t, script, `PULSE_BOOTSTRAP_COMMAND_WITH_ENV='curl -fsSL '"'"'https://pulse.example/api/setup-script?backup_perms=true&host=https%3A%2F%2Fpve1.local%3A8006&pulse_url=https%3A%2F%2Fpulse.example&type=pve'"'"' | `)
	require.Contains(t, script, `PULSE_SETUP_TOKEN="${PULSE_SETUP_TOKEN:-setup-token-123}"`)
	require.Contains(t, script, `pveum aclmod /storage -user pulse-monitor@pve -role PVEDatastoreAdmin`)
}

func TestInstallBaseURLRequiresInsecure(t *testing.T) {
	require.True(t, installBaseURLRequiresInsecure("http://pulse.example.com:7655"))
	require.True(t, installBaseURLRequiresInsecure(" HTTP://pulse.example.com:7655/ "))
	require.False(t, installBaseURLRequiresInsecure("https://pulse.example.com"))
	require.False(t, installBaseURLRequiresInsecure(""))
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

func TestResolveConfigAgentInstallBaseURL_PreservesConfiguredPublicSchemeOnLoopback(t *testing.T) {
	cfg := &config.Config{
		PublicURL:    "https://public.example.com/base/",
		FrontendPort: 7655,
	}
	req := httptest.NewRequest("POST", "/api/agent-install-command", nil)
	req.Host = "127.0.0.1:7655"

	require.Equal(t, "https://public.example.com/base", resolveConfigAgentInstallBaseURL(req, cfg))
}

func TestResolveConfigAgentInstallBaseURL_UsesConfiguredPublicURLForIPv6Loopback(t *testing.T) {
	cfg := &config.Config{
		PublicURL:    "https://public.example.com",
		FrontendPort: 7655,
	}
	req := httptest.NewRequest("POST", "/api/agent-install-command", nil)
	req.Host = "[::1]:7655"

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
