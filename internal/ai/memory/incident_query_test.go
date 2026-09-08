package memory

import (
	"errors"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/stretchr/testify/require"
)

func TestIncidentQueryCanonicalOccurrencesAndNotes(t *testing.T) {
	for _, backend := range []string{"memory", "sqlite"} {
		t.Run(backend, func(t *testing.T) {
			var canonical unifiedresources.ResourceStore
			if backend == "memory" {
				canonical = unifiedresources.NewMemoryStore()
			} else {
				store, err := unifiedresources.NewSQLiteResourceStore(t.TempDir(), "default")
				require.NoError(t, err)
				t.Cleanup(func() { require.NoError(t, store.Close()) })
				canonical = store
			}
			config := IncidentStoreConfig{DataDir: t.TempDir()}
			memory := NewIncidentStore(config)
			memory.SetResourceTimelineStore(canonical)
			start := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
			second := start.Add(2 * time.Minute)
			record := func(id string, kind unifiedresources.ChangeKind, at time.Time) {
				observed := at.Add(10 * time.Second)
				require.NoError(t, canonical.RecordChange(unifiedresources.ResourceChange{ID: id, ResourceID: "resource-canonical", Kind: kind, ObservedAt: observed, OccurredAt: &at, SourceType: unifiedresources.SourcePlatformEvent, SourceAdapter: unifiedresources.AdapterProxmox, Actor: "operator", Confidence: unifiedresources.ConfidenceHigh, Metadata: map[string]any{unifiedresources.MetadataAlertIdentifier: "repeat-alert", "nested": map[string]any{"risk": "storage"}}}))
			}
			record("first-fired", unifiedresources.ChangeAlertFired, start)
			record("first-resolved", unifiedresources.ChangeAlertResolved, start.Add(time.Minute))
			record("second-fired", unifiedresources.ChangeAlertFired, second)
			record("second-ack", unifiedresources.ChangeAlertAcknowledged, second.Add(time.Minute))
			page, err := memory.QueryIncidents(IncidentQuery{ResourceID: "resource-canonical", Limit: 10})
			require.NoError(t, err)
			require.Len(t, page.Incidents, 2)
			require.NotEmpty(t, page.Incidents[0].ID)
			require.Equal(t, IncidentStatusOpen, page.Incidents[0].Status)
			require.True(t, page.Incidents[0].Acknowledged)
			first := page.Incidents[1]
			require.NotEqual(t, page.Incidents[0].ID, first.ID)
			require.Equal(t, IncidentStatusResolved, first.Status)
			require.False(t, first.Acknowledged)
			require.Len(t, first.Events, 2)
			require.Equal(t, start.Add(time.Minute), *first.ClosedAt)
			require.Equal(t, start, first.OpenedAt)
			require.Equal(t, "canonical_resource_history", first.Events[0].Source)
			require.Equal(t, start.Add(10*time.Second), first.Events[0].Evidence.ObservedAt)
			require.Equal(t, start, *first.Events[0].Evidence.OccurredAt)
			require.Equal(t, "operator", first.Events[0].Evidence.Actor)
			first.Events[0].Evidence.Metadata["nested"].(map[string]any)["risk"] = "mutated"
			again, err := memory.QueryIncidents(IncidentQuery{AlertIdentifier: "repeat-alert", StartedAt: start, Limit: 1})
			require.NoError(t, err)
			require.Len(t, again.Incidents, 1)
			require.Equal(t, "storage", again.Incidents[0].Events[0].Evidence.Metadata["nested"].(map[string]any)["risk"])
			require.True(t, memory.RecordNote("repeat-alert", first.ID, "Check the old pool before replacing it", "operator"))
			noted, err := memory.QueryIncidents(IncidentQuery{AlertIdentifier: "repeat-alert", StartedAt: start, Limit: 1})
			require.NoError(t, err)
			require.Len(t, noted.Incidents, 1)
			require.Len(t, noted.Incidents[0].Events, 3)
			require.Equal(t, IncidentStatusResolved, noted.Incidents[0].Status)
			require.Equal(t, IncidentEventNote, noted.Incidents[0].Events[2].Type)
			require.Equal(t, "operator_note", noted.Incidents[0].Events[2].Source)
			require.NoError(t, memory.saveToDisk())
			restored := NewIncidentStore(config)
			restored.SetResourceTimelineStore(canonical)
			persisted, err := restored.QueryIncidents(IncidentQuery{AlertIdentifier: "repeat-alert", StartedAt: start, Limit: 1})
			require.NoError(t, err)
			require.Len(t, persisted.Incidents, 1)
			require.Equal(t, first.ID, persisted.Incidents[0].ID)
			require.Equal(t, "Check the old pool before replacing it", persisted.Incidents[0].Events[2].Details["note"])

			latest, err := memory.QueryIncidents(IncidentQuery{AlertIdentifier: "repeat-alert", Limit: 1})
			require.NoError(t, err)
			require.True(t, latest.History.HasMoreIncidents)
			require.Len(t, latest.Incidents[0].Events, 2) // old note remains attached to old occurrence
			formatted := memory.FormatForAlert("repeat-alert", 1)
			require.Contains(t, formatted, "Showing latest 1 of 2 recorded events")
			require.Contains(t, formatted, "Occurrence evidence source: canonical_resource_history")
			bounded, err := memory.QueryIncidents(IncidentQuery{AlertIdentifier: "repeat-alert", ChangeLimit: 1})
			require.NoError(t, err)
			require.True(t, bounded.History.HasMoreChanges)
			// The saved first occurrence remains separate from incomplete later evidence.
			require.Contains(t, formatIncidentCoverage(bounded), "truncated")
		})
	}
}

