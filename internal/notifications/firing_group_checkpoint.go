package notifications

import (
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rs/zerolog/log"
)

// checkpointPendingFiringGroupLocked transfers the accepted window into ordinary
// queued delivery while n.mu is held and queue workers are stopped. A restart is
// not a flush: retain the original membership, destination snapshots and deadline
// (or the later quiet-hours deadline). Later process admissions start a new
// window, just as work admitted after an ordinary flush does.
//
// Admitted queue/policy cancellations finish before the handoff. Never fall
// back to an immediate provider send on a checkpoint error: it would violate the
// window and quiet hours. Storage errors are reported rather than called durable.
func (n *NotificationManager) checkpointPendingFiringGroupLocked(queue *NotificationQueue) {
	defer func() {
		clear(n.pendingAlerts)
		n.pendingAlerts = n.pendingAlerts[:0]
		n.pendingAlertIndex = nil
		n.groupDeadline = time.Time{}
	}()
	if len(n.pendingAlerts) == 0 || !n.enabled {
		return
	}
	if queue == nil {
		log.Error().Int("alertCount", len(n.pendingAlerts)).Msg("cannot preserve buffered notifications at shutdown: queue unavailable")
		return
	}
	deadline := n.groupDeadline
	if deadline.IsZero() {
		// Legacy manually constructed managers may lack a timer deadline.
		deadline = time.Now().Add(n.groupWindow)
	}
	jobs := buildNotificationDeliveryJobsForTarget(copyEmailConfig(n.emailConfig), copyWebhookConfigs(n.webhooks),
		copyAppriseConfig(n.appriseConfig), n.pendingAlerts, eventAlert, time.Time{}, n.initialTarget)
	for _, job := range jobs {
		var dueAlerts []*alerts.Alert
		for _, alert := range job.Alerts {
			record, found, err := queue.deliveryReceipt(alert, notificationDeliveryDestinationKey(job))
			if err != nil || !found || !receiptInCooldown(record, alert, n.cooldown) {
				dueAlerts = append(dueAlerts, alert)
			}
		}
		job.Alerts = dueAlerts
		if len(dueAlerts) == 0 {
			continue
		}
		epoch, err := queue.activateDeliveryDestination(notificationDeliveryDestinationKey(job), notificationDeliveryConfigurationFingerprint(job))
		if err != nil {
			n.logNotificationJobError(job, err, "failed to preserve notification destination generation")
		} else {
			job.ReceiptEpoch = epoch
			job.ReceiptEpochBound = true
		}
		configJSON, err := configJSONForNotificationDeliveryJob(job)
		if err != nil {
			n.logNotificationJobError(job, err, "failed to preserve buffered notifications at shutdown")
			continue
		}
		for _, bucket := range notificationQueueBucketsForJob(job, time.Now()) {
			due := deadline
			if bucket.nextRetryAt != nil && bucket.nextRetryAt.After(due) {
				due = *bucket.nextRetryAt
			}
			// Queue timestamps have second precision; truncation must never
			// allow a send before either policy deadline.
			if rounded := due.Truncate(time.Second); rounded.Before(due) {
				due = rounded.Add(time.Second)
			}
			row := &QueuedNotification{Type: queueTypeForNotificationDeliveryJob(bucket.job),
				DestinationID: notificationDestinationIDForJob(bucket.job), Alerts: bucket.job.Alerts,
				Config: configJSON, MaxAttempts: 3, NextRetryAt: &due, ReceiptEpoch: deliveryJobEpoch(job)}
			if err := queue.Enqueue(row); err != nil {
				n.logNotificationJobError(bucket.job, err, "failed to preserve buffered notifications at shutdown; no early direct send")
			}
		}
	}
}
