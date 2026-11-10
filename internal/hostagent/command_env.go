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
	for _, marker := range []string{"TOKEN", "SECRET", "PASSWORD", "PASSWD", "PASSPHRASE", "API_KEY", "APIKEY", "ACCESS_KEY", "PRIVATE_KEY", "CREDENTIAL"} {
		if strings.Contains(upper, marker) {
			return true
		}
	}
	return false
}
