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
	command := fmt.Sprintf("curl %s %s | bash -s -- \\\n  --url %s \\\n  --enable-proxmox", curlFlags, posixShellQuote(installScriptURL), posixShellQuote(baseURL))
	if opts.Insecure || strings.HasPrefix(strings.ToLower(baseURL), "http://") {
		command += " \\\n  --insecure"
	}
	if opts.IncludeInstallType {
		command += fmt.Sprintf(" \\\n  --proxmox-type %s", posixShellQuote(opts.InstallType))
	}
	if opts.EnableCommands {
		command += " \\\n  --enable-commands"
	}
	if token == "" {
		return withPrivilegeEscalation(command)
	}

	rootCommand := command + " \\\n  --token-file \"$token_file\""
	sudoCommand := strings.Replace(command, "| bash -s --", "| sudo bash -s --", 1) + " \\\n  --token-file \"$token_file\""
	return fmt.Sprintf(`(
  set -e
  token_dir=""
  cleanup() {
    if [ -n "${token_dir:-}" ]; then
      if [ "$(id -u)" -eq 0 ]; then
        rm -rf -- "$token_dir"
      elif command -v sudo >/dev/null 2>&1; then
        sudo rm -rf -- "$token_dir" >/dev/null 2>&1 || true
      fi
    fi
  }
  trap cleanup EXIT HUP INT TERM
  if [ "$(id -u)" -eq 0 ]; then
    token_dir=$(mktemp -d /tmp/pulse-agent-bootstrap.XXXXXX)
    token_file="$token_dir/token"
    umask 077
    printf %%s %s > "$token_file"
    %s
  elif command -v sudo >/dev/null 2>&1; then
    token_dir=$(sudo mktemp -d /tmp/pulse-agent-bootstrap.XXXXXX)
    token_file="$token_dir/token"
    printf %%s %s | sudo tee "$token_file" >/dev/null
    sudo chmod 0600 "$token_file"
    %s
  else
    echo "Root privileges required. Run as root (su -) and retry." >&2
    exit 1
  fi
)`, posixShellQuote(token), rootCommand, posixShellQuote(token), sudoCommand)
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
