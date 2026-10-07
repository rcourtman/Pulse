package memory

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

func TestIncidentIDsUniqueAcrossConcurrentBursts(t *testing.T) {
	for _, tc := range []struct {
		name, prefix string
		generate     func() string
	}{
		{"incidents", "inc-", generateIncidentID},
		{"events", "inc-evt-", generateIncidentEventID},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const workers, perWorker = 16, 256
			ids := make(chan string, workers*perWorker)
			var wg sync.WaitGroup
			for worker := 0; worker < workers; worker++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for n := 0; n < perWorker; n++ {
						ids <- tc.generate()
					}
				}()
			}
			wg.Wait()
			close(ids)
			seen := make(map[string]struct{}, workers*perWorker)
			for id := range ids {
				if !strings.HasPrefix(id, tc.prefix) || id == tc.prefix {
					t.Fatalf("invalid ID %q", id)
				}
				if _, duplicate := seen[id]; duplicate {
					t.Fatalf("duplicate ID across concurrent burst: %q", id)
				}
				seen[id] = struct{}{}
			}
			if len(seen) != workers*perWorker {
				t.Fatal("burst lost IDs")
			}
		})
	}
}

func TestIncidentIdentityIsolatedAcrossConcurrentStores(t *testing.T) {
	const tenants, occurrences = 8, 160 // More than the old 1,000-ID wrap per second.
	stores := make([]*IncidentStore, tenants)
	for tenant := range stores {
		stores[tenant] = NewIncidentStore(IncidentStoreConfig{MaxIncidents: occurrences, MaxEventsPerIncident: 4})
	}
	started := time.Now().UTC()
	var wg sync.WaitGroup
	for tenant, store := range stores {
		wg.Add(1)
		go func(tenant int, store *IncidentStore) {
			defer wg.Done()
			for n := 0; n < occurrences; n++ {
				// The same canonical alert identifiers in independent stores must not
				// make either the persisted occurrence or its event IDs global aliases.
				store.RecordAlertFired(&alerts.Alert{ID: fmt.Sprintf("shared-alert-%d", n), ResourceID: "shared-resource", ResourceName: "VM", Type: "cpu", Level: alerts.AlertLevelWarning, StartTime: started})
			}
		}(tenant, store)
	}
	wg.Wait()
	incidentIDs, eventIDs := map[string]struct{}{}, map[string]struct{}{}
	for tenant, store := range stores {
		for n := 0; n < occurrences; n++ {
			alertID := fmt.Sprintf("shared-alert-%d", n)
			incident := store.GetTimelineByAlertIdentifier(alertID)
			if incident == nil || len(incident.Events) != 1 {
				t.Fatalf("tenant %d occurrence %d absent", tenant, n)
			}
			if _, exists := incidentIDs[incident.ID]; exists {
				t.Fatalf("stores shared incident ID %q", incident.ID)
			}
			incidentIDs[incident.ID] = struct{}{}
			// By-ID operator notes must land on this exact occurrence, not another
			// one that happened to receive a wrapped identifier in the same store.
			note := fmt.Sprintf("tenant %d occurrence %d", tenant, n)
			if !store.RecordNote("", incident.ID, note, "operator") {
				t.Fatalf("note rejected for %q", incident.ID)
			}
			updated := store.GetTimelineByAlertIdentifier(alertID)
			if len(updated.Events) != 2 || updated.Events[1].Details["note"] != note {
				t.Fatalf("ID addressed wrong occurrence: tenant %d occurrence %d", tenant, n)
			}
			for _, event := range updated.Events {
				if _, exists := eventIDs[event.ID]; exists {
					t.Fatalf("stores shared event ID %q", event.ID)
				}
				eventIDs[event.ID] = struct{}{}
			}
		}
	}
	if len(incidentIDs) != tenants*occurrences || len(eventIDs) != 2*tenants*occurrences {
		t.Fatal("missing occurrence or event identity")
	}
}

func TestIncidentIdentityRetainsLegacyPersistedReferences(t *testing.T) {
	dir := t.TempDir()
	original := NewIncidentStore(IncidentStoreConfig{DataDir: dir})
	now := time.Now().UTC()
	const legacyID = "inc-20261007120000-42"
	const legacyEventID = "inc-evt-20261007120000-17"
	original.incidents = []*incidentShell{{ID: legacyID, AlertIdentifier: "legacy-alert", ResourceID: "legacy-resource", OpenedAt: now, Events: []IncidentEvent{{ID: legacyEventID, Type: IncidentEventAlertFired, Timestamp: now}}}}
	if err := original.saveToDisk(); err != nil {
		t.Fatal(err)
	}
	reloaded := NewIncidentStore(IncidentStoreConfig{DataDir: dir})
	if !reloaded.RecordNote("", legacyID, "legacy note", "operator") {
		t.Fatal("legacy ID no longer addresses its occurrence")
	}
	reloaded.flush()
	got := reloaded.GetTimelineByAlertIdentifier("legacy-alert")
	if got == nil || got.ID != legacyID || len(got.Events) != 2 || got.Events[0].ID != legacyEventID {
		t.Fatalf("legacy references rewritten: %#v", got)
	}
	if got.Events[1].ID == legacyEventID {
		t.Fatal("new event reused legacy event ID")
	}
	again := NewIncidentStore(IncidentStoreConfig{DataDir: dir})
	saved := again.GetTimelineByAlertIdentifier("legacy-alert")
	if saved == nil || saved.ID != legacyID || len(saved.Events) != 2 || saved.Events[1].ID != got.Events[1].ID {
		t.Fatal("new event reference changed after JSON reload")
	}
}
