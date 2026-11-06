package notifications

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/stretchr/testify/require"
)

func TestDurableCooldownKeepsSeverityTimeAndDestination(t *testing.T) {
	for _, cooldown := range []int{0, 30} {
		t.Run(time.Duration(cooldown).String(), func(t *testing.T) {
			dir := t.TempDir()
			hook := WebhookConfig{ID: "ops", URL: "https://ops.example.test", Enabled: true}
			other := WebhookConfig{ID: "other", URL: "https://other.example.test", Enabled: true}
			a := &alerts.Alert{ID: "repeat", StartTime: time.Now().Add(-time.Hour), Level: alerts.AlertLevelCritical}
			open := func() *NotificationManager {
				m := NewNotificationManagerWithDeferredQueue("", dir)
				t.Cleanup(m.Stop)
				m.SetCooldown(cooldown)
				m.SetGroupingWindow(3600)
				m.AddWebhook(hook)
				return m
			}
			m := open()
			key := notificationDeliveryDestinationKey(notificationDeliveryJob{Type: "webhook", WebhookConfig: &hook})
			latest := time.Now().Add(-time.Minute).Round(0)
			require.NoError(t, recordFixtureDeliveryReceipt(m.queue, a.ID, a.StartTime, key, latest, notificationSeverityRank(a.Level)))
			require.NoError(t, recordFixtureDeliveryReceipt(m.queue, a.ID, a.StartTime, key, latest.Add(-time.Hour), notificationSeverityRank(alerts.AlertLevelWarning)))
			m.Stop()
			m = open()
			m.SendAlert(a)
			require.Empty(t, m.pendingAlerts, "restart/cache miss repeated critical")
			record, found, err := m.queue.deliveryReceipt(a, key)
			require.NoError(t, err)
			require.True(t, found)
			require.Equal(t, latest, record.lastSent)
			require.Equal(t, alerts.AlertLevelCritical, record.level)
			// A cache eviction is not a new notification entitlement.
			delete(m.lastNotified, a.ID)
			m.SendAlert(a)
			require.Empty(t, m.pendingAlerts)
			// An unrelated endpoint cannot lend cooldown to a replacement.
			require.NoError(t, m.UpdateWebhook(hook.ID, other))
			m.SendAlert(a)
			require.Len(t, m.pendingAlerts, 1)
			m.pendingAlerts = nil
			m.pendingAlertIndex = nil
			// Nor can a different occurrence inherit the prior severity history.
			a.StartTime = a.StartTime.Add(time.Nanosecond)
			m.SendAlert(a)
			require.Len(t, m.pendingAlerts, 1)
		})
	}
}

func TestDurableCooldownAllowsIncreasesExpiryAndLegacy(t *testing.T) {
	for _, tc := range []struct {
		name   string
		level  alerts.AlertLevel
		age    time.Duration
		legacy bool
		want   int
	}{
		{"critical increase", alerts.AlertLevelWarning, time.Minute, false, 1},
		{"same critical", alerts.AlertLevelCritical, time.Minute, false, 0},
		{"expired critical", alerts.AlertLevelCritical, time.Hour, false, 1},
		{"unknown legacy severity", "", time.Minute, true, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := NewNotificationManagerWithDeferredQueue("", t.TempDir())
			defer m.Stop()
			m.SetCooldown(30)
			m.SetGroupingWindow(3600)
			hook := WebhookConfig{ID: "ops", URL: "https://ops.example.test", Enabled: true}
			m.AddWebhook(hook)
			a := &alerts.Alert{ID: "same", StartTime: time.Now().Add(-2 * time.Hour), Level: alerts.AlertLevelCritical}
			key := notificationDeliveryDestinationKey(notificationDeliveryJob{Type: "webhook", WebhookConfig: &hook})
			if tc.legacy {
				require.NoError(t, recordFixtureDeliveryReceipt(m.queue, a.ID, a.StartTime, key, time.Now().Add(-tc.age), 0))
			} else {
				require.NoError(t, recordFixtureDeliveryReceipt(m.queue, a.ID, a.StartTime, key, time.Now().Add(-tc.age), notificationSeverityRank(tc.level)))
			}
			m.SendAlert(a)
			require.Len(t, m.pendingAlerts, tc.want)
			has, err := m.queue.HasDeliveryReceipt(a.ID, a.StartTime, key)
			require.NoError(t, err)
			require.True(t, has, "legacy delivery must still authorise recovery")
		})
	}
}

