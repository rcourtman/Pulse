package alerts

import (
	"fmt"
	"time"

	alertspecs "github.com/rcourtman/pulse-go-rewrite/internal/alerts/specs"
	"github.com/rs/zerolog/log"
)

func metricPreviousState(spec alertspecs.ResourceAlertSpec, existing *Alert) alertspecs.EvaluatorState {
	if existing == nil {
		return alertspecs.EvaluatorState{
			SpecID: spec.ID,
			State:  alertspecs.AlertStateClear,
		}
	}

	severity := alertspecs.AlertSeverityWarning
	if existing.Level == AlertLevelCritical {
		severity = alertspecs.AlertSeverityCritical
	}

	return alertspecs.EvaluatorState{
		SpecID:         spec.ID,
		State:          alertspecs.AlertStateFiring,
		Severity:       severity,
		ActiveSince:    existing.StartTime,
		FirstMatchedAt: existing.StartTime,
		LastObservedAt: existing.LastSeen,
	}
}

func metricClearThreshold(spec *alertspecs.MetricThresholdSpec, threshold *HysteresisThreshold) float64 {
	if threshold != nil && threshold.Clear > 0 {
		return threshold.Clear
	}
	if spec != nil && spec.Recovery != nil {
		return *spec.Recovery
	}
	if spec != nil {
		return spec.Trigger
	}
	return 0
}

func metricAlertMessage(resourceType, metricType string, value float64, opts *metricOptions) (string, string) {
	if opts != nil && opts.Message != "" {
		return opts.Message, ""
	}

	switch metricType {
	case "usage":
		return resourceType + " at " + formatMetricValue(value, "%"), "%"
	case "diskRead", "diskWrite", "networkIn", "networkOut":
		return resourceType + " " + metricType + " at " + formatMetricValue(value, " MB/s"), "MB/s"
	case "temperature", "disk_temperature", "diskTemperature":
		return resourceType + " " + metricType + " at " + formatMetricValue(value, "°C"), "°C"
	default:
		return resourceType + " " + metricType + " at " + formatMetricValue(value, "%"), ""
	}
}

func formatMetricValue(value float64, suffix string) string {
	return fmt.Sprintf("%.1f%s", value, suffix)
}

