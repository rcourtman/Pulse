package hostmetrics

import (
	"context"
	"math"
	"testing"
	"time"

	gocpu "github.com/shirou/gopsutil/v4/cpu"
	godisk "github.com/shirou/gopsutil/v4/disk"
	goload "github.com/shirou/gopsutil/v4/load"
	gomem "github.com/shirou/gopsutil/v4/mem"
	gonet "github.com/shirou/gopsutil/v4/net"
)

// Two near-synchronous reporting loops must each retain a full-interval
// baseline, not measure the brief busy burst between their reports (#1894).
func TestIssue1894InterleavedCollectors(t *testing.T) {
	origCPUCounts := cpuCounts
	origCPUPercent := cpuPercent
	origCPUTimes := cpuTimes
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
	cpuTimes = func(ctx context.Context, percpu bool) ([]gocpu.TimesStat, error) {
		return []gocpu.TimesStat{{CPU: "cpu-total", User: 100, System: 50, Idle: 1000}}, nil
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
		cpuTimes = origCPUTimes
		loadAvg = origLoadAvg
		virtualMemory = origVirtualMemory
		diskPartitions = origDiskPartitions
		diskUsage = origDiskUsage
		diskIOCounters = origDiskIOCounters
		netInterfaces = origNetInterfaces
		netIOCounters = origNetIOCounters
	})

	samples := []gocpu.TimesStat{
		{User: 100, Idle: 1000},
		{User: 100.8, Idle: 1000.2},
		{User: 103, Idle: 1057},
		{User: 103.8, Idle: 1057.2},
		{User: 106, Idle: 1114},
		{User: 106.8, Idle: 1114.2},
	}
	index, spots := 0, 0
	cpuTimes = func(context.Context, bool) ([]gocpu.TimesStat, error) {
		sample := samples[index]
		index++
		return []gocpu.TimesStat{sample}, nil
	}
	cpuPercent = func(context.Context, time.Duration, bool) ([]float64, error) {
		spots++
		return []float64{80}, nil
	}
	var host, docker Collector
	for cycle := 0; cycle < 3; cycle++ {
		a, err := host.Collect(context.Background(), nil)
		if err != nil {
			t.Fatal(err)
		}
		b, err := docker.CollectWithDiskFilters(context.Background(), nil, []string{"/"})
		if err != nil {
			t.Fatal(err)
		}
		want := 5.0
		if cycle == 0 {
			want = 80
		}
		for _, got := range []float64{a.CPUUsagePercent, b.CPUUsagePercent} {
			if math.Abs(got-want) > 1e-9 {
				t.Fatalf("cycle %d: CPU = %v, want %v", cycle, got, want)
			}
		}
	}
	if spots != 2 {
		t.Fatalf("spot samples = %d, want startup only (2)", spots)
	}
}
