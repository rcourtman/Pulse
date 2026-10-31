package api

import unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"

// AgentActionResultSummary is a content-free view of validated action truth.
// It is deliberately not ActionResultV2: withholding evidence must not leave
// an apparently complete canonical envelope with missing or rewritten digests.
// The full result remains behind the existing command-disclosure authority.
type AgentActionResultSummary struct {
	DetailsRedacted bool                           `json:"detailsRedacted"`
	Execution       AgentActionExecutionSummary    `json:"execution"`
	Verification    AgentActionVerificationSummary `json:"verification"`
	Compensation    AgentActionCompensationSummary `json:"compensation"`
}

type AgentActionExecutionSummary struct {
	Status unified.ActionExecutionStatus `json:"status"`
}

type AgentActionVerificationSummary struct {
	Status        unified.ActionVerificationStatus `json:"status"`
	EvidenceClass unified.ActionEvidenceClass      `json:"evidenceClass"`
}

type AgentActionCompensationSummary struct {
	Support      unified.ActionCompensationSupport `json:"support"`
	Status       unified.ActionCompensationStatus  `json:"status"`
	Execution    *AgentActionExecutionSummary      `json:"execution,omitempty"`
	Verification *AgentActionVerificationSummary   `json:"verification,omitempty"`
}

func summarizeAgentActionResult(result *unified.ActionResultV2) *AgentActionResultSummary {
	if result == nil {
		return nil
	}
	// Normalize a private copy before projecting the closed status vocabulary.
	// Normalization owns nested slices; never let one subscriber rewrite the
	// evidence another subscriber (or the durable store) will receive.
	copy := *result
	copy.Verification = copyAgentActionVerification(result.Verification)
	copy.Compensation.Evidence = copyAgentActionEvidence(result.Compensation.Evidence)
	if result.Compensation.Verification != nil {
		verification := copyAgentActionVerification(*result.Compensation.Verification)
		copy.Compensation.Verification = &verification
	}
	canonical := unified.CanonicalActionResultV2(unified.ActionAuditRecord{
		Result: &unified.ExecutionResult{ActionResultV2: &copy},
	})
	summary := &AgentActionResultSummary{
		DetailsRedacted: true,
		Execution:       AgentActionExecutionSummary{Status: canonical.Execution.Status},
		Verification: AgentActionVerificationSummary{
			Status: canonical.Verification.Status, EvidenceClass: canonical.Verification.EvidenceClass,
		},
		Compensation: AgentActionCompensationSummary{
			Support: canonical.Compensation.Support, Status: canonical.Compensation.Status,
		},
	}
	if execution := canonical.Compensation.Execution; execution != nil {
		summary.Compensation.Execution = &AgentActionExecutionSummary{Status: execution.Status}
	}
	if verification := canonical.Compensation.Verification; verification != nil {
		summary.Compensation.Verification = &AgentActionVerificationSummary{
			Status: verification.Status, EvidenceClass: verification.EvidenceClass,
		}
	}
	return summary
}

func copyAgentActionVerification(verification unified.ActionVerificationTruth) unified.ActionVerificationTruth {
	verification.Evidence = copyAgentActionEvidence(verification.Evidence)
	return verification
}

func copyAgentActionEvidence(evidence []unified.ActionEvidence) []unified.ActionEvidence {
	if evidence == nil {
		return nil
	}
	copy := append([]unified.ActionEvidence(nil), evidence...)
	for i := range copy {
		copy[i].Refs = append([]unified.ActionEvidenceRef(nil), evidence[i].Refs...)
	}
	return copy
}
