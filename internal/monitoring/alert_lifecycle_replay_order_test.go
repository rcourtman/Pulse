package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/memory"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/eventlog"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/stretchr/testify/require"
)

const replayOrderOccurrences = 2*alertProjectionCheckpointEvery + 76

// disorderedReplaySnapshots models a legacy history import: entries are written
// in history order, but each one's event time is its final LastSeen, so every
// long-running (even) occurrence has a lower durable id and a later time than
// the brief (odd) occurrences imported after it.
func disorderedReplaySnapshots() []alerts.Alert {
	base := time.Now().UTC().Add(-30 * 24 * time.Hour).Truncate(time.Second)
	snapshots := make([]alerts.Alert, replayOrderOccurrences)
	for i := range snapshots {
		start := base.Add(time.Duration(i) * time.Minute)
		lastSeen := start.Add(2 * time.Second)
		if i%2 == 0 {
			lastSeen = base.Add(time.Duration(replayOrderOccurrences+i) * time.Minute)
		}
		ack := start.Add(time.Second)
		snapshots[i] = alerts.Alert{ID: fmt.Sprintf("replay-order-%d", i), ResourceID: fmt.Sprintf("replay-order-res-%d", i),
			Type: "cpu", Level: alerts.AlertLevelWarning, StartTime: start, LastSeen: lastSeen,
			Acknowledged: true, AckTime: &ack, AckUser: "synthetic-operator"}
	}
	return snapshots
}

// interruptingReplayRecorder stops a real replay pass part-way through.
// Graceful mode cancels the pass during one occurrence's first canonical write;
// the replay finishes that occurrence and records its normal cancellation
// watermark. Simulated-crash mode is not a native process crash: once the
// replay's own mid-pass checkpoint is durable, it cancels the pass and
// discards every later canonical write, and the test then rolls the watermark
// back to that checkpoint, which is the state a process killed before the
// pass's final watermark store would leave behind.
type interruptingReplayRecorder struct {
	*unifiedresources.MonitorAdapter
	manager          *alerts.Manager
	cancel           context.CancelFunc
	simulateCrash    bool
	cancelAfterFired int
	fired            int
	interrupted      bool
	checkpoint       int64
}

func (s *interruptingReplayRecorder) RecordChange(change unifiedresources.ResourceChange) error {
	if s.simulateCrash && s.interrupted {
		return nil // written after the checkpoint by a process that never finished
	}
	if change.Kind == unifiedresources.ChangeAlertFired {
		s.fired++
		if s.simulateCrash {
			if checkpoint := s.manager.LifecycleProjectionWatermark(alertLifecycleProjectionConsumer); checkpoint > 0 {
				s.checkpoint, s.interrupted = checkpoint, true
				s.cancel()
				return nil
			}
		} else if s.fired == s.cancelAfterFired {
			s.interrupted = true
			s.cancel()
		}
	}
	return s.MonitorAdapter.RecordChange(change)
}

