package cloudcp

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/api"
)

// Retained unread authentication input is not active authority, but remains
// part of the ordinary private data tree. The real backup and runtime-copy
// consumers must keep its exact bytes without failing on original mode000.
func TestProviderMSPBackupRetainsUnreadSessionsWithoutBlockingCopy(t *testing.T) {
	dir := t.TempDir()
	original := []byte("{unverified synthetic session bytes")
	if err := os.WriteFile(filepath.Join(dir, "sessions.json"), original, 0000); err != nil {
		t.Fatal(err)
	}
	store := api.NewSessionStore(dir)
	store.CreateSession("new-synthetic-session", time.Hour, "fixture", "127.0.0.1", "new-user")
	store.Shutdown()
	retained, err := filepath.Glob(filepath.Join(dir, ".sessions-unread-*", "sessions.json"))
	if err != nil || len(retained) != 1 {
		t.Fatalf("retained input count = %d, error = %v", len(retained), err)
	}
	rel, err := filepath.Rel(dir, retained[0])
	if err != nil {
		t.Fatal(err)
	}

	var archive bytes.Buffer
	tw := tar.NewWriter(&archive)
	_, _, err = addProviderMSPBackupDir(context.Background(), tw, t.TempDir(), dir, "tenant", false, time.Now().UTC())
	if err != nil {
		t.Fatalf("backup traversal rejected a preserved session file: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	reader := tar.NewReader(&archive)
	found := false
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if header.Name == filepath.ToSlash(filepath.Join("tenant", rel)) {
			data, err := io.ReadAll(reader)
			if err != nil || !bytes.Equal(data, original) || header.Mode != 0600 {
				t.Fatalf("backup lost original bytes or owner-only file mode: %v", err)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("backup silently omitted the retained original")
	}

	copied := filepath.Join(t.TempDir(), "runtime-copy")
	if err := syncTenantRuntimeTree(context.Background(), dir, copied); err != nil {
		t.Fatalf("runtime copy rejected a preserved session file: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(copied, rel))
	if err != nil || !bytes.Equal(data, original) {
		t.Fatalf("runtime copy did not retain original bytes: %v", err)
	}
	info, err := os.Stat(filepath.Dir(filepath.Join(copied, rel)))
	if err != nil || info.Mode().Perm()&0077 != 0 {
		t.Fatalf("runtime copy widened the retained directory access: %v", err)
	}
	restarted := api.NewSessionStore(copied)
	defer restarted.Shutdown()
	if !restarted.ValidateSession("new-synthetic-session") {
		t.Fatal("runtime copy did not retain the new authenticated session")
	}
}
