package notifications

import (
	"database/sql"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

// Unit fixtures explicitly establish a current admitted receiver generation.
// Runtime completion has no unbound/legacy writer that can grant authority.
func fixtureDeliveryEpoch(q *NotificationQueue, key string) (string, error) {
	q.mu.RLock()
	var epoch string
	err := q.db.QueryRow(`SELECT epoch FROM notification_destination_state WHERE destination_key=?`, key).Scan(&epoch)
	q.mu.RUnlock()
	if err == sql.ErrNoRows {
		return q.activateDeliveryDestination(key, "")
	}
	return epoch, err
}

func recordFixtureDeliveryReceipt(q *NotificationQueue, id string, start time.Time, key string, at time.Time, rank int, epochs ...string) error {
	epoch := ""
	if len(epochs) > 0 {
		epoch = epochs[0]
	} else {
		var err error
		epoch, err = fixtureDeliveryEpoch(q, key)
		if err != nil {
			return err
		}
	}
	return q.recordDeliveryReceipt(id, start, key, at, rank, epoch)
}

func recordFixtureSuccessfulDelivery(n *NotificationManager, job notificationDeliveryJob, at time.Time) error {
	if q := n.GetQueue(); q != nil && !job.ReceiptEpochBound {
		key := notificationDeliveryDestinationKey(job)
		var err error
		job.ReceiptEpoch, err = fixtureDeliveryEpoch(q, key)
		if err != nil {
			return err
		}
		job.ReceiptEpochBound = true
	}
	return n.recordSuccessfulDelivery(job, at)
}

func sendFixtureNotificationsDirect(n *NotificationManager, email EmailConfig, hooks []WebhookConfig, apprise AppriseConfig, list []*alerts.Alert) {
	n.SetEnabled(true)
	n.dispatchNotificationJobsAsync(n.bindDeliveryEpochs(buildNotificationDeliveryJobs(email, hooks, apprise, list, eventAlert, time.Time{})))
}

func sendFixtureResolvedNotificationsDirect(n *NotificationManager, email EmailConfig, hooks []WebhookConfig, apprise AppriseConfig, list []*alerts.Alert, at time.Time) {
	n.SetEnabled(true)
	n.dispatchNotificationJobsAsync(n.bindDeliveryEpochs(buildNotificationDeliveryJobs(email, hooks, apprise, list, eventResolved, at)))
}
