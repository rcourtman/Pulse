package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/memory"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/eventlog"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/stretchr/testify/require"
)

// Hold a real replay at the exported canonical writer boundary. No database
// implementation, workload or timing assumption chooses the overlap.
type projectionShutdownRecorder struct {
	*unifiedresources.MonitorAdapter
	resourceID string
	entered    chan struct{}
	release    chan struct{}
	enterOnce  sync.Once
	writes     atomic.Int64
}

func (s *projectionShutdownRecorder) RecordChange(change unifiedresources.ResourceChange) error {
	if change.ResourceID == s.resourceID {
		s.enterOnce.Do(func() {
			close(s.entered)
			<-s.release
		})
		s.writes.Add(1)
	}
	return s.MonitorAdapter.RecordChange(change)
}

func blockedProjectionRecorder(t *testing.T) (*projectionShutdownRecorder, func()) {
	t.Helper()
	store := unifiedresources.NewMemoryStore()
	s := &projectionShutdownRecorder{
		MonitorAdapter: unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(store)),
		resourceID:     "shutdown-history-resource",
		entered:        make(chan struct{}),
		release:        make(chan struct{}),
	}
	var once sync.Once
	unblock := func() { once.Do(func() { close(s.release) }) }
	t.Cleanup(unblock)
	return s, unblock
}

func shutdownHistorySnapshots() []alerts.Alert {
	start := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	ack := start.Add(time.Second)
	return []alerts.Alert{{
		ID: "shutdown-history-alert", ResourceID: "shutdown-history-resource",
		Type: "cpu", Level: alerts.AlertLevelWarning,
		StartTime: start, LastSeen: start.Add(2 * time.Second),
		Acknowledged: true, AckTime: &ack, AckUser: "synthetic-operator",
	}}
}

func TestMonitorStartJoinsAlertProjectionBeforeReturning(t *testing.T) {
	configureAlertShutdownFixture(t)
	monitor, err := New(&config.Config{DataPath: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	recorder, unblock := blockedProjectionRecorder(t)
	importReplaySnapshots(t, monitor.GetAlertManager(), shutdownHistorySnapshots())
	monitor.SetResourceStore(recorder)
	select {
	case <-recorder.entered:
	case <-time.After(5 * time.Second):
		unblock()
		monitor.alertProjectionWG.Wait()
		monitor.Stop()
		t.Fatal("history replay did not reach the canonical writer")
	}

	// Replay starts during router attachment, before Start. Even a runtime
	// cancelled before startup must join that already-admitted writer.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		monitor.Start(ctx, nil)
	}()
	t.Cleanup(func() {
		unblock()
		select {
		case <-done:
			monitor.alertProjectionWG.Wait()
			monitor.Stop()
		case <-time.After(5 * time.Second):
			t.Error("monitor did not finish after releasing its history writer")
		}
	})
	select {
	case <-done:
		t.Error("monitor reported completion while alert-history replay could still write")
	case <-time.After(100 * time.Millisecond):
	}
	unblock()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("monitor failed to join its completed history writer")
	}
	monitor.alertProjectionWG.Wait()
	if got := recorder.writes.Load(); got != 3 {
		t.Fatalf("admitted snapshot wrote %d changes, want its complete fired/acknowledged/resolved occurrence", got)
	}
}

func projectionCancellation(m *Monitor) <-chan struct{} {
	m.alertProjectionMu.Lock()
	defer m.alertProjectionMu.Unlock()
	return m.alertProjectionCtx.Done()
}

