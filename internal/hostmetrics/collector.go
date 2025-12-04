package hostmetrics

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	"github.com/rcourtman/pulse-go-rewrite/pkg/fsfilters"
	gocpu "github.com/shirou/gopsutil/v4/cpu"
	godisk "github.com/shirou/gopsutil/v4/disk"
	goload "github.com/shirou/gopsutil/v4/load"
	gomem "github.com/shirou/gopsutil/v4/mem"
	gonet "github.com/shirou/gopsutil/v4/net"
)

// System call wrappers for testing
var (
	cpuCounts      = gocpu.CountsWithContext
	cpuPercent     = gocpu.PercentWithContext
	loadAvg        = goload.AvgWithContext
	virtualMemory  = gomem.VirtualMemoryWithContext
	diskPartitions = godisk.PartitionsWithContext
	diskUsage      = godisk.UsageWithContext
	netInterfaces  = gonet.InterfacesWithContext
	netIOCounters  = gonet.IOCountersWithContext
)

// Snapshot represents a host resource utilisation sample.
type Snapshot struct {
	CPUUsagePercent float64
	CPUCount        int
	LoadAverage     []float64
	Memory          agentshost.MemoryMetric
	Disks           []agentshost.Disk
	Network         []agentshost.NetworkInterface
}

// Collect gathers a point-in-time snapshot of host resource utilisation.
func Collect(ctx context.Context) (Snapshot, error) {
	collectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var snapshot Snapshot

	if cpuCount, err := cpuCounts(collectCtx, true); err == nil {
		snapshot.CPUCount = cpuCount
	}

	if cpuUsage, err := collectCPUUsage(collectCtx); err == nil {
		snapshot.CPUUsagePercent = cpuUsage
	}

	if loadAvg, err := loadAvg(collectCtx); err == nil && loadAvg != nil {
		snapshot.LoadAverage = []float64{loadAvg.Load1, loadAvg.Load5, loadAvg.Load15}
	}

	memStats, err := virtualMemory(collectCtx)
	if err != nil {
		return Snapshot{}, fmt.Errorf("memory stats: %w", err)
	}

	swapUsed := int64(0)
	if memStats.SwapTotal > memStats.SwapFree {
		swapUsed = int64(memStats.SwapTotal - memStats.SwapFree)
	}

	snapshot.Memory = agentshost.MemoryMetric{
		TotalBytes: int64(memStats.Total),
		UsedBytes:  int64(memStats.Used),
		FreeBytes:  int64(memStats.Free),
		Usage:      memStats.UsedPercent,
		SwapTotal:  int64(memStats.SwapTotal),
		SwapUsed:   swapUsed,
	}

	snapshot.Disks = collectDisks(collectCtx)
	snapshot.Network = collectNetwork(collectCtx)

	return snapshot, nil
}

func collectCPUUsage(ctx context.Context) (float64, error) {
	percentages, err := cpuPercent(ctx, time.Second, false)
	if err != nil {
		return 0, err
	}
	if len(percentages) == 0 {
		return 0, nil
	}

	usage := percentages[0]
	if usage < 0 {
		usage = 0
	}
	if usage > 100 {
		usage = 100
	}
	return usage, nil
}

func collectDisks(ctx context.Context) []agentshost.Disk {
	partitions, err := diskPartitions(ctx, true)
	if err != nil {
		return nil
	}

	disks := make([]agentshost.Disk, 0, len(partitions))
	seen := make(map[string]struct{}, len(partitions))
	zfsDatasets := make([]zfsDatasetUsage, 0)

	for _, part := range partitions {
		if part.Mountpoint == "" {
			continue
		}
		if _, ok := seen[part.Mountpoint]; ok {
			continue
		}
		seen[part.Mountpoint] = struct{}{}

		usage, err := diskUsage(ctx, part.Mountpoint)
		if err != nil {
			continue
		}
		if usage.Total == 0 {
			continue
		}

		if strings.EqualFold(part.Fstype, "zfs") || strings.EqualFold(part.Fstype, "fuse.zfs") {
			pool := zfsPoolFromDevice(part.Device)
			if pool == "" {
				continue
			}
			if fsfilters.ShouldIgnoreReadOnlyFilesystem(part.Fstype, usage.Total, usage.Used) {
				continue
			}

			zfsDatasets = append(zfsDatasets, zfsDatasetUsage{
				Pool:       pool,
				Dataset:    part.Device,
				Mountpoint: part.Mountpoint,
				Total:      usage.Total,
				Used:       usage.Used,
				Free:       usage.Free,
			})
			continue
		}

		// Skip filesystems that shouldn't be counted toward disk usage:
		// - Read-only filesystems (squashfs, erofs, iso9660) - always report near-full
		// - Virtual/pseudo filesystems (tmpfs, devtmpfs, cgroup, etc.)
		// - Container overlay paths (Docker/Podman layers on ZFS, including TrueNAS .ix-apps)
		// See issues #505, #690, #718, #790.
		if shouldSkip, _ := fsfilters.ShouldSkipFilesystem(part.Fstype, part.Mountpoint, usage.Total, usage.Used); shouldSkip {
			continue
		}

		disks = append(disks, agentshost.Disk{
			Device:     part.Device,
			Mountpoint: part.Mountpoint,
			Filesystem: part.Fstype,
			Type:       part.Fstype,
			TotalBytes: int64(usage.Total),
			UsedBytes:  int64(usage.Used),
			FreeBytes:  int64(usage.Free),
			Usage:      usage.UsedPercent,
		})
	}

	disks = append(disks, summarizeZFSPools(ctx, zfsDatasets)...)

	sort.Slice(disks, func(i, j int) bool { return disks[i].Mountpoint < disks[j].Mountpoint })
	return disks
}

func collectNetwork(ctx context.Context) []agentshost.NetworkInterface {
	ifaces, err := netInterfaces(ctx)
	if err != nil {
		return nil
	}

	ioCounters, err := netIOCounters(ctx, true)
	if err != nil {
		ioCounters = nil
	}
	ioMap := make(map[string]gonet.IOCountersStat, len(ioCounters))
	for _, stat := range ioCounters {
		ioMap[stat.Name] = stat
	}

	interfaces := make([]agentshost.NetworkInterface, 0, len(ifaces))

	for _, iface := range ifaces {
		if len(iface.Addrs) == 0 {
			continue
		}
		if isLoopback(iface.Flags) {
			continue
		}

		addresses := make([]string, 0, len(iface.Addrs))
		for _, addr := range iface.Addrs {
			if addr.Addr != "" {
				addresses = append(addresses, addr.Addr)
			}
		}
		if len(addresses) == 0 {
			continue
		}

		counter := ioMap[iface.Name]
		ifaceEntry := agentshost.NetworkInterface{
			Name:      iface.Name,
			MAC:       iface.HardwareAddr,
			Addresses: addresses,
			RXBytes:   counter.BytesRecv,
			TXBytes:   counter.BytesSent,
		}

		interfaces = append(interfaces, ifaceEntry)
	}

	sort.Slice(interfaces, func(i, j int) bool { return interfaces[i].Name < interfaces[j].Name })
	return interfaces
}

func isLoopback(flags []string) bool {
	for _, flag := range flags {
		if strings.EqualFold(flag, "loopback") {
			return true
		}
	}
	return false
}
