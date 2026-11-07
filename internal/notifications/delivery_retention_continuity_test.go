package notifications

// Real HTTP and persistent-queue regression for an aged firing receipt.
import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

type continuityRetentionReceipt struct {
	Event  string         `json:"event"`
	Alerts []alerts.Alert `json:"alerts"`
}

func TestNotificationReceiptRetentionAcrossRestart(t *testing.T) {
	for _, tc := range []struct {
		name           string
		receiptAgeDays int
	}{
		{name: "control receipt one day old", receiptAgeDays: 1},
		{name: "receipt older than thirty days while occurrence still active", receiptAgeDays: 31},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var mu sync.Mutex
			var got []continuityRetentionReceipt
			server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer r.Body.Close()
				var receipt continuityRetentionReceipt
				if err := json.NewDecoder(r.Body).Decode(&receipt); err != nil {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				mu.Lock()
				got = append(got, receipt)
				mu.Unlock()
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()
			count := func(event string) int {
				mu.Lock()
				defer mu.Unlock()
				n := 0
				for _, receipt := range got {
					if receipt.Event == event {
						n++
					}
				}
				return n
			}
			dataDir := t.TempDir()
			webhook := WebhookConfig{ID: "ops", Name: "ops", URL: server.URL, Enabled: true, Service: "generic"}
			open := func() *NotificationManager {
				m := NewNotificationManagerWithDataDir("", dataDir)
				m.webhookClient = server.Client()
				if err := m.UpdateAllowedPrivateCIDRs("127.0.0.1/32"); err != nil {
					t.Fatal(err)
				}
				m.SetGroupingConfig(false, 0, false, false)
				m.SetCooldown(30)
				m.SetNotifyOnResolve(true)
				m.AddWebhook(webhook)
				return m
			}
			drain := func(m *NotificationManager) {
				deadline := time.Now().Add(10 * time.Second)
				for {
					stats, err := m.GetQueueStats()
					if err != nil {
						t.Fatal(err)
					}
					if stats["pending"]+stats["sending"] == 0 {
						return
					}
					if time.Now().After(deadline) {
						t.Fatalf("queue did not drain: %v", stats)
					}
					time.Sleep(5 * time.Millisecond)
				}
			}
			alert := &alerts.Alert{
				ID: "storage-pbs-primary-backups-usage", Type: "usage", Level: alerts.AlertLevelCritical,
				ResourceID: "pbs-primary-backups", ResourceName: "backups", Node: "pbs-primary",
				Message: "Storage usage 95%", Value: 95, Threshold: 80,
				StartTime: time.Now().Add(-time.Duration(tc.receiptAgeDays) * 24 * time.Hour).Truncate(time.Second),
			}

			m := open()
			m.SendAlert(alert.Clone())
			deadline := time.Now().Add(10 * time.Second)
			for count("") < 1 {
				if time.Now().After(deadline) {
					t.Fatal("firing not received")
				}
				time.Sleep(5 * time.Millisecond)
			}
			drain(m)
			q := m.GetQueue()
			if ok, err := q.HasDeliveryReceipt(alert.ID, alert.StartTime, notificationDeliveryDestinationKey(notificationDeliveryJob{Type: "webhook", WebhookConfig: &webhook})); err != nil || !ok {
				t.Fatalf("firing receipt missing after delivery: ok=%v err=%v", ok, err)
			}

			// Elapsed-time simulation: only the receipt's delivered_at is aged.
			deliveredAt := time.Now().Add(-time.Duration(tc.receiptAgeDays) * 24 * time.Hour).Unix()
			q.mu.Lock()
			_, err := q.db.Exec(`UPDATE notification_delivery_receipts SET delivered_at = ? WHERE alert_id = ?`, deliveredAt, alert.ID)
			q.mu.Unlock()
			if err != nil {
				t.Fatal(err)
			}
			q.performCleanup() // the real hourly cleanup routine
			var remaining int
			q.mu.RLock()
			_ = q.db.QueryRow(`SELECT COUNT(*) FROM notification_delivery_receipts WHERE alert_id = ?`, alert.ID).Scan(&remaining)
			q.mu.RUnlock()
			if remaining != 1 {
				t.Fatalf("active occurrence lost receipt at age %dd: count=%d", tc.receiptAgeDays, remaining)
			}
			t.Logf("receipt age %dd: receipts remaining after cleanup = %d (occurrence still active, no repeat sent)", tc.receiptAgeDays, remaining)

			// The occurrence stays active across a graceful reopen, then recovers.
			m.Stop()
			m = open()
			t.Cleanup(m.Stop)
			m.SendResolvedAlert(&alerts.ResolvedAlert{Alert: alert.Clone(), ResolvedTime: time.Now()})
			drain(m)
			deadline = time.Now().Add(3 * time.Second)
			for count("resolved") < 1 && time.Now().Before(deadline) {
				time.Sleep(10 * time.Millisecond)
			}
			t.Logf("receipt age %dd: firing deliveries = %d, recovery deliveries = %d", tc.receiptAgeDays, count(""), count("resolved"))
			if count("resolved") != 1 {
				t.Errorf("destination that received the firing did not receive its recovery (receipt age %dd)", tc.receiptAgeDays)
			}
		})
	}
}

