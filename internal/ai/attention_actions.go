package ai

import (
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

const (
	AttentionDockerHealthKind        = "docker-container-health"
	AttentionDockerRestartCapability = "restart"
	AttentionDockerLifecycleHandler  = "docker.container.lifecycle"
)

type AttentionActionEligibilityReason string

const (
	AttentionActionEligible                  AttentionActionEligibilityReason = "eligible"
	AttentionActionUnsupportedCondition      AttentionActionEligibilityReason = "unsupported_condition"
	AttentionActionLifecycleInactive         AttentionActionEligibilityReason = "lifecycle_inactive"
	AttentionActionEvidenceMissing           AttentionActionEligibilityReason = "evidence_missing"
	AttentionActionEvidenceStale             AttentionActionEligibilityReason = "evidence_stale"
	AttentionActionEvidenceIncomplete        AttentionActionEligibilityReason = "evidence_incomplete"
	AttentionActionEvidenceUnconfirmed       AttentionActionEligibilityReason = "evidence_unconfirmed"
	AttentionActionEvidencePermissionLimited AttentionActionEligibilityReason = "evidence_permission_limited"
	AttentionActionEvidenceSubjectMismatch   AttentionActionEligibilityReason = "evidence_subject_mismatch"
	AttentionActionResourceUnavailable       AttentionActionEligibilityReason = "resource_unavailable"
	AttentionActionCapabilityUnavailable     AttentionActionEligibilityReason = "capability_unavailable"
	AttentionActionExecutionUnavailable      AttentionActionEligibilityReason = "execution_unavailable"
	AttentionActionOperatorUnauthorized      AttentionActionEligibilityReason = "operator_unauthorized"
)

type AttentionActionCandidate struct {
	Resource   *unified.Resource
	Readiness  unified.ResourceActionReadiness
	Authorized bool
	Action     *unified.ActionAuditRecord
}

// ProjectAttentionAction evaluates the one deliberately narrow first action
// against canonical lifecycle evidence and live resource capability state.
// API and UI callers must not reconstruct these gates from labels or metadata.
func ProjectAttentionAction(
	detail *AttentionItemDetail,
	candidate AttentionActionCandidate,
	now time.Time,
) (AttentionActionOffer, AttentionActionEligibilityReason) {
	if detail == nil {
		return AttentionActionOffer{}, AttentionActionUnsupportedCondition
	}
	item := &detail.Item
	if existing := candidate.Action; AttentionActionMatchesItem(*item, existing) {
		if !candidate.Authorized {
			return AttentionActionOffer{}, AttentionActionOperatorUnauthorized
		}
		return attentionDockerRestartOffer(*item, detail.Evidence, existing), AttentionActionEligible
	}
	if item.Kind != AttentionDockerHealthKind ||
		item.SubjectResourceType != string(unified.ResourceTypeAppContainer) {
		return AttentionActionOffer{}, AttentionActionUnsupportedCondition
	}
	if item.State != operationaltrust.OperationalOpen &&
		item.State != operationaltrust.OperationalAcknowledged {
		return AttentionActionOffer{}, AttentionActionLifecycleInactive
	}
	if len(detail.Evidence) == 0 {
		return AttentionActionOffer{}, AttentionActionEvidenceMissing
	}
	for _, evidence := range detail.Evidence {
		if evidence.FreshnessAt(now) != operationaltrust.EvidenceFresh {
			return AttentionActionOffer{}, AttentionActionEvidenceStale
		}
		if evidence.Completeness != operationaltrust.EvidenceComplete {
			return AttentionActionOffer{}, AttentionActionEvidenceIncomplete
		}
		if evidence.Confidence != operationaltrust.EvidenceConfirmed {
			return AttentionActionOffer{}, AttentionActionEvidenceUnconfirmed
		}
		if evidence.Permissions != operationaltrust.EvidencePermissionsSufficient {
			return AttentionActionOffer{}, AttentionActionEvidencePermissionLimited
		}
		if unified.CanonicalResourceID(evidence.Subject.ResourceID) !=
			unified.CanonicalResourceID(item.SubjectResourceID) {
			return AttentionActionOffer{}, AttentionActionEvidenceSubjectMismatch
		}
		if evidence.Correlation != nil && evidence.Correlation.CandidateCount != 1 {
			return AttentionActionOffer{}, AttentionActionEvidenceSubjectMismatch
		}
	}
	if candidate.Resource == nil ||
		unified.CanonicalResourceID(candidate.Resource.ID) !=
			unified.CanonicalResourceID(item.SubjectResourceID) {
		return AttentionActionOffer{}, AttentionActionResourceUnavailable
	}
	found := false
	for _, capability := range candidate.Resource.Capabilities {
		if strings.TrimSpace(capability.Name) == AttentionDockerRestartCapability &&
			strings.TrimSpace(capability.InternalHandler) == AttentionDockerLifecycleHandler &&
			capability.MinimumApprovalLevel == unified.ApprovalAdmin {
			found = true
			break
		}
	}
	if !found {
		return AttentionActionOffer{}, AttentionActionCapabilityUnavailable
	}
	if !candidate.Readiness.Available ||
		strings.TrimSpace(candidate.Readiness.Name) != AttentionDockerRestartCapability {
		return AttentionActionOffer{}, AttentionActionExecutionUnavailable
	}
	if !candidate.Authorized {
		return AttentionActionOffer{}, AttentionActionOperatorUnauthorized
	}
	return attentionDockerRestartOffer(*item, detail.Evidence, nil), AttentionActionEligible
}

func attentionDockerRestartOffer(
	item AttentionItem,
	evidence []operationaltrust.EvidenceEnvelope,
	action *unified.ActionAuditRecord,
) AttentionActionOffer {
	evidenceIDs := make([]string, 0, len(evidence))
	if action != nil && action.Origin != nil {
		evidenceIDs = append(evidenceIDs, action.Origin.EvidenceIDs...)
	} else {
		for _, envelope := range evidence {
			evidenceIDs = append(evidenceIDs, envelope.ID)
		}
	}
	evidenceIDs = uniqueSortedAttentionEvidenceIDs(evidenceIDs)
	offer := AttentionActionOffer{
		TargetResourceID:      item.SubjectResourceID,
		Capability:            AttentionDockerRestartCapability,
		Kind:                  "container_restart",
		Label:                 "Restart this container",
		Mode:                  "plan",
		Risk:                  "low",
		Approval:              "required",
		Eligibility:           "eligible",
		Reasons:               []string{"fresh_confirmed_unhealthy_container", "declared_live_capability"},
		EvidenceIDs:           evidenceIDs,
		ExpectedPostcondition: "The same container is observed running after the restart.",
		VerificationPolicy:    "Pulse requires a fresh container readback and records whether it is agent-attested or independently observed.",
		RequiresApproval:      true,
	}
	if action == nil {
		return offer
	}
	offer.ActionID = action.ID
	offer.Mode = "execute"
	offer.Approval = attentionActionApproval(*action)
	return offer
}

// AttentionActionMatchesItem validates that a durable action belongs to the
// exact canonical record, resource, and bounded capability being projected.
func AttentionActionMatchesItem(
	item AttentionItem,
	action *unified.ActionAuditRecord,
) bool {
	if action == nil || action.Origin == nil {
		return false
	}
	return strings.TrimSpace(action.Origin.OperationalRecordID) ==
		strings.TrimSpace(item.OperationalRecordID) &&
		unified.CanonicalResourceID(action.Request.ResourceID) ==
			unified.CanonicalResourceID(item.SubjectResourceID) &&
		strings.TrimSpace(action.Request.CapabilityName) ==
			AttentionDockerRestartCapability
}

func uniqueSortedAttentionEvidenceIDs(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, found := seen[value]; found {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func attentionActionApproval(action unified.ActionAuditRecord) string {
	switch action.State {
	case unified.ActionStateApproved,
		unified.ActionStateExecuting,
		unified.ActionStateCompleted,
		unified.ActionStateFailed:
		return "granted"
	case unified.ActionStateRejected:
		return "denied"
	default:
		return "required"
	}
}

func AttentionActionVerificationState(
	action *unified.ActionAuditRecord,
) AttentionVerificationState {
	if action == nil {
		return AttentionVerificationNotAvailable
	}
	switch action.State {
	case unified.ActionStatePlanned,
		unified.ActionStatePending,
		unified.ActionStateApproved,
		unified.ActionStateExecuting:
		return AttentionVerificationPending
	case unified.ActionStateRejected, unified.ActionStateExpired:
		return AttentionVerificationNotAvailable
	}
	if action.Result != nil && action.Result.ActionResultV2 != nil {
		switch action.Result.ActionResultV2.Verification.Status {
		case unified.ActionVerificationConfirmed:
			return AttentionVerificationSucceeded
		case unified.ActionVerificationContradicted:
			return AttentionVerificationFailed
		case unified.ActionVerificationInconclusive,
			unified.ActionVerificationNotAttempted:
			return AttentionVerificationUnknown
		}
	}
	if action.State == unified.ActionStateFailed {
		return AttentionVerificationFailed
	}
	return AttentionVerificationUnknown
}
