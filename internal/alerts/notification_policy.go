package alerts

import (
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rs/zerolog/log"
)

const (
	// MetadataQuietHoursSuppressed marks notifications that were held by alert
	// quiet-hours policy and must be replayed by the notification queue.
	MetadataQuietHoursSuppressed = "quietHoursSuppressed"
	// MetadataQuietHoursSuppressionReason records the quiet-hours rule that
	// deferred notification delivery.
	MetadataQuietHoursSuppressionReason = "quietHoursSuppressionReason"
	// MetadataQuietHoursReplayAt stores the earliest RFC3339 UTC time that the
	// notification queue may retry delivery.
	MetadataQuietHoursReplayAt = "quietHoursReplayAt"
)

// checkFlappingLocked detects alert flapping and returns whether the alert
// should be suppressed and whether this call is the first transition into
// the flapping state for the current cooldown window. justTransitioned is
// the signal callers use to fire a one-shot postmortem hook on first
// detection -- it is true only on the call that flips
// flappingActive[trackingKey] from false to true.
//
// It modifies flappingHistory, flappingActive, and suppressedUntil maps.
// IMPORTANT: Caller MUST hold m.mu before calling this function. The
// transition signal is returned so the caller can dispatch the
// flapping-detected callback OUTSIDE the lock (typically via a goroutine).
func (m *Manager) checkFlappingLocked(trackingKey string) (suppress bool, justTransitioned bool) {
	if !m.config.FlappingEnabled {
		return false, false
	}

	now := time.Now()
	windowDuration := time.Duration(m.config.FlappingWindowSeconds) * time.Second

	// Record this state change
	m.flappingHistory[trackingKey] = append(m.flappingHistory[trackingKey], now)

	// Remove state changes outside the window
	history := m.flappingHistory[trackingKey]
	validHistory := []time.Time{}
	for _, t := range history {
		if now.Sub(t) <= windowDuration {
			validHistory = append(validHistory, t)
		}
	}
	// Limit to max 10 entries to prevent unbounded growth
	const maxFlappingHistory = 10
	if len(validHistory) > maxFlappingHistory {
		validHistory = validHistory[len(validHistory)-maxFlappingHistory:]
	}
	m.flappingHistory[trackingKey] = validHistory

	// Check if we've exceeded the threshold
	if len(validHistory) >= m.config.FlappingThreshold {
		// Mark as flapping
		if !m.flappingActive[trackingKey] {
			log.Warn().
				Str("trackingKey", trackingKey).
				Int("stateChanges", len(validHistory)).
				Int("threshold", m.config.FlappingThreshold).
				Int("windowSeconds", m.config.FlappingWindowSeconds).
				Msg("Flapping detected - suppressing alert")

			m.flappingActive[trackingKey] = true

			// Set cooldown period
			cooldownDuration := time.Duration(m.config.FlappingCooldownMinutes) * time.Minute
			m.suppressedUntil[trackingKey] = now.Add(cooldownDuration)

			// Record suppression metric
			if recordAlertSuppressed != nil {
				recordAlertSuppressed("flapping")
			}
			return true, true
		}
		return true, false
	}

	return false, false
}

