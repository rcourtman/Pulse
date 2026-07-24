package hostmetrics

import (
	"context"
	"encoding/json"
	"testing"

	godisk "github.com/shirou/gopsutil/v4/disk"
	gomem "github.com/shirou/gopsutil/v4/mem"
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

func TestCollectDisksExcludesFreeBSDFdescfsBeforeUsage(t *testing.T) {
	origPartitions := diskPartitions
	origUsage := diskUsage
	t.Cleanup(func() {
		diskPartitions = origPartitions
		diskUsage = origUsage
	})

	diskPartitions = func(context.Context, bool) ([]godisk.PartitionStat, error) {
		return []godisk.PartitionStat{
			{Device: "fdescfs", Mountpoint: "/var/run/samba/fd", Fstype: "fdescfs"},
			{Device: "/dev/ada0p2", Mountpoint: "/", Fstype: "ufs"},
		}, nil
	}

	usageCalls := make([]string, 0, 1)
	diskUsage = func(_ context.Context, path string) (*godisk.UsageStat, error) {
		usageCalls = append(usageCalls, path)
		return &godisk.UsageStat{
			Total:       100,
			Used:        25,
			Free:        75,
			UsedPercent: 25,
		}, nil
	}

	disks := collectDisks(context.Background(), []string{"/var/run/samba/fd"})
	if len(disks) != 1 || disks[0].Mountpoint != "/" {
		t.Fatalf("excluded fdescfs mount was reported: %+v", disks)
	}
	if len(usageCalls) != 1 || usageCalls[0] != "/" {
		t.Fatalf("disk usage was queried for an excluded fdescfs mount: %v", usageCalls)
	}
}

func TestCollectSplitsReclaimableCache(t *testing.T) {
	origVirtualMemory := virtualMemory
	t.Cleanup(func() { virtualMemory = origVirtualMemory })

	const gib = uint64(1024 * 1024 * 1024)
	virtualMemory = func(ctx context.Context) (*gomem.VirtualMemoryStat, error) {
		return &gomem.VirtualMemoryStat{
			Total: 16 * gib,
			Used:  6 * gib,
			Free:  4 * gib,
			// Available - Free = 6 GiB of reclaimable buff/cache.
			Available:   10 * gib,
			UsedPercent: 37.5,
		}, nil
	}

	snapshot, err := Collect(context.Background(), nil)
	if err != nil {
		t.Fatalf("Collect failed: %v", err)
	}

	if got, want := snapshot.Memory.CacheBytes, int64(6*gib); got != want {
		t.Fatalf("CacheBytes = %d, want %d", got, want)
	}
	if got, want := snapshot.Memory.UsedBytes, int64(6*gib); got != want {
		t.Fatalf("UsedBytes = %d, want %d", got, want)
	}
	if got, want := snapshot.Memory.FreeBytes, int64(4*gib); got != want {
		t.Fatalf("FreeBytes = %d, want %d", got, want)
	}
	sum := snapshot.Memory.UsedBytes + snapshot.Memory.CacheBytes + snapshot.Memory.FreeBytes
	if sum > snapshot.Memory.TotalBytes {
		t.Fatalf("used+cache+free = %d exceeds total %d", sum, snapshot.Memory.TotalBytes)
	}
}

func TestCollectDerivesMemoryPressureFromAvailableMemory(t *testing.T) {
	origVirtualMemory := virtualMemory
	t.Cleanup(func() { virtualMemory = origVirtualMemory })

	const gib = uint64(1024 * 1024 * 1024)
	virtualMemory = func(ctx context.Context) (*gomem.VirtualMemoryStat, error) {
		return &gomem.VirtualMemoryStat{
			Total: 16 * gib,
			// Some Linux/gopsutil paths can report Used as cache-inclusive. The
			// operator-facing pressure value should follow MemAvailable instead.
			Used:        15 * gib,
			Free:        1 * gib,
			Available:   8 * gib,
			UsedPercent: 93.75,
		}, nil
	}

	snapshot, err := Collect(context.Background(), nil)
	if err != nil {
		t.Fatalf("Collect failed: %v", err)
	}

	if got, want := snapshot.Memory.UsedBytes, int64(8*gib); got != want {
		t.Fatalf("UsedBytes = %d, want %d", got, want)
	}
	if got, want := snapshot.Memory.CacheBytes, int64(7*gib); got != want {
		t.Fatalf("CacheBytes = %d, want %d", got, want)
	}
	if got, want := snapshot.Memory.FreeBytes, int64(1*gib); got != want {
		t.Fatalf("FreeBytes = %d, want %d", got, want)
	}
	if got, want := snapshot.Memory.Usage, 50.0; got != want {
		t.Fatalf("Usage = %.2f, want %.2f", got, want)
	}
	sum := snapshot.Memory.UsedBytes + snapshot.Memory.CacheBytes + snapshot.Memory.FreeBytes
	if got, want := sum, snapshot.Memory.TotalBytes; got != want {
		t.Fatalf("used+cache+free = %d, want total %d", got, want)
	}
}
