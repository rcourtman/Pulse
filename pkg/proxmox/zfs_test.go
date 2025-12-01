package proxmox

import "testing"

func TestConvertDeviceRecursiveClassifiesVdevs(t *testing.T) {
	mirror := ZFSPoolDevice{
		Name:  "mirror-0",
		State: "degraded",
		Leaf:  0,
		Children: []ZFSPoolDevice{
			{Name: "sda", State: "ONLINE", Leaf: 1},
			{Name: "sdb", State: "ONLINE", Leaf: 1},
		},
	}

	devices := convertDeviceRecursive(mirror, "")
	if len(devices) != 1 {
		t.Fatalf("expected single vdev entry, got %d", len(devices))
	}
	vdev := devices[0]
	if vdev.Type != "mirror" {
		t.Fatalf("expected mirror type, got %s", vdev.Type)
	}
	if vdev.State != "DEGRADED" {
		t.Fatalf("expected DEGRADED state, got %s", vdev.State)
	}
}

func TestConvertDeviceRecursiveLeavesIncludeErrors(t *testing.T) {
	dev := ZFSPoolDevice{
		Name:  "nvme0n1",
		State: "degRaDed",
		Leaf:  1,
		Read:  1,
		Write: 2,
		Cksum: 3,
		Msg:   "device error",
	}

	devices := convertDeviceRecursive(dev, "")
	if len(devices) != 1 {
		t.Fatalf("expected single device, got %d", len(devices))
	}
	leaf := devices[0]
	if !leaf.IsLeaf {
		t.Fatalf("expected leaf device")
	}
	if leaf.Type != "disk" {
		t.Fatalf("expected disk type, got %s", leaf.Type)
	}
	if leaf.ReadErrors != 1 || leaf.WriteErrors != 2 || leaf.ChecksumErrors != 3 {
		t.Fatalf("unexpected error counts: %+v", leaf)
	}
	if leaf.Message != "device error" {
		t.Fatalf("expected message to propagate, got %q", leaf.Message)
	}
}

func TestConvertDeviceRecursiveSkipsHealthyLeaf(t *testing.T) {
	dev := ZFSPoolDevice{
		Name:  "nvme0n1",
		State: "ONLINE",
		Leaf:  1,
	}

	devices := convertDeviceRecursive(dev, "")
	if len(devices) != 0 {
		t.Fatalf("expected healthy leaf to be omitted, got %d entries", len(devices))
	}
}

func TestConvertDeviceRecursive_RaidZTypes(t *testing.T) {
	tests := []struct {
		name         string
		vdevName     string
		expectedType string
	}{
		{"raidz1", "raidz1-0", "raidz1-0"},
		{"raidz2", "raidz2-0", "raidz2-0"},
		{"raidz3", "raidz3-0", "raidz3-0"},
		{"raidz", "raidz-0", "raidz-0"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dev := ZFSPoolDevice{
				Name:  tc.vdevName,
				State: "DEGRADED",
				Leaf:  0,
				Children: []ZFSPoolDevice{
					{Name: "sda", State: "ONLINE", Leaf: 1},
				},
			}

			devices := convertDeviceRecursive(dev, "")
			if len(devices) != 1 {
				t.Fatalf("expected 1 device, got %d", len(devices))
			}
			if devices[0].Type != tc.expectedType {
				t.Errorf("Type = %q, want %q", devices[0].Type, tc.expectedType)
			}
		})
	}
}

func TestConvertDeviceRecursive_LogDevices(t *testing.T) {
	// Log vdev container
	logVdev := ZFSPoolDevice{
		Name:  "logs",
		State: "",
		Leaf:  0,
		Children: []ZFSPoolDevice{
			{Name: "nvme0n1p1", State: "ONLINE", Leaf: 1, Read: 1}, // has errors, should appear
		},
	}

	devices := convertDeviceRecursive(logVdev, "")

	// Should have the child with errors
	if len(devices) != 1 {
		t.Fatalf("expected 1 device (child with errors), got %d", len(devices))
	}
	if devices[0].Type != "log" {
		t.Errorf("child Type = %q, want log", devices[0].Type)
	}
}

