// findings_update_safety.go implements the update-safety watcher for Docker
// containers. It snapshot-diffs ImageDigest across patrol observe trips and
// emits a reliability finding when a container's image changes unexpectedly
// (e.g. Watchtower-class auto-update). It auto-resolves via a lazy sentinel
// on the next observe trip once the container has been stable for the
// verification window with no new restarts.
package ai

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

const (
	// UpdateSafetyFindingPrefix is the dedup-key prefix for update-safety findings.
	// Concrete keys are prefix + ":" + containerKey where containerKey is hostID/containerID.
	UpdateSafetyFindingPrefix = "docker:image:update_divergence"

	// updateSafetyVerifyWindow is how long the watcher waits after a digest
	// change, with no new restarts, before emitting a resolve sentinel.
	updateSafetyVerifyWindow = 90 * time.Second

	updateSafetySource        = "update-safety"
	updateSafetyResolveReason = "update_safety:verified_clean"
	updateSafetySnapshotCap   = 2048
)

// resolveSentinel carries the dedup key and reason for a lazy auto-resolve.
// The caller routes it through FindingsStore.ResolveWithReason.
type resolveSentinel struct {
	DedupKey string
	Reason   string
}

// updateSafetySnapshot holds per-container state between observe trips.
type updateSafetySnapshot struct {
	digest       string    // digest as of the last observe trip
	restartCount int       // restartCount as of the last observe trip
	lastSeenAt   time.Time // when this snapshot was last updated

	// Fields populated once a digest change is detected.
	detectedAt          time.Time // zero when no active change is being verified
	priorDigest         string    // digest before the change
	changeDigest        string    // digest after the change
	baseRestarts        int       // restartCount at the moment the change was detected
	lastEmittedRestarts int       // restartCount at the time of the most recent emit
}

// UpdateSafetyWatcher snapshot-diffs container image digests across patrol
// observe trips. In-memory only; no persistence needed at MVP.
type UpdateSafetyWatcher struct {
	mu        sync.Mutex
	snapshots map[string]*updateSafetySnapshot // key: hostID/containerID
	cap       int
}

// newUpdateSafetyWatcher returns an initialized watcher ready for use.
func newUpdateSafetyWatcher() *UpdateSafetyWatcher {
	return &UpdateSafetyWatcher{
		snapshots: make(map[string]*updateSafetySnapshot),
		cap:       updateSafetySnapshotCap,
	}
}

// Observe is called on every patrol cycle with the current DockerHosts slice.
// On first call for a container it records a baseline and returns nothing.
// On subsequent calls it diffs ImageDigest and RestartCount:
//   - Digest changed -> emit a reliability finding (Info or Warning).
//   - Already-detected change, restarts increased -> re-emit as Warning.
//   - Already-detected change, stable for >= verifyWindow, no new restarts -> emit resolve sentinel.
func (w *UpdateSafetyWatcher) Observe(hosts []models.DockerHost, now time.Time) (emit []*Finding, resolve []resolveSentinel) {
	w.mu.Lock()
	defer w.mu.Unlock()

	seen := make(map[string]struct{}, len(hosts)*8)

	for _, host := range hosts {
		for _, c := range host.Containers {
			if c.ImageDigest == "" {
				continue
			}
			key := host.ID + "/" + c.ID
			seen[key] = struct{}{}

			snap, exists := w.snapshots[key]
			if !exists {
				// First observation -- record baseline, emit nothing.
				w.snapshots[key] = &updateSafetySnapshot{
					digest:       c.ImageDigest,
					restartCount: c.RestartCount,
					lastSeenAt:   now,
				}
				continue
			}
			snap.lastSeenAt = now

			if snap.detectedAt.IsZero() {
				// State A: no change detected yet.
				if c.ImageDigest == snap.digest {
					snap.restartCount = c.RestartCount
					continue
				}
				// Digest changed -- transition to state B.
				snap.priorDigest         = snap.digest
				snap.changeDigest        = c.ImageDigest
				snap.baseRestarts        = snap.restartCount
				snap.lastEmittedRestarts = snap.restartCount
				snap.detectedAt          = now
				snap.digest              = c.ImageDigest
				snap.restartCount        = c.RestartCount

				severity := FindingSeverityInfo
				if c.RestartCount > snap.baseRestarts {
					severity = FindingSeverityWarning
					snap.lastEmittedRestarts = c.RestartCount
				}
				emit = append(emit, buildUpdateSafetyFinding(key, host, c, snap, severity, now, now))
				continue
			}

			// State B: change already detected, verifying stability.
			snap.digest       = c.ImageDigest
			snap.restartCount = c.RestartCount
			restartsAfterChange := c.RestartCount - snap.baseRestarts

			if restartsAfterChange > snap.lastEmittedRestarts-snap.baseRestarts {
				// New restarts since last emission -- escalate to Warning.
				snap.lastEmittedRestarts = c.RestartCount
				emit = append(emit, buildUpdateSafetyFinding(key, host, c, snap, FindingSeverityWarning, snap.detectedAt, now))
				continue
			}

			if now.Sub(snap.detectedAt) >= updateSafetyVerifyWindow && restartsAfterChange == 0 {
				// Stable for the full window -- emit resolve sentinel and reset.
				dedupKey := UpdateSafetyFindingPrefix + ":" + key
				resolve = append(resolve, resolveSentinel{DedupKey: dedupKey, Reason: updateSafetyResolveReason})
				snap.detectedAt          = time.Time{}
				snap.priorDigest         = ""
				snap.changeDigest        = ""
				snap.baseRestarts        = 0
				snap.lastEmittedRestarts = 0
			}
			// Otherwise: still in window, no new restarts -- do nothing.
		}
	}

	w.pruneLRULocked(seen)
	return emit, resolve
}

