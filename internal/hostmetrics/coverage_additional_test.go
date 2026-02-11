package hostmetrics

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	godisk "github.com/shirou/gopsutil/v4/disk"
)

func TestCollectCPUUsageBranches(t *testing.T) {
	origCPUPercent := cpuPercent
	t.Cleanup(func() {
		cpuPercent = origCPUPercent
	})

	cpuPercent = func(ctx context.Context, interval time.Duration, percpu bool) ([]float64, error) {
		return nil, errors.New("boom")
	}
	if _, err := collectCPUUsage(context.Background()); err == nil {
		t.Fatal("expected collectCPUUsage to return error when cpuPercent fails")
	}

	cpuPercent = func(ctx context.Context, interval time.Duration, percpu bool) ([]float64, error) {
		return []float64{}, nil
	}
	usage, err := collectCPUUsage(context.Background())
	if err != nil {
		t.Fatalf("collectCPUUsage unexpected error for empty percentages: %v", err)
	}
	if usage != 0 {
		t.Fatalf("expected empty percentages to return 0, got %v", usage)
	}

	cpuPercent = func(ctx context.Context, interval time.Duration, percpu bool) ([]float64, error) {
		return []float64{-4.2}, nil
	}
	usage, err = collectCPUUsage(context.Background())
	if err != nil {
		t.Fatalf("collectCPUUsage unexpected error for negative value: %v", err)
	}
	if usage != 0 {
		t.Fatalf("expected negative usage to clamp to 0, got %v", usage)
	}

	cpuPercent = func(ctx context.Context, interval time.Duration, percpu bool) ([]float64, error) {
		return []float64{147.9}, nil
	}
	usage, err = collectCPUUsage(context.Background())
	if err != nil {
		t.Fatalf("collectCPUUsage unexpected error for >100 value: %v", err)
	}
	if usage != 100 {
		t.Fatalf("expected >100 usage to clamp to 100, got %v", usage)
	}
}

func TestCollectDisksBranchCoverage(t *testing.T) {
	origPartitions := diskPartitions
	origUsage := diskUsage
	origQueryZpoolStats := queryZpoolStats
	t.Cleanup(func() {
		diskPartitions = origPartitions
		diskUsage = origUsage
		queryZpoolStats = origQueryZpoolStats
	})

	diskPartitions = func(ctx context.Context, all bool) ([]godisk.PartitionStat, error) {
		return []godisk.PartitionStat{
			{Device: "/dev/ignore", Mountpoint: "", Fstype: "ext4"},
			{Device: "/dev/dup-a", Mountpoint: "/dup", Fstype: "ext4"},
			{Device: "/dev/dup-b", Mountpoint: "/dup", Fstype: "ext4"},
			{Device: "/dev/excluded", Mountpoint: "/skip/me", Fstype: "ext4"},
			{Device: "/dev/err", Mountpoint: "/err", Fstype: "ext4"},
			{Device: "/dev/zero", Mountpoint: "/zero", Fstype: "ext4"},
			{Device: "/dev/tmp", Mountpoint: "/tmpfs", Fstype: "tmpfs"},
			{Device: "", Mountpoint: "/zfs-empty", Fstype: "zfs"},
			{Device: "tank/root", Mountpoint: "/zfs", Fstype: "zfs"},
			{Device: "/dev/data", Mountpoint: "/data", Fstype: "ext4"},
		}, nil
	}

	diskUsage = func(ctx context.Context, path string) (*godisk.UsageStat, error) {
		switch path {
		case "/dup":
			return &godisk.UsageStat{Total: 10, Used: 4, Free: 6, UsedPercent: 40}, nil
		case "/skip/me":
			return &godisk.UsageStat{Total: 100, Used: 10, Free: 90, UsedPercent: 10}, nil
		case "/err":
			return nil, errors.New("usage failure")
		case "/zero":
			return &godisk.UsageStat{Total: 0, Used: 0, Free: 0, UsedPercent: 0}, nil
		case "/tmpfs":
			return &godisk.UsageStat{Total: 100, Used: 100, Free: 0, UsedPercent: 100}, nil
		case "/zfs-empty":
			return &godisk.UsageStat{Total: 100, Used: 20, Free: 80, UsedPercent: 20}, nil
		case "/zfs":
			return &godisk.UsageStat{Total: 100, Used: 30, Free: 70, UsedPercent: 30}, nil
		case "/data":
			return &godisk.UsageStat{Total: 200, Used: 50, Free: 150, UsedPercent: 25}, nil
		default:
			return nil, errors.New("unexpected mountpoint")
		}
	}

	queryZpoolStats = func(ctx context.Context, pools []string) (map[string]zpoolStats, error) {
		return map[string]zpoolStats{
			"tank": {Size: 100, Alloc: 30, Free: 70},
		}, nil
	}

	disks := collectDisks(context.Background(), []string{"/skip/*"})
	if len(disks) != 3 {
		t.Fatalf("expected 3 disks, got %d: %+v", len(disks), disks)
	}

	byMount := make(map[string]bool, len(disks))
	for _, disk := range disks {
		byMount[disk.Mountpoint] = true
	}
	for _, expected := range []string{"/data", "/dup", "/zfs"} {
		if !byMount[expected] {
			t.Fatalf("expected mountpoint %s in collected disks: %+v", expected, disks)
		}
	}
	for _, skipped := range []string{"/skip/me", "/err", "/zero", "/tmpfs", "/zfs-empty"} {
		if byMount[skipped] {
			t.Fatalf("did not expect skipped mountpoint %s in collected disks: %+v", skipped, disks)
		}
	}
}

