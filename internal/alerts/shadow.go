package alerts

// Shadow-mode runtime feed (docs/ALERT_ENGINE_EVOLUTION.md, Phase 1
// capstone): the deterministic reducer runs continuously against the same
// production observations the live manager evaluates, and every
// disagreement on an alert's state is counted and recorded in the alert
// event log as a shadow_divergence event. This converts the parity
// harness's test-time guarantee into an always-on invariant, and its
// divergence rate is the go/no-go evidence for each Phase 2 family
// cutover.
//
// Scope: the discrete confirmation family (connectivity, powered-state,
// discrete-state kinds) through evaluateCanonicalLifecycleAlert, the
// poll-driven recovery paths, manual acknowledge/unacknowledge/clear, and
// persisted-alert seeding. After reporting a divergence the reducer is
// resynced to the manager's state for that key, so one divergence
// produces one event rather than a stream — including divergences caused
// by manager mutations the feed does not observe.
//
// All shadow methods are NoLock: callers hold m.mu.

import (
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/eventlog"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/reducer"
	alertspecs "github.com/rcourtman/pulse-go-rewrite/internal/alerts/specs"
)

// shadowReportMinInterval rate-limits divergence events per key; the
// counter still counts every divergence.
const shadowReportMinInterval = 10 * time.Minute

type shadowFeed struct {
	state        *reducer.State
	divergences  atomic.Int64
	lastReported map[string]time.Time
}

// EnableShadowFeed starts the shadow reducer, seeded from the currently
// active canonical alerts so a restart does not read as mass divergence.
// The monitoring bootstrap calls this once per manager, after
// EnableEventLog.
func (m *Manager) EnableShadowFeed() {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	feed := &shadowFeed{
		state:        reducer.NewState(),
		lastReported: make(map[string]time.Time),
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
		feed.state.SeedFiringIncident(
			alert.ResourceID,
			alert.CanonicalSpecID,
			shadowSeverityForLevel(alert.Level),
			alert.StartTime,
			alert.Acknowledged,
			alert.AckUser,
			ackAt,
		)
	}
	m.shadow = feed
}

// ShadowDivergences reports how many shadow divergences have been observed
// since the feed was enabled.
func (m *Manager) ShadowDivergences() int64 {
	if m == nil || m.shadow == nil {
		return 0
	}
	return m.shadow.divergences.Load()
}

// shadowDiscreteObservation derives the reducer observation for the kinds
// the shadow feed covers. The match predicates mirror the spec evaluator's
// matches() for those kinds.
func shadowDiscreteObservation(spec alertspecs.ResourceAlertSpec, evidence alertspecs.AlertEvidence) (bool, bool) {
	switch spec.Kind {
	case alertspecs.AlertSpecKindConnectivity:
		if evidence.Connectivity == nil {
			return false, false
		}
		return !evidence.Connectivity.Connected, true
	case alertspecs.AlertSpecKindPoweredState:
		if evidence.PoweredState == nil {
			return false, false
		}
		return evidence.PoweredState.Observed != evidence.PoweredState.Expected, true
	case alertspecs.AlertSpecKindDiscreteState:
		if evidence.DiscreteState == nil || spec.DiscreteState == nil {
			return false, false
		}
		observed := strings.TrimSpace(evidence.DiscreteState.Observed)
		for _, trigger := range spec.DiscreteState.TriggerStates {
			if strings.TrimSpace(trigger) == observed {
				return true, true
			}
		}
		return false, true
	default:
		return false, false
	}
}

func shadowSeverityForLevel(level AlertLevel) reducer.Severity {
	if level == AlertLevelCritical {
		return reducer.SeverityCritical
	}
	return reducer.SeverityWarning
}

func shadowSpecSeverity(spec alertspecs.ResourceAlertSpec) reducer.Severity {
	if spec.Severity == alertspecs.AlertSeverityCritical {
		return reducer.SeverityCritical
	}
	return reducer.SeverityWarning
}

func specConfirmationsRequired(spec alertspecs.ResourceAlertSpec) int {
	if spec.ConfirmationsRequired > 0 {
		return spec.ConfirmationsRequired
	}
	switch spec.Kind {
	case alertspecs.AlertSpecKindConnectivity:
		return 3
	case alertspecs.AlertSpecKindPoweredState:
		return 2
	default:
		return 1
	}
}

