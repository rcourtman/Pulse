package monitoring

import (
	"testing"
)

func TestMakeNodeSnapshotKey(t *testing.T) {
	tests := []struct {
		name     string
		instance string
		node     string
		expected string
	}{
		{
			name:     "simple instance and node",
			instance: "pve1",
			node:     "node1",
			expected: "pve1|node1",
		},
		{
			name:     "instance with port",
			instance: "pve.example.com:8006",
			node:     "pve-node",
			expected: "pve.example.com:8006|pve-node",
		},
		{
			name:     "IP address instance",
			instance: "192.168.1.100",
			node:     "server01",
			expected: "192.168.1.100|server01",
		},
		{
			name:     "empty instance",
			instance: "",
			node:     "node1",
			expected: "|node1",
		},
		{
			name:     "empty node",
			instance: "pve1",
			node:     "",
			expected: "pve1|",
		},
		{
			name:     "both empty",
			instance: "",
			node:     "",
			expected: "|",
		},
		{
			name:     "special characters in node name",
			instance: "pve1",
			node:     "node-with-dashes_and_underscores",
			expected: "pve1|node-with-dashes_and_underscores",
		},
		{
			name:     "FQDN instance",
			instance: "proxmox.datacenter.example.com",
			node:     "pve-cluster-node-01",
			expected: "proxmox.datacenter.example.com|pve-cluster-node-01",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := makeNodeSnapshotKey(tc.instance, tc.node)
			if got != tc.expected {
				t.Errorf("makeNodeSnapshotKey(%q, %q) = %q, want %q",
					tc.instance, tc.node, got, tc.expected)
			}
		})
	}
}

func TestMakeGuestSnapshotKey(t *testing.T) {
	tests := []struct {
		name      string
		instance  string
		guestType string
		node      string
		vmid      int
		expected  string
	}{
		{
			name:      "VM guest",
			instance:  "pve1",
			guestType: "qemu",
			node:      "node1",
			vmid:      100,
			expected:  "pve1|qemu|node1|100",
		},
		{
			name:      "LXC container",
			instance:  "pve1",
			guestType: "lxc",
			node:      "node1",
			vmid:      200,
			expected:  "pve1|lxc|node1|200",
		},
		{
			name:      "high VMID",
			instance:  "pve1",
			guestType: "qemu",
			node:      "node1",
			vmid:      999999,
			expected:  "pve1|qemu|node1|999999",
		},
		{
			name:      "zero VMID",
			instance:  "pve1",
			guestType: "qemu",
			node:      "node1",
			vmid:      0,
			expected:  "pve1|qemu|node1|0",
		},
		{
			name:      "negative VMID",
			instance:  "pve1",
			guestType: "qemu",
			node:      "node1",
			vmid:      -1,
			expected:  "pve1|qemu|node1|-1",
		},
		{
			name:      "instance with port",
			instance:  "pve.example.com:8006",
			guestType: "lxc",
			node:      "pve-node",
			vmid:      101,
			expected:  "pve.example.com:8006|lxc|pve-node|101",
		},
		{
			name:      "empty instance",
			instance:  "",
			guestType: "qemu",
			node:      "node1",
			vmid:      100,
			expected:  "|qemu|node1|100",
		},
		{
			name:      "empty guest type",
			instance:  "pve1",
			guestType: "",
			node:      "node1",
			vmid:      100,
			expected:  "pve1||node1|100",
		},
		{
			name:      "empty node",
			instance:  "pve1",
			guestType: "qemu",
			node:      "",
			vmid:      100,
			expected:  "pve1|qemu||100",
		},
		{
			name:      "all empty except VMID",
			instance:  "",
			guestType: "",
			node:      "",
			vmid:      42,
			expected:  "|||42",
		},
		{
			name:      "FQDN with complex node name",
			instance:  "proxmox.datacenter.example.com",
			guestType: "qemu",
			node:      "pve-cluster-node-01",
			vmid:      12345,
			expected:  "proxmox.datacenter.example.com|qemu|pve-cluster-node-01|12345",
		},
		{
			name:      "IP address instance",
			instance:  "192.168.1.100",
			guestType: "lxc",
			node:      "server01",
			vmid:      500,
			expected:  "192.168.1.100|lxc|server01|500",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := makeGuestSnapshotKey(tc.instance, tc.guestType, tc.node, tc.vmid)
			if got != tc.expected {
				t.Errorf("makeGuestSnapshotKey(%q, %q, %q, %d) = %q, want %q",
					tc.instance, tc.guestType, tc.node, tc.vmid, got, tc.expected)
			}
		})
	}
}

