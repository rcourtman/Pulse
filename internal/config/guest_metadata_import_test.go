package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"
)

type importBlockedMetadataFS struct {
	FileSystem
	once    sync.Once
	entered chan struct{}
	release chan struct{}
}

func (f *importBlockedMetadataFS) WriteFile(path string, data []byte, perm os.FileMode) error {
	if filepath.Base(path) == "guest_metadata.json.tmp" {
		f.once.Do(func() { close(f.entered); <-f.release })
	}
	return f.FileSystem.WriteFile(path, data, perm)
}

func TestGuestMetadataImportOrdersPendingSnapshotsAndCopiesInput(t *testing.T) {
	dir := t.TempDir()
	fs := &importBlockedMetadataFS{FileSystem: defaultFileSystem{}, entered: make(chan struct{}), release: make(chan struct{})}
	store := NewGuestMetadataStore(dir, fs)
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(fs.release) }) }
	t.Cleanup(func() {
		release()
		if err := store.Close(time.Second); err != nil {
			t.Error(err)
		}
	})
	if err := store.SetAsync("old:101", &GuestMetadata{Description: "Old snapshot"}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-fs.entered:
	case <-time.After(time.Second):
		t.Fatal("pending metadata writer did not enter")
	}
	if err := store.RememberIdentity("old:101", "Old guest", "qemu"); err != nil {
		t.Fatal(err)
	}
	tx, err := newImportTransaction(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Cleanup()
	settings := filepath.Join(dir, "system.json")
	if err := os.WriteFile(settings, []byte("old settings"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := tx.StageFile(settings, []byte("imported settings"), 0600); err != nil {
		t.Fatal(err)
	}
	metadata := map[string]*GuestMetadata{"new:202": {ID: "wrong", Description: "Imported snapshot", Tags: []string{"imported"}, Notes: []string{"note"}}}
	done := make(chan error, 1)
	go func() { done <- store.commitImport(tx, metadata) }()
	select {
	case err := <-done:
		t.Fatalf("import overtook an in-flight metadata snapshot: %v", err)
	case <-time.After(50 * time.Millisecond):
		// A bounded negative observation, followed by the positive joined proof.
	}
	if data, err := os.ReadFile(settings); err != nil || string(data) != "old settings" {
		t.Fatal("configuration committed while the old metadata write was still in flight")
	}
	release()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("import did not complete after the prior snapshot")
	}
	if !store.WaitForPendingWrites(time.Second) {
		t.Fatal("pending writer did not drain")
	}
	expected := map[string]*GuestMetadata{"new:202": {ID: "new:202", Description: "Imported snapshot", Tags: []string{"imported"}, Notes: []string{"note"}}}
	metadata["new:202"].Tags[0] = "caller mutation"
	metadata["new:202"].Notes[0] = "caller mutation"
	if !reflect.DeepEqual(store.GetAll(), expected) {
		t.Fatal("import retained old records or caller-owned slices")
	}
	data, err := os.ReadFile(filepath.Join(dir, "guest_metadata.json"))
	if err != nil {
		t.Fatal(err)
	}
	var persisted map[string]*GuestMetadata
	if err := json.Unmarshal(data, &persisted); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(persisted, expected) {
		t.Fatal("pending snapshot overwrote the imported metadata file")
	}
	if data, err := os.ReadFile(settings); err != nil || string(data) != "imported settings" {
		t.Fatal("metadata import did not commit its settings")
	}
	if err := store.Load(); err != nil {
		t.Fatalf("committed revision was left pending: %v", err)
	}
}

func TestGuestMetadataImportStageFailurePreservesMemory(t *testing.T) {
	cp := seedImportValidationState(t)
	store := cp.GetGuestMetadataStore()
	before := store.GetAll()
	tx, err := newImportTransaction(cp.configDir)
	if err != nil {
		t.Fatal(err)
	}
	// An occupied staging pathname is an actual filesystem failure before
	// commit, not a simulated successful persistence result.
	if err := os.Remove(tx.stagingDir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tx.stagingDir, []byte("block staging"), 0600); err != nil {
		t.Fatal(err)
	}
	defer tx.Cleanup()
	if err := store.commitImport(tx, map[string]*GuestMetadata{"new:202": {Description: "Do not publish"}}); err == nil {
		t.Fatal("metadata staging failure was accepted")
	}
	if !reflect.DeepEqual(before, store.GetAll()) || tx.committed {
		t.Fatal("staging failure changed live metadata or committed configuration")
	}
}
