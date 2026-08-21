package hostmetrics

import (
	"context"
	"testing"
	"time"

	godisk "github.com/shirou/gopsutil/v4/disk"
)

// A hard-mounted network filesystem whose server is unreachable blocks
// statfs in an uninterruptible kernel wait. The collector must bound that
// call, keep reporting the healthy mounts, skip the stuck mount on later
// cycles without stacking goroutines, and re-include it once the stalled
// call finally returns. Refs discussion #1747.
func TestCollectDisks_UnresponsiveMountDoesNotBlockCycle(t *testing.T) {
	origPartitions := diskPartitions
	origUsage := diskUsage
	origTimeout := diskUsageTimeout
	defer func() {
		diskPartitions = origPartitions
		diskUsage = origUsage
		diskUsageTimeout = origTimeout
	}()

	diskUsageTimeout = 50 * time.Millisecond

	diskPartitions = func(ctx context.Context, all bool) ([]godisk.PartitionStat, error) {
		return []godisk.PartitionStat{
			{Device: "/dev/sda1", Mountpoint: "/", Fstype: "ext4"},
			{Device: "/dev/mapper/slow", Mountpoint: "/mnt/slow", Fstype: "ext4"},
		}, nil
	}

	release := make(chan struct{})
	inFlight := make(chan struct{}, 16)
	diskUsage = func(ctx context.Context, path string) (*godisk.UsageStat, error) {
		if path == "/mnt/slow" {
			inFlight <- struct{}{}
			<-release
		}
		total := uint64(100 * 1024 * 1024 * 1024)
		return &godisk.UsageStat{Total: total, Used: total / 2, Free: total / 2, UsedPercent: 50}, nil
	}

	mounts := func(ctx context.Context) []string {
		names := []string{}
		for _, disk := range collectDisksWithIncludes(ctx, nil, nil) {
			names = append(names, disk.Mountpoint)
		}
		return names
	}

	ctx := context.Background()

	start := time.Now()
	got := mounts(ctx)
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("collection took %s, the stuck mount blocked the cycle", elapsed)
	}
	if len(got) != 1 || got[0] != "/" {
		t.Fatalf("first cycle mounts = %v, want just /", got)
	}

	// Second cycle: the stalled call is still in flight, so the mount is
	// skipped without issuing another syscall.
	got = mounts(ctx)
	if len(got) != 1 || got[0] != "/" {
		t.Fatalf("second cycle mounts = %v, want just /", got)
	}
	if len(inFlight) != 1 {
		t.Fatalf("expected exactly one in-flight usage call for the stuck mount, got %d", len(inFlight))
	}

	// The mount recovers: the stalled call returns and the mount is
	// re-included on a later cycle.
	close(release)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, stuck := stuckDiskMounts.Load("/mnt/slow"); !stuck {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	got = mounts(ctx)
	if len(got) != 2 {
		t.Fatalf("post-recovery mounts = %v, want / and /mnt/slow", got)
	}
}

// Network filesystems are dropped by the fstype filter, so their usage must
// be decided before the syscall: a hard NFS mount with an unreachable server
// blocks statfs indefinitely even though the result would be discarded.
// Refs discussion #1747.
func TestCollectDisks_NetworkMountsNeverReachUsageSyscall(t *testing.T) {
	origPartitions := diskPartitions
	origUsage := diskUsage
	defer func() {
		diskPartitions = origPartitions
		diskUsage = origUsage
	}()

	diskPartitions = func(ctx context.Context, all bool) ([]godisk.PartitionStat, error) {
		return []godisk.PartitionStat{
			{Device: "/dev/sda1", Mountpoint: "/", Fstype: "ext4"},
			{Device: "nas1:/export", Mountpoint: "/mnt/pve/nas1", Fstype: "nfs4"},
			{Device: "//nas2/share", Mountpoint: "/mnt/smb", Fstype: "cifs"},
		}, nil
	}

	probed := map[string]bool{}
	diskUsage = func(ctx context.Context, path string) (*godisk.UsageStat, error) {
		probed[path] = true
		total := uint64(10 * 1024 * 1024 * 1024)
		return &godisk.UsageStat{Total: total, Used: total / 2, Free: total / 2, UsedPercent: 50}, nil
	}

	disks := collectDisksWithIncludes(context.Background(), nil, nil)
	if len(disks) != 1 || disks[0].Mountpoint != "/" {
		t.Fatalf("expected only / to be collected, got %+v", disks)
	}
	for _, mount := range []string{"/mnt/pve/nas1", "/mnt/smb"} {
		if probed[mount] {
			t.Fatalf("usage syscall was issued for filtered network mount %s", mount)
		}
	}

	// An explicit include opts the network mount back in, and only then is
	// its usage collected.
	probed = map[string]bool{}
	disks = collectDisksWithIncludes(context.Background(), nil, []string{"/mnt/pve/nas1"})
	if !probed["/mnt/pve/nas1"] {
		t.Fatal("explicitly included network mount was not probed")
	}
	found := false
	for _, disk := range disks {
		if disk.Mountpoint == "/mnt/pve/nas1" {
			found = true
		}
	}
	if !found {
		t.Fatalf("explicitly included network mount missing from collection: %+v", disks)
	}
}
