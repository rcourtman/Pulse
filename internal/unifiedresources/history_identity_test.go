package unifiedresources

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/stretchr/testify/require"
)

func TestHistoryIdentityLegacyDockerReference(t *testing.T) {
	container := strings.Repeat("a", 64)
	for _, ref := range []string{"docker:host/" + container, "docker:container/" + container} {
		source, id, ok := legacyDockerHistoryIdentity(ref)
		require.True(t, ok)
		require.Equal(t, SourceSpecificID(ResourceTypeAppContainer, SourceDocker, source), id)
	}
	for _, ref := range []string{"docker:host/worker", "docker:host/" + container[:12], "docker:host/" + strings.ToUpper(container), "docker:/" + container, "docker:host/" + strings.Repeat("z", 64), "docker:host", "vm:host/" + container} {
		_, _, ok := legacyDockerHistoryIdentity(ref)
		require.False(t, ok, ref)
	}
}

// Exercise the actual scoped history read as the unrelated identity index grows.
// Timing is reported for qualification, without a machine-dependent pass threshold.
func BenchmarkHistoryIdentityQuery(b *testing.B) {
	for _, size := range []int{1, 20000} {
		b.Run(fmt.Sprintf("aliases-%d", size), func(b *testing.B) {
			store, err := NewSQLiteResourceStore(b.TempDir(), "benchmark")
			require.NoError(b, err)
			b.Cleanup(func() { require.NoError(b, store.Close()) })
			tx, err := store.db.Begin()
			require.NoError(b, err)
			for i := 0; i < size; i++ {
				_, err := tx.Exec(`INSERT INTO resource_history_aliases (source_id, canonical_id) VALUES (?, ?)`, fmt.Sprintf("legacy-%d", i), fmt.Sprintf("app-container-%d", i))
				require.NoError(b, err)
				require.NoError(b, recordChangeSQL(tx, ResourceChange{ID: fmt.Sprintf("event-%d", i), ResourceID: fmt.Sprintf("legacy-%d", i), ObservedAt: time.Now(), Kind: ChangeAlertFired}, store.resourceChangesHasTimestamp))
			}
			require.NoError(b, tx.Commit())
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				got, err := store.GetRecentChanges("app-container-0", time.Time{}, 50)
				if err != nil || len(got) != 1 {
					b.Fatalf("scoped history: count=%d err=%v", len(got), err)
				}
			}
		})
	}
}

func TestHistoryIdentityMigrationPreservesEventsAndAuthority(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSQLiteResourceStore(dir, "default")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	legacy := "docker:tower/" + strings.Repeat("b", 64)
	_, canonical, _ := legacyDockerHistoryIdentity(legacy)
	now := time.Now().UTC().Truncate(time.Second)
	event := ResourceChange{ID: "legacy-fired", ResourceID: legacy, ObservedAt: now, Kind: ChangeAlertFired, SourceType: SourcePulseDiff, Reason: "container unhealthy", Metadata: map[string]any{"alert_id": "health-test"}}
	require.NoError(t, store.RecordChange(event))
	require.NoError(t, store.SetResourceOperatorState(ResourceOperatorState{CanonicalID: legacy, NeverAutoRemediate: true, Note: "keep authority binding"}))
	_, err = store.db.Exec(`INSERT INTO action_audits (id, action_id, canonical_id, request_id, created_at, updated_at, state, request_json, plan_json)
		VALUES ('history-action', 'history-action', ?, 'request-1', ?, ?, 'pending', '{"binding":"original"}', '{}')`, legacy, now, now)
	require.NoError(t, err)
	require.NoError(t, store.Close())
	// No inventory survives this restart. The legacy full ID still identifies
	// the same container, and the original event is never rewritten.
	store, err = NewSQLiteResourceStore(dir, "default")
	require.NoError(t, err)
	for _, id := range []string{legacy, canonical} {
		got, err := store.GetRecentChanges(id, now.Add(-time.Minute), 10)
		require.NoError(t, err)
		require.Equal(t, []ResourceChange{event}, got)
		count, err := store.CountRecentChanges(id, now.Add(-time.Minute))
		require.NoError(t, err)
		require.Equal(t, 1, count)
		kinds, err := store.CountRecentChangesByKind(id, now.Add(-time.Minute))
		require.NoError(t, err)
		require.Equal(t, 1, kinds[ChangeAlertFired])
	}
	state, found, err := store.GetResourceOperatorState(legacy)
	require.NoError(t, err)
	require.True(t, found)
	require.True(t, state.NeverAutoRemediate)
	require.Equal(t, "keep authority binding", state.Note)
	_, found, err = store.GetResourceOperatorState(canonical)
	require.NoError(t, err)
	require.False(t, found)
	var actionID, request, eventID string
	require.NoError(t, store.db.QueryRow(`SELECT canonical_id, request_json FROM action_audits WHERE id = 'history-action'`).Scan(&actionID, &request))
	require.Equal(t, legacy, actionID)
	require.Equal(t, `{"binding":"original"}`, request)
	require.NoError(t, store.db.QueryRow(`SELECT canonical_id FROM resource_changes WHERE id = 'legacy-fired'`).Scan(&eventID))
	require.Equal(t, legacy, eventID)
}

