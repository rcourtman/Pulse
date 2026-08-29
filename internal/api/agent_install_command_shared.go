package api

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/api/agenttokens"
	"github.com/rcourtman/pulse-go-rewrite/internal/api/configapi"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

const (
	proxmoxInstallTypePVE = "pve"
	proxmoxInstallTypePBS = "pbs"
	// agentInstallTypeHost marks install tokens minted for the generic unified
	// host agent flow (Settings > Infrastructure > Add Pulse Agent), as opposed
	// to the Proxmox-specific pve/pbs installer.
	agentInstallTypeHost = "host"
)

var (
	errAgentInstallTokenGeneration = agenttokens.ErrGeneration
	errAgentInstallTokenRecord     = agenttokens.ErrRecord
	errAgentInstallTokenPersist    = agenttokens.ErrPersist
)

func normalizeProxmoxInstallType(raw string) (string, error) {
	installType := strings.ToLower(strings.TrimSpace(raw))
	if installType != proxmoxInstallTypePVE && installType != proxmoxInstallTypePBS {
		return "", fmt.Errorf("Type must be 'pve' or 'pbs'")
	}
	return installType, nil
}

func proxmoxAgentInstallScopes(enableCommands bool) []string {
	return agenttokens.ProxmoxScopes(enableCommands)
}

// hostAgentInstallScopes returns the scopes for a generic unified host agent
// install token. The exec scope is included only when the operator asked for
// command execution, because the token is minted before the agent enrols and
// scopes cannot be upgraded on an existing token.
func hostAgentInstallScopes(enableCommands bool) []string {
	return agenttokens.HostScopes(enableCommands)
}

type issueAgentInstallTokenOptions = agenttokens.IssueOptions

func issueAndPersistAgentInstallToken(cfg *config.Config, persistence *config.ConfigPersistence, opts issueAgentInstallTokenOptions) (string, *config.APITokenRecord, error) {
	return agenttokens.IssueAndPersist(cfg, persistence, opts)
}

type agentInstallCommandOptions = configapi.AgentInstallCommandOptions

type setupScriptInstallArtifact = configapi.SetupScriptInstallArtifact

func normalizeAgentInstallBaseURL(raw string) string {
	return strings.TrimRight(strings.TrimSpace(raw), "/")
}

func posixShellQuote(value string) string {
	escaped := strings.ReplaceAll(value, "'", `'"'"'`)
	return "'" + escaped + "'"
}

func installBaseURLRequiresInsecure(raw string) bool {
	baseURL := strings.ToLower(strings.TrimSpace(raw))
	return strings.HasPrefix(baseURL, "http://")
}

func authConfiguredForAgentLifecycle(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}

	return (strings.TrimSpace(cfg.AuthUser) != "" && strings.TrimSpace(cfg.AuthPass) != "") ||
		cfg.HasAPITokens() ||
		strings.TrimSpace(cfg.ProxyAuthSecret) != "" ||
		hasEnabledSSOProvidersForAuth(cfg)
}

func withPrivilegeEscalation(command string) string {
	const installPipe = "| bash -s --"

	idx := strings.Index(command, installPipe)
	if idx == -1 {
		return command
	}

	args := command[idx+len(installPipe):]
	return command[:idx] +
		`| { if [ "$(id -u)" -eq 0 ]; then bash -s --` + args +
		`; elif command -v sudo >/dev/null 2>&1; then sudo bash -s --` + args +
		`; else echo "Root privileges required. Run as root (su -) and retry." >&2; exit 1; fi; }`
}

func buildProxmoxAgentInstallCommand(opts agentInstallCommandOptions) string {
	return configapi.BuildProxmoxAgentInstallCommand(opts)
}

func containerRuntimeAgentScopes(enableHost bool) []string {
	scopes := []string{config.ScopeDockerReport}
	if enableHost {
		scopes = append(scopes,
			config.ScopeAgentReport,
			config.ScopeAgentConfigRead,
			config.ScopeAgentManage,
		)
	}
	return scopes
}

func containerRuntimeAgentHostFlag(enableHost bool) string {
	if enableHost {
		return "--enable-host"
	}
	return "--enable-host=false"
}

