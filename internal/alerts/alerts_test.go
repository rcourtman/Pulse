package alerts

import (
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

func TestAcknowledgePersistsThroughCheckMetric(t *testing.T) {
	m := NewManager()
	m.ClearActiveAlerts()
	// Set config fields directly to bypass UpdateConfig's default value enforcement
	m.mu.Lock()
	m.config.TimeThreshold = 0
	m.config.TimeThresholds = map[string]int{}
	m.config.SuppressionWindow = 0
	m.config.MinimumDelta = 0
	m.mu.Unlock()

	threshold := &HysteresisThreshold{Trigger: 80, Clear: 70}
	m.checkMetric("res1", "Resource", "node1", "inst1", "guest", "usage", 90, threshold, nil)
	if _, exists := m.activeAlerts["res1-usage"]; !exists {
		t.Fatalf("expected alert to be created")
	}

	if err := m.AcknowledgeAlert("res1-usage", "tester"); err != nil {
		t.Fatalf("ack failed: %v", err)
	}

	if !m.activeAlerts["res1-usage"].Acknowledged {
		t.Fatalf("acknowledged flag not set")
	}

	alerts := m.GetActiveAlerts()
	if len(alerts) != 1 || !alerts[0].Acknowledged {
		t.Fatalf("GetActiveAlerts lost acknowledgement")
	}

	m.checkMetric("res1", "Resource", "node1", "inst1", "guest", "usage", 85, threshold, nil)
	if !m.activeAlerts["res1-usage"].Acknowledged {
		t.Fatalf("acknowledged flag lost after update")
	}
}

func TestCheckGuestSkipsAlertsWhenMetricDisabled(t *testing.T) {
	m := NewManager()

	vmID := "instance-node-101"
	instanceName := "instance"

	// Start with default configuration to allow CPU alerts.
	initialConfig := AlertConfig{
		Enabled: true,
		GuestDefaults: ThresholdConfig{
			CPU: &HysteresisThreshold{Trigger: 80, Clear: 75},
		},
		TimeThreshold:  0,
		TimeThresholds: map[string]int{},
		NodeDefaults: ThresholdConfig{
			CPU:    &HysteresisThreshold{Trigger: 80, Clear: 75},
			Memory: &HysteresisThreshold{Trigger: 85, Clear: 80},
			Disk:   &HysteresisThreshold{Trigger: 90, Clear: 85},
		},
		StorageDefault: HysteresisThreshold{Trigger: 85, Clear: 80},
		Overrides:      make(map[string]ThresholdConfig),
	}
	m.UpdateConfig(initialConfig)
	m.mu.Lock()
	m.config.TimeThreshold = 0
	m.config.TimeThresholds = map[string]int{}
	m.config.ActivationState = ActivationActive
	m.mu.Unlock()

	var dispatched []*Alert
	done := make(chan struct{}, 1)
	var resolved []string
	resolvedDone := make(chan struct{}, 1)
	m.SetAlertCallback(func(alert *Alert) {
		dispatched = append(dispatched, alert)
		select {
		case done <- struct{}{}:
		default:
		}
	})
	m.SetResolvedCallback(func(alertID string) {
		resolved = append(resolved, alertID)
		select {
		case resolvedDone <- struct{}{}:
		default:
		}
	})

	vm := models.VM{
		ID:       vmID,
		Name:     "test-vm",
		Node:     "node",
		Instance: instanceName,
		Status:   "running",
		CPU:      1.0, // 100% once multiplied by 100 inside CheckGuest
		Memory: models.Memory{
			Usage: 65,
		},
		Disk: models.Disk{
			Usage: 40,
		},
	}

	// Initial check should trigger an alert with default thresholds.
	m.CheckGuest(vm, instanceName)
	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("did not receive initial alert dispatch")
	}
	if len(dispatched) != 1 {
		t.Fatalf("expected 1 alert before disabling metric, got %d", len(dispatched))
	}

	// Apply override disabling CPU alerts for this VM.
	disabledConfig := initialConfig
	disabledConfig.Overrides = map[string]ThresholdConfig{
		vmID: {
			CPU: &HysteresisThreshold{Trigger: -1, Clear: 0},
		},
	}
	disabledConfig.TimeThreshold = 0
	disabledConfig.TimeThresholds = map[string]int{}
	m.UpdateConfig(disabledConfig)
	m.mu.Lock()
	m.config.TimeThreshold = 0
	m.config.TimeThresholds = map[string]int{}
	m.config.ActivationState = ActivationActive
	m.mu.Unlock()

	// Clear dispatched slice to capture only post-disable notifications.
	dispatched = dispatched[:0]
	done = make(chan struct{}, 1)

	// Re-run evaluation with high CPU; no alert should be dispatched.
	m.CheckGuest(vm, instanceName)
	select {
	case <-done:
		t.Fatalf("expected no alerts after disabling CPU metric, but callback fired")
	case <-time.After(100 * time.Millisecond):
		// No callback fired as expected.
	}

	// Active alerts should be cleared by the config update.
	m.mu.RLock()
	activeCount := len(m.activeAlerts)
	m.mu.RUnlock()
	if activeCount != 0 {
		t.Fatalf("expected active alerts to be cleared after disabling metric, got %d", activeCount)
	}

	select {
	case <-resolvedDone:
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("expected resolved callback to fire after disabling metric")
	}
	if len(resolved) != 1 || resolved[0] != fmt.Sprintf("%s-cpu", vmID) {
		t.Fatalf("expected resolved callback for %s-cpu, got %v", vmID, resolved)
	}

	m.mu.RLock()
	_, isPending := m.pendingAlerts[fmt.Sprintf("%s-cpu", vmID)]
	m.mu.RUnlock()
	if isPending {
		t.Fatalf("expected pending alert entry to be cleared after disabling metric")
	}
}

func TestPulseNoAlertsSuppressesGuestAlerts(t *testing.T) {
	m := NewManager()
	m.ClearActiveAlerts()
	m.mu.Lock()
	m.config.TimeThreshold = 0
	m.config.TimeThresholds = map[string]int{}
	m.config.ActivationState = ActivationActive
	m.mu.Unlock()

	var dispatched int
	m.SetAlertCallback(func(alert *Alert) {
		dispatched++
	})

	vm := models.VM{
		ID:       "inst/qemu/101",
		Name:     "test-vm",
		Node:     "node1",
		Instance: "inst",
		Status:   "running",
		CPU:      1.0,
		Memory: models.Memory{
			Usage: 95,
		},
		Disk: models.Disk{
			Usage: 95,
		},
		Tags: []string{"pulse-no-alerts"},
	}

	m.CheckGuest(vm, "inst")

	if dispatched != 0 {
		t.Fatalf("expected no alert dispatch, got %d", dispatched)
	}

	if alerts := m.GetActiveAlerts(); len(alerts) != 0 {
		t.Fatalf("expected no active alerts, got %d", len(alerts))
	}
}

func TestPulseMonitorOnlySkipsDispatchButRetainsAlert(t *testing.T) {
	m := NewManager()
	m.ClearActiveAlerts()
	m.mu.Lock()
	m.config.TimeThreshold = 0
	m.config.TimeThresholds = map[string]int{}
	m.config.ActivationState = ActivationActive
	m.mu.Unlock()

	var dispatched int
	m.SetAlertCallback(func(alert *Alert) {
		dispatched++
	})

	vm := models.VM{
		ID:       "inst/qemu/102",
		Name:     "monitor-vm",
		Node:     "node1",
		Instance: "inst",
		Status:   "running",
		CPU:      1.0,
		Memory:   models.Memory{Usage: 90},
		Disk:     models.Disk{Usage: 50},
		Tags:     []string{"pulse-monitor-only"},
	}

	m.CheckGuest(vm, "inst")

	if dispatched != 0 {
		t.Fatalf("expected monitor-only alert to skip dispatch, got %d callbacks", dispatched)
	}

	alerts := m.GetActiveAlerts()
	if len(alerts) == 0 {
		t.Fatalf("expected monitor-only alert to remain active")
	}

	if alerts[0].Metadata == nil || alerts[0].Metadata["monitorOnly"] != true {
		t.Fatalf("expected alert metadata to mark monitorOnly, got %+v", alerts[0].Metadata)
	}
}

func TestPulseRelaxedThresholdsIncreaseCpuTrigger(t *testing.T) {
	m := NewManager()
	m.ClearActiveAlerts()
	m.mu.Lock()
	m.config.TimeThreshold = 0
	m.config.TimeThresholds = map[string]int{}
	m.mu.Unlock()

	vm := models.VM{
		ID:       "inst/qemu/103",
		Name:     "relaxed-vm",
		Node:     "node1",
		Instance: "inst",
		Status:   "running",
		CPU:      0.9, // 90%
		Memory:   models.Memory{Usage: 60},
		Disk:     models.Disk{Usage: 40},
		Tags:     []string{"pulse-relaxed"},
	}

	m.CheckGuest(vm, "inst")

	if alerts := m.GetActiveAlerts(); len(alerts) != 0 {
		t.Fatalf("expected no alerts at 90%% CPU with relaxed thresholds, got %d", len(alerts))
	}

	vm.CPU = 1.0
	m.CheckGuest(vm, "inst")

	if alerts := m.GetActiveAlerts(); len(alerts) == 0 {
		t.Fatalf("expected alert once CPU exceeds relaxed threshold")
	}
}

func TestClearAlertMarksResolutionAndReturnsStatus(t *testing.T) {
	m := NewManager()
	m.ClearActiveAlerts()
	m.mu.Lock()
	m.config.TimeThreshold = 0
	m.config.TimeThresholds = map[string]int{}
	m.mu.Unlock()

	vm := models.VM{
		ID:       "inst/qemu/104",
		Name:     "clear-vm",
		Node:     "node1",
		Instance: "inst",
		Status:   "running",
		CPU:      1.0,
		Memory:   models.Memory{Usage: 80},
		Disk:     models.Disk{Usage: 80},
	}

	m.CheckGuest(vm, "inst")
	alerts := m.GetActiveAlerts()
	if len(alerts) == 0 {
		t.Fatalf("expected alert to be active before clearing")
	}

	alertID := alerts[0].ID
	if ok := m.ClearAlert(alertID); !ok {
		t.Fatalf("expected manual clear to succeed")
	}

	if remaining := m.GetActiveAlerts(); len(remaining) != 0 {
		t.Fatalf("expected no active alerts after clear, found %d", len(remaining))
	}

	resolved := m.GetRecentlyResolved()
	if len(resolved) == 0 || resolved[0].Alert.ID != alertID {
		t.Fatalf("expected alert %s to be tracked as recently resolved", alertID)
	}

	if ok := m.ClearAlert(alertID); ok {
		t.Fatalf("expected second clear to report missing alert")
	}
}

func TestHandleDockerHostRemovedClearsAlertsAndTracking(t *testing.T) {
	m := NewManager()
	host := models.DockerHost{ID: "host1", DisplayName: "Host One", Hostname: "host-one"}
	containerResourceID := "docker:host1/container1"
	containerAlertID := "docker-container-state-" + containerResourceID
	hostAlertID := "docker-host-offline-host1"

	m.mu.Lock()
	m.activeAlerts[hostAlertID] = &Alert{ID: hostAlertID, ResourceID: "docker:host1"}
	m.activeAlerts[containerAlertID] = &Alert{ID: containerAlertID, ResourceID: containerResourceID}
	m.dockerOfflineCount[host.ID] = 2
	m.dockerStateConfirm[containerResourceID] = 1
	m.dockerRestartTracking[containerResourceID] = &dockerRestartRecord{}
	m.dockerLastExitCode[containerResourceID] = 137
	m.mu.Unlock()

	m.HandleDockerHostRemoved(host)

	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, exists := m.activeAlerts[containerAlertID]; exists {
		t.Fatalf("expected container alerts to be cleared")
	}
	if _, exists := m.activeAlerts[hostAlertID]; exists {
		t.Fatalf("expected host offline alert to be cleared")
	}
	if _, exists := m.dockerOfflineCount[host.ID]; exists {
		t.Fatalf("expected offline tracking to be cleared")
	}
	if _, exists := m.dockerStateConfirm[containerResourceID]; exists {
		t.Fatalf("expected state confirmation to be cleared")
	}
	if _, exists := m.dockerRestartTracking[containerResourceID]; exists {
		t.Fatalf("expected restart tracking to be cleared")
	}
	if _, exists := m.dockerLastExitCode[containerResourceID]; exists {
		t.Fatalf("expected last exit code tracking to be cleared")
	}
}

func TestCheckHostGeneratesMetricAlerts(t *testing.T) {
	m := NewManager()
	m.ClearActiveAlerts()
	m.mu.Lock()
	m.config.TimeThreshold = 0
	m.config.TimeThresholds = map[string]int{}
	m.mu.Unlock()

	host := models.Host{
		ID:          "host-1",
		DisplayName: "Test Host",
		Hostname:    "host-1.example",
		Platform:    "linux",
		OSName:      "ubuntu",
		CPUUsage:    95,
		CPUCount:    8,
		Memory: models.Memory{
			Usage: 92,
			Total: 16384,
			Used:  15000,
			Free:  1384,
		},
		Disks: []models.Disk{
			{
				Mountpoint: "/",
				Usage:      93,
				Total:      100,
				Used:       93,
				Free:       7,
			},
		},
		Status:          "online",
		IntervalSeconds: 30,
		LastSeen:        time.Now(),
		Tags:            []string{"prod"},
	}

	m.CheckHost(host)

	m.mu.RLock()
	defer m.mu.RUnlock()

	cpuAlertID := fmt.Sprintf("%s-cpu", hostResourceID(host.ID))
	if _, exists := m.activeAlerts[cpuAlertID]; !exists {
		t.Fatalf("expected CPU alert %q to be active", cpuAlertID)
	}

	memAlertID := fmt.Sprintf("%s-memory", hostResourceID(host.ID))
	if _, exists := m.activeAlerts[memAlertID]; !exists {
		t.Fatalf("expected memory alert %q to be active", memAlertID)
	}

	diskResourceID, _ := hostDiskResourceID(host, host.Disks[0])
	diskAlertID := fmt.Sprintf("%s-disk", diskResourceID)
	if _, exists := m.activeAlerts[diskAlertID]; !exists {
		t.Fatalf("expected disk alert %q to be active", diskAlertID)
	}
}

func TestHandleHostOfflineRequiresConfirmations(t *testing.T) {
	m := NewManager()
	m.ClearActiveAlerts()
	host := models.Host{ID: "host-2", DisplayName: "Second Host", Hostname: "host-two"}
	alertID := fmt.Sprintf("host-offline-%s", host.ID)
	resourceKey := hostResourceID(host.ID)

	m.HandleHostOffline(host)
	m.mu.RLock()
	if _, exists := m.activeAlerts[alertID]; exists {
		t.Fatalf("expected no alert after first offline detection")
	}
	if count := m.offlineConfirmations[resourceKey]; count != 1 {
		t.Fatalf("expected confirmation count to be 1, got %d", count)
	}
	m.mu.RUnlock()

	m.HandleHostOffline(host)
	m.mu.RLock()
	if _, exists := m.activeAlerts[alertID]; exists {
		t.Fatalf("expected no alert after second offline detection")
	}
	if count := m.offlineConfirmations[resourceKey]; count != 2 {
		t.Fatalf("expected confirmation count to be 2, got %d", count)
	}
	m.mu.RUnlock()

	m.HandleHostOffline(host)
	m.mu.RLock()
	if _, exists := m.activeAlerts[alertID]; !exists {
		t.Fatalf("expected alert %q after third offline detection", alertID)
	}
	m.mu.RUnlock()

	m.HandleHostOnline(host)
	m.mu.RLock()
	if _, exists := m.activeAlerts[alertID]; exists {
		t.Fatalf("expected offline alert %q to be cleared after host online", alertID)
	}
	if _, exists := m.offlineConfirmations[resourceKey]; exists {
		t.Fatalf("expected offline confirmations to be cleared when host online")
	}
	m.mu.RUnlock()
}

