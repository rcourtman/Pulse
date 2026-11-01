package monitoring

import (
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// IOMetrics is an alias for models.IOMetrics.
type IOMetrics = models.IOMetrics

type counterBaseline[T int64 | uint64] struct {
	value       T
	observedAt  time.Time
	initialized bool
}

type counterHistory struct {
	diskRead       counterBaseline[int64]
	diskWrite      counterBaseline[int64]
	diskBusy       counterBaseline[int64]
	networkIn      counterBaseline[int64]
	networkOut     counterBaseline[int64]
	cpuTime        counterBaseline[uint64]
	lastObservedAt time.Time
	sourceUptime   uint64
	sampleClock    string
}

// counterSampleTime is when a producer sampled a set of cumulative counters,
// on a named clock whose samples the producer has already ordered. Rates
// measure intervals on that clock, while IOMetrics.Timestamp stays Pulse
// receipt time for freshness and cleanup. The zero value keeps receipt time
// for both, which pull-based collectors observe directly.
type counterSampleTime struct {
	clock string
	at    time.Time
}

// counterTime returns when a counter was sampled: the named clock's time, or
// else the counter's own receipt time with the payload receipt as fallback.
func (sampled counterSampleTime) counterTime(counterReceivedAt, receivedAt time.Time) time.Time {
	if sampled.clock != "" {
		return sampled.at
	}
	return observationTime(counterReceivedAt, receivedAt)
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
// out-of-order, no earlier observation exists, or no comparable interval
// exists on the sample's clock. A returned zero is a valid observed
// idle/reset interval.
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
	return rt.calculateSampledRates(guestID, current, counterSampleTime{})
}

// calculateSampledRates measures each counter interval on sampled's clock.
// Samples on a different clock than the baseline start a new counter epoch,
// because their times cannot be subtracted from each other.
func (rt *RateTracker) calculateSampledRates(guestID string, current IOMetrics, sampled counterSampleTime) (diskReadRate, diskWriteRate, diskBusyPct, netInRate, netOutRate float64) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	if current.Timestamp.IsZero() {
		return -1, -1, -1, -1, -1
	}
	history, sampled := rt.sampledHistory(guestID, current.Timestamp, sampled)
	if current.SourceUptime > 0 {
		if history.sourceUptime > 0 && current.SourceUptime < history.sourceUptime {
			history.resetCounterEpoch()
		}
		history.sourceUptime = current.SourceUptime
	}

	presence := current.Presence.Effective()
	ordered := sampled.clock != ""
	diskReadRate = sampledCounterRate(&history.diskRead, current.DiskRead, sampled.counterTime(current.ObservedAt.DiskRead, current.Timestamp), presence.DiskRead, ordered)
	diskWriteRate = sampledCounterRate(&history.diskWrite, current.DiskWrite, sampled.counterTime(current.ObservedAt.DiskWrite, current.Timestamp), presence.DiskWrite, ordered)
	diskBusyRate := sampledCounterRate(&history.diskBusy, current.DiskBusy, sampled.counterTime(current.ObservedAt.DiskBusy, current.Timestamp), presence.DiskBusy, ordered)
	netInRate = sampledCounterRate(&history.networkIn, current.NetworkIn, sampled.counterTime(current.ObservedAt.NetworkIn, current.Timestamp), presence.NetworkIn, ordered)
	netOutRate = sampledCounterRate(&history.networkOut, current.NetworkOut, sampled.counterTime(current.ObservedAt.NetworkOut, current.Timestamp), presence.NetworkOut, ordered)

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

	return
}

// sampledHistory applies clock epochs and receipt-time cleanup to every
// counter series. The caller holds rt.mu and supplies a nonzero receipt time.
func (rt *RateTracker) sampledHistory(seriesID string, receivedAt time.Time, sampled counterSampleTime) (*counterHistory, counterSampleTime) {
	if sampled.at.IsZero() {
		sampled = counterSampleTime{}
	}
	history := rt.history[seriesID]
	if history == nil {
		history = &counterHistory{}
		rt.history[seriesID] = history
	}
	if sampled.clock != history.sampleClock {
		history.resetCounterEpoch()
		history.sampleClock = sampled.clock
	}
	if receivedAt.After(history.lastObservedAt) {
		history.lastObservedAt = receivedAt
	}
	return history, sampled
}

// calculateSampledCounterRate measures one cumulative counter outside the disk
// and network set, such as a libvirt domain's CPU time, under the same clock,
// baseline and cleanup rules as calculateSampledRates. Preserve its unsigned
// range: cumulative CPU nanoseconds can exceed the signed byte-counter range.
func (rt *RateTracker) calculateSampledCounterRate(seriesID string, value uint64, receivedAt time.Time, sampled counterSampleTime) float64 {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if receivedAt.IsZero() {
		return -1
	}
	history, sampled := rt.sampledHistory(seriesID, receivedAt, sampled)
	return sampledCounterRate(&history.cpuTime, value, sampled.counterTime(time.Time{}, receivedAt), true, sampled.clock != "")
}

func (history *counterHistory) resetCounterEpoch() {
	history.diskRead = counterBaseline[int64]{}
	history.diskWrite = counterBaseline[int64]{}
	history.diskBusy = counterBaseline[int64]{}
	history.networkIn = counterBaseline[int64]{}
	history.networkOut = counterBaseline[int64]{}
	history.cpuTime = counterBaseline[uint64]{}
}

func observationTime(counterTime, fallback time.Time) time.Time {
	if counterTime.IsZero() {
		return fallback
	}
	return counterTime
}

func calculateCounterRate[T int64 | uint64](baseline *counterBaseline[T], value T, observedAt time.Time, present bool) float64 {
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

// sampledCounterRate is calculateCounterRate for a sample that may sit on an
// ordered clock. There a time that does not advance past the baseline is a
// clock step or a malformed time, not a late sample, so neither sample bounds
// a known interval: the baseline is dropped, this sample is unknown, and the
// next sample starts a new baseline.
func sampledCounterRate[T int64 | uint64](baseline *counterBaseline[T], value T, observedAt time.Time, present, ordered bool) float64 {
	if ordered && present && baseline.initialized && !observedAt.After(baseline.observedAt) {
		*baseline = counterBaseline[T]{}
		return -1
	}
	return calculateCounterRate(baseline, value, observedAt, present)
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
