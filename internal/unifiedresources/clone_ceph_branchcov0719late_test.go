package unifiedresources

import "testing"

// Branch-coverage tests for the Ceph clone helpers in clone.go.
//
// These exercise the defensive nil/empty arms and the deep-copy
// independence guarantee for the seven Ceph-specific cloners:
//
//   - cloneHostCephHealthMeta
//   - cloneHostCephMonitorMapMeta
//   - cloneHostCephMonitorMetaSlice
//   - cloneHostCephPoolMetaSlice
//   - cloneHostCephServiceMetaSlice
//   - cloneCephPoolMetaSlice
//   - cloneCephServiceMetaSlice
//
// For each cloner we assert (a) the nil/empty defensive arm produces a
// safe value without panic, and (b) a populated input clones equal-by-value
// AND remains fully independent when the clone (and any nested slice/map
// element) is mutated.

// --- cloneHostCephHealthMeta ---

func TestCloneCephBranchcov0719late_HealthMeta_Empty(t *testing.T) {
	in := HostCephHealthMeta{}
	out := cloneHostCephHealthMeta(in)

	if out.Status != "" {
		t.Errorf("empty input Status: got %q, want empty", out.Status)
	}
	// cloneHostCephHealthMeta defensively allocates empty maps/slices even
	// when the input fields are nil, so callers never need nil-checks.
	if out.Checks == nil {
		t.Error("empty input should still produce non-nil Checks map")
	}
	if len(out.Checks) != 0 {
		t.Errorf("empty input Checks len: got %d, want 0", len(out.Checks))
	}
	if out.Summary == nil {
		t.Error("empty input should still produce non-nil Summary slice")
	}
	if len(out.Summary) != 0 {
		t.Errorf("empty input Summary len: got %d, want 0", len(out.Summary))
	}
}

func TestCloneCephBranchcov0719late_HealthMeta_Isolation(t *testing.T) {
	in := HostCephHealthMeta{
		Status: "HEALTH_WARN",
		Checks: map[string]HostCephCheckMeta{
			"POOL_NO_REDUNDANCY": {
				Severity: "WARNING",
				Message:  "no redundancy",
				Detail:   []string{"pool=foo", "min_size=1"},
			},
		},
		Summary: []HostCephHealthSummaryMeta{
			{Severity: "WARNING", Message: "1 pool has no redundancy"},
		},
	}
	cloned := cloneHostCephHealthMeta(in)

	// Equal-by-value assertions.
	if cloned.Status != "HEALTH_WARN" {
		t.Errorf("Status: got %q, want HEALTH_WARN", cloned.Status)
	}
	if got := cloned.Checks["POOL_NO_REDUNDANCY"].Severity; got != "WARNING" {
		t.Errorf("Checks[POOL_NO_REDUNDANCY].Severity: got %q, want WARNING", got)
	}
	if got := cloned.Checks["POOL_NO_REDUNDANCY"].Message; got != "no redundancy" {
		t.Errorf("Checks[POOL_NO_REDUNDANCY].Message: got %q, want 'no redundancy'", got)
	}
	if got := len(cloned.Checks["POOL_NO_REDUNDANCY"].Detail); got != 2 {
		t.Errorf("Checks[POOL_NO_REDUNDANCY].Detail len: got %d, want 2", got)
	}
	if got := cloned.Checks["POOL_NO_REDUNDANCY"].Detail[0]; got != "pool=foo" {
		t.Errorf("Checks[POOL_NO_REDUNDANCY].Detail[0]: got %q, want 'pool=foo'", got)
	}
	if len(cloned.Summary) != 1 {
		t.Fatalf("Summary len: got %d, want 1", len(cloned.Summary))
	}
	if cloned.Summary[0].Severity != "WARNING" || cloned.Summary[0].Message != "1 pool has no redundancy" {
		t.Errorf("Summary[0]: got %+v, want WARNING/'1 pool has no redundancy'", cloned.Summary[0])
	}

	// Mutate clone's map (add entry) — must not affect original.
	cloned.Checks["NEW_CHECK"] = HostCephCheckMeta{Severity: "NEW"}
	if _, exists := in.Checks["NEW_CHECK"]; exists {
		t.Error("adding key to cloned Checks must not propagate to original")
	}

	// Mutate clone's map entry field via copy-out / put-back (required
	// because Go disallows direct assignment to a struct field in a map).
	c := cloned.Checks["POOL_NO_REDUNDANCY"]
	c.Severity = "MUTATED_SEV"
	c.Message = "MUTATED_MSG"
	c.Detail[0] = "MUTATED_DETAIL"
	cloned.Checks["POOL_NO_REDUNDANCY"] = c

	if in.Checks["POOL_NO_REDUNDANCY"].Severity == "MUTATED_SEV" {
		t.Error("mutating cloned Checks entry Severity must not propagate to original")
	}
	if in.Checks["POOL_NO_REDUNDANCY"].Message == "MUTATED_MSG" {
		t.Error("mutating cloned Checks entry Message must not propagate to original")
	}
	if in.Checks["POOL_NO_REDUNDANCY"].Detail[0] == "MUTATED_DETAIL" {
		t.Error("mutating cloned Checks entry Detail slice element must not propagate to original")
	}

	// Mutate clone's Summary slice — must not affect original.
	cloned.Summary[0].Severity = "MUTATED"
	cloned.Summary[0].Message = "MUTATED"
	if in.Summary[0].Severity == "MUTATED" || in.Summary[0].Message == "MUTATED" {
		t.Error("mutating cloned Summary must not propagate to original")
	}
}

