package alerts

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func awaitProducerSignal(t *testing.T, name string, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for %s", name)
	}
}

func awaitCallbackClosure(t *testing.T, m *Manager) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		m.stopMu.RLock()
		closed := m.callbacksClosed
		m.stopMu.RUnlock()
		if closed {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("Stop did not close callback admission")
		}
		time.Sleep(time.Millisecond)
	}
}

// Hold the actual producer callbacks, not a sleep before an imagined send.
// Shutdown must wait at both the asynchronous and synchronous callback seams,
// without holding the manager lock the callbacks can legitimately re-enter.
func TestAlertStopDrainsProducerCallbacks(t *testing.T) {
	for _, path := range []string{"firing-async", "firing-sync", "resolved-async", "resolved-sync", "acknowledged", "unacknowledged", "escalated", "AI", "restored"} {
		t.Run(path, func(t *testing.T) {
			m := NewManagerWithDataDir(t.TempDir())
			m.UpdateConfig(AlertConfig{Enabled: true, ActivationState: ActivationActive,
				TimeThresholds: map[string]int{"node": 0}, NodeDefaults: ThresholdConfig{CPU: &HysteresisThreshold{Trigger: 80, Clear: 70}}})
			entered, release, completed := make(chan struct{}), make(chan struct{}), make(chan struct{})
			var releaseOnce sync.Once
			unblock := func() { releaseOnce.Do(func() { close(release) }) }
			// Unblock before Stop even if an assertion fails.
			t.Cleanup(func() { unblock(); m.Stop() })
			callback := func() {
				close(entered)
				<-release
				if path != "firing-sync" {
					_ = m.GetActiveAlerts()
				}
				close(completed)
			}
			a := &Alert{ID: "producer-" + path, Type: "cpu", Level: AlertLevelWarning, StartTime: time.Now()}
			switch path {
			case "firing-async", "firing-sync":
				m.SetAlertCallback(func(*Alert) { callback() })
				go func() {
					m.mu.Lock()
					m.dispatchAlert(a, path == "firing-async")
					m.mu.Unlock()
				}()
			case "resolved-async", "resolved-sync":
				m.SetResolvedAlertCallback(func(*ResolvedAlert) { callback() })
				go m.safeCallResolvedAlertCallback(&ResolvedAlert{Alert: a, ResolvedTime: time.Now()}, a.ID, path == "resolved-async")
			case "acknowledged":
				m.SetAcknowledgedCallback(func(*Alert, string) { callback() })
				m.safeCallAcknowledgedCallback(a, "fixture")
			case "unacknowledged":
				m.SetUnacknowledgedCallback(func(*Alert, string) { callback() })
				m.safeCallUnacknowledgedCallback(a, "fixture")
			case "escalated":
				m.SetEscalateCallback(func(*Alert, int) { callback() })
				m.safeCallEscalateCallback(a, 1)
			case "AI":
				m.SetAlertForAICallback(func(*Alert) { callback() })
				m.CheckNode(models.Node{ID: "fixture", Name: "fixture", Status: "online", CPU: 0.99})
			case "restored":
				// Startup's delay is owned too, and the stop signal cancels it.
				m.SetAlertCallback(func(*Alert) { t.Error("stopped startup timer dispatched") })
				m.scheduleRestoredCriticalAlert(a, m.restoredAlertEpoch.Load())
				m.Stop()
				return
			}
			awaitProducerSignal(t, "callback entry", entered)
			stopped := make(chan struct{})
			go func() { m.Stop(); close(stopped) }()
			// The synchronous firing holds mu, so Stop may be waiting on a
			// checkpoint worker instead of reaching callback closure yet.
			if path != "firing-sync" {
				awaitCallbackClosure(t, m)
			}
			select {
			case <-stopped:
				t.Fatal("Stop returned before the accepted callback completed")
			default:
			}
			unblock()
			awaitProducerSignal(t, "callback completion", completed)
			awaitProducerSignal(t, "Stop", stopped)
		})
	}
}

func TestAlertStoppedProducerDoesNotClaimDispatch(t *testing.T) {
	m := NewManagerWithDataDir(t.TempDir())
	m.UpdateConfig(AlertConfig{Enabled: true, ActivationState: ActivationActive})
	var calls atomic.Int32
	m.SetAlertCallback(func(*Alert) { calls.Add(1) })
	m.SetResolvedAlertCallback(func(*ResolvedAlert) { calls.Add(1) })
	m.SetEscalateCallback(func(*Alert, int) { calls.Add(1) })
	m.Stop()
	a := &Alert{ID: "after-stop", Type: "cpu", Level: AlertLevelWarning, StartTime: time.Now()}
	m.mu.Lock()
	accepted := m.dispatchAlert(a, true)
	m.mu.Unlock()
	if accepted || a.LastNotified != nil {
		t.Fatal("closed producer claimed a notification dispatch")
	}
	m.safeCallResolvedAlertCallback(&ResolvedAlert{Alert: a}, a.ID, false)
	m.safeCallEscalateCallback(a, 1)
	m.Stop()
	if calls.Load() != 0 {
		t.Fatal("callback started after producer shutdown")
	}
}

func TestAlertCallbackPanicReleasesProducerAdmission(t *testing.T) {
	m := NewManagerWithDataDir(t.TempDir())
	m.SetResolvedAlertCallback(func(*ResolvedAlert) { panic("synthetic callback panic") })
	m.safeCallResolvedAlertCallback(&ResolvedAlert{Alert: &Alert{ID: "panic"}}, "panic", true)
	stopped := make(chan struct{})
	go func() { m.Stop(); close(stopped) }()
	awaitProducerSignal(t, "Stop after recovered callback panic", stopped)
}