func (m *Manager) evaluateCanonicalMetricAlert(spec alertspecs.ResourceAlertSpec, resourceName, node, instance, resourceType string, value float64, threshold *HysteresisThreshold, opts *metricOptions) {
	if spec.MetricThreshold == nil {
		return
	}

	alertID := spec.ID
	metricType := spec.MetricThreshold.Metric
	if spec.Disabled || spec.MetricThreshold.Trigger <= 0 {
		m.mu.Lock()
		delete(m.pendingAlerts, alertID)
		m.mu.Unlock()
		m.clearAlert(alertID)
		return
	}

	observedAt := time.Now()

	m.mu.Lock()
	defer m.mu.Unlock()

	existingAlert, exists := m.activeAlerts[alertID]
	monitorOnly := opts != nil && opts.MonitorOnly

	if suppressUntil, suppressed := m.suppressedUntil[alertID]; suppressed && time.Now().Before(suppressUntil) {
		log.Debug().
			Str("alertID", alertID).
			Time("suppressedUntil", suppressUntil).
			Msg("Canonical metric alert suppressed")
		return
	}

	triggered := alertspecsMetricTriggered(spec.MetricThreshold, value)
	alertStartTime := observedAt
	if !exists && triggered {
		timeThreshold := m.getTimeThreshold(spec.ResourceID, resourceType, metricType)
		if timeThreshold > 0 {
			if pendingTime, isPending := m.pendingAlerts[alertID]; isPending {
				if time.Since(pendingTime) >= time.Duration(timeThreshold)*time.Second {
					delete(m.pendingAlerts, alertID)
					if !pendingTime.IsZero() {
						alertStartTime = pendingTime
					}
				} else {
					return
				}
			} else {
				m.pendingAlerts[alertID] = alertStartTime
				return
			}
		}

		if recent, hasRecent := m.recentAlerts[alertID]; hasRecent &&
			m.config.MinimumDelta > 0 &&
			time.Since(recent.StartTime) < time.Duration(m.config.SuppressionWindow)*time.Minute &&
			abs(recent.Value-value) < m.config.MinimumDelta {
			m.suppressedUntil[alertID] = time.Now().Add(time.Duration(m.config.SuppressionWindow) * time.Minute)
			return
		}
	}

	if !triggered {
		delete(m.pendingAlerts, alertID)
	}

	result, err := alertspecs.Evaluate(spec, metricPreviousState(spec, existingAlert), alertspecs.AlertEvidence{
		ObservedAt: observedAt,
		MetricThreshold: &alertspecs.MetricThresholdEvidence{
			Metric:    metricType,
			Direction: spec.MetricThreshold.Direction,
			Observed:  value,
			Trigger:   spec.MetricThreshold.Trigger,
			Recovery:  spec.MetricThreshold.Recovery,
			Critical:  spec.MetricThreshold.Critical,
		},
	})
	if err != nil {
		log.Warn().
			Err(err).
			Str("alertID", alertID).
			Str("resourceID", spec.ResourceID).
			Str("metricType", metricType).
			Msg("Skipping invalid canonical metric evaluation")
		return
	}

	switch result.State.State {
	case alertspecs.AlertStateFiring:
		level, ok := alertLevelFromCanonicalSeverity(result.State.Severity)
		if !ok {
			level = AlertLevelWarning
		}

		message, unit := metricAlertMessage(resourceType, metricType, value, opts)
		alertMetadata := map[string]interface{}{
			"resourceType":   resourceType,
			"clearThreshold": metricClearThreshold(spec.MetricThreshold, threshold),
			"monitorOnly":    monitorOnly,
		}
		if unit != "" {
			alertMetadata["unit"] = unit
		}
		if opts != nil && opts.Metadata != nil {
			for k, v := range opts.Metadata {
				alertMetadata[k] = v
			}
		}

		if !exists {
			alert := &Alert{
				ID:              alertID,
				Type:            metricType,
				Level:           level,
				ResourceID:      spec.ResourceID,
				ResourceName:    resourceName,
				Node:            node,
				NodeDisplayName: m.resolveNodeDisplayName(node),
				Instance:        instance,
				Message:         message,
				Value:           value,
				Threshold:       spec.MetricThreshold.Trigger,
				StartTime:       alertStartTime,
				LastSeen:        observedAt,
				Metadata:        alertMetadata,
			}

			applyCanonicalIdentity(alert, spec.ID, string(spec.Kind))
			m.preserveAlertState(alertID, alert)
			m.activeAlerts[alertID] = alert
			m.recentAlerts[alertID] = alert
			m.historyManager.AddAlert(*alert)

			asyncSaveActiveAlerts("canonical metric create", m.SaveActiveAlerts)

			if alertForAICallback := m.getAlertForAICallback(); alertForAICallback != nil {
				alertCopy := alert.Clone()
				go func(a *Alert) {
					defer func() {
						if r := recover(); r != nil {
							log.Error().Interface("panic", r).Str("alertID", a.ID).Msg("panic in AI alert callback")
						}
					}()
					alertForAICallback(a)
				}(alertCopy)
			}

			if !m.checkRateLimit(alertID) {
				return
			}

			if m.getAlertCallback() != nil {
				now := time.Now()
				alert.LastNotified = &now
				if !m.dispatchAlert(alert, true) {
					alert.LastNotified = nil
				}
			}
			return
		}

		if !triggered && result.Transition == nil {
			return
		}

		oldLevel := existingAlert.Level
		existingAlert.LastSeen = observedAt
		existingAlert.Value = value
		existingAlert.Threshold = spec.MetricThreshold.Trigger
		existingAlert.Level = level
		if dn := m.resolveNodeDisplayName(existingAlert.Node); dn != "" {
			existingAlert.NodeDisplayName = dn
		}
		if opts != nil && opts.Message != "" {
			existingAlert.Message = opts.Message
		}
		if existingAlert.Metadata == nil {
			existingAlert.Metadata = map[string]interface{}{}
		}
		for k, v := range alertMetadata {
			existingAlert.Metadata[k] = v
		}
		applyCanonicalIdentity(existingAlert, spec.ID, string(spec.Kind))

		shouldRenotify := false
		if existingAlert.Acknowledged {
		} else if m.shouldNotifyAfterCooldown(existingAlert) {
			shouldRenotify = true
		} else if oldLevel != existingAlert.Level && existingAlert.Level == AlertLevelCritical {
			shouldRenotify = true
		}

		if shouldRenotify && m.getAlertCallback() != nil {
			now := time.Now()
			existingAlert.LastNotified = &now
			if !m.dispatchAlert(existingAlert, true) {
				existingAlert.LastNotified = nil
			}
		}
	default:
		if !exists {
			return
		}

		resolvedAlert := &ResolvedAlert{
			Alert:        existingAlert,
			ResolvedTime: observedAt,
		}
		m.removeActiveAlertNoLock(alertID)
		asyncSaveActiveAlerts("canonical metric resolution", m.SaveActiveAlerts)
		m.addRecentlyResolvedWithPrimaryLock(alertID, resolvedAlert)
		m.safeCallResolvedCallback(alertID, true)
	}
}

func alertspecsMetricTriggered(spec *alertspecs.MetricThresholdSpec, observed float64) bool {
	if spec == nil {
		return false
	}
	switch spec.Direction {
	case alertspecs.ThresholdDirectionAbove:
		return observed >= spec.Trigger
	case alertspecs.ThresholdDirectionBelow:
		return observed <= spec.Trigger
	default:
		return false
	}
}
