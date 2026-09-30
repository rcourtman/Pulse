package alerts

import (
	"fmt"
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
