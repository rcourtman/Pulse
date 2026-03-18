package monitoring

import (
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// IOMetrics is an alias for models.IOMetrics
type IOMetrics = models.IOMetrics

// rateWindowSize is the number of counter samples retained per guest.
// Rate is computed from the oldest to the newest sample, giving an average
// over (rateWindowSize-1) polling intervals. With a 10s poll interval and
// window size 4, this produces a 30-second sliding window — the same approach
// Prometheus rate() uses to smooth out per-interval counter jitter.
const rateWindowSize = 4

// counterRing is a fixed-size ring buffer of IOMetrics samples.
type counterRing struct {
	entries [rateWindowSize]IOMetrics
	count   int // number of entries stored (up to rateWindowSize)
	head    int // next write position
}

func (r *counterRing) add(m IOMetrics) {
	r.entries[r.head] = m
	r.head = (r.head + 1) % rateWindowSize
	if r.count < rateWindowSize {
		r.count++
	}
}

func (r *counterRing) oldest() IOMetrics {
	if r.count < rateWindowSize {
		return r.entries[0]
	}
	return r.entries[r.head] // head points to the oldest when full
}

func (r *counterRing) newest() IOMetrics {
	return r.entries[(r.head-1+rateWindowSize)%rateWindowSize]
}

// newestTimestamp returns the timestamp of the most recent entry.
func (r *counterRing) newestTimestamp() time.Time {
	return r.newest().Timestamp
}

// RateTracker tracks I/O metrics to calculate rates
type RateTracker struct {
	mu        sync.RWMutex
	history   map[string]*counterRing
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
		history:   make(map[string]*counterRing),
		lastRates: make(map[string]RateCache),
	}
}

// CalculateRates calculates I/O rates for a guest
// Returns -1 for rates that don't have enough data yet (will be converted to null in JSON)
func (rt *RateTracker) CalculateRates(guestID string, current IOMetrics) (diskReadRate, diskWriteRate, netInRate, netOutRate float64) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	ring, exists := rt.history[guestID]

	if !exists {
		// No previous data, store it and return -1 to indicate no data available
		ring = &counterRing{}
		ring.add(current)
		rt.history[guestID] = ring
		return -1, -1, -1, -1
	}

	prev := ring.newest()

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

	// Data has changed, add to ring buffer
	ring.add(current)

	// Calculate rate over the full window (oldest to current), like Prometheus rate().
	// This naturally smooths out per-interval jitter from Proxmox's lumpy counter
	// reporting by averaging over a wider time span.
	oldest := ring.oldest()
	timeDiff := current.Timestamp.Sub(oldest.Timestamp).Seconds()
	if timeDiff <= 0 {
		// Return last known rates if time hasn't advanced
		if lastRate, hasRate := rt.lastRates[guestID]; hasRate {
			return lastRate.DiskReadRate, lastRate.DiskWriteRate, lastRate.NetInRate, lastRate.NetOutRate
		}
		return 0, 0, 0, 0
	}

	// Calculate rates (bytes per second) over the window
	if current.DiskRead >= oldest.DiskRead {
		diskReadRate = float64(current.DiskRead-oldest.DiskRead) / timeDiff
	}
	if current.DiskWrite >= oldest.DiskWrite {
		diskWriteRate = float64(current.DiskWrite-oldest.DiskWrite) / timeDiff
	}
	if current.NetworkIn >= oldest.NetworkIn {
		netInRate = float64(current.NetworkIn-oldest.NetworkIn) / timeDiff
	}
	if current.NetworkOut >= oldest.NetworkOut {
		netOutRate = float64(current.NetworkOut-oldest.NetworkOut) / timeDiff
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
	rt.history = make(map[string]*counterRing)
	rt.lastRates = make(map[string]RateCache)
}

// Cleanup removes entries for resources that haven't reported data since the cutoff time.
// This prevents unbounded memory growth when containers/VMs are deleted.
func (rt *RateTracker) Cleanup(cutoff time.Time) (removed int) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	for guestID, ring := range rt.history {
		if ring.newestTimestamp().Before(cutoff) {
			delete(rt.history, guestID)
			delete(rt.lastRates, guestID)
			removed++
		}
	}
	return removed
}
