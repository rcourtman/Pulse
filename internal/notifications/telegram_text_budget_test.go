package notifications

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf16"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
	"github.com/stretchr/testify/require"
)

type telegramSizeObservation struct {
	content telegramLiteralContent
	status  int
	units   int
}

func telegramSizeReceiver(t *testing.T, reject string) (string, func() []telegramSizeObservation) {
	t.Helper()
	var mu sync.Mutex
	var requests []telegramSizeObservation
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload telegramLiteralPayload
		err := json.NewDecoder(r.Body).Decode(&payload)
		content, parseErr := parseTelegramLiteralContent(payload)
		units := len(utf16.Encode([]rune(content.text))) // independent of the planner
		status := http.StatusOK
		if err != nil || parseErr != nil || units > 4096 || payload.ChatID != "-1001234" ||
			(reject != "" && strings.Contains(content.text, reject)) {
			status = http.StatusBadRequest
		}
		mu.Lock()
		requests = append(requests, telegramSizeObservation{content, status, units})
		mu.Unlock()
		w.WriteHeader(status)
	}))
	t.Cleanup(server.Close)
	return server.URL + "/sendMessage?chat_id=-1001234", func() []telegramSizeObservation {
		mu.Lock()
		defer mu.Unlock()
		return append([]telegramSizeObservation(nil), requests...)
	}
}

func telegramSizeManager(t *testing.T, dir, url string) (*NotificationManager, WebhookConfig) {
	t.Helper()
	m := NewNotificationManagerWithDeferredQueue("https://pulse.example.test", dir)
	t.Cleanup(m.Stop)
	require.NoError(t, m.UpdateAllowedPrivateCIDRs("127.0.0.1/32"))
	hook := WebhookConfig{ID: "telegram", Name: "Synthetic Telegram", Service: "telegram", Enabled: true, URL: url}
	m.AddWebhook(hook)
	m.SetGroupingConfig(true, 60, false, false)
	return m, hook
}

func awaitTelegramSizeQueue(t *testing.T, q *NotificationQueue, sent, dead, held int) {
	t.Helper()
	require.Eventually(t, func() bool {
		stats, err := q.GetQueueStats()
		return err == nil && stats["sent"] == sent && stats["dlq"] == dead &&
			stats["pending"] == held && stats["sending"] == 0
	}, 10*time.Second, 5*time.Millisecond)
}

// The receiver enforces just the built-in provider's documented text budget
// after entity parsing; it is not a model of Telegram's network or service.
func TestTelegramOversizedGroupDeliversEveryMember(t *testing.T) {
	url, received := telegramSizeReceiver(t, "")
	m, _ := telegramSizeManager(t, t.TempDir(), url)
	batch := telegramReportedAlerts()
	for _, alert := range batch {
		alert.Message += "  " + strings.Repeat("<&🧪>_", 185) + " end of " + alert.ID + "."
		m.SendAlert(alert)
	}
	m.sendGroupedAlerts()
	original, err := m.queue.GetPending(10)
	require.NoError(t, err)
	require.Len(t, original, 1, "normal grouping still selects one delivery")
	require.NoError(t, m.StartQueueProcessing())
	awaitTelegramSizeQueue(t, m.queue, 2, 0, 0)

	requests := received()
	require.Len(t, requests, 2)
	for _, request := range requests {
		require.Equal(t, http.StatusOK, request.status)
		require.LessOrEqual(t, request.units, 4096)
		require.Equal(t, 2, request.content.bold)
		require.Len(t, request.content.links, 1)
	}
	for _, alert := range batch {
		matches := 0
		for _, request := range requests {
			matches += strings.Count(request.content.text, alert.Message)
		}
		require.Equal(t, 1, matches, "each firing member's full literal message is delivered once")
	}
	require.Empty(t, mustTelegramDLQ(t, m.queue))
}

func mustTelegramDLQ(t *testing.T, q *NotificationQueue) []*QueuedNotification {
	t.Helper()
	rows, err := q.GetDLQ(10)
	require.NoError(t, err)
	return rows
}

