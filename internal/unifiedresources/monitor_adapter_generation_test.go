package unifiedresources

import (
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

type blockingChangeStore struct {
	*MemoryStore
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (s *blockingChangeStore) RecordChange(change ResourceChange) error {
	s.once.Do(func() {
		close(s.entered)
		<-s.release
	})
	return s.MemoryStore.RecordChange(change)
}

func TestMonitorAdapterSerializesSupplementalMutationAfterSnapshotPublication(t *testing.T) {
	store := &blockingChangeStore{
		MemoryStore: NewMemoryStore(),
		entered:     make(chan struct{}),
		release:     make(chan struct{}),
	}
	adapter := NewMonitorAdapter(NewRegistry(store))

	rebuildDone := make(chan struct{})
	go func() {
		adapter.PopulateFromSnapshot(models.StateSnapshot{
			LastUpdate: time.Now().UTC(),
			VMs: []models.VM{{
				ID:       "lab:node-a:101",
				VMID:     101,
				Name:     "database",
				Node:     "node-a",
				Instance: "lab",
				Status:   "running",
				LastSeen: time.Now().UTC(),
			}},
		})
		close(rebuildDone)
	}()

	select {
	case <-store.entered:
	case <-time.After(time.Second):
		t.Fatal("snapshot rebuild did not reach change publication")
	}

	supplementalDone := make(chan struct{})
	go func() {
		adapter.PopulateSupplementalRecords(SourceAgent, []IngestRecord{{
			SourceID: "host-supplemental",
			Resource: Resource{
				Type:     ResourceTypeAgent,
				Name:     "host-supplemental",
				Status:   StatusOnline,
				LastSeen: time.Now().UTC(),
			},
		}})
		close(supplementalDone)
	}()

	select {
	case <-supplementalDone:
		t.Fatal("supplemental mutation bypassed the in-flight snapshot generation")
	case <-time.After(20 * time.Millisecond):
	}

	close(store.release)
	select {
	case <-rebuildDone:
	case <-time.After(time.Second):
		t.Fatal("snapshot rebuild did not complete")
	}
	select {
	case <-supplementalDone:
	case <-time.After(time.Second):
		t.Fatal("supplemental mutation did not resume")
	}

	resources := adapter.GetAll()
	if len(resources) != 2 {
		t.Fatalf("final generation contains %d resources, want snapshot plus supplemental record: %+v", len(resources), resources)
	}
	var foundVM, foundSupplemental bool
	for _, resource := range resources {
		foundVM = foundVM || resource.Name == "database"
		foundSupplemental = foundSupplemental || resource.Name == "host-supplemental"
	}
	if !foundVM || !foundSupplemental {
		t.Fatalf("final generation lost a writer: %+v", resources)
	}
}
