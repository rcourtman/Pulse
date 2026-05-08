// Package aicontracts defines the shared types, interfaces, and constants for
// the AI investigation and remediation subsystems. These types live in pkg/ so
// that both the OSS binary (which uses them as interfaces) and the enterprise
// binary (which provides concrete implementations) can import them without
// hitting Go's internal/ visibility constraint.
//
// This package contains ONLY types and constants — zero business logic.
package aicontracts

import (
	"errors"
	"time"
)

// ---------------------------------------------------------------------------
// Investigation status
// ---------------------------------------------------------------------------

// InvestigationStatus represents the current state of an investigation.
type InvestigationStatus string

const (
	InvestigationStatusPending        InvestigationStatus = "pending"
	InvestigationStatusRunning        InvestigationStatus = "running"
	InvestigationStatusCompleted      InvestigationStatus = "completed"
	InvestigationStatusFailed         InvestigationStatus = "failed"
	InvestigationStatusNeedsAttention InvestigationStatus = "needs_attention"
)

// ---------------------------------------------------------------------------
// Investigation outcome
// ---------------------------------------------------------------------------

// InvestigationOutcome represents the result of an investigation.
type InvestigationOutcome string

const (
	OutcomeResolved               InvestigationOutcome = "resolved"
	OutcomeFixQueued              InvestigationOutcome = "fix_queued"
	OutcomeFixExecuted            InvestigationOutcome = "fix_executed"
	OutcomeFixFailed              InvestigationOutcome = "fix_failed"
	OutcomeFixVerified            InvestigationOutcome = "fix_verified"
	OutcomeFixVerificationFailed  InvestigationOutcome = "fix_verification_failed"
	OutcomeFixVerificationUnknown InvestigationOutcome = "fix_verification_unknown"
	OutcomeNeedsAttention         InvestigationOutcome = "needs_attention"
	OutcomeCannotFix              InvestigationOutcome = "cannot_fix"
	OutcomeTimedOut               InvestigationOutcome = "timed_out"
)

// ErrVerificationUnknown indicates the verifier could not conclusively determine
// whether a fix resolved the underlying issue. Callers may treat this as a
// distinct outcome from "verification failed" (issue persists).
var ErrVerificationUnknown = errors.New("verification inconclusive")

// ---------------------------------------------------------------------------
// Finding
// ---------------------------------------------------------------------------

// Finding represents a patrol finding with investigation metadata.
// This is the canonical finding shape shared between patrol and investigation.
type Finding struct {
	ID                     string     `json:"id"`
	Key                    string     `json:"key,omitempty"`
	Severity               string     `json:"severity"`
	Category               string     `json:"category"`
	ResourceID             string     `json:"resource_id"`
	ResourceName           string     `json:"resource_name"`
	ResourceType           string     `json:"resource_type"`
	Title                  string     `json:"title"`
	Description            string     `json:"description"`
	Recommendation         string     `json:"recommendation,omitempty"`
	Evidence               string     `json:"evidence,omitempty"`
	InvestigationSessionID string     `json:"investigation_session_id,omitempty"`
	InvestigationStatus    string     `json:"investigation_status,omitempty"`
	InvestigationOutcome   string     `json:"investigation_outcome,omitempty"`
	LastInvestigatedAt     *time.Time `json:"last_investigated_at,omitempty"`
	InvestigationAttempts  int        `json:"investigation_attempts"`
}

// ---------------------------------------------------------------------------
// Investigation session
// ---------------------------------------------------------------------------

// InvestigationSession represents an AI investigation of a finding.
type InvestigationSession struct {
	ID             string               `json:"id"`
	FindingID      string               `json:"finding_id"`
	SessionID      string               `json:"session_id"` // Chat session ID
	Status         InvestigationStatus  `json:"status"`
	StartedAt      time.Time            `json:"started_at"`
	CompletedAt    *time.Time           `json:"completed_at,omitempty"`
	TurnCount      int                  `json:"turn_count"`
	Outcome        InvestigationOutcome `json:"outcome,omitempty"`
	ProposedFix    *Fix                 `json:"proposed_fix,omitempty"`
	ApprovalID     string               `json:"approval_id,omitempty"`
	ToolsAvailable []string             `json:"tools_available"`
	ToolsUsed      []string             `json:"tools_used"`
	EvidenceIDs    []string             `json:"evidence_ids"`
	Summary        string               `json:"summary,omitempty"`
	Error          string               `json:"error,omitempty"`
}

func EmptyInvestigationSession() InvestigationSession {
	return InvestigationSession{}.NormalizeCollections()
}

func (s InvestigationSession) NormalizeCollections() InvestigationSession {
	if s.ToolsAvailable == nil {
		s.ToolsAvailable = []string{}
	}
	if s.ToolsUsed == nil {
		s.ToolsUsed = []string{}
	}
	if s.EvidenceIDs == nil {
		s.EvidenceIDs = []string{}
	}
	if s.ProposedFix != nil {
		normalizedFix := s.ProposedFix.NormalizeCollections()
		s.ProposedFix = &normalizedFix
	}
	return s
}

// ---------------------------------------------------------------------------
// Investigation record
// ---------------------------------------------------------------------------

// InvestigationRecordConfidence is the confidence level for a durable
// investigation record.
type InvestigationRecordConfidence string

const (
	InvestigationRecordConfidenceLow    InvestigationRecordConfidence = "low"
	InvestigationRecordConfidenceMedium InvestigationRecordConfidence = "medium"
	InvestigationRecordConfidenceHigh   InvestigationRecordConfidence = "high"
)

