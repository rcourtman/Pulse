package safety

import (
	"path/filepath"
	"strings"
)

// IsSensitivePath returns (true, reason) when a file path is likely to contain secrets.
// This is used to block AI-initiated reads/writes of high-risk locations (SSH keys, shadow files, etc.).
func IsSensitivePath(path string) (bool, string) {
	if path == "" {
		return false, ""
	}

	clean := filepath.Clean(path)
	lower := strings.ToLower(clean)

	// System account databases.
	switch lower {
	case "/etc/shadow", "/etc/gshadow", "/etc/sudoers":
		return true, "system credential file"
	}

	// SSH keys and configs.
	if strings.Contains(lower, "/.ssh/") {
		return true, "ssh key/config directory"
	}
	for _, name := range []string{"id_rsa", "id_ed25519", "authorized_keys", "known_hosts"} {
		if strings.HasSuffix(lower, "/"+name) {
			return true, "ssh key material"
		}
	}

	// Secrets mounts.
	for _, prefix := range []string{"/run/secrets/", "/var/run/secrets/", "/etc/secrets/", "/secrets/"} {
		if strings.HasPrefix(lower, prefix) {
			return true, "secrets directory"
		}
	}

	// Process environment frequently contains secrets.
	if strings.HasPrefix(lower, "/proc/") && strings.HasSuffix(lower, "/environ") {
		return true, "process environment file"
	}

	// Private key / certificate formats.
	for _, ext := range []string{".pem", ".key", ".p12", ".pfx"} {
		if strings.HasSuffix(lower, ext) {
			return true, "private key or certificate file"
		}
	}

	// Pulse AI encrypted configuration store.
	if strings.HasSuffix(lower, "/ai.enc") || strings.Contains(lower, "ai.enc") {
		return true, "pulse ai encrypted config store"
	}

	// Common dotfiles that may include tokens.
	for _, base := range []string{".env", ".npmrc", ".pypirc", ".netrc", ".aws/credentials"} {
		if strings.HasSuffix(lower, "/"+base) {
			return true, "credentials dotfile"
		}
	}

	return false, ""
}

// CommandTouchesSensitivePath is a best-effort heuristic for blocking command execution that
// obviously reads secrets. This is not a full shell parser; it intentionally catches only
// high-confidence cases.
func CommandTouchesSensitivePath(command string) (bool, string) {
	cmdLower := strings.ToLower(command)
	if cmdLower == "" {
		return false, ""
	}

	// High-confidence substrings.
	for _, s := range []string{"/etc/shadow", "/etc/gshadow", "/etc/sudoers", "/run/secrets/", "/var/run/secrets/", "/.ssh/", "ai.enc"} {
		if strings.Contains(cmdLower, s) {
			return true, "references sensitive path"
		}
	}
	if strings.Contains(cmdLower, "/proc/") && strings.Contains(cmdLower, "environ") {
		return true, "references process environment file"
	}
	return false, ""
}