func TestInterruptedDisorderedImportReplayResumesEveryOccurrence(t *testing.T) {
	for _, tc := range []struct {
		name          string
		simulateCrash bool
	}{
		{name: "graceful cancellation past the checkpoint boundary"},
		{name: "simulated crash after the mid-pass checkpoint", simulateCrash: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var webhookRequests atomic.Int64
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				webhookRequests.Add(1)
				w.WriteHeader(http.StatusOK)
			}))
			t.Cleanup(server.Close)
			notifier := notifications.NewNotificationManagerWithDataDir("http://pulse.example", t.TempDir())
			t.Cleanup(notifier.Stop)
			require.NoError(t, notifier.UpdateAllowedPrivateCIDRs("127.0.0.1/32,::1/128"))
			notifier.AddWebhook(notifications.WebhookConfig{ID: "replay-order", URL: server.URL, Enabled: true, Service: "generic"})
			notifier.SetGroupingWindow(0)

			manager := alerts.NewManagerWithDataDir(t.TempDir(), alerts.WithoutPersistedAlertRestore())
			t.Cleanup(manager.Stop)
			snapshots := disorderedReplaySnapshots()
			importReplaySnapshots(t, manager, snapshots)

			// Production delivery wiring: any lifecycle callback during replay
			// would reach the webhook through the notification manager.
			var deliveryCallbacks atomic.Int64
			delivery := &Monitor{alertManager: manager, notificationMgr: notifier}
			manager.SetAlertCallback(func(alert *alerts.Alert) { deliveryCallbacks.Add(1); delivery.handleAlertFired(alert) })
			manager.SetResolvedCallback(func(id string) { deliveryCallbacks.Add(1); delivery.handleAlertResolved(id) })

			canonical := unifiedresources.NewMemoryStore()
			adapter := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(canonical))
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			recorder := &interruptingReplayRecorder{MonitorAdapter: adapter, manager: manager, cancel: cancel,
				simulateCrash: tc.simulateCrash, cancelAfterFired: alertProjectionCheckpointEvery + 188}
			interruptedIncidents := memory.NewIncidentStore(memory.IncidentStoreConfig{})
			interruptedIncidents.SetResourceTimelineStore(recorder)
			first := &Monitor{alertManager: manager, incidentStore: interruptedIncidents, resourceStore: recorder, notificationMgr: notifier}
			first.replayAlertLifecycleProjections(ctx)
			require.True(t, recorder.interrupted, "the first pass must stop part-way")

			watermark := manager.LifecycleProjectionWatermark(alertLifecycleProjectionConsumer)
			if tc.simulateCrash {
				// Event ids are the import order, so the checkpoint covers
				// exactly the first 512 occurrences and nothing beyond them.
				require.EqualValues(t, alertProjectionCheckpointEvery, recorder.checkpoint)
				manager.StoreLifecycleProjectionWatermark(alertLifecycleProjectionConsumer, recorder.checkpoint)
				watermark = recorder.checkpoint
			} else {
				require.EqualValues(t, recorder.cancelAfterFired, watermark,
					"cancellation records only the contiguous prefix it completed")
			}
			require.Less(t, watermark, int64(len(snapshots)))

			// Resume with a cold monitor and incident cache over the same
			// canonical history, as the next process start would.
			resumedIncidents := memory.NewIncidentStore(memory.IncidentStoreConfig{})
			resumedIncidents.SetResourceTimelineStore(adapter)
			resumed := &Monitor{alertManager: manager, incidentStore: resumedIncidents, resourceStore: adapter, notificationMgr: notifier}
			resumed.replayAlertLifecycleProjections(context.Background())
			require.EqualValues(t, len(snapshots), manager.LifecycleProjectionWatermark(alertLifecycleProjectionConsumer))

			readBack := memory.NewIncidentStore(memory.IncidentStoreConfig{})
			readBack.SetResourceTimelineStore(adapter)
			for _, alert := range snapshots {
				changes, err := canonical.GetRecentChanges(alert.ResourceID, time.Time{}, 10)
				require.NoError(t, err)
				kinds := map[unifiedresources.ChangeKind]int{}
				for _, change := range changes {
					kinds[change.Kind]++
				}
				require.Equal(t, map[unifiedresources.ChangeKind]int{
					unifiedresources.ChangeAlertFired:        1,
					unifiedresources.ChangeAlertAcknowledged: 1,
					unifiedresources.ChangeAlertResolved:     1,
				}, kinds, "occurrence %s must be projected exactly once", alert.ID)

				page, err := readBack.QueryIncidents(memory.IncidentQuery{AlertIdentifier: alert.ID, StartedAt: alert.StartTime})
				require.NoError(t, err)
				require.Len(t, page.Incidents, 1, alert.ID)
				incident := page.Incidents[0]
				require.Equal(t, memory.IncidentStatusResolved, incident.Status, alert.ID)
				require.Equal(t, &alert.LastSeen, incident.ClosedAt, alert.ID)
				require.True(t, incident.Acknowledged, alert.ID)
				require.Equal(t, alert.AckUser, incident.AckUser, alert.ID)
				require.Len(t, incident.Events, 3, alert.ID)
			}

			require.Zero(t, deliveryCallbacks.Load(), "replay must not invoke delivery callbacks")
			stats, err := notifier.GetQueueStats()
			require.NoError(t, err)
			require.Empty(t, stats, "replay must not queue notifications")
			time.Sleep(100 * time.Millisecond)
			require.Zero(t, webhookRequests.Load(), "replay must not deliver notifications")
			t.Logf("interrupted watermark=%d; resumed watermark=%d; occurrences=%d",
				watermark, manager.LifecycleProjectionWatermark(alertLifecycleProjectionConsumer), len(snapshots))
		})
	}
}