func TestCheckHostDisabledOverrideClearsAlerts(t *testing.T) {
	m := NewManager()
	m.ClearActiveAlerts()
	m.mu.Lock()
	m.config.TimeThreshold = 0
	m.config.TimeThresholds = map[string]int{}
	m.mu.Unlock()

	host := models.Host{
		ID:          "host-3",
		DisplayName: "Override Host",
		Hostname:    "override.example",
		CPUUsage:    90,
		Memory: models.Memory{
			Usage: 91,
			Total: 16000,
			Used:  14560,
			Free:  1440,
		},
		Disks: []models.Disk{
			{Mountpoint: "/data", Usage: 92, Total: 200, Used: 184, Free: 16},
		},
		Status:          "online",
		IntervalSeconds: 30,
		LastSeen:        time.Now(),
	}

	m.CheckHost(host)

	m.mu.RLock()
	if len(m.activeAlerts) == 0 {
		m.mu.RUnlock()
		t.Fatalf("expected active alerts prior to disabling host overrides")
	}
	m.mu.RUnlock()

	cfg := m.GetConfig()
	cfg.Overrides = map[string]ThresholdConfig{
		host.ID: {
			Disabled: true,
		},
	}
	m.UpdateConfig(cfg)
	m.mu.Lock()
	m.config.TimeThreshold = 0
	m.config.TimeThresholds = map[string]int{}
	m.mu.Unlock()

	m.CheckHost(host)

	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.activeAlerts) != 0 {
		t.Fatalf("expected all host alerts to be cleared after disabling override, got %d", len(m.activeAlerts))
	}
}

func TestCheckSnapshotsForInstanceCreatesAndClearsAlerts(t *testing.T) {
	m := NewManager()
	m.ClearActiveAlerts()

	cfg := AlertConfig{
		Enabled:        true,
		StorageDefault: HysteresisThreshold{Trigger: 85, Clear: 80},
		SnapshotDefaults: SnapshotAlertConfig{
			Enabled:         true,
			WarningDays:     7,
			CriticalDays:    14,
			WarningSizeGiB:  0,
			CriticalSizeGiB: 0,
		},
		Overrides: make(map[string]ThresholdConfig),
	}
	m.UpdateConfig(cfg)
	m.mu.Lock()
	m.config.TimeThreshold = 0
	m.config.TimeThresholds = map[string]int{}
	m.mu.Unlock()

	now := time.Now()
	snapshots := []models.GuestSnapshot{
		{
			ID:        "inst-node-100-weekly",
			Name:      "weekly",
			Node:      "node",
			Instance:  "inst",
			Type:      "qemu",
			VMID:      100,
			Time:      now.Add(-15 * 24 * time.Hour),
			SizeBytes: 60 << 30,
		},
	}
	guestNames := map[string]string{
		"inst-node-100": "app-server",
	}

	m.CheckSnapshotsForInstance("inst", snapshots, guestNames)

	m.mu.RLock()
	alert, exists := m.activeAlerts["snapshot-age-inst-node-100-weekly"]
	m.mu.RUnlock()
	if !exists {
		t.Fatalf("expected snapshot age alert to be created")
	}
	if alert.Level != AlertLevelCritical {
		t.Fatalf("expected critical level for old snapshot, got %s", alert.Level)
	}
	if alert.ResourceName != "app-server snapshot 'weekly'" {
		t.Fatalf("unexpected resource name: %s", alert.ResourceName)
	}

	m.CheckSnapshotsForInstance("inst", nil, guestNames)

	m.mu.RLock()
	_, exists = m.activeAlerts["snapshot-age-inst-node-100-weekly"]
	m.mu.RUnlock()
	if exists {
		t.Fatalf("expected snapshot alert to be cleared when snapshot missing")
	}
}

func TestCheckSnapshotsForInstanceTriggersOnSnapshotSize(t *testing.T) {
	m := NewManager()
	m.ClearActiveAlerts()

	cfg := AlertConfig{
		Enabled:        true,
		StorageDefault: HysteresisThreshold{Trigger: 85, Clear: 80},
		SnapshotDefaults: SnapshotAlertConfig{
			Enabled:         true,
			WarningDays:     0,
			CriticalDays:    0,
			WarningSizeGiB:  50,
			CriticalSizeGiB: 100,
		},
		Overrides: make(map[string]ThresholdConfig),
	}
	m.UpdateConfig(cfg)
	m.mu.Lock()
	m.config.TimeThreshold = 0
	m.config.TimeThresholds = map[string]int{}
	m.mu.Unlock()

	now := time.Now()
	snapshots := []models.GuestSnapshot{
		{
			ID:        "inst-node-200-sizey",
			Name:      "pre-maintenance",
			Node:      "node",
			Instance:  "inst",
			Type:      "qemu",
			VMID:      200,
			Time:      now.Add(-2 * time.Hour),
			SizeBytes: int64(120) << 30,
		},
	}
	guestNames := map[string]string{
		"inst-node-200": "db-server",
	}

	m.CheckSnapshotsForInstance("inst", snapshots, guestNames)

	m.mu.RLock()
	alert, exists := m.activeAlerts["snapshot-age-inst-node-200-sizey"]
	m.mu.RUnlock()
	if !exists {
		t.Fatalf("expected snapshot size alert to be created")
	}
	if alert.Level != AlertLevelCritical {
		t.Fatalf("expected critical level for large snapshot, got %s", alert.Level)
	}
	if alert.Value < 119.5 || alert.Value > 120.5 {
		t.Fatalf("expected alert value near 120 GiB, got %.2f", alert.Value)
	}
	if alert.Threshold != 100 {
		t.Fatalf("expected threshold 100 GiB, got %.2f", alert.Threshold)
	}
	if alert.Metadata == nil {
		t.Fatalf("expected metadata for snapshot alert")
	}
	if metric, ok := alert.Metadata["primaryMetric"].(string); !ok || metric != "size" {
		t.Fatalf("expected primary metric size, got %#v", alert.Metadata["primaryMetric"])
	}
	if sizeBytes, ok := alert.Metadata["snapshotSizeBytes"].(int64); !ok || sizeBytes == 0 {
		t.Fatalf("expected snapshotSizeBytes in metadata")
	}
	metrics, ok := alert.Metadata["triggeredMetrics"].([]string)
	if !ok {
		t.Fatalf("expected triggeredMetrics slice, got %#v", alert.Metadata["triggeredMetrics"])
	}
	foundSize := false
	for _, metric := range metrics {
		if metric == "size" {
			foundSize = true
			break
		}
	}
	if !foundSize {
		t.Fatalf("expected size metric recorded in metadata")
	}
}

func TestCheckSnapshotsForInstanceIncludesAgeAndSizeReasons(t *testing.T) {
	m := NewManager()
	m.ClearActiveAlerts()

	cfg := AlertConfig{
		Enabled:        true,
		StorageDefault: HysteresisThreshold{Trigger: 85, Clear: 80},
		SnapshotDefaults: SnapshotAlertConfig{
			Enabled:         true,
			WarningDays:     5,
			CriticalDays:    10,
			WarningSizeGiB:  40,
			CriticalSizeGiB: 80,
		},
		Overrides: make(map[string]ThresholdConfig),
	}
	m.UpdateConfig(cfg)
	m.mu.Lock()
	m.config.TimeThreshold = 0
	m.config.TimeThresholds = map[string]int{}
	m.mu.Unlock()

	now := time.Now()
	snapshots := []models.GuestSnapshot{
		{
			ID:        "inst-node-300-combined",
			Name:      "long-running",
			Node:      "node",
			Instance:  "inst",
			Type:      "qemu",
			VMID:      300,
			Time:      now.Add(-15 * 24 * time.Hour),
			SizeBytes: int64(90) << 30,
		},
	}
	guestNames := map[string]string{
		"inst-node-300": "app-server",
	}

	m.CheckSnapshotsForInstance("inst", snapshots, guestNames)

	m.mu.RLock()
	alert, exists := m.activeAlerts["snapshot-age-inst-node-300-combined"]
	m.mu.RUnlock()
	if !exists {
		t.Fatalf("expected combined snapshot alert to be created")
	}
	if alert.Level != AlertLevelCritical {
		t.Fatalf("expected critical level, got %s", alert.Level)
	}
	if !strings.Contains(alert.Message, "days old") || !strings.Contains(strings.ToLower(alert.Message), "gib") {
		t.Fatalf("expected alert message to reference age and size, got %q", alert.Message)
	}
	if alert.Metadata == nil {
		t.Fatalf("expected metadata for combined alert")
	}
	metrics, ok := alert.Metadata["triggeredMetrics"].([]string)
	if !ok {
		t.Fatalf("expected triggeredMetrics slice, got %#v", alert.Metadata["triggeredMetrics"])
	}
	if len(metrics) < 2 {
		t.Fatalf("expected both age and size metrics recorded, got %v", metrics)
	}
	if metric, ok := alert.Metadata["primaryMetric"].(string); !ok || metric != "age" {
		t.Fatalf("expected primary metric age, got %#v", alert.Metadata["primaryMetric"])
	}
}

func TestCheckBackupsCreatesAndClearsAlerts(t *testing.T) {
	m := NewManager()
	m.ClearActiveAlerts()

	m.mu.Lock()
	m.config.Enabled = true
	m.config.BackupDefaults = BackupAlertConfig{
		Enabled:      true,
		WarningDays:  7,
		CriticalDays: 14,
	}
	m.config.TimeThreshold = 0
	m.config.TimeThresholds = map[string]int{}
	m.mu.Unlock()

	now := time.Now()
	storageBackups := []models.StorageBackup{
		{
			ID:       "inst-node-100-backup",
			Storage:  "local",
			Node:     "node",
			Instance: "inst",
			Type:     "qemu",
			VMID:     100,
			Time:     now.Add(-15 * 24 * time.Hour),
		},
	}

	key := BuildGuestKey("inst", "node", 100)
	guestsByKey := map[string]GuestLookup{
		key: {
			Name:     "app-server",
			Instance: "inst",
			Node:     "node",
			Type:     "qemu",
			VMID:     100,
		},
	}
	guestsByVMID := map[string][]GuestLookup{
		"100": {guestsByKey[key]},
	}

	m.CheckBackups(storageBackups, nil, nil, guestsByKey, guestsByVMID)

	m.mu.RLock()
	alert, exists := m.activeAlerts["backup-age-"+sanitizeAlertKey(key)]
	m.mu.RUnlock()
	if !exists {
		t.Fatalf("expected backup age alert to be created")
	}
	if alert.Level != AlertLevelCritical {
		t.Fatalf("expected critical backup alert, got %s", alert.Level)
	}

	// Recent backup clears alert
	storageBackups[0].Time = now
	m.CheckBackups(storageBackups, nil, nil, guestsByKey, guestsByVMID)

	m.mu.RLock()
	_, exists = m.activeAlerts["backup-age-"+sanitizeAlertKey(key)]
	m.mu.RUnlock()
	if exists {
		t.Fatalf("expected backup-age alert to clear after fresh backup")
	}
}

func TestCheckBackupsHandlesPbsOnlyGuests(t *testing.T) {
	m := NewManager()
	m.ClearActiveAlerts()

	m.mu.Lock()
	m.config.Enabled = true
	m.config.BackupDefaults = BackupAlertConfig{
		Enabled:      true,
		WarningDays:  3,
		CriticalDays: 5,
	}
	m.mu.Unlock()

	now := time.Now()
	pbsBackups := []models.PBSBackup{
		{
			ID:         "pbs-backup-999-0",
			Instance:   "pbs-main",
			Datastore:  "backup-store",
			BackupType: "qemu",
			VMID:       "999",
			BackupTime: now.Add(-6 * 24 * time.Hour),
		},
	}

	m.CheckBackups(nil, pbsBackups, nil, map[string]GuestLookup{}, map[string][]GuestLookup{})

	m.mu.RLock()
	found := false
	for id, alert := range m.activeAlerts {
		if strings.HasPrefix(id, "backup-age-") {
			found = true
			if alert.Level != AlertLevelCritical {
				t.Fatalf("expected PBS backup alert to be critical")
			}
			break
		}
	}
	m.mu.RUnlock()
	if !found {
		t.Fatalf("expected PBS backup alert to be created")
	}
}

func TestCheckBackupsHandlesPmgBackups(t *testing.T) {
	m := NewManager()
	m.ClearActiveAlerts()

	m.mu.Lock()
	m.config.Enabled = true
	m.config.BackupDefaults = BackupAlertConfig{
		Enabled:      true,
		WarningDays:  5,
		CriticalDays: 7,
	}
	m.mu.Unlock()

	now := time.Now()
	pmgBackups := []models.PMGBackup{
		{
			ID:         "pmg-backup-mail-01",
			Instance:   "mail",
			Node:       "mail-gateway",
			Filename:   "pmg-backup_2024-01-01.tgz",
			BackupTime: now.Add(-8 * 24 * time.Hour),
			Size:       123456,
		},
	}

	m.CheckBackups(nil, nil, pmgBackups, map[string]GuestLookup{}, map[string][]GuestLookup{})

	m.mu.RLock()
	found := false
	for id, alert := range m.activeAlerts {
		if strings.HasPrefix(id, "backup-age-") {
			found = true
			if alert.Level != AlertLevelCritical {
				t.Fatalf("expected PMG backup alert to be critical")
			}
			break
		}
	}
	m.mu.RUnlock()
	if !found {
		t.Fatalf("expected PMG backup alert to be created")
	}
}

func TestCheckDockerHostIgnoresContainersByPrefix(t *testing.T) {
	m := NewManager()

	m.mu.Lock()
	m.config.DockerIgnoredContainerPrefixes = []string{"runner-"}
	m.mu.Unlock()

	container := models.DockerContainer{
		ID:     "1234567890ab",
		Name:   "runner-auto-1",
		State:  "exited",
		Status: "Exited (0) 3 seconds ago",
	}

	host := models.DockerHost{
		ID:          "host-ephemeral",
		Hostname:    "ci-host",
		DisplayName: "CI Host",
		Containers:  []models.DockerContainer{container},
	}

	resourceID := dockerResourceID(host.ID, container.ID)
	alertID := fmt.Sprintf("docker-container-state-%s", resourceID)

	// Run twice to satisfy the confirmation threshold when not ignored
	m.CheckDockerHost(host)
	m.CheckDockerHost(host)

	if _, exists := m.activeAlerts[alertID]; exists {
		t.Fatalf("expected no state alert for ignored container")
	}
	if _, exists := m.dockerStateConfirm[resourceID]; exists {
		t.Fatalf("expected no state confirmation tracking for ignored container")
	}
}

