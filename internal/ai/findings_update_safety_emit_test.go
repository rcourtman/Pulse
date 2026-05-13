package ai

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// TestUpdateSafetyEmit_FullCycle wires a real FindingsStore to an
// UpdateSafetyWatcher and drives three observe cycles, mirroring the
// storm-throttler proof in findings_storm_emit_test.go.
//
// Cycle 1: baseline -- no findings emitted.
// Cycle 2: digest changed -- watcher emits one finding; store accepts it.
// Cycle 3: same digest, window elapsed, no restarts -- watcher returns a
//          resolve sentinel; ResolveWithReason marks the finding AutoResolved.
func TestUpdateSafetyEmit_FullCycle(t *testing.T) {
	store := NewFindingsStore()
	w := newUpdateSafetyWatcher()
	t0 := time.Now()

	hostA := "host-a"
	ctrA := "ctr-1"
	key := UpdateSafetyFindingPrefix + ":" + hostA + "/" + ctrA

	// Cycle 1 -- baseline.
	emit1, res1 := w.Observe([]models.DockerHost{makeHost(hostA, ctrA, "sha256:aaa", 0)}, t0)
	if len(emit1) != 0 || len(res1) != 0 {
		t.Fatalf("cycle 1: want silent, got emit=%d resolve=%d", len(emit1), len(res1))
	}

	// Cycle 2 -- digest changed.
	emit2, res2 := w.Observe([]models.DockerHost{makeHost(hostA, ctrA, "sha256:bbb", 0)}, t0.Add(5*time.Second))
	if len(emit2) != 1 {
		t.Fatalf("cycle 2: want 1 finding, got %d", len(emit2))
	}
	if len(res2) != 0 {
		t.Fatalf("cycle 2: want 0 resolves, got %d", len(res2))
	}

	// Feed finding into the real store.
	isNew := store.Add(emit2[0])
	if !isNew {
		t.Fatal("cycle 2: store.Add should report new finding")
	}
	stored := store.Get(key)
	if stored == nil {
		t.Fatalf("cycle 2: finding %q missing from store after Add", key)
	}
	if !stored.IsActive() {
		t.Errorf("cycle 2: finding should be active, got %+v", stored)
	}
	if stored.Category != FindingCategoryReliability {
		t.Errorf("cycle 2 category: want %q, got %q", FindingCategoryReliability, stored.Category)
	}
	if stored.ResourceType != "app-container" {
		t.Errorf("cycle 2 resource_type: want %q, got %q", "app-container", stored.ResourceType)
	}

	// Cycle 3 -- same digest, window elapsed, no restarts.
	afterWindow := t0.Add(5*time.Second + updateSafetyVerifyWindow + time.Second)
	emit3, res3 := w.Observe([]models.DockerHost{makeHost(hostA, ctrA, "sha256:bbb", 0)}, afterWindow)
	if len(emit3) != 0 {
		t.Errorf("cycle 3: want 0 findings, got %d", len(emit3))
	}
	if len(res3) != 1 {
		t.Fatalf("cycle 3: want 1 resolve sentinel, got %d", len(res3))
	}
	sentinel := res3[0]
	if sentinel.DedupKey != key {
		t.Errorf("sentinel key: want %q, got %q", key, sentinel.DedupKey)
	}
	if sentinel.Reason != updateSafetyResolveReason {
		t.Errorf("sentinel reason: want %q, got %q", updateSafetyResolveReason, sentinel.Reason)
	}

	// Route sentinel through ResolveWithReason.
	if !store.ResolveWithReason(sentinel.DedupKey, sentinel.Reason) {
		t.Fatal("ResolveWithReason returned false -- finding missing or already resolved")
	}
	resolved := store.Get(key)
	if resolved == nil {
		t.Fatal("resolved finding missing from store")
	}
	if resolved.IsActive() {
		t.Errorf("resolved finding still active: %+v", resolved)
	}
	if !resolved.AutoResolved {
		t.Errorf("AutoResolved: want true, got false")
	}
	if resolved.ResolveReason != updateSafetyResolveReason {
		t.Errorf("ResolveReason: want %q, got %q", updateSafetyResolveReason, resolved.ResolveReason)
	}
}