// replayDeliveryProbe wires the production fired/resolved delivery path to a
// counting loopback webhook, so any notification caused by replay is visible.
type replayDeliveryProbe struct {
	notifier  *notifications.NotificationManager
	callbacks atomic.Int64
	requests  atomic.Int64
}

func newReplayDeliveryProbe(t *testing.T) *replayDeliveryProbe {
	t.Helper()
	probe := &replayDeliveryProbe{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		probe.requests.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	probe.notifier = notifications.NewNotificationManagerWithDataDir("http://pulse.example", t.TempDir())
	t.Cleanup(probe.notifier.Stop)
	require.NoError(t, probe.notifier.UpdateAllowedPrivateCIDRs("127.0.0.1/32,::1/128"))
	probe.notifier.AddWebhook(notifications.WebhookConfig{ID: "replay-upgrade", URL: server.URL, Enabled: true, Service: "generic"})
	probe.notifier.SetGroupingWindow(0)
	return probe
}

func (p *replayDeliveryProbe) attach(manager *alerts.Manager) {
	delivery := &Monitor{alertManager: manager, notificationMgr: p.notifier}
	manager.SetAlertCallback(func(alert *alerts.Alert) { p.callbacks.Add(1); delivery.handleAlertFired(alert) })
	manager.SetResolvedCallback(func(id string) { p.callbacks.Add(1); delivery.handleAlertResolved(id) })
}

func (p *replayDeliveryProbe) requireNoDelivery(t *testing.T) {
	t.Helper()
	require.Zero(t, p.callbacks.Load(), "replay must not invoke delivery callbacks")
	stats, err := p.notifier.GetQueueStats()
	require.NoError(t, err)
	require.Empty(t, stats, "replay must not queue notifications")
	time.Sleep(100 * time.Millisecond)
	require.Zero(t, p.requests.Load(), "replay must not deliver notifications")
}

// countingReplayRecorder counts canonical writes made by a replay pass.
type countingReplayRecorder struct {
	*unifiedresources.MonitorAdapter
	writes atomic.Int64
}

func (s *countingReplayRecorder) RecordChange(change unifiedresources.ResourceChange) error {
	s.writes.Add(1)
	return s.MonitorAdapter.RecordChange(change)
}

func canonicalAlertKinds(t *testing.T, canonical *unifiedresources.MemoryStore, resourceID string) map[unifiedresources.ChangeKind]int {
	t.Helper()
	changes, err := canonical.GetRecentChanges(resourceID, time.Time{}, 10)
	require.NoError(t, err)
	kinds := map[unifiedresources.ChangeKind]int{}
	for _, change := range changes {
		kinds[change.Kind]++
	}
	return kinds
}

func requireEveryOccurrenceProjectedOnce(t *testing.T, canonical *unifiedresources.MemoryStore, adapter *unifiedresources.MonitorAdapter, snapshots []alerts.Alert) {
	t.Helper()
	readBack := memory.NewIncidentStore(memory.IncidentStoreConfig{})
	readBack.SetResourceTimelineStore(adapter)
	for _, alert := range snapshots {
		require.Equal(t, map[unifiedresources.ChangeKind]int{
			unifiedresources.ChangeAlertFired:        1,
			unifiedresources.ChangeAlertAcknowledged: 1,
			unifiedresources.ChangeAlertResolved:     1,
		}, canonicalAlertKinds(t, canonical, alert.ResourceID), "occurrence %s must be projected exactly once", alert.ID)
		page, err := readBack.QueryIncidents(memory.IncidentQuery{AlertIdentifier: alert.ID, StartedAt: alert.StartTime})
		require.NoError(t, err)
		require.Len(t, page.Incidents, 1, alert.ID)
		incident := page.Incidents[0]
		require.Equal(t, memory.IncidentStatusResolved, incident.Status, alert.ID)
		require.Equal(t, &alert.LastSeen, incident.ClosedAt, alert.ID)
		require.True(t, incident.Acknowledged, alert.ID)
		require.Equal(t, alert.AckUser, incident.AckUser, alert.ID)
		require.Len(t, incident.Events, 3, alert.ID)
	}
}

// The release before ID-ordered replay stored its cursor under the bare
// consumer name. This test seeds that persisted state directly (it does not
// run the old binary): the first part of a time-ordered pass is projected and
// the legacy cursor records the highest id it reached, above lower-id
// occurrences that pass never visited.
func TestUpgradeIgnoresUnsafeLegacyReplayCursorAndBackfillsRetainedEvents(t *testing.T) {
	probe := newReplayDeliveryProbe(t)
	eventDir := t.TempDir()
	store, err := eventlog.Open(eventDir)
	require.NoError(t, err)
	manager := alerts.NewManagerWithDataDir(t.TempDir(), alerts.WithoutPersistedAlertRestore())
	t.Cleanup(manager.Stop)
	manager.SetEventLog(store) // the manager owns Close
	snapshots := disorderedReplaySnapshots()
	imports := make([]eventlog.Event, len(snapshots))
	for i, alert := range snapshots {
		payload, err := json.Marshal(alert)
		require.NoError(t, err)
		imports[i] = eventlog.Event{Type: eventlog.TypeHistoryImported, AlertID: alert.ID,
			ResourceID: alert.ResourceID, OccurredAt: alert.LastSeen, Snapshot: payload}
	}
	require.NoError(t, store.ImportEvents(imports))

	canonical := unifiedresources.NewMemoryStore()
	adapter := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(canonical))
	seededIncidents := memory.NewIncidentStore(memory.IncidentStoreConfig{})
	seededIncidents.SetResourceTimelineStore(adapter)
	legacy := &Monitor{alertManager: manager, incidentStore: seededIncidents, resourceStore: adapter}
	const legacyVisited = alertProjectionCheckpointEvery + 88
	var legacyCursor int64
	visited := 0
	require.NoError(t, store.WalkOldest(eventlog.Filter{Types: []string{eventlog.TypeHistoryImported}}, func(event eventlog.Event) error {
		if visited == legacyVisited {
			return nil
		}
		var alert alerts.Alert
		require.NoError(t, json.Unmarshal(event.Snapshot, &alert))
		legacy.projectAlertLifecycleEvent(alerts.LifecycleEvent{Type: event.Type, OccurredAt: event.OccurredAt,
			Alert: &alert, Persisted: true}, false)
		legacyCursor = max(legacyCursor, event.ID)
		visited++
		return nil
	}))
	require.NoError(t, store.SetProjectionWatermark(alertLifecycleProjectionConsumer, legacyCursor))
	require.NoError(t, store.SetProjectionWatermark("unrelated-consumer", 7))
	skipped := 0
	for i, alert := range snapshots {
		if int64(i+1) < legacyCursor && len(canonicalAlertKinds(t, canonical, alert.ResourceID)) == 0 {
			skipped++
		}
	}
	require.Positive(t, skipped, "seeded legacy cursor must sit above unprojected lower-id occurrences")

	// Upgrade: the ID-ordered cursor does not inherit the legacy value.
	require.Zero(t, manager.LifecycleProjectionWatermark(alertLifecycleProjectionConsumer))
	probe.attach(manager)
	upgradedIncidents := memory.NewIncidentStore(memory.IncidentStoreConfig{})
	upgradedIncidents.SetResourceTimelineStore(adapter)
	upgraded := &Monitor{alertManager: manager, incidentStore: upgradedIncidents, resourceStore: adapter, notificationMgr: probe.notifier}
	upgraded.replayAlertLifecycleProjections(context.Background())
	requireEveryOccurrenceProjectedOnce(t, canonical, adapter, snapshots)
	require.EqualValues(t, len(snapshots), manager.LifecycleProjectionWatermark(alertLifecycleProjectionConsumer))
	require.Equal(t, legacyCursor, store.ProjectionWatermark(alertLifecycleProjectionConsumer), "legacy key is left untouched")
	require.EqualValues(t, 7, store.ProjectionWatermark("unrelated-consumer"))
	manager.Stop()

	// The new cursor survives reopening the event log.
	reopened, err := eventlog.Open(eventDir)
	require.NoError(t, err)
	restarted := alerts.NewManagerWithDataDir(t.TempDir(), alerts.WithoutPersistedAlertRestore())
	t.Cleanup(restarted.Stop)
	restarted.SetEventLog(reopened)
	probe.attach(restarted)
	require.EqualValues(t, len(snapshots), restarted.LifecycleProjectionWatermark(alertLifecycleProjectionConsumer))
	require.Equal(t, legacyCursor, reopened.ProjectionWatermark(alertLifecycleProjectionConsumer))
	require.EqualValues(t, 7, reopened.ProjectionWatermark("unrelated-consumer"))

	// The next pass has nothing left to replay.
	counter := &countingReplayRecorder{MonitorAdapter: adapter}
	quietIncidents := memory.NewIncidentStore(memory.IncidentStoreConfig{})
	quietIncidents.SetResourceTimelineStore(counter)
	next := &Monitor{alertManager: restarted, incidentStore: quietIncidents, resourceStore: counter, notificationMgr: probe.notifier}
	next.replayAlertLifecycleProjections(context.Background())
	require.Zero(t, counter.writes.Load(), "a completed migration must not replay again")
	require.EqualValues(t, len(snapshots), restarted.LifecycleProjectionWatermark(alertLifecycleProjectionConsumer))

	// Resetting the cursor to zero still requests a full replay, and replaying
	// over existing canonical event identities stays idempotent.
	restarted.StoreLifecycleProjectionWatermark(alertLifecycleProjectionConsumer, 0)
	require.Zero(t, restarted.LifecycleProjectionWatermark(alertLifecycleProjectionConsumer))
	next.replayAlertLifecycleProjections(context.Background())
	require.Positive(t, counter.writes.Load(), "reset must replay the retained log")
	requireEveryOccurrenceProjectedOnce(t, canonical, adapter, snapshots)
	require.EqualValues(t, len(snapshots), restarted.LifecycleProjectionWatermark(alertLifecycleProjectionConsumer))
	require.Equal(t, legacyCursor, reopened.ProjectionWatermark(alertLifecycleProjectionConsumer))
	require.EqualValues(t, 7, reopened.ProjectionWatermark("unrelated-consumer"))
	probe.requireNoDelivery(t)
	t.Logf("legacy cursor=%d with %d lower-id occurrences unprojected; ID-ordered cursor after upgrade=%d; reset replay writes=%d",
		legacyCursor, skipped, restarted.LifecycleProjectionWatermark(alertLifecycleProjectionConsumer), counter.writes.Load())
}