func TestMonitorStopCheckpointsWholeAlertOccurrenceAndResumesTail(t *testing.T) {
	manager := alerts.NewManagerWithDataDir(t.TempDir(), alerts.WithoutPersistedAlertRestore())
	t.Cleanup(manager.Stop)
	eventDir := t.TempDir()
	logStore, err := eventlog.Open(eventDir)
	require.NoError(t, err)
	manager.SetEventLog(logStore)
	first := shutdownHistorySnapshots()[0]
	snapshots := make([]alerts.Alert, 3)
	for i := range snapshots {
		snapshot := first.Clone()
		snapshot.ID = fmt.Sprintf("shutdown-occurrence-%d", i)
		snapshot.StartTime = first.StartTime.Add(time.Duration(i) * time.Minute)
		ack := snapshot.StartTime.Add(time.Second)
		snapshot.AckTime, snapshot.LastSeen = &ack, snapshot.StartTime.Add(2*time.Second)
		snapshots[i] = *snapshot
		payload, err := json.Marshal(snapshot)
		require.NoError(t, err)
		require.NoError(t, logStore.ImportEvents([]eventlog.Event{{Type: eventlog.TypeHistoryImported,
			AlertID: snapshot.ID, ResourceID: snapshot.ResourceID, OccurredAt: snapshot.LastSeen, Snapshot: payload}}))
	}
	firstEvents, err := manager.AlertEvents(eventlog.Filter{AlertID: snapshots[0].ID})
	require.NoError(t, err)
	require.Len(t, firstEvents, 1)

	recorder, unblock := blockedProjectionRecorder(t)
	incidents := memory.NewIncidentStore(memory.IncidentStoreConfig{})
	incidents.SetResourceTimelineStore(recorder)
	m := &Monitor{alertManager: manager, incidentStore: incidents, resourceStore: recorder}
	m.scheduleAlertProjectionCatchUp()
	t.Cleanup(func() { unblock(); m.alertProjectionWG.Wait() })
	select {
	case <-recorder.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("replay did not reach its first canonical write")
	}
	cancelled := projectionCancellation(m)
	stopped := make(chan struct{})
	go func() { m.Stop(); close(stopped) }()
	select {
	case <-cancelled:
	case <-time.After(5 * time.Second):
		t.Fatal("Stop did not close replay admission")
	}
	select {
	case <-stopped:
		t.Fatal("Stop closed history while an occurrence was only partly applied")
	case <-time.After(100 * time.Millisecond):
	}
	unblock()
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("Stop did not join its completed occurrence")
	}
	require.EqualValues(t, 3, recorder.writes.Load(), "cancel only between occurrences, not between their three projections")

	// Stop really closed the log. Reopen through the public interface, then
	// attach a fresh monitor and cold incident cache to the retained evidence.
	resumedLog, err := eventlog.Open(eventDir)
	require.NoError(t, err)
	resumedManager := alerts.NewManagerWithDataDir(t.TempDir(), alerts.WithoutPersistedAlertRestore())
	resumedManager.SetEventLog(resumedLog)
	t.Cleanup(resumedManager.Stop)
	checkpoint := resumedManager.LifecycleProjectionWatermark(alertLifecycleProjectionConsumer)
	require.Equal(t, firstEvents[0].ID, checkpoint, "checkpoint only the fully applied first occurrence")
	resumedIncidents := memory.NewIncidentStore(memory.IncidentStoreConfig{})
	resumedIncidents.SetResourceTimelineStore(recorder)
	resumed := &Monitor{alertManager: resumedManager, incidentStore: resumedIncidents, resourceStore: recorder}
	resumed.scheduleAlertProjectionCatchUp()
	resumed.alertProjectionWG.Wait()
	require.EqualValues(t, 9, recorder.writes.Load(), "resume only the two unapplied occurrences")
	resumed.scheduleAlertProjectionCatchUp()
	resumed.alertProjectionWG.Wait()
	require.EqualValues(t, 9, recorder.writes.Load(), "completed replay must remain a no-op")
	for _, snapshot := range snapshots {
		page, err := resumedIncidents.QueryIncidents(memory.IncidentQuery{AlertIdentifier: snapshot.ID, StartedAt: snapshot.StartTime})
		require.NoError(t, err)
		require.Len(t, page.Incidents, 1)
		incident := page.Incidents[0]
		require.Equal(t, memory.IncidentStatusResolved, incident.Status)
		require.Equal(t, &snapshot.LastSeen, incident.ClosedAt)
		require.True(t, incident.Acknowledged)
		require.Equal(t, snapshot.AckUser, incident.AckUser)
		require.Len(t, incident.Events, 3)
	}
	t.Logf("interrupted watermark=%d; resumed watermark=%d; complete canonical writes=%d", checkpoint,
		resumedManager.LifecycleProjectionWatermark(alertLifecycleProjectionConsumer), recorder.writes.Load())
	resumed.Stop()
}

func TestAlertProjectionShutdownRefusesQueuedAndLateAdmissions(t *testing.T) {
	manager := alerts.NewManagerWithDataDir(t.TempDir(), alerts.WithoutPersistedAlertRestore())
	t.Cleanup(manager.Stop)
	importReplaySnapshots(t, manager, shutdownHistorySnapshots())
	recorder, unblock := blockedProjectionRecorder(t)
	unblock() // Only the replay serialization gate below holds these requests.
	m := &Monitor{alertManager: manager, incidentStore: memory.NewIncidentStore(memory.IncidentStoreConfig{}), resourceStore: recorder}
	m.alertProjectionReplayMu.Lock()
	m.scheduleAlertProjectionCatchUp()
	cancelled := projectionCancellation(m)
	stopped := make(chan struct{})
	go func() { m.stopAlertProjectionCatchUp(); close(stopped) }()
	var callers sync.WaitGroup
	for i := 0; i < 64; i++ {
		callers.Go(m.scheduleAlertProjectionCatchUp)
	}
	select {
	case <-cancelled:
	case <-time.After(5 * time.Second):
		m.alertProjectionReplayMu.Unlock()
		t.Fatal("shutdown did not cancel queued projection work")
	}
	callers.Wait()
	m.alertProjectionReplayMu.Unlock()
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("cancelled queued work did not join")
	}
	for i := 0; i < 64; i++ {
		m.scheduleAlertProjectionCatchUp()
	}
	m.alertProjectionWG.Wait()
	require.Zero(t, recorder.writes.Load(), "queued or late catch-up cannot revive a stopped writer")
	require.Zero(t, manager.LifecycleProjectionWatermark(alertLifecycleProjectionConsumer), "unapplied evidence must remain replayable")
}

