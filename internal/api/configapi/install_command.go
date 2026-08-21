package configapi

import (
	"fmt"
	"strings"
)

type AgentInstallCommandOptions struct {
	BaseURL            string
	Token              string
	InstallType        string
	IncludeInstallType bool
	EnableCommands     bool
	Insecure           bool
}

type agentInstallCommandOptions = AgentInstallCommandOptions

func BuildProxmoxAgentInstallCommand(opts AgentInstallCommandOptions) string {
	baseURL := strings.TrimRight(strings.TrimSpace(opts.BaseURL), "/")
	installScriptURL := baseURL + "/install.sh"
	curlFlags := "-fsSL"
	if opts.Insecure {
		curlFlags = "-kfsSL"
	}
	token := strings.TrimSpace(opts.Token)
	tokenSetup, tokenArg, tokenCleanup := "", "", ""
	if token != "" {
		tokenSetup = fmt.Sprintf(`token_file=$(mktemp) && chmod 600 "$token_file" && printf %%s %s > "$token_file" && `, posixShellQuote(token))
		tokenArg = " \\\n  --token-file \"$token_file\""
		tokenCleanup = `; rc=$?; rm -f "$token_file"; exit $rc`
	}
	command := fmt.Sprintf("%scurl %s %s | bash -s -- \\\n  --url %s \\\n  --enable-proxmox", tokenSetup, curlFlags, posixShellQuote(installScriptURL), posixShellQuote(baseURL))
	command += tokenArg
	if opts.Insecure || strings.HasPrefix(strings.ToLower(baseURL), "http://") {
		command += " \\\n  --insecure"
	}
	if opts.IncludeInstallType {
		command += fmt.Sprintf(" \\\n  --proxmox-type %s", posixShellQuote(opts.InstallType))
	}
	if opts.EnableCommands {
		command += " \\\n  --enable-commands"
	}
	return withPrivilegeEscalation(command) + tokenCleanup
}

func buildProxmoxAgentInstallCommand(opts agentInstallCommandOptions) string {
	return BuildProxmoxAgentInstallCommand(opts)
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
