package monitoring

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
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

func TestRecordGuestSnapshot(t *testing.T) {
	t.Run("nil Monitor is no-op", func(t *testing.T) {
		var m *Monitor
		// Should not panic
		m.recordGuestSnapshot("instance", "qemu", "node1", 100, GuestMemorySnapshot{})
	})

	t.Run("records single guest snapshot", func(t *testing.T) {
		m := &Monitor{
			nodeSnapshots:  make(map[string]NodeMemorySnapshot),
			guestSnapshots: make(map[string]GuestMemorySnapshot),
		}

		m.recordGuestSnapshot("pve1", "qemu", "node1", 100, GuestMemorySnapshot{
			Name:         "testvm",
			Status:       "running",
			MemorySource: "balloon",
		})

		if len(m.guestSnapshots) != 1 {
			t.Fatalf("Expected 1 guest snapshot, got %d", len(m.guestSnapshots))
		}

		key := makeGuestSnapshotKey("pve1", "qemu", "node1", 100)
		snapshot, ok := m.guestSnapshots[key]
		if !ok {
			t.Fatalf("Expected snapshot with key %q not found", key)
		}

		if snapshot.Name != "testvm" {
			t.Errorf("Name = %q, want %q", snapshot.Name, "testvm")
		}
		if snapshot.Status != "running" {
			t.Errorf("Status = %q, want %q", snapshot.Status, "running")
		}
		if snapshot.MemorySource != "balloon" {
			t.Errorf("MemorySource = %q, want %q", snapshot.MemorySource, "balloon")
		}
	})

	t.Run("sets Instance, GuestType, Node, VMID from parameters", func(t *testing.T) {
		m := &Monitor{
			nodeSnapshots:  make(map[string]NodeMemorySnapshot),
			guestSnapshots: make(map[string]GuestMemorySnapshot),
		}

		m.recordGuestSnapshot("my-instance", "lxc", "my-node", 200, GuestMemorySnapshot{
			Name: "testct",
		})

		key := makeGuestSnapshotKey("my-instance", "lxc", "my-node", 200)
		snapshot := m.guestSnapshots[key]

		if snapshot.Instance != "my-instance" {
			t.Errorf("Instance = %q, want %q", snapshot.Instance, "my-instance")
		}
		if snapshot.GuestType != "lxc" {
			t.Errorf("GuestType = %q, want %q", snapshot.GuestType, "lxc")
		}
		if snapshot.Node != "my-node" {
			t.Errorf("Node = %q, want %q", snapshot.Node, "my-node")
		}
		if snapshot.VMID != 200 {
			t.Errorf("VMID = %d, want %d", snapshot.VMID, 200)
		}
	})

	t.Run("sets RetrievedAt to current time when zero", func(t *testing.T) {
		m := &Monitor{
			nodeSnapshots:  make(map[string]NodeMemorySnapshot),
			guestSnapshots: make(map[string]GuestMemorySnapshot),
		}

		beforeRecord := time.Now()
		m.recordGuestSnapshot("pve1", "qemu", "node1", 100, GuestMemorySnapshot{})
		afterRecord := time.Now()

		key := makeGuestSnapshotKey("pve1", "qemu", "node1", 100)
		snapshot := m.guestSnapshots[key]

		if snapshot.RetrievedAt.Before(beforeRecord) || snapshot.RetrievedAt.After(afterRecord) {
			t.Errorf("RetrievedAt = %v, want between %v and %v", snapshot.RetrievedAt, beforeRecord, afterRecord)
		}
	})

	t.Run("preserves non-zero RetrievedAt", func(t *testing.T) {
		m := &Monitor{
			nodeSnapshots:  make(map[string]NodeMemorySnapshot),
			guestSnapshots: make(map[string]GuestMemorySnapshot),
		}

		specificTime := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)
		m.recordGuestSnapshot("pve1", "qemu", "node1", 100, GuestMemorySnapshot{
			RetrievedAt: specificTime,
		})

		key := makeGuestSnapshotKey("pve1", "qemu", "node1", 100)
		snapshot := m.guestSnapshots[key]

		if !snapshot.RetrievedAt.Equal(specificTime) {
			t.Errorf("RetrievedAt = %v, want %v", snapshot.RetrievedAt, specificTime)
		}
	})

	t.Run("records multiple guests with different keys", func(t *testing.T) {
		m := &Monitor{
			nodeSnapshots:  make(map[string]NodeMemorySnapshot),
			guestSnapshots: make(map[string]GuestMemorySnapshot),
		}

		m.recordGuestSnapshot("pve1", "qemu", "node1", 100, GuestMemorySnapshot{Name: "vm1"})
		m.recordGuestSnapshot("pve1", "qemu", "node1", 101, GuestMemorySnapshot{Name: "vm2"})
		m.recordGuestSnapshot("pve1", "lxc", "node1", 200, GuestMemorySnapshot{Name: "ct1"})
		m.recordGuestSnapshot("pve2", "qemu", "node2", 100, GuestMemorySnapshot{Name: "vm3"})

		if len(m.guestSnapshots) != 4 {
			t.Fatalf("Expected 4 guest snapshots, got %d", len(m.guestSnapshots))
		}

		// Verify each one exists
		key1 := makeGuestSnapshotKey("pve1", "qemu", "node1", 100)
		key2 := makeGuestSnapshotKey("pve1", "qemu", "node1", 101)
		key3 := makeGuestSnapshotKey("pve1", "lxc", "node1", 200)
		key4 := makeGuestSnapshotKey("pve2", "qemu", "node2", 100)

		if m.guestSnapshots[key1].Name != "vm1" {
			t.Errorf("Snapshot 1 Name = %q, want %q", m.guestSnapshots[key1].Name, "vm1")
		}
		if m.guestSnapshots[key2].Name != "vm2" {
			t.Errorf("Snapshot 2 Name = %q, want %q", m.guestSnapshots[key2].Name, "vm2")
		}
		if m.guestSnapshots[key3].Name != "ct1" {
			t.Errorf("Snapshot 3 Name = %q, want %q", m.guestSnapshots[key3].Name, "ct1")
		}
		if m.guestSnapshots[key4].Name != "vm3" {
			t.Errorf("Snapshot 4 Name = %q, want %q", m.guestSnapshots[key4].Name, "vm3")
		}
	})

	t.Run("overwrites existing guest snapshot with same key", func(t *testing.T) {
		m := &Monitor{
			nodeSnapshots:  make(map[string]NodeMemorySnapshot),
			guestSnapshots: make(map[string]GuestMemorySnapshot),
		}

		m.recordGuestSnapshot("pve1", "qemu", "node1", 100, GuestMemorySnapshot{
			Name:         "oldname",
			Status:       "stopped",
			MemorySource: "listing",
		})

		m.recordGuestSnapshot("pve1", "qemu", "node1", 100, GuestMemorySnapshot{
			Name:         "newname",
			Status:       "running",
			MemorySource: "balloon",
		})

		if len(m.guestSnapshots) != 1 {
			t.Fatalf("Expected 1 guest snapshot after overwrite, got %d", len(m.guestSnapshots))
		}

		key := makeGuestSnapshotKey("pve1", "qemu", "node1", 100)
		snapshot := m.guestSnapshots[key]

		if snapshot.Name != "newname" {
			t.Errorf("Name = %q, want %q", snapshot.Name, "newname")
		}
		if snapshot.Status != "running" {
			t.Errorf("Status = %q, want %q", snapshot.Status, "running")
		}
		if snapshot.MemorySource != "balloon" {
			t.Errorf("MemorySource = %q, want %q", snapshot.MemorySource, "balloon")
		}
	})

	t.Run("initializes nil guestSnapshots map", func(t *testing.T) {
		m := &Monitor{
			nodeSnapshots: make(map[string]NodeMemorySnapshot),
			// guestSnapshots intentionally nil
		}

		m.recordGuestSnapshot("pve1", "qemu", "node1", 100, GuestMemorySnapshot{Name: "test"})

		if m.guestSnapshots == nil {
			t.Fatal("guestSnapshots should have been initialized")
		}
		if len(m.guestSnapshots) != 1 {
			t.Errorf("Expected 1 guest snapshot, got %d", len(m.guestSnapshots))
		}
	})
}