func (m *Manager) dispatchAlert(alert *Alert, async bool) bool {
	callbacks := m.getAlertCallbacks()
	if len(callbacks) == 0 || alert == nil {
		return false
	}

	// Don't dispatch notifications for acknowledged alerts
	if alert.Acknowledged {
		log.Debug().
			Str("alertID", alert.ID).
			Str("ackUser", alert.AckUser).
			Msg("Alert notification suppressed - already acknowledged")
		return false
	}

	trackingKey := canonicalTrackingKeyForAlert(alert)

	// Check for flapping (caller must hold m.mu). When this call is the
	// first transition into the flapping state for the current cooldown
	// window, fire the flapping-detected callback so a postmortem patrol
	// can be triggered. The callback runs in its own goroutine because
	// the caller of dispatchAlert holds m.mu and the callback is allowed
	// to take its own locks or re-enter the alerts package.
	suppress, justTransitioned := m.checkFlappingLocked(trackingKey)
	if justTransitioned {
		cb := m.callbacks.flappingDetectedCallback()
		if cb != nil {
			alertCopy := cloneAlertForOutput(alert)
			go func(a *Alert, key string, fn func(*Alert, string)) {
				defer func() {
					if r := recover(); r != nil {
						log.Error().
							Interface("panic", r).
							Str("alertID", a.ID).
							Str("trackingKey", key).
							Msg("Panic in onFlappingDetected callback")
					}
				}()
				fn(a, key)
			}(alertCopy, trackingKey, cb)
		}
	}
	if suppress {
		log.Debug().
			Str("alertID", alert.ID).
			Str("trackingKey", trackingKey).
			Msg("Alert suppressed due to flapping")
		return false
	}

	// Check activation state - only dispatch notifications if active
	if m.config.ActivationState != ActivationActive {
		log.Debug().
			Str("alertID", alert.ID).
			Str("activationState", string(m.config.ActivationState)).
			Msg("Alert notification suppressed - not activated")
		return false
	}

	if suppressed, reason := m.shouldSuppressNotification(alert); suppressed {
		replayAt := m.quietHoursReplayAt()
		markQuietHoursNotificationReplay(alert, reason, replayAt)
		log.Debug().
			Str("alertID", alert.ID).
			Str("type", alert.Type).
			Str("level", string(alert.Level)).
			Str("quietHoursRule", reason).
			Time("replayAt", replayAt).
			Msg("Alert notification deferred during quiet hours")
	} else {
		clearQuietHoursNotificationReplay(alert)
	}

	if isMonitorOnlyAlert(alert) {
		log.Info().
			Str("alertID", alert.ID).
			Str("resource", alert.ResourceName).
			Bool("monitorOnly", true).
			Msg("Monitor-only alert detected, skipping alert dispatch")
		return false
	}

	// Record metric for fired alert
	if recordAlertFired != nil {
		recordAlertFired(alert)
	}

	notifiedAt := time.Now()
	m.recordAlertNotifiedNoLock(alert, notifiedAt)
	m.saveActiveAlertsAsync("alert-dispatch")

	alertCopy := cloneAlertForOutput(alert)
	if async {
		go func(a *Alert, fns []func(*Alert)) {
			defer func() {
				if r := recover(); r != nil {
					log.Error().
						Interface("panic", r).
						Str("alertID", a.ID).
						Str("type", a.Type).
						Msg("Panic in onAlert callback")
				}
			}()
			for _, callback := range fns {
				callback(a)
			}
		}(alertCopy, callbacks)
	} else {
		// Synchronous calls also need panic recovery to prevent service crash
		func(fns []func(*Alert)) {
			defer func() {
				if r := recover(); r != nil {
					log.Error().
						Interface("panic", r).
						Str("alertID", alertCopy.ID).
						Str("type", alertCopy.Type).
						Msg("Panic in onAlert callback (synchronous)")
				}
			}()
			for _, callback := range fns {
				callback(alertCopy)
			}
		}(callbacks)
	}
	return true
}

func (m *Manager) recordAlertNotifiedNoLock(alert *Alert, notifiedAt time.Time) {
	if alert == nil {
		return
	}

	setAlertLastNotified(alert, notifiedAt)

	if trackingKey := canonicalTrackingKeyForAlert(alert); trackingKey != "" {
		if active, exists := m.getActiveAlertNoLock(trackingKey); exists && active != nil {
			setAlertLastNotified(active, notifiedAt)
			return
		}
	}
	if alert.ID != "" {
		if active, exists := m.getActiveAlertNoLock(alert.ID); exists && active != nil {
			setAlertLastNotified(active, notifiedAt)
		}
	}
}

func setAlertLastNotified(alert *Alert, notifiedAt time.Time) {
	if alert == nil {
		return
	}
	t := notifiedAt
	alert.LastNotified = &t
}

func isMonitorOnlyAlert(alert *Alert) bool {
	if alert == nil || alert.Metadata == nil {
		return false
	}

	if value, ok := alert.Metadata["monitorOnly"]; ok {
		switch v := value.(type) {
		case bool:
			return v
		case string:
			return strings.EqualFold(v, "true")
		}
	}
	return false
}

