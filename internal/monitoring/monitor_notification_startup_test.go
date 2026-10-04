package monitoring

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
)

// A due persisted row must receive current saved quiet-hours policy BEFORE the
// autonomous queue is activated, on initial construction and reconstruction.
// This observes local HTTP acceptance only, not an installed destination.
func TestNewRevalidatesPersistedQuietHoursBeforeDelivery(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", dir)
	var deliveries atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		deliveries.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	quiet := alerts.QuietHours{Enabled: true, Start: "00:00", End: "23:59", Timezone: "UTC",
		Days: map[string]bool{"sunday": true, "monday": true, "tuesday": true,
			"wednesday": true, "thursday": true, "friday": true, "saturday": true}}
	persistence := config.NewConfigPersistence(dir)
	saved := alerts.AlertConfig{Enabled: true, ActivationState: alerts.ActivationActive,
		Schedule: alerts.ScheduleConfig{QuietHours: quiet}}
	if err := persistence.SaveAlertConfig(saved); err != nil {
		t.Fatal(err)
	}
	hook := notifications.WebhookConfig{ID: "saved-local-ops", Name: "saved-local-ops", Enabled: true, URL: server.URL, Service: "generic"}
	if err := persistence.SaveWebhooks([]notifications.WebhookConfig{hook}); err != nil {
		t.Fatal(err)
	}
	configJSON, err := json.Marshal(hook)
	if err != nil {
		t.Fatal(err)
	}
	for startup := 0; startup < 2; startup++ {
		seed, err := notifications.NewNotificationQueue(dir)
		if err != nil {
			t.Fatal(err)
		}
		due := time.Now().Add(-time.Hour)
		id := "saved-quiet-first"
		if startup == 1 {
			id = "saved-quiet-second"
		}
		if err := seed.Enqueue(&notifications.QueuedNotification{ID: id, Type: "webhook", Config: configJSON,
			Alerts:      []*alerts.Alert{{ID: id, Type: "cpu", Level: alerts.AlertLevelWarning, StartTime: due}},
			NextRetryAt: &due}); err != nil {
			t.Fatal(err)
		}
		if err := seed.Stop(); err != nil {
			t.Fatal(err)
		}
		m, err := New(&config.Config{DataPath: dir})
		if err != nil {
			t.Fatal(err)
		}
		n := m.GetNotificationManager()
		if err := n.UpdateAllowedPrivateCIDRs("127.0.0.1/32"); err != nil {
			m.Stop()
			t.Fatal(err)
		}
		q := n.GetQueue()
		deadline := time.Now().Add(5 * time.Second)
		for {
			pending, err := q.GetPending(10)
			if err != nil {
				m.Stop()
				t.Fatal(err)
			}
			if len(pending) == 0 {
				break
			}
			if time.Now().After(deadline) {
				m.Stop()
				t.Fatal("persisted quiet notification was not revalidated")
			}
			time.Sleep(10 * time.Millisecond)
		}
		stats, err := q.GetQueueStats()
		if err != nil || stats["pending"] != startup+1 || stats["sent"] != 0 || stats["dlq"] != 0 || deliveries.Load() != 0 {
			m.Stop()
			t.Fatalf("QUIET_BOOTSTRAP: saved current schedule did not hold every persisted row: stats=%v deliveries=%d err=%v", stats, deliveries.Load(), err)
		}
		logs, err := q.GetDeliveryLog(time.Now().Add(-time.Hour), 10)
		if err != nil || len(logs) != 0 {
			m.Stop()
			t.Fatalf("quiet bootstrap manufactured provider attempts: %+v, %v", logs, err)
		}
		m.Stop()
	}
}

