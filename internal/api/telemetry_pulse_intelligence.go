package api

import (
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/telemetry"
	unifiedresources "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rs/zerolog/log"
)

// ApplyUpdateTelemetrySnapshot adds router-owned, content-free update funnel
// counters to the outbound usage telemetry snapshot.
func (r *Router) ApplyUpdateTelemetrySnapshot(s *telemetry.Snapshot, now time.Time) {
	if r == nil || s == nil {
		return
	}
	telemetry.ApplyUpdateTelemetrySnapshot(s, r.updateHistory, now)
}

// GetPulseIntelligenceActionTelemetry returns count-only action-governance
// telemetry for the outbound Pulse Intelligence usage loop. It deliberately drops
// command text, approval actors/reasons, action outputs, and resource IDs.
func (r *Router) GetPulseIntelligenceActionTelemetry(since time.Time) telemetry.PulseIntelligenceActionSnapshot {
	var snapshot telemetry.PulseIntelligenceActionSnapshot
	if r == nil || r.resourceHandlers == nil {
		return snapshot
	}

	for _, orgID := range r.pulseIntelligenceTelemetryOrgIDs() {
		store, err := r.resourceHandlers.getStore(orgID)
		if err != nil || store == nil {
			log.Warn().Err(err).Str("org_id", orgID).Msg("Unable to resolve action audit store for telemetry summary")
			continue
		}
		approvedAttemptIDs, approvedSuccessIDs := pulseIntelligenceApprovedActionOutcomeIDs(store, orgID, since)
		rejectedDecisionIDs := pulseIntelligenceRejectedActionDecisionIDs(store, orgID, since)
		approvedDecisionIDs := pulseIntelligenceApprovedActionDecisionIDs(store, orgID, since)
		records, err := store.GetActionAudits("", since, 0)
		if err != nil {
			log.Warn().Err(err).Str("org_id", orgID).Msg("Unable to query action audit telemetry summary")
			continue
		}
		for _, record := range records {
			snapshot.ActionPlans30d++
			if pulseIntelligenceActionRequiresApproval(record) {
				snapshot.ApprovalRequests30d++
			}
			if pulseIntelligenceActionWasRejected(record) {
				if actionID := strings.TrimSpace(record.ID); actionID != "" {
					rejectedDecisionIDs[actionID] = struct{}{}
				}
			}
			if pulseIntelligenceActionWasApprovedSince(record, since) {
				if actionID := strings.TrimSpace(record.ID); actionID != "" {
					approvedDecisionIDs[actionID] = struct{}{}
				}
			}
			if pulseIntelligenceApprovedActionAttempt(record) {
				if actionID := strings.TrimSpace(record.ID); actionID != "" {
					approvedAttemptIDs[actionID] = struct{}{}
				}
			}
			if pulseIntelligenceApprovedActionSuccess(record) {
				if actionID := strings.TrimSpace(record.ID); actionID != "" {
					approvedSuccessIDs[actionID] = struct{}{}
				}
			}
		}
		snapshot.RejectedActionDecisions30d += len(rejectedDecisionIDs)
		snapshot.ApprovedActionDecisions30d += len(approvedDecisionIDs)
		snapshot.ApprovedActionAttempts30d += len(approvedAttemptIDs)
		snapshot.ApprovedActionSuccesses30d += len(approvedSuccessIDs)
	}

	return snapshot
}

func (r *Router) pulseIntelligenceTelemetryOrgIDs() []string {
	if r == nil || r.multiTenant == nil {
		return []string{"default"}
	}
	orgs, err := r.multiTenant.ListOrganizations()
	if err != nil {
		log.Warn().Err(err).Msg("Unable to list organizations for telemetry summary")
		return []string{"default"}
	}
	seen := map[string]struct{}{}
	orgIDs := make([]string, 0, len(orgs))
	for _, org := range orgs {
		if org == nil {
			continue
		}
		orgID := strings.TrimSpace(org.ID)
		if orgID == "" {
			orgID = "default"
		}
		if _, ok := seen[orgID]; ok {
			continue
		}
		seen[orgID] = struct{}{}
		orgIDs = append(orgIDs, orgID)
	}
	if len(orgIDs) == 0 {
		return []string{"default"}
	}
	return orgIDs
}

func pulseIntelligenceActionRequiresApproval(record unifiedresources.ActionAuditRecord) bool {
	if record.Plan.RequiresApproval {
		return true
	}
	return len(record.Approvals) > 0 || record.State == unifiedresources.ActionStatePending ||
		record.State == unifiedresources.ActionStateApproved || record.State == unifiedresources.ActionStateRejected
}

