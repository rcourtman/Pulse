package models

import "testing"

// TestBranchCovCephPoolStorage exercises the two branches of the free function
// CephPoolStorage: the early "no pools -> nil" return and the pool-projection
// loop. The projection itself delegates to StorageFromCephPool, so the
// cluster-health-driven status/active flags are asserted end-to-end through
// CephPoolStorage's returned slice (covering the HEALTH_ERR path that the
// existing StorageFromCephPool happy-path test does not reach).
func TestBranchCovCephPoolStorage(t *testing.T) {
	tests := []struct {
		name       string
		cluster    CephCluster
		wantNil    bool
		wantLen    int
		wantStatus string // asserted on every projected entry when wantNil is false
		wantActive bool
	}{
		{
			name:    "nil pools returns nil",
			cluster: CephCluster{Instance: "pve1", Health: "HEALTH_OK", Pools: nil},
			wantNil: true,
		},
		{
			name:    "empty (non-nil) pools slice returns nil",
			cluster: CephCluster{Instance: "pve1", Health: "HEALTH_OK", Pools: []CephPool{}},
			wantNil: true,
		},
		{
			name: "healthy cluster projects available active storage",
			cluster: CephCluster{
				Instance: "pve1",
				Health:   "HEALTH_OK",
				Pools:    []CephPool{{ID: 1, Name: "data"}},
			},
			wantLen:    1,
			wantStatus: "available",
			wantActive: true,
		},
		{
			name: "health_err cluster projects unavailable inactive storage",
			cluster: CephCluster{
				Instance: "pve1",
				Health:   "HEALTH_ERR",
				Pools:    []CephPool{{ID: 2, Name: "ssd"}},
			},
			wantLen:    1,
			wantStatus: "unavailable",
			wantActive: false,
		},
		{
			name: "multiple pools projected in order",
			cluster: CephCluster{
				Instance: "pve1",
				Health:   "HEALTH_WARN",
				Pools: []CephPool{
					{ID: 1, Name: "data"},
					{ID: 2, Name: "ssd"},
				},
			},
			wantLen:    2,
			wantStatus: "available",
			wantActive: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := CephPoolStorage(tc.cluster)
			if tc.wantNil {
				if got != nil {
					t.Fatalf("CephPoolStorage = %#v, want nil for empty/nil pools", got)
				}
				return
			}
			if got == nil {
				t.Fatal("CephPoolStorage = nil, want non-nil slice")
			}
			if len(got) != tc.wantLen {
				t.Fatalf("len = %d, want %d", len(got), tc.wantLen)
			}
			for i, st := range got {
				if st.Type != "ceph" {
					t.Errorf("entry %d Type = %q, want ceph", i, st.Type)
				}
				if !st.Shared || !st.Enabled {
					t.Errorf("entry %d flags Shared=%v Enabled=%v, want both true", i, st.Shared, st.Enabled)
				}
				if st.Status != tc.wantStatus {
					t.Errorf("entry %d Status = %q, want %q", i, st.Status, tc.wantStatus)
				}
				if st.Active != tc.wantActive {
					t.Errorf("entry %d Active = %v, want %v", i, st.Active, tc.wantActive)
				}
				if st.Name != tc.cluster.Pools[i].Name {
					t.Errorf("entry %d Name = %q, want %q (pool order not preserved)", i, st.Name, tc.cluster.Pools[i].Name)
				}
			}
		})
	}
}

// TestBranchCovCephPoolStorage_NamesFromEmptyPoolID covers the boundary where a
// CephPool has an empty name: StorageFromCephPool falls back to "pool-<id>".
// This is reached through CephPoolStorage and is not covered elsewhere.
func TestBranchCovCephPoolStorage_NamesFromEmptyPoolID(t *testing.T) {
	got := CephPoolStorage(CephCluster{
		Instance: "pve1",
		Health:   "HEALTH_OK",
		Pools:    []CephPool{{ID: 9, Name: ""}},
	})
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].Name != "pool-9" {
		t.Errorf("empty-named pool projected Name = %q, want pool-9", got[0].Name)
	}
	if got[0].Pool != "pool-9" {
		t.Errorf("empty-named pool projected Pool = %q, want pool-9", got[0].Pool)
	}
}

