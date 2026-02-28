package licensing

import (
	"sync"
	"testing"
	"time"
)

func TestCRLCache_NewCRLCache(t *testing.T) {
	t.Run("default TTL when zero", func(t *testing.T) {
		c := NewCRLCache(0)
		if c.staleTTL != DefaultStaleTTL {
			t.Errorf("staleTTL = %v, want %v", c.staleTTL, DefaultStaleTTL)
		}
	})

	t.Run("custom TTL", func(t *testing.T) {
		c := NewCRLCache(24 * time.Hour)
		if c.staleTTL != 24*time.Hour {
			t.Errorf("staleTTL = %v, want 24h", c.staleTTL)
		}
	})

	t.Run("starts empty", func(t *testing.T) {
		c := NewCRLCache(0)
		if c.Size() != 0 {
			t.Errorf("Size() = %d, want 0", c.Size())
		}
	})

	t.Run("starts stale (never updated)", func(t *testing.T) {
		c := NewCRLCache(0)
		if !c.IsStale() {
			t.Error("new cache should be stale (never updated)")
		}
	})
}

func TestCRLCache_IsRevoked_FreshCache(t *testing.T) {
	c := NewCRLCache(1 * time.Hour)
	c.Update([]string{"lic_bad", "lic_stolen"})

	t.Run("revoked ID returns true", func(t *testing.T) {
		revoked, stale := c.IsRevoked("lic_bad")
		if !revoked {
			t.Error("expected revoked=true for lic_bad")
		}
		if stale {
			t.Error("expected stale=false after fresh update")
		}
	})

	t.Run("second revoked ID returns true", func(t *testing.T) {
		revoked, stale := c.IsRevoked("lic_stolen")
		if !revoked {
			t.Error("expected revoked=true for lic_stolen")
		}
		if stale {
			t.Error("expected stale=false")
		}
	})

	t.Run("non-revoked ID returns false", func(t *testing.T) {
		revoked, stale := c.IsRevoked("lic_good")
		if revoked {
			t.Error("expected revoked=false for lic_good")
		}
		if stale {
			t.Error("expected stale=false")
		}
	})

	t.Run("empty string ID returns false", func(t *testing.T) {
		revoked, stale := c.IsRevoked("")
		if revoked {
			t.Error("expected revoked=false for empty string")
		}
		if stale {
			t.Error("expected stale=false")
		}
	})
}

func TestCRLCache_IsRevoked_StaleCache_FailsOpen(t *testing.T) {
	// Use a very short TTL so cache goes stale quickly.
	c := NewCRLCache(1 * time.Millisecond)
	c.Update([]string{"lic_bad"})

	// Wait for staleness.
	time.Sleep(5 * time.Millisecond)

	revoked, stale := c.IsRevoked("lic_bad")
	if revoked {
		t.Error("stale cache must fail open: expected revoked=false")
	}
	if !stale {
		t.Error("expected stale=true after TTL expired")
	}
}

func TestCRLCache_IsRevoked_NeverUpdated_FailsOpen(t *testing.T) {
	c := NewCRLCache(1 * time.Hour)

	revoked, stale := c.IsRevoked("lic_anything")
	if revoked {
		t.Error("never-updated cache must fail open: expected revoked=false")
	}
	if !stale {
		t.Error("expected stale=true for never-updated cache")
	}
}

func TestCRLCache_Update(t *testing.T) {
	c := NewCRLCache(1 * time.Hour)

	t.Run("replaces entire cache on update", func(t *testing.T) {
		c.Update([]string{"lic_a", "lic_b"})
		if c.Size() != 2 {
			t.Fatalf("Size() = %d, want 2", c.Size())
		}

		// Update with a different set.
		c.Update([]string{"lic_c"})
		if c.Size() != 1 {
			t.Fatalf("Size() = %d, want 1 after replacement", c.Size())
		}

		// Old IDs should be gone.
		revoked, _ := c.IsRevoked("lic_a")
		if revoked {
			t.Error("lic_a should not be revoked after cache replacement")
		}

		// New ID should be present.
		revoked, _ = c.IsRevoked("lic_c")
		if !revoked {
			t.Error("lic_c should be revoked after update")
		}
	})

	t.Run("empty update clears cache", func(t *testing.T) {
		c.Update([]string{"lic_x"})
		if c.Size() != 1 {
			t.Fatal("precondition: cache should have 1 entry")
		}
		c.Update([]string{})
		if c.Size() != 0 {
			t.Errorf("Size() = %d, want 0 after empty update", c.Size())
		}
	})

	t.Run("nil update clears cache", func(t *testing.T) {
		c.Update([]string{"lic_y"})
		c.Update(nil)
		if c.Size() != 0 {
			t.Errorf("Size() = %d, want 0 after nil update", c.Size())
		}
	})

	t.Run("update resets staleness", func(t *testing.T) {
		staleCache := NewCRLCache(1 * time.Millisecond)

		// First update makes it fresh.
		staleCache.Update([]string{"lic_old"})
		if staleCache.IsStale() {
			t.Fatal("precondition: cache should be fresh right after update")
		}

		// Wait for TTL to expire — cache becomes stale.
		time.Sleep(5 * time.Millisecond)
		if !staleCache.IsStale() {
			t.Fatal("precondition: cache should be stale after TTL")
		}

		// Second update resets staleness.
		staleCache.Update([]string{"lic_new"})
		if staleCache.IsStale() {
			t.Error("cache should not be stale immediately after second update")
		}
	})

	t.Run("duplicate IDs in update are deduplicated", func(t *testing.T) {
		c.Update([]string{"lic_dup", "lic_dup", "lic_dup"})
		if c.Size() != 1 {
			t.Errorf("Size() = %d, want 1 (duplicates should merge)", c.Size())
		}
		revoked, _ := c.IsRevoked("lic_dup")
		if !revoked {
			t.Error("lic_dup should be revoked")
		}
	})
}

func TestCRLCache_LastUpdated(t *testing.T) {
	c := NewCRLCache(0)

	if !c.LastUpdated().IsZero() {
		t.Error("LastUpdated should be zero before any update")
	}

	before := time.Now()
	c.Update([]string{})
	after := time.Now()

	lu := c.LastUpdated()
	if lu.Before(before) || lu.After(after) {
		t.Errorf("LastUpdated = %v, want between %v and %v", lu, before, after)
	}
}

func TestCRLCache_ConcurrentAccess(t *testing.T) {
	c := NewCRLCache(1 * time.Hour)
	c.Update([]string{"lic_1", "lic_2", "lic_3"})

	var wg sync.WaitGroup
	const goroutines = 50

	// Concurrent reads.
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.IsRevoked("lic_1")
			c.IsRevoked("lic_unknown")
			c.IsStale()
			c.Size()
			c.LastUpdated()
		}()
	}

	// Concurrent writes interleaved with reads.
	for i := 0; i < goroutines/5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ids := []string{"lic_new"}
			if i%2 == 0 {
				ids = append(ids, "lic_1")
			}
			c.Update(ids)
		}(i)
	}

	wg.Wait()
	// If we reach here without race detector complaints, concurrency is safe.
}
