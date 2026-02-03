package monitoring

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/types"
)

func TestCalculateRates(t *testing.T) {
	tests := []struct {
		name              string
		guestID           string
		previous          map[string]types.IOMetrics
		lastRates         map[string]RateCache
		current           types.IOMetrics
		wantDiskReadRate  float64
		wantDiskWriteRate float64
		wantNetInRate     float64
		wantNetOutRate    float64
	}{
		{
			name:              "first call for guest returns -1 for all rates",
			guestID:           "vm-100",
			previous:          map[string]types.IOMetrics{},
			lastRates:         map[string]RateCache{},
			current:           types.IOMetrics{DiskRead: 1000, DiskWrite: 2000, NetworkIn: 3000, NetworkOut: 4000, Timestamp: time.Now()},
			wantDiskReadRate:  -1,
			wantDiskWriteRate: -1,
			wantNetInRate:     -1,
			wantNetOutRate:    -1,
		},
		{
			name:    "stale data with cached rates returns cached rates",
			guestID: "vm-101",
			previous: map[string]types.IOMetrics{
				"vm-101": {DiskRead: 1000, DiskWrite: 2000, NetworkIn: 3000, NetworkOut: 4000, Timestamp: time.Now().Add(-5 * time.Second)},
			},
			lastRates: map[string]RateCache{
				"vm-101": {DiskReadRate: 100, DiskWriteRate: 200, NetInRate: 300, NetOutRate: 400},
			},
			current:           types.IOMetrics{DiskRead: 1000, DiskWrite: 2000, NetworkIn: 3000, NetworkOut: 4000, Timestamp: time.Now()},
			wantDiskReadRate:  100,
			wantDiskWriteRate: 200,
			wantNetInRate:     300,
			wantNetOutRate:    400,
		},
		{
			name:    "stale data without cached rates returns zeros",
			guestID: "vm-102",
			previous: map[string]types.IOMetrics{
				"vm-102": {DiskRead: 1000, DiskWrite: 2000, NetworkIn: 3000, NetworkOut: 4000, Timestamp: time.Now().Add(-5 * time.Second)},
			},
			lastRates:         map[string]RateCache{},
			current:           types.IOMetrics{DiskRead: 1000, DiskWrite: 2000, NetworkIn: 3000, NetworkOut: 4000, Timestamp: time.Now()},
			wantDiskReadRate:  0,
			wantDiskWriteRate: 0,
			wantNetInRate:     0,
			wantNetOutRate:    0,
		},
		{
			name:    "zero time difference with cached rates returns cached rates",
			guestID: "vm-103",
			previous: map[string]types.IOMetrics{
				"vm-103": {DiskRead: 1000, DiskWrite: 2000, NetworkIn: 3000, NetworkOut: 4000, Timestamp: time.Unix(1000, 0)},
			},
			lastRates: map[string]RateCache{
				"vm-103": {DiskReadRate: 50, DiskWriteRate: 100, NetInRate: 150, NetOutRate: 200},
			},
			current:           types.IOMetrics{DiskRead: 2000, DiskWrite: 3000, NetworkIn: 4000, NetworkOut: 5000, Timestamp: time.Unix(1000, 0)},
			wantDiskReadRate:  50,
			wantDiskWriteRate: 100,
			wantNetInRate:     150,
			wantNetOutRate:    200,
		},
		{
			name:    "zero time difference without cached rates returns zeros",
			guestID: "vm-104",
			previous: map[string]types.IOMetrics{
				"vm-104": {DiskRead: 1000, DiskWrite: 2000, NetworkIn: 3000, NetworkOut: 4000, Timestamp: time.Unix(1000, 0)},
			},
			lastRates:         map[string]RateCache{},
			current:           types.IOMetrics{DiskRead: 2000, DiskWrite: 3000, NetworkIn: 4000, NetworkOut: 5000, Timestamp: time.Unix(1000, 0)},
			wantDiskReadRate:  0,
			wantDiskWriteRate: 0,
			wantNetInRate:     0,
			wantNetOutRate:    0,
		},
		{
			name:    "negative time difference with cached rates returns cached rates",
			guestID: "vm-105",
			previous: map[string]types.IOMetrics{
				"vm-105": {DiskRead: 1000, DiskWrite: 2000, NetworkIn: 3000, NetworkOut: 4000, Timestamp: time.Unix(2000, 0)},
			},
			lastRates: map[string]RateCache{
				"vm-105": {DiskReadRate: 75, DiskWriteRate: 125, NetInRate: 175, NetOutRate: 225},
			},
			current:           types.IOMetrics{DiskRead: 2000, DiskWrite: 3000, NetworkIn: 4000, NetworkOut: 5000, Timestamp: time.Unix(1000, 0)},
			wantDiskReadRate:  75,
			wantDiskWriteRate: 125,
			wantNetInRate:     175,
			wantNetOutRate:    225,
		},
		{
			name:    "normal rate calculation",
			guestID: "vm-106",
			previous: map[string]types.IOMetrics{
				"vm-106": {DiskRead: 1000, DiskWrite: 2000, NetworkIn: 3000, NetworkOut: 4000, Timestamp: time.Unix(1000, 0)},
			},
			lastRates:         map[string]RateCache{},
			current:           types.IOMetrics{DiskRead: 6000, DiskWrite: 12000, NetworkIn: 18000, NetworkOut: 24000, Timestamp: time.Unix(1010, 0)},
			wantDiskReadRate:  500,  // (6000-1000)/10 = 500
			wantDiskWriteRate: 1000, // (12000-2000)/10 = 1000
			wantNetInRate:     1500, // (18000-3000)/10 = 1500
			wantNetOutRate:    2000, // (24000-4000)/10 = 2000
		},
		{
			name:    "counter rollover on disk read returns zero for that metric",
			guestID: "vm-107",
			previous: map[string]types.IOMetrics{
				"vm-107": {DiskRead: 5000, DiskWrite: 2000, NetworkIn: 3000, NetworkOut: 4000, Timestamp: time.Unix(1000, 0)},
			},
			lastRates:         map[string]RateCache{},
			current:           types.IOMetrics{DiskRead: 1000, DiskWrite: 12000, NetworkIn: 18000, NetworkOut: 24000, Timestamp: time.Unix(1010, 0)},
			wantDiskReadRate:  0,    // counter decreased, return 0
			wantDiskWriteRate: 1000, // (12000-2000)/10 = 1000
			wantNetInRate:     1500, // (18000-3000)/10 = 1500
			wantNetOutRate:    2000, // (24000-4000)/10 = 2000
		},
		{
			name:    "counter rollover on disk write returns zero for that metric",
			guestID: "vm-108",
			previous: map[string]types.IOMetrics{
				"vm-108": {DiskRead: 1000, DiskWrite: 12000, NetworkIn: 3000, NetworkOut: 4000, Timestamp: time.Unix(1000, 0)},
			},
			lastRates:         map[string]RateCache{},
			current:           types.IOMetrics{DiskRead: 6000, DiskWrite: 2000, NetworkIn: 18000, NetworkOut: 24000, Timestamp: time.Unix(1010, 0)},
			wantDiskReadRate:  500, // (6000-1000)/10 = 500
			wantDiskWriteRate: 0,   // counter decreased, return 0
			wantNetInRate:     1500,
			wantNetOutRate:    2000,
		},
		{
			name:    "counter rollover on network in returns zero for that metric",
			guestID: "vm-109",
			previous: map[string]types.IOMetrics{
				"vm-109": {DiskRead: 1000, DiskWrite: 2000, NetworkIn: 18000, NetworkOut: 4000, Timestamp: time.Unix(1000, 0)},
			},
			lastRates:         map[string]RateCache{},
			current:           types.IOMetrics{DiskRead: 6000, DiskWrite: 12000, NetworkIn: 3000, NetworkOut: 24000, Timestamp: time.Unix(1010, 0)},
			wantDiskReadRate:  500,
			wantDiskWriteRate: 1000,
			wantNetInRate:     0, // counter decreased, return 0
			wantNetOutRate:    2000,
		},
		{
			name:    "counter rollover on network out returns zero for that metric",
			guestID: "vm-110",
			previous: map[string]types.IOMetrics{
				"vm-110": {DiskRead: 1000, DiskWrite: 2000, NetworkIn: 3000, NetworkOut: 24000, Timestamp: time.Unix(1000, 0)},
			},
			lastRates:         map[string]RateCache{},
			current:           types.IOMetrics{DiskRead: 6000, DiskWrite: 12000, NetworkIn: 18000, NetworkOut: 4000, Timestamp: time.Unix(1010, 0)},
			wantDiskReadRate:  500,
			wantDiskWriteRate: 1000,
			wantNetInRate:     1500,
			wantNetOutRate:    0, // counter decreased, return 0
		},
		{
			name:    "all counters rollover returns zeros",
			guestID: "vm-111",
			previous: map[string]types.IOMetrics{
				"vm-111": {DiskRead: 6000, DiskWrite: 12000, NetworkIn: 18000, NetworkOut: 24000, Timestamp: time.Unix(1000, 0)},
			},
			lastRates:         map[string]RateCache{},
			current:           types.IOMetrics{DiskRead: 1000, DiskWrite: 2000, NetworkIn: 3000, NetworkOut: 4000, Timestamp: time.Unix(1010, 0)},
			wantDiskReadRate:  0,
			wantDiskWriteRate: 0,
			wantNetInRate:     0,
			wantNetOutRate:    0,
		},
		{
			name:    "mixed scenario: disk read stale, others changed",
			guestID: "vm-112",
			previous: map[string]types.IOMetrics{
				"vm-112": {DiskRead: 1000, DiskWrite: 2000, NetworkIn: 3000, NetworkOut: 4000, Timestamp: time.Unix(1000, 0)},
			},
			lastRates:         map[string]RateCache{},
			current:           types.IOMetrics{DiskRead: 1000, DiskWrite: 12000, NetworkIn: 18000, NetworkOut: 24000, Timestamp: time.Unix(1010, 0)},
			wantDiskReadRate:  0, // no change: (1000-1000)/10 = 0
			wantDiskWriteRate: 1000,
			wantNetInRate:     1500,
			wantNetOutRate:    2000,
		},
		{
			name:    "zero values with time elapsed",
			guestID: "vm-113",
			previous: map[string]types.IOMetrics{
				"vm-113": {DiskRead: 0, DiskWrite: 0, NetworkIn: 0, NetworkOut: 0, Timestamp: time.Unix(1000, 0)},
			},
			lastRates:         map[string]RateCache{},
			current:           types.IOMetrics{DiskRead: 5000, DiskWrite: 10000, NetworkIn: 15000, NetworkOut: 20000, Timestamp: time.Unix(1005, 0)},
			wantDiskReadRate:  1000, // 5000/5 = 1000
			wantDiskWriteRate: 2000, // 10000/5 = 2000
			wantNetInRate:     3000, // 15000/5 = 3000
			wantNetOutRate:    4000, // 20000/5 = 4000
		},
		{
			name:    "fractional time difference",
			guestID: "vm-114",
			previous: map[string]types.IOMetrics{
				"vm-114": {DiskRead: 1000, DiskWrite: 2000, NetworkIn: 3000, NetworkOut: 4000, Timestamp: time.Unix(1000, 0)},
			},
			lastRates:         map[string]RateCache{},
			current:           types.IOMetrics{DiskRead: 1500, DiskWrite: 2500, NetworkIn: 3500, NetworkOut: 4500, Timestamp: time.Unix(1000, 500000000)},
			wantDiskReadRate:  1000, // 500/0.5 = 1000
			wantDiskWriteRate: 1000, // 500/0.5 = 1000
			wantNetInRate:     1000, // 500/0.5 = 1000
			wantNetOutRate:    1000, // 500/0.5 = 1000
		},
		{
			name:    "large values",
			guestID: "vm-115",
			previous: map[string]types.IOMetrics{
				"vm-115": {DiskRead: 1000000000, DiskWrite: 2000000000, NetworkIn: 3000000000, NetworkOut: 4000000000, Timestamp: time.Unix(1000, 0)},
			},
			lastRates:         map[string]RateCache{},
			current:           types.IOMetrics{DiskRead: 1100000000, DiskWrite: 2200000000, NetworkIn: 3300000000, NetworkOut: 4400000000, Timestamp: time.Unix(1100, 0)},
			wantDiskReadRate:  1000000, // 100000000/100 = 1000000
			wantDiskWriteRate: 2000000, // 200000000/100 = 2000000
			wantNetInRate:     3000000, // 300000000/100 = 3000000
			wantNetOutRate:    4000000, // 400000000/100 = 4000000
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt := &RateTracker{
				previous:  tt.previous,
				lastRates: tt.lastRates,
			}

			gotDiskRead, gotDiskWrite, gotNetIn, gotNetOut := rt.CalculateRates(tt.guestID, tt.current)

			if gotDiskRead != tt.wantDiskReadRate {
				t.Errorf("DiskReadRate = %v, want %v", gotDiskRead, tt.wantDiskReadRate)
			}
			if gotDiskWrite != tt.wantDiskWriteRate {
				t.Errorf("DiskWriteRate = %v, want %v", gotDiskWrite, tt.wantDiskWriteRate)
			}
			if gotNetIn != tt.wantNetInRate {
				t.Errorf("NetInRate = %v, want %v", gotNetIn, tt.wantNetInRate)
			}
			if gotNetOut != tt.wantNetOutRate {
				t.Errorf("NetOutRate = %v, want %v", gotNetOut, tt.wantNetOutRate)
			}
		})
	}
}