// TestBranchCovMergeTagColors covers every branch of State.MergeTagColors:
// the empty/nil early return (no lock, no map init, no LastUpdate bump), the
// nil-map initialization arm, the reuse-existing-map arm, and the per-entry
// tag/color normalization (TrimSpace + ToLower on keys, TrimSpace on values).
func TestBranchCovMergeTagColors(t *testing.T) {
	// Read state fields directly: MergeTagColors has already returned, the test
	// is single-goroutine, and reading the raw fields verifies the *true*
	// internal state (nil vs initialized map) rather than the post-processed
	// snapshot, which NormalizeCollections() turns into a non-nil empty map.
	t.Run("nil colors is a no-op and leaves map nil and LastUpdate untouched", func(t *testing.T) {
		state := &State{}
		before := state.LastUpdate
		state.MergeTagColors(nil)

		if state.PVETagColors != nil {
			t.Fatalf("nil colors should leave PVETagColors nil, got %#v", state.PVETagColors)
		}
		if state.LastUpdate != before {
			t.Fatal("nil colors must not touch LastUpdate")
		}
	})

	t.Run("empty colors is a no-op and leaves LastUpdate untouched", func(t *testing.T) {
		state := &State{}
		before := state.LastUpdate
		state.MergeTagColors(map[string]string{})

		if state.PVETagColors != nil {
			t.Fatalf("empty colors should leave PVETagColors nil, got %#v", state.PVETagColors)
		}
		if state.LastUpdate != before {
			t.Fatal("empty colors must not touch LastUpdate")
		}
	})

	t.Run("nil destination map is initialized and entries normalized", func(t *testing.T) {
		state := &State{}
		state.MergeTagColors(map[string]string{
			"  Production ": "  #ff0000  ",
			"\tBACKUP\t":    "#00ff00",
		})

		if state.PVETagColors == nil {
			t.Fatal("nil destination map should be initialized by a non-empty merge")
		}
		if got := state.PVETagColors["production"]; got != "#ff0000" {
			t.Errorf("production color = %q, want #ff0000 (tag trimmed+lowercased, color trimmed)", got)
		}
		if got := state.PVETagColors["backup"]; got != "#00ff00" {
			t.Errorf("backup color = %q, want #00ff00", got)
		}
		if _, ok := state.PVETagColors["  Production "]; ok {
			t.Error("un-normalized tag key should not be present")
		}
	})

	t.Run("existing destination map is reused and merged", func(t *testing.T) {
		state := NewState() // PVETagColors starts as a non-nil empty map
		state.MergeTagColors(map[string]string{"existing": "#111111"})
		state.MergeTagColors(map[string]string{"added": "#222222"})

		if got := state.PVETagColors["existing"]; got != "#111111" {
			t.Errorf("existing color = %q, want #111111 (prior merge must survive)", got)
		}
		if got := state.PVETagColors["added"]; got != "#222222" {
			t.Errorf("added color = %q, want #222222", got)
		}
		if len(state.PVETagColors) != 2 {
			t.Errorf("merged color count = %d, want 2", len(state.PVETagColors))
		}
	})

	t.Run("non-empty merge bumps LastUpdate", func(t *testing.T) {
		state := &State{}
		before := state.LastUpdate
		state.MergeTagColors(map[string]string{"x": "#000000"})

		if state.LastUpdate == before {
			t.Fatal("LastUpdate should advance after a non-empty merge")
		}
	})
}