// --- cloneHostCephMonitorMapMeta ---

func TestCloneCephBranchcov0719late_MonMap_Empty(t *testing.T) {
	in := HostCephMonitorMapMeta{}
	out := cloneHostCephMonitorMapMeta(in)

	if out.Epoch != 0 || out.NumMons != 0 {
		t.Errorf("empty input: got Epoch=%d NumMons=%d, want zeros", out.Epoch, out.NumMons)
	}
	if out.Monitors != nil {
		t.Errorf("empty input Monitors: got %v, want nil", out.Monitors)
	}
}

func TestCloneCephBranchcov0719late_MonMap_Isolation(t *testing.T) {
	in := HostCephMonitorMapMeta{
		Epoch:   3,
		NumMons: 1,
		Monitors: []HostCephMonitorMeta{
			{Name: "mon.a", Rank: 0, Addr: "10.0.0.1:6789", Status: "ok"},
		},
	}
	cloned := cloneHostCephMonitorMapMeta(in)

	if cloned.Epoch != 3 || cloned.NumMons != 1 {
		t.Errorf("scalar fields: got Epoch=%d NumMons=%d, want 3/1", cloned.Epoch, cloned.NumMons)
	}
	if len(cloned.Monitors) != 1 {
		t.Fatalf("Monitors len: got %d, want 1", len(cloned.Monitors))
	}
	if cloned.Monitors[0].Name != "mon.a" || cloned.Monitors[0].Addr != "10.0.0.1:6789" {
		t.Errorf("Monitors[0]: got %+v, want Name=mon.a Addr=10.0.0.1:6789", cloned.Monitors[0])
	}

	// Mutate clone's Monitors slice — must not affect original.
	cloned.Monitors[0].Name = "MUTATED"
	cloned.Monitors[0].Status = "DOWN"
	if in.Monitors[0].Name == "MUTATED" || in.Monitors[0].Status == "DOWN" {
		t.Error("mutating cloned Monitors entry must not propagate to original")
	}
}

// --- cloneHostCephMonitorMetaSlice ---

func TestCloneCephBranchcov0719late_HostMonSlice_Nil(t *testing.T) {
	if got := cloneHostCephMonitorMetaSlice(nil); got != nil {
		t.Errorf("nil input: got %v, want nil", got)
	}
}

func TestCloneCephBranchcov0719late_HostMonSlice_Empty(t *testing.T) {
	in := []HostCephMonitorMeta{}
	out := cloneHostCephMonitorMetaSlice(in)
	if out == nil {
		t.Error("empty (non-nil) input should produce non-nil empty slice")
	}
	if len(out) != 0 {
		t.Errorf("empty input: got len=%d, want 0", len(out))
	}
}

