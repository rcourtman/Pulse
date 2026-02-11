package hostmetrics

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	"github.com/rs/zerolog/log"
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
var zpoolLookPath = exec.LookPath
var zpoolStat = os.Stat

func summarizeZFSPools(ctx context.Context, datasets []zfsDatasetUsage) []agentshost.Disk {
	if len(datasets) == 0 {
		log.Debug().Msg("zfs: no datasets to summarize")
		return nil
	}

	pools := uniqueZFSPools(datasets)
	if len(pools) == 0 {
		log.Debug().Msg("zfs: no unique pools found from datasets")
		return nil
	}
	log.Debug().Int("datasetCount", len(datasets)).Strs("pools", pools).Msg("zfs: summarizing pools")

	bestDatasets := bestZFSPoolDatasets(datasets)
	mountpoints := bestZFSMountpoints(datasets)
	for pool, ds := range bestDatasets {
		log.Debug().Str("pool", pool).Str("dataset", ds.Dataset).Str("mount", ds.Mountpoint).Uint64("total", ds.Total).Uint64("used", ds.Used).Msg("zfs: best dataset for pool")
	}

	stats, err := queryZpoolStats(ctx, pools)
	if err == nil && len(stats) > 0 {
		log.Debug().Int("zpoolStatsCount", len(stats)).Msg("zfs: using zpool stats")
		return disksFromZpoolStats(pools, stats, mountpoints, bestDatasets)
	}

	log.Debug().Err(err).Msg("zfs: zpool stats unavailable, using fallback")
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

		ds := bestDatasets[pool]
		log.Debug().Str("pool", pool).Bool("hasZpoolStats", ok).Uint64("zpoolSize", stat.Size).Uint64("zpoolAlloc", stat.Alloc).Uint64("zpoolFree", stat.Free).Uint64("dsTotal", ds.Total).Uint64("dsUsed", ds.Used).Str("mount", mp).Msg("zfs: processing pool")

		if ok && stat.Size > 0 {
			// Compute pool-level usable capacity by combining zpool stats with
			// dataset stats. ZFS statfs on a dataset returns per-dataset Used
			// (missing zvols and other datasets), but its Free reflects real
			// pool-available space. We use the ratio ds.Free/stat.Free to
			// convert the raw zpool Size to usable capacity. This handles
			// RAIDZ (parity overhead), mirrors, and simple pools uniformly,
			// and Used = Total - Free captures all pool consumers including
			// zvols. (issues #1052, mirror-vdev fix)
			totalBytes := stat.Size
			freeBytes := stat.Free
			if ds.Free > 0 && stat.Free > 0 && stat.Free >= ds.Free {
				// Convert raw pool total to usable capacity using the
				// raw-to-usable ratio derived from free space.
				// For mirrors the ratio is ~1 (no overhead).
				// For RAIDZ the ratio is (N-P)/N (parity overhead).
				totalBytes = uint64(float64(stat.Size) * (float64(ds.Free) / float64(stat.Free)))
				freeBytes = ds.Free
				log.Debug().Str("pool", pool).Uint64("usableTotal", totalBytes).Uint64("usableFree", freeBytes).Uint64("zpoolSize", stat.Size).Uint64("zpoolFree", stat.Free).Uint64("dsFree", ds.Free).Msg("zfs: computed usable capacity from free-space ratio")
			} else {
				log.Debug().Str("pool", pool).Uint64("zpoolSize", stat.Size).Uint64("zpoolFree", stat.Free).Uint64("dsFree", ds.Free).Msg("zfs: using raw zpool stats (no usable dataset free)")
			}
			usedBytes := totalBytes - freeBytes
			if freeBytes > totalBytes {
				usedBytes = 0
			}

			usage := clampPercent(calculatePercent(totalBytes, usedBytes))
			log.Debug().Str("pool", pool).Int64("totalBytes", int64(totalBytes)).Int64("usedBytes", int64(usedBytes)).Int64("freeBytes", int64(freeBytes)).Float64("usage", usage).Msg("zfs: emitting disk entry")
			disks = append(disks, agentshost.Disk{
				Device:     pool,
				Mountpoint: mp,
				Filesystem: "zfs",
				Type:       "zfs",
				TotalBytes: int64(totalBytes),
				UsedBytes:  int64(usedBytes),
				FreeBytes:  int64(freeBytes),
				Usage:      usage,
			})
			continue
		}

		if ds.Total > 0 {
			usage := clampPercent(calculatePercent(ds.Total, ds.Used))
			log.Debug().Str("pool", pool).Int64("totalBytes", int64(ds.Total)).Int64("usedBytes", int64(ds.Used)).Float64("usage", usage).Msg("zfs: emitting disk entry from dataset only (no zpool stats)")
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
		} else {
			log.Debug().Str("pool", pool).Msg("zfs: skipping pool with no zpool stats and zero dataset total")
		}
	}

	return disks
}

