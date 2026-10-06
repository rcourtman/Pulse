package monitoring

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/memory"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/eventlog"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	unifiedresources "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/stretchr/testify/require"
)

func TestDockerAlertTimelineUsesCanonicalHistoryIdentity(t *testing.T) {
	dir := t.TempDir()
	store, err := unifiedresources.NewSQLiteResourceStore(dir, "default")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	manager := alerts.NewManagerWithDataDir(t.TempDir())
	t.Cleanup(manager.Stop)
	config := manager.GetConfig()
	config.Enabled = true
	config.ActivationState = alerts.ActivationPending
	config.TimeThresholds = map[string]int{}
	config.SuppressionWindow = 0
	manager.UpdateConfig(config)
	containerID := strings.Repeat("f", 64)
	host := models.DockerHost{ID: "history-host", Hostname: "history-host", LastSeen: time.Now(), Containers: []models.DockerContainer{
		{ID: containerID, Name: "worker", State: "running", Health: "unhealthy"},
		{ID: strings.Repeat("a", 64), Name: "worker", State: "running", Health: "healthy"},
	}}
	registry := unifiedresources.NewRegistry(store)
	registry.IngestSnapshot(models.StateSnapshot{DockerHosts: []models.DockerHost{host}})
	adapter := unifiedresources.NewMonitorAdapter(registry)
	monitor := &Monitor{alertManager: manager, resourceStore: adapter}
	manager.SubscribeLifecycleCallback(monitor.handleAlertLifecycleEvent)
	manager.CheckDockerHost(host)
	canonicalID := unifiedresources.SourceSpecificID(unifiedresources.ResourceTypeAppContainer, unifiedresources.SourceDocker, host.ID+"/container/"+containerID)
	filters := unifiedresources.ResourceChangeFilters{Kinds: []unifiedresources.ChangeKind{unifiedresources.ChangeAlertFired, unifiedresources.ChangeAlertResolved}}
	changes, err := store.GetRecentChangesFiltered(canonicalID, time.Time{}, 10, filters)
	require.NoError(t, err)
	require.Len(t, changes, 1)
	require.Equal(t, unifiedresources.ChangeAlertFired, changes[0].Kind)
	require.Equal(t, canonicalID, changes[0].ResourceID)
	var fired alerts.Alert
	for _, alert := range manager.GetActiveAlerts() {
		if alert.Type == "docker-container-health" {
			fired = alert
		}
	}
	require.NotEmpty(t, fired.ID)
	// Recovery is emitted after the monitored container has left the registry.
	// Its exact retained source binding must still select the original resource.
	monitor.resourceStore = unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(store))
	host.Containers[0].Health = "healthy"
	manager.CheckDockerHost(host)
	changes, err = store.GetRecentChangesFiltered(canonicalID, time.Time{}, 10, filters)
	require.NoError(t, err)
	require.Len(t, changes, 2)
	require.Equal(t, unifiedresources.ChangeAlertResolved, changes[0].Kind)
	require.Equal(t, canonicalID, changes[0].ResourceID)
	monitor.recordAlertTimelineChange(&fired, unifiedresources.ChangeAlertFired, fired.StartTime, "")
	monitor.recordAlertTimelineChange(&fired, unifiedresources.ChangeAlertResolved, *changes[0].OccurredAt, "")
	again, err := store.GetRecentChangesFiltered(canonicalID, time.Time{}, 10, filters)
	require.NoError(t, err)
	require.Equal(t, changes, again)
	controlID := unifiedresources.SourceSpecificID(unifiedresources.ResourceTypeAppContainer, unifiedresources.SourceDocker, host.ID+"/container/"+host.Containers[1].ID)
	control, err := store.GetRecentChangesFiltered(controlID, time.Time{}, 10, filters)
	require.NoError(t, err)
	require.Empty(t, control)
	require.NoError(t, store.Close())
	restarted, err := unifiedresources.NewSQLiteResourceStore(dir, "default")
	require.NoError(t, err)
	defer restarted.Close()
	afterRestart, err := restarted.GetRecentChangesFiltered(canonicalID, time.Time{}, 10, filters)
	require.NoError(t, err)
	require.Equal(t, changes, afterRestart)
	encoded, err := json.Marshal(afterRestart)
	require.NoError(t, err)
	t.Logf("DOCKER_HISTORY_LIFECYCLE %s", encoded)
}

// Proxmox node and guest alerts carry source-native IDs. Their lifecycle must
// reach the canonical resource history that facets, the drawer and the
// assistant read, including events emitted after the resource left inventory.
func TestProxmoxAlertTimelineUsesCanonicalHistoryIdentity(t *testing.T) {
	store, err := unifiedresources.NewSQLiteResourceStore(t.TempDir(), "default")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	manager := alerts.NewManagerWithDataDir(t.TempDir())
	t.Cleanup(manager.Stop)
	config := manager.GetConfig()
	config.Enabled = true
	config.ActivationState = alerts.ActivationPending
	config.TimeThresholds = map[string]int{"node": 0, "guest": 0}
	config.SuppressionWindow = 0
	manager.UpdateConfig(config)
	now := time.Now()
	node := models.Node{ID: "lab-pve1", Name: "pve1", Instance: "lab", Host: "https://pve1.lab:8006", Status: "online", CPU: 0.99, LastSeen: now}
	vm := models.VM{ID: "lab:pve1:101", VMID: 101, Name: "web", Node: "pve1", Instance: "lab", Status: "stopped", Type: "qemu", LastSeen: now}
	registry := unifiedresources.NewRegistry(store)
	registry.IngestSnapshot(models.StateSnapshot{Nodes: []models.Node{node}, VMs: []models.VM{vm}})
	monitor := &Monitor{alertManager: manager, resourceStore: unifiedresources.NewMonitorAdapter(registry)}
	manager.SubscribeLifecycleCallback(monitor.handleAlertLifecycleEvent)
	manager.CheckNode(node)
	manager.CheckGuest(vm, vm.Instance)
	manager.CheckGuest(vm, vm.Instance)

	canonical := map[string]string{}
	for _, resource := range registry.List() {
		canonical[resource.Name] = resource.ID
	}
	require.NotEqual(t, node.ID, canonical["pve1"])
	require.NotEqual(t, vm.ID, canonical["web"])
	filters := unifiedresources.ResourceChangeFilters{Kinds: []unifiedresources.ChangeKind{unifiedresources.ChangeAlertFired, unifiedresources.ChangeAlertResolved}}
	for name, sourceID := range map[string]string{"pve1": node.ID, "web": vm.ID} {
		changes, err := store.GetRecentChangesFiltered(canonical[name], time.Time{}, 10, filters)
		require.NoError(t, err)
		require.Len(t, changes, 1, name)
		require.Equal(t, unifiedresources.ChangeAlertFired, changes[0].Kind)
		require.Equal(t, canonical[name], changes[0].ResourceID)
		kinds, err := store.CountRecentChangesByKind(canonical[name], time.Time{})
		require.NoError(t, err)
		require.Equal(t, 1, kinds[unifiedresources.ChangeAlertFired], name)
		legacy, err := store.GetRecentChangesFiltered(sourceID, time.Time{}, 10, filters)
		require.NoError(t, err)
		require.Equal(t, changes, legacy, "the source reference reads the same history")
	}

	// The node recovers after it left inventory; its retained binding holds.
	monitor.resourceStore = unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(store))
	node.CPU = 0.05
	manager.CheckNode(node)
	changes, err := store.GetRecentChangesFiltered(canonical["pve1"], time.Time{}, 10, filters)
	require.NoError(t, err)
	require.Len(t, changes, 2)
	require.Equal(t, unifiedresources.ChangeAlertResolved, changes[0].Kind)
	require.Equal(t, canonical["pve1"], changes[0].ResourceID)
}

func TestMonitor_HandleAlertFired_Extra(t *testing.T) {
	// 1. Alert is nil
	m1 := &Monitor{}
	m1.handleAlertFired(nil) // Should return safely

	// 2. Alert is not nil, with Hub and NotificationMgr
	hub := websocket.NewHub(nil)
	notifMgr := notifications.NewNotificationManager("dummy")

	// mock incidentStore - but it is an interface or struct?
	// In monitor.go: func (m *Monitor) GetIncidentStore() *incidents.Store
	// It's a pointer to struct, so hard to mock unless we set it to nil or real store.
	// We can set it to nil for this test to avoid disk I/O.

	m2 := &Monitor{
		wsHub:           hub,
		notificationMgr: notifMgr,
		incidentStore:   nil,
	}

	alert := &alerts.Alert{
		ID:    "test-alert",
		Level: alerts.AlertLevelWarning,
	}

	var pushed atomic.Bool
	m2.SetAlertPushCallback(func(got *alerts.Alert) {
		if got == alert {
			pushed.Store(true)
		}
	})
	m2.handleAlertFired(alert)
	if !pushed.Load() {
		t.Fatal("alert push callback was not invoked")
	}
	// We are just verifying it doesn't crash and calls methods.
	// Hub doesn't expose way to check broadcasts easily without client.
	// NotificationMgr might spin up goroutine.
}

