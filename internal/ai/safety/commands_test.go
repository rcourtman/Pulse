package safety

import (
	"testing"
)

func TestIsBlockedCommand_AllPatterns(t *testing.T) {
	// Every pattern in BlockedCommands should be detected
	for _, pattern := range BlockedCommands {
		if !IsBlockedCommand(pattern) {
			t.Errorf("IsBlockedCommand(%q) = false, want true", pattern)
		}
	}
}

func TestIsBlockedCommand_CaseInsensitive(t *testing.T) {
	tests := []struct {
		command string
		want    bool
	}{
		{"RM -RF /tmp", true},
		{"Rm -Rf /data", true},
		{"drop database users", true},
		{"DROP TABLE sessions", true},
		{"Truncate table logs", true},
	}
	for _, tt := range tests {
		if got := IsBlockedCommand(tt.command); got != tt.want {
			t.Errorf("IsBlockedCommand(%q) = %v, want %v", tt.command, got, tt.want)
		}
	}
}

func TestIsBlockedCommand_SafeCommands(t *testing.T) {
	safeCommands := []string{
		"ls -la /tmp",
		"cat /var/log/syslog",
		"systemctl status nginx",
		"df -h",
		"ps aux",
		"docker ps",
		"zpool status",
		"zfs list",
		"pct list",
		"qm list",
		"free -m",
		"uptime",
		"journalctl -u nginx",
		"SELECT * FROM users",
	}

	for _, cmd := range safeCommands {
		if IsBlockedCommand(cmd) {
			t.Errorf("IsBlockedCommand(%q) = true, want false (safe command)", cmd)
		}
	}
}

func TestIsBlockedCommand_EmptyCommand(t *testing.T) {
	if IsBlockedCommand("") {
		t.Error("IsBlockedCommand(\"\") = true, want false")
	}
}

func TestIsBlockedCommand_EmbeddedPatterns(t *testing.T) {
	// Commands that contain blocked patterns as substrings
	tests := []struct {
		command string
		want    bool
	}{
		{"sudo rm -rf /var/cache/apt", true},
		{"bash -c 'dd if=/dev/zero of=/dev/sda'", true},
		{"echo y | mkfs.ext4 /dev/sdb1", true},
		{"zfs destroy tank/dataset", true},
		{"apt purge nginx", true},
		{"systemctl stop nginx", true},
		{"pkill -9 java", true},
	}
	for _, tt := range tests {
		if got := IsBlockedCommand(tt.command); got != tt.want {
			t.Errorf("IsBlockedCommand(%q) = %v, want %v", tt.command, got, tt.want)
		}
	}
}

func TestIsDestructiveCommand_DelegatesToBlocked(t *testing.T) {
	// IsDestructiveCommand should behave identically to IsBlockedCommand
	for _, pattern := range BlockedCommands {
		if !IsDestructiveCommand(pattern) {
			t.Errorf("IsDestructiveCommand(%q) = false, want true", pattern)
		}
	}
	if IsDestructiveCommand("ls -la") {
		t.Error("IsDestructiveCommand(\"ls -la\") = true, want false")
	}
}

func TestBlockedCommandsCount(t *testing.T) {
	// Ensure we have a reasonable number of patterns (the union should be >= both originals)
	// Investigation had 34 patterns, remediation had 17, union has more
	if len(BlockedCommands) < 34 {
		t.Errorf("BlockedCommands has %d patterns, expected at least 34 (investigation list size)", len(BlockedCommands))
	}
}

func TestDestructivePatternsIsBlockedCommands(t *testing.T) {
	// DestructivePatterns should be the same slice as BlockedCommands
	if len(DestructivePatterns) != len(BlockedCommands) {
		t.Errorf("DestructivePatterns has %d patterns, BlockedCommands has %d", len(DestructivePatterns), len(BlockedCommands))
	}
}
