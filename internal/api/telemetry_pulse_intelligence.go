package api

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/telemetry"
	unifiedresources "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
	"github.com/rs/zerolog/log"
)

// pulseIntelligencePatrolBlockedCause returns the fixed machine cause code
// for a Patrol that is enabled but blocked from running (for example
// provider_not_configured), or an empty string when Patrol is disabled,
// healthy, mid-run, or unavailable. Only the enum cause code leaves the
// process: no blocked-reason text, provider endpoint, model name, or
// configuration.
func (r *Router) pulseIntelligencePatrolBlockedCause() string {
	if r == nil || r.aiSettingsHandler == nil {
		return ""
	}
	patrol := r.aiSettingsHandler.getPatrolService(context.Background())
	if patrol == nil {
		return ""
	}
	status := retireQuickstartPatrolStatus(patrol.GetStatus())
	return pulseIntelligencePatrolBlockedCauseForTelemetry(status)
}

// pulseIntelligencePatrolBlockedCauseForTelemetry reduces a patrol status to
// the content-free cause code exported by usage telemetry. Anything but a
// currently blocked Patrol exports nothing, and a blocked reason that carries
// no typed cause exports nothing rather than free text.
func pulseIntelligencePatrolBlockedCauseForTelemetry(status ai.PatrolStatus) string {
	if status.RuntimeState != ai.PatrolRuntimeStateBlocked {
		return ""
	}
	cause := strings.TrimSpace(string(status.BlockedCause))
	if cause == string(ai.PatrolFailureCauseNone) {
		return ""
	}
	if len(cause) > 64 {
		cause = cause[:64]
	}
	return cause
}

