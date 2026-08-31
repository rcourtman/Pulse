//go:build unix

package agenthelper

import (
	"errors"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

func openFileNoFollow(path string) (*os.File, error) {
	// Quarantine files are collector-owned. O_NONBLOCK ensures a FIFO or
	// device swap cannot wedge the privileged helper before descriptor metadata
	// rejects the object as non-regular. The returned descriptor is the sole
	// authority for all subsequent type, ownership, mode, and size checks.
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), path), nil
}

func StrictRootOwnedFile(file *os.File) error {
	return FileOwnedByUID(0)(file)
}

func FileOwnedByUID(uid uint32) func(*os.File) error {
	return func(file *os.File) error {
		info, err := file.Stat()
		if err != nil {
			return err
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok || stat.Uid != uid {
			return errors.New("file ownership does not match the required UID")
		}
		return nil
	}
}
