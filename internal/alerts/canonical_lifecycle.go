package alerts

import (
	"time"

	alertspecs "github.com/rcourtman/pulse-go-rewrite/internal/alerts/specs"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rs/zerolog/log"
)

type canonicalLifecycleAlertParams struct {
	Spec          alertspecs.ResourceAlertSpec
	Evidence      alertspecs.AlertEvidence
	Tracking      map[string]int
	TrackingKey   string
	AlertID       string
	AlertType     string
	ResourceID    string
	ResourceName  string
	Node          string
	Instance      string
	Message       string
	Metadata      map[string]interface{}
	AddToRecent   bool
	AddToHistory  bool
	RateLimit     bool
	DispatchAsync bool
	IntentBackup  BackupIntentContext
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

func lifecyclePreviousState(spec alertspecs.ResourceAlertSpec, existing *Alert, confirmations int, observedAt time.Time) alertspecs.EvaluatorState {
	if existing != nil {
		required := spec.ConfirmationsRequired
		if confirmations > required {
			required = confirmations
		}
		return alertspecs.EvaluatorState{
			SpecID:             spec.ID,
			State:              alertspecs.AlertStateFiring,
			Severity:           canonicalAlertSeverity(existing.Level),
			ConsecutiveMatches: required,
			FirstMatchedAt:     existing.StartTime,
			ActiveSince:        existing.StartTime,
			LastObservedAt:     existing.LastSeen,
		}
	}
	if confirmations > 0 {
		return alertspecs.EvaluatorState{
			SpecID:             spec.ID,
			State:              alertspecs.AlertStatePending,
			Severity:           spec.Severity,
			ConsecutiveMatches: confirmations,
			FirstMatchedAt:     observedAt,
		}
	}
	return alertspecs.EvaluatorState{
		SpecID: spec.ID,
		State:  alertspecs.AlertStateClear,
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

	storageKey := canonicalTrackingKeyForSpec(params.Spec, params.AlertID)
	trackingKey := storageKey

	var existing *Alert
	if current, ok := m.getActiveAlertNoLock(storageKey); ok {
		existing = current
	} else if migrated := m.migrateGuestAlertNoLock(storageKey, params.Spec.ID, string(params.Spec.Kind), params.Spec.ResourceID, params.ResourceName, params.Node, params.Instance, string(params.Spec.ResourceType)); migrated != nil {
		existing = migrated
		migratedAlertIdentity = true
	}

	confirmations := 0
	if params.Tracking != nil {
		confirmations = params.Tracking[params.TrackingKey]
	}

	result, err := alertspecs.Evaluate(params.Spec, lifecyclePreviousState(params.Spec, existing, confirmations, params.Evidence.ObservedAt), params.Evidence)
	if err != nil {
		log.Warn().
			Err(err).
			Str("alertID", storageKey).
			Str("resourceID", params.ResourceID).
			Str("specID", params.Spec.ID).
			Msg("Skipping invalid canonical lifecycle evaluation")
		return alertspecs.EvaluationResult{}, false
	}

	// Persist the canonical confirmation state before applying intent policy.
	// Grace and operator context are an additional gate; they must not prevent
	// the underlying evaluator from reaching its configured confirmation count.
	if params.Tracking != nil {
		if result.State.ConsecutiveMatches > 0 {
			params.Tracking[params.TrackingKey] = result.State.ConsecutiveMatches
		} else {
			delete(params.Tracking, params.TrackingKey)
		}
	}

	intentSignal := ""
	if params.Spec.Kind == alertspecs.AlertSpecKindConnectivity || params.Spec.Kind == alertspecs.AlertSpecKindPoweredState {
		intentSignal = string(AlertIntentSignalOffline)
	}
	if intentSignal != "" && existing == nil {
		conditionActive := result.State.State == alertspecs.AlertStatePending || result.State.State == alertspecs.AlertStateFiring
		decision := m.evaluateIntentNoLock(params.Spec.ResourceID, string(params.Spec.ResourceType), intentSignal, storageKey, params.Evidence.ObservedAt, conditionActive, params.IntentBackup)
		if decision.StateChanged {
			m.saveActiveAlertsAsync("lifecycle intent state")
		}
		if decision.Effective.Explicit && conditionActive && !decision.ShouldActivate {
			result.State.State = alertspecs.AlertStatePending
			result.State.Reason = decision.Reason
			if pending, ok := m.intentPending[storageKey]; ok && !pending.FirstMatchedAt.IsZero() {
				result.State.FirstMatchedAt = pending.FirstMatchedAt
			}
			return result, true
		}
		if decision.Effective.Explicit && result.State.State == alertspecs.AlertStateFiring {
			if pending, ok := m.intentPending[storageKey]; ok && !pending.FirstMatchedAt.IsZero() {
				result.State.FirstMatchedAt = pending.FirstMatchedAt
			}
			delete(m.intentPending, storageKey)
			m.saveActiveAlertsAsync("lifecycle intent activated")
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
			StartTime:    params.Evidence.ObservedAt,
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
		if !result.State.FirstMatchedAt.IsZero() {
			alert.StartTime = result.State.FirstMatchedAt
		}
		applyCanonicalOperationalEvidence(alert, params.Spec, params.Evidence, time.Now())
		m.preserveAlertState(storageKey, alert)
		m.setActiveAlertNoLock(storageKey, alert)
		if params.AddToRecent {
			m.recentAlerts[trackingKey] = alert
		}

		if existing != nil {
			return result, true
		}

		// The resolved maps are resolvedMutex-guarded; take it for the
		// lookup/removal only and keep history/dispatch outside the lock.
		var reactivatedAt time.Time
		reactivated := false
		m.resolvedMutex.Lock()
		if resolved, ok := m.getResolvedAlertNoLock(storageKey); ok && resolved != nil && resolved.Alert != nil {
			if resolved.ResolvedTime.After(time.Now().Add(-5 * time.Minute)) {
				if !resolved.Alert.StartTime.IsZero() {
					alert.StartTime = resolved.Alert.StartTime
				}
				m.removeResolvedAlertUnlocked(storageKey)
				reactivated = true
				reactivatedAt = resolved.ResolvedTime
			}
		}
		m.resolvedMutex.Unlock()
		if reactivated {
			if params.AddToHistory {
				m.historyManager.UpdateAlertLastSeenForAlert(alert, alert.LastSeen)
			}
			log.Debug().
				Str("alertID", storageKey).
				Time("resolvedAt", reactivatedAt).
				Msg("Alert re-fired within cooldown, reactivated without new history entry")
			if params.RateLimit && !m.checkRateLimit(trackingKey) {
				return result, true
			}
			m.dispatchAlert(alert, params.DispatchAsync)
			return result, true
		}

		if params.AddToHistory {
			m.historyManager.AddAlert(*alert)
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
			// The resolved maps are resolvedMutex-guarded; take it for the
			// lookup/removal only and keep history/dispatch outside the lock.
			var reactivatedAt time.Time
			reactivated := false
			m.resolvedMutex.Lock()
			if resolved, ok := m.getResolvedAlertNoLock(storageKey); ok && resolved != nil && resolved.Alert != nil {
				if resolved.ResolvedTime.After(time.Now().Add(-5 * time.Minute)) {
					if !resolved.Alert.StartTime.IsZero() {
						alert.StartTime = resolved.Alert.StartTime
					}
					m.removeResolvedAlertUnlocked(storageKey)
					reactivated = true
					reactivatedAt = resolved.ResolvedTime
				}
			}
			m.resolvedMutex.Unlock()
			if reactivated {
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
				m.historyManager.AddAlert(*alert)
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
				m.historyManager.AddAlert(*alert)
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