func TestTelegramOversizedSingletonFitsBeforeEscaping(t *testing.T) {
	url, received := telegramSizeReceiver(t, "")
	m, hook := telegramSizeManager(t, t.TempDir(), url)
	alert := telegramReportedAlerts()[0]
	alert.Message = "poll_nodes failed <timeout> & " + strings.Repeat("🧪<&>_\n", 1000)
	original := alert.Clone()
	require.NoError(t, m.sendGroupedWebhook(hook, []*alerts.Alert{alert}))
	require.Equal(t, original, alert)
	firing := received()[0]
	require.Equal(t, http.StatusOK, firing.status)
	require.GreaterOrEqual(t, firing.units, 4095)
	require.Contains(t, firing.content.text, "poll_nodes failed <timeout> &")
	require.Contains(t, firing.content.text, "…\n\nDetails:")
	require.Len(t, firing.content.links, 1)
	require.Contains(t, firing.content.links[0], alert.ID)
	require.NotContains(t, firing.content.text, "�", "do not cut an astral character or an HTML entity")

	longLabel := "long resource <&> " + strings.Repeat("🧪", 5000)
	alert.ResourceName, alert.Node = longLabel, longLabel
	original = alert.Clone()
	require.NoError(t, m.sendResolvedWebhook(hook, []*alerts.Alert{alert}, time.Now()))
	require.Equal(t, original, alert)
	recovery := received()[1]
	require.Equal(t, http.StatusOK, recovery.status)
	require.Contains(t, recovery.content.text, "Resolved: long resource <&>")
	require.Contains(t, recovery.content.text, "…")
	require.NotContains(t, recovery.content.text, "�")
}

func TestTelegramPartsKeepRateHoldAndProviderRejectionIndependent(t *testing.T) {
	url, received := telegramSizeReceiver(t, "reject-this-part")
	dir := t.TempDir()
	m, hook := telegramSizeManager(t, dir, url)
	start := time.Now().Add(-time.Minute)
	var batch []*alerts.Alert
	for _, id := range []string{"first-part", "reject-this-part", "third-part"} {
		alert := quietReplayAlert(id, alerts.AlertLevelWarning, start)
		alert.Message = id + " " + strings.Repeat("🧪<&", 550)
		batch = append(batch, alert)
		m.SendAlert(alert)
	}
	m.sendGroupedAlerts()
	q := m.GetQueue()
	pending, err := q.GetPending(10)
	require.NoError(t, err)
	require.Len(t, pending, 1)
	original := pending[0]
	for i := 0; i < WebhookRateLimitMax-1; i++ {
		require.True(t, m.checkWebhookRateLimit(url))
	}
	require.NoError(t, m.StartQueueProcessing())
	// The first part receives the one remaining slot. The other parts
	// independently reach the same provider-free hold, not the DLQ.
	awaitTelegramSizeQueue(t, q, 1, 0, 2)
	require.Eventually(t, func() bool {
		var held int
		err := q.db.QueryRow("SELECT count(*) FROM notification_queue WHERE status='pending' AND last_error=?", telegramRateHoldMessage).Scan(&held)
		return err == nil && held == 2
	}, 10*time.Second, 5*time.Millisecond)
	require.Len(t, received(), 1)
	for i := 1; i < len(batch); i++ {
		row := loadQuietReplay(t, q, fmt.Sprintf("%s:telegram:%d", original.ID, i))
		require.Equal(t, QueueStatusPending, row.Status)
		require.Zero(t, row.Attempts)
		require.Nil(t, row.LastAttempt)
		require.NotNil(t, row.NextRetryAt)
		require.True(t, row.NextRetryAt.After(time.Now().Add(30*time.Second)))
		require.Equal(t, original.Config, row.Config)
		require.Equal(t, original.ReceiptEpoch, row.ReceiptEpoch)
		require.Len(t, row.Links, 1)
		require.Equal(t, row.ID, row.Links[0].NotificationID)
		require.Equal(t, batch[i].OperationalRecord.ID, row.Links[0].OperationalRecordID)
		require.Equal(t, original.Links[0].DestinationID, row.Links[0].DestinationID)
	}
	var audits int
	require.NoError(t, q.db.QueryRow("SELECT count(*) FROM notification_audit").Scan(&audits))
	require.Equal(t, 1, audits, "local gating is not a provider attempt")
	m.Stop()

	// Reopen committed work. Advance only the fixture's pending timestamps:
	// deadline and attempts were asserted before reopening, with no 60 s sleep.
	m, _ = telegramSizeManager(t, dir, url)
	q = m.GetQueue()
	_, err = q.db.Exec("UPDATE notification_queue SET next_retry_at=? WHERE status='pending'", time.Now().Add(-time.Second).Unix())
	require.NoError(t, err)
	require.NoError(t, m.StartQueueProcessing())
	awaitTelegramSizeQueue(t, q, 2, 1, 0)
	requests := received()
	require.Len(t, requests, 3, "already-accepted first part was not resubmitted")
	dlq := mustTelegramDLQ(t, q)
	require.Len(t, dlq, 1)
	require.Len(t, dlq[0].Alerts, 1)
	require.Equal(t, "reject-this-part", dlq[0].Alerts[0].ID)
	require.Equal(t, 1, dlq[0].Attempts)
	for i, alert := range batch {
		_, found, err := q.deliveryReceipt(alert, notificationDeliveryDestinationKey(notificationDeliveryJob{Type: "webhook", Event: eventAlert, WebhookConfig: &hook}))
		require.NoError(t, err)
		require.Equal(t, i != 1, found, "only an accepted part may own a recovery")
	}
}

