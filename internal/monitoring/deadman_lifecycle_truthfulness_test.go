package monitoring

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	"github.com/stretchr/testify/require"
)

func hasActiveSystemAlert(manager *alerts.Manager, alertType string) bool {
	targetID := alerts.SystemAlertID(alertType)
	for _, alert := range manager.GetActiveAlerts() {
		if alert.ID == targetID {
			return true
		}
	}
	return false
}

func TestDeadManMonitoringRecoveryFollowedByDeliveryFailureTransitionsToDeliveryFailed(t *testing.T) {
	t.Run("fresh progress clears stall during in-flight send and reports delivery_failed on error", func(t *testing.T) {
		now := time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)
		endpoint := "https://watchdog.example.com/ping/recovery-token"
		var responseStatus int = http.StatusOK
		var observedInFlight DeadManStatus
		var captureInFlight bool

		var runtime *deadManRuntime
		runtime = newDeadManTestRuntime(t, now, deadManRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			if captureInFlight {
				observedInFlight = runtime.statusSnapshot()
			}
			return deadManResponse(responseStatus, "OK"), nil
		}))
		manager := alerts.NewManagerWithDataDir(t.TempDir(), alerts.WithoutPersistedAlertRestore())
		t.Cleanup(manager.Stop)

		// Cycle 1: monitoring is stalled; /fail signal succeeds.
		runtime.runCycle(context.Background(), func() string { return endpoint }, func() time.Time {
			return now.Add(-2 * time.Minute)
		})
		runtime.alertProjection.drain(manager)
		require.Equal(t, "monitor_stalled", runtime.statusSnapshot().State)
		require.True(t, hasActiveSystemAlert(manager, alerts.DeadManMonitoringStalledAlertType))

		// Cycle 2: monitoring recovers (fresh), but external endpoint returns HTTP 503.
		now = now.Add(time.Minute)
		runtime.now = func() time.Time { return now }
		responseStatus = http.StatusServiceUnavailable
		captureInFlight = true
		runtime.runCycle(context.Background(), func() string { return endpoint }, func() time.Time {
			return now.Add(-5 * time.Second)
		})
		runtime.alertProjection.drain(manager)

		// While the healthy recovery signal is in flight, state must already leave monitor_stalled.
		require.Equal(t, "starting", observedInFlight.State)
		require.Empty(t, observedInFlight.LastError)

		status := runtime.statusSnapshot()
		require.Equal(t, "delivery_failed", status.State)
		require.Equal(t, 1, status.ConsecutiveFailures)
		require.Contains(t, status.LastError, "HTTP 503")
		require.NotNil(t, status.LastMonitoringProgress)
		require.True(t, status.LastMonitoringProgress.Equal(now.Add(-5*time.Second)))
		require.False(t, hasActiveSystemAlert(manager, alerts.DeadManMonitoringStalledAlertType))
	})

	t.Run("stall plus delivery failure transitions to delivery_failed when monitoring recovers", func(t *testing.T) {
		now := time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)
		endpoint := "https://watchdog.example.com/ping/recovery-token-2"
		var observedInFlight DeadManStatus
		var captureInFlight bool

		var runtime *deadManRuntime
		runtime = newDeadManTestRuntime(t, now, deadManRoundTripFunc(func(_ *http.Request) (*http.Response, error) {
			if captureInFlight {
				observedInFlight = runtime.statusSnapshot()
			}
			return deadManResponse(http.StatusServiceUnavailable, "unavailable"), nil
		}))

		// Cycle 1: both stalled and failing delivery -> monitor_stalled takes precedence.
		runtime.runCycle(context.Background(), func() string { return endpoint }, func() time.Time {
			return now.Add(-2 * time.Minute)
		})
		stalled := runtime.statusSnapshot()
		require.Equal(t, "monitor_stalled", stalled.State)
		require.Equal(t, 1, stalled.ConsecutiveFailures)

		// Cycle 2: monitoring recovers while external endpoint still returns 503.
		now = now.Add(time.Minute)
		runtime.now = func() time.Time { return now }
		captureInFlight = true
		runtime.runCycle(context.Background(), func() string { return endpoint }, func() time.Time {
			return now.Add(-2 * time.Second)
		})

		require.Equal(t, "delivery_failed", observedInFlight.State)
		require.Contains(t, observedInFlight.LastError, "HTTP 503")

		status := runtime.statusSnapshot()
		require.Equal(t, "delivery_failed", status.State)
		require.Equal(t, 2, status.ConsecutiveFailures)
		require.Contains(t, status.LastError, "HTTP 503")
	})
}