func TestCalculateRates_MultipleGuestsTrackedIndependently(t *testing.T) {
	rt := NewRateTracker()
	baseTime := time.Unix(1000, 0)

	// First call for guest A - should return -1 for all
	metricsA1 := types.IOMetrics{
		DiskRead:   1000,
		DiskWrite:  2000,
		NetworkIn:  3000,
		NetworkOut: 4000,
		Timestamp:  baseTime,
	}
	diskReadA1, diskWriteA1, netInA1, netOutA1 := rt.CalculateRates("vm-100", metricsA1)
	if diskReadA1 != -1 || diskWriteA1 != -1 || netInA1 != -1 || netOutA1 != -1 {
		t.Errorf("first call for vm-100: got (%v, %v, %v, %v), want (-1, -1, -1, -1)",
			diskReadA1, diskWriteA1, netInA1, netOutA1)
	}

	// First call for guest B - should also return -1 for all
	metricsB1 := types.IOMetrics{
		DiskRead:   5000,
		DiskWrite:  6000,
		NetworkIn:  7000,
		NetworkOut: 8000,
		Timestamp:  baseTime,
	}
	diskReadB1, diskWriteB1, netInB1, netOutB1 := rt.CalculateRates("vm-200", metricsB1)
	if diskReadB1 != -1 || diskWriteB1 != -1 || netInB1 != -1 || netOutB1 != -1 {
		t.Errorf("first call for vm-200: got (%v, %v, %v, %v), want (-1, -1, -1, -1)",
			diskReadB1, diskWriteB1, netInB1, netOutB1)
	}

	// Second call for guest A - should calculate rates
	metricsA2 := types.IOMetrics{
		DiskRead:   11000,
		DiskWrite:  22000,
		NetworkIn:  33000,
		NetworkOut: 44000,
		Timestamp:  baseTime.Add(10 * time.Second),
	}
	diskReadA2, diskWriteA2, netInA2, netOutA2 := rt.CalculateRates("vm-100", metricsA2)
	// (11000-1000)/10 = 1000, (22000-2000)/10 = 2000, etc.
	if diskReadA2 != 1000 || diskWriteA2 != 2000 || netInA2 != 3000 || netOutA2 != 4000 {
		t.Errorf("second call for vm-100: got (%v, %v, %v, %v), want (1000, 2000, 3000, 4000)",
			diskReadA2, diskWriteA2, netInA2, netOutA2)
	}

	// Second call for guest B - should calculate different rates
	metricsB2 := types.IOMetrics{
		DiskRead:   10000,
		DiskWrite:  16000,
		NetworkIn:  22000,
		NetworkOut: 28000,
		Timestamp:  baseTime.Add(5 * time.Second),
	}
	diskReadB2, diskWriteB2, netInB2, netOutB2 := rt.CalculateRates("vm-200", metricsB2)
	// (10000-5000)/5 = 1000, (16000-6000)/5 = 2000, etc.
	if diskReadB2 != 1000 || diskWriteB2 != 2000 || netInB2 != 3000 || netOutB2 != 4000 {
		t.Errorf("second call for vm-200: got (%v, %v, %v, %v), want (1000, 2000, 3000, 4000)",
			diskReadB2, diskWriteB2, netInB2, netOutB2)
	}

	// Third call for guest A - should use new previous values, not affected by guest B
	metricsA3 := types.IOMetrics{
		DiskRead:   21000,
		DiskWrite:  42000,
		NetworkIn:  63000,
		NetworkOut: 84000,
		Timestamp:  baseTime.Add(20 * time.Second),
	}
	diskReadA3, diskWriteA3, netInA3, netOutA3 := rt.CalculateRates("vm-100", metricsA3)
	// (21000-11000)/10 = 1000, (42000-22000)/10 = 2000, etc.
	if diskReadA3 != 1000 || diskWriteA3 != 2000 || netInA3 != 3000 || netOutA3 != 4000 {
		t.Errorf("third call for vm-100: got (%v, %v, %v, %v), want (1000, 2000, 3000, 4000)",
			diskReadA3, diskWriteA3, netInA3, netOutA3)
	}
}

