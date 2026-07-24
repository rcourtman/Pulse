package alerts

import (
	"fmt"
	"strings"
	"time"

	alertspecs "github.com/rcourtman/pulse-go-rewrite/internal/alerts/specs"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
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

func resourceTypeLabel(resourceType string) string {
	switch strings.TrimSpace(resourceType) {
	case "agent-disk":
		return "Disk"
	case "agent":
		return "Agent"
	case "node":
		return "Node"
	case "guest":
		return "Guest"
	case "storage":
		return "Storage"
	case "pbs":
		return "PBS"
	case "pmg":
		return "PMG"
	case "":
		return "Resource"
	default:
		return resourceType
	}
}

func metricAlertMessage(resourceType, metricType string, value float64, opts *metricOptions) (string, string) {
	if opts != nil && opts.Message != "" {
		return opts.Message, ""
	}

	label := resourceTypeLabel(resourceType)
	switch metricType {
	case "usage", "disk":
		return label + " at " + formatMetricValue(value, "%"), "%"
	case "diskRead", "diskWrite", "networkIn", "networkOut":
		return label + " " + metricType + " at " + formatMetricValue(value, " MB/s"), "MB/s"
	case "temperature", "disk_temperature", "diskTemperature":
		return label + " " + metricType + " at " + formatMetricValue(value, "°C"), "°C"
	default:
		return label + " " + metricType + " at " + formatMetricValue(value, "%"), ""
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
	storageKey := canonicalTrackingKeyForSpec(spec, alertID)
	trackingKey := storageKey
	metricType := spec.MetricThreshold.Metric
	if spec.Disabled || spec.MetricThreshold.Trigger <= 0 {
		m.mu.Lock()
		delete(m.pendingAlerts, trackingKey)
		m.mu.Unlock()
		// A guest that moved nodes may hold this alert under its old node-scoped
		// identity; re-home it first so the clear below can resolve it.
		m.rehomeStrandedGuestAlert(storageKey, spec.ID, string(spec.Kind), spec.ResourceID, resourceName, node, instance, resourceType)
		m.clearAlert(storageKey)
		return
	}

	observedAt := time.Now()

	m.mu.Lock()
	migratedAlertIdentity := false
	defer func() {
		if migratedAlertIdentity {
			m.saveActiveAlertsAsync("canonical guest metric node move")
		}
	}()
	defer m.mu.Unlock()

	existingAlert, exists := m.getActiveAlertNoLock(storageKey)
	if !exists {
		if migrated := m.migrateGuestAlertNoLock(storageKey, spec.ID, string(spec.Kind), spec.ResourceID, resourceName, node, instance, resourceType); migrated != nil {
			existingAlert = migrated
			exists = true
			migratedAlertIdentity = true
		}
	}
	monitorOnly := opts != nil && opts.MonitorOnly

	if suppressUntil, suppressed := m.suppressedUntil[trackingKey]; suppressed && time.Now().Before(suppressUntil) {
		log.Debug().
			Str("alertID", storageKey).
			Str("trackingKey", trackingKey).
			Time("suppressedUntil", suppressUntil).
			Msg("Canonical metric alert suppressed")
		return
	}

	triggered := alertspecsMetricTriggered(spec.MetricThreshold, value)
	alertStartTime := observedAt
	if !exists && triggered {
		effectiveIntent := m.resolveEffectiveIntentPolicyNoLock(spec.ResourceID, resourceType, MetricAlertIntentSignal(metricType))
		if effectiveIntent.Explicit {
			decision := m.evaluateIntentNoLock(spec.ResourceID, resourceType, MetricAlertIntentSignal(metricType), trackingKey, observedAt, true, BackupIntentContext{})
			if pending, ok := m.intentPending[trackingKey]; ok && !pending.FirstMatchedAt.IsZero() {
				alertStartTime = pending.FirstMatchedAt
			}
			if decision.StateChanged {
				m.saveActiveAlertsAsync("canonical metric intent pending state")
			}
			if !decision.ShouldActivate {
				return
			}
			m.clearIntentPendingNoLock(trackingKey)
			m.saveActiveAlertsAsync("canonical metric intent activated")
		} else if timeThreshold := m.getTimeThreshold(spec.ResourceID, resourceType, metricType); timeThreshold > 0 {
			if pendingTime, isPending := m.pendingAlerts[trackingKey]; isPending {
				if time.Since(pendingTime) >= time.Duration(timeThreshold)*time.Second {
					delete(m.pendingAlerts, trackingKey)
					if !pendingTime.IsZero() {
						alertStartTime = pendingTime
					}
				} else {
					return
				}
			} else {
				m.pendingAlerts[trackingKey] = alertStartTime
				return
			}
		}

		if recent, hasRecent := m.recentAlerts[trackingKey]; hasRecent &&
			m.config.MinimumDelta > 0 &&
			time.Since(recent.StartTime) < time.Duration(m.config.SuppressionWindow)*time.Minute &&
			abs(recent.Value-value) < m.config.MinimumDelta {
			m.suppressedUntil[trackingKey] = time.Now().Add(time.Duration(m.config.SuppressionWindow) * time.Minute)
			return
		}
	}

	if !triggered {
		decision := m.evaluateIntentNoLock(spec.ResourceID, resourceType, MetricAlertIntentSignal(metricType), trackingKey, observedAt, false, BackupIntentContext{})
		if decision.StateChanged {
			m.saveActiveAlertsAsync("canonical metric intent cleared")
		}
		delete(m.pendingAlerts, trackingKey)
	}

	evidence := alertspecs.AlertEvidence{
		ObservedAt: observedAt,
		MetricThreshold: &alertspecs.MetricThresholdEvidence{
			Metric:    metricType,
			Direction: spec.MetricThreshold.Direction,
			Observed:  value,
			Trigger:   spec.MetricThreshold.Trigger,
			Recovery:  spec.MetricThreshold.Recovery,
			Critical:  spec.MetricThreshold.Critical,
		},
	}
	result, err := alertspecs.Evaluate(spec, metricPreviousState(spec, existingAlert), evidence)
	if err != nil {
		log.Warn().
			Err(err).
			Str("alertID", storageKey).
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
				ID:              storageKey,
				Type:            metricType,
				Level:           level,
				ResourceID:      spec.ResourceID,
				ResourceName:    resourceName,
				Node:            node,
				NodeDisplayName: m.resolveNodeDisplayName(instance, node),
				Instance:        instance,
				Message:         message,
				Value:           value,
				Threshold:       spec.MetricThreshold.Trigger,
				StartTime:       alertStartTime,
				LastSeen:        observedAt,
				Metadata:        alertMetadata,
			}

			applyCanonicalIdentity(alert, spec.ID, string(spec.Kind))
			applyCanonicalOperationalEvidence(alert, spec, evidence, time.Now())
			m.preserveAlertState(storageKey, alert)
			m.setActiveAlertNoLock(storageKey, alert)
			m.recentAlerts[trackingKey] = alert
			m.historyManager.AddAlert(*alert)

			m.saveActiveAlertsAsync("canonical metric create")

			if alertForAICallback := m.getAlertForAICallback(); alertForAICallback != nil {
				alertCopy := cloneAlertForOutput(alert)
				go func(a *Alert) {
					defer func() {
						if r := recover(); r != nil {
							log.Error().Interface("panic", r).Str("alertID", a.ID).Msg("panic in AI alert callback")
						}
					}()
					alertForAICallback(a)
				}(alertCopy)
			}

			if !m.checkRateLimit(trackingKey) {
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
		if dn := m.resolveNodeDisplayName(existingAlert.Instance, existingAlert.Node); dn != "" {
			existingAlert.NodeDisplayName = dn
		}
		existingAlert.Message = message
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
		applyCanonicalOperationalEvidence(existingAlert, spec, evidence, time.Now())

		shouldRenotify := false
		if existingAlert.Acknowledged {
		} else if m.shouldNotifyAfterCooldown(existingAlert) {
			shouldRenotify = m.allowNotificationByRateLimit(trackingKey, existingAlert, "cooldown")
		} else if oldLevel != existingAlert.Level && existingAlert.Level == AlertLevelCritical {
			shouldRenotify = m.allowNotificationByRateLimit(trackingKey, existingAlert, "critical-escalation")
		}

		if shouldRenotify && m.getAlertCallback() != nil {
			now := time.Now()
			existingAlert.LastNotified = &now
			if !m.dispatchAlert(existingAlert, true) {
				existingAlert.LastNotified = nil
			}
		}
		m.setActiveAlertNoLock(storageKey, existingAlert)
	default:
		if !exists {
			return
		}

		recoveryEvidence, hasRecoveryEvidence := canonicalAlertEvidenceEnvelope(
			spec,
			evidence,
			existingAlert.Instance,
			time.Now(),
		)
		var recoveryEvidenceRef *operationaltrust.EvidenceEnvelope
		if hasRecoveryEvidence {
			recoveryEvidenceRef = &recoveryEvidence
		}
		resolvedAlert := m.newResolvedAlert(existingAlert, observedAt, recoveryEvidenceRef)
		m.removeActiveAlertNoLock(storageKey)
		m.saveActiveAlertsAsync("canonical metric resolution")
		m.addRecentlyResolvedWithPrimaryLock(resolvedAlert)
		m.safeCallResolvedAlertCallback(existingAlert, storageKey, true)
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
