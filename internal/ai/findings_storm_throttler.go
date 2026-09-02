// findings_storm_throttler.go implements the MVP finding-storm throttler.
//
// The throttler clusters new-finding emissions by ResourceID and, when a
// cluster crosses stormThreshold within stormWindow, emits a single
// meta-finding (FindingCategoryReliability, Source=stormFindingSource)
// that points the operator at the noisy resource instead of letting many
// per-symptom findings drown the surface.
//
// The observer (observeLocked) is called from FindingsStore.Add with the
// store mutex held; it owns its own internal mutex so the throttler does
// not couple to the store's lock-ordering invariants. The observer must
// NOT call back into FindingsStore.
//
// The same throttler owns flap detection (observeFlapLocked): a finding
// whose open/resolved state changes at least findingFlapThreshold times
// inside findingFlapWindow is "flapping". The store then collapses those
// transitions into one lifecycle entry carrying the count instead of
// appending one regressed/resolved row per change, and the attention
// projection reads the same constants so alert and finding surfaces agree
// on what "flapping" means.

package ai

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	stormWindow          = 60 * time.Second
	stormThreshold       = 3
	stormFindingSource   = "finding-storm"
	stormFindingIDPrefix = "finding-storm:"
	stormClusterCap      = 1024
	stormResolveReason   = "finding-storm:rate_dropped_below_threshold"

	// findingFlapWindow is the sliding window over which open/resolved
	// transitions count toward flapping. Twenty-four hours matches the
	// operator-visible complaint (eleven transitions in a day) rather than
	// the alert engine's five-minute notification-suppression window.
	findingFlapWindow = 24 * time.Hour
	// findingFlapThreshold is the number of open/resolved transitions inside
	// findingFlapWindow at which an item is labelled flapping. Four means
	// the condition came back at least twice; a single recurrence is a
	// regression, not a flap.
	findingFlapThreshold = 4
	// findingFlapTrackerCap bounds per-finding transition tracking.
	findingFlapTrackerCap = 4096
	// FindingLifecycleFlapping is the collapsed lifecycle event type that
	// stands in for the individual regressed/resolved rows while a finding
	// is flapping. Its metadata carries transition_count and window_hours.
	FindingLifecycleFlapping = "flapping"
)

// FindingFlapping summarises a finding whose open/resolved state keeps
// changing. It is stamped by the store while the transition count inside
// the window is at or above findingFlapThreshold and cleared otherwise.
type FindingFlapping struct {
	// TransitionCount is the number of open/resolved transitions observed
	// inside the window ending at LastTransitionAt.
	TransitionCount int `json:"transition_count"`
	// WindowHours is the length of the sliding window in hours.
	WindowHours int `json:"window_hours"`
	// FirstTransitionAt is the oldest transition still inside the window.
	FirstTransitionAt time.Time `json:"first_transition_at"`
	// LastTransitionAt is the most recent transition.
	LastTransitionAt time.Time `json:"last_transition_at"`
}

// findingFlapTracker holds the sliding window of open/resolved transition
// timestamps for one finding.
type findingFlapTracker struct {
	transitions   []time.Time
	hydrated      bool
	lastTouchedAt time.Time
}

// findingStormCluster carries per-clusterKey state: the sliding window of
// emission timestamps, identity of the storm finding currently emitted for
// the cluster (if any), and the freshest contributor metadata so the storm
// finding's operator-facing pointer tracks the latest symptom.
type findingStormCluster struct {
	emissions        []time.Time
	emittedFindingID string
	lastEmittedAt    time.Time
	lastTouchedAt    time.Time

	lastResourceID   string
	lastResourceName string
	lastResourceType string
	lastNode         string

	contributorTitles []string
}

// findingStormThrottler clusters new-finding emissions by clusterKey and
// emits a single storm finding per cluster while the cluster is above
// threshold. Safe for concurrent use.
type findingStormThrottler struct {
	mu       sync.Mutex
	clusters map[string]*findingStormCluster
	cap      int

	flaps   map[string]*findingFlapTracker
	flapCap int
}

func newFindingStormThrottler() *findingStormThrottler {
	return &findingStormThrottler{
		clusters: make(map[string]*findingStormCluster),
		cap:      stormClusterCap,
		flaps:    make(map[string]*findingFlapTracker),
		flapCap:  findingFlapTrackerCap,
	}
}