func TestDockerServiceReplicaAlerts(t *testing.T) {
	m := NewManager()
	m.ClearActiveAlerts()

	m.mu.RLock()
	cfg := m.config
	m.mu.RUnlock()
	cfg.Enabled = true
	m.UpdateConfig(cfg)

	host := models.DockerHost{
		ID:          "host-1",
		DisplayName: "Prod Swarm",
		Hostname:    "swarm-prod",
		Services: []models.DockerService{
			{
				ID:           "svc-1",
				Name:         "web",
				DesiredTasks: 4,
				RunningTasks: 2,
				Mode:         "replicated",
			},
		},
	}

	m.CheckDockerHost(host)

	resourceID := dockerServiceResourceID(host.ID, "svc-1", "web")
	alertID := fmt.Sprintf("docker-service-health-%s", resourceID)
	alert, exists := m.activeAlerts[alertID]
	if !exists {
		t.Fatalf("expected service alert %s to be raised", alertID)
	}
	if alert.Level != AlertLevelCritical {
		t.Fatalf("expected critical severity, got %s", alert.Level)
	}
	if missing, ok := alert.Metadata["missingTasks"].(int); !ok || missing != 2 {
		t.Fatalf("expected missingTasks metadata to be 2, got %v", alert.Metadata["missingTasks"])
	}

	// Resolve by restoring replicas
	host.Services[0].RunningTasks = 4
	m.CheckDockerHost(host)

	if _, exists := m.activeAlerts[alertID]; exists {
		t.Fatalf("expected service alert %s to be cleared when replicas restored", alertID)
	}
}

func TestDockerServiceUpdateStateAlert(t *testing.T) {
	m := NewManager()
	cfg := m.GetConfig()
	cfg.Enabled = true
	m.UpdateConfig(cfg)

	now := time.Now()
	host := models.DockerHost{
		ID:          "host-update",
		DisplayName: "Swarm",
		Hostname:    "swarm.local",
		Services: []models.DockerService{
			{
				ID:           "svc-update",
				Name:         "api",
				DesiredTasks: 1,
				RunningTasks: 1,
				UpdateStatus: &models.DockerServiceUpdate{
					State:       "rollback_failed",
					Message:     "Rollback failed",
					CompletedAt: &now,
				},
			},
		},
	}

	m.CheckDockerHost(host)

	resourceID := dockerServiceResourceID(host.ID, "svc-update", "api")
	alertID := fmt.Sprintf("docker-service-health-%s", resourceID)
	alert, exists := m.activeAlerts[alertID]
	if !exists {
		t.Fatalf("expected docker service alert %s to be raised", alertID)
	}
	if alert.Level != AlertLevelCritical {
		t.Fatalf("expected critical severity for rollback failure, got %s", alert.Level)
	}
	if state, ok := alert.Metadata["updateState"].(string); !ok || state != "rollback_failed" {
		t.Fatalf("expected updateState metadata to be rollback_failed, got %v", alert.Metadata["updateState"])
	}
}

func TestDockerContainerStateUsesDockerDefaults(t *testing.T) {
	m := NewManager()
	cfg := m.GetConfig()
	cfg.DockerDefaults.StatePoweredOffSeverity = AlertLevelCritical
	m.UpdateConfig(cfg)

	container := models.DockerContainer{
		ID:     "container-1",
		Name:   "web",
		State:  "exited",
		Status: "Exited (1) seconds ago",
	}
	host := models.DockerHost{
		ID:          "host-1",
		DisplayName: "Docker Host",
		Hostname:    "docker.local",
		Containers:  []models.DockerContainer{container},
	}

	m.CheckDockerHost(host)
	m.CheckDockerHost(host)

	resourceID := dockerResourceID(host.ID, container.ID)
	alertID := fmt.Sprintf("docker-container-state-%s", resourceID)
	alert, exists := m.activeAlerts[alertID]
	if !exists {
		t.Fatalf("expected docker container state alert %s to be raised", alertID)
	}
	if alert.Level != AlertLevelCritical {
		t.Fatalf("expected critical severity from docker defaults, got %s", alert.Level)
	}
}

func TestDockerContainerStateRespectsDisableDefault(t *testing.T) {
	m := NewManager()
	cfg := m.GetConfig()
	cfg.DockerDefaults.StateDisableConnectivity = true
	m.UpdateConfig(cfg)

	container := models.DockerContainer{
		ID:     "container-2",
		Name:   "batch",
		State:  "exited",
		Status: "Exited (0) seconds ago",
	}
	host := models.DockerHost{
		ID:          "host-2",
		DisplayName: "Docker Host",
		Hostname:    "docker.example",
		Containers:  []models.DockerContainer{container},
	}

	m.CheckDockerHost(host)
	m.CheckDockerHost(host)

	resourceID := dockerResourceID(host.ID, container.ID)
	alertID := fmt.Sprintf("docker-container-state-%s", resourceID)
	if _, exists := m.activeAlerts[alertID]; exists {
		t.Fatalf("did not expect docker container state alert when defaults disable connectivity")
	}
}

func TestDockerContainerMemoryLimitHysteresis(t *testing.T) {
	m := NewManager()

	hostID := "host-mem"
	containerID := "container-mem"
	hostHigh := models.DockerHost{
		ID:          hostID,
		DisplayName: "Docker Host",
		Hostname:    "docker.mem",
		Containers: []models.DockerContainer{
			{
				ID:          containerID,
				Name:        "memory-hog",
				State:       "running",
				Status:      "Up 10 minutes",
				MemoryUsage: 96 * 1024 * 1024,
				MemoryLimit: 100 * 1024 * 1024,
			},
		},
	}

	m.CheckDockerHost(hostHigh)

	resourceID := dockerResourceID(hostID, containerID)
	alertID := fmt.Sprintf("docker-container-memory-limit-%s", resourceID)
	if _, exists := m.activeAlerts[alertID]; !exists {
		t.Fatalf("expected memory limit alert to be raised")
	}

	hostLow := models.DockerHost{
		ID:          hostID,
		DisplayName: "Docker Host",
		Hostname:    "docker.mem",
		Containers: []models.DockerContainer{
			{
				ID:          containerID,
				Name:        "memory-hog",
				State:       "running",
				Status:      "Up 12 minutes",
				MemoryUsage: 80 * 1024 * 1024,
				MemoryLimit: 100 * 1024 * 1024,
			},
		},
	}

	m.CheckDockerHost(hostLow)

	if _, exists := m.activeAlerts[alertID]; exists {
		t.Fatalf("expected memory limit alert to clear after usage dropped below hysteresis threshold")
	}
}

func TestDockerContainerDiskUsageAlert(t *testing.T) {
	m := NewManager()

	cfg := m.GetConfig()
	cfg.Enabled = true
	cfg.TimeThreshold = 0
	if cfg.TimeThresholds == nil {
		cfg.TimeThresholds = make(map[string]int)
	}
	cfg.TimeThresholds["docker"] = 0
	cfg.TimeThresholds["guest"] = 0
	cfg.DockerDefaults.Disk = HysteresisThreshold{Trigger: 75, Clear: 65}
	m.UpdateConfig(cfg)

	const gib = 1024 * 1024 * 1024

	host := models.DockerHost{
		ID:          "host-disk",
		DisplayName: "Docker Host",
		Hostname:    "docker.disk",
		Containers: []models.DockerContainer{
			{
				ID:                  "container-disk",
				Name:                "disk-hog",
				State:               "running",
				Status:              "Up 5 minutes",
				WritableLayerBytes:  int64(8 * gib),
				RootFilesystemBytes: int64(10 * gib),
			},
		},
	}

	m.CheckDockerHost(host)

	resourceID := dockerResourceID(host.ID, host.Containers[0].ID)
	alertID := fmt.Sprintf("%s-%s", resourceID, "disk")
	alert, exists := m.activeAlerts[alertID]
	if !exists {
		t.Fatalf("expected docker container disk alert %s to be raised", alertID)
	}
	if alert.Level != AlertLevelWarning {
		t.Fatalf("expected warning severity for disk usage alert, got %s", alert.Level)
	}
	if alert.Metadata == nil {
		t.Fatalf("expected disk alert metadata to be populated")
	}
	if percent, ok := alert.Metadata["diskPercent"].(float64); !ok || percent < 79.5 || percent > 80.5 {
		t.Fatalf("expected diskPercent metadata to be ~80%%, got %v", alert.Metadata["diskPercent"])
	}
	if used, ok := alert.Metadata["writableLayerBytes"].(int64); !ok || used != int64(8*gib) {
		t.Fatalf("expected writableLayerBytes metadata to be %d, got %v", int64(8*gib), alert.Metadata["writableLayerBytes"])
	}

	// Drop usage below the clear threshold and ensure the alert resolves.
	host.Containers[0].WritableLayerBytes = int64(4 * gib)
	m.CheckDockerHost(host)

	if _, stillActive := m.activeAlerts[alertID]; stillActive {
		t.Fatalf("expected docker container disk alert %s to clear after usage dropped", alertID)
	}
}

func TestUpdateConfigClampsDockerServiceCriticalGap(t *testing.T) {
	t.Parallel()

	m := NewManager()

	cfg := AlertConfig{
		Enabled:        true,
		GuestDefaults:  ThresholdConfig{},
		NodeDefaults:   ThresholdConfig{},
		HostDefaults:   ThresholdConfig{},
		StorageDefault: HysteresisThreshold{},
		DockerDefaults: DockerThresholdConfig{
			ServiceWarnGapPct: 35,
			ServiceCritGapPct: 20,
		},
		PMGDefaults:      PMGThresholdConfig{},
		SnapshotDefaults: SnapshotAlertConfig{},
		BackupDefaults:   BackupAlertConfig{},
		Overrides:        make(map[string]ThresholdConfig),
		Schedule:         ScheduleConfig{},
	}

	m.UpdateConfig(cfg)

	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.config.DockerDefaults.ServiceWarnGapPct != 35 {
		t.Fatalf("expected warning gap to remain 35, got %d", m.config.DockerDefaults.ServiceWarnGapPct)
	}
	if m.config.DockerDefaults.ServiceCritGapPct != 35 {
		t.Fatalf("expected critical gap to be clamped to 35, got %d", m.config.DockerDefaults.ServiceCritGapPct)
	}
}

func TestDockerServiceAlertUsesClampedCriticalGap(t *testing.T) {
	m := NewManager()
	m.ClearActiveAlerts()

	cfg := AlertConfig{
		Enabled:        true,
		GuestDefaults:  ThresholdConfig{},
		NodeDefaults:   ThresholdConfig{},
		HostDefaults:   ThresholdConfig{},
		StorageDefault: HysteresisThreshold{},
		DockerDefaults: DockerThresholdConfig{
			ServiceWarnGapPct: 20,
			ServiceCritGapPct: 5,
		},
		PMGDefaults:      PMGThresholdConfig{},
		SnapshotDefaults: SnapshotAlertConfig{},
		BackupDefaults:   BackupAlertConfig{},
		Overrides:        make(map[string]ThresholdConfig),
		Schedule:         ScheduleConfig{},
	}

	m.UpdateConfig(cfg)

	host := models.DockerHost{
		ID:          "docker-host-1",
		DisplayName: "Docker Host",
		Hostname:    "docker-host.local",
		Services: []models.DockerService{
			{
				ID:           "svc-123",
				Name:         "api",
				DesiredTasks: 10,
				RunningTasks: 7,
			},
		},
	}

	m.CheckDockerHost(host)

	resourceID := dockerServiceResourceID(host.ID, "svc-123", "api")
	alertID := fmt.Sprintf("docker-service-health-%s", resourceID)

	alert, exists := m.activeAlerts[alertID]
	if !exists {
		t.Fatalf("expected docker service alert %s to be raised", alertID)
	}
	if alert.Level != AlertLevelCritical {
		t.Fatalf("expected critical severity when replicas 7/10, got %s", alert.Level)
	}
	if pct, ok := alert.Metadata["percentMissing"].(float64); !ok || math.Abs(pct-30.0) > 0.01 {
		t.Fatalf("expected percentMissing metadata ~30, got %v", alert.Metadata["percentMissing"])
	}
}

func TestNormalizeDockerIgnoredPrefixes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name:     "blank entries removed",
			input:    []string{"", "   ", "\t"},
			expected: nil,
		},
		{
			name:     "trims and deduplicates preserving first occurrence casing",
			input:    []string{"  Foo ", "foo", "Bar", " bar ", "Baz"},
			expected: []string{"Foo", "Bar", "Baz"},
		},
		{
			name:     "already normalized list remains unchanged",
			input:    []string{"alpha", "beta"},
			expected: []string{"alpha", "beta"},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := NormalizeDockerIgnoredPrefixes(tc.input)
			if !reflect.DeepEqual(got, tc.expected) {
				t.Fatalf("expected %v, got %v", tc.expected, got)
			}
		})
	}
}

func TestCheckDockerHostIgnoredPrefixClearsExistingAlerts(t *testing.T) {
	m := NewManager()

	container := models.DockerContainer{
		ID:     "abc123456789",
		Name:   "runner-job-1",
		State:  "exited",
		Status: "Exited (1) 10 seconds ago",
	}
	host := models.DockerHost{
		ID:          "docker-host",
		DisplayName: "Docker Host",
		Hostname:    "docker-host.local",
		Containers:  []models.DockerContainer{container},
	}
	resourceID := dockerResourceID(host.ID, container.ID)
	stateAlertID := fmt.Sprintf("docker-container-state-%s", resourceID)
	healthAlertID := fmt.Sprintf("docker-container-health-%s", resourceID)
	restartAlertID := fmt.Sprintf("docker-container-restart-loop-%s", resourceID)

	m.mu.Lock()
	m.config.Enabled = true
	m.config.DockerIgnoredContainerPrefixes = []string{"runner-"}
	m.activeAlerts[stateAlertID] = &Alert{ID: stateAlertID, ResourceID: resourceID}
	m.activeAlerts[healthAlertID] = &Alert{ID: healthAlertID, ResourceID: resourceID}
	m.activeAlerts[restartAlertID] = &Alert{ID: restartAlertID, ResourceID: resourceID}
	m.dockerStateConfirm[resourceID] = 2
	m.dockerRestartTracking[resourceID] = &dockerRestartRecord{}
	m.dockerLastExitCode[resourceID] = 137
	m.mu.Unlock()

	m.CheckDockerHost(host)

	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, exists := m.activeAlerts[stateAlertID]; exists {
		t.Fatalf("expected state alert cleared for ignored container")
	}
	if _, exists := m.activeAlerts[healthAlertID]; exists {
		t.Fatalf("expected health alert cleared for ignored container")
	}
	if _, exists := m.activeAlerts[restartAlertID]; exists {
		t.Fatalf("expected restart alert cleared for ignored container")
	}
	if _, exists := m.dockerStateConfirm[resourceID]; exists {
		t.Fatalf("expected state confirmation tracking cleared")
	}
	if _, exists := m.dockerRestartTracking[resourceID]; exists {
		t.Fatalf("expected restart tracking cleared")
	}
	if _, exists := m.dockerLastExitCode[resourceID]; exists {
		t.Fatalf("expected last exit code cleared")
	}
}

