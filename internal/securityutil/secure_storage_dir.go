package securityutil

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

var (
	secureStorageDirMkdirAllFn = os.MkdirAll
	secureStorageDirChmodFn    = os.Chmod
	secureStorageDirLstatFn    = os.Lstat
)

// EnsureSecureStorageDir creates or hardens a storage directory to the desired
// permissions when possible. For pre-mounted runtime storage roots such as
// Kubernetes volume mounts, the running process may be able to write inside the
// directory without owning the mount root itself. In that case, permission
// errors from chmod are tolerated as long as the existing directory is not
// world-writable.
func EnsureSecureStorageDir(dir string, perm os.FileMode) error {
	if err := secureStorageDirMkdirAllFn(dir, perm); err != nil {
		return err
	}

	info, err := secureStorageDirLstatFn(dir)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing symlink directory path %q", dir)
	}
	if !info.IsDir() {
		return fmt.Errorf("non-directory path %q", dir)
	}

	if err := secureStorageDirChmodFn(dir, perm); err != nil {
		if !isStorageDirPermissionError(err) {
			return err
		}
		if info.Mode().Perm()&0o002 != 0 {
			return fmt.Errorf("cannot leave world-writable storage directory %q unhardened: %w", dir, err)
		}
		return nil
	}

	return nil
}

func isStorageDirPermissionError(err error) bool {
	return errors.Is(err, os.ErrPermission) ||
		errors.Is(err, syscall.EPERM) ||
		errors.Is(err, syscall.EACCES)
}
