package monitoring

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	"github.com/stretchr/testify/require"
)

func awaitNotificationProducer(t *testing.T, what string, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
	}
}

// Both producer boundaries must remain owned: the alert callback can be
// scheduled but not yet have reached the monitor, and the monitor can in turn
// have scheduled work that has not yet reached notification admission.
// Use a deferred real queue, so no provider send can hide a lost enqueue.
func TestMonitorStopKeepsScheduledNotificationForRestart(t *testing.T) {
	for _, boundary := range []string{"alert-callback", "monitor-send"} {
		t.Run(boundary, func(t *testing.T) {
			received := make(chan *alerts.Alert, 4)
			receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer r.Body.Close()
				var body struct {
					Alerts []*alerts.Alert `json:"alerts"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.Alerts) != 1 {
					t.Error("invalid local receiver payload")
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				received <- body.Alerts[0]
				w.WriteHeader(http.StatusOK)
			}))
			t.Cleanup(receiver.Close)
			dir := t.TempDir()
			hook := notifications.WebhookConfig{ID: "shutdown-proof", URL: receiver.URL, Enabled: true, Service: "generic"}
			openNotifications := func() *notifications.NotificationManager {
				n := notifications.NewNotificationManagerWithDeferredQueue("", dir)
				t.Cleanup(n.Stop)
				require.NoError(t, n.UpdateAllowedPrivateCIDRs("127.0.0.1/32,::1/128"))
				n.SetGroupingConfig(false, 0, false, false)
				n.SetCooldown(0)
				n.AddWebhook(hook)
				return n
			}
			n := openNotifications()
			a := alerts.NewManagerWithDataDir(t.TempDir())
			cfg := a.GetConfig()
			cfg.Enabled, cfg.ActivationState = true, alerts.ActivationActive
			cfg.TimeThresholds = map[string]int{"storage": 0}
			cfg.StorageDefault = alerts.HysteresisThreshold{Trigger: 80, Clear: 70}
			a.UpdateConfig(cfg)
			m := &Monitor{alertManager: a, notificationMgr: n}
			entered, release := make(chan struct{}), make(chan struct{})
			var releaseOnce sync.Once
			unblock := func() { releaseOnce.Do(func() { close(release) }) }
			t.Cleanup(func() { unblock(); m.Stop() })
			a.SetAlertCallback(func(alert *alerts.Alert) {
				close(entered)
				<-release
				m.handleAlertFired(alert)
			})
			var occurrence alerts.Alert
			if boundary == "alert-callback" {
				s := models.Storage{ID: "backup-store", Name: "backup-store", Node: "pve", Instance: "lab", Type: "dir", Status: "available", Enabled: true, Active: true,
					Total: 1000, Used: 850, Free: 150, Usage: 85}
				for range 5 {
					a.CheckStorage(s)
				}
				awaitNotificationProducer(t, "detector callback entry", entered)
				active := a.GetActiveAlerts()
				require.Len(t, active, 1)
				occurrence = active[0]
			} else {
				occurrence = alerts.Alert{ID: "scheduled-firing", Type: "cpu", Level: alerts.AlertLevelWarning, ResourceName: "proof-host", StartTime: time.Now().UTC()}
				require.True(t, m.notificationProducers.start(func() {
					close(entered)
					<-release
					n.SendAlert(&occurrence)
				}))
				awaitNotificationProducer(t, "monitor producer entry", entered)
			}
			stopped := make(chan struct{})
			go func() { m.Stop(); close(stopped) }()
			// This gate is closed only after the alert manager drained. For
			// the earlier boundary we instead hold the callback across two
			// concurrent Stops, then verify the persisted result after both.
			if boundary == "monitor-send" {
				require.Eventually(t, func() bool {
					m.notificationProducers.mu.Lock()
					defer m.notificationProducers.mu.Unlock()
					return m.notificationProducers.closed
				}, 5*time.Second, time.Millisecond)
				require.True(t, n.IsEnabled(), "notification storage retired before scheduled send")
				select {
				case <-stopped:
					t.Fatal("Stop lost its scheduled send")
				default:
				}
			} else {
				require.Eventually(t, func() bool {
					return a.NotificationOccurrenceLifecycle(occurrence.ID, occurrence.StartTime).State == alerts.NotificationOccurrenceUnavailable
				}, 5*time.Second, time.Millisecond, "alert manager must actually enter Stop while callback is held")
				require.True(t, n.IsEnabled(), "downstream manager retired before producer callback returned")
			}
			unblock()
			awaitNotificationProducer(t, "monitor Stop", stopped)
			require.False(t, m.notificationProducers.start(func() { t.Error("post-stop send started") }))
			next := openNotifications()
			pending, err := next.GetQueue().GetPending(10)
			require.NoError(t, err)
			require.Len(t, pending, 1, "accepted producer firing must survive graceful reopen")
			require.Len(t, pending[0].Alerts, 1)
			require.Equal(t, occurrence.ID, pending[0].Alerts[0].ID)
			require.True(t, occurrence.StartTime.Equal(pending[0].Alerts[0].StartTime))
			select {
			case <-received:
				t.Fatal("deferred queue sent before restart activation")
			default:
			}
			require.NoError(t, next.StartQueueProcessing())
			select {
			case got := <-received:
				require.Equal(t, occurrence.ID, got.ID)
				require.True(t, occurrence.StartTime.Equal(got.StartTime))
			case <-time.After(10 * time.Second):
				t.Fatal("restarted queue lost owed notification")
			}
			require.Eventually(t, func() bool {
				stats, err := next.GetQueueStats()
				return err == nil && stats["pending"]+stats["sending"] == 0 && stats["failed"]+stats["dlq"] == 0
			}, 5*time.Second, time.Millisecond)
		})
	}
}

func TestMonitorNotificationProducerConcurrentStop(t *testing.T) {
	var p notificationProducers
	var starts, finishes sync.WaitGroup
	for range 64 {
		starts.Add(1)
		go func() {
			defer starts.Done()
			p.start(func() {})
		}()
	}
	for range 4 {
		finishes.Add(1)
		go func() { defer finishes.Done(); p.stop() }()
	}
	starts.Wait()
	finishes.Wait()
	require.False(t, p.start(func() { t.Error("retired monitor producer reopened") }))
}
