package ai

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"
)

const (
	// removedResourceSweepInterval is how often Patrol compares its open
	// findings with the inventory between runs.
	removedResourceSweepInterval = time.Minute
	// removedResourceGrace is how long a finding's resource must be missing
	// from every look before the finding is closed. A run closes on one look.
	// The sweep looks far more often, so it would also look while the
	// inventory is refilling after a restart or a monitor reload, when agents
	// have not reported yet. Agents report well inside this.
	removedResourceGrace = 3 * time.Minute

	removedResourceResolveReason = "Resource no longer exists in infrastructure"
)

// resolveFindingForRemovedResource closes a finding whose resource has left
// the inventory, in the findings store and the unified store.
func (p *PatrolService) resolveFindingForRemovedResource(f *Finding) bool {
	if p == nil || p.findings == nil || f == nil {
		return false
	}
	if !p.findings.ResolveWithReason(f.ID, removedResourceResolveReason) {
		return false
	}
	p.mu.RLock()
	resolveUnified := p.unifiedFindingResolver
	p.mu.RUnlock()
	if resolveUnified != nil {
		resolveUnified(f.ID)
	}
	log.Info().
		Str("finding_id", f.ID).
		Str("resource_id", f.ResourceID).
		Str("resource_name", f.ResourceName).
		Msg("AI Patrol: Auto-resolved finding for deleted resource")
	return true
}

// removedResourceSweepLoop closes findings on resources that have left the
// inventory without waiting for a run.
//
// A run closes them when it builds its seed. Between runs nothing did, so with
// the default schedule a finding on a removed container stayed open for up to
// six hours, shown as a current problem on something that no longer exists.
// Nothing tells Patrol that a resource left: the inventory has no removal
// event. So the loop compares the open findings with the inventory Patrol
// already reads, once a minute. It calls no model and records no run.
func (p *PatrolService) removedResourceSweepLoop(ctx context.Context) {
	p.mu.RLock()
	stop := p.stopCh
	p.mu.RUnlock()

	missingSince := make(map[string]time.Time)
	ticker := time.NewTicker(removedResourceSweepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			p.sweepFindingsForRemovedResources(time.Now(), missingSince)
		case <-stop:
			return
		case <-ctx.Done():
			return
		}
	}
}

// sweepFindingsForRemovedResources takes one look. missingSince remembers, per
// finding, when its resource was first seen missing. A finding is closed once
// its resource has been missing on every look for removedResourceGrace. It
// returns how many findings it closed.
//
// The test for "missing" is the run's: findingResourceInInventory against the
// whole current inventory. An inventory with nothing in it says nothing about
// any one resource, so it closes nothing and counts for nothing.
func (p *PatrolService) sweepFindingsForRemovedResources(now time.Time, missingSince map[string]time.Time) int {
	if p == nil || p.findings == nil || missingSince == nil {
		return 0
	}
	active := p.findings.GetActive(FindingSeverityInfo)
	if len(active) == 0 {
		clear(missingSince)
		return 0
	}
	if !p.hasPatrolRuntimeInputs() {
		clear(missingSince)
		return 0
	}
	known := patrolRuntimeKnownResources(p.currentPatrolRuntimeState())
	if len(known) == 0 {
		clear(missingSince)
		return 0
	}

	closed := 0
	stillMissing := make(map[string]bool, len(missingSince))
	for _, f := range active {
		if patrolFindingUsesSyntheticRuntimeResource(f) || findingResourceInInventory(f, known) {
			continue
		}
		since, seen := missingSince[f.ID]
		if !seen {
			missingSince[f.ID] = now
			stillMissing[f.ID] = true
			continue
		}
		if now.Sub(since) < removedResourceGrace {
			stillMissing[f.ID] = true
			continue
		}
		if p.resolveFindingForRemovedResource(f) {
			closed++
		}
	}
	// A resource that is back, or a finding that is no longer open, starts
	// from nothing if it goes missing again.
	for id := range missingSince {
		if !stillMissing[id] {
			delete(missingSince, id)
		}
	}
	return closed
}
