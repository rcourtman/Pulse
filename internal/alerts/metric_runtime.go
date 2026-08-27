package alerts

import (
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/eventlog"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/reducer"
	alertspecs "github.com/rcourtman/pulse-go-rewrite/internal/alerts/specs"
	"github.com/rs/zerolog/log"
)

func isMetricThresholdAlertType(metricType string) bool {
	switch metricType {
	case "cpu", "memory", "disk", "diskRead", "diskWrite", "networkIn", "networkOut", "temperature", "usage":
		return true
	default:
		return false
	}
}

// getThresholdForMetric returns the threshold for a specific metric type from a ThresholdConfig.
func getThresholdForMetric(config ThresholdConfig, metricType string) *HysteresisThreshold {
	switch metricType {
	case "cpu":
		return config.CPU
	case "memory":
		return config.Memory
	case "disk":
		return config.Disk
	case "diskRead":
		return config.DiskRead
	case "diskWrite":
		return config.DiskWrite
	case "networkIn":
		return config.NetworkIn
	case "networkOut":
		return config.NetworkOut
	case "temperature":
		return config.Temperature
	case "usage":
		return config.Usage
	default:
		return nil
	}
}

// getThresholdForMetricFromConfig returns the threshold for a specific metric type from a ThresholdConfig
// ensuring hysteresis is properly set.
func getThresholdForMetricFromConfig(config ThresholdConfig, metricType string) *HysteresisThreshold {
	th := getThresholdForMetric(config, metricType)
	if th == nil {
		return nil
	}
	return ensureHysteresisThreshold(th)
}

// getTimeThreshold determines the delay to apply for a metric/resource combination.
func (m *Manager) getTimeThreshold(resourceID string, resourceType, metricType string) int {
	effective := m.resolveEffectiveIntentPolicyNoLock(resourceID, resourceType, MetricAlertIntentSignal(metricType))
	return effective.GraceSeconds
}

func (m *Manager) getLegacyTimeThresholdWithSource(resourceType, metricType string) (int, string, bool) {
	if delay, ok := m.getMetricTimeThreshold(resourceType, metricType); ok {
		return delay, "legacy.metricTimeThresholds." + strings.ToLower(strings.TrimSpace(resourceType)) + "." + strings.ToLower(strings.TrimSpace(metricType)), true
	}

	base, hasTypeSpecific := m.getBaseTimeThreshold(resourceType)

	if !hasTypeSpecific {
		if delay, ok := m.getGlobalMetricTimeThreshold(metricType); ok {
			return delay, "legacy.metricTimeThresholds.all." + strings.ToLower(strings.TrimSpace(metricType)), true
		}
	}
	if hasTypeSpecific {
		return base, "legacy.timeThresholds." + strings.ToLower(strings.TrimSpace(resourceType)), true
	}
	if base != 0 {
		return base, "legacy.timeThresholds.all", true
	}
	return 0, "", false
}

// getMetricTimeThreshold returns a metric-specific delay if configured at the resource-type level.
func (m *Manager) getMetricTimeThreshold(resourceType, metricType string) (int, bool) {
	if len(m.config.MetricTimeThresholds) == 0 {
		return 0, false
	}

	metricKey := strings.ToLower(strings.TrimSpace(metricType))
	if metricKey == "" {
		return 0, false
	}

	for _, typeKey := range CanonicalResourceTypeKeys(resourceType) {
		perType, ok := m.config.MetricTimeThresholds[typeKey]
		if !ok || len(perType) == 0 {
			continue
		}

		if delay, ok := perType[metricKey]; ok {
			return delay, true
		}
		if delay, ok := perType["default"]; ok {
			return delay, true
		}
		if delay, ok := perType["_default"]; ok {
			return delay, true
		}
		if delay, ok := perType["*"]; ok {
			return delay, true
		}
	}

	return 0, false
}

