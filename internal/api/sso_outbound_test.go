package api

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
)

func TestReadSSORegularFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "idp.pem")
	if err := os.WriteFile(path, []byte("certificate"), 0o600); err != nil {
		t.Fatalf("write SSO file: %v", err)
	}

	data, err := readSSORegularFile(path)
	if err != nil {
		t.Fatalf("readSSORegularFile() error = %v", err)
	}
	if string(data) != "certificate" {
		t.Fatalf("readSSORegularFile() = %q", data)
	}
}

func TestReadSSORegularFileRejectsSymlinkAndOversizedFile(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target.pem")
	if err := os.WriteFile(target, []byte("certificate"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	symlink := filepath.Join(root, "idp.pem")
	if err := os.Symlink(target, symlink); err != nil {
		t.Fatalf("create symlink: %v", err)
	}
	if _, err := readSSORegularFile(symlink); !errors.Is(err, securityutil.ErrUnsafeStorageFile) {
		t.Fatalf("symlink error = %v, want ErrUnsafeStorageFile", err)
	}

	oversized := filepath.Join(root, "oversized.pem")
	file, err := os.Create(oversized)
	if err != nil {
		t.Fatalf("create oversized file: %v", err)
	}
	if err := file.Truncate(maxSSOFileSize + 1); err != nil {
		_ = file.Close()
		t.Fatalf("truncate oversized file: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close oversized file: %v", err)
	}
	if _, err := readSSORegularFile(oversized); !errors.Is(err, securityutil.ErrUnsafeStorageFile) {
		t.Fatalf("oversized error = %v, want ErrUnsafeStorageFile", err)
	}
}