func fallbackZFSDisks(bestDatasets map[string]zfsDatasetUsage, mountpoints map[string]string) []agentshost.Disk {
	log.Debug().Int("poolCount", len(bestDatasets)).Msg("zfs: fallback disk generation")
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

// findZpool returns the path to the zpool binary by preferring known absolute
// locations first, then falling back to PATH lookup.
func findZpool() (string, error) {
	// Prefer common absolute paths so execution does not depend on PATH order.
	// This is especially important for TrueNAS SCALE where the agent
	// might run with a restricted PATH that doesn't include /usr/sbin
	for _, path := range commonZpoolPaths {
		if _, err := zpoolStat(path); err == nil {
			log.Debug().Str("path", path).Msg("zfs: found zpool at hardcoded path")
			return path, nil
		}
	}

	// Fall back to PATH lookup for non-standard installations.
	path, err := zpoolLookPath("zpool")
	if err != nil {
		log.Debug().Msg("zfs: zpool binary not found in PATH or common locations")
		return "", fmt.Errorf("zpool binary not found in PATH or common locations")
	}
	path = filepath.Clean(path)
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("zpool path is not absolute: %q", path)
	}
	if _, err := zpoolStat(path); err != nil {
		return "", fmt.Errorf("zpool path unavailable: %w", err)
	}

	log.Debug().Str("path", path).Msg("zfs: found zpool via PATH")
	return path, nil
}

func fetchZpoolStats(ctx context.Context, pools []string) (map[string]zpoolStats, error) {
	if len(pools) == 0 {
		return nil, nil
	}
	pools = filterValidZFSPoolNames(pools)
	if len(pools) == 0 {
		return nil, fmt.Errorf("no valid zfs pool names to query")
	}

	path, err := findZpool()
	if err != nil {
		return nil, err
	}

	args := []string{"list", "-Hp", "-o", "name,size,allocated,free"}
	args = append(args, pools...)

	cmd := exec.CommandContext(ctx, path, args...)
	log.Debug().Str("cmd", cmd.String()).Msg("zfs: executing zpool list")
	output, err := cmd.Output()
	if err != nil {
		log.Debug().Err(err).Str("cmd", cmd.String()).Msg("zfs: zpool list failed")
		return nil, err
	}
	log.Debug().Int("outputBytes", len(output)).Msg("zfs: zpool list succeeded")

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
		pool, ok := normalizeZFSPoolName(fields[0])
		if !ok {
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

		stats[pool] = zpoolStats{
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
		pool, ok := normalizeZFSPoolName(ds.Pool)
		if !ok {
			continue
		}
		set[pool] = struct{}{}
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

func filterValidZFSPoolNames(pools []string) []string {
	filtered := make([]string, 0, len(pools))
	seen := make(map[string]struct{}, len(pools))
	for _, pool := range pools {
		normalized, ok := normalizeZFSPoolName(pool)
		if !ok {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		filtered = append(filtered, normalized)
	}
	return filtered
}

func normalizeZFSPoolName(pool string) (string, bool) {
	pool = strings.TrimSpace(pool)
	if pool == "" || len(pool) > 255 {
		return "", false
	}
	if strings.HasPrefix(pool, "-") {
		return "", false
	}

	for _, r := range pool {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '_' || r == '-' || r == '.' || r == ':':
		default:
			return "", false
		}
	}

	return pool, true
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
