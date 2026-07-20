package hostmetrics

import (
	"context"
	"runtime"
	"runtime/debug"
	"testing"
	"time"

	godisk "github.com/shirou/gopsutil/v4/disk"
	goload "github.com/shirou/gopsutil/v4/load"
	gomem "github.com/shirou/gopsutil/v4/mem"
	gonet "github.com/shirou/gopsutil/v4/net"
)

func TestCollectUsesMemAvailableForLinuxHostMemory(t *testing.T) {
	origCPUCounts := cpuCounts
	origCPUPercent := cpuPercent
	origLoadAvg := loadAvg
	origVirtualMemory := virtualMemory
	origDiskPartitions := diskPartitions
	origDiskUsage := diskUsage
	origDiskIOCounters := diskIOCounters
	origNetInterfaces := netInterfaces
	origNetIOCounters := netIOCounters

	kib := uint64(1024)
	total := uint64(7796964) * kib
	free := uint64(444824) * kib
	available := uint64(1872820) * kib

	cpuCounts = func(ctx context.Context, logical bool) (int, error) { return 4, nil }
	cpuPercent = func(ctx context.Context, interval time.Duration, percpu bool) ([]float64, error) {
		return []float64{5.0}, nil
	}
	loadAvg = func(ctx context.Context) (*goload.AvgStat, error) {
		return &goload.AvgStat{Load1: 0.1, Load5: 0.2, Load15: 0.3}, nil
	}
	virtualMemory = func(ctx context.Context) (*gomem.VirtualMemoryStat, error) {
		return &gomem.VirtualMemoryStat{
			Total:       total,
			Used:        total - free,
			Free:        free,
			Available:   available,
			UsedPercent: float64(total-free) / float64(total) * 100,
		}, nil
	}
	diskPartitions = func(ctx context.Context, all bool) ([]godisk.PartitionStat, error) { return nil, nil }
	diskUsage = func(ctx context.Context, path string) (*godisk.UsageStat, error) { return nil, nil }
	diskIOCounters = func(ctx context.Context, names ...string) (map[string]godisk.IOCountersStat, error) {
		return nil, nil
	}
	netInterfaces = func(ctx context.Context) (gonet.InterfaceStatList, error) { return nil, nil }
	netIOCounters = func(ctx context.Context, pernic bool) ([]gonet.IOCountersStat, error) { return nil, nil }

	t.Cleanup(func() {
		cpuCounts = origCPUCounts
		cpuPercent = origCPUPercent
		loadAvg = origLoadAvg
		virtualMemory = origVirtualMemory
		diskPartitions = origDiskPartitions
		diskUsage = origDiskUsage
		diskIOCounters = origDiskIOCounters
		netInterfaces = origNetInterfaces
		netIOCounters = origNetIOCounters
	})

	snapshot, err := Collect(context.Background(), nil)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}

	wantUsed := int64(total - available)
	if snapshot.Memory.UsedBytes != wantUsed {
		t.Fatalf("UsedBytes = %d, want %d", snapshot.Memory.UsedBytes, wantUsed)
	}
	if snapshot.Memory.FreeBytes != int64(available) {
		t.Fatalf("FreeBytes = %d, want available %d", snapshot.Memory.FreeBytes, available)
	}
	wantUsage := float64(total-available) / float64(total) * 100
	if diff := snapshot.Memory.Usage - wantUsage; diff < -0.001 || diff > 0.001 {
		t.Fatalf("Usage = %.4f, want %.4f", snapshot.Memory.Usage, wantUsage)
	}
}

