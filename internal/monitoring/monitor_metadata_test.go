package monitoring

import (
	"os"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestPersistGuestIdentity_Concurrent(t *testing.T) {
	// Setup temporary metadata store
	tmpDir, err := os.MkdirTemp("", "persist_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// metadataFile := filepath.Join(tmpDir, "guest_metadata.json") // Actually NewGuestMetadataStore takes directory not file usually? No, it takes root dir?
	// Let's verify standard usage. "store := NewGuestMetadataStore(tmpDir, nil)"
	// Implementation: func NewGuestMetadataStore(dataPath string, fs FileSystem)
	// Inside it does filepath.Join(dataPath, "guest_metadata.json") ?
	// Let's re-read NewGuestMetadataStore in internal/config/guest_metadata.go via grep or similar if needed.
	// But based on "config.NewGuestMetadataStore(metadataFile)" from my previous code failing, and grep showing "dataPath", it likely takes a Dir or File path.
	// grep output: guestMetadataStore := NewGuestMetadataStore(dataPath, c.fs)
	// Most likely directory.

	store := config.NewGuestMetadataStore(tmpDir, nil)

	guestKey := "pve1:node1:100"

	// Test basic persistence
	persistGuestIdentity(store, guestKey, "VM 100", "qemu")

	// Wait a bit since persistGuestIdentity is async
	time.Sleep(50 * time.Millisecond)

	meta := store.Get(guestKey)
	if meta == nil || meta.LastKnownName != "VM 100" || meta.LastKnownType != "qemu" {
		t.Errorf("Failed to persist guest identity: %+v", meta)
	}

	// Test persistence with "downgrade" prevention
	// Set type to "oci" manually first
	ociMeta := &config.GuestMetadata{
		ID:            guestKey,
		LastKnownName: "VM 100",
		LastKnownType: "oci",
	}
	_ = store.Set(guestKey, ociMeta)

	// Try to update to "lxc"
	persistGuestIdentity(store, guestKey, "VM 100", "lxc")
	time.Sleep(50 * time.Millisecond)

	meta = store.Get(guestKey)
	if meta.LastKnownType != "oci" {
		t.Errorf("Should ensure type 'oci' is preserved, got '%s'", meta.LastKnownType)
	}

	// Test persistence that shouldn't happen (no change) -> coverage of the if check
	// Should not trigger Set()
	persistGuestIdentity(store, guestKey, "VM 100", "oci")
}

func TestEnrichWithPersistedMetadata_Detail(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "enrich_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	store := config.NewGuestMetadataStore(tmpDir, nil)

	// 1. Add some metadata
	_ = store.Set("pve1:node1:100", &config.GuestMetadata{ID: "pve1:node1:100", LastKnownName: "PersistedVM", LastKnownType: "qemu"})
	_ = store.Set("pve1:node1:101", &config.GuestMetadata{ID: "pve1:node1:101", LastKnownName: "LiveVM", LastKnownType: "qemu"})
	_ = store.Set("invalid:key", &config.GuestMetadata{ID: "invalid:key", LastKnownName: "BadKey", LastKnownType: "qemu"})          // coverage for bad key
	_ = store.Set("pve1:node1:badid", &config.GuestMetadata{ID: "pve1:node1:badid", LastKnownName: "BadID", LastKnownType: "qemu"}) // coverage for atoi error

	// 2. Setup existing lookup
	lookup := make(map[string][]alerts.GuestLookup)
	// VM 101 is live
	lookup["101"] = []alerts.GuestLookup{
		{Name: "LiveVM", Instance: "pve1", Node: "node1", VMID: 101},
	}

	// 3. Run enrich
	enrichWithPersistedMetadata(store, lookup)

	// 4. Verify
	// 100 should be added
	if entries, ok := lookup["100"]; !ok || len(entries) != 1 {
		t.Error("Expected VM 100 to be enriched")
	} else {
		if entries[0].Name != "PersistedVM" {
			t.Errorf("Expected name PersistedVM, got %s", entries[0].Name)
		}
	}

	// 101 should not be duplicated (it was live)
	if len(lookup["101"]) != 1 {
		t.Error("VM 101 should not be duplicated")
	}
}
