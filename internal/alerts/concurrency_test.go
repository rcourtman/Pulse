package alerts

import (
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

// TestClearAlertNoLockConcurrentRecentlyResolved exercises concurrent access to recentlyResolved.
func TestClearAlertNoLockConcurrentRecentlyResolved(t *testing.T) {
	origLevel := zerolog.GlobalLevel()
	zerolog.SetGlobalLevel(zerolog.Disabled)
	t.Cleanup(func() {
		zerolog.SetGlobalLevel(origLevel)
	})

	manager := NewManager()
	t.Cleanup(func() {
		manager.Stop()
	})

	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)

	// Writer goroutine repeatedly adds and clears alerts while holding the primary lock.
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < 1000; i++ {
			manager.mu.Lock()
			alertID := "resource-cpu"
			manager.activeAlerts[alertID] = &Alert{
				ID:        alertID,
				Type:      "cpu",
				Level:     AlertLevelWarning,
				StartTime: time.Now(),
			}
			manager.clearAlertNoLock(alertID)
			manager.mu.Unlock()
		}
	}()

	// Reader goroutine continuously snapshots recently resolved alerts.
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < 1000; i++ {
			_ = manager.GetRecentlyResolved()
		}
	}()

	close(start)
	wg.Wait()
}

func TestManagerCallbacksConcurrentSetAndUse(t *testing.T) {
	origLevel := zerolog.GlobalLevel()
	zerolog.SetGlobalLevel(zerolog.Disabled)
	t.Cleanup(func() {
		zerolog.SetGlobalLevel(origLevel)
	})

	manager := NewManagerWithDataDir(t.TempDir())
	t.Cleanup(func() {
		manager.Stop()
	})

	alert := &Alert{
		ID:        "concurrency-alert",
		Type:      "cpu",
		Level:     AlertLevelWarning,
		StartTime: time.Now(),
	}

	const iterations = 500
	var wg sync.WaitGroup
	start := make(chan struct{})
	wg.Add(2)

	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < iterations; i++ {
			manager.SetAlertCallback(func(alert *Alert) {})
			manager.SetResolvedCallback(func(alertID string) {})
			manager.SetAcknowledgedCallback(func(alert *Alert, user string) {})
			manager.SetUnacknowledgedCallback(func(alert *Alert, user string) {})
			manager.SetEscalateCallback(func(alert *Alert, level int) {})
		}
	}()

	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < iterations; i++ {
			manager.safeCallResolvedCallback("concurrency-alert", false)
			manager.safeCallAcknowledgedCallback(alert, "test-user")
			manager.safeCallUnacknowledgedCallback(alert, "test-user")
			manager.safeCallEscalateCallback(alert, 1)

			manager.mu.Lock()
			_ = manager.dispatchAlert(alert, false)
			manager.mu.Unlock()
		}
	}()

	close(start)
	wg.Wait()
}

func TestManagerStopIdempotent(t *testing.T) {
	manager := NewManagerWithDataDir(t.TempDir())

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			manager.Stop()
		}()
	}
	wg.Wait()

	select {
	case <-manager.escalationStop:
	default:
		t.Fatal("escalationStop should be closed after Stop")
	}

	select {
	case <-manager.cleanupStop:
	default:
		t.Fatal("cleanupStop should be closed after Stop")
	}
}