// shadowObserveLifecycleNoLock feeds one canonical lifecycle observation to
// the shadow reducer and compares the outcome against the manager's state
// for the same key. intent carries the resolved intent context when the
// manager evaluated it this observation (nil otherwise).
func (m *Manager) shadowObserveLifecycleNoLock(
	spec alertspecs.ResourceAlertSpec,
	evidence alertspecs.AlertEvidence,
	intent *reducer.DiscreteIntent,
	managerAlertID string,
) {
	if m == nil || m.shadow == nil {
		return
	}
	matched, supported := shadowDiscreteObservation(spec, evidence)
	if !supported {
		return
	}
	observedAt := evidence.ObservedAt
	if observedAt.IsZero() {
		observedAt = m.policyNow()
	}
	m.shadow.state.ApplyDiscrete(reducer.DiscreteSignal{
		ResourceID:       spec.ResourceID,
		Key:              spec.ID,
		Matched:          matched,
		Severity:         shadowSpecSeverity(spec),
		RuntimeTick:      m.intentTickNoLock(),
		RuntimeTickValid: true,
		ObservedAt:       observedAt,
	}, reducer.DiscreteRule{
		Confirmations: specConfirmationsRequired(spec),
		Disabled:      spec.Disabled,
		Intent:        intent,
	})
	m.shadowCompareNoLock(spec.ResourceID, spec.ID, managerAlertID, observedAt)
}

// shadowObserveRecoveryNoLock feeds one healthy poll from the poll-driven
// offline recovery paths (which never reach the evaluator) to the shadow
// reducer.
func (m *Manager) shadowObserveRecoveryNoLock(resourceID, specKey, managerAlertID string, requiredRecovery int) {
	if m == nil || m.shadow == nil || resourceID == "" || specKey == "" {
		return
	}
	observedAt := m.policyNow()
	m.shadow.state.ApplyDiscrete(reducer.DiscreteSignal{
		ResourceID: resourceID,
		Key:        specKey,
		Matched:    false,
		ObservedAt: observedAt,
	}, reducer.DiscreteRule{RecoveryConfirmations: requiredRecovery})
	m.shadowCompareNoLock(resourceID, specKey, managerAlertID, observedAt)
}

// shadowCompareNoLock diffs the reducer's incident for a key against the
// manager's alert, records a divergence when they disagree, and resyncs
// the reducer to the manager so a single divergence produces a single
// event.
func (m *Manager) shadowCompareNoLock(resourceID, specKey, managerAlertID string, observedAt time.Time) {
	feed := m.shadow
	if feed == nil {
		return
	}

	alert, exists := m.getActiveAlertNoLock(managerAlertID)
	managerFiring := exists && alert != nil
	incident, ok := feed.state.Incident(resourceID, specKey)
	reducerFiring := ok && incident.State == reducer.StateFiring

	severityAgrees := true
	if managerFiring && reducerFiring {
		severityAgrees = shadowSeverityForLevel(alert.Level) == incident.Severity
	}
	if managerFiring == reducerFiring && severityAgrees {
		return
	}

	feed.divergences.Add(1)

	// Resync before rate limiting so state converges even when the report
	// is suppressed.
	if managerFiring {
		ackAt := time.Time{}
		if alert.AckTime != nil {
			ackAt = *alert.AckTime
		}
		feed.state.SeedFiringIncident(
			resourceID, specKey,
			shadowSeverityForLevel(alert.Level),
			alert.StartTime,
			alert.Acknowledged,
			alert.AckUser,
			ackAt,
		)
	} else {
		feed.state.Forget(resourceID, specKey, observedAt)
	}

	key := resourceID + "\x00" + specKey
	if last, reported := feed.lastReported[key]; reported && observedAt.Sub(last) < shadowReportMinInterval {
		return
	}
	feed.lastReported[key] = observedAt

	details := map[string]string{
		"specKey":       specKey,
		"managerFiring": fmt.Sprintf("%t", managerFiring),
		"reducerFiring": fmt.Sprintf("%t", reducerFiring),
	}
	if managerFiring {
		details["managerSeverity"] = string(alert.Level)
	}
	if reducerFiring {
		details["reducerSeverity"] = string(incident.Severity)
	}
	m.recordAlertEvent(
		eventlog.TypeShadowDivergence,
		alert,
		managerAlertID,
		"shadow_divergence",
		fmt.Sprintf("Shadow reducer disagreed with the live manager (manager firing=%t, reducer firing=%t).", managerFiring, reducerFiring),
		details,
	)
}
