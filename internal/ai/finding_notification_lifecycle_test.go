package ai

import (
	"errors"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/stretchr/testify/require"
)

func lifecycleFinding(start time.Time) *Finding {
	return &Finding{ID: "capacity-fixture", ResourceID: "fixture-host", Title: "Storage above threshold",
		Severity: FindingSeverityWarning, Category: FindingCategoryCapacity, DetectedAt: start}
}

func TestFindingNotificationLifecycleRequiresExactAuthority(t *testing.T) {
	start := time.Now().Add(-time.Hour).Round(0)
	var nilStore *FindingsStore
	require.Equal(t, alerts.NotificationOccurrenceUnavailable, nilStore.NotificationOccurrenceLifecycle("id", start).State)
	s := NewFindingsStore()
	for _, query := range []struct {
		id    string
		start time.Time
	}{{"", start}, {"missing", start}, {"missing", time.Time{}}, {"", time.Time{}}} {
		state := s.NotificationOccurrenceLifecycle(query.id, query.start)
		require.Equal(t, alerts.NotificationOccurrenceUnavailable, state.State)
		require.False(t, state.CurrentInactive)
	}
	require.True(t, s.Add(lifecycleFinding(start)))
	state := s.NotificationOccurrenceLifecycle("capacity-fixture", start)
	require.Equal(t, alerts.NotificationOccurrenceActive, state.State)
	require.Equal(t, start, state.LatestActivationAt)
	require.False(t, state.CurrentInactive)
	state = s.NotificationOccurrenceLifecycle("capacity-fixture", start.Add(time.Nanosecond))
	require.Equal(t, alerts.NotificationOccurrenceUnavailable, state.State)
	require.False(t, state.CurrentInactive)
}

func TestFindingNotificationLifecycleIntentDoesNotResolve(t *testing.T) {
	for _, intent := range []string{"expected_behavior", "not_an_issue", DismissReasonWillFixLater, "snoozed", "suppressed", "acknowledged"} {
		t.Run(intent, func(t *testing.T) {
			s := NewFindingsStore()
			start := time.Now().Add(-40 * 24 * time.Hour).Round(0)
			require.True(t, s.Add(lifecycleFinding(start)))
			switch intent {
			case "snoozed":
				require.True(t, s.Snooze("capacity-fixture", time.Hour))
			case "suppressed":
				require.True(t, s.Suppress("capacity-fixture"))
			case "acknowledged":
				require.True(t, s.Acknowledge("capacity-fixture"))
			default:
				require.True(t, s.Dismiss("capacity-fixture", intent, "fixture intent"))
			}
			state := s.NotificationOccurrenceLifecycle("capacity-fixture", start)
			require.Equal(t, alerts.NotificationOccurrenceActive, state.State, "an uncompleted receipt obligation survives operator intent")
			require.False(t, state.CurrentInactive)
			require.True(t, state.ResolvedAt.IsZero())
		})
	}
}

func TestFindingNotificationLifecycleResolutionAndRegression(t *testing.T) {
	s := NewFindingsStore()
	start := time.Now().Add(-time.Hour).Round(0)
	require.True(t, s.Add(lifecycleFinding(start)))
	require.True(t, s.Resolve("capacity-fixture", false))
	resolved := s.Get("capacity-fixture")
	state := s.NotificationOccurrenceLifecycle(resolved.ID, start)
	require.Equal(t, alerts.NotificationOccurrenceResolved, state.State)
	require.True(t, state.CurrentInactive)
	require.Equal(t, *resolved.ResolvedAt, state.ResolvedAt)
	// Finding identity/DetectedAt survive regression. The new activation must
	// override the earlier terminal snapshot, not settle it by elapsed age.
	require.False(t, s.Add(lifecycleFinding(start)))
	regressed := s.Get("capacity-fixture")
	state = s.NotificationOccurrenceLifecycle(regressed.ID, start)
	require.Equal(t, alerts.NotificationOccurrenceActive, state.State)
	require.Equal(t, *regressed.LastRegressionAt, state.LatestActivationAt)
	require.True(t, state.ResolvedAt.IsZero())

	// Malformed imported ordering cannot turn a later activation into an end.
	s.mu.Lock()
	s.findings[regressed.ID].ResolvedAt = resolved.ResolvedAt
	s.mu.Unlock()
	require.Equal(t, alerts.NotificationOccurrenceUnavailable, s.NotificationOccurrenceLifecycle(regressed.ID, start).State)
}

type notificationLifecycleLoadingPersistence struct {
	entered, release chan struct{}
}

func (*notificationLifecycleLoadingPersistence) SaveFindings(map[string]*Finding) error { return nil }
func (p *notificationLifecycleLoadingPersistence) LoadFindings() (map[string]*Finding, error) {
	close(p.entered)
	<-p.release
	return map[string]*Finding{}, nil
}

func TestFindingNotificationLifecycleLoadAndCompaction(t *testing.T) {
	s := NewFindingsStore()
	start := time.Now().Add(-40 * 24 * time.Hour).Round(0)
	end := start.Add(time.Hour)
	f := lifecycleFinding(start)
	f.ResolvedAt = &end
	p := &notificationLifecycleLoadingPersistence{make(chan struct{}), make(chan struct{})}
	done := make(chan error, 1)
	go func() { done <- s.SetPersistence(p) }()
	<-p.entered
	state := s.NotificationOccurrenceLifecycle(f.ID, start)
	require.Equal(t, alerts.NotificationOccurrenceUnavailable, state.State)
	require.False(t, state.CurrentInactive)
	close(p.release)
	require.NoError(t, <-done)
	state = s.NotificationOccurrenceLifecycle(f.ID, start)
	require.Equal(t, alerts.NotificationOccurrenceNoRecord, state.State)
	require.True(t, state.CurrentInactive)
	require.True(t, state.ResolvedAt.IsZero(), "complete absence still supplies no resolution")

	require.NoError(t, s.SetPersistence(&recordingPersistence{findings: map[string]*Finding{f.ID: f}}))
	require.Equal(t, alerts.NotificationOccurrenceResolved, s.NotificationOccurrenceLifecycle(f.ID, start).State)
	require.Equal(t, 1, s.Cleanup(24*time.Hour))
	state = s.NotificationOccurrenceLifecycle(f.ID, start)
	require.Equal(t, alerts.NotificationOccurrenceNoRecord, state.State)
	require.True(t, state.ResolvedAt.IsZero())

	loadErr := errors.New("fixture finding load unavailable")
	require.ErrorIs(t, s.SetPersistence(&recordingPersistence{loadErr: loadErr}), loadErr)
	state = s.NotificationOccurrenceLifecycle(f.ID, start)
	require.Equal(t, alerts.NotificationOccurrenceUnavailable, state.State)
	require.False(t, state.CurrentInactive)
}
