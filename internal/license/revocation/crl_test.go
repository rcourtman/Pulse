package revocation

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestCRLCacheIsRevoked_FreshCache(t *testing.T) {
	cache := NewCRLCache(0)
	cache.Update([]string{"license-1", "license-2"})

	revoked, stale := cache.IsRevoked("license-1")
	if !revoked || stale {
		t.Fatalf("expected (true, false), got (%t, %t)", revoked, stale)
	}

	revoked, stale = cache.IsRevoked("license-3")
	if revoked || stale {
		t.Fatalf("expected (false, false), got (%t, %t)", revoked, stale)
	}
}

func TestCRLCacheIsRevoked_StaleCache(t *testing.T) {
	cache := NewCRLCache(1 * time.Millisecond)
	cache.Update([]string{"license-1"})

	time.Sleep(5 * time.Millisecond)

	revoked, stale := cache.IsRevoked("license-1")
	if revoked || !stale {
		t.Fatalf("expected (false, true), got (%t, %t)", revoked, stale)
	}
}

func TestCRLCacheIsRevoked_NeverUpdated(t *testing.T) {
	cache := NewCRLCache(0)

	revoked, stale := cache.IsRevoked("anything")
	if revoked || !stale {
		t.Fatalf("expected (false, true), got (%t, %t)", revoked, stale)
	}
}

func TestCRLCacheUpdate(t *testing.T) {
	cache := NewCRLCache(0)

	cache.Update([]string{"a", "b"})
	if got := cache.Size(); got != 2 {
		t.Fatalf("expected size 2, got %d", got)
	}

	cache.Update([]string{"c"})
	if got := cache.Size(); got != 1 {
		t.Fatalf("expected size 1, got %d", got)
	}

	if got := cache.LastUpdated(); got.IsZero() || time.Since(got) > time.Second {
		t.Fatalf("expected recent LastUpdated, got %s", got)
	}
}

func TestCRLCacheIsStale(t *testing.T) {
	staleCache := NewCRLCache(1 * time.Millisecond)
	staleCache.Update([]string{"license-1"})
	time.Sleep(5 * time.Millisecond)

	if !staleCache.IsStale() {
		t.Fatal("expected stale cache to be stale")
	}

	freshCache := NewCRLCache(time.Hour)
	freshCache.Update([]string{"license-1"})
	if freshCache.IsStale() {
		t.Fatal("expected fresh cache to not be stale")
	}
}

func TestCRLCacheConcurrency(t *testing.T) {
	cache := NewCRLCache(time.Hour)
	cache.Update([]string{"seed"})

	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				cache.Update([]string{
					fmt.Sprintf("license-%d-%d", worker, j),
					fmt.Sprintf("license-%d", j),
				})
			}
		}(i)
	}

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				_, _ = cache.IsRevoked(fmt.Sprintf("license-%d-%d", worker, j))
				_ = cache.Size()
				_ = cache.LastUpdated()
				_ = cache.IsStale()
			}
		}(i)
	}

	wg.Wait()
}
