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

func requireHistoryReferenceBindings(t *testing.T, store *SQLiteResourceStore, bound map[string]string, unbound []string) {
	t.Helper()
	byResource := make(map[string][]string)
	for ref, canonicalID := range bound {
		byResource[canonicalID] = append(byResource[canonicalID], "fired-"+ref)
		id, found, err := store.ResolveHistorySourceIdentity(ref)
		require.NoError(t, err)
		require.True(t, found, ref)
		require.Equal(t, canonicalID, id, ref)
	}
	for canonicalID, want := range byResource {
		got, err := store.GetRecentChangesFiltered(canonicalID, time.Time{}, 50, ResourceChangeFilters{Kinds: []ChangeKind{ChangeAlertFired}})
		require.NoError(t, err)
		ids := make([]string, 0, len(got))
		for _, change := range got {
			ids = append(ids, change.ID)
			// A row journaled before its binding keeps its recorded reference.
			require.Contains(t, []string{canonicalID, strings.TrimPrefix(change.ID, "fired-")}, change.ResourceID, change.ID)
		}
		require.ElementsMatch(t, want, ids, canonicalID)
	}
	for _, ref := range unbound {
		_, found, err := store.ResolveHistorySourceIdentity(ref)
		require.NoError(t, err)
		require.False(t, found, ref)
		got, err := store.GetRecentChanges(ref, time.Time{}, 10)
		require.NoError(t, err)
		require.Len(t, got, 1, ref)
		require.Equal(t, ref, got[0].ResourceID, ref)
	}
}

