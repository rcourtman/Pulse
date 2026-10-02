package monitoring

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
