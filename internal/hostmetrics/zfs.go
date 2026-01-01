package hostmetrics

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

// zpoolStats represents capacity data reported by `zpool list`.
type zpoolStats struct {
	Size  uint64
	Alloc uint64
	Free  uint64
}

// zfsDatasetUsage preserves per-dataset usage so we can reconcile pools later.
type zfsDatasetUsage struct {
	Pool       string
	Dataset    string
	Mountpoint string
	Total      uint64
	Used       uint64
	Free       uint64
}

var queryZpoolStats = fetchZpoolStats

func summarizeZFSPools(ctx context.Context, datasets []zfsDatasetUsage) []agentshost.Disk {
	if len(datasets) == 0 {
		return nil
	}

	pools := uniqueZFSPools(datasets)
	if len(pools) == 0 {
		return nil
	}

	bestDatasets := bestZFSPoolDatasets(datasets)
	mountpoints := bestZFSMountpoints(datasets)
	stats, err := queryZpoolStats(ctx, pools)
	if err == nil && len(stats) > 0 {
		return disksFromZpoolStats(pools, stats, mountpoints, bestDatasets)
	}

	return fallbackZFSDisks(bestDatasets, mountpoints)
}

func disksFromZpoolStats(
	pools []string,
	stats map[string]zpoolStats,
	mountpoints map[string]string,
	bestDatasets map[string]zfsDatasetUsage,
) []agentshost.Disk {
	disks := make([]agentshost.Disk, 0, len(pools))

	for _, pool := range pools {
		stat, ok := stats[pool]
		mp := mountpoints[pool]
		if mp == "" {
			mp = fmt.Sprintf("zpool:%s", pool)
		}

		if ok && stat.Size > 0 {
			usage := clampPercent(calculatePercent(stat.Size, stat.Alloc))
			disks = append(disks, agentshost.Disk{
				Device:     pool,
				Mountpoint: mp,
				Filesystem: "zfs",
				Type:       "zfs",
				TotalBytes: int64(stat.Size),
				UsedBytes:  int64(stat.Alloc),
				FreeBytes:  int64(stat.Free),
				Usage:      usage,
			})
			continue
		}

		if ds, ok := bestDatasets[pool]; ok && ds.Total > 0 {
			usage := clampPercent(calculatePercent(ds.Total, ds.Used))
			disks = append(disks, agentshost.Disk{
				Device:     pool,
				Mountpoint: mp,
				Filesystem: "zfs",
				Type:       "zfs",
				TotalBytes: int64(ds.Total),
				UsedBytes:  int64(ds.Used),
				FreeBytes:  int64(ds.Free),
				Usage:      usage,
			})
		}
	}

	return disks
}

func fallbackZFSDisks(bestDatasets map[string]zfsDatasetUsage, mountpoints map[string]string) []agentshost.Disk {
	if len(bestDatasets) == 0 {
		return nil
	}

	pools := make([]string, 0, len(bestDatasets))
	for pool := range bestDatasets {
		pools = append(pools, pool)
	}
	sort.Strings(pools)

	disks := make([]agentshost.Disk, 0, len(pools))
	for _, pool := range pools {
		ds := bestDatasets[pool]
		if ds.Total == 0 {
			continue
		}

		mp := mountpoints[pool]
		if mp == "" {
			mp = fmt.Sprintf("zpool:%s", pool)
		}

		usage := clampPercent(calculatePercent(ds.Total, ds.Used))
		disks = append(disks, agentshost.Disk{
			Device:     pool,
			Mountpoint: mp,
			Filesystem: "zfs",
			Type:       "zfs",
			TotalBytes: int64(ds.Total),
			UsedBytes:  int64(ds.Used),
			FreeBytes:  int64(ds.Free),
			Usage:      usage,
		})
	}

	return disks
}

// commonZpoolPaths lists common locations for the zpool binary.
// TrueNAS SCALE, FreeBSD, and various Linux distributions may install
// zpool in different locations that might not be in the agent's PATH.
// This helps fix issue #718 where TrueNAS reports inflated storage.
var commonZpoolPaths = []string{
	"/usr/sbin/zpool",       // TrueNAS SCALE, Debian, Ubuntu
	"/sbin/zpool",           // FreeBSD, older Linux
	"/usr/local/sbin/zpool", // FreeBSD ports, custom builds
	"/usr/local/bin/zpool",  // Custom installations
	"/opt/zfs/bin/zpool",    // Some enterprise Linux
	"/usr/bin/zpool",        // Some distributions
}