func TestCalculateRates_CachesRates(t *testing.T) {
	rt := NewRateTracker()
	baseTime := time.Unix(1000, 0)

	// First call
	metrics1 := types.IOMetrics{
		DiskRead:   1000,
		DiskWrite:  2000,
		NetworkIn:  3000,
		NetworkOut: 4000,
		Timestamp:  baseTime,
	}
	rt.CalculateRates("vm-100", metrics1)

	// Second call with changed values - should cache the calculated rates
	metrics2 := types.IOMetrics{
		DiskRead:   11000,
		DiskWrite:  22000,
		NetworkIn:  33000,
		NetworkOut: 44000,
		Timestamp:  baseTime.Add(10 * time.Second),
	}
	diskRead2, diskWrite2, netIn2, netOut2 := rt.CalculateRates("vm-100", metrics2)

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

	// Third call with stale data - should return cached rates
	metrics3 := types.IOMetrics{
		DiskRead:   11000, // same as metrics2
		DiskWrite:  22000,
		NetworkIn:  33000,
		NetworkOut: 44000,
		Timestamp:  baseTime.Add(15 * time.Second), // different time but same values
	}
	diskRead3, diskWrite3, netIn3, netOut3 := rt.CalculateRates("vm-100", metrics3)
	if diskRead3 != diskRead2 || diskWrite3 != diskWrite2 || netIn3 != netIn2 || netOut3 != netOut2 {
		t.Errorf("stale data call: got (%v, %v, %v, %v), want cached (%v, %v, %v, %v)",
			diskRead3, diskWrite3, netIn3, netOut3, diskRead2, diskWrite2, netIn2, netOut2)
	}
}