func TestCloneCephBranchcov0719late_HostMonSlice_Isolation(t *testing.T) {
	in := []HostCephMonitorMeta{
		{Name: "mon.a", Rank: 0, Addr: "10.0.0.1:6789", Status: "ok"},
		{Name: "mon.b", Rank: 1, Addr: "10.0.0.2:6789", Status: "ok"},
	}
	cloned := cloneHostCephMonitorMetaSlice(in)

	if len(cloned) != 2 {
		t.Fatalf("len: got %d, want 2", len(cloned))
	}
	if cloned[0].Name != "mon.a" || cloned[1].Name != "mon.b" {
		t.Errorf("values: got %q / %q, want mon.a / mon.b", cloned[0].Name, cloned[1].Name)
	}
	if cloned[0].Addr != "10.0.0.1:6789" || cloned[1].Rank != 1 {
		t.Errorf(" Addr/Rank: got Addr=%q Rank=%d, want 10.0.0.1:6789 / 1", cloned[0].Addr, cloned[1].Rank)
	}

	cloned[0].Name = "MUTATED"
	cloned[0].Rank = 99
	if in[0].Name == "MUTATED" || in[0].Rank == 99 {
		t.Error("mutating cloned element must not propagate to original")
	}
}

// --- cloneHostCephPoolMetaSlice ---

func TestCloneCephBranchcov0719late_HostPoolSlice_Nil(t *testing.T) {
	if got := cloneHostCephPoolMetaSlice(nil); got != nil {
		t.Errorf("nil input: got %v, want nil", got)
	}
}

func TestCloneCephBranchcov0719late_HostPoolSlice_Empty(t *testing.T) {
	in := []HostCephPoolMeta{}
	out := cloneHostCephPoolMetaSlice(in)
	if out == nil {
		t.Error("empty (non-nil) input should produce non-nil empty slice")
	}
	if len(out) != 0 {
		t.Errorf("empty input: got len=%d, want 0", len(out))
	}
}

func TestCloneCephBranchcov0719late_HostPoolSlice_Isolation(t *testing.T) {
	in := []HostCephPoolMeta{
		{ID: 1, Name: "pool-1", BytesUsed: 1024, BytesAvailable: 2048, Objects: 5, PercentUsed: 0.33},
		{ID: 2, Name: "pool-2", BytesUsed: 4096, BytesAvailable: 8192, Objects: 7, PercentUsed: 0.5},
	}
	cloned := cloneHostCephPoolMetaSlice(in)

	if len(cloned) != 2 {
		t.Fatalf("len: got %d, want 2", len(cloned))
	}
	if cloned[0].Name != "pool-1" || cloned[1].Name != "pool-2" {
		t.Errorf("names: got %q / %q, want pool-1 / pool-2", cloned[0].Name, cloned[1].Name)
	}
	if cloned[0].BytesUsed != 1024 || cloned[1].Objects != 7 {
		t.Errorf("numeric values: got BytesUsed=%d Objects=%d, want 1024 / 7", cloned[0].BytesUsed, cloned[1].Objects)
	}

	cloned[0].Name = "MUTATED"
	cloned[0].BytesUsed = 999
	if in[0].Name == "MUTATED" || in[0].BytesUsed == 999 {
		t.Error("mutating cloned element must not propagate to original")
	}
}

// --- cloneHostCephServiceMetaSlice ---

func TestCloneCephBranchcov0719late_HostSvcSlice_Nil(t *testing.T) {
	if got := cloneHostCephServiceMetaSlice(nil); got != nil {
		t.Errorf("nil input: got %v, want nil", got)
	}
}

func TestCloneCephBranchcov0719late_HostSvcSlice_Empty(t *testing.T) {
	in := []HostCephServiceMeta{}
	out := cloneHostCephServiceMetaSlice(in)
	if out == nil {
		t.Error("empty (non-nil) input should produce non-nil empty slice")
	}
	if len(out) != 0 {
		t.Errorf("empty input: got len=%d, want 0", len(out))
	}
}

func TestCloneCephBranchcov0719late_HostSvcSlice_Isolation(t *testing.T) {
	in := []HostCephServiceMeta{
		{Type: "mon", Running: 3, Total: 3, Daemons: []string{"mon.a", "mon.b", "mon.c"}},
		{Type: "osd", Running: 5, Total: 6, Daemons: []string{"osd.0", "osd.1"}},
	}
	cloned := cloneHostCephServiceMetaSlice(in)

	if len(cloned) != 2 {
		t.Fatalf("len: got %d, want 2", len(cloned))
	}
	if cloned[0].Type != "mon" || cloned[1].Type != "osd" {
		t.Errorf("Type: got %q / %q, want mon / osd", cloned[0].Type, cloned[1].Type)
	}
	if len(cloned[0].Daemons) != 3 || cloned[0].Daemons[0] != "mon.a" {
		t.Errorf("Daemons[0]: got %+v, want [mon.a mon.b mon.c]", cloned[0].Daemons)
	}

	// Mutate scalar field — must not affect original.
	cloned[0].Type = "MUTATED"
	cloned[0].Running = 99
	if in[0].Type == "MUTATED" || in[0].Running == 99 {
		t.Error("mutating cloned scalar fields must not propagate to original")
	}

	// Mutate nested Daemons slice — must not affect original (this is the
	// reason cloneHostCephServiceMetaSlice does a per-element Daemons clone).
	cloned[0].Daemons[0] = "MUTATED_DAEMON"
	if in[0].Daemons[0] == "MUTATED_DAEMON" {
		t.Error("mutating cloned nested Daemons slice must not propagate to original")
	}
}