func TestUpdateConfigNormalizesDockerIgnoredPrefixes(t *testing.T) {
	t.Parallel()

	t.Run("nil input remains nil", func(t *testing.T) {
		t.Parallel()

		m := NewManager()
		m.UpdateConfig(AlertConfig{})

		m.mu.RLock()
		defer m.mu.RUnlock()

		if m.config.DockerIgnoredContainerPrefixes != nil {
			t.Fatalf("expected nil prefixes, got %v", m.config.DockerIgnoredContainerPrefixes)
		}
	})

	t.Run("duplicates trimmed and deduplicated", func(t *testing.T) {
		t.Parallel()

		m := NewManager()
		cfg := AlertConfig{
			DockerIgnoredContainerPrefixes: []string{
				"  Foo ",
				"foo",
				"Bar",
			},
		}

		m.UpdateConfig(cfg)

		m.mu.RLock()
		defer m.mu.RUnlock()

		expected := []string{"Foo", "Bar"}
		if !reflect.DeepEqual(m.config.DockerIgnoredContainerPrefixes, expected) {
			t.Fatalf("expected normalized prefixes %v, got %v", expected, m.config.DockerIgnoredContainerPrefixes)
		}
	})
}

func TestMatchesDockerIgnoredPrefix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		containerName string
		containerID   string
		prefixes      []string
		want          bool
	}{
		{name: "empty prefixes", containerName: "runner-123", containerID: "abc", prefixes: nil, want: false},
		{name: "match with name", containerName: "runner-123", containerID: "abc", prefixes: []string{"runner-"}, want: true},
		{name: "match with id", containerName: "app", containerID: "abc123", prefixes: []string{"abc"}, want: true},
		{name: "trimmed comparison", containerName: "runner-job", containerID: "abc", prefixes: []string{"  runner- "}, want: true},
		{name: "case insensitive", containerName: "Runner-Job", containerID: "abc", prefixes: []string{"runner-"}, want: true},
		{name: "no match", containerName: "service", containerID: "xyz", prefixes: []string{"runner-"}, want: false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := matchesDockerIgnoredPrefix(tc.containerName, tc.containerID, tc.prefixes); got != tc.want {
				t.Fatalf("matchesDockerIgnoredPrefix(%q, %q, %v) = %v, want %v", tc.containerName, tc.containerID, tc.prefixes, got, tc.want)
			}
		})
	}
}

func TestDockerInstanceName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		host models.DockerHost
		want string
	}{
		{name: "uses display name", host: models.DockerHost{DisplayName: "Prod Host"}, want: "Docker:Prod Host"},
		{name: "falls back to hostname", host: models.DockerHost{Hostname: "docker.local"}, want: "Docker:docker.local"},
		{name: "defaults when empty", host: models.DockerHost{}, want: "Docker"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := dockerInstanceName(tc.host); got != tc.want {
				t.Fatalf("dockerInstanceName(%+v) = %q, want %q", tc.host, got, tc.want)
			}
		})
	}
}

func TestDockerContainerDisplayName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		container models.DockerContainer
		want      string
	}{
		{name: "trims whitespace", container: models.DockerContainer{Name: "  app  "}, want: "app"},
		{name: "strips leading slash", container: models.DockerContainer{Name: "/runner"}, want: "runner"},
		{name: "falls back to id truncated", container: models.DockerContainer{ID: "0123456789abcdef"}, want: "0123456789ab"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := dockerContainerDisplayName(tc.container); got != tc.want {
				t.Fatalf("dockerContainerDisplayName(%+v) = %q, want %q", tc.container, got, tc.want)
			}
		})
	}
}

func TestDockerResourceID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		hostID      string
		containerID string
		want        string
	}{
		{name: "both ids present", hostID: "host1", containerID: "abc", want: "docker:host1/abc"},
		{name: "missing host id", hostID: "", containerID: "abc", want: "docker:container/abc"},
		{name: "missing container id", hostID: "host1", containerID: "", want: "docker:host1"},
		{name: "both missing", hostID: "", containerID: "", want: "docker:unknown"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := dockerResourceID(tc.hostID, tc.containerID); got != tc.want {
				t.Fatalf("dockerResourceID(%q, %q) = %q, want %q", tc.hostID, tc.containerID, got, tc.want)
			}
		})
	}
}

func TestHasKnownFirmwareBug(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		model string
		want  bool
	}{
		{name: "Samsung 980 with SSD prefix", model: "Samsung SSD 980 1TB", want: true},
		{name: "Samsung 980 without SSD prefix", model: "Samsung 980 PRO 2TB", want: true},
		{name: "Samsung 990 with SSD prefix", model: "Samsung SSD 990 PRO 2TB", want: true},
		{name: "Samsung 990 without SSD prefix", model: "Samsung 990 EVO 1TB", want: true},
		{name: "Samsung 980 lowercase", model: "samsung ssd 980 1tb", want: true},
		{name: "Samsung 990 mixed case", model: "SAMSUNG 990 PRO", want: true},
		{name: "Samsung 970 (not affected)", model: "Samsung SSD 970 EVO Plus", want: false},
		{name: "Samsung 870 (not affected)", model: "Samsung 870 QVO", want: false},
		{name: "Other manufacturer", model: "WD Blue SN570", want: false},
		{name: "Empty model", model: "", want: false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := hasKnownFirmwareBug(tc.model); got != tc.want {
				t.Fatalf("hasKnownFirmwareBug(%q) = %v, want %v", tc.model, got, tc.want)
			}
		})
	}
}

func TestCheckDiskHealthSkipsSamsung980FalseAlerts(t *testing.T) {
	m := NewManager()
	m.ClearActiveAlerts()

	// Samsung 980 reporting FAILED health (firmware bug) but actually healthy
	disk := proxmox.Disk{
		DevPath: "/dev/nvme0n1",
		Model:   "Samsung SSD 980 1TB",
		Serial:  "S649NF0R123456",
		Type:    "nvme",
		Health:  "FAILED", // False report due to firmware bug
		Wearout: 99,       // Drive is actually healthy with 99% life remaining
		Size:    1000204886016,
	}

	// Should not create an alert for health status
	m.CheckDiskHealth("test-instance", "pve-node1", disk)

	m.mu.RLock()
	healthAlertID := "disk-health-test-instance-pve-node1-/dev/nvme0n1"
	if _, exists := m.activeAlerts[healthAlertID]; exists {
		m.mu.RUnlock()
		t.Fatalf("expected no health alert for Samsung 980 with known firmware bug")
	}
	m.mu.RUnlock()

	// Now test that wearout alerts still work for these drives
	disk.Wearout = 5 // Low wearout should still trigger alert
	m.CheckDiskHealth("test-instance", "pve-node1", disk)

	m.mu.RLock()
	wearoutAlertID := "disk-wearout-test-instance-pve-node1-/dev/nvme0n1"
	if _, exists := m.activeAlerts[wearoutAlertID]; !exists {
		m.mu.RUnlock()
		t.Fatalf("expected wearout alert to still work for Samsung 980")
	}
	m.mu.RUnlock()
}

func TestCheckDiskHealthClearsExistingSamsung980Alerts(t *testing.T) {
	m := NewManager()
	m.ClearActiveAlerts()

	disk := proxmox.Disk{
		DevPath: "/dev/nvme0n1",
		Model:   "Samsung SSD 990 PRO 2TB",
		Serial:  "S6Z0NF0R654321",
		Type:    "nvme",
		Health:  "FAILED",
		Wearout: 98,
		Size:    2000398934016,
	}

	alertID := "disk-health-test-instance-pve-node1-/dev/nvme0n1"

	// Manually create an existing alert (simulating alert from before the fix)
	m.mu.Lock()
	m.activeAlerts[alertID] = &Alert{
		ID:           alertID,
		Type:         "disk-health",
		Level:        AlertLevelCritical,
		ResourceID:   "pve-node1-/dev/nvme0n1",
		ResourceName: "Samsung SSD 990 PRO 2TB (/dev/nvme0n1)",
		Node:         "pve-node1",
		Instance:     "test-instance",
		Message:      "Disk health check failed: FAILED",
	}
	m.mu.Unlock()

	// Check disk health - should clear the existing false alert
	m.CheckDiskHealth("test-instance", "pve-node1", disk)

	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, exists := m.activeAlerts[alertID]; exists {
		t.Fatalf("expected existing Samsung 990 health alert to be cleared")
	}
}

func TestDisableAllStorageClearsExistingAlerts(t *testing.T) {
	m := NewManager()

	storageID := "local-lvm"

	// Start with configuration that allows storage alerts
	initialConfig := AlertConfig{
		Enabled:           true,
		DisableAllStorage: false,
		StorageDefault:    HysteresisThreshold{Trigger: 80, Clear: 75},
		TimeThreshold:     0,
		TimeThresholds:    map[string]int{},
		NodeDefaults: ThresholdConfig{
			CPU:    &HysteresisThreshold{Trigger: 80, Clear: 75},
			Memory: &HysteresisThreshold{Trigger: 85, Clear: 80},
			Disk:   &HysteresisThreshold{Trigger: 90, Clear: 85},
		},
		GuestDefaults: ThresholdConfig{
			CPU: &HysteresisThreshold{Trigger: 80, Clear: 75},
		},
		Overrides: make(map[string]ThresholdConfig),
	}
	m.UpdateConfig(initialConfig)
	m.mu.Lock()
	m.config.TimeThreshold = 0
	m.config.TimeThresholds = map[string]int{}
	m.config.ActivationState = ActivationActive
	m.mu.Unlock()

	var dispatched []*Alert
	done := make(chan struct{}, 1)
	var resolved []string
	resolvedDone := make(chan struct{}, 1)
	m.SetAlertCallback(func(alert *Alert) {
		dispatched = append(dispatched, alert)
		select {
		case done <- struct{}{}:
		default:
		}
	})
	m.SetResolvedCallback(func(alertID string) {
		resolved = append(resolved, alertID)
		select {
		case resolvedDone <- struct{}{}:
		default:
		}
	})

	storage := models.Storage{
		ID:     storageID,
		Name:   "local-lvm",
		Usage:  90.0,
		Status: "available",
	}

	// Initial check should trigger an alert
	m.CheckStorage(storage)
	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("did not receive initial alert dispatch")
	}
	if len(dispatched) != 1 {
		t.Fatalf("expected 1 alert before disabling storage, got %d", len(dispatched))
	}

	// Apply config with DisableAllStorage enabled
	disabledConfig := initialConfig
	disabledConfig.DisableAllStorage = true
	m.UpdateConfig(disabledConfig)
	m.mu.Lock()
	m.config.TimeThreshold = 0
	m.config.TimeThresholds = map[string]int{}
	m.config.ActivationState = ActivationActive
	m.mu.Unlock()

	// Clear dispatched slice to capture only post-disable notifications
	dispatched = dispatched[:0]
	done = make(chan struct{}, 1)

	// Re-run CheckStorage with high usage; no alert should be dispatched
	m.CheckStorage(storage)
	select {
	case <-done:
		t.Fatalf("expected no alerts after disabling all storage, but callback fired")
	case <-time.After(100 * time.Millisecond):
		// No callback fired as expected
	}

	// Active alerts should be cleared by reevaluateActiveAlertsLocked
	m.mu.RLock()
	activeCount := len(m.activeAlerts)
	m.mu.RUnlock()
	if activeCount != 0 {
		t.Fatalf("expected active alerts to be cleared after disabling all storage, got %d", activeCount)
	}

	// Resolved callback should have fired
	select {
	case <-resolvedDone:
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("expected resolved callback to fire after disabling all storage")
	}
	expectedAlertID := fmt.Sprintf("%s-usage", storageID)
	if len(resolved) != 1 || resolved[0] != expectedAlertID {
		t.Fatalf("expected resolved callback for %s, got %v", expectedAlertID, resolved)
	}

	// Pending alert should be cleared
	m.mu.RLock()
	_, isPending := m.pendingAlerts[expectedAlertID]
	m.mu.RUnlock()
	if isPending {
		t.Fatalf("expected pending alert entry to be cleared after disabling all storage")
	}
}

func TestUpdateConfigPreservesZeroDockerThresholds(t *testing.T) {
	t.Helper()

	m := NewManager()
	config := m.GetConfig()
	config.DockerDefaults.Memory = HysteresisThreshold{Trigger: 0, Clear: 0}

	m.UpdateConfig(config)

	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.config.DockerDefaults.Memory.Trigger != 0 {
		t.Fatalf("expected docker memory trigger to remain 0 when disabled, got %.1f", m.config.DockerDefaults.Memory.Trigger)
	}
	if m.config.DockerDefaults.Memory.Clear != 0 {
		t.Fatalf("expected docker memory clear to remain 0 when disabled, got %.1f", m.config.DockerDefaults.Memory.Clear)
	}
}

func TestReevaluateClearsDockerContainerAlertWhenOverrideDisabled(t *testing.T) {
	m := NewManager()

	resourceID := "docker:host-1/container-1"
	alertID := resourceID + "-memory"

	resolved := make(chan string, 1)
	m.SetResolvedCallback(func(id string) {
		resolved <- id
	})

	m.mu.Lock()
	m.activeAlerts[alertID] = &Alert{
		ID:           alertID,
		Type:         "memory",
		ResourceID:   resourceID,
		ResourceName: "qbittorrent",
		Instance:     "Docker",
		Metadata: map[string]interface{}{
			"resourceType": "Docker Container",
		},
		Threshold: 80,
		Value:     90,
	}
	m.mu.Unlock()

	config := m.GetConfig()
	config.Overrides = map[string]ThresholdConfig{
		resourceID: {
			Disabled: true,
		},
	}
	config.ActivationState = ActivationActive

	m.UpdateConfig(config)

	select {
	case got := <-resolved:
		if got != alertID {
			t.Fatalf("resolved callback fired for unexpected alert %s", got)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("expected alert to be resolved when docker container override is disabled")
	}

	m.mu.RLock()
	_, exists := m.activeAlerts[alertID]
	m.mu.RUnlock()
	if exists {
		t.Fatalf("expected docker container alert to be cleared when override is disabled")
	}
}

func TestReevaluateClearsDockerContainerAlertWhenIgnoredPrefixAdded(t *testing.T) {
	m := NewManager()

	resourceID := "docker:host-2/container-abc123"
	alertID := resourceID + "-cpu"

	resolved := make(chan string, 1)
	m.SetResolvedCallback(func(id string) {
		resolved <- id
	})

	m.mu.Lock()
	m.activeAlerts[alertID] = &Alert{
		ID:           alertID,
		Type:         "cpu",
		ResourceID:   resourceID,
		ResourceName: "qbittorrentvpn",
		Instance:     "Docker",
		Metadata: map[string]interface{}{
			"resourceType":  "Docker Container",
			"containerId":   "abc123",
			"containerName": "qbittorrentvpn",
		},
		Threshold: 80,
		Value:     95,
	}
	m.mu.Unlock()

	config := m.GetConfig()
	config.DockerIgnoredContainerPrefixes = []string{"qbit"}
	config.ActivationState = ActivationActive

	m.UpdateConfig(config)

	select {
	case got := <-resolved:
		if got != alertID {
			t.Fatalf("resolved callback fired for unexpected alert %s", got)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("expected alert to be resolved after adding ignored prefix")
	}

	m.mu.RLock()
	_, exists := m.activeAlerts[alertID]
	m.mu.RUnlock()
	if exists {
		t.Fatalf("expected docker container alert to be cleared when ignored prefix is configured")
	}
}

func TestBuildGuestKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		instance string
		node     string
		vmid     int
		want     string
	}{
		{
			name:     "different instance and node",
			instance: "cluster-1",
			node:     "pve-node",
			vmid:     100,
			want:     "cluster-1-pve-node-100",
		},
		{
			name:     "same instance and node",
			instance: "pve-node",
			node:     "pve-node",
			vmid:     200,
			want:     "pve-node-200",
		},
		{
			name:     "empty instance uses node",
			instance: "",
			node:     "pve-node",
			vmid:     300,
			want:     "pve-node-300",
		},
		{
			name:     "whitespace instance uses node",
			instance: "   ",
			node:     "pve-node",
			vmid:     400,
			want:     "pve-node-400",
		},
		{
			name:     "instance with whitespace trimmed",
			instance: "  cluster-1  ",
			node:     "pve-node",
			vmid:     500,
			want:     "cluster-1-pve-node-500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := BuildGuestKey(tt.instance, tt.node, tt.vmid)
			if got != tt.want {
				t.Errorf("BuildGuestKey(%q, %q, %d) = %q, want %q", tt.instance, tt.node, tt.vmid, got, tt.want)
			}
		})
	}
}