func TestTelegramSizedRecoveriesKeepTheirWindowAndReceipts(t *testing.T) {
	url, received := telegramSizeReceiver(t, "")
	m, hook := telegramSizeManager(t, t.TempDir(), url)
	m.SetNotifyOnResolve(true)
	m.SetGroupingConfig(true, 3, false, false)
	var batch []*alerts.Alert
	start := time.Now().Add(-time.Minute)
	for _, id := range []string{"alpha-recovered", "beta-recovered", "gamma-recovered"} {
		a := quietReplayAlert(id, alerts.AlertLevelWarning, start)
		a.ResourceName = id
		a.Node = strings.Repeat("🧪<&", 475) // two resolved entries cannot fit in one
		batch = append(batch, a)
		m.SendAlert(a)
	}
	m.sendGroupedAlerts()
	q := m.GetQueue()
	require.NoError(t, m.StartQueueProcessing())
	// The firing layout includes the leader node and the secondary nodes.
	awaitTelegramSizeQueue(t, q, 3, 0, 0)
	for _, a := range batch {
		require.False(t, m.CancelResolvedAlert(a))
		m.SendResolvedAlert(&alerts.ResolvedAlert{Alert: a, ResolvedTime: time.Now()})
	}
	stats, err := q.GetQueueStats()
	require.NoError(t, err)
	require.Equal(t, 1, stats["pending"], "the recovery window should still coalesce")
	require.Len(t, received(), 3, "no early recovery")
	early, err := q.GetPending(10)
	require.NoError(t, err)
	require.Empty(t, early)
	awaitTelegramSizeQueue(t, q, 6, 0, 0)
	requests := received()
	for _, a := range batch {
		occurrences := 0
		for _, request := range requests {
			require.Equal(t, http.StatusOK, request.status)
			if strings.Contains(request.content.text, "Resolved: "+a.ResourceName) {
				occurrences++
			}
		}
		require.Equal(t, 1, occurrences)
		_, found, err := q.deliveryReceipt(a, notificationDeliveryDestinationKey(notificationDeliveryJob{Type: "webhook", WebhookConfig: &hook}))
		require.NoError(t, err)
		require.False(t, found, "recovery must consume exactly its own receipt")
	}
}

