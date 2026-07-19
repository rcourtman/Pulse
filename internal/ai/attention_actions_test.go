package ai

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestProjectAttentionActionRequiresFreshConfirmedCanonicalEvidence(t *testing.T) {
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	detail := attentionDockerHealthDetail(now)
	resource := attentionDockerResource(detail.Item.SubjectResourceID)
	eligible := AttentionActionCandidate{
		Resource:   &resource,
		Readiness:  unified.ResourceActionReadiness{Name: "restart", Available: true},
		Authorized: true,
	}

	offer, reason := ProjectAttentionAction(&detail, eligible, now)
	if reason != AttentionActionEligible {
		t.Fatalf("reason = %q, want eligible", reason)
	}
	if offer.Capability != "restart" ||
		offer.TargetResourceID != detail.Item.SubjectResourceID ||
		offer.Approval != "required" ||
		offer.Eligibility != "eligible" ||
		len(offer.EvidenceIDs) != 1 {
		t.Fatalf("offer = %+v", offer)
	}

	tests := []struct {
		name   string
		mutate func(*AttentionItemDetail, *AttentionActionCandidate)
		want   AttentionActionEligibilityReason
	}{
		{
			name: "stale evidence",
			mutate: func(detail *AttentionItemDetail, _ *AttentionActionCandidate) {
				expired := now.Add(-time.Second)
				detail.Evidence[0].ValidUntil = &expired
			},
			want: AttentionActionEvidenceStale,
		},
		{
			name: "partial evidence",
			mutate: func(detail *AttentionItemDetail, _ *AttentionActionCandidate) {
				detail.Evidence[0].Completeness = operationaltrust.EvidencePartial
			},
			want: AttentionActionEvidenceIncomplete,
		},
		{
			name: "permission limited",
			mutate: func(detail *AttentionItemDetail, _ *AttentionActionCandidate) {
				detail.Evidence[0].Permissions = operationaltrust.EvidencePermissionsPartial
			},
			want: AttentionActionEvidencePermissionLimited,
		},
		{
			name: "ambiguous subject",
			mutate: func(detail *AttentionItemDetail, _ *AttentionActionCandidate) {
				detail.Evidence[0].Correlation = &operationaltrust.IdentityCorrelation{
					Rule: "hostname",
					MatchedFields: map[string]string{
						"hostname": "api",
					},
					CandidateCount: 2,
				}
			},
			want: AttentionActionEvidenceSubjectMismatch,
		},
		{
			name: "capability missing",
			mutate: func(_ *AttentionItemDetail, candidate *AttentionActionCandidate) {
				candidate.Resource.Capabilities = nil
			},
			want: AttentionActionCapabilityUnavailable,
		},
		{
			name: "executor refused",
			mutate: func(_ *AttentionItemDetail, candidate *AttentionActionCandidate) {
				candidate.Readiness.Available = false
				candidate.Readiness.ReasonCode = "agent_disconnected"
			},
			want: AttentionActionExecutionUnavailable,
		},
		{
			name: "executor readiness missing",
			mutate: func(_ *AttentionItemDetail, candidate *AttentionActionCandidate) {
				candidate.Readiness = unified.ResourceActionReadiness{}
			},
			want: AttentionActionExecutionUnavailable,
		},
		{
			name: "operator unauthorized",
			mutate: func(_ *AttentionItemDetail, candidate *AttentionActionCandidate) {
				candidate.Authorized = false
			},
			want: AttentionActionOperatorUnauthorized,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			currentDetail := attentionDockerHealthDetail(now)
			currentResource := attentionDockerResource(currentDetail.Item.SubjectResourceID)
			currentCandidate := AttentionActionCandidate{
				Resource:   &currentResource,
				Readiness:  unified.ResourceActionReadiness{Name: "restart", Available: true},
				Authorized: true,
			}
			test.mutate(&currentDetail, &currentCandidate)
			if offer, got := ProjectAttentionAction(&currentDetail, currentCandidate, now); got != test.want || offer.Capability != "" {
				t.Fatalf("offer=%+v reason=%q, want empty/%q", offer, got, test.want)
			}
		})
	}
}

