package reducer

// The confirmation (discrete-state) family characterizes the manager's
// canonical lifecycle path: alertspecs.Evaluate's match-spec semantics
// (connectivity, powered-state, discrete-state kinds) as wrapped by
// Manager.evaluateCanonicalLifecycleAlert. N consecutive matching
// observations activate; one non-matching observation clears at this
// layer (the manager's poll-driven offline paths add recovery
// confirmations as a separate gate, characterized in a later slice).
// Severity is carried by the rule/spec, re-derived on every observation
// while firing.

import "time"

// DiscreteSignal is one observation of a discrete condition on a resource:
// Matched reports whether the trigger condition was observed (offline,
// powered-state mismatch, state in the trigger set).
type DiscreteSignal struct {
	ResourceID string
	// Key identifies the condition, e.g. "connectivity" or a state key.
	Key        string
	Matched    bool
	Severity   Severity
	ObservedAt time.Time
}

// DiscreteRule is the resolved policy for one discrete condition.
type DiscreteRule struct {
	// Confirmations is the number of consecutive matching observations
	// required to fire. Values <= 0 mean 1, mirroring the evaluator's
	// floor. (The manager's spec defaults: connectivity 3, powered-state
	// 2; callers pass the resolved value.)
	Confirmations int
	// Disabled clears any pending or firing incident, mirroring the
	// evaluator's terminal disabled transition and the manager's resolve
	// of an existing alert.
	Disabled bool
}

// ApplyDiscrete advances the state for one discrete observation under one
// rule and returns the transition events, in order. Deterministic: time
// enters only through the signal's ObservedAt.
func (s *State) ApplyDiscrete(signal DiscreteSignal, rule DiscreteRule) []Event {
	key := incidentKey(signal.ResourceID, signal.Key)
	incident := s.incidents[key]

	event := func(eventType EventType, severity Severity) Event {
		return Event{
			Type:       eventType,
			ResourceID: signal.ResourceID,
			Key:        signal.Key,
			Severity:   severity,
			At:         signal.ObservedAt,
		}
	}

	// A non-matching observation and a disabled rule both clear at this
	// layer: pending resets entirely, firing resolves.
	if rule.Disabled || !signal.Matched {
		if incident == nil {
			return nil
		}
		delete(s.incidents, key)
		if incident.State == StateFiring {
			return []Event{event(EventResolved, incident.Severity)}
		}
		return []Event{event(EventPendingCleared, "")}
	}

	required := rule.Confirmations
	if required <= 0 {
		required = 1
	}

	// Matching while firing: stay firing, re-derive severity from the
	// rule's current severity, keep the confirmation count clamped at the
	// requirement — mirroring the evaluator's firing branch.
	if incident != nil && incident.State == StateFiring {
		previous := incident.Severity
		incident.Severity = signal.Severity
		incident.Confirmations = required
		incident.LastObservedAt = signal.ObservedAt
		if incident.Severity != previous {
			return []Event{event(EventSeverityChanged, incident.Severity)}
		}
		return nil
	}

	// Matching while pending: count consecutive matches; fire once the
	// requirement is met, with StartedAt at the first matched observation
	// (the manager uses FirstMatchedAt as the alert start time).
	if incident != nil {
		incident.Confirmations++
		incident.Severity = signal.Severity
		incident.LastObservedAt = signal.ObservedAt
		if incident.Confirmations < required {
			return nil
		}
		incident.State = StateFiring
		incident.Confirmations = required
		incident.StartedAt = incident.PendingSince
		return []Event{event(EventFired, incident.Severity)}
	}

	// First matching observation.
	if required <= 1 {
		s.incidents[key] = &Incident{
			ResourceID:     signal.ResourceID,
			Key:            signal.Key,
			State:          StateFiring,
			Severity:       signal.Severity,
			StartedAt:      signal.ObservedAt,
			Confirmations:  1,
			LastObservedAt: signal.ObservedAt,
		}
		return []Event{event(EventFired, signal.Severity)}
	}
	s.incidents[key] = &Incident{
		ResourceID:     signal.ResourceID,
		Key:            signal.Key,
		State:          StatePending,
		Severity:       signal.Severity,
		PendingSince:   signal.ObservedAt,
		Confirmations:  1,
		LastObservedAt: signal.ObservedAt,
	}
	return []Event{event(EventPending, "")}
}
