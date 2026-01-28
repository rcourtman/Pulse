package hostmetrics

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	"github.com/rcourtman/pulse-go-rewrite/pkg/fsfilters"
	"github.com/rs/zerolog/log"
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
	diskIOCounters = godisk.IOCountersWithContext
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
	DiskIO          []agentshost.DiskIO
	Network         []agentshost.NetworkInterface
}

// Collect gathers a point-in-time snapshot of host resource utilisation.
// diskExclude contains user-defined patterns for mount points to exclude.
func Collect(ctx context.Context, diskExclude []string) (Snapshot, error) {
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

	snapshot.Disks = collectDisks(collectCtx, diskExclude)
	snapshot.DiskIO = collectDiskIO(collectCtx, diskExclude)
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

func collectDisks(ctx context.Context, diskExclude []string) []agentshost.Disk {
	partitions, err := diskPartitions(ctx, true)
	if err != nil {
		log.Debug().Err(err).Msg("disk: failed to list partitions")
		return nil
	}
	log.Debug().Int("count", len(partitions)).Msg("disk: discovered partitions")

	disks := make([]agentshost.Disk, 0, len(partitions))
	seen := make(map[string]struct{}, len(partitions))
	zfsDatasets := make([]zfsDatasetUsage, 0)

	// Track device+total combinations to deduplicate shared folders (Synology, BTRFS bind mounts).
	// Key: "device:total_bytes", Value: mountpoint we already recorded.
	// This prevents counting the same underlying volume multiple times. Related to #953.
	deviceTotals := make(map[string]string, len(partitions))

	for _, part := range partitions {
		if part.Mountpoint == "" {
			continue
		}
		if _, ok := seen[part.Mountpoint]; ok {
			continue
		}
		seen[part.Mountpoint] = struct{}{}

		// Check user-defined exclusions first (issue #896, #1142)
		// Check both device path and mountpoint to support patterns like "/dev/sda" or "/mnt/backup"
		if fsfilters.MatchesDiskExclude(part.Device, part.Mountpoint, diskExclude) {
			continue
		}

		usage, err := diskUsage(ctx, part.Mountpoint)
		if err != nil {
			log.Debug().Err(err).Str("mount", part.Mountpoint).Str("device", part.Device).Str("fstype", part.Fstype).Msg("disk: failed to get usage")
			continue
		}
		if usage.Total == 0 {
			log.Debug().Str("mount", part.Mountpoint).Str("device", part.Device).Str("fstype", part.Fstype).Msg("disk: skipping partition with zero total")
			continue
		}

		if strings.EqualFold(part.Fstype, "zfs") || strings.EqualFold(part.Fstype, "fuse.zfs") {
			pool := zfsPoolFromDevice(part.Device)
			if pool == "" {
				log.Debug().Str("device", part.Device).Str("mount", part.Mountpoint).Msg("disk: zfs partition with empty pool name, skipping")
				continue
			}
			if fsfilters.ShouldIgnoreReadOnlyFilesystem(part.Fstype, usage.Total, usage.Used) {
				log.Debug().Str("pool", pool).Str("mount", part.Mountpoint).Msg("disk: zfs read-only filesystem, skipping")
				continue
			}

			log.Debug().Str("pool", pool).Str("dataset", part.Device).Str("mount", part.Mountpoint).Uint64("total", usage.Total).Uint64("used", usage.Used).Msg("disk: collected zfs dataset")
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

		// Deduplicate by device + total bytes (issue #953).
		// Synology NAS and similar systems create multiple "shared folders" as bind mounts
		// or BTRFS subvolumes that all report the same device and total capacity.
		// Only count each unique device+total combination once.
		deviceKey := fmt.Sprintf("%s:%d", part.Device, usage.Total)
		if existingMount, exists := deviceTotals[deviceKey]; exists {
			// Prefer shorter/shallower mountpoints (e.g., /volume1 over /volume1/docker)
			if len(part.Mountpoint) >= len(existingMount) {
				continue
			}
			// This mountpoint is shallower - remove the old entry and use this one
			for i := len(disks) - 1; i >= 0; i-- {
				if disks[i].Mountpoint == existingMount {
					disks = append(disks[:i], disks[i+1:]...)
					break
				}
			}
		}
		deviceTotals[deviceKey] = part.Mountpoint

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

	zfsDisks := summarizeZFSPools(ctx, zfsDatasets)
	log.Debug().Int("zfsDatasets", len(zfsDatasets)).Int("zfsDisks", len(zfsDisks)).Int("regularDisks", len(disks)).Msg("disk: collection summary")
	disks = append(disks, zfsDisks...)

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

// collectDiskIO gathers I/O statistics for physical block devices.
// Only reports whole disks (nvme0n1, sda), not partitions (nvme0n1p1, sda1).
// Respects user-defined disk exclusions to avoid reporting excluded devices.
func collectDiskIO(ctx context.Context, diskExclude []string) []agentshost.DiskIO {
	counters, err := diskIOCounters(ctx)
	if err != nil {
		return nil
	}

	devices := make([]agentshost.DiskIO, 0, len(counters))
	for name, stats := range counters {
		// Skip partitions - only report whole devices
		if isPartition(name) {
			continue
		}
		// Skip loop devices and ram disks
		if strings.HasPrefix(name, "loop") || strings.HasPrefix(name, "ram") {
			continue
		}
		// Skip device-mapper and md devices (report at physical level)
		if strings.HasPrefix(name, "dm-") {
			continue
		}
		// Skip user-excluded devices (issue #1142)
		if fsfilters.MatchesDeviceExclude(name, diskExclude) {
			continue
		}

		devices = append(devices, agentshost.DiskIO{
			Device:     name,
			ReadBytes:  stats.ReadBytes,
			WriteBytes: stats.WriteBytes,
			ReadOps:    stats.ReadCount,
			WriteOps:   stats.WriteCount,
			ReadTime:   stats.ReadTime,
			WriteTime:  stats.WriteTime,
			IOTime:     stats.IoTime,
		})
	}

	sort.Slice(devices, func(i, j int) bool { return devices[i].Device < devices[j].Device })
	return devices
}

// isPartition returns true if the device name looks like a partition
// e.g., sda1, nvme0n1p1, vda2
func isPartition(name string) bool {
	// NVMe partitions: nvme0n1p1, nvme0n1p2
	if strings.Contains(name, "n") && strings.Contains(name, "p") {
		// Check if it ends with pN where N is a digit
		idx := strings.LastIndex(name, "p")
		if idx > 0 && idx < len(name)-1 {
			rest := name[idx+1:]
			if len(rest) > 0 && rest[0] >= '0' && rest[0] <= '9' {
				return true
			}
		}
	}
	// Traditional partitions: sda1, vda2, hda1
	if len(name) > 2 {
		last := name[len(name)-1]
		if last >= '0' && last <= '9' {
			// Check if second-to-last is a letter (sda1) or also a digit (sda10)
			secondLast := name[len(name)-2]
			if (secondLast >= 'a' && secondLast <= 'z') || (secondLast >= '0' && secondLast <= '9') {
				// Exclude things like "md0" (whole device) - check for common prefixes
				if strings.HasPrefix(name, "sd") || strings.HasPrefix(name, "vd") ||
					strings.HasPrefix(name, "hd") || strings.HasPrefix(name, "xvd") {
					return true
				}
			}
		}
	}
	// ZFS devices: zd0p1, zd16p1
	if strings.HasPrefix(name, "zd") && strings.Contains(name, "p") {
		return true
	}
	return false
}
