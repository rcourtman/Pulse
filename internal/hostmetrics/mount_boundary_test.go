package hostmetrics

import (
	"context"
	"reflect"
	"testing"

	godisk "github.com/shirou/gopsutil/v4/disk"
)

func TestCollectDisksKeepsSystemNamedSiblingVolumes(t *testing.T) {
	origPartitions, origUsage := diskPartitions, diskUsage
	t.Cleanup(func() { diskPartitions, diskUsage = origPartitions, origUsage })
	diskPartitions = func(context.Context, bool) ([]godisk.PartitionStat, error) {
		return []godisk.PartitionStat{
			{Device: "/dev/vda1", Mountpoint: "/snapshots", Fstype: "ext4"},
			{Device: "/dev/vdb1", Mountpoint: "/runtime", Fstype: "xfs"},
			{Device: "/dev/vdc1", Mountpoint: "/var/lib/docker-data", Fstype: "ext4"},
			{Device: "snap-image", Mountpoint: "/snap/image", Fstype: "squashfs"},
			{Device: "runtime", Mountpoint: "/run/worker", Fstype: "ext4"},
			{Device: "remote", Mountpoint: "/snapshots-remote", Fstype: "nfs4"},
			{Device: "/dev/vdd1", Mountpoint: "/snapshots-private", Fstype: "ext4"},
		}, nil
	}
	var calls []string
	diskUsage = func(_ context.Context, path string) (*godisk.UsageStat, error) {
		calls = append(calls, path)
		return &godisk.UsageStat{Total: 1000, Used: 950, Free: 50, UsedPercent: 95}, nil
	}
	disks := collectDisks(context.Background(), []string{"/snapshots-private"})
	want := []string{"/snapshots", "/runtime", "/var/lib/docker-data"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("usage probes = %v, want only eligible local volumes %v", calls, want)
	}
	if len(disks) != len(want) {
		t.Fatalf("local volumes missing from agent inventory: %+v", disks)
	}
	seen := map[string]bool{}
	for _, disk := range disks {
		seen[disk.Mountpoint] = true
		if disk.TotalBytes != 1000 || disk.UsedBytes != 950 || disk.FreeBytes != 50 || disk.Usage != 95 || disk.ExplicitlyIncluded {
			t.Fatalf("automatically admitted volume lost usage or became an override: %+v", disk)
		}
	}
	for _, mount := range want {
		if !seen[mount] {
			t.Errorf("missing local volume %q", mount)
		}
	}
}