func TestNotificationReceiptRetentionAuthorityAndPendingRecovery(t *testing.T) {
	for _, tc := range []struct {
		name        string
		known       bool
		resolvedAge int
		pending     bool
		keep        bool
	}{
		{"active", true, 0, false, true},
		{"unknown", false, 100, false, true},
		{"recent resolution", true, 1, false, true},
		{"expired resolution", true, 31, false, false},
		{"pending recovery pins expired resolution", true, 31, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			n := NewNotificationManagerWithDeferredQueue("", t.TempDir())
			defer n.Stop()
			hook := WebhookConfig{ID: "ops", URL: "http://127.0.0.1:9", Enabled: true, Service: "generic"}
			a := &alerts.Alert{ID: "long-active", StartTime: time.Now().Add(-100 * 24 * time.Hour), Level: alerts.AlertLevelCritical}
			key := notificationDeliveryDestinationKey(notificationDeliveryJob{Type: "webhook", WebhookConfig: &hook})
			q := n.GetQueue()
			if err := recordFixtureDeliveryReceipt(q, a.ID, a.StartTime, key, time.Now().Add(-99*24*time.Hour), 3); err != nil {
				t.Fatal(err)
			}
			n.SetOccurrenceLifecycleProvider(func(id string, start time.Time) alerts.NotificationOccurrenceLifecycle {
				if id != a.ID || !start.Equal(a.StartTime) {
					t.Errorf("wrong lifecycle query")
				}
				if tc.resolvedAge == 0 {
					if tc.known {
						return alerts.NotificationOccurrenceLifecycle{State: alerts.NotificationOccurrenceActive}
					}
					return alerts.NotificationOccurrenceLifecycle{State: alerts.NotificationOccurrenceUnavailable}
				}
				if tc.known {
					return alerts.NotificationOccurrenceLifecycle{State: alerts.NotificationOccurrenceResolved, ResolvedAt: time.Now().Add(-time.Duration(tc.resolvedAge) * 24 * time.Hour)}
				}
				return alerts.NotificationOccurrenceLifecycle{State: alerts.NotificationOccurrenceUnavailable}
			})
			var pending *QueuedNotification
			if tc.pending {
				config, _ := json.Marshal(hook)
				pending = &QueuedNotification{Type: "webhook_resolved", Alerts: []*alerts.Alert{a}, Config: config}
				if err := q.Enqueue(pending); err != nil {
					t.Fatal(err)
				}
			}
			q.performCleanup()
			found, err := q.HasDeliveryReceipt(a.ID, a.StartTime, key)
			if err != nil || found != tc.keep {
				t.Fatalf("receipt=%v err=%v, want %v", found, err, tc.keep)
			}
			if pending != nil {
				if err := q.UpdateStatus(pending.ID, QueueStatusSent, ""); err != nil {
					t.Fatal(err)
				}
				q.performCleanup()
				if found, err := q.HasDeliveryReceipt(a.ID, a.StartTime, key); err != nil || found {
					t.Fatalf("settled recovery still pins receipt: %v %v", found, err)
				}
			}
		})
	}
}

func TestNotificationDestinationRetirementAndRevert(t *testing.T) {
	var mu sync.Mutex
	counts := map[string]int{}
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		counts[r.URL.Path]++
		mu.Unlock()
		w.WriteHeader(200)
	}))
	defer server.Close()
	dir := t.TempDir()
	original := WebhookConfig{ID: "ops", URL: server.URL + "/original", Enabled: true, Service: "generic"}
	rotated := original
	rotated.URL = server.URL + "/rotated"
	n := continuityManager(t, dir, original)
	a := &alerts.Alert{ID: "rotation", StartTime: time.Now(), Level: alerts.AlertLevelCritical}
	n.SendAlert(a)
	drainContinuity(t, n)
	if err := n.UpdateWebhook(original.ID, rotated); err != nil {
		t.Fatal(err)
	}
	// Reverting configuration must not revive a retired successful firing.
	if err := n.UpdateWebhook(original.ID, original); err != nil {
		t.Fatal(err)
	}
	n.SendResolvedAlert(&alerts.ResolvedAlert{Alert: a.Clone(), ResolvedTime: time.Now()})
	drainContinuity(t, n)
	mu.Lock()
	count := counts["/original"]
	mu.Unlock()
	if count != 1 {
		t.Fatalf("revert resurrected recovery: %d", count)
	}
	n.SendAlert(a)
	drainContinuity(t, n)
	n.Stop()
	n = continuityManager(t, dir, rotated)
	// Complete startup owns retirement of a destination changed while stopped.
	n.GetQueue().performCleanup()
	if err := n.UpdateWebhook(original.ID, original); err != nil {
		t.Fatal(err)
	}
	n.SendResolvedAlert(&alerts.ResolvedAlert{Alert: a.Clone(), ResolvedTime: time.Now()})
	drainContinuity(t, n)
	n.SendAlert(a)
	drainContinuity(t, n)
	n.SendResolvedAlert(&alerts.ResolvedAlert{Alert: a.Clone(), ResolvedTime: time.Now()})
	drainContinuity(t, n)
	mu.Lock()
	defer mu.Unlock()
	if counts["/original"] != 4 || counts["/rotated"] != 0 {
		t.Fatalf("delivery counts: %v", counts)
	}
}

