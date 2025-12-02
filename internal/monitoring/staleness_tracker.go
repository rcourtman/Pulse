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

// StalenessTracker maintains freshness metadata and exposes normalized staleness scores.
type StalenessTracker struct {
	mu       sync.RWMutex
	entries  map[string]FreshnessSnapshot
	baseTTL  time.Duration
	maxStale time.Duration
	metrics  *PollMetrics
}

// NewStalenessTracker builds a tracker wired to poll metrics for last-success signal and using default parameters.
func NewStalenessTracker(metrics *PollMetrics) *StalenessTracker {
	return &StalenessTracker{
		entries:  make(map[string]FreshnessSnapshot),
		baseTTL:  10 * time.Second,
		maxStale: 5 * time.Minute,
		metrics:  metrics,
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

	key := trackerKey(instanceType, instance)
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

	snap, ok := t.snapshot(instanceType, instance)
	if !ok {
		return 0, false
	}

	if !snap.LastSuccess.IsZero() && t.metrics != nil {
		if ts, ok := t.metrics.lastSuccessFor(string(instanceType), instance); ok {
			snap.LastSuccess = ts
		}
	}

	if snap.LastSuccess.IsZero() {
		return 1, true
	}

	now := time.Now()
	age := now.Sub(snap.LastSuccess)
	if age <= 0 {
		return 0, true
	}

	max := t.maxStale
	if max <= 0 {
		max = 5 * time.Minute
	}
	score := age.Seconds() / max.Seconds()
	if score > 1 {
		score = 1
	}
	return score, true
}

func (t *StalenessTracker) setSnapshot(snapshot FreshnessSnapshot) {
	t.mu.Lock()
	defer t.mu.Unlock()
	key := trackerKey(snapshot.InstanceType, snapshot.Instance)
	t.entries[key] = snapshot
}

func (t *StalenessTracker) mergeSnapshot(snapshot FreshnessSnapshot) {
	t.mu.Lock()
	defer t.mu.Unlock()

	key := trackerKey(snapshot.InstanceType, snapshot.Instance)
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
	snap, ok := t.entries[trackerKey(instanceType, instance)]
	return snap, ok
}

func trackerKey(instanceType InstanceType, instance string) string {
	return string(instanceType) + "::" + instance
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
		return nil
	}

	t.mu.RLock()
	entries := make([]FreshnessSnapshot, 0, len(t.entries))
	for _, entry := range t.entries {
		entries = append(entries, entry)
	}
	t.mu.RUnlock()

	result := make([]StalenessSnapshot, 0, len(entries))
	for _, entry := range entries {
		score, _ := t.StalenessScore(entry.InstanceType, entry.Instance)
		result = append(result, StalenessSnapshot{
			Instance:    entry.Instance,
			Type:        string(entry.InstanceType),
			Score:       score,
			LastSuccess: entry.LastSuccess,
			LastError:   entry.LastError,
		})
	}
	return result
}