func TestReceiptContinuityMigratesOriginalSchema(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, notificationQueueFileName)
	db, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE TABLE notification_delivery_receipts(alert_id TEXT NOT NULL, alert_start_nanos INTEGER NOT NULL, destination_key TEXT NOT NULL, delivered_at INTEGER NOT NULL, PRIMARY KEY(alert_id,alert_start_nanos,destination_key));
	INSERT INTO notification_delivery_receipts VALUES ('legacy',123,'webhook:ops',123)`)
	require.NoError(t, err)
	require.NoError(t, db.Close())
	for range 2 {
		q, err := NewNotificationQueue(dir)
		require.NoError(t, err)
		has, err := q.HasDeliveryReceipt("legacy", time.Unix(0, 123), "webhook:ops")
		require.NoError(t, err)
		require.True(t, has)
		record, known, err := q.deliveryReceipt(&alerts.Alert{ID: "legacy", StartTime: time.Unix(0, 123)}, "webhook:ops")
		require.NoError(t, err)
		require.True(t, known, "legacy recovery authority must survive")
		require.Empty(t, record.level)
		require.NoError(t, q.Stop())
	}
}

func TestReceiptRetirementIsPositiveBoundedAndPinned(t *testing.T) {
	for _, policy := range []string{"active", "unknown", "resolved", "rotated", "pending-recovery", "retryable-recovery", "unreadable-pending", "revert", "recent-resolution"} {
		t.Run(policy, func(t *testing.T) {
			m := NewNotificationManagerWithDeferredQueue("", t.TempDir())
			defer m.Stop()
			m.SetGroupingConfig(false, 0, false, false)
			hook := WebhookConfig{ID: "ops", URL: "https://old.example.test", Enabled: true}
			m.AddWebhook(hook)
			q := m.GetQueue()
			old := time.Now().Add(-31 * 24 * time.Hour)
			a := &alerts.Alert{ID: "owed", StartTime: old.Add(-time.Hour), Level: alerts.AlertLevelCritical}
			job := notificationDeliveryJob{Type: "webhook", Event: eventAlert, Alerts: []*alerts.Alert{a}, WebhookConfig: &hook}
			key := notificationDeliveryDestinationKey(job)
			require.NoError(t, recordFixtureDeliveryReceipt(q, a.ID, a.StartTime, key, old, 3))
			m.SetOccurrenceLifecycleProvider(func(string, time.Time) alerts.NotificationOccurrenceLifecycle {
				if policy == "active" {
					return alerts.NotificationOccurrenceLifecycle{State: alerts.NotificationOccurrenceActive}
				}
				if policy == "unknown" {
					return alerts.NotificationOccurrenceLifecycle{State: alerts.NotificationOccurrenceUnavailable}
				}
				return alerts.NotificationOccurrenceLifecycle{State: alerts.NotificationOccurrenceNoRecord}
			})
			if policy == "resolved" || policy == "pending-recovery" || policy == "retryable-recovery" || policy == "unreadable-pending" {
				require.NoError(t, q.markReceiptOccurrenceResolved(a.ID, a.StartTime, old))
			}
			if policy == "recent-resolution" {
				m.CancelResolvedAlert(a)
			}
			if policy == "pending-recovery" || policy == "retryable-recovery" || policy == "unreadable-pending" {
				job.Event = eventResolved
				config, err := configJSONForNotificationDeliveryJob(job)
				require.NoError(t, err)
				require.NoError(t, q.Enqueue(&QueuedNotification{ID: "owed-recovery", Type: queueTypeForNotificationDeliveryJob(job), Alerts: job.Alerts, Config: config}))
				if policy == "retryable-recovery" {
					_, err = q.db.Exec(`UPDATE notification_queue SET status='dlq'`)
					require.NoError(t, err)
				}
				if policy == "unreadable-pending" {
					_, err = q.db.Exec(`UPDATE notification_queue SET config='{'`)
					require.NoError(t, err)
				}
			}
			if policy == "rotated" || policy == "revert" {
				replacement := hook
				replacement.URL = "https://new.example.test"
				require.NoError(t, m.UpdateWebhook(hook.ID, replacement))
				if policy == "revert" {
					require.NoError(t, m.UpdateWebhook(hook.ID, hook))
				}
			}
			q.performCleanup()
			has, err := q.HasDeliveryReceipt(a.ID, a.StartTime, key)
			require.NoError(t, err)
			require.Equal(t, policy != "resolved" && policy != "rotated" && policy != "revert", has)
		})
	}
}

func TestDestinationRetirementTracksLateReceiptsAndAllTransports(t *testing.T) {
	for _, kind := range []string{"webhook", "email", "apprise"} {
		t.Run(kind, func(t *testing.T) {
			m := NewNotificationManagerWithDeferredQueue("", t.TempDir())
			defer m.Stop()
			var key string
			var remove func()
			switch kind {
			case "webhook":
				c := WebhookConfig{ID: "ops", URL: "https://ops.example.test", Enabled: true}
				m.AddWebhook(c)
				key = notificationDeliveryDestinationKey(notificationDeliveryJob{Type: kind, WebhookConfig: &c})
				remove = func() { require.NoError(t, m.DeleteWebhook(c.ID)) }
			case "email":
				c := EmailConfig{Enabled: true, From: "pulse@example.test", To: []string{"ops@example.test"}}
				require.NoError(t, m.SetEmailConfig(c))
				key = notificationDeliveryDestinationKey(notificationDeliveryJob{Type: kind, EmailConfig: &c})
				remove = func() { c.Enabled = false; require.NoError(t, m.SetEmailConfig(c)) }
			case "apprise":
				c := AppriseConfig{Enabled: true, Mode: AppriseModeCLI, Targets: []string{"json://ops.example.test"}}
				require.NoError(t, m.SetAppriseConfig(c))
				key = notificationDeliveryDestinationKey(notificationDeliveryJob{Type: kind, AppriseConfig: &c})
				remove = func() { c.Enabled = false; require.NoError(t, m.SetAppriseConfig(c)) }
			}
			q := m.GetQueue()
			epoch, err := fixtureDeliveryEpoch(q, key)
			require.NoError(t, err)
			remove()
			old := time.Now().Add(-31 * 24 * time.Hour)
			a := &alerts.Alert{ID: "late", StartTime: old.Add(-time.Hour)}
			require.NoError(t, q.recordDeliveryReceipt(a.ID, a.StartTime, key, old, 0, epoch))
			q.performCleanup()
			has, err := q.HasDeliveryReceipt(a.ID, a.StartTime, key)
			require.NoError(t, err)
			require.False(t, has)
		})
	}
}

func TestCooldownCacheMissIsOwnedByShutdownAdmission(t *testing.T) {
	f := newShutdownAdmissionFixture(t)
	m := f.open(nil)
	q := m.GetQueue()
	a := quietReplayAlert("restored", alerts.AlertLevelCritical, time.Now().Add(-time.Hour))
	key := notificationDeliveryDestinationKey(notificationDeliveryJob{Type: "webhook", WebhookConfig: &f.hook})
	require.NoError(t, recordFixtureDeliveryReceipt(q, a.ID, a.StartTime, key, time.Now(), notificationSeverityRank(a.Level)))
	q.mu.Lock()
	sent := make(chan struct{})
	go func() { m.SendAlert(a); close(sent) }()
	// Admission takes the lease before waiting for this blocked storage read.
	awaitShutdownCondition(t, "cooldown read admission", func() bool { m.lifecycle.mu.Lock(); defer m.lifecycle.mu.Unlock(); return m.lifecycle.leases > 0 })
	stopped := make(chan struct{})
	go func() { m.Stop(); close(stopped) }()
	select {
	case <-stopped:
		t.Fatal("Stop closed an admitted cooldown read")
	default:
	}
	q.mu.Unlock()
	awaitShutdownSignal(t, "restored cooldown", sent)
	awaitShutdownSignal(t, "manager stop", stopped)
	require.Empty(t, m.pendingAlerts)
}

func TestLateDestinationCompletionDoesNotLendCooldown(t *testing.T) {
	m := NewNotificationManagerWithDeferredQueue("", t.TempDir())
	defer m.Stop()
	m.SetGroupingWindow(3600)
	original := WebhookConfig{ID: "ops", URL: "https://old.example.test", Enabled: true}
	m.AddWebhook(original)
	key := notificationDeliveryDestinationKey(notificationDeliveryJob{Type: "webhook", WebhookConfig: &original})
	epoch, err := fixtureDeliveryEpoch(m.GetQueue(), key)
	require.NoError(t, err)
	replacement := original
	replacement.URL = "https://new.example.test"
	require.NoError(t, m.UpdateWebhook(original.ID, replacement))
	a := &alerts.Alert{ID: "late-rotation", StartTime: time.Now(), Level: alerts.AlertLevelCritical}
	require.NoError(t, m.GetQueue().recordDeliveryReceipt(a.ID, a.StartTime, key, time.Now(), 3, epoch))
	m.SendAlert(a)
	require.Len(t, m.pendingAlerts, 1, "old URL completion muted the new destination")
}

func TestOldCooldownRestoreCannotEvictNewOccurrence(t *testing.T) {
	m := NewNotificationManagerWithDeferredQueue("", t.TempDir())
	defer m.Stop()
	m.SetGroupingWindow(3600)
	m.SetCooldown(0)
	hook := WebhookConfig{ID: "ops", URL: "https://ops.example.test", Enabled: true}
	m.AddWebhook(hook)
	old := &alerts.Alert{ID: "restored-old", StartTime: time.Now().Add(-time.Hour), Level: alerts.AlertLevelCritical}
	current := old.Clone()
	current.StartTime = current.StartTime.Add(time.Minute)
	key := notificationDeliveryDestinationKey(notificationDeliveryJob{Type: "webhook", WebhookConfig: &hook})
	require.NoError(t, recordFixtureDeliveryReceipt(m.queue, old.ID, old.StartTime, key, time.Now(), notificationSeverityRank(old.Level)))
	m.markAlertsNotified([]*alerts.Alert{current}, time.Now())
	m.SendAlert(old)
	require.Equal(t, current.StartTime, m.lastNotified[current.ID].alertStart)
	require.Empty(t, m.pendingAlerts)
}

func TestReceiptRetirementKeepsPolicyReadableWhileWaitingForQueue(t *testing.T) {
	for _, action := range []string{"webhook-delete", "webhook-rotate", "apprise-disable"} {
		t.Run(action, func(t *testing.T) {
			f := newShutdownAdmissionFixture(t)
			m := f.open(func(m *NotificationManager) {
				if action == "apprise-disable" {
					m.SetAppriseConfig(AppriseConfig{Enabled: true, Mode: AppriseModeCLI, Targets: []string{"json://ops.example.test"}})
				}
			})
			barrier := newAdmissionBarrier()
			a := quietReplayAlert("retirement-"+action, alerts.AlertLevelWarning, time.Now().Add(-time.Hour))
			a.Metadata = map[string]interface{}{"barrier": barrier}
			admitted := make(chan struct{})
			go func() { defer close(admitted); m.SendAlert(a) }()
			awaitShutdownSignal(t, "accepted enqueue holding the queue lock", barrier.entered)
			acted := make(chan struct{})
			go func() {
				defer close(acted)
				switch action {
				case "webhook-delete":
					if err := m.DeleteWebhook(f.hook.ID); err != nil {
						t.Error(err)
					}
				case "webhook-rotate":
					updated := f.hook
					updated.URL = "https://new.example.test"
					if err := m.UpdateWebhook(f.hook.ID, updated); err != nil {
						t.Error(err)
					}
				case "apprise-disable":
					config := m.GetAppriseConfig()
					config.Enabled = false
					m.SetAppriseConfig(config)
				}
			}()
			awaitShutdownCondition(t, "retirement admitted behind the blocked queue", func() bool { m.lifecycle.mu.Lock(); defer m.lifecycle.mu.Unlock(); return m.lifecycle.leases >= 2 })
			if action == "apprise-disable" {
				require.True(t, m.GetAppriseConfig().Enabled)
			} else {
				require.Equal(t, f.hook.URL, m.GetWebhooks()[0].URL)
			}

			stopped := make(chan struct{})
			go func() { m.Stop(); close(stopped) }()
			close(barrier.release)
			awaitShutdownSignal(t, "accepted enqueue", admitted)
			awaitShutdownSignal(t, "retirement write", acted)
			awaitShutdownSignal(t, "stop owns retirement write", stopped)
			q, err := NewNotificationQueue(f.dir)
			require.NoError(t, err)
			defer q.Stop()
			var oldKey string
			if action == "apprise-disable" {
				c := AppriseConfig{Enabled: true, Mode: AppriseModeCLI, Targets: []string{"json://ops.example.test"}}
				oldKey = notificationDeliveryDestinationKey(notificationDeliveryJob{Type: "apprise", AppriseConfig: &c})
			} else {
				oldKey = notificationDeliveryDestinationKey(notificationDeliveryJob{Type: "webhook", WebhookConfig: &f.hook})
			}
			var current int
			require.NoError(t, q.db.QueryRow(`SELECT COUNT(*) FROM notification_destination_state WHERE destination_key=?`, oldKey).Scan(&current))
			require.Zero(t, current, "retired generation remained current across shutdown")
		})
	}
}