// isInQuietHours checks if the current time is within quiet hours
func (m *Manager) isInQuietHours() bool {
	if !m.config.Schedule.QuietHours.Enabled {
		return false
	}

	// Use cached location if available
	loc := m.quietHoursLoc
	if loc == nil {
		// Fallback to loading if not cached yet (shouldn't happen with UpdateConfig)
		var err error
		loc, err = time.LoadLocation(m.config.Schedule.QuietHours.Timezone)
		if err != nil {
			log.Warn().Err(err).Str("timezone", m.config.Schedule.QuietHours.Timezone).Msg("failed to load timezone, using local time")
			loc = time.Local
		}
		m.quietHoursLoc = loc
	}

	nowFn := m.now
	if nowFn == nil {
		nowFn = time.Now
	}
	now := nowFn().In(loc).Truncate(time.Minute)
	dayName := strings.ToLower(now.Format("Monday"))

	// Check if today is enabled for quiet hours
	if enabled, ok := m.config.Schedule.QuietHours.Days[dayName]; !ok || !enabled {
		return false
	}

	// Parse start and end times
	startTime, err := time.ParseInLocation("15:04", m.config.Schedule.QuietHours.Start, loc)
	if err != nil {
		log.Warn().Err(err).Str("start", m.config.Schedule.QuietHours.Start).Msg("failed to parse quiet hours start time")
		return false
	}

	endTime, err := time.ParseInLocation("15:04", m.config.Schedule.QuietHours.End, loc)
	if err != nil {
		log.Warn().Err(err).Str("end", m.config.Schedule.QuietHours.End).Msg("failed to parse quiet hours end time")
		return false
	}

	// Set to today's date
	startTime = time.Date(now.Year(), now.Month(), now.Day(), startTime.Hour(), startTime.Minute(), 0, 0, loc)
	endTime = time.Date(now.Year(), now.Month(), now.Day(), endTime.Hour(), endTime.Minute(), 0, 0, loc)

	// Quiet hours are configured with minute precision, so treat the start and
	// end minute as inclusive for user-facing schedules such as 00:00-23:59.
	endExclusive := endTime.Add(time.Minute)

	// Handle overnight quiet hours (e.g., 22:00 to 08:00)
	if endTime.Before(startTime) {
		if !now.Before(startTime) || now.Before(endExclusive) {
			return true
		}
	} else {
		if !now.Before(startTime) && now.Before(endExclusive) {
			return true
		}
	}

	return false
}

func quietHoursCategoryForAlert(alert *Alert) string {
	if alert == nil {
		return ""
	}

	switch alert.Type {
	case "cpu", "memory", "disk", "diskRead", "diskWrite", "networkIn", "networkOut", "temperature":
		return "performance"
	case "queue-depth", "queue-deferred", "queue-hold", "message-age",
		"docker-container-health", "docker-container-restart-loop",
		"docker-container-oom-kill", "docker-container-memory-limit":
		return "performance"
	case "usage", "disk-health", "disk-wearout", "zfs-pool-state", "zfs-pool-errors", "zfs-device", "storage-incident", "backup-storage-incident", "backup-posture-incident":
		return "storage"
	case "resource-incident":
		if metadataStringValue(alert.Metadata, "incidentCategory") == unifiedresources.IncidentCategoryAvailability {
			return "offline"
		}
		return "performance"
	case "connectivity", "offline", "powered-off", "docker-host-offline":
		return "offline"
	}

	if strings.HasPrefix(alert.Type, "docker-container-") {
		if alert.Type == "docker-container-state" {
			return "offline"
		}
		return "performance"
	}

	return ""
}

func (m *Manager) shouldSuppressNotification(alert *Alert) (bool, string) {
	if alert == nil {
		return false, ""
	}

	if !m.isInQuietHours() {
		return false, ""
	}

	if alert.Level != AlertLevelCritical {
		return true, "non-critical"
	}

	category := quietHoursCategoryForAlert(alert)
	switch category {
	case "performance":
		if m.config.Schedule.QuietHours.Suppress.Performance {
			return true, category
		}
	case "storage":
		if m.config.Schedule.QuietHours.Suppress.Storage {
			return true, category
		}
	case "offline":
		if m.config.Schedule.QuietHours.Suppress.Offline {
			return true, category
		}
	}

	return false, ""
}