// --- cloneCephPoolMetaSlice (non-Host variant) ---

func TestCloneCephBranchcov0719late_PoolSlice_Nil(t *testing.T) {
	if got := cloneCephPoolMetaSlice(nil); got != nil {
		t.Errorf("nil input: got %v, want nil", got)
	}
}

func TestCloneCephBranchcov0719late_PoolSlice_Empty(t *testing.T) {
	in := []CephPoolMeta{}
	out := cloneCephPoolMetaSlice(in)
	if out == nil {
		t.Error("empty (non-nil) input should produce non-nil empty slice")
	}
	if len(out) != 0 {
		t.Errorf("empty input: got len=%d, want 0", len(out))
	}
}

func TestCloneCephBranchcov0719late_PoolSlice_Isolation(t *testing.T) {
	in := []CephPoolMeta{
		{Name: "pool-1", StoredBytes: 1024, AvailableBytes: 2048, Objects: 5, PercentUsed: 0.33},
		{Name: "pool-2", StoredBytes: 4096, AvailableBytes: 8192, Objects: 7, PercentUsed: 0.5},
	}
	cloned := cloneCephPoolMetaSlice(in)

	if len(cloned) != 2 {
		t.Fatalf("len: got %d, want 2", len(cloned))
	}
	if cloned[0].Name != "pool-1" || cloned[1].Name != "pool-2" {
		t.Errorf("names: got %q / %q, want pool-1 / pool-2", cloned[0].Name, cloned[1].Name)
	}
	if cloned[0].StoredBytes != 1024 || cloned[1].Objects != 7 {
		t.Errorf("numeric values: got StoredBytes=%d Objects=%d, want 1024 / 7", cloned[0].StoredBytes, cloned[1].Objects)
	}

	cloned[0].Name = "MUTATED"
	cloned[0].StoredBytes = 999
	if in[0].Name == "MUTATED" || in[0].StoredBytes == 999 {
		t.Error("mutating cloned element must not propagate to original")
	}
}

// --- cloneCephServiceMetaSlice (non-Host variant) ---

func TestCloneCephBranchcov0719late_SvcSlice_Nil(t *testing.T) {
	if got := cloneCephServiceMetaSlice(nil); got != nil {
		t.Errorf("nil input: got %v, want nil", got)
	}
}

func TestCloneCephBranchcov0719late_SvcSlice_Empty(t *testing.T) {
	in := []CephServiceMeta{}
	out := cloneCephServiceMetaSlice(in)
	if out == nil {
		t.Error("empty (non-nil) input should produce non-nil empty slice")
	}
	if len(out) != 0 {
		t.Errorf("empty input: got len=%d, want 0", len(out))
	}
}

func TestCloneCephBranchcov0719late_SvcSlice_Isolation(t *testing.T) {
	in := []CephServiceMeta{
		{Type: "mon", Running: 3, Total: 3},
		{Type: "osd", Running: 5, Total: 6},
	}
	cloned := cloneCephServiceMetaSlice(in)

	if len(cloned) != 2 {
		t.Fatalf("len: got %d, want 2", len(cloned))
	}
	if cloned[0].Type != "mon" || cloned[1].Type != "osd" {
		t.Errorf("Type: got %q / %q, want mon / osd", cloned[0].Type, cloned[1].Type)
	}
	if cloned[0].Running != 3 || cloned[1].Total != 6 {
		t.Errorf("values: got Running=%d Total=%d, want 3 / 6", cloned[0].Running, cloned[1].Total)
	}

	cloned[0].Type = "MUTATED"
	cloned[0].Running = 99
	if in[0].Type == "MUTATED" || in[0].Running == 99 {
		t.Error("mutating cloned element must not propagate to original")
	}
}
