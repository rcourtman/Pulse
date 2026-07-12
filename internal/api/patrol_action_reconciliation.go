package api

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
	"github.com/rs/zerolog/log"
)

// ReconcilePatrolActionTransition projects the authoritative canonical action
// audit onto the Patrol investigation and finding that originated it. The
// callback payload is only a wake-up signal: re-reading by action ID prevents
// duplicate or out-of-order publications from regressing a surface record.
func (h *AISettingsHandler) ReconcilePatrolActionTransition(orgID string, transition unifiedresources.ActionAuditRecord) {
	if h == nil || !isPatrolActionOrigin(transition.Origin) {
		return
	}
	store, err := h.actionAuditStore(orgID)
	if err != nil {
		log.Warn().Err(err).Str("orgID", orgID).Str("actionID", transition.ID).Msg("Unable to reconcile Patrol action transition")
		return
	}
	authoritative, found, err := store.GetActionAudit(transition.ID)
	if err != nil {
		log.Warn().Err(err).Str("orgID", orgID).Str("actionID", transition.ID).Msg("Unable to read authoritative Patrol action transition")
		return
	}
	if !found || !isPatrolActionOrigin(authoritative.Origin) {
		return
	}
	h.applyPatrolActionAudit(orgID, authoritative)
}

// hydratePatrolInvestigationAction repairs a missed transition callback before
// an investigation is returned. Action audit remains authoritative: existing
// references hydrate by action ID, while older/missed records recover through
// the broker-owned origin index.
func (h *AISettingsHandler) hydratePatrolInvestigationAction(orgID string, investigation *aicontracts.InvestigationSession) *aicontracts.InvestigationSession {
	if h == nil || investigation == nil {
		return investigation
	}
	store, err := h.actionAuditStore(orgID)
	if err != nil {
		log.Warn().Err(err).Str("orgID", orgID).Str("investigationID", investigation.ID).Msg("Unable to hydrate Patrol action")
		return investigation
	}

	var audit unifiedresources.ActionAuditRecord
	var found bool
	if investigation.Action != nil && strings.TrimSpace(investigation.Action.ActionID) != "" {
		audit, found, err = store.GetActionAudit(investigation.Action.ActionID)
	} else if reader, ok := store.(unifiedresources.ActionAuditOriginReader); ok {
		audit, found, err = reader.GetLatestActionAuditByOrigin(patrolActionOriginSurface, investigation.ID)
	}
	if err != nil {
		log.Warn().Err(err).Str("orgID", orgID).Str("investigationID", investigation.ID).Msg("Unable to query Patrol action for hydration")
		return investigation
	}
	if !found || !isPatrolActionOrigin(audit.Origin) || audit.Origin.InvestigationID != investigation.ID || audit.Origin.FindingID != investigation.FindingID {
		return investigation
	}

	h.applyPatrolActionAudit(orgID, audit)
	// Return the hydrated projection even if the backing investigation store
	// disappeared between lookup and repair.
	copy := *investigation
	copy.Action = actionReferenceFromAudit(audit)
	if outcome := patrolOutcomeForActionAudit(audit); outcome != "" {
		copy.Outcome = outcome
	}
	return &copy
}

func (h *AISettingsHandler) applyPatrolActionAudit(orgID string, audit unifiedresources.ActionAuditRecord) {
	origin := audit.Origin
	if !isPatrolActionOrigin(origin) {
		return
	}
	h.investigationMu.RLock()
	store := h.investigationStores[orgID]
	h.investigationMu.RUnlock()
	if store == nil {
		return
	}
	investigation := store.Get(origin.InvestigationID)
	if investigation == nil || strings.TrimSpace(investigation.FindingID) != origin.FindingID {
		return
	}
	reference := actionReferenceFromAudit(audit)
	changed := false
	if !reflect.DeepEqual(investigation.Action, reference) {
		investigation.Action = reference
		changed = true
	}
	if outcome := patrolOutcomeForActionAudit(audit); outcome != "" {
		if investigation.Outcome != outcome {
			investigation.Outcome = outcome
			changed = true
		}
		if changed {
			store.Update(investigation)
		}
		ctx := context.WithValue(context.Background(), OrgIDContextKey, orgID)
		h.updateFindingOutcome(ctx, orgID, origin.FindingID, string(outcome))
		return
	}
	if changed {
		store.Update(investigation)
	}
}

func (h *AISettingsHandler) actionAuditStore(orgID string) (unifiedresources.ResourceStore, error) {
	h.stateMu.RLock()
	provider := h.resourceStoreProvider
	h.stateMu.RUnlock()
	if provider == nil {
		return nil, fmt.Errorf("action audit store unavailable")
	}
	return provider(strings.TrimSpace(orgID))
}

func isPatrolActionOrigin(origin *unifiedresources.ActionOrigin) bool {
	return origin != nil &&
		strings.TrimSpace(origin.Surface) == patrolActionOriginSurface &&
		strings.TrimSpace(origin.FindingID) != "" &&
		strings.TrimSpace(origin.InvestigationID) != ""
}

func actionReferenceFromAudit(audit unifiedresources.ActionAuditRecord) *aicontracts.ActionReference {
	plan := approvalPlanRequestToInfo(&audit.Plan)
	return &aicontracts.ActionReference{
		ActionID:       audit.ID,
		ProposalID:     audit.Origin.ProposalID,
		ResourceID:     audit.Request.ResourceID,
		CapabilityName: audit.Request.CapabilityName,
		State:          string(audit.State),
		Plan:           *plan,
	}
}

func patrolOutcomeForActionAudit(audit unifiedresources.ActionAuditRecord) aicontracts.InvestigationOutcome {
	switch audit.State {
	case unifiedresources.ActionStatePlanned, unifiedresources.ActionStatePending, unifiedresources.ActionStateApproved, unifiedresources.ActionStateExecuting:
		return aicontracts.OutcomeFixQueued
	case unifiedresources.ActionStateRejected:
		return aicontracts.OutcomeFixRejected
	case unifiedresources.ActionStateFailed, unifiedresources.ActionStateCompleted:
		truth := unifiedresources.CanonicalActionResultV2(audit)
		if truth.Execution.Status == unifiedresources.ActionExecutionFailed || truth.Execution.Status == unifiedresources.ActionExecutionNotRun {
			return aicontracts.OutcomeFixFailed
		}
		if truth.Execution.Status != unifiedresources.ActionExecutionSucceeded {
			return aicontracts.OutcomeFixVerificationUnknown
		}
		switch truth.Verification.Status {
		case unifiedresources.ActionVerificationConfirmed:
			if truth.Verification.EvidenceClass == unifiedresources.ActionEvidenceIndependent {
				return aicontracts.OutcomeFixVerified
			}
			return aicontracts.OutcomeFixVerificationUnknown
		case unifiedresources.ActionVerificationContradicted:
			return aicontracts.OutcomeFixVerificationFailed
		default:
			return aicontracts.OutcomeFixVerificationUnknown
		}
	default:
		return ""
	}
}
