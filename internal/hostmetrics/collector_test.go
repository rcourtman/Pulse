package hostmetrics

import (
	"context"
	"testing"

	godisk "github.com/shirou/gopsutil/v4/disk"
	"github.com/stretchr/testify/assert"
)

func TestCollectDisks_ZFS_Deduplication(t *testing.T) {
	// Save original functions and restore after test
	origDiskPartitions := diskPartitions
	origDiskUsage := diskUsage
	defer func() {
		diskPartitions = origDiskPartitions
		diskUsage = origDiskUsage
	}()

	// Mock partitions
	diskPartitions = func(ctx context.Context, all bool) ([]godisk.PartitionStat, error) {
		return []godisk.PartitionStat{
			{Device: "tank/dataset1", Mountpoint: "/mnt/dataset1", Fstype: "zfs"},
			{Device: "tank/dataset2", Mountpoint: "/mnt/dataset2", Fstype: "zfs"},
			{Device: "tank/dataset3", Mountpoint: "/mnt/dataset3", Fstype: "zfs"},
		}, nil
	}

	// Mock usage
	diskUsage = func(ctx context.Context, path string) (*godisk.UsageStat, error) {
		// All datasets on the same pool share the same free space
		return &godisk.UsageStat{
			Total:       1000,
			Used:        500,
			Free:        500,
			UsedPercent: 50.0,
		}, nil
	}

	// Mock zpool stats query to fail so we test the fallback logic (which uses the datasets)
	// We can't easily mock queryZpoolStats because it's in zfs.go and not exported/variable-ized in the same way,
	// but we can rely on the fact that `exec.LookPath("zpool")` will likely fail or `zpool list` will fail in the test environment.
	// However, to be sure, we should probably mock it.
	// But wait, `summarizeZFSPools` calls `queryZpoolStats`.
	// `queryZpoolStats` is a variable in zfs.go! `var queryZpoolStats = fetchZpoolStats`

	// Let's mock queryZpoolStats too.
	origQueryZpoolStats := queryZpoolStats
	defer func() { queryZpoolStats = origQueryZpoolStats }()

	queryZpoolStats = func(ctx context.Context, pools []string) (map[string]zpoolStats, error) {
		return nil, assert.AnError // Simulate failure
	}

	ctx := context.Background()
	disks := collectDisks(ctx)

	// Should return 1 disk (the pool), not 3
	assert.Len(t, disks, 1)
	assert.Equal(t, "tank", disks[0].Device)
	assert.Equal(t, int64(1000), disks[0].TotalBytes)
}

func TestCollectDisks_ZFS_CaseInsensitive(t *testing.T) {
	// Save original functions
	origDiskPartitions := diskPartitions
	origDiskUsage := diskUsage
	defer func() {
		diskPartitions = origDiskPartitions
		diskUsage = origDiskUsage
	}()

	// Mock partitions with "ZFS" (uppercase)
	diskPartitions = func(ctx context.Context, all bool) ([]godisk.PartitionStat, error) {
		return []godisk.PartitionStat{
			{Device: "tank/dataset1", Mountpoint: "/mnt/dataset1", Fstype: "ZFS"},
			{Device: "tank/dataset2", Mountpoint: "/mnt/dataset2", Fstype: "ZFS"},
		}, nil
	}

	diskUsage = func(ctx context.Context, path string) (*godisk.UsageStat, error) {
		return &godisk.UsageStat{Total: 1000, Used: 500, Free: 500}, nil
	}

	// Mock queryZpoolStats to fail
	origQueryZpoolStats := queryZpoolStats
	defer func() { queryZpoolStats = origQueryZpoolStats }()
	queryZpoolStats = func(ctx context.Context, pools []string) (map[string]zpoolStats, error) {
		return nil, assert.AnError
	}

	ctx := context.Background()
	disks := collectDisks(ctx)

	// If case-sensitive check fails, it will return 2 disks
	// If we fix it, it should return 1
	assert.Len(t, disks, 1)
}

func TestCollectDisks_ZFS_Fuse(t *testing.T) {
	// Save original functions
	origDiskPartitions := diskPartitions
	origDiskUsage := diskUsage
	defer func() {
		diskPartitions = origDiskPartitions
		diskUsage = origDiskUsage
	}()

	// Mock partitions with "fuse.zfs"
	diskPartitions = func(ctx context.Context, all bool) ([]godisk.PartitionStat, error) {
		return []godisk.PartitionStat{
			{Device: "tank/dataset1", Mountpoint: "/mnt/dataset1", Fstype: "fuse.zfs"},
			{Device: "tank/dataset2", Mountpoint: "/mnt/dataset2", Fstype: "fuse.zfs"},
		}, nil
	}

	diskUsage = func(ctx context.Context, path string) (*godisk.UsageStat, error) {
		return &godisk.UsageStat{Total: 1000, Used: 500, Free: 500}, nil
	}

	// Mock queryZpoolStats to fail
	origQueryZpoolStats := queryZpoolStats
	defer func() { queryZpoolStats = origQueryZpoolStats }()
	queryZpoolStats = func(ctx context.Context, pools []string) (map[string]zpoolStats, error) {
		return nil, assert.AnError
	}

	ctx := context.Background()
	disks := collectDisks(ctx)

	// If check fails, it will return 2 disks
	assert.Len(t, disks, 1)
}