func TestCalculateRates_UpdatesPreviousMetrics(t *testing.T) {
	rt := NewRateTracker()
	baseTime := time.Unix(1000, 0)

	// First call
	metrics1 := types.IOMetrics{
		DiskRead:   1000,
		DiskWrite:  2000,
		NetworkIn:  3000,
		NetworkOut: 4000,
		Timestamp:  baseTime,
	}
	rt.CalculateRates("vm-100", metrics1)

	// Verify previous metrics are stored
	prev, exists := rt.previous["vm-100"]
	if !exists {
		t.Fatal("expected previous metrics to be stored for vm-100")
	}
	if prev.DiskRead != metrics1.DiskRead {
		t.Errorf("previous DiskRead = %v, want %v", prev.DiskRead, metrics1.DiskRead)
	}

	// Second call with changed values
	metrics2 := types.IOMetrics{
		DiskRead:   11000,
		DiskWrite:  22000,
		NetworkIn:  33000,
		NetworkOut: 44000,
		Timestamp:  baseTime.Add(10 * time.Second),
	}
	rt.CalculateRates("vm-100", metrics2)

	// Verify previous metrics are updated
	prev2, exists := rt.previous["vm-100"]
	if !exists {
		t.Fatal("expected previous metrics to still be stored for vm-100")
	}
	if prev2.DiskRead != metrics2.DiskRead {
		t.Errorf("updated previous DiskRead = %v, want %v", prev2.DiskRead, metrics2.DiskRead)
	}
	if prev2.DiskWrite != metrics2.DiskWrite {
		t.Errorf("updated previous DiskWrite = %v, want %v", prev2.DiskWrite, metrics2.DiskWrite)
	}
	if prev2.NetworkIn != metrics2.NetworkIn {
		t.Errorf("updated previous NetworkIn = %v, want %v", prev2.NetworkIn, metrics2.NetworkIn)
	}
	if prev2.NetworkOut != metrics2.NetworkOut {
		t.Errorf("updated previous NetworkOut = %v, want %v", prev2.NetworkOut, metrics2.NetworkOut)
	}
	if !prev2.Timestamp.Equal(metrics2.Timestamp) {
		t.Errorf("updated previous Timestamp = %v, want %v", prev2.Timestamp, metrics2.Timestamp)
	}
}

