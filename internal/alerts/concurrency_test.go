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
