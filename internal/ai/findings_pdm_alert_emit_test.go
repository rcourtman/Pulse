package ai

import (
	"context"
	"testing"
	"time"
)

// TestPDMAlertEmit_FullCycle wires a real FindingsStore to a pdmAlertBridge
// and drives four observe cycles, mirroring the update-safety proof in
// findings_update_safety_emit_test.go.
//
// Cycle 1: no prior state -- nothing emitted; store unchanged.
// Cycle 2: node transitions to offline -- one finding emitted; store accepts.
// Cycle 3: node back to online -- resolve sentinel marks finding AutoResolved.
// Cycle 4: unknown status on a fresh resource -- nothing emitted; no seed.
func TestPDMAlertEmit_FullCycle(t *testing.T) {
	store := NewFindingsStore()
	src := &fakePDMSource{
		snapshots: [][]pdmResource{
			{pdmRes("dc-a", "node", "pve1", "online")},
			{pdmRes("dc-a", "node", "pve1", "offline")},
			{pdmRes("dc-a", "node", "pve1", "online")},
			{pdmRes("dc-a", "node", "pve2", "unknown")},
		},
	}
	b := newPDMAlertBridge(src)
	t0 := time.Now()

	wantKey := PDMAlertFindingPrefix + ":dc-a/node/pve1"

	// Cycle 1 -- baseline (no prior state).
	priorStoreCount := len(store.GetAll(nil))
	emit1, res1 := b.Observe(context.Background(), t0)
	if len(emit1) != 0 || len(res1) != 0 {
		t.Fatalf("cycle 1: want silent, got emit=%d resolve=%d", len(emit1), len(res1))
	}
	if got := len(store.GetAll(nil)); got != priorStoreCount {
		t.Fatalf("cycle 1: store count should be unchanged, was %d now %d", priorStoreCount, got)
	}

	// Cycle 2 -- node offline.
	emit2, res2 := b.Observe(context.Background(), t0.Add(5*time.Second))
	if len(emit2) != 1 {
		t.Fatalf("cycle 2: want 1 finding, got %d", len(emit2))
	}
	if len(res2) != 0 {
		t.Fatalf("cycle 2: want 0 resolves, got %d", len(res2))
	}
	if !store.Add(emit2[0]) {
		t.Fatal("cycle 2: store.Add should report new finding")
	}
	stored := store.Get(wantKey)
	if stored == nil {
		t.Fatalf("cycle 2: finding %q missing from store after Add", wantKey)
	}
	if !stored.IsActive() {
		t.Errorf("cycle 2: finding should be active, got %+v", stored)
	}
	if stored.Category != FindingCategoryReliability {
		t.Errorf("cycle 2 category: want %q, got %q", FindingCategoryReliability, stored.Category)
	}
	if stored.ResourceType != "node" {
		t.Errorf("cycle 2 resource_type: want %q, got %q", "node", stored.ResourceType)
	}
	if stored.Source != pdmAlertSourceLabel {
		t.Errorf("cycle 2 source: want %q, got %q", pdmAlertSourceLabel, stored.Source)
	}

	// Cycle 3 -- node back online -> resolve sentinel.
	emit3, res3 := b.Observe(context.Background(), t0.Add(10*time.Second))
	if len(emit3) != 0 {
		t.Errorf("cycle 3: want 0 findings, got %d", len(emit3))
	}
	if len(res3) != 1 {
		t.Fatalf("cycle 3: want 1 resolve sentinel, got %d", len(res3))
	}
	sentinel := res3[0]
	if sentinel.DedupKey != wantKey {
		t.Errorf("sentinel key: want %q, got %q", wantKey, sentinel.DedupKey)
	}
	if sentinel.Reason != pdmAlertResolveReason {
		t.Errorf("sentinel reason: want %q, got %q", pdmAlertResolveReason, sentinel.Reason)
	}
	if !store.ResolveWithReason(sentinel.DedupKey, sentinel.Reason) {
		t.Fatal("ResolveWithReason returned false -- finding missing or already resolved")
	}
	resolved := store.Get(wantKey)
	if resolved == nil {
		t.Fatal("resolved finding missing from store")
	}
	if resolved.IsActive() {
		t.Errorf("resolved finding still active: %+v", resolved)
	}
	if !resolved.AutoResolved {
		t.Errorf("AutoResolved: want true, got false")
	}
	if resolved.ResolveReason != pdmAlertResolveReason {
		t.Errorf("ResolveReason: want %q, got %q", pdmAlertResolveReason, resolved.ResolveReason)
	}

	// Cycle 4 -- unknown status arrives for a previously-unseen resource:
	// no emit, no seed (so a later actionable status will still seed silently).
	storeCountBefore := len(store.GetAll(nil))
	emit4, res4 := b.Observe(context.Background(), t0.Add(15*time.Second))
	if len(emit4) != 0 || len(res4) != 0 {
		t.Fatalf("cycle 4: want silent on unknown, got emit=%d resolve=%d", len(emit4), len(res4))
	}
	if got := len(store.GetAll(nil)); got != storeCountBefore {
		t.Errorf("cycle 4: store count should be unchanged, was %d now %d", storeCountBefore, got)
	}
}

// TestPDMAlertEmit_TwoResourcesDontCollideDedupKeys proves that offline
// transitions on two separate resources produce non-colliding finding IDs in
// the store, and that resolving one does not affect the other.
func TestPDMAlertEmit_TwoResourcesDontCollideDedupKeys(t *testing.T) {
	store := NewFindingsStore()
	src := &fakePDMSource{
		snapshots: [][]pdmResource{
			{
				pdmRes("dc1", "node", "pveA", "online"),
				pdmRes("dc1", "qemu", "vm-101", "running"),
			},
			{
				pdmRes("dc1", "node", "pveA", "offline"),
				pdmRes("dc1", "qemu", "vm-101", "stopped"),
			},
			{
				pdmRes("dc1", "node", "pveA", "online"),
				pdmRes("dc1", "qemu", "vm-101", "stopped"),
			},
		},
	}
	b := newPDMAlertBridge(src)
	t0 := time.Now()

	// Seed.
	b.Observe(context.Background(), t0)

	// Both go offline.
	emit2, _ := b.Observe(context.Background(), t0.Add(5*time.Second))
	if len(emit2) != 2 {
		t.Fatalf("want 2 findings for 2 offline resources, got %d", len(emit2))
	}
	for _, f := range emit2 {
		if !store.Add(f) {
			t.Errorf("store.Add returned false (duplicate?) for finding %q", f.ID)
		}
	}

	keyNode := PDMAlertFindingPrefix + ":dc1/node/pveA"
	keyVM := PDMAlertFindingPrefix + ":dc1/qemu/vm-101"
	fNode := store.Get(keyNode)
	fVM := store.Get(keyVM)
	if fNode == nil || fVM == nil {
		t.Fatalf("one or both findings missing: fNode=%v fVM=%v", fNode, fVM)
	}

	// Node back online; VM still stopped. Only node resolves.
	_, res3 := b.Observe(context.Background(), t0.Add(10*time.Second))
	if len(res3) != 1 {
		t.Fatalf("want 1 resolve sentinel for node only, got %d", len(res3))
	}
	if res3[0].DedupKey != keyNode {
		t.Errorf("resolve key: want %q, got %q", keyNode, res3[0].DedupKey)
	}
	store.ResolveWithReason(res3[0].DedupKey, res3[0].Reason)

	if store.Get(keyNode).IsActive() {
		t.Errorf("node finding should be resolved")
	}
	if !store.Get(keyVM).IsActive() {
		t.Errorf("vm finding should still be active")
	}
}
