package notifications

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/stretchr/testify/require"
)

// The checkpoint becomes normal queue work, not a successful send. The grouping
// deadline belongs to first admission, regardless of later config edits/reopens.
func TestStopCheckpointsFiringWindowPolicy(t *testing.T) {
	for _, policy := range []string{"ordinary", "quiet-later", "cancel-old", "pause", "email-disabled", "webhook-only", "tenant-isolation"} {
		t.Run(policy, func(t *testing.T) {
			f := newShutdownAdmissionFixture(t)
			email := EmailConfig{Enabled: true, SMTPHost: "smtp.example.invalid", SMTPPort: 587, From: "pulse@example.test", To: []string{"ops@example.test"}}
			apprise := AppriseConfig{Enabled: true, Targets: []string{"ntfy://example.invalid/pulse"}}
			m := f.open(func(m *NotificationManager) {
				m.SetEmailConfig(email)
				m.SetAppriseConfig(apprise)
				m.SetGroupingConfig(true, 600, true, true)
			})
			start := time.Now().UTC().Add(-time.Minute)
			old := pendingGroupSnapshot("reused", start, start, alerts.AlertLevelWarning, "old")
			current := pendingGroupSnapshot("reused", start.Add(time.Nanosecond), start, alerts.AlertLevelWarning, "current")
			current.OperationalRecord.ID += "-current"
			peer := pendingGroupSnapshot("peer", start, start, alerts.AlertLevelWarning, "peer")
			if policy == "quiet-later" {
				peer.Metadata[alerts.MetadataQuietHoursReplayAt] = time.Now().Add(2 * time.Hour).UTC().Format(time.RFC3339Nano)
			}
			if policy == "webhook-only" {
				m.SetInitialNotifyTarget("webhook")
			}
			for _, a := range []*alerts.Alert{old, current, peer} {
				m.SendAlert(a)
			}
			m.mu.RLock()
			deadline := m.groupDeadline
			m.mu.RUnlock()
			current.LastSeen = start.Add(time.Second)
			current.Message = "latest"
			m.SendAlert(current)
			// Editing a live window never resets an existing timer.
			m.SetGroupingWindow(3600)
			switch policy {
			case "cancel-old":
				require.True(t, m.CancelResolvedAlert(old))
			case "pause":
				m.SetEnabled(false)
			case "email-disabled":
				email.Enabled = false
				m.SetEmailConfig(email)
			}
			m.Stop()
			m = f.open(func(m *NotificationManager) { m.SetEmailConfig(email); m.SetAppriseConfig(apprise) })
			rows := quietReplayRows(t, m.GetQueue())
			if policy == "pause" {
				require.Empty(t, rows)
				return
			}
			want := 3
			switch policy {
			case "quiet-later":
				want = 6
			case "email-disabled":
				want = 2
			case "webhook-only":
				want = 1
			}
			require.Len(t, rows, want)
			for _, row := range rows {
				require.Equal(t, QueueStatusPending, row.Status)
				require.Zero(t, row.Attempts)
				require.Nil(t, row.LastAttempt)
				require.NotNil(t, row.NextRetryAt)
				require.False(t, row.NextRetryAt.Before(deadline), "shutdown rounded the original window down")
				if policy != "quiet-later" || row.Alerts[0].ID != peer.ID {
					require.Less(t, row.NextRetryAt.Sub(deadline), time.Second, "shutdown reset the window to edited configuration")
				}
				wantAlerts := 3
				switch policy {
				case "cancel-old":
					wantAlerts = 2
				case "quiet-later":
					wantAlerts = 2
					if row.Alerts[0].ID == peer.ID {
						wantAlerts = 1
						require.True(t, row.NextRetryAt.After(time.Now().Add(time.Hour)))
					}
				}
				require.Len(t, row.Alerts, wantAlerts)
				require.Len(t, row.Links, wantAlerts)
				for i, a := range row.Alerts {
					require.Equal(t, a.LatestTransition.ID, row.Links[i].TransitionID)
					if a.StartTime.Equal(current.StartTime) && a.ID == current.ID {
						require.Equal(t, "latest", a.Message)
					}
					if policy == "cancel-old" {
						require.False(t, a.ID == old.ID && a.StartTime.Equal(old.StartTime))
					}
				}
				if row.Type == "webhook" {
					var saved WebhookConfig
					require.NoError(t, json.Unmarshal(row.Config, &saved))
					require.Equal(t, f.hook, saved)
				}
			}
			due, err := m.GetQueue().GetPending(100)
			require.NoError(t, err)
			require.Empty(t, due, "delayed pending work must not be ready before its original deadline")
			if policy == "tenant-isolation" {
				other := NewNotificationManagerWithDeferredQueue("", t.TempDir())
				defer other.Stop()
				require.Empty(t, quietReplayRows(t, other.GetQueue()), "a second tenant directory must not inherit this buffer")
			}
			select {
			case <-f.spawned:
				t.Fatal("checkpoint dispatched direct delivery")
			default:
			}
		})
	}
}