// Docker host alerts reference the host's source ID ("docker:<host ID>") and
// Swarm service alerts the service ID ("docker:<host ID>/service/<ID>") from
// every manager that reports the service. Both join the canonical host and
// service history, also for a host merged into its agent's machine. Hostnames,
// service names, container names and shortened container IDs never bind, not
// even to the history of what they name, and a cluster with a service that
// lacks an ID, whose alerts carry its normalized name, binds no service.
func TestHistoryIdentityMonitorAdapterResolvesDockerHostAndServiceReferences(t *testing.T) {
	store, err := NewSQLiteResourceStore(t.TempDir(), "default")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	now := time.Now().UTC().Truncate(time.Second)
	container := strings.Repeat("f", 64)
	serviceID := "x7k2m9q4w1e8r5t3y6u0i2o4p"
	otherServiceID := "q1w2e3r4t5y6u7i8o9p0a1s2d"
	manager := func(id, hostname, cluster string, services ...models.DockerService) models.DockerHost {
		return models.DockerHost{ID: id, Hostname: hostname, Status: "online", LastSeen: now, Services: services,
			Swarm: &models.DockerSwarmInfo{NodeID: id + "-node", NodeRole: "manager", LocalState: "active", ControlAvailable: true, ClusterID: cluster}}
	}
	web := models.DockerService{ID: serviceID, Name: "web"}
	// hostA runs on a machine whose agent also reports, so the two merge.
	hostA := manager("dh-7f3a", "tower", "swarm-1", web)
	hostA.AgentID, hostA.MachineID = "host-tower", "0123456789abcdef"
	hostA.Containers = []models.DockerContainer{{ID: container, Name: "worker", State: "running"}}
	hostB := manager("dh-91c0", "rack", "swarm-1", web)
	// A container without an ID, named like its host's ID.
	hostB.Containers = []models.DockerContainer{{Name: "dh-91c0", State: "running"}}
	// In swarm-2 a service without an ID is named like another service's ID.
	hostC := manager("dh-c4d2", "edge", "swarm-2", models.DockerService{ID: otherServiceID, Name: "api"}, models.DockerService{Name: strings.ToUpper(otherServiceID)})
	// Rows journaled before this process started are bound by a later generation.
	legacy := []string{"docker:dh-91c0", "docker:dh-91c0/service/" + serviceID, "docker:dh-7f3a/worker"}
	for i, ref := range legacy {
		require.NoError(t, store.RecordChange(ResourceChange{ID: "fired-" + ref, ResourceID: ref, Kind: ChangeAlertFired, SourceType: SourceHeuristic, ObservedAt: now.Add(time.Duration(i) * time.Second)}))
	}
	adapter := NewMonitorAdapter(NewRegistry(store))
	adapter.PopulateFromSnapshot(models.StateSnapshot{
		Hosts:       []models.Host{{ID: "host-tower", Hostname: "tower", MachineID: "0123456789abcdef", Status: "online", LastSeen: now}},
		DockerHosts: []models.DockerHost{hostA, hostB, hostC},
	})
	registry := adapter.currentRegistry()
	hostAID := registry.sourceResourceID(SourceDocker, "dh-7f3a")
	hostBID := registry.sourceResourceID(SourceDocker, "dh-91c0")
	serviceResourceID := historyIdentityResourceID(t, registry, ResourceTypeDockerService, "web")
	apiID := historyIdentityResourceID(t, registry, ResourceTypeDockerService, "api")
	containerID := registry.sourceResourceID(SourceDocker, "dh-7f3a/container/"+container)
	require.Equal(t, registry.sourceResourceID(SourceAgent, "host-tower"), hostAID, "the Docker host merges into its agent's machine")
	require.NotEqual(t, hostAID, hostBID)
	require.NotEmpty(t, containerID)

	bound := map[string]string{
		"docker:dh-91c0":                      hostBID,
		"docker:dh-91c0/service/" + serviceID: serviceResourceID,
		"docker:dh-7f3a":                      hostAID,
		"docker:dh-7f3a/service/" + serviceID: serviceResourceID,
		// A service without an ID in the same cluster no longer shares this
		// reference, so it binds.
		"docker:dh-c4d2/service/" + otherServiceID: apiID,
	}
	unbound := []string{
		"docker:tower",                                  // hostname
		"docker:dh-7f3a/service/web",                    // service name
		"docker:dh-c4d2/service/name:" + otherServiceID, // service without an ID
		"docker-service:name:web",                       // hostless service name
		"docker:dh-7f3a/worker",                         // container name
		"docker:dh-7f3a/" + container[:12],              // shortened container ID
		"docker:dh-91c0/name:dh-91c0",                   // container without an ID
		"docker:unknown",
	}
	ids := []string{"docker:dh-7f3a", "docker:dh-7f3a/service/" + serviceID, "docker:dh-c4d2/service/" + otherServiceID}
	for i, ref := range append(ids, unbound...) {
		if ref == "docker:dh-7f3a/worker" {
			continue // journaled above
		}
		require.NoError(t, adapter.RecordChange(ResourceChange{ID: "fired-" + ref, ResourceID: ref, Kind: ChangeAlertFired, SourceType: SourceHeuristic, ObservedAt: now.Add(time.Minute + time.Duration(i)*time.Second)}))
	}
	requireHistoryReferenceBindings(t, store, bound, unbound)
	// Names and shortened IDs are never retried against later inventory.
	require.Contains(t, adapter.legacyHistory.pending, "docker:tower")
	for _, ref := range []string{"docker:dh-7f3a/worker", "docker:dh-7f3a/" + container[:12], "docker-service:name:web",
		"docker:dh-91c0/name:dh-91c0", "docker:dh-c4d2/service/name:" + otherServiceID} {
		require.NotContains(t, adapter.legacyHistory.pending, ref)
	}
	got, err := store.GetRecentChangesFiltered(containerID, time.Time{}, 10, ResourceChangeFilters{Kinds: []ChangeKind{ChangeAlertFired}})
	require.NoError(t, err)
	require.Empty(t, got, "a container name or short ID never joins the container's history")
	// The service's ID reference follows its binding once the service has left
	// inventory, while the service without an ID is still there; the name
	// reference stays its own.
	hostC.Services = hostC.Services[1:]
	adapter.PopulateFromSnapshot(models.StateSnapshot{
		Hosts:       []models.Host{{ID: "host-tower", Hostname: "tower", MachineID: "0123456789abcdef", Status: "online", LastSeen: now}},
		DockerHosts: []models.DockerHost{hostA, hostB, hostC},
	})
	nameRef := "docker:dh-c4d2/service/name:" + otherServiceID
	for id, ref := range map[string]string{"api-removed": "docker:dh-c4d2/service/" + otherServiceID, "name-removed": nameRef} {
		require.NoError(t, adapter.RecordChange(ResourceChange{ID: id, ResourceID: ref, Kind: ChangeAlertResolved, SourceType: SourceHeuristic, ObservedAt: now.Add(3 * time.Minute)}))
	}
	for id, want := range map[string]string{"api-removed": apiID, "name-removed": nameRef} {
		var recordedAs string
		require.NoError(t, store.db.QueryRow(`SELECT canonical_id FROM resource_changes WHERE id = ?`, id).Scan(&recordedAs))
		require.Equal(t, want, recordedAs, id)
	}

	// After the service leaves inventory its alerts follow the retained binding;
	// a container name still binds nothing.
	removed := NewMonitorAdapter(NewRegistry(store))
	for _, ref := range []string{"docker:dh-91c0/service/" + serviceID, "docker:dh-7f3a/worker"} {
		require.NoError(t, removed.RecordChange(ResourceChange{ID: "resolved-" + ref, ResourceID: ref, Kind: ChangeAlertResolved, SourceType: SourceHeuristic, ObservedAt: now.Add(time.Hour)}))
	}
	got, err = store.GetRecentChangesFiltered(serviceResourceID, time.Time{}, 10, ResourceChangeFilters{Kinds: []ChangeKind{ChangeAlertFired, ChangeAlertResolved}})
	require.NoError(t, err)
	require.Len(t, got, 3)
	require.Equal(t, "resolved-docker:dh-91c0/service/"+serviceID, got[0].ID)
	require.Equal(t, serviceResourceID, got[0].ResourceID)
	got, err = store.GetRecentChanges("docker:dh-7f3a/worker", time.Time{}, 10)
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, "docker:dh-7f3a/worker", got[0].ResourceID)
}

