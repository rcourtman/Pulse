package ai

import "time"

// maxDeferredInvestigations bounds how many findings can wait for an
// investigation slot. A finding refused here is picked up the old way, when a
// later Patrol run reports it again.
const maxDeferredInvestigations = 64

// DeferInvestigation remembers a finding whose investigation could not start
// because every investigation slot was busy. It reports whether the finding is
// now waiting. The list lives in memory only: after a restart a waiting
// finding is investigated when a later run reports it, as before.
func (s *FindingsStore) DeferInvestigation(findingID string, now time.Time) bool {
	if s == nil || findingID == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, waiting := s.deferredInvestigations[findingID]; waiting {
		return true
	}
	if _, exists := s.findings[findingID]; !exists {
		return false
	}
	if len(s.deferredInvestigations) >= maxDeferredInvestigations {
		return false
	}
	if s.deferredInvestigations == nil {
		s.deferredInvestigations = make(map[string]time.Time)
	}
	s.deferredInvestigations[findingID] = now
	return true
}

// TakeNextDeferredInvestigation removes and returns the finding that should
// get the next free investigation slot: a critical finding before a warning,
// then the longest waiting. Entries whose finding no longer exists are
// dropped. It returns nil when nothing is waiting.
func (s *FindingsStore) TakeNextDeferredInvestigation() *Finding {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	var (
		next      *Finding
		nextSince time.Time
	)
	for id, since := range s.deferredInvestigations {
		f, exists := s.findings[id]
		if !exists {
			delete(s.deferredInvestigations, id)
			continue
		}
		if next != nil {
			critical := f.Severity == FindingSeverityCritical
			nextCritical := next.Severity == FindingSeverityCritical
			switch {
			case critical != nextCritical:
				if !critical {
					continue
				}
			case !since.Equal(nextSince):
				if since.After(nextSince) {
					continue
				}
			case f.ID > next.ID:
				continue
			}
		}
		next, nextSince = f, since
	}
	if next == nil {
		return nil
	}
	delete(s.deferredInvestigations, next.ID)
	findingCopy := *next
	return &findingCopy
}

// startDeferredInvestigations gives a freed investigation slot to the finding
// that has been waiting for one. maybeInvestigateFinding applies the usual
// gate again (mode, severity, attempts, cooldown, resolved or dismissed), so
// this starts no investigation that was not already owed. A waiting finding
// that no longer passes the gate is dropped and the next one is tried.
//
// It starts at most one investigation per call. The slot check is advisory
// and the orchestrator reserves the slot inside the investigation's own
// goroutine, so starting two here would let a later finding overtake an
// earlier one. Each investigation that ends calls this again.
func (p *PatrolService) startDeferredInvestigations() {
	if p == nil || p.findings == nil {
		return
	}
	p.mu.RLock()
	orchestrator := p.investigationOrchestrator
	p.mu.RUnlock()
	if orchestrator == nil {
		return
	}
	for attempts := 0; attempts < maxDeferredInvestigations; attempts++ {
		if !orchestrator.CanStartInvestigation() {
			return
		}
		finding := p.findings.TakeNextDeferredInvestigation()
		if finding == nil || p.maybeInvestigateFinding(finding) {
			return
		}
	}
}
