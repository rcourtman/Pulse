package api

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/stretchr/testify/require"
)

func TestPatrolFindingNotificationLifecycleRoutesOnlyOwningProducer(t *testing.T) {
	start := time.Now().Add(-time.Hour).Round(0)
	defaultStore, tenantStore := ai.NewFindingsStore(), ai.NewFindingsStore()
	newFinding := func() *ai.Finding {
		return &ai.Finding{ID: "same-finding", ResourceID: "fixture-host", Title: "Space above threshold",
			Severity: ai.FindingSeverityWarning, Category: ai.FindingCategoryCapacity, DetectedAt: start}
	}
	require.True(t, defaultStore.Add(newFinding()))
	require.True(t, tenantStore.Add(newFinding()))
	require.True(t, defaultStore.Resolve("same-finding", false))
	fallbackCalls := 0
	fallback := func(id string, at time.Time) alerts.NotificationOccurrenceLifecycle {
		fallbackCalls++
		require.Equal(t, "threshold-owned", id)
		require.Equal(t, start, at)
		return alerts.NotificationOccurrenceLifecycle{State: alerts.NotificationOccurrenceActive}
	}
	defaultLifecycle := patrolFindingNotificationLifecycle(defaultStore, fallback)
	tenantLifecycle := patrolFindingNotificationLifecycle(tenantStore, fallback)
	projected := patrolFindingNotificationAlert(tenantStore.Get("same-finding"))
	require.Equal(t, alerts.NotificationOccurrenceResolved, defaultLifecycle(projected.ID, projected.StartTime).State)
	require.Equal(t, alerts.NotificationOccurrenceActive, tenantLifecycle(projected.ID, projected.StartTime).State)
	require.Zero(t, fallbackCalls, "a finding must not borrow threshold or another organization's authority")
	require.Equal(t, alerts.NotificationOccurrenceActive, tenantLifecycle("threshold-owned", start).State)
	require.Equal(t, 1, fallbackCalls)

	missing := patrolFindingNotificationLifecycle(nil, fallback)
	state := missing(projected.ID, start)
	require.Equal(t, alerts.NotificationOccurrenceUnavailable, state.State)
	require.False(t, state.CurrentInactive)
	require.Equal(t, 1, fallbackCalls)
	state = tenantLifecycle(projected.ID, start.Add(time.Nanosecond))
	require.Equal(t, alerts.NotificationOccurrenceUnavailable, state.State)
	require.False(t, state.CurrentInactive)
	require.Equal(t, alerts.NotificationOccurrenceUnavailable, patrolFindingNotificationLifecycle(nil, nil)("threshold-owned", start).State)
}
