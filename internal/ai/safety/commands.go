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

// IsBlockedCommand checks if a command contains any blocked pattern (case-insensitive).
// Used by the remediation engine to reject plans containing dangerous commands.
func IsBlockedCommand(command string) bool {
	if command == "" {
		return false
	}
	cmdLower := strings.ToLower(command)
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
