package notifications

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
	"github.com/stretchr/testify/require"
)

func TestNotificationDiagnosticAliasesPreserveDestinationAndQueueLinkage(t *testing.T) {
	dir := t.TempDir()
	q, err := NewNotificationQueue(dir)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, q.Stop()) })
	start := time.Now().UTC().Add(-time.Minute)
	email := EmailConfig{To: []string{"ops@example.invalid"}}
	other := EmailConfig{To: []string{"other@example.invalid"}}
	hook := WebhookConfig{ID: "configured-ops"}
	jobs := []notificationDeliveryJob{
		{Type: "email", EmailConfig: &email},
		{Type: "email", EmailConfig: &other},
		{Type: "webhook", WebhookConfig: &hook},
	}
	rawIDs := make(map[string]string)
	for i, job := range jobs {
		item := &QueuedNotification{
			ID:            []string{"old-ops", "old-other", "old-hook"}[i],
			Type:          job.Type,
			DestinationID: notificationDestinationIDForJob(job),
			Status:        QueueStatusDLQ,
			CreatedAt:     start,
			Config:        json.RawMessage(`{}`),
			Alerts: []*alerts.Alert{{
				ID: "fixture-alert", OperationalRecord: &operationaltrust.OperationalRecord{ID: "record"},
				LatestTransition: &operationaltrust.LifecycleTransition{
					ID: "transition", To: operationaltrust.OperationalOpen, CauseKey: "fixture",
				},
			}},
		}
		rawIDs[item.ID] = item.DestinationID
		require.NoError(t, q.Enqueue(item))
		require.NoError(t, q.RecordAudit(item, false, "synthetic refusal"))
	}
	// Also cover delivery log entries from before the dedicated column.
	_, err = q.db.Exec(`UPDATE notification_audit SET destination_id = '' WHERE notification_id = 'old-other'`)
	require.NoError(t, err)

	readAliases := func(q *NotificationQueue) map[string]string {
		t.Helper()
		log, err := q.GetDeliveryLog(start, 10)
		require.NoError(t, err)
		require.Len(t, log, len(jobs))
		aliases := make(map[string]string, len(log))
		for _, entry := range log {
			aliases[entry.NotificationID] = entry.DestinationID
			require.Equal(t, DeliveryOutcomeDeadLetter, entry.Outcome)
		}
		require.Equal(t, "webhook:configured-ops", aliases["old-hook"])
		for _, id := range []string{"old-ops", "old-other"} {
			require.NotEqual(t, rawIDs[id], aliases[id])
			require.Contains(t, aliases[id], destinationAliasPrefix)
		}
		require.NotEqual(t, aliases["old-ops"], aliases["old-other"])
		rows, err := q.GetDLQ(10)
		require.NoError(t, err)
		require.Len(t, rows, len(jobs))
		for _, row := range rows {
			require.Equal(t, aliases[row.ID], row.DestinationID)
			require.Len(t, row.Links, 1)
			link := row.Links[0]
			require.Equal(t, row.DestinationID, link.DestinationID)
			require.Equal(t, row.ID, link.NotificationID)
			require.Equal(t, "transition", link.TransitionID)
			require.Equal(t, operationaltrust.NotificationDeadLetter, link.DeliveryState)
			stored, err := q.readNotificationLinksNoLock(row.ID)
			require.NoError(t, err)
			require.Len(t, stored, 1)
			require.Equal(t, rawIDs[row.ID], stored[0].DestinationID, "projection must not rewrite queue links")
		}
		return aliases
	}
	before := readAliases(q)
	require.NoError(t, q.Stop())
	q, err = NewNotificationQueue(dir)
	require.NoError(t, err)
	require.Equal(t, before, readAliases(q))

	// New linked queue handles must not be derived from the unprojected ID.
	item := &QueuedNotification{
		DestinationID: rawIDs["old-ops"], Type: "email", Status: QueueStatusDLQ, Config: json.RawMessage(`{}`),
		CreatedAt: start, Links: []operationaltrust.NotificationLink{{
			OperationalRecordID: "record", TransitionID: "transition",
			LifecycleState: operationaltrust.OperationalOpen, CauseKey: "fixture",
		}},
	}
	require.NoError(t, q.Enqueue(item))
	wantID, err := operationaltrust.NewNotificationID(before["old-ops"], item.Type, start, []string{"transition"})
	require.NoError(t, err)
	require.Equal(t, wantID, item.ID)

	independent, err := NewNotificationQueue(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, independent.Stop()) })
	require.NotEqual(t, before["old-ops"], independent.diagnosticDestinationID(rawIDs["old-ops"]))
}

func TestNotificationDiagnosticAliasesFallbackLeavesQueueAvailable(t *testing.T) {
	dir := t.TempDir()
	q, err := NewNotificationQueue(dir)
	require.NoError(t, err)
	raw := notificationDestinationIDForJob(notificationDeliveryJob{
		Type: "apprise", AppriseConfig: &AppriseConfig{Mode: AppriseModeHTTP, ServerURL: "https://notify.example.invalid"},
	})
	first := q.diagnosticDestinationID(raw)
	_, err = q.db.Exec(`DELETE FROM notification_diagnostic_identity;
		CREATE TRIGGER reject_diagnostic_key BEFORE INSERT ON notification_diagnostic_identity
		BEGIN SELECT RAISE(ABORT, 'synthetic key persistence fault'); END;`)
	require.NoError(t, err)
	require.NoError(t, q.Stop())
	q, err = NewNotificationQueue(dir)
	require.NoError(t, err, "a diagnostics storage fault must not fail queue construction")
	t.Cleanup(func() { require.NoError(t, q.Stop()) })
	temporary := q.diagnosticDestinationID(raw)
	require.Contains(t, temporary, destinationAliasPrefix)
	require.NotEqual(t, first, temporary)
	require.NotEqual(t, raw, temporary)
	require.Equal(t, temporary, q.diagnosticDestinationID(raw))
	require.NoError(t, q.Enqueue(&QueuedNotification{ID: "still-queued", Type: "apprise", Config: json.RawMessage(`{}`)}))
	stats, err := q.GetQueueStats()
	require.NoError(t, err)
	require.Equal(t, 1, stats["pending"])
}
