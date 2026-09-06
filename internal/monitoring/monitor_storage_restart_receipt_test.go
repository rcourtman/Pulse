package monitoring

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
)

// Local HTTP receipt through the monitor callbacks and ordinary queue dispatcher.
// This is graceful SQLite reopen coverage, not installed-provider or crash proof.
func TestStorageMissingCapacityRestartNotificationReceipts(t *testing.T) {
	type receipt struct {
		Event      string         `json:"event"`
		Identifier string         `json:"alertIdentifier"`
		Alerts     []alerts.Alert `json:"alerts"`
	}
	received := make(chan receipt, 32)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var got receipt
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("decode receipt: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		received <- got
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	alertDir, notificationDir := t.TempDir(), t.TempDir()
	start := func() *Monitor {
		a := alerts.NewManagerWithDataDir(alertDir)
		n := notifications.NewNotificationManagerWithDataDir("http://pulse.example", notificationDir)
		t.Cleanup(a.Stop)
		t.Cleanup(n.Stop)
		if err := n.UpdateAllowedPrivateCIDRs("127.0.0.1/32,::1/128"); err != nil {
			t.Fatal(err)
		}
		n.AddWebhook(notifications.WebhookConfig{ID: "storage-receipt", URL: server.URL, Enabled: true, Service: "generic"})
		n.SetNotifyOnResolve(true)
		n.SetGroupingWindow(0)
		n.SetCooldown(0)
		a.UpdateConfig(alerts.AlertConfig{Enabled: true, ActivationState: alerts.ActivationActive,
			TimeThresholds: map[string]int{"storage": 0},
			StorageDefault: alerts.HysteresisThreshold{Trigger: 80, Clear: 70}})
		m := &Monitor{alertManager: a, notificationMgr: n}
		a.SetAlertCallback(m.handleAlertFired)
		a.SetResolvedCallback(m.handleAlertResolved)
		return m
	}
	stop := func(m *Monitor) { m.alertManager.Stop(); m.notificationMgr.Stop() }
	observe := func(m *Monitor, s models.Storage) {
		for range 5 {
			m.alertManager.CheckStorage(s)
		}
	}
	take := func() receipt {
		t.Helper()
		select {
		case got := <-received:
			return got
		case <-time.After(10 * time.Second):
			t.Fatal("missing HTTP receipt")
		}
		return receipt{}
	}
	quiet := func() {
		t.Helper()
		select {
		case got := <-received:
			t.Fatalf("unexpected HTTP receipt: %+v", got)
		case <-time.After(150 * time.Millisecond):
		}
	}
	// Wait for durable queue success, not merely arrival at the receiver.
	drain := func(m *Monitor) {
		t.Helper()
		deadline := time.Now().Add(10 * time.Second)
		for {
			stats, err := m.notificationMgr.GetQueueStats()
			if err != nil {
				t.Fatal(err)
			}
			if stats["pending"]+stats["sending"] == 0 {
				if stats["failed"]+stats["dlq"] != 0 {
					t.Fatalf("delivery failures: %v", stats)
				}
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("queue did not drain: %v", stats)
			}
			time.Sleep(time.Millisecond)
		}
	}
	s := models.Storage{ID: "pbs-primary-backups", Name: "backups", Instance: "pbs-primary", Type: "pbs", Status: "online",
		Total: 1000, Used: 850, Free: 150, Usage: 85}
	m := start()
	observe(m, s)
	first := take()
	if first.Event != "" || len(first.Alerts) != 1 {
		t.Fatalf("bad firing receipt: %+v", first)
	}
	original := first.Alerts[0]
	if original.ResourceID != s.ID || original.StartTime.IsZero() {
		t.Fatalf("bad identity: %+v", original)
	}
	drain(m)
	missing := s
	missing.Total, missing.Used, missing.Free, missing.Usage = 0, 0, 0, 0
	for _, restart := range []bool{false, true} {
		if restart {
			stop(m)
			m = start()
		}
		observe(m, missing)
		active := m.alertManager.GetActiveAlerts()
		if len(active) != 1 || active[0].ID != original.ID || !active[0].StartTime.Equal(original.StartTime) {
			t.Fatalf("missing observation changed incident: %+v", active)
		}
		drain(m)
		quiet()
	}
	empty := s
	empty.Used, empty.Usage, empty.Free = 0, 0, empty.Total
	observe(m, empty)
	recovery := take()
	if recovery.Event != "resolved" || recovery.Identifier != original.ID || len(recovery.Alerts) != 1 ||
		recovery.Alerts[0].ID != original.ID || !recovery.Alerts[0].StartTime.Equal(original.StartTime) {
		t.Fatalf("bad recovery receipt: %+v", recovery)
	}
	if len(m.alertManager.GetActiveAlerts()) != 0 {
		t.Fatal("healthy storage remained active")
	}
	drain(m)
	stop(m)
	m = start()
	observe(m, missing)
	if len(m.alertManager.GetActiveAlerts()) != 0 {
		t.Fatal("missing data resurrected resolved incident")
	}
	quiet()
	observe(m, s)
	recurrence := take()
	if recurrence.Event != "" || len(recurrence.Alerts) != 1 || recurrence.Alerts[0].ID != original.ID ||
		!recurrence.Alerts[0].StartTime.After(original.StartTime) {
		t.Fatalf("bad recurrence receipt: %+v", recurrence)
	}
	drain(m)
	stop(m)
	quiet()
}
