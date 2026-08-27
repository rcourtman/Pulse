//go:build linux

package hostagent

import (
	"fmt"
	"syscall"
)

// FilesystemUsage satisfies filesystemUsageProber on Linux. Statfs through a
// /proc/<pid>/root path traverses into that process's mount namespace, so the
// numbers match what df reports inside the container.
func (c *defaultCollector) FilesystemUsage(path string) (hostFilesystemUsage, error) {
	var fs syscall.Statfs_t
	if err := syscall.Statfs(path, &fs); err != nil {
		return hostFilesystemUsage{}, fmt.Errorf("statfs %s: %w", path, err)
	}
	var st syscall.Stat_t
	if err := syscall.Stat(path, &st); err != nil {
		return hostFilesystemUsage{}, fmt.Errorf("stat %s: %w", path, err)
	}
	blockSize := int64(fs.Frsize)
	if blockSize <= 0 {
		blockSize = int64(fs.Bsize)
	}
	if blockSize <= 0 {
		return hostFilesystemUsage{}, fmt.Errorf("statfs %s: invalid block size", path)
	}
	total := int64(fs.Blocks) * blockSize
	free := int64(fs.Bfree) * blockSize
	used := total - free
	if used < 0 {
		used = 0
	}
	return hostFilesystemUsage{
		TotalBytes: total,
		UsedBytes:  used,
		AvailBytes: int64(fs.Bavail) * blockSize,
		Device:     uint64(st.Dev),
	}, nil
}