type failingIncidentHistory struct{ *unifiedresources.MemoryStore }

func (s failingIncidentHistory) GetRecentChangesFiltered(string, time.Time, int, unifiedresources.ResourceChangeFilters) ([]unifiedresources.ResourceChange, error) {
	return nil, errors.New("history read denied")
}

func TestIncidentQueryUnavailableAndMissingStart(t *testing.T) {
	memory := NewIncidentStore(IncidentStoreConfig{})
	memory.SetResourceTimelineStore(failingIncidentHistory{unifiedresources.NewMemoryStore()})
	_, err := memory.QueryIncidents(IncidentQuery{ResourceID: "resource-a"})
	require.ErrorContains(t, err, "history read denied")
	require.Contains(t, memory.FormatForResource("resource-a", 5), "unavailable")
	require.Contains(t, memory.FormatForAlert("alert", 5), "unavailable")
	require.Contains(t, memory.FormatForPatrol(5), "unavailable")
	canonical := unifiedresources.NewMemoryStore()
	memory.SetResourceTimelineStore(canonical)
	now := time.Now().UTC().Add(-time.Minute)
	require.NoError(t, canonical.RecordChange(unifiedresources.ResourceChange{ID: "orphan-ack", ResourceID: "resource-a", ObservedAt: now, Kind: unifiedresources.ChangeAlertAcknowledged, Metadata: map[string]any{unifiedresources.MetadataAlertIdentifier: "alert"}}))
	page, err := memory.QueryIncidents(IncidentQuery{ResourceID: "resource-a"})
	require.NoError(t, err)
	require.Len(t, page.Incidents, 1)
	require.Equal(t, IncidentStatus("unknown"), page.Incidents[0].Status)
	require.True(t, page.Incidents[0].OpenedAt.IsZero())
	require.True(t, incidentEventTimestamp(unifiedresources.ResourceChange{}).IsZero())
}