func TestConvertDeviceRecursive_CacheDevices(t *testing.T) {
	// Cache vdev container
	cacheVdev := ZFSPoolDevice{
		Name:  "cache",
		State: "",
		Leaf:  0,
		Children: []ZFSPoolDevice{
			{Name: "nvme1n1", State: "ONLINE", Leaf: 1, Write: 5}, // has errors
		},
	}

	devices := convertDeviceRecursive(cacheVdev, "")

	if len(devices) != 1 {
		t.Fatalf("expected 1 device, got %d", len(devices))
	}
	if devices[0].Type != "cache" {
		t.Errorf("Type = %q, want cache", devices[0].Type)
	}
}

func TestConvertDeviceRecursive_SpareDevices(t *testing.T) {
	// Spare vdev container
	spareVdev := ZFSPoolDevice{
		Name:  "spares",
		State: "",
		Leaf:  0,
		Children: []ZFSPoolDevice{
			{Name: "sdc", State: "AVAIL", Leaf: 1},
		},
	}

	devices := convertDeviceRecursive(spareVdev, "")

	// Healthy spares with state AVAIL should be skipped
	if len(devices) != 0 {
		t.Fatalf("expected 0 devices (healthy spare), got %d", len(devices))
	}
}

func TestConvertDeviceRecursive_SpareWithErrors(t *testing.T) {
	// When a spare is directly a leaf with "spare-X" name pattern
	spareLeaf := ZFSPoolDevice{
		Name:  "spare-0",
		State: "FAULTED",
		Leaf:  1,
	}

	devices := convertDeviceRecursive(spareLeaf, "")

	if len(devices) != 1 {
		t.Fatalf("expected 1 device (faulted spare), got %d", len(devices))
	}
	if devices[0].Type != "spare" {
		t.Errorf("Type = %q, want spare", devices[0].Type)
	}
	if devices[0].State != "FAULTED" {
		t.Errorf("State = %q, want FAULTED", devices[0].State)
	}
}

func TestConvertDeviceRecursive_NestedMirror(t *testing.T) {
	// Nested structure: mirror with healthy children
	mirror := ZFSPoolDevice{
		Name:  "mirror-0",
		State: "ONLINE",
		Leaf:  0,
		Children: []ZFSPoolDevice{
			{Name: "sda", State: "ONLINE", Leaf: 1},
			{Name: "sdb", State: "ONLINE", Leaf: 1},
		},
	}

	devices := convertDeviceRecursive(mirror, "")

	// All healthy, should be empty
	if len(devices) != 0 {
		t.Fatalf("expected 0 devices (all healthy), got %d", len(devices))
	}
}

func TestConvertDeviceRecursive_EmptyState(t *testing.T) {
	dev := ZFSPoolDevice{
		Name:  "sda",
		State: "",
		Leaf:  1,
		Read:  1, // has errors
	}

	devices := convertDeviceRecursive(dev, "")

	if len(devices) != 1 {
		t.Fatalf("expected 1 device, got %d", len(devices))
	}
	if devices[0].State != "UNKNOWN" {
		t.Errorf("State = %q, want UNKNOWN", devices[0].State)
	}
}

func TestConvertDeviceRecursive_SlogDevice(t *testing.T) {
	// Separate log device (slog)
	slog := ZFSPoolDevice{
		Name:  "slog-0",
		State: "DEGRADED",
		Leaf:  0,
		Children: []ZFSPoolDevice{
			{Name: "nvme0n1p1", State: "ONLINE", Leaf: 1},
		},
	}

	devices := convertDeviceRecursive(slog, "")

	if len(devices) != 1 {
		t.Fatalf("expected 1 device (degraded slog vdev), got %d", len(devices))
	}
	if devices[0].Type != "log" {
		t.Errorf("Type = %q, want log", devices[0].Type)
	}
}

func TestConvertDeviceRecursive_L2arcDevice(t *testing.T) {
	l2arc := ZFSPoolDevice{
		Name:  "l2arc-0",
		State: "DEGRADED",
		Leaf:  0,
		Children: []ZFSPoolDevice{
			{Name: "nvme1n1", State: "ONLINE", Leaf: 1},
		},
	}

	devices := convertDeviceRecursive(l2arc, "")

	if len(devices) != 1 {
		t.Fatalf("expected 1 device, got %d", len(devices))
	}
	if devices[0].Type != "cache" {
		t.Errorf("Type = %q, want cache", devices[0].Type)
	}
}