func TestCollectDisksPrefersShallowerMountpoint(t *testing.T) {
	origPartitions := diskPartitions
	origUsage := diskUsage
	t.Cleanup(func() {
		diskPartitions = origPartitions
		diskUsage = origUsage
	})

	diskPartitions = func(ctx context.Context, all bool) ([]godisk.PartitionStat, error) {
		return []godisk.PartitionStat{
			{Device: "/dev/sdb1", Mountpoint: "/srv/data/sub", Fstype: "ext4"},
			{Device: "/dev/sdb1", Mountpoint: "/srv/data", Fstype: "ext4"},
		}, nil
	}
	diskUsage = func(ctx context.Context, path string) (*godisk.UsageStat, error) {
		return &godisk.UsageStat{Total: 500, Used: 125, Free: 375, UsedPercent: 25}, nil
	}

	disks := collectDisks(context.Background(), nil)
	if len(disks) != 1 {
		t.Fatalf("expected 1 deduplicated disk, got %d: %+v", len(disks), disks)
	}
	if disks[0].Mountpoint != "/srv/data" {
		t.Fatalf("expected shallower mountpoint /srv/data, got %s", disks[0].Mountpoint)
	}
}

func TestCollectDiskIOFiltersAndError(t *testing.T) {
	origDiskIOCounters := diskIOCounters
	t.Cleanup(func() {
		diskIOCounters = origDiskIOCounters
	})

	diskIOCounters = func(ctx context.Context, names ...string) (map[string]godisk.IOCountersStat, error) {
		return map[string]godisk.IOCountersStat{
			"sda":       {ReadBytes: 1},
			"sda1":      {ReadBytes: 2},
			"nvme0n1":   {ReadBytes: 3},
			"nvme0n1p1": {ReadBytes: 4},
			"xvda10":    {ReadBytes: 5},
			"loop0":     {ReadBytes: 6},
			"ram0":      {ReadBytes: 7},
			"dm-0":      {ReadBytes: 8},
			"sdb":       {ReadBytes: 9},
			"md0":       {ReadBytes: 10},
		}, nil
	}

	disks := collectDiskIO(context.Background(), []string{"sdb"})
	if len(disks) != 3 {
		t.Fatalf("expected 3 disk I/O entries after filtering, got %d: %+v", len(disks), disks)
	}

	gotDevices := make([]string, 0, len(disks))
	for _, disk := range disks {
		gotDevices = append(gotDevices, disk.Device)
	}
	wantDevices := []string{"md0", "nvme0n1", "sda"}
	if !reflect.DeepEqual(gotDevices, wantDevices) {
		t.Fatalf("unexpected disk I/O devices: got %v, want %v", gotDevices, wantDevices)
	}

	diskIOCounters = func(ctx context.Context, names ...string) (map[string]godisk.IOCountersStat, error) {
		return nil, errors.New("io unavailable")
	}
	if got := collectDiskIO(context.Background(), nil); got != nil {
		t.Fatalf("expected nil when diskIOCounters errors, got %+v", got)
	}
}