// Concurrent resolutions and pause retain the existing lifecycle lease and the
// queue's cancellation semantics; no gap between memory and durable queue rows.
func TestStopCheckpointConcurrentCancellation(t *testing.T) {
	for _, policy := range []string{"resolve", "pause", "disable-channel", "disable-grouping"} {
		t.Run(policy, func(t *testing.T) {
			f := newShutdownAdmissionFixture(t)
			m := f.open(func(m *NotificationManager) { m.SetGroupingConfig(true, 600, false, false) })
			start := time.Now().UTC().Add(-time.Minute)
			a := quietReplayAlert("buffered", alerts.AlertLevelWarning, start)
			m.SendAlert(a)
			// An owned direct send holds the draining stage open. This avoids
			// treating an operation begun after shutdown closed as a cancellation.
			require.True(t, m.lifecycle.join())
			stopped := make(chan struct{})
			go func() { m.Stop(); close(stopped) }()
			awaitShutdownCondition(t, "draining lifecycle", func() bool { return !m.lifecycle.accepting() })
			switch policy {
			case "resolve":
				m.CancelResolvedAlert(a)
			case "pause":
				m.SetEnabled(false)
			case "disable-channel":
				hook := f.hook
				hook.Enabled = false
				require.NoError(t, m.UpdateWebhook(hook.ID, hook))
			case "disable-grouping":
				m.SetGroupingConfig(false, 0, false, false)
			}
			m.lifecycle.release()
			awaitShutdownSignal(t, "Stop after operator policy", stopped)
			m = f.open(nil)
			rows := quietReplayRows(t, m.GetQueue())
			for _, row := range rows {
				if policy == "resolve" || policy == "pause" {
					require.Equal(t, QueueStatusCancelled, row.Status)
				} else {
					require.Equal(t, QueueStatusPending, row.Status)
					require.NotNil(t, row.NextRetryAt)
				}
			}
			if policy == "disable-grouping" {
				require.Len(t, rows, 1, "a policy edit during drain lost the buffer")
			}
			select {
			case <-f.spawned:
				t.Fatal("operator policy during shutdown sent directly")
			default:
			}
		})
	}
}

func TestStopCheckpointStorageErrorNeverSendsEarly(t *testing.T) {
	f := newShutdownAdmissionFixture(t)
	m := f.open(func(m *NotificationManager) { m.SetGroupingConfig(true, 600, false, false) })
	_, err := m.GetQueue().db.Exec(`CREATE TRIGGER reject_checkpoint BEFORE INSERT ON notification_queue BEGIN SELECT RAISE(ABORT, 'fixture checkpoint rejected'); END`)
	require.NoError(t, err)
	m.SendAlert(quietReplayAlert("pending", alerts.AlertLevelWarning, time.Now().UTC()))
	m.Stop()
	select {
	case <-f.spawned:
		t.Fatal("checkpoint failure bypassed the grouping window with a direct send")
	default:
	}
}

// A timer already taking a lease and a concurrent Stop may race, but exactly one
// path owns the batch. No provider is activated in this persistent queue check.
func TestStopCheckpointRacesTimerOnce(t *testing.T) {
	f := newShutdownAdmissionFixture(t)
	for range 12 {
		m := f.open(func(m *NotificationManager) { m.SetGroupingConfig(true, 600, false, false) })
		a := quietReplayAlert(time.Now().UTC().Format(time.RFC3339Nano), alerts.AlertLevelWarning, time.Now().UTC())
		m.SendAlert(a)
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); m.sendGroupedAlerts() }()
		go func() { defer wg.Done(); m.Stop() }()
		wg.Wait()
	}
	rows := quietReplayRows(t, f.open(nil).GetQueue())
	require.Len(t, rows, 12)
	for _, row := range rows {
		require.Zero(t, row.Attempts)
		require.Len(t, row.Alerts, 1)
		require.Equal(t, QueueStatusPending, row.Status)
	}
}
