package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/memory"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/eventlog"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/stretchr/testify/require"
)

// Count only the public history reads, not store implementation details. Replay
// writes the occurrence shell and canonical changes; it has no returned
// timeline to serve and must not hydrate one for each imported snapshot.
type replayHistoryReadCounter struct {
	memory.IncidentTimelineStore
	filteredReads atomic.Int64
	identityReads atomic.Int64
	recentReads   atomic.Int64
}

func (s *replayHistoryReadCounter) GetRecentChangesFiltered(id string, since time.Time, limit int, filters unifiedresources.ResourceChangeFilters) ([]unifiedresources.ResourceChange, error) {
	s.filteredReads.Add(1)
	return s.IncidentTimelineStore.GetRecentChangesFiltered(id, since, limit, filters)
}

func (s *replayHistoryReadCounter) ResourceHistoryIDs(id string) ([]string, error) {
	s.identityReads.Add(1)
	return s.IncidentTimelineStore.ResourceHistoryIDs(id)
}

func (s *replayHistoryReadCounter) GetRecentChanges(id string, since time.Time, limit int) ([]unifiedresources.ResourceChange, error) {
	s.recentReads.Add(1)
	return s.IncidentTimelineStore.GetRecentChanges(id, since, limit)
}

func importReplaySnapshots(t *testing.T, manager *alerts.Manager, snapshots []alerts.Alert) {
	t.Helper()
	store, err := eventlog.Open(t.TempDir())
	require.NoError(t, err)
	manager.SetEventLog(store) // the manager owns Close
	events := make([]eventlog.Event, len(snapshots))
	for i, alert := range snapshots {
		payload, err := json.Marshal(alert)
		require.NoError(t, err)
		events[i] = eventlog.Event{Type: eventlog.TypeHistoryImported, AlertID: alert.ID,
			ResourceID: alert.ResourceID, OccurredAt: alert.LastSeen, Snapshot: payload}
	}
	require.NoError(t, store.ImportEvents(events))
}

func TestImportedTimelineReplayDoesNotReadDiscardedProjections(t *testing.T) {
	manager := alerts.NewManagerWithDataDir(t.TempDir(), alerts.WithoutPersistedAlertRestore())
	t.Cleanup(manager.Stop)
	start := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	snapshots := make([]alerts.Alert, 8)
	for i := range snapshots {
		opened := start.Add(time.Duration(i) * time.Minute)
		ack := opened.Add(time.Second)
		snapshots[i] = alerts.Alert{ID: fmt.Sprintf("import-%d", i), ResourceID: fmt.Sprintf("resource-%d", i),
			StartTime: opened, LastSeen: opened.Add(2 * time.Second), Type: "cpu", Level: alerts.AlertLevelWarning,
			Acknowledged: true, AckTime: &ack, AckUser: "synthetic-operator"}
	}
	importReplaySnapshots(t, manager, snapshots)
	store := unifiedresources.NewMemoryStore()
	adapter := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(store))
	history := &replayHistoryReadCounter{IncidentTimelineStore: adapter}
	incidents := memory.NewIncidentStore(memory.IncidentStoreConfig{})
	incidents.SetResourceTimelineStore(history)
	m := &Monitor{alertManager: manager, resourceStore: adapter, incidentStore: incidents}
	m.replayAlertLifecycleProjections(context.Background())
	m.replayAlertLifecycleProjections(context.Background()) // the watermark must make this a no-op
	reads := history.filteredReads.Load() + history.identityReads.Load() + history.recentReads.Load()
	t.Logf("imported=%d discarded-projection reads: filtered=%d identities=%d recent=%d", len(snapshots),
		history.filteredReads.Load(), history.identityReads.Load(), history.recentReads.Load())

	// Explicit user reads still hydrate canonical timelines, including with a
	// completely missing incident checkpoint. A writer optimisation must not
	// remove these reads or turn recovered evidence into empty history.
	recovered := memory.NewIncidentStore(memory.IncidentStoreConfig{})
	recovered.SetResourceTimelineStore(adapter)
	for _, alert := range snapshots {
		page, err := recovered.QueryIncidents(memory.IncidentQuery{AlertIdentifier: alert.ID, StartedAt: alert.StartTime})
		require.NoError(t, err)
		require.Len(t, page.Incidents, 1)
		incident := page.Incidents[0]
		require.Equal(t, memory.IncidentStatusResolved, incident.Status)
		require.Equal(t, &alert.LastSeen, incident.ClosedAt)
		require.True(t, incident.Acknowledged)
		require.Equal(t, alert.AckUser, incident.AckUser)
		require.Len(t, incident.Events, 3)
		require.Equal(t, "canonical_resource_history", page.History.Source)
		changes, err := store.GetRecentChanges(alert.ResourceID, time.Time{}, 10)
		require.NoError(t, err)
		require.Len(t, changes, 3, "replay must remain idempotent")
	}
	require.Zero(t, reads, "replay must not query a projection it discards")
}

