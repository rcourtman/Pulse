package hostmetrics

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	"github.com/rcourtman/pulse-go-rewrite/pkg/fsfilters"
	"github.com/rs/zerolog/log"
	gopsutilcommon "github.com/shirou/gopsutil/v4/common"
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
	cpuTimes       = gocpu.TimesWithContext
	loadAvg        = goload.AvgWithContext
	virtualMemory  = gomem.VirtualMemoryWithContext
	diskPartitions = partitionsVisibleToAgent
	diskUsage      = godisk.UsageWithContext
	diskIOCounters = godisk.IOCountersWithContext
	netInterfaces  = gonet.InterfacesWithContext
	netIOCounters  = gonet.IOCountersWithContext
)

// partitionsVisibleToAgent asks gopsutil to enumerate the mount namespace the
// collector actually runs in. gopsutil otherwise prefers /proc/1/mountinfo on
// Linux. A systemd service with PrivateTmp (as installed by Pulse) has its own
// mount namespace, so PID 1 can omit mounts that the collector can access and
// that an operator explicitly selected with --disk-include.
//
// Containerised collectors can deliberately point gopsutil at host procfs.
// Preserve either form of that override rather than replacing it with the
// container process's namespace.
func partitionsVisibleToAgent(ctx context.Context, all bool) ([]godisk.PartitionStat, error) {
	return godisk.PartitionsWithContext(mountEnumerationContext(ctx), all)
}

func mountEnumerationContext(ctx context.Context) context.Context {
	if runtime.GOOS != "linux" || hasConfiguredHostProc(ctx) {
		return ctx
	}
	env := cloneGopsutilEnv(ctx)
	env[gopsutilcommon.HostProcMountinfo] = "/proc/self/mountinfo"
	return context.WithValue(ctx, gopsutilcommon.EnvKey, env)
}

func hasConfiguredHostProc(ctx context.Context) bool {
	if strings.TrimSpace(os.Getenv("HOST_PROC")) != "" || strings.TrimSpace(os.Getenv("HOST_PROC_MOUNTINFO")) != "" {
		return true
	}
	env, _ := ctx.Value(gopsutilcommon.EnvKey).(gopsutilcommon.EnvMap)
	return strings.TrimSpace(env[gopsutilcommon.HostProcEnvKey]) != "" || strings.TrimSpace(env[gopsutilcommon.HostProcMountinfo]) != ""
}

func cloneGopsutilEnv(ctx context.Context) gopsutilcommon.EnvMap {
	cloned := make(gopsutilcommon.EnvMap)
	env, _ := ctx.Value(gopsutilcommon.EnvKey).(gopsutilcommon.EnvMap)
	for key, value := range env {
		cloned[key] = value
	}
	return cloned
}

// diskUsageTimeout bounds a single mountpoint usage syscall. statfs on a
// healthy filesystem answers in microseconds; a hard-mounted network
// filesystem whose server is unreachable blocks it in an uninterruptible
// kernel wait, which would otherwise freeze the whole reporting cycle.
var diskUsageTimeout = 5 * time.Second

// stuckDiskMounts tracks all in-flight usage calls, including calls that have
// not timed out yet. Host and Docker collectors share the same mount probes.
// Entries are inserted and deleted on every healthy probe. A locked, typed map
// avoids sync.Map's per-entry bookkeeping for this short-lived workload.
// Never hold the lock across a syscall or a wait.
var stuckDiskMounts = struct {
	sync.Mutex
	calls map[string]*diskUsageCall
}{calls: make(map[string]*diskUsageCall)}

type diskUsageCall struct {
	done     chan struct{}
	deadline time.Time
	usage    *godisk.UsageStat
	err      error
}

// guardedDiskUsage bounds the wait for a mount probe and leaves at most one
// syscall in flight per mountpoint, even when reporting loops overlap.
func guardedDiskUsage(ctx context.Context, mountpoint string) (*godisk.UsageStat, error) {
	stuckDiskMounts.Lock()
	call, loaded := stuckDiskMounts.calls[mountpoint]
	if !loaded {
		call = &diskUsageCall{
			done:     make(chan struct{}),
			deadline: time.Now().Add(diskUsageTimeout),
		}
		stuckDiskMounts.calls[mountpoint] = call
	}
	stuckDiskMounts.Unlock()
	if !loaded {
		go func() {
			call.usage, call.err = diskUsage(ctx, mountpoint)
			// Publish the result before admitting a new probe.
			close(call.done)
			stuckDiskMounts.Lock()
			delete(stuckDiskMounts.calls, mountpoint)
			stuckDiskMounts.Unlock()
			if time.Now().After(call.deadline) {
				log.Info().Str("mount", mountpoint).Msg("disk: stalled usage call returned, mount re-included")
			}
		}()
	}

	timer := time.NewTimer(time.Until(call.deadline))
	defer timer.Stop()
	select {
	case <-call.done:
		return call.usage, call.err
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer.C:
		if !loaded {
			log.Warn().Str("mount", mountpoint).Dur("timeout", diskUsageTimeout).Msg("disk: usage call did not answer, excluding mount until it does (unresponsive network mount?)")
		}
		return nil, fmt.Errorf("usage call for %s did not answer within %s", mountpoint, diskUsageTimeout)
	}
}

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

