package securityutil

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type stubFileInfo struct {
	mode os.FileMode
	size int64
}

func (s stubFileInfo) Name() string       { return "stub" }
func (s stubFileInfo) Size() int64        { return s.size }
func (s stubFileInfo) Mode() os.FileMode  { return s.mode }
func (s stubFileInfo) ModTime() time.Time { return time.Time{} }
func (s stubFileInfo) IsDir() bool        { return s.mode.IsDir() }
func (s stubFileInfo) Sys() any           { return nil }

func resetSecureStorageDirFns() {
	secureStorageDirMkdirAllFn = os.MkdirAll
	secureStorageDirChmodFn = os.Chmod
	secureStorageDirLstatFn = os.Lstat
	secureStorageFileLstatFn = os.Lstat
	secureStorageFileReadFn = os.ReadFile
	secureStorageFileTempFn = os.CreateTemp
	secureStorageFileRenameFn = os.Rename
	secureStorageFileRemoveFn = os.Remove
	secureStorageFileChmodFn = os.Chmod
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

func TestEnsureSecureStorageDir_AllowsMountedDirWhenChmodNotPermitted(t *testing.T) {
	t.Cleanup(resetSecureStorageDirFns)

	secureStorageDirMkdirAllFn = func(string, os.FileMode) error { return nil }
	secureStorageDirLstatFn = func(string) (os.FileInfo, error) {
		return stubFileInfo{mode: os.ModeDir | 0o777}, nil
	}
	secureStorageDirChmodFn = func(string, os.FileMode) error { return os.ErrPermission }
	if err := EnsureSecureStorageDir("/data", 0o700); err != nil {
		t.Fatalf("EnsureSecureStorageDir() error: %v", err)
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

func TestEnsureSecureStorageDir_RejectsNonDirectoryPaths(t *testing.T) {
	t.Cleanup(resetSecureStorageDirFns)

	secureStorageDirMkdirAllFn = func(string, os.FileMode) error { return nil }
	secureStorageDirLstatFn = func(string) (os.FileInfo, error) {
		return stubFileInfo{mode: 0o600}, nil
	}

	if err := EnsureSecureStorageDir("/data", 0o700); err == nil {
		t.Fatal("expected non-directory path error")
	}
}

func TestReadSecureStorageFile_RejectsSymlinkPaths(t *testing.T) {
	t.Cleanup(resetSecureStorageDirFns)

	secureStorageFileLstatFn = func(string) (os.FileInfo, error) {
		return stubFileInfo{mode: os.ModeSymlink}, nil
	}

	_, err := ReadSecureStorageFile("/data/key", 32)
	if !errors.Is(err, ErrUnsafeStorageFile) {
		t.Fatalf("expected ErrUnsafeStorageFile, got %v", err)
	}
}

func TestReadSecureStorageFile_RejectsOversizedFiles(t *testing.T) {
	t.Cleanup(resetSecureStorageDirFns)

	secureStorageFileLstatFn = func(string) (os.FileInfo, error) {
		return stubFileInfo{mode: 0o600, size: 33}, nil
	}

	_, err := ReadSecureStorageFile("/data/key", 32)
	if !errors.Is(err, ErrUnsafeStorageFile) {
		t.Fatalf("expected ErrUnsafeStorageFile, got %v", err)
	}
}

func TestWriteSecureStorageFile_CreatesOwnerOnlyFile(t *testing.T) {
	t.Cleanup(resetSecureStorageDirFns)

	root := t.TempDir()
	target := filepath.Join(root, "secure", "key.bin")
	want := []byte("secret")

	if err := WriteSecureStorageFile(target, want, 0o700, 0o600); err != nil {
		t.Fatalf("WriteSecureStorageFile() error: %v", err)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if string(data) != string(want) {
		t.Fatalf("contents = %q, want %q", data, want)
	}

	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat target: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("permissions = %o, want 600", got)
	}
}

func TestRenameSecureStorageFile(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.json")
	destination := filepath.Join(root, "source.json.bak")
	if err := os.WriteFile(source, []byte("payload"), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}

	if err := RenameSecureStorageFile(source, destination); err != nil {
		t.Fatalf("RenameSecureStorageFile() error = %v", err)
	}
	if _, err := os.Stat(source); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("source stat error = %v, want not exist", err)
	}
	if data, err := os.ReadFile(destination); err != nil || string(data) != "payload" {
		t.Fatalf("destination = %q, err = %v", data, err)
	}
}

func TestRenameSecureStorageFileRejectsUnsafePaths(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.json")
	destination := filepath.Join(root, "source.json.bak")
	if err := os.WriteFile(source, []byte("payload"), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	if err := os.WriteFile(destination, []byte("existing"), 0o600); err != nil {
		t.Fatalf("write destination: %v", err)
	}
	if err := RenameSecureStorageFile(source, destination); !errors.Is(err, ErrUnsafeStorageFile) {
		t.Fatalf("existing destination error = %v, want ErrUnsafeStorageFile", err)
	}
	if err := RenameSecureStorageFile(source, filepath.Join(t.TempDir(), "other.json")); err == nil {
		t.Fatal("expected cross-directory rename rejection")
	}
}
