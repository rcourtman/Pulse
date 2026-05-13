package ai

import (
	"context"
	"errors"
	"testing"
	"time"
)

// fakePDMSource returns scripted ResourceList snapshots, one per call.
type fakePDMSource struct {
	snapshots [][]pdmResource
	err       error
	idx       int
}

func (f *fakePDMSource) ResourceList(_ context.Context) ([]pdmResource, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.idx >= len(f.snapshots) {
		return f.snapshots[len(f.snapshots)-1], nil
	}
	snap := f.snapshots[f.idx]
	f.idx++
	return snap, nil
}

func pdmRes(remote, typ, name, status string) pdmResource {
	return pdmResource{
		ID:       remote + "/" + typ + "/" + name,
		RemoteID: remote,
		Name:     name,
		Type:     typ,
		Status:   status,
	}
}

// TestPDMAlertBridge_NilSourceIsNoOp verifies that a bridge constructed with
// a nil source emits nothing on Observe -- the patrol-loop wire-in relies on
// this for the MVP no-op behavior.
func TestPDMAlertBridge_NilSourceIsNoOp(t *testing.T) {
	b := newPDMAlertBridge(nil)
	emit, resolve := b.Observe(context.Background(), time.Now())
	if len(emit) != 0 || len(resolve) != 0 {
		t.Fatalf("nil source: want silent, got emit=%d resolve=%d", len(emit), len(resolve))
	}
}

// TestPDMAlertBridge_SeedOfflineOnlineCycle drives the bridge through three
// snapshots:
//   1. seed   -- node online, first observation, nothing emitted.
//   2. offline -- node transitions to offline, one finding emitted.
//   3. online  -- node returns to online, one resolve sentinel emitted.
func TestPDMAlertBridge_SeedOfflineOnlineCycle(t *testing.T) {
	src := &fakePDMSource{
		snapshots: [][]pdmResource{
			{pdmRes("datacenter-a", "node", "pve-edge-01", "online")},
			{pdmRes("datacenter-a", "node", "pve-edge-01", "offline")},
			{pdmRes("datacenter-a", "node", "pve-edge-01", "online")},
		},
	}
	b := newPDMAlertBridge(src)
	t0 := time.Now()

	emit1, res1 := b.Observe(context.Background(), t0)
	if len(emit1) != 0 || len(res1) != 0 {
		t.Fatalf("seed trip: want silent, got emit=%d resolve=%d", len(emit1), len(res1))
	}

	emit2, res2 := b.Observe(context.Background(), t0.Add(5*time.Second))
	if len(emit2) != 1 {
		t.Fatalf("offline trip: want 1 finding, got %d", len(emit2))
	}
	if len(res2) != 0 {
		t.Fatalf("offline trip: want 0 resolves, got %d", len(res2))
	}
	f := emit2[0]
	wantKey := PDMAlertFindingPrefix + ":datacenter-a/node/pve-edge-01"
	if f.ID != wantKey {
		t.Errorf("dedup id: want %q, got %q", wantKey, f.ID)
	}
	if f.Key != wantKey {
		t.Errorf("dedup key: want %q, got %q", wantKey, f.Key)
	}
	if f.ResourceType != "node" {
		t.Errorf("resource_type: want %q, got %q", "node", f.ResourceType)
	}
	if f.ResourceName != "pve-edge-01" {
		t.Errorf("resource_name: want %q, got %q", "pve-edge-01", f.ResourceName)
	}
	if f.Severity != FindingSeverityWarning {
		t.Errorf("severity: want %q, got %q", FindingSeverityWarning, f.Severity)
	}
	if f.Category != FindingCategoryReliability {
		t.Errorf("category: want %q, got %q", FindingCategoryReliability, f.Category)
	}
	if f.Source != pdmAlertSourceLabel {
		t.Errorf("source: want %q, got %q", pdmAlertSourceLabel, f.Source)
	}
	if f.Node != "datacenter-a" {
		t.Errorf("node: want %q, got %q", "datacenter-a", f.Node)
	}

	emit3, res3 := b.Observe(context.Background(), t0.Add(10*time.Second))
	if len(emit3) != 0 {
		t.Fatalf("online trip: want 0 findings, got %d", len(emit3))
	}
	if len(res3) != 1 {
		t.Fatalf("online trip: want 1 resolve sentinel, got %d", len(res3))
	}
	if res3[0].DedupKey != wantKey {
		t.Errorf("resolve DedupKey: want %q, got %q", wantKey, res3[0].DedupKey)
	}
	if res3[0].Reason != pdmAlertResolveReason {
		t.Errorf("resolve Reason: want %q, got %q", pdmAlertResolveReason, res3[0].Reason)
	}
}