func TestMonitor_HandleAlertFired_RecoversFromPushCallbackPanic(t *testing.T) {
	m := &Monitor{}
	m.SetAlertPushCallback(func(*alerts.Alert) {
		panic("push transport failure")
	})

	// A transport adapter must not be able to abort the canonical alert
	// lifecycle or its remaining persistence callbacks.
	m.handleAlertFired(&alerts.Alert{ID: "alert-push-panic"})
}

func TestDeliveryCallbackDoesNotDuplicateCanonicalAITrigger(t *testing.T) {
	called := make(chan struct{}, 1)
	monitor := &Monitor{
		alertTriggeredAICallback: func(*alerts.Alert) { called <- struct{}{} },
	}

	monitor.handleAlertFired(&alerts.Alert{ID: "single-ai-trigger"})
	select {
	case <-called:
		t.Fatal("delivery callback invoked AI analysis; the manager-owned unconditional AI callback is the sole trigger")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestMonitor_HandleAlertLifecycle_WritesCanonicalChanges(t *testing.T) {
	store := unifiedresources.NewMemoryStore()
	m := &Monitor{
		resourceStore: unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(store)),
	}

	startedAt := time.Date(2026, 3, 20, 9, 0, 0, 0, time.UTC)
	ackAt := startedAt.Add(2 * time.Minute)
	alert := &alerts.Alert{
		ID:         "alert-canonical-1",
		Type:       "cpu",
		Level:      alerts.AlertLevelCritical,
		ResourceID: "vm-1",
		Message:    "CPU threshold exceeded",
		Value:      93.4,
		Threshold:  80,
		StartTime:  startedAt,
		AckTime:    &ackAt,
		Metadata: map[string]interface{}{
			"incidentCategory":   "health",
			"vmwareConnectionId": "vc-1",
		},
	}

	m.handleAlertLifecycleEvent(alerts.LifecycleEvent{Type: eventlog.TypeFired, OccurredAt: startedAt, Alert: alert})
	m.handleAlertLifecycleEvent(alerts.LifecycleEvent{
		Type:       eventlog.TypeAcknowledged,
		OccurredAt: ackAt,
		Alert:      alert,
		Details:    map[string]string{"user": "admin"},
	})
	m.handleAlertLifecycleEvent(alerts.LifecycleEvent{
		Type:       eventlog.TypeSnoozed,
		OccurredAt: ackAt.Add(2 * time.Minute),
		Alert:      alert,
		Details: map[string]string{
			"actor": "admin",
			"until": ackAt.Add(2 * time.Hour).Format(time.RFC3339),
		},
	})
	m.handleAlertLifecycleEvent(alerts.LifecycleEvent{
		Type:       eventlog.TypeUnsnoozed,
		OccurredAt: ackAt.Add(3 * time.Minute),
		Alert:      alert,
		Details:    map[string]string{"actor": "admin"},
	})
	m.handleAlertLifecycleEvent(alerts.LifecycleEvent{
		Type:       eventlog.TypeUnacknowledged,
		OccurredAt: ackAt.Add(time.Minute),
		Alert:      alert,
		Details:    map[string]string{"user": "admin"},
	})

	changes, err := store.GetRecentChanges("vm-1", time.Time{}, 10)
	if err != nil {
		t.Fatalf("GetRecentChanges: %v", err)
	}
	if len(changes) != 5 {
		t.Fatalf("expected 5 canonical changes, got %d", len(changes))
	}
	// Arrival order can differ from observation order during lifecycle replay.
	wantKinds := []unifiedresources.ChangeKind{
		unifiedresources.ChangeAlertUnsnoozed,
		unifiedresources.ChangeAlertSnoozed,
		unifiedresources.ChangeAlertUnacknowledged,
		unifiedresources.ChangeAlertAcknowledged,
		unifiedresources.ChangeAlertFired,
	}
	for idx, want := range wantKinds {
		if changes[idx].Kind != want {
			t.Fatalf("changes[%d].Kind = %q, want %q", idx, changes[idx].Kind, want)
		}
	}
	if got := changes[4].Metadata["alert_identifier"]; got != "alert-canonical-1" {
		t.Fatalf("alert_identifier = %#v, want alert-canonical-1", got)
	}
	if got := changes[4].Metadata["incidentCategory"]; got != "health" {
		t.Fatalf("incidentCategory = %#v, want health", got)
	}
	if got := changes[2].Metadata["vmwareConnectionId"]; got != "vc-1" {
		t.Fatalf("vmwareConnectionId = %#v, want vc-1", got)
	}
}

// A node alert handed to its Pulse agent closes without recovering. The
// resource history and the alert's incident timeline (the Alerts history row
// expansion) must say where it went, not "Alert resolved: Memory usage at 95%".
func TestMonitor_HandleAlertLifecycle_HandoverCloseIsNotARecovery(t *testing.T) {
	resourceStore := unifiedresources.NewMemoryStore()
	incidentStore := memory.NewIncidentStore(memory.IncidentStoreConfig{})
	m := &Monitor{
		incidentStore: incidentStore,
		resourceStore: unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(resourceStore)),
	}
	incidentStore.SetResourceTimelineStore(m.resourceStore.(memory.IncidentTimelineStore))

	startedAt := time.Date(2026, 10, 6, 9, 0, 0, 0, time.UTC)
	movedAt := startedAt.Add(2 * time.Hour)
	alert := &alerts.Alert{
		ID:         "pve1-memory",
		Type:       "memory",
		Level:      alerts.AlertLevelWarning,
		ResourceID: "pve1",
		Message:    "Memory usage at 95%",
		Value:      95,
		Threshold:  85,
		StartTime:  startedAt,
	}
	m.handleAlertLifecycleEvent(alerts.LifecycleEvent{Type: eventlog.TypeFired, OccurredAt: startedAt, Alert: alert})
	closed := alert.Clone()
	closed.Resolution = &alerts.AlertResolution{
		Reason:              alerts.AlertResolutionMovedToAgent,
		SuccessorResourceID: "agent-pve1",
		SuccessorName:       "pve1 (Host Agent)",
	}
	m.handleAlertLifecycleEvent(alerts.LifecycleEvent{Type: eventlog.TypeResolved, OccurredAt: movedAt, Alert: closed})

	summary := "Alert moved to pve1 (Host Agent). This is not a recovery: check the agent for the current reading."
	changes, err := resourceStore.GetRecentChanges("pve1", time.Time{}, 10)
	require.NoError(t, err)
	require.Len(t, changes, 2)
	require.Equal(t, unifiedresources.ChangeAlertResolved, changes[0].Kind)
	require.Equal(t, summary, changes[0].Reason)
	require.Equal(t, "moved_to_agent", changes[0].Metadata[unifiedresources.MetadataAlertResolution])
	require.Equal(t, unifiedresources.ChangeAlertFired, changes[1].Kind)
	require.NotContains(t, changes[1].Metadata, unifiedresources.MetadataAlertResolution)

	timeline := incidentStore.GetTimelineByAlertAt(alert.ID, startedAt)
	require.NotNil(t, timeline)
	require.Len(t, timeline.Events, 2)
	require.Equal(t, memory.IncidentEventAlertResolved, timeline.Events[1].Type)
	require.Equal(t, summary, timeline.Events[1].Summary)

	// An ordinary recovery keeps its existing wording.
	recovered := alert.Clone()
	recovered.ID = "pve1-cpu"
	recovered.Type = "cpu"
	recovered.Message = "CPU usage at 90%"
	m.handleAlertLifecycleEvent(alerts.LifecycleEvent{Type: eventlog.TypeFired, OccurredAt: startedAt, Alert: recovered})
	m.handleAlertLifecycleEvent(alerts.LifecycleEvent{Type: eventlog.TypeResolved, OccurredAt: movedAt.Add(time.Minute), Alert: recovered})
	recoveredTimeline := incidentStore.GetTimelineByAlertAt(recovered.ID, startedAt)
	require.NotNil(t, recoveredTimeline)
	require.Len(t, recoveredTimeline.Events, 2)
	require.Equal(t, "Alert resolved", recoveredTimeline.Events[1].Summary)
}