func TestNotificationRetirementPinsAdmittedRecovery(t *testing.T) {
	n := NewNotificationManagerWithDeferredQueue("", t.TempDir())
	defer n.Stop()
	hook := WebhookConfig{ID: "ops", URL: "http://127.0.0.1:9/original", Enabled: true, Service: "generic"}
	n.AddWebhook(hook)
	q := n.GetQueue()
	a := &alerts.Alert{ID: "owed", StartTime: time.Now(), Level: alerts.AlertLevelCritical}
	key := notificationDeliveryDestinationKey(notificationDeliveryJob{Type: "webhook", WebhookConfig: &hook})
	if err := recordFixtureDeliveryReceipt(q, a.ID, a.StartTime, key, time.Now(), 3); err != nil {
		t.Fatal(err)
	}
	config, _ := json.Marshal(hook)
	recovery := &QueuedNotification{Type: "webhook_resolved", Alerts: []*alerts.Alert{a}, Config: config}
	if err := q.Enqueue(recovery); err != nil {
		t.Fatal(err)
	}
	changed := hook
	changed.URL = "http://127.0.0.1:9/new"
	if err := n.UpdateWebhook(hook.ID, changed); err != nil {
		t.Fatal(err)
	}
	q.performCleanup()
	var count int
	if err := q.db.QueryRow(`SELECT COUNT(*) FROM notification_delivery_receipts`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatal("retired pending recovery lost its authority")
	}
	if found, err := q.HasDeliveryReceipt(a.ID, a.StartTime, key); err != nil || found {
		t.Fatalf("retired receipt admitted new recovery: %v %v", found, err)
	}
	if err := q.UpdateStatus(recovery.ID, QueueStatusCancelled, ""); err != nil {
		t.Fatal(err)
	}
	q.performCleanup()
	if err := q.db.QueryRow(`SELECT COUNT(*) FROM notification_delivery_receipts`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("settled retired receipt was not compacted")
	}
}

