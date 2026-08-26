package alerts

import (
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/eventlog"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/reducer"
	alertspecs "github.com/rcourtman/pulse-go-rewrite/internal/alerts/specs"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rs/zerolog/log"
)

type canonicalLifecycleAlertParams struct {
	Spec                 alertspecs.ResourceAlertSpec
	Evidence             alertspecs.AlertEvidence
	IntentSignal         string
	PolicyDisabledNoLock func() bool
	Tracking             map[string]int
	TrackingKey          string
	AlertID              string
	AlertType            string
	ResourceID           string
	ResourceName         string
	Node                 string
	Instance             string
	Message              string
	Metadata             map[string]interface{}
	AddToRecent          bool
	AddToHistory         bool
	RateLimit            bool
	DispatchAsync        bool
	IntentBackup         BackupIntentContext
}

type canonicalStatefulAlertParams struct {
	Spec                         alertspecs.ResourceAlertSpec
	Evidence                     alertspecs.AlertEvidence
	PendingTracking              map[string]time.Time
	PendingKey                   string
	AlertID                      string
	AlertType                    string
	ResourceID                   string
	ResourceName                 string
	Node                         string
	Instance                     string
	Message                      string
	Value                        float64
	Threshold                    float64
	StartTimeOverride            time.Time
	Metadata                     map[string]interface{}
	AddToRecent                  bool
	AddToHistory                 bool
	MessageBuilder               func(alertspecs.EvaluationResult) (string, float64, float64)
	RateLimit                    bool
	NotifyOnSeverityChange       bool
	AddToHistoryOnSeverityChange bool
	DispatchAsync                bool
}

func buildCanonicalConnectivitySpec(resourceID, title string, resourceType unifiedresources.ResourceType, severity AlertLevel, confirmations int, disabled bool) (alertspecs.ResourceAlertSpec, error) {
	spec := alertspecs.ResourceAlertSpec{
		ID:                    resourceID + "-connectivity",
		ResourceID:            resourceID,
		ResourceType:          resourceType,
		Kind:                  alertspecs.AlertSpecKindConnectivity,
		Severity:              canonicalAlertSeverity(severity),
		Title:                 title,
		Disabled:              disabled,
		ConfirmationsRequired: confirmations,
		Connectivity: &alertspecs.ConnectivitySpec{
			Signal:    "status",
			LostAfter: time.Second,
		},
	}

	return spec, spec.Validate()
}

func buildCanonicalPoweredStateSpec(resourceID, title string, resourceType unifiedresources.ResourceType, severity AlertLevel, confirmations int, disabled bool) (alertspecs.ResourceAlertSpec, error) {
	spec := alertspecs.ResourceAlertSpec{
		ID:                    resourceID + "-powered-state",
		ResourceID:            resourceID,
		ResourceType:          resourceType,
		Kind:                  alertspecs.AlertSpecKindPoweredState,
		Severity:              canonicalAlertSeverity(severity),
		Title:                 title,
		Disabled:              disabled,
		ConfirmationsRequired: confirmations,
		PoweredState: &alertspecs.PoweredStateSpec{
			Expected: alertspecs.PowerStateOn,
		},
	}

	return spec, spec.Validate()
}

func buildCanonicalDiscreteStateSpec(resourceID, title string, resourceType unifiedresources.ResourceType, severity AlertLevel, confirmations int, disabled bool, stateKey string, triggerStates []string) (alertspecs.ResourceAlertSpec, error) {
	spec := alertspecs.ResourceAlertSpec{
		ID:                    resourceID + "-" + stateKey,
		ResourceID:            resourceID,
		ResourceType:          resourceType,
		Kind:                  alertspecs.AlertSpecKindDiscreteState,
		Severity:              canonicalAlertSeverity(severity),
		Title:                 title,
		Disabled:              disabled,
		ConfirmationsRequired: confirmations,
		DiscreteState: &alertspecs.DiscreteStateSpec{
			StateKey:      stateKey,
			TriggerStates: append([]string(nil), triggerStates...),
		},
	}

	return spec, spec.Validate()
}

