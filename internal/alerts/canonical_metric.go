package alerts

import (
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/eventlog"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/reducer"
	alertspecs "github.com/rcourtman/pulse-go-rewrite/internal/alerts/specs"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
	"github.com/rs/zerolog/log"
)

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
	case "vm":
		return "VM"
	case "system-container":
		return "System container"
	case "app-container", "oci-container":
		return "Application container"
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
		m.core.ApplyMetric(reducer.MetricSignal{
			ResourceID: spec.ResourceID,
			Key:        spec.ID,
			Metric:     metricType,
			Value:      value,
			ObservedAt: time.Now(),
		}, reducer.MetricRule{})
		m.mu.Unlock()
		// A guest that moved nodes may hold this alert under its old node-scoped
		// identity; re-home it first so the clear below can resolve it.
		m.rehomeStrandedGuestAlert(storageKey, spec.ID, string(spec.Kind), spec.ResourceID, resourceName, node, instance, resourceType)
		m.clearAlert(storageKey)
		return
	}

	observedAt := m.policyNow()
	windowed := m.evaluateMetricWindow(spec.ResourceID, resourceType, metricType, value, observedAt)
	if !windowed.Ready {
		return
	}
	value = windowed.Value
	opts = metricWindowOptions(opts, metricType, resourceType, windowed)

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

	// An existing alert the core does not know about (guest migration,
	// direct injection) means it was firing: adopt it.
	if exists && existingAlert != nil {
		if _, known := m.core.Incident(spec.ResourceID, spec.ID); !known {
			ackAt := time.Time{}
			if existingAlert.AckTime != nil {
				ackAt = *existingAlert.AckTime
			}
			m.core.SeedFiringIncident(spec.ResourceID, spec.ID, shadowSeverityForLevel(existingAlert.Level), existingAlert.StartTime, existingAlert.Acknowledged, existingAlert.AckUser, ackAt)
		}
	}

	// Suppress a would-be new occurrence within the minimum delta of a
	// recent one before it reaches the core.
	coreIncidentBefore, hadCoreIncident := m.core.Incident(spec.ResourceID, spec.ID)
	wouldBeNew := !hadCoreIncident || coreIncidentBefore.State != reducer.StateFiring
	if wouldBeNew && triggered {
		if recent, hasRecent := m.recentAlerts[trackingKey]; hasRecent &&
			m.config.MinimumDelta > 0 &&
			time.Since(recent.StartTime) < time.Duration(m.config.SuppressionWindow)*time.Minute &&
			abs(recent.Value-value) < m.config.MinimumDelta {
			m.suppressedUntil[trackingKey] = time.Now().Add(time.Duration(m.config.SuppressionWindow) * time.Minute)
			return
		}
	}

	// Intent context: explicit metric policies replace the legacy delay.
	var intent *reducer.DiscreteIntent
	delaySeconds := 0
	effectiveIntent := m.resolveEffectiveIntentPolicyNoLock(spec.ResourceID, resourceType, MetricAlertIntentSignal(metricType))
	stability := metricStabilityPolicy{}
	if effectiveIntent.Explicit {
		decision := m.evaluateIntentNoLock(spec.ResourceID, resourceType, MetricAlertIntentSignal(metricType), trackingKey, observedAt, triggered, BackupIntentContext{})
		if decision.StateChanged {
			m.saveActiveAlertsAsync("canonical metric intent pending state")
		}
		intent = &reducer.DiscreteIntent{
			Explicit:           decision.Effective.Explicit,
			GraceSeconds:       decision.Effective.GraceSeconds,
			OperatorSuppressed: decision.Suppressed && strings.HasPrefix(decision.Reason, "operator_"),
			OperatorReason:     decision.Reason,
		}
	} else if triggered {
		delaySeconds = m.getTimeThreshold(spec.ResourceID, resourceType, metricType)
	}
	if !effectiveIntent.Explicit {
		stability = metricStabilityFor(metricType, effectiveIntent.GraceSeconds)
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

	clearThreshold := 0.0
	if spec.MetricThreshold.Recovery != nil {
		clearThreshold = *spec.MetricThreshold.Recovery
	}
	events := m.core.ApplyMetric(reducer.MetricSignal{
		ResourceID:       spec.ResourceID,
		Key:              spec.ID,
		Metric:           metricType,
		Value:            value,
		RuntimeTick:      m.intentTickNoLock(),
		RuntimeTickValid: true,
		ObservedAt:       observedAt,
	}, reducer.MetricRule{
		Trigger:               spec.MetricThreshold.Trigger,
		Clear:                 clearThreshold,
		Critical:              spec.MetricThreshold.Critical,
		CriticalDisabled:      spec.MetricThreshold.Critical == nil,
		DelaySeconds:          delaySeconds,
		CriticalBypassesDelay: stability.CriticalBypassesDelay,
		RecoveryDelaySeconds:  stability.RecoveryDelaySeconds,
		Intent:                intent,
	})
	primary := reducer.EventType("")
	if len(events) > 0 {
		primary = events[0].Type
	}
	incident, hasIncident := m.core.Incident(spec.ResourceID, spec.ID)

	switch {
	case hasIncident && incident.State == reducer.StatePending:
		return
	case hasIncident && incident.State == reducer.StateFiring:
		if intent != nil && (primary == reducer.EventFired || primary == reducer.EventRefired) {
			m.clearIntentPendingNoLock(trackingKey)
			m.saveActiveAlertsAsync("canonical metric intent activated")
		}
		level := AlertLevelWarning
		if incident.Severity == reducer.SeverityCritical {
			level = AlertLevelCritical
		}
		alertStartTime := incident.StartedAt

		message, unit := metricAlertMessage(resourceType, metricType, value, opts)
		alertMetadata := map[string]interface{}{
			"resourceType":   resourceType,
			"clearThreshold": metricClearThreshold(spec.MetricThreshold, threshold),
			"monitorOnly":    monitorOnly,
		}
		if stability.RecoveryDelaySeconds > 0 {
			alertMetadata["stabilityWindowSeconds"] = stability.RecoveryDelaySeconds
			alertMetadata["criticalBypassesStability"] = stability.CriticalBypassesDelay
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
			m.recentAlerts[trackingKey] = alert
			m.historyManager.AddAlert(*alert)
			m.recordAlertEvent(eventlog.TypeFired, alert, storageKey, "metric-threshold", message, nil)

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

		if !triggered && primary == "" {
			// Hysteresis latch: below trigger but above recovery — hold
			// without refreshing, as the pre-cutover engine did.
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
		if opts != nil {
			for _, key := range opts.RemoveMetadata {
				delete(existingAlert.Metadata, key)
			}
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
		if !exists || existingAlert == nil {
			return
		}

		// Publish the observation that cleared the incident, not the last
		// firing sample. Both history and resolved callbacks consume this snapshot.
		existingAlert = existingAlert.Clone()
		existingAlert.Value = value
		existingAlert.LastSeen = observedAt
		message, _ := metricAlertMessage(resourceType, metricType, value, opts)
		existingAlert.Message = "Resolved: " + message

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