// Sub-resource alerts (ZFS pools and devices, host filesystems, disks, RAID
// arrays, sensors, the Unraid array) name no resource of their own. They join
// the history of the owner their reference carries a durable ID for: the
// storage, the host, or the Unraid array storage. Device labels never bind a
// physical disk, and an owner of the wrong type or a name binds nothing.
func TestHistoryIdentityMonitorAdapterResolvesSubResourceReferences(t *testing.T) {
	store, err := NewSQLiteResourceStore(t.TempDir(), "default")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	now := time.Now().UTC().Truncate(time.Second)
	snapshot := proxmoxHistoryIdentitySnapshot(now)
	snapshot.Storage = []models.Storage{{ID: "lab-pve1-local-zfs", Name: "local-zfs", Node: "pve1", Instance: "lab", Type: "zfspool", Status: "available", Total: 100, Used: 10, LastSeen: now}}
	snapshot.Hosts[0].Sensors.SMART = []models.HostDiskSMART{{Device: "sda", Serial: "S3Z1NB0K", Model: "Samsung SSD", Health: "PASSED", Temperature: 34}}
	snapshot.Hosts = append(snapshot.Hosts, models.Host{ID: "host-nas", Hostname: "nas", MachineID: "fedcba9876543210", Status: "online", LastSeen: now,
		Unraid: &models.HostUnraidStorage{ArrayStarted: true, ArrayState: "STARTED", Disks: []models.HostUnraidDisk{{Name: "disk1", Device: "sdb", Role: "data", Status: "DISK_OK", Serial: "WD-1"}}}})
	adapter := NewMonitorAdapter(NewRegistry(store))
	adapter.PopulateFromSnapshot(snapshot)
	registry := adapter.currentRegistry()
	nodeID := historyIdentityResourceID(t, registry, ResourceTypeAgent, "pve1")
	storageID := registry.sourceResourceID(SourceProxmox, "lab-pve1-local-zfs")
	unraidID := registry.sourceResourceID(SourceAgent, "host-nas/storage:unraid-array")
	diskID := registry.sourceResourceID(SourceAgent, HostSMARTDiskSourceID(snapshot.Hosts[0], snapshot.Hosts[0].Sensors.SMART[0]))
	require.NotEmpty(t, storageID)
	require.NotEmpty(t, unraidID)
	require.NotEmpty(t, diskID)

	bound := map[string]string{
		"lab-pve1-local-zfs/zfs-pool:local-zfs":             storageID,
		"lab-pve1-local-zfs/zfs-pool:local-zfs/device:sda2": storageID,
		"agent:host-pve1/disk:var":                          nodeID,
		"agent:host-pve1/disk:sda":                          nodeID,
		"agent:host-pve1/disk_temp:sda":                     nodeID,
		"agent:host-pve1/raid:md0":                          nodeID,
		"agent:host-pve1/custom:fan1":                       nodeID,
		"agent:host-nas/storage:unraid-array":               unraidID,
	}
	unbound := []string{
		"lab-pve1/zfs-pool:local-zfs",  // the owner part names a node, not a storage
		"local-zfs/zfs-pool:local-zfs", // a storage name, not its ID
		"agent:pve1/disk:var",          // a hostname, not the host ID
		"agent:host-pve1/service:sshd", // not a sub-resource shape any producer emits
		"agent:host-pve1/disk:",
	}
	refs := append([]string{}, unbound...)
	for ref := range bound {
		refs = append(refs, ref)
	}
	for i, ref := range refs {
		require.NoError(t, adapter.RecordChange(ResourceChange{ID: "fired-" + ref, ResourceID: ref, Kind: ChangeAlertFired, SourceType: SourceHeuristic, ObservedAt: now.Add(time.Duration(i) * time.Second)}))
	}
	requireHistoryReferenceBindings(t, store, bound, unbound)
	got, err := store.GetRecentChangesFiltered(diskID, time.Time{}, 10, ResourceChangeFilters{Kinds: []ChangeKind{ChangeAlertFired}})
	require.NoError(t, err)
	require.Empty(t, got, "a kernel device label never binds a physical disk")

	// Recovery after the storage left inventory follows the retained binding.
	pool := "lab-pve1-local-zfs/zfs-pool:local-zfs"
	removed := NewMonitorAdapter(NewRegistry(store))
	require.NoError(t, removed.RecordChange(ResourceChange{ID: "resolved-" + pool, ResourceID: pool, Kind: ChangeAlertResolved, SourceType: SourceHeuristic, ObservedAt: now.Add(time.Hour)}))
	got, err = store.GetRecentChangesFiltered(storageID, time.Time{}, 10, ResourceChangeFilters{Kinds: []ChangeKind{ChangeAlertResolved}})
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, storageID, got[0].ResourceID)

	// An owner reference that two resources answer to, or that names a resource
	// of another type, binds nothing and must not follow the retained binding.
	vmID := historyIdentityResourceID(t, registry, ResourceTypeVM, "web")
	for _, conflict := range []struct {
		name   string
		source DataSource
	}{{"ambiguous", SourcePBS}, {"wrong-type", SourceProxmox}} {
		registry.mu.Lock()
		delete(registry.bySource[SourcePBS], "lab-pve1-local-zfs")
		if registry.bySource[conflict.source] == nil {
			registry.bySource[conflict.source] = make(map[string]string)
		}
		registry.bySource[conflict.source]["lab-pve1-local-zfs"] = vmID
		registry.mu.Unlock()
		id := conflict.name + "-" + pool
		require.NoError(t, adapter.RecordChange(ResourceChange{ID: id, ResourceID: pool, Kind: ChangeAlertFired, SourceType: SourceHeuristic, ObservedAt: now.Add(2 * time.Hour)}))
		var recordedAs string
		require.NoError(t, store.db.QueryRow(`SELECT canonical_id FROM resource_changes WHERE id = ?`, id).Scan(&recordedAs))
		require.Equal(t, pool, recordedAs, conflict.name)
		bound, _, err := store.ResolveHistorySourceIdentity(pool)
		require.NoError(t, err)
		require.Equal(t, storageID, bound, conflict.name)
	}
	// A conflicting reference without a binding is retried, like one found at
	// startup, and binds once a generation names its owner unambiguously.
	device := "lab-pve1-local-zfs/zfs-pool:local-zfs/device:sdc2"
	require.NoError(t, adapter.RecordChange(ResourceChange{ID: "fired-" + device, ResourceID: device, Kind: ChangeAlertFired, SourceType: SourceHeuristic, ObservedAt: now.Add(3 * time.Hour)}))
	require.Contains(t, adapter.legacyHistory.pending, device)
	adapter.PopulateFromSnapshot(snapshot)
	deviceBinding, found, err := store.ResolveHistorySourceIdentity(device)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, storageID, deviceBinding)
}

