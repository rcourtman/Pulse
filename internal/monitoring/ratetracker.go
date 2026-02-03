package monitoring

import (
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/types"
)

// Use IOMetrics from types package
type IOMetrics = types.IOMetrics

// RateTracker tracks I/O metrics to calculate rates
type RateTracker struct {
	mu        sync.RWMutex
	previous  map[string]IOMetrics
	lastRates map[string]RateCache
}

// RateCache stores the last calculated rates for a guest
type RateCache struct {
	DiskReadRate  float64
	DiskWriteRate float64
	NetInRate     float64
	NetOutRate    float64
}

// NewRateTracker creates a new rate tracker
func NewRateTracker() *RateTracker {
	return &RateTracker{
		previous:  make(map[string]IOMetrics),
		lastRates: make(map[string]RateCache),
	}
}

// CalculateRates calculates I/O rates for a guest
// Returns -1 for rates that don't have enough data yet (will be converted to null in JSON)
func (rt *RateTracker) CalculateRates(guestID string, current IOMetrics) (diskReadRate, diskWriteRate, netInRate, netOutRate float64) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	prev, exists := rt.previous[guestID]

	if !exists {
		// No previous data, store it and return -1 to indicate no data available
		rt.previous[guestID] = current
		return -1, -1, -1, -1
	}

	// Check if the values have actually changed (detect stale data)
	// If all cumulative values are the same, we're getting cached data from Proxmox
	if current.DiskRead == prev.DiskRead &&
		current.DiskWrite == prev.DiskWrite &&
		current.NetworkIn == prev.NetworkIn &&
		current.NetworkOut == prev.NetworkOut {
		// Data hasn't changed - return last known good rates
		if lastRate, hasRate := rt.lastRates[guestID]; hasRate {
			return lastRate.DiskReadRate, lastRate.DiskWriteRate, lastRate.NetInRate, lastRate.NetOutRate
		}
		// No last rates available, return 0
		return 0, 0, 0, 0
	}

	// Data has changed, update our cache
	rt.previous[guestID] = current

	// Calculate time difference in seconds
	timeDiff := current.Timestamp.Sub(prev.Timestamp).Seconds()
	if timeDiff <= 0 {
		// Return last known rates if time hasn't advanced
		if lastRate, hasRate := rt.lastRates[guestID]; hasRate {
			return lastRate.DiskReadRate, lastRate.DiskWriteRate, lastRate.NetInRate, lastRate.NetOutRate
		}
		return 0, 0, 0, 0
	}

	// Calculate rates (bytes per second)
	if current.DiskRead >= prev.DiskRead {
		diskReadRate = float64(current.DiskRead-prev.DiskRead) / timeDiff
	}
	if current.DiskWrite >= prev.DiskWrite {
		diskWriteRate = float64(current.DiskWrite-prev.DiskWrite) / timeDiff
	}
	if current.NetworkIn >= prev.NetworkIn {
		netInRate = float64(current.NetworkIn-prev.NetworkIn) / timeDiff
	}
	if current.NetworkOut >= prev.NetworkOut {
		netOutRate = float64(current.NetworkOut-prev.NetworkOut) / timeDiff
	}

	// Cache the calculated rates
	rt.lastRates[guestID] = RateCache{
		DiskReadRate:  diskReadRate,
		DiskWriteRate: diskWriteRate,
		NetInRate:     netInRate,
		NetOutRate:    netOutRate,
	}

	return
}

// Clear removes all tracked data
func (rt *RateTracker) Clear() {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	rt.previous = make(map[string]IOMetrics)
	rt.lastRates = make(map[string]RateCache)
}

// Cleanup removes entries for resources that haven't reported data since the cutoff time.
// This prevents unbounded memory growth when containers/VMs are deleted.
func (rt *RateTracker) Cleanup(cutoff time.Time) (removed int) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	for guestID, metrics := range rt.previous {
		if metrics.Timestamp.Before(cutoff) {
			delete(rt.previous, guestID)
			delete(rt.lastRates, guestID)
			removed++
		}
	}
	return removed
}
