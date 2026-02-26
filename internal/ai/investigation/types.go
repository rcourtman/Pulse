// Package investigation provides autonomous investigation and remediation for patrol findings.
package investigation

import (
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/safety"
	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
)

// ---------------------------------------------------------------------------
// Type aliases — these re-export types from pkg/aicontracts so that existing
// code importing "internal/ai/investigation" continues to compile unchanged.
// New code should import pkg/aicontracts directly.
// ---------------------------------------------------------------------------

// Finding is the canonical patrol finding shape.
type Finding = aicontracts.Finding

// InvestigationSession represents an AI investigation of a finding.
type InvestigationSession = aicontracts.InvestigationSession

// Fix represents a proposed remediation action.
type Fix = aicontracts.Fix

// InvestigationConfig holds configuration for investigations.
type InvestigationConfig = aicontracts.InvestigationConfig

// InvestigationStore is the interface for investigation session persistence.
type InvestigationStore = aicontracts.InvestigationStore

// Status represents the current state of an investigation.
type Status = aicontracts.InvestigationStatus

// Outcome represents the result of an investigation.
type Outcome = aicontracts.InvestigationOutcome

// Status constants.
const (
	StatusPending        = aicontracts.InvestigationStatusPending
	StatusRunning        = aicontracts.InvestigationStatusRunning
	StatusCompleted      = aicontracts.InvestigationStatusCompleted
	StatusFailed         = aicontracts.InvestigationStatusFailed
	StatusNeedsAttention = aicontracts.InvestigationStatusNeedsAttention
)

// Outcome constants.
const (
	OutcomeResolved               = aicontracts.OutcomeResolved
	OutcomeFixQueued              = aicontracts.OutcomeFixQueued
	OutcomeFixExecuted            = aicontracts.OutcomeFixExecuted
	OutcomeFixFailed              = aicontracts.OutcomeFixFailed
	OutcomeFixVerified            = aicontracts.OutcomeFixVerified
	OutcomeFixVerificationFailed  = aicontracts.OutcomeFixVerificationFailed
	OutcomeFixVerificationUnknown = aicontracts.OutcomeFixVerificationUnknown
	OutcomeNeedsAttention         = aicontracts.OutcomeNeedsAttention
	OutcomeCannotFix              = aicontracts.OutcomeCannotFix
	OutcomeTimedOut               = aicontracts.OutcomeTimedOut
)

// ErrVerificationUnknown indicates the verifier could not conclusively determine
// whether a fix resolved the underlying issue.
var ErrVerificationUnknown = aicontracts.ErrVerificationUnknown

// DefaultConfig returns the default investigation configuration.
func DefaultConfig() InvestigationConfig {
	return aicontracts.DefaultInvestigationConfig()
}

// DestructivePatterns delegates to the shared safety package for the canonical
// list of destructive command patterns. All subsystems use the same list.
var DestructivePatterns = safety.DestructivePatterns

// IsDestructive checks if a command matches known destructive patterns.
// Delegates to the shared safety package.
func IsDestructive(command string) bool {
	return safety.IsDestructiveCommand(command)
}

// containsPattern checks if a command contains a pattern (case-insensitive partial match)
func containsPattern(command, pattern string) bool {
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