// PVE disk health and wearout alerts reference the disk by instance, node and
// device path. Operator mutes resolve that reference to the disk at the path
// (#2112), but history never binds it: the path passes to a replacement disk
// in the same slot, and a binding would carry every row journaled under the
// reference, and every read of it, to whichever disk held the path last. A
// live row goes to the disk its recorded hardware identity names; a row
// journaled under the reference before stays there, unrewritten.
func TestHistoryIdentityNeverBindsProxmoxDiskAlertReferences(t *testing.T) {
	store, err := NewSQLiteResourceStore(t.TempDir(), "default")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	now := time.Now().UTC().Truncate(time.Second)
	ref := ProxmoxPhysicalDiskAlertResourceID("lab", "pve1", "/dev/sda")
	// Journaled before this process started, then raised live.
	require.NoError(t, store.RecordChange(pveDiskAlertChange("legacy", "pve1", "/dev/sda", ChangeAlertFired, "ZA1A2B3C", "", now.Add(-time.Hour))))
	snapshot := proxmoxHistoryIdentitySnapshot(now)
	snapshot.PhysicalDisks = []models.PhysicalDisk{pveDiskHistoryTestDisk("pve1", "/dev/sda", "ZA1A2B3C", "", now)}
	adapter := NewMonitorAdapter(NewRegistry(store))
	adapter.PopulateFromSnapshot(snapshot)
	registry := adapter.currentRegistry()
	diskID, ok := registry.ResolveReferenceID(ref)
	require.True(t, ok, "operator mutes resolve the reference to the disk")
	require.Equal(t, ResourceTypePhysicalDisk, registry.resources[diskID].Type)
	require.NotContains(t, adapter.legacyHistory.pending, ref, "the reference is never retried for a binding")
	require.NoError(t, adapter.RecordChange(pveDiskAlertChange("live", "pve1", "/dev/sda", ChangeAlertFired, "ZA1A2B3C", "", now)))

	_, found, err := store.ResolveHistorySourceIdentity(ref)
	require.NoError(t, err)
	require.False(t, found)
	byRef := pveDiskHistoryRows(t, store, ref)
	require.ElementsMatch(t, []string{"legacy", "live"}, pveDiskHistoryIDs(byRef))
	require.Equal(t, ref, byRef["legacy"].ResourceID)
	require.Equal(t, diskID, byRef["live"].ResourceID)
	require.ElementsMatch(t, []string{"live"}, pveDiskHistoryIDs(pveDiskHistoryRows(t, store, diskID)))
}

