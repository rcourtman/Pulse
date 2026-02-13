package monitoring

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestCalculateRates_FirstCallReturnsNegativeOnes(t *testing.T) {
	rt := NewRateTracker()
	d, w, ni, no := rt.CalculateRates("vm-100", models.IOMetrics{
		DiskRead: 1000, DiskWrite: 2000, NetworkIn: 3000, NetworkOut: 4000,
		Timestamp: time.Now(),
	})
	if d != -1 || w != -1 || ni != -1 || no != -1 {
		t.Errorf("first call: got (%v, %v, %v, %v), want (-1, -1, -1, -1)", d, w, ni, no)
	}
}

func TestCalculateRates_StaleDataReturnsCachedRates(t *testing.T) {
	rt := NewRateTracker()
	base := time.Unix(1000, 0)

	// Seed
	rt.CalculateRates("vm-100", models.IOMetrics{
		DiskRead: 1000, DiskWrite: 2000, NetworkIn: 3000, NetworkOut: 4000,
		Timestamp: base,
	})
	// Establish rates
	rt.CalculateRates("vm-100", models.IOMetrics{
		DiskRead: 6000, DiskWrite: 12000, NetworkIn: 18000, NetworkOut: 24000,
		Timestamp: base.Add(10 * time.Second),
	})

	// Send stale data (same counter values, different timestamp)
	d, w, ni, no := rt.CalculateRates("vm-100", models.IOMetrics{
		DiskRead: 6000, DiskWrite: 12000, NetworkIn: 18000, NetworkOut: 24000,
		Timestamp: base.Add(20 * time.Second),
	})
	if d != 500 || w != 1000 || ni != 1500 || no != 2000 {
		t.Errorf("stale data: got (%v, %v, %v, %v), want (500, 1000, 1500, 2000)", d, w, ni, no)
	}
}

func TestCalculateRates_StaleDataWithoutCachedRatesReturnsZeros(t *testing.T) {
	rt := NewRateTracker()
	base := time.Unix(1000, 0)

	// Seed with values
	rt.CalculateRates("vm-100", models.IOMetrics{
		DiskRead: 1000, DiskWrite: 2000, NetworkIn: 3000, NetworkOut: 4000,
		Timestamp: base,
	})

	// Send identical values (stale) — no cached rates yet
	d, w, ni, no := rt.CalculateRates("vm-100", models.IOMetrics{
		DiskRead: 1000, DiskWrite: 2000, NetworkIn: 3000, NetworkOut: 4000,
		Timestamp: base.Add(10 * time.Second),
	})
	if d != 0 || w != 0 || ni != 0 || no != 0 {
		t.Errorf("stale without cache: got (%v, %v, %v, %v), want (0, 0, 0, 0)", d, w, ni, no)
	}
}

func TestCalculateRates_NormalRateCalculation(t *testing.T) {
	rt := NewRateTracker()
	base := time.Unix(1000, 0)

	// Seed
	rt.CalculateRates("vm-100", models.IOMetrics{
		DiskRead: 1000, DiskWrite: 2000, NetworkIn: 3000, NetworkOut: 4000,
		Timestamp: base,
	})

	// Second call — rate over 1 interval (ring only has 2 entries)
	d, w, ni, no := rt.CalculateRates("vm-100", models.IOMetrics{
		DiskRead: 6000, DiskWrite: 12000, NetworkIn: 18000, NetworkOut: 24000,
		Timestamp: base.Add(10 * time.Second),
	})
	if d != 500 || w != 1000 || ni != 1500 || no != 2000 {
		t.Errorf("normal rate: got (%v, %v, %v, %v), want (500, 1000, 1500, 2000)", d, w, ni, no)
	}
}

func TestCalculateRates_CounterRolloverReturnsZero(t *testing.T) {
	rt := NewRateTracker()
	base := time.Unix(1000, 0)

	rt.CalculateRates("vm-100", models.IOMetrics{
		DiskRead: 5000, DiskWrite: 2000, NetworkIn: 3000, NetworkOut: 4000,
		Timestamp: base,
	})

	// DiskRead decreased (counter rollover)
	d, w, ni, no := rt.CalculateRates("vm-100", models.IOMetrics{
		DiskRead: 1000, DiskWrite: 12000, NetworkIn: 18000, NetworkOut: 24000,
		Timestamp: base.Add(10 * time.Second),
	})
	if d != 0 {
		t.Errorf("DiskRead rollover: got %v, want 0", d)
	}
	if w != 1000 || ni != 1500 || no != 2000 {
		t.Errorf("other rates: got (%v, %v, %v), want (1000, 1500, 2000)", w, ni, no)
	}
}