func TestConvertDeviceRecursive_InUseSpare(t *testing.T) {
	spare := ZFSPoolDevice{
		Name:  "spare-0",
		State: "INUSE",
		Leaf:  1,
	}

	devices := convertDeviceRecursive(spare, "")

	// INUSE is a healthy state, should be skipped
	if len(devices) != 0 {
		t.Fatalf("expected 0 devices (INUSE spare is healthy), got %d", len(devices))
	}
}

func TestZFSPoolInfo_ConvertToModelZFSPool(t *testing.T) {
	info := &ZFSPoolInfo{
		Name:   "tank",
		Health: "ONLINE",
		Size:   1000000000000,
		Alloc:  500000000000,
		Free:   500000000000,
		Frag:   10,
		Dedup:  1.5,
		State:  "ONLINE",
		Status: "All vdevs healthy",
		Scan:   "scrub completed",
		Errors: "No known data errors",
		Devices: []ZFSPoolDevice{
			{
				Name:  "mirror-0",
				State: "ONLINE",
				Leaf:  0,
				Children: []ZFSPoolDevice{
					{Name: "sda", State: "ONLINE", Leaf: 1, Read: 1, Write: 2, Cksum: 3},
					{Name: "sdb", State: "ONLINE", Leaf: 1},
				},
			},
		},
	}

	pool := info.ConvertToModelZFSPool()

	if pool == nil {
		t.Fatal("ConvertToModelZFSPool returned nil")
	}
	if pool.Name != "tank" {
		t.Errorf("Name = %q, want tank", pool.Name)
	}
	if pool.State != "ONLINE" {
		t.Errorf("State = %q, want ONLINE", pool.State)
	}
	if pool.Health != "ONLINE" {
		t.Errorf("Health = %q, want ONLINE", pool.Health)
	}
	if pool.Status != "All vdevs healthy" {
		t.Errorf("Status = %q, want 'All vdevs healthy'", pool.Status)
	}
	if pool.Scan != "scrub completed" {
		t.Errorf("Scan = %q, want 'scrub completed'", pool.Scan)
	}
	if pool.Errors != "No known data errors" {
		t.Errorf("Errors = %q, want 'No known data errors'", pool.Errors)
	}
	// Only device with errors should be in the list
	if len(pool.Devices) != 1 {
		t.Errorf("Devices count = %d, want 1", len(pool.Devices))
	}
	if pool.ReadErrors != 1 {
		t.Errorf("ReadErrors = %d, want 1", pool.ReadErrors)
	}
	if pool.WriteErrors != 2 {
		t.Errorf("WriteErrors = %d, want 2", pool.WriteErrors)
	}
	if pool.ChecksumErrors != 3 {
		t.Errorf("ChecksumErrors = %d, want 3", pool.ChecksumErrors)
	}
}

func TestZFSPoolInfo_ConvertToModelZFSPool_NilReceiver(t *testing.T) {
	var info *ZFSPoolInfo
	pool := info.ConvertToModelZFSPool()

	if pool != nil {
		t.Error("ConvertToModelZFSPool on nil should return nil")
	}
}

func TestZFSPoolInfo_ConvertToModelZFSPool_StateFallback(t *testing.T) {
	// When State is empty, should fall back to Health
	info := &ZFSPoolInfo{
		Name:   "tank",
		Health: "DEGRADED",
		State:  "", // empty
	}

	pool := info.ConvertToModelZFSPool()

	if pool.State != "DEGRADED" {
		t.Errorf("State = %q, want DEGRADED (from Health)", pool.State)
	}
}

func TestZFSPoolInfo_ConvertToModelZFSPool_NoDevices(t *testing.T) {
	info := &ZFSPoolInfo{
		Name:    "tank",
		Health:  "ONLINE",
		Devices: nil,
	}

	pool := info.ConvertToModelZFSPool()

	if pool.Devices == nil {
		t.Error("Devices should be initialized to empty slice, not nil")
	}
	if len(pool.Devices) != 0 {
		t.Errorf("Devices count = %d, want 0", len(pool.Devices))
	}
}