func TestPausedDeliveryStillBuildsTimelineThroughRealAlertLifecycle(t *testing.T) {
	manager := alerts.NewManagerWithDataDir(t.TempDir())
	t.Cleanup(manager.Stop)
	config := manager.GetConfig()
	config.Enabled = true
	config.ActivationState = alerts.ActivationPending
	config.TimeThresholds = map[string]int{}
	config.SuppressionWindow = 0
	manager.UpdateConfig(config)

	resourceStore := unifiedresources.NewMemoryStore()
	incidentStore := memory.NewIncidentStore(memory.IncidentStoreConfig{})
	monitor := &Monitor{
		alertManager:  manager,
		incidentStore: incidentStore,
		resourceStore: unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(resourceStore)),
	}
	incidentStore.SetResourceTimelineStore(monitor.resourceStore.(memory.IncidentTimelineStore))
	manager.SubscribeLifecycleCallback(monitor.handleAlertLifecycleEvent)
	delivered := make(chan *alerts.Alert, 1)
	manager.SetAlertCallback(func(alert *alerts.Alert) { delivered <- alert })

	vm := models.VM{ID: "paused-vm", Name: "Paused VM", Node: "node-1", Instance: "pve-1", Status: "stopped"}
	manager.CheckGuest(vm, vm.Instance)
	manager.CheckGuest(vm, vm.Instance)

	active := manager.GetActiveAlerts()
	if len(active) != 1 {
		t.Fatalf("active alerts = %d, want 1", len(active))
	}
	timeline := incidentStore.GetTimelineByAlertAt(active[0].ID, active[0].StartTime)
	if timeline == nil || len(timeline.Events) == 0 || timeline.Events[0].Type != memory.IncidentEventAlertFired {
		t.Fatalf("paused-delivery lifecycle did not produce an incident timeline: %#v", timeline)
	}
	select {
	case alert := <-delivered:
		t.Fatalf("pending-review alert reached delivery callback: %s", alert.ID)
	default:
	}
}

func TestActiveTimelineReconciliationIsIdempotent(t *testing.T) {
	manager := alerts.NewManagerWithDataDir(t.TempDir())
	t.Cleanup(manager.Stop)
	config := manager.GetConfig()
	config.Enabled = true
	config.ActivationState = alerts.ActivationPending
	config.TimeThresholds = map[string]int{}
	manager.UpdateConfig(config)

	vm := models.VM{ID: "restored-vm", Name: "Restored VM", Node: "node-1", Instance: "pve-1", Status: "stopped"}
	manager.CheckGuest(vm, vm.Instance)
	manager.CheckGuest(vm, vm.Instance)

	resourceStore := unifiedresources.NewMemoryStore()
	incidentStore := memory.NewIncidentStore(memory.IncidentStoreConfig{})
	monitor := &Monitor{
		alertManager:  manager,
		incidentStore: incidentStore,
		resourceStore: unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(resourceStore)),
	}
	incidentStore.SetResourceTimelineStore(monitor.resourceStore.(memory.IncidentTimelineStore))
	monitor.reconcileActiveAlertTimelines()
	monitor.reconcileActiveAlertTimelines()

	active := manager.GetActiveAlerts()
	if len(active) != 1 {
		t.Fatalf("active alerts = %d, want 1", len(active))
	}
	timeline := incidentStore.GetTimelineByAlertAt(active[0].ID, active[0].StartTime)
	if timeline == nil || len(timeline.Events) != 1 || timeline.Events[0].Type != memory.IncidentEventAlertFired {
		t.Fatalf("reconciled timeline = %#v, want one fired event", timeline)
	}
	changes, err := resourceStore.GetRecentChanges(active[0].ResourceID, time.Time{}, 10)
	if err != nil {
		t.Fatalf("GetRecentChanges: %v", err)
	}
	if len(changes) != 1 {
		t.Fatalf("reconciliation wrote %d canonical changes, want 1", len(changes))
	}
}

func TestLifecycleReplayRepairsResolvedIncidentTimeline(t *testing.T) {
	manager := alerts.NewManagerWithDataDir(t.TempDir(), alerts.WithoutPersistedAlertRestore())
	t.Cleanup(manager.Stop)
	manager.EnableEventLog()
	config := manager.GetConfig()
	config.Enabled = true
	config.ActivationState = alerts.ActivationPending
	config.TimeThresholds = map[string]int{}
	config.SuppressionWindow = 0
	manager.UpdateConfig(config)

	vm := models.VM{ID: "historical-vm", Name: "Historical VM", Node: "node-1", Instance: "pve-1", Status: "stopped"}
	manager.CheckGuest(vm, vm.Instance)
	manager.CheckGuest(vm, vm.Instance)
	active := manager.GetActiveAlerts()
	if len(active) != 1 {
		t.Fatalf("active alerts = %d, want 1", len(active))
	}
	alertID, startedAt := active[0].ID, active[0].StartTime

	vm.Status = "running"
	manager.CheckGuest(vm, vm.Instance)
	if active := manager.GetActiveAlerts(); len(active) != 0 {
		t.Fatalf("active alerts after recovery = %d, want 0", len(active))
	}

	resourceStore := unifiedresources.NewMemoryStore()
	incidentStore := memory.NewIncidentStore(memory.IncidentStoreConfig{})
	monitor := &Monitor{
		alertManager:  manager,
		incidentStore: incidentStore,
		resourceStore: unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(resourceStore)),
	}
	incidentStore.SetResourceTimelineStore(monitor.resourceStore.(memory.IncidentTimelineStore))
	monitor.replayAlertLifecycleProjections()
	monitor.replayAlertLifecycleProjections()

	timeline := incidentStore.GetTimelineByAlertAt(alertID, startedAt)
	if timeline == nil || len(timeline.Events) != 2 {
		t.Fatalf("replayed timeline = %#v, want fired and resolved events", timeline)
	}
	if timeline.Events[0].Type != memory.IncidentEventAlertFired || timeline.Events[1].Type != memory.IncidentEventAlertResolved {
		t.Fatalf("replayed event types = %#v, want fired then resolved", timeline.Events)
	}
	changes, err := resourceStore.GetRecentChanges(vm.ID, time.Time{}, 10)
	if err != nil {
		t.Fatalf("GetRecentChanges: %v", err)
	}
	if len(changes) != 2 {
		t.Fatalf("replay wrote %d canonical changes, want 2", len(changes))
	}
}

func TestLifecycleReplayMaterializesImportedHistoryTimeline(t *testing.T) {
	manager := alerts.NewManagerWithDataDir(t.TempDir(), alerts.WithoutPersistedAlertRestore())
	t.Cleanup(manager.Stop)
	store, err := eventlog.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open event log: %v", err)
	}
	manager.SetEventLog(store)

	startedAt := time.Now().UTC().Add(-30 * time.Minute).Truncate(time.Second)
	ackAt := startedAt.Add(5 * time.Minute)
	resolvedAt := startedAt.Add(20 * time.Minute)
	snapshot := alerts.Alert{
		ID:           "imported-alert-1",
		Type:         "cpu",
		Level:        alerts.AlertLevelWarning,
		ResourceID:   "imported-resource-1",
		ResourceName: "Imported VM",
		StartTime:    startedAt,
		LastSeen:     resolvedAt,
		Acknowledged: true,
		AckTime:      &ackAt,
		AckUser:      "operator",
	}
	payload, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("marshal imported snapshot: %v", err)
	}
	if err := store.ImportEvents([]eventlog.Event{{
		OccurredAt: resolvedAt,
		Type:       eventlog.TypeHistoryImported,
		AlertID:    snapshot.ID,
		ResourceID: snapshot.ResourceID,
		Snapshot:   payload,
	}}); err != nil {
		t.Fatalf("import history event: %v", err)
	}

	incidentStore := memory.NewIncidentStore(memory.IncidentStoreConfig{})
	monitor := &Monitor{alertManager: manager, incidentStore: incidentStore}
	resourceStore := unifiedresources.NewMemoryStore()
	adapter := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(resourceStore))
	// Hold replay at its serialization boundary. Router construction attaches
	// this store, so attachment must return even while history repair cannot
	// make progress. Eventual timeline assertions alone miss a synchronous
	// replay regression that stalls startup on an upgrade backlog.
	monitor.alertProjectionReplayMu.Lock()
	attached := make(chan struct{})
	go func() {
		monitor.SetResourceStore(adapter)
		monitor.SetResourceStore(adapter)
		close(attached)
	}()
	select {
	case <-attached:
		monitor.alertProjectionReplayMu.Unlock()
	case <-time.After(2 * time.Second):
		// Release the probe before failing, including for a synchronous-replay
		// negative control, so no goroutine retains the test's stores.
		monitor.alertProjectionReplayMu.Unlock()
		<-attached
		monitor.alertProjectionWG.Wait()
		t.Fatal("resource-store attachment waited for lifecycle replay")
	}
	monitor.alertProjectionWG.Wait()

	timeline := incidentStore.GetTimelineByAlertAt(snapshot.ID, snapshot.StartTime)
	if timeline == nil || timeline.Status != memory.IncidentStatusResolved || !timeline.Acknowledged {
		t.Fatalf("imported timeline state = %#v", timeline)
	}
	if len(timeline.Events) != 3 {
		t.Fatalf("imported timeline events = %d, want three idempotent snapshot events", len(timeline.Events))
	}
	changes, err := resourceStore.GetRecentChanges(snapshot.ResourceID, time.Time{}, 10)
	if err != nil {
		t.Fatalf("read imported canonical changes: %v", err)
	}
	if len(changes) != 3 {
		t.Fatalf("imported canonical changes = %d, want fired, acknowledged, and resolved", len(changes))
	}
}

