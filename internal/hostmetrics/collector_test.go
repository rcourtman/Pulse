package hostmetrics

import (
	"context"
	"encoding/json"
	"testing"

	godisk "github.com/shirou/gopsutil/v4/disk"
)

func TestCollectDiskIO(t *testing.T) {
	ctx := context.Background()

	snapshot, err := Collect(ctx, nil)
	if err != nil {
		t.Fatalf("Collect failed: %v", err)
	}

	t.Logf("DiskIO count: %d", len(snapshot.DiskIO))

	if len(snapshot.DiskIO) == 0 {
		t.Error("Expected disk IO data but got none")
	}

	data, _ := json.MarshalIndent(snapshot.DiskIO, "", "  ")
	t.Logf("DiskIO data:\n%s", string(data))
}

// TestCollectDisks_DeviceDeduplication verifies that multiple mount points
// sharing the same device and total bytes are deduplicated (issue #953).
// This prevents Synology NAS and similar systems from over-counting storage.
func TestCollectDisks_DeviceDeduplication(t *testing.T) {
	// Save original function and restore after test
	origPartitions := diskPartitions
	origUsage := diskUsage
	defer func() {
		diskPartitions = origPartitions
		diskUsage = origUsage
	}()

	// Simulate Synology-like scenario: multiple shared folders on same volume
	diskPartitions = func(ctx context.Context, all bool) ([]godisk.PartitionStat, error) {
		return []godisk.PartitionStat{
			{Device: "/dev/vg1000/lv", Mountpoint: "/volume1", Fstype: "btrfs"},
			{Device: "/dev/vg1000/lv", Mountpoint: "/volume1/docker", Fstype: "btrfs"},
			{Device: "/dev/vg1000/lv", Mountpoint: "/volume1/photos", Fstype: "btrfs"},
			{Device: "/dev/vg1000/lv", Mountpoint: "/volume1/music", Fstype: "btrfs"},
			{Device: "/dev/sdb1", Mountpoint: "/mnt/backup", Fstype: "ext4"}, // Different device
		}, nil
	}

	// All volume1 mounts report the same 16TB total
	const volume1Total = uint64(16 * 1024 * 1024 * 1024 * 1024) // 16 TB
	const backupTotal = uint64(4 * 1024 * 1024 * 1024 * 1024)   // 4 TB

	diskUsage = func(ctx context.Context, path string) (*godisk.UsageStat, error) {
		if path == "/mnt/backup" {
			return &godisk.UsageStat{
				Total:       backupTotal,
				Used:        backupTotal / 2,
				Free:        backupTotal / 2,
				UsedPercent: 50.0,
			}, nil
		}
		// All volume1 paths report the same totals
		return &godisk.UsageStat{
			Total:       volume1Total,
			Used:        volume1Total / 4,
			Free:        volume1Total * 3 / 4,
			UsedPercent: 25.0,
		}, nil
	}

	ctx := context.Background()
	disks := collectDisks(ctx, nil)

	// Should have exactly 2 disks: /volume1 (deduplicated) and /mnt/backup
	if len(disks) != 2 {
		t.Errorf("Expected 2 disks after deduplication, got %d", len(disks))
		for _, d := range disks {
			t.Logf("  - %s (%s): %d bytes", d.Mountpoint, d.Device, d.TotalBytes)
		}
	}

	// Verify the shallowest mountpoint (/volume1) was kept
	foundVolume1 := false
	foundBackup := false
	for _, d := range disks {
		if d.Mountpoint == "/volume1" {
			foundVolume1 = true
		}
		if d.Mountpoint == "/mnt/backup" {
			foundBackup = true
		}
	}

	if !foundVolume1 {
		t.Error("Expected /volume1 to be included (shallowest path)")
	}
	if !foundBackup {
		t.Error("Expected /mnt/backup to be included (different device)")
	}

	// Calculate total - should be 16TB + 4TB = 20TB, not 64TB + 4TB = 68TB
	var total int64
	for _, d := range disks {
		total += d.TotalBytes
	}
	expectedTotal := int64(volume1Total + backupTotal)
	if total != expectedTotal {
		t.Errorf("Total storage should be %d bytes, got %d bytes", expectedTotal, total)
	}
}
