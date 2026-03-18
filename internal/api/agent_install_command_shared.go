package api

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

const (
	proxmoxInstallTypePVE = "pve"
	proxmoxInstallTypePBS = "pbs"
)

var (
	errAgentInstallTokenGeneration = errors.New("agent install token generation failed")
	errAgentInstallTokenRecord     = errors.New("agent install token record failed")
	errAgentInstallTokenPersist    = errors.New("agent install token persistence failed")
)

func normalizeProxmoxInstallType(raw string) (string, error) {
	installType := strings.ToLower(strings.TrimSpace(raw))
	if installType != proxmoxInstallTypePVE && installType != proxmoxInstallTypePBS {
		return "", fmt.Errorf("Type must be 'pve' or 'pbs'")
	}
	return installType, nil
}

func proxmoxAgentInstallScopes() []string {
	return []string{
		config.ScopeAgentReport,
		config.ScopeAgentConfigRead,
		config.ScopeAgentManage,
		config.ScopeAgentExec,
	}
}

type issueAgentInstallTokenOptions struct {
	TokenName string
	OrgID     string
	Metadata  map[string]string
}

func issueAndPersistAgentInstallToken(cfg *config.Config, persistence *config.ConfigPersistence, opts issueAgentInstallTokenOptions) (string, *config.APITokenRecord, error) {
	if cfg == nil {
		return "", nil, fmt.Errorf("config is required")
	}

	rawToken, err := internalauth.GenerateAPIToken()
	if err != nil {
		return "", nil, fmt.Errorf("%w: %w", errAgentInstallTokenGeneration, err)
	}

	record, err := config.NewAPITokenRecord(rawToken, opts.TokenName, proxmoxAgentInstallScopes())
	if err != nil {
		return "", nil, fmt.Errorf("%w: %w", errAgentInstallTokenRecord, err)
	}

	record.OrgID = strings.TrimSpace(opts.OrgID)
	if len(opts.Metadata) > 0 {
		record.Metadata = make(map[string]string, len(opts.Metadata))
		for k, v := range opts.Metadata {
			record.Metadata[k] = v
		}
	}

	config.Mu.Lock()
	defer config.Mu.Unlock()

	cfg.APITokens = append(cfg.APITokens, *record)
	cfg.SortAPITokens()
	if persistence != nil {
		if err := persistence.SaveAPITokens(cfg.APITokens); err != nil {
			cfg.APITokens = cfg.APITokens[:len(cfg.APITokens)-1]
			return "", nil, fmt.Errorf("%w: %w", errAgentInstallTokenPersist, err)
		}
	}

	return rawToken, record, nil
}

type agentInstallCommandOptions struct {
	BaseURL            string
	Token              string
	InstallType        string
	IncludeInstallType bool
}

type setupScriptInstallArtifact struct {
	Type              string `json:"type"`
	Host              string `json:"host"`
	URL               string `json:"url"`
	DownloadURL       string `json:"downloadURL"`
	ScriptFileName    string `json:"scriptFileName"`
	Command           string `json:"command"`
	CommandWithEnv    string `json:"commandWithEnv"`
	CommandWithoutEnv string `json:"commandWithoutEnv"`
	Expires           int64  `json:"expires"`
	SetupToken        string `json:"setupToken"`
	TokenHint         string `json:"tokenHint"`
}

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

func requestTargetsLocalFrontend(host string, frontendPort int) bool {
	if frontendPort <= 0 {
		return false
	}

	parsedHost, parsedPort, err := net.SplitHostPort(strings.TrimSpace(host))
	if err != nil || parsedPort != strconv.Itoa(frontendPort) {
		return false
	}

	switch strings.Trim(parsedHost, "[]") {
	case "127.0.0.1", "localhost", "::1":
		return true
	default:
		return false
	}
}

func resolveLoopbackAwarePublicBaseURL(req *http.Request, cfg *config.Config) string {
	if req != nil && cfg != nil && requestTargetsLocalFrontend(req.Host, cfg.FrontendPort) {
		if publicURL := strings.TrimSpace(cfg.PublicURL); publicURL != "" {
			if parsedURL, err := url.Parse(publicURL); err == nil && parsedURL.Scheme != "" && parsedURL.Host != "" {
				return strings.TrimRight(parsedURL.String(), "/")
			}
		}
	}

	host := ""
	if req != nil {
		host = strings.TrimSpace(req.Host)
	}
	if host == "" {
		if cfg != nil && cfg.FrontendPort > 0 {
			host = fmt.Sprintf("localhost:%d", cfg.FrontendPort)
		} else {
			host = "localhost:7655"
		}
	}

	scheme := "http"
	if req != nil && (req.TLS != nil || strings.EqualFold(req.Header.Get("X-Forwarded-Proto"), "https")) {
		scheme = "https"
	}

	return fmt.Sprintf("%s://%s", scheme, host)
}

func buildProxmoxAgentInstallCommand(opts agentInstallCommandOptions) string {
	baseURL := normalizeAgentInstallBaseURL(opts.BaseURL)
	installScriptURL := baseURL + "/install.sh"
	command := fmt.Sprintf(`curl -fsSL %s | bash -s -- \
  --url %s \
  --enable-proxmox`,
		posixShellQuote(installScriptURL), posixShellQuote(baseURL))

	if token := strings.TrimSpace(opts.Token); token != "" {
		command += fmt.Sprintf(` \
  --token %s`, posixShellQuote(token))
	}

	if installBaseURLRequiresInsecure(baseURL) {
		command += ` \
  --insecure`
	}

	if opts.IncludeInstallType {
		command += fmt.Sprintf(` \
  --proxmox-type %s`, posixShellQuote(opts.InstallType))
	}

	return withPrivilegeEscalation(command)
}

func buildContainerRuntimeAgentInstallCommand(baseURL string, token string) string {
	normalizedBaseURL := normalizeAgentInstallBaseURL(baseURL)
	installScriptURL := normalizedBaseURL + "/install.sh"
	command := fmt.Sprintf(`curl -fsSL %s | bash -s -- \
  --url %s \
  --enable-docker \
  --enable-host=false \
  --interval 30s`,
		posixShellQuote(installScriptURL), posixShellQuote(normalizedBaseURL))

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

func resolveConfigAgentInstallBaseURL(req *http.Request, cfg *config.Config) string {
	if cfg != nil {
		if agentConnectURL := strings.TrimSpace(cfg.AgentConnectURL); agentConnectURL != "" {
			return normalizeAgentInstallBaseURL(agentConnectURL)
		}
	}

	return resolveLoopbackAwarePublicBaseURL(req, cfg)
}
