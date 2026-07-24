package models

import "time"

// MetricPoint represents a single metric value at a point in time
type MetricPoint struct {
	Value     float64   `json:"value"`
	Timestamp time.Time `json:"timestamp"`
}

// IOMetrics represents I/O metrics at a point in time
type IOMetrics struct {
	DiskRead   int64                     `json:"diskRead"`
	DiskWrite  int64                     `json:"diskWrite"`
	DiskBusy   int64                     `json:"diskBusy,omitempty"`
	NetworkIn  int64                     `json:"networkIn"`
	NetworkOut int64                     `json:"networkOut"`
	Timestamp  time.Time                 `json:"timestamp"`
	Presence   IOCounterPresence         `json:"-"`
	ObservedAt IOCounterObservationTimes `json:"-"`
	// SourceUptime is an optional counter-epoch hint. A decrease proves the
	// source restarted even when a busy guest has already surpassed its old
	// counter value before the next poll.
	SourceUptime uint64 `json:"-"`
}

// IOCounterObservationTimes keeps the receipt time of each independently
// sampled counter. A zero field falls back to IOMetrics.Timestamp for legacy
// producers that obtain every counter in one response.
type IOCounterObservationTimes struct {
	DiskRead   time.Time
	DiskWrite  time.Time
	DiskBusy   time.Time
	NetworkIn  time.Time
	NetworkOut time.Time
}

// IOCounterPresence distinguishes a counter that was explicitly observed at
// zero from one that was absent in the upstream sample. Explicit=false keeps
// legacy producers compatible by treating every counter as present.
type IOCounterPresence struct {
	Explicit   bool
	DiskRead   bool
	DiskWrite  bool
	DiskBusy   bool
	NetworkIn  bool
	NetworkOut bool
}

// Effective returns the presence contract used by rate calculation.
func (p IOCounterPresence) Effective() IOCounterPresence {
	if p.Explicit {
		return p
	}
	return IOCounterPresence{
		Explicit:   true,
		DiskRead:   true,
		DiskWrite:  true,
		DiskBusy:   true,
		NetworkIn:  true,
		NetworkOut: true,
	}
}

// IORateValidity records which numeric guest rate fields represent an
// observed rate. It is intentionally excluded from JSON: legacy API and
// websocket contracts continue to expose stable numbers, while internal
// history, alerting, and unified-resource consumers can preserve unknown.
type IORateValidity struct {
	Explicit   bool
	DiskRead   bool
	DiskWrite  bool
	NetworkIn  bool
	NetworkOut bool
}

// EffectiveForRates preserves compatibility with legacy producers that did
// not carry explicit validity. Their non-zero values are usable evidence, but
// a legacy zero remains ambiguous and must not override a supplemental source
// or prove alert recovery.
func (v IORateValidity) EffectiveForRates(diskRead, diskWrite, networkIn, networkOut int64) IORateValidity {
	if v.Explicit {
		return v
	}
	return IORateValidity{
		Explicit:   true,
		DiskRead:   diskRead != 0,
		DiskWrite:  diskWrite != 0,
		NetworkIn:  networkIn != 0,
		NetworkOut: networkOut != 0,
	}
}
