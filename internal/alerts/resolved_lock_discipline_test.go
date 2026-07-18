package alerts

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// Regression for issue #1590. The canonical alert evaluation paths used to
// read and mutate recentlyResolved/resolvedAlias while holding only m.mu,
// while the broadcast and recovery paths guarded them with resolvedMutex, so
// the two lock domains did not exclude each other and Go's runtime aborted
// the process with a concurrent map access fault on flapping resources. Run
// with -race: this exercises the cooldown-reactivation lookup pattern against
// the broadcaster and the recovery writer concurrently.
func TestRecentlyResolvedConcurrentAccessIsRaceFree(t *testing.T) {
	m := NewManagerWithDataDir(t.TempDir())

	mk := func(id string) *ResolvedAlert {
		return &ResolvedAlert{
			Alert:        &Alert{ID: id, ResourceID: id, Type: "connectivity"},
			ResolvedTime: time.Now(),
		}
	}

	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(3)

	// Recovery path: writes resolved alerts under resolvedMutex.
	go func() {
		defer wg.Done()
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			m.addRecentlyResolvedUnlocked(mk(fmt.Sprintf("res-%d", i%8)))
		}
	}()

	// Broadcast path: iterates the resolved map every poll cycle.
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			m.GetRecentlyResolved()
			m.GetResolvedAlert("res-1")
		}
	}()

	// Canonical eval path: cooldown lookup and removal while holding m.mu,
	// with the resolved maps guarded by the subordinate resolvedMutex.
	go func() {
		defer wg.Done()
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			key := fmt.Sprintf("res-%d", i%8)
			m.mu.Lock()
			m.resolvedMutex.Lock()
			if resolved, ok := m.getResolvedAlertNoLock(key); ok && resolved != nil {
				m.removeResolvedAlertUnlocked(key)
			}
			m.resolvedMutex.Unlock()
			m.mu.Unlock()
		}
	}()

	time.Sleep(500 * time.Millisecond)
	close(stop)
	wg.Wait()
}