func TestDeadManReplacingOrDisablingDestinationClearsPreviousDeliveryTimestamps(t *testing.T) {
	now := time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)
	endpointA := "https://watchdog.example.com/ping/endpoint-a"
	endpointB := "https://watchdog.example.com/ping/endpoint-b"

	t.Run("replacing healthy destination immediately clears stale LastSuccessAt and LastAttemptAt", func(t *testing.T) {
		var statusByPath = map[string]int{
			"/ping/endpoint-a": http.StatusOK,
			"/ping/endpoint-b": http.StatusNotFound,
		}
		runtime := newDeadManTestRuntime(t, now, deadManRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			code := statusByPath[req.URL.Path]
			return deadManResponse(code, "response"), nil
		}))
		manager := alerts.NewManagerWithDataDir(t.TempDir(), alerts.WithoutPersistedAlertRestore())
		t.Cleanup(manager.Stop)
		m := &Monitor{
			alertManager:  manager,
			configPersist: config.NewConfigPersistence(t.TempDir()),
			deadMan:       runtime,
			deadManConfig: notifications.DeadManConfig{PingURL: endpointA},
		}

		// Endpoint A succeeds.
		runtime.runCycle(context.Background(), func() string { return m.deadManConfigSnapshot().PingURL }, func() time.Time { return now })
		healthy := m.DeadManStatus()
		require.Equal(t, "healthy", healthy.State)
		require.NotNil(t, healthy.LastSuccessAt)
		require.NotNil(t, healthy.LastAttemptAt)

		// Re-saving the same endpoint keeps its LastSuccessAt.
		require.NoError(t, m.UpdateDeadManConfig(notifications.DeadManConfig{PingURL: endpointA}))
		sameSave := m.DeadManStatus()
		require.Equal(t, "healthy", sameSave.State)
		require.NotNil(t, sameSave.LastSuccessAt)
		require.True(t, sameSave.LastSuccessAt.Equal(*healthy.LastSuccessAt))

		// Replacing with Endpoint B resets status synchronously before the next cycle runs.
		require.NoError(t, m.UpdateDeadManConfig(notifications.DeadManConfig{PingURL: endpointB}))
		afterSave := m.DeadManStatus()
		require.True(t, afterSave.Configured)
		require.Equal(t, "starting", afterSave.State)
		require.Nil(t, afterSave.LastSuccessAt)
		require.Nil(t, afterSave.LastAttemptAt)
		require.Zero(t, afterSave.ConsecutiveFailures)
		require.Empty(t, afterSave.LastError)

		// Next cycle hits Endpoint B (404): must report delivery_failed with LastSuccessAt == nil.
		now = now.Add(2 * time.Minute)
		runtime.now = func() time.Time { return now }
		runtime.runCycle(context.Background(), func() string { return m.deadManConfigSnapshot().PingURL }, func() time.Time { return now })
		failedB := m.DeadManStatus()
		require.Equal(t, "delivery_failed", failedB.State)
		require.Equal(t, 1, failedB.ConsecutiveFailures)
		require.Nil(t, failedB.LastSuccessAt)
		require.NotNil(t, failedB.LastAttemptAt)
		require.Contains(t, failedB.LastError, "HTTP 404")
	})

	t.Run("disable then re-enable with broken destination does not inherit old LastSuccessAt", func(t *testing.T) {
		runtime := newDeadManTestRuntime(t, now, deadManRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path == "/ping/endpoint-a" {
				return deadManResponse(http.StatusOK, "OK"), nil
			}
			return deadManResponse(http.StatusNotFound, "missing"), nil
		}))

		runtime.runCycle(context.Background(), func() string { return endpointA }, func() time.Time { return now })
		require.NotNil(t, runtime.statusSnapshot().LastSuccessAt)

		// Disable clears runtime timestamps directly.
		runtime.runCycle(context.Background(), func() string { return "" }, func() time.Time { return now })
		disabled := runtime.statusSnapshot()
		require.Equal(t, "disabled", disabled.State)
		require.Nil(t, disabled.LastSuccessAt)
		require.Nil(t, disabled.LastAttemptAt)

		// Re-enable with Endpoint B (404).
		runtime.runCycle(context.Background(), func() string { return endpointB }, func() time.Time { return now })
		failed := runtime.statusSnapshot()
		require.Equal(t, "delivery_failed", failed.State)
		require.Nil(t, failed.LastSuccessAt)
		require.NotNil(t, failed.LastAttemptAt)
	})

	t.Run("misconfigured destination clears stale success timestamps and stall alert", func(t *testing.T) {
		runtime := newDeadManTestRuntime(t, now, deadManRoundTripFunc(func(_ *http.Request) (*http.Response, error) {
			return deadManResponse(http.StatusOK, "OK"), nil
		}))
		manager := alerts.NewManagerWithDataDir(t.TempDir(), alerts.WithoutPersistedAlertRestore())
		t.Cleanup(manager.Stop)

		runtime.runCycle(context.Background(), func() string { return endpointA }, func() time.Time { return now })
		require.NotNil(t, runtime.statusSnapshot().LastSuccessAt)

		runtime.runCycle(context.Background(), func() string { return "http://127.0.0.1/ping/local" }, func() time.Time { return now })
		runtime.alertProjection.drain(manager)
		status := runtime.statusSnapshot()
		require.Equal(t, "misconfigured", status.State)
		require.Nil(t, status.LastSuccessAt)
		require.Nil(t, status.LastAttemptAt)
		require.Zero(t, status.ConsecutiveFailures)
		require.True(t, hasActiveSystemAlert(manager, alerts.DeadManDeliveryAlertType))
		require.False(t, hasActiveSystemAlert(manager, alerts.DeadManMonitoringStalledAlertType))
	})
}