// TestUpdateSafetyEmit_TwoContainersDontCollideDedupKeys proves that digest
// changes on two separate containers produce non-colliding finding IDs in the
// store, and that resolving one does not affect the other.
func TestUpdateSafetyEmit_TwoContainersDontCollideDedupKeys(t *testing.T) {
	store := NewFindingsStore()
	w := newUpdateSafetyWatcher()
	t0 := time.Now()

	hosts1 := []models.DockerHost{
		{
			ID:       "h1",
			Hostname: "h1.host",
			Containers: []models.DockerContainer{
				{ID: "cX", Name: "alpha", ImageDigest: "sha256:aaa", RestartCount: 0},
				{ID: "cY", Name: "beta", ImageDigest: "sha256:zzz", RestartCount: 0},
			},
		},
	}
	w.Observe(hosts1, t0)

	// Change both digests.
	hosts2 := []models.DockerHost{
		{
			ID:       "h1",
			Hostname: "h1.host",
			Containers: []models.DockerContainer{
				{ID: "cX", Name: "alpha", ImageDigest: "sha256:bbb", RestartCount: 0},
				{ID: "cY", Name: "beta", ImageDigest: "sha256:yyy", RestartCount: 0},
			},
		},
	}
	emit2, _ := w.Observe(hosts2, t0.Add(5*time.Second))
	if len(emit2) != 2 {
		t.Fatalf("want 2 findings for 2 changed digests, got %d", len(emit2))
	}
	for _, f := range emit2 {
		if !store.Add(f) {
			t.Errorf("store.Add returned false (duplicate?) for finding %q", f.ID)
		}
	}

	keyX := UpdateSafetyFindingPrefix + ":h1/cX"
	keyY := UpdateSafetyFindingPrefix + ":h1/cY"
	fX := store.Get(keyX)
	fY := store.Get(keyY)
	if fX == nil || fY == nil {
		t.Fatalf("one or both findings missing: fX=%v fY=%v", fX, fY)
	}

	// Resolve cX via sentinel.
	afterWindow := t0.Add(5*time.Second + updateSafetyVerifyWindow + time.Second)
	// Only feed cX into the next observe trip.
	hostsX := []models.DockerHost{
		{ID: "h1", Hostname: "h1.host", Containers: []models.DockerContainer{
			{ID: "cX", Name: "alpha", ImageDigest: "sha256:bbb", RestartCount: 0},
		}},
	}
	_, res3 := w.Observe(hostsX, afterWindow)
	var sentinelX *resolveSentinel
	for i := range res3 {
		if res3[i].DedupKey == keyX {
			sentinelX = &res3[i]
			break
		}
	}
	if sentinelX == nil {
		t.Fatal("expected resolve sentinel for cX, got none")
	}
	store.ResolveWithReason(sentinelX.DedupKey, sentinelX.Reason)

	// cX should be resolved; cY should still be active.
	if store.Get(keyX).IsActive() {
		t.Errorf("cX finding should be resolved")
	}
	if !store.Get(keyY).IsActive() {
		t.Errorf("cY finding should still be active")
	}
}

// TestUpdateSafetyEmit_EscalationUpdatesExistingFinding proves that when a
// restart occurs after the initial Info emission, re-adding the finding with
// Warning severity updates the existing record (TimesRaised advances, severity
// escalates) rather than spawning a duplicate.
func TestUpdateSafetyEmit_EscalationUpdatesExistingFinding(t *testing.T) {
	store := NewFindingsStore()
	w := newUpdateSafetyWatcher()
	t0 := time.Now()

	// Trip 1: baseline.
	w.Observe([]models.DockerHost{makeHost("h2", "c1", "sha256:aaa", 0)}, t0)

	// Trip 2: digest changed, no restarts -- Info.
	emit2, _ := w.Observe([]models.DockerHost{makeHost("h2", "c1", "sha256:bbb", 0)}, t0.Add(5*time.Second))
	if len(emit2) != 1 {
		t.Fatalf("trip 2: want 1 finding, got %d", len(emit2))
	}
	store.Add(emit2[0])
	key := UpdateSafetyFindingPrefix + ":h2/c1"
	afterFirst := store.Get(key)
	if afterFirst == nil {
		t.Fatal("finding missing after first add")
	}
	if afterFirst.Severity != FindingSeverityInfo {
		t.Errorf("first emit severity: want Info, got %q", afterFirst.Severity)
	}
	raisedAfterFirst := afterFirst.TimesRaised

	// Trip 3: restart count increased -- Warning escalation.
	emit3, _ := w.Observe([]models.DockerHost{makeHost("h2", "c1", "sha256:bbb", 3)}, t0.Add(20*time.Second))
	if len(emit3) != 1 {
		t.Fatalf("trip 3: want 1 escalated finding, got %d", len(emit3))
	}
	if emit3[0].Severity != FindingSeverityWarning {
		t.Errorf("escalated severity: want Warning, got %q", emit3[0].Severity)
	}
	store.Add(emit3[0])

	// Same finding ID -- should be updated, not duplicated.
	afterEscalation := store.Get(key)
	if afterEscalation == nil {
		t.Fatal("finding missing after escalation add")
	}
	if afterEscalation.Severity != FindingSeverityWarning {
		t.Errorf("post-escalation severity: want Warning, got %q", afterEscalation.Severity)
	}
	if afterEscalation.TimesRaised <= raisedAfterFirst {
		t.Errorf("TimesRaised should advance: was %d, now %d", raisedAfterFirst, afterEscalation.TimesRaised)
	}
}