func buildCanonicalServiceGapSpec(resourceID, title string, resourceType unifiedresources.ResourceType, service string, warningPercent, criticalPercent float64, disabled bool) (alertspecs.ResourceAlertSpec, error) {
	if criticalPercent > 0 && warningPercent > 0 && criticalPercent < warningPercent {
		warningPercent = criticalPercent
	}
	spec := alertspecs.ResourceAlertSpec{
		ID:           resourceID + "-service-gap",
		ResourceID:   resourceID,
		ResourceType: resourceType,
		Kind:         alertspecs.AlertSpecKindServiceGap,
		Severity:     alertspecs.AlertSeverityWarning,
		Title:        title,
		Disabled:     disabled,
		ServiceGap: &alertspecs.ServiceGapSpec{
			Service:         service,
			WarningPercent:  warningPercent,
			CriticalPercent: criticalPercent,
		},
	}

	return spec, spec.Validate()
}

func buildCanonicalSeverityThresholdSpecWithDirection(specID, resourceID, title string, resourceType unifiedresources.ResourceType, metric string, direction alertspecs.ThresholdDirection, warning, critical float64, disabled bool) (alertspecs.ResourceAlertSpec, error) {
	spec := alertspecs.ResourceAlertSpec{
		ID:           specID,
		ResourceID:   resourceID,
		ResourceType: resourceType,
		Kind:         alertspecs.AlertSpecKindSeverityThreshold,
		Severity:     alertspecs.AlertSeverityWarning,
		Title:        title,
		Disabled:     disabled,
		SeverityThreshold: &alertspecs.SeverityThresholdSpec{
			Metric:    metric,
			Direction: direction,
			Warning:   warning,
			Critical:  critical,
		},
	}

	return spec, spec.Validate()
}

func buildCanonicalSeverityThresholdSpec(specID, resourceID, title string, resourceType unifiedresources.ResourceType, metric string, warning, critical float64, disabled bool) (alertspecs.ResourceAlertSpec, error) {
	return buildCanonicalSeverityThresholdSpecWithDirection(specID, resourceID, title, resourceType, metric, alertspecs.ThresholdDirectionAbove, warning, critical, disabled)
}

func buildCanonicalSeverityThresholdSpecWithRecovery(specID, resourceID, title string, resourceType unifiedresources.ResourceType, metric string, warning, critical float64, recovery *float64, disabled bool) (alertspecs.ResourceAlertSpec, error) {
	spec, err := buildCanonicalSeverityThresholdSpec(specID, resourceID, title, resourceType, metric, warning, critical, disabled)
	if err != nil {
		return spec, err
	}
	spec.SeverityThreshold.Recovery = recovery
	return spec, spec.Validate()
}

func buildCanonicalChangeThresholdSpec(specID, resourceID, title string, resourceType unifiedresources.ResourceType, metric string, warningCurrent, criticalCurrent, warningDelta, criticalDelta, warningPercent, criticalPercent float64, window time.Duration, disabled bool) (alertspecs.ResourceAlertSpec, error) {
	spec := alertspecs.ResourceAlertSpec{
		ID:           specID,
		ResourceID:   resourceID,
		ResourceType: resourceType,
		Kind:         alertspecs.AlertSpecKindChangeThreshold,
		Severity:     alertspecs.AlertSeverityWarning,
		Title:        title,
		Disabled:     disabled,
		ChangeThreshold: &alertspecs.ChangeThresholdSpec{
			Metric:          metric,
			ReferenceWindow: window,
			WarningCurrent:  warningCurrent,
			CriticalCurrent: criticalCurrent,
			WarningDelta:    warningDelta,
			CriticalDelta:   criticalDelta,
			WarningPercent:  warningPercent,
			CriticalPercent: criticalPercent,
		},
	}

	return spec, spec.Validate()
}