func pulseIntelligenceApprovedActionOutcomeIDs(store unifiedresources.ResourceStore, orgID string, since time.Time) (map[string]struct{}, map[string]struct{}) {
	attemptIDs := map[string]struct{}{}
	successIDs := map[string]struct{}{}
	if store == nil {
		return attemptIDs, successIDs
	}
	events, err := store.GetActionLifecycleEvents("", since, 0)
	if err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("Unable to query action lifecycle telemetry summary")
		return attemptIDs, successIDs
	}
	for _, event := range events {
		if !pulseIntelligenceActionLifecycleIndicatesAttempt(event) {
			continue
		}
		actionID := strings.TrimSpace(event.ActionID)
		if actionID == "" {
			continue
		}
		record, ok, err := store.GetActionAudit(actionID)
		if err != nil {
			log.Warn().Err(err).Str("org_id", orgID).Msg("Unable to resolve action audit for lifecycle telemetry summary")
			continue
		}
		if ok && pulseIntelligenceActionWasApproved(record) {
			attemptIDs[actionID] = struct{}{}
			if event.State == unifiedresources.ActionStateCompleted && pulseIntelligenceApprovedActionSuccess(record) {
				successIDs[actionID] = struct{}{}
			}
		}
	}
	return attemptIDs, successIDs
}

func pulseIntelligenceRejectedActionDecisionIDs(store unifiedresources.ResourceStore, orgID string, since time.Time) map[string]struct{} {
	return pulseIntelligenceActionDecisionIDs(store, orgID, since, unifiedresources.ActionStateRejected, "rejected", pulseIntelligenceActionWasRejected)
}

func pulseIntelligenceApprovedActionDecisionIDs(store unifiedresources.ResourceStore, orgID string, since time.Time) map[string]struct{} {
	return pulseIntelligenceActionDecisionIDs(store, orgID, since, unifiedresources.ActionStateApproved, "approved", pulseIntelligenceActionWasApproved)
}

func pulseIntelligenceActionDecisionIDs(store unifiedresources.ResourceStore, orgID string, since time.Time, state unifiedresources.ActionState, decisionLabel string, auditMatches func(unifiedresources.ActionAuditRecord) bool) map[string]struct{} {
	decisionIDs := map[string]struct{}{}
	if store == nil {
		return decisionIDs
	}
	events, err := store.GetActionLifecycleEvents("", since, 0)
	if err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msgf("Unable to query %s action lifecycle telemetry summary", decisionLabel)
		return decisionIDs
	}
	for _, event := range events {
		if event.State != state {
			continue
		}
		actionID := strings.TrimSpace(event.ActionID)
		if actionID == "" {
			continue
		}
		record, ok, err := store.GetActionAudit(actionID)
		if err != nil {
			log.Warn().Err(err).Str("org_id", orgID).Msgf("Unable to resolve %s action audit for telemetry summary", decisionLabel)
			continue
		}
		if ok && auditMatches(record) {
			decisionIDs[actionID] = struct{}{}
		}
	}
	return decisionIDs
}

func pulseIntelligenceApprovedActionAttempt(record unifiedresources.ActionAuditRecord) bool {
	if !pulseIntelligenceActionWasApproved(record) {
		return false
	}
	switch record.State {
	case unifiedresources.ActionStateExecuting, unifiedresources.ActionStateCompleted, unifiedresources.ActionStateFailed:
		return true
	default:
		return false
	}
}

func pulseIntelligenceApprovedActionSuccess(record unifiedresources.ActionAuditRecord) bool {
	if record.State != unifiedresources.ActionStateCompleted {
		return false
	}
	return pulseIntelligenceActionVerifiedOutcome(record)
}

func pulseIntelligenceActionVerifiedOutcome(record unifiedresources.ActionAuditRecord) bool {
	if !pulseIntelligenceActionWasApproved(record) {
		return false
	}
	truth := unifiedresources.CanonicalActionResultV2(record)
	return truth.Execution.Status == unifiedresources.ActionExecutionSucceeded && truth.Verification.Status == unifiedresources.ActionVerificationConfirmed
}

func pulseIntelligenceActionLifecycleIndicatesAttempt(event unifiedresources.ActionLifecycleEvent) bool {
	switch event.State {
	case unifiedresources.ActionStateExecuting, unifiedresources.ActionStateCompleted, unifiedresources.ActionStateFailed:
		return true
	default:
		return false
	}
}

func pulseIntelligenceActionWasApproved(record unifiedresources.ActionAuditRecord) bool {
	if record.State == unifiedresources.ActionStateApproved {
		return true
	}
	for _, approval := range record.Approvals {
		if approval.Outcome == unifiedresources.OutcomeApproved {
			return true
		}
	}
	return false
}

func pulseIntelligenceActionWasApprovedSince(record unifiedresources.ActionAuditRecord, since time.Time) bool {
	for _, approval := range record.Approvals {
		if approval.Outcome != unifiedresources.OutcomeApproved {
			continue
		}
		if approval.Timestamp.IsZero() || approval.Timestamp.Before(since) {
			continue
		}
		return true
	}
	return false
}

func pulseIntelligenceActionWasRejected(record unifiedresources.ActionAuditRecord) bool {
	if record.State == unifiedresources.ActionStateRejected {
		return true
	}
	for _, approval := range record.Approvals {
		if approval.Outcome == unifiedresources.OutcomeRejected {
			return true
		}
	}
	return false
}
