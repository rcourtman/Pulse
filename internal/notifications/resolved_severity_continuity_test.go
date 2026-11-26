package notifications

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/stretchr/testify/require"
)

// Regression test for Pulse #5191: when an alert fires as critical, silently
// demotes to warning without re-notifying, and then clears, the resolved
// webhook (both custom templated {{.Level}} and generic JSON) must preserve
// the critical severity that the destination actually received.
func TestResolvedWebhookPreservesNotifiedCriticalLevelAfterSilentDemotion(t *testing.T) {
	type customPayload struct {
		ID    string `json:"id"`
		Event string `json:"event"`
		Level string `json:"level"`
	}
	type genericPayload struct {
		Event  string          `json:"event"`
		Alerts []*alerts.Alert `json:"alerts"`
	}

	customCh := make(chan customPayload, 4)
	genericCh := make(chan genericPayload, 4)

	customServer := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var p customPayload
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			t.Errorf("decode custom payload: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		customCh <- p
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(customServer.Close)

	genericServer := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var p genericPayload
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			t.Errorf("decode generic payload: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		genericCh <- p
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(genericServer.Close)

	customHook := WebhookConfig{
		ID:       "home-assistant",
		Name:     "Home Assistant",
		URL:      customServer.URL,
		Enabled:  true,
		Service:  "custom",
		Template: `{"id":"{{.ID}}","event":"{{.Event}}","level":"{{.Level}}"}`,
	}
	genericHook := WebhookConfig{
		ID:      "generic-ops",
		Name:    "Generic",
		URL:     genericServer.URL,
		Enabled: true,
		Service: "generic",
	}

	nm := NewNotificationManagerWithDataDir("", t.TempDir())
	t.Cleanup(nm.Stop)
	nm.SetGroupingConfig(false, 0, false, false)
	nm.SetNotifyOnResolve(true)
	require.NoError(t, nm.UpdateAllowedPrivateCIDRs("127.0.0.1/32"))
	nm.AddWebhook(customHook)
	nm.AddWebhook(genericHook)

	am := alerts.NewManagerWithDataDir(t.TempDir())
	t.Cleanup(am.Stop)
	cfg := am.GetConfig()
	cfg.ActivationState = alerts.ActivationActive
	cfg.TimeThresholds["guest"] = 0
	cfg.GuestDefaults.CPU = &alerts.HysteresisThreshold{Trigger: 80, Clear: 75}
	cfg.Schedule.Cooldown = 60 // Prevent cooldown re-notify on silent demotion
	am.UpdateConfig(cfg)

	am.SubscribeAlertCallback(nm.SendAlert)
	am.SetResolvedAlertCallback(nm.SendResolvedAlert)

	container := models.Container{
		ID:       "pve1:node1:105",
		VMID:     105,
		Name:     "app-ct",
		Node:     "node1",
		Instance: "pve1",
		Status:   "running",
		CPU:      0.995, // 99.5% -> critical (trigger 80, critical 90)
	}

	// Step 1: Fire at critical.
	am.CheckGuest(container, "pve1")
	active := am.GetActiveAlerts()
	require.Len(t, active, 1)
	require.Equal(t, alerts.AlertLevelCritical, active[0].Level)
	alertID := active[0].ID

	select {
	case msg := <-customCh:
		require.Equal(t, alertID, msg.ID)
		require.Equal(t, "alert", msg.Event)
		require.Equal(t, "critical", msg.Level)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for custom firing webhook")
	}
	select {
	case msg := <-genericCh:
		require.Len(t, msg.Alerts, 1)
		require.Equal(t, alerts.AlertLevelCritical, msg.Alerts[0].Level)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for generic firing webhook")
	}

	// Wait until both destinations have recorded their durable delivery receipts.
	for _, hook := range []WebhookConfig{customHook, genericHook} {
		hookCopy := hook
		require.Eventually(t, func() bool {
			job := notificationDeliveryJob{
				Type:          "webhook",
				Event:         eventResolved,
				Alerts:        []*alerts.Alert{&active[0]},
				WebhookConfig: &hookCopy,
			}
			return len(nm.filterResolvedJobsByDeliveryReceipt([]notificationDeliveryJob{job})) == 1
		}, 5*time.Second, 5*time.Millisecond)
	}

	// Step 2: Drop into warning band (85% -> between trigger 80 and critical 90).
	// Active alert silently demotes to warning without sending a new firing webhook.
	container.CPU = 0.85
	am.CheckGuest(container, "pve1")
	active = am.GetActiveAlerts()
	require.Len(t, active, 1)
	require.Equal(t, alerts.AlertLevelWarning, active[0].Level)

	select {
	case extra := <-customCh:
		t.Fatalf("unexpected re-notification on silent demotion: %+v", extra)
	case <-time.After(150 * time.Millisecond):
	}

	// Step 3: Clear below clear threshold (70% < 75%).
	// Resolved webhooks must still report level: "critical" to match the firing notification.
	container.CPU = 0.70
	am.CheckGuest(container, "pve1")
	require.Empty(t, am.GetActiveAlerts())

	select {
	case msg := <-customCh:
		require.Equal(t, alertID, msg.ID)
		require.Equal(t, "resolved", msg.Event)
		require.Equal(t, "critical", msg.Level)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for custom resolved webhook")
	}
	select {
	case msg := <-genericCh:
		require.Equal(t, "resolved", msg.Event)
		require.Len(t, msg.Alerts, 1)
		require.Equal(t, alerts.AlertLevelCritical, msg.Alerts[0].Level)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for generic resolved webhook")
	}
}

func TestFilterResolvedJobsAlignsSeverityPerDestinationAndHandlesLegacyAndMemoryModes(t *testing.T) {
	t.Run("queued-per-destination-and-legacy-fallback", func(t *testing.T) {
		nm := NewNotificationManagerWithDataDir("", t.TempDir())
		t.Cleanup(nm.Stop)
		q := nm.GetQueue()
		require.NotNil(t, q)

		hookA := WebhookConfig{ID: "dest-a", URL: "https://example.invalid/a", Enabled: true}
		hookB := WebhookConfig{ID: "dest-b", URL: "https://example.invalid/b", Enabled: true}
		hookLegacy := WebhookConfig{ID: "dest-legacy", URL: "https://example.invalid/legacy", Enabled: true}
		nm.AddWebhook(hookA)
		nm.AddWebhook(hookB)
		nm.AddWebhook(hookLegacy)

		start := time.Now().Add(-10 * time.Minute).Truncate(time.Millisecond)
		demoted := &alerts.Alert{
			ID:        "ct-cpu-105",
			Type:      "cpu",
			Level:     alerts.AlertLevelWarning,
			StartTime: start,
		}

		jobs := []notificationDeliveryJob{
			{Type: "webhook", Event: eventResolved, Alerts: []*alerts.Alert{demoted}, WebhookConfig: &hookA},
			{Type: "webhook", Event: eventResolved, Alerts: []*alerts.Alert{demoted}, WebhookConfig: &hookB},
			{Type: "webhook", Event: eventResolved, Alerts: []*alerts.Alert{demoted}, WebhookConfig: &hookLegacy},
		}

		nm.destinationMu.Lock()
		bound := nm.bindDeliveryEpochsLocked(jobs)
		nm.destinationMu.Unlock()

		// dest-a was notified at critical, dest-b at warning, dest-legacy has legacy unknown severity (0).
		require.NoError(t, q.recordDeliveryReceipt(demoted.ID, demoted.StartTime, notificationDeliveryDestinationKey(bound[0]), start.Add(time.Second), notificationSeverityRank(alerts.AlertLevelCritical), bound[0].ReceiptEpoch))
		require.NoError(t, q.recordDeliveryReceipt(demoted.ID, demoted.StartTime, notificationDeliveryDestinationKey(bound[1]), start.Add(time.Second), notificationSeverityRank(alerts.AlertLevelWarning), bound[1].ReceiptEpoch))
		require.NoError(t, q.recordDeliveryReceipt(demoted.ID, demoted.StartTime, notificationDeliveryDestinationKey(bound[2]), start.Add(time.Second), 0, bound[2].ReceiptEpoch))

		filtered := nm.filterResolvedJobsByDeliveryReceipt(jobs)
		require.Len(t, filtered, 3)

		levelsByID := map[string]alerts.AlertLevel{}
		for _, job := range filtered {
			require.Len(t, job.Alerts, 1)
			levelsByID[job.WebhookConfig.ID] = job.Alerts[0].Level
		}
		require.Equal(t, alerts.AlertLevelCritical, levelsByID["dest-a"])
		require.Equal(t, alerts.AlertLevelWarning, levelsByID["dest-b"])
		require.Equal(t, alerts.AlertLevelWarning, levelsByID["dest-legacy"])
		// Original input alert must not be mutated across destinations.
		require.Equal(t, alerts.AlertLevelWarning, demoted.Level)
	})

	t.Run("queue-less-in-memory-preserves-notified-level", func(t *testing.T) {
		nm := &NotificationManager{
			enabled:               true,
			notifyOnResolve:       true,
			lastNotified:          make(map[string]notificationRecord),
			deliveryReceipts:      make(map[string]struct{}),
			deliveryReceiptLevels: make(map[string]alerts.AlertLevel),
		}
		hook := WebhookConfig{ID: "mem-hook", URL: "https://example.invalid/mem", Enabled: true}
		start := time.Now().Add(-5 * time.Minute)
		firing := &alerts.Alert{ID: "mem-alert", Level: alerts.AlertLevelCritical, StartTime: start}
		firingJob := notificationDeliveryJob{Type: "webhook", Event: eventAlert, Alerts: []*alerts.Alert{firing}, WebhookConfig: &hook}
		require.NoError(t, nm.recordNotificationDeliveryReceipt(firingJob, time.Now()))

		demoted := &alerts.Alert{ID: "mem-alert", Level: alerts.AlertLevelWarning, StartTime: start}
		resolvedJob := notificationDeliveryJob{Type: "webhook", Event: eventResolved, Alerts: []*alerts.Alert{demoted}, WebhookConfig: &hook}
		filtered := nm.filterResolvedJobsByDeliveryReceipt([]notificationDeliveryJob{resolvedJob})
		require.Len(t, filtered, 1)
		require.Len(t, filtered[0].Alerts, 1)
		require.Equal(t, alerts.AlertLevelCritical, filtered[0].Alerts[0].Level)
		require.Equal(t, alerts.AlertLevelWarning, demoted.Level)

		// Recording resolved delivery clears both receipt presence and level.
		require.NoError(t, nm.recordNotificationDeliveryReceipt(filtered[0], time.Now()))
		require.Empty(t, nm.filterResolvedJobsByDeliveryReceipt([]notificationDeliveryJob{resolvedJob}))
	})
}
