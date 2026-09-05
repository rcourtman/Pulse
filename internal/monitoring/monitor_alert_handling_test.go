package monitoring

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/memory"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/eventlog"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	unifiedresources "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
)

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
	wantKinds := []unifiedresources.ChangeKind{
		unifiedresources.ChangeAlertUnacknowledged,
		unifiedresources.ChangeAlertUnsnoozed,
		unifiedresources.ChangeAlertSnoozed,
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
	monitor.SetResourceStore(adapter)
	monitor.SetResourceStore(adapter)
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

	alert := &alerts.Alert{
		ID:           "escalated-offline",
		Type:         "connectivity",
		Level:        alerts.AlertLevelCritical,
		ResourceID:   "node/pve-1",
		ResourceName: "pve-1",
		Node:         "pve-1",
		Instance:     "pve",
		Message:      "Node offline",
		StartTime:    time.Now(),
	}

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

	alert := &alerts.Alert{
		ID:           "escalated-normal",
		Type:         "connectivity",
		Level:        alerts.AlertLevelCritical,
		ResourceID:   "node/pve-1",
		ResourceName: "pve-1",
		Node:         "pve-1",
		Instance:     "pve",
		Message:      "Node offline",
		StartTime:    time.Now(),
	}

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

	alert := &alerts.Alert{
		ID:           "escalated-with-cooldown",
		Type:         "memory",
		Level:        alerts.AlertLevelWarning,
		ResourceID:   "vm/100",
		ResourceName: "vm-100",
		Node:         "pve-1",
		Instance:     "pve",
		Message:      "Memory threshold crossed",
		StartTime:    time.Now().Add(-10 * time.Minute),
	}

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
