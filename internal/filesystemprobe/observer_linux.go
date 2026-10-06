//go:build linux

package filesystemprobe

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path"
	"strconv"
	"strings"
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
	if err := verifyIdentity(proc, req); err != nil {
		return nil, err
	}
	root, err := unix.Openat(proc, "root", unix.O_PATH|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("open container filesystem namespace: %w", err)
	}
	defer unix.Close(root)
	if err := verifyIdentity(proc, req); err != nil {
		return nil, err
	}
	observations := make([]filesystem.Observation, 0, len(req.Mountpoints))
	for _, mount := range req.Mountpoints {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		observation := filesystem.Observation{Mountpoint: mount, Source: Source}
		if req.MountsOnly {
			if mounted, mountErr := mountedWithinRoot(root, mount); mountErr == nil && !mounted {
				observation.ObservedAt = time.Now().UTC()
				observation.Error = ErrNotMounted.Error()
				observations = append(observations, observation)
				continue
			}
		}
		observation.Type, observation.Usage, err = observePath(root, mount)
		observation.ObservedAt = time.Now().UTC()
		if err != nil {
			observation.Error = err.Error()
		}
		observations = append(observations, observation)
	}
	if err := verifyIdentity(proc, req); err != nil {
		return nil, err
	}
	return observations, nil
}

func verifyIdentity(proc int, req ContainerRequest) error {
	if err := verifyCgroup(proc, req); err != nil {
		return err
	}
	if req.Runtime == "lxc" {
		return verifyNamespaceInit(proc, req.PID)
	}
	return nil
}

// verifyNamespaceInit requires the pinned process to be the guest's init, so a
// process of a namespace nested inside the guest cannot supply its own root.
func verifyNamespaceInit(proc, pid int) error {
	fd, err := unix.Openat(proc, "status", unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return fmt.Errorf("read container process status: %w", err)
	}
	file := os.NewFile(uintptr(fd), "status")
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, 65537))
	if err != nil || len(raw) > 65536 {
		return errors.New("container process status is unavailable")
	}
	if !IsNamespaceInit(string(raw), pid) {
		return errors.New("process is not the container's init")
	}
	return nil
}

// mountedWithinRoot reports whether mount is a different filesystem from its
// parent directory, both resolved within the resource root.
func mountedWithinRoot(root int, mount string) (bool, error) {
	if mount == "/" {
		return true, nil
	}
	device := func(target string) (uint64, error) {
		fd, err := openInRoot(root, target)
		if err != nil {
			return 0, err
		}
		defer unix.Close(fd)
		var stat unix.Stat_t
		if err := unix.Fstat(fd, &stat); err != nil {
			return 0, err
		}
		return uint64(stat.Dev), nil
	}
	own, err := device(mount)
	if err != nil {
		return false, err
	}
	parent, err := device(path.Dir(mount))
	if err != nil {
		return false, err
	}
	return own != parent, nil
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

// openInRoot opens mount beneath root without ever leaving it. With openat2,
// absolute symlinks stay within the resource root and magic links cannot
// escape it. systemd answers openat2 with ENOSYS under RestrictSUIDSGID,
// because seccomp cannot inspect its flags, and the typed helper sets that, so
// there the clean path is walked one component at a time and any symbolic link
// is refused rather than resolved. Neither path falls back to unconfined
// resolution.
func openInRoot(root int, mount string) (int, error) {
	fd, err := unix.Openat2(root, mount, &unix.OpenHow{
		Flags:   unix.O_PATH | unix.O_CLOEXEC,
		Resolve: unix.RESOLVE_IN_ROOT | unix.RESOLVE_NO_MAGICLINKS,
	})
	if !errors.Is(err, unix.ENOSYS) {
		return fd, err
	}
	return openBeneathWithoutSymlinks(root, mount)
}

func openBeneathWithoutSymlinks(root int, mount string) (int, error) {
	if !path.IsAbs(mount) || path.Clean(mount) != mount {
		return -1, errors.New("mount path must be clean and absolute")
	}
	current, err := unix.Dup(root)
	if err != nil {
		return -1, err
	}
	for _, component := range strings.Split(strings.TrimPrefix(mount, "/"), "/") {
		if component == "" {
			continue
		}
		next, err := unix.Openat(current, component, unix.O_PATH|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		unix.Close(current)
		if err != nil {
			return -1, err
		}
		var stat unix.Stat_t
		if err := unix.Fstat(next, &stat); err != nil {
			unix.Close(next)
			return -1, err
		}
		if stat.Mode&unix.S_IFMT == unix.S_IFLNK {
			unix.Close(next)
			return -1, errors.New("mount path crosses a symbolic link")
		}
		current = next
	}
	return current, nil
}

func observePath(root int, mount string) (string, *filesystem.Usage, error) {
	fd, err := openInRoot(root, mount)
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
