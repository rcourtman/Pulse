package monitoring

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
)

func activeEscalationFixture(t *testing.T, m *alerts.Manager, kind string, level alerts.AlertLevel) *alerts.Alert {
	t.Helper()
	t.Cleanup(m.Stop)
	cfg := m.GetConfig()
	cfg.Enabled = true
	cfg.ActivationState = alerts.ActivationActive
	cfg.Schedule.Escalation.Enabled = true
	m.UpdateConfig(cfg)
	m.RaiseSystemAlert(alerts.SystemAlertInput{Type: kind, Level: level, Message: "escalation fixture"})
	active := m.GetActiveAlerts()
	if len(active) != 1 {
		t.Fatalf("active fixtures = %d", len(active))
	}
	return active[0].Clone()
}

// The callback carries a snapshot and can run after a config edit or recovery.
// Invoke it after that boundary, rather than relying on goroutine timing.
func TestHandleAlertEscalatedRejectsStaleAdmission(t *testing.T) {
	for _, action := range []string{"disabled", "inactive", "alerts-disabled", "resolved", "new-occurrence", "acknowledged"} {
		t.Run(action, func(t *testing.T) {
			t.Setenv("PULSE_DATA_DIR", t.TempDir())
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
			defer server.Close()
			n := notifications.NewNotificationManager("https://pulse.example.test")
			defer n.Stop()
			if err := n.UpdateAllowedPrivateCIDRs("127.0.0.1/32"); err != nil {
				t.Fatal(err)
			}
			n.AddWebhook(notifications.WebhookConfig{ID: "test", URL: server.URL, Enabled: true})
			manager := alerts.NewManager()
			cfg := manager.GetConfig()
			cfg.Schedule.Escalation.Levels = []alerts.EscalationLevel{{After: 180, Notify: "webhook"}}
			cfg.Schedule.QuietHours.Enabled = false
			manager.UpdateConfig(cfg)
			snapshot := activeEscalationFixture(t, manager, "stale-escalation", alerts.AlertLevelCritical)
			cfg = manager.GetConfig()
			switch action {
			case "disabled":
				cfg.Schedule.Escalation.Enabled = false
				manager.UpdateConfig(cfg)
			case "inactive":
				cfg.ActivationState = alerts.ActivationPending
				manager.UpdateConfig(cfg)
			case "alerts-disabled":
				cfg.Enabled = false
				manager.UpdateConfig(cfg)
			case "resolved", "new-occurrence":
				if !manager.ClearAlert(snapshot.ID) {
					t.Fatal("clear failed")
				}
				if action == "new-occurrence" {
					activeEscalationFixture(t, manager, "stale-escalation", alerts.AlertLevelCritical)
				}
			case "acknowledged":
				if err := manager.AcknowledgeAlert(snapshot.ID, "test"); err != nil {
					t.Fatal(err)
				}
			}
			(&Monitor{alertManager: manager, notificationMgr: n}).handleAlertEscalated(nil, snapshot, 1)
			stats, err := n.GetQueue().GetQueueStats()
			if err != nil {
				t.Fatal(err)
			}
			for status, count := range stats {
				if count != 0 {
					t.Errorf("stale callback queued %s=%d", status, count)
				}
			}
		})
	}
}