func pveDiskHistoryTestDisk(node, device, serial, wwn string, now time.Time) models.PhysicalDisk {
	return models.PhysicalDisk{ID: ProxmoxPhysicalDiskSourceID("lab", node, device, "", ""), Node: node, Instance: "lab",
		DevPath: device, Model: "Disk " + node + device, Serial: serial, WWN: wwn, Type: "sata", Health: "FAILED", Wearout: -1, LastChecked: now}
}

// pveDiskAlertChange is a lifecycle row as the alert manager journals it:
// under the path reference, with the evaluated disk's hardware identity.
func pveDiskAlertChange(id, node, device string, kind ChangeKind, serial, wwn string, at time.Time) ResourceChange {
	ref := ProxmoxPhysicalDiskAlertResourceID("lab", node, device)
	change := BuildAlertTimelineChange(ref, kind, at, "", AlertTimelineChange{
		AlertIdentifier: ProxmoxPhysicalDiskAlertIdentifiers(ref)[0], AlertStartedAt: at, AlertType: "disk-health",
		AlertMetadata: map[string]any{"disk_path": device, MetadataDiskSerial: serial, MetadataDiskWWN: wwn},
	})
	change.ID = id
	return *change
}

func pveDiskHistoryRows(t *testing.T, store ResourceStore, resourceID string) map[string]ResourceChange {
	t.Helper()
	changes, err := store.GetRecentChangesFiltered(resourceID, time.Time{}, 100, ResourceChangeFilters{Kinds: []ChangeKind{ChangeAlertFired}})
	require.NoError(t, err)
	rows := make(map[string]ResourceChange, len(changes))
	for _, change := range changes {
		rows[change.ID] = change
	}
	return rows
}

func pveDiskHistoryIDs(rows map[string]ResourceChange) []string {
	ids := make([]string, 0, len(rows))
	for id := range rows {
		ids = append(ids, id)
	}
	return ids
}

