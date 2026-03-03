package hostmetrics

import (
	"context"
	"encoding/json"
	"errors"
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

// TestCollectDisks_SkipsNetworkMountsBeforeUsageProbe verifies network filesystems
// are filtered before disk usage calls to prevent hangs on stale remote mounts.
func TestCollectDisks_SkipsNetworkMountsBeforeUsageProbe(t *testing.T) {
	origPartitions := diskPartitions
	origUsage := diskUsage
	defer func() {
		diskPartitions = origPartitions
		diskUsage = origUsage
	}()

	diskPartitions = func(ctx context.Context, all bool) ([]godisk.PartitionStat, error) {
		return []godisk.PartitionStat{
			{Device: "nas:/pool/share", Mountpoint: "/mnt/nas", Fstype: "nfs4"},
			{Device: "/dev/sda1", Mountpoint: "/", Fstype: "ext4"},
		}, nil
	}

	usageCalls := 0
	diskUsage = func(ctx context.Context, path string) (*godisk.UsageStat, error) {
		usageCalls++
		if path == "/mnt/nas" {
			t.Fatalf("diskUsage was called for network mount %q", path)
		}
		return &godisk.UsageStat{
			Total:       1000,
			Used:        400,
			Free:        600,
			UsedPercent: 40.0,
		}, nil
	}

	disks := collectDisks(context.Background(), nil)
	if usageCalls != 1 {
		t.Fatalf("expected diskUsage to be called once for local filesystems, got %d calls", usageCalls)
	}
	if len(disks) != 1 {
		t.Fatalf("expected 1 disk after skipping network mounts, got %d", len(disks))
	}
	if disks[0].Mountpoint != "/" {
		t.Fatalf("expected local root mount to remain, got %q", disks[0].Mountpoint)
	}
}

func TestCollectDisks_DoesNotPreSkipFuseZFS(t *testing.T) {
	origPartitions := diskPartitions
	origUsage := diskUsage
	origQuery := queryZpoolStats
	defer func() {
		diskPartitions = origPartitions
		diskUsage = origUsage
		queryZpoolStats = origQuery
	}()

	diskPartitions = func(ctx context.Context, all bool) ([]godisk.PartitionStat, error) {
		return []godisk.PartitionStat{
			{Device: "tank/data", Mountpoint: "/tank/data", Fstype: "fuse.zfs"},
		}, nil
	}

	usageCalls := 0
	diskUsage = func(ctx context.Context, path string) (*godisk.UsageStat, error) {
		usageCalls++
		return &godisk.UsageStat{
			Total:       1000,
			Used:        200,
			Free:        800,
			UsedPercent: 20.0,
		}, nil
	}

	// Force fallback path so this test does not depend on system zpool binaries.
	queryZpoolStats = func(ctx context.Context, pools []string) (map[string]zpoolStats, error) {
		return nil, errors.New("zpool unavailable in test")
	}

	disks := collectDisks(context.Background(), nil)
	if usageCalls != 1 {
		t.Fatalf("expected diskUsage to be called for fuse.zfs dataset, got %d calls", usageCalls)
	}
	if len(disks) != 1 {
		t.Fatalf("expected 1 summarized zfs disk, got %d", len(disks))
	}
	if disks[0].Filesystem != "zfs" {
		t.Fatalf("expected filesystem type zfs, got %q", disks[0].Filesystem)
	}
}
