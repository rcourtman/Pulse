package hostmetrics

import (
	"context"
	"testing"

	godisk "github.com/shirou/gopsutil/v4/disk"
)

// BenchmarkCollectDiskFiltering measures the disk filtering and deduplication
// logic that runs on every collection cycle.
func BenchmarkCollectDiskFiltering(b *testing.B) {
	origPartitions := diskPartitions
	origUsage := diskUsage
	b.Cleanup(func() {
		diskPartitions = origPartitions
		diskUsage = origUsage
	})

	partitions := buildSamplePartitions(30)
	diskPartitions = func(ctx context.Context, all bool) ([]godisk.PartitionStat, error) {
		return partitions, nil
	}
	diskUsage = func(ctx context.Context, path string) (*godisk.UsageStat, error) {
		return &godisk.UsageStat{
			Path:        path,
			Fstype:      "ext4",
			Total:       1e12,
			Used:        5e11,
			Free:        5e11,
			UsedPercent: 50.0,
		}, nil
	}

	exclude := []string{"/snap/*", "/boot/efi"}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = collectDisks(context.Background(), exclude)
	}
}

// BenchmarkSummarizeZFSPools measures ZFS pool summarization with a mix of pool types.
func BenchmarkSummarizeZFSPools(b *testing.B) {
	datasets := []zfsDatasetUsage{
		{Pool: "rpool", Dataset: "rpool", Used: 100e9, Free: 400e9, Total: 500e9, Mountpoint: "/"},
		{Pool: "rpool", Dataset: "rpool/data", Used: 50e9, Free: 400e9, Total: 450e9, Mountpoint: "/data"},
		{Pool: "tank", Dataset: "tank", Used: 2e12, Free: 8e12, Total: 10e12, Mountpoint: "/tank"},
		{Pool: "tank", Dataset: "tank/vm-100-disk-0", Used: 100e9, Free: 8e12, Total: 8.1e12, Mountpoint: "-"},
		{Pool: "tank", Dataset: "tank/vm-101-disk-0", Used: 200e9, Free: 8e12, Total: 8.2e12, Mountpoint: "-"},
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = summarizeZFSPools(context.Background(), datasets)
	}
}

func buildSamplePartitions(n int) []godisk.PartitionStat {
	parts := make([]godisk.PartitionStat, n)
	mounts := []string{"/", "/home", "/var", "/tmp", "/boot", "/data", "/opt",
		"/snap/core/1234", "/snap/lxd/5678", "/boot/efi",
		"/mnt/backup1", "/mnt/backup2", "/srv", "/var/lib/docker",
		"/data/db", "/data/logs", "/data/cache", "/data/media",
		"/nfs/share1", "/nfs/share2", "/var/lib/kubelet",
		"/run/user/1000", "/dev/shm", "/sys/fs/cgroup",
		"/proc", "/dev/pts", "/run", "/run/lock",
		"/mnt/usb1", "/mnt/usb2", "/mnt/nfs"}
	for i := range parts {
		mount := mounts[i%len(mounts)]
		parts[i] = godisk.PartitionStat{
			Device:     "/dev/sda1",
			Mountpoint: mount,
			Fstype:     "ext4",
		}
	}
	return parts
}
