package ai

import (
	"strings"
	"time"
)

// AlertExaminedSince reports whether a Patrol run that named this alert has
// started since the given time and did not end in an error. The alert bridge
// uses it so an alert that has stayed active is looked at once per episode
// even across a restart: the run history is persisted, the bridge's own
// bookkeeping is not.
func (p *PatrolService) AlertExaminedSince(alertIdentifier string, since time.Time) bool {
	alertIdentifier = strings.TrimSpace(alertIdentifier)
	if p == nil || p.runHistoryStore == nil || alertIdentifier == "" {
		return false
	}
	for _, run := range p.runHistoryStore.GetAll() {
		if run.AlertIdentifier != alertIdentifier || run.StartedAt.Before(since) {
			continue
		}
		// A cleared-alert check verified a different episode's end, and a run
		// that failed never looked.
		if run.TriggerReason == string(TriggerReasonAlertCleared) || strings.EqualFold(run.Status, "error") {
			continue
		}
		return true
	}
	return false
}
