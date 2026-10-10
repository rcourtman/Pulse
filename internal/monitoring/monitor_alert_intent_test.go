package monitoring

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

func TestInstallOperatorIntentResolverProjectsCanonicalResourcePolicy(t *testing.T) {
	store := unifiedresources.NewMemoryStore()
	registry := unifiedresources.NewRegistry(store)
	registry.IngestResources([]unifiedresources.Resource{{
		ID: "vm:101", Type: unifiedresources.ResourceTypeVM, Name: "database",
	}})
	if err := store.SetResourceOperatorState(unifiedresources.ResourceOperatorState{
		CanonicalID:    "vm:101",
		MonitoringMode: unifiedresources.MonitoringModeMuted,
		LifecycleState: unifiedresources.LifecycleStateRetired,
	}); err != nil {
		t.Fatal(err)
	}

	manager := alerts.NewManagerWithDataDir(t.TempDir())
	t.Cleanup(manager.Stop)
	monitor := &Monitor{alertManager: manager}
	monitor.installOperatorIntentResolver(unifiedresources.NewMonitorAdapter(registry))
	preview, err := manager.PreviewIntentPolicy(alerts.AlertIntentPolicyPreviewRequest{
		ResourceID: "vm:101", ResourceType: "vm", Signal: string(alerts.AlertIntentSignalOffline), ConditionActive: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if preview.Reason != "operator_retired" || preview.Status != "expected_transient" {
		t.Fatalf("canonical operator policy preview = %+v", preview)
	}
}

func TestProxmoxPhysicalDiskMuteResolvesAndSuppressesWearoutAlert(t *testing.T) {
	store := unifiedresources.NewMemoryStore()
	registry := unifiedresources.NewRegistry(store)
	instance, node := "pve", "rocket"
	makeDisk := func(path string) models.PhysicalDisk {
		return models.PhysicalDisk{
			ID:       unifiedresources.ProxmoxPhysicalDiskSourceID(instance, node, path, "", ""),
			Instance: instance, Node: node, DevPath: path,
			Model: "KINGSTON SA400", Type: "ssd", Health: "PASSED", Wearout: 0,
			LastChecked: time.Now().UTC(),
		}
	}
	registry.IngestSnapshot(models.StateSnapshot{
		PhysicalDisks: []models.PhysicalDisk{makeDisk("/dev/sda"), makeDisk("/dev/sdb")},
	})
	diskID := unifiedresources.ProxmoxPhysicalDiskAlertResourceID(instance, node, "/dev/sda")
	canonicalID, found := registry.ResolveReferenceID(diskID)
	if !found {
		t.Fatalf("PVE wearout alert resource %q did not resolve to a physical disk", diskID)
	}
	otherID := unifiedresources.ProxmoxPhysicalDiskAlertResourceID(instance, node, "/dev/sdb")
	if other, ok := registry.ResolveReferenceID(otherID); !ok || other == canonicalID {
		t.Fatalf("other PVE disk identity = %q, %v; want a different disk", other, ok)
	}

	manager := alerts.NewManagerWithDataDir(t.TempDir())
	t.Cleanup(manager.Stop)
	worn := proxmox.Disk{DevPath: "/dev/sda", Model: "KINGSTON SA400", Type: "ssd", Health: "PASSED", Wearout: 0}
	manager.CheckDiskHealth(instance, node, worn)
	if got := len(manager.GetActiveAlerts()); got != 1 {
		t.Fatalf("unmuted worn disk raised %d alerts, want one", got)
	}
	if err := store.SetResourceOperatorState(unifiedresources.ResourceOperatorState{
		CanonicalID: canonicalID, MonitoringMode: unifiedresources.MonitoringModeMuted,
	}); err != nil {
		t.Fatal(err)
	}
	monitor := &Monitor{alertManager: manager}
	monitor.installOperatorIntentResolver(unifiedresources.NewMonitorAdapter(registry))
	if got := len(manager.GetActiveAlerts()); got != 0 {
		t.Fatalf("mute left %d existing PVE wearout alerts active", got)
	}
	manager.CheckDiskHealth(instance, node, worn)
	if got := len(manager.GetActiveAlerts()); got != 0 {
		t.Fatalf("muted PVE disk raised %d new wearout alerts", got)
	}
	worn.DevPath = "/dev/sdb"
	manager.CheckDiskHealth(instance, node, worn)
	if got := len(manager.GetActiveAlerts()); got != 1 {
		t.Fatalf("other PVE disk raised %d alerts, want one", got)
	}
}

func TestInstallOperatorIntentResolverInheritsScopedMaintenanceFromParent(t *testing.T) {
	store := unifiedresources.NewMemoryStore()
	registry := unifiedresources.NewRegistry(store)
	parentID := "node:pve-a"
	registry.IngestResources([]unifiedresources.Resource{
		{ID: parentID, Type: unifiedresources.ResourceTypeAgent, Name: "pve-a"},
		{ID: "vm:101", Type: unifiedresources.ResourceTypeVM, Name: "database", ParentID: &parentID},
	})
	now := time.Now().UTC()
	start, end := now.Add(-time.Hour), now.Add(2*time.Hour)
	if err := store.SetResourceOperatorState(unifiedresources.ResourceOperatorState{
		CanonicalID:        parentID,
		MaintenanceStartAt: &start,
		MaintenanceEndAt:   &end,
		MaintenanceReason:  "hypervisor patching",
		MaintenanceScope:   unifiedresources.MaintenanceScopeResourceAndDescendants,
	}); err != nil {
		t.Fatal(err)
	}

	manager := alerts.NewManagerWithDataDir(t.TempDir())
	t.Cleanup(manager.Stop)
	monitor := &Monitor{alertManager: manager}
	monitor.installOperatorIntentResolver(unifiedresources.NewMonitorAdapter(registry))
	preview, err := manager.PreviewIntentPolicy(alerts.AlertIntentPolicyPreviewRequest{
		ResourceID: "vm:101", ResourceType: "vm", Signal: string(alerts.AlertIntentSignalOffline), ConditionActive: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if preview.Reason != "operator_maintenance" || preview.Status != "expected_transient" {
		t.Fatalf("inherited maintenance preview = %+v", preview)
	}
	if preview.EligibleAt == nil || !preview.EligibleAt.Equal(end) {
		t.Fatalf("eligibleAt = %v, want %v", preview.EligibleAt, end)
	}

	// Scope is explicit: the same parent window must not leak to descendants
	// when changed back to resource-only.
	if err := store.SetResourceOperatorState(unifiedresources.ResourceOperatorState{
		CanonicalID:        parentID,
		MaintenanceStartAt: &start,
		MaintenanceEndAt:   &end,
		MaintenanceScope:   unifiedresources.MaintenanceScopeResource,
	}); err != nil {
		t.Fatal(err)
	}
	preview, err = manager.PreviewIntentPolicy(alerts.AlertIntentPolicyPreviewRequest{
		ResourceID: "vm:101", ResourceType: "vm", Signal: string(alerts.AlertIntentSignalOffline), ConditionActive: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if preview.Status != "would_activate" {
		t.Fatalf("resource-only parent window leaked to child: %+v", preview)
	}
}

func TestPlatformMonitorResourceIdentityConstructors(t *testing.T) {
	if got := PBSMonitorResourceID("backup-main"); got != "pbs-backup-main" {
		t.Fatalf("PBS monitor resource ID = %q", got)
	}
	if got := PMGMonitorResourceID("mail-main"); got != "pmg-mail-main" {
		t.Fatalf("PMG monitor resource ID = %q", got)
	}
}

func TestResolveBackupIntentContextRequiresFreshActiveMatchingEvidence(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	state := models.NewState()
	state.UpdateBackupTasksForInstance("pve-a", []models.BackupTask{
		{
			ID:         "active-101",
			Instance:   "pve-a",
			Node:       "node-a",
			VMID:       101,
			Status:     "running",
			ObservedAt: now.Add(-time.Minute),
		},
		{
			ID:         "stale-102",
			Instance:   "pve-a",
			Node:       "node-a",
			VMID:       102,
			Status:     "running",
			ObservedAt: now.Add(-backupIntentEvidenceMaxAge - time.Second),
		},
		{
			ID:         "finished-103",
			Instance:   "pve-a",
			Node:       "node-a",
			VMID:       103,
			Status:     "OK",
			ObservedAt: now.Add(-time.Minute),
			EndTime:    now.Add(-30 * time.Second),
		},
	})

	monitor := &Monitor{state: state}
	context, found := monitor.resolveBackupIntentContext("", "pve-a", "node-a", 101, now)
	if !found || !context.Active {
		t.Fatalf("fresh active task did not resolve: found=%v context=%+v", found, context)
	}
	if context.ObservedAt != now.Add(-time.Minute) {
		t.Fatalf("observedAt = %v, want %v", context.ObservedAt, now.Add(-time.Minute))
	}
	if context.Evidence != "pve_vzdump_task:active-101" {
		t.Fatalf("evidence = %q, want active task identity", context.Evidence)
	}

	for _, tc := range []struct {
		name     string
		instance string
		node     string
		vmid     int
	}{
		{name: "wrong instance", instance: "pve-b", node: "node-a", vmid: 101},
		{name: "wrong node", instance: "pve-a", node: "node-b", vmid: 101},
		{name: "stale", instance: "pve-a", node: "node-a", vmid: 102},
		{name: "finished", instance: "pve-a", node: "node-a", vmid: 103},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got, ok := monitor.resolveBackupIntentContext("", tc.instance, tc.node, tc.vmid, now); ok {
				t.Fatalf("unexpected backup intent context: %+v", got)
			}
		})
	}
}

// A guest covered by a running multi-guest vzdump job has no task of its own;
// pollBackupTasks synthesizes one from the job log. That synthetic task must
// count as backup-intent evidence so the guest's alerts are suppressed while
// the job is backing it up, and must stop counting once its section finishes.
func TestResolveBackupIntentContextAcceptsSynthesizedJobGuestTask(t *testing.T) {
	now := time.Date(2026, 7, 25, 2, 5, 0, 0, time.UTC)
	jobTask := models.BackupTask{
		ID:        "pve-a-UPID:node-a:000E9F2C:0AC734B2:68A1B2C3:vzdump::root@pam:",
		Node:      "node-a",
		Instance:  "pve-a",
		Type:      "vzdump",
		StartTime: now.Add(-4 * time.Minute),
		// no EndTime: the job is still running
	}

	synthesized := parseVzdumpJobLog(jobTask, "UPID:node-a:000E9F2C:0AC734B2:68A1B2C3:vzdump::root@pam:", []proxmox.TaskLogLine{
		{LineNumber: 1, Text: "INFO: Starting Backup of VM 101 (qemu)"},
		{LineNumber: 2, Text: "INFO: Finished Backup of VM 101 (00:01:30)"},
		{LineNumber: 3, Text: "INFO: Starting Backup of VM 102 (lxc)"},
	})
	for i := range synthesized {
		synthesized[i].ObservedAt = now.Add(-30 * time.Second)
	}

	state := models.NewState()
	state.UpdateBackupTasksForInstance("pve-a", append([]models.BackupTask{jobTask}, synthesized...))
	monitor := &Monitor{state: state}

	context, found := monitor.resolveBackupIntentContext("", "pve-a", "node-a", 102, now)
	if !found || !context.Active {
		t.Fatalf("guest being backed up by running job did not resolve: found=%v context=%+v", found, context)
	}

	if got, ok := monitor.resolveBackupIntentContext("", "pve-a", "node-a", 101, now); ok {
		t.Fatalf("guest whose job section already finished should not carry intent: %+v", got)
	}
}

func TestSyncAlertsToStateCarriesLiveMetricStatusOfHeldAlert(t *testing.T) {
	m := &Monitor{
		state:        models.NewState(),
		alertManager: alerts.NewManagerWithDataDir(t.TempDir()),
	}
	defer m.alertManager.Stop()

	cfg := m.alertManager.GetConfig()
	cfg.TimeThresholds = map[string]int{"node": 0}
	cfg.NodeDefaults.Temperature = &alerts.HysteresisThreshold{Trigger: 80, Clear: 75}
	m.alertManager.UpdateConfig(cfg)

	node := models.Node{
		ID: "homelab-minipc", Name: "minipc", Instance: "homelab", Status: "online",
		Temperature: &models.Temperature{Available: true, CPUPackage: 85},
	}
	m.alertManager.CheckNode(node)
	var breachedAt time.Time
	for _, alert := range m.alertManager.GetActiveAlerts() {
		if alert.Type == "temperature" {
			breachedAt = alert.LastSeen
		}
	}
	if breachedAt.IsZero() {
		t.Fatal("breaching temperature did not open the alert")
	}
	node.Temperature = &models.Temperature{Available: true, CPUPackage: 78}
	m.alertManager.CheckNode(node)

	m.syncAlertsToState()

	var held *models.Alert
	snapshot := m.state.GetSnapshot()
	for i := range snapshot.ActiveAlerts {
		if snapshot.ActiveAlerts[i].Type == "temperature" {
			held = &snapshot.ActiveAlerts[i]
		}
	}
	if held == nil {
		t.Fatal("expected the temperature alert to stay open at 78°C")
	}
	if held.Value != 85 {
		t.Fatalf("legacy value = %v, want the last breach 85", held.Value)
	}
	status := held.MetricStatus
	if status == nil {
		t.Fatal("websocket state must carry the live reading")
	}
	if status.Phase != models.MetricAlertPhaseLatched || status.Value != 78 || status.Recovery != 75 {
		t.Fatalf("live status = %+v, want latched at 78 clearing at 75", status)
	}
	// The concrete /api/state and websocket projection has no LastSeen. The
	// live status must date the held breach without advancing it to this poll.
	data, err := json.Marshal(m.state.ToFrontend())
	if err != nil {
		t.Fatal(err)
	}
	var wire struct {
		ActiveAlerts []struct {
			Type         string     `json:"type"`
			LastSeen     *time.Time `json:"lastSeen"`
			MetricStatus *struct {
				LastBreachAt time.Time `json:"lastBreachAt"`
				ObservedAt   time.Time `json:"observedAt"`
			} `json:"metricStatus"`
		} `json:"activeAlerts"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, alert := range wire.ActiveAlerts {
		if alert.Type != "temperature" {
			continue
		}
		found = true
		if alert.LastSeen != nil {
			t.Fatal("projection added poll-varying legacy LastSeen")
		}
		if alert.MetricStatus == nil || !alert.MetricStatus.LastBreachAt.Equal(breachedAt) {
			t.Fatalf("projected breach time = %+v, want %v", alert.MetricStatus, breachedAt)
		}
		if !alert.MetricStatus.ObservedAt.After(breachedAt) {
			t.Fatal("hold did not carry a newer independent observation")
		}
	}
	if !found {
		t.Fatal("frontend projection lost the held temperature alert")
	}
}

// After a restart, a host that has not reported again exists only through
// saved-host continuity, and the host-offline alert continuity raises names it
// as agent:<host ID>. Intent the operator set on the saved host's row must
// reach that alert, as it does for Patrol findings and resources API reads,
// although the published registry alone does not know the host.
func TestOperatorIntentReachesSavedHostAlertsAfterRestart(t *testing.T) {
	now := time.Now().UTC()
	saved := config.HostContinuityEntry{
		HostID: "host-retired", MachineID: "machine-retired", Hostname: "retired",
		Platform: "linux", LastSeen: now.Add(-10 * time.Minute),
	}
	continuity := config.NewHostContinuityStore(t.TempDir(), nil)
	if err := continuity.Upsert(saved); err != nil {
		t.Fatal(err)
	}
	store := unifiedresources.NewMemoryStore()
	savedID := unifiedresources.MachineIdentityCanonicalID(unifiedresources.ResourceTypeAgent, saved.MachineID)
	if err := store.SetResourceOperatorState(unifiedresources.ResourceOperatorState{
		CanonicalID: savedID, IntentionallyOffline: true,
	}); err != nil {
		t.Fatal(err)
	}
	adapter := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(store))
	adapter.PopulateFromSnapshot(models.StateSnapshot{LastUpdate: now})
	if _, ok := adapter.ResolveCanonicalResourceID("agent:" + saved.HostID); ok {
		t.Fatal("published registry already knows the saved host; the case needs it absent")
	}

	manager := alerts.NewManagerWithDataDir(t.TempDir())
	t.Cleanup(manager.Stop)
	m := &Monitor{state: models.NewState(), alertManager: manager, hostContinuityStore: continuity}
	m.SetResourceStore(adapter)

	preview, err := manager.PreviewIntentPolicy(alerts.AlertIntentPolicyPreviewRequest{
		ResourceID: "agent:" + saved.HostID, ResourceType: "agent",
		Signal: string(alerts.AlertIntentSignalOffline), ConditionActive: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if preview.Reason != "operator_expected_offline" {
		t.Fatalf("saved host offline alert intent = %+v, want the intentionally-offline state set on %s", preview, savedID)
	}

	// The saved host reports again: the published registry knows it, and its
	// alerts keep reading the same row.
	adapter.PopulateFromSnapshot(models.StateSnapshot{
		Hosts: []models.Host{{
			ID: saved.HostID, MachineID: saved.MachineID, Hostname: saved.Hostname,
			Platform: "linux", Status: "online", LastSeen: now, IntervalSeconds: 30,
		}},
		LastUpdate: now.Add(time.Second),
	})
	m.state.Hosts = []models.Host{{ID: saved.HostID, MachineID: saved.MachineID, Hostname: saved.Hostname, Status: "online", LastSeen: now}}
	preview, err = manager.PreviewIntentPolicy(alerts.AlertIntentPolicyPreviewRequest{
		ResourceID: "agent:" + saved.HostID, ResourceType: "agent",
		Signal: string(alerts.AlertIntentSignalOffline), ConditionActive: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if preview.Reason != "operator_expected_offline" {
		t.Fatalf("reporting host offline alert intent = %+v, want the same intentionally-offline state", preview)
	}
}

// Alert intent keeps one saved-host overlay per published generation, so
// lookups made under the alert manager's lock reuse what publication built.
// A new publication replaces it even when it shares the last one's timestamp
// or is stamped earlier, and mock mode answers from the published registry
// and drops the real-mode overlay.
func TestOperatorIntentIdentityKeepsOneOverlayPerGeneration(t *testing.T) {
	now := time.Now().UTC()
	saved := config.HostContinuityEntry{
		HostID: "host-retired", MachineID: "machine-retired", Hostname: "retired",
		Platform: "linux", LastSeen: now.Add(-10 * time.Minute),
	}
	continuity := config.NewHostContinuityStore(t.TempDir(), nil)
	if err := continuity.Upsert(saved); err != nil {
		t.Fatal(err)
	}
	adapter := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(unifiedresources.NewMemoryStore()))
	adapter.PopulateFromSnapshot(models.StateSnapshot{LastUpdate: now.Add(time.Hour)})
	m := &Monitor{state: models.NewState(), hostContinuityStore: continuity}
	identity := m.newOperatorIntentIdentity(adapter)
	if identity == nil {
		t.Fatal("monitor adapter offers no operator intent identity")
	}
	resolvesSavedHost := func(readState any) bool {
		_, ok := readState.(resourceIntentIdentityReader).ResolveCanonicalResourceID("agent:" + saved.HostID)
		return ok
	}

	identity.refresh()
	first := identity.current()
	if first == any(adapter) || !resolvesSavedHost(first) {
		t.Fatal("refresh did not build the saved-host overlay")
	}
	if again := identity.current(); again != first {
		t.Fatal("lookup rebuilt the overlay within one generation")
	}

	// A rebuild of a snapshot stamped in the future keeps its time, so two
	// publications can share a timestamp.
	adapter.PopulateFromSnapshot(models.StateSnapshot{LastUpdate: now.Add(time.Hour)})
	identity.refresh()
	sameStamp := identity.current()
	if sameStamp == first || !resolvesSavedHost(sameStamp) {
		t.Fatal("a publication sharing the last one's timestamp kept its overlay")
	}

	adapter.PopulateFromSnapshot(models.StateSnapshot{LastUpdate: now})
	identity.refresh()
	second := identity.current()
	if second == sameStamp || !resolvesSavedHost(second) {
		t.Fatal("an earlier-stamped generation kept the previous overlay")
	}
	if again := identity.current(); again != second {
		t.Fatal("lookups rebuild the overlay after an earlier-stamped generation")
	}

	previous := mock.IsMockEnabled()
	if err := mock.SetEnabled(true); err != nil {
		t.Fatalf("enable mock mode: %v", err)
	}
	t.Cleanup(func() { _ = mock.SetEnabled(previous) })
	if got := identity.current(); got != any(adapter) {
		t.Fatal("mock mode resolved through the real-mode saved-host overlay")
	}
	identity.refresh()
	if err := mock.SetEnabled(false); err != nil {
		t.Fatalf("disable mock mode: %v", err)
	}
	// Alert evaluation resolves from inside an admitted mock-mode fence call,
	// so the rebuild's own fenced store nests in it.
	var third any
	if !m.mockModeFence.begin().run(func() { third = identity.current() }) {
		t.Fatal("fence refused the lookup")
	}
	if third == any(adapter) || !resolvesSavedHost(third) {
		t.Fatal("leaving mock mode kept answering from the published registry alone")
	}
	if again := identity.current(); again != third {
		t.Fatal("the rebuild made inside a fenced call was not kept")
	}
}
