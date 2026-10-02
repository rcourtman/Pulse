package alerts

import (
	"fmt"
	"reflect"
	"sync/atomic"
	"testing"
	"time"
)

// A positive threshold survives the normal configuration update. Its history
// bound must not silently make that accepted storm-control setting unreachable.
func TestFlappingConfiguredThresholdGatesDispatch(t *testing.T) {
	for _, threshold := range []int{1, 5, 10, 11, 25, 100} {
		t.Run(fmt.Sprintf("threshold_%d", threshold), func(t *testing.T) {
			m := NewManagerWithDataDir(t.TempDir())
			t.Cleanup(m.Stop)
			cfg := m.GetConfig()
			cfg.Enabled = true
			cfg.ActivationState = ActivationActive
			cfg.FlappingEnabled = true
			cfg.FlappingThreshold = threshold
			cfg.FlappingWindowSeconds = 300
			cfg.FlappingCooldownMinutes = 15
			m.UpdateConfig(cfg)
			if got := m.GetConfig().FlappingThreshold; got != threshold {
				t.Fatalf("accepted threshold = %d, want %d", got, threshold)
			}

			_, alert := testNewCanonicalAlert("vm-100", "vm-100-cpu", "vm", "cpu")
			alert.Level = AlertLevelWarning
			alert.StartTime = time.Now()
			alert.LastSeen = alert.StartTime
			m.mu.Lock()
			m.setActiveAlertNoLock(alert.ID, alert)
			m.mu.Unlock()

			delivered := 0
			m.SetAlertCallback(func(*Alert) { delivered++ })
			for attempt := 1; attempt <= threshold; attempt++ {
				m.mu.Lock()
				dispatched := m.dispatchAlert(alert, false)
				m.mu.Unlock()
				if want := attempt < threshold; dispatched != want {
					t.Fatalf("dispatch %d = %v, want %v at threshold %d", attempt, dispatched, want, threshold)
				}
			}
			if delivered != threshold-1 {
				t.Fatalf("notifications admitted = %d, want %d", delivered, threshold-1)
			}

			// Repeated attempts during the same cooldown must neither escape to a
			// destination nor grow the retained history or extend the deadline.
			key := canonicalTrackingKeyForAlert(alert)
			m.mu.Lock()
			deadline := m.suppressedUntil[key]
			lastNotified := alert.LastNotified
			for attempt := 0; attempt < 500; attempt++ {
				if m.dispatchAlert(alert, false) {
					m.mu.Unlock()
					t.Fatal("notification escaped the flapping cooldown")
				}
			}
			historySize := len(m.flappingHistory[key])
			unchanged := m.suppressedUntil[key].Equal(deadline) && alert.LastNotified == lastNotified
			m.mu.Unlock()
			if delivered != threshold-1 || historySize != threshold || !unchanged {
				t.Fatalf("cooldown: delivered=%d history=%d unchanged=%v", delivered, historySize, unchanged)
			}

			diagnosis, found := m.DiagnoseAlertDelivery(alert.ID)
			if !found || diagnosis.Reason != AlertDeliveryReasonFlapping || !diagnosis.FlappingActive ||
				diagnosis.FlappingThreshold != threshold || diagnosis.FlappingHistoryInWindow != threshold ||
				diagnosis.SuppressedUntil == nil || !diagnosis.SuppressedUntil.Equal(deadline) {
				t.Fatalf("delivery diagnosis does not explain the enforced setting: %+v", diagnosis)
			}
		})
	}
}

func TestFlappingHighThresholdPrunesExpiresAndRearms(t *testing.T) {
	m := NewManagerWithDataDir(t.TempDir())
	t.Cleanup(m.Stop)
	cfg := m.GetConfig()
	cfg.FlappingEnabled = true
	cfg.FlappingThreshold = 25
	cfg.FlappingWindowSeconds = 60
	cfg.FlappingCooldownMinutes = 15
	m.UpdateConfig(cfg)

	const key = "high-threshold-window"
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for i := 0; i < 100; i++ {
		m.flappingHistory[key] = append(m.flappingHistory[key], now.Add(-2*time.Minute))
	}
	for i := 0; i < 23; i++ {
		m.flappingHistory[key] = append(m.flappingHistory[key], now.Add(-time.Second))
	}
	suppressed, transitioned := m.checkFlappingLocked(key)
	if suppressed || transitioned || len(m.flappingHistory[key]) != 24 {
		t.Fatalf("old changes counted or current changes lost: suppressed=%v transitioned=%v history=%d", suppressed, transitioned, len(m.flappingHistory[key]))
	}
	suppressed, transitioned = m.checkFlappingLocked(key)
	if !suppressed || !transitioned || len(m.flappingHistory[key]) != 25 {
		t.Fatalf("25th current change did not arm cooldown: suppressed=%v transitioned=%v history=%d", suppressed, transitioned, len(m.flappingHistory[key]))
	}

	deadline := m.suppressedUntil[key]
	m.flappingHistory[key] = nil // The observation window drains before cooldown ends.
	suppressed, transitioned = m.checkFlappingLocked(key)
	if !suppressed || transitioned || !m.suppressedUntil[key].Equal(deadline) {
		t.Fatal("cooldown did not hold its original deadline after the window drained")
	}
	m.suppressedUntil[key] = now.Add(-time.Second)
	suppressed, transitioned = m.checkFlappingLocked(key)
	if suppressed || transitioned || m.flappingActive[key] || len(m.flappingHistory[key]) != 1 {
		t.Fatal("served cooldown did not release and reset the episode")
	}
	for change := 2; change <= 25; change++ {
		suppressed, transitioned = m.checkFlappingLocked(key)
		if want := change == 25; suppressed != want || transitioned != want {
			t.Fatalf("new episode change %d: suppressed=%v transitioned=%v", change, suppressed, transitioned)
		}
	}
}

