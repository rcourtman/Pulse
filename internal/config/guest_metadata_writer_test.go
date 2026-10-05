package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func guestWriterTestStore(t *testing.T) (*GuestMetadataStore, *controlledGuestWriterFS) {
	t.Helper()
	fs := &controlledGuestWriterFS{FileSystem: defaultFileSystem{}, entered: make(chan struct{}), release: make(chan struct{})}
	store := NewGuestMetadataStore(t.TempDir(), fs)
	var releaseOnce sync.Once
	t.Cleanup(func() {
		releaseOnce.Do(func() {
			select {
			case <-fs.release:
			default:
				close(fs.release)
			}
		})
		if !store.WaitForPendingWrites(time.Second) {
			t.Error("writer did not exit during cleanup")
		}
	})
	if err := store.SetAsync("first", &GuestMetadata{LastKnownName: "first"}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-fs.entered:
	case <-time.After(time.Second):
		t.Fatal("writer not started")
	}
	return store, fs
}
func capturedGuestSnapshot(t *testing.T, fs *controlledGuestWriterFS) map[string]*GuestMetadata {
	t.Helper()
	fs.mu.Lock()
	data := append([]byte(nil), fs.data...)
	fs.mu.Unlock()
	var result map[string]*GuestMetadata
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	return result
}
func TestGuestMetadataWriterOrderedReplacementsAndReadsDuringIO(t *testing.T) {
	store, fs := guestWriterTestStore(t)
	for i := 0; i < 100; i++ {
		if err := store.SetAsync("same", &GuestMetadata{LastKnownName: fmt.Sprint(i)}); err != nil {
			t.Fatal(err)
		}
	}
	// This is an admission-state read, not a persistence-success assertion.
	read := make(chan *GuestMetadata, 1)
	go func() { read <- store.Get("same") }()
	select {
	case got := <-read:
		if got.LastKnownName != "99" {
			t.Fatalf("wrong admission order: %+v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("disk I/O blocked metadata reads")
	}
	close(fs.release)
	if err := store.Close(time.Second); err != nil {
		t.Fatal(err)
	}
	if got := capturedGuestSnapshot(t, fs)["same"]; got.LastKnownName != "99" {
		t.Fatalf("older update persisted last: %+v", got)
	}
	fs.mu.Lock()
	defer fs.mu.Unlock()
	if fs.writes != 2 {
		t.Fatalf("100 replacements during one blocked write caused %d writes", fs.writes)
	}
}
func TestGuestMetadataWriterSynchronousMutationsCannotBeOverwritten(t *testing.T) {
	for _, operation := range []string{"set", "delete", "replace", "update", "migration"} {
		t.Run(operation, func(t *testing.T) {
			store, fs := guestWriterTestStore(t)
			id := "pve:node:100"
			if err := store.SetAsync(id, &GuestMetadata{LastKnownName: "queued"}); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() {
				switch operation {
				case "set":
					done <- store.Set(id, &GuestMetadata{LastKnownName: "operator", Notes: []string{"new"}})
				case "delete":
					done <- store.Delete(id)
				case "replace":
					done <- store.ReplaceAll(map[string]*GuestMetadata{"replacement": {LastKnownName: "replacement"}})
				case "update":
					done <- store.UpdateAll(func(m map[string]*GuestMetadata) bool { m[id].Notes = []string{"edited"}; return true })
				case "migration":
					if store.GetWithLegacyMigration("pve:other:100", "pve", "other", 100) == nil {
						done <- errors.New("migration missing")
					} else {
						done <- nil
					}
				}
			}()
			close(fs.release)
			select {
			case err := <-done:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(time.Second):
				t.Fatal("synchronous mutation blocked")
			}
			if err := store.Close(time.Second); err != nil {
				t.Fatal(err)
			}
			saved := capturedGuestSnapshot(t, fs)
			if !reflect.DeepEqual(saved, store.GetAll()) {
				t.Fatalf("disk snapshot differs after %s: saved=%+v memory=%+v", operation, saved, store.GetAll())
			}
			switch operation {
			case "set":
				if saved[id].LastKnownName != "operator" || saved[id].Notes[0] != "new" {
					t.Fatal("operator replacement lost")
				}
			case "delete":
				if saved[id] != nil {
					t.Fatal("deleted entry resurrected")
				}
			case "replace":
				if len(saved) != 1 || saved["replacement"] == nil {
					t.Fatal("replace resurrected old entries")
				}
			case "update":
				if saved[id].Notes[0] != "edited" {
					t.Fatal("update lost")
				}
			case "migration":
				if saved[id] != nil || saved["pve:other:100"] == nil {
					t.Fatal("old node resurrected")
				}
			}
		})
	}
}
func TestGuestMetadataWriterIdentityPreservesOperatorFieldsAndOCI(t *testing.T) {
	store := NewGuestMetadataStore(t.TempDir(), nil)
	original := &GuestMetadata{CustomURL: "https://operator.example", Description: "operator", Tags: []string{"tag"}, Notes: []string{"note"}, LastKnownName: "old", LastKnownType: "oci"}
	if err := store.Set("pve:node:100", original); err != nil {
		t.Fatal(err)
	}
	if err := store.RememberIdentity("pve:node:100", "new", " lxc "); err != nil {
		t.Fatal(err)
	}
	got := store.Get("pve:node:100")
	if got.LastKnownName != "new" || got.LastKnownType != "oci" {
		t.Fatalf("identity/OCI lost: %+v", got)
	}
	original.ID = "pve:node:100"
	original.LastKnownName = "new"
	if !reflect.DeepEqual(got, original) {
		t.Fatalf("identity changed operator data: %+v", got)
	}
	if !store.WaitForPendingWrites(time.Second) {
		t.Fatal("identity not drained")
	}
	before := store.revision
	if err := store.RememberIdentity("pve:node:100", "new", "oci"); err != nil {
		t.Fatal(err)
	}
	if err := store.RememberIdentity("pve:node:100", "ignored", " "); err != nil {
		t.Fatal(err)
	}
	if store.revision != before {
		t.Fatal("unchanged/empty type triggered a write")
	}
	if err := store.Close(time.Second); err != nil {
		t.Fatal(err)
	}
	reloaded := NewGuestMetadataStore(store.dataPath, nil)
	if !reflect.DeepEqual(reloaded.Get("pve:node:100"), got) {
		t.Fatal("identity/operator fields not persisted")
	}
}
func TestGuestMetadataWriterCloseBoundsBlockedAsyncAndSyncIO(t *testing.T) {
	for _, synchronous := range []bool{false, true} {
		t.Run(fmt.Sprint(synchronous), func(t *testing.T) {
			fs := &controlledGuestWriterFS{FileSystem: defaultFileSystem{}, entered: make(chan struct{}), release: make(chan struct{})}
			store := NewGuestMetadataStore(t.TempDir(), fs)
			done := make(chan error, 1)
			if synchronous {
				go func() { done <- store.Set("first", &GuestMetadata{LastKnownName: "first"}) }()
			} else {
				done <- store.SetAsync("first", &GuestMetadata{LastKnownName: "first"})
			}
			select {
			case <-fs.entered:
			case <-time.After(time.Second):
				t.Fatal("write did not start")
			}
			t.Cleanup(func() {
				select {
				case <-fs.release:
				default:
					close(fs.release)
				}
				_ = store.Close(time.Second)
			})
			start := time.Now()
			if err := store.Close(10 * time.Millisecond); err == nil {
				t.Fatal("blocked writer reported close success")
			}
			if elapsed := time.Since(start); elapsed > time.Second {
				t.Fatalf("close exceeded predeclared bounded budget: %s", elapsed)
			}
			close(fs.release)
			select {
			case err := <-done:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(time.Second):
				t.Fatal("write still running")
			}
			if err := store.Close(time.Second); err != nil {
				t.Fatal(err)
			}
			for _, mutation := range []func() error{
				func() error { return store.SetAsync("later", &GuestMetadata{}) },
				func() error { return store.RememberIdentity("later", "later", "qemu") },
				func() error { return store.Set("later", &GuestMetadata{}) },
				func() error { return store.Delete("first") },
				func() error { return store.ReplaceAll(nil) },
				func() error { return store.UpdateAll(func(map[string]*GuestMetadata) bool { return true }) },
				store.Load,
			} {
				if err := mutation(); !errors.Is(err, ErrGuestMetadataStoreClosed) {
					t.Fatalf("closed admission accepted: %v", err)
				}
			}
			if store.GetWithLegacyMigration("node:100", "pve", "node", 100) != nil {
				t.Fatal("closed store migrated metadata")
			}
			if capturedGuestSnapshot(t, fs)["later"] != nil {
				t.Fatal("post-close write")
			}
		})
	}
}

type guestWriterFaultFS struct {
	FileSystem
	failWrite  atomic.Bool
	failRename atomic.Bool
	writes     atomic.Int32
}

func (f *guestWriterFaultFS) WriteFile(path string, data []byte, perm os.FileMode) error {
	f.writes.Add(1)
	if f.failWrite.Load() {
		return errors.New("controlled write failure")
	}
	return f.FileSystem.WriteFile(path, data, perm)
}
func (f *guestWriterFaultFS) Rename(a, b string) error {
	if f.failRename.Load() {
		return errors.New("controlled rename failure")
	}
	return f.FileSystem.Rename(a, b)
}
func TestGuestMetadataWriterFailuresAreOwnedWithoutSpinRetry(t *testing.T) {
	for _, phase := range []string{"write", "rename"} {
		t.Run(phase, func(t *testing.T) {
			fs := &guestWriterFaultFS{FileSystem: defaultFileSystem{}}
			store := NewGuestMetadataStore(t.TempDir(), fs)
			if err := store.Set("key", &GuestMetadata{LastKnownName: "original"}); err != nil {
				t.Fatal(err)
			}
			fs.failWrite.Store(phase == "write")
			fs.failRename.Store(phase == "rename")
			if err := store.RememberIdentity("key", "changed", "qemu"); err != nil {
				t.Fatal(err)
			}
			if !store.WaitForPendingWrites(time.Second) {
				t.Fatal("failed attempt not quiescent")
			}
			if fs.writes.Load() != 2 {
				t.Fatalf("failure retried automatically: %d", fs.writes.Load())
			}
			if got := NewGuestMetadataStore(store.dataPath, nil).Get("key"); got.LastKnownName != "original" {
				t.Fatal("failure replaced good disk snapshot")
			}
			fs.failWrite.Store(false)
			fs.failRename.Store(false)
			// A later ordinary poll retries the unpersisted identity, not a tight loop.
			if err := store.RememberIdentity("key", "changed", "qemu"); err != nil {
				t.Fatal(err)
			}
			if err := store.Close(time.Second); err != nil {
				t.Fatal(err)
			}
			if fs.writes.Load() != 3 {
				t.Fatalf("explicit recovery writes=%d", fs.writes.Load())
			}
			if got := NewGuestMetadataStore(store.dataPath, nil).Get("key"); got.LastKnownName != "changed" {
				t.Fatal("recovery not persisted")
			}
			fs.failWrite.Store(true)
			failed := NewGuestMetadataStore(t.TempDir(), fs)
			if err := failed.SetAsync("lost", &GuestMetadata{}); err != nil {
				t.Fatal(err)
			}
			if err := failed.Close(time.Second); err == nil {
				t.Fatal("failed final write reported durable close")
			}
		})
	}
}
func TestGuestMetadataWriterRealFilesystemTenantIsolation(t *testing.T) {
	root := t.TempDir()
	a := NewGuestMetadataStore(filepath.Join(root, "orgs", "a"), nil)
	b := NewGuestMetadataStore(filepath.Join(root, "orgs", "b"), nil)
	for i := 0; i < 100; i++ {
		id := fmt.Sprintf("pve:node:%d", i)
		if err := a.RememberIdentity(id, "a", "qemu"); err != nil {
			t.Fatal(err)
		}
		if err := b.RememberIdentity(id, "b", "lxc"); err != nil {
			t.Fatal(err)
		}
	}
	for _, store := range []*GuestMetadataStore{a, b} {
		if err := store.Close(10 * time.Second); err != nil {
			t.Fatal(err)
		}
		reloaded := NewGuestMetadataStore(store.dataPath, nil)
		if !reflect.DeepEqual(reloaded.GetAll(), store.GetAll()) {
			t.Fatal("real filesystem lost entries")
		}
		info, err := os.Stat(filepath.Join(store.dataPath, "guest_metadata.json"))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("unsafe file permissions: %v", info.Mode())
		}
		if _, err := os.Stat(filepath.Join(store.dataPath, "guest_metadata.json.tmp")); !os.IsNotExist(err) {
			t.Fatalf("temporary file survived close: %v", err)
		}
	}
	if a.Get("pve:node:1").LastKnownName == b.Get("pve:node:1").LastKnownName {
		t.Fatal("tenants share identity")
	}
}

// Retention after a failed close is not a permanent offboarding dead end.
// A later explicit close may flush retained state after the filesystem recovers,
// without accepting new mutations or overlapping the original writer.
func TestGuestMetadataWriterExplicitCloseRetryPersistsRetainedState(t *testing.T) {
	fs := &guestWriterFaultFS{FileSystem: defaultFileSystem{}}
	fs.failWrite.Store(true)
	store := NewGuestMetadataStore(t.TempDir(), fs)
	if err := store.SetAsync("retained", &GuestMetadata{LastKnownName: "retained"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(time.Second); err == nil {
		t.Fatal("first failed close reported success")
	}
	if fs.writes.Load() != 1 {
		t.Fatalf("failed close retried without an explicit later call: %d", fs.writes.Load())
	}
	if err := store.SetAsync("late", &GuestMetadata{}); !errors.Is(err, ErrGuestMetadataStoreClosed) {
		t.Fatalf("failed close reopened admission: %v", err)
	}
	fs.failWrite.Store(false)
	if err := store.Close(time.Second); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(time.Second); err != nil {
		t.Fatal(err)
	}
	if fs.writes.Load() != 2 {
		t.Fatalf("explicit recovery or successful idempotent close wrote unexpected snapshots: %d", fs.writes.Load())
	}
	reloaded := NewGuestMetadataStore(store.dataPath, nil)
	if reloaded.Get("retained") == nil || reloaded.Get("late") != nil {
		t.Fatal("explicit close recovery lost retained state or wrote a late admission")
	}
}