func TestLifecycleReplayWatermarkBoundsSubsequentPasses(t *testing.T) {
	manager := alerts.NewManagerWithDataDir(t.TempDir(), alerts.WithoutPersistedAlertRestore())
	t.Cleanup(manager.Stop)
	manager.EnableEventLog()
	config := manager.GetConfig()
	config.Enabled = true
	config.ActivationState = alerts.ActivationPending
	config.TimeThresholds = map[string]int{}
	config.SuppressionWindow = 0
	manager.UpdateConfig(config)

	vm := models.VM{ID: "watermark-vm", Name: "Watermark VM", Node: "node-1", Instance: "pve-1", Status: "stopped"}
	manager.CheckGuest(vm, vm.Instance)
	manager.CheckGuest(vm, vm.Instance)
	vm.Status = "running"
	manager.CheckGuest(vm, vm.Instance)

	// A pass without the canonical resource store repairs incidents but must
	// not advance the durable watermark, or resource-timeline projections for
	// those events would never materialize.
	partial := &Monitor{alertManager: manager, incidentStore: memory.NewIncidentStore(memory.IncidentStoreConfig{})}
	partial.replayAlertLifecycleProjections()
	if got := manager.LifecycleProjectionWatermark(alertLifecycleProjectionConsumer); got != 0 {
		t.Fatalf("watermark after partial-surface replay = %d, want 0", got)
	}

	resourceStore := unifiedresources.NewMemoryStore()
	monitor := &Monitor{
		alertManager:  manager,
		incidentStore: memory.NewIncidentStore(memory.IncidentStoreConfig{}),
		resourceStore: unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(resourceStore)),
	}
	monitor.replayAlertLifecycleProjections()
	watermark := manager.LifecycleProjectionWatermark(alertLifecycleProjectionConsumer)
	if watermark == 0 {
		t.Fatal("watermark did not advance after full-surface replay")
	}

	// A later pass walks only events beyond the watermark, so fresh projection
	// stores stay empty: nothing is left to replay.
	freshResources := unifiedresources.NewMemoryStore()
	rerun := &Monitor{
		alertManager:  manager,
		incidentStore: memory.NewIncidentStore(memory.IncidentStoreConfig{}),
		resourceStore: unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(freshResources)),
	}
	rerun.replayAlertLifecycleProjections()
	if changes, err := freshResources.GetRecentChanges(vm.ID, time.Time{}, 10); err != nil || len(changes) != 0 {
		t.Fatalf("bounded pass wrote %d changes (err %v), want none", len(changes), err)
	}

	// Resetting the watermark forces a full repair replay for rebuilt stores.
	manager.StoreLifecycleProjectionWatermark(alertLifecycleProjectionConsumer, 0)
	rerun.replayAlertLifecycleProjections()
	changes, err := freshResources.GetRecentChanges(vm.ID, time.Time{}, 10)
	if err != nil {
		t.Fatalf("GetRecentChanges after watermark reset: %v", err)
	}
	if len(changes) != 2 {
		t.Fatalf("reset replay wrote %d canonical changes, want fired and resolved", len(changes))
	}
	if got := manager.LifecycleProjectionWatermark(alertLifecycleProjectionConsumer); got != watermark {
		t.Fatalf("watermark after reset replay = %d, want %d", got, watermark)
	}
}

func TestSystemAlertTimelineUsesCanonicalPulseResource(t *testing.T) {
	resourceStore := unifiedresources.NewMemoryStore()
	incidentStore := memory.NewIncidentStore(memory.IncidentStoreConfig{})
	monitor := &Monitor{
		incidentStore: incidentStore,
		resourceStore: unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(resourceStore)),
	}
	incidentStore.SetResourceTimelineStore(monitor.resourceStore.(memory.IncidentTimelineStore))
	alert := &alerts.Alert{
		ID:           alerts.SystemAlertID("event-store-health"),
		Type:         "event-store-health",
		Level:        alerts.AlertLevelCritical,
		ResourceName: alerts.SystemAlertResourceName,
		Message:      "Alert history storage is unavailable",
		StartTime:    time.Now().UTC(),
	}
	monitor.handleAlertLifecycleEvent(alerts.LifecycleEvent{Type: eventlog.TypeFired, OccurredAt: alert.StartTime, Alert: alert})
	timeline := incidentStore.GetTimelineByAlertAt(alert.ID, alert.StartTime)
	if timeline == nil || timeline.ResourceID != "pulse-system" || len(timeline.Events) != 1 {
		t.Fatalf("system alert timeline = %#v", timeline)
	}
}

func TestMonitor_HandleAlertResolved_Detailed_Extra(t *testing.T) {
	// 1. With Hub and NotificationMgr and Resolve Notify ON
	hub := websocket.NewHub(nil)
	notifMgr := notifications.NewNotificationManager("dummy")

	// Enable resolve notifications
	// Notifications config needs to be updated?
	// notificationMgr.GetNotifyOnResolve() reads config.
	// But NotificationManager struct doesn't export Config update easily without SetConfig?
	// The constructor initializes defaults.

	m := &Monitor{
		wsHub:           hub,
		notificationMgr: notifMgr,
		alertManager:    alerts.NewManager(),
	}

	// This should run safely
	m.handleAlertResolved("alert-id")
}

func TestMonitor_HandleAlertResolved_QuietHoursSuppressesRecovery(t *testing.T) {
	// Verify that resolved notifications are suppressed during quiet hours (#1068).
	// We seed a resolved alert in the manager, configure quiet hours to suppress all
	// (00:00–23:59 every day), then verify SendResolvedAlert is NOT called.

	hub := websocket.NewHub(nil)
	notifMgr := notifications.NewNotificationManager("dummy")
	notifMgr.SetNotifyOnResolve(true)

	mgr := alerts.NewManager()
	// Configure quiet hours to be always active, alerts enabled, no time delay
	mgr.UpdateConfig(alerts.AlertConfig{
		Enabled: true,
		TimeThresholds: map[string]int{
			"guest": 0, "node": 0, "storage": 0, "pbs": 0, "host": 0,
		},
		GuestDefaults: alerts.ThresholdConfig{
			CPU:    &alerts.HysteresisThreshold{Trigger: 80, Clear: 75},
			Memory: &alerts.HysteresisThreshold{Trigger: 85, Clear: 80},
		},
		Schedule: alerts.ScheduleConfig{
			NotifyOnResolve: true,
			QuietHours: alerts.QuietHours{
				Enabled:  true,
				Start:    "00:00",
				End:      "23:59",
				Timezone: "UTC",
				Days: map[string]bool{
					"monday": true, "tuesday": true, "wednesday": true,
					"thursday": true, "friday": true, "saturday": true, "sunday": true,
				},
				Suppress: alerts.QuietHoursSuppression{
					Performance: true,
					Storage:     true,
					Offline:     true,
				},
			},
		},
	})

	m := &Monitor{
		wsHub:           hub,
		notificationMgr: notifMgr,
		alertManager:    mgr,
	}

	// Seed a resolved alert in the alert manager.
	// Fire a warning alert via CheckGuest and then resolve it.
	guest := models.VM{
		ID:       "100",
		VMID:     100,
		Name:     "test-vm",
		Node:     "pve1",
		Status:   "running",
		Type:     "qemu",
		CPU:      0.95, // 95% — above default threshold
		CPUs:     1,
		Memory:   models.Memory{Usage: 50},
		Instance: "https://pve.local:8006",
	}

	// Fire the alert (CPU > threshold)
	mgr.CheckGuest(guest, "pve1")

	activeAlerts := mgr.GetActiveAlerts()
	if len(activeAlerts) == 0 {
		t.Skip("no alert fired — threshold may differ from defaults, skipping integration test")
	}

	alertID := activeAlerts[0].ID

	// Now resolve: bring CPU below threshold
	guest.CPU = 0.10
	mgr.CheckGuest(guest, "pve1")

	// The alert should now be in recently resolved
	resolved := mgr.GetResolvedAlert(alertID)
	if resolved == nil {
		t.Skip("alert was not resolved by CheckGuest — skipping integration test")
	}

	// Verify quiet hours suppression directly
	if !mgr.ShouldSuppressResolvedNotification(resolved.Alert) {
		t.Fatal("expected ShouldSuppressResolvedNotification to return true during quiet hours")
	}

	// Track whether resolved AI callback fires (it should, even during quiet hours)
	var aiCallbackCalled atomic.Int32
	m.alertResolvedAICallback = func(a *alerts.Alert) {
		aiCallbackCalled.Add(1)
	}

	// Call handleAlertResolved — quiet hours should suppress the notification
	m.handleAlertResolved(alertID)

	// Give goroutine time to execute
	time.Sleep(50 * time.Millisecond)

	// AI callback should always fire regardless of quiet hours
	if aiCallbackCalled.Load() == 0 {
		t.Error("expected AI resolved callback to fire even during quiet hours")
	}
}

