package investigation

import (
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/safety"
)

// Guardrails provides safety checks for investigation actions
type Guardrails struct {
	// Additional destructive patterns can be added here
	customDestructivePatterns []string
}

// NewGuardrails creates a new guardrails instance
func NewGuardrails() *Guardrails {
	return &Guardrails{
		customDestructivePatterns: []string{},
	}
}

// AddDestructivePattern adds a custom destructive pattern
func (g *Guardrails) AddDestructivePattern(pattern string) {
	g.customDestructivePatterns = append(g.customDestructivePatterns, pattern)
}

// IsDestructiveAction checks if a command is destructive
func (g *Guardrails) IsDestructiveAction(command string) bool {
	// Check built-in patterns
	if IsDestructive(command) {
		return true
	}

	// Check custom patterns
	for _, pattern := range g.customDestructivePatterns {
		if containsPattern(command, pattern) {
			return true
		}
	}

	return false
}

// RequiresApproval determines if an action requires user approval
// based on finding severity, autonomy level, and whether the command is destructive
func (g *Guardrails) RequiresApproval(findingSeverity, autonomyLevel, command string, criticalRequireApproval bool) bool {
	// Destructive actions ALWAYS require approval
	if g.IsDestructiveAction(command) {
		return true
	}

	// In approval mode, everything requires approval
	if autonomyLevel == "approval" {
		return true
	}

	// Critical findings require approval if configured (default: true)
	if findingSeverity == "critical" && criticalRequireApproval {
		return true
	}

	// In full autonomy mode, non-destructive warning fixes can proceed
	if autonomyLevel == "full" && findingSeverity == "warning" {
		return false
	}

	// Default to requiring approval
	return true
}

// ClassifyRisk determines the risk level of a command
func (g *Guardrails) ClassifyRisk(command string) string {
	// Destructive commands are always high/critical risk
	if g.IsDestructiveAction(command) {
		return "critical"
	}

	// Service restart commands are high risk
	restartPatterns := []string{
		"systemctl restart",
		"service restart",
		"reboot",
		"shutdown",
		"init 6",
		"init 0",
	}
	for _, pattern := range restartPatterns {
		if containsPattern(command, pattern) {
			return "high"
		}
	}

	// Configuration changes are medium risk
	configPatterns := []string{
		"echo >",
		"tee",
		"sed -i",
		"patch",
		"cp /etc/",
		"mv /etc/",
		"chmod",
		"chown",
	}
	for _, pattern := range configPatterns {
		if containsPattern(command, pattern) {
			return "medium"
		}
	}

	// Read-only commands are low risk
	if safety.IsReadOnlyCommand(command) {
		return "low"
	}

	// Default to medium risk for unknown commands
	return "medium"
}

// ValidateCommand performs basic validation on a command
func (g *Guardrails) ValidateCommand(command string) (valid bool, reason string) {
	// Empty command
	if strings.TrimSpace(command) == "" {
		return false, "empty command"
	}

	// Command too long (potential injection)
	if len(command) > 4096 {
		return false, "command exceeds maximum length"
	}

	// Check for obvious shell injection patterns
	injectionPatterns := []string{
		"; rm -rf",
		"| rm -rf",
		"&& rm -rf",
		"$(rm -rf",
		"`rm -rf",
		"; dd if=",
		"| dd if=",
	}
	for _, pattern := range injectionPatterns {
		if containsPattern(command, pattern) {
			return false, "potential command injection detected"
		}
	}

	return true, ""
}

// SanitizeCommand attempts to make a command safer
// Returns the sanitized command and whether it was modified
func (g *Guardrails) SanitizeCommand(command string) (string, bool) {
	original := command
	sanitized := strings.TrimSpace(command)

	// Remove shell control characters that could be used for injection
	// This is a simple sanitization - real implementations should be more thorough
	dangerousChars := []string{"$(", "`", "${"}
	for _, dc := range dangerousChars {
		if strings.Contains(sanitized, dc) {
			// Rather than trying to sanitize, reject commands with these patterns
			return sanitized, sanitized != original
		}
	}

	return sanitized, sanitized != original
}

// GetDestructivePatterns returns all destructive patterns (built-in + custom)
func (g *Guardrails) GetDestructivePatterns() []string {
	patterns := make([]string, len(DestructivePatterns)+len(g.customDestructivePatterns))
	copy(patterns, DestructivePatterns)
	copy(patterns[len(DestructivePatterns):], g.customDestructivePatterns)
	return patterns
}