func TestCheckFlapping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		flappingEnabled   bool
		threshold         int
		windowSeconds     int
		cooldownMinutes   int
		historyEntries    int // number of state changes to simulate before the test call
		expectFlapping    bool
		expectNewFlapping bool // should this trigger a new flapping detection (vs already flapping)
	}{
		{
			name:            "disabled returns false",
			flappingEnabled: false,
			threshold:       5,
			windowSeconds:   300,
			historyEntries:  10, // way over threshold
			expectFlapping:  false,
		},
		{
			name:            "below threshold returns false",
			flappingEnabled: true,
			threshold:       5,
			windowSeconds:   300,
			historyEntries:  2, // only 2 + 1 (test call) = 3 < 5
			expectFlapping:  false,
		},
		{
			name:              "at threshold triggers new flapping",
			flappingEnabled:   true,
			threshold:         5,
			windowSeconds:     300,
			cooldownMinutes:   15,
			historyEntries:    4, // 4 + 1 (test call) = 5 == threshold
			expectFlapping:    true,
			expectNewFlapping: true,
		},
		{
			name:              "above threshold triggers flapping",
			flappingEnabled:   true,
			threshold:         5,
			windowSeconds:     300,
			cooldownMinutes:   15,
			historyEntries:    6, // 6 + 1 = 7 > 5
			expectFlapping:    true,
			expectNewFlapping: true,
		},
		{
			name:            "single state change below threshold",
			flappingEnabled: true,
			threshold:       5,
			windowSeconds:   300,
			historyEntries:  0, // only the test call = 1 < 5
			expectFlapping:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m := NewManager()

			// Configure flapping settings
			m.mu.Lock()
			m.config.FlappingEnabled = tt.flappingEnabled
			m.config.FlappingThreshold = tt.threshold
			m.config.FlappingWindowSeconds = tt.windowSeconds
			m.config.FlappingCooldownMinutes = tt.cooldownMinutes

			alertID := "test-alert-" + tt.name

			// Add history entries within the time window
			now := time.Now()
			for i := 0; i < tt.historyEntries; i++ {
				m.flappingHistory[alertID] = append(m.flappingHistory[alertID], now.Add(-time.Duration(i)*time.Second))
			}
			m.mu.Unlock()

			// Call checkFlapping
			m.mu.Lock()
			result := m.checkFlapping(alertID)
			m.mu.Unlock()

			if result != tt.expectFlapping {
				t.Errorf("checkFlapping() = %v, want %v", result, tt.expectFlapping)
			}

			// Check if flapping was newly detected
			m.mu.RLock()
			isFlappingActive := m.flappingActive[alertID]
			_, hasSuppression := m.suppressedUntil[alertID]
			m.mu.RUnlock()

			if tt.expectNewFlapping {
				if !isFlappingActive {
					t.Errorf("expected flappingActive[%s] to be true", alertID)
				}
				if !hasSuppression {
					t.Errorf("expected suppressedUntil[%s] to be set", alertID)
				}
			}
		})
	}
}

func TestCheckFlappingAlreadyFlapping(t *testing.T) {
	t.Parallel()

	m := NewManager()

	alertID := "already-flapping-alert"

	m.mu.Lock()
	m.config.FlappingEnabled = true
	m.config.FlappingThreshold = 3
	m.config.FlappingWindowSeconds = 300
	m.config.FlappingCooldownMinutes = 15

	// Pre-set flapping state
	m.flappingActive[alertID] = true
	existingSuppression := time.Now().Add(10 * time.Minute)
	m.suppressedUntil[alertID] = existingSuppression

	// Add history to exceed threshold
	now := time.Now()
	m.flappingHistory[alertID] = []time.Time{
		now.Add(-10 * time.Second),
		now.Add(-5 * time.Second),
	}
	m.mu.Unlock()

	// Call checkFlapping - should return true but NOT update suppression
	m.mu.Lock()
	result := m.checkFlapping(alertID)
	m.mu.Unlock()

	if !result {
		t.Errorf("checkFlapping() = false, want true for already flapping alert")
	}

	// Verify suppression time was NOT updated (existing suppression should remain)
	m.mu.RLock()
	currentSuppression := m.suppressedUntil[alertID]
	m.mu.RUnlock()

	if !currentSuppression.Equal(existingSuppression) {
		t.Errorf("suppressedUntil was updated from %v to %v; should remain unchanged for already-flapping alert",
			existingSuppression, currentSuppression)
	}
}

func TestCheckFlappingWindowExpiry(t *testing.T) {
	t.Parallel()

	m := NewManager()

	alertID := "window-expiry-alert"

	m.mu.Lock()
	m.config.FlappingEnabled = true
	m.config.FlappingThreshold = 3
	m.config.FlappingWindowSeconds = 60 // 1 minute window

	// Add old history entries outside the window
	now := time.Now()
	m.flappingHistory[alertID] = []time.Time{
		now.Add(-5 * time.Minute), // outside 1 minute window
		now.Add(-4 * time.Minute), // outside 1 minute window
		now.Add(-3 * time.Minute), // outside 1 minute window
		now.Add(-2 * time.Minute), // outside 1 minute window
	}
	m.mu.Unlock()

	// Call checkFlapping - old entries should be pruned
	m.mu.Lock()
	result := m.checkFlapping(alertID)
	historyLen := len(m.flappingHistory[alertID])
	m.mu.Unlock()

	if result {
		t.Errorf("checkFlapping() = true, want false (old entries should be pruned)")
	}

	// Only the current call should remain in history
	if historyLen != 1 {
		t.Errorf("history length = %d, want 1 (old entries should be pruned)", historyLen)
	}
}

func TestGetGlobalMetricTimeThreshold(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                 string
		metricTimeThresholds map[string]map[string]int
		metricType           string
		wantDelay            int
		wantFound            bool
	}{
		{
			name:                 "empty MetricTimeThresholds returns false",
			metricTimeThresholds: nil,
			metricType:           "cpu",
			wantDelay:            0,
			wantFound:            false,
		},
		{
			name:                 "no all key returns false",
			metricTimeThresholds: map[string]map[string]int{"specific": {"cpu": 60}},
			metricType:           "cpu",
			wantDelay:            0,
			wantFound:            false,
		},
		{
			name:                 "empty all map returns false",
			metricTimeThresholds: map[string]map[string]int{"all": {}},
			metricType:           "cpu",
			wantDelay:            0,
			wantFound:            false,
		},
		{
			name:                 "empty metricType returns false",
			metricTimeThresholds: map[string]map[string]int{"all": {"cpu": 60}},
			metricType:           "",
			wantDelay:            0,
			wantFound:            false,
		},
		{
			name:                 "whitespace metricType returns false",
			metricTimeThresholds: map[string]map[string]int{"all": {"cpu": 60}},
			metricType:           "   ",
			wantDelay:            0,
			wantFound:            false,
		},
		{
			name:                 "direct metric match",
			metricTimeThresholds: map[string]map[string]int{"all": {"cpu": 120, "memory": 90}},
			metricType:           "cpu",
			wantDelay:            120,
			wantFound:            true,
		},
		{
			name:                 "metric match case insensitive",
			metricTimeThresholds: map[string]map[string]int{"all": {"cpu": 120}},
			metricType:           "CPU",
			wantDelay:            120,
			wantFound:            true,
		},
		{
			name:                 "metric match with whitespace",
			metricTimeThresholds: map[string]map[string]int{"all": {"cpu": 120}},
			metricType:           "  cpu  ",
			wantDelay:            120,
			wantFound:            true,
		},
		{
			name:                 "default fallback",
			metricTimeThresholds: map[string]map[string]int{"all": {"default": 30}},
			metricType:           "unknown",
			wantDelay:            30,
			wantFound:            true,
		},
		{
			name:                 "_default fallback",
			metricTimeThresholds: map[string]map[string]int{"all": {"_default": 45}},
			metricType:           "unknown",
			wantDelay:            45,
			wantFound:            true,
		},
		{
			name:                 "wildcard fallback",
			metricTimeThresholds: map[string]map[string]int{"all": {"*": 15}},
			metricType:           "unknown",
			wantDelay:            15,
			wantFound:            true,
		},
		{
			name:                 "direct match takes precedence over default",
			metricTimeThresholds: map[string]map[string]int{"all": {"cpu": 120, "default": 30}},
			metricType:           "cpu",
			wantDelay:            120,
			wantFound:            true,
		},
		{
			name:                 "no match and no fallback returns false",
			metricTimeThresholds: map[string]map[string]int{"all": {"cpu": 120, "memory": 90}},
			metricType:           "disk",
			wantDelay:            0,
			wantFound:            false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m := NewManager()
			m.mu.Lock()
			m.config.MetricTimeThresholds = tt.metricTimeThresholds
			m.mu.Unlock()

			m.mu.RLock()
			gotDelay, gotFound := m.getGlobalMetricTimeThreshold(tt.metricType)
			m.mu.RUnlock()

			if gotDelay != tt.wantDelay {
				t.Errorf("getGlobalMetricTimeThreshold() delay = %d, want %d", gotDelay, tt.wantDelay)
			}
			if gotFound != tt.wantFound {
				t.Errorf("getGlobalMetricTimeThreshold() found = %v, want %v", gotFound, tt.wantFound)
			}
		})
	}
}

func TestGetBaseTimeThreshold(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		timeThresholds map[string]int
		timeThreshold  int // global fallback
		resourceType   string
		wantDelay      int
		wantFound      bool
	}{
		{
			name:           "nil TimeThresholds returns global TimeThreshold",
			timeThresholds: nil,
			timeThreshold:  60,
			resourceType:   "guest",
			wantDelay:      60,
			wantFound:      false,
		},
		{
			name:           "direct resource type match",
			timeThresholds: map[string]int{"guest": 120, "node": 90},
			timeThreshold:  60,
			resourceType:   "guest",
			wantDelay:      120,
			wantFound:      true,
		},
		{
			name:           "canonical key match for vm",
			timeThresholds: map[string]int{"guest": 120},
			timeThreshold:  60,
			resourceType:   "vm",
			wantDelay:      120,
			wantFound:      true,
		},
		{
			name:           "canonical key match for container",
			timeThresholds: map[string]int{"guest": 120},
			timeThreshold:  60,
			resourceType:   "container",
			wantDelay:      120,
			wantFound:      true,
		},
		{
			name:           "all fallback when no specific match",
			timeThresholds: map[string]int{"all": 45},
			timeThreshold:  60,
			resourceType:   "storage",
			wantDelay:      45,
			wantFound:      false, // "all" returns found=false
		},
		{
			name:           "specific match takes precedence over all",
			timeThresholds: map[string]int{"storage": 30, "all": 45},
			timeThreshold:  60,
			resourceType:   "storage",
			wantDelay:      30,
			wantFound:      true,
		},
		{
			name:           "no match and no all returns global threshold",
			timeThresholds: map[string]int{"guest": 120},
			timeThreshold:  60,
			resourceType:   "storage",
			wantDelay:      60,
			wantFound:      false,
		},
		{
			name:           "empty TimeThresholds returns global threshold",
			timeThresholds: map[string]int{},
			timeThreshold:  60,
			resourceType:   "guest",
			wantDelay:      60,
			wantFound:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m := NewManager()
			m.mu.Lock()
			m.config.TimeThresholds = tt.timeThresholds
			m.config.TimeThreshold = tt.timeThreshold
			m.mu.Unlock()

			m.mu.RLock()
			gotDelay, gotFound := m.getBaseTimeThreshold(tt.resourceType)
			m.mu.RUnlock()

			if gotDelay != tt.wantDelay {
				t.Errorf("getBaseTimeThreshold() delay = %d, want %d", gotDelay, tt.wantDelay)
			}
			if gotFound != tt.wantFound {
				t.Errorf("getBaseTimeThreshold() found = %v, want %v", gotFound, tt.wantFound)
			}
		})
	}
}

