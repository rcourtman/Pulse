package notifications

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
)

func TestGetDeliveryLogReturnsWindowedAttemptsNewestFirst(t *testing.T) {
	nq, err := NewNotificationQueue(t.TempDir())
	if err != nil {
		t.Fatalf("NewNotificationQueue: %v", err)
	}
	defer func() { _ = nq.Stop() }()

	now := time.Now().UTC()
	sent := &QueuedNotification{
		ID:            "email-sent",
		Type:          "email",
		DestinationID: "destination:aaaa",
		Status:        QueueStatusPending,
		Alerts:        []*alerts.Alert{{ID: "disk-critical-1", StartTime: now.Add(-time.Hour)}},
		Config:        []byte(`{}`),
		CreatedAt:     now.Add(-time.Hour),
	}
	failed := &QueuedNotification{
		ID:            "webhook-dlq",
		Type:          "webhook",
		DestinationID: "webhook:wh-ops",
		Status:        QueueStatusPending,
		Alerts:        []*alerts.Alert{{ID: "vm-offline-101", StartTime: now.Add(-time.Hour)}},
		Config:        []byte(`{}`),
		CreatedAt:     now.Add(-time.Hour),
	}
	old := &QueuedNotification{
		ID:        "email-old",
		Type:      "email",
		Status:    QueueStatusPending,
		Config:    []byte(`{}`),
		CreatedAt: now.Add(-8 * 24 * time.Hour),
	}
	for _, notif := range []*QueuedNotification{sent, failed, old} {
		if err := nq.Enqueue(notif); err != nil {
			t.Fatalf("enqueue %s: %v", notif.ID, err)
		}
	}

	old.Attempts = 1
	old.Status = QueueStatusSent
	if err := nq.RecordAudit(old, true, ""); err != nil {
		t.Fatalf("record old delivery: %v", err)
	}
	sent.Attempts = 1
	sent.Status = QueueStatusSent
	if err := nq.RecordAudit(sent, true, ""); err != nil {
		t.Fatalf("record sent delivery: %v", err)
	}
	failed.Attempts = 1
	failed.Status = QueueStatusPending
	if err := nq.RecordAudit(failed, false, "HTTP 401 Unauthorized"); err != nil {
		t.Fatalf("record retry attempt: %v", err)
	}
	failed.Attempts = 3
	failed.Status = QueueStatusDLQ
	if err := nq.RecordAudit(failed, false, "HTTP 401 Unauthorized"); err != nil {
		t.Fatalf("record dead-letter attempt: %v", err)
	}
	if _, err := nq.db.Exec(
		`UPDATE notification_audit SET timestamp = ? WHERE notification_id = ?`,
		now.Add(-8*24*time.Hour).Unix(),
		"email-old",
	); err != nil {
		t.Fatalf("age old audit row: %v", err)
	}

	entries, err := nq.GetDeliveryLog(now.Add(-7*24*time.Hour), 0)
	if err != nil {
		t.Fatalf("GetDeliveryLog: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("entries = %d, want 3 (windowed rows only): %#v", len(entries), entries)
	}

	deadLetter := entries[0]
	if deadLetter.NotificationID != "webhook-dlq" ||
		deadLetter.Outcome != DeliveryOutcomeDeadLetter ||
		deadLetter.DestinationID != "webhook:wh-ops" ||
		deadLetter.Attempts != 3 ||
		deadLetter.Success ||
		deadLetter.FailureClass != "authentication" {
		t.Fatalf("dead-letter entry = %#v", deadLetter)
	}
	if len(deadLetter.AlertIDs) != 1 || deadLetter.AlertIDs[0] != "vm-offline-101" ||
		deadLetter.AlertCount != 1 {
		t.Fatalf("dead-letter alerts = %#v", deadLetter)
	}

	retry := entries[1]
	if retry.NotificationID != "webhook-dlq" || retry.Outcome != DeliveryOutcomeRetry {
		t.Fatalf("retry entry = %#v", retry)
	}

	delivered := entries[2]
	if delivered.NotificationID != "email-sent" ||
		delivered.Outcome != DeliveryOutcomeSent ||
		!delivered.Success ||
		delivered.DestinationID != "destination:aaaa" ||
		delivered.FailureClass != "" ||
		delivered.ErrorMessage != "" {
		t.Fatalf("delivered entry = %#v", delivered)
	}
	if delivered.Timestamp.IsZero() {
		t.Fatalf("delivered entry has no timestamp: %#v", delivered)
	}
}

func TestGetDeliveryLogHonorsLimitAndCap(t *testing.T) {
	nq, err := NewNotificationQueue(t.TempDir())
	if err != nil {
		t.Fatalf("NewNotificationQueue: %v", err)
	}
	defer func() { _ = nq.Stop() }()

	now := time.Now().UTC()
	notif := &QueuedNotification{
		ID:        "email-1",
		Type:      "email",
		Status:    QueueStatusPending,
		Config:    []byte(`{}`),
		CreatedAt: now,
	}
	if err := nq.Enqueue(notif); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	notif.Status = QueueStatusSent
	for i := 0; i < 3; i++ {
		notif.Attempts = i + 1
		if err := nq.RecordAudit(notif, true, ""); err != nil {
			t.Fatalf("record delivery %d: %v", i, err)
		}
	}

	entries, err := nq.GetDeliveryLog(now.Add(-time.Hour), 2)
	if err != nil {
		t.Fatalf("GetDeliveryLog: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries = %d, want limit of 2", len(entries))
	}
	if entries[0].Attempts != 3 {
		t.Fatalf("newest entry = %#v, want the latest attempt first", entries[0])
	}

	if capped, err := nq.GetDeliveryLog(now.Add(-time.Hour), maxDeliveryLogEntries*10); err != nil {
		t.Fatalf("GetDeliveryLog capped: %v", err)
	} else if len(capped) != 3 {
		t.Fatalf("capped entries = %d, want all 3", len(capped))
	}
}

func TestGetDeliveryLogCanReadRetainedDeadLetterWindow(t *testing.T) {
	nq, err := NewNotificationQueue(t.TempDir())
	if err != nil {
		t.Fatalf("NewNotificationQueue: %v", err)
	}
	defer func() { _ = nq.Stop() }()

	now := time.Now().UTC()
	notif := &QueuedNotification{
		ID:        "webhook-retained-dlq",
		Type:      "webhook",
		Status:    QueueStatusPending,
		Config:    []byte(`{}`),
		CreatedAt: now.Add(-20 * 24 * time.Hour),
	}
	if err := nq.Enqueue(notif); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	notif.Status = QueueStatusDLQ
	notif.Attempts = 3
	if err := nq.RecordAudit(notif, false, "destination unreachable"); err != nil {
		t.Fatalf("record delivery: %v", err)
	}
	if _, err := nq.db.Exec(
		`UPDATE notification_audit SET timestamp = ? WHERE notification_id = ?`,
		now.Add(-20*24*time.Hour).Unix(),
		notif.ID,
	); err != nil {
		t.Fatalf("age dead-letter audit row: %v", err)
	}

	sevenDayEntries, err := nq.GetDeliveryLog(now.Add(-7*24*time.Hour), 0)
	if err != nil {
		t.Fatalf("GetDeliveryLog seven-day window: %v", err)
	}
	if len(sevenDayEntries) != 0 {
		t.Fatalf("seven-day entries = %#v, want old dead-letter excluded", sevenDayEntries)
	}

	thirtyDayEntries, err := nq.GetDeliveryLog(now.Add(-30*24*time.Hour), 0)
	if err != nil {
		t.Fatalf("GetDeliveryLog thirty-day window: %v", err)
	}
	if len(thirtyDayEntries) != 1 ||
		thirtyDayEntries[0].NotificationID != notif.ID ||
		thirtyDayEntries[0].Outcome != DeliveryOutcomeDeadLetter {
		t.Fatalf("thirty-day entries = %#v, want retained dead-letter evidence", thirtyDayEntries)
	}
}

func TestGetDeliveryLogFallsBackToOperationalLinksForLegacyRows(t *testing.T) {
	nq, err := NewNotificationQueue(t.TempDir())
	if err != nil {
		t.Fatalf("NewNotificationQueue: %v", err)
	}
	defer func() { _ = nq.Stop() }()

	now := time.Now().UTC()
	notif := &QueuedNotification{
		ID:        "webhook-legacy",
		Type:      "webhook",
		Status:    QueueStatusPending,
		Config:    []byte(`{}`),
		CreatedAt: now,
	}
	if err := nq.Enqueue(notif); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	notif.Status = QueueStatusSent
	notif.Attempts = 1
	notif.Links = []operationaltrust.NotificationLink{{DestinationID: "webhook:legacy"}}
	if err := nq.RecordAudit(notif, true, ""); err != nil {
		t.Fatalf("record delivery: %v", err)
	}
	// Simulate a row written before the destination_id column existed.
	if _, err := nq.db.Exec(
		`UPDATE notification_audit SET destination_id = '' WHERE notification_id = ?`,
		"webhook-legacy",
	); err != nil {
		t.Fatalf("clear destination column: %v", err)
	}

	entries, err := nq.GetDeliveryLog(now.Add(-time.Hour), 0)
	if err != nil {
		t.Fatalf("GetDeliveryLog: %v", err)
	}
	if len(entries) != 1 || entries[0].DestinationID != "webhook:legacy" {
		t.Fatalf("entries = %#v, want destination decoded from links", entries)
	}
}

func TestGetDeliveryLogFailsClosedWithoutQueue(t *testing.T) {
	var nq *NotificationQueue
	if _, err := nq.GetDeliveryLog(time.Now().Add(-time.Hour), 0); err == nil {
		t.Fatal("nil queue returned a delivery log instead of an error")
	}
	var manager *NotificationManager
	if _, err := manager.GetDeliveryLog(time.Now().Add(-time.Hour), 0); err == nil {
		t.Fatal("nil manager returned a delivery log instead of an error")
	}
}