func TestDeadManSupersededOrStoppedCycleCannotClobberStateOrCleanShutdown(t *testing.T) {
	now := time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)
	endpointA := "https://watchdog.example.com/ping/stop-race-a"
	endpointB := "https://watchdog.example.com/ping/stop-race-b"

	t.Run("stop during config read prevents ensureEndpoint from clobbering clean shutdown", func(t *testing.T) {
		requests := 0
		runtime := newDeadManTestRuntime(t, now, deadManRoundTripFunc(func(_ *http.Request) (*http.Response, error) {
			requests++
			return deadManResponse(http.StatusOK, "OK"), nil
		}))
		// Establish healthy state on Endpoint A first.
		runtime.runCycle(context.Background(), func() string { return endpointA }, func() time.Time { return now })
		require.Equal(t, 1, requests)

		stoppedAt := now.Add(45 * time.Second)
		// Next cycle tries to initialize Endpoint B, but stop() lands during configURL read.
		runtime.runCycle(context.Background(), func() string {
			runtime.stop(stoppedAt, nil)
			return endpointB
		}, func() time.Time { return stoppedAt })

		require.Equal(t, 1, requests, "stopped cycle must not send HTTP request")
		stored, err := loadDeadManState(runtime.statePath)
		require.NoError(t, err)
		require.True(t, stored.StoppedAt.Equal(stoppedAt), "clean shutdown StoppedAt must survive stopped cycle")
		require.Equal(t, deadManEndpointFingerprint(endpointA), stored.EndpointFingerprint)
	})

	t.Run("stop during progress read prevents recordHealthyProgress from clearing clean shutdown", func(t *testing.T) {
		requests := 0
		runtime := newDeadManTestRuntime(t, now, deadManRoundTripFunc(func(_ *http.Request) (*http.Response, error) {
			requests++
			return deadManResponse(http.StatusOK, "OK"), nil
		}))
		runtime.runCycle(context.Background(), func() string { return endpointA }, func() time.Time { return now })
		require.Equal(t, 1, requests)

		stoppedAt := now.Add(30 * time.Second)
		runtime.runCycle(context.Background(), func() string { return endpointA }, func() time.Time {
			runtime.stop(stoppedAt, nil)
			return stoppedAt
		})

		require.Equal(t, 1, requests, "stopped cycle must not send HTTP request")
		stored, err := loadDeadManState(runtime.statePath)
		require.NoError(t, err)
		require.True(t, stored.StoppedAt.Equal(stoppedAt), "clean shutdown StoppedAt must not be cleared")
	})

	t.Run("superseded empty-config cycle cannot disable newly saved destination", func(t *testing.T) {
		runtime := newDeadManTestRuntime(t, now, deadManRoundTripFunc(func(_ *http.Request) (*http.Response, error) {
			return deadManResponse(http.StatusOK, "OK"), nil
		}))
		m := &Monitor{
			configPersist: config.NewConfigPersistence(t.TempDir()),
			deadMan:       runtime,
		}
		// Cycle reads empty endpoint, then operator saves endpointB before disable() runs.
		runtime.runCycle(context.Background(), func() string {
			require.NoError(t, m.UpdateDeadManConfig(notifications.DeadManConfig{PingURL: endpointB}))
			return ""
		}, func() time.Time { return now })

		status := m.DeadManStatus()
		require.True(t, status.Configured)
		require.Equal(t, "starting", status.State)
	})

	t.Run("config removal before setAttempt does not record attempt or send request", func(t *testing.T) {
		requests := 0
		var mu sync.Mutex
		runtime := newDeadManTestRuntime(t, now, deadManRoundTripFunc(func(_ *http.Request) (*http.Response, error) {
			mu.Lock()
			requests++
			mu.Unlock()
			return deadManResponse(http.StatusOK, "OK"), nil
		}))
		nowCalls := 0
		runtime.now = func() time.Time {
			nowCalls++
			// Call 1: start of runCycle; Call 2: argument to setAttempt just before send.
			if nowCalls == 2 {
				runtime.notifyConfigChangedFor("")
			}
			return now
		}

		runtime.runCycle(context.Background(), func() string { return endpointA }, func() time.Time { return now })

		mu.Lock()
		gotRequests := requests
		mu.Unlock()
		require.Zero(t, gotRequests)
		status := runtime.statusSnapshot()
		require.Equal(t, "disabled", status.State)
		require.Nil(t, status.LastAttemptAt)
	})

	t.Run("successful disable persistence clears prior state persistence alert", func(t *testing.T) {
		baseDir := t.TempDir()
		blocker := filepath.Join(baseDir, "alerts")
		// Block directory creation with a regular file so initial persistCurrentState fails.
		require.NoError(t, os.WriteFile(blocker, []byte("not-a-dir"), 0o600))

		runtime := newDeadManRuntime(baseDir)
		runtime.startupAt = now
		runtime.now = func() time.Time { return now }
		runtime.retryDelays = nil
		runtime.client = &http.Client{Transport: deadManRoundTripFunc(func(_ *http.Request) (*http.Response, error) {
			return deadManResponse(http.StatusOK, "OK"), nil
		})}
		manager := alerts.NewManagerWithDataDir(t.TempDir(), alerts.WithoutPersistedAlertRestore())
		t.Cleanup(manager.Stop)

		runtime.runCycle(context.Background(), func() string { return endpointA }, func() time.Time { return now })
		runtime.alertProjection.drain(manager)
		require.True(t, hasActiveSystemAlert(manager, alerts.DeadManStateAlertType))

		// Restore directory writability and disable; disable persistence succeeds and clears DeadManStateAlertType.
		require.NoError(t, os.Remove(blocker))
		runtime.runCycle(context.Background(), func() string { return "" }, func() time.Time { return now })
		runtime.alertProjection.drain(manager)
		require.False(t, hasActiveSystemAlert(manager, alerts.DeadManStateAlertType))
		require.Equal(t, "disabled", runtime.statusSnapshot().State)
	})
}

