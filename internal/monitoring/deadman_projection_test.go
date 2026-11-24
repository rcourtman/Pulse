package monitoring

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	"github.com/stretchr/testify/require"
)

type deadManStandingFixtureResult struct {
	Status            DeadManStatus  `json:"status"`
	History           []alerts.Alert `json:"history"`
	Active            []alerts.Alert `json:"active"`
	FiringCallbacks   int64          `json:"firingCallbacks"`
	DeliveryCallbacks int64          `json:"deliveryCallbacks"`
	WebhookDeliveries int64          `json:"webhookDeliveries"`
}

func TestDeadManStandingAlertAdmission(t *testing.T) {
	for _, tc := range []struct {
		name, state    string
		wantStallCount int
	}{
		{"recovered", "healthy", 0},
		{"removed", "disabled", 0},
		{"still-stalled", "monitor_stalled", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := exerciseDeadManStandingAlert(t, tc.name)
			const stall = alerts.DeadManMonitoringStalledAlertType
			stallCount, restartCount := 0, 0
			for _, entry := range got.History {
				switch entry.Type {
				case stall:
					stallCount++
				case alerts.DeadManInterruptionAlertType:
					restartCount++
				}
			}
			require.NotNil(t, got.Status.LastInterruption)
			require.EqualValues(t, 300, got.Status.LastInterruption.DurationSecs)
			require.Equal(t, 1, restartCount, "real restart history")
			require.Equal(t, tc.state, got.Status.State)
			t.Logf("state=%s stall_history=%d restart_history=%d firing_callbacks=%d delivery_callbacks=%d webhook_receipts=%d",
				got.Status.State, stallCount, restartCount, got.FiringCallbacks, got.DeliveryCallbacks, got.WebhookDeliveries)
			// Stop() has joined both callback types. Final active state alone
			// would miss the obsolete occurrence and its notification.
			asserted := []int64{int64(stallCount), got.FiringCallbacks, got.DeliveryCallbacks, got.WebhookDeliveries}
			require.Equal(t, []int64{int64(tc.wantStallCount), int64(tc.wantStallCount), int64(tc.wantStallCount), int64(tc.wantStallCount)},
				asserted, "stall history, firing callback, delivery callback, webhook receipt")
			require.Len(t, got.Active, tc.wantStallCount)
		})
	}
}

