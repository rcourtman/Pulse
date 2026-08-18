package notifications

// Delivery health is the queue's own verdict on whether configured
// destinations are reaching anyone. It lives here, next to the queue that
// produces the counts, so the API surface and the monitoring loop that raises
// the notification-delivery system alert share one rule instead of each
// carrying their own copy of it.

// DeliveryHealthStatus is the coarse verdict callers act on.
type DeliveryHealthStatus string

const (
	// DeliveryHealthy means no terminal delivery failure is retained.
	DeliveryHealthy DeliveryHealthStatus = "healthy"
	// DeliveryDegraded means retained failed or dead-lettered deliveries exist.
	DeliveryDegraded DeliveryHealthStatus = "degraded"
	// DeliveryUnavailable means the queue could not report, so delivery cannot
	// be vouched for either way.
	DeliveryUnavailable DeliveryHealthStatus = "unavailable"
)

const (
	reasonRetainedFailedDeliveries     = "retained_failed_deliveries"
	reasonRetainedDeadLetterDeliveries = "retained_dead_letter_deliveries"
	reasonQueueStatsUnavailable        = "queue_stats_unavailable"
)

// DeliveryHealth is a content-free view of delivery outcomes. It carries no
// destination, alert, or recipient identity.
type DeliveryHealth struct {
	Status            DeliveryHealthStatus
	Healthy           bool
	AttentionRequired int
	ReasonCodes       []string
	Failed            int
	DeadLetter        int
}

// ClassifyQueueHealth turns raw queue counts into the delivery verdict.
// Recoverable retry attempts deliberately do not count: only outcomes that
// have reached a terminal failed or dead-letter state mean something was not
// delivered.
func ClassifyQueueHealth(stats map[string]int) DeliveryHealth {
	failed := stats[string(QueueStatusFailed)]
	deadLetter := stats[string(QueueStatusDLQ)]

	reasonCodes := make([]string, 0, 2)
	if failed > 0 {
		reasonCodes = append(reasonCodes, reasonRetainedFailedDeliveries)
	}
	if deadLetter > 0 {
		reasonCodes = append(reasonCodes, reasonRetainedDeadLetterDeliveries)
	}

	attentionRequired := failed + deadLetter
	health := DeliveryHealth{
		Healthy:           attentionRequired == 0,
		AttentionRequired: attentionRequired,
		ReasonCodes:       reasonCodes,
		Failed:            failed,
		DeadLetter:        deadLetter,
	}
	if health.Healthy {
		health.Status = DeliveryHealthy
	} else {
		health.Status = DeliveryDegraded
	}
	return health
}

// UnavailableDeliveryHealth is the verdict when the queue cannot be read.
func UnavailableDeliveryHealth() DeliveryHealth {
	return DeliveryHealth{
		Status:      DeliveryUnavailable,
		Healthy:     false,
		ReasonCodes: []string{reasonQueueStatsUnavailable},
	}
}

// DeliveryHealth reports the queue's current delivery verdict. A queue that
// cannot be read is reported as unavailable rather than healthy, because the
// whole point of this signal is that silence must not read as success.
func (n *NotificationManager) DeliveryHealth() DeliveryHealth {
	if n == nil {
		return UnavailableDeliveryHealth()
	}
	stats, err := n.GetQueueStats()
	if err != nil || stats == nil {
		return UnavailableDeliveryHealth()
	}
	return ClassifyQueueHealth(stats)
}
