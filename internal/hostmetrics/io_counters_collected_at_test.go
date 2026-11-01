package hostmetrics

import (
	"context"
	"testing"
	"time"

	gocpu "github.com/shirou/gopsutil/v4/cpu"
	godisk "github.com/shirou/gopsutil/v4/disk"
	goload "github.com/shirou/gopsutil/v4/load"
	gomem "github.com/shirou/gopsutil/v4/mem"
	gonet "github.com/shirou/gopsutil/v4/net"
)

// The counter read stamp is what the server divides counter deltas by, so it
// must mark the disk and network counter reads rather than the start of the
// snapshot: earlier steps can stall, here a memory read that takes 20s. Every
// seam the snapshot reaches is stubbed, so the test touches no host or proc
// file; it replaces package globals and must stay sequential.
func TestCollectWithDiskFiltersStampsIOCountersAtTheirRead(t *testing.T) {
	start := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	clock := start
	var diskReadAt, netReadAt time.Time

	origCounts, origPercent, origTimes, origLoad, origMemory := cpuCounts, cpuPercent, cpuTimes, loadAvg, virtualMemory
	origPartitions, origDiskIO, origInterfaces, origNetIO, origNow := diskPartitions, diskIOCounters, netInterfaces, netIOCounters, ioCountersReadNow
	origARC := readZFSARCSize
	t.Cleanup(func() {
		cpuCounts, cpuPercent, cpuTimes, loadAvg, virtualMemory = origCounts, origPercent, origTimes, origLoad, origMemory
		diskPartitions, diskIOCounters, netInterfaces, netIOCounters, ioCountersReadNow = origPartitions, origDiskIO, origInterfaces, origNetIO, origNow
		readZFSARCSize = origARC
	})

	readZFSARCSize = func() (uint64, error) { return 0, nil }
	cpuCounts = func(context.Context, bool) (int, error) { return 4, nil }
	cpuPercent = func(context.Context, time.Duration, bool) ([]float64, error) { return []float64{5}, nil }
	cpuTimes = func(context.Context, bool) ([]gocpu.TimesStat, error) {
		return []gocpu.TimesStat{{User: 10, Idle: 90}}, nil
	}
	loadAvg = func(context.Context) (*goload.AvgStat, error) { return &goload.AvgStat{}, nil }
	virtualMemory = func(context.Context) (*gomem.VirtualMemoryStat, error) {
		clock = clock.Add(20 * time.Second)
		return &gomem.VirtualMemoryStat{Total: 8 << 30, Available: 4 << 30, Free: 2 << 30, Used: 4 << 30}, nil
	}
	diskPartitions = func(context.Context, bool) ([]godisk.PartitionStat, error) { return nil, nil }
	diskIOCounters = func(context.Context, ...string) (map[string]godisk.IOCountersStat, error) {
		diskReadAt = clock
		clock = clock.Add(time.Millisecond)
		return map[string]godisk.IOCountersStat{"sda": {Name: "sda", ReadBytes: 1 << 20, WriteBytes: 2 << 20}}, nil
	}
	netInterfaces = func(context.Context) (gonet.InterfaceStatList, error) {
		clock = clock.Add(time.Millisecond)
		return gonet.InterfaceStatList{{Name: "eth0", Addrs: gonet.InterfaceAddrList{{Addr: "192.0.2.10/24"}}}}, nil
	}
	netIOCounters = func(context.Context, bool) ([]gonet.IOCountersStat, error) {
		netReadAt = clock
		return []gonet.IOCountersStat{{Name: "eth0", BytesRecv: 3 << 20, BytesSent: 4 << 20}}, nil
	}
	ioCountersReadNow = func() time.Time { return clock }

	var collector Collector
	snapshot, err := collector.CollectWithDiskFilters(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("CollectWithDiskFilters: %v", err)
	}
	if len(snapshot.DiskIO) != 1 || len(snapshot.Network) != 1 {
		t.Fatalf("snapshot counters = disk %+v network %+v, want one of each", snapshot.DiskIO, snapshot.Network)
	}
	if !snapshot.IOCountersCollectedAt.Equal(netReadAt) {
		t.Fatalf("IOCountersCollectedAt = %v, want the network counter read at %v", snapshot.IOCountersCollectedAt, netReadAt)
	}
	if lag := snapshot.IOCountersCollectedAt.Sub(diskReadAt); lag < 0 || lag > 2*time.Millisecond {
		t.Fatalf("stamp is %v after the disk counter read, want only the interface listing between them", lag)
	}
	if snapshot.IOCountersCollectedAt.Sub(start) < 20*time.Second {
		t.Fatalf("stamp %v predates the stalled memory read; it must mark the counter reads", snapshot.IOCountersCollectedAt)
	}
}
