package monitoring

import (
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
)

// notificationDeliveryCheckInterval throttles the delivery-health evaluation.
// The poll ticker runs on the configured polling cadence, which can be a few
// seconds, and reading queue health costs a SQLite query, so the check runs on
// its own slower schedule rather than on every poll.
const notificationDeliveryCheckInterval = 5 * time.Minute

// evaluateNotificationDelivery raises or clears the notification-delivery
// system alert. A destination that has stopped delivering cannot announce
// itself through a notification, so this is the path that puts it in the alert
// list and the navigation badge instead.
//
// RaiseSystemAlert is idempotent for an unchanged condition, so repeating this
// on a timer neither re-notifies nor accumulates alerts.
func (m *Monitor) evaluateNotificationDelivery(now time.Time) {
	if m == nil {
		return
	}
	if now.IsZero() {
		now = time.Now()
	}

	m.mu.Lock()
	if !m.lastDeliveryHealthCheck.IsZero() &&
		now.Sub(m.lastDeliveryHealthCheck) < notificationDeliveryCheckInterval {
		m.mu.Unlock()
		return
	}
	m.lastDeliveryHealthCheck = now
	notificationMgr := m.notificationMgr
	m.mu.Unlock()

	alertManager := m.GetAlertManager()
	if alertManager == nil || notificationMgr == nil {
		return
	}

	health := notificationMgr.DeliveryHealth()
	if health.Healthy {
		alertManager.ClearSystemAlert(alerts.NotificationDeliveryAlertType)
		return
	}

	alertManager.RaiseSystemAlert(alerts.SystemAlertInput{
		Type:    alerts.NotificationDeliveryAlertType,
		Level:   alerts.AlertLevelWarning,
		Message: notificationDeliveryAlertMessage(health),
		// The message embeds delivery counts, which move as retained failures
		// accumulate or expire. Fingerprinting on status and reason codes keeps
		// the standing alert's text current without re-notifying on every
		// count tick (#1721).
		Fingerprint: deliveryHealthFingerprint(health),
		Metadata: map[string]interface{}{
			"deliveryStatus":    string(health.Status),
			"failedDeliveries":  health.Failed,
			"deadLetterCount":   health.DeadLetter,
			"attentionRequired": health.AttentionRequired,
			"reasonCodes":       health.ReasonCodes,
		},
	})
}

// deliveryHealthFingerprint identifies the notify-worthy state of the delivery
// verdict: which coarse status holds and which failure classes are present.
// Counts deliberately stay out so their drift does not page.
func deliveryHealthFingerprint(health notifications.DeliveryHealth) string {
	return string(health.Status) + "|" + strings.Join(health.ReasonCodes, ",")
}

// notificationDeliveryAlertMessage says what failed and where to fix it. The
// operator reading this has, by definition, not received a notification about
// it, so the message has to stand on its own.
func notificationDeliveryAlertMessage(health notifications.DeliveryHealth) string {
	if health.Status == notifications.DeliveryUnavailable {
		return "Pulse cannot read the notification queue, so alert delivery cannot be confirmed. Check the Pulse logs and the notification settings."
	}

	switch {
	case health.Failed > 0 && health.DeadLetter > 0:
		return fmt.Sprintf(
			"Alert notifications are not reaching their destinations. %s and %s were not delivered. Check destination credentials and settings under Alerts, Notifications.",
			pluralDeliveries(health.Failed, "failed delivery", "failed deliveries"),
			pluralDeliveries(health.DeadLetter, "dead-lettered delivery", "dead-lettered deliveries"),
		)
	case health.DeadLetter > 0:
		return fmt.Sprintf(
			"Alert notifications are not reaching their destinations. %s gave up after repeated failures. Check destination credentials and settings under Alerts, Notifications.",
			pluralDeliveries(health.DeadLetter, "dead-lettered delivery", "dead-lettered deliveries"),
		)
	default:
		return fmt.Sprintf(
			"Alert notifications are not reaching their destinations. %s were not delivered. Check destination credentials and settings under Alerts, Notifications.",
			pluralDeliveries(health.Failed, "failed delivery", "failed deliveries"),
		)
	}
}

func pluralDeliveries(count int, singular, plural string) string {
	if count == 1 {
		return fmt.Sprintf("%d %s", count, singular)
	}
	return fmt.Sprintf("%d %s", count, plural)
}
