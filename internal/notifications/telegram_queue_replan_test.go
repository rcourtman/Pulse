package notifications

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
	"github.com/stretchr/testify/require"
)

func TestTelegramHeldRecoveryReplansWithoutReusingSibling(t *testing.T) {
	url, received := telegramSizeReceiver(t, "")
	m, hook := telegramSizeManager(t, t.TempDir(), url)
	q := m.GetQueue()
	start := time.Now().Add(-time.Minute)
	var batch []*alerts.Alert
	for i, size := range []int{1, 1900, 900, 900} {
		label := string(rune('A' + i))
		a := quietReplayAlert("recovery-"+label, alerts.AlertLevelWarning, start)
		a.ResourceName, a.Node = strings.Repeat(label, size), "node"
		batch = append(batch, a)
		m.SendAlert(a)
	}
	m.sendGroupedAlerts()
	require.NoError(t, m.StartQueueProcessing())
	const firings = 2
	awaitTelegramSizeQueue(t, q, firings, 0, 0)
	require.Len(t, received(), firings, "ordinary grouped firing establishes receipts for all four members")

	resolved := time.Now()
	for _, a := range batch {
		require.False(t, m.CancelResolvedAlert(a))
		m.SendResolvedAlert(&alerts.ResolvedAlert{Alert: a, ResolvedTime: resolved})
	}
	var recovery *QueuedNotification
	for _, row := range quietReplayRows(t, q) {
		if _, event := normalizeQueueType(row.Type); event == eventResolved {
			require.Nil(t, recovery, "admitted recoveries still share their fixed window")
			recovery = row
		}
	}
	require.NotNil(t, recovery)
	require.Equal(t, alertIdentifiersFromAlerts(batch), alertIdentifiersFromAlerts(recovery.Alerts))
	require.NotNil(t, recovery.NextRetryAt)
	require.True(t, recovery.NextRetryAt.After(time.Now()))

	// Fill the local window, then advance just this fixture's grouping deadline.
	for range WebhookRateLimitMax {
		m.checkWebhookRateLimit(url)
	}
	makeDue := func(id string) {
		t.Helper()
		_, err := q.db.Exec("UPDATE notification_queue SET next_retry_at=0 WHERE id=? AND status='pending'", id)
		require.NoError(t, err)
		q.notifyChan <- struct{}{}
	}
	makeDue(recovery.ID)
	require.Eventually(t, func() bool {
		var held int
		err := q.db.QueryRow("SELECT count(*) FROM notification_queue WHERE status='pending' AND last_error=?", telegramRateHoldMessage).Scan(&held)
		return err == nil && held == 2
	}, 10*time.Second, 5*time.Millisecond, "both initial parts should hold before any HTTP recovery")

	first := loadQuietReplay(t, q, recovery.ID)
	require.Equal(t, []string{"recovery-A", "recovery-B", "recovery-C"}, alertIdentifiersFromAlerts(first.Alerts))
	var sibling *QueuedNotification
	for _, row := range quietReplayRows(t, q) {
		if row.ID != first.ID && row.Status == QueueStatusPending {
			require.Nil(t, sibling)
			sibling = row
		}
	}
	require.NotNil(t, sibling)
	require.Equal(t, []string{"recovery-D"}, alertIdentifiersFromAlerts(sibling.Alerts))
	for _, row := range []*QueuedNotification{first, sibling} {
		require.Equal(t, "telegram-sized", row.Method, "a held part cannot absorb fresh recoveries")
		require.Zero(t, row.Attempts)
		require.Nil(t, row.LastAttempt)
		require.NotNil(t, row.NextRetryAt)
		require.True(t, row.NextRetryAt.After(time.Now().Add(30*time.Second)))
	}
	require.Len(t, received(), firings, "the local hold must not reach the receiver")
	log, err := q.GetDeliveryLog(time.Time{}, 20)
	require.NoError(t, err)
	require.Len(t, log, firings, "the hold must not record an attempt")

	// The lifecycle owner reports A active again; production cancellation
	// removes only its recovery. B now leads the retained group, so its longer
	// heading/details require B and C to be partitioned again.
	m.SetOccurrenceLifecycleProvider(func(id string, at time.Time) alerts.NotificationOccurrenceLifecycle {
		if id == batch[0].ID && at.Equal(start) {
			return alerts.NotificationOccurrenceLifecycle{State: alerts.NotificationOccurrenceActive}
		}
		return alerts.NotificationOccurrenceLifecycle{State: alerts.NotificationOccurrenceUnavailable}
	})
	require.NoError(t, q.reopenReceiptOccurrence(batch[0].ID, start))
	first = loadQuietReplay(t, q, first.ID)
	require.Equal(t, []string{"recovery-B", "recovery-C"}, alertIdentifiersFromAlerts(first.Alerts))
	require.Len(t, first.Links, 2)

	// Expire the synthetic limiter window without sleeping. Let D settle first
	// to keep a real, terminal sibling and its provider acknowledgement intact.
	m.webhookRateMu.Lock()
	m.webhookRateLimits[url].lastSent = time.Now().Add(-2 * WebhookRateLimitWindow)
	m.webhookRateMu.Unlock()
	makeDue(sibling.ID)
	awaitTelegramSizeQueue(t, q, firings+1, 0, 1)
	terminalSibling := loadQuietReplay(t, q, sibling.ID)
	makeDue(first.ID)
	awaitTelegramSizeQueue(t, q, firings+3, 0, 0)
	require.Equal(t, terminalSibling, loadQuietReplay(t, q, sibling.ID))

	requests := received()
	require.Len(t, requests, firings+3, "the original firings plus one recovery each for B, C, D")
	byLink := make(map[string]telegramSizeObservation)
	for _, request := range requests[firings:] {
		require.Equal(t, http.StatusOK, request.status)
		require.LessOrEqual(t, request.units, 4096)
		require.Len(t, request.content.links, 1)
		link := request.content.links[0]
		require.NotContains(t, byLink, link, "no duplicate recovery or sibling replay")
		byLink[link] = request
	}

	log, err = q.GetDeliveryLog(time.Time{}, 20)
	require.NoError(t, err)
	require.Len(t, log, firings+3)
	recoveries := make(map[string]DeliveryLogEntry)
	for _, entry := range log {
		if _, event := normalizeQueueType(entry.Type); event == eventResolved {
			require.Equal(t, DeliveryOutcomeSent, entry.Outcome)
			require.Equal(t, 1, entry.Attempts)
			require.Len(t, entry.AlertIDs, 1)
			require.NotContains(t, recoveries, entry.AlertIDs[0])
			recoveries[entry.AlertIDs[0]] = entry
		}
	}
	require.Len(t, recoveries, 3)
	destination := notificationDeliveryDestinationKey(notificationDeliveryJob{Type: "webhook", WebhookConfig: &hook})
	require.Equal(t, sibling.ID, recoveries[batch[3].ID].NotificationID)
	for i, a := range batch {
		_, found, err := q.deliveryReceipt(a, destination)
		require.NoError(t, err)
		require.Equal(t, i == 0, found, "only an accepted recovery consumes its firing receipt")
		link := fmt.Sprintf("https://pulse.example.test/alerts/history?alert=%s", a.ID)
		if i == 0 {
			require.NotContains(t, recoveries, a.ID)
			require.NotContains(t, byLink, link)
			continue
		}
		require.Contains(t, byLink[link].content.text, "Resolved: "+a.ResourceName)
		entry := recoveries[a.ID]
		row := loadQuietReplay(t, q, entry.NotificationID)
		require.Equal(t, "telegram-sized", row.Method)
		require.Equal(t, recovery.Config, row.Config)
		require.Equal(t, recovery.ReceiptEpoch, row.ReceiptEpoch)
		require.Equal(t, recovery.MaxAttempts, row.MaxAttempts)
		require.Len(t, row.Links, 1)
		require.Equal(t, row.ID, row.Links[0].NotificationID)
		require.Equal(t, a.OperationalRecord.ID, row.Links[0].OperationalRecordID)
		require.Equal(t, a.LatestTransition.ID, row.Links[0].TransitionID)
		require.Equal(t, recovery.Links[0].DestinationID, row.Links[0].DestinationID)
		require.Equal(t, operationaltrust.NotificationDelivered, row.Links[0].DeliveryState)
	}
	t.Log("initial parts [A,B,C]/[D] held without HTTP; D settled first; B/C/D each delivered once with own link and receipt; A did not recover")
}
