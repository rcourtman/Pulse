package alerts

import (
	"encoding/json"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestGuestSnapshotResourceTypeUsesCanonicalSystemContainer(t *testing.T) {
	containerSnapshot := guestSnapshotFromContainer(models.Container{})
	if got := containerSnapshot.resourceType(); got != "system-container" {
		t.Fatalf("expected container resourceType to be system-container, got %q", got)
	}

	vmSnapshot := guestSnapshotFromVM(models.VM{})
	if got := vmSnapshot.resourceType(); got != "vm" {
		t.Fatalf("expected VM resourceType to be vm, got %q", got)
	}
}

func TestEmptyGuestSnapshot_NormalizesCollections(t *testing.T) {
	snapshot := emptyGuestSnapshot()
	if snapshot.Disks == nil || snapshot.Tags == nil {
		t.Fatalf("expected empty guest snapshot slices to be initialized, got %#v", snapshot)
	}

	encoded, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("marshal guest snapshot: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("decode guest snapshot: %v", err)
	}

	for _, key := range []string{"Disks", "Tags"} {
		values, ok := payload[key].([]any)
		if !ok {
			t.Fatalf("expected %s to serialize as an array, got %T (%v)", key, payload[key], payload[key])
		}
		if len(values) != 0 {
			t.Fatalf("expected %s to serialize as an empty array, got %v", key, values)
		}
	}
}

func TestExtractGuestSnapshot_UnknownTypeReturnsCanonicalEmptySnapshot(t *testing.T) {
	snapshot, ok := extractGuestSnapshot(struct{}{})
	if ok {
		t.Fatal("expected unknown guest type extraction to fail")
	}
	if snapshot.Disks == nil || snapshot.Tags == nil {
		t.Fatalf("expected canonical empty guest snapshot on unknown type, got %#v", snapshot)
	}
}