func TestLogNodeMemorySource(t *testing.T) {
	t.Run("nil Monitor is no-op", func(t *testing.T) {
		var m *Monitor
		// Should not panic
		m.logNodeMemorySource("instance", "node1", NodeMemorySnapshot{
			MemorySource: "rrd-data",
		})
	})

	t.Run("same source as existing snapshot - no log emitted", func(t *testing.T) {
		m := &Monitor{
			nodeSnapshots:  make(map[string]NodeMemorySnapshot),
			guestSnapshots: make(map[string]GuestMemorySnapshot),
		}

		// Pre-populate with existing snapshot
		key := makeNodeSnapshotKey("pve1", "node1")
		m.nodeSnapshots[key] = NodeMemorySnapshot{
			MemorySource: "rrd-data",
		}

		// Should not panic and should return early (same source)
		m.logNodeMemorySource("pve1", "node1", NodeMemorySnapshot{
			MemorySource: "rrd-data",
		})
	})

	t.Run("empty source triggers warn level", func(t *testing.T) {
		m := &Monitor{
			nodeSnapshots:  make(map[string]NodeMemorySnapshot),
			guestSnapshots: make(map[string]GuestMemorySnapshot),
		}

		// Should not panic - empty source triggers Warn
		m.logNodeMemorySource("pve1", "node1", NodeMemorySnapshot{
			MemorySource: "",
		})
	})

	t.Run("nodes-endpoint source triggers warn level", func(t *testing.T) {
		m := &Monitor{
			nodeSnapshots:  make(map[string]NodeMemorySnapshot),
			guestSnapshots: make(map[string]GuestMemorySnapshot),
		}

		// Should not panic - nodes-endpoint triggers Warn
		m.logNodeMemorySource("pve1", "node1", NodeMemorySnapshot{
			MemorySource: "nodes-endpoint",
		})
	})

	t.Run("node-status-used source triggers warn level", func(t *testing.T) {
		m := &Monitor{
			nodeSnapshots:  make(map[string]NodeMemorySnapshot),
			guestSnapshots: make(map[string]GuestMemorySnapshot),
		}

		// Should not panic - node-status-used triggers Warn
		m.logNodeMemorySource("pve1", "node1", NodeMemorySnapshot{
			MemorySource: "node-status-used",
		})
	})

	t.Run("previous-snapshot source triggers warn level", func(t *testing.T) {
		m := &Monitor{
			nodeSnapshots:  make(map[string]NodeMemorySnapshot),
			guestSnapshots: make(map[string]GuestMemorySnapshot),
		}

		// Should not panic - previous-snapshot triggers Warn
		m.logNodeMemorySource("pve1", "node1", NodeMemorySnapshot{
			MemorySource: "previous-snapshot",
		})
	})

	t.Run("rrd-data source triggers debug level", func(t *testing.T) {
		m := &Monitor{
			nodeSnapshots:  make(map[string]NodeMemorySnapshot),
			guestSnapshots: make(map[string]GuestMemorySnapshot),
		}

		// Should not panic - rrd-data triggers Debug
		m.logNodeMemorySource("pve1", "node1", NodeMemorySnapshot{
			MemorySource: "rrd-data",
		})
	})

	t.Run("rrd-available source triggers debug level", func(t *testing.T) {
		m := &Monitor{
			nodeSnapshots:  make(map[string]NodeMemorySnapshot),
			guestSnapshots: make(map[string]GuestMemorySnapshot),
		}

		// Should not panic - rrd-available triggers Debug
		m.logNodeMemorySource("pve1", "node1", NodeMemorySnapshot{
			MemorySource: "rrd-available",
		})
	})

	t.Run("source change from existing triggers log", func(t *testing.T) {
		m := &Monitor{
			nodeSnapshots:  make(map[string]NodeMemorySnapshot),
			guestSnapshots: make(map[string]GuestMemorySnapshot),
		}

		// Pre-populate with existing snapshot
		key := makeNodeSnapshotKey("pve1", "node1")
		m.nodeSnapshots[key] = NodeMemorySnapshot{
			MemorySource: "rrd-data",
		}

		// Different source should trigger logging
		m.logNodeMemorySource("pve1", "node1", NodeMemorySnapshot{
			MemorySource: "rrd-available",
		})
	})

	t.Run("FallbackReason is logged when present", func(t *testing.T) {
		m := &Monitor{
			nodeSnapshots:  make(map[string]NodeMemorySnapshot),
			guestSnapshots: make(map[string]GuestMemorySnapshot),
		}

		// Should not panic - FallbackReason present
		m.logNodeMemorySource("pve1", "node1", NodeMemorySnapshot{
			MemorySource:   "previous-snapshot",
			FallbackReason: "no rrd data available",
		})
	})

	t.Run("Raw fields are logged when > 0", func(t *testing.T) {
		m := &Monitor{
			nodeSnapshots:  make(map[string]NodeMemorySnapshot),
			guestSnapshots: make(map[string]GuestMemorySnapshot),
		}

		// Should not panic - various Raw fields > 0
		m.logNodeMemorySource("pve1", "node1", NodeMemorySnapshot{
			MemorySource: "rrd-data",
			Raw: NodeMemoryRaw{
				Available:           1000000000,
				Buffers:             200000000,
				Cached:              500000000,
				TotalMinusUsed:      800000000,
				RRDAvailable:        900000000,
				RRDUsed:             100000000,
				RRDTotal:            1000000000,
				ProxmoxMemorySource: "rrd",
			},
		})
	})

	t.Run("Memory fields are logged when > 0", func(t *testing.T) {
		m := &Monitor{
			nodeSnapshots:  make(map[string]NodeMemorySnapshot),
			guestSnapshots: make(map[string]GuestMemorySnapshot),
		}

		// Should not panic - Memory fields > 0
		m.logNodeMemorySource("pve1", "node1", NodeMemorySnapshot{
			MemorySource: "rrd-data",
			Memory: models.Memory{
				Total: 16000000000,
				Used:  8000000000,
				Free:  8000000000,
				Usage: 0.5,
			},
		})
	})

	t.Run("all warn sources", func(t *testing.T) {
		warnSources := []string{"", "nodes-endpoint", "node-status-used", "previous-snapshot"}

		for _, source := range warnSources {
			t.Run("source_"+source, func(t *testing.T) {
				m := &Monitor{
					nodeSnapshots:  make(map[string]NodeMemorySnapshot),
					guestSnapshots: make(map[string]GuestMemorySnapshot),
				}

				// Should not panic
				m.logNodeMemorySource("pve1", "node1", NodeMemorySnapshot{
					MemorySource: source,
				})
			})
		}
	})

	t.Run("debug sources", func(t *testing.T) {
		debugSources := []string{"rrd-data", "rrd-available", "node-status-available", "calculated"}

		for _, source := range debugSources {
			t.Run("source_"+source, func(t *testing.T) {
				m := &Monitor{
					nodeSnapshots:  make(map[string]NodeMemorySnapshot),
					guestSnapshots: make(map[string]GuestMemorySnapshot),
				}

				// Should not panic
				m.logNodeMemorySource("pve1", "node1", NodeMemorySnapshot{
					MemorySource: source,
				})
			})
		}
	})

	t.Run("new node with no prior snapshot logs", func(t *testing.T) {
		m := &Monitor{
			nodeSnapshots:  make(map[string]NodeMemorySnapshot),
			guestSnapshots: make(map[string]GuestMemorySnapshot),
		}

		// No existing snapshot, empty prevSource should not match non-empty source
		m.logNodeMemorySource("pve1", "node1", NodeMemorySnapshot{
			MemorySource: "rrd-data",
		})
	})

	t.Run("new node with empty source matches no prior snapshot", func(t *testing.T) {
		m := &Monitor{
			nodeSnapshots:  make(map[string]NodeMemorySnapshot),
			guestSnapshots: make(map[string]GuestMemorySnapshot),
		}

		// No existing snapshot means prevSource is "", matching empty MemorySource
		// This should skip logging due to prevSource == snapshot.MemorySource
		m.logNodeMemorySource("pve1", "node1", NodeMemorySnapshot{
			MemorySource: "",
		})
	})
}