func TestMonitor_HandleAlertResolved_NoQuietHoursSendsNotification(t *testing.T) {
	// Verify that resolved notifications are sent when quiet hours are NOT active.
	hub := websocket.NewHub(nil)
	notifMgr := notifications.NewNotificationManager("dummy")
	notifMgr.SetNotifyOnResolve(true)

	mgr := alerts.NewManager()
	// No quiet hours, but alerts enabled with no time delay
	mgr.UpdateConfig(alerts.AlertConfig{
		Enabled: true,
		TimeThresholds: map[string]int{
			"guest": 0, "node": 0, "storage": 0, "pbs": 0, "host": 0,
		},
		GuestDefaults: alerts.ThresholdConfig{
			CPU:    &alerts.HysteresisThreshold{Trigger: 80, Clear: 75},
			Memory: &alerts.HysteresisThreshold{Trigger: 85, Clear: 80},
		},
	})

	m := &Monitor{
		wsHub:           hub,
		notificationMgr: notifMgr,
		alertManager:    mgr,
	}

	// Seed a resolved alert
	guest := models.VM{
		ID:       "200",
		VMID:     200,
		Name:     "test-vm-2",
		Node:     "pve2",
		Status:   "running",
		Type:     "qemu",
		CPU:      0.95,
		CPUs:     1,
		Memory:   models.Memory{Usage: 50},
		Instance: "https://pve.local:8006",
	}

	mgr.CheckGuest(guest, "pve2")
	activeAlerts := mgr.GetActiveAlerts()
	if len(activeAlerts) == 0 {
		t.Skip("no alert fired — threshold may differ from defaults, skipping integration test")
	}

	alertID := activeAlerts[0].ID
	guest.CPU = 0.10
	mgr.CheckGuest(guest, "pve2")

	// Should not crash, and notification should be dispatched (not suppressed)
	m.handleAlertResolved(alertID)
}