func TestCollectMemoryStability(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping memory regression in short mode")
	}

	origCPUCounts := cpuCounts
	origCPUPercent := cpuPercent
	origLoadAvg := loadAvg
	origVirtualMemory := virtualMemory
	origDiskPartitions := diskPartitions
	origDiskUsage := diskUsage
	origDiskIOCounters := diskIOCounters
	origNetInterfaces := netInterfaces
	origNetIOCounters := netIOCounters

	cpuCounts = func(ctx context.Context, logical bool) (int, error) { return 4, nil }
	cpuPercent = func(ctx context.Context, interval time.Duration, percpu bool) ([]float64, error) {
		return []float64{5.0}, nil
	}
	loadAvg = func(ctx context.Context) (*goload.AvgStat, error) {
		return &goload.AvgStat{Load1: 0.1, Load5: 0.2, Load15: 0.3}, nil
	}
	virtualMemory = func(ctx context.Context) (*gomem.VirtualMemoryStat, error) {
		return &gomem.VirtualMemoryStat{
			Total:       8 * 1024 * 1024 * 1024,
			Used:        4 * 1024 * 1024 * 1024,
			Free:        4 * 1024 * 1024 * 1024,
			UsedPercent: 50,
			SwapTotal:   1024 * 1024 * 1024,
			SwapFree:    512 * 1024 * 1024,
		}, nil
	}
	diskPartitions = func(ctx context.Context, all bool) ([]godisk.PartitionStat, error) {
		return []godisk.PartitionStat{{Device: "/dev/sda", Mountpoint: "/", Fstype: "ext4"}}, nil
	}
	diskUsage = func(ctx context.Context, path string) (*godisk.UsageStat, error) {
		return &godisk.UsageStat{Total: 100, Used: 50, Free: 50, UsedPercent: 50}, nil
	}
	diskIOCounters = func(ctx context.Context, names ...string) (map[string]godisk.IOCountersStat, error) {
		return map[string]godisk.IOCountersStat{
			"sda": {
				ReadBytes:  1,
				WriteBytes: 2,
				ReadCount:  1,
				WriteCount: 2,
			},
		}, nil
	}
	netInterfaces = func(ctx context.Context) (gonet.InterfaceStatList, error) {
		return gonet.InterfaceStatList{
			{
				Name:         "eth0",
				HardwareAddr: "00:11:22:33:44:55",
				Flags:        []string{"up"},
				Addrs:        []gonet.InterfaceAddr{{Addr: "192.168.1.10/24"}},
			},
		}, nil
	}
	netIOCounters = func(ctx context.Context, pernic bool) ([]gonet.IOCountersStat, error) {
		return []gonet.IOCountersStat{{Name: "eth0", BytesRecv: 1, BytesSent: 2}}, nil
	}

	t.Cleanup(func() {
		cpuCounts = origCPUCounts
		cpuPercent = origCPUPercent
		loadAvg = origLoadAvg
		virtualMemory = origVirtualMemory
		diskPartitions = origDiskPartitions
		diskUsage = origDiskUsage
		diskIOCounters = origDiskIOCounters
		netInterfaces = origNetInterfaces
		netIOCounters = origNetIOCounters
	})

	warmupIterations := 50
	measureIterations := 200

	for i := 0; i < warmupIterations; i++ {
		if _, err := Collect(context.Background(), nil); err != nil {
			t.Fatalf("warmup collect: %v", err)
		}
	}

	runtime.GC()
	debug.FreeOSMemory()
	var baseline runtime.MemStats
	runtime.ReadMemStats(&baseline)

	for i := 0; i < measureIterations; i++ {
		if _, err := Collect(context.Background(), nil); err != nil {
			t.Fatalf("collect: %v", err)
		}
	}

	runtime.GC()
	debug.FreeOSMemory()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	if baseline.HeapAlloc > 0 {
		allowed := baseline.HeapAlloc + 5*1024*1024
		growthRatio := float64(after.HeapAlloc) / float64(baseline.HeapAlloc)
		if after.HeapAlloc > allowed && growthRatio > 1.25 {
			t.Fatalf("heap allocation grew too much: baseline=%d final=%d ratio=%.2f", baseline.HeapAlloc, after.HeapAlloc, growthRatio)
		}
	}
}
