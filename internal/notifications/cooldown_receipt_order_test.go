package notifications

import (
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/stretchr/testify/require"
)

func TestNotificationCooldownReceiptOrdering(t *testing.T) {
	start := time.Date(2026, 10, 3, 1, 0, 0, 123, time.UTC)
	for _, tc := range []struct {
		name       string
		priorStart time.Time
		lateStart  time.Time
		lateSent   time.Time
	}{
		{"older_occurrence", start, start.Add(-time.Nanosecond), start.Add(3 * time.Minute)},
		{"unknown_legacy_occurrence", start, time.Time{}, start.Add(3 * time.Minute)},
		{"earlier_same_occurrence_completion", start, start, start.Add(time.Minute)},
		{"earlier_legacy_completion", time.Time{}, time.Time{}, start.Add(time.Minute)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := NewNotificationManagerWithDeferredQueue("", t.TempDir())
			t.Cleanup(m.Stop)
			m.SetCooldown(0) // No ordinary repeats for the delivered occurrence.
			m.SetGroupingConfig(true, 3600, false, false)
			current := &alerts.Alert{ID: "cpu-order", Level: alerts.AlertLevelCritical, StartTime: tc.priorStart}
			m.markAlertsNotified([]*alerts.Alert{current}, start.Add(2*time.Minute))
			m.mu.RLock()
			before := m.lastNotified[current.ID]
			m.mu.RUnlock()
			late := current.Clone()
			late.StartTime = tc.lateStart
			late.Level = alerts.AlertLevelWarning
			m.markAlertsNotified([]*alerts.Alert{nil, late}, tc.lateSent)
			m.mu.RLock()
			after := m.lastNotified[current.ID]
			m.mu.RUnlock()
			require.Equal(t, before, after, "late receipt replaced the current cooldown")
			m.SendAlert(current)
			m.mu.RLock()
			pending := len(m.pendingAlerts)
			m.mu.RUnlock()
			require.Zero(t, pending, "delivered current occurrence was admitted again")
		})
	}
}

// Use the actual persistent sender and loopback receiver. Two firing callbacks
// for a reusable ID can complete in reverse occurrence order; no fabricated
// successful-delivery marker or modified scheduling clock is needed here.
func TestLateFiringHTTPReceiptPreservesCurrentCooldown(t *testing.T) {
	oldArrived := make(chan struct{}, 1)
	releaseOld := make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(releaseOld) }) }
	var mu sync.Mutex
	var bodies []string
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read firing: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		mu.Lock()
		bodies = append(bodies, string(body))
		mu.Unlock()
		if strings.Contains(string(body), "older firing") {
			select {
			case oldArrived <- struct{}{}:
			default:
			}
			select {
			case <-releaseOld:
			case <-r.Context().Done():
				return
			}
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	dir := t.TempDir()
	webhook := WebhookConfig{ID: "ops", URL: server.URL + "/hook", Enabled: true, Service: "generic"}
	m := NewNotificationManagerWithDeferredQueue("", dir)
	t.Cleanup(m.Stop)
	t.Cleanup(unblock) // Release an in-flight provider before waiting for shutdown.
	m.webhookClient = server.Client()
	require.NoError(t, m.UpdateAllowedPrivateCIDRs("127.0.0.1/32"))
	m.SetGroupingConfig(false, 0, false, false)
	m.SetCooldown(60)
	m.AddWebhook(webhook)
	queue := m.GetQueue()
	old := &alerts.Alert{ID: "recurring-cpu", ResourceName: "cpu-node", Type: "cpu",
		Level: alerts.AlertLevelCritical, Message: "older firing", StartTime: time.Now().UTC().Add(-time.Minute)}
	m.SendAlert(old)
	current := old.Clone()
	current.StartTime = time.Now().UTC()
	current.Level = alerts.AlertLevelWarning
	current.Message = "current warning firing"
	m.SendAlert(current)
	// Workers run concurrently within one discovered batch, but discovery waits
	// for that batch to finish. Admit both snapshots before normal activation so
	// the HTTP ordering is possible without a second dispatcher or clock edits.
	m.StartQueueProcessing()
	select {
	case <-oldArrived:
	case <-time.After(10 * time.Second):
		t.Fatal("old firing did not reach the HTTP receiver")
	}
	wait := func(condition func() bool) {
		t.Helper()
		deadline := time.Now().Add(10 * time.Second)
		for !condition() {
			if time.Now().After(deadline) {
				t.Fatal("notification completion timed out")
			}
			time.Sleep(time.Millisecond)
		}
	}
	wait(func() bool {
		m.mu.RLock()
		defer m.mu.RUnlock()
		return m.lastNotified[current.ID].alertStart.Equal(current.StartTime)
	})
	unblock()
	wait(func() bool {
		stats, err := queue.GetQueueStats()
		require.NoError(t, err)
		return stats["sent"] == 2
	})
	// Sending synchronously admits queue work unless the cooldown rejects it.
	// Assert admission itself, rather than hoping an excess send stays silent.
	m.SendAlert(current)
	stats, err := queue.GetQueueStats()
	require.NoError(t, err)
	require.Equal(t, map[string]int{"sent": 2}, stats, "late old completion admitted a duplicate current firing")
	// Old critical severity must not leak into the newer warning occurrence.
	current.Level = alerts.AlertLevelCritical
	current.Message = "current critical firing"
	m.SendAlert(current)
	wait(func() bool {
		stats, err := queue.GetQueueStats()
		require.NoError(t, err)
		return stats["sent"] == 3
	})
	mu.Lock()
	received := append([]string(nil), bodies...)
	mu.Unlock()
	require.Len(t, received, 3)
	for _, message := range []string{"older firing", "current warning firing", "current critical firing"} {
		count := 0
		for _, body := range received {
			if strings.Contains(body, message) {
				count++
			}
		}
		require.Equal(t, 1, count, "receiver must see exactly one %s", message)
	}
	m.Stop()
	// The old completion still owns a durable recovery receipt, independently
	// of which occurrence owns the transient repeat-delivery cooldown.
	reopened := NewNotificationManagerWithDeferredQueue("", dir)
	t.Cleanup(reopened.Stop)
	for _, alert := range []*alerts.Alert{old, current} {
		jobs := buildNotificationDeliveryJobs(EmailConfig{}, []WebhookConfig{webhook}, AppriseConfig{}, []*alerts.Alert{alert}, eventResolved, time.Now())
		require.Len(t, reopened.filterResolvedJobsByDeliveryReceipt(jobs), 1, "firing receipt lost across reopen")
	}
	t.Log("Two reverse-order HTTP 200 completions retain the current cooldown; repeat adds no row, severity increase delivers, both occurrence receipts survive queue reopen.")
}