// Cover the real monitor constructor, not a fixture which reapplies manager
// setters after restart. Saving here uses persistence directly: this is not
// browser/API save acceptance, a process restart, or destination receipt proof.
func TestNewRestoresSavedNotificationChoices(t *testing.T) {
	for _, tc := range []struct {
		name, target     string
		resolve, enabled bool
		activation       alerts.ActivationState
	}{
		{"webhook_without_recovery", "webhook", false, true, alerts.ActivationActive},
		{"apprise_with_recovery", "apprise", true, true, alerts.ActivationActive},
		{"email_disabled", "email", true, false, alerts.ActivationActive},
		{"all_pending", "all", false, true, alerts.ActivationPending},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("PULSE_DATA_DIR", dir)
			persistence := config.NewConfigPersistence(dir)
			saved := alerts.AlertConfig{Enabled: tc.enabled, ActivationState: tc.activation,
				Schedule: alerts.ScheduleConfig{InitialNotify: tc.target, NotifyOnResolve: tc.resolve}}
			if err := persistence.SaveAlertConfig(saved); err != nil {
				t.Fatal(err)
			}
			webhooks := []notifications.WebhookConfig{{ID: "saved-ops", Name: "saved-ops", URL: "https://example.invalid/alerts", Enabled: true, Service: "generic", MinimumSeverity: "warning", TagFilter: []string{"ops"}, TagMode: "any"}}
			if err := persistence.SaveWebhooks(webhooks); err != nil {
				t.Fatal(err)
			}
			// Synthetic credentials only; no notification is sent by this test.
			email := notifications.EmailConfig{
				Enabled: tc.enabled, Provider: "custom", SMTPHost: "smtp.example.invalid", SMTPPort: 587,
				Username: "fixture-user", Password: "synthetic-email-password", From: "pulse@example.invalid",
				To: []string{"ops@example.invalid", "backup@example.invalid"}, StartTLS: true, RateLimit: 17,
				MinimumSeverity: "critical", TagFilter: []string{"ops"}, TagMode: "all",
			}
			if err := persistence.SaveEmailConfig(email); err != nil {
				t.Fatal(err)
			}
			apprise := notifications.AppriseConfig{
				Enabled: tc.enabled, Mode: notifications.AppriseModeHTTP, CLIPath: "apprise",
				Targets: []string{"json://example.invalid/alerts"}, TimeoutSeconds: 23,
				ServerURL: "https://apprise.example.invalid", ConfigKey: "fixture-config",
				APIKey: "synthetic-apprise-key", APIKeyHeader: "X-Fixture-Key", MinimumSeverity: "warning",
			}
			if err := persistence.SaveAppriseConfig(apprise); err != nil {
				t.Fatal(err)
			}
			for startup := 0; startup < 2; startup++ {
				func() {
					m, err := New(&config.Config{DataPath: dir})
					if err != nil {
						t.Fatal(err)
					}
					defer m.Stop()
					n := m.GetNotificationManager()
					if got := n.GetInitialNotifyTarget(); got != tc.target {
						t.Errorf("startup %d target=%q, want %q", startup, got, tc.target)
					}
					if got := n.GetNotifyOnResolve(); got != tc.resolve {
						t.Errorf("startup %d resolve=%t, want %t", startup, got, tc.resolve)
					}
					wantEnabled := tc.enabled && tc.activation == alerts.ActivationActive
					if got := n.IsEnabled(); got != wantEnabled {
						t.Errorf("startup %d enabled=%t, want %t", startup, got, wantEnabled)
					}
					if got := n.GetEmailConfig(); !reflect.DeepEqual(got, email) {
						t.Errorf("startup %d did not restore saved email configuration", startup)
					}
					if got := n.GetAppriseConfig(); !reflect.DeepEqual(got, apprise) {
						t.Errorf("startup %d did not restore saved Apprise configuration", startup)
					}
					if got := n.GetWebhooks(); !reflect.DeepEqual(got, webhooks) {
						t.Errorf("startup %d did not restore saved webhook configuration", startup)
					}
				}()
			}
		})
	}
}

// Real PBS breach -> recovery -> new breach, with one callback deliberately
// held at its asynchronous consumer boundary. No clocks or manager maps change.
func TestMonitorResolvedSnapshotKeepsRefiringPBSDelivery(t *testing.T) {
	endpoint, receipts := occurrenceEndpoint(t)
	n := occurrenceNotifier(t, t.TempDir(), endpoint)
	a, pbs := occurrenceManager(t)
	m := &Monitor{alertManager: a, notificationMgr: n}
	m.wireExternalAlertCallbacks(nil)
	entered, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })
	unsubscribe := a.SubscribeResolvedCallback(func(string) { close(entered); <-release })
	t.Cleanup(unsubscribe)
	a.CheckPBS(pbs)
	first := awaitOccurrenceRows(t, n, 1)[0].Alerts[0]
	pbs.CPU = 0
	a.CheckPBS(pbs)
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("resolution callback did not start")
	}
	pbs.CPU = 95
	a.CheckPBS(pbs)
	awaitOccurrenceRows(t, n, 2)
	active := a.GetActiveAlerts()
	if len(active) != 1 || active[0].ID != first.ID || active[0].StartTime.Equal(first.StartTime) {
		t.Fatalf("not a new recurring PBS occurrence: %+v", active)
	}
	current := active[0].Clone()
	releaseOnce.Do(func() { close(release) })
	remaining := awaitOccurrenceRows(t, n, 1)
	if !remaining[0].Alerts[0].StartTime.Equal(current.StartTime) {
		t.Fatal("delayed resolution kept the wrong occurrence")
	}
	assertOccurrenceHTTP(t, n, receipts, current)
}
