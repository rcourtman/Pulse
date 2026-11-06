package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	"github.com/stretchr/testify/require"
)

func patrolReceiptFinding(id string, start time.Time) *ai.Finding {
	return &ai.Finding{ID: id, ResourceID: "fixture-" + id, Title: "Space above threshold",
		Severity: ai.FindingSeverityWarning, Category: ai.FindingCategoryCapacity, DetectedAt: start}
}

func patrolReceiptDB(t *testing.T, dir string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(dir, "notification_queue.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	return db
}

// Explicit accepted-receipt fixture. Real delivery is exercised separately
// below through this package's production finding projector and notifier.
func seedPatrolReceipt(t *testing.T, db *sql.DB, id string, start, at time.Time) {
	t.Helper()
	_, err := db.Exec(`INSERT OR IGNORE INTO notification_destination_state(destination_key,epoch,fingerprint) VALUES('webhook:fixture','00000000-0000-4000-8000-000000000001','fixture')`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO notification_delivery_receipts(alert_id,alert_start_nanos,destination_key,delivery_epoch,delivered_at,delivered_nanos,severity,success_order)
		VALUES(?,?,'webhook:fixture','00000000-0000-4000-8000-000000000001',?,?,2,1)`, id, start.UnixNano(), at.Unix(), at.UnixNano())
	require.NoError(t, err)
}

func patrolReceiptStore(t *testing.T, dir string) *ai.FindingsStore {
	t.Helper()
	s := ai.NewFindingsStore()
	require.NoError(t, s.SetPersistence(ai.NewFindingsPersistenceAdapter(config.NewConfigPersistence(dir))))
	t.Cleanup(func() { require.NoError(t, s.ForceSave()) })
	return s
}

func TestPatrolReceiptLifecycleSettlesOnlyPositiveTerminalFacts(t *testing.T) {
	store := ai.NewFindingsStore()
	notificationDir := t.TempDir()
	n := notifications.NewNotificationManagerWithDeferredQueue("", notificationDir)
	t.Cleanup(n.Stop)
	n.SetOccurrenceLifecycleProvider(patrolFindingNotificationLifecycle(store, nil))
	db := patrolReceiptDB(t, notificationDir)
	start := time.Now().Add(-40 * 24 * time.Hour).Round(0)
	end := time.Now().Add(-31 * 24 * time.Hour).Round(0)
	// Seed explicit accepted receipts, not provider calls or invented audits.
	seed := func(id string) {
		seedPatrolReceipt(t, db, alerts.PatrolFindingNotificationIDPrefix+id, start, start)
	}
	const completed = 128
	historical := make(map[string]*ai.Finding, completed)
	for i := 0; i < completed; i++ {
		id := fmt.Sprintf("settled-%03d", i)
		f := patrolReceiptFinding(id, start)
		f.ResolvedAt = &end
		historical[id] = f
		seed(id)
	}
	// Model one saved historical snapshot, not 128 unrelated durable live
	// edits. The real loader still supplies every terminal finding fact.
	persistence := ai.NewFindingsPersistenceAdapter(config.NewConfigPersistence(t.TempDir()))
	require.NoError(t, persistence.SaveFindings(historical))
	require.NoError(t, store.SetPersistence(persistence))
	t.Cleanup(func() { require.NoError(t, store.ForceSave()) })
	for _, id := range []string{"active", "dismissed", "missing"} {
		if id != "missing" {
			require.True(t, store.Add(patrolReceiptFinding(id, start)))
		}
		if id == "dismissed" {
			require.True(t, store.Dismiss(id, "expected_behavior", "fixture intent"))
		}
		seed(id)
	}
	count := func() int {
		var n int
		require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM notification_delivery_receipts`).Scan(&n))
		return n
	}
	require.Equal(t, completed+3, count())
	beforeLog, err := n.GetDeliveryLog(time.Time{}, 200)
	require.NoError(t, err)
	n.ReconcileOccurrenceReceipts()
	require.Equal(t, 3, count(), "known old terminal findings must not accumulate in every hourly receipt scan")
	for _, id := range []string{"active", "dismissed", "missing"} {
		var resolved int64
		require.NoError(t, db.QueryRow(`SELECT resolved_at FROM notification_delivery_receipts WHERE alert_id=?`, alerts.PatrolFindingNotificationIDPrefix+id).Scan(&resolved))
		require.Zero(t, resolved, "age, intent or absence invented a terminal fact")
	}
	require.Equal(t, completed, store.Cleanup(24*time.Hour))
	n.ReconcileOccurrenceReceipts()
	require.Equal(t, 3, count())
	afterLog, err := n.GetDeliveryLog(time.Time{}, 200)
	require.NoError(t, err)
	require.Equal(t, beforeLog, afterLog, "receipt housekeeping must not change rendered delivery history")
	t.Logf("PATROL_FINAL_FRAME completed=%d receipts_before=%d receipts_after=3 active=1 dismissed=1 unknown=1 delivery_log_unchanged=true production_contact=false", completed, completed+3)
}

