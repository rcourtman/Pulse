package securityutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHardenPrivatePathProtectsDirectoryAndFile(t *testing.T) {
	dir := t.TempDir()
	if err := HardenPrivatePath(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	dirInfo, err := os.Lstat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidatePrivatePath(dir, dirInfo); err != nil {
		t.Fatalf("private directory validation failed: %v", err)
	}

	path := filepath.Join(dir, "state.json")
	if err := os.WriteFile(path, []byte("state"), 0o666); err != nil {
		t.Fatal(err)
	}
	if err := HardenPrivatePath(path, 0o600); err != nil {
		t.Fatal(err)
	}
	fileInfo, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidatePrivatePath(path, fileInfo); err != nil {
		t.Fatalf("private file validation failed: %v", err)
	}
}
