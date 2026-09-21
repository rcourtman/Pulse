package ai

import (
	"time"

	"github.com/rs/zerolog/log"
)

func patrolObjectiveNeedsPlanning(objective PatrolObjective) bool {
	return objective.Status == PatrolObjectiveActive &&
		(objective.Observer == nil || objective.Observer.State == PatrolObserverDisabled)
}

// Saved intent is the durable setup request. The trigger queue is only a
// delivery mechanism and may disappear on restart or reject work while busy.
func (p *PatrolService) queueMissingObjectiveCoverage(objectives []PatrolObjective, now time.Time) {
	p.mu.RLock()
	canPlan := p.config.Enabled && p.config.RuntimeBlockedReason == "" && !p.runInProgress
	p.mu.RUnlock()
	if !canPlan || IsDemoMode() {
		return
	}
	for _, objective := range objectives {
		if patrolObjectiveNeedsPlanning(objective) && p.objectiveCoverageAttemptDue(objective, now) {
			p.QueueObjectiveCoverage(objective)
		}
	}
}

// A delivered attempt with no retained proposal retries at the operator's
// existing Patrol interval, not at the five-second observer sampling rate.
// The timestamp belongs to retained intent, so pruning run history or
// restarting the service cannot erase the interval between provider calls.
func (p *PatrolService) objectiveCoverageAttemptDue(objective PatrolObjective, now time.Time) bool {
	p.mu.RLock()
	interval := p.config.GetInterval()
	p.mu.RUnlock()
	return objective.LastPlanningAttemptAt == nil || !now.Before(objective.LastPlanningAttemptAt.Add(interval))
}

// Recheck after acquiring the run slot. A queued snapshot must not start a
// second model call after another run supplied the observer, or act on intent
// that the operator has since edited, paused, or deleted.
func (p *PatrolService) beginObjectivePlanning(scope PatrolScope, now time.Time) bool {
	p.mu.RLock()
	store := p.objectiveStore
	interval := p.config.GetInterval()
	p.mu.RUnlock()
	if store == nil || scope.ObjectiveContext == nil {
		return false
	}
	accepted, err := store.beginObserverPlanning(scope.ObjectiveContext.ObjectiveID, scope.ObjectiveContext.Revision, now, interval)
	if err != nil {
		log.Warn().Err(err).Str("objective_id", scope.ObjectiveContext.ObjectiveID).Msg("Patrol objective setup attempt could not be retained")
	}
	return accepted && err == nil
}

// Claim the exact revision and persist before calling the provider. This is
// scheduling metadata, not an operator edit or evidence of monitoring coverage.
func (s *PatrolObjectiveStore) beginObserverPlanning(id string, revision uint64, now time.Time, interval time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	objective, found := s.objectives[id]
	if !found || objective.Revision != revision || !patrolObjectiveNeedsPlanning(*objective) ||
		(objective.LastPlanningAttemptAt != nil && now.Before(objective.LastPlanningAttemptAt.Add(interval))) {
		return false, nil
	}
	next := clonePatrolObjectiveMap(s.objectives)
	attemptAt := now.UTC()
	next[id].LastPlanningAttemptAt = &attemptAt
	if err := s.persistLocked(next); err != nil {
		return false, err
	}
	s.objectives = next
	return true, nil
}