func buildContainerRuntimeAgentInstallCommand(baseURL string, token string, enableHost bool) string {
	normalizedBaseURL := normalizeAgentInstallBaseURL(baseURL)
	installScriptURL := normalizedBaseURL + "/install.sh"
	command := fmt.Sprintf(`curl -fsSL %s | bash -s -- \
  --url %s \
  --enable-docker \
  %s \
  --interval 30s`,
		posixShellQuote(installScriptURL), posixShellQuote(normalizedBaseURL), containerRuntimeAgentHostFlag(enableHost))

	if trimmedToken := strings.TrimSpace(token); trimmedToken != "" {
		command += fmt.Sprintf(` \
  --token %s`, posixShellQuote(trimmedToken))
	}

	if installBaseURLRequiresInsecure(normalizedBaseURL) {
		command += ` \
  --insecure`
	}

	return withPrivilegeEscalation(command)
}

func buildSetupScriptCommand(scriptURL string, token string) string {
	curlCommand := "curl -fsSL " + posixShellQuote(strings.TrimSpace(scriptURL)) + " | "
	bashCommand := "bash"
	sudoCommand := "sudo bash"
	if trimmedToken := strings.TrimSpace(token); trimmedToken != "" {
		envPrefix := "PULSE_SETUP_TOKEN=" + posixShellQuote(trimmedToken) + " "
		bashCommand = envPrefix + bashCommand
		sudoCommand = "sudo env " + envPrefix + "bash"
	}

	return curlCommand +
		`{ if [ "$(id -u)" -eq 0 ]; then ` + bashCommand +
		`; elif command -v sudo >/dev/null 2>&1; then ` + sudoCommand +
		`; else echo "Root privileges required. Run as root (su -) and retry." >&2; exit 1; fi; }`
}

func buildSetupScriptTokenHint(token string) string {
	trimmed := strings.TrimSpace(token)
	if len(trimmed) <= 6 {
		return trimmed
	}
	return fmt.Sprintf("%s…%s", trimmed[:3], trimmed[len(trimmed)-3:])
}

func buildSetupScriptURL(baseURL string, installType string, host string, pulseURL string, backupPerms bool) string {
	query := url.Values{}
	query.Set("type", strings.TrimSpace(installType))

	if trimmedHost := strings.TrimSpace(host); trimmedHost != "" {
		query.Set("host", trimmedHost)
	}

	if trimmedPulseURL := strings.TrimSpace(pulseURL); trimmedPulseURL != "" {
		query.Set("pulse_url", trimmedPulseURL)
	}

	if backupPerms && strings.TrimSpace(installType) == "pve" {
		query.Set("backup_perms", "true")
	}

	return normalizeAgentInstallBaseURL(baseURL) + "/api/setup-script?" + query.Encode()
}

func buildSetupScriptDownloadURL(baseURL string, installType string, host string, pulseURL string, backupPerms bool, setupToken string) string {
	downloadURL := buildSetupScriptURL(baseURL, installType, host, pulseURL, backupPerms)
	trimmedToken := strings.TrimSpace(setupToken)
	if trimmedToken == "" {
		return downloadURL
	}

	parsed, err := url.Parse(downloadURL)
	if err != nil {
		return downloadURL
	}

	query := parsed.Query()
	query.Set("setup_token", trimmedToken)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func buildSetupScriptFileName(installType string) string {
	return fmt.Sprintf("pulse-setup-%s.sh", strings.TrimSpace(installType))
}

func buildSetupScriptInstallArtifact(baseURL string, installType string, host string, pulseURL string, backupPerms bool, setupToken string, expiresAt int64) setupScriptInstallArtifact {
	scriptURL := buildSetupScriptURL(baseURL, installType, host, pulseURL, backupPerms)
	commandWithEnv := buildSetupScriptCommand(scriptURL, setupToken)

	return setupScriptInstallArtifact{
		Type:              strings.TrimSpace(installType),
		Host:              strings.TrimSpace(host),
		URL:               scriptURL,
		DownloadURL:       buildSetupScriptDownloadURL(baseURL, installType, host, pulseURL, backupPerms, setupToken),
		ScriptFileName:    buildSetupScriptFileName(installType),
		Command:           commandWithEnv,
		CommandWithEnv:    commandWithEnv,
		CommandWithoutEnv: buildSetupScriptCommand(scriptURL, ""),
		Expires:           expiresAt,
		SetupToken:        strings.TrimSpace(setupToken),
		TokenHint:         buildSetupScriptTokenHint(setupToken),
	}
}

func resolveConfigAgentInstallBaseURL(req *http.Request, cfg *config.Config, hostedMode bool) string {
	return resolveConfiguredPublicBaseURL(req, cfg, hostedMode)
}

func writeConfigAgentInstallBaseURLUnavailable(w http.ResponseWriter) {
	http.Error(w, "A valid external Pulse URL is required", http.StatusServiceUnavailable)
}