func TestCalculateRates_AllCountersRollover(t *testing.T) {
	rt := NewRateTracker()
	base := time.Unix(1000, 0)

	rt.CalculateRates("vm-100", models.IOMetrics{
		DiskRead: 6000, DiskWrite: 12000, NetworkIn: 18000, NetworkOut: 24000,
		Timestamp: base,
	})

	d, w, ni, no := rt.CalculateRates("vm-100", models.IOMetrics{
		DiskRead: 1000, DiskWrite: 2000, NetworkIn: 3000, NetworkOut: 4000,
		Timestamp: base.Add(10 * time.Second),
	})
	if d != 0 || w != 0 || ni != 0 || no != 0 {
		t.Errorf("all rollover: got (%v, %v, %v, %v), want (0, 0, 0, 0)", d, w, ni, no)
	}
}

func TestCalculateRates_FractionalTimeDifference(t *testing.T) {
	rt := NewRateTracker()
	base := time.Unix(1000, 0)

	rt.CalculateRates("vm-100", models.IOMetrics{
		DiskRead: 1000, DiskWrite: 2000, NetworkIn: 3000, NetworkOut: 4000,
		Timestamp: base,
	})

	d, w, ni, no := rt.CalculateRates("vm-100", models.IOMetrics{
		DiskRead: 1500, DiskWrite: 2500, NetworkIn: 3500, NetworkOut: 4500,
		Timestamp: base.Add(500 * time.Millisecond),
	})
	// 500 / 0.5 = 1000
	if d != 1000 || w != 1000 || ni != 1000 || no != 1000 {
		t.Errorf("fractional time: got (%v, %v, %v, %v), want (1000, 1000, 1000, 1000)", d, w, ni, no)
	}
}

func TestCalculateRates_LargeValues(t *testing.T) {
	rt := NewRateTracker()
	base := time.Unix(1000, 0)

	rt.CalculateRates("vm-100", models.IOMetrics{
		DiskRead: 1000000000, DiskWrite: 2000000000, NetworkIn: 3000000000, NetworkOut: 4000000000,
		Timestamp: base,
	})

	d, w, ni, no := rt.CalculateRates("vm-100", models.IOMetrics{
		DiskRead: 1100000000, DiskWrite: 2200000000, NetworkIn: 3300000000, NetworkOut: 4400000000,
		Timestamp: base.Add(100 * time.Second),
	})
	if d != 1000000 || w != 2000000 || ni != 3000000 || no != 4000000 {
		t.Errorf("large values: got (%v, %v, %v, %v), want (1000000, 2000000, 3000000, 4000000)", d, w, ni, no)
	}
}

func TestCalculateRates_WindowSmooths(t *testing.T) {
	rt := NewRateTracker()
	base := time.Unix(1000, 0)

	// Simulate a steady 1000 bytes/sec download with Proxmox's lumpy counter updates.
	// Over 4 intervals (40 seconds), 40000 bytes should arrive.
	// But Proxmox distributes them unevenly across intervals.

	// T=0: seed
	rt.CalculateRates("vm-100", models.IOMetrics{
		NetworkIn: 0, Timestamp: base,
	})

	// T=10: normal interval (10000 bytes in 10s = 1000 B/s)
	rt.CalculateRates("vm-100", models.IOMetrics{
		NetworkIn: 10000, Timestamp: base.Add(10 * time.Second),
	})

	// T=20: short-changed interval (only 5000 bytes reported)
	rt.CalculateRates("vm-100", models.IOMetrics{
		NetworkIn: 15000, Timestamp: base.Add(20 * time.Second),
	})

	// T=30: lumpy interval (15000 bytes — makes up for the deficit + normal)
	// Without windowing, raw rate would be 15000/10 = 1500 B/s (50% spike).
	// With windowing (oldest=T=0, current=T=30), rate = 30000/30 = 1000 B/s.
	_, _, ni, _ := rt.CalculateRates("vm-100", models.IOMetrics{
		NetworkIn: 30000, Timestamp: base.Add(30 * time.Second),
	})

	if ni != 1000 {
		t.Errorf("windowed rate during lumpy interval: got %v, want 1000", ni)
	}

	// T=40: ring is now full (4 entries), oldest is T=10.
	// Rate = (40000-10000)/(40-10) = 30000/30 = 1000 B/s
	_, _, ni, _ = rt.CalculateRates("vm-100", models.IOMetrics{
		NetworkIn: 40000, Timestamp: base.Add(40 * time.Second),
	})

	if ni != 1000 {
		t.Errorf("windowed rate after ring full: got %v, want 1000", ni)
	}
}