func buildCanonicalBaselineAnomalySpec(specID, resourceID, title string, resourceType unifiedresources.ResourceType, metric string, confirmations int, disabled bool) (alertspecs.ResourceAlertSpec, error) {
	spec := alertspecs.ResourceAlertSpec{
		ID:                    specID,
		ResourceID:            resourceID,
		ResourceType:          resourceType,
		Kind:                  alertspecs.AlertSpecKindBaselineAnomaly,
		Severity:              alertspecs.AlertSeverityWarning,
		Title:                 title,
		Disabled:              disabled,
		ConfirmationsRequired: confirmations,
		BaselineAnomaly: &alertspecs.BaselineAnomalySpec{
			Metric:             metric,
			QuietBaseline:      40,
			WarningRatio:       1.8,
			CriticalRatio:      2.5,
			WarningDelta:       150,
			CriticalDelta:      300,
			QuietWarningDelta:  60,
			QuietCriticalDelta: 120,
		},
	}

	return spec, spec.Validate()
}

func buildCanonicalHealthAssessmentSpec(specID, resourceID, title string, resourceType unifiedresources.ResourceType, signal string, codes []string, disabled bool) (alertspecs.ResourceAlertSpec, error) {
	spec := alertspecs.ResourceAlertSpec{
		ID:           specID,
		ResourceID:   resourceID,
		ResourceType: resourceType,
		Kind:         alertspecs.AlertSpecKindHealthAssessment,
		Severity:     alertspecs.AlertSeverityWarning,
		Title:        title,
		Disabled:     disabled,
		HealthAssessment: &alertspecs.HealthAssessmentSpec{
			Signal: signal,
			Codes:  append([]string(nil), codes...),
		},
	}

	return spec, spec.Validate()
}

func buildCanonicalPostureThresholdSpec(specID, resourceID, title string, resourceType unifiedresources.ResourceType, ageMetric string, warningAge, criticalAge float64, sizeMetric string, warningSize, criticalSize float64, disabled bool) (alertspecs.ResourceAlertSpec, error) {
	spec := alertspecs.ResourceAlertSpec{
		ID:           specID,
		ResourceID:   resourceID,
		ResourceType: resourceType,
		Kind:         alertspecs.AlertSpecKindPostureThreshold,
		Severity:     alertspecs.AlertSeverityWarning,
		Title:        title,
		Disabled:     disabled,
		PostureThreshold: &alertspecs.PostureThresholdSpec{
			AgeMetric:    ageMetric,
			WarningAge:   warningAge,
			CriticalAge:  criticalAge,
			SizeMetric:   sizeMetric,
			WarningSize:  warningSize,
			CriticalSize: criticalSize,
		},
	}

	return spec, spec.Validate()
}

func canonicalAlertSeverity(level AlertLevel) alertspecs.AlertSeverity {
	switch level {
	case AlertLevelCritical:
		return alertspecs.AlertSeverityCritical
	default:
		return alertspecs.AlertSeverityWarning
	}
}

func statefulPreviousState(spec alertspecs.ResourceAlertSpec, existing *Alert, pendingSince time.Time) alertspecs.EvaluatorState {
	if existing != nil {
		return alertspecs.EvaluatorState{
			SpecID:         spec.ID,
			State:          alertspecs.AlertStateFiring,
			Severity:       canonicalAlertSeverity(existing.Level),
			Reason:         "",
			ActiveSince:    existing.StartTime,
			FirstMatchedAt: existing.StartTime,
			LastObservedAt: existing.LastSeen,
		}
	}
	if !pendingSince.IsZero() {
		return alertspecs.EvaluatorState{
			SpecID:             spec.ID,
			State:              alertspecs.AlertStatePending,
			Severity:           spec.Severity,
			ConsecutiveMatches: 1,
			FirstMatchedAt:     pendingSince,
			LastObservedAt:     pendingSince,
		}
	}
	return alertspecs.EvaluatorState{
		SpecID: spec.ID,
		State:  alertspecs.AlertStateClear,
	}
}