// TestBranchCovStorageNormalizeCollections covers both arms of each conditional
// in Storage.NormalizeCollections: the nil-collection initialization arms
// (Nodes, NodeIDs) and both sides of the ZFSPool branch (nil pointer stays nil;
// non-nil pointer is recursively normalized, including its own nil Devices).
func TestBranchCovStorageNormalizeCollections(t *testing.T) {
	t.Run("nil slices initialized and nil zfs left untouched", func(t *testing.T) {
		s := Storage{} // Nodes, NodeIDs, ZFSPool all nil
		out := s.NormalizeCollections()

		if out.Nodes == nil {
			t.Error("Nodes should be initialized to non-nil empty slice")
		}
		if len(out.Nodes) != 0 {
			t.Errorf("Nodes len = %d, want 0", len(out.Nodes))
		}
		if out.NodeIDs == nil {
			t.Error("NodeIDs should be initialized to non-nil empty slice")
		}
		if out.ZFSPool != nil {
			t.Errorf("ZFSPool = %#v, want nil (must not synthesize a pointer)", out.ZFSPool)
		}
	})

	t.Run("non-nil slices preserved and zfs recursively normalized", func(t *testing.T) {
		s := Storage{
			Nodes:   []string{"n1"},
			NodeIDs: []string{"id1"},
			ZFSPool: &ZFSPool{Name: "tank"}, // Devices nil -> must be normalized
		}
		out := s.NormalizeCollections()

		if len(out.Nodes) != 1 || out.Nodes[0] != "n1" {
			t.Errorf("Nodes = %#v, want [n1] (already-set slice must be preserved)", out.Nodes)
		}
		if len(out.NodeIDs) != 1 || out.NodeIDs[0] != "id1" {
			t.Errorf("NodeIDs = %#v, want [id1]", out.NodeIDs)
		}
		if out.ZFSPool == nil {
			t.Fatal("ZFSPool should remain non-nil")
		}
		if out.ZFSPool.Devices == nil {
			t.Error("nested ZFSPool.Devices should be initialized to non-nil empty slice")
		}
		if len(out.ZFSPool.Devices) != 0 {
			t.Errorf("ZFSPool.Devices len = %d, want 0", len(out.ZFSPool.Devices))
		}
	})
}

// TestBranchCovCephClusterNormalizeCollections covers the nil-initialization
// arms for InstanceAliases/Pools/Services and the else path where pre-set
// slices pass through unchanged.
func TestBranchCovCephClusterNormalizeCollections(t *testing.T) {
	t.Run("nil collections initialized", func(t *testing.T) {
		c := CephCluster{}
		out := c.NormalizeCollections()

		if out.InstanceAliases == nil || out.Pools == nil || out.Services == nil {
			t.Fatalf("nil collections should be initialized: %#v", out)
		}
		if len(out.InstanceAliases) != 0 || len(out.Pools) != 0 || len(out.Services) != 0 {
			t.Errorf("initialized collections should be empty: %#v", out)
		}
	})

	t.Run("existing collections preserved", func(t *testing.T) {
		c := CephCluster{
			InstanceAliases: []string{"a"},
			Pools:           []CephPool{{ID: 1, Name: "data"}},
			Services:        []CephServiceStatus{{Type: "mon", Running: 1, Total: 1}},
		}
		out := c.NormalizeCollections()

		if len(out.InstanceAliases) != 1 || out.InstanceAliases[0] != "a" {
			t.Errorf("InstanceAliases = %#v, want [a]", out.InstanceAliases)
		}
		if len(out.Pools) != 1 || out.Pools[0].Name != "data" {
			t.Errorf("Pools = %#v, want unchanged", out.Pools)
		}
		if len(out.Services) != 1 || out.Services[0].Type != "mon" {
			t.Errorf("Services = %#v, want unchanged", out.Services)
		}
	})
}