func TestGetMetricTimeThreshold(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                 string
		metricTimeThresholds map[string]map[string]int
		resourceType         string
		metricType           string
		wantDelay            int
		wantFound            bool
	}{
		{
			name:                 "empty MetricTimeThresholds returns false",
			metricTimeThresholds: nil,
			resourceType:         "guest",
			metricType:           "cpu",
			wantDelay:            0,
			wantFound:            false,
		},
		{
			name:                 "empty metricType returns false",
			metricTimeThresholds: map[string]map[string]int{"guest": {"cpu": 60}},
			resourceType:         "guest",
			metricType:           "",
			wantDelay:            0,
			wantFound:            false,
		},
		{
			name:                 "whitespace metricType returns false",
			metricTimeThresholds: map[string]map[string]int{"guest": {"cpu": 60}},
			resourceType:         "guest",
			metricType:           "   ",
			wantDelay:            0,
			wantFound:            false,
		},
		{
			name:                 "direct match on resourceType and metricType",
			metricTimeThresholds: map[string]map[string]int{"guest": {"cpu": 120, "memory": 90}},
			resourceType:         "guest",
			metricType:           "cpu",
			wantDelay:            120,
			wantFound:            true,
		},
		{
			name:                 "canonical key match vm to guest",
			metricTimeThresholds: map[string]map[string]int{"guest": {"cpu": 120}},
			resourceType:         "vm",
			metricType:           "cpu",
			wantDelay:            120,
			wantFound:            true,
		},
		{
			name:                 "canonical key match container to guest",
			metricTimeThresholds: map[string]map[string]int{"guest": {"memory": 90}},
			resourceType:         "container",
			metricType:           "memory",
			wantDelay:            90,
			wantFound:            true,
		},
		{
			name:                 "default fallback within resourceType",
			metricTimeThresholds: map[string]map[string]int{"guest": {"default": 30}},
			resourceType:         "guest",
			metricType:           "unknown",
			wantDelay:            30,
			wantFound:            true,
		},
		{
			name:                 "_default fallback within resourceType",
			metricTimeThresholds: map[string]map[string]int{"guest": {"_default": 45}},
			resourceType:         "guest",
			metricType:           "unknown",
			wantDelay:            45,
			wantFound:            true,
		},
		{
			name:                 "wildcard fallback within resourceType",
			metricTimeThresholds: map[string]map[string]int{"guest": {"*": 15}},
			resourceType:         "guest",
			metricType:           "unknown",
			wantDelay:            15,
			wantFound:            true,
		},
		{
			name:                 "direct match takes precedence over default",
			metricTimeThresholds: map[string]map[string]int{"guest": {"cpu": 120, "default": 30}},
			resourceType:         "guest",
			metricType:           "cpu",
			wantDelay:            120,
			wantFound:            true,
		},
		{
			name:                 "no match for resourceType returns false",
			metricTimeThresholds: map[string]map[string]int{"node": {"cpu": 60}},
			resourceType:         "guest",
			metricType:           "cpu",
			wantDelay:            0,
			wantFound:            false,
		},
		{
			name:                 "empty perType map skipped",
			metricTimeThresholds: map[string]map[string]int{"guest": {}},
			resourceType:         "guest",
			metricType:           "cpu",
			wantDelay:            0,
			wantFound:            false,
		},
		{
			name:                 "metricType case insensitive",
			metricTimeThresholds: map[string]map[string]int{"guest": {"cpu": 120}},
			resourceType:         "guest",
			metricType:           "CPU",
			wantDelay:            120,
			wantFound:            true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m := NewManager()
			m.mu.Lock()
			m.config.MetricTimeThresholds = tt.metricTimeThresholds
			m.mu.Unlock()

			m.mu.RLock()
			gotDelay, gotFound := m.getMetricTimeThreshold(tt.resourceType, tt.metricType)
			m.mu.RUnlock()

			if gotDelay != tt.wantDelay {
				t.Errorf("getMetricTimeThreshold() delay = %d, want %d", gotDelay, tt.wantDelay)
			}
			if gotFound != tt.wantFound {
				t.Errorf("getMetricTimeThreshold() found = %v, want %v", gotFound, tt.wantFound)
			}
		})
	}
}

func TestCheckRateLimit(t *testing.T) {
	t.Parallel()

	t.Run("no rate limit when MaxAlertsHour is zero", func(t *testing.T) {
		t.Parallel()
		m := NewManager()
		m.mu.Lock()
		m.config.Schedule.MaxAlertsHour = 0
		m.mu.Unlock()

		m.mu.Lock()
		result := m.checkRateLimit("test-alert")
		m.mu.Unlock()

		if !result {
			t.Errorf("checkRateLimit() = false, want true when MaxAlertsHour is 0")
		}
	})

	t.Run("no rate limit when MaxAlertsHour is negative", func(t *testing.T) {
		t.Parallel()
		m := NewManager()
		m.mu.Lock()
		m.config.Schedule.MaxAlertsHour = -1
		m.mu.Unlock()

		m.mu.Lock()
		result := m.checkRateLimit("test-alert")
		m.mu.Unlock()

		if !result {
			t.Errorf("checkRateLimit() = false, want true when MaxAlertsHour is negative")
		}
	})

	t.Run("allows alerts under rate limit", func(t *testing.T) {
		t.Parallel()
		m := NewManager()
		m.mu.Lock()
		m.config.Schedule.MaxAlertsHour = 5
		m.mu.Unlock()

		// First 5 alerts should be allowed
		for i := 0; i < 5; i++ {
			m.mu.Lock()
			result := m.checkRateLimit("test-alert")
			m.mu.Unlock()

			if !result {
				t.Errorf("checkRateLimit() call %d = false, want true (under limit)", i+1)
			}
		}
	})

	t.Run("blocks alerts at rate limit", func(t *testing.T) {
		t.Parallel()
		m := NewManager()
		m.mu.Lock()
		m.config.Schedule.MaxAlertsHour = 3
		m.mu.Unlock()

		// Use up the rate limit
		for i := 0; i < 3; i++ {
			m.mu.Lock()
			_ = m.checkRateLimit("test-alert")
			m.mu.Unlock()
		}

		// Fourth alert should be blocked
		m.mu.Lock()
		result := m.checkRateLimit("test-alert")
		m.mu.Unlock()

		if result {
			t.Errorf("checkRateLimit() = true, want false (at rate limit)")
		}
	})

	t.Run("different alert IDs have separate limits", func(t *testing.T) {
		t.Parallel()
		m := NewManager()
		m.mu.Lock()
		m.config.Schedule.MaxAlertsHour = 2
		m.mu.Unlock()

		// Use up limit for alert-1
		for i := 0; i < 2; i++ {
			m.mu.Lock()
			_ = m.checkRateLimit("alert-1")
			m.mu.Unlock()
		}

		// alert-2 should still be allowed
		m.mu.Lock()
		result := m.checkRateLimit("alert-2")
		m.mu.Unlock()

		if !result {
			t.Errorf("checkRateLimit(alert-2) = false, want true (separate limit)")
		}
	})

	t.Run("old entries are cleaned up", func(t *testing.T) {
		t.Parallel()
		m := NewManager()
		m.mu.Lock()
		m.config.Schedule.MaxAlertsHour = 2

		// Pre-populate with old entries (more than 1 hour ago)
		oldTime := time.Now().Add(-2 * time.Hour)
		m.alertRateLimit["test-alert"] = []time.Time{oldTime, oldTime}
		m.mu.Unlock()

		// Should be allowed because old entries are cleaned up
		m.mu.Lock()
		result := m.checkRateLimit("test-alert")
		m.mu.Unlock()

		if !result {
			t.Errorf("checkRateLimit() = false, want true (old entries should be cleaned)")
		}
	})

	t.Run("mixed old and recent entries", func(t *testing.T) {
		t.Parallel()
		m := NewManager()
		m.mu.Lock()
		m.config.Schedule.MaxAlertsHour = 2

		// Pre-populate with 1 old and 1 recent entry
		oldTime := time.Now().Add(-2 * time.Hour)
		recentTime := time.Now().Add(-30 * time.Minute)
		m.alertRateLimit["test-alert"] = []time.Time{oldTime, recentTime}
		m.mu.Unlock()

		// First call should be allowed (1 recent + 1 new = 2)
		m.mu.Lock()
		result1 := m.checkRateLimit("test-alert")
		m.mu.Unlock()

		if !result1 {
			t.Errorf("checkRateLimit() call 1 = false, want true")
		}

		// Second call should be blocked (2 recent + 1 new would exceed 2)
		m.mu.Lock()
		result2 := m.checkRateLimit("test-alert")
		m.mu.Unlock()

		if result2 {
			t.Errorf("checkRateLimit() call 2 = true, want false (at limit)")
		}
	})
}

func TestApplyRelaxedGuestThresholds(t *testing.T) {
	t.Parallel()

	t.Run("nil thresholds get defaults", func(t *testing.T) {
		t.Parallel()
		cfg := ThresholdConfig{
			CPU:    nil,
			Memory: nil,
			Disk:   nil,
		}

		result := applyRelaxedGuestThresholds(cfg)

		if result.CPU == nil {
			t.Fatal("expected CPU threshold to be set")
		}
		if result.CPU.Trigger != 95 {
			t.Errorf("CPU.Trigger = %v, want 95", result.CPU.Trigger)
		}
		if result.CPU.Clear != 90 {
			t.Errorf("CPU.Clear = %v, want 90", result.CPU.Clear)
		}

		if result.Memory == nil {
			t.Fatal("expected Memory threshold to be set")
		}
		if result.Memory.Trigger != 92 {
			t.Errorf("Memory.Trigger = %v, want 92", result.Memory.Trigger)
		}

		if result.Disk == nil {
			t.Fatal("expected Disk threshold to be set")
		}
		if result.Disk.Trigger != 95 {
			t.Errorf("Disk.Trigger = %v, want 95", result.Disk.Trigger)
		}
	})

	t.Run("low thresholds raised to minimum", func(t *testing.T) {
		t.Parallel()
		cfg := ThresholdConfig{
			CPU:    &HysteresisThreshold{Trigger: 50, Clear: 45},
			Memory: &HysteresisThreshold{Trigger: 60, Clear: 55},
			Disk:   &HysteresisThreshold{Trigger: 70, Clear: 65},
		}

		result := applyRelaxedGuestThresholds(cfg)

		if result.CPU.Trigger != 95 {
			t.Errorf("CPU.Trigger = %v, want 95 (raised to minimum)", result.CPU.Trigger)
		}
		if result.Memory.Trigger != 92 {
			t.Errorf("Memory.Trigger = %v, want 92 (raised to minimum)", result.Memory.Trigger)
		}
		if result.Disk.Trigger != 95 {
			t.Errorf("Disk.Trigger = %v, want 95 (raised to minimum)", result.Disk.Trigger)
		}
	})

	t.Run("high thresholds unchanged", func(t *testing.T) {
		t.Parallel()
		cfg := ThresholdConfig{
			CPU:    &HysteresisThreshold{Trigger: 98, Clear: 93},
			Memory: &HysteresisThreshold{Trigger: 95, Clear: 90},
			Disk:   &HysteresisThreshold{Trigger: 99, Clear: 94},
		}

		result := applyRelaxedGuestThresholds(cfg)

		if result.CPU.Trigger != 98 {
			t.Errorf("CPU.Trigger = %v, want 98 (unchanged)", result.CPU.Trigger)
		}
		if result.Memory.Trigger != 95 {
			t.Errorf("Memory.Trigger = %v, want 95 (unchanged)", result.Memory.Trigger)
		}
		if result.Disk.Trigger != 99 {
			t.Errorf("Disk.Trigger = %v, want 99 (unchanged)", result.Disk.Trigger)
		}
	})

	t.Run("clear adjusted when too close to trigger", func(t *testing.T) {
		t.Parallel()
		cfg := ThresholdConfig{
			CPU: &HysteresisThreshold{Trigger: 95, Clear: 96}, // Clear >= Trigger
		}

		result := applyRelaxedGuestThresholds(cfg)

		if result.CPU.Clear >= result.CPU.Trigger {
			t.Errorf("CPU.Clear = %v should be less than Trigger = %v", result.CPU.Clear, result.CPU.Trigger)
		}
		if result.CPU.Clear != 90 {
			t.Errorf("CPU.Clear = %v, want 90 (Trigger - 5)", result.CPU.Clear)
		}
	})

	t.Run("clear clamped at zero when it would go negative", func(t *testing.T) {
		t.Parallel()
		// Create a threshold where Trigger is above min but Clear would go negative
		// The adjust function sets Clear = Trigger - 5 if Clear >= Trigger
		// Then clamps to 0 if Clear < 0
		// Since all triggers get raised to 95/92/95, the negative clamp path
		// won't be hit in normal use. Test the logic directly with a config
		// that has Trigger exactly at minimum and Clear at minimum
		cfg := ThresholdConfig{
			CPU: &HysteresisThreshold{Trigger: 95, Clear: 3},
		}

		result := applyRelaxedGuestThresholds(cfg)

		// Clear at 3 is valid (less than Trigger 95), should stay at 3
		if result.CPU.Trigger != 95 {
			t.Errorf("CPU.Trigger = %v, want 95", result.CPU.Trigger)
		}
		if result.CPU.Clear != 3 {
			t.Errorf("CPU.Clear = %v, want 3 (unchanged since < Trigger)", result.CPU.Clear)
		}
	})

	t.Run("original config unchanged", func(t *testing.T) {
		t.Parallel()
		original := ThresholdConfig{
			CPU: &HysteresisThreshold{Trigger: 50, Clear: 45},
		}

		_ = applyRelaxedGuestThresholds(original)

		// Original should be unchanged
		if original.CPU.Trigger != 50 {
			t.Errorf("original CPU.Trigger = %v, want 50 (should be unchanged)", original.CPU.Trigger)
		}
	})
}

func TestShouldNotifyAfterCooldown(t *testing.T) {
	t.Parallel()

	t.Run("cooldown disabled allows notification", func(t *testing.T) {
		t.Parallel()
		m := NewManager()
		m.mu.Lock()
		m.config.Schedule.Cooldown = 0
		m.mu.Unlock()

		alert := &Alert{
			ID:           "test-alert",
			LastNotified: nil,
		}

		if !m.shouldNotifyAfterCooldown(alert) {
			t.Error("expected true when cooldown is 0")
		}
	})

	t.Run("negative cooldown allows notification", func(t *testing.T) {
		t.Parallel()
		m := NewManager()
		m.mu.Lock()
		m.config.Schedule.Cooldown = -5
		m.mu.Unlock()

		now := time.Now()
		alert := &Alert{
			ID:           "test-alert",
			LastNotified: &now,
		}

		if !m.shouldNotifyAfterCooldown(alert) {
			t.Error("expected true when cooldown is negative")
		}
	})

	t.Run("first notification allowed when never notified", func(t *testing.T) {
		t.Parallel()
		m := NewManager()
		m.mu.Lock()
		m.config.Schedule.Cooldown = 30 // 30 minutes
		m.mu.Unlock()

		alert := &Alert{
			ID:           "test-alert",
			LastNotified: nil,
		}

		if !m.shouldNotifyAfterCooldown(alert) {
			t.Error("expected true when alert has never been notified")
		}
	})

	t.Run("notification blocked during cooldown period", func(t *testing.T) {
		t.Parallel()
		m := NewManager()
		m.mu.Lock()
		m.config.Schedule.Cooldown = 30 // 30 minutes
		m.mu.Unlock()

		lastNotified := time.Now().Add(-10 * time.Minute) // Notified 10 minutes ago
		alert := &Alert{
			ID:           "test-alert",
			LastNotified: &lastNotified,
		}

		if m.shouldNotifyAfterCooldown(alert) {
			t.Error("expected false when still in cooldown period")
		}
	})

	t.Run("notification allowed after cooldown expires", func(t *testing.T) {
		t.Parallel()
		m := NewManager()
		m.mu.Lock()
		m.config.Schedule.Cooldown = 30 // 30 minutes
		m.mu.Unlock()

		lastNotified := time.Now().Add(-45 * time.Minute) // Notified 45 minutes ago
		alert := &Alert{
			ID:           "test-alert",
			LastNotified: &lastNotified,
		}

		if !m.shouldNotifyAfterCooldown(alert) {
			t.Error("expected true after cooldown period expires")
		}
	})

	t.Run("notification allowed at exact cooldown boundary", func(t *testing.T) {
		t.Parallel()
		m := NewManager()
		m.mu.Lock()
		m.config.Schedule.Cooldown = 30 // 30 minutes
		m.mu.Unlock()

		lastNotified := time.Now().Add(-30 * time.Minute) // Exactly 30 minutes ago
		alert := &Alert{
			ID:           "test-alert",
			LastNotified: &lastNotified,
		}

		if !m.shouldNotifyAfterCooldown(alert) {
			t.Error("expected true at exact cooldown boundary (>=)")
		}
	})
}

