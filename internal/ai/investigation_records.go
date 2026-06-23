package ai

import (
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
)

// BuildFindingInvestigationRecord converts the latest finding state and the
// latest investigation session into the durable record used by product-facing
// intelligence surfaces.
func BuildFindingInvestigationRecord(f *Finding, session *InvestigationSession) *aicontracts.InvestigationRecord {
	if f == nil {
		return nil
	}

	record := &aicontracts.InvestigationRecord{
		ID:        fallbackInvestigationRecordID(f.ID, ""),
		FindingID: strings.TrimSpace(f.ID),
		SessionID: strings.TrimSpace(f.InvestigationSessionID),
		Subject: aicontracts.InvestigationRecordSubject{
			ResourceID:   strings.TrimSpace(f.ResourceID),
			ResourceName: strings.TrimSpace(f.ResourceName),
			ResourceType: strings.TrimSpace(f.ResourceType),
			Node:         strings.TrimSpace(f.Node),
		},
		Trigger: aicontracts.InvestigationRecordTrigger{
			FindingKey:  strings.TrimSpace(f.Key),
			Source:      strings.TrimSpace(f.Source),
			Severity:    strings.TrimSpace(string(f.Severity)),
			Category:    strings.TrimSpace(string(f.Category)),
			Title:       strings.TrimSpace(f.Title),
			DetectedAt:  f.DetectedAt,
			Description: strings.TrimSpace(f.Description),
			Cause:       strings.TrimSpace(f.FailureCause),
		},
		Status:            aicontracts.InvestigationStatus(strings.TrimSpace(f.InvestigationStatus)),
		Outcome:           aicontracts.InvestigationOutcome(strings.TrimSpace(f.InvestigationOutcome)),
		Evidence:          investigationRecordEvidenceForFinding(f),
		Conclusion:        strings.TrimSpace(f.Description),
		Impact:            strings.TrimSpace(f.Impact),
		RecommendedAction: strings.TrimSpace(f.Recommendation),
		Verification:      investigationRecordVerificationNotes(aicontracts.InvestigationOutcome(strings.TrimSpace(f.InvestigationOutcome))),
		ToolsUsed:         []string{},
	}

	if session != nil {
		normalized := session.NormalizeCollections()
		record.ID = fallbackInvestigationRecordID(f.ID, normalized.ID)
		record.SessionID = firstNonEmpty(normalized.SessionID, f.InvestigationSessionID, normalized.ID)
		if normalized.FindingID != "" {
			record.FindingID = strings.TrimSpace(normalized.FindingID)
		}
		if normalized.Status != "" {
			record.Status = normalized.Status
		}
		if normalized.Outcome != "" {
			record.Outcome = normalized.Outcome
		}
		record.StartedAt = normalized.StartedAt
		record.CompletedAt = normalized.CompletedAt
		record.ToolsUsed = uniqueNonEmptyStrings(normalized.ToolsUsed)
		record.ApprovalID = strings.TrimSpace(normalized.ApprovalID)
		record.Error = strings.TrimSpace(normalized.Error)
		if summary := strings.TrimSpace(normalized.Summary); summary != "" {
			record.Conclusion = summary
		}
		record.Evidence = append(record.Evidence, investigationRecordEvidenceIDs(normalized.EvidenceIDs)...)
		if normalized.ProposedFix != nil {
			record.ProposedFix = investigationRecordFixFromSession(normalized.ProposedFix)
			if record.RecommendedAction == "" {
				record.RecommendedAction = strings.TrimSpace(normalized.ProposedFix.Description)
			}
		}
		record.Verification = investigationRecordVerificationNotes(record.Outcome)
	}

	if record.StartedAt.IsZero() {
		record.StartedAt = firstNonZeroTimePtr(f.LastInvestigatedAt, f.DetectedAt)
	}
	if record.CompletedAt == nil && f.LastInvestigatedAt != nil && investigationRecordStatusIsTerminal(record.Status) {
		completedAt := *f.LastInvestigatedAt
		record.CompletedAt = &completedAt
	}
	if record.RecommendedAction == "" && record.ProposedFix != nil {
		record.RecommendedAction = strings.TrimSpace(record.ProposedFix.Description)
	}
	record.Confidence = deriveInvestigationRecordConfidence(record)

	normalized := record.NormalizeCollections()
	return &normalized
}

func fallbackInvestigationRecordID(findingID, sessionID string) string {
	findingID = strings.TrimSpace(findingID)
	sessionID = strings.TrimSpace(sessionID)
	if sessionID != "" {
		return sessionID
	}
	if findingID != "" {
		return fmt.Sprintf("%s:investigation", findingID)
	}
	return fmt.Sprintf("investigation:%d", time.Now().UnixNano())
}

