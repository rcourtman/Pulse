package memory

import (
	"os"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/stretchr/testify/require"
)

func TestIncidentMaterializationPreservesSnapshotCheckpointAndReadRepair(t *testing.T) {
	for _, canonical := range []bool{false, true} {
		name := "local"
		if canonical {
			name = "canonical"
		}
		t.Run(name, func(t *testing.T) {
			cfg := IncidentStoreConfig{DataDir: t.TempDir(), MaxIncidents: 1, MaxAgeDays: 30}
			store := NewIncidentStore(cfg)
			t.Cleanup(store.flush)
			var history *unifiedresources.MemoryStore
			if canonical {
				history = unifiedresources.NewMemoryStore()
				store.SetResourceTimelineStore(history)
			}
			start := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
			ack, resolved := start.Add(time.Minute), start.Add(2*time.Minute)
			alert := &alerts.Alert{ID: "snapshot", ResourceID: "resource", Type: "cpu", StartTime: start,
				LastSeen: resolved, Acknowledged: true, AckTime: &ack, AckUser: "operator"}
			store.MaterializeAlertOccurrence(alert, &resolved)
			store.flush()
			first := store.GetTimelineByAlertAt(alert.ID, start)
			require.NotNil(t, first)
			require.Len(t, first.Events, 3)
			require.Equal(t, IncidentStatusResolved, first.Status)
			require.True(t, first.Acknowledged)
			require.Equal(t, alert.AckUser, first.AckUser)
			require.Equal(t, &resolved, first.ClosedAt)
			before, err := os.Stat(store.filePath)
			require.NoError(t, err)
			for i := 0; i < 8; i++ {
				store.MaterializeAlertOccurrence(alert, &resolved)
			}
			store.flush()
			after, err := os.Stat(store.filePath)
			require.NoError(t, err)
			require.True(t, os.SameFile(before, after), "unchanged materialization must not rewrite its checkpoint")
			reloaded := NewIncidentStore(cfg)
			t.Cleanup(reloaded.flush)
			if canonical {
				reloaded.SetResourceTimelineStore(history)
			}
			require.Equal(t, first, reloaded.GetTimelineByAlertAt(alert.ID, start))

			// Eviction is still bounded and the existing read-facing method can
			// repair a requested retained occurrence beyond that cache.
			newer := &alerts.Alert{ID: "newer", ResourceID: "other-resource", StartTime: start.Add(10 * time.Minute)}
			reloaded.MaterializeAlertOccurrence(newer, nil)
			require.Len(t, reloaded.incidents, 1)
			require.Equal(t, newer.ID, reloaded.incidents[0].AlertIdentifier)
			repaired := reloaded.EnsureAlertOccurrence(alert, &resolved)
			require.NotNil(t, repaired)
			require.Len(t, repaired.Events, 3)
			require.Len(t, reloaded.incidents, 1)
			require.Equal(t, alert.ID, reloaded.incidents[0].AlertIdentifier)

			// Age retention is not replaced by count-only retention.
			expired := &alerts.Alert{ID: "expired", StartTime: start.Add(-45 * 24 * time.Hour)}
			expiredEnd := expired.StartTime.Add(time.Minute)
			reloaded.MaterializeAlertOccurrence(expired, &expiredEnd)
			require.Len(t, reloaded.incidents, 1)
			require.Equal(t, alert.ID, reloaded.incidents[0].AlertIdentifier)
		})
	}
}

func TestIncidentMaterializationDoesNotMaskReadFailure(t *testing.T) {
	store := NewIncidentStore(IncidentStoreConfig{DataDir: t.TempDir()})
	t.Cleanup(store.flush)
	store.SetResourceTimelineStore(failingIncidentHistory{unifiedresources.NewMemoryStore()})
	alert := &alerts.Alert{ID: "read-failure", ResourceID: "resource", StartTime: time.Now().UTC().Add(-time.Minute)}
	store.MaterializeAlertOccurrence(alert, nil)
	store.flush()
	require.Len(t, store.incidents, 1, "the writer still creates the recovery shell")
	_, err := store.QueryIncidents(IncidentQuery{AlertIdentifier: alert.ID})
	require.ErrorContains(t, err, "history read denied", "an actual read must still expose unavailable canonical history")
	require.Nil(t, store.EnsureAlertOccurrence(alert, nil))
	store.SetResourceTimelineStore(unifiedresources.NewMemoryStore())
	repaired := store.EnsureAlertOccurrence(alert, nil)
	require.NotNil(t, repaired)
	require.Len(t, repaired.Events, 1)
}

func TestIncidentMaterializationRejectsInvalidSnapshots(t *testing.T) {
	var missing *IncidentStore
	missing.MaterializeAlertOccurrence(&alerts.Alert{ID: "valid"}, nil)
	require.Nil(t, missing.EnsureAlertOccurrence(&alerts.Alert{ID: "valid"}, nil))
	store := NewIncidentStore(IncidentStoreConfig{})
	for _, alert := range []*alerts.Alert{nil, {}, {ID: " \t"}} {
		store.MaterializeAlertOccurrence(alert, nil)
		require.Nil(t, store.EnsureAlertOccurrence(alert, nil))
	}
	require.Empty(t, store.incidents)
}
