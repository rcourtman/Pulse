package alerts

import (
	"fmt"
	"strings"
	"time"

	alertspecs "github.com/rcourtman/pulse-go-rewrite/internal/alerts/specs"
	"github.com/rs/zerolog/log"
)

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
func (m *Manager) getTimeThreshold(_ string, resourceType, metricType string) int {
	if delay, ok := m.getMetricTimeThreshold(resourceType, metricType); ok {
		return delay
	}

	base, hasTypeSpecific := m.getBaseTimeThreshold(resourceType)

	if !hasTypeSpecific {
		if delay, ok := m.getGlobalMetricTimeThreshold(metricType); ok {
			return delay
		}
	}

	return base
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
	Metadata map[string]interface{}
	Message  string
	// MonitorOnly suppresses external notifications while still tracking the alert.
	MonitorOnly bool
}

func (m *Manager) checkMetric(resourceID, resourceName, node, instance, resourceType, metricType string, value float64, threshold *HysteresisThreshold, opts *metricOptions) {
	alertID := fmt.Sprintf("%s-%s", resourceID, metricType)
	canonicalSpecID := "metric-threshold:" + metricType
	canonicalStateID := buildCanonicalStateID(resourceID, canonicalSpecID)

	if threshold == nil || threshold.Trigger <= 0 {
		m.clearAlert(canonicalStateID)
		m.clearAlert(alertID)
		return
	}

	log.Debug().
		Str("resource", resourceName).
		Str("metric", metricType).
		Float64("value", value).
		Float64("trigger", threshold.Trigger).
		Float64("clear", threshold.Clear).
		Bool("exceeds", value >= threshold.Trigger).
		Msg("Checking metric threshold")

	m.mu.Lock()
	migratedAlertIdentity := false
	defer func() {
		if migratedAlertIdentity {
			asyncSaveActiveAlerts("guest metric node move", m.SaveActiveAlerts)
		}
	}()
	defer m.mu.Unlock()

	existingAlert, exists := m.getActiveAlertNoLock(alertID)
	if !exists && canonicalStateID != "" {
		existingAlert, exists = m.getActiveAlertNoLock(canonicalStateID)
	}
	if !exists && canonicalStateID != "" {
		if migrated := m.migrateGuestMetricAlertNoLock(canonicalStateID, canonicalSpecID, string(alertspecs.AlertSpecKindMetricThreshold), resourceID, resourceName, node, instance, resourceType); migrated != nil {
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

	if value >= threshold.Trigger {
		// Threshold exceeded
		if !exists {
			alertStartTime := time.Now()

			// Determine the appropriate time threshold based on resource/metric type
			timeThreshold := m.getTimeThreshold(resourceID, resourceType, metricType)

			// Check if we have a time threshold configured
			if timeThreshold > 0 {
				// Check if this threshold was already pending
				if pendingTime, isPending := m.pendingAlerts[trackingKey]; isPending {
					// Check if enough time has passed
					if time.Since(pendingTime) >= time.Duration(timeThreshold)*time.Second {
						// Time threshold met, proceed with alert
						delete(m.pendingAlerts, trackingKey)
						if !pendingTime.IsZero() {
							alertStartTime = pendingTime
						}
						log.Debug().
							Str("alertID", alertID).
							Int("timeThreshold", timeThreshold).
							Dur("elapsed", time.Since(pendingTime)).
							Msg("Time threshold met, triggering alert")
					} else {
						// Still waiting for time threshold
						log.Debug().
							Str("alertID", alertID).
							Int("timeThreshold", timeThreshold).
							Dur("elapsed", time.Since(pendingTime)).
							Msg("Threshold exceeded but waiting for time threshold")
						return
					}
				} else {
					// First time exceeding threshold, start tracking
					m.pendingAlerts[trackingKey] = alertStartTime
					log.Debug().
						Str("alertID", alertID).
						Str("trackingKey", trackingKey).
						Int("timeThreshold", timeThreshold).
						Msg("Threshold exceeded, starting time threshold tracking")
					return
				}
			}

			// Check for recent similar alert to prevent spam
			if recent, hasRecent := m.recentAlerts[trackingKey]; hasRecent {
				// Check minimum delta
				if m.config.MinimumDelta > 0 &&
					time.Since(recent.StartTime) < time.Duration(m.config.SuppressionWindow)*time.Minute &&
					abs(recent.Value-value) < m.config.MinimumDelta {
					log.Debug().
						Str("alertID", alertID).
						Float64("recentValue", recent.Value).
						Float64("currentValue", value).
						Float64("delta", abs(recent.Value-value)).
						Float64("minimumDelta", m.config.MinimumDelta).
						Msg("Alert suppressed due to minimum delta")

					// Set suppression window
					m.suppressedUntil[trackingKey] = time.Now().Add(time.Duration(m.config.SuppressionWindow) * time.Minute)
					return
				}
			}

			// New alert
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
				Level:           AlertLevelWarning,
				ResourceID:      resourceID,
				ResourceName:    resourceName,
				Node:            node,
				NodeDisplayName: m.resolveNodeDisplayName(instance, node),
				Instance:        instance,
				Message:         message,
				Value:           value,
				Threshold:       threshold.Trigger,
				StartTime:       alertStartTime,
				LastSeen:        time.Now(),
				Metadata:        alertMetadata,
			}
			applyCanonicalIdentity(alert, canonicalSpecID, string(alertspecs.AlertSpecKindMetricThreshold))

			// Set level based on how much over threshold
			if value >= threshold.Trigger+10 {
				alert.Level = AlertLevelCritical
			}

			log.Debug().
				Str("alertID", alertID).
				Time("alertStartTime", alertStartTime).
				Time("now", time.Now()).
				Dur("initialDuration", time.Since(alertStartTime)).
				Msg("Creating new alert with start time")

			m.preserveAlertState(canonicalStateID, alert)
			trackingKey = canonicalTrackingKeyOrFallback(alert, canonicalStateID)
			m.setActiveAlertNoLock(canonicalStateID, alert)
			m.recentAlerts[trackingKey] = alert
			m.historyManager.AddAlert(*alert)

			// Save active alerts after adding new one
			go func() {
				defer func() {
					if r := recover(); r != nil {
						log.Error().Interface("panic", r).Msg("panic in SaveActiveAlerts goroutine")
					}
				}()
				if err := m.SaveActiveAlerts(); err != nil {
					log.Error().Err(err).Msg("failed to save active alerts after creation")
				}
			}()

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
				// Don't delete the alert, just suppress notifications
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
		} else {
			// Update existing alert
			applyCanonicalIdentity(existingAlert, canonicalSpecID, string(alertspecs.AlertSpecKindMetricThreshold))
			m.setActiveAlertNoLock(canonicalStateID, existingAlert)
			existingAlert.LastSeen = time.Now()
			existingAlert.Value = value
			// Keep display name current (handles upgrades and renames).
			if dn := m.resolveNodeDisplayName(existingAlert.Instance, existingAlert.Node); dn != "" {
				existingAlert.NodeDisplayName = dn
			}
			if existingAlert.Metadata == nil {
				existingAlert.Metadata = map[string]interface{}{}
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

			// Update level if needed
			oldLevel := existingAlert.Level
			if value >= threshold.Trigger+10 {
				existingAlert.Level = AlertLevelCritical
			} else {
				existingAlert.Level = AlertLevelWarning
			}

			// Check if we should re-notify based on cooldown period
			// Never re-notify acknowledged alerts (user has already seen it)
			shouldRenotify := false
			if existingAlert.Acknowledged {
				log.Debug().
					Str("alertID", alertID).
					Msg("Alert is acknowledged, skipping re-notification")
			} else if m.shouldNotifyAfterCooldown(existingAlert) {
				shouldRenotify = m.allowNotificationByRateLimit(trackingKey, existingAlert, "cooldown")
				if shouldRenotify {
					log.Debug().
						Str("alertID", alertID).
						Dur("cooldown", time.Duration(m.config.Schedule.Cooldown)*time.Minute).
						Msg("Cooldown period has passed, will re-notify")
				}
			} else if oldLevel != existingAlert.Level && existingAlert.Level == AlertLevelCritical {
				// Always re-notify if alert escalated to critical
				shouldRenotify = m.allowNotificationByRateLimit(trackingKey, existingAlert, "critical-escalation")
				if shouldRenotify {
					log.Debug().
						Str("alertID", alertID).
						Msg("Alert escalated to critical, will re-notify despite cooldown")
				}
			}

			// Send re-notification if appropriate (may be suppressed by quiet hours)
			if shouldRenotify && len(m.getAlertCallbacks()) > 0 {
				now := time.Now()
				existingAlert.LastNotified = &now
				// Dispatch asynchronously so callback I/O cannot block alert evaluation.
				if m.dispatchAlert(existingAlert, true) {
					log.Info().
						Str("alertID", alertID).
						Str("level", string(existingAlert.Level)).
						Msg("Re-notifying for existing alert")
				} else {
					existingAlert.LastNotified = nil
				}
			}
		}
	} else {
		// Value is below trigger threshold
		// Clear any pending alert for this metric
		if _, isPending := m.pendingAlerts[trackingKey]; isPending {
			delete(m.pendingAlerts, trackingKey)
			log.Debug().
				Str("alertID", alertID).
				Str("trackingKey", trackingKey).
				Msg("Value dropped below threshold, clearing pending alert")
		}

		if exists {
			// Use hysteresis for resolution - only resolve if below clear threshold
			clearThreshold := threshold.Clear
			if clearThreshold <= 0 {
				clearThreshold = threshold.Trigger // Fallback to trigger if clear not set
			}

			if value <= clearThreshold {
				// Threshold cleared with hysteresis - auto resolve
				resolvedAlert := &ResolvedAlert{
					Alert:        existingAlert,
					ResolvedTime: time.Now(),
				}

				// Remove from active alerts
				m.removeActiveAlertNoLock(alertID)

				// Save active alerts after resolution
				go func() {
					defer func() {
						if r := recover(); r != nil {
							log.Error().Interface("panic", r).Msg("panic in SaveActiveAlerts goroutine (resolution)")
						}
					}()
					if err := m.SaveActiveAlerts(); err != nil {
						log.Error().Err(err).Msg("failed to save active alerts after resolution")
					}
				}()

				// Add to recently resolved while preventing lock-order inversions
				m.addRecentlyResolvedWithPrimaryLock(resolvedAlert)

				log.Info().
					Str("alertID", alertID).
					Msg("Added alert to recently resolved")

				log.Info().
					Str("resource", resourceName).
					Str("metric", metricType).
					Float64("value", value).
					Float64("clearThreshold", clearThreshold).
					Bool("wasAcknowledged", existingAlert.Acknowledged).
					Msg("Alert resolved with hysteresis")

				m.safeCallResolvedAlertCallback(existingAlert, alertID, true)
			}
		}
	}
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
