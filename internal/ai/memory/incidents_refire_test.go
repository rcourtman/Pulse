package memory

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/stretchr/testify/require"
)

func TestIncidentRefireCheckpointPreservesTransitionOrder(t *testing.T) {
	dir := t.TempDir()
	config := IncidentStoreConfig{DataDir: dir}
	store := NewIncidentStore(config)
	start := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	alert := &alerts.Alert{ID: "retained-node", ResourceID: "node", StartTime: start}
	resolved := start.Add(time.Minute)
	refired := start.Add(2 * time.Minute)
	store.RecordAlertFired(alert)
	store.RecordAlertResolved(alert, resolved)
	store.RecordAlertRefired(alert, refired)
	store.flush()
	store = NewIncidentStore(config)
	t.Cleanup(store.flush)
	for i := 0; i < 10; i++ {
		store.RecordAlertFired(alert)
		store.RecordAlertResolved(alert, resolved)
		store.RecordAlertRefired(alert, refired)
	}
	store.EnsureAlertOccurrence(alert, &resolved)
	current := store.GetTimelineByAlertAt(alert.ID, start)
	require.NotNil(t, current)
	require.Equal(t, IncidentStatusOpen, current.Status)
	require.Nil(t, current.ClosedAt)
	require.Len(t, current.Events, 3)
	finalResolution := start.Add(3 * time.Minute)
	store.RecordAlertResolved(alert, finalResolution)
	store.flush()
	store = NewIncidentStore(config)
	t.Cleanup(store.flush)
	store.RecordAlertRefired(alert, refired)
	current = store.GetTimelineByAlertAt(alert.ID, start)
	require.Equal(t, IncidentStatusResolved, current.Status)
	require.Equal(t, &finalResolution, current.ClosedAt)
	require.Len(t, current.Events, 4)
}
