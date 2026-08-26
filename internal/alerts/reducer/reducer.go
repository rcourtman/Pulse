// Package reducer is the deterministic alert transition core scoped by
// docs/ALERT_ENGINE_EVOLUTION.md Phase 1. It reimplements the alert
// manager's transition semantics as a pure function over explicit state:
// no locks, no I/O, no wall clock — time enters only through the signal's
// ObservedAt. That determinism is what lets the parity harness in the
// alerts package diff this core against the live manager on identical
// input sequences, and what will let the lifecycle contract suite run
// exhaustively once families cut over (Phase 2).
//
// Scope of this slice: the metric-threshold family — hysteresis
// trigger/clear, sustained-for delay with dip reset, and warning/critical
// severity derivation. The semantics are characterized from
// Manager.checkMetric, not designed fresh; behavior differences are bugs
// here, never improvements. Notification policy (cooldown, quiet hours,
// rate limits) is out of scope by design: it stays a downstream consumer
// of transition events.
package reducer

import (
	"fmt"
	"time"
)

// MetricSignal is one observation of one metric on one resource.
type MetricSignal struct {
	ResourceID string
	// Key is the incident sub-key (the canonical metric spec ID). When
	// empty it falls back to Metric, which older callers and tests use.
	Key string
	// Metric is the metric name, used for percentage classification.
	Metric string
	Value  float64
	// RuntimeTick mirrors DiscreteSignal: a monotonic reading (valid when
	// RuntimeTickValid) that the intent gate accrues grace on.
	RuntimeTick      time.Duration
	RuntimeTickValid bool
	ObservedAt       time.Time
}

func (s MetricSignal) subKey() string {
	if s.Key != "" {
		return s.Key
	}
	return s.Metric
}

// MetricRule is the resolved threshold policy for one resource+metric.
// A Trigger <= 0 disables the rule: any pending or firing incident for the
// key is cleared, mirroring the manager's disabled-threshold path.
type MetricRule struct {
	Trigger float64
	// Clear is the hysteresis release threshold. When <= 0 it falls back to
	// Trigger, exactly as the manager does.
	Clear float64
	// Critical, when set, escalates severity at/above it (the canonical
	// spec family carries an explicit critical threshold). When nil, the
	// legacy derived escalation (trigger+10, percent-capped at 99) applies
	// unless CriticalDisabled — the canonical family omits escalation
	// entirely at triggers where the derived value would not exceed the
	// trigger.
	Critical         *float64
	CriticalDisabled bool
	// DelaySeconds is the sustained-above-trigger duration required before
	// firing. Zero fires on the first exceeding observation. Ignored when
	// an explicit Intent gate is supplied — the manager's explicit intent
	// policies replace the legacy time-threshold delay.
	DelaySeconds int
	// Intent is the resolved intent-policy context for this observation
	// (metric.<name> signals); nil means no gate.
	Intent *DiscreteIntent
}

// Severity mirrors the manager's two-level model.
type Severity string