func TestNotificationCleanupRevalidatesConcurrentFiringAdmission(t *testing.T) {
	n := NewNotificationManagerWithDeferredQueue("", t.TempDir())
	defer n.Stop()
	hook := WebhookConfig{ID: "ops", URL: "http://127.0.0.1:9", Enabled: true, Service: "generic"}
	n.AddWebhook(hook)
	q := n.GetQueue()
	a := &alerts.Alert{ID: "race", StartTime: time.Now().Add(-100 * 24 * time.Hour), Level: alerts.AlertLevelCritical}
	key := notificationDeliveryDestinationKey(notificationDeliveryJob{Type: "webhook", WebhookConfig: &hook})
	if err := recordFixtureDeliveryReceipt(q, a.ID, a.StartTime, key, time.Now(), 2); err != nil {
		t.Fatal(err)
	}
	if err := q.markReceiptOccurrenceResolved(a.ID, a.StartTime, time.Now().Add(-31*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	// A stale resolved snapshot races an admitted firing after cleanup's receipt
	// snapshot. The queue's disposition revision must prevent stale deletion.
	var advanced atomic.Bool
	n.SetOccurrenceLifecycleProvider(func(string, time.Time) alerts.NotificationOccurrenceLifecycle {
		if !advanced.CompareAndSwap(false, true) {
			return alerts.NotificationOccurrenceLifecycle{State: alerts.NotificationOccurrenceActive}
		}
		n.SendAlert(a)
		return alerts.NotificationOccurrenceLifecycle{State: alerts.NotificationOccurrenceResolved, ResolvedAt: time.Now().Add(-31 * 24 * time.Hour)}
	})
	q.performCleanup()
	if found, err := q.HasDeliveryReceipt(a.ID, a.StartTime, key); err != nil || !found {
		t.Fatalf("concurrent firing lost its receipt: %v %v", found, err)
	}
	var resolved, revision int64
	if err := q.db.QueryRow(`SELECT resolved_at,lifecycle_revision FROM notification_delivery_receipts`).Scan(&resolved, &revision); err != nil {
		t.Fatal(err)
	}
	if resolved == 0 || revision < 2 {
		t.Fatal("firing admission must retain positive disposition and invalidate stale cleanup")
	}
}

func TestNotificationReceiptDispositionSurvivesHistoryCompaction(t *testing.T) {
	n := NewNotificationManagerWithDeferredQueue("", t.TempDir())
	defer n.Stop()
	q := n.GetQueue()
	start := time.Now().Add(-100 * 24 * time.Hour)
	if err := recordFixtureDeliveryReceipt(q, "ended", start, "destination", time.Now().Add(-90*24*time.Hour), 3); err != nil {
		t.Fatal(err)
	}
	if err := q.markReceiptOccurrenceResolved("ended", start, time.Now().Add(-31*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	// Healthy compacted history can use an explicit retained disposition.
	n.SetOccurrenceLifecycleProvider(func(string, time.Time) alerts.NotificationOccurrenceLifecycle {
		return alerts.NotificationOccurrenceLifecycle{State: alerts.NotificationOccurrenceNoRecord}
	})
	q.performCleanup()
	if found, err := q.HasDeliveryReceipt("ended", start, "destination"); err != nil || found {
		t.Fatalf("settled obligation was never compacted: %v %v", found, err)
	}
}

func TestNotificationRetirementOrdersInflightHTTPCompletion(t *testing.T) {
	for _, operation := range []string{"rotate", "delete", "disable", "revert-before-completion", "legacy-rotate", "legacy-revert-before-completion"} {
		t.Run(operation, func(t *testing.T) {
			legacy := strings.HasPrefix(operation, "legacy-")
			operation := strings.TrimPrefix(operation, "legacy-")
			arrived := make(chan struct{}, 1)
			release := make(chan struct{})
			var once sync.Once
			unblock := func() { once.Do(func() { close(release) }) }
			var mu sync.Mutex
			count := 0
			server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				count++
				first := count == 1
				mu.Unlock()
				if first {
					arrived <- struct{}{}
					select {
					case <-release:
					case <-r.Context().Done():
						return
					}
				}
				w.WriteHeader(200)
			}))
			t.Cleanup(server.Close)
			hook := WebhookConfig{ID: "ops", URL: server.URL + "/original", Enabled: true, Service: "generic"}
			n := continuityManager(t, t.TempDir(), hook)
			if legacy {
				key := notificationDeliveryDestinationKey(notificationDeliveryJob{Type: "webhook", WebhookConfig: &hook})
				if _, err := n.GetQueue().db.Exec(`UPDATE notification_destination_state SET epoch='' WHERE destination_key=?`, key); err != nil {
					t.Fatal(err)
				}
			}
			t.Cleanup(unblock)
			a := &alerts.Alert{ID: "inflight", StartTime: time.Now(), Level: alerts.AlertLevelCritical}
			n.SendAlert(a)
			select {
			case <-arrived:
			case <-time.After(5 * time.Second):
				t.Fatal("firing did not arrive")
			}
			switch operation {
			case "rotate", "revert-before-completion":
				changed := hook
				changed.URL = server.URL + "/new"
				if err := n.UpdateWebhook(hook.ID, changed); err != nil {
					t.Fatal(err)
				}
				if operation == "revert-before-completion" {
					if err := n.UpdateWebhook(hook.ID, hook); err != nil {
						t.Fatal(err)
					}
				}
			case "delete":
				if err := n.DeleteWebhook(hook.ID); err != nil {
					t.Fatal(err)
				}
			case "disable":
				changed := hook
				changed.Enabled = false
				if err := n.UpdateWebhook(hook.ID, changed); err != nil {
					t.Fatal(err)
				}
			}
			unblock()
			drainContinuity(t, n)
			if operation == "delete" {
				n.AddWebhook(hook)
			} else if err := n.UpdateWebhook(hook.ID, hook); err != nil {
				t.Fatal(err)
			}
			n.SendResolvedAlert(&alerts.ResolvedAlert{Alert: a.Clone(), ResolvedTime: time.Now()})
			drainContinuity(t, n)
			mu.Lock()
			observed := count
			mu.Unlock()
			if observed != 1 {
				t.Fatalf("late completion revived retired receipt: %d deliveries", observed)
			}
			n.SendAlert(a)
			drainContinuity(t, n)
			mu.Lock()
			defer mu.Unlock()
			if count != 2 {
				t.Fatalf("reverted destination did not get fresh firing: %d deliveries", count)
			}
		})
	}
}

func TestNotificationReceiptDispositionRequiresAvailableAuthority(t *testing.T) {
	for _, state := range []alerts.NotificationOccurrenceLifecycle{
		{State: alerts.NotificationOccurrenceUnavailable},
		{State: alerts.NotificationOccurrenceActive},
		{State: alerts.NotificationOccurrenceNoRecord, LatestActivationAt: time.Now()},
	} {
		t.Run(string(state.State)+state.LatestActivationAt.String(), func(t *testing.T) {
			n := NewNotificationManagerWithDeferredQueue("", t.TempDir())
			defer n.Stop()
			q := n.GetQueue()
			start := time.Now().Add(-100 * 24 * time.Hour)
			if err := recordFixtureDeliveryReceipt(q, "uncertain", start, "webhook:ops", time.Now().Add(-90*24*time.Hour), 3); err != nil {
				t.Fatal(err)
			}
			if err := q.markReceiptOccurrenceResolved("uncertain", start, time.Now().Add(-31*24*time.Hour)); err != nil {
				t.Fatal(err)
			}
			n.SetOccurrenceLifecycleProvider(func(string, time.Time) alerts.NotificationOccurrenceLifecycle { return state })
			q.performCleanup()
			if found, err := q.HasDeliveryReceipt("uncertain", start, "webhook:ops"); err != nil || !found {
				t.Fatalf("uncertain/resumed lifecycle lost its obligation: %v %v", found, err)
			}
		})
	}
}

func TestNotificationExplicitResolutionSettlesWithCompleteCurrentAbsenceDespiteHistoryGap(t *testing.T) {
	for _, currentInactive := range []bool{false, true} {
		t.Run(fmt.Sprintf("current_absence_%t", currentInactive), func(t *testing.T) {
			n := deferredContinuityManager(t, t.TempDir())
			q := n.GetQueue()
			start := time.Now().Add(-100 * 24 * time.Hour)
			for _, id := range []string{"resolved", "unknown"} {
				if err := recordFixtureDeliveryReceipt(q, id, start, "webhook:ops", time.Now().Add(-90*24*time.Hour), 3); err != nil {
					t.Fatal(err)
				}
			}
			if err := q.markReceiptOccurrenceResolved("resolved", start, time.Now().Add(-31*24*time.Hour)); err != nil {
				t.Fatal(err)
			}
			n.SetOccurrenceLifecycleProvider(func(string, time.Time) alerts.NotificationOccurrenceLifecycle {
				return alerts.NotificationOccurrenceLifecycle{State: alerts.NotificationOccurrenceUnavailable, CurrentInactive: currentInactive}
			})
			q.performCleanup()
			for _, id := range []string{"resolved", "unknown"} {
				found, err := q.HasDeliveryReceipt(id, start, "webhook:ops")
				if err != nil || found != (id == "unknown" || !currentInactive) {
					t.Fatalf("id=%s currentInactive=%t found=%t err=%v", id, currentInactive, found, err)
				}
			}
		})
	}
}

func TestNotificationMalformedRecoveryPinsOnlyAffectedProvider(t *testing.T) {
	n := NewNotificationManagerWithDeferredQueue("", t.TempDir())
	defer n.Stop()
	q := n.GetQueue()
	start := time.Now().Add(-100 * 24 * time.Hour)
	hook := WebhookConfig{ID: "ops", URL: "http://127.0.0.1:9", Enabled: true}
	key := notificationDeliveryDestinationKey(notificationDeliveryJob{Type: "webhook", WebhookConfig: &hook})
	for _, destination := range []string{key, "email:ops"} {
		if err := recordFixtureDeliveryReceipt(q, "old", start, destination, time.Now().Add(-90*24*time.Hour), 3); err != nil {
			t.Fatal(err)
		}
	}
	if err := q.markReceiptOccurrenceResolved("old", start, time.Now().Add(-31*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	n.SetOccurrenceLifecycleProvider(func(string, time.Time) alerts.NotificationOccurrenceLifecycle {
		return alerts.NotificationOccurrenceLifecycle{State: alerts.NotificationOccurrenceNoRecord}
	})
	config, _ := json.Marshal(hook)
	row := &QueuedNotification{Type: "webhook_resolved", Config: config, Alerts: []*alerts.Alert{{ID: "old", StartTime: start}}}
	if err := q.Enqueue(row); err != nil {
		t.Fatal(err)
	}
	if _, err := q.db.Exec(`UPDATE notification_queue SET config='{' WHERE id=?`, row.ID); err != nil {
		t.Fatal(err)
	}
	q.performCleanup()
	if found, err := q.HasDeliveryReceipt("old", start, key); err != nil || !found {
		t.Fatalf("malformed pending recovery lost provider authority: %v %v", found, err)
	}
	if found, err := q.HasDeliveryReceipt("old", start, "email:ops"); err != nil || found {
		t.Fatalf("unrelated provider compaction was poisoned: %v %v", found, err)
	}
}

func TestNotificationSameURLRecipientRotationRetiresGeneration(t *testing.T) {
	var mu sync.Mutex
	var recipients []string
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		recipients = append(recipients, r.Header.Get("X-Recipient"))
		mu.Unlock()
		w.WriteHeader(200)
	}))
	t.Cleanup(server.Close)
	hook := WebhookConfig{ID: "ops", URL: server.URL, Enabled: true, Service: "generic", Headers: map[string]string{"X-Recipient": "A"}}
	n := continuityManager(t, t.TempDir(), hook)
	a := &alerts.Alert{ID: "recipient", StartTime: time.Now(), Level: alerts.AlertLevelCritical}
	n.SendAlert(a)
	drainContinuity(t, n)
	changed := copyWebhookConfig(hook)
	changed.Headers["X-Recipient"] = "B"
	if err := n.UpdateWebhook(hook.ID, changed); err != nil {
		t.Fatal(err)
	}
	n.SendResolvedAlert(&alerts.ResolvedAlert{Alert: a.Clone(), ResolvedTime: time.Now()})
	drainContinuity(t, n)
	n.SendAlert(a)
	drainContinuity(t, n)
	n.SendResolvedAlert(&alerts.ResolvedAlert{Alert: a.Clone(), ResolvedTime: time.Now()})
	drainContinuity(t, n)
	mu.Lock()
	defer mu.Unlock()
	if len(recipients) != 3 || recipients[0] != "A" || recipients[1] != "B" || recipients[2] != "B" {
		t.Fatalf("receiver identity was not scoped to authenticated configuration: %v", recipients)
	}
}

