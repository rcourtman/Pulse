package configapi

import (
	"fmt"
	"net/url"
	"strings"
)

type SetupScriptInstallArtifact struct {
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

type setupScriptInstallArtifact = SetupScriptInstallArtifact

func BuildSetupScriptCommand(scriptURL string, token string) string {
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

func BuildSetupScriptURL(baseURL string, installType string, host string, pulseURL string, backupPerms bool) string {
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
	return strings.TrimRight(strings.TrimSpace(baseURL), "/") + "/api/setup-script?" + query.Encode()
}

func BuildSetupScriptDownloadURL(baseURL string, installType string, host string, pulseURL string, backupPerms bool, setupToken string) string {
	downloadURL := BuildSetupScriptURL(baseURL, installType, host, pulseURL, backupPerms)
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

func BuildSetupScriptInstallArtifact(baseURL string, installType string, host string, pulseURL string, backupPerms bool, setupToken string, expiresAt int64) SetupScriptInstallArtifact {
	scriptURL := BuildSetupScriptURL(baseURL, installType, host, pulseURL, backupPerms)
	commandWithEnv := BuildSetupScriptCommand(scriptURL, setupToken)
	return SetupScriptInstallArtifact{
		Type:              strings.TrimSpace(installType),
		Host:              strings.TrimSpace(host),
		URL:               scriptURL,
		DownloadURL:       BuildSetupScriptDownloadURL(baseURL, installType, host, pulseURL, backupPerms, setupToken),
		ScriptFileName:    fmt.Sprintf("pulse-setup-%s.sh", strings.TrimSpace(installType)),
		Command:           commandWithEnv,
		CommandWithEnv:    commandWithEnv,
		CommandWithoutEnv: BuildSetupScriptCommand(scriptURL, ""),
		Expires:           expiresAt,
		SetupToken:        strings.TrimSpace(setupToken),
		TokenHint:         setupScriptTokenHint(setupToken),
	}
}

func buildSetupScriptInstallArtifact(baseURL string, installType string, host string, pulseURL string, backupPerms bool, setupToken string, expiresAt int64) setupScriptInstallArtifact {
	return BuildSetupScriptInstallArtifact(baseURL, installType, host, pulseURL, backupPerms, setupToken, expiresAt)
}

func buildSetupScriptFileName(installType string) string {
	return fmt.Sprintf("pulse-setup-%s.sh", strings.TrimSpace(installType))
}

func setupScriptTokenHint(token string) string {
	trimmed := strings.TrimSpace(token)
	if len(trimmed) <= 6 {
		return trimmed
	}
	return fmt.Sprintf("%s…%s", trimmed[:3], trimmed[len(trimmed)-3:])
}

func posixShellQuote(value string) string {
	escaped := strings.ReplaceAll(value, "'", `'"'"'`)
	return "'" + escaped + "'"
}
