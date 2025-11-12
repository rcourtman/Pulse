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

	if cpuCount, err := gocpu.CountsWithContext(collectCtx, true); err == nil {
		snapshot.CPUCount = cpuCount
	}

	if cpuUsage, err := collectCPUUsage(collectCtx); err == nil {
		snapshot.CPUUsagePercent = cpuUsage
	}

	if loadAvg, err := goload.AvgWithContext(collectCtx); err == nil && loadAvg != nil {
		snapshot.LoadAverage = []float64{loadAvg.Load1, loadAvg.Load5, loadAvg.Load15}
	}

	memStats, err := gomem.VirtualMemoryWithContext(collectCtx)
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
	percentages, err := gocpu.PercentWithContext(ctx, time.Second, false)
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
	partitions, err := godisk.PartitionsWithContext(ctx, true)
	if err != nil {
		return nil
	}

	disks := make([]agentshost.Disk, 0, len(partitions))
	seen := make(map[string]struct{}, len(partitions))

	for _, part := range partitions {
		if part.Mountpoint == "" {
			continue
		}
		if _, ok := seen[part.Mountpoint]; ok {
			continue
		}
		seen[part.Mountpoint] = struct{}{}

		usage, err := godisk.UsageWithContext(ctx, part.Mountpoint)
		if err != nil {
			continue
		}
		if usage.Total == 0 {
			continue
		}

		// Skip read-only filesystems like squashfs (snap mounts), erofs, iso9660, etc.
		// These are immutable and always report near-full usage, which causes false alerts.
		// See issues #505 (Home Assistant OS) and #690 (Ubuntu snap mounts).
		if fsfilters.ShouldIgnoreReadOnlyFilesystem(part.Fstype, usage.Total, usage.Used) {
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

	sort.Slice(disks, func(i, j int) bool { return disks[i].Mountpoint < disks[j].Mountpoint })
	return disks
}

func collectNetwork(ctx context.Context) []agentshost.NetworkInterface {
	ifaces, err := gonet.InterfacesWithContext(ctx)
	if err != nil {
		return nil
	}

	ioCounters, err := gonet.IOCountersWithContext(ctx, true)
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