func TestZFSPoolInfo_ConvertToModelZFSPool_AggregatesErrors(t *testing.T) {
	info := &ZFSPoolInfo{
		Name:   "tank",
		Health: "DEGRADED",
		Devices: []ZFSPoolDevice{
			{Name: "sda", State: "DEGRADED", Leaf: 1, Read: 10, Write: 20, Cksum: 30},
			{Name: "sdb", State: "FAULTED", Leaf: 1, Read: 5, Write: 10, Cksum: 15},
		},
	}

	pool := info.ConvertToModelZFSPool()

	// Should aggregate errors from all devices
	if pool.ReadErrors != 15 {
		t.Errorf("ReadErrors = %d, want 15", pool.ReadErrors)
	}
	if pool.WriteErrors != 30 {
		t.Errorf("WriteErrors = %d, want 30", pool.WriteErrors)
	}
	if pool.ChecksumErrors != 45 {
		t.Errorf("ChecksumErrors = %d, want 45", pool.ChecksumErrors)
	}
}

func TestZFSPool_Fields(t *testing.T) {
	pool := ZFSPool{
		Name:           "tank",
		State:          "ONLINE",
		Health:         "ONLINE",
		Status:         "healthy",
		Scan:           "none requested",
		Errors:         "No known data errors",
		ReadErrors:     10,
		WriteErrors:    20,
		ChecksumErrors: 30,
		Devices: []ZFSDevice{
			{Name: "sda", Type: "disk", State: "ONLINE", IsLeaf: true},
		},
	}

	if pool.Name != "tank" {
		t.Errorf("Name = %q, want tank", pool.Name)
	}
	if pool.ReadErrors != 10 {
		t.Errorf("ReadErrors = %d, want 10", pool.ReadErrors)
	}
	if pool.WriteErrors != 20 {
		t.Errorf("WriteErrors = %d, want 20", pool.WriteErrors)
	}
	if pool.ChecksumErrors != 30 {
		t.Errorf("ChecksumErrors = %d, want 30", pool.ChecksumErrors)
	}
	if len(pool.Devices) != 1 {
		t.Errorf("Devices count = %d, want 1", len(pool.Devices))
	}
}

func TestZFSDevice_Fields(t *testing.T) {
	dev := ZFSDevice{
		Name:           "nvme0n1",
		Type:           "disk",
		State:          "ONLINE",
		ReadErrors:     1,
		WriteErrors:    2,
		ChecksumErrors: 3,
		IsLeaf:         true,
		Message:        "device is failing",
	}

	if dev.Name != "nvme0n1" {
		t.Errorf("Name = %q, want nvme0n1", dev.Name)
	}
	if dev.Type != "disk" {
		t.Errorf("Type = %q, want disk", dev.Type)
	}
	if dev.State != "ONLINE" {
		t.Errorf("State = %q, want ONLINE", dev.State)
	}
	if !dev.IsLeaf {
		t.Error("IsLeaf should be true")
	}
	if dev.Message != "device is failing" {
		t.Errorf("Message = %q, want 'device is failing'", dev.Message)
	}
}

func TestZFSPoolInfo_Fields(t *testing.T) {
	info := ZFSPoolInfo{
		Name:   "tank",
		Health: "ONLINE",
		Size:   1000000000000,
		Alloc:  500000000000,
		Free:   500000000000,
		Frag:   10,
		Dedup:  1.5,
		State:  "ONLINE",
		Status: "All healthy",
		Scan:   "scrub completed",
		Errors: "No errors",
	}

	if info.Name != "tank" {
		t.Errorf("Name = %q, want tank", info.Name)
	}
	if info.Health != "ONLINE" {
		t.Errorf("Health = %q, want ONLINE", info.Health)
	}
	if info.Size != 1000000000000 {
		t.Errorf("Size = %d, want 1000000000000", info.Size)
	}
	if info.Alloc != 500000000000 {
		t.Errorf("Alloc = %d, want 500000000000", info.Alloc)
	}
	if info.Free != 500000000000 {
		t.Errorf("Free = %d, want 500000000000", info.Free)
	}
	if info.Frag != 10 {
		t.Errorf("Frag = %d, want 10", info.Frag)
	}
	if info.Dedup != 1.5 {
		t.Errorf("Dedup = %f, want 1.5", info.Dedup)
	}
}
