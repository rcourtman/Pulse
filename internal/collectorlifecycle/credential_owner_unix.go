//go:build !windows

package collectorlifecycle

import (
	"errors"
	"fmt"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

func openCredentialFile(path string) (*os.File, error) {
	// O_NONBLOCK prevents an attacker-controlled FIFO from hanging the root
	// installer before descriptor metadata can reject the non-regular object.
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), path), nil
}

func validateCredentialFileOwner(path string, info os.FileInfo, tokenOwnerUID *uint64) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat == nil {
		return errors.New("collector lifecycle token file owner is unavailable")
	}
	if !credentialFileOwnerAllowed(uint64(stat.Uid), tokenOwnerUID) {
		return fmt.Errorf("collector lifecycle token file %s is not owned by root or the configured collector identity", path)
	}
	return nil
}

func credentialFileOwnerAllowed(ownerUID uint64, tokenOwnerUID *uint64) bool {
	// Lifecycle commands run as root in production, so the effective-owner case
	// collapses to UID 0 there. Keeping it explicit also lets non-root package
	// tests and diagnostic invocations read only their own private files.
	return ownerUID == 0 || ownerUID == uint64(os.Geteuid()) || tokenOwnerUID != nil && ownerUID == *tokenOwnerUID
}