func TestMakeSnapshotKey_Uniqueness(t *testing.T) {
	// Test that different inputs produce different keys

	t.Run("node keys with different instances are unique", func(t *testing.T) {
		key1 := makeNodeSnapshotKey("pve1", "node1")
		key2 := makeNodeSnapshotKey("pve2", "node1")
		if key1 == key2 {
			t.Errorf("Keys should be unique: %q == %q", key1, key2)
		}
	})

	t.Run("node keys with different nodes are unique", func(t *testing.T) {
		key1 := makeNodeSnapshotKey("pve1", "node1")
		key2 := makeNodeSnapshotKey("pve1", "node2")
		if key1 == key2 {
			t.Errorf("Keys should be unique: %q == %q", key1, key2)
		}
	})

	t.Run("guest keys with different VMIDs are unique", func(t *testing.T) {
		key1 := makeGuestSnapshotKey("pve1", "qemu", "node1", 100)
		key2 := makeGuestSnapshotKey("pve1", "qemu", "node1", 101)
		if key1 == key2 {
			t.Errorf("Keys should be unique: %q == %q", key1, key2)
		}
	})

	t.Run("guest keys with different guest types are unique", func(t *testing.T) {
		key1 := makeGuestSnapshotKey("pve1", "qemu", "node1", 100)
		key2 := makeGuestSnapshotKey("pve1", "lxc", "node1", 100)
		if key1 == key2 {
			t.Errorf("Keys should be unique: %q == %q", key1, key2)
		}
	})

	t.Run("guest keys with different nodes are unique", func(t *testing.T) {
		key1 := makeGuestSnapshotKey("pve1", "qemu", "node1", 100)
		key2 := makeGuestSnapshotKey("pve1", "qemu", "node2", 100)
		if key1 == key2 {
			t.Errorf("Keys should be unique: %q == %q", key1, key2)
		}
	})

	t.Run("guest keys with different instances are unique", func(t *testing.T) {
		key1 := makeGuestSnapshotKey("pve1", "qemu", "node1", 100)
		key2 := makeGuestSnapshotKey("pve2", "qemu", "node1", 100)
		if key1 == key2 {
			t.Errorf("Keys should be unique: %q == %q", key1, key2)
		}
	})
}

func TestDiagnosticSnapshotSet_Fields(t *testing.T) {
	// Test struct field initialization
	set := DiagnosticSnapshotSet{
		Nodes:  []NodeMemorySnapshot{},
		Guests: []GuestMemorySnapshot{},
	}

	if set.Nodes == nil {
		t.Error("Nodes should not be nil")
	}
	if set.Guests == nil {
		t.Error("Guests should not be nil")
	}
	if len(set.Nodes) != 0 {
		t.Errorf("Nodes length = %d, want 0", len(set.Nodes))
	}
	if len(set.Guests) != 0 {
		t.Errorf("Guests length = %d, want 0", len(set.Guests))
	}
}

func TestNodeMemoryRaw_Fields(t *testing.T) {
	raw := NodeMemoryRaw{
		Total:               16000000000,
		Used:                8000000000,
		Free:                8000000000,
		Available:           10000000000,
		Buffers:             500000000,
		Cached:              2000000000,
		ProxmoxMemorySource: "rrd-available",
	}

	if raw.Total != 16000000000 {
		t.Errorf("Total = %d, want 16000000000", raw.Total)
	}
	if raw.Used != 8000000000 {
		t.Errorf("Used = %d, want 8000000000", raw.Used)
	}
	if raw.Free != 8000000000 {
		t.Errorf("Free = %d, want 8000000000", raw.Free)
	}
	if raw.Available != 10000000000 {
		t.Errorf("Available = %d, want 10000000000", raw.Available)
	}
	if raw.Buffers != 500000000 {
		t.Errorf("Buffers = %d, want 500000000", raw.Buffers)
	}
	if raw.Cached != 2000000000 {
		t.Errorf("Cached = %d, want 2000000000", raw.Cached)
	}
	if raw.ProxmoxMemorySource != "rrd-available" {
		t.Errorf("ProxmoxMemorySource = %q, want %q", raw.ProxmoxMemorySource, "rrd-available")
	}
}

func TestVMMemoryRaw_Fields(t *testing.T) {
	raw := VMMemoryRaw{
		ListingMem:    2000000000,
		ListingMaxMem: 4000000000,
		StatusMem:     2100000000,
		StatusMaxMem:  4000000000,
		Balloon:       3000000000,
		BalloonMin:    1000000000,
		Agent:         1,
	}

	if raw.ListingMem != 2000000000 {
		t.Errorf("ListingMem = %d, want 2000000000", raw.ListingMem)
	}
	if raw.ListingMaxMem != 4000000000 {
		t.Errorf("ListingMaxMem = %d, want 4000000000", raw.ListingMaxMem)
	}
	if raw.StatusMem != 2100000000 {
		t.Errorf("StatusMem = %d, want 2100000000", raw.StatusMem)
	}
	if raw.StatusMaxMem != 4000000000 {
		t.Errorf("StatusMaxMem = %d, want 4000000000", raw.StatusMaxMem)
	}
	if raw.Balloon != 3000000000 {
		t.Errorf("Balloon = %d, want 3000000000", raw.Balloon)
	}
	if raw.BalloonMin != 1000000000 {
		t.Errorf("BalloonMin = %d, want 1000000000", raw.BalloonMin)
	}
	if raw.Agent != 1 {
		t.Errorf("Agent = %d, want 1", raw.Agent)
	}
}