type replayCheckpointRecorder struct {
	*unifiedresources.MonitorAdapter
	manager    *alerts.Manager
	imports    int
	checkpoint int64
}

func (s *replayCheckpointRecorder) RecordChange(change unifiedresources.ResourceChange) error {
	if change.Kind == unifiedresources.ChangeAlertFired {
		if s.imports == alertProjectionCheckpointEvery {
			s.checkpoint = s.manager.LifecycleProjectionWatermark(alertLifecycleProjectionConsumer)
		}
		s.imports++
	}
	return s.MonitorAdapter.RecordChange(change)
}

func TestImportedTimelineReplayCheckpointAndCacheRecovery(t *testing.T) {
	manager := alerts.NewManagerWithDataDir(t.TempDir(), alerts.WithoutPersistedAlertRestore())
	t.Cleanup(manager.Stop)
	start := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Second)
	snapshots := make([]alerts.Alert, alertProjectionCheckpointEvery+1)
	for i := range snapshots {
		opened := start.Add(time.Duration(i) * time.Second)
		snapshots[i] = alerts.Alert{ID: fmt.Sprintf("checkpoint-%d", i), ResourceID: "shared-resource",
			StartTime: opened, LastSeen: opened.Add(time.Second), Type: "cpu", Level: alerts.AlertLevelWarning}
	}
	importReplaySnapshots(t, manager, snapshots)
	store := unifiedresources.NewMemoryStore()
	adapter := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(store))
	recorder := &replayCheckpointRecorder{MonitorAdapter: adapter, manager: manager}
	incidents := memory.NewIncidentStore(memory.IncidentStoreConfig{MaxIncidents: 2})
	incidents.SetResourceTimelineStore(adapter)
	m := &Monitor{alertManager: manager, resourceStore: recorder, incidentStore: incidents}
	m.replayAlertLifecycleProjections(context.Background())
	require.Equal(t, len(snapshots), recorder.imports)
	require.Positive(t, recorder.checkpoint, "checkpoint must persist before the next import")
	complete := manager.LifecycleProjectionWatermark(alertLifecycleProjectionConsumer)
	require.Greater(t, complete, recorder.checkpoint)

	// Resume from the actually captured mid-pass checkpoint with a new bounded
	// cache. Only the remaining event is visited and its canonical rows dedupe.
	manager.StoreLifecycleProjectionWatermark(alertLifecycleProjectionConsumer, recorder.checkpoint)
	repaired := memory.NewIncidentStore(memory.IncidentStoreConfig{MaxIncidents: 2})
	repaired.SetResourceTimelineStore(adapter)
	resume := &Monitor{alertManager: manager, resourceStore: recorder, incidentStore: repaired}
	before := recorder.imports
	resume.replayAlertLifecycleProjections(context.Background())
	require.Equal(t, before+1, recorder.imports)
	require.Equal(t, complete, manager.LifecycleProjectionWatermark(alertLifecycleProjectionConsumer))
	changes, err := store.GetRecentChanges("shared-resource", time.Time{}, 2*len(snapshots)+1)
	require.NoError(t, err)
	require.Len(t, changes, 2*len(snapshots))

	// Both the evicted earliest occurrence and the resumed tail remain readable
	// from canonical evidence, not just from the two-entry snapshot cache.
	for _, i := range []int{0, len(snapshots) - 1} {
		alert := snapshots[i]
		page, err := repaired.QueryIncidents(memory.IncidentQuery{AlertIdentifier: alert.ID, StartedAt: alert.StartTime})
		require.NoError(t, err)
		require.Len(t, page.Incidents, 1)
		require.Equal(t, memory.IncidentStatusResolved, page.Incidents[0].Status)
		require.Len(t, page.Incidents[0].Events, 2)
	}
}