func TestProjectAttentionActionUsesOnlyTheExistingRecordBoundAction(t *testing.T) {
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	detail := attentionDockerHealthDetail(now)
	detail.Item.State = operationaltrust.OperationalResolved
	action := unified.ActionAuditRecord{
		ID:    "action-1",
		State: unified.ActionStateCompleted,
		Request: unified.ActionRequest{
			ResourceID:     detail.Item.SubjectResourceID,
			CapabilityName: AttentionDockerRestartCapability,
		},
		Origin: &unified.ActionOrigin{
			OperationalRecordID: detail.Item.OperationalRecordID,
			EvidenceIDs:         []string{"planned-evidence", "planned-evidence"},
		},
	}
	offer, reason := ProjectAttentionAction(
		&detail,
		AttentionActionCandidate{Action: &action, Authorized: true},
		now,
	)
	if reason != AttentionActionEligible ||
		offer.ActionID != action.ID ||
		len(offer.EvidenceIDs) != 1 ||
		offer.EvidenceIDs[0] != "planned-evidence" {
		t.Fatalf("offer=%+v reason=%q", offer, reason)
	}

	action.Request.ResourceID = "docker:other/container"
	detail.Item.State = operationaltrust.OperationalOpen
	resource := attentionDockerResource(detail.Item.SubjectResourceID)
	offer, reason = ProjectAttentionAction(
		&detail,
		AttentionActionCandidate{
			Resource:   &resource,
			Readiness:  unified.ResourceActionReadiness{Name: "restart", Available: true},
			Authorized: true,
			Action:     &action,
		},
		now,
	)
	if reason != AttentionActionEligible || offer.ActionID != "" {
		t.Fatalf("mismatched action offer=%+v reason=%q", offer, reason)
	}
}

func TestAttentionActionVerificationStatePreservesExecutionAndPostconditionTruth(t *testing.T) {
	record := unified.ActionAuditRecord{State: unified.ActionStateExecuting}
	if got := AttentionActionVerificationState(&record); got != AttentionVerificationPending {
		t.Fatalf("executing = %q", got)
	}
	record.State = unified.ActionStateCompleted
	record.Result = &unified.ExecutionResult{ActionResultV2: &unified.ActionResultV2{
		Version: unified.ActionResultV2Version,
		Verification: unified.ActionVerificationTruth{
			Status:        unified.ActionVerificationConfirmed,
			EvidenceClass: unified.ActionEvidenceAgentAttested,
		},
	}}
	if got := AttentionActionVerificationState(&record); got != AttentionVerificationSucceeded {
		t.Fatalf("confirmed = %q", got)
	}
	record.Result.ActionResultV2.Verification.Status = unified.ActionVerificationContradicted
	if got := AttentionActionVerificationState(&record); got != AttentionVerificationFailed {
		t.Fatalf("contradicted = %q", got)
	}
	record.Result.ActionResultV2.Verification.Status = unified.ActionVerificationInconclusive
	if got := AttentionActionVerificationState(&record); got != AttentionVerificationUnknown {
		t.Fatalf("inconclusive = %q", got)
	}
}

func attentionDockerHealthDetail(now time.Time) AttentionItemDetail {
	resourceID := "docker:host-1/container-1"
	evidence := attentionTestEvidence(resourceID, now)
	return AttentionItemDetail{
		Item: AttentionItem{
			ID:                  "operational-docker-health",
			OperationalRecordID: "operational-docker-health",
			SubjectResourceID:   resourceID,
			SubjectResourceName: "api",
			SubjectResourceType: string(unified.ResourceTypeAppContainer),
			Kind:                AttentionDockerHealthKind,
			State:               operationaltrust.OperationalOpen,
		},
		OperationalRecord: operationaltrust.OperationalRecord{
			ID:                "operational-docker-health",
			SubjectResourceID: resourceID,
		},
		Evidence: []operationaltrust.EvidenceEnvelope{evidence},
	}
}

func attentionDockerResource(resourceID string) unified.Resource {
	return unified.Resource{
		ID:   resourceID,
		Type: unified.ResourceTypeAppContainer,
		Capabilities: []unified.ResourceCapability{{
			Name:                 AttentionDockerRestartCapability,
			Type:                 unified.CapabilityTypeCommon,
			MinimumApprovalLevel: unified.ApprovalAdmin,
			InternalHandler:      AttentionDockerLifecycleHandler,
		}},
	}
}
