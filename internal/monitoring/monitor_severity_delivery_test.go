package monitoring

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	"github.com/stretchr/testify/require"
)

// Issue 1801: exercise actual observation, manager admission, monitor callback,
// persistent queue, HTTP destination and delivery history, not just callbacks.
func TestMonitorSeverityDelivery(t *testing.T) {
	for _, tc := range []struct {
		cooldown, group int
		policy          string
	}{
		{0, 0, "ready"}, {30, 0, "ready"}, {30, 1, "ready"},
		{30, 0, "acknowledged"}, {30, 0, "snoozed"}, {30, 0, "rate-limited"},
		{30, 0, "routing"}, {30, 0, "metric"}, {30, 0, "health"},
	} {
		t.Run(fmt.Sprintf("%s/cooldown%d/group%d", tc.policy, tc.cooldown, tc.group), func(t *testing.T) {
			requests := make(chan string, 20)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var payload struct {
					Level string `json:"level"`
				}
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Error(err)
				}
				requests <- payload.Level
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()
			nm := notifications.NewNotificationManagerWithDataDir("", t.TempDir())
			defer nm.Stop()
			require.NotNil(t, nm.GetQueue())
			nm.SetCooldown(tc.cooldown)
			nm.SetGroupingWindow(tc.group)
			require.NoError(t, nm.UpdateAllowedPrivateCIDRs("127.0.0.1/32"))
			nm.AddWebhook(notifications.WebhookConfig{ID: "hook", Name: "hook", URL: server.URL, Enabled: true, Template: `{"level":"{{.Level}}"}`})
			mgr := alerts.NewManagerWithDataDir(t.TempDir())
			defer mgr.Stop()
			cfg := mgr.GetConfig()
			cfg.Enabled = true
			cfg.ActivationState = alerts.ActivationActive
			cfg.FlappingEnabled = false
			cfg.Schedule.MaxAlertsHour = 100
			// Disable fixture activation delay explicitly; absent entries default to five seconds.
			cfg.TimeThresholds = map[string]int{"agent": 0, "storage": 0}
			cfg.SuppressionWindow = 0
			cfg.MetricEvaluationWindows = map[string]map[string]int{"agent": {"cpu": 0}}
			if tc.policy == "metric" {
				cfg.AgentDefaults.CPU = &alerts.HysteresisThreshold{Trigger: 80, Clear: 75}
			}
			if tc.policy == "rate-limited" {
				cfg.Schedule.MaxAlertsHour = 1
			}
			mgr.UpdateConfig(cfg)
			monitor := &Monitor{alertManager: mgr, notificationMgr: nm}
			mgr.SetAlertCallback(monitor.handleAlertFired)
			host := models.Host{ID: "sensor-host", Hostname: "sensor-host", Sensors: models.HostSensorSummary{
				Custom: []models.HostCustomSensorMetric{{ID: "probe", Name: "Probe", Status: "warning", ObservedAt: time.Now()}},
			}}
			alertType := "custom-sensor"
			storage := models.Storage{ID: "pool-store", Name: "tank", Status: "online", ZFSPool: &models.ZFSPool{Name: "tank", State: "DEGRADED"}}
			if tc.policy == "metric" {
				host.Sensors.Custom = nil
				host.CPUUsage = 85
				alertType = "cpu"
			}
			if tc.policy == "health" {
				alertType = "zfs-pool-state"
			}
			observe := func(critical bool) {
				if tc.policy == "health" {
					if critical {
						storage.ZFSPool.State = "FAULTED"
					} else {
						storage.ZFSPool.State = "DEGRADED"
					}
					mgr.CheckStorage(storage)
				} else {
					if tc.policy == "metric" {
						if critical {
							host.CPUUsage = 95
						} else {
							host.CPUUsage = 85
						}
					} else {
						if critical {
							host.Sensors.Custom[0].Status = "critical"
						} else {
							host.Sensors.Custom[0].Status = "warning"
						}
					}
					mgr.CheckHost(host)
				}
			}
			ready := tc.policy == "ready" || tc.policy == "metric" || tc.policy == "health"
			await := func(level string, sent int) {
				t.Helper()
				select {
				case got := <-requests:
					require.Equal(t, level, got)
				case <-time.After(4 * time.Second):
					t.Fatalf("missing %s destination delivery; history=%+v", level, deliveryHistory(t, nm))
				}
				// Wait for successful queue completion (which follows durable receipts and
				// the cooldown marker), not merely arrival at the HTTP handler.
				require.Eventually(t, func() bool {
					stats, err := nm.GetQueueStats()
					return err == nil && stats["sent"] == sent && len(deliveryHistory(t, nm)) == sent
				}, 3*time.Second, 10*time.Millisecond)
				history := deliveryHistory(t, nm)
				require.Len(t, history, sent)
				for _, row := range history {
					require.True(t, row.Success)
					require.Equal(t, notifications.DeliveryOutcomeSent, row.Outcome)
				}
			}
			observe(false)
			await("warning", 1)
			var original alerts.Alert
			for _, a := range mgr.GetActiveAlerts() {
				if a.Type == alertType {
					original = a
				}
			}
			require.NotEmpty(t, original.ID)
			switch tc.policy {
			case "acknowledged":
				require.NoError(t, mgr.AcknowledgeAlert(original.ID, "test"))
			case "snoozed":
				require.NoError(t, mgr.SnoozeAlert(original.ID, "test", time.Now().Add(time.Hour)))
			case "routing":
				nm.SetInitialNotifyTarget("email") // no email configured; must not widen to webhook
			}
			observe(false)
			observe(true)
			want := 1
			if ready {
				await("critical", 2)
				want = 2
			}
			found := false
			for _, a := range mgr.GetActiveAlerts() {
				if a.ID == original.ID {
					found = true
					require.Equal(t, alerts.AlertLevelCritical, a.Level)
					require.Equal(t, original.StartTime, a.StartTime)
				}
			}
			require.True(t, found, "same occurrence must remain active")
			observe(true)
			observe(false)
			// Also probe notification-layer repeat/downgrade suppression directly once
			// the ready path has completed both sends.
			if ready {
				nm.SendAlert(&original)
				critical := original.Clone()
				critical.Level = alerts.AlertLevelCritical
				nm.SendAlert(critical)
			}
			select {
			case extra := <-requests:
				t.Fatalf("unexpected repeat/suppressed delivery: %s", extra)
			case <-time.After(1200 * time.Millisecond):
			}
			history := deliveryHistory(t, nm)
			require.Len(t, history, want)
			for _, row := range history {
				require.Equal(t, []string{original.ID}, row.AlertIDs)
				require.Equal(t, "webhook:hook", row.DestinationID)
				require.Equal(t, 1, row.Attempts)
			}
			dlq, err := nm.GetQueue().GetDLQ(100)
			require.NoError(t, err)
			require.Empty(t, dlq)
		})
	}
}

func deliveryHistory(t *testing.T, nm *notifications.NotificationManager) []notifications.DeliveryLogEntry {
	t.Helper()
	rows, err := nm.GetDeliveryLog(time.Time{}, 100)
	require.NoError(t, err)
	return rows
}
