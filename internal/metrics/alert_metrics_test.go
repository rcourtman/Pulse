package metrics

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

func TestRecordAlertFired(t *testing.T) {
	alert := &alerts.Alert{
		ID:        "test-alert-1",
		Level:     alerts.AlertLevelWarning,
		Type:      "container_cpu",
		StartTime: time.Now(),
	}

	// Should not panic
	RecordAlertFired(alert)
}

func TestRecordAlertResolved(t *testing.T) {
	now := time.Now()
	alert := &alerts.Alert{
		ID:        "test-alert-2",
		Level:     alerts.AlertLevelWarning,
		Type:      "container_memory",
		StartTime: now.Add(-5 * time.Minute),
		LastSeen:  now,
	}

	// Should not panic
	RecordAlertResolved(alert)
}

func TestRecordAlertAcknowledged(t *testing.T) {
	// Should not panic
	RecordAlertAcknowledged()
}

func TestRecordNotificationSent_Success(t *testing.T) {
	// Should not panic
	RecordNotificationSent("email", true)
}

func TestRecordNotificationSent_Failure(t *testing.T) {
	// Should not panic
	RecordNotificationSent("webhook", false)
}

func TestRecordNotificationRetry(t *testing.T) {
	// Should not panic
	RecordNotificationRetry("email")
}

func TestRecordNotificationDLQ(t *testing.T) {
	// Should not panic
	RecordNotificationDLQ()
}

func TestRecordGroupedNotification(t *testing.T) {
	// Should not panic with various counts
	RecordGroupedNotification(1)
	RecordGroupedNotification(5)
	RecordGroupedNotification(50)
}

func TestRecordAlertEscalation(t *testing.T) {
	// Should not panic with various levels
	RecordAlertEscalation(1)
	RecordAlertEscalation(2)
	RecordAlertEscalation(3)
}

func TestRecordAlertSuppressed(t *testing.T) {
	// Should not panic with various reasons
	RecordAlertSuppressed("quiet_hours")
	RecordAlertSuppressed("rate_limit")
	RecordAlertSuppressed("duplicate")
}

func TestUpdateQueueDepth(t *testing.T) {
	// Should not panic with various statuses and counts
	UpdateQueueDepth("pending", 10)
	UpdateQueueDepth("sending", 5)
	UpdateQueueDepth("dlq", 0)
}

func TestUpdateHistorySize(t *testing.T) {
	// Should not panic with various sizes
	UpdateHistorySize(0)
	UpdateHistorySize(100)
	UpdateHistorySize(1000)
}

func TestRecordHistorySaveError(t *testing.T) {
	// Should not panic
	RecordHistorySaveError()
}

func TestMetricVectors_NotNil(t *testing.T) {
	// Verify that metric vectors are properly initialized
	if AlertsActive == nil {
		t.Error("AlertsActive should not be nil")
	}
	if AlertsFiredTotal == nil {
		t.Error("AlertsFiredTotal should not be nil")
	}
	if AlertsResolvedTotal == nil {
		t.Error("AlertsResolvedTotal should not be nil")
	}
	if AlertsAcknowledgedTotal == nil {
		t.Error("AlertsAcknowledgedTotal should not be nil")
	}
	if AlertDurationSeconds == nil {
		t.Error("AlertDurationSeconds should not be nil")
	}
	if NotificationsSentTotal == nil {
		t.Error("NotificationsSentTotal should not be nil")
	}
	if NotificationQueueDepth == nil {
		t.Error("NotificationQueueDepth should not be nil")
	}
	if NotificationDeliveryDuration == nil {
		t.Error("NotificationDeliveryDuration should not be nil")
	}
	if NotificationRetriesTotal == nil {
		t.Error("NotificationRetriesTotal should not be nil")
	}
	if NotificationDLQTotal == nil {
		t.Error("NotificationDLQTotal should not be nil")
	}
	if NotificationsGroupedTotal == nil {
		t.Error("NotificationsGroupedTotal should not be nil")
	}
	if AlertsGroupedCount == nil {
		t.Error("AlertsGroupedCount should not be nil")
	}
	if AlertEscalationsTotal == nil {
		t.Error("AlertEscalationsTotal should not be nil")
	}
	if AlertHistorySize == nil {
		t.Error("AlertHistorySize should not be nil")
	}
	if AlertHistorySaveErrors == nil {
		t.Error("AlertHistorySaveErrors should not be nil")
	}
	if AlertsSuppressedTotal == nil {
		t.Error("AlertsSuppressedTotal should not be nil")
	}
	if AlertsRateLimitedTotal == nil {
		t.Error("AlertsRateLimitedTotal should not be nil")
	}
}