func TestHistoryIdentitySeparateHandlesSeeBindingAndReplay(t *testing.T) {
	dir := t.TempDir()
	writer, err := NewSQLiteResourceStore(dir, "default")
	require.NoError(t, err)
	defer writer.Close()
	reader, err := NewSQLiteResourceStore(dir, "default")
	require.NoError(t, err)
	defer reader.Close()
	legacy := "docker:tower/" + strings.Repeat("c", 64)
	_, canonical, _ := legacyDockerHistoryIdentity(legacy)
	now := time.Now().UTC().Truncate(time.Second)
	old := ResourceChange{ID: "first", ResourceID: legacy, ObservedAt: now, Kind: ChangeAlertFired, SourceType: SourcePulseDiff}
	require.NoError(t, writer.RecordChange(old))
	got, err := reader.GetRecentChanges(canonical, time.Time{}, 10)
	require.NoError(t, err)
	require.Empty(t, got)
	replayed := old
	replayed.ResourceID = canonical
	require.NoError(t, writer.RecordChangeWithSourceIdentity(replayed, legacy))
	second := ResourceChange{ID: "second", ResourceID: canonical, ObservedAt: now.Add(time.Second), Kind: ChangeAlertResolved, SourceType: SourcePulseDiff}
	require.NoError(t, writer.RecordChangeWithSourceIdentity(second, legacy))
	require.NoError(t, writer.RecordChangeWithSourceIdentity(second, legacy))
	for _, id := range []string{legacy, canonical} {
		got, err := reader.GetRecentChanges(id, time.Time{}, 10)
		require.NoError(t, err)
		require.Equal(t, []ResourceChange{second, old}, got)
	}
	// The same source identifier in a different organization cannot see this binding.
	other, err := NewSQLiteResourceStore(dir, "other-org")
	require.NoError(t, err)
	defer other.Close()
	got, err = other.GetRecentChanges(canonical, time.Time{}, 10)
	require.NoError(t, err)
	require.Empty(t, got)
}

func TestHistoryIdentityMonitorAdapterUsesExactContainerIdentity(t *testing.T) {
	store := NewMemoryStore()
	container := strings.Repeat("d", 64)
	host := models.DockerHost{ID: "tower", Hostname: "tower", LastSeen: time.Now(), Containers: []models.DockerContainer{{ID: container, Name: "worker", State: "running"}}}
	registry := NewRegistry(store)
	registry.IngestSnapshot(models.StateSnapshot{DockerHosts: []models.DockerHost{host}})
	adapter := NewMonitorAdapter(registry)
	legacy := "docker:tower/" + container
	_, canonical, _ := legacyDockerHistoryIdentity(legacy)
	for i, ref := range []string{legacy, "docker:tower/worker", "docker:tower/" + container[:12]} {
		require.NoError(t, adapter.RecordChange(ResourceChange{ID: ref, ResourceID: ref, Kind: ChangeAlertFired, ObservedAt: time.Now().Add(time.Duration(i) * time.Second)}))
	}
	got, err := store.GetRecentChanges(canonical, time.Time{}, 10)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, canonical, got[0].ResourceID)
	// A retained authoritative binding survives loss of the registry.
	require.NoError(t, store.RecordChangeWithSourceIdentity(ResourceChange{ID: "binding", ResourceID: "app-container-retained", ObservedAt: time.Now()}, legacy))
	removed := NewMonitorAdapter(NewRegistry(store))
	require.NoError(t, removed.RecordChange(ResourceChange{ID: "after-removal", ResourceID: legacy, Kind: ChangeAlertResolved, ObservedAt: time.Now()}))
	got, err = store.GetRecentChanges("app-container-retained", time.Time{}, 10)
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, "app-container-retained", got[0].ResourceID)
}