func (m *Manager) evaluateCanonicalLifecycleAlert(params canonicalLifecycleAlertParams) (alertspecs.EvaluationResult, bool) {
	if params.Evidence.ObservedAt.IsZero() {
		params.Evidence.ObservedAt = time.Now()
	}

	m.mu.Lock()
	migratedAlertIdentity := false
	defer func() {
		if migratedAlertIdentity {
			m.saveActiveAlertsAsync("guest lifecycle alert node move")
		}
	}()
	defer m.mu.Unlock()

	// Recheck mutable policy while holding the same lock used for lifecycle
	// state and dispatch. This closes the save-vs-dispatch race where policy
	// could be disabled after a detector's initial snapshot but before it
	// activated the alert.
	if params.PolicyDisabledNoLock != nil && params.PolicyDisabledNoLock() {
		params.Spec.Disabled = true
	}

	storageKey := canonicalTrackingKeyForSpec(params.Spec, params.AlertID)
	trackingKey := storageKey

	// Shadow feed: after this evaluation settles (any exit), replay the same
	// observation through the shadow reducer and compare outcomes. Runs
	// before the deferred unlock (LIFO), so it still holds m.mu.
	var shadowIntent *reducer.DiscreteIntent
	shadowEligible := false
	defer func() {
		if shadowEligible {
			m.shadowObserveLifecycleNoLock(params.Spec, params.Evidence, shadowIntent, storageKey)
		}
	}()

	var existing *Alert
	if current, ok := m.getActiveAlertNoLock(storageKey); ok {
		existing = current
	} else if migrated := m.migrateGuestAlertNoLock(storageKey, params.Spec.ID, string(params.Spec.Kind), params.Spec.ResourceID, params.ResourceName, params.Node, params.Instance, string(params.Spec.ResourceType)); migrated != nil {
		existing = migrated
		migratedAlertIdentity = true
	}

	// Validate the spec and evidence exactly as the evaluator did, so
	// malformed input is skipped rather than misread as a recovery.
	if err := params.Spec.Validate(); err != nil {
		log.Warn().
			Err(err).
			Str("alertID", storageKey).
			Str("resourceID", params.ResourceID).
			Str("specID", params.Spec.ID).
			Msg("Skipping invalid canonical lifecycle evaluation")
		return alertspecs.EvaluationResult{}, false
	}
	if err := alertspecs.ValidateEvidence(params.Spec.Kind, params.Evidence); err != nil {
		log.Warn().
			Err(err).
			Str("alertID", storageKey).
			Str("resourceID", params.ResourceID).
			Str("specID", params.Spec.ID).
			Msg("Skipping invalid canonical lifecycle evidence")
		return alertspecs.EvaluationResult{}, false
	}

	matched, matchSeverity, matchReason := alertspecs.Match(params.Spec, params.Evidence)

	// Intent context resolves before the transition, exactly when the old
	// engine consulted it: only while no alert is active. Its bookkeeping
	// (intentPending, preview surfaces) is preserved; the gate itself now
	// lives in the reducer.
	intentSignal := params.IntentSignal
	if intentSignal == "" && (params.Spec.Kind == alertspecs.AlertSpecKindConnectivity || params.Spec.Kind == alertspecs.AlertSpecKindPoweredState) {
		intentSignal = string(AlertIntentSignalOffline)
	}
	var intent *reducer.DiscreteIntent
	if intentSignal != "" && existing == nil {
		decision := m.evaluateIntentNoLock(params.Spec.ResourceID, string(params.Spec.ResourceType), intentSignal, storageKey, params.Evidence.ObservedAt, matched, params.IntentBackup)
		intent = &reducer.DiscreteIntent{
			Explicit:     decision.Effective.Explicit,
			GraceSeconds: decision.Effective.GraceSeconds,
			// Operator suppression only: the backup-offline deferral is
			// modeled independently below so the reducer computes its own
			// hold rather than echoing the manager's.
			OperatorSuppressed: decision.Suppressed && strings.HasPrefix(decision.Reason, "operator_"),
			OperatorReason:     decision.Reason,
		}
		if backupOffline := decision.Effective.BackupOffline; backupOffline != nil && backupOffline.Enabled && intentSignal == string(AlertIntentSignalOffline) {
			intent.BackupEnabled = true
			intent.BackupActive = params.IntentBackup.Active
			intent.BackupPostGraceSeconds = backupOffline.PostGraceSeconds
			intent.BackupMaxDeferralSeconds = backupOffline.MaxDeferralSeconds
		}
		shadowIntent = intent
		if decision.StateChanged {
			m.saveActiveAlertsAsync("lifecycle intent state")
		}
	}

	severity := reducer.SeverityWarning
	if matchSeverity == alertspecs.AlertSeverityCritical {
		severity = reducer.SeverityCritical
	}

	// An existing alert the core does not know about (guest migration,
	// direct injection, any unhooked path) means it was firing: adopt it,
	// exactly as the old engine reconstructed previous state as firing
	// whenever an active alert existed.
	if existing != nil {
		if _, known := m.core.Incident(params.Spec.ResourceID, params.Spec.ID); !known {
			ackAt := time.Time{}
			if existing.AckTime != nil {
				ackAt = *existing.AckTime
			}
			m.core.SeedFiringIncident(
				params.Spec.ResourceID,
				params.Spec.ID,
				shadowSeverityForLevel(existing.Level),
				existing.StartTime,
				existing.Acknowledged,
				existing.AckUser,
				ackAt,
			)
		}
	}

	events := m.core.ApplyDiscrete(reducer.DiscreteSignal{
		ResourceID:       params.Spec.ResourceID,
		Key:              params.Spec.ID,
		Matched:          matched,
		Severity:         severity,
		RuntimeTick:      m.intentTickNoLock(),
		RuntimeTickValid: true,
		ObservedAt:       params.Evidence.ObservedAt,
	}, reducer.DiscreteRule{
		Confirmations: specConfirmationsRequired(params.Spec),
		Disabled:      params.Spec.Disabled,
		Intent:        intent,
	})
	primary := reducer.EventType("")
	if len(events) > 0 {
		primary = events[0].Type
	}

	shadowEligible = true

	incident, hasIncident := m.core.Incident(params.Spec.ResourceID, params.Spec.ID)

	// The legacy count maps are maintained as read-only mirrors of the core
	// during the Phase 2 transition, for external readers; the engine never
	// consults them. They are deleted with the Phase 3 cleanup.
	if params.Tracking != nil {
		if hasIncident {
			params.Tracking[params.TrackingKey] = incident.Confirmations
		} else {
			delete(params.Tracking, params.TrackingKey)
		}
	}

	// Synthesize the evaluator-shaped result the callers consume.
	result := alertspecs.EvaluationResult{}
	result.State.SpecID = params.Spec.ID
	result.State.LastObservedAt = params.Evidence.ObservedAt
	result.State.Reason = matchReason

	transition := func(kind alertspecs.EvaluationTransitionKind, from, to alertspecs.AlertState) *alertspecs.EvaluationTransition {
		return &alertspecs.EvaluationTransition{
			Kind:       kind,
			SpecID:     params.Spec.ID,
			ResourceID: params.Spec.ResourceID,
			From:       from,
			To:         to,
			At:         params.Evidence.ObservedAt,
			Severity:   matchSeverity,
			Reason:     matchReason,
			Evidence:   params.Evidence,
		}
	}

	if hasIncident && incident.State == reducer.StatePending {
		result.State.State = alertspecs.AlertStatePending
		result.State.Severity = matchSeverity
		result.State.ConsecutiveMatches = incident.Confirmations
		result.State.FirstMatchedAt = incident.PendingSince
		if intent != nil && intent.OperatorReason != "" {
			result.State.Reason = intent.OperatorReason
		}
		if primary == reducer.EventPending {
			result.Transition = transition(alertspecs.EvaluationTransitionPending, alertspecs.AlertStateClear, alertspecs.AlertStatePending)
		}
		return result, true
	}

	if hasIncident && incident.State == reducer.StateFiring {
		level := AlertLevelWarning
		if incident.Severity == reducer.SeverityCritical {
			level = AlertLevelCritical
		}
		alert := &Alert{
			ID:           storageKey,
			Type:         params.AlertType,
			Level:        level,
			ResourceID:   params.Spec.ResourceID,
			ResourceName: params.ResourceName,
			Node:         params.Node,
			Instance:     params.Instance,
			Message:      params.Message,
			Value:        0,
			Threshold:    0,
			StartTime:    incident.StartedAt,
			LastSeen:     params.Evidence.ObservedAt,
			Metadata:     cloneMetadata(params.Metadata),
		}
		if alert.Metadata == nil {
			alert.Metadata = make(map[string]interface{}, 2)
		}
		if _, ok := alert.Metadata["resourceType"]; !ok && params.Spec.ResourceType != "" {
			alert.Metadata["resourceType"] = string(params.Spec.ResourceType)
		}
		applyCanonicalIdentity(alert, params.Spec.ID, string(params.Spec.Kind))
		applyCanonicalOperationalEvidence(alert, params.Spec, params.Evidence, time.Now())
		m.preserveAlertState(storageKey, alert)
		// The reducer core is authoritative for occurrence start and
		// acknowledgement restoration.
		alert.StartTime = incident.StartedAt
		alert.Acknowledged = incident.Acknowledged
		alert.AckUser = incident.AckUser
		if incident.Acknowledged && !incident.AckAt.IsZero() {
			ackAt := incident.AckAt
			alert.AckTime = &ackAt
		} else if !incident.Acknowledged {
			alert.AckTime = nil
			alert.AckUser = ""
		}
		m.setActiveAlertNoLock(storageKey, alert)
		if params.AddToRecent {
			m.recentAlerts[trackingKey] = alert
		}

		result.State.State = alertspecs.AlertStateFiring
		result.State.Severity = matchSeverity
		result.State.ConsecutiveMatches = incident.Confirmations
		result.State.FirstMatchedAt = incident.StartedAt
		result.State.ActiveSince = incident.StartedAt

		if existing != nil {
			if primary == reducer.EventSeverityChanged {
				result.Transition = transition(alertspecs.EvaluationTransitionSeverityChanged, alertspecs.AlertStateFiring, alertspecs.AlertStateFiring)
			}
			return result, true
		}

		// Activation. Intent bookkeeping closes out exactly as before.
		if intent != nil && intent.Explicit {
			m.clearIntentPendingNoLock(storageKey)
			m.saveActiveAlertsAsync("lifecycle intent activated")
		}
		result.Transition = transition(alertspecs.EvaluationTransitionActivated, alertspecs.AlertStatePending, alertspecs.AlertStateFiring)

		firedType := eventlog.TypeFired
		if primary == reducer.EventRefired {
			firedType = eventlog.TypeRefired
		}
		m.recordAlertEvent(firedType, alert, storageKey, matchReason, params.Message, nil)

		// Consume the recently-resolved entry for map hygiene and the
		// new-occurrence history distinction; the reducer already decided
		// whether this is a reactivation (EventRefired) and restored the
		// start time.
		_, _, _, hadResolvedOccurrence := m.consumeRecentlyResolvedForRefireWithPrimaryLock(storageKey, time.Now())
		if primary == reducer.EventRefired {
			if params.AddToHistory {
				m.historyManager.UpdateAlertLastSeenForAlert(alert, alert.LastSeen)
			}
			log.Debug().
				Str("alertID", storageKey).
				Msg("Alert re-fired within cooldown, reactivated without new history entry")
			if params.RateLimit && !m.checkRateLimit(trackingKey) {
				return result, true
			}
			m.dispatchAlert(alert, params.DispatchAsync)
			return result, true
		}

		if params.AddToHistory {
			if hadResolvedOccurrence {
				m.historyManager.AddAlertTransition(*alert)
			} else {
				m.historyManager.AddAlert(*alert)
			}
		}

		if params.RateLimit && !m.checkRateLimit(trackingKey) {
			log.Debug().
				Str("alertID", storageKey).
				Str("trackingKey", trackingKey).
				Int("maxPerHour", m.config.Schedule.MaxAlertsHour).
				Msg("Lifecycle alert notification suppressed due to rate limit")
			return result, true
		}

		m.dispatchAlert(alert, params.DispatchAsync)
		return result, true
	}

	// Clear: the core holds no incident for this key.
	result.State.State = alertspecs.AlertStateClear
	if existing == nil {
		return result, true
	}

	result.Transition = transition(alertspecs.EvaluationTransitionRecovered, alertspecs.AlertStateFiring, alertspecs.AlertStateClear)
	m.removeActiveAlertNoLock(storageKey)
	recoveryEvidence, hasRecoveryEvidence := canonicalAlertEvidenceEnvelope(
		params.Spec,
		params.Evidence,
		existing.Instance,
		time.Now(),
	)
	var recoveryEvidenceRef *operationaltrust.EvidenceEnvelope
	if hasRecoveryEvidence {
		recoveryEvidenceRef = &recoveryEvidence
	}
	resolvedAlert := m.newResolvedAlert(
		existing,
		params.Evidence.ObservedAt,
		recoveryEvidenceRef,
	)
	m.addRecentlyResolvedWithPrimaryLock(resolvedAlert)
	m.safeCallResolvedAlertCallback(existing, storageKey, true)
	return result, true
}

