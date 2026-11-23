package notifications

import (
	"encoding/json"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/stretchr/testify/require"
)

type recoveryWebhookAttempt struct {
	alertID string
	status  int
}

func newRecoveryWebhook(t *testing.T) (WebhookConfig, *atomic.Bool, func() []recoveryWebhookAttempt) {
	t.Helper()
	down := new(atomic.Bool)
	down.Store(true)
	var mu sync.Mutex
	var attempts []recoveryWebhookAttempt
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Alerts []*alerts.Alert `json:"alerts"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.Alerts) != 1 || body.Alerts[0] == nil {
			t.Errorf("invalid fixture webhook payload: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		status := http.StatusNoContent
		if down.Load() {
			status = http.StatusServiceUnavailable
		}
		mu.Lock()
		attempts = append(attempts, recoveryWebhookAttempt{body.Alerts[0].ID, status})
		mu.Unlock()
		w.WriteHeader(status)
	}))
	return WebhookConfig{ID: "recovery", Name: "Recovery fixture", URL: server.URL, Enabled: true, Service: "generic"}, down, func() []recoveryWebhookAttempt {
		mu.Lock()
		defer mu.Unlock()
		return append([]recoveryWebhookAttempt(nil), attempts...)
	}
}

// Use one queue attempt to reach the terminal state without waiting through
// production retry backoff. The manager, saved destination and transport are real.
func enqueueRecoveryWebhook(t *testing.T, n *NotificationManager, id string) *alerts.Alert {
	t.Helper()
	cfg, err := json.Marshal(n.GetWebhooks()[0])
	require.NoError(t, err)
	alert := &alerts.Alert{ID: id, StartTime: time.Now().UTC(), ResourceName: id, Type: "cpu", Level: alerts.AlertLevelWarning}
	require.NoError(t, n.GetQueue().Enqueue(&QueuedNotification{
		ID: id, Type: "webhook", Config: cfg, MaxAttempts: 1, Alerts: []*alerts.Alert{alert},
	}))
	return alert
}

func recoveryCompletions(q *NotificationQueue) <-chan struct{} {
	changed := make(chan struct{}, 16)
	q.SetDeliveryHealthChangedCallback(func() {
		select {
		case changed <- struct{}{}:
		default:
		}
	})
	return changed
}

func waitRecoveryCompletion(t *testing.T, changed <-chan struct{}) {
	t.Helper()
	select {
	case <-changed:
	case <-time.After(5 * time.Second):
		t.Fatal("notification processing did not reconcile delivery health")
	}
}

func TestAutoReplayDLQOnDestinationRecovery_RealProcessorOnlyAfterFreshSuccess(t *testing.T) {
	hook, down, recipient := newRecoveryWebhook(t)
	dir := t.TempDir()
	n := continuityManager(t, dir, hook)
	q := n.GetQueue()
	changed := recoveryCompletions(q)

	enqueueRecoveryWebhook(t, n, "eligible")
	waitRecoveryCompletion(t, changed)
	waitForNotificationStatus(t, q, "eligible", QueueStatusDLQ, 1)

	// An older resolved occurrence may have the same destination and an
	// otherwise retryable failure. Keep its historical attempt, never send it.
	obsolete := enqueueRecoveryWebhook(t, n, "obsolete")
	waitRecoveryCompletion(t, changed)
	waitForNotificationStatus(t, q, "obsolete", QueueStatusDLQ, 1)
	n.CancelResolvedAlert(obsolete)
	waitForNotificationStatus(t, q, "obsolete", QueueStatusCancelled, 1)

	down.Store(false)
	// The provider accepts this alert, but the local completion write fails.
	// There must be no release of other work while the acceptance is pending.
	_, err := q.db.Exec(`CREATE TRIGGER hold_recovery_fixture_completion BEFORE UPDATE OF status ON notification_queue
		WHEN NEW.id='accepted-before-restart' AND NEW.status='sent' BEGIN SELECT RAISE(ABORT, 'fixture completion hold'); END`)
	require.NoError(t, err)
	enqueueRecoveryWebhook(t, n, "accepted-before-restart")
	require.Eventually(t, func() bool {
		stats, err := q.GetQueueStats()
		return err == nil && stats["settlement_pending"] == 1 && stats["pending"] == 1 && stats["sending"] == 0
	}, 5*time.Second, time.Millisecond)
	require.Equal(t, []recoveryWebhookAttempt{{"eligible", 503}, {"obsolete", 503}, {"accepted-before-restart", 204}}, recipient())
	n.Stop()

	n = deferredContinuityManager(t, dir, hook)
	q = n.GetQueue()
	changed = recoveryCompletions(q)
	_, err = q.db.Exec(`DROP TRIGGER hold_recovery_fixture_completion`)
	require.NoError(t, err)
	retryContinuitySettlementNow(t, q)
	require.NoError(t, n.StartQueueProcessingWithConfiguration(true))
	waitRecoveryCompletion(t, changed)
	waitForNotificationStatus(t, q, "accepted-before-restart", QueueStatusSent, 1)
	waitForNotificationStatus(t, q, "eligible", QueueStatusDLQ, 1)
	require.Equal(t, []recoveryWebhookAttempt{{"eligible", 503}, {"obsolete", 503}, {"accepted-before-restart", 204}}, recipient(),
		"local completion recontacted the recipient or released a recovery batch")

	enqueueRecoveryWebhook(t, n, "fresh")
	waitRecoveryCompletion(t, changed)
	waitForNotificationStatus(t, q, "fresh", QueueStatusSent, 1)
	waitForNotificationStatus(t, q, "eligible", QueueStatusSent, 2)
	waitForNotificationStatus(t, q, "obsolete", QueueStatusCancelled, 1)
	n.Stop()
	require.Equal(t, []recoveryWebhookAttempt{
		{"eligible", 503}, {"obsolete", 503}, {"accepted-before-restart", 204}, {"fresh", 204}, {"eligible", 204},
	}, recipient())
}

func TestAutoReplayDLQOnDestinationRecovery_IsolatesQueuedRehearsals(t *testing.T) {
	for _, rehearsalFails := range []bool{false, true} {
		name := "rehearsal_acceptance_does_not_trigger"
		if rehearsalFails {
			name = "ordinary_acceptance_leaves_rehearsal_parked"
		}
		t.Run(name, func(t *testing.T) {
			hook, down, recipient := newRecoveryWebhook(t)
			n := deferredContinuityManager(t, t.TempDir(), hook)
			q := n.GetQueue()
			changed := recoveryCompletions(q)
			startRehearsal := func() string {
				t.Helper()
				id := uuid.NewString()
				_, err := n.StartDeliveryRehearsal(RehearsalRequest{ID: id, Level: alerts.AlertLevelWarning})
				require.NoError(t, err)
				return id
			}
			var rehearsal string
			deadID, sentID := "ordinary", "ordinary"
			if rehearsalFails {
				rehearsal = startRehearsal()
				require.NoError(t, q.db.QueryRow(`SELECT notification_id FROM notification_rehearsal_jobs WHERE rehearsal_id=?`, rehearsal).Scan(&deadID))
				_, err := q.db.Exec(`UPDATE notification_queue SET max_attempts=1 WHERE id=?`, deadID)
				require.NoError(t, err)
			} else {
				enqueueRecoveryWebhook(t, n, deadID)
			}
			require.NoError(t, n.StartQueueProcessingWithConfiguration(true))
			waitRecoveryCompletion(t, changed)
			waitForNotificationStatus(t, q, deadID, QueueStatusDLQ, 1)
			down.Store(false)
			if rehearsalFails {
				enqueueRecoveryWebhook(t, n, sentID)
			} else {
				rehearsal = startRehearsal()
				require.NoError(t, q.db.QueryRow(`SELECT notification_id FROM notification_rehearsal_jobs WHERE rehearsal_id=?`, rehearsal).Scan(&sentID))
			}
			waitRecoveryCompletion(t, changed)
			waitForNotificationStatus(t, q, sentID, QueueStatusSent, 1)
			waitForNotificationStatus(t, q, deadID, QueueStatusDLQ, 1)
			// Make sure snapshot isolation didn't accidentally satisfy the
			// rehearsal test instead of the producer-specific guard.
			var sameConfig bool
			require.NoError(t, q.db.QueryRow(`SELECT a.config=b.config FROM notification_queue a, notification_queue b WHERE a.id=? AND b.id=?`, deadID, sentID).Scan(&sameConfig))
			require.True(t, sameConfig)
			n.Stop()

			failed, sent := "ordinary", "delivery-rehearsal:"+rehearsal
			if rehearsalFails {
				failed, sent = sent, failed
			}
			require.Equal(t, []recoveryWebhookAttempt{{failed, 503}, {sent, 204}}, recipient())
		})
	}
}
