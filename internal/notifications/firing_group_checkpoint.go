package notifications

import (
	"time"

	"github.com/rs/zerolog/log"
)

// checkpointPendingFiringGroupLocked transfers the accepted window into ordinary
// queued delivery while n.mu is held and queue workers are stopped. A restart is
// not a flush: retain the original membership, destination snapshots and deadline
// (or the later quiet-hours deadline). Later process admissions start a new
// window, just as work admitted after an ordinary flush does.
//
// Queue/policy cancellation remains available during the handoff. Never fall
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
				Config: configJSON, MaxAttempts: 3, NextRetryAt: &due}
			if err := queue.Enqueue(row); err != nil {
				n.logNotificationJobError(bucket.job, err, "failed to preserve buffered notifications at shutdown; no early direct send")
			}
		}
	}
}