func TestIncidentQueryDoesNotMergeRapidOccurrences(t *testing.T) {
	canonical := unifiedresources.NewMemoryStore()
	store := NewIncidentStore(IncidentStoreConfig{})
	store.SetResourceTimelineStore(canonical)
	first := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	for _, at := range []time.Time{first, first.Add(500 * time.Millisecond)} {
		change := unifiedresources.BuildAlertTimelineChange("resource-a", unifiedresources.ChangeAlertFired, at, "", unifiedresources.AlertTimelineChange{AlertIdentifier: "rapid"})
		require.NoError(t, canonical.RecordChange(*change))
	}
	page, err := store.QueryIncidents(IncidentQuery{AlertIdentifier: "rapid"})
	require.NoError(t, err)
	require.Len(t, page.Incidents, 2)
	exact, err := store.QueryIncidents(IncidentQuery{AlertIdentifier: "rapid", StartedAt: first, Limit: 1})
	require.NoError(t, err)
	require.Len(t, exact.Incidents, 1)
	require.Equal(t, first, exact.Incidents[0].OpenedAt)
	stableID := exact.Incidents[0].ID
	// An independently recorded copy of the same firing is still one occurrence.
	duplicate := unifiedresources.BuildAlertTimelineChange("resource-a", unifiedresources.ChangeAlertFired, first, "", unifiedresources.AlertTimelineChange{AlertIdentifier: "rapid"})
	require.NoError(t, canonical.RecordChange(*duplicate))
	exact, err = store.QueryIncidents(IncidentQuery{AlertIdentifier: "rapid", StartedAt: first, Limit: 1})
	require.NoError(t, err)
	require.Len(t, exact.Incidents, 1)
	require.Equal(t, stableID, exact.Incidents[0].ID)
	require.Len(t, exact.Incidents[0].Events, 2)
}

func TestIncidentQueryMergesDuplicateSavedOccurrencesWithoutLosingNotes(t *testing.T) {
	canonical := unifiedresources.NewMemoryStore()
	store := NewIncidentStore(IncidentStoreConfig{})
	store.SetResourceTimelineStore(canonical)
	start := time.Now().UTC().Add(-time.Hour)
	closed := start.Add(time.Minute)
	// Reproduce independently imported legacy shells carrying the same firing.
	store.incidents = []*incidentShell{
		{ID: "original", AlertIdentifier: "duplicated", ResourceID: "resource-a", OpenedAt: start, Events: []IncidentEvent{{ID: "note-a", Type: IncidentEventNote, Timestamp: start, Details: map[string]any{"note": "first note"}}}},
		{ID: "imported", AlertIdentifier: "duplicated", ResourceID: "resource-a", OpenedAt: start, OccurrenceClosedAt: &closed, Events: []IncidentEvent{{ID: "note-b", Type: IncidentEventNote, Timestamp: closed, Details: map[string]any{"note": "second note"}}}},
	}
	for _, kind := range []unifiedresources.ChangeKind{unifiedresources.ChangeAlertFired, unifiedresources.ChangeAlertResolved} {
		at := start
		if kind == unifiedresources.ChangeAlertResolved {
			at = closed
		}
		change := unifiedresources.BuildAlertTimelineChange("resource-a", kind, at, "", unifiedresources.AlertTimelineChange{AlertIdentifier: "duplicated"})
		require.NoError(t, canonical.RecordChange(*change))
	}
	page, err := store.QueryIncidents(IncidentQuery{AlertIdentifier: "duplicated", StartedAt: start})
	require.NoError(t, err)
	require.Len(t, page.Incidents, 1)
	require.Equal(t, IncidentStatusResolved, page.Incidents[0].Status)
	require.Len(t, page.Incidents[0].Events, 4)
	for _, id := range []string{"original", "imported"} {
		require.True(t, store.RecordNote("duplicated", id, "after merge "+id, "operator"))
		aliased, err := store.QueryIncidents(IncidentQuery{IncidentID: id, AlertIdentifier: "duplicated"})
		require.NoError(t, err)
		require.Len(t, aliased.Incidents, 1)
		require.Equal(t, page.Incidents[0].ID, aliased.Incidents[0].ID)
	}
	final, err := store.QueryIncidents(IncidentQuery{AlertIdentifier: "duplicated"})
	require.NoError(t, err)
	require.Len(t, final.Incidents[0].Events, 6)
}