func TestTelegramSizingPreservesExistingDestinationAndAttemptContracts(t *testing.T) {
	for _, tc := range []struct {
		name, service, url, template string
		attempted                    bool
	}{
		{name: "user body", service: "telegram", url: "https://example.test/sendMessage", template: `{"text":"{{.Message | jsonString}}"}`},
		{name: "templated recipient", service: "telegram", url: "https://example.test/sendMessage?chat_id={{.AlertCount}}"},
		{name: "generic", service: "generic", url: "https://example.test/sendMessage"},
		{name: "attempted", service: "telegram", url: "https://example.test/sendMessage", attempted: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			q, err := NewInMemoryNotificationQueue()
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, q.Stop()) })
			hook := WebhookConfig{ID: "configured", Service: tc.service, URL: tc.url, Enabled: true, Template: tc.template}
			raw, err := json.Marshal(hook)
			require.NoError(t, err)
			batch := telegramReportedAlerts()
			for _, a := range batch {
				a.Message += strings.Repeat("🧪<&", 800)
			}
			row := &QueuedNotification{Type: "webhook", Alerts: batch, Config: raw, MaxAttempts: 3}
			require.NoError(t, q.Enqueue(row))
			if tc.attempted {
				require.NoError(t, q.IncrementAttemptAndSetStatus(row.ID, QueueStatusPending))
			}
			pending, err := q.GetPending(10)
			require.NoError(t, err)
			require.Len(t, pending, 1)
			claimed, err := q.claimPendingForDelivery(pending[0])
			require.NoError(t, err)
			require.True(t, claimed)
			require.Len(t, pending[0].Alerts, len(batch))
			require.Equal(t, row.Config, pending[0].Config)
			other, err := q.GetPending(10)
			require.NoError(t, err)
			require.Empty(t, other, "nothing silently reroutes or resends a previously attempted group")
		})
	}
}

func TestTelegramBatchLinkCannotBelongToTwoParts(t *testing.T) {
	start := time.Now()
	one := quietReplayAlert("shared-link", alerts.AlertLevelWarning, start)
	two := quietReplayAlert("shared-link", alerts.AlertLevelWarning, start)
	two.StartTime = start.Add(time.Second)
	link := operationaltrust.NotificationLink{
		OperationalRecordID: one.OperationalRecord.ID, TransitionID: one.LatestTransition.ID,
	}
	_, err := telegramBatchLinks([]operationaltrust.NotificationLink{link}, [][]*alerts.Alert{{one}, {two}}, []string{"first", "second"})
	require.ErrorContains(t, err, "ambiguous operational link")
}

func TestTelegramQueuePartitionFailureLeavesOriginalUnattempted(t *testing.T) {
	q, err := NewInMemoryNotificationQueue()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, q.Stop()) })
	batch := telegramReportedAlerts()
	for _, a := range batch {
		a.Message += strings.Repeat("🧪<&", 800)
	}
	hook := WebhookConfig{ID: "telegram", Service: "telegram", Enabled: true, URL: "https://example.test/sendMessage?chat_id=-1001234"}
	raw, err := json.Marshal(hook)
	require.NoError(t, err)
	row := &QueuedNotification{Type: "webhook", Alerts: batch, Config: raw, MaxAttempts: 3}
	require.NoError(t, q.Enqueue(row))
	_, err = q.db.Exec(`CREATE TRIGGER abort_fixture BEFORE INSERT ON notification_queue
		WHEN NEW.id LIKE '%:telegram:2' BEGIN SELECT RAISE(ABORT,'fixture partition refused'); END;`)
	require.NoError(t, err)
	pending, err := q.GetPending(10)
	require.NoError(t, err)
	require.Len(t, pending, 1)
	claimed, err := q.claimPendingForDelivery(pending[0])
	require.ErrorContains(t, err, "fixture partition refused")
	require.False(t, claimed)
	pending, err = q.GetPending(10)
	require.NoError(t, err)
	require.Len(t, pending, 1)
	require.Equal(t, row.ID, pending[0].ID)
	require.Len(t, pending[0].Alerts, len(batch))
	require.Zero(t, pending[0].Attempts)
	require.Nil(t, pending[0].LastAttempt)
}