func TestGetDiagnosticSnapshots(t *testing.T) {
	t.Run("nil Monitor returns empty set with non-nil slices", func(t *testing.T) {
		var m *Monitor
		result := m.GetDiagnosticSnapshots()

		if result.Nodes == nil {
			t.Error("Nodes should not be nil")
		}
		if result.Guests == nil {
			t.Error("Guests should not be nil")
		}
		if len(result.Nodes) != 0 {
			t.Errorf("Nodes length = %d, want 0", len(result.Nodes))
		}
		if len(result.Guests) != 0 {
			t.Errorf("Guests length = %d, want 0", len(result.Guests))
		}
	})

	t.Run("empty Monitor returns empty set", func(t *testing.T) {
		m := &Monitor{
			nodeSnapshots:  make(map[string]NodeMemorySnapshot),
			guestSnapshots: make(map[string]GuestMemorySnapshot),
		}

		result := m.GetDiagnosticSnapshots()

		if len(result.Nodes) != 0 {
			t.Errorf("Nodes length = %d, want 0", len(result.Nodes))
		}
		if len(result.Guests) != 0 {
			t.Errorf("Guests length = %d, want 0", len(result.Guests))
		}
	})

	t.Run("single node snapshot is returned", func(t *testing.T) {
		m := &Monitor{
			nodeSnapshots:  make(map[string]NodeMemorySnapshot),
			guestSnapshots: make(map[string]GuestMemorySnapshot),
		}

		m.nodeSnapshots[makeNodeSnapshotKey("pve1", "node1")] = NodeMemorySnapshot{
			Instance:     "pve1",
			Node:         "node1",
			MemorySource: "rrd-available",
		}

		result := m.GetDiagnosticSnapshots()

		if len(result.Nodes) != 1 {
			t.Fatalf("Nodes length = %d, want 1", len(result.Nodes))
		}

		if result.Nodes[0].Instance != "pve1" {
			t.Errorf("Instance = %q, want %q", result.Nodes[0].Instance, "pve1")
		}
		if result.Nodes[0].Node != "node1" {
			t.Errorf("Node = %q, want %q", result.Nodes[0].Node, "node1")
		}
		if result.Nodes[0].MemorySource != "rrd-available" {
			t.Errorf("MemorySource = %q, want %q", result.Nodes[0].MemorySource, "rrd-available")
		}
	})

	t.Run("single guest snapshot is returned", func(t *testing.T) {
		m := &Monitor{
			nodeSnapshots:  make(map[string]NodeMemorySnapshot),
			guestSnapshots: make(map[string]GuestMemorySnapshot),
		}

		m.guestSnapshots[makeGuestSnapshotKey("pve1", "qemu", "node1", 100)] = GuestMemorySnapshot{
			Instance:     "pve1",
			GuestType:    "qemu",
			Node:         "node1",
			VMID:         100,
			Name:         "testvm",
			MemorySource: "balloon",
		}

		result := m.GetDiagnosticSnapshots()

		if len(result.Guests) != 1 {
			t.Fatalf("Guests length = %d, want 1", len(result.Guests))
		}

		if result.Guests[0].Instance != "pve1" {
			t.Errorf("Instance = %q, want %q", result.Guests[0].Instance, "pve1")
		}
		if result.Guests[0].GuestType != "qemu" {
			t.Errorf("GuestType = %q, want %q", result.Guests[0].GuestType, "qemu")
		}
		if result.Guests[0].Node != "node1" {
			t.Errorf("Node = %q, want %q", result.Guests[0].Node, "node1")
		}
		if result.Guests[0].VMID != 100 {
			t.Errorf("VMID = %d, want %d", result.Guests[0].VMID, 100)
		}
		if result.Guests[0].Name != "testvm" {
			t.Errorf("Name = %q, want %q", result.Guests[0].Name, "testvm")
		}
	})

	t.Run("multiple nodes are sorted by instance then node", func(t *testing.T) {
		m := &Monitor{
			nodeSnapshots:  make(map[string]NodeMemorySnapshot),
			guestSnapshots: make(map[string]GuestMemorySnapshot),
		}

		// Add in unsorted order
		m.nodeSnapshots[makeNodeSnapshotKey("pve2", "node2")] = NodeMemorySnapshot{Instance: "pve2", Node: "node2"}
		m.nodeSnapshots[makeNodeSnapshotKey("pve1", "node2")] = NodeMemorySnapshot{Instance: "pve1", Node: "node2"}
		m.nodeSnapshots[makeNodeSnapshotKey("pve2", "node1")] = NodeMemorySnapshot{Instance: "pve2", Node: "node1"}
		m.nodeSnapshots[makeNodeSnapshotKey("pve1", "node1")] = NodeMemorySnapshot{Instance: "pve1", Node: "node1"}

		result := m.GetDiagnosticSnapshots()

		if len(result.Nodes) != 4 {
			t.Fatalf("Nodes length = %d, want 4", len(result.Nodes))
		}

		// Expected order: pve1|node1, pve1|node2, pve2|node1, pve2|node2
		expectedOrder := []struct {
			instance string
			node     string
		}{
			{"pve1", "node1"},
			{"pve1", "node2"},
			{"pve2", "node1"},
			{"pve2", "node2"},
		}

		for i, expected := range expectedOrder {
			if result.Nodes[i].Instance != expected.instance || result.Nodes[i].Node != expected.node {
				t.Errorf("Nodes[%d] = {%q, %q}, want {%q, %q}",
					i, result.Nodes[i].Instance, result.Nodes[i].Node, expected.instance, expected.node)
			}
		}
	})

	t.Run("multiple guests are sorted by instance then node then guestType then VMID", func(t *testing.T) {
		m := &Monitor{
			nodeSnapshots:  make(map[string]NodeMemorySnapshot),
			guestSnapshots: make(map[string]GuestMemorySnapshot),
		}

		// Add in unsorted order
		m.guestSnapshots[makeGuestSnapshotKey("pve2", "qemu", "node1", 100)] = GuestMemorySnapshot{Instance: "pve2", Node: "node1", GuestType: "qemu", VMID: 100}
		m.guestSnapshots[makeGuestSnapshotKey("pve1", "qemu", "node1", 101)] = GuestMemorySnapshot{Instance: "pve1", Node: "node1", GuestType: "qemu", VMID: 101}
		m.guestSnapshots[makeGuestSnapshotKey("pve1", "lxc", "node1", 200)] = GuestMemorySnapshot{Instance: "pve1", Node: "node1", GuestType: "lxc", VMID: 200}
		m.guestSnapshots[makeGuestSnapshotKey("pve1", "qemu", "node2", 100)] = GuestMemorySnapshot{Instance: "pve1", Node: "node2", GuestType: "qemu", VMID: 100}
		m.guestSnapshots[makeGuestSnapshotKey("pve1", "qemu", "node1", 100)] = GuestMemorySnapshot{Instance: "pve1", Node: "node1", GuestType: "qemu", VMID: 100}

		result := m.GetDiagnosticSnapshots()

		if len(result.Guests) != 5 {
			t.Fatalf("Guests length = %d, want 5", len(result.Guests))
		}

		// Expected order:
		// pve1|node1|lxc|200 (lxc < qemu)
		// pve1|node1|qemu|100
		// pve1|node1|qemu|101
		// pve1|node2|qemu|100
		// pve2|node1|qemu|100
		expectedOrder := []struct {
			instance  string
			node      string
			guestType string
			vmid      int
		}{
			{"pve1", "node1", "lxc", 200},
			{"pve1", "node1", "qemu", 100},
			{"pve1", "node1", "qemu", 101},
			{"pve1", "node2", "qemu", 100},
			{"pve2", "node1", "qemu", 100},
		}

		for i, expected := range expectedOrder {
			got := result.Guests[i]
			if got.Instance != expected.instance || got.Node != expected.node ||
				got.GuestType != expected.guestType || got.VMID != expected.vmid {
				t.Errorf("Guests[%d] = {%q, %q, %q, %d}, want {%q, %q, %q, %d}",
					i, got.Instance, got.Node, got.GuestType, got.VMID,
					expected.instance, expected.node, expected.guestType, expected.vmid)
			}
		}
	})

	t.Run("mix of nodes and guests", func(t *testing.T) {
		m := &Monitor{
			nodeSnapshots:  make(map[string]NodeMemorySnapshot),
			guestSnapshots: make(map[string]GuestMemorySnapshot),
		}

		// Add nodes
		m.nodeSnapshots[makeNodeSnapshotKey("pve1", "node1")] = NodeMemorySnapshot{Instance: "pve1", Node: "node1"}
		m.nodeSnapshots[makeNodeSnapshotKey("pve1", "node2")] = NodeMemorySnapshot{Instance: "pve1", Node: "node2"}

		// Add guests
		m.guestSnapshots[makeGuestSnapshotKey("pve1", "qemu", "node1", 100)] = GuestMemorySnapshot{Instance: "pve1", Node: "node1", GuestType: "qemu", VMID: 100}
		m.guestSnapshots[makeGuestSnapshotKey("pve1", "lxc", "node2", 200)] = GuestMemorySnapshot{Instance: "pve1", Node: "node2", GuestType: "lxc", VMID: 200}

		result := m.GetDiagnosticSnapshots()

		if len(result.Nodes) != 2 {
			t.Errorf("Nodes length = %d, want 2", len(result.Nodes))
		}
		if len(result.Guests) != 2 {
			t.Errorf("Guests length = %d, want 2", len(result.Guests))
		}

		// Verify nodes are sorted
		if result.Nodes[0].Node != "node1" || result.Nodes[1].Node != "node2" {
			t.Errorf("Nodes not in expected order: got [%s, %s], want [node1, node2]",
				result.Nodes[0].Node, result.Nodes[1].Node)
		}

		// Verify guests are sorted
		if result.Guests[0].Node != "node1" || result.Guests[1].Node != "node2" {
			t.Errorf("Guests not in expected order: got [%s, %s], want [node1, node2]",
				result.Guests[0].Node, result.Guests[1].Node)
		}
	})
}