// Each PVE disk alert row belongs to the disk its recorded serial or WWN
// names, wherever that disk sits in the registry generation the event is
// evaluated against, and never to the disk the path names: a reboot reorder
// or a replacement disk before its first poll reaches the registry cannot
// claim another disk's history. Reads by the path reference still return
// every row raised under it.
func TestProxmoxDiskAlertRowsFollowRecordedHardwareIdentity(t *testing.T) {
	for _, backend := range []string{"sqlite", "memory"} {
		t.Run(backend, func(t *testing.T) {
			var store ResourceStore = NewMemoryStore()
			if backend == "sqlite" {
				sqlite, err := NewSQLiteResourceStore(t.TempDir(), "default")
				require.NoError(t, err)
				t.Cleanup(func() { require.NoError(t, sqlite.Close()) })
				store = sqlite
			}
			now := time.Now().UTC().Truncate(time.Second)
			const wwn = "0x5000c500a1b2c3d4"
			snapshot := proxmoxHistoryIdentitySnapshot(now)
			snapshot.PhysicalDisks = []models.PhysicalDisk{
				pveDiskHistoryTestDisk("pve1", "/dev/sda", "ZA1A2B3C", "", now),
				pveDiskHistoryTestDisk("pve1", "/dev/sdb", "ZA4D5E6F", "", now),
				pveDiskHistoryTestDisk("pve1", "/dev/sdc", "", wwn, now),
				pveDiskHistoryTestDisk("pve1", "/dev/sdd", "", "", now),
			}
			adapter := NewMonitorAdapter(NewRegistry(store))
			adapter.PopulateFromSnapshot(snapshot)
			registry := adapter.currentRegistry()
			diskA := historyIdentityResourceID(t, registry, ResourceTypePhysicalDisk, "Disk pve1/dev/sda")
			diskB := historyIdentityResourceID(t, registry, ResourceTypePhysicalDisk, "Disk pve1/dev/sdb")
			diskW := historyIdentityResourceID(t, registry, ResourceTypePhysicalDisk, "Disk pve1/dev/sdc")
			diskP := historyIdentityResourceID(t, registry, ResourceTypePhysicalDisk, "Disk pve1/dev/sdd")
			sda := ProxmoxPhysicalDiskAlertResourceID("lab", "pve1", "/dev/sda")
			sdc := ProxmoxPhysicalDiskAlertResourceID("lab", "pve1", "/dev/sdc")
			sdd := ProxmoxPhysicalDiskAlertResourceID("lab", "pve1", "/dev/sdd")
			replacement := MachineIdentityCanonicalID(ResourceTypePhysicalDisk, "ZC9X8Y7Z")
			replacementWWN := MachineIdentityCanonicalID(ResourceTypePhysicalDisk, "0x5000c500ffeeddcc")

			want := map[string]string{}
			record := func(change ResourceChange, owner string) {
				t.Helper()
				require.NoError(t, adapter.RecordChange(change))
				want[change.ID] = owner
			}
			record(pveDiskAlertChange("own", "pve1", "/dev/sda", ChangeAlertFired, "ZA1A2B3C", "", now), diskA)
			// A reboot put B at /dev/sda; the registry still places it at /dev/sdb.
			record(pveDiskAlertChange("reordered", "pve1", "/dev/sda", ChangeAlertFired, "ZA4D5E6F", "", now), diskB)
			// A replacement disk fired before its first poll reached the registry.
			record(pveDiskAlertChange("replacement", "pve1", "/dev/sda", ChangeAlertFired, "ZC9X8Y7Z", "", now), replacement)
			record(pveDiskAlertChange("wwn", "pve1", "/dev/sdc", ChangeAlertFired, "", wwn, now), diskW)
			record(pveDiskAlertChange("wwn-replacement", "pve1", "/dev/sdc", ChangeAlertFired, "", "0x5000c500ffeeddcc", now), replacementWWN)
			record(pveDiskAlertChange("path-only", "pve1", "/dev/sdd", ChangeAlertFired, "", "", now), diskP)
			// No usable identity, and the disk at the path reports one: the row
			// stays under the reference rather than following the path.
			record(pveDiskAlertChange("unidentified", "pve1", "/dev/sda", ChangeAlertFired, "", "", now), sda)
			record(pveDiskAlertChange("placeholder", "pve1", "/dev/sda", ChangeAlertFired, "To Be Filled By O.E.M.", "", now), sda)

			all := pveDiskHistoryRows(t, store, "")
			for id, owner := range want {
				row, ok := all[id]
				require.True(t, ok, id)
				require.Equal(t, owner, row.ResourceID, id)
				if strings.Contains(owner, ":disk:") {
					require.Empty(t, OwnedAlertReference(row), id)
				} else {
					require.Equal(t, ProxmoxPhysicalDiskAlertResourceID("lab", "pve1", row.Metadata["disk_path"].(string)), OwnedAlertReference(row), id)
				}
			}
			for _, ref := range []string{sda, sdc, sdd} {
				_, bound, err := store.(resourceHistoryIdentityWriter).ResolveHistorySourceIdentity(ref)
				require.NoError(t, err)
				require.False(t, bound, "a path reference never binds: %s", ref)
			}
			require.ElementsMatch(t, []string{"own"}, pveDiskHistoryIDs(pveDiskHistoryRows(t, store, diskA)))
			require.ElementsMatch(t, []string{"reordered"}, pveDiskHistoryIDs(pveDiskHistoryRows(t, store, diskB)))
			require.ElementsMatch(t, []string{"wwn"}, pveDiskHistoryIDs(pveDiskHistoryRows(t, store, diskW)))
			require.ElementsMatch(t, []string{"path-only"}, pveDiskHistoryIDs(pveDiskHistoryRows(t, store, diskP)))

			byRef := pveDiskHistoryRows(t, store, sda)
			require.ElementsMatch(t, []string{"own", "reordered", "replacement", "unidentified", "placeholder"}, pveDiskHistoryIDs(byRef))
			require.Equal(t, diskB, byRef["reordered"].ResourceID, "a read by the reference returns rows where they are owned")
			count, err := store.CountRecentChangesFiltered(sda, time.Time{}, ResourceChangeFilters{Kinds: []ChangeKind{ChangeAlertFired}})
			require.NoError(t, err)
			require.Equal(t, len(byRef), count)
			byKind, err := store.CountRecentChangesByKind(sda, time.Time{})
			require.NoError(t, err)
			require.Equal(t, len(byRef), byKind[ChangeAlertFired])

			// Once the replacement reaches the registry it owns the row it raised.
			snapshot.PhysicalDisks[0] = pveDiskHistoryTestDisk("pve1", "/dev/sda", "ZC9X8Y7Z", "", now)
			adapter.PopulateFromSnapshot(snapshot)
			require.Equal(t, replacement, historyIdentityResourceID(t, adapter.currentRegistry(), ResourceTypePhysicalDisk, "Disk pve1/dev/sda"))
			require.ElementsMatch(t, []string{"replacement"}, pveDiskHistoryIDs(pveDiskHistoryRows(t, store, replacement)))
		})
	}
}

