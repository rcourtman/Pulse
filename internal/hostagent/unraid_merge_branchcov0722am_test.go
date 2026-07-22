package hostagent

import (
	"testing"

	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

// These tests target mergeUnraidDiskINI, mergeUnraidDisk and defaultUnraidDiskName
// in unraid.go, which previously had 0.0% coverage. They assert the real
// branch behaviour of each function (nil/empty inputs, whitespace-only field
// values, base/incoming precedence, keying and ordering) without modifying any
// source file.

// branchcovStringMerge exercises the whitespace-guarded merge rule shared by
// every TrimSpace-protected string field on UnraidDisk. Each guarded field gets
// its own subtest that delegates here so the three arms (empty keeps base,
// whitespace-only keeps base, populated wins and is trimmed) are all hit.
func branchcovStringMerge(
	t *testing.T,
	set func(*agentshost.UnraidDisk, string),
	get func(agentshost.UnraidDisk) string,
) {
	t.Helper()
	const baseVal = "baseval"
	newBase := func() agentshost.UnraidDisk {
		var d agentshost.UnraidDisk
		set(&d, baseVal)
		return d
	}
	incoming := func(v string) agentshost.UnraidDisk {
		var d agentshost.UnraidDisk
		set(&d, v)
		return d
	}
	if got := get(mergeUnraidDisk(newBase(), incoming(""))); got != baseVal {
		t.Errorf("empty incoming keeps base: got %q, want %q", got, baseVal)
	}
	if got := get(mergeUnraidDisk(newBase(), incoming("   "))); got != baseVal {
		t.Errorf("whitespace-only incoming keeps base: got %q, want %q", got, baseVal)
	}
	if got := get(mergeUnraidDisk(newBase(), incoming("  fresh  "))); got != "fresh" {
		t.Errorf("populated incoming wins and is trimmed: got %q, want fresh", got)
	}
}

// branchcovInt64Merge exercises the "> 0" merge guard shared by the int64
// numeric fields. Each guarded field gets its own subtest that delegates here.
func branchcovInt64Merge(
	t *testing.T,
	set func(*agentshost.UnraidDisk, int64),
	get func(agentshost.UnraidDisk) int64,
) {
	t.Helper()
	const baseVal int64 = 100
	newBase := func() agentshost.UnraidDisk {
		var d agentshost.UnraidDisk
		set(&d, baseVal)
		return d
	}
	incoming := func(v int64) agentshost.UnraidDisk {
		var d agentshost.UnraidDisk
		set(&d, v)
		return d
	}
	if got := get(mergeUnraidDisk(newBase(), incoming(0))); got != baseVal {
		t.Errorf("zero incoming keeps base: got %d, want %d", got, baseVal)
	}
	if got := get(mergeUnraidDisk(newBase(), incoming(-9))); got != baseVal {
		t.Errorf("negative incoming keeps base: got %d, want %d", got, baseVal)
	}
	if got := get(mergeUnraidDisk(newBase(), incoming(42))); got != 42 {
		t.Errorf("positive incoming wins: got %d, want 42", got)
	}
}

func TestBranchcov0722R2MergeUnraidDiskINI(t *testing.T) {
	t.Run("NilStorageAndNilIniAllocatesEmptyStorage", func(t *testing.T) {
		got := mergeUnraidDiskINI(nil, nil)
		if got == nil {
			t.Fatal("expected non-nil storage allocated for nil input")
		}
		if len(got.Disks) != 0 {
			t.Fatalf("expected no disks, got %+v", got.Disks)
		}
	})

	t.Run("NilStorageAndEmptyIniSliceAllocatesEmptyStorage", func(t *testing.T) {
		got := mergeUnraidDiskINI(nil, []agentshost.UnraidDisk{})
		if got == nil {
			t.Fatal("expected non-nil storage allocated for nil input with empty slice")
		}
		if len(got.Disks) != 0 {
			t.Fatalf("expected no disks, got %+v", got.Disks)
		}
	})

	t.Run("NilStorageWithIniDisksBuildsStorageSortedBySlot", func(t *testing.T) {
		iniDisks := []agentshost.UnraidDisk{
			{Name: "disk3", Slot: 3},
			{Name: "disk1", Slot: 1},
			{Name: "disk2", Slot: 2},
		}
		got := mergeUnraidDiskINI(nil, iniDisks)
		if got == nil {
			t.Fatal("expected non-nil storage")
		}
		if len(got.Disks) != 3 {
			t.Fatalf("disk count = %d, want 3: %+v", len(got.Disks), got.Disks)
		}
		wantSlots := []int{1, 2, 3}
		for i, want := range wantSlots {
			if got.Disks[i].Slot != want {
				t.Fatalf("disks[%d].Slot = %d, want %d (sorted ascending)", i, got.Disks[i].Slot, want)
			}
		}
	})

	t.Run("EmptyIniReturnsSamePointerUnmutated", func(t *testing.T) {
		original := &agentshost.UnraidStorage{
			ArrayStarted: true,
			ArrayState:   "STARTED",
			Disks: []agentshost.UnraidDisk{
				{Name: "disk1", Slot: 1},
			},
		}
		originalSnapshot := append([]agentshost.UnraidDisk(nil), original.Disks...)
		got := mergeUnraidDiskINI(original, nil)
		if got != original {
			t.Fatalf("expected same pointer returned for empty ini, got %p want %p", got, original)
		}
		if !got.ArrayStarted || got.ArrayState != "STARTED" {
			t.Fatalf("non-Disk fields not preserved: %+v", got)
		}
		if len(got.Disks) != len(originalSnapshot) {
			t.Fatalf("Disks mutated on empty ini: got %+v", got.Disks)
		}
	})

	t.Run("MatchingSlotMergesWithoutDuplicating", func(t *testing.T) {
		storage := &agentshost.UnraidStorage{
			Disks: []agentshost.UnraidDisk{
				{Name: "disk1", Slot: 1, SizeBytes: 1000},
			},
		}
		iniDisks := []agentshost.UnraidDisk{
			{Slot: 1, Serial: "SER123", Status: "online"},
		}
		got := mergeUnraidDiskINI(storage, iniDisks)
		if len(got.Disks) != 1 {
			t.Fatalf("expected merged disk count 1 (no duplicate), got %d: %+v", len(got.Disks), got.Disks)
		}
		d := got.Disks[0]
		if d.Name != "disk1" {
			t.Errorf("Name = %q, want disk1 (base kept when incoming empty)", d.Name)
		}
		if d.SizeBytes != 1000 {
			t.Errorf("SizeBytes = %d, want 1000 (base kept when incoming zero)", d.SizeBytes)
		}
		if d.Serial != "SER123" {
			t.Errorf("Serial = %q, want SER123 (incoming wins)", d.Serial)
		}
		if d.Status != "online" {
			t.Errorf("Status = %q, want online (incoming wins)", d.Status)
		}
	})

	t.Run("NewSlotAppendsDisk", func(t *testing.T) {
		storage := &agentshost.UnraidStorage{
			Disks: []agentshost.UnraidDisk{
				{Name: "disk1", Slot: 1},
			},
		}
		iniDisks := []agentshost.UnraidDisk{
			{Name: "disk2", Slot: 2, Serial: "SER2"},
		}
		got := mergeUnraidDiskINI(storage, iniDisks)
		if len(got.Disks) != 2 {
			t.Fatalf("expected 2 disks, got %d: %+v", len(got.Disks), got.Disks)
		}
		if got.Disks[0].Slot != 1 || got.Disks[1].Slot != 2 {
			t.Fatalf("unexpected slot order: %+v", got.Disks)
		}
		if got.Disks[1].Name != "disk2" || got.Disks[1].Serial != "SER2" {
			t.Errorf("appended disk not preserved: %+v", got.Disks[1])
		}
	})

	t.Run("IniDiskLowerSlotSortedBeforeExisting", func(t *testing.T) {
		storage := &agentshost.UnraidStorage{
			Disks: []agentshost.UnraidDisk{
				{Name: "disk5", Slot: 5},
			},
		}
		iniDisks := []agentshost.UnraidDisk{
			{Name: "parity", Slot: 0},
		}
		got := mergeUnraidDiskINI(storage, iniDisks)
		if len(got.Disks) != 2 {
			t.Fatalf("expected 2 disks, got %d", len(got.Disks))
		}
		if got.Disks[0].Slot != 0 || got.Disks[0].Name != "parity" {
			t.Fatalf("incoming slot 0 not sorted first: %+v", got.Disks[0])
		}
		if got.Disks[1].Slot != 5 || got.Disks[1].Name != "disk5" {
			t.Fatalf("existing slot 5 not second: %+v", got.Disks[1])
		}
	})

	t.Run("MixOfMatchingAndNewSlotsStablySorted", func(t *testing.T) {
		storage := &agentshost.UnraidStorage{
			Disks: []agentshost.UnraidDisk{
				{Name: "disk1", Slot: 1, SizeBytes: 500},
				{Name: "disk3", Slot: 3, SizeBytes: 3000},
			},
		}
		iniDisks := []agentshost.UnraidDisk{
			{Slot: 1, Serial: "S1"},
			{Slot: 2, Name: "disk2", Serial: "S2"},
		}
		got := mergeUnraidDiskINI(storage, iniDisks)
		if len(got.Disks) != 3 {
			t.Fatalf("expected 3 disks, got %d: %+v", len(got.Disks), got.Disks)
		}
		if got.Disks[0].Slot != 1 || got.Disks[0].Name != "disk1" || got.Disks[0].Serial != "S1" {
			t.Errorf("slot 1 merge unexpected: %+v", got.Disks[0])
		}
		if got.Disks[0].SizeBytes != 500 {
			t.Errorf("slot 1 base SizeBytes not kept: %d", got.Disks[0].SizeBytes)
		}
		if got.Disks[1].Slot != 2 || got.Disks[1].Name != "disk2" || got.Disks[1].Serial != "S2" {
			t.Errorf("slot 2 append unexpected: %+v", got.Disks[1])
		}
		if got.Disks[2].Slot != 3 || got.Disks[2].SizeBytes != 3000 || got.Disks[2].Name != "disk3" {
			t.Errorf("slot 3 untouched unexpected: %+v", got.Disks[2])
		}
	})

	t.Run("MutatesInputStorageDisksFieldAndReturnsSamePointer", func(t *testing.T) {
		storage := &agentshost.UnraidStorage{
			Disks: []agentshost.UnraidDisk{
				{Name: "disk1", Slot: 1},
			},
		}
		originalLen := len(storage.Disks)
		got := mergeUnraidDiskINI(storage, []agentshost.UnraidDisk{{Name: "disk2", Slot: 2}})
		if got != storage {
			t.Fatalf("expected the same pointer returned (in-place mutation), got %p want %p", got, storage)
		}
		if len(storage.Disks) == originalLen {
			t.Fatalf("expected input storage.Disks to be mutated, still len %d", len(storage.Disks))
		}
		if len(storage.Disks) != 2 {
			t.Fatalf("input storage.Disks len = %d, want 2: %+v", len(storage.Disks), storage.Disks)
		}
	})
}

func TestBranchcov0722R2MergeUnraidDisk(t *testing.T) {
	t.Run("Name", func(t *testing.T) {
		branchcovStringMerge(t,
			func(d *agentshost.UnraidDisk, v string) { d.Name = v },
			func(d agentshost.UnraidDisk) string { return d.Name })
	})
	t.Run("Device", func(t *testing.T) {
		branchcovStringMerge(t,
			func(d *agentshost.UnraidDisk, v string) { d.Device = v },
			func(d agentshost.UnraidDisk) string { return d.Device })
	})
	t.Run("Role", func(t *testing.T) {
		branchcovStringMerge(t,
			func(d *agentshost.UnraidDisk, v string) { d.Role = v },
			func(d agentshost.UnraidDisk) string { return d.Role })
	})
	t.Run("Status", func(t *testing.T) {
		branchcovStringMerge(t,
			func(d *agentshost.UnraidDisk, v string) { d.Status = v },
			func(d agentshost.UnraidDisk) string { return d.Status })
	})
	t.Run("RawStatus", func(t *testing.T) {
		branchcovStringMerge(t,
			func(d *agentshost.UnraidDisk, v string) { d.RawStatus = v },
			func(d agentshost.UnraidDisk) string { return d.RawStatus })
	})
	t.Run("Model", func(t *testing.T) {
		branchcovStringMerge(t,
			func(d *agentshost.UnraidDisk, v string) { d.Model = v },
			func(d agentshost.UnraidDisk) string { return d.Model })
	})
	t.Run("Serial", func(t *testing.T) {
		branchcovStringMerge(t,
			func(d *agentshost.UnraidDisk, v string) { d.Serial = v },
			func(d agentshost.UnraidDisk) string { return d.Serial })
	})
	t.Run("Filesystem", func(t *testing.T) {
		branchcovStringMerge(t,
			func(d *agentshost.UnraidDisk, v string) { d.Filesystem = v },
			func(d agentshost.UnraidDisk) string { return d.Filesystem })
	})
	t.Run("Transport", func(t *testing.T) {
		branchcovStringMerge(t,
			func(d *agentshost.UnraidDisk, v string) { d.Transport = v },
			func(d agentshost.UnraidDisk) string { return d.Transport })
	})

	t.Run("SizeBytes", func(t *testing.T) {
		branchcovInt64Merge(t,
			func(d *agentshost.UnraidDisk, v int64) { d.SizeBytes = v },
			func(d agentshost.UnraidDisk) int64 { return d.SizeBytes })
	})
	t.Run("UsedBytes", func(t *testing.T) {
		branchcovInt64Merge(t,
			func(d *agentshost.UnraidDisk, v int64) { d.UsedBytes = v },
			func(d agentshost.UnraidDisk) int64 { return d.UsedBytes })
	})
	t.Run("FreeBytes", func(t *testing.T) {
		branchcovInt64Merge(t,
			func(d *agentshost.UnraidDisk, v int64) { d.FreeBytes = v },
			func(d agentshost.UnraidDisk) int64 { return d.FreeBytes })
	})
	t.Run("ReadCount", func(t *testing.T) {
		branchcovInt64Merge(t,
			func(d *agentshost.UnraidDisk, v int64) { d.ReadCount = v },
			func(d agentshost.UnraidDisk) int64 { return d.ReadCount })
	})
	t.Run("WriteCount", func(t *testing.T) {
		branchcovInt64Merge(t,
			func(d *agentshost.UnraidDisk, v int64) { d.WriteCount = v },
			func(d agentshost.UnraidDisk) int64 { return d.WriteCount })
	})
	t.Run("ErrorCount", func(t *testing.T) {
		branchcovInt64Merge(t,
			func(d *agentshost.UnraidDisk, v int64) { d.ErrorCount = v },
			func(d agentshost.UnraidDisk) int64 { return d.ErrorCount })
	})

	t.Run("Temperature", func(t *testing.T) {
		const baseVal = 100
		newBase := func() agentshost.UnraidDisk {
			return agentshost.UnraidDisk{Temperature: baseVal}
		}
		if got := mergeUnraidDisk(newBase(), agentshost.UnraidDisk{Temperature: 0}).Temperature; got != baseVal {
			t.Errorf("zero incoming keeps base: got %d, want %d", got, baseVal)
		}
		if got := mergeUnraidDisk(newBase(), agentshost.UnraidDisk{Temperature: -9}).Temperature; got != baseVal {
			t.Errorf("negative incoming keeps base: got %d, want %d", got, baseVal)
		}
		if got := mergeUnraidDisk(newBase(), agentshost.UnraidDisk{Temperature: 42}).Temperature; got != 42 {
			t.Errorf("positive incoming wins: got %d, want 42", got)
		}
	})

	// SpunDown has NO guard: it is unconditionally overwritten by incoming,
	// even when incoming is the zero value (false). This subtest documents that
	// real behaviour, which differs from every other field in the merge.
	t.Run("SpunDownAlwaysOverwrites", func(t *testing.T) {
		if got := mergeUnraidDisk(agentshost.UnraidDisk{SpunDown: true}, agentshost.UnraidDisk{SpunDown: false}).SpunDown; got {
			t.Errorf("true base, false incoming: got %v, want false (unconditional overwrite)", got)
		}
		if got := mergeUnraidDisk(agentshost.UnraidDisk{SpunDown: false}, agentshost.UnraidDisk{SpunDown: true}).SpunDown; !got {
			t.Errorf("false base, true incoming: got %v, want true", got)
		}
		// The zero-value incoming (false) still clobbers a populated base.
		if got := mergeUnraidDisk(agentshost.UnraidDisk{SpunDown: true}, agentshost.UnraidDisk{}).SpunDown; got {
			t.Errorf("true base, zero incoming: got %v, want false (zero-value clobbers base)", got)
		}
	})

	// Slot uses a composite rule: out.Slot = incoming.Slot when
	// incoming.Slot > 0 OR out.Slot == 0; otherwise the base slot is kept.
	t.Run("SlotCompositeRule", func(t *testing.T) {
		if got := mergeUnraidDisk(agentshost.UnraidDisk{Slot: 5}, agentshost.UnraidDisk{Slot: 7}).Slot; got != 7 {
			t.Errorf("positive incoming wins: got %d, want 7", got)
		}
		if got := mergeUnraidDisk(agentshost.UnraidDisk{Slot: 5}, agentshost.UnraidDisk{Slot: 0}).Slot; got != 5 {
			t.Errorf("zero incoming, non-zero base keeps base: got %d, want 5", got)
		}
		if got := mergeUnraidDisk(agentshost.UnraidDisk{Slot: 5}, agentshost.UnraidDisk{Slot: -1}).Slot; got != 5 {
			t.Errorf("negative incoming, non-zero base keeps base: got %d, want 5", got)
		}
		if got := mergeUnraidDisk(agentshost.UnraidDisk{Slot: 0}, agentshost.UnraidDisk{Slot: 0}).Slot; got != 0 {
			t.Errorf("zero base, zero incoming: got %d, want 0 (out.Slot==0 branch takes incoming)", got)
		}
		// When base slot is 0, the out.Slot==0 branch takes incoming even if it
		// is negative.
		if got := mergeUnraidDisk(agentshost.UnraidDisk{Slot: 0}, agentshost.UnraidDisk{Slot: -3}).Slot; got != -3 {
			t.Errorf("zero base, negative incoming: got %d, want -3 (out.Slot==0 branch takes incoming)", got)
		}
	})
}

func TestBranchcov0722R2DefaultUnraidDiskName(t *testing.T) {
	tests := []struct {
		name string
		role string
		idx  int
		want string
	}{
		{"parity index 0 returns bare parity", "parity", 0, "parity"},
		{"parity negative index returns bare parity", "parity", -1, "parity"},
		{"parity positive index appends number", "parity", 1, "parity1"},
		{"parity large index appends number", "parity", 29, "parity29"},
		{"data index 0 formats as disk0", "data", 0, "disk0"},
		{"data index 1 formats as disk1", "data", 1, "disk1"},
		{"data large index formats number", "data", 1234, "disk1234"},
		{"unknown role cache returns empty", "cache", 30, ""},
		{"unknown role flash returns empty", "flash", 1, ""},
		{"empty role returns empty", "", 0, ""},
		{"uppercase PARITY is default arm (case sensitive)", "PARITY", 1, ""},
		{"mixed-case Parity is default arm (case sensitive)", "Parity", 1, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := defaultUnraidDiskName(tt.role, tt.idx); got != tt.want {
				t.Errorf("defaultUnraidDiskName(%q, %d) = %q, want %q", tt.role, tt.idx, got, tt.want)
			}
		})
	}
}