func TestCalculateRates_DoesNotUpdatePreviousOnStaleData(t *testing.T) {
	rt := NewRateTracker()
	baseTime := time.Unix(1000, 0)

	// First call
	metrics1 := types.IOMetrics{
		DiskRead:   1000,
		DiskWrite:  2000,
		NetworkIn:  3000,
		NetworkOut: 4000,
		Timestamp:  baseTime,
	}
	rt.CalculateRates("vm-100", metrics1)

	// Second call with stale data (same values, different timestamp)
	metrics2 := types.IOMetrics{
		DiskRead:   1000,                           // same
		DiskWrite:  2000,                           // same
		NetworkIn:  3000,                           // same
		NetworkOut: 4000,                           // same
		Timestamp:  baseTime.Add(10 * time.Second), // different time
	}
	rt.CalculateRates("vm-100", metrics2)

	// Previous should still be metrics1, not metrics2
	prev, exists := rt.previous["vm-100"]
	if !exists {
		t.Fatal("expected previous metrics to be stored for vm-100")
	}
	if !prev.Timestamp.Equal(metrics1.Timestamp) {
		t.Errorf("previous should not be updated on stale data: got timestamp %v, want %v", prev.Timestamp, metrics1.Timestamp)
	}
}

func TestClear(t *testing.T) {
	rt := NewRateTracker()
	baseTime := time.Unix(1000, 0)

	// Add some data
	metrics := types.IOMetrics{
		DiskRead:   1000,
		DiskWrite:  2000,
		NetworkIn:  3000,
		NetworkOut: 4000,
		Timestamp:  baseTime,
	}
	rt.CalculateRates("vm-100", metrics)
	rt.CalculateRates("vm-200", metrics)

	// Verify data exists
	if len(rt.previous) != 2 {
		t.Fatalf("expected 2 entries in previous, got %d", len(rt.previous))
	}

	// Clear
	rt.Clear()

	// Verify all data is cleared
	if len(rt.previous) != 0 {
		t.Errorf("expected previous to be empty after Clear, got %d entries", len(rt.previous))
	}
	if len(rt.lastRates) != 0 {
		t.Errorf("expected lastRates to be empty after Clear, got %d entries", len(rt.lastRates))
	}

	// Verify next call after clear returns -1 (like first call)
	diskRead, diskWrite, netIn, netOut := rt.CalculateRates("vm-100", metrics)
	if diskRead != -1 || diskWrite != -1 || netIn != -1 || netOut != -1 {
		t.Errorf("after Clear, first call should return -1s: got (%v, %v, %v, %v)", diskRead, diskWrite, netIn, netOut)
	}
}