func TestIncidentHistorySummaryPreservesLegacyAttribution(t *testing.T) {
	page := IncidentPage{History: IncidentHistoryCoverage{Source: "canonical_resource_history"}, Incidents: []*Incident{{ID: "legacy", AlertType: "backup", Status: IncidentStatusUnknown, History: &IncidentHistoryCoverage{Source: "legacy_incident_memory"}}}}
	require.Contains(t, FormatIncidentPageForResource(page), "[source=legacy_incident_memory]")
}

func TestIncidentQueryCanonicalAlertContextOwnsLegacyShell(t *testing.T) {
	canonical := unifiedresources.NewMemoryStore()
	store := NewIncidentStore(IncidentStoreConfig{})
	store.SetResourceTimelineStore(canonical)
	start := time.Now().UTC().Add(-time.Hour)
	store.incidents = []*incidentShell{{
		ID: "saved-id", AlertIdentifier: "alert-context", ResourceID: "docker:legacy-id",
		ResourceType: "host", AlertType: "old-type", Level: "warning", Message: "old message", OpenedAt: start,
	}}
	require.NoError(t, canonical.RecordChange(unifiedresources.ResourceChange{
		ID: "canonical-fired", ResourceID: "app-container-current", Kind: unifiedresources.ChangeAlertFired,
		ObservedAt: start, OccurredAt: &start, Metadata: map[string]any{
			unifiedresources.MetadataAlertIdentifier: "alert-context", unifiedresources.MetadataAlertType: "docker-container-health",
			unifiedresources.MetadataAlertLevel: "critical", unifiedresources.MetadataAlertMessage: "Container health is unhealthy",
			"resourceType": "app-container",
		},
	}))
	require.NoError(t, canonical.RecordChange(unifiedresources.ResourceChange{
		ID: "related-command", ResourceID: "node-executor", Kind: unifiedresources.ChangeCommandExecuted,
		ObservedAt: start.Add(time.Minute), Metadata: map[string]any{unifiedresources.MetadataAlertIdentifier: "alert-context"},
	}))
	page, err := store.QueryIncidents(IncidentQuery{AlertIdentifier: "alert-context"})
	require.NoError(t, err)
	require.Len(t, page.Incidents, 1)
	incident := page.Incidents[0]
	require.Equal(t, "saved-id", incident.ID)
	require.Equal(t, "app-container-current", incident.ResourceID)
	require.Equal(t, "app-container", incident.ResourceType)
	require.Equal(t, "docker-container-health", incident.AlertType)
	require.Equal(t, "critical", incident.Level)
	require.Equal(t, "Container health is unhealthy", incident.Message)
	require.Equal(t, "docker:legacy-id", store.incidents[0].ResourceID)
}

func TestIncidentAlertContextRetainsOperatorNote(t *testing.T) {
	canonical := unifiedresources.NewMemoryStore()
	store := NewIncidentStore(IncidentStoreConfig{DataDir: t.TempDir()})
	store.SetResourceTimelineStore(canonical)
	started := time.Now().UTC().Add(-time.Hour)
	change := unifiedresources.BuildAlertTimelineChange("resource-a", unifiedresources.ChangeAlertFired, started, "", unifiedresources.AlertTimelineChange{AlertIdentifier: "noted-alert"})
	require.NoError(t, canonical.RecordChange(*change))
	page, err := store.QueryIncidents(IncidentQuery{AlertIdentifier: "noted-alert"})
	require.NoError(t, err)
	require.Len(t, page.Incidents, 1)
	require.True(t, store.RecordNote("noted-alert", page.Incidents[0].ID, "Keep the old pool until its replacement is verified", "operator"))
	context := store.FormatForAlert("noted-alert", 10)
	require.Contains(t, context, "Note added by operator: Keep the old pool until its replacement is verified")
	require.Contains(t, context, "source=operator_note")
}