func (m *Manager) quietHoursReplayAt() time.Time {
	nowFn := m.now
	if nowFn == nil {
		nowFn = time.Now
	}
	now := nowFn()

	loc := m.quietHoursLoc
	if loc == nil {
		var err error
		loc, err = time.LoadLocation(m.config.Schedule.QuietHours.Timezone)
		if err != nil {
			log.Warn().Err(err).Str("timezone", m.config.Schedule.QuietHours.Timezone).Msg("failed to load timezone for quiet-hours replay, using local time")
			loc = time.Local
		}
		m.quietHoursLoc = loc
	}

	localNow := now.In(loc).Truncate(time.Minute)
	startTime, startErr := time.ParseInLocation("15:04", m.config.Schedule.QuietHours.Start, loc)
	endTime, endErr := time.ParseInLocation("15:04", m.config.Schedule.QuietHours.End, loc)
	if startErr != nil || endErr != nil {
		return now.Add(time.Minute).UTC()
	}

	startTime = time.Date(localNow.Year(), localNow.Month(), localNow.Day(), startTime.Hour(), startTime.Minute(), 0, 0, loc)
	endTime = time.Date(localNow.Year(), localNow.Month(), localNow.Day(), endTime.Hour(), endTime.Minute(), 0, 0, loc)
	endExclusive := endTime.Add(time.Minute)
	if endTime.Before(startTime) && !localNow.Before(startTime) {
		endExclusive = endExclusive.AddDate(0, 0, 1)
	}

	if !endExclusive.After(localNow) {
		return now.Add(time.Minute).UTC()
	}
	return endExclusive.UTC()
}

func markQuietHoursNotificationReplay(alert *Alert, reason string, replayAt time.Time) {
	if alert == nil {
		return
	}
	if alert.Metadata == nil {
		alert.Metadata = make(map[string]interface{}, 3)
	}
	alert.Metadata[MetadataQuietHoursSuppressed] = true
	alert.Metadata[MetadataQuietHoursSuppressionReason] = reason
	alert.Metadata[MetadataQuietHoursReplayAt] = replayAt.UTC().Format(time.RFC3339)
}

func clearQuietHoursNotificationReplay(alert *Alert) {
	if alert == nil || alert.Metadata == nil {
		return
	}
	delete(alert.Metadata, MetadataQuietHoursSuppressed)
	delete(alert.Metadata, MetadataQuietHoursSuppressionReason)
	delete(alert.Metadata, MetadataQuietHoursReplayAt)
}

func hasQuietHoursNotificationReplay(alert *Alert) bool {
	if alert == nil || alert.Metadata == nil {
		return false
	}
	if replayAt, ok := alert.Metadata[MetadataQuietHoursReplayAt].(string); ok && strings.TrimSpace(replayAt) != "" {
		return true
	}
	if suppressed, ok := alert.Metadata[MetadataQuietHoursSuppressed].(bool); ok {
		return suppressed
	}
	return false
}