// The registry only translates a row's recorded hardware identity into a
// canonical ID. A WWN only one of two matching disks shares decides between
// them, the path never breaks a tie, and a row without usable identity names
// only the one identity-less disk at its path.
func TestProxmoxDiskAlertOwnerDecisions(t *testing.T) {
	registry := NewRegistry(nil)
	disk := func(id, node, device, serial, wwn string) {
		registry.resources[id] = &Resource{ID: id, Type: ResourceTypePhysicalDisk,
			PhysicalDisk: &PhysicalDiskMeta{DevPath: device, Serial: serial, WWN: wwn},
			Proxmox:      &ProxmoxData{Instance: "lab", NodeName: node}}
	}
	serialDisk := MachineIdentityCanonicalID(ResourceTypePhysicalDisk, "ZA1A2B3C")
	disk(serialDisk, "pve1", "/dev/sda", "ZA1A2B3C", "")
	disk("physical_disk-dup-pve1", "pve1", "/dev/sdb", "DUP00001", "")
	disk("physical_disk-dup-pve2", "pve2", "/dev/sdb", "DUP00001", "")
	disk("physical_disk-twin-1", "pve1", "/dev/sdg", "TWIN0001", "0x5000c500000000a1")
	disk("physical_disk-twin-2", "pve1", "/dev/sdh", "TWIN0001", "0x5000c500000000b2")
	// A RAID volume: PVE reports the full NAA as its serial and a truncated
	// udev WWN, and the merged agent observation keeps the full naa. WWN.
	disk("physical_disk-raid", "pve1", "/dev/sdi", "600508b1001c5c7a1b2c3d4e5f607080", "naa.600508b1001c5c7a1b2c3d4e5f607080")
	// Merged with its agent's observation, whose smartctl WWN the registry
	// keeps in a framing PVE does not use.
	disk(MachineIdentityCanonicalID(ResourceTypePhysicalDisk, "ZB5C6D7E"), "pve1", "/dev/sdj", "ZB5C6D7E", "5-c500-da60ca43")
	disk("physical_disk-wwn", "pve1", "/dev/sdc", "", "naa.5000c500a1b2c3d4")
	disk("physical_disk-path", "pve1", "/dev/sdd", "", "")
	disk("physical_disk-member-1", "pve1", "/dev/sde", "", "")
	disk("physical_disk-member-2", "pve1", "/dev/sde", "", "")
	disk(MachineIdentityCanonicalID(ResourceTypePhysicalDisk, "HELD0001"), "pve1", "/dev/sdf", "OTHER001", "")

	for _, tc := range []struct {
		name, node, device, serial, wwn, want string
	}{
		{"serial", "pve1", "/dev/sda", "ZA1A2B3C", "", serialDisk},
		{"serial after a move to another node", "pve2", "/dev/sdz", "ZA1A2B3C", "", serialDisk},
		// The path never breaks a tie: a stale generation could still place
		// the other disk there.
		{"shared serial", "pve1", "/dev/sdb", "DUP00001", "", ""},
		{"shared serial told apart by WWN", "pve1", "/dev/sdg", "TWIN0001", "naa.5000c500000000b2", "physical_disk-twin-2"},
		{"shared serial without WWN", "pve1", "/dev/sdg", "TWIN0001", "", ""},
		{"RAID volume with a truncated WWN", "pve1", "/dev/sdi", "600508b1001c5c7a1b2c3d4e5f607080", "0x600508b1001c5c7a", "physical_disk-raid"},
		{"agent-framed WWN on the merged disk", "pve1", "/dev/sdj", "ZB5C6D7E", "0x5000c500da60ca43", MachineIdentityCanonicalID(ResourceTypePhysicalDisk, "ZB5C6D7E")},
		{"WWN framing differs between reporters", "pve1", "/dev/sdc", "", "0x5000c500a1b2c3d4", "physical_disk-wwn"},
		{"unknown hardware", "pve1", "/dev/sda", "ZC9X8Y7Z", "", MachineIdentityCanonicalID(ResourceTypePhysicalDisk, "ZC9X8Y7Z")},
		{"minted ID held by other hardware", "pve1", "/dev/sda", "HELD0001", "", ""},
		{"no identity, identity-less disk at path", "pve1", "/dev/sdd", "", "", "physical_disk-path"},
		{"no identity, two disks at path", "pve1", "/dev/sde", "", "", ""},
		{"no identity, disk at path reports one", "pve1", "/dev/sda", "", "", ""},
		{"no identity, placeholder serial", "pve1", "/dev/sda", "0000000000000000", "", ""},
		{"no identity, nothing at path", "pve1", "/dev/sdz", "", "", ""},
	} {
		ref := ProxmoxPhysicalDiskAlertResourceID("lab", tc.node, tc.device)
		got := registry.proxmoxDiskAlertOwner(ref, map[string]any{MetadataDiskSerial: tc.serial, MetadataDiskWWN: tc.wwn})
		require.Equal(t, tc.want, got, tc.name)
		// Reports consume an immutable snapshot, not the live registry. Both
		// entry points must make the exact same recorded-hardware decision.
		var snapshot []Resource
		for _, resource := range registry.resources {
			snapshot = append(snapshot, *resource)
		}
		require.Equal(t, tc.want, ProxmoxPhysicalDiskAlertOwner(ref, map[string]any{MetadataDiskSerial: tc.serial, MetadataDiskWWN: tc.wwn}, snapshot), tc.name)
	}
	require.Empty(t, ProxmoxPhysicalDiskAlertOwner("not-a-PVE-reference", map[string]any{MetadataDiskSerial: "ZA1A2B3C"}, nil))
}

