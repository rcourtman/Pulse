// Package aicontracts safety utilities for AI operations.
// These pure functions are used by both the investigation orchestrator and
// remediation engine to classify and validate commands.
package aicontracts

import (
	"strings"
)

// BlockedCommands is the canonical list of command patterns that must never be
// executed by any automated system (investigation, remediation, or chat).
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
var DestructivePatterns = BlockedCommands

// normalizeCommandForCheck strips shell quoting, escape characters, and
// normalizes whitespace so that patterns like `'rm' -rf`, `\rm -rf`, or
// `rm\t-rf` are still matched against the blocked list.
func normalizeCommandForCheck(cmd string) string {
	replacer := strings.NewReplacer(
		"\\", "",
		"'", "",
		"\"", "",
		"`", "",
	)
	result := replacer.Replace(cmd)
	fields := strings.Fields(result)
	return strings.Join(fields, " ")
}

// IsBlockedCommand checks if a command contains any blocked pattern (case-insensitive).
// The command is normalized to strip quoting/escaping before pattern matching.
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
// Functionally identical to IsBlockedCommand.
func IsDestructiveCommand(command string) bool {
	return IsBlockedCommand(command)
}

// ReadOnlyPatterns defines commands that are safe to execute without approval.
var ReadOnlyPatterns = []string{
	// File/system inspection
	"cat ", "head ", "tail ", "less ", "more ",
	"ls ", "ls", "ll ", "ll", "dir ", "find ", "locate ",
	"which ", "whereis ", "file ", "stat ", "wc ",
	// System info
	"df ", "df", "du ", "free", "free ", "uptime",
	"uname ", "uname", "hostname", "whoami",
	"id ", "id", "date", "env", "printenv",
	"lscpu", "lsmem", "lsblk", "lspci", "lsusb", "lsof",
	"dmidecode", "hwinfo", "inxi",
	// Process inspection
	"ps ", "ps", "top -bn1", "top -b -n1", "pgrep ", "pidof ", "pstree",
	// Network inspection
	"netstat", "ss ", "ss",
	"ip addr", "ip a", "ip link", "ip route", "ip r",
	"ifconfig", "arp ", "arp",
	"ping -c", "traceroute", "tracepath",
	"dig ", "nslookup ", "host ", "getent ",
	// Logs and journals
	"journalctl", "dmesg", "last", "lastlog", "who", "w",
	// Service status (read-only)
	"systemctl status", "systemctl is-active", "systemctl is-enabled",
	"systemctl list-units", "systemctl list-timers",
	"service status", "service --status-all",
	// Proxmox read-only
	"pct list", "pct status", "pct config", "pct df", "pct fstrim",
	"qm list", "qm status", "qm config", "qm guest cmd",
	"pvesh get", "pvecm status", "pvecm nodes", "pveversion", "pvesm status",
	"zpool status", "zpool list", "zfs list", "zfs get",
	// Docker read-only
	"docker ps", "docker images", "docker logs", "docker inspect",
	"docker stats", "docker top", "docker port",
	"docker network ls", "docker network inspect",
	"docker volume ls", "docker volume inspect",
	"docker info", "docker version",
	"docker-compose ps", "docker compose ps",
	// Kubernetes read-only
	"kubectl get", "kubectl describe", "kubectl logs", "kubectl top",
	"kubectl cluster-info", "kubectl config view",
	"kubectl api-resources", "kubectl explain",
	// Package info (read-only)
	"apt list", "apt show", "apt-cache",
	"dpkg -l", "dpkg --list", "dpkg -s",
	"rpm -q", "rpm -qa", "yum list", "dnf list",
	"pacman -Q", "apk list",
	// Hardware/temperature
	"sensors", "hddtemp", "smartctl",
	"nvme list", "nvme smart-log",
	"mdadm --detail",
	"cat /proc/", "cat /sys/",
}

// containsShellChaining checks if a command contains shell operators that could
// chain a write operation after a read-only command.
func containsShellChaining(command string) bool {
	dangerousPatterns := []string{
		"&&", "||",
		"$(", ">(", "<(",
		";", "`",
		"\n",
	}
	for _, p := range dangerousPatterns {
		if strings.Contains(command, p) {
			return true
		}
	}
	if strings.Contains(command, "&") {
		return true
	}
	if strings.Contains(command, "(") {
		return true
	}
	return false
}

// IsReadOnlyCommand checks if a command is safe to execute without approval.
func IsReadOnlyCommand(command string) bool {
	normalizedCmd := strings.TrimSpace(strings.ToLower(command))
	if normalizedCmd == "" {
		return false
	}
	if containsShellChaining(command) {
		return false
	}
	if strings.Contains(command, "|") {
		parts := strings.Split(command, "|")
		for _, part := range parts {
			partCmd := strings.TrimSpace(part)
			if isSafePipeCommand(partCmd) {
				continue
			}
			if !isReadOnlyPart(partCmd) {
				return false
			}
		}
		return true
	}
	for _, pattern := range ReadOnlyPatterns {
		patternLower := strings.ToLower(pattern)
		if strings.HasPrefix(normalizedCmd, patternLower) {
			return true
		}
	}
	return false
}

func isReadOnlyPart(cmd string) bool {
	normalizedCmd := strings.TrimSpace(strings.ToLower(cmd))
	for _, pattern := range ReadOnlyPatterns {
		patternLower := strings.ToLower(pattern)
		if strings.HasPrefix(normalizedCmd, patternLower) {
			return true
		}
	}
	return false
}

func isSafePipeCommand(cmd string) bool {
	safePipePatterns := []string{
		"grep", "egrep", "fgrep",
		"awk", "sed",
		"sort", "uniq", "wc",
		"head", "tail",
		"cut", "tr",
		"less", "more",
		"jq", "yq",
		"column", "tee /dev/null",
		"xargs echo",
	}
	normalizedCmd := strings.TrimSpace(strings.ToLower(cmd))
	for _, pattern := range safePipePatterns {
		if strings.HasPrefix(normalizedCmd, pattern) {
			if strings.HasPrefix(normalizedCmd, "sed") && strings.Contains(normalizedCmd, "-i") {
				return false
			}
			return true
		}
	}
	return false
}