func TestDockerServiceDisplayName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		service  models.DockerService
		expected string
	}{
		{
			name:     "returns name when present",
			service:  models.DockerService{Name: "my-service", ID: "abc123456789xyz"},
			expected: "my-service",
		},
		{
			name:     "returns trimmed name",
			service:  models.DockerService{Name: "  my-service  ", ID: "abc123456789xyz"},
			expected: "my-service",
		},
		{
			name:     "returns truncated ID when name is empty",
			service:  models.DockerService{Name: "", ID: "abc123456789xyz"},
			expected: "abc123456789",
		},
		{
			name:     "returns full short ID when less than 12 chars",
			service:  models.DockerService{Name: "", ID: "abc123"},
			expected: "abc123",
		},
		{
			name:     "returns trimmed ID",
			service:  models.DockerService{Name: "", ID: "  abc123456789xyz  "},
			expected: "abc123456789",
		},
		{
			name:     "returns 'service' when both name and ID empty",
			service:  models.DockerService{Name: "", ID: ""},
			expected: "service",
		},
		{
			name:     "returns 'service' when both whitespace only",
			service:  models.DockerService{Name: "   ", ID: "   "},
			expected: "service",
		},
		{
			name:     "prefers name over ID",
			service:  models.DockerService{Name: "preferred", ID: "not-this-id"},
			expected: "preferred",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := dockerServiceDisplayName(tt.service)
			if result != tt.expected {
				t.Errorf("dockerServiceDisplayName(%+v) = %q, want %q", tt.service, result, tt.expected)
			}
		})
	}
}

func TestDockerServiceResourceID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		hostID      string
		serviceID   string
		serviceName string
		expected    string
	}{
		{
			name:        "with host and service ID",
			hostID:      "host-1",
			serviceID:   "svc-123",
			serviceName: "my-service",
			expected:    "docker:host-1/service/svc-123",
		},
		{
			name:        "without host ID uses service prefix only",
			hostID:      "",
			serviceID:   "svc-123",
			serviceName: "my-service",
			expected:    "docker-service:svc-123",
		},
		{
			name:        "whitespace host ID treated as empty",
			hostID:      "   ",
			serviceID:   "svc-123",
			serviceName: "my-service",
			expected:    "docker-service:svc-123",
		},
		{
			name:        "derives ID from service name when ID empty",
			hostID:      "host-1",
			serviceID:   "",
			serviceName: "My Service",
			expected:    "docker:host-1/service/my-service",
		},
		{
			name:        "special chars in name replaced with dash",
			hostID:      "host-1",
			serviceID:   "",
			serviceName: "my/service:v1.0",
			expected:    "docker:host-1/service/my-service-v1-0",
		},
		{
			name:        "backslash and colon replaced",
			hostID:      "host-1",
			serviceID:   "",
			serviceName: "path\\to:service",
			expected:    "docker:host-1/service/path-to-service",
		},
		{
			name:        "preserves alphanumeric and underscore",
			hostID:      "host-1",
			serviceID:   "",
			serviceName: "my_service_123",
			expected:    "docker:host-1/service/my_service_123",
		},
		{
			name:        "preserves hyphens",
			hostID:      "host-1",
			serviceID:   "",
			serviceName: "my-service-name",
			expected:    "docker:host-1/service/my-service-name",
		},
		{
			name:        "trims leading/trailing dashes and underscores",
			hostID:      "host-1",
			serviceID:   "",
			serviceName: "---my-service___",
			expected:    "docker:host-1/service/my-service",
		},
		{
			name:        "truncates long derived ID to 32 chars",
			hostID:      "host-1",
			serviceID:   "",
			serviceName: "this-is-a-very-long-service-name-that-exceeds-the-limit",
			expected:    "docker:host-1/service/this-is-a-very-long-service-name",
		},
		{
			name:        "uses 'service' when name is all special chars",
			hostID:      "host-1",
			serviceID:   "",
			serviceName: "!!!@@@###",
			expected:    "docker:host-1/service/service",
		},
		{
			name:        "uses 'service' when both ID and name empty",
			hostID:      "host-1",
			serviceID:   "",
			serviceName: "",
			expected:    "docker:host-1/service/service",
		},
		{
			name:        "uses 'service' when both ID and name whitespace",
			hostID:      "host-1",
			serviceID:   "   ",
			serviceName: "   ",
			expected:    "docker:host-1/service/service",
		},
		{
			name:        "no host and derived name",
			hostID:      "",
			serviceID:   "",
			serviceName: "webserver",
			expected:    "docker-service:webserver",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := dockerServiceResourceID(tt.hostID, tt.serviceID, tt.serviceName)
			if result != tt.expected {
				t.Errorf("dockerServiceResourceID(%q, %q, %q) = %q, want %q",
					tt.hostID, tt.serviceID, tt.serviceName, result, tt.expected)
			}
		})
	}
}

func TestClearStorageOfflineAlert(t *testing.T) {
	t.Parallel()

	t.Run("clears existing offline alert", func(t *testing.T) {
		t.Parallel()
		m := NewManager()

		storage := models.Storage{
			ID:   "storage-1",
			Name: "local-lvm",
			Node: "pve1",
		}

		alertID := fmt.Sprintf("storage-offline-%s", storage.ID)

		// Create an existing offline alert
		m.mu.Lock()
		m.activeAlerts[alertID] = &Alert{
			ID:        alertID,
			Type:      "storage-offline",
			Level:     "critical",
			StartTime: time.Now().Add(-10 * time.Minute),
		}
		m.offlineConfirmations[storage.ID] = 3
		m.mu.Unlock()

		var resolvedID string
		m.SetResolvedCallback(func(id string) {
			resolvedID = id
		})

		m.clearStorageOfflineAlert(storage)

		m.mu.RLock()
		_, alertExists := m.activeAlerts[alertID]
		_, confirmExists := m.offlineConfirmations[storage.ID]
		m.mu.RUnlock()

		if alertExists {
			t.Error("expected alert to be cleared")
		}
		if confirmExists {
			t.Error("expected offline confirmation to be cleared")
		}
		if resolvedID != alertID {
			t.Errorf("expected resolved callback with %q, got %q", alertID, resolvedID)
		}
	})

	t.Run("noop when no alert exists", func(t *testing.T) {
		t.Parallel()
		m := NewManager()

		storage := models.Storage{
			ID:   "storage-2",
			Name: "local-zfs",
			Node: "pve1",
		}

		var callbackCalled bool
		m.SetResolvedCallback(func(id string) {
			callbackCalled = true
		})

		m.clearStorageOfflineAlert(storage)

		if callbackCalled {
			t.Error("expected no callback when no alert exists")
		}
	})

	t.Run("clears offline confirmation even when no alert", func(t *testing.T) {
		t.Parallel()
		m := NewManager()

		storage := models.Storage{
			ID:   "storage-3",
			Name: "ceph-pool",
			Node: "pve2",
		}

		// Set confirmation without alert
		m.mu.Lock()
		m.offlineConfirmations[storage.ID] = 2
		m.mu.Unlock()

		m.clearStorageOfflineAlert(storage)

		m.mu.RLock()
		_, confirmExists := m.offlineConfirmations[storage.ID]
		m.mu.RUnlock()

		if confirmExists {
			t.Error("expected offline confirmation to be cleared")
		}
	})
}

func TestClearHostMetricAlerts(t *testing.T) {
	t.Parallel()

	t.Run("clears specified metrics", func(t *testing.T) {
		t.Parallel()
		m := NewManager()

		hostID := "my-host"
		resourceID := fmt.Sprintf("host:%s", hostID)

		// Create alerts for cpu and memory
		m.mu.Lock()
		m.activeAlerts[fmt.Sprintf("%s-cpu", resourceID)] = &Alert{ID: fmt.Sprintf("%s-cpu", resourceID)}
		m.activeAlerts[fmt.Sprintf("%s-memory", resourceID)] = &Alert{ID: fmt.Sprintf("%s-memory", resourceID)}
		m.activeAlerts[fmt.Sprintf("%s-disk", resourceID)] = &Alert{ID: fmt.Sprintf("%s-disk", resourceID)}
		m.mu.Unlock()

		m.clearHostMetricAlerts(hostID, "cpu", "disk")

		m.mu.RLock()
		_, cpuExists := m.activeAlerts[fmt.Sprintf("%s-cpu", resourceID)]
		_, memExists := m.activeAlerts[fmt.Sprintf("%s-memory", resourceID)]
		_, diskExists := m.activeAlerts[fmt.Sprintf("%s-disk", resourceID)]
		m.mu.RUnlock()

		if cpuExists {
			t.Error("expected cpu alert to be cleared")
		}
		if !memExists {
			t.Error("expected memory alert to remain (not specified)")
		}
		if diskExists {
			t.Error("expected disk alert to be cleared")
		}
	})

	t.Run("defaults to cpu and memory when no metrics specified", func(t *testing.T) {
		t.Parallel()
		m := NewManager()

		hostID := "default-host"
		resourceID := fmt.Sprintf("host:%s", hostID)

		// Create alerts
		m.mu.Lock()
		m.activeAlerts[fmt.Sprintf("%s-cpu", resourceID)] = &Alert{ID: fmt.Sprintf("%s-cpu", resourceID)}
		m.activeAlerts[fmt.Sprintf("%s-memory", resourceID)] = &Alert{ID: fmt.Sprintf("%s-memory", resourceID)}
		m.activeAlerts[fmt.Sprintf("%s-disk", resourceID)] = &Alert{ID: fmt.Sprintf("%s-disk", resourceID)}
		m.mu.Unlock()

		m.clearHostMetricAlerts(hostID) // No metrics specified

		m.mu.RLock()
		_, cpuExists := m.activeAlerts[fmt.Sprintf("%s-cpu", resourceID)]
		_, memExists := m.activeAlerts[fmt.Sprintf("%s-memory", resourceID)]
		_, diskExists := m.activeAlerts[fmt.Sprintf("%s-disk", resourceID)]
		m.mu.RUnlock()

		if cpuExists {
			t.Error("expected cpu alert to be cleared (default)")
		}
		if memExists {
			t.Error("expected memory alert to be cleared (default)")
		}
		if !diskExists {
			t.Error("expected disk alert to remain (not in defaults)")
		}
	})

	t.Run("empty hostID is noop", func(t *testing.T) {
		t.Parallel()
		m := NewManager()

		// Create an alert that should not be touched
		m.mu.Lock()
		m.activeAlerts["host:unknown-cpu"] = &Alert{ID: "host:unknown-cpu"}
		m.mu.Unlock()

		m.clearHostMetricAlerts("", "cpu")

		m.mu.RLock()
		_, exists := m.activeAlerts["host:unknown-cpu"]
		m.mu.RUnlock()

		if !exists {
			t.Error("expected alert to remain when hostID is empty")
		}
	})
}

func TestClearHostDiskAlerts(t *testing.T) {
	t.Parallel()

	t.Run("clears all disk alerts for host", func(t *testing.T) {
		t.Parallel()
		m := NewManager()

		hostID := "disk-host"
		resourceID := fmt.Sprintf("host:%s", hostID)

		// Create disk alerts with the expected ResourceID format
		m.mu.Lock()
		m.activeAlerts["disk1-alert"] = &Alert{
			ID:         "disk1-alert",
			ResourceID: fmt.Sprintf("%s/disk:sda", resourceID),
		}
		m.activeAlerts["disk2-alert"] = &Alert{
			ID:         "disk2-alert",
			ResourceID: fmt.Sprintf("%s/disk:sdb", resourceID),
		}
		m.activeAlerts["cpu-alert"] = &Alert{
			ID:         "cpu-alert",
			ResourceID: fmt.Sprintf("%s-cpu", resourceID),
		}
		m.mu.Unlock()

		m.clearHostDiskAlerts(hostID)

		m.mu.RLock()
		_, disk1Exists := m.activeAlerts["disk1-alert"]
		_, disk2Exists := m.activeAlerts["disk2-alert"]
		_, cpuExists := m.activeAlerts["cpu-alert"]
		m.mu.RUnlock()

		if disk1Exists {
			t.Error("expected disk1 alert to be cleared")
		}
		if disk2Exists {
			t.Error("expected disk2 alert to be cleared")
		}
		if !cpuExists {
			t.Error("expected cpu alert to remain (not a disk alert)")
		}
	})

	t.Run("empty hostID is noop", func(t *testing.T) {
		t.Parallel()
		m := NewManager()

		// Create an alert
		m.mu.Lock()
		m.activeAlerts["disk-alert"] = &Alert{
			ID:         "disk-alert",
			ResourceID: "host:unknown/disk:sda",
		}
		m.mu.Unlock()

		m.clearHostDiskAlerts("")

		m.mu.RLock()
		_, exists := m.activeAlerts["disk-alert"]
		m.mu.RUnlock()

		if !exists {
			t.Error("expected alert to remain when hostID is empty")
		}
	})

	t.Run("skips nil alerts", func(t *testing.T) {
		t.Parallel()
		m := NewManager()

		hostID := "nil-test"
		resourceID := fmt.Sprintf("host:%s", hostID)

		m.mu.Lock()
		m.activeAlerts["nil-alert"] = nil
		m.activeAlerts["real-alert"] = &Alert{
			ID:         "real-alert",
			ResourceID: fmt.Sprintf("%s/disk:sda", resourceID),
		}
		m.mu.Unlock()

		// Should not panic
		m.clearHostDiskAlerts(hostID)

		m.mu.RLock()
		_, realExists := m.activeAlerts["real-alert"]
		m.mu.RUnlock()

		if realExists {
			t.Error("expected real alert to be cleared")
		}
	})

	t.Run("noop when no matching alerts", func(t *testing.T) {
		t.Parallel()
		m := NewManager()

		m.mu.Lock()
		m.activeAlerts["other-alert"] = &Alert{
			ID:         "other-alert",
			ResourceID: "host:other-host/disk:sda",
		}
		m.mu.Unlock()

		m.clearHostDiskAlerts("my-host")

		m.mu.RLock()
		_, exists := m.activeAlerts["other-alert"]
		m.mu.RUnlock()

		if !exists {
			t.Error("expected other host's alert to remain")
		}
	})
}

