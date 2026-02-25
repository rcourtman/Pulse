// Package licensing — host_lifecycle.go
//
// HostLifecycleTracker implements the v6 host counting rules:
//
//   - H2: A host is only counted toward the license limit after 10+ minutes
//     of stable heartbeat (prevents transient connections from consuming slots).
//   - H3: A host slot is released after 72 hours of inactivity (no heartbeat).
//
// Wiring instructions:
//
//   - node_limit.go should call tracker.StableActiveHosts() to filter which
//     hosts count toward the license limit, instead of counting all hosts
//     unconditionally.
//   - The monitor's update loop (where host heartbeats arrive) should call
//     tracker.RecordHeartbeat(hostID) on every heartbeat.
//   - The host ledger (host_ledger.go) can use tracker.FirstSeen(hostID) for
//     the first_seen field in ledger entries.
//   - Call tracker.Prune() periodically (e.g. every hour) to release stale
//     entries and free memory.
//   - On startup, call LoadState(path) to restore persisted lifecycle state.
//     On shutdown (or periodically), call SaveState(path) to persist it.
package licensing

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	// HostStabilizationPeriod is the minimum duration of heartbeats before a
	// host is considered "stable" and counted toward the license limit.
	HostStabilizationPeriod = 10 * time.Minute

	// HostInactivityTimeout is the duration after which a host with no
	// heartbeat is considered inactive and its slot is released.
	HostInactivityTimeout = 72 * time.Hour
)

// hostEntry tracks lifecycle timestamps for a single host.
type hostEntry struct {
	FirstSeen time.Time  `json:"first_seen"`
	LastSeen  time.Time  `json:"last_seen"`
	StableAt  *time.Time `json:"stable_at,omitempty"`
}

// HostLifecycleTracker tracks heartbeat stability and inactivity per host.
// All methods are safe for concurrent use.
type HostLifecycleTracker struct {
	mu    sync.RWMutex
	hosts map[string]*hostEntry

	// nowFunc is used for time; overridden in tests.
	nowFunc func() time.Time
}

// NewHostLifecycleTracker creates a new tracker with an empty host map.
func NewHostLifecycleTracker() *HostLifecycleTracker {
	return &HostLifecycleTracker{
		hosts:   make(map[string]*hostEntry),
		nowFunc: time.Now,
	}
}

// RecordHeartbeat records a heartbeat for the given host. If the host is new,
// its firstSeen is set. Once HostStabilizationPeriod has elapsed since
// firstSeen, stableAt is set (and never cleared). Empty hostID is ignored.
func (t *HostLifecycleTracker) RecordHeartbeat(hostID string) {
	if hostID == "" {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	now := t.nowFunc()
	e, ok := t.hosts[hostID]
	if !ok {
		e = &hostEntry{
			FirstSeen: now,
		}
		t.hosts[hostID] = e
	}
	e.LastSeen = now

	if e.StableAt == nil && now.Sub(e.FirstSeen) >= HostStabilizationPeriod {
		ts := now
		e.StableAt = &ts
	}
}

// IsStable reports whether the host has achieved stability (10+ minutes of
// heartbeats since first seen).
func (t *HostLifecycleTracker) IsStable(hostID string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	e, ok := t.hosts[hostID]
	if !ok {
		return false
	}
	return e.StableAt != nil
}

// IsActive reports whether the host has sent a heartbeat within the last 72
// hours.
func (t *HostLifecycleTracker) IsActive(hostID string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	e, ok := t.hosts[hostID]
	if !ok {
		return false
	}
	return t.nowFunc().Sub(e.LastSeen) <= HostInactivityTimeout
}

// FirstSeen returns the time the host was first seen. Returns the zero value
// if the host is unknown.
func (t *HostLifecycleTracker) FirstSeen(hostID string) time.Time {
	t.mu.RLock()
	defer t.mu.RUnlock()

	e, ok := t.hosts[hostID]
	if !ok {
		return time.Time{}
	}
	return e.FirstSeen
}

// StableActiveHosts returns the IDs of all hosts that are both stable (10+
// minutes of heartbeats) and active (heartbeat within the last 72 hours).
// The returned slice is in no particular order.
func (t *HostLifecycleTracker) StableActiveHosts() []string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	now := t.nowFunc()
	var result []string
	for id, e := range t.hosts {
		if e.StableAt != nil && now.Sub(e.LastSeen) <= HostInactivityTimeout {
			result = append(result, id)
		}
	}
	return result
}

// Prune removes hosts that have been inactive for longer than
// HostInactivityTimeout. Call periodically (e.g. hourly) to free memory.
func (t *HostLifecycleTracker) Prune() {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := t.nowFunc()
	for id, e := range t.hosts {
		if now.Sub(e.LastSeen) > HostInactivityTimeout {
			delete(t.hosts, id)
		}
	}
}

// HostCount returns the total number of tracked hosts (regardless of state).
func (t *HostLifecycleTracker) HostCount() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.hosts)
}

// ---------------------------------------------------------------------------
// Persistence
// ---------------------------------------------------------------------------

// persistedState is the JSON-serializable representation of the tracker state.
type persistedState struct {
	Hosts map[string]*hostEntry `json:"hosts"`
}

// SaveState persists the current tracker state to a JSON file at the given
// path. The file is written atomically (temp file + rename).
func (t *HostLifecycleTracker) SaveState(path string) error {
	t.mu.RLock()
	state := persistedState{Hosts: t.hosts}
	data, err := json.MarshalIndent(state, "", "  ")
	t.mu.RUnlock()
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	f, err := os.CreateTemp(dir, filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()

	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// LoadState restores tracker state from a JSON file previously written by
// SaveState. If the file does not exist, the tracker is left empty (no error).
func (t *HostLifecycleTracker) LoadState(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if len(data) == 0 {
		t.mu.Lock()
		t.hosts = make(map[string]*hostEntry)
		t.mu.Unlock()
		return nil
	}

	var state persistedState
	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if state.Hosts == nil {
		t.hosts = make(map[string]*hostEntry)
		return nil
	}

	// Filter out nil entries that could cause panics on read paths.
	for id, e := range state.Hosts {
		if e == nil {
			delete(state.Hosts, id)
		}
	}
	t.hosts = state.Hosts
	return nil
}
