package alerts

import (
	"time"

	alertspecs "github.com/rcourtman/pulse-go-rewrite/internal/alerts/specs"
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

func (m *Manager) evaluateCanonicalLifecycleAlert(params canonicalLifecycleAlertParams) {
	if params.Evidence.ObservedAt.IsZero() {
		params.Evidence.ObservedAt = time.Now()
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	var existing *Alert
	if current, ok := m.activeAlerts[params.AlertID]; ok {
		existing = current
	}

	confirmations := 0
	if params.Tracking != nil {
		confirmations = params.Tracking[params.TrackingKey]
	}

	result, err := alertspecs.Evaluate(params.Spec, lifecyclePreviousState(params.Spec, existing, confirmations, params.Evidence.ObservedAt), params.Evidence)
	if err != nil {
		log.Warn().
			Err(err).
			Str("alertID", params.AlertID).
			Str("resourceID", params.ResourceID).
			Str("specID", params.Spec.ID).
			Msg("Skipping invalid canonical lifecycle evaluation")
		return
	}

	if params.Tracking != nil {
		if result.State.ConsecutiveMatches > 0 {
			params.Tracking[params.TrackingKey] = result.State.ConsecutiveMatches
		} else {
			delete(params.Tracking, params.TrackingKey)
		}
	}

	switch result.State.State {
	case alertspecs.AlertStatePending:
		return
	case alertspecs.AlertStateFiring:
		level, ok := alertLevelFromCanonicalSeverity(result.State.Severity)
		if !ok {
			level = AlertLevelWarning
		}
		alert := &Alert{
			ID:           params.AlertID,
			Type:         params.AlertType,
			Level:        level,
			ResourceID:   params.ResourceID,
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
		alert.Metadata["canonicalSpecID"] = params.Spec.ID
		alert.Metadata["canonicalAlertKind"] = string(params.Spec.Kind)

		m.preserveAlertState(params.AlertID, alert)
		m.activeAlerts[params.AlertID] = alert
		if params.AddToRecent {
			m.recentAlerts[params.AlertID] = alert
		}

		if existing != nil {
			return
		}

		if params.AddToHistory {
			m.historyManager.AddAlert(*alert)
		}

		if params.RateLimit && !m.checkRateLimit(params.AlertID) {
			log.Debug().
				Str("alertID", params.AlertID).
				Int("maxPerHour", m.config.Schedule.MaxAlertsHour).
				Msg("Lifecycle alert notification suppressed due to rate limit")
			return
		}

		m.dispatchAlert(alert, params.DispatchAsync)
		return
	default:
		if existing == nil {
			return
		}

		m.removeActiveAlertNoLock(params.AlertID)
		resolvedAlert := &ResolvedAlert{
			Alert:        existing,
			ResolvedTime: params.Evidence.ObservedAt,
		}
		m.addRecentlyResolvedWithPrimaryLock(params.AlertID, resolvedAlert)
		m.safeCallResolvedCallback(params.AlertID, true)
	}
}
