package securityutil

import (
	"os"
	"testing"
	"time"
)

type stubFileInfo struct {
	mode os.FileMode
}

func (s stubFileInfo) Name() string       { return "stub" }
func (s stubFileInfo) Size() int64        { return 0 }
func (s stubFileInfo) Mode() os.FileMode  { return s.mode }
func (s stubFileInfo) ModTime() time.Time { return time.Time{} }
func (s stubFileInfo) IsDir() bool        { return s.mode.IsDir() }
func (s stubFileInfo) Sys() any           { return nil }

func resetSecureStorageDirFns() {
	secureStorageDirMkdirAllFn = os.MkdirAll
	secureStorageDirChmodFn = os.Chmod
	secureStorageDirLstatFn = os.Lstat
}

func TestEnsureSecureStorageDir_CreatesOwnerOnlyDirectory(t *testing.T) {
	t.Cleanup(resetSecureStorageDirFns)

	root := t.TempDir()
	target := root + "/secure"

	if err := EnsureSecureStorageDir(target, 0o700); err != nil {
		t.Fatalf("EnsureSecureStorageDir() error: %v", err)
	}

	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat target: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o700 {
		t.Fatalf("permissions = %o, want 700", got)
	}
}

func TestEnsureSecureStorageDir_AllowsExistingCompatibleDirWhenChmodNotPermitted(t *testing.T) {
	t.Cleanup(resetSecureStorageDirFns)

	secureStorageDirMkdirAllFn = func(string, os.FileMode) error { return nil }
	secureStorageDirLstatFn = func(string) (os.FileInfo, error) {
		return stubFileInfo{mode: os.ModeDir | 0o775}, nil
	}
	secureStorageDirChmodFn = func(string, os.FileMode) error { return os.ErrPermission }

	if err := EnsureSecureStorageDir("/data", 0o700); err != nil {
		t.Fatalf("EnsureSecureStorageDir() error: %v", err)
	}
}

func TestEnsureSecureStorageDir_RejectsWorldWritableExistingDirWhenChmodNotPermitted(t *testing.T) {
	t.Cleanup(resetSecureStorageDirFns)

	secureStorageDirMkdirAllFn = func(string, os.FileMode) error { return nil }
	secureStorageDirLstatFn = func(string) (os.FileInfo, error) {
		return stubFileInfo{mode: os.ModeDir | 0o777}, nil
	}
	secureStorageDirChmodFn = func(string, os.FileMode) error { return os.ErrPermission }

	if err := EnsureSecureStorageDir("/data", 0o700); err == nil {
		t.Fatal("expected world-writable directory error")
	}
}

func TestEnsureSecureStorageDir_RejectsSymlinkPaths(t *testing.T) {
	t.Cleanup(resetSecureStorageDirFns)

	secureStorageDirMkdirAllFn = func(string, os.FileMode) error { return nil }
	secureStorageDirLstatFn = func(string) (os.FileInfo, error) {
		return stubFileInfo{mode: os.ModeSymlink}, nil
	}

	if err := EnsureSecureStorageDir("/data", 0o700); err == nil {
		t.Fatal("expected symlink directory error")
	}
}
