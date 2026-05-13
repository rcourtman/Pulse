package ai

import (
	"fmt"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// makeHost builds a minimal DockerHost with one container for test use.
func makeHost(hostID, cID, digest string, restarts int) models.DockerHost {
	return models.DockerHost{
		ID:       hostID,
		Hostname: hostID + ".host",
		Containers: []models.DockerContainer{
			{
				ID:           cID,
				Name:         "container-" + cID,
				ImageDigest:  digest,
				RestartCount: restarts,
			},
		},
	}
}

func containerKey(hostID, cID string) string { return hostID + "/" + cID }

// TestUpdateSafety_FirstObserveIsSilent verifies no findings are returned
// on the very first observe trip (baseline-only).
func TestUpdateSafety_FirstObserveIsSilent(t *testing.T) {
	w := newUpdateSafetyWatcher()
	now := time.Now()
	hosts := []models.DockerHost{makeHost("h1", "c1", "sha256:aaa", 0)}
	emit, resolve := w.Observe(hosts, now)
	if len(emit) != 0 || len(resolve) != 0 {
		t.Fatalf("first observe: want empty emit+resolve, got emit=%d resolve=%d", len(emit), len(resolve))
	}
}

// TestUpdateSafety_DigestChangeEmitsFinding verifies that a changed digest on
// the second trip produces exactly one Info finding with the correct shape.
func TestUpdateSafety_DigestChangeEmitsFinding(t *testing.T) {
	w := newUpdateSafetyWatcher()
	now := time.Now()
	hosts1 := []models.DockerHost{makeHost("h1", "c1", "sha256:aaa", 0)}
	w.Observe(hosts1, now)

	hosts2 := []models.DockerHost{makeHost("h1", "c1", "sha256:bbb", 0)}
	emit, resolve := w.Observe(hosts2, now.Add(5*time.Second))
	if len(emit) != 1 {
		t.Fatalf("digest change: want 1 finding, got %d", len(emit))
	}
	if len(resolve) != 0 {
		t.Fatalf("digest change: want 0 resolves, got %d", len(resolve))
	}
	f := emit[0]
	if f.Category != FindingCategoryReliability {
		t.Errorf("category: want %q, got %q", FindingCategoryReliability, f.Category)
	}
	if f.Severity != FindingSeverityInfo {
		t.Errorf("severity: want %q, got %q (no restarts yet)", FindingSeverityInfo, f.Severity)
	}
	if f.ResourceType != "app-container" {
		t.Errorf("resource_type: want %q, got %q", "app-container", f.ResourceType)
	}
	key := containerKey("h1", "c1")
	wantDedupKey := UpdateSafetyFindingPrefix + ":" + key
	if f.ID != wantDedupKey {
		t.Errorf("ID: want %q, got %q", wantDedupKey, f.ID)
	}
	if f.Source != updateSafetySource {
		t.Errorf("source: want %q, got %q", updateSafetySource, f.Source)
	}
}

// TestUpdateSafety_RestartAfterDigestEscalatesToWarning verifies that when
// RestartCount increases after a digest change, the next observe emits a
// Warning-severity finding via the same dedup key.
func TestUpdateSafety_RestartAfterDigestEscalatesToWarning(t *testing.T) {
	w := newUpdateSafetyWatcher()
	t0 := time.Now()

	// Trip 1: baseline.
	w.Observe([]models.DockerHost{makeHost("h1", "c1", "sha256:aaa", 0)}, t0)

	// Trip 2: digest changed, no restarts yet -- should emit Info.
	emit2, _ := w.Observe([]models.DockerHost{makeHost("h1", "c1", "sha256:bbb", 0)}, t0.Add(5*time.Second))
	if len(emit2) != 1 || emit2[0].Severity != FindingSeverityInfo {
		t.Fatalf("trip 2: want 1 Info finding, got %v", emit2)
	}

	// Trip 3: same digest, but restart count increased -- should escalate.
	emit3, resolve3 := w.Observe([]models.DockerHost{makeHost("h1", "c1", "sha256:bbb", 2)}, t0.Add(10*time.Second))
	if len(emit3) != 1 {
		t.Fatalf("trip 3: want 1 escalated finding, got %d", len(emit3))
	}
	if emit3[0].Severity != FindingSeverityWarning {
		t.Errorf("trip 3 severity: want %q, got %q", FindingSeverityWarning, emit3[0].Severity)
	}
	if len(resolve3) != 0 {
		t.Errorf("trip 3: want 0 resolves, got %d", len(resolve3))
	}
}

// TestUpdateSafety_StableWindowEmitsResolveSentinel verifies that after the
// verify window has elapsed with no new restarts, Observe returns a resolve
// sentinel and no new findings.
func TestUpdateSafety_StableWindowEmitsResolveSentinel(t *testing.T) {
	w := newUpdateSafetyWatcher()
	t0 := time.Now()

	// Trip 1: baseline.
	w.Observe([]models.DockerHost{makeHost("h1", "c1", "sha256:aaa", 0)}, t0)

	// Trip 2: digest changed -- emit finding.
	emit2, _ := w.Observe([]models.DockerHost{makeHost("h1", "c1", "sha256:bbb", 0)}, t0.Add(5*time.Second))
	if len(emit2) != 1 {
		t.Fatalf("trip 2: want 1 finding, got %d", len(emit2))
	}

	// Trip 3: same digest, no restarts, window elapsed -- should emit resolve sentinel.
	afterWindow := t0.Add(5*time.Second + updateSafetyVerifyWindow + time.Second)
	emit3, resolve3 := w.Observe([]models.DockerHost{makeHost("h1", "c1", "sha256:bbb", 0)}, afterWindow)
	if len(emit3) != 0 {
		t.Errorf("trip 3: want 0 findings after stable window, got %d", len(emit3))
	}
	if len(resolve3) != 1 {
		t.Fatalf("trip 3: want 1 resolve sentinel, got %d", len(resolve3))
	}
	key := containerKey("h1", "c1")
	wantKey := UpdateSafetyFindingPrefix + ":" + key
	if resolve3[0].DedupKey != wantKey {
		t.Errorf("resolve DedupKey: want %q, got %q", wantKey, resolve3[0].DedupKey)
	}
	if resolve3[0].Reason != updateSafetyResolveReason {
		t.Errorf("resolve Reason: want %q, got %q", updateSafetyResolveReason, resolve3[0].Reason)
	}
}

// TestUpdateSafety_SecondDigestChangeResetsVerificationWindow verifies that a
// second image update during the verification window restarts the window and
// updates the existing finding evidence rather than resolving the old update.
func TestUpdateSafety_SecondDigestChangeResetsVerificationWindow(t *testing.T) {
	w := newUpdateSafetyWatcher()
	t0 := time.Now()

	w.Observe([]models.DockerHost{makeHost("h1", "c1", "sha256:aaa", 0)}, t0)

	emit1, resolve1 := w.Observe([]models.DockerHost{makeHost("h1", "c1", "sha256:bbb", 0)}, t0.Add(5*time.Second))
	if len(emit1) != 1 || len(resolve1) != 0 {
		t.Fatalf("first update: want 1 emit and 0 resolves, got emit=%d resolve=%d", len(emit1), len(resolve1))
	}

	secondUpdateAt := t0.Add(20 * time.Second)
	emit2, resolve2 := w.Observe([]models.DockerHost{makeHost("h1", "c1", "sha256:ccc", 0)}, secondUpdateAt)
	if len(emit2) != 1 || len(resolve2) != 0 {
		t.Fatalf("second update: want 1 emit and 0 resolves, got emit=%d resolve=%d", len(emit2), len(resolve2))
	}
	if emit2[0].Evidence != "prior_digest=sha256:bbb new_digest=sha256:ccc restart_count=0" {
		t.Fatalf("second update evidence = %q", emit2[0].Evidence)
	}

	// The first update's window has elapsed, but the second update's window has
	// not. Resolving now would close against stale evidence.
	oldWindowElapsed := t0.Add(5*time.Second + updateSafetyVerifyWindow + time.Second)
	emit3, resolve3 := w.Observe([]models.DockerHost{makeHost("h1", "c1", "sha256:ccc", 0)}, oldWindowElapsed)
	if len(emit3) != 0 || len(resolve3) != 0 {
		t.Fatalf("old window elapsed: want silent, got emit=%d resolve=%d", len(emit3), len(resolve3))
	}

	emit4, resolve4 := w.Observe([]models.DockerHost{makeHost("h1", "c1", "sha256:ccc", 0)}, secondUpdateAt.Add(updateSafetyVerifyWindow+time.Second))
	if len(emit4) != 0 || len(resolve4) != 1 {
		t.Fatalf("second window elapsed: want 0 emit and 1 resolve, got emit=%d resolve=%d", len(emit4), len(resolve4))
	}
}

// TestUpdateSafety_EmptyDigestEmitsNothing verifies that containers with an
// empty ImageDigest (agent not yet reporting one) are silently skipped.
func TestUpdateSafety_EmptyDigestEmitsNothing(t *testing.T) {
	w := newUpdateSafetyWatcher()
	now := time.Now()

	hosts := []models.DockerHost{
		{
			ID:       "h1",
			Hostname: "h1.host",
			Containers: []models.DockerContainer{
				{ID: "c1", Name: "web", ImageDigest: "", RestartCount: 0},
			},
		},
	}
	// Multiple trips -- should always be silent.
	for i := 0; i < 3; i++ {
		emit, resolve := w.Observe(hosts, now.Add(time.Duration(i)*10*time.Second))
		if len(emit) != 0 || len(resolve) != 0 {
			t.Fatalf("trip %d: empty digest should emit nothing, got emit=%d resolve=%d", i+1, len(emit), len(resolve))
		}
	}
}

// TestUpdateSafety_TwoContainersDontCollide verifies that digest changes on
// two different containers produce distinct, non-colliding dedup keys.
func TestUpdateSafety_TwoContainersDontCollide(t *testing.T) {
	w := newUpdateSafetyWatcher()
	t0 := time.Now()

	hosts1 := []models.DockerHost{
		{
			ID:       "h1",
			Hostname: "h1.host",
			Containers: []models.DockerContainer{
				{ID: "cA", Name: "alpha", ImageDigest: "sha256:aaa", RestartCount: 0},
				{ID: "cB", Name: "beta", ImageDigest: "sha256:zzz", RestartCount: 0},
			},
		},
	}
	w.Observe(hosts1, t0)

	// Change digests on both containers simultaneously.
	hosts2 := []models.DockerHost{
		{
			ID:       "h1",
			Hostname: "h1.host",
			Containers: []models.DockerContainer{
				{ID: "cA", Name: "alpha", ImageDigest: "sha256:bbb", RestartCount: 0},
				{ID: "cB", Name: "beta", ImageDigest: "sha256:yyy", RestartCount: 0},
			},
		},
	}
	emit, _ := w.Observe(hosts2, t0.Add(5*time.Second))
	if len(emit) != 2 {
		t.Fatalf("two containers: want 2 findings, got %d", len(emit))
	}
	keyA := UpdateSafetyFindingPrefix + ":h1/cA"
	keyB := UpdateSafetyFindingPrefix + ":h1/cB"
	ids := map[string]bool{emit[0].ID: true, emit[1].ID: true}
	if !ids[keyA] || !ids[keyB] {
		t.Errorf("dedup keys: want %q and %q, got %v", keyA, keyB, ids)
	}
}

// TestUpdateSafety_LRUPrunesUnseenStateAEntries verifies that state-A
// snapshots for containers that disappear from the host list are evicted.
func TestUpdateSafety_LRUPrunesUnseenStateAEntries(t *testing.T) {
	w := newUpdateSafetyWatcher()
	w.cap = 5
	now := time.Now()

	// Seed 5 containers in state A.
	hosts := make([]models.DockerHost, 5)
	for i := 0; i < 5; i++ {
		hosts[i] = makeHost("h1", fmt.Sprintf("c%d", i), "sha256:aaa", 0)
	}
	// Flatten into a single host with all containers.
	allContainers := make([]models.DockerContainer, 0, 5)
	for _, h := range hosts {
		allContainers = append(allContainers, h.Containers...)
	}
	combined := []models.DockerHost{{ID: "h1", Hostname: "h1.host", Containers: allContainers}}
	w.Observe(combined, now)
	if len(w.snapshots) != 5 {
		t.Fatalf("want 5 snapshots after seeding, got %d", len(w.snapshots))
	}

	// Next observe: only 2 containers remain -- state-A unseen ones should be pruned.
	reduced := []models.DockerHost{{
		ID:       "h1",
		Hostname: "h1.host",
		Containers: []models.DockerContainer{
			{ID: "c0", Name: "container-c0", ImageDigest: "sha256:aaa", RestartCount: 0},
			{ID: "c1", Name: "container-c1", ImageDigest: "sha256:aaa", RestartCount: 0},
		},
	}}
	w.Observe(reduced, now.Add(10*time.Second))
	if len(w.snapshots) != 2 {
		t.Errorf("after pruning: want 2 snapshots, got %d", len(w.snapshots))
	}
}

// TestUpdateSafety_WindowNotElapsedNoResolve verifies that a stable digest
// within the verify window does NOT emit a resolve sentinel.
func TestUpdateSafety_WindowNotElapsedNoResolve(t *testing.T) {
	w := newUpdateSafetyWatcher()
	t0 := time.Now()

	w.Observe([]models.DockerHost{makeHost("h1", "c1", "sha256:aaa", 0)}, t0)
	w.Observe([]models.DockerHost{makeHost("h1", "c1", "sha256:bbb", 0)}, t0.Add(5*time.Second))

	// Well within the window.
	emit, resolve := w.Observe([]models.DockerHost{makeHost("h1", "c1", "sha256:bbb", 0)}, t0.Add(30*time.Second))
	if len(emit) != 0 || len(resolve) != 0 {
		t.Errorf("within window: want silent, got emit=%d resolve=%d", len(emit), len(resolve))
	}
}