// A read by a PVE disk alert reference reaches the rows owned away from it
// through the alert identity index, not a journal scan.
func TestProxmoxDiskAlertReferenceReadUsesIndexes(t *testing.T) {
	store, err := NewSQLiteResourceStore(t.TempDir(), "default")
	require.NoError(t, err)
	defer store.Close()
	before := time.Now()
	// The incident projection's read (memory.IncidentStore.QueryIncidents).
	filters := ResourceChangeFilters{ObservedBefore: &before, Kinds: []ChangeKind{
		ChangeAlertFired, ChangeAlertResolved, ChangeAlertAcknowledged, ChangeAlertUnacknowledged,
		ChangeAlertSnoozed, ChangeAlertUnsnoozed, ChangeCommandExecuted, ChangeRunbookExecuted,
	}}
	ref := ProxmoxPhysicalDiskAlertResourceID("lab", "pve1", "/dev/sda")
	query, args := buildRecentChangeCountQuery([]string{ref}, before.Add(-resourceChangesRetention), filters, "EXPLAIN QUERY PLAN SELECT id FROM resource_changes", store.resourceChangesObservedAtExpr(), store.resourceChangesSourceTypeExpr(), store.resourceChangesSourceAdapterExpr())
	rows, err := store.db.Query(query, args...)
	require.NoError(t, err)
	defer rows.Close()
	var plans []string
	for rows.Next() {
		var id, parent, unused int
		var detail string
		require.NoError(t, rows.Scan(&id, &parent, &unused, &detail))
		plans = append(plans, detail)
	}
	require.NoError(t, rows.Err())
	plan := strings.Join(plans, "\n")
	require.Contains(t, plan, "idx_resource_changes_canonical_time")
	require.Contains(t, plan, "idx_resource_changes_alert_time")
	require.NotContains(t, plan, "SCAN resource_changes")
}

func TestProxmoxPhysicalDiskAlertReferenceShape(t *testing.T) {
	for _, ref := range []string{
		ProxmoxPhysicalDiskAlertResourceID("lab", "pve1", "/dev/sda"),
		ProxmoxPhysicalDiskAlertResourceID("Production West", "pve5", "/dev/disk/by-id/ATA_DISK"),
		ProxmoxPhysicalDiskAlertResourceID("lab", "pve1", "/"),
	} {
		require.True(t, isProxmoxPhysicalDiskAlertReference(ref), ref)
		require.Len(t, ProxmoxPhysicalDiskAlertIdentifiers(ref), 2, ref)
	}
	for _, ref := range []string{
		ProxmoxPhysicalDiskAlertResourceID("lab", "pve1", ""),
		"agent:host-1/disk:sda", "lab-pve1", "lab:pve1:101", "docker:host/disk:sda", "pve1:disk:dev-sda", "lab:pve1:disk:Dev-SDA",
	} {
		require.False(t, isProxmoxPhysicalDiskAlertReference(ref), ref)
		require.Nil(t, ProxmoxPhysicalDiskAlertIdentifiers(ref), ref)
	}
}