// TestBranchCovPBSInstanceNormalizeCollections covers the nil-initialization
// arms of all seven PBS slice fields and the Datastores normalization loop body
// (a datastore with nil Namespaces is recursed into and normalized), plus the
// else path where pre-populated job slices pass through unchanged.
func TestBranchCovPBSInstanceNormalizeCollections(t *testing.T) {
	t.Run("nil collections initialized and datastore recursion runs", func(t *testing.T) {
		inst := PBSInstance{
			Datastores: []PBSDatastore{{Name: "store1"}}, // Namespaces nil
		}
		out := inst.NormalizeCollections()

		if out.Datastores == nil || len(out.Datastores) != 1 {
			t.Fatalf("Datastores = %#v, want single entry preserved", out.Datastores)
		}
		if out.Datastores[0].Namespaces == nil {
			t.Error("nested Datastore.Namespaces should be initialized by the loop body")
		}
		for field, got := range map[string][]struct{}{
			"BackupJobs":        toIfaces(out.BackupJobs),
			"SyncJobs":          toIfaces(out.SyncJobs),
			"VerifyJobs":        toIfaces(out.VerifyJobs),
			"PruneJobs":         toIfaces(out.PruneJobs),
			"GarbageJobs":       toIfaces(out.GarbageJobs),
			"JobHealthEvidence": toIfaces(out.JobHealthEvidence),
		} {
			if got == nil {
				t.Errorf("%s should be initialized to non-nil empty slice", field)
			}
		}
	})

	t.Run("all nil fields become empty slices", func(t *testing.T) {
		out := PBSInstance{}.NormalizeCollections()
		checks := map[string][]struct{}{
			"Datastores":        toIfaces(out.Datastores),
			"BackupJobs":        toIfaces(out.BackupJobs),
			"SyncJobs":          toIfaces(out.SyncJobs),
			"VerifyJobs":        toIfaces(out.VerifyJobs),
			"PruneJobs":         toIfaces(out.PruneJobs),
			"GarbageJobs":       toIfaces(out.GarbageJobs),
			"JobHealthEvidence": toIfaces(out.JobHealthEvidence),
		}
		for field, got := range checks {
			if got == nil {
				t.Errorf("%s should be initialized to non-nil empty slice", field)
			}
			if len(got) != 0 {
				t.Errorf("%s len = %d, want 0", field, len(got))
			}
		}
	})

	t.Run("existing job slices preserved unchanged", func(t *testing.T) {
		inst := PBSInstance{
			BackupJobs:  []PBSBackupJob{{ID: "j1"}},
			SyncJobs:    []PBSSyncJob{{ID: "s1"}},
			VerifyJobs:  []PBSVerifyJob{{ID: "v1"}},
			PruneJobs:   []PBSPruneJob{{ID: "p1"}},
			GarbageJobs: []PBSGarbageJob{{ID: "g1"}},
		}
		out := inst.NormalizeCollections()

		if len(out.BackupJobs) != 1 || out.BackupJobs[0].ID != "j1" {
			t.Errorf("BackupJobs = %#v, want [j1]", out.BackupJobs)
		}
		if len(out.SyncJobs) != 1 || out.SyncJobs[0].ID != "s1" {
			t.Errorf("SyncJobs = %#v, want [s1]", out.SyncJobs)
		}
		if len(out.VerifyJobs) != 1 || out.VerifyJobs[0].ID != "v1" {
			t.Errorf("VerifyJobs = %#v, want [v1]", out.VerifyJobs)
		}
		if len(out.PruneJobs) != 1 || out.PruneJobs[0].ID != "p1" {
			t.Errorf("PruneJobs = %#v, want [p1]", out.PruneJobs)
		}
		if len(out.GarbageJobs) != 1 || out.GarbageJobs[0].ID != "g1" {
			t.Errorf("GarbageJobs = %#v, want [g1]", out.GarbageJobs)
		}
	})
}

// toIfaces is a tiny local helper that lets the table-driven collection checks
// uniformly assert non-nil/length without reflecting over each distinct slice
// type. It preserves nil-ness (a nil input yields a nil result).
func toIfaces[T any](s []T) []struct{} {
	if s == nil {
		return nil
	}
	out := make([]struct{}, len(s))
	return out
}
