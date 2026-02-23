package unifiedresources

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewSQLiteResourceStoreCreatesSecureDirectory(t *testing.T) {
	dataDir := t.TempDir()

	store, err := NewSQLiteResourceStore(dataDir, "org-1")
	if err != nil {
		t.Fatalf("NewSQLiteResourceStore() error = %v", err)
	}
	defer store.Close()

	resourcesDir := filepath.Join(dataDir, "orgs", "org-1", "resources")
	info, err := os.Stat(resourcesDir)
	if err != nil {
		t.Fatalf("Stat(%q) error = %v", resourcesDir, err)
	}

	if perms := info.Mode().Perm(); perms != 0o700 {
		t.Fatalf("resources directory permissions = %o, want 700", perms)
	}
}
