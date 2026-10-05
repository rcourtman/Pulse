package config

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"
)

type controlledGuestWriterFS struct {
	FileSystem
	mu      sync.Mutex
	writes  int
	data    []byte
	entered chan struct{}
	release chan struct{}
}

func (f *controlledGuestWriterFS) MkdirAll(string, os.FileMode) error { return nil }
func (f *controlledGuestWriterFS) Rename(string, string) error        { return nil }
func (f *controlledGuestWriterFS) WriteFile(_ string, data []byte, _ os.FileMode) error {
	f.mu.Lock()
	f.writes++
	first := f.writes == 1
	f.mu.Unlock()
	if first {
		close(f.entered)
		<-f.release
	}
	f.mu.Lock()
	f.data = append([]byte(nil), data...)
	f.mu.Unlock()
	return nil
}
func TestGuestWriterControlledBurst(t *testing.T) {
	for _, count := range []int{25, 100} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			fs := &controlledGuestWriterFS{FileSystem: defaultFileSystem{}, entered: make(chan struct{}), release: make(chan struct{})}
			store := NewGuestMetadataStore(t.TempDir(), fs)
			store.SetAsync("first", &GuestMetadata{LastKnownName: "first"})
			select {
			case <-fs.entered:
			case <-time.After(time.Second):
				t.Fatal("first write did not start")
			}
			for i := 0; i < count; i++ {
				store.SetAsync(fmt.Sprint(i), &GuestMetadata{LastKnownName: fmt.Sprint(i)})
			}
			close(fs.release)
			if !store.WaitForPendingWrites(10 * time.Second) {
				t.Fatal("drain failed")
			}
			fs.mu.Lock()
			writes := fs.writes
			data := append([]byte(nil), fs.data...)
			fs.mu.Unlock()
			t.Logf("admitted=%d complete-file-writes=%d", count+1, writes)
			if writes > 2 {
				t.Errorf("one blocked snapshot plus one complete follow-up needed, got %d", writes)
			}
			var saved map[string]*GuestMetadata
			if err := json.Unmarshal(data, &saved); err != nil {
				t.Fatal(err)
			}
			if len(saved) != count+1 {
				t.Fatalf("lost guests: got %d want %d", len(saved), count+1)
			}
			for i := 0; i < count; i++ {
				if saved[fmt.Sprint(i)].LastKnownName != fmt.Sprint(i) {
					t.Errorf("wrong guest %d", i)
				}
			}
		})
	}
}
func TestGuestWriterCopiesAtAdmission(t *testing.T) {
	fs := &controlledGuestWriterFS{FileSystem: defaultFileSystem{}, entered: make(chan struct{}), release: make(chan struct{})}
	store := NewGuestMetadataStore(t.TempDir(), fs)
	store.SetAsync("first", &GuestMetadata{LastKnownName: "first"})
	select {
	case <-fs.entered:
	case <-time.After(time.Second):
		t.Fatal("first write did not start")
	}
	meta := &GuestMetadata{LastKnownName: "original", Tags: []string{"original"}, Notes: []string{"original"}}
	store.SetAsync("second", meta)
	meta.LastKnownName = "mutated"
	meta.Tags[0] = "mutated"
	meta.Notes[0] = "mutated"
	close(fs.release)
	if !store.WaitForPendingWrites(10 * time.Second) {
		t.Fatal("drain failed")
	}
	got := store.Get("second")
	if got.LastKnownName != "original" || got.Tags[0] != "original" || got.Notes[0] != "original" {
		t.Fatalf("caller mutation reached stored metadata: %+v", got)
	}
}
