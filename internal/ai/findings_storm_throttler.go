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

package ai

import (
	"fmt"
	"sort"
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
)

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
}

func newFindingStormThrottler() *findingStormThrottler {
	return &findingStormThrottler{
		clusters: make(map[string]*findingStormCluster),
		cap:      stormClusterCap,
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