func exerciseDeadManStandingAlert(t *testing.T, settle string) deadManStandingFixtureResult {
	t.Helper()
	start := time.Now().UTC().Truncate(time.Second)
	const endpoint = "https://watchdog.example.com/ping/fixture"
	const stall = alerts.DeadManMonitoringStalledAlertType

	type signal struct{ method, path, body string }
	signals := make(chan signal, 4)
	receiver := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		signals <- signal{r.Method, r.URL.Path, string(body)}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(receiver.Close)
	fixtureURL, err := url.Parse(receiver.URL)
	require.NoError(t, err)

	var webhookDeliveries, firingCallbacks, deliveryCallbacks atomic.Int64
	webhook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct{ Type string }
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.Type != stall {
			t.Errorf("ordinary webhook must carry the stalled alert, type=%q err=%v", payload.Type, err)
		}
		webhookDeliveries.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(webhook.Close)
	nm := notifications.NewNotificationManagerWithDeferredQueue("", t.TempDir())
	t.Cleanup(nm.Stop)
	require.NoError(t, nm.UpdateAllowedPrivateCIDRs("127.0.0.1/32"))
	nm.SetGroupingConfig(false, 0, false, false)
	nm.AddWebhook(notifications.WebhookConfig{ID: "fixture", Name: "Fixture", Enabled: true, URL: webhook.URL,
		Service: "generic", Template: `{"type":"{{.Type}}"}`})

	am := alerts.NewManagerWithDataDir(t.TempDir(), alerts.WithoutPersistedAlertRestore())
	t.Cleanup(am.Stop)
	am.UpdateConfig(alerts.AlertConfig{Enabled: true, ActivationState: alerts.ActivationActive})
	am.SubscribeAlertFiringCallback(func(a *alerts.Alert) {
		if a.Type == stall {
			firingCallbacks.Add(1)
		}
	})
	am.SetAlertCallback(func(a *alerts.Alert) {
		if a.Type == stall {
			deliveryCallbacks.Add(1)
			nm.SendAlert(a)
		}
	})
	dir := t.TempDir()
	require.NoError(t, persistDeadManState(dir+"/alerts/deadman-state.json", deadManPersistedState{
		SchemaVersion: deadManStateSchemaVersion, EndpointFingerprint: deadManEndpointFingerprint(endpoint),
		Enabled: true, StartedAt: start.Add(-time.Hour), LastHealthyAt: start.Add(-5 * time.Minute),
	}))
	d := newDeadManRuntime(dir)
	d.startupAt = start
	d.retryDelays = nil
	d.client = &http.Client{Timeout: time.Second, Transport: deadManRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		// Map only the synthetic authority to the test receiver. Production
		// URL validation, host separation and transport settings are untouched.
		if r.URL.Host != "watchdog.example.com" || r.URL.Scheme != "https" {
			t.Errorf("unexpected authority: %s", r.URL.Host)
			return deadManResponse(http.StatusBadRequest, ""), nil
		}
		request := r.Clone(r.Context())
		request.Host, request.URL.Host = "", fixtureURL.Host
		return receiver.Client().Transport.RoundTrip(request)
	})}
	persistence := config.NewConfigPersistence(t.TempDir())
	initialConfig := notifications.DeadManConfig{PingURL: endpoint}
	require.NoError(t, persistence.SaveDeadManConfig(initialConfig))
	m := &Monitor{deadMan: d, alertManager: am, configPersist: persistence, deadManConfig: initialConfig}
	cycle := func(at, progress time.Time, path, body string) {
		t.Helper()
		d.now = func() time.Time { return at }
		cycleDone := make(chan struct{})
		go func() {
			defer close(cycleDone)
			d.runCycle(context.Background(), func() string { return m.deadManConfigSnapshot().PingURL }, func() time.Time { return progress })
		}()
		select {
		case <-cycleDone:
		case <-time.After(5 * time.Second):
			t.Fatal("watchdog cycle did not settle independently of alert projection")
		}
		select {
		case got := <-signals:
			require.Equal(t, path, got.path)
			if body == "" {
				require.Equal(t, http.MethodGet, got.method)
				require.Empty(t, got.body)
			} else {
				require.Equal(t, http.MethodPost, got.method)
				require.Contains(t, got.body, body)
			}
		default:
			t.Fatalf("watchdog signal not received: %s", path)
		}
	}
	cycle(start, start, "/ping/fixture", "previous unexpected shutdown")
	// Finish startup, so the next item to enter the manager is the stall,
	// not a restart observation or an initial clear.
	d.alertProjection.drain(am)

	unblock := holdDeadManAlertManager(t, am)
	projected := make(chan struct{})
	go func() {
		defer close(projected)
		d.alertProjection.run(am)
	}()
	finish := func() {
		d.alertProjection.close()
		unblock()
		<-projected
	}
	t.Cleanup(finish)
	cycle(start.Add(time.Minute), start.Add(-2*time.Minute), "/ping/fixture/fail", "monitoring loop has stopped")
	// No superseding updates have happened. Since the stall was queued
	// before the fixture got /fail, its absence proves dequeue, while the
	// real manager lock prevents any manager-side admission.
	require.Eventually(t, func() bool {
		p := &d.alertProjection
		p.mu.Lock()
		defer p.mu.Unlock()
		return !slices.ContainsFunc(p.pending, func(update deadManAlertUpdate) bool {
			return update.alertType == stall && update.raise != nil
		})
	}, 3*time.Second, time.Millisecond, "standing raise did not leave the projection queue")
	switch settle {
	case "recovered":
		recovered := start.Add(2 * time.Minute)
		cycle(recovered, recovered, "/ping/fixture", "")
		require.Equal(t, "healthy", m.DeadManStatus().State)
	case "removed":
		saved := make(chan error, 1)
		go func() { saved <- m.UpdateDeadManConfig(notifications.DeadManConfig{}) }()
		select {
		case err := <-saved:
			require.NoError(t, err)
		case <-time.After(5 * time.Second):
			t.Fatal("saved removal waited for the alert manager")
		}
		cfg, err := persistence.LoadDeadManConfig()
		require.NoError(t, err)
		require.Empty(t, cfg.PingURL)
		require.Equal(t, "disabled", m.DeadManStatus().State)
		// Deliberately don't start the next watchdog cycle. The completed
		// save, rather than a later disabled tick, must revoke the raise.
	}
	finish()
	am.Stop()

	queue := nm.GetQueue()
	require.NotNil(t, queue)
	rows, err := queue.GetPending(10)
	require.NoError(t, err)
	require.Len(t, rows, int(deliveryCallbacks.Load()), "ordinary notification queue")
	for _, row := range rows {
		require.NoError(t, nm.ProcessQueuedNotification(row))
	}
	return deadManStandingFixtureResult{m.DeadManStatus(), am.GetAlertHistory(0), am.GetActiveAlerts(),
		firingCallbacks.Load(), deliveryCallbacks.Load(), webhookDeliveries.Load()}
}
