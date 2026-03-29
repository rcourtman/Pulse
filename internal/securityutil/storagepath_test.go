package securityutil

import (
	"path/filepath"
	"testing"
)

func TestNormalizeStorageDir(t *testing.T) {
	dir, err := NormalizeStorageDir("  /tmp/pulse/../pulse  ")
	if err != nil {
		t.Fatalf("NormalizeStorageDir() error = %v", err)
	}
	if dir != filepath.Clean("/tmp/pulse/../pulse") {
		t.Fatalf("NormalizeStorageDir() = %q", dir)
	}
}

func TestNormalizeStorageDirRejectsBlank(t *testing.T) {
	if _, err := NormalizeStorageDir(" \t "); err == nil {
		t.Fatal("expected blank storage dir to be rejected")
	}
}

func TestJoinStorageLeaf(t *testing.T) {
	path, err := JoinStorageLeaf("/tmp/pulse", "session.json")
	if err != nil {
		t.Fatalf("JoinStorageLeaf() error = %v", err)
	}
	if path != "/tmp/pulse/session.json" {
		t.Fatalf("JoinStorageLeaf() = %q", path)
	}
}

func TestJoinStorageLeafRejectsPathSeparators(t *testing.T) {
	tests := []string{
		"../session.json",
		"nested/session.json",
		`nested\session.json`,
		".",
		"..",
		"",
	}

	for _, leaf := range tests {
		if _, err := JoinStorageLeaf("/tmp/pulse", leaf); err == nil {
			t.Fatalf("expected %q to be rejected", leaf)
		}
	}
}