// observeLocked is invoked from FindingsStore.Add with the store mutex
// held. It returns:
//   - nil: no action.
//   - *Finding with ResolvedAt == nil: a storm finding the caller should
//     re-enter through s.Add. Re-entry with the same ID lands in the
//     existing-finding branch on subsequent emissions, so the storm
//     finding is updated rather than duplicated.
//   - *Finding with ResolvedAt != nil: a sentinel; the caller should
//     route through s.ResolveWithReason(finding.ID, stormResolveReason).
//
// The "Locked" suffix names the caller's lock state (s.mu held), not this
// method's — the throttler uses its own internal mutex.
func (t *findingStormThrottler) observeLocked(f *Finding, now time.Time) *Finding {
	if t == nil || f == nil {
		return nil
	}
	// Cycle guard: storm findings must not be observed; their own
	// emission re-enters Add and would otherwise recurse.
	if f.Source == stormFindingSource {
		return nil
	}
	clusterKey := strings.TrimSpace(f.ResourceID)
	if clusterKey == "" {
		return nil
	}
	// The synthetic patrol-runtime resource is a single shared bucket
	// for provider-misconfiguration findings; a config loop on Patrol
	// itself must not masquerade as a resource-level storm.
	if clusterKey == patrolRuntimeResourceID {
		return nil
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	cluster, ok := t.clusters[clusterKey]
	if !ok {
		cluster = &findingStormCluster{}
		t.clusters[clusterKey] = cluster
	}
	cluster.lastTouchedAt = now

	cutoff := now.Add(-stormWindow)
	keep := cluster.emissions[:0]
	for _, ts := range cluster.emissions {
		if !ts.Before(cutoff) {
			keep = append(keep, ts)
		}
	}
	cluster.emissions = keep
	cluster.emissions = append(cluster.emissions, now)

	cluster.lastResourceID = f.ResourceID
	cluster.lastResourceName = f.ResourceName
	cluster.lastResourceType = f.ResourceType
	cluster.lastNode = f.Node

	title := strings.TrimSpace(f.Title)
	if title != "" {
		alreadyListed := false
		for _, existing := range cluster.contributorTitles {
			if existing == title {
				alreadyListed = true
				break
			}
		}
		if !alreadyListed {
			if len(cluster.contributorTitles) >= 8 {
				cluster.contributorTitles = cluster.contributorTitles[1:]
			}
			cluster.contributorTitles = append(cluster.contributorTitles, title)
		}
	}

	t.pruneLRULocked(clusterKey)

	count := len(cluster.emissions)
	if count >= stormThreshold {
		stormID := stormFindingIDPrefix + clusterKey
		cluster.emittedFindingID = stormID
		cluster.lastEmittedAt = now
		return t.buildStormFindingLocked(cluster, clusterKey, count, now)
	}

	// Below threshold. If a prior storm emission has aged out (no
	// refresh for at least 2 * stormWindow), signal the caller to
	// resolve it. This is the lazy auto-resolve documented in the
	// brief: it fires the next time any finding lands on the same
	// cluster, not on a separate sweep goroutine.
	if cluster.emittedFindingID != "" && !cluster.lastEmittedAt.IsZero() &&
		now.Sub(cluster.lastEmittedAt) >= 2*stormWindow {
		stormID := cluster.emittedFindingID
		cluster.emittedFindingID = ""
		cluster.lastEmittedAt = time.Time{}
		cluster.contributorTitles = nil
		resolvedAt := now
		return &Finding{
			ID:         stormID,
			ResolvedAt: &resolvedAt,
		}
	}

	return nil
}

// pruneLRULocked evicts the least-recently-touched clusters (excluding
// the cluster the caller just touched) until the tracker is at or under
// the cap. Called while t.mu is held.
func (t *findingStormThrottler) pruneLRULocked(protectKey string) {
	if t.cap <= 0 {
		return
	}
	if len(t.clusters) <= t.cap {
		return
	}
	type entry struct {
		key string
		at  time.Time
	}
	candidates := make([]entry, 0, len(t.clusters)-1)
	for k, c := range t.clusters {
		if k == protectKey {
			continue
		}
		candidates = append(candidates, entry{key: k, at: c.lastTouchedAt})
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].at.Before(candidates[j].at)
	})
	excess := len(t.clusters) - t.cap
	for i := 0; i < excess && i < len(candidates); i++ {
		delete(t.clusters, candidates[i].key)
	}
}

// buildStormFindingLocked constructs the storm finding for a cluster.
// Called while t.mu is held. The finding has a stable ID so subsequent
// emissions on the same cluster dedup through FindingsStore.Add's
// existing-finding branch rather than spawning duplicates.
func (t *findingStormThrottler) buildStormFindingLocked(cluster *findingStormCluster, clusterKey string, count int, now time.Time) *Finding {
	resourceName := cluster.lastResourceName
	if strings.TrimSpace(resourceName) == "" {
		resourceName = clusterKey
	}
	stormID := stormFindingIDPrefix + clusterKey

	var contributorClause string
	if len(cluster.contributorTitles) > 0 {
		contributorClause = " Contributors: " + strings.Join(cluster.contributorTitles, "; ") + "."
	}
	description := fmt.Sprintf(
		"Patrol has emitted %d distinct findings against %s within %s. Inspect this resource directly; the surface is showing several independent symptoms in a tight window.%s",
		count,
		resourceName,
		stormWindow,
		contributorClause,
	)

	return &Finding{
		ID:             stormID,
		Key:            stormID,
		Severity:       FindingSeverityWarning,
		Category:       FindingCategoryReliability,
		ResourceID:     cluster.lastResourceID,
		ResourceName:   resourceName,
		ResourceType:   cluster.lastResourceType,
		Node:           cluster.lastNode,
		Title:          fmt.Sprintf("Multiple findings emitted against %s in %s", resourceName, stormWindow),
		Description:    description,
		Recommendation: "Inspect this resource directly; Patrol is surfacing several independent symptoms in a tight window.",
		Evidence:       fmt.Sprintf("clusterKey=%s emissions=%d windowSeconds=%d", clusterKey, count, int(stormWindow/time.Second)),
		Source:         stormFindingSource,
		DetectedAt:     now,
		LastSeenAt:     now,
	}
}

