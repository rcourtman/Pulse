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

	// Change persistence may block, but it must not retain either registry's
	// read lock. Readers still use the previously published generation.
	readDone := make(chan struct{})
	go func() {
		adapter.GetAll()
		close(readDone)
	}()
	select {
	case <-readDone:
	case <-time.After(time.Second):
		close(store.release)
		t.Fatal("registry read blocked behind change persistence")
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

func TestMonitorAdapterKeepsInMemoryManualLinksAcrossGenerations(t *testing.T) {
	now := time.Now().UTC()
	snapshot := models.StateSnapshot{
		Hosts: []models.Host{
			{ID: "host-app", Hostname: "app-guest", MachineID: "machine-app", Status: "online", LastSeen: now},
			{ID: "host-nas", Hostname: "nas-a-mgmt", MachineID: "machine-nas", Status: "online", LastSeen: now},
		},
		LastUpdate: now,
	}
	baseline := NewRegistry(nil)
	baseline.IngestSnapshot(snapshot)
	listed := baseline.List()
	if len(listed) != 2 {
		t.Fatalf("fixture hosts did not stay separate: %+v", listed)
	}
	link := ResourceLink{ResourceA: listed[0].ID, ResourceB: listed[1].ID, PrimaryID: listed[0].ID}
	adapter := NewMonitorAdapter(NewRegistryWithManualLinks([]ResourceLink{link}))
	for generation := 0; generation < 3; generation++ {
		adapter.PopulateFromSnapshot(snapshot)
		got := adapter.GetAll()
		if len(got) != 1 || got[0].ID != link.PrimaryID {
			t.Fatalf("generation %d lost the in-memory operator link: %+v", generation, got)
		}
		if links := adapter.ManualLinks(); len(links) != 1 || links[0] != link {
			t.Fatalf("generation %d changed the operator's link: %+v", generation, links)
		}
		if adapter.currentRegistry().store != nil {
			t.Fatal("a storeless generation acquired persistence")
		}
	}
}
