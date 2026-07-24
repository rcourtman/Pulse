package monitoring

import (
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// IOMetrics is an alias for models.IOMetrics.
type IOMetrics = models.IOMetrics

type counterBaseline struct {
	value       int64
	observedAt  time.Time
	initialized bool
}

type counterHistory struct {
	diskRead       counterBaseline
	diskWrite      counterBaseline
	diskBusy       counterBaseline
	networkIn      counterBaseline
	networkOut     counterBaseline
	lastObservedAt time.Time
	sourceUptime   uint64
}

// RateTracker converts cumulative byte counters into adjacent-sample rates.
// Each counter has an independent baseline because Proxmox may omit only part
// of an otherwise valid status payload.
type RateTracker struct {
	mu      sync.RWMutex
	history map[string]*counterHistory
}

// NewRateTracker creates a new rate tracker.
func NewRateTracker() *RateTracker {
	return &RateTracker{history: make(map[string]*counterHistory)}
}

// CalculateRates calculates disk and network rates in bytes per second.
// A negative result means the upstream counter was absent, the sample was
// out-of-order, or no earlier observation exists. A returned zero is a valid
// observed idle/reset interval.
func (rt *RateTracker) CalculateRates(guestID string, current IOMetrics) (diskReadRate, diskWriteRate, netInRate, netOutRate float64) {
	diskReadRate, diskWriteRate, _, netInRate, netOutRate = rt.calculateRates(guestID, current)
	return
}

// CalculateRatesWithBusy also calculates disk busy percent from a cumulative
// millisecond counter.
func (rt *RateTracker) CalculateRatesWithBusy(guestID string, current IOMetrics) (diskReadRate, diskWriteRate, diskBusyPct, netInRate, netOutRate float64) {
	return rt.calculateRates(guestID, current)
}

func (rt *RateTracker) calculateRates(guestID string, current IOMetrics) (diskReadRate, diskWriteRate, diskBusyPct, netInRate, netOutRate float64) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	if current.Timestamp.IsZero() {
		return -1, -1, -1, -1, -1
	}

	history := rt.history[guestID]
	if history == nil {
		history = &counterHistory{}
		rt.history[guestID] = history
	}
	if current.SourceUptime > 0 {
		if history.sourceUptime > 0 && current.SourceUptime < history.sourceUptime {
			history.resetCounterEpoch()
		}
		history.sourceUptime = current.SourceUptime
	}

	presence := current.Presence.Effective()
	diskReadRate = calculateCounterRate(&history.diskRead, current.DiskRead, observationTime(current.ObservedAt.DiskRead, current.Timestamp), presence.DiskRead)
	diskWriteRate = calculateCounterRate(&history.diskWrite, current.DiskWrite, observationTime(current.ObservedAt.DiskWrite, current.Timestamp), presence.DiskWrite)
	diskBusyRate := calculateCounterRate(&history.diskBusy, current.DiskBusy, observationTime(current.ObservedAt.DiskBusy, current.Timestamp), presence.DiskBusy)
	netInRate = calculateCounterRate(&history.networkIn, current.NetworkIn, observationTime(current.ObservedAt.NetworkIn, current.Timestamp), presence.NetworkIn)
	netOutRate = calculateCounterRate(&history.networkOut, current.NetworkOut, observationTime(current.ObservedAt.NetworkOut, current.Timestamp), presence.NetworkOut)

	if diskBusyRate < 0 {
		diskBusyPct = -1
	} else {
		// DiskBusy is cumulative busy milliseconds, so ms/s divided by ten is
		// the percentage of wall time spent busy.
		diskBusyPct = diskBusyRate / 10
		if diskBusyPct > 100 {
			diskBusyPct = 100
		}
	}

	if current.Timestamp.After(history.lastObservedAt) {
		history.lastObservedAt = current.Timestamp
	}
	return
}

func (history *counterHistory) resetCounterEpoch() {
	history.diskRead = counterBaseline{}
	history.diskWrite = counterBaseline{}
	history.diskBusy = counterBaseline{}
	history.networkIn = counterBaseline{}
	history.networkOut = counterBaseline{}
}

func observationTime(counterTime, fallback time.Time) time.Time {
	if counterTime.IsZero() {
		return fallback
	}
	return counterTime
}

func calculateCounterRate(baseline *counterBaseline, value int64, observedAt time.Time, present bool) float64 {
	if !present {
		return -1
	}
	if !baseline.initialized {
		baseline.value = value
		baseline.observedAt = observedAt
		baseline.initialized = true
		return -1
	}
	if !observedAt.After(baseline.observedAt) {
		return -1
	}

	elapsed := observedAt.Sub(baseline.observedAt).Seconds()
	previous := baseline.value
	baseline.value = value
	baseline.observedAt = observedAt

	if value < previous {
		// A guest restart, migration reconnect, or counter wrap starts a new
		// cumulative epoch. Rebase without inventing a negative or huge rate.
		return 0
	}
	return float64(value-previous) / elapsed
}

// Clear removes all tracked data.
func (rt *RateTracker) Clear() {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	rt.history = make(map[string]*counterHistory)
}

// Cleanup removes resources that have not supplied any newer sample since the
// cutoff. Idle and partial samples still refresh resource observation time.
func (rt *RateTracker) Cleanup(cutoff time.Time) (removed int) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	for guestID, history := range rt.history {
		if history.lastObservedAt.Before(cutoff) {
			delete(rt.history, guestID)
			removed++
		}
	}
	return removed
}