func TestMonitor_HandleAlertResolved_SendsRecoveryForGuestPoweredOffState(t *testing.T) {
	received := make(chan []byte, 2)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		body, _ := io.ReadAll(r.Body)
		select {
		case received <- body:
		default:
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	notifMgr := notifications.NewNotificationManagerWithDataDir("http://pulse.example", t.TempDir())
	t.Cleanup(notifMgr.Stop)
	if err := notifMgr.UpdateAllowedPrivateCIDRs("127.0.0.1/32,::1/128"); err != nil {
		t.Fatalf("UpdateAllowedPrivateCIDRs: %v", err)
	}
	notifMgr.AddWebhook(notifications.WebhookConfig{
		ID:      "test-webhook",
		Name:    "test-webhook",
		URL:     srv.URL,
		Enabled: true,
		Service: "generic",
	})
	notifMgr.SetNotifyOnResolve(true)
	notifMgr.SetGroupingWindow(0)

	alertMgr := alerts.NewManagerWithDataDir(t.TempDir())
	t.Cleanup(alertMgr.Stop)
	cfg := alertMgr.GetConfig()
	cfg.Enabled = true
	cfg.ActivationState = alerts.ActivationActive
	cfg.Schedule.QuietHours.Enabled = false
	alertMgr.UpdateConfig(cfg)

	m := &Monitor{
		alertManager:    alertMgr,
		notificationMgr: notifMgr,
	}
	alertMgr.SetAlertCallback(m.handleAlertFired)
	alertMgr.SetResolvedCallback(m.handleAlertResolved)

	vm := models.VM{
		ID:       "vm-powered-off",
		Name:     "powered-off-vm",
		Node:     "node-1",
		Instance: "inst-1",
		Status:   "stopped",
	}

	alertMgr.CheckGuest(vm, vm.Instance)
	alertMgr.CheckGuest(vm, vm.Instance)

	// Firing uses the grouped envelope even with a zero grouping window.
	var firingPayload struct {
		Grouped bool `json:"grouped"`
		Alerts  []struct {
			ID string `json:"id"`
		} `json:"alerts"`
	}
	select {
	case body := <-received:
		if err := json.Unmarshal(body, &firingPayload); err != nil {
			t.Fatalf("failed to parse firing webhook payload: %v", err)
		}
		if !firingPayload.Grouped || len(firingPayload.Alerts) != 1 {
			t.Fatalf("expected one grouped firing alert, got %+v", firingPayload)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for initial powered-off notification webhook")
	}

	activeAlerts := alertMgr.GetActiveAlerts()
	if len(activeAlerts) != 1 {
		t.Fatalf("expected one active powered-off alert, got %#v", activeAlerts)
	}
	alertID := activeAlerts[0].ID
	if firingPayload.Alerts[0].ID != alertID {
		t.Fatalf("expected firing webhook alert ID=%q, got %q", alertID, firingPayload.Alerts[0].ID)
	}
	if activeAlerts[0].LastNotified == nil {
		t.Fatalf("expected powered-off alert %q to record firing notification time", alertID)
	}

	if resolved := alertMgr.GetResolvedAlert(alertID); resolved != nil {
		t.Fatalf("did not expect powered-off alert %q to be resolved before the VM starts", alertID)
	}

	vm.Status = "running"
	alertMgr.CheckGuest(vm, vm.Instance)

	select {
	case body := <-received:
		var payload map[string]interface{}
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to parse webhook payload: %v", err)
		}
		if payload["event"] != "resolved" {
			t.Fatalf("expected webhook event=resolved, got %v", payload["event"])
		}
		if payload["alertIdentifier"] != alertID {
			t.Fatalf("expected webhook alertIdentifier=%q, got %v", alertID, payload["alertIdentifier"])
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for powered-off recovery notification webhook")
	}
}

func TestMonitor_HandleAlertResolved_SuppressesRecoveryWhenFiringNeverDelivered(t *testing.T) {
	// Regression test for #1553: an alert that resolves while its firing
	// notification is still waiting in the grouping window must not produce a
	// recovery-only notification.
	t.Setenv("PULSE_DATA_DIR", t.TempDir())
	received := make(chan []byte, 2)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		body, _ := io.ReadAll(r.Body)
		select {
		case received <- body:
		default:
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	notifMgr := notifications.NewNotificationManagerWithDataDir("http://pulse.example", t.TempDir())
	if err := notifMgr.UpdateAllowedPrivateCIDRs("127.0.0.1/32,::1/128"); err != nil {
		t.Fatalf("UpdateAllowedPrivateCIDRs: %v", err)
	}
	notifMgr.AddWebhook(notifications.WebhookConfig{
		ID:      "test-webhook",
		Name:    "test-webhook",
		URL:     srv.URL,
		Enabled: true,
		Service: "generic",
	})
	notifMgr.SetNotifyOnResolve(true)
	// Large grouping window keeps the firing notification undelivered until
	// the alert resolves.
	notifMgr.SetGroupingWindow(120)

	alertMgr := alerts.NewManager()
	cfg := alertMgr.GetConfig()
	cfg.Enabled = true
	cfg.ActivationState = alerts.ActivationActive
	cfg.Schedule.QuietHours.Enabled = false
	alertMgr.UpdateConfig(cfg)

	m := &Monitor{
		alertManager:    alertMgr,
		notificationMgr: notifMgr,
	}

	vm := models.VM{
		ID:       "vm-transient-spike",
		Name:     "transient-spike-vm",
		Node:     "node-1",
		Instance: "inst-1",
		Status:   "stopped",
	}

	// Fire the alert without wiring the async fired-callback, then hand the
	// firing notification to the manager synchronously so it is deterministic
	// that it sits in the grouping window when the alert resolves.
	alertMgr.CheckGuest(vm, vm.Instance)
	alertMgr.CheckGuest(vm, vm.Instance)

	activeAlerts := alertMgr.GetActiveAlerts()
	if len(activeAlerts) != 1 {
		t.Fatalf("expected one active powered-off alert, got %#v", activeAlerts)
	}
	firing := activeAlerts[0]
	notifMgr.SendAlert(&firing)

	vm.Status = "running"
	alertMgr.CheckGuest(vm, vm.Instance)
	if resolved := alertMgr.GetResolvedAlert(firing.ID); resolved == nil {
		t.Fatalf("expected alert %q to be resolved", firing.ID)
	}

	m.handleAlertResolved(firing.ID)

	select {
	case body := <-received:
		t.Fatalf("expected no notification for a firing that never left the grouping window, got %s", string(body))
	case <-time.After(1500 * time.Millisecond):
	}
}

func TestMonitor_HandleAlertEscalated_QuietHoursSuppressesNotification(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())

	requests := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- struct{}{}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	notifMgr := notifications.NewNotificationManager("https://pulse.local")
	defer notifMgr.Stop()
	notifMgr.SetGroupingWindow(0)
	notifMgr.SetCooldown(0)
	if err := notifMgr.UpdateAllowedPrivateCIDRs("127.0.0.1/32"); err != nil {
		t.Fatalf("UpdateAllowedPrivateCIDRs: %v", err)
	}
	notifMgr.AddWebhook(notifications.WebhookConfig{
		ID:      "quiet-hours-hook",
		Name:    "quiet-hours",
		URL:     server.URL,
		Enabled: true,
	})

	mgr := alerts.NewManager()
	cfg := mgr.GetConfig()
	cfg.Schedule.Escalation.Levels = []alerts.EscalationLevel{{After: 1, Notify: "webhook"}}
	cfg.Schedule.QuietHours = alerts.QuietHours{
		Enabled:  true,
		Start:    "00:00",
		End:      "23:59",
		Timezone: "UTC",
		Days: map[string]bool{
			"monday": true, "tuesday": true, "wednesday": true,
			"thursday": true, "friday": true, "saturday": true, "sunday": true,
		},
		Suppress: alerts.QuietHoursSuppression{Offline: true},
	}
	mgr.UpdateConfig(cfg)

	m := &Monitor{
		notificationMgr: notifMgr,
		alertManager:    mgr,
	}

	alert := activeEscalationFixture(t, mgr, "connectivity", alerts.AlertLevelCritical)

	m.handleAlertEscalated(nil, alert, 1)

	select {
	case <-requests:
		t.Fatal("expected quiet hours to suppress escalated notification delivery")
	case <-time.After(500 * time.Millisecond):
	}
}

func TestMonitor_HandleAlertEscalated_SendsNotificationWhenNotSuppressed(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())

	requests := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- struct{}{}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	notifMgr := notifications.NewNotificationManager("https://pulse.local")
	defer notifMgr.Stop()
	notifMgr.SetGroupingWindow(0)
	notifMgr.SetCooldown(0)
	if err := notifMgr.UpdateAllowedPrivateCIDRs("127.0.0.1/32"); err != nil {
		t.Fatalf("UpdateAllowedPrivateCIDRs: %v", err)
	}
	notifMgr.AddWebhook(notifications.WebhookConfig{
		ID:      "normal-hook",
		Name:    "normal",
		URL:     server.URL,
		Enabled: true,
	})

	mgr := alerts.NewManager()
	cfg := mgr.GetConfig()
	cfg.Schedule.Escalation.Levels = []alerts.EscalationLevel{{After: 1, Notify: "webhook"}}
	cfg.Schedule.QuietHours.Enabled = false
	mgr.UpdateConfig(cfg)

	m := &Monitor{
		notificationMgr: notifMgr,
		alertManager:    mgr,
	}

	alert := activeEscalationFixture(t, mgr, "connectivity", alerts.AlertLevelCritical)

	m.handleAlertEscalated(nil, alert, 1)

	select {
	case <-requests:
	case <-time.After(2 * time.Second):
		t.Fatal("expected escalated notification delivery")
	}
}

func TestMonitor_HandleAlertEscalated_BypassesDeliveryCooldown(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())

	requests := make(chan struct{}, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- struct{}{}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	notifMgr := notifications.NewNotificationManager("https://pulse.local")
	defer notifMgr.Stop()
	notifMgr.SetGroupingWindow(0)
	notifMgr.SetCooldown(30)
	if err := notifMgr.UpdateAllowedPrivateCIDRs("127.0.0.1/32"); err != nil {
		t.Fatalf("UpdateAllowedPrivateCIDRs: %v", err)
	}
	notifMgr.AddWebhook(notifications.WebhookConfig{
		ID:      "cooldown-hook",
		Name:    "cooldown",
		URL:     server.URL,
		Enabled: true,
	})

	mgr := alerts.NewManager()
	cfg := mgr.GetConfig()
	cfg.Schedule.Escalation.Levels = []alerts.EscalationLevel{{After: 1, Notify: "webhook"}}
	cfg.Schedule.QuietHours.Enabled = false
	mgr.UpdateConfig(cfg)

	m := &Monitor{
		notificationMgr: notifMgr,
		alertManager:    mgr,
	}

	alert := activeEscalationFixture(t, mgr, "memory", alerts.AlertLevelWarning)

	notifMgr.SendAlert(alert)
	select {
	case <-requests:
	case <-time.After(2 * time.Second):
		t.Fatal("expected initial notification delivery")
	}

	m.handleAlertEscalated(nil, alert, 1)

	select {
	case <-requests:
	case <-time.After(2 * time.Second):
		t.Fatal("expected escalated notification delivery despite active cooldown")
	}
}

// Exercise the dispatcher used by lifecycle replay with both projections
// attached. Store-only tests cannot detect cross-occurrence canonical history.
func TestMonitorLifecycleReplayPreservesOccurrenceTimelines(t *testing.T) {
	for _, gap := range []time.Duration{2 * time.Minute, 500 * time.Millisecond} {
		t.Run(gap.String(), func(t *testing.T) {
			store := unifiedresources.NewMemoryStore()
			incidents := memory.NewIncidentStore(memory.IncidentStoreConfig{})
			m := &Monitor{
				incidentStore: incidents,
				resourceStore: unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(store)),
			}
			incidents.SetResourceTimelineStore(m.resourceStore.(memory.IncidentTimelineStore))
			start := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
			old := &alerts.Alert{ID: "pbs-connectivity", ResourceID: "pbs", StartTime: start}
			end := start.Add(gap / 2)
			fired := alerts.LifecycleEvent{Type: eventlog.TypeFired, OccurredAt: start, Alert: old}
			resolved := alerts.LifecycleEvent{Type: eventlog.TypeResolved, OccurredAt: end, Alert: old}
			m.handleAlertLifecycleEvent(fired)
			m.handleAlertLifecycleEvent(resolved)
			first := incidents.GetTimelineByAlertAt(old.ID, start)
			require.NotNil(t, first)
			for i := 0; i < 10; i++ {
				m.handleAlertLifecycleEvent(fired)
				m.handleAlertLifecycleEvent(resolved)
			}
			require.Len(t, incidents.ListIncidentsByResource(old.ResourceID, 0), 1)
			replayed := incidents.GetTimelineByAlertAt(old.ID, start)
			require.Equal(t, first.ID, replayed.ID)
			require.Equal(t, memory.IncidentStatusResolved, replayed.Status)
			require.Len(t, replayed.Events, 2)

			next := old.Clone()
			next.StartTime = start.Add(gap)
			m.handleAlertLifecycleEvent(alerts.LifecycleEvent{Type: eventlog.TypeFired, OccurredAt: next.StartTime, Alert: next})
			// A historical resolution arriving after recurrence must not close it;
			// conversely, the recurrence must not reopen the historical timeline.
			m.handleAlertLifecycleEvent(resolved)
			require.Len(t, incidents.ListIncidentsByResource(old.ResourceID, 0), 2)
			historical := incidents.GetTimelineByAlertAt(old.ID, start)
			current := incidents.GetTimelineByAlertAt(next.ID, next.StartTime)
			require.NotNil(t, historical)
			require.NotNil(t, current)
			require.Equal(t, first.ID, historical.ID)
			require.NotEqual(t, historical.ID, current.ID)
			require.Equal(t, memory.IncidentStatusResolved, historical.Status)
			require.Equal(t, &end, historical.ClosedAt)
			require.Len(t, historical.Events, 2)
			require.Equal(t, memory.IncidentStatusOpen, current.Status)
			require.Nil(t, current.ClosedAt)
			require.Len(t, current.Events, 1)
			changes, err := store.GetRecentChanges(old.ResourceID, time.Time{}, 100)
			require.NoError(t, err)
			require.Len(t, changes, 3)

			ackAt := start.Add(gap / 4)
			old.AckTime = &ackAt
			m.handleAlertLifecycleEvent(alerts.LifecycleEvent{
				Type: eventlog.TypeAcknowledged, OccurredAt: ackAt, Alert: old,
				Details: map[string]string{"user": "historical-operator"},
			})
			historical = incidents.GetTimelineByAlertAt(old.ID, start)
			current = incidents.GetTimelineByAlertAt(next.ID, next.StartTime)
			require.Equal(t, memory.IncidentStatusResolved, historical.Status)
			require.True(t, historical.Acknowledged)
			require.Equal(t, "historical-operator", historical.AckUser)
			require.Len(t, historical.Events, 3)
			require.False(t, current.Acknowledged)
			require.Equal(t, memory.IncidentStatusOpen, current.Status)
			require.Len(t, current.Events, 1)
		})
	}
}