func (m *Manager) evaluateCanonicalStatefulAlert(params canonicalStatefulAlertParams) (alertspecs.EvaluationResult, bool) {
	if params.Evidence.ObservedAt.IsZero() {
		params.Evidence.ObservedAt = time.Now()
	}

	m.mu.Lock()
	migratedAlertIdentity := false
	defer func() {
		if migratedAlertIdentity {
			m.saveActiveAlertsAsync("guest stateful alert node move")
		}
	}()
	defer m.mu.Unlock()

	storageKey := canonicalTrackingKeyForSpec(params.Spec, params.AlertID)
	trackingKey := storageKey

	var existing *Alert
	if current, ok := m.getActiveAlertNoLock(storageKey); ok {
		existing = current
	} else if migrated := m.migrateGuestAlertNoLock(storageKey, params.Spec.ID, string(params.Spec.Kind), params.Spec.ResourceID, params.ResourceName, params.Node, params.Instance, string(params.Spec.ResourceType)); migrated != nil {
		existing = migrated
		migratedAlertIdentity = true
	}

	var pendingSince time.Time
	if params.PendingTracking != nil {
		pendingSince = params.PendingTracking[params.PendingKey]
	}

	result, err := alertspecs.Evaluate(params.Spec, statefulPreviousState(params.Spec, existing, pendingSince), params.Evidence)
	if err != nil {
		log.Warn().
			Err(err).
			Str("alertID", storageKey).
			Str("resourceID", params.ResourceID).
			Str("specID", params.Spec.ID).
			Msg("Skipping invalid canonical stateful evaluation")
		return alertspecs.EvaluationResult{}, false
	}

	if params.PendingTracking != nil {
		switch result.State.State {
		case alertspecs.AlertStatePending:
			if pendingSince.IsZero() {
				params.PendingTracking[params.PendingKey] = params.Evidence.ObservedAt
			}
		default:
			delete(params.PendingTracking, params.PendingKey)
		}
	}

	switch result.State.State {
	case alertspecs.AlertStatePending:
		return result, true
	case alertspecs.AlertStateFiring:
		level, ok := alertLevelFromCanonicalSeverity(result.State.Severity)
		if !ok {
			level = AlertLevelWarning
		}
		message := params.Message
		value := params.Value
		threshold := params.Threshold
		if params.MessageBuilder != nil {
			message, value, threshold = params.MessageBuilder(result)
		}
		startTime := params.Evidence.ObservedAt
		if !params.StartTimeOverride.IsZero() {
			startTime = params.StartTimeOverride
		}
		alert := &Alert{
			ID:           storageKey,
			Type:         params.AlertType,
			Level:        level,
			ResourceID:   params.Spec.ResourceID,
			ResourceName: params.ResourceName,
			Node:         params.Node,
			Instance:     params.Instance,
			Message:      message,
			Value:        value,
			Threshold:    threshold,
			StartTime:    startTime,
			LastSeen:     params.Evidence.ObservedAt,
			Metadata:     cloneMetadata(params.Metadata),
		}
		if alert.Metadata == nil {
			alert.Metadata = make(map[string]interface{}, 2)
		}
		if _, ok := alert.Metadata["resourceType"]; !ok && params.Spec.ResourceType != "" {
			alert.Metadata["resourceType"] = string(params.Spec.ResourceType)
		}
		applyCanonicalIdentity(alert, params.Spec.ID, string(params.Spec.Kind))
		applyCanonicalOperationalEvidence(alert, params.Spec, params.Evidence, time.Now())
		m.preserveAlertState(storageKey, alert)
		m.setActiveAlertNoLock(storageKey, alert)
		if params.AddToRecent {
			m.recentAlerts[trackingKey] = alert
		}

		if existing == nil {
			reactivatedStart, reactivatedAt, reactivated, hadResolvedOccurrence := m.consumeRecentlyResolvedForRefireWithPrimaryLock(storageKey, time.Now())
			if reactivated {
				if !reactivatedStart.IsZero() {
					alert.StartTime = reactivatedStart
				}
				if params.AddToHistory {
					m.historyManager.UpdateAlertLastSeenForAlert(alert, alert.LastSeen)
				}
				log.Debug().
					Str("alertID", storageKey).
					Time("resolvedAt", reactivatedAt).
					Msg("Stateful alert re-fired within cooldown, reactivated without new history entry")
				if params.RateLimit && !m.checkRateLimit(trackingKey) {
					return result, true
				}
				m.dispatchAlert(alert, params.DispatchAsync)
				return result, true
			}

			if params.AddToHistory {
				if hadResolvedOccurrence {
					m.historyManager.AddAlertTransition(*alert)
				} else {
					m.historyManager.AddAlert(*alert)
				}
			}
			if params.RateLimit && !m.checkRateLimit(trackingKey) {
				log.Debug().
					Str("alertID", storageKey).
					Str("trackingKey", trackingKey).
					Int("maxPerHour", m.config.Schedule.MaxAlertsHour).
					Msg("Stateful alert notification suppressed due to rate limit")
				return result, true
			}
			m.dispatchAlert(alert, params.DispatchAsync)
			return result, true
		}

		if result.Transition != nil && result.Transition.Kind == alertspecs.EvaluationTransitionSeverityChanged && params.NotifyOnSeverityChange {
			if params.AddToHistoryOnSeverityChange {
				m.historyManager.AddAlertTransition(*alert)
			}
			if params.RateLimit && !m.checkRateLimit(trackingKey) {
				log.Debug().
					Str("alertID", storageKey).
					Str("trackingKey", trackingKey).
					Int("maxPerHour", m.config.Schedule.MaxAlertsHour).
					Msg("Stateful escalation notification suppressed due to rate limit")
				return result, true
			}
			m.dispatchAlert(alert, params.DispatchAsync)
		}
		m.setActiveAlertNoLock(storageKey, alert)
		return result, true
	default:
		if existing == nil {
			return result, true
		}

		m.removeActiveAlertNoLock(storageKey)
		recoveryEvidence, hasRecoveryEvidence := canonicalAlertEvidenceEnvelope(
			params.Spec,
			params.Evidence,
			existing.Instance,
			time.Now(),
		)
		var recoveryEvidenceRef *operationaltrust.EvidenceEnvelope
		if hasRecoveryEvidence {
			recoveryEvidenceRef = &recoveryEvidence
		}
		resolvedAlert := m.newResolvedAlert(
			existing,
			params.Evidence.ObservedAt,
			recoveryEvidenceRef,
		)
		m.addRecentlyResolvedWithPrimaryLock(resolvedAlert)
		m.safeCallResolvedAlertCallback(existing, storageKey, true)
		return result, true
	}
}
