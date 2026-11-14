package hostagent

import "strings"

// commandSearchPath is prepended to PATH for Unix commands so container and
// cluster CLIs installed outside a service's minimal PATH are found.
const commandSearchPath = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

// commandEnvironment builds the environment for commands Pulse sends to the
// agent. Commands keep the host's runtime settings (PATH, locale, container
// runtime and cluster endpoints, proxies) but never the agent's own
// configuration: an agent configured through its environment holds its API
// token in PULSE_TOKEN, and no command needs it. Commands that run without an
// approval or a trusted Pulse-internal origin (the AI's read-only path) also
// lose other credential-named variables; approved and trusted commands keep
// them, because an operator-approved database or cloud CLI may rely on them.
// The read-only path also drops variables that change how sh or grep behave
// (exported shell functions, shell startup files and options, GREP_OPTIONS)
// or point an approved program at another file or output (BLKID_FILE,
// APT_CONFIG, PS_FORMAT), so the command runs as the policy read it.
func commandEnvironment(environ []string, goos string, keepCredentials bool) []string {
	out := make([]string, 0, len(environ)+1)
	currentPath := ""
	for _, entry := range environ {
		name, value, found := strings.Cut(entry, "=")
		if found && name != "" && commandEnvironmentWithholds(name, keepCredentials) {
			continue
		}
		if goos != "windows" && name == "PATH" {
			currentPath = value
			continue
		}
		out = append(out, entry)
	}
	if goos != "windows" {
		out = append(out, "PATH="+commandSearchPath+":"+currentPath)
	}
	return out
}

func commandEnvironmentWithholds(name string, keepCredentials bool) bool {
	upper := strings.ToUpper(name)
	if strings.HasPrefix(upper, "PULSE_") {
		return true
	}
	if keepCredentials {
		return false
	}
	if commandEnvironmentChangesInterpretation(upper) {
		return true
	}
	for _, marker := range []string{"TOKEN", "SECRET", "PASSWORD", "PASSWD", "PASSPHRASE", "API_KEY", "APIKEY", "ACCESS_KEY", "PRIVATE_KEY", "CREDENTIAL"} {
		if strings.Contains(upper, marker) {
			return true
		}
	}
	return false
}

// commandEnvironmentChangesInterpretation reports variables that alter what a
// command line does rather than describe the host: bash imports exported
// functions (BASH_FUNC_name%%) that can stand in for a program name and reads
// SHELLOPTS, BASHOPTS, BASH_ENV and ENV; PS4 is expanded under xtrace; grep
// up to 2.20 (CentOS 7) prepends GREP_OPTIONS to its arguments; blkid and
// apt-cache read their cache and configuration paths from BLKID_FILE,
// BLKID_CONF and APT_CONFIG; procps ps takes its default columns and parsing
// mode from PS_FORMAT, PS_PERSONALITY and CMD_ENV (PS_FORMAT=environ prints
// every process's environment).
func commandEnvironmentChangesInterpretation(upper string) bool {
	if strings.HasPrefix(upper, "BASH_FUNC_") {
		return true
	}
	switch upper {
	case "GREP_OPTIONS", "BASH_ENV", "ENV", "SHELLOPTS", "BASHOPTS", "PS4",
		"BLKID_FILE", "BLKID_CONF", "APT_CONFIG", "PS_FORMAT", "PS_PERSONALITY", "CMD_ENV":
		return true
	}
	return false
}
