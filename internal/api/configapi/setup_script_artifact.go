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

// The legacy token argument is intentionally not interpolated. All command
// aliases now prompt privately; the separate SetupToken field is the reveal.
func BuildSetupScriptCommand(scriptURL string, _ string) string {
	return privateBootstrapCommand(scriptURL, "-fsSL", "",
		`PULSE_SETUP_TOKEN_FILE="$token_file" bash "$1";`, true, "Pulse setup token")
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

// Downloads contain no credential, including in the request URL or script.
func BuildSetupScriptDownloadURL(baseURL string, installType string, host string, pulseURL string, backupPerms bool, _ string) string {
	return BuildSetupScriptURL(baseURL, installType, host, pulseURL, backupPerms)
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