// Collector owns the CPU baseline for one reporting loop. Its zero value is
// ready to use. Retain it across reports; do not copy it after first use.
type Collector struct {
	cpu cpuUsageTracker
}

// Collect gathers metrics using this reporting loop's CPU baseline.
func (c *Collector) Collect(ctx context.Context, diskExclude []string) (Snapshot, error) {
	return c.CollectWithDiskFilters(ctx, diskExclude, nil)
}

var defaultCollector Collector

// Collect gathers a point-in-time snapshot of host resource utilisation.
// diskExclude contains user-defined patterns for devices or mount points to
// exclude. It preserves the original API for callers that do not need includes.
// Package-level calls share a baseline; independent reporting loops must use
// their own retained Collector.
func Collect(ctx context.Context, diskExclude []string) (Snapshot, error) {
	return CollectWithDiskFilters(ctx, diskExclude, nil)
}

// CollectWithDiskFilters gathers a snapshot with user-defined disk exclusions
// and includes. Includes opt matching filesystems back in after Pulse's
// automatic pseudo-filesystem filtering. Explicit exclusions always win.
func CollectWithDiskFilters(ctx context.Context, diskExclude, diskInclude []string) (Snapshot, error) {
	return defaultCollector.CollectWithDiskFilters(ctx, diskExclude, diskInclude)
}

// CollectWithDiskFilters gathers metrics with this loop's CPU baseline and
// applies the same disk filtering as the package-level convenience function.
func (c *Collector) CollectWithDiskFilters(ctx context.Context, diskExclude, diskInclude []string) (Snapshot, error) {
	collectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var snapshot Snapshot

	if cpuCount, err := cpuCounts(collectCtx, true); err == nil {
		snapshot.CPUCount = cpuCount
	} else {
		log.Debug().Err(err).Msg("hostmetrics: failed to collect cpu count")
	}

	if cpuUsage, err := c.cpu.collect(collectCtx); err == nil {
		snapshot.CPUUsagePercent = cpuUsage
	} else {
		log.Debug().Err(err).Msg("hostmetrics: failed to collect cpu usage")
	}

	if loadAvg, err := loadAvg(collectCtx); err == nil && loadAvg != nil {
		snapshot.LoadAverage = []float64{loadAvg.Load1, loadAvg.Load5, loadAvg.Load15}
	} else if err != nil {
		log.Debug().Err(err).Msg("hostmetrics: failed to collect load average")
	}

	memStats, err := virtualMemory(collectCtx)
	if err != nil {
		log.Error().Err(err).Msg("failed to collect memory stats")
		return Snapshot{}, fmt.Errorf("memory stats: %w", err)
	}

	freeBytes := memStats.Free
	usedBytes := memStats.Used
	usedPercent := memStats.UsedPercent

	if memStats.Total > 0 && memStats.Available > 0 && memStats.Available <= memStats.Total {
		usedBytes = memStats.Total - memStats.Available
		usedPercent = memoryUsagePercent(usedBytes, memStats.Total)
	}

	// Reclaimable page cache: Available counts the pages the kernel would hand
	// back under pressure on top of truly-free ones, so the gap is buff/cache.
	// Reported separately so the memory bar can show used | cache | free.
	cacheBytes := uint64(0)
	if memStats.Available > memStats.Free {
		cacheBytes = memStats.Available - memStats.Free
	}

	// ZFS ARC memory is reclaimable under pressure but is counted as "used" by
	// both FreeBSD (wired memory) and Linux (not in MemAvailable, openzfs/zfs#10255).
	// Subtract it from Used to reflect actual memory pressure. Refs: #1264/#1051
	if arcSize, err := readARCSize(); err == nil && arcSize > 0 {
		if arcSize < usedBytes {
			usedBytes -= arcSize
		} else {
			usedBytes = 0
		}
		usedPercent = memoryUsagePercent(usedBytes, memStats.Total)
		// Recompute free so used + cache + free still covers the total after
		// the ARC pages move out of used.
		if memStats.Total >= usedBytes+cacheBytes {
			freeBytes = memStats.Total - usedBytes - cacheBytes
		} else if memStats.Total >= usedBytes {
			cacheBytes = memStats.Total - usedBytes
			freeBytes = 0
		}
	}

	swapUsed := int64(0)
	if memStats.SwapTotal > memStats.SwapFree {
		swapUsed = int64(memStats.SwapTotal - memStats.SwapFree)
	}

	snapshot.Memory = agentshost.MemoryMetric{
		TotalBytes: int64(memStats.Total),
		UsedBytes:  int64(usedBytes),
		FreeBytes:  int64(freeBytes),
		CacheBytes: int64(cacheBytes),
		Usage:      usedPercent,
		SwapTotal:  int64(memStats.SwapTotal),
		SwapUsed:   swapUsed,
	}

	snapshot.Disks = collectDisksWithIncludes(collectCtx, diskExclude, diskInclude)
	snapshot.DiskIO = collectDiskIO(collectCtx, diskExclude)
	snapshot.Network = collectNetwork(collectCtx)

	return snapshot, nil
}