func TestCancelledAlertReconciliationDoesNotReadOrWriteHistory(t *testing.T) {
	manager := alerts.NewManagerWithDataDir(t.TempDir(), alerts.WithoutPersistedAlertRestore())
	t.Cleanup(manager.Stop)
	cfg := manager.GetConfig()
	cfg.Enabled, cfg.ActivationState = true, alerts.ActivationPending
	cfg.TimeThresholds = map[string]int{}
	manager.UpdateConfig(cfg)
	vm := models.VM{ID: "cancelled-reconciliation", Name: "Synthetic VM", Node: "node", Instance: "pve", Status: "stopped"}
	manager.CheckGuest(vm, vm.Instance)
	manager.CheckGuest(vm, vm.Instance)
	require.Len(t, manager.GetActiveAlerts(), 1)
	recorder, unblock := blockedProjectionRecorder(t)
	unblock()
	history := &replayHistoryReadCounter{IncidentTimelineStore: recorder}
	incidents := memory.NewIncidentStore(memory.IncidentStoreConfig{})
	incidents.SetResourceTimelineStore(history)
	m := &Monitor{alertManager: manager, incidentStore: incidents, resourceStore: recorder}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	m.reconcileActiveAlertTimelines(ctx)
	require.Zero(t, history.filteredReads.Load()+history.identityReads.Load()+history.recentReads.Load())
	changes, err := recorder.GetRecentChanges(vm.ID, time.Time{}, 10)
	require.NoError(t, err)
	require.Empty(t, changes)
}

func TestTenantDeletionRetainsBlockedAlertProjectionThenRemovesCompletedRuntime(t *testing.T) {
	configureAlertShutdownFixture(t)
	shortenTenantShutdownBudgets(t, 100*time.Millisecond, 100*time.Millisecond)
	root := t.TempDir()
	persistence := config.NewMultiTenantPersistence(root)
	require.NoError(t, persistence.SaveOrganization(&models.Organization{ID: "history-owner", DisplayName: "History owner"}))
	manager := NewMultiTenantMonitor(&config.Config{DataPath: root}, persistence, nil)
	recorder, unblock := blockedProjectionRecorder(t)
	manager.SetMonitorInitializer(func(m *Monitor) {
		importReplaySnapshots(t, m.GetAlertManager(), shutdownHistorySnapshots())
		m.SetResourceStore(recorder)
	})
	monitor, err := manager.GetMonitor("history-owner")
	require.NoError(t, err)
	t.Cleanup(func() { unblock(); manager.Stop() })
	select {
	case <-recorder.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("tenant replay did not reach its canonical writer")
	}
	// The existing deletion budget must refuse a live replay writer without
	// closing its stores, dropping its owner or permitting lazy recreation.
	require.Error(t, manager.BeginTenantDeletion("history-owner"))
	retained, ok := manager.PeekMonitor("history-owner")
	require.True(t, ok)
	require.Same(t, monitor, retained)
	manager.FinishTenantDeletion("history-owner")
	_, err = manager.GetMonitor("history-owner")
	require.Error(t, err)
	orgPath := filepath.Join(root, "orgs", "history-owner")
	_, err = os.Stat(filepath.Join(orgPath, "org.json"))
	require.NoError(t, err, "blocked writer must retain organization bytes")
	unblock()
	require.NoError(t, manager.BeginTenantDeletion("history-owner"))
	require.NoError(t, persistence.DeleteOrganization("history-owner"))
	manager.FinishTenantDeletion("history-owner")
	_, ok = manager.PeekMonitor("history-owner")
	require.False(t, ok)
	_, err = manager.GetMonitor("history-owner")
	require.Error(t, err)
	_, err = os.Stat(orgPath)
	require.True(t, os.IsNotExist(err), "completed history writer must permit safe removal: %v", err)
	require.EqualValues(t, 3, recorder.writes.Load())
}
