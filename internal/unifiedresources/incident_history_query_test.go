package unifiedresources

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// These queries must not lose a relevant incident behind a fleet-wide limit.
// Use both durable and in-memory stores, including late observations and aliases.
func TestIncidentHistoryQueryFiltersBeforeLimit(t *testing.T) {
	for _, backend := range []string{"memory", "sqlite"} {
		t.Run(backend, func(t *testing.T) {
			var store ResourceStore
			if backend == "memory" {
				store = NewMemoryStore()
			} else {
				sqlite, err := NewSQLiteResourceStore(t.TempDir(), "default")
				require.NoError(t, err)
				t.Cleanup(func() { require.NoError(t, sqlite.Close()) })
				store = sqlite
			}
			now := time.Now().UTC().Truncate(time.Second)
			older := now.Add(-time.Hour)
			record := func(id, alert string, observed time.Time) {
				require.NoError(t, store.RecordChange(ResourceChange{ID: id, ResourceID: "resource-a", Kind: ChangeAlertFired, ObservedAt: observed, OccurredAt: &older, SourceType: SourceHeuristic, SourceAdapter: AdapterProxmox, Metadata: map[string]any{MetadataAlertIdentifier: alert}}))
			}
			record("target-a", "wanted", now)
			record("target-b", "wanted", now) // equal timestamps have stable ID order
			for i := 0; i < 300; i++ {
				record(fmt.Sprintf("noise-%03d", i), "unrelated", now.Add(time.Second))
			}
			record("late-insertion", "wanted", older)
			record("upper-bound", "wanted", now.Add(2*time.Second))
			before := now.Add(2 * time.Second)
			filters := ResourceChangeFilters{AlertIdentifiers: []string{"wanted"}, ObservedBefore: &before, Kinds: []ChangeKind{ChangeAlertFired}, SourceTypes: []ChangeSourceType{SourceHeuristic}, SourceAdapters: []ChangeSourceAdapter{AdapterProxmox}}
			got, err := store.GetRecentChangesFiltered("", older, 2, filters)
			require.NoError(t, err)
			require.Len(t, got, 2)
			require.Equal(t, "target-b", got[0].ID)
			require.Equal(t, "target-a", got[1].ID)
			adapter := NewMonitorAdapter(NewRegistry(store))
			forwarded, err := adapter.GetRecentChangesFiltered("", older, 2, filters)
			require.NoError(t, err)
			require.Equal(t, got, forwarded)
			require.Equal(t, older, *got[0].OccurredAt) // observation bounds do not rewrite occurrence time
			count, err := store.CountRecentChangesFiltered("", older, filters)
			require.NoError(t, err)
			require.Equal(t, 3, count)
			kinds, err := store.CountRecentChangesByKindFiltered("", older, filters)
			require.NoError(t, err)
			require.Equal(t, 3, kinds[ChangeAlertFired])
			sources, err := store.CountRecentChangesBySourceTypeFiltered("", older, filters)
			require.NoError(t, err)
			require.Equal(t, 3, sources[SourceHeuristic])
			adapters, err := store.CountRecentChangesBySourceAdapterFiltered("", older, filters)
			require.NoError(t, err)
			require.Equal(t, 3, adapters[AdapterProxmox])

			// A history alias selects the original evidence without rewriting its
			// resource identity, or promoting the alias into action authority.
			writer := store.(interface {
				RecordChangeWithSourceIdentity(ResourceChange, string) error
			})
			require.NoError(t, writer.RecordChangeWithSourceIdentity(ResourceChange{ID: "alias-event", ResourceID: "resource-canonical", Kind: ChangeAlertResolved, ObservedAt: now, Metadata: map[string]any{MetadataAlertIdentifier: "alias-alert"}}, "resource-legacy"))
			aliasFilters := ResourceChangeFilters{AlertIdentifiers: []string{"alias-alert"}, ObservedBefore: &before}
			for _, resource := range []string{"resource-legacy", "resource-canonical"} {
				identities, err := adapter.ResourceHistoryIDs(resource)
				require.NoError(t, err)
				require.Contains(t, identities, "resource-legacy")
				require.Contains(t, identities, "resource-canonical")
				aliased, err := store.GetRecentChangesFiltered(resource, older, 1, aliasFilters)
				require.NoError(t, err)
				require.Len(t, aliased, 1)
				require.Equal(t, "resource-canonical", aliased[0].ResourceID)
				count, err := store.CountRecentChangesFiltered(resource, older, aliasFilters)
				require.NoError(t, err)
				require.Equal(t, 1, count)
			}
			// JSON numbers must not compare equal to textual alert identifiers.
			require.NoError(t, store.RecordChange(ResourceChange{ID: "numeric-identity", ResourceID: "resource-a", Kind: ChangeAlertFired, ObservedAt: now, Metadata: map[string]any{MetadataAlertIdentifier: 123}}))
			malformed, err := store.GetRecentChangesFiltered("", older, 10, ResourceChangeFilters{AlertIdentifiers: []string{"123"}})
			require.NoError(t, err)
			require.Empty(t, malformed)
			filters.AlertIdentifiers = []string{"wanted' OR 1=1 --"}
			got, err = store.GetRecentChangesFiltered("", older, 2, filters)
			require.NoError(t, err)
			require.Empty(t, got)
		})
	}
}

