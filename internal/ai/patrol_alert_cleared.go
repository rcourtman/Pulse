package ai

import "strings"

// clearedAlertResolveReason is shown on a finding that closed because the
// alert it restated cleared.
const clearedAlertResolveReason = "The alert for this condition cleared."

// ResolveFindingsForClearedAlert closes the open findings that restate an
// alert which has just resolved: findings stamped as mirroring it while it was
// active, findings linked to it by identifier, and findings on the same
// resource with the same condition class. It does not look at what an
// investigation concluded, so a finding left at "needs attention" closes when
// the operator fixes the problem, and it does not need the resource to exist
// any more, so a container that was recreated does not leave its old finding
// open. A finding about something the alert did not state (a cause, a
// different condition) is left for the next run to judge. It returns how many
// findings it closed.
func (p *PatrolService) ResolveFindingsForClearedAlert(alertIdentifier, alertResourceID, alertType string) int {
	if p == nil || p.findings == nil {
		return 0
	}
	alertIdentifier = strings.TrimSpace(alertIdentifier)
	if alertIdentifier == "" {
		return 0
	}
	candidate := alertMirrorCandidate{ID: alertIdentifier, ResourceID: alertResourceID, Type: alertType}
	if strings.TrimSpace(alertResourceID) != "" && p.hasPatrolRuntimeInputs() {
		subjects := patrolAlertReferenceResolver(p.currentPatrolRuntimeState())
		if binding, _ := subjects.ResolveAlertResourceReference(alertResourceID); !binding.Component {
			candidate.CanonicalResourceID = binding.ResourceID
		}
	}

	p.mu.RLock()
	resolveUnified := p.unifiedFindingResolver
	p.mu.RUnlock()

	closed := 0
	for _, f := range p.findings.GetAll(nil) {
		if f == nil || f.ResolvedAt != nil || f.Source == stormFindingSource {
			continue
		}
		if strings.TrimSpace(f.MirrorsAlertID) != alertIdentifier && !findingMirrorsAlert(f, candidate) {
			continue
		}
		if !p.findings.ResolveWithReason(f.ID, clearedAlertResolveReason) {
			continue
		}
		closed++
		if resolveUnified != nil {
			resolveUnified(f.ID)
		}
	}
	return closed
}
