package models

import "testing"

// TestMergeLinkedHostDisksIntoGuests covers #1438: when a unified pulse-agent
// runs inside a VM (LinkedVMID set), filesystems it reports that the
// qemu-guest-agent's fsinfo missed, typically ZFS mounts on PBS, must be
// surfaced in the VM Overview filesystem list. Mountpoints already covered by
// qemu-guest-agent fsinfo take precedence and are not touched.
func TestMergeLinkedHostDisksIntoGuests(t *testing.T) {
	t.Run("adds host-only mounts to a linked VM and updates aggregate", func(t *testing.T) {
		s := &StateSnapshot{
			VMs: []VM{
				{
					ID: "pve01-101",
					Disk: Disk{
						Total: 85_100_000_000,
						Used:  20_500_000_000,
						Free:  64_600_000_000,
						Usage: 24,
					},
					Disks: []Disk{
						{Mountpoint: "/", Total: 25_600_000_000, Used: 6_990_000_000, Free: 18_610_000_000, Type: "ext4"},
						{Mountpoint: "/mnt/datastore/pbs01s3repl01", Total: 59_500_000_000, Used: 13_500_000_000, Free: 46_000_000_000, Type: "ext4"},
					},
				},
			},
			Hosts: []Host{
				{
					ID:         "host-pbs01",
					Hostname:   "pbs01",
					LinkedVMID: "pve01-101",
					Disks: []Disk{
						{Mountpoint: "/", Total: 27_000_000_000, Used: 6_990_000_000, Type: "ext4"},
						{Mountpoint: "/mnt/datastore/pbs01replic", Total: 934_000_000_000, Used: 575_000_000_000, Free: 359_000_000_000, Type: "zfs"},
						{Mountpoint: "/mnt/datastore/pbs01s3repl01", Total: 62_700_000_000, Used: 13_500_000_000, Type: "zfs"},
					},
				},
			},
		}

		s.MergeLinkedHostDisksIntoGuests()

		vm := s.VMs[0]
		if got := len(vm.Disks); got != 3 {
			t.Fatalf("expected 3 disks, got %d", got)
		}
		if vm.Disks[0].Mountpoint != "/" || vm.Disks[0].Type != "ext4" {
			t.Errorf("first disk mutated: %+v", vm.Disks[0])
		}
		if vm.Disks[1].Mountpoint != "/mnt/datastore/pbs01s3repl01" || vm.Disks[1].Type != "ext4" {
			t.Errorf("second disk should remain qemu-guest-agent ext4 entry, got: %+v", vm.Disks[1])
		}
		if vm.Disks[2].Mountpoint != "/mnt/datastore/pbs01replic" || vm.Disks[2].Type != "zfs" {
			t.Errorf("expected ZFS dataset appended, got: %+v", vm.Disks[2])
		}

		expectedTotal := int64(85_100_000_000 + 934_000_000_000)
		if vm.Disk.Total != expectedTotal {
			t.Errorf("aggregate Total = %d, want %d", vm.Disk.Total, expectedTotal)
		}
		expectedUsed := int64(20_500_000_000 + 575_000_000_000)
		if vm.Disk.Used != expectedUsed {
			t.Errorf("aggregate Used = %d, want %d", vm.Disk.Used, expectedUsed)
		}
		if vm.Disk.Usage <= 0 || vm.Disk.Usage > 100 {
			t.Errorf("aggregate Usage out of range: %.2f", vm.Disk.Usage)
		}
	})

	t.Run("does not mutate VM when no host is linked", func(t *testing.T) {
		s := &StateSnapshot{
			VMs: []VM{
				{
					ID:    "pve01-101",
					Disk:  Disk{Total: 1000, Used: 500, Usage: 50},
					Disks: []Disk{{Mountpoint: "/", Total: 1000, Used: 500}},
				},
			},
			Hosts: []Host{
				{ID: "host-other", LinkedVMID: "pve01-999", Disks: []Disk{{Mountpoint: "/data", Total: 9999}}},
			},
		}

		s.MergeLinkedHostDisksIntoGuests()

		if len(s.VMs[0].Disks) != 1 {
			t.Errorf("expected VM to be unchanged, got %d disks", len(s.VMs[0].Disks))
		}
		if s.VMs[0].Disk.Total != 1000 {
			t.Errorf("expected aggregate unchanged, got Total=%d", s.VMs[0].Disk.Total)
		}
	})

	t.Run("merges into linked container", func(t *testing.T) {
		s := &StateSnapshot{
			Containers: []Container{
				{
					ID:    "pve01-200",
					Disks: []Disk{{Mountpoint: "/", Total: 8_000_000_000, Used: 1_000_000_000, Type: "ext4"}},
					Disk:  Disk{Total: 8_000_000_000, Used: 1_000_000_000, Usage: 12.5},
				},
			},
			Hosts: []Host{
				{
					ID:                "host-ct",
					LinkedContainerID: "pve01-200",
					Disks:             []Disk{{Mountpoint: "/var/lib/zfs", Total: 100_000_000_000, Used: 50_000_000_000, Type: "zfs"}},
				},
			},
		}

		s.MergeLinkedHostDisksIntoGuests()

		if len(s.Containers[0].Disks) != 2 {
			t.Fatalf("expected 2 disks on container, got %d", len(s.Containers[0].Disks))
		}
		if s.Containers[0].Disks[1].Mountpoint != "/var/lib/zfs" {
			t.Errorf("expected ZFS dataset appended to container, got %+v", s.Containers[0].Disks[1])
		}
		if s.Containers[0].Disk.Total != 108_000_000_000 {
			t.Errorf("aggregate Total = %d, want %d", s.Containers[0].Disk.Total, 108_000_000_000)
		}
	})

	t.Run("ignores host disks with empty mountpoints", func(t *testing.T) {
		s := &StateSnapshot{
			VMs: []VM{
				{ID: "pve01-101", Disks: []Disk{{Mountpoint: "/", Total: 1000}}},
			},
			Hosts: []Host{
				{
					ID:         "host-pbs01",
					LinkedVMID: "pve01-101",
					Disks: []Disk{
						{Mountpoint: "", Total: 9999},
						{Mountpoint: "/zfs", Total: 5000, Type: "zfs"},
					},
				},
			},
		}

		s.MergeLinkedHostDisksIntoGuests()

		if len(s.VMs[0].Disks) != 2 {
			t.Fatalf("expected only valid mountpoints merged, got %d", len(s.VMs[0].Disks))
		}
		if s.VMs[0].Disks[1].Mountpoint != "/zfs" {
			t.Errorf("expected /zfs appended, got %+v", s.VMs[0].Disks[1])
		}
	})

	t.Run("does not mutate underlying state slice", func(t *testing.T) {
		original := []Disk{{Mountpoint: "/", Total: 1000}}
		s := &StateSnapshot{
			VMs: []VM{
				{ID: "pve01-101", Disks: original},
			},
			Hosts: []Host{
				{ID: "host-pbs01", LinkedVMID: "pve01-101", Disks: []Disk{{Mountpoint: "/zfs", Total: 5000}}},
			},
		}

		s.MergeLinkedHostDisksIntoGuests()

		if len(original) != 1 {
			t.Errorf("merge mutated the original slice: len=%d, want 1", len(original))
		}
	})
}