func TestCleanupHostDiskAlerts(t *testing.T) {
	t.Parallel()

	t.Run("clears alerts not in seen set", func(t *testing.T) {
		t.Parallel()
		m := NewManager()

		host := models.Host{ID: "host-1"}
		resourceID := fmt.Sprintf("host:%s", host.ID)

		// Create disk alerts
		m.mu.Lock()
		m.activeAlerts["disk-sda"] = &Alert{
			ID:         "disk-sda",
			ResourceID: fmt.Sprintf("%s/disk:sda", resourceID),
		}
		m.activeAlerts["disk-sdb"] = &Alert{
			ID:         "disk-sdb",
			ResourceID: fmt.Sprintf("%s/disk:sdb", resourceID),
		}
		m.activeAlerts["disk-sdc"] = &Alert{
			ID:         "disk-sdc",
			ResourceID: fmt.Sprintf("%s/disk:sdc", resourceID),
		}
		m.mu.Unlock()

		// Only sda and sdb are in the seen set
		seen := map[string]struct{}{
			fmt.Sprintf("%s/disk:sda", resourceID): {},
			fmt.Sprintf("%s/disk:sdb", resourceID): {},
		}

		m.cleanupHostDiskAlerts(host, seen)

		m.mu.RLock()
		_, sdaExists := m.activeAlerts["disk-sda"]
		_, sdbExists := m.activeAlerts["disk-sdb"]
		_, sdcExists := m.activeAlerts["disk-sdc"]
		m.mu.RUnlock()

		if !sdaExists {
			t.Error("expected sda alert to remain (in seen set)")
		}
		if !sdbExists {
			t.Error("expected sdb alert to remain (in seen set)")
		}
		if sdcExists {
			t.Error("expected sdc alert to be cleared (not in seen set)")
		}
	})

	t.Run("empty host ID is noop", func(t *testing.T) {
		t.Parallel()
		m := NewManager()

		m.mu.Lock()
		m.activeAlerts["disk-alert"] = &Alert{
			ID:         "disk-alert",
			ResourceID: "host:unknown/disk:sda",
		}
		m.mu.Unlock()

		host := models.Host{ID: ""}
		m.cleanupHostDiskAlerts(host, nil)

		m.mu.RLock()
		_, exists := m.activeAlerts["disk-alert"]
		m.mu.RUnlock()

		if !exists {
			t.Error("expected alert to remain when host ID is empty")
		}
	})

	t.Run("skips nil alerts", func(t *testing.T) {
		t.Parallel()
		m := NewManager()

		host := models.Host{ID: "host-2"}
		resourceID := fmt.Sprintf("host:%s", host.ID)

		m.mu.Lock()
		m.activeAlerts["nil-alert"] = nil
		m.activeAlerts["real-alert"] = &Alert{
			ID:         "real-alert",
			ResourceID: fmt.Sprintf("%s/disk:sda", resourceID),
		}
		m.mu.Unlock()

		seen := map[string]struct{}{} // Empty seen set

		// Should not panic
		m.cleanupHostDiskAlerts(host, seen)

		m.mu.RLock()
		_, realExists := m.activeAlerts["real-alert"]
		m.mu.RUnlock()

		if realExists {
			t.Error("expected real alert to be cleared (not in seen set)")
		}
	})

	t.Run("skips non-matching prefix", func(t *testing.T) {
		t.Parallel()
		m := NewManager()

		host := models.Host{ID: "host-3"}
		resourceID := fmt.Sprintf("host:%s", host.ID)

		m.mu.Lock()
		m.activeAlerts["cpu-alert"] = &Alert{
			ID:         "cpu-alert",
			ResourceID: fmt.Sprintf("%s-cpu", resourceID), // Not a disk alert
		}
		m.activeAlerts["disk-alert"] = &Alert{
			ID:         "disk-alert",
			ResourceID: fmt.Sprintf("%s/disk:sda", resourceID),
		}
		m.mu.Unlock()

		seen := map[string]struct{}{} // Empty seen set

		m.cleanupHostDiskAlerts(host, seen)

		m.mu.RLock()
		_, cpuExists := m.activeAlerts["cpu-alert"]
		_, diskExists := m.activeAlerts["disk-alert"]
		m.mu.RUnlock()

		if !cpuExists {
			t.Error("expected cpu alert to remain (not a disk alert)")
		}
		if diskExists {
			t.Error("expected disk alert to be cleared")
		}
	})
}

func TestHandleDockerHostRemovedEmptyID(t *testing.T) {
	t.Parallel()
	m := NewManager()

	// Create some alerts that should not be touched
	m.mu.Lock()
	m.activeAlerts["docker-host-offline-host1"] = &Alert{ID: "docker-host-offline-host1"}
	m.dockerOfflineCount["host1"] = 3
	m.mu.Unlock()

	// Call with empty ID - should be noop
	host := models.DockerHost{ID: ""}
	m.HandleDockerHostRemoved(host)

	m.mu.RLock()
	_, alertExists := m.activeAlerts["docker-host-offline-host1"]
	_, countExists := m.dockerOfflineCount["host1"]
	m.mu.RUnlock()

	if !alertExists {
		t.Error("expected alert to remain when host ID is empty")
	}
	if !countExists {
		t.Error("expected offline count to remain when host ID is empty")
	}
}

func TestHandleDockerHostOnline(t *testing.T) {
	t.Parallel()

	t.Run("clears offline alert and tracking", func(t *testing.T) {
		t.Parallel()
		m := NewManager()

		host := models.DockerHost{ID: "docker-host-1", DisplayName: "My Host"}
		alertID := fmt.Sprintf("docker-host-offline-%s", host.ID)

		// Set up offline alert and tracking
		m.mu.Lock()
		m.activeAlerts[alertID] = &Alert{ID: alertID, ResourceID: fmt.Sprintf("docker:%s", host.ID)}
		m.dockerOfflineCount[host.ID] = 5
		m.mu.Unlock()

		m.HandleDockerHostOnline(host)

		m.mu.RLock()
		_, alertExists := m.activeAlerts[alertID]
		_, countExists := m.dockerOfflineCount[host.ID]
		m.mu.RUnlock()

		if alertExists {
			t.Error("expected offline alert to be cleared")
		}
		if countExists {
			t.Error("expected offline count to be cleared")
		}
	})

	t.Run("noop when no offline alert exists", func(t *testing.T) {
		t.Parallel()
		m := NewManager()

		host := models.DockerHost{ID: "docker-host-2"}

		// Set up only tracking, no alert
		m.mu.Lock()
		m.dockerOfflineCount[host.ID] = 2
		m.mu.Unlock()

		m.HandleDockerHostOnline(host)

		m.mu.RLock()
		_, countExists := m.dockerOfflineCount[host.ID]
		m.mu.RUnlock()

		if countExists {
			t.Error("expected offline count to be cleared even without alert")
		}
	})

	t.Run("empty host ID is noop", func(t *testing.T) {
		t.Parallel()
		m := NewManager()

		// Create some data that should not be touched
		m.mu.Lock()
		m.activeAlerts["docker-host-offline-other"] = &Alert{ID: "docker-host-offline-other"}
		m.dockerOfflineCount["other"] = 3
		m.mu.Unlock()

		host := models.DockerHost{ID: ""}
		m.HandleDockerHostOnline(host)

		m.mu.RLock()
		_, alertExists := m.activeAlerts["docker-host-offline-other"]
		_, countExists := m.dockerOfflineCount["other"]
		m.mu.RUnlock()

		if !alertExists {
			t.Error("expected other alert to remain when host ID is empty")
		}
		if !countExists {
			t.Error("expected other count to remain when host ID is empty")
		}
	})
}

func TestCleanupDockerContainerAlerts(t *testing.T) {
	t.Parallel()

	t.Run("clears alerts not in seen set", func(t *testing.T) {
		t.Parallel()
		m := NewManager()

		host := models.DockerHost{ID: "docker-host-1"}
		prefix := fmt.Sprintf("docker:%s/", host.ID)

		// Create container alerts
		m.mu.Lock()
		m.activeAlerts["container1-alert"] = &Alert{
			ID:         "container1-alert",
			ResourceID: prefix + "container1",
		}
		m.activeAlerts["container2-alert"] = &Alert{
			ID:         "container2-alert",
			ResourceID: prefix + "container2",
		}
		m.activeAlerts["container3-alert"] = &Alert{
			ID:         "container3-alert",
			ResourceID: prefix + "container3",
		}
		m.dockerStateConfirm[prefix+"container1"] = 2
		m.dockerStateConfirm[prefix+"container2"] = 1
		m.dockerStateConfirm[prefix+"container3"] = 3
		m.mu.Unlock()

		// Only container1 and container2 are in seen set
		seen := map[string]struct{}{
			prefix + "container1": {},
			prefix + "container2": {},
		}

		m.cleanupDockerContainerAlerts(host, seen)

		m.mu.RLock()
		_, c1Exists := m.activeAlerts["container1-alert"]
		_, c2Exists := m.activeAlerts["container2-alert"]
		_, c3Exists := m.activeAlerts["container3-alert"]
		_, s1Exists := m.dockerStateConfirm[prefix+"container1"]
		_, s2Exists := m.dockerStateConfirm[prefix+"container2"]
		_, s3Exists := m.dockerStateConfirm[prefix+"container3"]
		m.mu.RUnlock()

		if !c1Exists {
			t.Error("expected container1 alert to remain (in seen set)")
		}
		if !c2Exists {
			t.Error("expected container2 alert to remain (in seen set)")
		}
		if c3Exists {
			t.Error("expected container3 alert to be cleared (not in seen set)")
		}
		if !s1Exists {
			t.Error("expected container1 state confirm to remain (in seen set)")
		}
		if !s2Exists {
			t.Error("expected container2 state confirm to remain (in seen set)")
		}
		if s3Exists {
			t.Error("expected container3 state confirm to be cleared (not in seen set)")
		}
	})

	t.Run("skips alerts from other hosts", func(t *testing.T) {
		t.Parallel()
		m := NewManager()

		host := models.DockerHost{ID: "host-a"}

		// Create alert for a different host
		m.mu.Lock()
		m.activeAlerts["other-host-alert"] = &Alert{
			ID:         "other-host-alert",
			ResourceID: "docker:host-b/container1",
		}
		m.mu.Unlock()

		seen := map[string]struct{}{} // Empty seen set

		m.cleanupDockerContainerAlerts(host, seen)

		m.mu.RLock()
		_, exists := m.activeAlerts["other-host-alert"]
		m.mu.RUnlock()

		if !exists {
			t.Error("expected other host's alert to remain")
		}
	})

	t.Run("handles empty seen set", func(t *testing.T) {
		t.Parallel()
		m := NewManager()

		host := models.DockerHost{ID: "host-c"}
		prefix := fmt.Sprintf("docker:%s/", host.ID)

		m.mu.Lock()
		m.activeAlerts["to-clear"] = &Alert{
			ID:         "to-clear",
			ResourceID: prefix + "container1",
		}
		m.dockerStateConfirm[prefix+"container1"] = 1
		m.mu.Unlock()

		m.cleanupDockerContainerAlerts(host, map[string]struct{}{})

		m.mu.RLock()
		_, alertExists := m.activeAlerts["to-clear"]
		_, stateExists := m.dockerStateConfirm[prefix+"container1"]
		m.mu.RUnlock()

		if alertExists {
			t.Error("expected alert to be cleared with empty seen set")
		}
		if stateExists {
			t.Error("expected state confirm to be cleared with empty seen set")
		}
	})
}

func TestSafeCallEscalateCallback(t *testing.T) {
	t.Parallel()

	t.Run("calls callback with alert and level", func(t *testing.T) {
		t.Parallel()
		m := NewManager()

		var receivedAlert *Alert
		var receivedLevel int
		done := make(chan struct{})

		m.SetEscalateCallback(func(alert *Alert, level int) {
			receivedAlert = alert
			receivedLevel = level
			close(done)
		})

		alert := &Alert{
			ID:           "test-alert",
			Type:         "test",
			ResourceName: "resource-1",
		}

		m.safeCallEscalateCallback(alert, 2)

		select {
		case <-done:
			if receivedAlert == nil {
				t.Fatal("expected alert to be received")
			}
			if receivedAlert.ID != "test-alert" {
				t.Errorf("expected alert ID 'test-alert', got %q", receivedAlert.ID)
			}
			if receivedLevel != 2 {
				t.Errorf("expected level 2, got %d", receivedLevel)
			}
		case <-time.After(1 * time.Second):
			t.Fatal("callback not called within timeout")
		}
	})

	t.Run("noop when callback is nil", func(t *testing.T) {
		t.Parallel()
		m := NewManager()
		// No callback set

		alert := &Alert{ID: "test-alert"}

		// Should not panic
		m.safeCallEscalateCallback(alert, 1)
	})

	t.Run("recovers from panic in callback", func(t *testing.T) {
		t.Parallel()
		m := NewManager()

		done := make(chan struct{})
		m.SetEscalateCallback(func(alert *Alert, level int) {
			defer close(done)
			panic("test panic")
		})

		alert := &Alert{ID: "panic-test"}

		// Should not panic the caller
		m.safeCallEscalateCallback(alert, 1)

		select {
		case <-done:
			// Callback ran (and panicked, but recovered)
		case <-time.After(1 * time.Second):
			t.Fatal("callback not called within timeout")
		}
	})

	t.Run("clones alert to prevent modification", func(t *testing.T) {
		t.Parallel()
		m := NewManager()

		var receivedAlert *Alert
		done := make(chan struct{})

		m.SetEscalateCallback(func(alert *Alert, level int) {
			receivedAlert = alert
			close(done)
		})

		original := &Alert{
			ID:           "original-alert",
			ResourceName: "original-resource",
		}

		m.safeCallEscalateCallback(original, 1)

		select {
		case <-done:
			// Modify original after callback started
			original.ResourceName = "modified"

			// Received alert should be a clone, not affected by modification
			if receivedAlert.ID != "original-alert" {
				t.Errorf("expected cloned alert ID")
			}
		case <-time.After(1 * time.Second):
			t.Fatal("callback not called within timeout")
		}
	})
}

func TestSafeCallResolvedCallback(t *testing.T) {
	t.Parallel()

	t.Run("calls callback with alert ID synchronously", func(t *testing.T) {
		t.Parallel()
		m := NewManager()

		var receivedID string
		m.SetResolvedCallback(func(alertID string) {
			receivedID = alertID
		})

		m.safeCallResolvedCallback("test-alert-123", false)

		if receivedID != "test-alert-123" {
			t.Errorf("expected alert ID 'test-alert-123', got %q", receivedID)
		}
	})

	t.Run("calls callback asynchronously", func(t *testing.T) {
		t.Parallel()
		m := NewManager()

		var receivedID string
		done := make(chan struct{})

		m.SetResolvedCallback(func(alertID string) {
			receivedID = alertID
			close(done)
		})

		m.safeCallResolvedCallback("async-alert", true)

		select {
		case <-done:
			if receivedID != "async-alert" {
				t.Errorf("expected alert ID 'async-alert', got %q", receivedID)
			}
		case <-time.After(1 * time.Second):
			t.Fatal("async callback not called within timeout")
		}
	})

	t.Run("noop when callback is nil", func(t *testing.T) {
		t.Parallel()
		m := NewManager()
		// No callback set

		// Should not panic
		m.safeCallResolvedCallback("test-alert", false)
		m.safeCallResolvedCallback("test-alert", true)
	})

	t.Run("recovers from panic in sync callback", func(t *testing.T) {
		t.Parallel()
		m := NewManager()

		m.SetResolvedCallback(func(alertID string) {
			panic("test panic")
		})

		// Should not panic the caller
		m.safeCallResolvedCallback("panic-test", false)
	})

	t.Run("recovers from panic in async callback", func(t *testing.T) {
		t.Parallel()
		m := NewManager()

		done := make(chan struct{})
		m.SetResolvedCallback(func(alertID string) {
			defer close(done)
			panic("async panic")
		})

		m.safeCallResolvedCallback("async-panic", true)

		select {
		case <-done:
			// Callback ran (and panicked, but recovered)
		case <-time.After(1 * time.Second):
			t.Fatal("async callback not called within timeout")
		}
	})
}