func TestRateTrackerCleanup(t *testing.T) {
	rt := NewRateTracker()
	now := time.Now()

	// Add metrics for three guests:
	// - "active-guest" has recent data
	// - "stale-guest" has old data (will be removed)
	// - "mixed-guest" has recent data (last update is recent)

	activeMetrics := types.IOMetrics{
		DiskRead:   1000,
		DiskWrite:  2000,
		NetworkIn:  3000,
		NetworkOut: 4000,
		Timestamp:  now.Add(-1 * time.Hour), // Recent
	}

	staleMetrics := types.IOMetrics{
		DiskRead:   1000,
		DiskWrite:  2000,
		NetworkIn:  3000,
		NetworkOut: 4000,
		Timestamp:  now.Add(-48 * time.Hour), // Old
	}

	rt.CalculateRates("active-guest", activeMetrics)
	rt.CalculateRates("stale-guest", staleMetrics)

	// Verify both entries exist
	if len(rt.previous) != 2 {
		t.Fatalf("expected 2 entries in previous, got %d", len(rt.previous))
	}

	// Run cleanup with 24-hour cutoff
	cutoff := now.Add(-24 * time.Hour)
	removed := rt.Cleanup(cutoff)

	// Verify stale entry was removed
	if removed != 1 {
		t.Errorf("expected 1 entry removed, got %d", removed)
	}

	if len(rt.previous) != 1 {
		t.Errorf("expected 1 entry remaining in previous, got %d", len(rt.previous))
	}

	if _, exists := rt.previous["active-guest"]; !exists {
		t.Error("active-guest should still exist after cleanup")
	}

	if _, exists := rt.previous["stale-guest"]; exists {
		t.Error("stale-guest should be removed after cleanup")
	}
}

func TestRateTrackerCleanupEmpty(t *testing.T) {
	rt := NewRateTracker()
	cutoff := time.Now().Add(-24 * time.Hour)

	// Cleanup on empty tracker should not panic
	removed := rt.Cleanup(cutoff)

	if removed != 0 {
		t.Errorf("expected 0 entries removed from empty tracker, got %d", removed)
	}
}
