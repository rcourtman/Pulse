//go:build windows

package collectorlifecycle

import (
	"errors"
	"fmt"
	"os"

	internalsecurity "github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
)

func openCredentialFile(path string) (*os.File, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() {
		return nil, errors.New("collector lifecycle token file must not be a symlink or reparse point")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	after, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}
	if !os.SameFile(before, after) {
		file.Close()
		return nil, errors.New("collector lifecycle token file changed while it was opened")
	}
	return file, nil
}

func validateCredentialFileOwner(path string, info os.FileInfo, _ *uint64) error {
	if err := internalsecurity.ValidatePrivatePath(path, info); err != nil {
		return fmt.Errorf("collector lifecycle token file owner or DACL is not trusted: %w", err)
	}
	return nil
}