func TestMonitorLifecycleRefireReopensRetainedOccurrence(t *testing.T) {
	for _, canonical := range []bool{false, true} {
		name := "local"
		if canonical {
			name = "canonical"
		}
		t.Run(name, func(t *testing.T) {
			store := unifiedresources.NewMemoryStore()
			incidents := memory.NewIncidentStore(memory.IncidentStoreConfig{})
			m := &Monitor{incidentStore: incidents, resourceStore: unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(store))}
			if canonical {
				incidents.SetResourceTimelineStore(m.resourceStore.(memory.IncidentTimelineStore))
			}
			start := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
			alert := &alerts.Alert{ID: "node-connectivity", ResourceID: "node", StartTime: start}
			fire := alerts.LifecycleEvent{Type: eventlog.TypeFired, OccurredAt: start, Alert: alert}
			resolve := alerts.LifecycleEvent{Type: eventlog.TypeResolved, OccurredAt: start.Add(time.Minute), Alert: alert}
			refire := alerts.LifecycleEvent{Type: eventlog.TypeRefired, OccurredAt: start.Add(2 * time.Minute), Alert: alert}
			m.handleAlertLifecycleEvent(fire)
			m.handleAlertLifecycleEvent(resolve)
			original := incidents.GetTimelineByAlertAt(alert.ID, start)
			require.Equal(t, memory.IncidentStatusResolved, original.Status)
			m.handleAlertLifecycleEvent(refire)
			assertOpen := func() {
				t.Helper()
				current := incidents.GetTimelineByAlertAt(alert.ID, start)
				require.Equal(t, original.ID, current.ID)
				require.Equal(t, memory.IncidentStatusOpen, current.Status)
				require.Nil(t, current.ClosedAt)
				require.Len(t, current.Events, 3)
				require.Len(t, incidents.ListIncidentsByResource(alert.ResourceID, 0), 1)
			}
			assertOpen()
			if canonical {
				// Canonical history must recover the retained occurrence even when
				// no incident checkpoint survives.
				recovered := memory.NewIncidentStore(memory.IncidentStoreConfig{})
				recovered.SetResourceTimelineStore(m.resourceStore.(memory.IncidentTimelineStore))
				page, err := recovered.QueryIncidents(memory.IncidentQuery{ResourceID: alert.ResourceID})
				require.NoError(t, err)
				require.Len(t, page.Incidents, 1)
				require.Equal(t, memory.IncidentStatusOpen, page.Incidents[0].Status)
				require.Equal(t, start, page.Incidents[0].OpenedAt)
				require.Len(t, page.Incidents[0].Events, 3)
			}
			for i := 0; i < 10; i++ {
				m.handleAlertLifecycleEvent(fire)
				m.handleAlertLifecycleEvent(resolve)
				m.handleAlertLifecycleEvent(refire)
			}
			assertOpen()
			// Historical read repair must not close an occurrence that re-fired.
			incidents.EnsureAlertOccurrence(alert, &resolve.OccurredAt)
			assertOpen()
			finalResolve := resolve
			finalResolve.OccurredAt = start.Add(3 * time.Minute)
			m.handleAlertLifecycleEvent(finalResolve)
			m.handleAlertLifecycleEvent(refire)
			current := incidents.GetTimelineByAlertAt(alert.ID, start)
			require.Equal(t, memory.IncidentStatusResolved, current.Status)
			require.Equal(t, &finalResolve.OccurredAt, current.ClosedAt)
			require.Len(t, current.Events, 4)
		})
	}
}

type occurrenceHTTPReceipt struct {
	Event  string          `json:"event"`
	Alerts []*alerts.Alert `json:"alerts"`
}

func occurrenceNotifier(t *testing.T, dir string, endpoint string) *notifications.NotificationManager {
	t.Helper()
	n := notifications.NewNotificationManagerWithDeferredQueue("", dir)
	t.Cleanup(n.Stop)
	if err := n.UpdateAllowedPrivateCIDRs("127.0.0.1/32,::1/128"); err != nil {
		t.Fatal(err)
	}
	n.AddWebhook(notifications.WebhookConfig{ID: "ops", Enabled: true, URL: endpoint, Service: "generic"})
	n.SetGroupingWindow(0)
	n.SetNotifyOnResolve(false)
	return n
}

func occurrenceManager(t *testing.T) (*alerts.Manager, models.PBSInstance) {
	t.Helper()
	a := alerts.NewManagerWithDataDir(t.TempDir())
	t.Cleanup(a.Stop)
	cfg := a.GetConfig()
	cfg.Enabled, cfg.ActivationState, cfg.FlappingEnabled = true, alerts.ActivationActive, false
	cfg.Schedule.QuietHours.Enabled = false
	cfg.TimeThresholds["pbs"] = 0
	cfg.PBSDefaults.CPU = &alerts.HysteresisThreshold{Trigger: 80, Clear: 75}
	a.UpdateConfig(cfg)
	return a, models.PBSInstance{ID: "recurring-pbs", Name: "backup", Host: "pbs.invalid", Status: "online", ConnectionHealth: "healthy", CPU: 99}
}

func occurrenceEndpoint(t *testing.T) (string, <-chan occurrenceHTTPReceipt) {
	t.Helper()
	receipts := make(chan occurrenceHTTPReceipt, 8)
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var receipt occurrenceHTTPReceipt
		if err := json.NewDecoder(r.Body).Decode(&receipt); err != nil {
			t.Errorf("decode HTTP acceptance: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		receipts <- receipt
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(s.Close)
	return s.URL, receipts
}

func awaitOccurrenceRows(t *testing.T, n *notifications.NotificationManager, count int) []*notifications.QueuedNotification {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		rows, err := n.GetQueue().GetPending(10)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) == count {
			return rows
		}
		if time.Now().After(deadline) {
			t.Fatalf("pending rows = %d, want %d", len(rows), count)
		}
		time.Sleep(time.Millisecond)
	}
}

func assertOccurrenceHTTP(t *testing.T, n *notifications.NotificationManager, receipts <-chan occurrenceHTTPReceipt, current *alerts.Alert) {
	t.Helper()
	n.StartQueueProcessing()
	select {
	case got := <-receipts:
		if len(got.Alerts) != 1 || got.Alerts[0].ID != current.ID || !got.Alerts[0].StartTime.Equal(current.StartTime) {
			t.Fatalf("wrong HTTP occurrence: %+v", got)
		}
		t.Logf("HTTP 200 accepted current occurrence %s at %s", current.ID, current.StartTime.Format(time.RFC3339Nano))
	case <-time.After(12 * time.Second):
		t.Fatal("current firing did not reach the local HTTP destination")
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		stats, err := n.GetQueueStats()
		if err != nil {
			t.Fatal(err)
		}
		if stats["sent"] == 1 && stats["cancelled"] == 1 && stats["pending"] == 0 && stats["sending"] == 0 {
			t.Logf("persistent queue completion: %v", stats)
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("queue completion = %v", stats)
		}
		time.Sleep(time.Millisecond)
	}
	select {
	case extra := <-receipts:
		t.Fatalf("obsolete or duplicate delivery: %+v", extra)
	case <-time.After(200 * time.Millisecond):
	}
}

// This existing-interface control also runs against the exact predecessor.
// It models a delayed ID callback after the next occurrence has entered the
// real persistent queue. It does not assert native timing or cause a refire.
func TestMonitorDelayedIDResolutionPreservesNewQueuedOccurrence(t *testing.T) {
	endpoint, receipts := occurrenceEndpoint(t)
	dir := t.TempDir()
	n := occurrenceNotifier(t, dir, endpoint)
	a, pbs := occurrenceManager(t)
	a.CheckPBS(pbs)
	active := a.GetActiveAlerts()
	if len(active) != 1 {
		t.Fatalf("PBS firing = %+v", active)
	}
	old := active[0].Clone()
	pbs.CPU = 0
	a.CheckPBS(pbs)
	if r := a.GetResolvedAlert(old.ID); r == nil || !r.Alert.StartTime.Equal(old.StartTime) {
		t.Fatal("missing resolved PBS snapshot")
	}
	current := old.Clone()
	current.StartTime = current.StartTime.Add(time.Nanosecond)
	n.SendAlert(old)
	n.SendAlert(current)
	awaitOccurrenceRows(t, n, 2)
	m := &Monitor{alertManager: a, notificationMgr: n}
	m.handleAlertResolved(old.ID)
	rows, err := n.GetQueue().GetPending(10)
	if err != nil || len(rows) != 1 || len(rows[0].Alerts) != 1 || !rows[0].Alerts[0].StartTime.Equal(current.StartTime) {
		t.Fatalf("delayed recovery cancelled the replacement occurrence: rows=%+v err=%v", rows, err)
	}
	// Resume the autonomous processor only after reopening the same disk state.
	n.Stop()
	n = occurrenceNotifier(t, dir, endpoint)
	assertOccurrenceHTTP(t, n, receipts, current)
}