func TestFlappingLoweredThresholdBoundsRetainedHistory(t *testing.T) {
	m := NewManagerWithDataDir(t.TempDir())
	t.Cleanup(m.Stop)
	cfg := m.GetConfig()
	cfg.FlappingEnabled = true
	cfg.FlappingThreshold = 100
	m.UpdateConfig(cfg)

	const key = "threshold-edited"
	m.mu.Lock()
	for change := 0; change < 30; change++ {
		if suppressed, _ := m.checkFlappingLocked(key); suppressed {
			m.mu.Unlock()
			t.Fatal("notification suppressed below the original threshold")
		}
	}
	m.mu.Unlock()
	cfg.FlappingThreshold = 12
	m.UpdateConfig(cfg)

	m.mu.Lock()
	suppressed, transitioned := m.checkFlappingLocked(key)
	historySize := len(m.flappingHistory[key])
	m.mu.Unlock()
	if !suppressed || !transitioned || historySize != 12 {
		t.Fatalf("lowered threshold not enforced with bounded history: suppressed=%v transitioned=%v history=%d", suppressed, transitioned, historySize)
	}
}

// Model elapsed cooldown time, but exercise the real cleanup, dispatch,
// one-shot callback and diagnosis paths. A sweep must not turn a timed hold
// into an indefinitely latched episode, whether its observation window drained
// or still contains the previous burst.
func TestFlappingCleanupReleasesAndRearmsDelivery(t *testing.T) {
	for _, sweep := range []struct {
		name string
		run  func(*Manager)
	}{
		{"ordinary", func(m *Manager) { m.Cleanup(time.Hour) }},
		{"hourly", (*Manager).cleanupStaleMaps},
	} {
		for _, window := range []int{60, 3600} {
			t.Run(fmt.Sprintf("%s/window_%d", sweep.name, window), func(t *testing.T) {
				m := NewManagerWithDataDir(t.TempDir())
				t.Cleanup(m.Stop)
				cfg := m.GetConfig()
				cfg.Enabled = true
				cfg.ActivationState = ActivationActive
				cfg.AutoAcknowledgeAfterHours = 0
				cfg.Schedule.Cooldown = 0
				cfg.Schedule.QuietHours.Enabled = false
				cfg.FlappingEnabled = true
				cfg.FlappingThreshold = 3
				cfg.FlappingWindowSeconds = window
				cfg.FlappingCooldownMinutes = 15
				m.UpdateConfig(cfg)

				_, alert := testNewCanonicalAlert("vm-cleanup", "vm-cleanup-cpu", "vm", "cpu")
				alert.Level = AlertLevelWarning
				alert.StartTime = time.Now()
				alert.LastSeen = alert.StartTime
				m.mu.Lock()
				m.setActiveAlertNoLock(alert.ID, alert)
				m.mu.Unlock()
				key := canonicalTrackingKeyForAlert(alert)
				var deliveries atomic.Int32
				m.SetAlertCallback(func(*Alert) { deliveries.Add(1) })
				transitions := make(chan string, 4)
				m.SetFlappingDetectedCallback(func(a *Alert, trackingKey string) {
					// Re-enter the manager: the one-shot callback must remain outside its lock.
					active := m.GetActiveAlerts()
					if len(active) != 1 || a.ID != alert.ID || trackingKey != key {
						t.Error("flapping callback lost the continuing occurrence identity")
					}
					transitions <- trackingKey
				})
				dispatch := func() bool {
					m.mu.Lock()
					defer m.mu.Unlock()
					return m.dispatchAlert(alert, false)
				}
				receiveTransition := func() {
					t.Helper()
					select {
					case <-transitions:
					case <-time.After(2 * time.Second):
						t.Error("new flapping episode did not notify its one-shot callback")
					}
				}
				for attempt := 1; attempt <= cfg.FlappingThreshold; attempt++ {
					if got, want := dispatch(), attempt < cfg.FlappingThreshold; got != want {
						t.Fatalf("initial dispatch %d = %v, want %v", attempt, got, want)
					}
				}
				receiveTransition()

				m.mu.Lock()
				for i := range m.flappingHistory[key] {
					m.flappingHistory[key][i] = m.flappingHistory[key][i].Add(-16 * time.Minute)
				}
				m.suppressedUntil[key] = time.Now().Add(-time.Second)
				m.mu.Unlock()
				sweep.run(m)
				diagnosis, found := m.DiagnoseAlertDelivery(alert.ID)
				if !found || diagnosis.FlappingActive || diagnosis.FlappingHistoryInWindow != 0 ||
					diagnosis.SuppressedUntil != nil || diagnosis.Reason != AlertDeliveryReasonCooldown {
					t.Errorf("served episode not retired coherently: %+v", diagnosis)
				}
				for attempt := 1; attempt <= cfg.FlappingThreshold; attempt++ {
					if got, want := dispatch(), attempt < cfg.FlappingThreshold; got != want {
						t.Errorf("post-cleanup dispatch %d = %v, want %v", attempt, got, want)
					}
				}
				receiveTransition()
				diagnosis, _ = m.DiagnoseAlertDelivery(alert.ID)
				if diagnosis.Reason != AlertDeliveryReasonFlapping || !diagnosis.FlappingActive ||
					diagnosis.SuppressedUntil == nil || !diagnosis.SuppressedUntil.After(time.Now()) {
					t.Errorf("new storm has no bounded cooldown: %+v", diagnosis)
				}
				deadline := diagnosis.SuppressedUntil
				for range 20 {
					sweep.run(m)
					if dispatch() {
						t.Error("cleanup released an unexpired cooldown")
					}
				}
				final, _ := m.DiagnoseAlertDelivery(alert.ID)
				if !reflect.DeepEqual(final.SuppressedUntil, deadline) || final.FlappingHistoryInWindow != 3 ||
					deliveries.Load() != 4 || len(transitions) != 0 {
					t.Errorf("cooldown changed during cleanup: before=%+v after=%+v deliveries=%d extra callbacks=%d",
						diagnosis, final, deliveries.Load(), len(transitions))
				}
				active := m.GetActiveAlerts()
				if len(active) != 1 || active[0].ID != alert.ID || !active[0].StartTime.Equal(alert.StartTime) || active[0].Acknowledged {
					t.Fatal("suppression expiry changed or removed the still-active occurrence")
				}
				t.Logf("two bounded episodes: %d dispatch callbacks; both cleanup and delivery diagnosis preserve occurrence %s", deliveries.Load(), alert.ID)
			})
		}
	}
}