func memoryUsagePercent(usedBytes, totalBytes uint64) float64 {
	if totalBytes == 0 {
		return 0
	}
	percent := float64(usedBytes) / float64(totalBytes) * 100.0
	if percent < 0 {
		return 0
	}
	if percent > 100 {
		return 100
	}
	return percent
}

// cpuUsageTracker computes CPU usage as the busy/total delta of the cumulative
// CPU time counters between consecutive collections, so the reported value
// averages over the whole report interval instead of a 1-second spot sample.
// Short spot samples badly overstate CPU on mostly-idle guests and block the
// collect path for a full second (issue #1648).
type cpuUsageTracker struct {
	mu      sync.Mutex
	prev    gocpu.TimesStat
	hasPrev bool
}

// collectCPUUsage preserves the package-level convenience path. Independent
// reporting loops must retain their own Collector instead.
func collectCPUUsage(ctx context.Context) (float64, error) {
	return defaultCollector.cpu.collect(ctx)
}

func (t *cpuUsageTracker) collect(ctx context.Context) (float64, error) {
	times, err := cpuTimes(ctx, false)
	if err != nil || len(times) == 0 {
		return spotCPUUsage(ctx)
	}
	cur := times[0]

	t.mu.Lock()
	prev, hasPrev := t.prev, t.hasPrev
	t.prev, t.hasPrev = cur, true
	t.mu.Unlock()

	if !hasPrev {
		// No baseline yet on the very first collection.
		return spotCPUUsage(ctx)
	}

	prevTotal, prevBusy := cpuBusyTotal(prev)
	curTotal, curBusy := cpuBusyTotal(cur)
	if curBusy < prevBusy || curTotal <= prevTotal {
		// Counters went backwards (reboot, VM migration) or did not advance.
		return spotCPUUsage(ctx)
	}

	return clampPercent((curBusy - prevBusy) / (curTotal - prevTotal) * 100), nil
}

// cpuBusyTotal mirrors gopsutil's getAllBusy: guest time is already folded
// into user/nice by the Linux kernel, so it is subtracted from the total to
// avoid double counting (it is zero on other platforms), and iowait counts
// as idle.
func cpuBusyTotal(t gocpu.TimesStat) (total, busy float64) {
	total = t.Total() - t.Guest - t.GuestNice
	busy = total - t.Idle - t.Iowait
	return total, busy
}

// spotCPUUsage is the legacy blocking 1-second sample, kept as the fallback
// when no cross-interval delta is available.
func spotCPUUsage(ctx context.Context) (float64, error) {
	percentages, err := cpuPercent(ctx, time.Second, false)
	if err != nil {
		return 0, err
	}
	if len(percentages) == 0 {
		return 0, nil
	}
	return clampPercent(percentages[0]), nil
}

func collectDisks(ctx context.Context, diskExclude []string) []agentshost.Disk {
	return collectDisksWithIncludes(ctx, diskExclude, nil)
}

