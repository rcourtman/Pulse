package api

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/rcourtman/pulse-go-rewrite/internal/actionlifecycle"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
	"github.com/rs/zerolog/log"
)

// patrolInvestigationLocks serializes the projection of each Patrol
// investigation's action. A transition callback, a read that hydrates the
// investigation and a run's close each read the action audit and apply it under
// the investigation's lock, so a projection read from an older audit state
// cannot land after one read from a newer state.
var patrolInvestigationLocks keyedLocks

// keyedLocks is a mutex per key, held only while some caller uses it.
type keyedLocks struct {
	mu    sync.Mutex
	locks map[string]*keyedLock
}

type keyedLock struct {
	mu    sync.Mutex
	users int
}

// lock acquires key's mutex and returns its release.
func (k *keyedLocks) lock(key string) func() {
	k.mu.Lock()
	if k.locks == nil {
		k.locks = make(map[string]*keyedLock)
	}
	entry := k.locks[key]
	if entry == nil {
		entry = &keyedLock{}
		k.locks[key] = entry
	}
	entry.users++
	k.mu.Unlock()
	entry.mu.Lock()
	return func() {
		entry.mu.Unlock()
		k.mu.Lock()
		entry.users--
		if entry.users == 0 {
			delete(k.locks, key)
		}
		k.mu.Unlock()
	}
}

func lockPatrolInvestigation(orgID, investigationID string) func() {
	return patrolInvestigationLocks.lock(strings.TrimSpace(orgID) + "\x00" + strings.TrimSpace(investigationID))
}

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
	defer lockPatrolInvestigation(orgID, transition.Origin.InvestigationID)()
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

// closePatrolInvestigationRun reconciles a closed run's action and finalizes the
// run under the investigation's lock, so no projection of the action lands
// between the two: the orchestrator writes the action back as it was planned,
// which an approval, rejection or expiry during the run may have overtaken.
// Once the orchestrator's own close has landed, its stored session is the one
// finalized, rather than a copy the stuck-run sweep marked failed.
func (h *AISettingsHandler) closePatrolInvestigationRun(orgID string, session *aicontracts.InvestigationSession, finalize func(*aicontracts.InvestigationSession)) {
	if h == nil || session == nil || finalize == nil {
		return
	}
	defer lockPatrolInvestigation(orgID, session.ID)()
	if stored := h.storedInvestigation(orgID, session.ID); stored != nil &&
		stored.Status != aicontracts.InvestigationStatusPending && stored.Status != aicontracts.InvestigationStatusRunning {
		session = stored
	}
	finalize(h.hydratePatrolInvestigationActionLocked(orgID, session))
}

// hydratePatrolInvestigationAction repairs a missed transition callback before
// an investigation is returned. Action audit remains authoritative: existing
// references hydrate by action ID, while older/missed records recover through
// the broker-owned origin index. The caller's session comes back with the
// action and outcome its store holds after the repair, so a copy taken before
// a refresh replaced its plan does not bring the replaced plan back.
func (h *AISettingsHandler) hydratePatrolInvestigationAction(orgID string, investigation *aicontracts.InvestigationSession) *aicontracts.InvestigationSession {
	if h == nil || investigation == nil {
		return investigation
	}
	defer lockPatrolInvestigation(orgID, investigation.ID)()
	return h.hydratePatrolInvestigationActionLocked(orgID, investigation)
}

// hydratePatrolInvestigationActionLocked is hydratePatrolInvestigationAction for
// a caller that holds the investigation's lock.
func (h *AISettingsHandler) hydratePatrolInvestigationActionLocked(orgID string, investigation *aicontracts.InvestigationSession) *aicontracts.InvestigationSession {
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
	// Fall back to the audit's own projection if the backing investigation
	// store disappeared between lookup and repair.
	copy := *investigation
	if stored := h.storedInvestigation(orgID, investigation.ID); stored != nil {
		copy.Action = stored.Action
		copy.Outcome = stored.Outcome
		return &copy
	}
	copy.Action = actionReferenceFromAudit(audit)
	if outcome := patrolOutcomeForActionAudit(audit); outcome != "" {
		copy.Outcome = outcome
	}
	return &copy
}

