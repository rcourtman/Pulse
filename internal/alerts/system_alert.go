package alerts

import (
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/eventlog"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// System-scoped alerts report on Pulse itself rather than on a monitored
// resource. They exist because some failures cannot be observed from the
// outside by the person running Pulse: the clearest case is notification
// delivery, where the channel that would have carried the warning is the thing
// that broke. Routing these through the ordinary alert pipeline puts them in
// the alert list and the navigation badge, which is the only escalation path
// that does not depend on delivery working.
const (
	// SystemAlertResourceName is what the alert list shows in place of a
	// monitored resource name.
	SystemAlertResourceName = "Pulse"

	// SystemAlertIDPrefix namespaces system alert IDs so they cannot collide
	// with a resource-derived alert ID.
	SystemAlertIDPrefix = "pulse-system-"

	// NotificationDeliveryAlertType is the first system alert: configured
	// notification destinations are not delivering.
	NotificationDeliveryAlertType = "notification-delivery"

	// DeadManDeliveryAlertType reports that Pulse is healthy but cannot reach
	// the configured external watchdog.
	DeadManDeliveryAlertType = "deadman-delivery"
	// DeadManMonitoringStalledAlertType reports that the watchdog worker is
	// alive but the canonical monitoring loop has stopped making progress.
	DeadManMonitoringStalledAlertType = "deadman-monitoring-stalled"
	// DeadManInterruptionAlertType records a monitoring availability gap found
	// when Pulse restarts.
	DeadManInterruptionAlertType = "deadman-interruption"
	// DeadManStateAlertType reports loss of the durable restart-gap record.
	DeadManStateAlertType = "deadman-state"
)

// SystemAlertInput describes a system-scoped condition. Type is required and
// identifies the condition; the same Type always maps to the same alert, so
// repeated raises update one alert rather than accumulating.
type SystemAlertInput struct {
	Type    string
	Level   AlertLevel
	Message string
	// Fingerprint, when set, identifies the notify-worthy state of the
	// condition. A re-raise whose level and fingerprint both match the
	// standing alert refreshes the message and metadata silently, so a
	// message carrying a moving counter does not notify on every tick.
	// Callers that leave it empty keep the message itself as the change
	// signal.
	Fingerprint string
	Metadata    map[string]interface{}
}

// systemAlertFingerprintKey stores the raise fingerprint on the alert metadata
// so the next raise can compare against it.
const systemAlertFingerprintKey = "systemAlertFingerprint"

// SystemAlertID returns the stable alert ID for a system alert type.
func SystemAlertID(alertType string) string {
	alertType = strings.TrimSpace(alertType)
	if alertType == "" {
		return ""
	}
	return SystemAlertIDPrefix + alertType
}

// IsSystemAlert reports whether an alert is system-scoped.
func IsSystemAlert(alert *Alert) bool {
	if alert == nil {
		return false
	}
	return strings.HasPrefix(alert.ID, SystemAlertIDPrefix)
}

// RaiseSystemAlert raises a system-scoped alert, or refreshes one that is
// already standing. It is deliberately idempotent: re-raising an unchanged
// condition only advances LastSeen and does not notify again, so a condition
// that is re-evaluated on a timer cannot turn into a notification storm. A
// change of level or message is treated as new information and does notify.
//
// It reports whether this call raised the alert or changed it.
func (m *Manager) RaiseSystemAlert(input SystemAlertInput) bool {
	alertType := strings.TrimSpace(input.Type)
	if alertType == "" {
		return false
	}
	alertID := SystemAlertID(alertType)
	level := input.Level
	if level == "" {
		level = AlertLevelWarning
	}

	m.mu.Lock()

	now := time.Now()
	existing, exists := m.getActiveAlertNoLock(alertID)
	if exists && existing != nil {
		unchanged := existing.Level == level
		if unchanged {
			existingFingerprint, _ := existing.Metadata[systemAlertFingerprintKey].(string)
			if input.Fingerprint != "" || existingFingerprint != "" {
				unchanged = existingFingerprint == input.Fingerprint
			} else {
				unchanged = existing.Message == input.Message
			}
		}
		existing.LastSeen = now
		if unchanged {
			// Same condition state: keep the presentation current (a message
			// may carry counters) without treating it as new information.
			existing.Message = input.Message
			existing.Metadata = systemAlertMetadata(alertType, input.Fingerprint, input.Metadata)
			m.setActiveAlertNoLock(alertID, existing)
			m.mu.Unlock()
			m.saveActiveAlertsAsync("system-alert-refresh")
			return false
		}

		existing.Level = level
		existing.Message = input.Message
		existing.Metadata = systemAlertMetadata(alertType, input.Fingerprint, input.Metadata)
		m.setActiveAlertNoLock(alertID, existing)
		// dispatchAlert reads flapping and schedule state through the primary
		// lock, so it must run before the unlock rather than after.
		m.dispatchAlert(existing, true)
		m.mu.Unlock()

		m.saveActiveAlertsAsync("system-alert-update")
		log.Info().
			Str("alertID", alertID).
			Str("level", string(level)).
			Msg("System alert updated")
		return true
	}

	alert := &Alert{
		ID:   alertID,
		Type: alertType,
		// A system alert has no monitored resource. ResourceID stays empty so
		// surfaces that link an alert back to a resource skip that affordance
		// rather than offering a link to nothing.
		ResourceID:   "",
		ResourceName: SystemAlertResourceName,
		Level:        level,
		Message:      input.Message,
		StartTime:    now,
		LastSeen:     now,
		Metadata:     systemAlertMetadata(alertType, input.Fingerprint, input.Metadata),
	}

	m.setActiveAlertNoLock(alertID, alert)
	if m.historyManager != nil {
		m.historyManager.AddAlert(*alert)
	}
	m.recordAlertEvent(eventlog.TypeFired, alert, alertID, "system-alert", input.Message, nil)
	m.dispatchAlert(alert, true)
	m.mu.Unlock()

	m.saveActiveAlertsAsync("system-alert-raise")
	log.Warn().
		Str("alertID", alertID).
		Str("level", string(level)).
		Str("message", input.Message).
		Msg("System alert raised")
	return true
}

// ClearSystemAlert clears a standing system alert and reports whether one was
// cleared. Clearing an alert that is not standing is a no-op.
func (m *Manager) ClearSystemAlert(alertType string) bool {
	alertID := SystemAlertID(alertType)
	if alertID == "" {
		return false
	}
	if !m.ClearAlert(alertID) {
		return false
	}
	log.Info().
		Str("alertID", alertID).
		Msg("System alert cleared")
	return true
}

// systemAlertMetadata stamps the marker that surfaces use to tell a
// system-scoped alert apart from a resource alert, without letting a caller
// overwrite it.
func systemAlertMetadata(alertType, fingerprint string, extra map[string]interface{}) map[string]interface{} {
	metadata := make(map[string]interface{}, len(extra)+3)
	for key, value := range extra {
		metadata[key] = value
	}
	metadata["systemAlert"] = true
	metadata["systemAlertType"] = alertType
	if fingerprint != "" {
		metadata[systemAlertFingerprintKey] = fingerprint
	}
	return metadata
}
