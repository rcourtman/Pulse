// Package investigation provides autonomous investigation and remediation for patrol findings.
package investigation

import (
	"time"
)

// InvestigationSession represents an AI investigation of a finding
type InvestigationSession struct {
	ID          string     `json:"id"`
	FindingID   string     `json:"finding_id"`
	SessionID   string     `json:"session_id"` // Chat session ID
	Status      Status     `json:"status"`
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	TurnCount   int        `json:"turn_count"`        // Number of agentic turns used
	Outcome     Outcome    `json:"outcome,omitempty"` // Result of investigation
	ProposedFix *Fix       `json:"proposed_fix,omitempty"`
	ApprovalID  string     `json:"approval_id,omitempty"` // If fix is queued for approval
	Summary     string     `json:"summary,omitempty"`     // AI-generated summary
	Error       string     `json:"error,omitempty"`       // Error message if failed
}

// Status represents the current state of an investigation
type Status string

const (
	StatusPending        Status = "pending"
	StatusRunning        Status = "running"
	StatusCompleted      Status = "completed"
	StatusFailed         Status = "failed"
	StatusNeedsAttention Status = "needs_attention"
)

// Outcome represents the result of an investigation
type Outcome string

const (
	OutcomeResolved       Outcome = "resolved"
	OutcomeFixQueued      Outcome = "fix_queued"
	OutcomeFixExecuted    Outcome = "fix_executed" // Fix was auto-executed successfully
	OutcomeFixFailed      Outcome = "fix_failed"   // Fix was attempted but failed
	OutcomeNeedsAttention Outcome = "needs_attention"
	OutcomeCannotFix      Outcome = "cannot_fix"
)

// Fix represents a proposed remediation action
type Fix struct {
	ID          string   `json:"id"`
	Description string   `json:"description"`
	Commands    []string `json:"commands,omitempty"`    // Shell commands to execute
	RiskLevel   string   `json:"risk_level,omitempty"`  // "low", "medium", "high", "critical"
	Destructive bool     `json:"destructive"`           // Whether this is a destructive action
	TargetHost  string   `json:"target_host,omitempty"` // Host where commands should run
	Rationale   string   `json:"rationale,omitempty"`   // Why this fix was suggested
}

// DestructivePatterns contains patterns that indicate destructive commands
var DestructivePatterns = []string{
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
	// Proxmox VM/container destruction
	"pct destroy",
	"qm destroy",
	"pvecm delnode",
	"zfs destroy",
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
}

// InvestigationConfig holds configuration for investigations
type InvestigationConfig struct {
	MaxTurns                int           // Maximum agentic turns per investigation
	Timeout                 time.Duration // Maximum duration per investigation
	MaxConcurrent           int           // Maximum concurrent investigations
	MaxAttemptsPerFinding   int           // Maximum investigation attempts per finding
	CooldownDuration        time.Duration // Cooldown before re-investigating
	CriticalRequireApproval bool          // Critical findings always require approval
}

// DefaultConfig returns the default investigation configuration
func DefaultConfig() InvestigationConfig {
	return InvestigationConfig{
		MaxTurns:                15,
		Timeout:                 5 * time.Minute,
		MaxConcurrent:           3,
		MaxAttemptsPerFinding:   3,
		CooldownDuration:        1 * time.Hour,
		CriticalRequireApproval: true,
	}
}

// IsDestructive checks if a command matches known destructive patterns
func IsDestructive(command string) bool {
	for _, pattern := range DestructivePatterns {
		if containsPattern(command, pattern) {
			return true
		}
	}
	return false
}

// containsPattern checks if a command contains a pattern (case-insensitive partial match)
func containsPattern(command, pattern string) bool {
	// Simple substring match for now
	// Could be enhanced with regex for more precise matching
	return len(command) >= len(pattern) && contains(command, pattern)
}

// contains does a case-insensitive substring check
func contains(s, substr string) bool {
	sLower := toLower(s)
	substrLower := toLower(substr)
	return indexString(sLower, substrLower) >= 0
}

// toLower converts a string to lowercase
func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		result[i] = c
	}
	return string(result)
}

// indexString returns the index of substr in s, or -1 if not found
func indexString(s, substr string) int {
	if len(substr) == 0 {
		return 0
	}
	if len(substr) > len(s) {
		return -1
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