// storedInvestigation returns the investigation as its org's store holds it.
func (h *AISettingsHandler) storedInvestigation(orgID, investigationID string) *aicontracts.InvestigationSession {
	h.investigationMu.RLock()
	store := h.investigationStores[orgID]
	h.investigationMu.RUnlock()
	if store == nil {
		return nil
	}
	return store.Get(investigationID)
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
	if h.patrolActionSuperseded(orgID, investigation.Action, audit) {
		return
	}
	reference := actionReferenceFromAudit(audit)
	// The model's causal attribution is not part of the action audit, so the
	// one the investigation recorded for this action stays.
	if investigation.Action != nil && investigation.Action.ActionID == reference.ActionID {
		reference.CausalResourceID = investigation.Action.CausalResourceID
	}
	actionChanged := !reflect.DeepEqual(investigation.Action, reference)
	if actionChanged {
		investigation.Action = reference
	}
	outcome := patrolOutcomeForActionAudit(audit)
	outcomeChanged := outcome != "" && investigation.Outcome != outcome
	if outcomeChanged {
		investigation.Outcome = outcome
	}
	saveInvestigationProjection(store, investigation, actionChanged, outcomeChanged)
	if outcome != "" {
		// The finding's record follows the session as stored, which a store
		// that merges the action, such as keeping the causal attribution,
		// may hold differently from this projection.
		if stored := store.Get(investigation.ID); stored != nil {
			investigation = stored
		}
		ctx := context.WithValue(context.Background(), OrgIDContextKey, orgID)
		h.updateFindingInvestigationOutcome(ctx, orgID, origin.FindingID, string(outcome), investigation)
	}
}

// saveInvestigationProjection records a reconciled action and outcome on the
// investigation's session. A store that records the action on its own does not
// replace the session, so progress the orchestrator saved after the session
// was read stays.
func saveInvestigationProjection(store aicontracts.InvestigationStore, investigation *aicontracts.InvestigationSession, actionChanged, outcomeChanged bool) {
	if !actionChanged && !outcomeChanged {
		return
	}
	actions, ok := store.(aicontracts.InvestigationActionStore)
	if !ok {
		store.Update(investigation)
		return
	}
	if actionChanged {
		actions.SetAction(investigation.ID, investigation.Action)
	}
	if outcomeChanged {
		store.SetOutcome(investigation.ID, investigation.Outcome)
	}
}

// maxPatrolActionRefreshLineage bounds the walk back through refreshes.
const maxPatrolActionRefreshLineage = 32

// patrolActionSuperseded reports whether audit is for a plan that the
// investigation's current action replaced through one or more refreshes. A
// refresh plans the replacement under the same origin and leaves the replaced
// plan to expire on its own; that plan's later transitions, such as its
// expiry, must not take the investigation back from its replacement.
func (h *AISettingsHandler) patrolActionSuperseded(orgID string, current *aicontracts.ActionReference, audit unifiedresources.ActionAuditRecord) bool {
	if current == nil || strings.TrimSpace(current.ActionID) == "" || current.ActionID == audit.ID {
		return false
	}
	store, err := h.actionAuditStore(orgID)
	if err != nil {
		return false
	}
	actionID := current.ActionID
	for range maxPatrolActionRefreshLineage {
		record, found, err := store.GetActionAudit(actionID)
		if err != nil || !found {
			return false
		}
		replaced, ok := actionlifecycle.RefreshedActionID(record)
		if !ok {
			return false
		}
		if replaced == audit.ID {
			return true
		}
		actionID = replaced
	}
	return false
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

// actionReferenceFromAudit projects an action audit onto the reference an
// investigation keeps, with the canonical result and verification status the
// broker reports for it (dispositionFromRecord).
func actionReferenceFromAudit(audit unifiedresources.ActionAuditRecord) *aicontracts.ActionReference {
	disposition := dispositionFromRecord(audit)
	return &aicontracts.ActionReference{
		ActionResultV2:     disposition.ActionResultV2,
		VerificationStatus: disposition.VerificationStatus,
		ActionID:           audit.ID,
		ProposalID:         audit.Origin.ProposalID,
		ResourceID:         audit.Request.ResourceID,
		CapabilityName:     audit.Request.CapabilityName,
		State:              string(audit.State),
		Plan:               disposition.Plan,
	}
}

func patrolOutcomeForActionAudit(audit unifiedresources.ActionAuditRecord) aicontracts.InvestigationOutcome {
	switch audit.State {
	case unifiedresources.ActionStatePlanned, unifiedresources.ActionStatePending, unifiedresources.ActionStateApproved, unifiedresources.ActionStateExecuting:
		return aicontracts.OutcomeFixQueued
	case unifiedresources.ActionStateExpired:
		return aicontracts.OutcomeNeedsAttention
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
