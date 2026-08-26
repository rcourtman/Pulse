package alerts

// Authoritative reducer-core plumbing (docs/ALERT_ENGINE_EVOLUTION.md,
// Phase 2). The core owns transition state for the canonical lifecycle
// family; manager mutations that originate outside evaluation — user
// acknowledgements, manual clears, persisted-alert restore — are mirrored
// into it (and into the shadow feed when enabled) so both reducers track
// the same reality. All methods are NoLock: callers hold m.mu.

import (
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/reducer"
)

// seedReducerCoreNoLock installs the currently active canonical alerts as
// firing incidents so a restart resumes them instead of re-running their
// confirmations.
func (m *Manager) seedReducerCoreNoLock() {
	if m == nil || m.core == nil {
		return
	}
	for _, alert := range m.activeAlerts {
		if alert == nil {
			continue
		}
		backfillCanonicalIdentity(alert)
		if alert.ResourceID == "" || alert.CanonicalSpecID == "" {
			continue
		}
		ackAt := time.Time{}
		if alert.AckTime != nil {
			ackAt = *alert.AckTime
		}
		m.core.SeedFiringIncident(
			alert.ResourceID,
			alert.CanonicalSpecID,
			shadowSeverityForLevel(alert.Level),
			alert.StartTime,
			alert.Acknowledged,
			alert.AckUser,
			ackAt,
		)
	}
}

// mirrorStatesNoLock returns the reducer states that must observe manager
// mutations: the authoritative core first, then the shadow when enabled.
func (m *Manager) mirrorStatesNoLock() []*reducer.State {
	states := make([]*reducer.State, 0, 2)
	if m.core != nil {
		states = append(states, m.core)
	}
	if m.shadow != nil {
		states = append(states, m.shadow.state)
	}
	return states
}

// mirrorAcknowledgeNoLock mirrors a user acknowledgement.
func (m *Manager) mirrorAcknowledgeNoLock(alert *Alert, user string, at time.Time) {
	if m == nil || alert == nil {
		return
	}
	backfillCanonicalIdentity(alert)
	if alert.ResourceID == "" || alert.CanonicalSpecID == "" {
		return
	}
	for _, state := range m.mirrorStatesNoLock() {
		state.Acknowledge(alert.ResourceID, alert.CanonicalSpecID, user, at)
	}
}

// mirrorUnacknowledgeNoLock mirrors an acknowledgement removal.
func (m *Manager) mirrorUnacknowledgeNoLock(alert *Alert) {
	if m == nil || alert == nil {
		return
	}
	backfillCanonicalIdentity(alert)
	if alert.ResourceID == "" || alert.CanonicalSpecID == "" {
		return
	}
	for _, state := range m.mirrorStatesNoLock() {
		state.Unacknowledge(alert.ResourceID, alert.CanonicalSpecID)
	}
}

// mirrorForgetAlertNoLock mirrors a manual clear: the reducers drop the
// incident without treating it as an evaluated recovery.
func (m *Manager) mirrorForgetAlertNoLock(alert *Alert) {
	if m == nil || alert == nil {
		return
	}
	backfillCanonicalIdentity(alert)
	if alert.ResourceID == "" || alert.CanonicalSpecID == "" {
		return
	}
	for _, state := range m.mirrorStatesNoLock() {
		state.Forget(alert.ResourceID, alert.CanonicalSpecID, m.policyNow())
	}
}