// ShouldSuppressNotification checks whether a notification must be dropped at
// the alert-policy layer. Quiet-hours matches are replayable, so this helper
// annotates the alert for notification-queue retry and returns false to let
// bypass callers hand the alert to the delivery owner.
func (m *Manager) ShouldSuppressNotification(alert *Alert) bool {
	if alert == nil {
		return false
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	suppressed, reason := m.shouldSuppressNotification(alert)
	if suppressed {
		replayAt := m.quietHoursReplayAt()
		markQuietHoursNotificationReplay(alert, reason, replayAt)
		log.Debug().
			Str("alertID", alert.ID).
			Str("type", alert.Type).
			Str("level", string(alert.Level)).
			Str("quietHoursRule", reason).
			Time("replayAt", replayAt).
			Msg("Notification deferred during quiet hours")
		return false
	}
	clearQuietHoursNotificationReplay(alert)
	return false
}

// ShouldSuppressResolvedNotification checks if a recovery notification should be suppressed.
// Recovery notifications keep the existing quiet-hours rule unless the firing
// notification was already deferred into the owned queue replay path; in that
// case the resolved notification is also allowed to enter the queue so the
// close-the-loop delivery is not lost.
func (m *Manager) ShouldSuppressResolvedNotification(alert *Alert) bool {
	if alert == nil {
		return false
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	if alert.Acknowledged {
		log.Debug().
			Str("alertID", alert.ID).
			Str("type", alert.Type).
			Msg("Recovery notification suppressed for acknowledged alert")
		return true
	}

	quietHoursReplay := hasQuietHoursNotificationReplay(alert)
	if alert.LastNotified == nil && !quietHoursReplay {
		log.Debug().
			Str("alertID", alert.ID).
			Str("type", alert.Type).
			Msg("Recovery notification suppressed because firing notification was never sent")
		return true
	}

	suppressed, reason := m.shouldSuppressNotification(alert)
	if suppressed {
		if quietHoursReplay {
			log.Debug().
				Str("alertID", alert.ID).
				Str("type", alert.Type).
				Str("level", string(alert.Level)).
				Str("quietHoursRule", reason).
				Msg("Recovery notification allowed into quiet-hours replay queue")
			return false
		}
		log.Debug().
			Str("alertID", alert.ID).
			Str("type", alert.Type).
			Str("level", string(alert.Level)).
			Str("quietHoursRule", reason).
			Msg("Recovery notification suppressed during quiet hours")
	}
	return suppressed
}

// shouldNotifyAfterCooldown decides whether a re-notification can be sent for
// an existing alert based on the configured cooldown.
//
// Cooldown semantics:
//   - cooldown > 0: re-notify after that many minutes have passed since the
//     last notification.
//   - cooldown <= 0: cooldown is disabled, so only the first notification for
//     an alert occurrence is allowed. The alert evaluation loop runs every
//     metric tick, so treating disabled cooldown as "always allow" causes
//     repeated notifications for the same active alert.
//
// Level-escalation re-notifications are handled separately at the call site.
// Returns true if a notification should be sent, false otherwise.
func (m *Manager) shouldNotifyAfterCooldown(alert *Alert) bool {
	if alert.LastNotified == nil {
		return true
	}

	if m.config.Schedule.Cooldown <= 0 {
		return false
	}

	cooldownDuration := time.Duration(m.config.Schedule.Cooldown) * time.Minute
	timeSinceLastNotification := time.Since(*alert.LastNotified)

	return timeSinceLastNotification >= cooldownDuration
}

func (m *Manager) allowNotificationByRateLimit(trackingKey string, alert *Alert, reason string) bool {
	if trackingKey == "" && alert != nil {
		trackingKey = canonicalTrackingKeyForAlert(alert)
	}
	if trackingKey == "" && alert != nil {
		trackingKey = alert.ID
	}
	if m.checkRateLimit(trackingKey) {
		return true
	}

	log.Debug().
		Str("alertID", alertIDForLog(alert)).
		Str("trackingKey", trackingKey).
		Str("reason", reason).
		Int("maxPerHour", m.config.Schedule.MaxAlertsHour).
		Msg("Alert notification suppressed due to rate limit")
	return false
}

func alertIDForLog(alert *Alert) string {
	if alert == nil {
		return ""
	}
	return alert.ID
}

// checkRateLimit checks if an alert has exceeded rate limit
func (m *Manager) checkRateLimit(alertID string) bool {
	if m.config.Schedule.MaxAlertsHour <= 0 {
		return true // No rate limit
	}

	now := time.Now()
	cutoff := now.Add(-1 * time.Hour)

	// Clean old entries and count recent alerts
	var recentAlerts []time.Time
	if times, exists := m.alertRateLimit[alertID]; exists {
		for _, t := range times {
			if t.After(cutoff) {
				recentAlerts = append(recentAlerts, t)
			}
		}
	}

	// Check if we've hit the limit
	if len(recentAlerts) >= m.config.Schedule.MaxAlertsHour {
		return false
	}

	// Add current time
	recentAlerts = append(recentAlerts, now)
	m.alertRateLimit[alertID] = recentAlerts

	return true
}
