package monitoring

import (
	"sync"
	
	"github.com/rcourtman/pulse-go-rewrite/internal/types"
)

// Use IOMetrics from types package
type IOMetrics = types.IOMetrics

// RateTracker tracks I/O metrics to calculate rates
type RateTracker struct {
	mu       sync.RWMutex
	previous map[string]IOMetrics
}

// NewRateTracker creates a new rate tracker
func NewRateTracker() *RateTracker {
	return &RateTracker{
		previous: make(map[string]IOMetrics),
	}
}

// CalculateRates calculates I/O rates for a guest
// Returns -1 for rates that don't have enough data yet (will be converted to null in JSON)
func (rt *RateTracker) CalculateRates(guestID string, current IOMetrics) (diskReadRate, diskWriteRate, netInRate, netOutRate float64) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	prev, exists := rt.previous[guestID]
	rt.previous[guestID] = current

	if !exists {
		// No previous data, return -1 to indicate no data available
		return -1, -1, -1, -1
	}

	// Calculate time difference in seconds
	timeDiff := current.Timestamp.Sub(prev.Timestamp).Seconds()
	if timeDiff <= 0 {
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

	return
}

// Clear removes all tracked data
func (rt *RateTracker) Clear() {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	rt.previous = make(map[string]IOMetrics)
}