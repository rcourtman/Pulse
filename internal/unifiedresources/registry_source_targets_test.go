package unifiedresources

import (
	"reflect"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func sourceTargetsFixtureRegistry() *ResourceRegistry {
	rr := NewRegistry(nil)
	now := time.Date(2026, 8, 16, 10, 0, 0, 0, time.UTC)

	rr.IngestSnapshot(models.StateSnapshot{
		Nodes: []models.Node{
			{
				ID:              "mock-cluster-pve1",
				NodeIdentity:    "mock-cluster-pve1",
				Name:            "pve1",
				Instance:        "Core Fabric",
				ClusterName:     "Core Fabric",
				IsClusterMember: true,
				Status:          "online",
				LastSeen:        now,
			},
			{
				ID:              "mock-cluster-pve2",
				NodeIdentity:    "mock-cluster-pve2",
				Name:            "pve2",
				Instance:        "Core Fabric",
				ClusterName:     "Core Fabric",
				IsClusterMember: true,
				Status:          "online",
				LastSeen:        now,
			},
		},
		VMs: []models.VM{
			{ID: "Core Fabric:pve1:100", VMID: 100, Name: "web-01", Node: "pve1", Instance: "Core Fabric", Status: "running", LastSeen: now},
			{ID: "Core Fabric:pve2:101", VMID: 101, Name: "db-01", Node: "pve2", Instance: "Core Fabric", Status: "running", LastSeen: now},
		},
		Containers: []models.Container{
			{ID: "Core Fabric:pve1:104", VMID: 104, Name: "auth-01", Node: "pve1", Instance: "Core Fabric", Status: "running", LastSeen: now},
		},
	})
	return rr
}

// The cached inverse index must resolve exactly the same metrics targets as
// the legacy per-resource scan over every bySource mapping.
func TestMetricsTargetIndexMatchesLegacyScan(t *testing.T) {
	rr := sourceTargetsFixtureRegistry()

	ids := make([]string, 0)
	for _, r := range rr.List() {
		ids = append(ids, r.ID)
	}
	if len(ids) < 5 {
		t.Fatalf("fixture too small: %d resources", len(ids))
	}

	// While views are dirty the legacy scan answers.
	legacy := make(map[string]*MetricsTarget, len(ids))
	rr.mu.Lock()
	if !rr.viewsDirty {
		rr.viewsDirty = true
	}
	rr.cachedSourceTargets = nil
	for _, id := range ids {
		legacy[id] = rr.metricsTargetForResourceLocked(id)
	}
	rr.mu.Unlock()

	// A view read rebuilds the caches, activating the index path.
	rr.VMs()
	rr.mu.RLock()
	if rr.viewsDirty {
		rr.mu.RUnlock()
		t.Fatal("views still dirty after rebuild; index path not active")
	}
	if rr.cachedSourceTargets == nil {
		rr.mu.RUnlock()
		t.Fatal("cachedSourceTargets not built by rebuildViews")
	}
	indexed := make(map[string]*MetricsTarget, len(ids))
	for _, id := range ids {
		indexed[id] = rr.metricsTargetForResourceLocked(id)
	}
	rr.mu.RUnlock()

	for _, id := range ids {
		if !reflect.DeepEqual(legacy[id], indexed[id]) {
			t.Fatalf("metrics target diverged for %s:\nlegacy:  %+v\nindexed: %+v", id, legacy[id], indexed[id])
		}
	}
}

// Any bySource mutation must drop the cached inverse index immediately:
// batch ingests release the lock between records and only mark viewsDirty in
// their epilogue, so a reader interleaving mid-batch must fall back to the
// live scan rather than being served pre-batch targets.
func TestSourceTargetIndexInvalidatesOnIngest(t *testing.T) {
	rr := sourceTargetsFixtureRegistry()

	rr.VMs()
	rr.mu.RLock()
	built := rr.cachedSourceTargets != nil
	rr.mu.RUnlock()
	if !built {
		t.Fatal("view read did not build the source-target index")
	}

	rr.IngestRecords(SourceAvailability, []IngestRecord{{
		SourceID: "avail-probe-1",
		Resource: Resource{Type: ResourceTypeAgent, Name: "probe"},
	}})

	// The epilogue marks views dirty, but the index must already have been
	// dropped by the per-record mutation path, not merely masked by the
	// dirty flag.
	rr.mu.RLock()
	stillCached := rr.cachedSourceTargets != nil
	rr.mu.RUnlock()
	if stillCached {
		t.Fatal("bySource mutation left the stale source-target index in place")
	}
}

// View-embedded metrics targets must match what the public per-ID API
// resolves for the same resource.
func TestRebuiltViewMetricsTargetsMatchPublicAPI(t *testing.T) {
	rr := sourceTargetsFixtureRegistry()

	for _, vm := range rr.VMs() {
		fromAPI := BuildMetricsTargetForRegistry(rr, vm.ID())
		if !reflect.DeepEqual(vm.MetricsTarget(), fromAPI) {
			t.Fatalf("VM %s view target %+v != API target %+v", vm.ID(), vm.MetricsTarget(), fromAPI)
		}
	}
	for _, ct := range rr.Containers() {
		fromAPI := BuildMetricsTargetForRegistry(rr, ct.ID())
		if !reflect.DeepEqual(ct.MetricsTarget(), fromAPI) {
			t.Fatalf("container %s view target %+v != API target %+v", ct.ID(), ct.MetricsTarget(), fromAPI)
		}
	}
}
