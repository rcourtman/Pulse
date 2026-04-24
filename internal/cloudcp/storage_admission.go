package cloudcp

import (
	"context"
	"fmt"
	"math"
	"strings"

	cpDocker "github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/docker"
	"golang.org/x/sys/unix"
)

type StorageDockerUsageProvider interface {
	DiskUsage(ctx context.Context) (*cpDocker.DiskUsageSnapshot, error)
}

type StorageFilesystemReport struct {
	Name              string
	Path              string
	AvailableBytes    int64
	TotalBytes        int64
	UsedBytes         int64
	MinAvailableBytes int64
	OK                bool
	Error             string
}

type StorageBuildCacheReport struct {
	TotalBytes       int64
	ReclaimableBytes int64
	ActiveCount      int64
	TotalCount       int64
	MaxBytes         int64
	OK               bool
	Error            string
}

type StorageGuardrailReport struct {
	Enabled     bool
	OK          bool
	Filesystems []StorageFilesystemReport
	BuildCache  StorageBuildCacheReport
	Failures    []string
}

func EnforceStorageAdmission(ctx context.Context, cfg *CPConfig, dockerUsage StorageDockerUsageProvider) error {
	report, err := CheckStorageGuardrails(ctx, cfg, dockerUsage)
	if err != nil {
		return err
	}
	if report.OK {
		return nil
	}
	return fmt.Errorf("storage admission failed: %s", strings.Join(report.Failures, "; "))
}

func CheckStorageGuardrails(ctx context.Context, cfg *CPConfig, dockerUsage StorageDockerUsageProvider) (*StorageGuardrailReport, error) {
	if cfg == nil {
		return nil, fmt.Errorf("control plane config is required")
	}
	report := &StorageGuardrailReport{
		Enabled: cfg.StorageGuardrailsEnabled,
		OK:      true,
	}
	if !cfg.StorageGuardrailsEnabled {
		return report, nil
	}

	for _, check := range []struct {
		name string
		path string
		min  int64
	}{
		{name: "root", path: cfg.StorageRootPath, min: cfg.StorageMinRootAvailableBytes},
		{name: "data", path: cfg.StorageDataPath, min: cfg.StorageMinDataAvailableBytes},
		{name: "docker", path: cfg.StorageDockerPath, min: cfg.StorageMinDockerAvailableBytes},
	} {
		fsReport := checkFilesystemStorage(check.name, check.path, check.min)
		if !fsReport.OK {
			report.OK = false
			if fsReport.Error != "" {
				report.Failures = append(report.Failures, fmt.Sprintf("%s path %s unavailable: %s", check.name, check.path, fsReport.Error))
			} else {
				report.Failures = append(report.Failures, fmt.Sprintf("%s path %s has %d bytes available, below %d byte threshold", check.name, check.path, fsReport.AvailableBytes, fsReport.MinAvailableBytes))
			}
		}
		report.Filesystems = append(report.Filesystems, fsReport)
	}

	buildCache := StorageBuildCacheReport{
		MaxBytes: cfg.StorageMaxDockerBuildCacheBytes,
		OK:       true,
	}
	if dockerUsage == nil {
		buildCache.OK = false
		buildCache.Error = "docker usage provider unavailable"
	} else {
		usage, err := dockerUsage.DiskUsage(ctx)
		if err != nil {
			buildCache.OK = false
			buildCache.Error = err.Error()
		} else if usage != nil {
			buildCache.TotalBytes = usage.BuildCache.TotalSize
			buildCache.ReclaimableBytes = usage.BuildCache.Reclaimable
			buildCache.ActiveCount = usage.BuildCache.ActiveCount
			buildCache.TotalCount = usage.BuildCache.TotalCount
			if buildCache.TotalBytes > buildCache.MaxBytes {
				buildCache.OK = false
			}
		}
	}
	if !buildCache.OK {
		report.OK = false
		if buildCache.Error != "" {
			report.Failures = append(report.Failures, "docker build cache unavailable: "+buildCache.Error)
		} else {
			report.Failures = append(report.Failures, fmt.Sprintf("docker build cache is %d bytes, above %d byte threshold", buildCache.TotalBytes, buildCache.MaxBytes))
		}
	}
	report.BuildCache = buildCache
	return report, nil
}

func checkFilesystemStorage(name, path string, minAvailableBytes int64) StorageFilesystemReport {
	report := StorageFilesystemReport{
		Name:              strings.TrimSpace(name),
		Path:              strings.TrimSpace(path),
		MinAvailableBytes: minAvailableBytes,
		OK:                true,
	}
	if report.Path == "" {
		report.OK = false
		report.Error = "path is required"
		return report
	}
	total, available, err := statFilesystemBytes(report.Path)
	if err != nil {
		report.OK = false
		report.Error = err.Error()
		return report
	}
	report.TotalBytes = total
	report.AvailableBytes = available
	if total >= available {
		report.UsedBytes = total - available
	}
	if available < minAvailableBytes {
		report.OK = false
	}
	return report
}

func statFilesystemBytes(path string) (totalBytes, availableBytes int64, err error) {
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		return 0, 0, err
	}
	if stat.Bsize <= 0 {
		return 0, 0, fmt.Errorf("filesystem block size is zero")
	}
	blockSize := uint64(stat.Bsize)
	return clampUint64ToInt64(uint64(stat.Blocks) * blockSize), clampUint64ToInt64(uint64(stat.Bavail) * blockSize), nil
}

func clampUint64ToInt64(value uint64) int64 {
	if value > math.MaxInt64 {
		return math.MaxInt64
	}
	return int64(value)
}