const (
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

// IncidentState is the explicit lifecycle state of one incident.
type IncidentState string

const (
	// StatePending: the value is above trigger but the sustained-for delay
	// has not elapsed yet.
	StatePending IncidentState = "pending"
	// StateFiring: the incident is active.
	StateFiring IncidentState = "firing"
)

// Incident is the reducer's record for one resource+metric key. Absence
// from State means idle.
type Incident struct {
	ResourceID string
	// Key is the incident sub-key: the metric name for the metric family,
	// the discrete state key for the confirmation family.
	Key          string
	State        IncidentState
	Severity     Severity
	PendingSince time.Time
	StartedAt    time.Time
	LastValue    float64
	// Confirmations is the consecutive-match count for the confirmation
	// family; unused by the metric family.
	Confirmations int
	// RecoveryCount is the consecutive non-matching count while firing,
	// toward DiscreteRule.RecoveryConfirmations; reset by any matching
	// observation. Unused by the metric family.
	RecoveryCount int
	Acknowledged  bool
	AckUser       string
	AckAt         time.Time
	// Backup-run bookkeeping for the intent gate's backup-offline deferral
	// sub-policy (confirmation family only).
	BackupActive       bool
	BackupEnded        bool
	BackupEndedElapsed time.Duration
	// GraceElapsed accrues the intent gate's condition-active time from the
	// signal's monotonic RuntimeTick when supplied, making the gate immune
	// to wall-clock jumps exactly as the manager's intent machinery is.
	GraceElapsed    time.Duration
	TicksSupplied   bool
	LastRuntimeTick time.Duration
	LastObservedAt  time.Time
}

// EventType enumerates transition events emitted by Apply.
type EventType string

const (
	EventPending        EventType = "pending"
	EventPendingCleared EventType = "pending_cleared"
	EventFired          EventType = "fired"
	// EventRefired marks an activation that consumed a recently resolved
	// occurrence within RefireRetention: the incident keeps the original
	// occurrence's StartedAt, and downstream consumers treat it as the
	// same occurrence (the manager reactivates without a new history
	// entry, while still notifying).
	EventRefired         EventType = "refired"
	EventSeverityChanged EventType = "severity_changed"
	EventResolved        EventType = "resolved"
)

// RefireRetention is how long a resolved occurrence remains consumable by a
// re-fire, mirroring the manager's recentlyResolvedRetention. Time is
// measured on the signal's ObservedAt.
const RefireRetention = 5 * time.Minute

// AckRetention is how long an acknowledgement survives after its incident
// resolves: a re-activation inside this window starts acknowledged. The
// manager restores from its ack records with no age check and relies on
// cleanup pruning (one-hour inactive TTL) for expiry; the reducer draws
// that line deterministically at the same hour, measured on the signal's
// ObservedAt.
const AckRetention = time.Hour

// Event is one transition emitted by Apply.
type Event struct {
	Type       EventType
	ResourceID string
	Key        string
	Severity   Severity
	Value      float64
	At         time.Time
}

// resolvedRecord remembers one resolved occurrence so a re-fire inside
// RefireRetention can restore its start time (the confirmation family's
// canonical lifecycle behavior).
type resolvedRecord struct {
	StartedAt  time.Time
	ResolvedAt time.Time
}

// ackRecord preserves an acknowledgement across incident rebuilds and
// short resolve/re-fire cycles, mirroring the manager's canonical ack
// records.
type ackRecord struct {
	User       string
	At         time.Time
	InactiveAt time.Time // zero while the incident is still firing
}

// State holds every incident the reducer tracks. It is not safe for
// concurrent use; the owner serializes Apply calls.
type State struct {
	incidents map[string]*Incident
	resolved  map[string]resolvedRecord
	acks      map[string]ackRecord
}

// NewState returns an empty reducer state.
func NewState() *State {
	return &State{
		incidents: make(map[string]*Incident),
		resolved:  make(map[string]resolvedRecord),
		acks:      make(map[string]ackRecord),
	}
}

// Acknowledge marks a firing incident acknowledged and records the
// acknowledgement so rebuilds and short resolve/re-fire cycles keep it.
// Returns false when no firing incident exists for the key, mirroring the
// manager's not-found error.
func (s *State) Acknowledge(resourceID, subKey, user string, at time.Time) bool {
	key := incidentKey(resourceID, subKey)
	incident, ok := s.incidents[key]
	if !ok || incident.State != StateFiring {
		return false
	}
	if incident.Acknowledged {
		return true
	}
	incident.Acknowledged = true
	incident.AckUser = user
	incident.AckAt = at
	s.acks[key] = ackRecord{User: user, At: at}
	return true
}

// Unacknowledge removes an acknowledgement from a firing incident and
// deletes its record. Returns false when no firing incident exists.
func (s *State) Unacknowledge(resourceID, subKey string) bool {
	key := incidentKey(resourceID, subKey)
	incident, ok := s.incidents[key]
	if !ok || incident.State != StateFiring {
		return false
	}
	incident.Acknowledged = false
	incident.AckUser = ""
	incident.AckAt = time.Time{}
	delete(s.acks, key)
	return true
}

// markAckInactive stamps the ack record when its incident resolves, which
// starts the AckRetention window.
func (s *State) markAckInactive(key string, at time.Time) {
	if record, ok := s.acks[key]; ok && record.InactiveAt.IsZero() {
		record.InactiveAt = at
		s.acks[key] = record
	}
}

// restoreAck applies a preserved acknowledgement to a newly activated
// incident when its record is still inside AckRetention; expired records
// are dropped.
func (s *State) restoreAck(key string, incident *Incident, observedAt time.Time) {
	record, ok := s.acks[key]
	if !ok {
		return
	}
	if !record.InactiveAt.IsZero() && !record.InactiveAt.After(observedAt.Add(-AckRetention)) {
		delete(s.acks, key)
		return
	}
	// The record becomes live again with the incident.
	record.InactiveAt = time.Time{}
	s.acks[key] = record
	incident.Acknowledged = true
	incident.AckUser = record.User
	incident.AckAt = record.At
}

func incidentKey(resourceID, subKey string) string {
	return fmt.Sprintf("%s\x00%s", resourceID, subKey)
}

// Incident returns a copy of the tracked incident for the key, if any.
func (s *State) Incident(resourceID, subKey string) (Incident, bool) {
	if s == nil {
		return Incident{}, false
	}
	incident, ok := s.incidents[incidentKey(resourceID, subKey)]
	if !ok {
		return Incident{}, false
	}
	return *incident, true
}

// FiringCount reports how many incidents are currently firing.
func (s *State) FiringCount() int {
	count := 0
	for _, incident := range s.incidents {
		if incident.State == StateFiring {
			count++
		}
	}
	return count
}

// isPercentageMetric mirrors the manager's classification: these metrics
// live on a 0–100 scale, which caps the derived critical threshold.
func isPercentageMetric(metric string) bool {
	switch metric {
	case "cpu", "memory", "disk", "usage":
		return true
	default:
		return false
	}
}

// criticalThreshold mirrors the manager: critical is trigger+10, capped at
// 99 for percentage metrics so escalation stays reachable at high triggers.
func criticalThreshold(trigger float64, metric string) float64 {
	critical := trigger + 10
	if isPercentageMetric(metric) && critical > 99 {
		return 99
	}
	return critical
}

func severityFor(value float64, rule MetricRule, metric string) Severity {
	if rule.Critical != nil {
		if value >= *rule.Critical {
			return SeverityCritical
		}
		return SeverityWarning
	}
	if rule.CriticalDisabled {
		return SeverityWarning
	}
	if value >= criticalThreshold(rule.Trigger, metric) {
		return SeverityCritical
	}
	return SeverityWarning
}

// ApplyMetric advances the state for one metric observation under one rule
// and returns the transition events, in order. It is deterministic: the
// same state, signal, and rule always produce the same result.
func (s *State) ApplyMetric(signal MetricSignal, rule MetricRule) []Event {
	key := incidentKey(signal.ResourceID, signal.subKey())
	incident := s.incidents[key]

	event := func(eventType EventType, severity Severity) Event {
		return Event{
			Type:       eventType,
			ResourceID: signal.ResourceID,
			Key:        signal.subKey(),
			Severity:   severity,
			Value:      signal.Value,
			At:         signal.ObservedAt,
		}
	}

	// Disabled rule: drop pending, resolve firing. Mirrors the manager's
	// nil/zero-trigger clear path.
	if rule.Trigger <= 0 {
		if incident == nil {
			return nil
		}
		delete(s.incidents, key)
		if incident.State == StateFiring {
			s.markAckInactive(key, signal.ObservedAt)
			return []Event{event(EventResolved, incident.Severity)}
		}
		return []Event{event(EventPendingCleared, "")}
	}

	if signal.Value >= rule.Trigger {
		// Firing already: refresh value and re-derive severity. The manager
		// recomputes level on every tick, so severity can demote as well as
		// escalate while firing.
		if incident != nil && incident.State == StateFiring {
			previous := incident.Severity
			incident.Severity = severityFor(signal.Value, rule, signal.Metric)
			incident.LastValue = signal.Value
			incident.LastObservedAt = signal.ObservedAt
			if incident.Severity != previous {
				return []Event{event(EventSeverityChanged, incident.Severity)}
			}
			return nil
		}

		// The run enters or advances pending, then attempts activation.
		// An explicit Intent gate replaces the legacy sustained-for delay,
		// exactly as the manager's explicit intent policies replace the
		// time-threshold branch. StartedAt is the pending entry time,
		// matching the manager's use of the pending timestamp as the alert
		// start time.
		entered := false
		if incident == nil {
			incident = &Incident{
				ResourceID:      signal.ResourceID,
				Key:             signal.subKey(),
				State:           StatePending,
				PendingSince:    signal.ObservedAt,
				LastValue:       signal.Value,
				TicksSupplied:   signal.RuntimeTickValid,
				LastRuntimeTick: signal.RuntimeTick,
				LastObservedAt:  signal.ObservedAt,
			}
			s.incidents[key] = incident
			entered = true
		} else {
			incident.LastValue = signal.Value
			incident.LastObservedAt = signal.ObservedAt
			if signal.RuntimeTickValid {
				if incident.TicksSupplied && signal.RuntimeTick >= incident.LastRuntimeTick {
					incident.GraceElapsed += signal.RuntimeTick - incident.LastRuntimeTick
				}
				incident.TicksSupplied = true
				incident.LastRuntimeTick = signal.RuntimeTick
			}
		}

		pendingResult := func() []Event {
			if entered {
				return []Event{event(EventPending, "")}
			}
			return nil
		}

		if rule.Intent != nil {
			if intentHoldsActivation(rule.Intent, incident, signal.ObservedAt) {
				return pendingResult()
			}
		} else if rule.DelaySeconds > 0 {
			if signal.ObservedAt.Sub(incident.PendingSince) < time.Duration(rule.DelaySeconds)*time.Second {
				return pendingResult()
			}
		}

		severity := severityFor(signal.Value, rule, signal.Metric)
		incident.State = StateFiring
		incident.Severity = severity
		incident.StartedAt = incident.PendingSince
		s.restoreAck(key, incident, signal.ObservedAt)
		return []Event{event(EventFired, severity)}
	}

	// Below trigger.
	if incident == nil {
		return nil
	}

	if incident.State == StatePending {
		// A dip below trigger resets the sustained-for delay entirely.
		delete(s.incidents, key)
		return []Event{event(EventPendingCleared, "")}
	}

	clear := rule.Clear
	if clear <= 0 {
		clear = rule.Trigger
	}
	if signal.Value <= clear {
		severity := incident.Severity
		delete(s.incidents, key)
		s.markAckInactive(key, signal.ObservedAt)
		return []Event{event(EventResolved, severity)}
	}

	// Hysteresis hold: between clear and trigger the incident stays firing.
	// The manager does not refresh value or last-seen in this band; the
	// reducer mirrors that so parity diffs stay exact.
	return nil
}

// SeedFiringIncident installs a firing incident directly, bypassing the
// transition functions. Shadow mode uses it to align the reducer with
// pre-existing manager state (persisted-alert restore, divergence resync);
// it is not part of normal evaluation.
func (s *State) SeedFiringIncident(resourceID, subKey string, severity Severity, startedAt time.Time, acknowledged bool, ackUser string, ackAt time.Time) {
	key := incidentKey(resourceID, subKey)
	incident := &Incident{
		ResourceID:     resourceID,
		Key:            subKey,
		State:          StateFiring,
		Severity:       severity,
		StartedAt:      startedAt,
		Confirmations:  1,
		Acknowledged:   acknowledged,
		AckUser:        ackUser,
		AckAt:          ackAt,
		LastObservedAt: startedAt,
	}
	s.incidents[key] = incident
	if acknowledged {
		if ackAt.IsZero() {
			ackAt = startedAt
		}
		s.acks[key] = ackRecord{User: ackUser, At: ackAt}
	}
}

// Forget drops any incident for the key without emitting events — the
// mirror for manual clears and the shadow feed's divergence resync. A
// firing incident is recorded as a resolved occurrence (the manager's
// manual clear adds to recently-resolved the same way), so a quick
// re-fire reactivates it, and its acknowledgement enters the retention
// window.
func (s *State) Forget(resourceID, subKey string, at time.Time) {
	key := incidentKey(resourceID, subKey)
	if incident, ok := s.incidents[key]; ok {
		if incident.State == StateFiring {
			s.recordResolved(key, incident, at)
			s.markAckInactive(key, at)
		}
		delete(s.incidents, key)
	}
}

// ShiftResolved moves every resolved-occurrence timestamp by delta
// (negative = older). It exists for tests and simulations that age
// resolved records past RefireRetention; production code has no reason to
// rewrite history.
func (s *State) ShiftResolved(delta time.Duration) {
	for key, record := range s.resolved {
		record.ResolvedAt = record.ResolvedAt.Add(delta)
		s.resolved[key] = record
	}
}

// ShiftPending moves every pending run's start (and observation anchor) by
// delta (negative = older), for tests that simulate elapsed sustained-for
// time without waiting.
func (s *State) ShiftPending(delta time.Duration) {
	for _, incident := range s.incidents {
		if incident.State == StatePending {
			incident.PendingSince = incident.PendingSince.Add(delta)
		}
	}
}
