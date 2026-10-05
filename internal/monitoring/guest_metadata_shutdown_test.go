package monitoring

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

type blockedGuestMetadataFS struct {
	config.FileSystem
	entered chan struct{}
	release chan struct{}
}

func (f *blockedGuestMetadataFS) ReadFile(p string) ([]byte, error)      { return os.ReadFile(p) }
func (f *blockedGuestMetadataFS) Stat(p string) (os.FileInfo, error)     { return os.Stat(p) }
func (f *blockedGuestMetadataFS) MkdirAll(p string, m os.FileMode) error { return os.MkdirAll(p, m) }
func (f *blockedGuestMetadataFS) WriteFile(p string, b []byte, m os.FileMode) error {
	close(f.entered)
	<-f.release
	return os.WriteFile(p, b, m)
}
func (f *blockedGuestMetadataFS) Rename(a, b string) error { return os.Rename(a, b) }

func TestTenantDeletionRetainsBlockedGuestWriterOwnership(t *testing.T) {
	path := t.TempDir()
	fs := &blockedGuestMetadataFS{entered: make(chan struct{}), release: make(chan struct{})}
	store := config.NewGuestMetadataStore(path, fs)
	monitor := &Monitor{guestMetadataStore: store}
	mtm := NewMultiTenantMonitor(&config.Config{DataPath: path}, nil, nil)
	mtm.monitors["blocked"] = monitor
	t.Cleanup(func() {
		select {
		case <-fs.release:
		default:
			close(fs.release)
		}
		_ = store.Close(time.Second)
		mtm.Stop()
	})
	persistGuestIdentity(store, "pve:node:100", "retained", "qemu")
	select {
	case <-fs.entered:
	case <-time.After(time.Second):
		t.Fatal("write did not start")
	}
	if err := mtm.BeginTenantDeletion("blocked"); err == nil {
		t.Fatal("deletion accepted incomplete drain")
	}
	if retained, ok := mtm.PeekMonitor("blocked"); !ok || retained != monitor {
		t.Fatal("in-flight writer owner was discarded")
	}
	mtm.FinishTenantDeletion("blocked")
	if _, err := mtm.GetMonitor("blocked"); err == nil {
		t.Fatal("deferred cleanup released unsafe lifecycle guard")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("data path not retained: %v", err)
	}
	close(fs.release)
	if err := store.Close(time.Second); err != nil {
		t.Fatal(err)
	}
	if err := mtm.BeginTenantDeletion("blocked"); err != nil {
		t.Fatal(err)
	}
	if _, ok := mtm.PeekMonitor("blocked"); ok {
		t.Fatal("completed shutdown not removed")
	}
	if err := store.RememberIdentity("late", "late", "qemu"); err == nil {
		t.Fatal("stopped store accepted late identity")
	}
	if _, err := os.Stat(filepath.Join(path, "guest_metadata.json")); err != nil {
		t.Fatal("accepted metadata not persisted")
	}
}
func TestTenantDeletionRetainsUnfinishedMonitorLoop(t *testing.T) {
	mtm := NewMultiTenantMonitor(&config.Config{}, nil, nil)
	_, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	monitor := &Monitor{}
	mtm.monitors["loop"] = monitor
	mtm.tenantCancel["loop"] = cancel
	mtm.tenantDone["loop"] = done
	t.Cleanup(func() {
		select {
		case <-done:
		default:
			close(done)
		}
		mtm.Stop()
	})
	if err := mtm.BeginTenantDeletion("loop"); err == nil {
		t.Fatal("unfinished monitoring loop admitted directory deletion")
	}
	if retained, ok := mtm.PeekMonitor("loop"); !ok || retained != monitor {
		t.Fatal("unfinished monitoring loop lost owner")
	}
	if _, err := mtm.GetMonitor("loop"); err == nil {
		t.Fatal("unfinished monitor replaced")
	}
	close(done)
	if err := mtm.BeginTenantDeletion("loop"); err != nil {
		t.Fatal(err)
	}
}
func TestPersistGuestIdentityEditsAreImmediateAndFieldScoped(t *testing.T) {
	store := config.NewGuestMetadataStore(t.TempDir(), nil)
	if err := store.Set("pve:node:100", &config.GuestMetadata{CustomURL: "https://operator.example", Tags: []string{"important"}, Notes: []string{"keep"}}); err != nil {
		t.Fatal(err)
	}
	persistGuestIdentity(store, "pve:node:100", "old", "qemu")
	persistGuestIdentity(store, "pve:node:100", "new", "qemu")
	got := store.Get("pve:node:100")
	if got.LastKnownName != "new" || got.CustomURL != "https://operator.example" || got.Tags[0] != "important" || got.Notes[0] != "keep" {
		t.Fatalf("producer corrupted metadata: %+v", got)
	}
	if err := store.Close(time.Second); err != nil {
		t.Fatal(err)
	}
}
