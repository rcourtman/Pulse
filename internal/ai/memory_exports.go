package ai

import (
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/memory"
)

// ChangeDetector is an alias for memory.ChangeDetector
type ChangeDetector = memory.ChangeDetector

// ChangeDetectorConfig is an alias for memory.ChangeDetectorConfig
type ChangeDetectorConfig = memory.ChangeDetectorConfig

// Change is an alias for memory.Change
type Change = memory.Change

// ResourceSnapshot is an alias for memory.ResourceSnapshot
type ResourceSnapshot = memory.ResourceSnapshot

// ChangeType is an alias for memory.ChangeType
type ChangeType = memory.ChangeType

// RemediationLog is an alias for memory.RemediationLog
type RemediationLog = memory.RemediationLog

// RemediationLogConfig is an alias for memory.RemediationLogConfig
type RemediationLogConfig = memory.RemediationLogConfig

// RemediationRecord is an alias for memory.RemediationRecord
type RemediationRecord = memory.RemediationRecord

// Outcome is an alias for memory.Outcome
type Outcome = memory.Outcome

// Change type constants
const (
	ChangeCreated   = memory.ChangeCreated
	ChangeDeleted   = memory.ChangeDeleted
	ChangeConfig    = memory.ChangeConfig
	ChangeStatus    = memory.ChangeStatus
	ChangeMigrated  = memory.ChangeMigrated
	ChangeRestarted = memory.ChangeRestarted
	ChangeBackedUp  = memory.ChangeBackedUp
)

// Outcome constants
const (
	OutcomeResolved = memory.OutcomeResolved
	OutcomePartial  = memory.OutcomePartial
	OutcomeFailed   = memory.OutcomeFailed
	OutcomeUnknown  = memory.OutcomeUnknown
)

// NewChangeDetector creates a new change detector
func NewChangeDetector(cfg ChangeDetectorConfig) *ChangeDetector {
	return memory.NewChangeDetector(cfg)
}

// NewRemediationLog creates a new remediation log
func NewRemediationLog(cfg RemediationLogConfig) *RemediationLog {
	return memory.NewRemediationLog(cfg)
}
