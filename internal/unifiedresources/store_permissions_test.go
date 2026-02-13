package unifiedresources

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewSQLiteResourceStoreCreatesSecureDirectory(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "resources")

	store, err := NewSQLiteResourceStore(dataDir, "org-1")
	if err != nil {
		t.Fatalf("NewSQLiteResourceStore() error = %v", err)
	}
	defer store.Close()

	info, err := os.Stat(dataDir)
	if err != nil {
		t.Fatalf("Stat(%q) error = %v", dataDir, err)
	}

	if perms := info.Mode().Perm(); perms != 0o700 {
		t.Fatalf("resources directory permissions = %o, want 700", perms)
	}
}
