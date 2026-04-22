package securityutil

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

var (
	secureStorageDirMkdirAllFn = os.MkdirAll
	secureStorageDirChmodFn    = os.Chmod
	secureStorageDirLstatFn    = os.Lstat
	secureStorageFileLstatFn   = os.Lstat
	secureStorageFileReadFn    = os.ReadFile
	secureStorageFileTempFn    = os.CreateTemp
	secureStorageFileRenameFn  = os.Rename
	secureStorageFileRemoveFn  = os.Remove
	secureStorageFileChmodFn   = os.Chmod
)

var ErrUnsafeStorageFile = errors.New("unsafe storage file")

// EnsureSecureStorageDir creates or hardens a storage directory to the desired
// permissions when possible. For pre-mounted runtime storage roots such as
// Kubernetes volume mounts, the running process may be able to write inside the
// directory without owning the mount root itself. In that case, permission
// errors from chmod are tolerated as long as the resolved path is still the
// expected real directory rather than a symlink or other filesystem object.
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
		return nil
	}

	return nil
}

func isStorageDirPermissionError(err error) bool {
	return errors.Is(err, os.ErrPermission) ||
		errors.Is(err, syscall.EPERM) ||
		errors.Is(err, syscall.EACCES)
}

// ReadSecureStorageFile reads a regular, non-symlink storage file while
// enforcing a caller-provided size ceiling both before and after the read.
func ReadSecureStorageFile(path string, maxSize int64) ([]byte, error) {
	if maxSize <= 0 {
		return nil, fmt.Errorf("max storage file size must be positive")
	}

	info, err := secureStorageFileLstatFn(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("%w: refusing symlink file path %q", ErrUnsafeStorageFile, path)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%w: non-regular file path %q", ErrUnsafeStorageFile, path)
	}
	if info.Size() > maxSize {
		return nil, fmt.Errorf("%w: file %q is too large (%d bytes)", ErrUnsafeStorageFile, path, info.Size())
	}

	data, err := secureStorageFileReadFn(path)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxSize {
		return nil, fmt.Errorf("%w: file %q exceeded size limit while reading", ErrUnsafeStorageFile, path)
	}

	return data, nil
}

// WriteSecureStorageFile writes a file via a temp file + rename inside an
// already validated storage directory so file creation stays owner-only and
// does not follow pre-existing symlinks at the destination path.
func WriteSecureStorageFile(path string, data []byte, dirPerm, filePerm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := EnsureSecureStorageDir(dir, dirPerm); err != nil {
		return err
	}

	tmpFile, err := secureStorageFileTempFn(dir, "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = secureStorageFileRemoveFn(tmpPath)
		}
	}()

	if err := tmpFile.Chmod(filePerm); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}
	if err := secureStorageFileRenameFn(tmpPath, path); err != nil {
		return err
	}
	cleanup = false

	return secureStorageFileChmodFn(path, filePerm)
}