// TestPDMAlertBridge_UnknownStatusIgnored verifies that resources reporting
// "unknown" status are not seeded as prior state and do not emit on any trip.
func TestPDMAlertBridge_UnknownStatusIgnored(t *testing.T) {
	src := &fakePDMSource{
		snapshots: [][]pdmResource{
			{pdmRes("dc1", "node", "pve1", "unknown")},
			{pdmRes("dc1", "node", "pve1", "unknown")},
			{pdmRes("dc1", "node", "pve1", "offline")},
		},
	}
	b := newPDMAlertBridge(src)
	t0 := time.Now()

	// First two trips: unknown -> never seeded, no emit.
	for i := 0; i < 2; i++ {
		emit, res := b.Observe(context.Background(), t0.Add(time.Duration(i)*time.Second))
		if len(emit) != 0 || len(res) != 0 {
			t.Fatalf("unknown trip %d: want silent, got emit=%d resolve=%d", i, len(emit), len(res))
		}
	}
	// Third trip: offline arrives as the first actionable observation -> seed,
	// no emit.
	emit3, res3 := b.Observe(context.Background(), t0.Add(2*time.Second))
	if len(emit3) != 0 || len(res3) != 0 {
		t.Fatalf("first actionable trip: want silent (seed), got emit=%d resolve=%d", len(emit3), len(res3))
	}
}

// TestPDMAlertBridge_SourceErrorIsSilent verifies that a source error short-
// circuits Observe without panicking or mutating prior-state.
func TestPDMAlertBridge_SourceErrorIsSilent(t *testing.T) {
	src := &fakePDMSource{err: errors.New("boom")}
	b := newPDMAlertBridge(src)
	emit, resolve := b.Observe(context.Background(), time.Now())
	if len(emit) != 0 || len(resolve) != 0 {
		t.Fatalf("source error: want silent, got emit=%d resolve=%d", len(emit), len(resolve))
	}
	if len(b.prior) != 0 {
		t.Errorf("source error: prior should not be mutated, got %d entries", len(b.prior))
	}
}

// TestPDMAlertBridge_VMStatusMapsToResourceTypeVM verifies that a PDM resource
// of type "qemu" surfaces as ResourceType "vm" on the finding, matching the
// vocabulary FindingsPanel already renders.
func TestPDMAlertBridge_VMStatusMapsToResourceTypeVM(t *testing.T) {
	src := &fakePDMSource{
		snapshots: [][]pdmResource{
			{pdmRes("dc1", "qemu", "vm-101", "running")},
			{pdmRes("dc1", "qemu", "vm-101", "stopped")},
		},
	}
	b := newPDMAlertBridge(src)
	t0 := time.Now()
	b.Observe(context.Background(), t0)

	emit, _ := b.Observe(context.Background(), t0.Add(time.Second))
	if len(emit) != 1 {
		t.Fatalf("stopped vm: want 1 finding, got %d", len(emit))
	}
	if emit[0].ResourceType != "vm" {
		t.Errorf("resource_type: want %q, got %q", "vm", emit[0].ResourceType)
	}
}