// pulseIntelligencePatrolAutonomyLevel returns the effective Patrol autonomy
// level for the default tenant after licence and Autopilot acknowledgement
// gating, bounded to the four released modes. An install without an AI
// service reports monitor, which is what the runtime enforces in that state.
func (r *Router) pulseIntelligencePatrolAutonomyLevel() string {
	if r == nil || r.aiSettingsHandler == nil {
		return telemetry.NormalizePatrolAutonomyLevelForTelemetry("")
	}
	svc := r.aiSettingsHandler.GetAIService(context.Background())
	if svc == nil {
		return telemetry.NormalizePatrolAutonomyLevelForTelemetry("")
	}
	return telemetry.NormalizePatrolAutonomyLevelForTelemetry(svc.GetEffectivePatrolAutonomyLevel())
}

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
	if r == nil {
		return snapshot
	}
	snapshot.PatrolBlockedCause = r.pulseIntelligencePatrolBlockedCause()
	snapshot.PatrolAutonomyLevel = r.pulseIntelligencePatrolAutonomyLevel()
	if r.resourceHandlers == nil {
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
		recordsByID := make(map[string]unifiedresources.ActionAuditRecord, len(records))
		for _, record := range records {
			if actionID := strings.TrimSpace(record.ID); actionID != "" {
				recordsByID[actionID] = record
			}
			snapshot.ActionPlans30d++
			patrolOrigin := isPatrolActionOrigin(record.Origin)
			if patrolOrigin {
				snapshot.PatrolActionPlans30d++
			}
			if pulseIntelligenceActionRequiresApproval(record) {
				snapshot.ApprovalRequests30d++
				if patrolOrigin {
					snapshot.PatrolApprovalRequests30d++
				}
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
		snapshot.PatrolRejectedActionDecisions30d += pulseIntelligencePatrolOriginActionCount(store, rejectedDecisionIDs, recordsByID)
		snapshot.PatrolApprovedActionDecisions30d += pulseIntelligencePatrolOriginActionCount(store, approvedDecisionIDs, recordsByID)
		snapshot.PatrolApprovedActionAttempts30d += pulseIntelligencePatrolOriginActionCount(store, approvedAttemptIDs, recordsByID)
		snapshot.PatrolApprovedActionSuccesses30d += pulseIntelligencePatrolOriginActionCount(store, approvedSuccessIDs, recordsByID)
		accumulatePulseIntelligenceApprovedActionOutcomes(&snapshot, store, orgID, approvedAttemptIDs, approvedSuccessIDs, recordsByID, time.Now().UTC())
	}

	return snapshot
}

func pulseIntelligencePatrolOriginActionCount(store unifiedresources.ResourceStore, actionIDs map[string]struct{}, recordsByID map[string]unifiedresources.ActionAuditRecord) int {
	count := 0
	for actionID := range actionIDs {
		record, ok := recordsByID[actionID]
		if !ok && store != nil {
			fetched, found, err := store.GetActionAudit(actionID)
			if err == nil && found {
				record = fetched
				ok = true
			}
		}
		if ok && isPatrolActionOrigin(record.Origin) {
			count++
		}
	}
	return count
}

// pulseIntelligenceStuckExecutingThreshold separates an in-flight dispatch
// from an abandoned one. The longest legitimate typed dispatch transport wait
// is 30 minutes, so an executing record untouched for longer is stuck.
const pulseIntelligenceStuckExecutingThreshold = time.Hour

// pulseIntelligenceReasonCodePattern bounds exported failure reason codes to
// closed machine-code shape so telemetry stays content-free even if a future
// executor misuses the reason-code field.
var pulseIntelligenceReasonCodePattern = regexp.MustCompile(`^[a-z0-9_.-]{1,64}$`)

// accumulatePulseIntelligenceApprovedActionOutcomes attributes every approved
// attempt to exactly one success, failure, stuck, in-flight, or unclassified
// bucket. It also records fixed refusal categories and the machine reason code
// of the most recent failure.
func accumulatePulseIntelligenceApprovedActionOutcomes(snapshot *telemetry.PulseIntelligenceActionSnapshot, store unifiedresources.ResourceStore, orgID string, attemptIDs, successIDs map[string]struct{}, recordsByID map[string]unifiedresources.ActionAuditRecord, now time.Time) {
	var lastFailureAt time.Time
	for actionID := range attemptIDs {
		_, isSuccess := successIDs[actionID]
		record, ok := recordsByID[actionID]
		if !ok {
			fetched, found, err := store.GetActionAudit(actionID)
			if err != nil || !found {
				if err != nil {
					log.Warn().Err(err).Str("org_id", orgID).Msg("Unable to resolve action audit for failure-cause telemetry summary")
				}
				// A success outcome is already accounted by its lifecycle event;
				// only unresolved non-success attempts belong in unclassified.
				if !isSuccess {
					snapshot.ApprovedActionUnclassified30d++
				}
				continue
			}
			record = fetched
		}
		if isSuccess {
			if pulseIntelligenceVerifiedFindingResolution(record) {
				snapshot.VerifiedFindingResolutions30d++
			}
			continue
		}
		cause, reason := pulseIntelligenceApprovedActionFailureCause(record, now)
		switch cause {
		case "pre_dispatch":
			snapshot.ApprovedActionFailuresPreDispatch30d++
			switch pulseIntelligenceApprovedActionRefusalCategory(reason) {
			case "plan_stale":
				snapshot.ApprovedActionRefusalsPlanStale30d++
			case "policy":
				snapshot.ApprovedActionRefusalsPolicy30d++
			case "capability":
				snapshot.ApprovedActionRefusalsCapability30d++
			case "target_changed":
				snapshot.ApprovedActionRefusalsTargetChanged30d++
			case "prerequisite":
				snapshot.ApprovedActionRefusalsPrerequisite30d++
			case "contract":
				snapshot.ApprovedActionRefusalsContract30d++
			case "uncoded":
				snapshot.ApprovedActionRefusalsUncoded30d++
			default:
				snapshot.ApprovedActionRefusalsOther30d++
			}
		case "execution":
			snapshot.ApprovedActionFailuresExecution30d++
		case "unverified":
			snapshot.ApprovedActionFailuresUnverified30d++
		case "stuck_executing":
			snapshot.ApprovedActionStuckExecuting30d++
		case "in_flight":
			snapshot.ApprovedActionInFlight30d++
			continue
		default:
			snapshot.ApprovedActionUnclassified30d++
			continue
		}
		if record.UpdatedAt.After(lastFailureAt) {
			lastFailureAt = record.UpdatedAt
			snapshot.ApprovedActionLastFailureReason30d = reason
		}
	}
}

// pulseIntelligenceApprovedActionFailureCause classifies an approved attempt
// that is not a verified success into a coarse cause bucket plus the specific
// machine reason code. A recently-executing record is explicitly classified as
// in flight because it may yet succeed.
func pulseIntelligenceApprovedActionFailureCause(record unifiedresources.ActionAuditRecord, now time.Time) (string, string) {
	switch record.State {
	case unifiedresources.ActionStateExecuting:
		if record.UpdatedAt.IsZero() || now.Sub(record.UpdatedAt) >= pulseIntelligenceStuckExecutingThreshold {
			return "stuck_executing", "stuck_executing"
		}
		return "in_flight", ""
	case unifiedresources.ActionStateFailed:
		truth := unifiedresources.CanonicalActionResultV2(record)
		if truth.Execution.Status == unifiedresources.ActionExecutionNotRun {
			return "pre_dispatch", pulseIntelligenceSanitizedReasonCode(truth.Execution.ReasonCode, "pre_dispatch_refused")
		}
		return "execution", pulseIntelligenceSanitizedReasonCode(truth.Execution.ReasonCode, "execution_failed")
	case unifiedresources.ActionStateCompleted:
		truth := unifiedresources.CanonicalActionResultV2(record)
		return "unverified", pulseIntelligenceSanitizedReasonCode(truth.Verification.ReasonCode, "verification_unconfirmed")
	default:
		return "", ""
	}
}

func pulseIntelligenceApprovedActionRefusalCategory(reason string) string {
	switch reason {
	case "plan_drift", "action_plan_expired", "action_replan_required":
		return "plan_stale"
	case "resource_remediation_locked", "policy_authorization_expired",
		"policy_authorization_invalid", "policy_authorization_revoked", "action_emergency_stop":
		return "policy"
	case "action_dry_run_only", "action_execution_unavailable", "agent_capability_unavailable":
		return "capability"
	case "target_state_changed", "target_precondition_failed", "package_inventory_changed", "cleanup_inventory_changed":
		return "target_changed"
	case "target_inspection_unavailable", "package_manager_busy", "package_index_refresh_failed",
		"package_preflight_failed", "package_manager_unhealthy", "cleanup_preflight_failed":
		return "prerequisite"
	case "action_contract_invalid":
		return "contract"
	case actionRefusalUncoded, "pre_dispatch_refused":
		// The refusal carried no machine reason code, so there is nothing to
		// categorize. This is what an agent older than the typed refusal
		// contract reports, and it must stay separable from a code this server
		// simply does not recognise: the first says the split is waiting on
		// agent rollout, the second says the split is wrong.
		return "uncoded"
	default:
		return "other"
	}
}

func pulseIntelligenceVerifiedFindingResolution(record unifiedresources.ActionAuditRecord) bool {
	return isPatrolActionOrigin(record.Origin) &&
		pulseIntelligenceApprovedActionSuccess(record) &&
		patrolOutcomeForActionAudit(record) == aicontracts.OutcomeFixVerified
}

func pulseIntelligenceSanitizedReasonCode(code, fallback string) string {
	code = strings.TrimSpace(code)
	if pulseIntelligenceReasonCodePattern.MatchString(code) {
		return code
	}
	return fallback
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
