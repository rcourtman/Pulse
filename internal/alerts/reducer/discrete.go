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
	// RecoveryConfirmations is the number of consecutive non-matching
	// observations required before a firing incident resolves. Values
	// <= 1 resolve on the first non-matching observation — the plain
	// evaluator behavior. The manager's poll-driven offline paths compose
	// this gate on top (default 3, storage 2), with any matching
	// observation resetting the run; pending incidents always clear on a
	// single non-matching observation regardless of this setting.
	RecoveryConfirmations int
	// Disabled clears any pending or firing incident immediately —
	// bypassing recovery confirmations, exactly as the manager's disable
	// paths call clearAlert directly.
	Disabled bool
	// Intent is the resolved intent-policy context for this observation;
	// nil means no gate. See DiscreteIntent.
	Intent *DiscreteIntent
}

// DiscreteIntent characterizes the manager's intent gate
// (evaluateIntentNoLock as composed by evaluateCanonicalLifecycleAlert):
// it holds ACTIVATION only — an already-firing incident is never
// suppressed by intent — and while held, the incident stays pending with
// the condition's first active observation preserved as the eventual
// start time. Operator context and policy are resolved by the caller per
// observation; the reducer only applies the gate. The backup-offline
// deferral sub-policy is not characterized yet (deferred slice).
type DiscreteIntent struct {
	// Explicit reports whether any intent rule applies to this
	// resource+signal; only explicit policies impose the grace gate.
	Explicit bool
	// GraceSeconds is how long the condition must have been active before
	// activation (explicit policies only). The manager accrues this on
	// monotonic process runtime; the reducer measures it on the signal
	// clock, which coincides under continuous operation.
	GraceSeconds int
	// OperatorSuppressed holds activation for operator context: an active
	// maintenance window, muted or retired monitoring, or expected-offline
	// for offline-family signals. The caller resolves it, honoring the
	// policy's HonorOperatorState (default true).
	OperatorSuppressed bool
	// OperatorReason names the suppression, e.g. "operator_maintenance".
	OperatorReason string
	// BackupEnabled arms the backup-offline deferral sub-policy for this
	// condition (offline-family signals with an explicit backupOffline
	// rule): while a backup is running the activation is deferred, bounded
	// by BackupMaxDeferralSeconds of total condition-active time; after
	// the backup ends the grace extends to the backup's end point plus
	// BackupPostGraceSeconds, still bounded by the cap.
	BackupEnabled bool
	// BackupActive is the observation-time backup context.
	BackupActive             bool
	BackupPostGraceSeconds   int
	BackupMaxDeferralSeconds int
}

// intentHoldsActivation reports whether the gate keeps a would-be
// activation pending, updating the incident's backup-run bookkeeping.
// Grace accrues from the condition run's first active observation,
// concurrently with any operator suppression — exactly as the manager
// accrues elapsed time while suppressed. The backup-offline sub-policy
// mirrors evaluateIntentNoLock: deferral while the backup runs, bounded
// by the max-deferral cap on total condition-active time; post-grace from
// the backup's end, still bounded by the cap.
func intentHoldsActivation(intent *DiscreteIntent, incident *Incident, observedAt time.Time) bool {
	if intent == nil {
		return false
	}
	if intent.OperatorSuppressed {
		return true
	}
	if !intent.Explicit {
		return false
	}
	elapsed := observedAt.Sub(incident.PendingSince)
	eligible := time.Duration(intent.GraceSeconds) * time.Second
	if intent.BackupEnabled {
		if intent.BackupActive {
			incident.BackupActive = true
			incident.BackupEnded = false
		} else if incident.BackupActive {
			incident.BackupActive = false
			incident.BackupEnded = true
			incident.BackupEndedElapsed = elapsed
		}
		deferralCap := time.Duration(intent.BackupMaxDeferralSeconds) * time.Second
		if incident.BackupActive && elapsed < deferralCap {
			return true
		}
		if incident.BackupEnded {
			post := incident.BackupEndedElapsed + time.Duration(intent.BackupPostGraceSeconds)*time.Second
			if post > eligible {
				eligible = post
			}
		}
		if eligible > deferralCap {
			eligible = deferralCap
		}
	}
	return elapsed < eligible
}

