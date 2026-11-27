package monitoring

import (
	"crypto/sha1"
	"encoding/hex"
	"sync"
	"time"
)

// FreshnessSnapshot captures the most recent freshness metadata available for a target instance.
type FreshnessSnapshot struct {
	InstanceType InstanceType
	Instance     string
	LastSuccess  time.Time
	LastError    time.Time
	LastMutated  time.Time
	ChangeHash   string
}

// freshnessTarget keeps the scheduler's exact target identity without
// joining the strings on each poll result or freshness lookup.
type freshnessTarget struct {
	kind InstanceType
	name string
}

// StalenessTracker maintains freshness metadata and exposes normalized staleness scores.
type StalenessTracker struct {
	mu       sync.RWMutex
	entries  map[freshnessTarget]FreshnessSnapshot
	baseTTL  time.Duration
	maxStale time.Duration
}

// NewStalenessTracker builds a monitor-local tracker with default bounds.
// The legacy metrics argument is ignored: process-wide labels do not identify
// a monitor's connection, and exporter samples must not feed back into polling.
func NewStalenessTracker(_ *PollMetrics) *StalenessTracker {
	return &StalenessTracker{
		entries:  make(map[freshnessTarget]FreshnessSnapshot),
		baseTTL:  10 * time.Second,
		maxStale: 5 * time.Minute,
	}
}

// SetBounds allows overriding score decay windows.
func (t *StalenessTracker) SetBounds(baseTTL, maxStale time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if baseTTL > 0 {
		t.baseTTL = baseTTL
	}
	if maxStale > 0 {
		t.maxStale = maxStale
	}
}

// UpdateSuccess records a successful poll along with a change hash derived from the payload.
func (t *StalenessTracker) UpdateSuccess(instanceType InstanceType, instance string, payload []byte) {
	if t == nil {
		return
	}

	now := time.Now()
	snapshot := FreshnessSnapshot{
		InstanceType: instanceType,
		Instance:     instance,
		LastSuccess:  now,
	}

	if len(payload) > 0 {
		sum := sha1.Sum(payload)
		snapshot.ChangeHash = hex.EncodeToString(sum[:])
		snapshot.LastMutated = now
	}

	t.setSnapshot(snapshot)
}

// UpdateError records the most recent error time for a target instance.
func (t *StalenessTracker) UpdateError(instanceType InstanceType, instance string) {
	if t == nil {
		return
	}

	snapshot := FreshnessSnapshot{
		InstanceType: instanceType,
		Instance:     instance,
		LastError:    time.Now(),
	}

	t.mergeSnapshot(snapshot)
}

// Remove deletes freshness metadata for a retired instance so deleted targets
// do not linger in staleness snapshots.
func (t *StalenessTracker) Remove(instanceType InstanceType, instance string) {
	if t == nil {
		return
	}

	t.mu.Lock()
	delete(t.entries, freshnessTarget{instanceType, instance})
	t.mu.Unlock()
}

// SetChangeHash updates the change fingerprint without affecting success timestamps.
func (t *StalenessTracker) SetChangeHash(instanceType InstanceType, instance string, payload []byte) {
	if t == nil || len(payload) == 0 {
		return
	}

	now := time.Now()
	sum := sha1.Sum(payload)
	hash := hex.EncodeToString(sum[:])

	t.mu.Lock()
	defer t.mu.Unlock()

	key := freshnessTarget{instanceType, instance}
	snap := t.entries[key]
	snap.InstanceType = instanceType
	snap.Instance = instance
	snap.ChangeHash = hash
	snap.LastMutated = now
	t.entries[key] = snap
}

// StalenessScore implements the StalenessSource interface and returns a normalized value in [0,1].
func (t *StalenessTracker) StalenessScore(instanceType InstanceType, instance string) (float64, bool) {
	if t == nil {
		return 0, false
	}

	t.mu.RLock()
	snap, ok := t.entries[freshnessTarget{instanceType, instance}]
	maxStale := t.maxStale
	t.mu.RUnlock()
	if !ok {
		return 0, false
	}

	return stalenessAt(time.Now(), snap.LastSuccess, maxStale), true
}

func stalenessAt(now, lastSuccess time.Time, maxStale time.Duration) float64 {
	if lastSuccess.IsZero() {
		return 1
	}

	age := now.Sub(lastSuccess)
	if age <= 0 {
		return 0
	}

	if maxStale <= 0 {
		maxStale = 5 * time.Minute
	}
	return min(age.Seconds()/maxStale.Seconds(), 1)
}

func (t *StalenessTracker) setSnapshot(snapshot FreshnessSnapshot) {
	t.mu.Lock()
	defer t.mu.Unlock()
	key := freshnessTarget{snapshot.InstanceType, snapshot.Instance}
	t.entries[key] = snapshot
}

func (t *StalenessTracker) mergeSnapshot(snapshot FreshnessSnapshot) {
	t.mu.Lock()
	defer t.mu.Unlock()

	key := freshnessTarget{snapshot.InstanceType, snapshot.Instance}
	existing := t.entries[key]

	if snapshot.LastSuccess.After(existing.LastSuccess) {
		existing.LastSuccess = snapshot.LastSuccess
	}
	if snapshot.LastError.After(existing.LastError) {
		existing.LastError = snapshot.LastError
	}
	if snapshot.LastMutated.After(existing.LastMutated) {
		existing.LastMutated = snapshot.LastMutated
	}
	if snapshot.ChangeHash != "" {
		existing.ChangeHash = snapshot.ChangeHash
	}

	existing.InstanceType = snapshot.InstanceType
	existing.Instance = snapshot.Instance

	t.entries[key] = existing
}

func (t *StalenessTracker) snapshot(instanceType InstanceType, instance string) (FreshnessSnapshot, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	snap, ok := t.entries[freshnessTarget{instanceType, instance}]
	return snap, ok
}

// StalenessSnapshot represents staleness data for a single instance.
type StalenessSnapshot struct {
	Instance    string    `json:"instance"`
	Type        string    `json:"type"`
	Score       float64   `json:"score"`
	LastSuccess time.Time `json:"lastSuccess"`
	LastError   time.Time `json:"lastError,omitempty"`
}

// Snapshot returns a copy of all staleness data for API exposure.
func (t *StalenessTracker) Snapshot() []StalenessSnapshot {
	if t == nil {
		return []StalenessSnapshot{}
	}

	t.mu.RLock()
	entries := make([]FreshnessSnapshot, 0, len(t.entries))
	maxStale := t.maxStale
	now := time.Now()
	for _, entry := range t.entries {
		entries = append(entries, entry)
	}
	t.mu.RUnlock()

	result := make([]StalenessSnapshot, 0, len(entries))
	for _, entry := range entries {
		result = append(result, StalenessSnapshot{
			Instance:    entry.Instance,
			Type:        string(entry.InstanceType),
			Score:       stalenessAt(now, entry.LastSuccess, maxStale),
			LastSuccess: entry.LastSuccess,
			LastError:   entry.LastError,
		})
	}
	return result
}