func TestHistoryIdentityRetentionAndUnavailableLookup(t *testing.T) {
	store, err := NewSQLiteResourceStore(t.TempDir(), "default")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	legacy := "docker:tower/" + strings.Repeat("e", 64)
	_, canonical, _ := legacyDockerHistoryIdentity(legacy)
	require.NoError(t, store.RecordChangeWithSourceIdentity(ResourceChange{ID: "expired", ResourceID: canonical, ObservedAt: time.Now().Add(-2 * resourceChangesRetention)}, legacy))
	store.pruneOldRecords()
	_, found, err := store.ResolveHistorySourceIdentity(legacy)
	require.NoError(t, err)
	require.False(t, found)
	require.NoError(t, store.Close())
	_, err = store.GetRecentChanges(canonical, time.Time{}, 10)
	require.Error(t, err)
}

func proxmoxHistoryIdentitySnapshot(now time.Time) models.StateSnapshot {
	return models.StateSnapshot{
		Nodes:      []models.Node{{ID: "lab-pve1", Name: "pve1", Instance: "lab", Host: "https://pve1.lab:8006", Status: "online", LinkedAgentID: "host-pve1", LastSeen: now}},
		Hosts:      []models.Host{{ID: "host-pve1", Hostname: "pve1", LinkedNodeID: "lab-pve1", MachineID: "0123456789abcdef", Status: "online", LastSeen: now}},
		VMs:        []models.VM{{ID: "lab:pve1:101", VMID: 101, Name: "web", Node: "pve1", Instance: "lab", Status: "running", Type: "qemu", LastSeen: now}},
		Containers: []models.Container{{ID: "lab-pve1-102", VMID: 102, Name: "dns", Node: "pve1", Instance: "lab", Status: "running", Type: "lxc", LastSeen: now}},
	}
}

func historyIdentityResourceID(t *testing.T, registry *ResourceRegistry, resourceType ResourceType, name string) string {
	t.Helper()
	for _, resource := range registry.List() {
		if resource.Type == resourceType && resource.Name == name {
			return resource.ID
		}
	}
	t.Fatalf("no %s resource named %q", resourceType, name)
	return ""
}