func TestCalculateRates_MultipleGuestsTrackedIndependently(t *testing.T) {
	rt := NewRateTracker()
	baseTime := time.Unix(1000, 0)

	// First call for guest A - should return -1 for all
	diskReadA1, diskWriteA1, netInA1, netOutA1 := rt.CalculateRates("vm-100", models.IOMetrics{
		DiskRead: 1000, DiskWrite: 2000, NetworkIn: 3000, NetworkOut: 4000,
		Timestamp: baseTime,
	})
	if diskReadA1 != -1 || diskWriteA1 != -1 || netInA1 != -1 || netOutA1 != -1 {
		t.Errorf("first call for vm-100: got (%v, %v, %v, %v), want (-1, -1, -1, -1)",
			diskReadA1, diskWriteA1, netInA1, netOutA1)
	}

	// First call for guest B - should also return -1 for all
	diskReadB1, diskWriteB1, netInB1, netOutB1 := rt.CalculateRates("vm-200", models.IOMetrics{
		DiskRead: 5000, DiskWrite: 6000, NetworkIn: 7000, NetworkOut: 8000,
		Timestamp: baseTime,
	})
	if diskReadB1 != -1 || diskWriteB1 != -1 || netInB1 != -1 || netOutB1 != -1 {
		t.Errorf("first call for vm-200: got (%v, %v, %v, %v), want (-1, -1, -1, -1)",
			diskReadB1, diskWriteB1, netInB1, netOutB1)
	}

	// Second call for guest A
	diskReadA2, diskWriteA2, netInA2, netOutA2 := rt.CalculateRates("vm-100", models.IOMetrics{
		DiskRead: 11000, DiskWrite: 22000, NetworkIn: 33000, NetworkOut: 44000,
		Timestamp: baseTime.Add(10 * time.Second),
	})
	if diskReadA2 != 1000 || diskWriteA2 != 2000 || netInA2 != 3000 || netOutA2 != 4000 {
		t.Errorf("second call for vm-100: got (%v, %v, %v, %v), want (1000, 2000, 3000, 4000)",
			diskReadA2, diskWriteA2, netInA2, netOutA2)
	}

	// Second call for guest B - different rates
	diskReadB2, diskWriteB2, netInB2, netOutB2 := rt.CalculateRates("vm-200", models.IOMetrics{
		DiskRead: 10000, DiskWrite: 16000, NetworkIn: 22000, NetworkOut: 28000,
		Timestamp: baseTime.Add(5 * time.Second),
	})
	if diskReadB2 != 1000 || diskWriteB2 != 2000 || netInB2 != 3000 || netOutB2 != 4000 {
		t.Errorf("second call for vm-200: got (%v, %v, %v, %v), want (1000, 2000, 3000, 4000)",
			diskReadB2, diskWriteB2, netInB2, netOutB2)
	}
}

func TestCalculateRates_CachesRates(t *testing.T) {
	rt := NewRateTracker()
	baseTime := time.Unix(1000, 0)

	// Seed
	rt.CalculateRates("vm-100", models.IOMetrics{
		DiskRead: 1000, DiskWrite: 2000, NetworkIn: 3000, NetworkOut: 4000,
		Timestamp: baseTime,
	})

	// Calculate rates
	diskRead2, diskWrite2, netIn2, netOut2 := rt.CalculateRates("vm-100", models.IOMetrics{
		DiskRead: 11000, DiskWrite: 22000, NetworkIn: 33000, NetworkOut: 44000,
		Timestamp: baseTime.Add(10 * time.Second),
	})

	// Verify rates are cached
	cachedRates, exists := rt.lastRates["vm-100"]
	if !exists {
		t.Fatal("expected rates to be cached for vm-100")
	}
	if cachedRates.DiskReadRate != diskRead2 {
		t.Errorf("cached DiskReadRate = %v, want %v", cachedRates.DiskReadRate, diskRead2)
	}
	if cachedRates.DiskWriteRate != diskWrite2 {
		t.Errorf("cached DiskWriteRate = %v, want %v", cachedRates.DiskWriteRate, diskWrite2)
	}
	if cachedRates.NetInRate != netIn2 {
		t.Errorf("cached NetInRate = %v, want %v", cachedRates.NetInRate, netIn2)
	}
	if cachedRates.NetOutRate != netOut2 {
		t.Errorf("cached NetOutRate = %v, want %v", cachedRates.NetOutRate, netOut2)
	}

	// Stale data returns cached rates
	diskRead3, diskWrite3, netIn3, netOut3 := rt.CalculateRates("vm-100", models.IOMetrics{
		DiskRead: 11000, DiskWrite: 22000, NetworkIn: 33000, NetworkOut: 44000,
		Timestamp: baseTime.Add(15 * time.Second),
	})
	if diskRead3 != diskRead2 || diskWrite3 != diskWrite2 || netIn3 != netIn2 || netOut3 != netOut2 {
		t.Errorf("stale data call: got (%v, %v, %v, %v), want cached (%v, %v, %v, %v)",
			diskRead3, diskWrite3, netIn3, netOut3, diskRead2, diskWrite2, netIn2, netOut2)
	}
}

