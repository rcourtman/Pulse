package safety

import (
	"strings"
)

// BlockedCommands is the canonical list of command patterns that must never be
// executed by any automated system (investigation, remediation, or chat).
// This is the union of patterns from investigation/types.go and remediation/engine.go.
var BlockedCommands = []string{
	// File/disk destruction
	"rm -rf",
	"rm -r",
	"rm -f",
	"rmdir",
	"dd if=",
	"mkfs",
	"fdisk",
	"wipefs",
	"shred",
	// Disk/partition manipulation
	"> /dev/sd",
	"format",
	"parted",
	// Proxmox VM/container destruction
	"pct destroy",
	"qm destroy",
	"pvecm delnode",
	"zfs destroy",
	"zpool destroy",
	// Docker/container destruction
	"docker rm -f",
	"docker system prune",
	"docker volume rm",
	"docker image prune",
	"podman rm -f",
	// Package removal
	"apt remove",
	"apt purge",
	"apt autoremove",
	"yum remove",
	"dnf remove",
	"pacman -R",
	// Service disruption
	"systemctl stop",
	"systemctl disable",
	"service stop",
	"killall",
	"pkill",
	// Network disruption
	"iptables -F",
	"ip link delete",
	"ifdown",
	// System shutdown/reboot
	"shutdown",
	"poweroff",
	"reboot",
	"init 0",
	"init 6",
	// Database destruction
	"DROP DATABASE",
	"DROP TABLE",
	"TRUNCATE",
}

// DestructivePatterns is an alias for BlockedCommands for backward compatibility.
// Both investigation and remediation subsystems should use this single source of truth.
var DestructivePatterns = BlockedCommands

// normalizeCommandForCheck strips shell quoting, escape characters, and
// normalizes whitespace so that patterns like `'rm' -rf`, `\rm -rf`, or
// `rm\t-rf` are still matched against the blocked list.
func normalizeCommandForCheck(cmd string) string {
	// Remove common escape/quoting characters that could be used to bypass pattern matching
	replacer := strings.NewReplacer(
		"\\", "", // backslash escapes: \rm -> rm
		"'", "", // single quotes: 'rm' -> rm
		"\"", "", // double quotes: "rm" -> rm
		"`", "", // backticks: `rm` -> rm
	)
	result := replacer.Replace(cmd)

	// Normalize all whitespace (tabs, multiple spaces, etc.) to single spaces
	fields := strings.Fields(result)
	return strings.Join(fields, " ")
}

// IsBlockedCommand checks if a command contains any blocked pattern (case-insensitive).
// Used by the remediation engine to reject plans containing dangerous commands.
// The command is normalized to strip quoting/escaping before pattern matching
// to prevent bypass via 'rm' -rf, \rm -rf, etc.
func IsBlockedCommand(command string) bool {
	if command == "" {
		return false
	}
	cmdLower := strings.ToLower(normalizeCommandForCheck(command))
	for _, pattern := range BlockedCommands {
		if strings.Contains(cmdLower, strings.ToLower(pattern)) {
			return true
		}
	}
	return false
}

// IsDestructiveCommand checks if a command matches destructive patterns (case-insensitive).
// Used by the investigation guardrails to flag commands needing approval.
// This is functionally identical to IsBlockedCommand but named to match the
// investigation subsystem's terminology.
func IsDestructiveCommand(command string) bool {
	return IsBlockedCommand(command)
}