type patrolReceiptUnavailablePersistence struct{}

func (patrolReceiptUnavailablePersistence) SaveFindings(map[string]*ai.Finding) error { return nil }
func (patrolReceiptUnavailablePersistence) LoadFindings() (map[string]*ai.Finding, error) {
	return nil, errors.New("fixture finding load unavailable")
}

func TestPatrolReceiptLifecycleRetainsOldDispositionUntilOwnerCanSettleIt(t *testing.T) {
	for _, status := range []string{"active", "expected_behavior", "not_an_issue", "will_fix_later", "snoozed", "regressed", "owner missing", "load failed", "start mismatch", "recent resolution"} {
		t.Run(status, func(t *testing.T) {
			store := patrolReceiptStore(t, t.TempDir())
			start := time.Now().Add(-40 * 24 * time.Hour).Round(0)
			f := patrolReceiptFinding("retained-fixture", start)
			if status == "start mismatch" {
				f.DetectedAt = start.Add(time.Nanosecond)
			}
			require.True(t, store.Add(f))
			switch status {
			case "expected_behavior", "not_an_issue", "will_fix_later":
				require.True(t, store.Dismiss(f.ID, status, "fixture intent"))
			case "snoozed":
				require.True(t, store.Snooze(f.ID, time.Hour))
			case "regressed":
				require.True(t, store.Resolve(f.ID, false))
				require.False(t, store.Add(patrolReceiptFinding(f.ID, start)))
			case "recent resolution":
				require.True(t, store.Resolve(f.ID, false))
			case "load failed":
				require.Error(t, store.SetPersistence(patrolReceiptUnavailablePersistence{}))
			case "owner missing":
				store = nil
			}
			notificationDir := t.TempDir()
			n := notifications.NewNotificationManagerWithDeferredQueue("", notificationDir)
			t.Cleanup(n.Stop)
			n.SetOccurrenceLifecycleProvider(patrolFindingNotificationLifecycle(store, nil))
			id := alerts.PatrolFindingNotificationIDPrefix + f.ID
			db := patrolReceiptDB(t, notificationDir)
			seedPatrolReceipt(t, db, id, start, start)
			// Even a receipt's old positive disposition cannot supersede an
			// active/newer occurrence, recent end or unavailable owning source.
			_, err := db.Exec(`UPDATE notification_delivery_receipts SET resolved_at=? WHERE alert_id=?`, time.Now().Add(-31*24*time.Hour).UnixNano(), id)
			require.NoError(t, err)
			n.ReconcileOccurrenceReceipts()
			var retained int
			require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM notification_delivery_receipts WHERE alert_id=?`, id).Scan(&retained))
			require.Equal(t, 1, retained)
		})
	}
}

func TestPatrolReceiptLifecycleOwedRecoverySurvivesPruningAndRestart(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	require.NoError(t, err)
	var mu sync.Mutex
	counts := map[string]int{}
	type receivedPayload struct {
		Event  string         `json:"event"`
		Alerts []alerts.Alert `json:"alerts"`
	}
	var received []receivedPayload
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var body receivedPayload
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.Alerts) != 1 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		mu.Lock()
		counts[body.Event]++
		received = append(received, body)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	require.NoError(t, server.Listener.Close())
	server.Listener = listener
	server.Start()
	t.Cleanup(server.Close)
	count := func(event string) int { mu.Lock(); defer mu.Unlock(); return counts[event] }
	findingDir, notificationDir := t.TempDir(), t.TempDir()
	store := patrolReceiptStore(t, findingDir)
	f := patrolReceiptFinding("recovery-fixture", time.Now().Add(-40*24*time.Hour).Round(0))
	require.True(t, store.Add(f))
	alert := patrolFindingNotificationAlert(store.Get(f.ID))
	hook := notifications.WebhookConfig{ID: "fixture", URL: server.URL, Enabled: true, Service: "generic"}
	open := func() *notifications.NotificationManager {
		n := notifications.NewNotificationManagerWithDeferredQueue("", notificationDir)
		t.Cleanup(n.Stop)
		require.NoError(t, n.UpdateAllowedPrivateCIDRs("127.0.0.1/32"))
		n.SetGroupingConfig(false, 0, false, false)
		n.SetNotifyOnResolve(true)
		n.SetCooldown(30)
		n.AddWebhook(hook)
		n.SetOccurrenceLifecycleProvider(patrolFindingNotificationLifecycle(store, nil))
		return n
	}
	drain := func(n *notifications.NotificationManager) {
		require.Eventually(t, func() bool {
			stats, err := n.GetQueueStats()
			return err == nil && stats["pending"]+stats["sending"] == 0
		}, 10*time.Second, 5*time.Millisecond)
	}
	n := open()
	require.NoError(t, n.StartQueueProcessingWithConfiguration(true))
	n.SendAlert(alert)
	drain(n)
	require.Equal(t, 1, count(""))
	n.Stop()
	require.True(t, store.Resolve(f.ID, false))
	require.NoError(t, store.ForceSave())

	// Reopen with processing deferred: observe the real resolved finding into
	// its actual durable accepted receipt before the 24-hour findings purge.
	n = open()
	q := n.GetQueue()
	db := patrolReceiptDB(t, notificationDir)
	beforeLog, err := n.GetDeliveryLog(time.Time{}, 200)
	require.NoError(t, err)
	require.Len(t, beforeLog, 1, "fixture must contain a real accepted delivery, not empty history")
	n.ReconcileOccurrenceReceipts()
	afterLog, err := n.GetDeliveryLog(time.Time{}, 200)
	require.NoError(t, err)
	require.Equal(t, beforeLog, afterLog, "harvesting a receipt resolution changed rendered delivery history")
	var resolved int64
	require.NoError(t, db.QueryRow(`SELECT resolved_at FROM notification_delivery_receipts WHERE alert_id=?`, alert.ID).Scan(&resolved))
	require.Positive(t, resolved)
	n.SendResolvedAlert(&alerts.ResolvedAlert{Alert: alert, ResolvedTime: *store.Get(f.ID).ResolvedAt})
	pending, err := q.GetPending(10)
	require.NoError(t, err)
	require.Len(t, pending, 1)
	require.Equal(t, "webhook_resolved", pending[0].Type)

	// Advance only the already-positive receipt's grace clock. No missing or
	// active finding is aged into resolution. Actual cleanup removes the
	// resolved finding, then a fresh loaded store confirms only current absence.
	_, err = db.Exec(`UPDATE notification_delivery_receipts SET resolved_at=? WHERE alert_id=?`, time.Now().Add(-31*24*time.Hour).UnixNano(), alert.ID)
	require.NoError(t, err)
	require.Equal(t, 1, store.Cleanup(0))
	require.NoError(t, store.ForceSave())
	store = patrolReceiptStore(t, findingDir)
	n.SetOccurrenceLifecycleProvider(patrolFindingNotificationLifecycle(store, nil))
	n.ReconcileOccurrenceReceipts()
	var retained int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM notification_delivery_receipts WHERE alert_id=?`, alert.ID).Scan(&retained))
	require.Equal(t, 1, retained, "pending recovery must pin the expired receipt after findings compaction")
	n.Stop()

	n = open()
	require.NoError(t, n.StartQueueProcessingWithConfiguration(true))
	drain(n)
	require.Equal(t, 1, count(""), "reopen replayed the acknowledged firing")
	require.Equal(t, 1, count("resolved"), "the saved destination did not receive its owed recovery")
	db = patrolReceiptDB(t, notificationDir)
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM notification_delivery_receipts WHERE alert_id=?`, alert.ID).Scan(&retained))
	require.Zero(t, retained)
	mu.Lock()
	observed := append([]receivedPayload(nil), received...)
	mu.Unlock()
	require.Len(t, observed, 2)
	for _, payload := range observed {
		require.Equal(t, alert.ID, payload.Alerts[0].ID)
		require.True(t, alert.StartTime.Equal(payload.Alerts[0].StartTime), "recovery changed the saved occurrence")
	}
	t.Logf("PATROL_RECOVERY_FRAME firing_acceptances=%d recovery_acceptances=%d receipt_pinned_before_restart=1 receipts_after=0 nonempty_delivery_log_unchanged=true occurrence=%s start=%s production_contact=false", count(""), count("resolved"), alert.ID, alert.StartTime.Format(time.RFC3339Nano))
}
