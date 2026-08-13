package ai

import (
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
)

// ProposeObserver is the narrow model-to-core monitor-builder adapter. It can
// create proposal material only; validation, installation, health leasing, and
// execution deliberately remain unavailable through this interface.
func (a *patrolFindingCreatorAdapter) ProposeObserver(input tools.PatrolObserverProposalInput) (tools.PatrolObserverProposalResult, error) {
	if a == nil || a.patrol == nil {
		return tools.PatrolObserverProposalResult{}, fmt.Errorf("patrol observer builder unavailable")
	}
	store := a.patrol.GetObjectiveStore()
	if store == nil {
		return tools.PatrolObserverProposalResult{}, fmt.Errorf("patrol objective store unavailable")
	}
	triggerKind := PatrolObserverTriggerKind(strings.ToLower(strings.TrimSpace(input.TriggerKind)))
	if !isPatrolObserverTriggerKind(triggerKind) {
		return tools.PatrolObserverProposalResult{}, fmt.Errorf("unsupported observer trigger kind %q", input.TriggerKind)
	}
	objective, err := store.ProposeObserver(input.ObjectiveID, ProposePatrolObserverInput{
		ExpectedRevision: input.ExpectedRevision,
		Interpretation:   input.Interpretation,
		TriggerKinds:     []PatrolObserverTriggerKind{triggerKind},
		ProbeJSON:        input.ProbeJSON,
		WakeEvidence:     input.WakeEvidence,
		RequirementsJSON: input.RequirementsJSON,
		Actor:            "patrol:model",
	}, time.Now().UTC())
	if err != nil {
		return tools.PatrolObserverProposalResult{}, err
	}
	if objective.Observer == nil {
		return tools.PatrolObserverProposalResult{}, fmt.Errorf("observer proposal was not retained")
	}
	return tools.PatrolObserverProposalResult{
		ObjectiveID:    objective.ID,
		Revision:       objective.Revision,
		ObserverID:     objective.Observer.ID,
		Version:        objective.Observer.Version,
		State:          string(objective.Observer.State),
		ArtifactDigest: objective.Observer.ArtifactDigest,
		CoverageState:  string(objective.Coverage.State),
		CoverageReason: objective.Coverage.ReasonCode,
	}, nil
}