// getBaseTimeThreshold returns the resource-type level delay.
func (m *Manager) getBaseTimeThreshold(resourceType string) (int, bool) {
	if m.config.TimeThresholds != nil {
		for _, key := range CanonicalResourceTypeKeys(resourceType) {
			if delay, ok := m.config.TimeThresholds[key]; ok {
				return delay, true
			}
		}
		if delay, ok := m.config.TimeThresholds["all"]; ok {
			return delay, false
		}
	}

	return 0, false
}

func (m *Manager) getGlobalMetricTimeThreshold(metricType string) (int, bool) {
	if len(m.config.MetricTimeThresholds) == 0 {
		return 0, false
	}

	perType, ok := m.config.MetricTimeThresholds["all"]
	if !ok || len(perType) == 0 {
		return 0, false
	}

	metricKey := strings.ToLower(strings.TrimSpace(metricType))
	if metricKey == "" {
		return 0, false
	}

	if delay, ok := perType[metricKey]; ok {
		return delay, true
	}
	if delay, ok := perType["default"]; ok {
		return delay, true
	}
	if delay, ok := perType["_default"]; ok {
		return delay, true
	}
	if delay, ok := perType["*"]; ok {
		return delay, true
	}

	return 0, false
}

// checkMetric checks a single metric against its threshold with hysteresis.
type metricOptions struct {
	Metadata       map[string]interface{}
	RemoveMetadata []string
	Message        string
	// MonitorOnly suppresses external notifications while still tracking the alert.
	MonitorOnly bool
}