// buildUpdateSafetyFinding constructs a Finding for a detected image change.
func buildUpdateSafetyFinding(key string, host models.DockerHost, c models.DockerContainer, snap *updateSafetySnapshot, severity FindingSeverity, detectedAt, lastSeenAt time.Time) *Finding {
	name := c.Name
	if name == "" {
		name = c.ID
	}
	dedupKey := UpdateSafetyFindingPrefix + ":" + key
	restartsAfterChange := c.RestartCount - snap.baseRestarts

	detail := fmt.Sprintf(
		"Image digest changed from %.16s to %.16s",
		snap.priorDigest, snap.changeDigest,
	)
	if restartsAfterChange > 0 {
		detail += fmt.Sprintf(
			" Container has restarted %d time(s) since the update.",
			restartsAfterChange,
		)
	}

	return &Finding{
		ID:           dedupKey,
		Key:          dedupKey,
		Severity:     severity,
		Category:     FindingCategoryReliability,
		ResourceID:   key,
		ResourceName: name,
		ResourceType: "app-container",
		Node:         host.Hostname,
		Title:        fmt.Sprintf("Container %q image updated", name),
		Description:  detail,
		Evidence: fmt.Sprintf(
			"prior_digest=%s new_digest=%s restart_count=%d",
			snap.priorDigest, snap.changeDigest, c.RestartCount,
		),
		Source:     updateSafetySource,
		DetectedAt: detectedAt,
		LastSeenAt: lastSeenAt,
	}
}

// pruneLRULocked evicts state-A snapshots for unseen containers, then if
// still over cap evicts the least-recently-seen non-current-cycle entries.
// Called with w.mu held.
func (w *UpdateSafetyWatcher) pruneLRULocked(seen map[string]struct{}) {
	// Remove state-A snapshots for containers no longer observed.
	for k, snap := range w.snapshots {
		if _, ok := seen[k]; !ok && snap.detectedAt.IsZero() {
			delete(w.snapshots, k)
		}
	}

	if w.cap <= 0 || len(w.snapshots) <= w.cap {
		return
	}

	type entry struct {
		key string
		at  time.Time
	}
	candidates := make([]entry, 0, len(w.snapshots))
	for k, snap := range w.snapshots {
		if _, protected := seen[k]; protected {
			continue
		}
		candidates = append(candidates, entry{key: k, at: snap.lastSeenAt})
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].at.Before(candidates[j].at)
	})
	excess := len(w.snapshots) - w.cap
	for i := 0; i < excess && i < len(candidates); i++ {
		delete(w.snapshots, candidates[i].key)
	}
}