// Proxmox alerts carry the source-native node and guest IDs. Their lifecycle
// must land in the history the drawer, facets and assistant read by canonical ID.
func TestHistoryIdentityMonitorAdapterResolvesProxmoxAlertReferences(t *testing.T) {
	store, err := NewSQLiteResourceStore(t.TempDir(), "default")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	now := time.Now().UTC().Truncate(time.Second)
	snapshot := proxmoxHistoryIdentitySnapshot(now)
	// A guest named like the system alert reference must not capture it.
	snapshot.VMs = append(snapshot.VMs, models.VM{ID: "lab:pve1:120", VMID: 120, Name: "pulse-system", Node: "pve1", Instance: "lab", Status: "running", Type: "qemu", LastSeen: now})
	registry := NewRegistry(store)
	registry.IngestSnapshot(snapshot)
	adapter := NewMonitorAdapter(registry)
	nodeID := historyIdentityResourceID(t, registry, ResourceTypeAgent, "pve1")
	vmID := historyIdentityResourceID(t, registry, ResourceTypeVM, "web")
	ctID := historyIdentityResourceID(t, registry, ResourceTypeSystemContainer, "dns")

	// "lab:pve2:101" names a node other than the guest's current one, as an
	// alert raised before a live migration does; it still names the guest.
	refs := []string{"lab-pve1", "agent:host-pve1", "lab:pve2:101", "lab-pve1-102", "pulse-system", "web"}
	for i, ref := range refs {
		require.NoError(t, adapter.RecordChange(ResourceChange{ID: "fired-" + ref, ResourceID: ref, Kind: ChangeAlertFired, SourceType: SourceHeuristic, ObservedAt: now.Add(time.Duration(i) * time.Second)}))
	}
	for canonicalID, want := range map[string][]string{nodeID: {"agent:host-pve1", "lab-pve1"}, vmID: {"lab:pve2:101"}, ctID: {"lab-pve1-102"}} {
		got, err := store.GetRecentChanges(canonicalID, time.Time{}, 10)
		require.NoError(t, err)
		require.Len(t, got, len(want), canonicalID)
		for i, ref := range want {
			require.Equal(t, "fired-"+ref, got[i].ID)
			require.Equal(t, canonicalID, got[i].ResourceID)
			bound, found, err := store.ResolveHistorySourceIdentity(ref)
			require.NoError(t, err)
			require.True(t, found, ref)
			require.Equal(t, canonicalID, bound)
		}
		kinds, err := store.CountRecentChangesByKind(canonicalID, time.Time{})
		require.NoError(t, err)
		require.Equal(t, len(want), kinds[ChangeAlertFired], canonicalID)
	}
	// Names are display identities: they keep their own history and no binding.
	for _, ref := range []string{"pulse-system", "web"} {
		got, err := store.GetRecentChanges(ref, time.Time{}, 10)
		require.NoError(t, err)
		require.Len(t, got, 1, ref)
		_, found, err := store.ResolveHistorySourceIdentity(ref)
		require.NoError(t, err)
		require.False(t, found, ref)
	}
	got, err := store.GetRecentChanges(historyIdentityResourceID(t, registry, ResourceTypeVM, "pulse-system"), time.Time{}, 10)
	require.NoError(t, err)
	require.Empty(t, got)

	// Recovery after the node left inventory still reaches the same history.
	removed := NewMonitorAdapter(NewRegistry(store))
	require.NoError(t, removed.RecordChange(ResourceChange{ID: "resolved-lab-pve1", ResourceID: "lab-pve1", Kind: ChangeAlertResolved, SourceType: SourceHeuristic, ObservedAt: now.Add(time.Minute)}))
	got, err = store.GetRecentChanges(nodeID, time.Time{}, 10)
	require.NoError(t, err)
	require.Len(t, got, 3)
	require.Equal(t, "resolved-lab-pve1", got[0].ID)
	require.Equal(t, nodeID, got[0].ResourceID)

	// When two resources answer to the reference, neither the registry nor the
	// retained binding may choose: the event keeps its own reference.
	registry.mu.Lock()
	registry.bySource[SourcePBS] = map[string]string{"lab-pve1": vmID}
	registry.mu.Unlock()
	require.NoError(t, adapter.RecordChange(ResourceChange{ID: "ambiguous-lab-pve1", ResourceID: "lab-pve1", Kind: ChangeAlertFired, SourceType: SourceHeuristic, ObservedAt: now.Add(2 * time.Minute)}))
	var recordedAs string
	require.NoError(t, store.db.QueryRow(`SELECT canonical_id FROM resource_changes WHERE id = 'ambiguous-lab-pve1'`).Scan(&recordedAs))
	require.Equal(t, "lab-pve1", recordedAs)
	bound, _, err := store.ResolveHistorySourceIdentity("lab-pve1")
	require.NoError(t, err)
	require.Equal(t, nodeID, bound, "an ambiguous reference never rebinds")
	got, err = store.GetRecentChanges(vmID, time.Time{}, 10)
	require.NoError(t, err)
	require.Len(t, got, 1)
}

