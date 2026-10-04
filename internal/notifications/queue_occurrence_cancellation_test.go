package notifications

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

func TestQueueOccurrenceCancellationPreservesRecurrenceAcrossRestart(t *testing.T) {
	for _, status := range []NotificationQueueStatus{QueueStatusPending, QueueStatusSending, QueueStatusFailed, QueueStatusDLQ} {
		t.Run(string(status), func(t *testing.T) {
			dir := t.TempDir()
			q, err := NewNotificationQueue(dir)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = q.Stop() })
			start := time.Now().UTC().Add(-time.Hour)
			old := quietReplayAlert("reusable", alerts.AlertLevelWarning, start)
			current := quietReplayAlert("reusable", alerts.AlertLevelCritical, start.Add(time.Nanosecond))
			current.OperationalRecord.ID += "-new"
			current.LatestTransition.ID += "-new"
			peer := quietReplayAlert("peer", alerts.AlertLevelWarning, start)
			for _, row := range []*QueuedNotification{
				{ID: "old", Type: "webhook", Alerts: []*alerts.Alert{old}},
				{ID: "new", Type: "webhook", Alerts: []*alerts.Alert{current}},
				{ID: "mixed", Type: "webhook", Alerts: []*alerts.Alert{old, current, peer}},
				{ID: "recovery", Type: "webhook_resolved", Alerts: []*alerts.Alert{old}},
				{ID: "unknown", Type: "webhook", Alerts: []*alerts.Alert{{ID: old.ID}}},
			} {
				row.Config = []byte("{}")
				row.DestinationID = "webhook:ops"
				row.MaxAttempts = 3
				if err := q.Enqueue(row); err != nil {
					t.Fatal(err)
				}
				if err := q.UpdateStatus(row.ID, status, "retained-attempt"); err != nil {
					t.Fatal(err)
				}
				if err := q.RecordAudit(row, false, "retained-attempt"); err != nil {
					t.Fatal(err)
				}
			}
			if err := q.RecordDeliveryReceipt(current.ID, current.StartTime, "webhook:ops", time.Now()); err != nil {
				t.Fatal(err)
			}
			for _, identity := range []struct {
				id    string
				start time.Time
			}{{"", start}, {old.ID, time.Time{}}} {
				if count, err := q.CancelByAlertOccurrence(identity.id, identity.start); err != nil || count != 0 {
					t.Fatalf("missing identity widened cancellation: %d, %v", count, err)
				}
			}
			count, err := q.CancelByAlertOccurrence(old.ID, old.StartTime)
			wantCount := 0
			if status == QueueStatusPending {
				wantCount = 2
			}
			if err != nil || count != wantCount {
				t.Fatalf("cancel count = %d, %v, want %d", count, err, wantCount)
			}
			if row := loadQuietReplay(t, q, "mixed"); row.Status != status || len(row.Alerts) != 2 {
				t.Fatalf("cancellation changed surviving row status: %+v", row)
			}
			if err := q.Stop(); err != nil {
				t.Fatal(err)
			}
			q, err = NewNotificationQueue(dir)
			if err != nil {
				t.Fatal(err)
			}
			reopenStatus := status
			if status == QueueStatusSending {
				// Existing crash recovery requeues interrupted sends. Exact
				// cancellation must preserve that behaviour for survivors.
				reopenStatus = QueueStatusPending
			}
			oldRow, mixed := loadQuietReplay(t, q, "old"), loadQuietReplay(t, q, "mixed")
			if oldRow.Status != QueueStatusCancelled || mixed.Status != reopenStatus || len(mixed.Alerts) != 2 ||
				!mixed.Alerts[0].StartTime.Equal(current.StartTime) || mixed.Alerts[1].ID != peer.ID || len(mixed.Links) != 2 ||
				mixed.Links[0].OperationalRecordID != current.OperationalRecord.ID || mixed.Links[1].OperationalRecordID != peer.OperationalRecord.ID {
				t.Fatalf("wrong retained rows/links: old=%+v mixed=%+v", oldRow, mixed)
			}
			for _, id := range []string{"new", "recovery", "unknown"} {
				if row := loadQuietReplay(t, q, id); row.Status != reopenStatus || len(row.Alerts) != 1 {
					t.Fatalf("unrelated row %s changed: %+v", id, row)
				}
			}
			if has, err := q.HasDeliveryReceipt(current.ID, current.StartTime, "webhook:ops"); err != nil || !has {
				t.Fatalf("current occurrence lost its receipt: %t, %v", has, err)
			}
			var audits int
			if err := q.db.QueryRow(`SELECT count(*) FROM notification_audit WHERE success = 0`).Scan(&audits); err != nil || audits != 5 {
				t.Fatalf("retained failed attempts = %d, %v", audits, err)
			}
		})
	}
}

func TestCancelResolvedAlertPreservesNewGroupingAndCooldown(t *testing.T) {
	start := time.Now().Add(-time.Minute)
	old := &alerts.Alert{ID: "reusable", StartTime: start}
	current := &alerts.Alert{ID: old.ID, StartTime: start.Add(time.Nanosecond)}
	record := notificationRecord{alertStart: current.StartTime, lastSent: time.Now(), level: alerts.AlertLevelCritical}
	n := &NotificationManager{pendingAlerts: []*alerts.Alert{old, current}, lastNotified: map[string]notificationRecord{old.ID: record}}
	if n.CancelResolvedAlert(&alerts.Alert{ID: old.ID}) || len(n.pendingAlerts) != 2 {
		t.Fatal("missing occurrence cancelled pending work")
	}
	if !n.CancelResolvedAlert(old) || len(n.pendingAlerts) != 1 || n.pendingAlerts[0] != current || n.lastNotified[current.ID] != record {
		t.Fatal("delayed recovery removed current grouping or cooldown state")
	}
	// Keep the explicit, identifier-wide cancellation contract for its callers.
	n.CancelAlert(current.ID)
	if len(n.pendingAlerts) != 0 || len(n.lastNotified) != 0 {
		t.Fatal("ID-wide cancellation regressed")
	}
}
