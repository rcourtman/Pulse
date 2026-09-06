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