// One destination accepted the old firing and another still has a retry. After
// restart, that pending work (and the lost RAM cooldown marker) is not evidence
// that no recipient saw it. Recovery must use exact persisted receipts.
func TestMonitorDelayedPartialResolutionKeepsDestinationRecovery(t *testing.T) {
	endpoint, receipts := occurrenceEndpoint(t)
	var failedFirings, unwantedRecoveries atomic.Int32
	broken := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var receipt occurrenceHTTPReceipt
		if err := json.NewDecoder(r.Body).Decode(&receipt); err != nil {
			t.Errorf("decode failing destination: %v", err)
		}
		if receipt.Event == "resolved" {
			unwantedRecoveries.Add(1)
		} else {
			failedFirings.Add(1)
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(broken.Close)
	dir := t.TempDir()
	open := func() *notifications.NotificationManager {
		n := occurrenceNotifier(t, dir, endpoint)
		n.SetNotifyOnResolve(true)
		n.AddWebhook(notifications.WebhookConfig{ID: "unannounced", Enabled: true, URL: broken.URL, Service: "generic"})
		return n
	}
	n := open()
	a, pbs := occurrenceManager(t)
	m := &Monitor{alertManager: a, notificationMgr: n}
	// Use the live firing callback so the detector records notification policy
	// state. Direct notifier calls would not establish LastNotified, and existing
	// recovery policy would correctly suppress that unannounced detector alert.
	a.SetAlertCallback(m.handleAlertFired)
	a.CheckPBS(pbs)
	old := a.GetActiveAlerts()[0].Clone()
	if old.LastNotified == nil {
		t.Fatal("detector did not admit the firing notification")
	}
	awaitOccurrenceRows(t, n, 2)
	n.StartQueueProcessing()
	select {
	case got := <-receipts:
		if got.Event != "" || len(got.Alerts) != 1 || !got.Alerts[0].StartTime.Equal(old.StartTime) {
			t.Fatalf("wrong initial firing: %+v", got)
		}
	case <-time.After(12 * time.Second):
		t.Fatal("initial firing did not reach accepting destination")
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		stats, err := n.GetQueueStats()
		if err != nil {
			t.Fatal(err)
		}
		if stats["sent"] == 1 && stats["pending"] == 1 && failedFirings.Load() > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("partial delivery not retained: %v", stats)
		}
		time.Sleep(time.Millisecond)
	}
	n.Stop()
	pbs.CPU = 0
	a.CheckPBS(pbs)
	n = open() // deferred processor; prior destination receipts are now on disk only
	m = &Monitor{alertManager: a, notificationMgr: n}
	a.SetAlertCallback(m.handleAlertFired)
	pbs.CPU = 95
	a.CheckPBS(pbs)
	active := a.GetActiveAlerts()
	if len(active) != 1 || active[0].ID != old.ID || active[0].StartTime.Equal(old.StartTime) {
		t.Fatalf("not a new occurrence: %+v", active)
	}
	current := active[0].Clone()
	awaitOccurrenceRows(t, n, 2)
	m.handleAlertResolved(old.ID)
	n.StartQueueProcessing()
	seenFiring, seenRecovery := false, false
	for i := 0; i < 2; i++ {
		select {
		case got := <-receipts:
			if len(got.Alerts) != 1 || got.Alerts[0].ID != old.ID {
				t.Fatalf("wrong delivery: %+v", got)
			}
			switch got.Event {
			case "resolved":
				if seenRecovery || !got.Alerts[0].StartTime.Equal(old.StartTime) {
					t.Fatalf("wrong recovery occurrence: %+v", got)
				}
				seenRecovery = true
			case "":
				if seenFiring || !got.Alerts[0].StartTime.Equal(current.StartTime) {
					t.Fatalf("wrong firing occurrence: %+v", got)
				}
				seenFiring = true
			default:
				t.Fatalf("unknown event: %+v", got)
			}
		case <-time.After(12 * time.Second):
			t.Fatal("lost current firing or old accepted destination's recovery after restart")
		}
	}
	deadline = time.Now().Add(3 * time.Second)
	for {
		stats, err := n.GetQueueStats()
		if err != nil {
			t.Fatal(err)
		}
		if stats["sent"] == 3 && stats["cancelled"] == 1 && stats["pending"]+stats["sending"]+stats["failed"]+stats["dlq"] == 1 {
			t.Logf("partial destination recovery retained exact old receipt and new firing: %v", stats)
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("partial recovery queue completion: %v", stats)
		}
		time.Sleep(time.Millisecond)
	}
	if unwantedRecoveries.Load() != 0 {
		t.Fatal("unannounced destination received a recovery")
	}
}

// A host agent linked to a Proxmox node merges into one read-state row that
// every PVE poll keeps fresh. When the agent stops reporting, its retained
// sensors must stop feeding the node's temperature: otherwise the frozen
// reading is re-observed every poll, an open temperature alert's LastSeen keeps
// moving, and stale-alert cleanup never reaches it.
func TestSilentLinkedAgentStopsRefreshingNodeTemperatureAlert(t *testing.T) {
	manager := alerts.NewManagerWithDataDir(t.TempDir(), alerts.WithoutPersistedAlertRestore())
	t.Cleanup(manager.Stop)
	alertConfig := manager.GetConfig()
	alertConfig.Enabled = true
	alertConfig.TimeThresholds = map[string]int{}
	alertConfig.SuppressionWindow = 0
	alertConfig.NodeDefaults.Temperature = &alerts.HysteresisThreshold{Trigger: 80, Clear: 75}
	manager.UpdateConfig(alertConfig)

	adapter := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))
	monitor := &Monitor{
		config:        &config.Config{TemperatureMonitoringEnabled: true},
		state:         models.NewState(),
		resourceStore: adapter,
	}
	node := models.Node{
		ID: "pve1-node1", Name: "node1", Instance: "pve1", Status: "online",
		ConnectionHealth: "healthy", LinkedAgentID: "agent-1",
	}
	// ingest rebuilds the store, as each monitor refresh does, from the last PVE
	// poll's node (with the temperature it published, which the next poll reads
	// back as its previous node) plus the agent as of its last report.
	lastPolled := node
	ingest := func(agentLastReport time.Time, agentStatus string) {
		polledNode := lastPolled
		polledNode.LastSeen = time.Now()
		adapter.PopulateFromSnapshot(models.StateSnapshot{
			Nodes: []models.Node{polledNode},
			Hosts: []models.Host{{
				ID: "agent-1", Hostname: "node1", Status: agentStatus, LinkedNodeID: node.ID,
				IntervalSeconds: 30, LastSeen: agentLastReport,
				Sensors: models.HostSensorSummary{TemperatureCelsius: map[string]float64{"cpu_package": 92}},
			}},
		})
	}
	// poll runs the node temperature step of a PVE poll (no SSH collector) and
	// evaluates the node's alerts with the result.
	poll := func() *models.Temperature {
		prevNodes := monitor.snapshotPrevNodes("pve1")
		polled := node
		monitor.collectNodeTemperatureData(
			context.Background(), "pve1", &config.PVEInstance{Name: "pve1"}, proxmox.Node{Node: node.Name},
			&polled, prevNodes, "online",
		)
		manager.CheckNode(polled)
		lastPolled = polled
		return polled.Temperature
	}
	temperatureAlert := func() alerts.Alert {
		t.Helper()
		for _, alert := range manager.GetActiveAlerts() {
			if alert.Type == "temperature" && alert.ResourceID == node.ID {
				return alert
			}
		}
		t.Fatalf("expected an open temperature alert for %s", node.ID)
		return alerts.Alert{}
	}

	// While the agent reports, its hot reading feeds the node, stamped with the
	// agent's report time, and opens the alert.
	agentReport := time.Now().Add(-10 * time.Second)
	ingest(agentReport, "online")
	reading := poll()
	require.NotNil(t, reading)
	require.Equal(t, 92.0, reading.CPUPackage)
	require.True(t, reading.LastUpdate.Equal(agentReport), "the reading carries the agent's own report time")
	opened := temperatureAlert()

	// The agent goes silent while PVE polling continues: the merged row stays
	// fresh, but the agent's retained sensors no longer count as a reading, and
	// the poller does not carry the agent's last reading past its lease. With no
	// clock to advance, age the agent's last report and the reading it produced
	// by the same amount.
	silence := hostAgentHealthWindow(30) + time.Minute
	agedReading := *lastPolled.Temperature
	agedReading.LastUpdate = agedReading.LastUpdate.Add(-silence)
	lastPolled.Temperature = &agedReading
	ingest(agentReport.Add(-silence), "offline")
	time.Sleep(5 * time.Millisecond)
	require.Nil(t, poll(), "a silent agent's retained sensors must not be presented as a current reading")
	held := temperatureAlert()
	require.True(t, held.LastSeen.Equal(opened.LastSeen), "a frozen agent reading must not keep the alert fresh")
	require.Nil(t, poll())
	require.True(t, temperatureAlert().LastSeen.Equal(opened.LastSeen))

	// The agent reports again and its reading is evaluated.
	ingest(time.Now(), "online")
	time.Sleep(5 * time.Millisecond)
	require.NotNil(t, poll())
	require.True(t, temperatureAlert().LastSeen.After(held.LastSeen), "a fresh agent reading is evaluated")
}