func TestNotificationOldRecoveryCannotConsumeRevertedGeneration(t *testing.T) {
	n := NewNotificationManagerWithDeferredQueue("", t.TempDir())
	defer n.Stop()
	hook := WebhookConfig{ID: "ops", URL: "http://127.0.0.1:9", Enabled: true, Service: "generic"}
	n.AddWebhook(hook)
	a := &alerts.Alert{ID: "generation", StartTime: time.Now(), Level: alerts.AlertLevelCritical}
	first := n.bindDeliveryEpochs([]notificationDeliveryJob{{Type: "webhook", Event: eventAlert, WebhookConfig: &hook, Alerts: []*alerts.Alert{a}}})[0]
	if err := recordFixtureSuccessfulDelivery(n, first, time.Now()); err != nil {
		t.Fatal(err)
	}
	recovery := n.filterResolvedJobsByDeliveryReceipt([]notificationDeliveryJob{{Type: "webhook", Event: eventResolved, WebhookConfig: &hook, Alerts: []*alerts.Alert{a}}})
	if len(recovery) != 1 {
		t.Fatal("missing admitted old recovery")
	}
	changed := hook
	changed.URL += "/new"
	if err := n.UpdateWebhook(hook.ID, changed); err != nil {
		t.Fatal(err)
	}
	if err := n.UpdateWebhook(hook.ID, hook); err != nil {
		t.Fatal(err)
	}
	current := n.bindDeliveryEpochs([]notificationDeliveryJob{{Type: "webhook", Event: eventAlert, WebhookConfig: &hook, Alerts: []*alerts.Alert{a}}})[0]
	if first.ReceiptEpoch == current.ReceiptEpoch {
		t.Fatal("exact revert reused a retired generation")
	}
	if err := recordFixtureSuccessfulDelivery(n, current, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := recordFixtureSuccessfulDelivery(n, recovery[0], time.Now()); err != nil {
		t.Fatal(err)
	}
	key := notificationDeliveryDestinationKey(current)
	_, epoch, found, err := n.GetQueue().deliveryReceiptWithEpoch(a, key)
	if err != nil || !found || epoch != current.ReceiptEpoch {
		t.Fatalf("old recovery consumed current generation: %v %s %v", found, epoch, err)
	}
}

func TestNotificationDestinationRetirementDriverFailureRestoresLiveConfig(t *testing.T) {
	for _, kind := range []string{"email", "apprise", "webhook", "delete"} {
		t.Run(kind, func(t *testing.T) {
			n := NewNotificationManagerWithDeferredQueue("", t.TempDir())
			defer n.Stop()
			email := EmailConfig{Enabled: true, SMTPHost: "smtp.local", SMTPPort: 587, From: "pulse@example.test", To: []string{"old@example.test"}}
			apprise := NormalizeAppriseConfig(AppriseConfig{Enabled: true, Mode: AppriseModeHTTP, ServerURL: "http://127.0.0.1:9", Targets: []string{"old"}})
			hook := WebhookConfig{ID: "ops", URL: "http://127.0.0.1:9/old", Enabled: true}
			job := notificationDeliveryJob{Type: "webhook", WebhookConfig: &hook, Event: eventAlert}
			switch kind {
			case "email":
				if err := n.SetEmailConfig(email); err != nil {
					t.Fatal(err)
				}
				job = notificationDeliveryJob{Type: "email", EmailConfig: &email, Event: eventAlert}
			case "apprise":
				if err := n.SetAppriseConfig(apprise); err != nil {
					t.Fatal(err)
				}
				job = notificationDeliveryJob{Type: "apprise", AppriseConfig: &apprise, Event: eventAlert}
			default:
				n.AddWebhook(hook)
			}
			a := &alerts.Alert{ID: "retire-fault", StartTime: time.Now(), Level: alerts.AlertLevelCritical}
			job.Alerts = []*alerts.Alert{a}
			job = n.bindDeliveryEpochs([]notificationDeliveryJob{job})[0]
			if err := recordFixtureSuccessfulDelivery(n, job, time.Now()); err != nil {
				t.Fatal(err)
			}
			q := n.GetQueue()
			if _, err := q.db.Exec(`CREATE TRIGGER reject_retirement BEFORE UPDATE ON notification_delivery_receipts BEGIN SELECT RAISE(ABORT,'synthetic retirement failure'); END`); err != nil {
				t.Fatal(err)
			}
			apply := func() error {
				switch kind {
				case "email":
					next := email
					next.To = []string{"new@example.test"}
					return n.SetEmailConfig(next)
				case "apprise":
					next := apprise
					next.Targets = []string{"new"}
					return n.SetAppriseConfig(next)
				case "webhook":
					next := hook
					next.URL = "http://127.0.0.1:9/new"
					return n.UpdateWebhook(hook.ID, next)
				default:
					return n.DeleteWebhook(hook.ID)
				}
			}
			if err := apply(); err == nil {
				t.Fatal("retirement driver failure was acknowledged as success")
			}
			switch kind {
			case "email":
				if n.GetEmailConfig().To[0] != email.To[0] {
					t.Fatal("failed email change remained live")
				}
			case "apprise":
				if n.GetAppriseConfig().Targets[0] != apprise.Targets[0] {
					t.Fatal("failed Apprise change remained live")
				}
			default:
				hooks := n.GetWebhooks()
				if len(hooks) != 1 || hooks[0].URL != hook.URL {
					t.Fatal("failed webhook change remained live")
				}
			}
			key := notificationDeliveryDestinationKey(job)
			if found, err := q.HasDeliveryReceipt(a.ID, a.StartTime, key); err != nil || !found {
				t.Fatalf("failed retirement lost prior authority: %v %v", found, err)
			}
			if _, err := q.db.Exec(`DROP TRIGGER reject_retirement`); err != nil {
				t.Fatal(err)
			}
			if err := apply(); err != nil {
				t.Fatal(err)
			}
			if found, err := q.HasDeliveryReceipt(a.ID, a.StartTime, key); err != nil || found {
				t.Fatalf("successful retirement left old authority active: %v %v", found, err)
			}
		})
	}
}

func TestNotificationReceiptCommitFailureRetainsRetryObligation(t *testing.T) {
	var mu sync.Mutex
	count := 0
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { mu.Lock(); count++; mu.Unlock(); w.WriteHeader(200) }))
	t.Cleanup(server.Close)
	hook := WebhookConfig{ID: "ops", URL: server.URL, Enabled: true, Service: "generic"}
	dir := t.TempDir()
	n := continuityManager(t, dir, hook)
	q := n.GetQueue()
	var calls atomic.Int64
	q.mu.Lock()
	original := q.processor
	q.processor = func(row *QueuedNotification) error { calls.Add(1); return original(row) }
	q.mu.Unlock()

	if _, err := q.db.Exec(`CREATE TRIGGER reject_receipt BEFORE INSERT ON notification_delivery_receipts BEGIN SELECT RAISE(ABORT,'synthetic receipt commit failure'); END`); err != nil {
		t.Fatal(err)
	}
	a := &alerts.Alert{ID: "receipt-commit", StartTime: time.Now(), Level: alerts.AlertLevelCritical}
	n.SendAlert(a)
	deadline := time.Now().Add(5 * time.Second)
	for {
		stats, err := q.GetQueueStats()
		if err != nil {
			t.Fatal(err)
		}
		mu.Lock()
		observed := count
		mu.Unlock()
		observedCalls := calls.Load()
		q.mu.RLock()
		owned := len(q.processingNotifications)
		q.mu.RUnlock()
		// The HTTP observation can be newer than the stats snapshot. Require
		// the actual settlement marker and the worker's return before advancing
		// its retry clock, or an in-flight first attempt can overwrite that clock.
		if observed == 1 && observedCalls >= 1 && stats["pending"] == 1 && stats["settlement_pending"] == 1 && stats["sending"] == 0 && stats["sent"] == 0 && owned == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("accepted but unsettled delivery did not retain retry: stats=%v http=%d calls=%d owned=%d", stats, observed, observedCalls, owned)
		}
		time.Sleep(time.Millisecond)
	}
	key := notificationDeliveryDestinationKey(notificationDeliveryJob{Type: "webhook", WebhookConfig: &hook})
	if found, err := q.HasDeliveryReceipt(a.ID, a.StartTime, key); err != nil || found {
		t.Fatalf("failed receipt commit invented evidence: %v %v", found, err)
	}
	n.mu.RLock()
	_, marked := n.lastNotified[a.ID]
	n.mu.RUnlock()
	if marked {
		t.Fatal("failed receipt commit populated cooldown")
	}
	for retry := 0; retry < 4; retry++ {
		beforeRetryCalls := calls.Load()
		retryContinuitySettlementNow(t, q)
		q.processBatch()
		deadline := time.Now().Add(5 * time.Second)
		for {
			mu.Lock()
			observed := count
			mu.Unlock()
			stats, err := q.GetQueueStats()
			if err != nil {
				t.Fatal(err)
			}
			observedCalls := calls.Load()
			q.mu.RLock()
			owned := len(q.processingNotifications)
			q.mu.RUnlock()
			// A pending row is written before its worker releases ownership.
			// Do not start the next simulated retry until this one has returned.
			if observedCalls > beforeRetryCalls && observed == 1 && stats["pending"] == 1 && stats["settlement_pending"] == 1 && stats["sending"] == 0 && owned == 0 {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("storage fault did not preserve repeated pending settlement: retry=%d calls=%d before=%d owned=%d http=%d stats=%v", retry, observedCalls, beforeRetryCalls, owned, observed, stats)
			}
			time.Sleep(time.Millisecond)
		}
	}
	var attempts, audits int
	if err := q.db.QueryRow(`SELECT attempts FROM notification_queue`).Scan(&attempts); err != nil || attempts != 1 {
		t.Fatalf("local settlement consumed provider budget: attempts=%d error=%v", attempts, err)
	}
	if err := q.db.QueryRow(`SELECT COUNT(*) FROM notification_audit`).Scan(&audits); err != nil || audits != 1 {
		t.Fatalf("local settlement did not retain exactly one provider acknowledgement: audits=%d error=%v", audits, err)
	}
	if health := n.DeliveryHealth(); health.Healthy || health.AttentionRequired != 1 {
		t.Fatalf("pending storage settlement reported healthy: %+v", health)
	}
	// Successful provider evidence remains authoritative while the receipt
	// projection is unavailable, and survives process-owner replacement.
	n.SendAlert(a)
	stats, err := q.GetQueueStats()
	if err != nil || stats["pending"]+stats["sending"] != 1 || stats["sent"] != 0 {
		t.Fatalf("acknowledged cooldown admitted another job: %v %v", stats, err)
	}
	if _, err := q.db.Exec(`UPDATE notification_audit SET timestamp=? WHERE success=1`, time.Now().Add(-8*24*time.Hour).Unix()); err != nil {
		t.Fatal(err)
	}
	q.performCleanup()
	if err := q.db.QueryRow(`SELECT COUNT(*) FROM notification_audit WHERE success=1`).Scan(&audits); err != nil || audits != 1 {
		t.Fatalf("cleanup pruned unsettled acknowledgement: %d %v", audits, err)
	}
	if _, err := q.db.Exec(`UPDATE notification_queue SET alerts='invalid',config='invalid',receipt_epoch='invalid' WHERE status='pending'`); err != nil {
		t.Fatal(err)
	}
	n.Stop()
	n = continuityManager(t, dir, hook)
	q = n.GetQueue()
	q.mu.Lock()
	_, err = q.db.Exec(`DROP TRIGGER reject_receipt`)
	q.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	retryContinuitySettlementNow(t, q)
	drainContinuity(t, n)
	if found, err := q.HasDeliveryReceipt(a.ID, a.StartTime, key); err != nil || !found {
		t.Fatalf("retry did not settle receipt obligation: %v %v", found, err)
	}
	mu.Lock()
	defer mu.Unlock()
	if count != 1 {
		t.Fatal("receipt settlement replay contacted an already acknowledged provider")
	}
}