func investigationRecordEvidenceForFinding(f *Finding) []aicontracts.InvestigationRecordEvidence {
	if f == nil {
		return []aicontracts.InvestigationRecordEvidence{}
	}
	evidence := strings.TrimSpace(f.Evidence)
	if evidence == "" {
		return []aicontracts.InvestigationRecordEvidence{}
	}
	return []aicontracts.InvestigationRecordEvidence{
		{
			Kind:    "finding_evidence",
			Summary: evidence,
		},
	}
}

func investigationRecordEvidenceIDs(ids []string) []aicontracts.InvestigationRecordEvidence {
	result := make([]aicontracts.InvestigationRecordEvidence, 0, len(ids))
	for _, id := range uniqueNonEmptyStrings(ids) {
		result = append(result, aicontracts.InvestigationRecordEvidence{
			ID:   id,
			Kind: "investigation_evidence",
		})
	}
	return result
}

func investigationRecordFixFromSession(fix *aicontracts.Fix) *aicontracts.InvestigationRecordFix {
	if fix == nil {
		return nil
	}
	normalized := fix.NormalizeCollections()
	recordFix := aicontracts.InvestigationRecordFix{
		ID:          strings.TrimSpace(normalized.ID),
		Description: strings.TrimSpace(normalized.Description),
		Commands:    uniqueNonEmptyStrings(normalized.Commands),
		RiskLevel:   strings.TrimSpace(normalized.RiskLevel),
		Destructive: normalized.Destructive,
		TargetHost:  strings.TrimSpace(normalized.TargetHost),
		Rationale:   strings.TrimSpace(normalized.Rationale),
	}.NormalizeCollections()
	return &recordFix
}

// AggregatePlanRollbackSteps lifts the rollback strings authored on a
// remediation plan's steps into a flat, deduplicated slice suitable for the
// record-level InvestigationRecord.Rollback field. Empty rollback steps are
// dropped, and nil plans return an empty slice. The caller is responsible
// for calling this only when a remediation plan exists for the finding.
func AggregatePlanRollbackSteps(plan *aicontracts.RemediationPlan) []string {
	if plan == nil {
		return []string{}
	}
	steps := make([]string, 0, len(plan.Steps))
	for _, step := range plan.Steps {
		steps = append(steps, step.Rollback)
	}
	return uniqueNonEmptyStrings(steps)
}

func investigationRecordVerificationNotes(outcome aicontracts.InvestigationOutcome) []string {
	switch outcome {
	case aicontracts.OutcomeResolved, aicontracts.OutcomeFixVerified:
		return []string{"Patrol verified the finding is resolved."}
	case aicontracts.OutcomeFixVerificationFailed:
		return []string{"Patrol verification found the issue is still present."}
	case aicontracts.OutcomeFixVerificationUnknown:
		return []string{"Patrol could not conclusively verify remediation."}
	case aicontracts.OutcomeFixFailed:
		return []string{"Patrol attempted remediation, but the fix did not complete successfully."}
	case aicontracts.OutcomeFixRejected:
		return []string{"The operator rejected the proposed fix before execution."}
	default:
		return []string{}
	}
}

func investigationRecordStatusIsTerminal(status aicontracts.InvestigationStatus) bool {
	switch status {
	case aicontracts.InvestigationStatusCompleted,
		aicontracts.InvestigationStatusFailed,
		aicontracts.InvestigationStatusNeedsAttention:
		return true
	default:
		return false
	}
}

func deriveInvestigationRecordConfidence(record *aicontracts.InvestigationRecord) aicontracts.InvestigationRecordConfidence {
	if record == nil {
		return aicontracts.InvestigationRecordConfidenceLow
	}
	switch record.Outcome {
	case aicontracts.OutcomeResolved, aicontracts.OutcomeFixVerified:
		return aicontracts.InvestigationRecordConfidenceHigh
	case aicontracts.OutcomeFixQueued, aicontracts.OutcomeFixExecuted, aicontracts.OutcomeCannotFix, aicontracts.OutcomeNeedsAttention:
		return aicontracts.InvestigationRecordConfidenceMedium
	case aicontracts.OutcomeFixRejected, aicontracts.OutcomeFixFailed, aicontracts.OutcomeFixVerificationFailed, aicontracts.OutcomeFixVerificationUnknown, aicontracts.OutcomeTimedOut:
		return aicontracts.InvestigationRecordConfidenceLow
	}
	if strings.TrimSpace(record.Conclusion) != "" || len(record.Evidence) > 0 {
		return aicontracts.InvestigationRecordConfidenceMedium
	}
	return aicontracts.InvestigationRecordConfidenceLow
}

func uniqueNonEmptyStrings(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func firstNonZeroTimePtr(value *time.Time, fallback time.Time) time.Time {
	if value != nil && !value.IsZero() {
		return *value
	}
	return fallback
}
