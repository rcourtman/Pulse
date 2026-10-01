package configapi

import (
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

// BuildProxmoxAgentInstallCommand never puts the credential in shell source.
// Token only selects whether private terminal input is required. The plaintext
// remains in the authenticated response's separate token field.
func BuildProxmoxAgentInstallCommand(opts AgentInstallCommandOptions) string {
	baseURL := strings.TrimRight(strings.TrimSpace(opts.BaseURL), "/")
	curlFlags := "-fsSL"
	if opts.Insecure {
		curlFlags = "-kfsSL"
	}
	trustArgs := ""
	if opts.Insecure || strings.HasPrefix(strings.ToLower(baseURL), "http://") {
		trustArgs = " --insecure"
	}
	args := " --url " + posixShellQuote(baseURL) + " --enable-proxmox"
	if opts.IncludeInstallType {
		args += " --proxmox-type " + posixShellQuote(opts.InstallType)
	}
	if opts.EnableCommands {
		args += " --enable-commands"
	}
	if strings.TrimSpace(opts.Token) != "" {
		args += ` --token-file "$token_file"`
	}
	args += trustArgs + " --non-interactive"
	return privateAgentBootstrapCommand(baseURL, curlFlags, trustArgs, args, strings.TrimSpace(opts.Token) != "")
}

// BuildContainerRuntimeAgentInstallCommand uses the same complete-download,
// preflight and private credential-entry boundary as the Proxmox installer.
// Token selects the prompt; its value is never included in the copied command.
func BuildContainerRuntimeAgentInstallCommand(baseURL, token string, enableHost bool) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	args := " --url " + posixShellQuote(baseURL) + " --enable-docker"
	if enableHost {
		args += " --enable-host"
	} else {
		args += " --enable-host=false"
	}
	args += " --interval 30s"
	needsToken := strings.TrimSpace(token) != ""
	if needsToken {
		args += ` --token-file "$token_file"`
	}
	trustArgs := ""
	if strings.HasPrefix(strings.ToLower(baseURL), "http://") {
		trustArgs = " --insecure"
	}
	args += trustArgs + " --non-interactive"
	return privateAgentBootstrapCommand(baseURL, "-fsSL", trustArgs, args, needsToken)
}

func privateAgentBootstrapCommand(baseURL, curlFlags, trustArgs, args string, needsToken bool) string {
	preflight := "bash \"$install_script\" --url " + posixShellQuote(baseURL) +
		" --preflight-only --output json --non-interactive" + trustArgs + ";"
	return privateBootstrapCommand(baseURL+"/install.sh", curlFlags, preflight,
		"bash \"$1\""+args+";", needsToken, "Pulse agent token")
}

// privateBootstrapCommand uses POSIX outer grammar for single-line paste hosts.
// The privileged Bash child owns both the silent input and its 0700/0600
// handoff. Neither sudo nor a child command receives the credential in argv or
// environment. Failed/partial downloads and preflight never reach that child.
func privateBootstrapCommand(scriptURL, curlFlags, preflight, run string, needsToken bool, prompt string) string {
	privileged := []string{"set +xv;", "set -eu;", "umask 077;"}
	if needsToken {
		privileged = append(privileged,
			`token_dir=$(mktemp -d /tmp/pulse-agent-bootstrap.XXXXXX);`,
			`token_file="$token_dir/token";`,
			`unset pulse_token pulse_discard tty_state;`,
			`cleanup() { unset pulse_token pulse_discard; if [ -n "${tty_state:-}" ]; then stty "$tty_state" </dev/tty 2>/dev/null || :; fi; rm -f -- "$token_file"; rmdir -- "$token_dir"; };`,
			`trap cleanup EXIT; trap 'exit 129' HUP; trap 'exit 130' INT; trap 'exit 143' TERM;`,
			// A normal canonical tty silently truncates long lines (4095 bytes on
			// Linux). Bounded character input preserves valid input and detects
			// overflow; keep echo off while draining the rest of an invalid line
			// so it cannot become a second command in the caller's shell history.
			`if ! tty_state=$(stty -g </dev/tty); then echo "A terminal is required to enter the token. No installation was attempted." >&2; exit 1; fi;`,
			`stty -echo </dev/tty;`,
			`if ! IFS= read -r -s -p `+posixShellQuote(prompt+" (paste at this prompt, not in the command): ")+` -n 4097 pulse_token </dev/tty; then echo "Token input was interrupted. No installation was attempted." >&2; exit 1; fi;`,
			`if [ "${#pulse_token}" -gt 4096 ]; then while IFS= read -r -s -n 4097 pulse_discard </dev/tty && [ "${#pulse_discard}" -eq 4097 ]; do :; done; unset pulse_discard; fi;`,
			`stty "$tty_state" </dev/tty; unset tty_state;`,
			`printf '\n' >/dev/tty;`,
			`if [ -z "$pulse_token" ] || [ "${#pulse_token}" -gt 4096 ]; then echo "A non-empty token of at most 4096 characters is required." >&2; exit 1; fi;`,
			`printf %s "$pulse_token" > "$token_file"; unset pulse_token;`,
		)
	}
	privileged = append(privileged, run)
	child := "bash -c " + posixShellQuote(strings.Join(privileged, " ")) + ` pulse-bootstrap "$install_script";`
	return strings.Join([]string{
		"(", "set +xv;", "set -eu;", "umask 077;",
		`if [ "$(id -u)" -eq 0 ]; then :; elif command -v sudo >/dev/null 2>&1; then sudo -v; else echo "Root privileges required. Run as root (su -) and retry." >&2; exit 1; fi;`,
		`bootstrap_dir=$(mktemp -d /tmp/pulse-bootstrap.XXXXXX);`,
		`install_script="$bootstrap_dir/install.sh";`,
		`cleanup() { rm -f -- "$install_script"; rmdir -- "$bootstrap_dir"; };`,
		`trap cleanup EXIT; trap 'exit 129' HUP; trap 'exit 130' INT; trap 'exit 143' TERM;`,
		"curl " + curlFlags + " " + posixShellQuote(strings.TrimSpace(scriptURL)) + ` -o "$install_script";`,
		preflight,
		`if [ "$(id -u)" -eq 0 ]; then ` + child + " else sudo " + child + " fi;",
		")",
	}, " ")
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