func TestCalculateRates_DoesNotAddStaleDataToRing(t *testing.T) {
	rt := NewRateTracker()
	base := time.Unix(1000, 0)

	// Seed
	rt.CalculateRates("vm-100", models.IOMetrics{
		DiskRead: 1000, NetworkIn: 3000, Timestamp: base,
	})

	// Real data
	rt.CalculateRates("vm-100", models.IOMetrics{
		DiskRead: 6000, NetworkIn: 18000, Timestamp: base.Add(10 * time.Second),
	})

	// Stale data — should not be added to ring
	rt.CalculateRates("vm-100", models.IOMetrics{
		DiskRead: 6000, NetworkIn: 18000, Timestamp: base.Add(20 * time.Second),
	})

	// Ring should still have 2 entries (seed + one real update)
	ring := rt.history["vm-100"]
	if ring.count != 2 {
		t.Errorf("ring count after stale data: got %d, want 2", ring.count)
	}
}

func TestClear(t *testing.T) {
	rt := NewRateTracker()
	base := time.Unix(1000, 0)

	rt.CalculateRates("vm-100", models.IOMetrics{
		DiskRead: 1000, Timestamp: base,
	})
	rt.CalculateRates("vm-200", models.IOMetrics{
		DiskRead: 1000, Timestamp: base,
	})

	if len(rt.history) != 2 {
		t.Fatalf("expected 2 entries in history, got %d", len(rt.history))
	}

	rt.Clear()

	if len(rt.history) != 0 {
		t.Errorf("expected history to be empty after Clear, got %d entries", len(rt.history))
	}
	if len(rt.lastRates) != 0 {
		t.Errorf("expected lastRates to be empty after Clear, got %d entries", len(rt.lastRates))
	}

	// After clear, first call returns -1
	d, w, ni, no := rt.CalculateRates("vm-100", models.IOMetrics{
		DiskRead: 1000, Timestamp: base,
	})
	if d != -1 || w != -1 || ni != -1 || no != -1 {
		t.Errorf("after Clear: got (%v, %v, %v, %v), want (-1, -1, -1, -1)", d, w, ni, no)
	}
}

func TestRateTrackerCleanup(t *testing.T) {
	rt := NewRateTracker()
	now := time.Now()

	rt.CalculateRates("active-guest", models.IOMetrics{
		DiskRead: 1000, Timestamp: now.Add(-1 * time.Hour),
	})
	rt.CalculateRates("stale-guest", models.IOMetrics{
		DiskRead: 1000, Timestamp: now.Add(-48 * time.Hour),
	})

	if len(rt.history) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(rt.history))
	}

	cutoff := now.Add(-24 * time.Hour)
	removed := rt.Cleanup(cutoff)

	if removed != 1 {
		t.Errorf("expected 1 entry removed, got %d", removed)
	}
	if len(rt.history) != 1 {
		t.Errorf("expected 1 entry remaining, got %d", len(rt.history))
	}
	if _, exists := rt.history["active-guest"]; !exists {
		t.Error("active-guest should still exist after cleanup")
	}
	if _, exists := rt.history["stale-guest"]; exists {
		t.Error("stale-guest should be removed after cleanup")
	}
}

func TestRateTrackerCleanupEmpty(t *testing.T) {
	rt := NewRateTracker()
	cutoff := time.Now().Add(-24 * time.Hour)

	removed := rt.Cleanup(cutoff)
	if removed != 0 {
		t.Errorf("expected 0 entries removed from empty tracker, got %d", removed)
	}
}