func TestNotificationDestinationGenerationFailureDefersProvider(t *testing.T) {
	var mu sync.Mutex
	count := 0
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		count++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	hook := WebhookConfig{ID: "generation", URL: server.URL, Enabled: true, Service: "generic"}
	n := continuityManager(t, t.TempDir(), hook)
	q := n.GetQueue()
	// Exercise a required new registry write, rather than an unchanged binding
	// that can legitimately complete using its existing durable generation.
	if _, err := q.db.Exec(`DELETE FROM notification_destination_state`); err != nil {
		t.Fatal(err)
	}
	if _, err := q.db.Exec(`CREATE TRIGGER reject_generation BEFORE INSERT ON notification_destination_state BEGIN SELECT RAISE(ABORT,'synthetic generation fault'); END`); err != nil {
		t.Fatal(err)
	}
	a := &alerts.Alert{ID: "generation-fault", StartTime: time.Now(), Level: alerts.AlertLevelCritical}
	n.SendAlert(a)
	deadline := time.Now().Add(5 * time.Second)
	for {
		var lastError string
		var epoch *string
		err := q.db.QueryRow(`SELECT last_error,receipt_epoch FROM notification_queue WHERE status='pending'`).Scan(&lastError, &epoch)
		if err == nil && lastError == notificationSettlementPendingMessage {
			if epoch != nil {
				t.Fatal("registry failure persisted fabricated generation")
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("generation failure did not retain retry: %v", err)
		}
		time.Sleep(time.Millisecond)
	}
	mu.Lock()
	observed := count
	mu.Unlock()
	if observed != 0 {
		t.Fatal("provider contacted before durable generation binding")
	}
	if _, err := q.db.Exec(`DROP TRIGGER reject_generation`); err != nil {
		t.Fatal(err)
	}
	retryContinuitySettlementNow(t, q)
	drainContinuity(t, n)
	mu.Lock()
	defer mu.Unlock()
	if count != 1 {
		t.Fatalf("recovered registry did not deliver exactly once: %d", count)
	}
	key := notificationDeliveryDestinationKey(notificationDeliveryJob{Type: "webhook", WebhookConfig: &hook})
	if found, err := q.HasDeliveryReceipt(a.ID, a.StartTime, key); err != nil || !found {
		t.Fatalf("recovered generation did not retain delivery authority: %v %v", found, err)
	}
}

func retryContinuitySettlementNow(t *testing.T, q *NotificationQueue) {
	t.Helper()
	q.mu.Lock()
	_, err := q.db.Exec(`UPDATE notification_queue SET next_retry_at=? WHERE status='pending' AND last_error=?`, time.Now().Add(-time.Second).Unix(), notificationSettlementPendingMessage)
	q.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	select {
	case q.notifyChan <- struct{}{}:
	default:
	}
}

func TestNotificationContinuityCheckpointSkipsDeliveredDestination(t *testing.T) {
	var mu sync.Mutex
	counts := map[string]int{}
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		counts[r.URL.Path]++
		mu.Unlock()
		w.WriteHeader(200)
	}))
	t.Cleanup(server.Close)
	aHook := WebhookConfig{ID: "a", URL: server.URL + "/a", Enabled: true, Service: "generic"}
	bHook := WebhookConfig{ID: "b", URL: server.URL + "/b", Enabled: true, Service: "generic"}
	dir := t.TempDir()
	n := continuityManager(t, dir, aHook)
	a := &alerts.Alert{ID: "checkpoint-cooldown", StartTime: time.Now(), Level: alerts.AlertLevelCritical}
	n.SendAlert(a)
	drainContinuity(t, n)
	n.AddWebhook(bHook)
	n.SetGroupingConfig(true, 3600, false, false)
	n.SendAlert(a)
	n.Stop()
	n = continuityManager(t, dir, aHook, bHook)
	q := n.GetQueue()
	rows, err := q.GetPending(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatal("checkpoint delivered before its deadline")
	}
	var pending int
	if err := q.db.QueryRow(`SELECT COUNT(*) FROM notification_queue WHERE status='pending'`).Scan(&pending); err != nil || pending != 1 {
		t.Fatalf("checkpoint queued cooled destination: pending=%d error=%v", pending, err)
	}
	if _, err := q.db.Exec(`UPDATE notification_queue SET next_retry_at=? WHERE status='pending'`, time.Now().Add(-time.Second).Unix()); err != nil {
		t.Fatal(err)
	}
	select {
	case q.notifyChan <- struct{}{}:
	default:
	}
	drainContinuity(t, n)
	mu.Lock()
	defer mu.Unlock()
	if counts["/a"] != 1 || counts["/b"] != 1 {
		t.Fatalf("checkpoint/restart delivery counts: %v", counts)
	}
}