func (m *Manager) checkMetric(resourceID, resourceName, node, instance, resourceType, metricType string, value float64, threshold *HysteresisThreshold, opts *metricOptions) {
	alertID := fmt.Sprintf("%s-%s", resourceID, metricType)
	canonicalSpecID := "metric-threshold:" + metricType
	canonicalStateID := buildCanonicalStateID(resourceID, canonicalSpecID)

	if threshold == nil || threshold.Trigger <= 0 {
		// A guest that moved nodes may hold this alert under its old node-scoped
		// identity; re-home it first so the clear below can resolve it.
		m.rehomeStrandedGuestAlert(canonicalStateID, canonicalSpecID, string(alertspecs.AlertSpecKindMetricThreshold), resourceID, resourceName, node, instance, resourceType)
		m.mu.Lock()
		m.core.ApplyMetric(reducer.MetricSignal{
			ResourceID: resourceID,
			Key:        canonicalSpecID,
			Metric:     metricType,
			Value:      value,
			ObservedAt: m.policyNow(),
		}, reducer.MetricRule{})
		m.mu.Unlock()
		m.clearAlert(canonicalStateID)
		m.clearAlert(alertID)
		return
	}

	m.mu.Lock()
	migratedAlertIdentity := false
	defer func() {
		if migratedAlertIdentity {
			m.saveActiveAlertsAsync("guest metric node move")
		}
	}()
	defer m.mu.Unlock()

	existingAlert, exists := m.getActiveAlertNoLock(alertID)
	if !exists && canonicalStateID != "" {
		existingAlert, exists = m.getActiveAlertNoLock(canonicalStateID)
	}
	if !exists && canonicalStateID != "" {
		if migrated := m.migrateGuestAlertNoLock(canonicalStateID, canonicalSpecID, string(alertspecs.AlertSpecKindMetricThreshold), resourceID, resourceName, node, instance, resourceType); migrated != nil {
			existingAlert = migrated
			exists = true
			migratedAlertIdentity = true
		}
	}
	trackingKey := canonicalTrackingKeyOrFallback(existingAlert, canonicalStateID)
	if trackingKey == "" {
		trackingKey = canonicalStateID
	}
	monitorOnly := opts != nil && opts.MonitorOnly

	// Check for suppression
	if suppressUntil, suppressed := m.suppressedUntil[trackingKey]; suppressed && time.Now().Before(suppressUntil) {
		log.Debug().
			Str("alertID", alertID).
			Str("trackingKey", trackingKey).
			Time("suppressedUntil", suppressUntil).
			Msg("Alert suppressed")
		return
	}

	// An existing alert the core does not know about (guest migration,
	// direct injection) means it was firing: adopt it.
	if exists && existingAlert != nil {
		if _, known := m.core.Incident(resourceID, canonicalSpecID); !known {
			ackAt := time.Time{}
			if existingAlert.AckTime != nil {
				ackAt = *existingAlert.AckTime
			}
			m.core.SeedFiringIncident(resourceID, canonicalSpecID, shadowSeverityForLevel(existingAlert.Level), existingAlert.StartTime, existingAlert.Acknowledged, existingAlert.AckUser, ackAt)
		}
	}

	// Suppress a would-be new occurrence when it is within the minimum
	// delta of a recent one (spam guard), before it reaches the core.
	coreIncidentBefore, hadCoreIncident := m.core.Incident(resourceID, canonicalSpecID)
	wouldBeNew := !hadCoreIncident || coreIncidentBefore.State != reducer.StateFiring
	if wouldBeNew && value >= threshold.Trigger {
		if recent, hasRecent := m.recentAlerts[trackingKey]; hasRecent {
			if m.config.MinimumDelta > 0 &&
				time.Since(recent.StartTime) < time.Duration(m.config.SuppressionWindow)*time.Minute &&
				abs(recent.Value-value) < m.config.MinimumDelta {
				log.Debug().
					Str("alertID", alertID).
					Float64("recentValue", recent.Value).
					Float64("currentValue", value).
					Float64("minimumDelta", m.config.MinimumDelta).
					Msg("Alert suppressed due to minimum delta")
				m.suppressedUntil[trackingKey] = time.Now().Add(time.Duration(m.config.SuppressionWindow) * time.Minute)
				return
			}
		}
	}

	// Intent context: explicit metric policies replace the legacy
	// time-threshold delay; the gate itself lives in the reducer.
	conditionActive := value >= threshold.Trigger
	effectiveIntent := m.resolveEffectiveIntentPolicyNoLock(resourceID, resourceType, MetricAlertIntentSignal(metricType))
	var intent *reducer.DiscreteIntent
	delaySeconds := 0
	if effectiveIntent.Explicit {
		decision := m.evaluateIntentNoLock(resourceID, resourceType, MetricAlertIntentSignal(metricType), trackingKey, time.Now(), conditionActive, BackupIntentContext{})
		if decision.StateChanged {
			m.saveActiveAlertsAsync("metric intent pending state")
		}
		intent = &reducer.DiscreteIntent{
			Explicit:           decision.Effective.Explicit,
			GraceSeconds:       decision.Effective.GraceSeconds,
			OperatorSuppressed: decision.Suppressed && strings.HasPrefix(decision.Reason, "operator_"),
			OperatorReason:     decision.Reason,
		}
	} else if conditionActive {
		delaySeconds = m.getTimeThreshold(resourceID, resourceType, metricType)
	}

	events := m.core.ApplyMetric(reducer.MetricSignal{
		ResourceID:       resourceID,
		Key:              canonicalSpecID,
		Metric:           metricType,
		Value:            value,
		RuntimeTick:      m.intentTickNoLock(),
		RuntimeTickValid: true,
		ObservedAt:       m.policyNow(),
	}, reducer.MetricRule{
		Trigger:      threshold.Trigger,
		Clear:        threshold.Clear,
		DelaySeconds: delaySeconds,
		Intent:       intent,
	})
	primary := reducer.EventType("")
	if len(events) > 0 {
		primary = events[0].Type
	}

	incident, hasIncident := m.core.Incident(resourceID, canonicalSpecID)

	if hasIncident && incident.State == reducer.StatePending {
		return
	}

	if hasIncident && incident.State == reducer.StateFiring {
		if intent != nil && intent.Explicit && (primary == reducer.EventFired || primary == reducer.EventRefired) {
			m.clearIntentPendingNoLock(trackingKey)
			m.saveActiveAlertsAsync("metric intent activated")
		}

		level := AlertLevelWarning
		if incident.Severity == reducer.SeverityCritical {
			level = AlertLevelCritical
		}

		if exists && existingAlert != nil {
			// Update existing alert
			applyCanonicalIdentity(existingAlert, canonicalSpecID, string(alertspecs.AlertSpecKindMetricThreshold))
			m.setActiveAlertNoLock(canonicalStateID, existingAlert)
			existingAlert.LastSeen = time.Now()
			existingAlert.Value = value
			if dn := m.resolveNodeDisplayName(existingAlert.Instance, existingAlert.Node); dn != "" {
				existingAlert.NodeDisplayName = dn
			}
			if existingAlert.Metadata == nil {
				existingAlert.Metadata = map[string]interface{}{}
			}
			if opts != nil {
				for _, key := range opts.RemoveMetadata {
					delete(existingAlert.Metadata, key)
				}
			}
			existingAlert.Metadata["resourceType"] = resourceType
			existingAlert.Metadata["clearThreshold"] = threshold.Clear
			existingAlert.Metadata["monitorOnly"] = monitorOnly
			if opts != nil {
				if opts.Message != "" {
					existingAlert.Message = opts.Message
				}
				if opts.Metadata != nil {
					for k, v := range opts.Metadata {
						existingAlert.Metadata[k] = v
					}
				}
			}

			oldLevel := existingAlert.Level
			existingAlert.Level = level

			// Check if we should re-notify based on cooldown period
			// Never re-notify acknowledged alerts (user has already seen it)
			shouldRenotify := false
			if existingAlert.Acknowledged {
				log.Debug().
					Str("alertID", alertID).
					Msg("Alert is acknowledged, skipping re-notification")
			} else if m.shouldNotifyAfterCooldown(existingAlert) {
				shouldRenotify = m.allowNotificationByRateLimit(trackingKey, existingAlert, "cooldown")
			} else if oldLevel != existingAlert.Level && existingAlert.Level == AlertLevelCritical {
				// Always re-notify if alert escalated to critical
				shouldRenotify = m.allowNotificationByRateLimit(trackingKey, existingAlert, "critical-escalation")
			}

			if shouldRenotify && len(m.getAlertCallbacks()) > 0 {
				now := time.Now()
				existingAlert.LastNotified = &now
				if m.dispatchAlert(existingAlert, true) {
					log.Info().
						Str("alertID", alertID).
						Str("level", string(existingAlert.Level)).
						Msg("Re-notifying for existing alert")
				} else {
					existingAlert.LastNotified = nil
				}
			}
			return
		}

		// New occurrence.
		message := ""
		var unit string
		if opts != nil && opts.Message != "" {
			message = opts.Message
		} else {
			switch metricType {
			case "usage":
				message = fmt.Sprintf("%s at %.1f%%", resourceType, value)
			case "diskRead", "diskWrite", "networkIn", "networkOut":
				message = fmt.Sprintf("%s %s at %.1f MB/s", resourceType, metricType, value)
				unit = "MB/s"
			case "temperature", "disk_temperature", "diskTemperature":
				message = fmt.Sprintf("%s %s at %.1f°C", resourceType, metricType, value)
				unit = "°C"
			default:
				message = fmt.Sprintf("%s %s at %.1f%%", resourceType, metricType, value)
			}
		}

		alertMetadata := map[string]interface{}{
			"resourceType":   resourceType,
			"clearThreshold": threshold.Clear,
		}
		if unit != "" {
			alertMetadata["unit"] = unit
		}
		if opts != nil && opts.Metadata != nil {
			for k, v := range opts.Metadata {
				alertMetadata[k] = v
			}
		}
		alertMetadata["monitorOnly"] = monitorOnly

		alert := &Alert{
			ID:              alertID,
			Type:            metricType,
			Level:           level,
			ResourceID:      resourceID,
			ResourceName:    resourceName,
			Node:            node,
			NodeDisplayName: m.resolveNodeDisplayName(instance, node),
			Instance:        instance,
			Message:         message,
			Value:           value,
			Threshold:       threshold.Trigger,
			StartTime:       incident.StartedAt,
			LastSeen:        time.Now(),
			Metadata:        alertMetadata,
		}
		applyCanonicalIdentity(alert, canonicalSpecID, string(alertspecs.AlertSpecKindMetricThreshold))

		m.preserveAlertState(canonicalStateID, alert)
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
		trackingKey = canonicalTrackingKeyOrFallback(alert, canonicalStateID)
		m.setActiveAlertNoLock(canonicalStateID, alert)
		m.recentAlerts[trackingKey] = alert
		m.historyManager.AddAlert(*alert)
		m.recordAlertEvent(eventlog.TypeFired, alert, canonicalStateID, "metric-threshold", message, nil)

		m.saveActiveAlertsAsync("metric create")

		log.Warn().
			Str("alertID", alertID).
			Str("resource", resourceName).
			Str("metric", metricType).
			Float64("value", value).
			Float64("trigger", threshold.Trigger).
			Float64("clear", threshold.Clear).
			Int("activeAlerts", len(m.activeAlerts)).
			Msg("Alert triggered")

		// Trigger AI analysis callback unconditionally (bypasses notification suppression)
		if callbacks := m.getAlertForAICallbacks(); len(callbacks) > 0 {
			alertCopy := cloneAlertForOutput(alert)
			go func(a *Alert, fns []func(*Alert)) {
				defer func() {
					if r := recover(); r != nil {
						log.Error().Interface("panic", r).Str("alertID", a.ID).Msg("panic in AI alert callback")
					}
				}()
				for _, callback := range fns {
					callback(a)
				}
			}(alertCopy, callbacks)
		}

		// Check rate limit (but don't remove alert from tracking)
		if !m.checkRateLimit(trackingKey) {
			log.Debug().
				Str("alertID", alertID).
				Str("trackingKey", trackingKey).
				Int("maxPerHour", m.config.Schedule.MaxAlertsHour).
				Msg("Alert notification suppressed due to rate limit")
			return
		}

		// Notify callback (may be suppressed by quiet hours)
		if len(m.getAlertCallbacks()) > 0 {
			now := time.Now()
			alert.LastNotified = &now
			if m.dispatchAlert(alert, true) {
				log.Info().Str("alertID", alertID).Msg("calling onAlert callback")
			} else {
				alert.LastNotified = nil
			}
		} else {
			log.Warn().Msg("no onAlert callback set!")
		}
		return
	}

	// Clear: the core holds no incident.
	if !exists || existingAlert == nil {
		return
	}
	resolvedAlert := m.newResolvedAlert(existingAlert, time.Now(), nil)
	m.removeActiveAlertNoLock(activeAlertStorageKey(existingAlert, alertID))
	m.saveActiveAlertsAsync("metric resolution")
	m.addRecentlyResolvedWithPrimaryLock(resolvedAlert)
	log.Info().
		Str("resource", resourceName).
		Str("metric", metricType).
		Float64("value", value).
		Bool("wasAcknowledged", existingAlert.Acknowledged).
		Msg("Alert resolved with hysteresis")
	m.safeCallResolvedAlertCallback(existingAlert, alertID, true)
}

func sanitizeAlertKey(label string) string {
	trimmed := strings.TrimSpace(label)
	if trimmed == "" {
		return ""
	}

	if trimmed == "/" {
		return "root"
	}

	trimmed = strings.Trim(trimmed, "/\\ ")
	if trimmed == "" {
		trimmed = "root"
	}

	lower := strings.ToLower(trimmed)
	var builder strings.Builder
	builder.Grow(len(lower))
	prevDash := false
	for _, r := range lower {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
			prevDash = false
			continue
		}
		if r == '.' {
			builder.WriteRune(r)
			prevDash = false
			continue
		}
		if !prevDash {
			builder.WriteRune('-')
			prevDash = true
		}
	}

	sanitized := strings.Trim(builder.String(), "-.")
	if sanitized == "" {
		sanitized = "disk"
	}

	return sanitized
}

// abs returns the absolute value of a float64.
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