func TestIncidentHistoryQueryReadFailureIsNotEmptyHistory(t *testing.T) {
	store, err := NewSQLiteResourceStore(t.TempDir(), "default")
	require.NoError(t, err)
	require.NoError(t, store.Close())
	filters := ResourceChangeFilters{AlertIdentifiers: []string{"wanted"}}
	_, err = store.GetRecentChangesFiltered("", time.Time{}, 1, filters)
	require.Error(t, err)
	_, err = store.CountRecentChangesFiltered("", time.Time{}, filters)
	require.Error(t, err)
	adapter := NewMonitorAdapter(NewRegistry(store))
	_, err = adapter.GetRecentChangesFiltered("", time.Time{}, 1, filters)
	require.Error(t, err)
	_, err = (&MonitorAdapter{}).GetRecentChangesFiltered("", time.Time{}, 1, filters)
	require.ErrorContains(t, err, "unavailable")
	_, err = (&MonitorAdapter{}).ResourceHistoryIDs("resource-a")
	require.ErrorContains(t, err, "unavailable")
}

func TestIncidentHistoryQueryInvalidLegacyMetadata(t *testing.T) {
	store, err := NewSQLiteResourceStore(t.TempDir(), "default")
	require.NoError(t, err)
	defer store.Close()
	require.NoError(t, store.RecordChange(ResourceChange{ID: "bad-json", ResourceID: "resource-a", Kind: ChangeAlertFired, ObservedAt: time.Now()}))
	_, err = store.db.Exec(`UPDATE resource_changes SET metadata_json = '{invalid' WHERE id = 'bad-json'`)
	require.NoError(t, err)
	filters := ResourceChangeFilters{AlertIdentifiers: []string{"wanted"}}
	got, err := store.GetRecentChangesFiltered("", time.Time{}, 1, filters)
	require.NoError(t, err)
	require.Empty(t, got)
	count, err := store.CountRecentChangesFiltered("", time.Time{}, filters)
	require.NoError(t, err)
	require.Zero(t, count)
}

func TestIncidentHistoryQueryUsesAlertIdentityIndex(t *testing.T) {
	store, err := NewSQLiteResourceStore(t.TempDir(), "default")
	require.NoError(t, err)
	defer store.Close()
	query, args := buildRecentChangeCountQuery(nil, time.Time{}, ResourceChangeFilters{AlertIdentifiers: []string{"wanted"}, Kinds: []ChangeKind{ChangeAlertFired, ChangeAlertResolved}}, "EXPLAIN QUERY PLAN SELECT COUNT(*) FROM resource_changes", store.resourceChangesObservedAtExpr(), store.resourceChangesSourceTypeExpr(), store.resourceChangesSourceAdapterExpr())
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
	require.Contains(t, strings.Join(plans, "\n"), "idx_resource_changes_alert_time")
}