func TestDeadManReplacingDestinationClearsPriorDeliveryAlert(t *testing.T) {
	now := time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)
	endpointA := "https://watchdog.example.com/ping/failing-a"
	endpointB := "https://watchdog.example.com/ping/fresh-b"

	runtime := newDeadManTestRuntime(t, now, deadManRoundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return deadManResponse(http.StatusServiceUnavailable, "down"), nil
	}))
	manager := alerts.NewManagerWithDataDir(t.TempDir(), alerts.WithoutPersistedAlertRestore())
	t.Cleanup(manager.Stop)
	m := &Monitor{
		alertManager:  manager,
		configPersist: config.NewConfigPersistence(t.TempDir()),
		deadMan:       runtime,
		deadManConfig: notifications.DeadManConfig{PingURL: endpointA},
	}

	for i := 0; i < deadManDeliveryAlertThreshold; i++ {
		runtime.runCycle(context.Background(), func() string { return m.deadManConfigSnapshot().PingURL }, func() time.Time { return now })
		runtime.alertProjection.drain(manager)
	}
	require.True(t, hasActiveSystemAlert(manager, alerts.DeadManDeliveryAlertType))
	require.Equal(t, "delivery_failed", m.DeadManStatus().State)

	// Operator replaces broken endpointA with fresh endpointB.
	require.NoError(t, m.UpdateDeadManConfig(notifications.DeadManConfig{PingURL: endpointB}))
	runtime.alertProjection.drain(manager)

	status := m.DeadManStatus()
	require.Equal(t, "starting", status.State)
	require.Zero(t, status.ConsecutiveFailures)
	require.Empty(t, status.LastError)
	require.Nil(t, status.LastSuccessAt)
	require.False(t, hasActiveSystemAlert(manager, alerts.DeadManDeliveryAlertType),
		"replacing destination must clear previous destination's delivery failure alert")
	require.False(t, strings.Contains(status.LastError, "failing-a"))
}