func TestIsPartitionPatterns(t *testing.T) {
	tests := []struct {
		name   string
		device string
		want   bool
	}{
		{name: "traditional single digit", device: "sda1", want: true},
		{name: "traditional multi digit", device: "sda10", want: true},
		{name: "virtio partition", device: "vda2", want: true},
		{name: "xen partition", device: "xvda12", want: true},
		{name: "nvme partition", device: "nvme0n1p1", want: true},
		{name: "zfs zd partition", device: "zd16p1", want: true},
		{name: "md whole device", device: "md0", want: false},
		{name: "loop device", device: "loop0", want: false},
		{name: "nvme whole device", device: "nvme0n1", want: false},
		{name: "traditional whole disk", device: "sda", want: false},
		{name: "zfs whole zd", device: "zd0", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPartition(tt.device)
			if got != tt.want {
				t.Fatalf("isPartition(%q) = %v, want %v", tt.device, got, tt.want)
			}
		})
	}
}

func TestFetchZpoolStats(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script based zpool mock is unix-only")
	}

	t.Run("empty pools", func(t *testing.T) {
		stats, err := fetchZpoolStats(context.Background(), nil)
		if err != nil {
			t.Fatalf("fetchZpoolStats unexpected error for empty pools: %v", err)
		}
		if stats != nil {
			t.Fatalf("expected nil stats for empty pools, got %+v", stats)
		}
	})

	t.Run("success", func(t *testing.T) {
		dir := writeFakeZpool(t, "#!/bin/sh\nprintf \"tank\\t100\\t40\\t60\\n\"\nprintf \"bpool\\t200\\t80\\t120\\n\"\n")
		t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

		stats, err := fetchZpoolStats(context.Background(), []string{"tank", "bpool"})
		if err != nil {
			t.Fatalf("fetchZpoolStats unexpected error: %v", err)
		}

		want := map[string]zpoolStats{
			"tank":  {Size: 100, Alloc: 40, Free: 60},
			"bpool": {Size: 200, Alloc: 80, Free: 120},
		}
		if !reflect.DeepEqual(stats, want) {
			t.Fatalf("fetchZpoolStats mismatch: got %+v, want %+v", stats, want)
		}
	})

	t.Run("command error", func(t *testing.T) {
		dir := writeFakeZpool(t, "#!/bin/sh\necho \"boom\" 1>&2\nexit 7\n")
		t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

		stats, err := fetchZpoolStats(context.Background(), []string{"tank"})
		if err == nil {
			t.Fatal("expected fetchZpoolStats to fail when zpool command exits non-zero")
		}
		if stats != nil {
			t.Fatalf("expected nil stats on command failure, got %+v", stats)
		}
	})

	t.Run("parse error", func(t *testing.T) {
		dir := writeFakeZpool(t, "#!/bin/sh\nprintf \"not\\tusable\\tdata\\n\"\n")
		t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

		stats, err := fetchZpoolStats(context.Background(), []string{"tank"})
		if err == nil {
			t.Fatal("expected parse error for unusable zpool output")
		}
		if !strings.Contains(err.Error(), "no usable data") {
			t.Fatalf("expected no usable data error, got %v", err)
		}
		if stats != nil {
			t.Fatalf("expected nil stats on parse failure, got %+v", stats)
		}
	})
}

func writeFakeZpool(t *testing.T, script string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "zpool")
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatalf("failed to write fake zpool binary: %v", err)
	}
	return dir
}