func collectDisksWithIncludes(ctx context.Context, diskExclude, diskInclude []string) []agentshost.Disk {
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

		explicitlyIncluded := fsfilters.MatchesDiskInclude(part.Device, part.Mountpoint, diskInclude)
		isZFS := strings.EqualFold(part.Fstype, "zfs") || strings.EqualFold(part.Fstype, "fuse.zfs")

		// Type- and mountpoint-based skips need no usage data. Deciding them
		// before the usage syscall keeps mounts that would be discarded
		// anyway, hard NFS and CIFS mounts in particular, from stalling the
		// collector inside statfs. ZFS stays after usage collection because
		// its datasets are aggregated before this filter applies.
		if !isZFS && !explicitlyIncluded {
			if shouldSkip, _ := fsfilters.ShouldSkipFilesystem(part.Fstype, part.Mountpoint, 0, 0); shouldSkip {
				continue
			}
		}

		usage, err := guardedDiskUsage(ctx, part.Mountpoint)
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
		automaticallyFiltered, _ := fsfilters.ShouldSkipFilesystem(part.Fstype, part.Mountpoint, usage.Total, usage.Used)
		if automaticallyFiltered && !explicitlyIncluded {
			continue
		}

		// Deduplicate normally visible storage by device + total bytes (issue
		// #953). Synology NAS and similar systems create multiple shared folders
		// that report the same underlying capacity. Automatically filtered
		// filesystems are different: generic sources such as "tmpfs" can name
		// multiple independent mounts with equal capacity, and these entries are
		// present only because the operator explicitly selected each one.
		if !automaticallyFiltered {
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

	zfsDisks := summarizeZFSPools(ctx, zfsDatasets)
	enrichZFSPoolDisksWithDatasets(ctx, zfsDisks, zfsDatasets)
	log.Debug().Int("zfsDatasets", len(zfsDatasets)).Int("zfsDisks", len(zfsDisks)).Int("regularDisks", len(disks)).Msg("disk: collection summary")
	disks = append(disks, zfsDisks...)

	sort.Slice(disks, func(i, j int) bool { return disks[i].Mountpoint < disks[j].Mountpoint })
	return disks
}

func collectNetwork(ctx context.Context) []agentshost.NetworkInterface {
	ifaces, err := netInterfaces(ctx)
	if err != nil {
		log.Debug().Err(err).Msg("network: failed to list interfaces")
		return nil
	}

	ioCounters, err := netIOCounters(ctx, true)
	if err != nil {
		log.Debug().Err(err).Msg("network: failed to get interface io counters")
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
	log.Debug().Int("count", len(interfaces)).Msg("network: collected interfaces")
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
		log.Debug().Err(err).Msg("diskio: failed to read disk io counters")
		return nil
	}

	devices := make([]agentshost.DiskIO, 0, len(counters))
	for name, stats := range counters {
		// Skip partitions - only report whole devices
		if isPartition(name) {
			continue
		}
		// Skip devices whose counters are not host physical disk activity
		// (loop, ram, zram, dm- aggregates, and ZFS zvols). ZFS-backed
		// Proxmox hosts expose one zd<N> device per zvol, so without this a
		// host with a few hundred guests reports a few hundred phantom
		// disks on every collection cycle (issue #1671).
		//
		// Deliberately NOT fsfilters.IsVirtualBlockDevice: that answers
		// "can this report SMART" and would drop vd*/xvd*, which are the
		// real disks of any agent running inside a VM.
		if fsfilters.IsNonPhysicalDiskIODevice(name) {
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
	log.Debug().Int("count", len(devices)).Msg("diskio: collected devices")
	return devices
}

// isPartition returns true if the device name looks like a partition
// e.g., sda1, nvme0n1p1, mmcblk0p1, vda2. Devices whose whole-disk name
// already ends in a digit use a p<partition-number> suffix; treating only
// NVMe that way lets the parent and partitions of MMC, MD, and persistent
// memory devices through and double-counts their kernel I/O counters.
func isPartition(name string) bool {
	// Whole-device names ending in a digit separate their partition number
	// with "p": nvme0n1p1, mmcblk0p1, md0p1, pmem0p1, and zd0p1.
	if idx := strings.LastIndexByte(name, 'p'); idx > 0 && idx < len(name)-1 && name[idx-1] >= '0' && name[idx-1] <= '9' {
		partitionNumber := name[idx+1:]
		allDigits := true
		for i := 0; i < len(partitionNumber); i++ {
			if partitionNumber[i] < '0' || partitionNumber[i] > '9' {
				allDigits = false
				break
			}
		}
		if allDigits {
			return true
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
	return false
}
