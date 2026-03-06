package ai

import "github.com/rcourtman/pulse-go-rewrite/internal/models"

// patrolRuntimeState is the patrol subsystem's internal runtime state contract.
// It starts as a narrow boundary over the legacy snapshot shape so patrol can
// stop depending directly on models.StateSnapshot in its internal flow.
type patrolRuntimeState models.StateSnapshot

func newPatrolRuntimeState(snapshot models.StateSnapshot) patrolRuntimeState {
	return patrolRuntimeState(snapshot)
}

func (s patrolRuntimeState) snapshot() models.StateSnapshot {
	return models.StateSnapshot(s)
}

func (p *PatrolService) currentPatrolRuntimeState() patrolRuntimeState {
	if p == nil || p.stateProvider == nil {
		return patrolRuntimeState{}
	}
	return newPatrolRuntimeState(p.stateProvider.ReadSnapshot())
}