// Alert rows recorded under source-native references before they resolved join
// canonical history once a registry generation can name their resource: rows
// journaled before this process started, and rows written before inventory
// knew the resource. The rows themselves are never rewritten.
func TestHistoryIdentityBindsLegacyAlertRowsFromRegistryGenerations(t *testing.T) {
	store, err := NewSQLiteResourceStore(t.TempDir(), "default")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	now := time.Now().UTC().Truncate(time.Second)
	for i, ref := range []string{"lab-pve1", "lab-pve1-102", "lab:pve1:103", "lab:pve1:999", "docker:tower/worker"} {
		require.NoError(t, store.RecordChange(ResourceChange{ID: "legacy-" + ref, ResourceID: ref, Kind: ChangeAlertFired, SourceType: SourceHeuristic, ObservedAt: now.Add(time.Duration(i) * time.Second)}))
	}
	adapter := NewMonitorAdapter(NewRegistry(store))
	snapshot := proxmoxHistoryIdentitySnapshot(now)
	adapter.PopulateFromSnapshot(snapshot)
	registry := adapter.currentRegistry()
	for ref, canonicalID := range map[string]string{
		"lab-pve1":     historyIdentityResourceID(t, registry, ResourceTypeAgent, "pve1"),
		"lab-pve1-102": historyIdentityResourceID(t, registry, ResourceTypeSystemContainer, "dns"),
	} {
		got, err := store.GetRecentChangesFiltered(canonicalID, time.Time{}, 10, ResourceChangeFilters{Kinds: []ChangeKind{ChangeAlertFired}})
		require.NoError(t, err)
		require.Len(t, got, 1, ref)
		require.Equal(t, "legacy-"+ref, got[0].ID)
		require.Equal(t, ref, got[0].ResourceID)
	}

	// An alert raised after the journal scan, before inventory names its guest.
	require.NoError(t, adapter.RecordChange(ResourceChange{ID: "early-lab:pve1:104", ResourceID: "lab:pve1:104", Kind: ChangeAlertFired, SourceType: SourceHeuristic, ObservedAt: now.Add(time.Minute)}))
	for _, ref := range []string{"lab:pve1:103", "lab:pve1:104"} {
		_, found, err := store.ResolveHistorySourceIdentity(ref)
		require.NoError(t, err)
		require.False(t, found, ref)
	}
	// Both guests join inventory in a later generation and are bound then.
	snapshot.VMs = append(snapshot.VMs,
		models.VM{ID: "lab:pve1:103", VMID: 103, Name: "late", Node: "pve1", Instance: "lab", Status: "running", Type: "qemu", LastSeen: now},
		models.VM{ID: "lab:pve1:104", VMID: 104, Name: "early", Node: "pve1", Instance: "lab", Status: "running", Type: "qemu", LastSeen: now})
	adapter.PopulateFromSnapshot(snapshot)
	for name, id := range map[string]string{"late": "legacy-lab:pve1:103", "early": "early-lab:pve1:104"} {
		got, err := store.GetRecentChangesFiltered(historyIdentityResourceID(t, adapter.currentRegistry(), ResourceTypeVM, name), time.Time{}, 10, ResourceChangeFilters{Kinds: []ChangeKind{ChangeAlertFired}})
		require.NoError(t, err)
		require.Len(t, got, 1, name)
		require.Equal(t, id, got[0].ID)
	}

	// Unknown guests and Docker names stay unbound, and retries end at the deadline.
	for _, ref := range []string{"lab:pve1:999", "docker:tower/worker"} {
		_, found, err := store.ResolveHistorySourceIdentity(ref)
		require.NoError(t, err)
		require.False(t, found, ref)
	}
	require.Len(t, adapter.legacyHistory.pending, 1)
	require.Contains(t, adapter.legacyHistory.pending, "lab:pve1:999")
	adapter.legacyHistory.pending["lab:pve1:999"] = time.Now().Add(-time.Second)
	adapter.PopulateFromSnapshot(snapshot)
	require.Empty(t, adapter.legacyHistory.pending)
	refs, err := store.unboundHistoryReferences()
	require.NoError(t, err)
	require.Equal(t, []string{"lab:pve1:999"}, refs)
}
