package unifiedresources

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func readRefreshSnapshot(name string) models.StateSnapshot {
	return models.StateSnapshot{
		LastUpdate: time.Now().UTC(),
		VMs: []models.VM{{
			ID:       "lab:node-a:101",
			VMID:     101,
			Name:     name,
			Node:     "node-a",
			Instance: "lab",
			Status:   "running",
			LastSeen: time.Now().UTC(),
		}},
	}
}

func TestTryReplaceRegistryForReadColdStartBlocksAndBuilds(t *testing.T) {
	adapter := NewMonitorAdapter(NewRegistry(NewMemoryStore()))

	supplied := false
	rebuilt := adapter.TryReplaceRegistryForRead(readRefreshSnapshot("database"), time.Minute, func() map[DataSource][]IngestRecord {
		supplied = true
		return nil
	})
	if !rebuilt {
		t.Fatal("cold-start read refresh must build the first generation")
	}
	if !supplied {
		t.Fatal("committed rebuild must materialize the supplemental payload")
	}
	if adapter.LastRebuiltAt().IsZero() {
		t.Fatal("rebuild must publish a generation timestamp")
	}
	if len(adapter.GetAll()) != 1 {
		t.Fatalf("expected 1 resource after cold-start build, got %d", len(adapter.GetAll()))
	}
}

func TestTryReplaceRegistryForReadSkipsFreshGeneration(t *testing.T) {
	adapter := NewMonitorAdapter(NewRegistry(NewMemoryStore()))
	adapter.PopulateFromSnapshot(readRefreshSnapshot("database"))

	rebuilt := adapter.TryReplaceRegistryForRead(readRefreshSnapshot("renamed"), time.Minute, func() map[DataSource][]IngestRecord {
		t.Fatal("skipped refresh must not drain the supplemental payload")
		return nil
	})
	if rebuilt {
		t.Fatal("read refresh must skip while the generation is fresh")
	}
	if got := adapter.GetAll(); len(got) != 1 || got[0].Name != "database" {
		t.Fatalf("fresh generation must be served unchanged, got %+v", got)
	}
}

func TestTryReplaceRegistryForReadRebuildsStaleGeneration(t *testing.T) {
	adapter := NewMonitorAdapter(NewRegistry(NewMemoryStore()))
	adapter.PopulateFromSnapshot(readRefreshSnapshot("database"))

	time.Sleep(20 * time.Millisecond)
	rebuilt := adapter.TryReplaceRegistryForRead(readRefreshSnapshot("renamed"), 10*time.Millisecond, nil)
	if !rebuilt {
		t.Fatal("read refresh must rebuild once the generation is older than maxAge")
	}
	if got := adapter.GetAll(); len(got) != 1 || got[0].Name != "renamed" {
		t.Fatalf("stale refresh must publish the new snapshot, got %+v", got)
	}
}

func TestTryReplaceRegistryForReadDoesNotQueueBehindInflightRebuild(t *testing.T) {
	store := &blockingChangeStore{
		MemoryStore: NewMemoryStore(),
		entered:     make(chan struct{}),
		release:     make(chan struct{}),
	}
	adapter := NewMonitorAdapter(NewRegistry(store))
	// Publish a baseline generation so the read path is past cold start. An
	// empty snapshot records no changes, leaving the store helper armed for
	// the ingest rebuild below.
	adapter.PopulateFromSnapshot(models.StateSnapshot{LastUpdate: time.Now().UTC()})
	if adapter.LastRebuiltAt().IsZero() {
		t.Fatal("baseline rebuild must publish a generation")
	}

	ingestDone := make(chan struct{})
	go func() {
		adapter.PopulateFromSnapshot(readRefreshSnapshot("database"))
		close(ingestDone)
	}()

	select {
	case <-store.entered:
	case <-time.After(time.Second):
		t.Fatal("ingest rebuild did not reach change publication")
	}

	readDone := make(chan bool, 1)
	go func() {
		readDone <- adapter.TryReplaceRegistryForRead(readRefreshSnapshot("read"), time.Nanosecond, func() map[DataSource][]IngestRecord {
			return nil
		})
	}()

	select {
	case rebuilt := <-readDone:
		if rebuilt {
			t.Fatal("read refresh must not rebuild while an ingest rebuild is in flight")
		}
	case <-time.After(time.Second):
		t.Fatal("read refresh queued behind the in-flight ingest rebuild")
	}

	close(store.release)
	select {
	case <-ingestDone:
	case <-time.After(time.Second):
		t.Fatal("ingest rebuild did not complete after release")
	}
}