func TestFlappingCleanupKeepsUnexpiredAndOtherKeys(t *testing.T) {
	for _, sweep := range []struct {
		name string
		run  func(*Manager)
	}{
		{"ordinary", func(m *Manager) { m.Cleanup(time.Hour) }},
		{"hourly", (*Manager).cleanupStaleMaps},
	} {
		t.Run(sweep.name, func(t *testing.T) {
			m := NewManagerWithDataDir(t.TempDir())
			t.Cleanup(m.Stop)
			now := time.Now()
			m.mu.Lock()
			m.config.AutoAcknowledgeAfterHours = 0
			for _, key := range []string{"expired-episode", "ongoing-episode", "ordinary-suppression", "pending-burst"} {
				m.activeAlerts[key] = &Alert{ID: key, StartTime: now, LastSeen: now}
				m.flappingHistory[key] = []time.Time{now}
			}
			m.flappingActive["expired-episode"] = true
			m.flappingActive["ongoing-episode"] = true
			m.suppressedUntil["expired-episode"] = now.Add(-time.Second)
			m.suppressedUntil["ongoing-episode"] = now.Add(time.Hour)
			m.suppressedUntil["ordinary-suppression"] = now.Add(time.Hour)
			m.mu.Unlock()
			sweep.run(m)
			m.mu.RLock()
			defer m.mu.RUnlock()
			if m.flappingActive["expired-episode"] || len(m.flappingHistory["expired-episode"]) != 0 {
				t.Error("expired suppression kept its episode")
			}
			if _, exists := m.suppressedUntil["expired-episode"]; exists {
				t.Error("expired deadline retained")
			}
			if !m.flappingActive["ongoing-episode"] || !m.suppressedUntil["ongoing-episode"].Equal(now.Add(time.Hour)) ||
				!m.suppressedUntil["ordinary-suppression"].Equal(now.Add(time.Hour)) || m.flappingActive["ordinary-suppression"] {
				t.Error("cleanup altered another key's unexpired policy")
			}
			for _, key := range []string{"ongoing-episode", "ordinary-suppression", "pending-burst"} {
				if !reflect.DeepEqual(m.flappingHistory[key], []time.Time{now}) {
					t.Errorf("cleanup altered %s observations", key)
				}
			}
		})
	}
}
