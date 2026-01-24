// Package safety provides shared safety utilities for AI operations
package safety

import (
	"strings"
)

// ReadOnlyPatterns defines commands that are safe to execute without approval
// These commands only read data and cannot modify system state
var ReadOnlyPatterns = []string{
	// File/system inspection
	"cat ",
	"head ",
	"tail ",
	"less ",
	"more ",
	"ls ",
	"ls",
	"ll ",
	"ll",
	"dir ",
	"find ",
	"locate ",
	"which ",
	"whereis ",
	"file ",
	"stat ",
	"wc ",

	// System info
	"df ",
	"df",
	"du ",
	"free",
	"free ",
	"uptime",
	"uname ",
	"uname",
	"hostname",
	"whoami",
	"id ",
	"id",
	"date",
	"env",
	"printenv",
	"lscpu",
	"lsmem",
	"lsblk",
	"lspci",
	"lsusb",
	"lsof",
	"dmidecode",
	"hwinfo",
	"inxi",

	// Process inspection
	"ps ",
	"ps",
	"top -bn1",
	"top -b -n1",
	"pgrep ",
	"pidof ",
	"pstree",

	// Network inspection
	"netstat",
	"ss ",
	"ss",
	"ip addr",
	"ip a",
	"ip link",
	"ip route",
	"ip r",
	"ifconfig",
	"arp ",
	"arp",
	"ping -c",
	"traceroute",
	"tracepath",
	"dig ",
	"nslookup ",
	"host ",
	"getent ",

	// Logs and journals
	"journalctl",
	"dmesg",
	"last",
	"lastlog",
	"who",
	"w",

	// Service status (read-only)
	"systemctl status",
	"systemctl is-active",
	"systemctl is-enabled",
	"systemctl list-units",
	"systemctl list-timers",
	"service status",
	"service --status-all",

	// Proxmox read-only
	"pct list",
	"pct status",
	"pct config",
	"pct df",
	"pct fstrim",
	"qm list",
	"qm status",
	"qm config",
	"qm guest cmd",
	"pvesh get",
	"pvecm status",
	"pvecm nodes",
	"pveversion",
	"pvesm status",
	"zpool status",
	"zpool list",
	"zfs list",
	"zfs get",

	// Docker read-only
	"docker ps",
	"docker images",
	"docker logs",
	"docker inspect",
	"docker stats",
	"docker top",
	"docker port",
	"docker network ls",
	"docker network inspect",
	"docker volume ls",
	"docker volume inspect",
	"docker info",
	"docker version",
	"docker-compose ps",
	"docker compose ps",

	// Kubernetes read-only
	"kubectl get",
	"kubectl describe",
	"kubectl logs",
	"kubectl top",
	"kubectl cluster-info",
	"kubectl config view",
	"kubectl api-resources",
	"kubectl explain",

	// Package info (read-only)
	"apt list",
	"apt show",
	"apt-cache",
	"dpkg -l",
	"dpkg --list",
	"dpkg -s",
	"rpm -q",
	"rpm -qa",
	"yum list",
	"dnf list",
	"pacman -Q",
	"apk list",

	// Hardware/temperature
	"sensors",
	"hddtemp",
	"smartctl",
	"nvme list",
	"nvme smart-log",
	"mdadm --detail",
	"cat /proc/",
	"cat /sys/",
}

// IsReadOnlyCommand checks if a command is safe to execute without approval
// Returns true for diagnostic/inspection commands that cannot modify system state
func IsReadOnlyCommand(command string) bool {
	normalizedCmd := strings.TrimSpace(strings.ToLower(command))

	if normalizedCmd == "" {
		return false
	}

	// Check for piped commands FIRST - only safe if ALL parts are read-only
	if strings.Contains(command, "|") {
		parts := strings.Split(command, "|")
		for _, part := range parts {
			partCmd := strings.TrimSpace(part)
			// Common safe pipe commands
			if isSafePipeCommand(partCmd) {
				continue
			}
			// If any part is not read-only, the whole command is not read-only
			if !isReadOnlyPart(partCmd) {
				return false
			}
		}
		return true
	}

	// Single command - check against patterns
	for _, pattern := range ReadOnlyPatterns {
		patternLower := strings.ToLower(pattern)
		if strings.HasPrefix(normalizedCmd, patternLower) {
			return true
		}
	}

	return false
}

// isReadOnlyPart checks if a single command part is read-only
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

// isSafePipeCommand checks for common safe pipe targets
func isSafePipeCommand(cmd string) bool {
	safePipePatterns := []string{
		"grep", "egrep", "fgrep",
		"awk", "sed", // sed without -i is read-only
		"sort", "uniq", "wc",
		"head", "tail",
		"cut", "tr",
		"less", "more",
		"jq", "yq",
		"column", "tee /dev/null",
		"xargs echo", // echo is safe
	}

	normalizedCmd := strings.TrimSpace(strings.ToLower(cmd))
	for _, pattern := range safePipePatterns {
		if strings.HasPrefix(normalizedCmd, pattern) {
			// Special case: sed -i is NOT safe
			if strings.HasPrefix(normalizedCmd, "sed") && strings.Contains(normalizedCmd, "-i") {
				return false
			}
			return true
		}
	}
	return false
}
