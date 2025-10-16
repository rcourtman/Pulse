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