// observeFlapLocked records one open/resolved transition for f at now and
// returns the resulting FindingFlapping summary, or nil while the finding
// is below the flapping threshold. Like observeLocked it is called with
// the store mutex held and uses the throttler's own mutex; it never calls
// back into the store.
//
// The first observation of a finding hydrates the window from the
// finding's persisted lifecycle so a restart does not forget that an item
// was flapping until the next transition lands.
func (t *findingStormThrottler) observeFlapLocked(f *Finding, now time.Time) *FindingFlapping {
	if t == nil || f == nil {
		return nil
	}
	key := strings.TrimSpace(f.ID)
	if key == "" {
		return nil
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if t.flaps == nil {
		t.flaps = make(map[string]*findingFlapTracker)
	}
	tracker, ok := t.flaps[key]
	if !ok {
		tracker = &findingFlapTracker{}
		t.flaps[key] = tracker
	}
	if !tracker.hydrated {
		tracker.transitions = flapTransitionsFromLifecycle(f.Lifecycle, now)
		tracker.hydrated = true
	}
	tracker.lastTouchedAt = now
	tracker.transitions = append(trimFlapWindow(tracker.transitions, now), now)
	t.pruneFlapLRULocked(key)

	return summarizeFlapTransitions(tracker.transitions, now)
}

// pruneFlapLRULocked evicts the least-recently-touched trackers (excluding
// the one just touched) until the map is at or under flapCap.
func (t *findingStormThrottler) pruneFlapLRULocked(protectKey string) {
	if t.flapCap <= 0 || len(t.flaps) <= t.flapCap {
		return
	}
	type entry struct {
		key string
		at  time.Time
	}
	candidates := make([]entry, 0, len(t.flaps)-1)
	for k, tracker := range t.flaps {
		if k == protectKey {
			continue
		}
		candidates = append(candidates, entry{key: k, at: tracker.lastTouchedAt})
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].at.Before(candidates[j].at)
	})
	excess := len(t.flaps) - t.flapCap
	for i := 0; i < excess && i < len(candidates); i++ {
		delete(t.flaps, candidates[i].key)
	}
}

// trimFlapWindow drops transitions older than findingFlapWindow before now.
func trimFlapWindow(transitions []time.Time, now time.Time) []time.Time {
	cutoff := now.Add(-findingFlapWindow)
	keep := transitions[:0]
	for _, ts := range transitions {
		if !ts.Before(cutoff) {
			keep = append(keep, ts)
		}
	}
	return keep
}

// summarizeFlapTransitions returns the flapping summary for a window of
// transition timestamps, or nil below the threshold. Shared by the finding
// store and the attention projection so both surfaces agree.
func summarizeFlapTransitions(transitions []time.Time, now time.Time) *FindingFlapping {
	inWindow := make([]time.Time, 0, len(transitions))
	cutoff := now.Add(-findingFlapWindow)
	for _, ts := range transitions {
		if !ts.Before(cutoff) {
			inWindow = append(inWindow, ts)
		}
	}
	if len(inWindow) < findingFlapThreshold {
		return nil
	}
	sort.Slice(inWindow, func(i, j int) bool { return inWindow[i].Before(inWindow[j]) })
	return &FindingFlapping{
		TransitionCount:   len(inWindow),
		WindowHours:       int(findingFlapWindow / time.Hour),
		FirstTransitionAt: inWindow[0],
		LastTransitionAt:  inWindow[len(inWindow)-1],
	}
}

// flapTransitionsFromLifecycle rebuilds the in-window transition
// timestamps from a persisted lifecycle log. Individual regressed and
// resolved rows count once each; a collapsed flapping row contributes its
// recorded transition_count, spread as repeats of its timestamp so the
// count survives without inventing intermediate times.
func flapTransitionsFromLifecycle(lifecycle []FindingLifecycleEvent, now time.Time) []time.Time {
	cutoff := now.Add(-findingFlapWindow)
	var transitions []time.Time
	for _, event := range lifecycle {
		if event.At.Before(cutoff) || event.At.After(now) {
			continue
		}
		switch event.Type {
		case "regressed", "resolved", "auto_resolved":
			transitions = append(transitions, event.At)
		case FindingLifecycleFlapping:
			count := 1
			if raw, ok := event.Metadata["transition_count"]; ok {
				if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
					count = parsed
				}
			}
			for i := 0; i < count; i++ {
				transitions = append(transitions, event.At)
			}
		}
	}
	return transitions
}