// recordResolved remembers a resolved occurrence for re-fire restoration.
func (s *State) recordResolved(key string, incident *Incident, resolvedAt time.Time) {
	s.resolved[key] = resolvedRecord{StartedAt: incident.StartedAt, ResolvedAt: resolvedAt}
}

// consumeRefire consumes a resolved occurrence still inside RefireRetention
// and returns its start time for restoration. A record too old to
// reactivate is dropped (the manager keeps it only for its history
// bookkeeping, then prunes it on the same retention).
func (s *State) consumeRefire(key string, observedAt time.Time) (time.Time, bool) {
	record, ok := s.resolved[key]
	if !ok {
		return time.Time{}, false
	}
	delete(s.resolved, key)
	if !record.ResolvedAt.After(observedAt.Add(-RefireRetention)) {
		return time.Time{}, false
	}
	return record.StartedAt, true
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

	// A disabled rule clears immediately: pending resets, firing resolves,
	// recovery confirmations do not apply.
	if rule.Disabled {
		if incident == nil {
			return nil
		}
		delete(s.incidents, key)
		if incident.State == StateFiring {
			s.recordResolved(key, incident, signal.ObservedAt)
			s.markAckInactive(key, signal.ObservedAt)
			return []Event{event(EventResolved, incident.Severity)}
		}
		return []Event{event(EventPendingCleared, "")}
	}

	if !signal.Matched {
		if incident == nil {
			return nil
		}
		// Pending always clears on a single non-matching observation.
		if incident.State != StateFiring {
			delete(s.incidents, key)
			return []Event{event(EventPendingCleared, "")}
		}
		// Firing: the recovery gate. Consecutive non-matching observations
		// count toward the requirement; the incident stays firing until it
		// is met.
		requiredRecovery := rule.RecoveryConfirmations
		if requiredRecovery <= 1 {
			delete(s.incidents, key)
			s.recordResolved(key, incident, signal.ObservedAt)
			s.markAckInactive(key, signal.ObservedAt)
			return []Event{event(EventResolved, incident.Severity)}
		}
		incident.RecoveryCount++
		incident.LastObservedAt = signal.ObservedAt
		if incident.RecoveryCount < requiredRecovery {
			return nil
		}
		delete(s.incidents, key)
		s.recordResolved(key, incident, signal.ObservedAt)
		s.markAckInactive(key, signal.ObservedAt)
		return []Event{event(EventResolved, incident.Severity)}
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
		// A matching observation interrupts any recovery run: recovery
		// confirmations must be consecutive.
		incident.RecoveryCount = 0
		incident.LastObservedAt = signal.ObservedAt
		if incident.Severity != previous {
			return []Event{event(EventSeverityChanged, incident.Severity)}
		}
		return nil
	}

	// Matching while pending (or first match): the run enters or advances
	// pending, then attempts activation. The intent gate holds activation
	// only — the incident stays pending with counts clamped at the
	// requirement, and the run's first active observation remains the
	// eventual start time (the manager uses FirstMatchedAt as the alert
	// start time).
	entered := false
	if incident == nil {
		incident = &Incident{
			ResourceID:     signal.ResourceID,
			Key:            signal.Key,
			State:          StatePending,
			Severity:       signal.Severity,
			PendingSince:   signal.ObservedAt,
			Confirmations:  1,
			LastObservedAt: signal.ObservedAt,
		}
		s.incidents[key] = incident
		entered = true
	} else {
		incident.Confirmations++
		incident.Severity = signal.Severity
		incident.LastObservedAt = signal.ObservedAt
	}

	pendingResult := func() []Event {
		if entered {
			return []Event{event(EventPending, "")}
		}
		return nil
	}

	if incident.Confirmations < required {
		return pendingResult()
	}
	incident.Confirmations = required

	if intentHoldsActivation(rule.Intent, incident, signal.ObservedAt) {
		return pendingResult()
	}

	incident.State = StateFiring
	incident.StartedAt = incident.PendingSince
	s.restoreAck(key, incident, signal.ObservedAt)
	if restored, ok := s.consumeRefire(key, signal.ObservedAt); ok {
		incident.StartedAt = restored
		return []Event{event(EventRefired, incident.Severity)}
	}
	return []Event{event(EventFired, incident.Severity)}
}