// InvestigationRecord is the durable product-facing summary of a Patrol
// investigation. It is intentionally separate from InvestigationSession:
// sessions are execution details, while records are the stable context that
// Patrol, Assistant, unified findings, persistence, and audit surfaces can share.
type InvestigationRecord struct {
	ID                string                        `json:"id"`
	FindingID         string                        `json:"finding_id"`
	SessionID         string                        `json:"session_id,omitempty"`
	Subject           InvestigationRecordSubject    `json:"subject"`
	Trigger           InvestigationRecordTrigger    `json:"trigger"`
	Status            InvestigationStatus           `json:"status"`
	Outcome           InvestigationOutcome          `json:"outcome,omitempty"`
	Confidence        InvestigationRecordConfidence `json:"confidence,omitempty"`
	Evidence          []InvestigationRecordEvidence `json:"evidence"`
	Conclusion        string                        `json:"conclusion,omitempty"`
	Impact            string                        `json:"impact,omitempty"`
	RecommendedAction string                        `json:"recommended_action,omitempty"`
	ProposedFix       *InvestigationRecordFix       `json:"proposed_fix,omitempty"`
	Verification      []string                      `json:"verification"`
	Rollback          []string                      `json:"rollback"`
	ToolsUsed         []string                      `json:"tools_used"`
	StartedAt         time.Time                     `json:"started_at"`
	CompletedAt       *time.Time                    `json:"completed_at,omitempty"`
	ApprovalID        string                        `json:"approval_id,omitempty"`
	Error             string                        `json:"error,omitempty"`
}

// InvestigationRecordSubject identifies the infrastructure object under
// investigation.
type InvestigationRecordSubject struct {
	ResourceID   string `json:"resource_id"`
	ResourceName string `json:"resource_name,omitempty"`
	ResourceType string `json:"resource_type,omitempty"`
	Node         string `json:"node,omitempty"`
}

// InvestigationRecordTrigger captures the Patrol finding that caused the
// investigation to run.
type InvestigationRecordTrigger struct {
	FindingKey  string    `json:"finding_key,omitempty"`
	Source      string    `json:"source,omitempty"`
	Severity    string    `json:"severity,omitempty"`
	Category    string    `json:"category,omitempty"`
	Title       string    `json:"title,omitempty"`
	DetectedAt  time.Time `json:"detected_at"`
	Description string    `json:"description,omitempty"`
	Cause       string    `json:"cause,omitempty"`
}

// InvestigationRecordEvidence points to evidence Patrol used or generated
// during investigation.
type InvestigationRecordEvidence struct {
	ID      string `json:"id,omitempty"`
	Kind    string `json:"kind"`
	Summary string `json:"summary,omitempty"`
}

// InvestigationRecordFix is the durable, product-facing version of a proposed
// remediation fix.
type InvestigationRecordFix struct {
	ID          string   `json:"id"`
	Description string   `json:"description"`
	Commands    []string `json:"commands"`
	RiskLevel   string   `json:"risk_level,omitempty"`
	Destructive bool     `json:"destructive"`
	TargetHost  string   `json:"target_host,omitempty"`
	Rationale   string   `json:"rationale,omitempty"`
}

func EmptyInvestigationRecord() InvestigationRecord {
	return InvestigationRecord{}.NormalizeCollections()
}

func (r InvestigationRecord) NormalizeCollections() InvestigationRecord {
	if r.Evidence == nil {
		r.Evidence = []InvestigationRecordEvidence{}
	}
	if r.Verification == nil {
		r.Verification = []string{}
	}
	if r.Rollback == nil {
		r.Rollback = []string{}
	}
	if r.ToolsUsed == nil {
		r.ToolsUsed = []string{}
	}
	if r.ProposedFix != nil {
		normalizedFix := r.ProposedFix.NormalizeCollections()
		r.ProposedFix = &normalizedFix
	}
	return r
}

func (f InvestigationRecordFix) NormalizeCollections() InvestigationRecordFix {
	if f.Commands == nil {
		f.Commands = []string{}
	}
	return f
}

// ---------------------------------------------------------------------------
// Fix
// ---------------------------------------------------------------------------

// Fix represents a proposed remediation action from an investigation.
type Fix struct {
	ID          string   `json:"id"`
	Description string   `json:"description"`
	Commands    []string `json:"commands"`
	RiskLevel   string   `json:"risk_level,omitempty"`
	Destructive bool     `json:"destructive"`
	TargetHost  string   `json:"target_host,omitempty"`
	Rationale   string   `json:"rationale,omitempty"`
}

func EmptyFix() Fix {
	return Fix{}.NormalizeCollections()
}

func (f Fix) NormalizeCollections() Fix {
	if f.Commands == nil {
		f.Commands = []string{}
	}
	return f
}

// ---------------------------------------------------------------------------
// Investigation config
// ---------------------------------------------------------------------------

// InvestigationConfig holds configuration for investigations.
type InvestigationConfig struct {
	MaxTurns                int
	Timeout                 time.Duration
	MaxConcurrent           int
	MaxAttemptsPerFinding   int
	CooldownDuration        time.Duration
	TimeoutCooldownDuration time.Duration
	VerificationDelay       time.Duration
}

// DefaultInvestigationConfig returns the default investigation configuration.
func DefaultInvestigationConfig() InvestigationConfig {
	return InvestigationConfig{
		MaxTurns:                15,
		Timeout:                 10 * time.Minute,
		MaxConcurrent:           3,
		MaxAttemptsPerFinding:   3,
		CooldownDuration:        1 * time.Hour,
		TimeoutCooldownDuration: 10 * time.Minute,
		VerificationDelay:       30 * time.Second,
	}
}
