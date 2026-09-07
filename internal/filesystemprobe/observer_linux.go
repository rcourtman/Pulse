//go:build linux

package filesystemprobe

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/agents/filesystem"
	"golang.org/x/sys/unix"
)

func observeContainer(ctx context.Context, req ContainerRequest) ([]filesystem.Observation, error) {
	// Hold this proc directory and root descriptor throughout the observation.
	// Never re-resolve /proc/PID after the identity check, where PID reuse could
	// otherwise change the process whose mount namespace is observed.
	proc, err := unix.Open("/proc/"+strconv.Itoa(req.PID), unix.O_PATH|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("open container process: %w", err)
	}
	defer unix.Close(proc)
	if err := verifyCgroup(proc, req); err != nil {
		return nil, err
	}
	root, err := unix.Openat(proc, "root", unix.O_PATH|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("open container filesystem namespace: %w", err)
	}
	defer unix.Close(root)
	if err := verifyCgroup(proc, req); err != nil {
		return nil, err
	}
	observations := make([]filesystem.Observation, 0, len(req.Mountpoints))
	for _, mount := range req.Mountpoints {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		observation := filesystem.Observation{Mountpoint: mount, Source: Source}
		observation.Type, observation.Usage, err = observePath(root, mount)
		observation.ObservedAt = time.Now().UTC()
		if err != nil {
			observation.Error = err.Error()
		}
		observations = append(observations, observation)
	}
	if err := verifyCgroup(proc, req); err != nil {
		return nil, err
	}
	return observations, nil
}

func verifyCgroup(proc int, req ContainerRequest) error {
	fd, err := unix.Openat(proc, "cgroup", unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return fmt.Errorf("read container process identity: %w", err)
	}
	file := os.NewFile(uintptr(fd), "cgroup")
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, 65537))
	if err != nil || len(raw) > 65536 {
		return errors.New("container process identity is unavailable")
	}
	if !cgroupMatches(string(raw), req) {
		return errors.New("process cgroup does not attest the exact container identity")
	}
	return nil
}

func observePath(root int, mount string) (string, *filesystem.Usage, error) {
	// Absolute symlinks stay within the resource root. Magic links cannot escape
	// it. Unsupported kernels fail rather than falling back to unsafe resolution.
	fd, err := unix.Openat2(root, mount, &unix.OpenHow{
		Flags:   unix.O_PATH | unix.O_CLOEXEC,
		Resolve: unix.RESOLVE_IN_ROOT | unix.RESOLVE_NO_MAGICLINKS,
	})
	if err != nil {
		return "", nil, fmt.Errorf("open mount within resource namespace: %w", err)
	}
	defer unix.Close(fd)
	var stat unix.Statfs_t
	if err := unix.Fstatfs(fd, &stat); err != nil {
		return "", nil, fmt.Errorf("read filesystem counters: %w", err)
	}
	usage, err := filesystemUsage(stat)
	if err != nil {
		return "", nil, err
	}
	return filesystemType(int64(stat.Type)), usage, nil
}

func filesystemUsage(stat unix.Statfs_t) (*filesystem.Usage, error) {
	blockSize := stat.Frsize
	if blockSize <= 0 {
		blockSize = stat.Bsize
	}
	if blockSize <= 0 || stat.Blocks == 0 || stat.Blocks > math.MaxUint64/uint64(blockSize) || stat.Bfree > stat.Blocks || stat.Bavail > stat.Bfree {
		return nil, errors.New("filesystem counters are unavailable or inconsistent")
	}
	usage := &filesystem.Usage{
		CapacityBytes:  stat.Blocks * uint64(blockSize),
		FreeBytes:      stat.Bfree * uint64(blockSize),
		AvailableBytes: stat.Bavail * uint64(blockSize),
	}
	if stat.Files > 0 && stat.Ffree <= stat.Files {
		usage.Inodes = &filesystem.InodeUsage{Capacity: stat.Files, Free: stat.Ffree}
	}
	return usage, nil
}

func filesystemType(kind int64) string {
	switch kind {
	case unix.TMPFS_MAGIC:
		return "tmpfs"
	case unix.OVERLAYFS_SUPER_MAGIC:
		return "overlay"
	case unix.EXT4_SUPER_MAGIC:
		return "ext"
	case unix.XFS_SUPER_MAGIC:
		return "xfs"
	case unix.BTRFS_SUPER_MAGIC:
		return "btrfs"
	case unix.NFS_SUPER_MAGIC:
		return "nfs"
	default:
		return fmt.Sprintf("0x%x", uint64(kind))
	}
}