// findZpool returns the path to the zpool binary by first trying exec.LookPath,
// then falling back to common hardcoded paths for TrueNAS/FreeBSD/Linux systems.
func findZpool() (string, error) {
	// First, try the standard PATH lookup
	if path, err := exec.LookPath("zpool"); err == nil {
		return path, nil
	}

	// If that fails, try common absolute paths
	// This is especially important for TrueNAS SCALE where the agent
	// might run with a restricted PATH that doesn't include /usr/sbin
	for _, path := range commonZpoolPaths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("zpool binary not found in PATH or common locations")
}

func fetchZpoolStats(ctx context.Context, pools []string) (map[string]zpoolStats, error) {
	if len(pools) == 0 {
		return nil, nil
	}

	path, err := findZpool()
	if err != nil {
		return nil, err
	}

	args := []string{"list", "-Hp", "-o", "name,size,allocated,free"}
	args = append(args, pools...)

	cmd := exec.CommandContext(ctx, path, args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	return parseZpoolList(output)
}

func parseZpoolList(output []byte) (map[string]zpoolStats, error) {
	stats := make(map[string]zpoolStats)
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		fields := strings.Split(line, "\t")
		if len(fields) < 4 {
			continue
		}

		size, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			continue
		}
		alloc, err := strconv.ParseUint(fields[2], 10, 64)
		if err != nil {
			continue
		}
		free, err := strconv.ParseUint(fields[3], 10, 64)
		if err != nil {
			continue
		}

		stats[fields[0]] = zpoolStats{
			Size:  size,
			Alloc: alloc,
			Free:  free,
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(stats) == 0 {
		return nil, fmt.Errorf("zpool list returned no usable data")
	}
	return stats, nil
}

func uniqueZFSPools(datasets []zfsDatasetUsage) []string {
	set := make(map[string]struct{}, len(datasets))
	for _, ds := range datasets {
		if ds.Pool != "" {
			set[ds.Pool] = struct{}{}
		}
	}
	if len(set) == 0 {
		return nil
	}

	pools := make([]string, 0, len(set))
	for pool := range set {
		pools = append(pools, pool)
	}
	sort.Strings(pools)
	return pools
}

func bestZFSMountpoints(datasets []zfsDatasetUsage) map[string]string {
	mounts := make(map[string]string, len(datasets))
	scores := make(map[string]int, len(datasets))

	for _, ds := range datasets {
		if ds.Pool == "" || ds.Mountpoint == "" {
			continue
		}

		score := zfsMountpointScore(ds)
		if current, ok := scores[ds.Pool]; ok && score >= current {
			continue
		}
		scores[ds.Pool] = score
		mounts[ds.Pool] = ds.Mountpoint
	}

	return mounts
}

func zfsMountpointScore(ds zfsDatasetUsage) int {
	if ds.Dataset != "" && !strings.Contains(ds.Dataset, "/") {
		return 0
	}
	path := strings.Trim(ds.Mountpoint, "/")
	if path == "" {
		return 1
	}
	return 1 + strings.Count(path, "/")
}

func zfsPoolFromDevice(device string) string {
	device = strings.TrimSpace(device)
	if device == "" {
		return ""
	}
	if idx := strings.Index(device, "/"); idx >= 0 {
		return device[:idx]
	}
	return device
}

func calculatePercent(total, used uint64) float64 {
	if total == 0 {
		return 0
	}
	return (float64(used) / float64(total)) * 100
}

func clampPercent(value float64) float64 {
	switch {
	case value < 0:
		return 0
	case value > 100:
		return 100
	default:
		return value
	}
}

func bestZFSPoolDatasets(datasets []zfsDatasetUsage) map[string]zfsDatasetUsage {
	best := make(map[string]zfsDatasetUsage)
	for _, ds := range datasets {
		if ds.Pool == "" {
			continue
		}
		if current, ok := best[ds.Pool]; !ok || ds.Total > current.Total {
			best[ds.Pool] = ds
		}
	}
	return best
}