func TestNotificationContinuitySameStartResumeCancelsDelayedRecovery(t *testing.T) {
	var mu sync.Mutex
	var events []string
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Event string `json:"event"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			w.WriteHeader(400)
			return
		}
		mu.Lock()
		events = append(events, payload.Event)
		mu.Unlock()
		w.WriteHeader(200)
	}))
	t.Cleanup(server.Close)
	hook := WebhookConfig{ID: "resume", URL: server.URL, Enabled: true, Service: "generic"}
	n := continuityManager(t, t.TempDir(), hook)
	a := &alerts.Alert{ID: "resumed", StartTime: time.Now(), Level: alerts.AlertLevelCritical}
	n.SendAlert(a)
	drainContinuity(t, n)
	n.CancelResolvedAlert(a)
	n.SetGroupingConfig(true, 3600, false, false)
	n.SendResolvedAlert(&alerts.ResolvedAlert{Alert: a.Clone(), ResolvedTime: time.Now()})
	n.SetGroupingConfig(false, 0, false, false)
	n.SetOccurrenceLifecycleProvider(func(string, time.Time) alerts.NotificationOccurrenceLifecycle {
		return alerts.NotificationOccurrenceLifecycle{State: alerts.NotificationOccurrenceActive}
	})
	n.SendAlert(a)
	drainContinuity(t, n)
	var cancelled int
	if err := n.GetQueue().db.QueryRow(`SELECT COUNT(*) FROM notification_queue WHERE type='webhook_resolved' AND status='cancelled'`).Scan(&cancelled); err != nil || cancelled != 1 {
		t.Fatalf("resume retained stale recovery: cancelled=%d error=%v", cancelled, err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(events) != 2 || events[0] != "" || events[1] != "" {
		t.Fatalf("resume delivery sequence: %v", events)
	}
}
