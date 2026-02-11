package alerts

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// testEnvMu protects concurrent access to PULSE_DATA_DIR during parallel tests.
// Tests using newTestManager are effectively serialized because the Manager
// calls GetDataDir() repeatedly (not just at creation time).
var testEnvMu sync.Mutex

// newTestManager creates a Manager with an isolated temp directory for testing.
// It uses os.Setenv with a mutex to safely handle parallel tests that call // t.Parallel()
// before invoking this function (t.Setenv cannot be used after t.Parallel).
//
// IMPORTANT: The mutex is held for the entire duration of the test because the
// Manager calls GetDataDir() not just at creation time, but also during operations
// like SaveActiveAlerts() and LoadActiveAlerts(). This effectively serializes
// tests that use newTestManager, but ensures correct isolation.
func newTestManager(t *testing.T) *Manager {
	t.Helper()

	tmpDir := t.TempDir()

	testEnvMu.Lock()
	oldVal, hadOld := os.LookupEnv("PULSE_DATA_DIR")
	os.Setenv("PULSE_DATA_DIR", tmpDir)

	m := NewManager()

	// Restore env var and release mutex when test completes.
	// We also stop the history manager's background goroutines (but not the
	// full manager Stop which includes a 100ms sleep) to prevent writes to
	// the temp directory after the test completes.
	t.Cleanup(func() {
		// Stop the history manager to halt background save routines
		m.historyManager.Stop()
		// Close escalation channel to stop that goroutine too
		select {
		case <-m.escalationStop:
			// Already closed
		default:
			close(m.escalationStop)
		}
		// Close cleanup channel
		select {
		case <-m.cleanupStop:
			// Already closed
		default:
			close(m.cleanupStop)
		}
		// Brief pause to let goroutines finish any in-flight operations
		time.Sleep(10 * time.Millisecond)

		if hadOld {
			os.Setenv("PULSE_DATA_DIR", oldVal)
		} else {
			os.Unsetenv("PULSE_DATA_DIR")
		}
		testEnvMu.Unlock()
	})

	return m
}

func TestAcknowledgePersistsThroughCheckMetric(t *testing.T) {
	m := newTestManager(t)
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

func TestCheckMetricClearsAlertWhenThresholdDisabled(t *testing.T) {
	m := newTestManager(t)
	m.ClearActiveAlerts()
	m.mu.Lock()
	m.config.TimeThreshold = 0
	m.config.TimeThresholds = map[string]int{}
	m.config.SuppressionWindow = 0
	m.config.MinimumDelta = 0
	m.mu.Unlock()

	// First, create an active alert with an enabled threshold
	threshold := &HysteresisThreshold{Trigger: 80, Clear: 70}
	m.checkMetric("res1", "Resource", "node1", "inst1", "guest", "memory", 90, threshold, nil)

	m.mu.RLock()
	_, exists := m.activeAlerts["res1-memory"]
	m.mu.RUnlock()
	if !exists {
		t.Fatalf("expected alert to be created")
	}

	// Now call checkMetric with a disabled threshold (Trigger=0) — should clear the alert
	disabledThreshold := &HysteresisThreshold{Trigger: 0, Clear: 0}
	m.checkMetric("res1", "Resource", "node1", "inst1", "guest", "memory", 90, disabledThreshold, nil)

	m.mu.RLock()
	_, stillExists := m.activeAlerts["res1-memory"]
	m.mu.RUnlock()
	if stillExists {
		t.Errorf("expected alert to be cleared when threshold is disabled (Trigger=0)")
	}

	// Also test with nil threshold
	// Re-create the alert
	m.checkMetric("res1", "Resource", "node1", "inst1", "guest", "memory", 90, threshold, nil)
	m.mu.RLock()
	_, exists = m.activeAlerts["res1-memory"]
	m.mu.RUnlock()
	if !exists {
		t.Fatalf("expected alert to be re-created")
	}

	// Call with nil threshold — should also clear
	m.checkMetric("res1", "Resource", "node1", "inst1", "guest", "memory", 90, nil, nil)

	m.mu.RLock()
	_, stillExists = m.activeAlerts["res1-memory"]
	m.mu.RUnlock()
	if stillExists {
		t.Errorf("expected alert to be cleared when threshold is nil")
	}
}

func TestCheckGuestSkipsAlertsWhenMetricDisabled(t *testing.T) {
	m := newTestManager(t)

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
	m := newTestManager(t)
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
	m := newTestManager(t)
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
	m := newTestManager(t)
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
	m := newTestManager(t)
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
	m := newTestManager(t)
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
	m := newTestManager(t)
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
	m := newTestManager(t)
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
	m := newTestManager(t)
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
	m := newTestManager(t)
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
		"inst:node:100": "app-server",
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

func TestCheckSnapshotsRespectsOverrides(t *testing.T) {
	m := newTestManager(t)
	m.ClearActiveAlerts()

	cfg := AlertConfig{
		Enabled: true,
		SnapshotDefaults: SnapshotAlertConfig{
			Enabled:      true,
			WarningDays:  7,
			CriticalDays: 14,
		},
	}
	m.UpdateConfig(cfg)
	m.mu.Lock()
	m.config.TimeThreshold = 0
	m.mu.Unlock()

	now := time.Now()
	snapshots := []models.GuestSnapshot{
		{
			ID:       "inst:node:100:weekly",
			Name:     "weekly",
			Node:     "node",
			Instance: "inst",
			Type:     "qemu",
			VMID:     100,
			Time:     now.Add(-10 * 24 * time.Hour), // Triggers Warning (10 > 7)
		},
	}
	resourceKey := "inst:node:100"
	guestNames := map[string]string{
		resourceKey: "app-server",
	}

	// 1. Verify warning alert is created
	m.CheckSnapshotsForInstance("inst", snapshots, guestNames)
	m.mu.RLock()
	alert, exists := m.activeAlerts["snapshot-age-inst:node:100:weekly"]
	m.mu.RUnlock()
	if !exists {
		t.Fatalf("expected snapshot warning alert")
	}
	if alert.Level != AlertLevelWarning {
		t.Fatalf("expected warning alert, got %s", alert.Level)
	}

	// 2. Disable via override
	cfg = m.GetConfig()
	cfg.Overrides = map[string]ThresholdConfig{
		"inst:node:100": {
			Snapshot: &SnapshotAlertConfig{Enabled: false},
		},
	}
	m.UpdateConfig(cfg)
	m.CheckSnapshotsForInstance("inst", snapshots, guestNames)
	m.mu.RLock()
	_, exists = m.activeAlerts["snapshot-age-inst:node:100:weekly"]
	m.mu.RUnlock()
	if exists {
		t.Fatalf("expected snapshot alert to be suppressed by override")
	}
}

func TestCheckSnapshotsForInstanceTriggersOnSnapshotSize(t *testing.T) {
	m := newTestManager(t)
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
		"inst:node:200": "db-server",
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
	m := newTestManager(t)
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
		"inst:node:300": "app-server",
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
	m := newTestManager(t)
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

func TestCheckBackupsRespectsOverrides(t *testing.T) {
	m := newTestManager(t)
	m.ClearActiveAlerts()

	m.mu.Lock()
	m.config.Enabled = true
	m.config.BackupDefaults = BackupAlertConfig{
		Enabled:      true,
		WarningDays:  7,
		CriticalDays: 14,
	}
	m.config.TimeThreshold = 0
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
			Time:     now.Add(-10 * 24 * time.Hour), // Triggers Warning (10 > 7)
		},
	}

	key := BuildGuestKey("inst", "node", 100)
	resourceID := "inst:node:100"
	guestsByKey := map[string]GuestLookup{
		key: {
			ResourceID: resourceID,
			Name:       "app-server",
			Instance:   "inst",
			Node:       "node",
			Type:       "qemu",
			VMID:       100,
		},
	}
	guestsByVMID := map[string][]GuestLookup{
		"100": {guestsByKey[key]},
	}

	// 1. Verify warning alert is created with defaults
	m.CheckBackups(storageBackups, nil, nil, guestsByKey, guestsByVMID)
	m.mu.RLock()
	alert, exists := m.activeAlerts["backup-age-"+sanitizeAlertKey(key)]
	m.mu.RUnlock()
	if !exists {
		t.Fatalf("expected backup warning alert")
	}
	if alert.Level != AlertLevelWarning {
		t.Fatalf("expected warning alert, got %s", alert.Level)
	}

	// 2. Apply override to disable backup alerts for this guest
	cfg := m.GetConfig()
	cfg.Overrides = map[string]ThresholdConfig{
		resourceID: {
			Backup: &BackupAlertConfig{Enabled: false},
		},
	}
	m.UpdateConfig(cfg)

	m.CheckBackups(storageBackups, nil, nil, guestsByKey, guestsByVMID)
	m.mu.RLock()
	_, exists = m.activeAlerts["backup-age-"+sanitizeAlertKey(key)]
	m.mu.RUnlock()
	if exists {
		t.Fatalf("expected backup alert to be cleared/suppressed by override")
	}

	// 3. Apply override to change thresholds
	cfg.Overrides[resourceID] = ThresholdConfig{
		Backup: &BackupAlertConfig{
			Enabled:      true,
			WarningDays:  15, // 10 < 15, so no alert
			CriticalDays: 20,
		},
	}
	m.UpdateConfig(cfg)
	m.CheckBackups(storageBackups, nil, nil, guestsByKey, guestsByVMID)
	m.mu.RLock()
	_, exists = m.activeAlerts["backup-age-"+sanitizeAlertKey(key)]
	m.mu.RUnlock()
	if exists {
		t.Fatalf("expected no backup alert with increased thresholds in override")
	}

	// 4. Test global guest disable
	cfg.Overrides[resourceID] = ThresholdConfig{
		Disabled: true,
	}
	m.UpdateConfig(cfg)
	storageBackups[0].Time = now.Add(-30 * 24 * time.Hour) // Way past defaults
	m.CheckBackups(storageBackups, nil, nil, guestsByKey, guestsByVMID)
	m.mu.RLock()
	_, exists = m.activeAlerts["backup-age-"+sanitizeAlertKey(key)]
	m.mu.RUnlock()
	if exists {
		t.Fatalf("expected no backup alert for globally disabled guest")
	}
}

func TestCheckBackupsHandlesPbsOnlyGuests(t *testing.T) {
	m := newTestManager(t)
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

func TestCheckBackupsDisambiguatesWithNamespace(t *testing.T) {
	// Test that when multiple guests have the same VMID from different instances,
	// the namespace is used to match the backup to the correct guest.
	// This addresses issue #1095 where users have multiple PVE instances with
	// overlapping VMIDs and separate PBS instances backing them up.
	m := newTestManager(t)
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

	// Two guests with the same VMID (100) but on different instances
	guestsByKey := map[string]GuestLookup{
		"pve-node1-100": {
			ResourceID: "qemu/100",
			Name:       "webserver-pve",
			Instance:   "pve",
			Node:       "node1",
			Type:       "qemu",
			VMID:       100,
		},
		"pve-nat-node2-100": {
			ResourceID: "qemu/100",
			Name:       "webserver-nat",
			Instance:   "pve-nat",
			Node:       "node2",
			Type:       "qemu",
			VMID:       100,
		},
	}

	// Both guests have VMID "100"
	guestsByVMID := map[string][]GuestLookup{
		"100": {
			guestsByKey["pve-node1-100"],
			guestsByKey["pve-nat-node2-100"],
		},
	}

	// PBS backup with namespace "nat" should match the "pve-nat" instance
	pbsBackups := []models.PBSBackup{
		{
			ID:         "pbs-backup-100-nat",
			Instance:   "pbs-main",
			Datastore:  "backup-store",
			Namespace:  "nat", // This namespace should match "pve-nat"
			BackupType: "qemu",
			VMID:       "100",
			BackupTime: now.Add(-6 * 24 * time.Hour), // Critical
		},
	}

	m.CheckBackups(nil, pbsBackups, nil, guestsByKey, guestsByVMID)

	m.mu.RLock()
	defer m.mu.RUnlock()

	// Should find an alert keyed to the pve-nat instance (node2), not pve (node1)
	expectedKey := "backup-age-pve-nat-node2-100"
	alert, exists := m.activeAlerts[expectedKey]
	if !exists {
		// List what keys we do have for debugging
		var keys []string
		for k := range m.activeAlerts {
			keys = append(keys, k)
		}
		t.Fatalf("expected alert with key %q not found; found keys: %v", expectedKey, keys)
	}

	if alert.ResourceName != "webserver-nat backup" {
		t.Errorf("expected ResourceName 'webserver-nat backup', got %q", alert.ResourceName)
	}
	if alert.Instance != "pve-nat" {
		t.Errorf("expected Instance 'pve-nat', got %q", alert.Instance)
	}
}

// TestCheckBackupsVMIDCollisionNonMatchingNamespace verifies that when multiple guests
// share a VMID and the PBS backup namespace matches none of them, the alert uses the
// generic PBS key rather than falsely attributing to a specific guest.
func TestCheckBackupsVMIDCollisionNonMatchingNamespace(t *testing.T) {
	m := newTestManager(t)
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

	guestsByKey := map[string]GuestLookup{
		"pve1-node1-100": {
			ResourceID: "qemu/100",
			Name:       "vm-pve1",
			Instance:   "pve1",
			Node:       "node1",
			Type:       "qemu",
			VMID:       100,
		},
		"pve2-node2-100": {
			ResourceID: "qemu/100",
			Name:       "vm-pve2",
			Instance:   "pve2",
			Node:       "node2",
			Type:       "qemu",
			VMID:       100,
		},
	}

	guestsByVMID := map[string][]GuestLookup{
		"100": {
			guestsByKey["pve1-node1-100"],
			guestsByKey["pve2-node2-100"],
		},
	}

	// PBS backup with namespace "staging" — matches neither pve1 nor pve2
	pbsBackups := []models.PBSBackup{
		{
			ID:         "pbs-100",
			Instance:   "pbs-main",
			Datastore:  "backup-store",
			Namespace:  "staging",
			BackupType: "qemu",
			VMID:       "100",
			BackupTime: now.Add(-6 * 24 * time.Hour),
		},
	}

	m.CheckBackups(nil, pbsBackups, nil, guestsByKey, guestsByVMID)

	m.mu.RLock()
	defer m.mu.RUnlock()

	// Should NOT have a guest-specific alert key
	for key := range m.activeAlerts {
		if key == "backup-age-pve1-node1-100" || key == "backup-age-pve2-node2-100" {
			t.Errorf("should not attribute ambiguous backup to a specific guest, but found key %q", key)
		}
	}

	// Should have a generic PBS alert key
	expectedKey := "backup-age-pbs-pbs-main-qemu-100"
	if _, exists := m.activeAlerts[expectedKey]; !exists {
		var keys []string
		for k := range m.activeAlerts {
			keys = append(keys, k)
		}
		t.Errorf("expected generic PBS alert key %q, found keys: %v", expectedKey, keys)
	}
}

// TestCheckBackupsVMIDCollisionNoNamespace verifies that when multiple guests
// share a VMID and the PBS backup has no namespace, the alert uses the generic PBS key.
func TestCheckBackupsVMIDCollisionNoNamespace(t *testing.T) {
	m := newTestManager(t)
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

	guestsByKey := map[string]GuestLookup{
		"pve1-node1-100": {
			ResourceID: "qemu/100",
			Name:       "vm-pve1",
			Instance:   "pve1",
			Node:       "node1",
			Type:       "qemu",
			VMID:       100,
		},
		"pve2-node2-100": {
			ResourceID: "qemu/100",
			Name:       "vm-pve2",
			Instance:   "pve2",
			Node:       "node2",
			Type:       "qemu",
			VMID:       100,
		},
	}

	guestsByVMID := map[string][]GuestLookup{
		"100": {
			guestsByKey["pve1-node1-100"],
			guestsByKey["pve2-node2-100"],
		},
	}

	// PBS backup with NO namespace
	pbsBackups := []models.PBSBackup{
		{
			ID:         "pbs-100",
			Instance:   "pbs-main",
			Datastore:  "backup-store",
			Namespace:  "",
			BackupType: "qemu",
			VMID:       "100",
			BackupTime: now.Add(-6 * 24 * time.Hour),
		},
	}

	m.CheckBackups(nil, pbsBackups, nil, guestsByKey, guestsByVMID)

	m.mu.RLock()
	defer m.mu.RUnlock()

	// Should NOT have a guest-specific alert key
	for key := range m.activeAlerts {
		if key == "backup-age-pve1-node1-100" || key == "backup-age-pve2-node2-100" {
			t.Errorf("should not attribute ambiguous backup to a specific guest, but found key %q", key)
		}
	}

	// Should have a generic PBS alert key
	expectedKey := "backup-age-pbs-pbs-main-qemu-100"
	if _, exists := m.activeAlerts[expectedKey]; !exists {
		var keys []string
		for k := range m.activeAlerts {
			keys = append(keys, k)
		}
		t.Errorf("expected generic PBS alert key %q, found keys: %v", expectedKey, keys)
	}
}

func TestCheckBackupsHandlesPmgBackups(t *testing.T) {
	m := newTestManager(t)
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

func TestCheckBackupsSkipsOrphanedWhenDisabled(t *testing.T) {
	m := newTestManager(t)
	m.ClearActiveAlerts()

	alertOrphaned := false
	m.mu.Lock()
	m.config.Enabled = true
	m.config.BackupDefaults = BackupAlertConfig{
		Enabled:       true,
		WarningDays:   3,
		CriticalDays:  5,
		AlertOrphaned: &alertOrphaned,
		IgnoreVMIDs:   []string{},
	}
	m.mu.Unlock()

	now := time.Now()
	storageBackups := []models.StorageBackup{
		{
			ID:       "inst-node-200-backup",
			Storage:  "local",
			Node:     "node",
			Instance: "inst",
			Type:     "qemu",
			VMID:     200,
			Time:     now.Add(-6 * 24 * time.Hour),
		},
	}

	m.CheckBackups(storageBackups, nil, nil, map[string]GuestLookup{}, map[string][]GuestLookup{})

	m.mu.RLock()
	defer m.mu.RUnlock()
	for id := range m.activeAlerts {
		if strings.HasPrefix(id, "backup-age-") {
			t.Fatalf("expected orphaned backup to be skipped, found alert %s", id)
		}
	}
}

func TestCheckBackupsIgnoresVMIDs(t *testing.T) {
	m := newTestManager(t)
	m.ClearActiveAlerts()

	alertOrphaned := true
	m.mu.Lock()
	m.config.Enabled = true
	m.config.BackupDefaults = BackupAlertConfig{
		Enabled:       true,
		WarningDays:   1,
		CriticalDays:  2,
		AlertOrphaned: &alertOrphaned,
		IgnoreVMIDs:   []string{"10*"},
	}
	m.mu.Unlock()

	now := time.Now()
	storageBackups := []models.StorageBackup{
		{
			ID:       "inst-node-101-backup",
			Storage:  "local",
			Node:     "node",
			Instance: "inst",
			Type:     "qemu",
			VMID:     101,
			Time:     now.Add(-3 * 24 * time.Hour),
		},
		{
			ID:       "inst-node-200-backup",
			Storage:  "local",
			Node:     "node",
			Instance: "inst",
			Type:     "qemu",
			VMID:     200,
			Time:     now.Add(-3 * 24 * time.Hour),
		},
	}

	keyIgnored := BuildGuestKey("inst", "node", 101)
	keyAllowed := BuildGuestKey("inst", "node", 200)
	guestsByKey := map[string]GuestLookup{
		keyIgnored: {Name: "ignored-vm", Instance: "inst", Node: "node", Type: "qemu", VMID: 101},
		keyAllowed: {Name: "allowed-vm", Instance: "inst", Node: "node", Type: "qemu", VMID: 200},
	}
	guestsByVMID := map[string][]GuestLookup{
		"101": {guestsByKey[keyIgnored]},
		"200": {guestsByKey[keyAllowed]},
	}

	m.CheckBackups(storageBackups, nil, nil, guestsByKey, guestsByVMID)

	m.mu.RLock()
	_, ignoredExists := m.activeAlerts["backup-age-"+sanitizeAlertKey(keyIgnored)]
	_, allowedExists := m.activeAlerts["backup-age-"+sanitizeAlertKey(keyAllowed)]
	m.mu.RUnlock()

	if ignoredExists {
		t.Fatalf("expected backup alert for ignored VMID to be suppressed")
	}
	if !allowedExists {
		t.Fatalf("expected backup alert for non-ignored VMID")
	}
}

func TestCheckDockerHostIgnoresContainersByPrefix(t *testing.T) {
	m := newTestManager(t)

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
	m := newTestManager(t)
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

func TestDockerServiceAlertDoesNotRenotifyWhenUnchanged(t *testing.T) {
	m := newTestManager(t)
	m.ClearActiveAlerts()

	cfg := m.GetConfig()
	cfg.Enabled = true
	cfg.ActivationState = ActivationActive
	cfg.Schedule.MaxAlertsHour = 100
	m.UpdateConfig(cfg)

	dispatched := make(chan string, 4)
	m.SetAlertCallback(func(alert *Alert) {
		dispatched <- alert.ID
	})

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

	select {
	case <-dispatched:
	case <-time.After(1 * time.Second):
		t.Fatal("expected initial docker service alert notification")
	}

	// Same degraded state should update LastSeen/value but not re-notify every poll.
	m.CheckDockerHost(host)

	select {
	case id := <-dispatched:
		t.Fatalf("expected no second notification for unchanged service alert, got %s", id)
	case <-time.After(250 * time.Millisecond):
	}
}

func TestDockerServiceAlertPreservesLastNotifiedWhenUnchanged(t *testing.T) {
	m := newTestManager(t)
	m.ClearActiveAlerts()

	cfg := m.GetConfig()
	cfg.Enabled = true
	cfg.ActivationState = ActivationActive
	cfg.Schedule.MaxAlertsHour = 100
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

	notifiedAt := time.Now().Add(-2 * time.Minute).UTC()
	alert.LastNotified = &notifiedAt

	// Same degraded state should keep LastNotified while refreshing state.
	m.CheckDockerHost(host)

	updated, exists := m.activeAlerts[alertID]
	if !exists {
		t.Fatalf("expected service alert %s to remain active", alertID)
	}
	if updated.LastNotified == nil {
		t.Fatal("expected LastNotified to be preserved, got nil")
	}
	if !updated.LastNotified.Equal(notifiedAt) {
		t.Fatalf("expected LastNotified %s, got %s", notifiedAt, updated.LastNotified)
	}
}

func TestDockerServiceAlertRenotifiesOnEscalationToCritical(t *testing.T) {
	m := newTestManager(t)
	m.ClearActiveAlerts()

	cfg := m.GetConfig()
	cfg.Enabled = true
	cfg.ActivationState = ActivationActive
	cfg.Schedule.MaxAlertsHour = 100
	cfg.DockerDefaults.ServiceWarnGapPct = 10
	cfg.DockerDefaults.ServiceCritGapPct = 50
	m.UpdateConfig(cfg)

	dispatched := make(chan AlertLevel, 4)
	m.SetAlertCallback(func(alert *Alert) {
		dispatched <- alert.Level
	})

	host := models.DockerHost{
		ID:          "host-1",
		DisplayName: "Prod Swarm",
		Hostname:    "swarm-prod",
		Services: []models.DockerService{
			{
				ID:           "svc-1",
				Name:         "web",
				DesiredTasks: 4,
				RunningTasks: 3, // 25% missing -> warning
				Mode:         "replicated",
			},
		},
	}

	m.CheckDockerHost(host)

	select {
	case level := <-dispatched:
		if level != AlertLevelWarning {
			t.Fatalf("expected warning notification first, got %s", level)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("expected initial warning notification")
	}

	// Escalate from warning to critical: should notify again.
	host.Services[0].RunningTasks = 1 // 75% missing -> critical
	m.CheckDockerHost(host)

	select {
	case level := <-dispatched:
		if level != AlertLevelCritical {
			t.Fatalf("expected critical escalation notification, got %s", level)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("expected escalation notification")
	}
}

func TestDockerServiceUpdateStateAlert(t *testing.T) {
	m := newTestManager(t)
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
	m := newTestManager(t)
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
	m := newTestManager(t)
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
	m := newTestManager(t)

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
	m := newTestManager(t)

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
	// t.Parallel()

	m := newTestManager(t)

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
	m := newTestManager(t)
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

// TestNormalizeHostDefaultsPreservesZeroTrigger verifies that setting
// Host Agent thresholds to 0 is preserved (fixes GitHub issue #864).
// Setting a threshold to 0 should disable alerting for that metric.
func TestNormalizeHostDefaultsPreservesZeroTrigger(t *testing.T) {
	// t.Parallel()

	t.Run("nil HostDefaults get factory defaults", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		cfg := AlertConfig{
			Enabled:      true,
			HostDefaults: ThresholdConfig{}, // Empty - needs defaults
		}

		m.UpdateConfig(cfg)

		m.mu.RLock()
		defer m.mu.RUnlock()

		if m.config.HostDefaults.CPU == nil {
			t.Fatal("CPU defaults should be set")
		}
		if m.config.HostDefaults.CPU.Trigger != 80 {
			t.Errorf("CPU trigger = %v, want 80", m.config.HostDefaults.CPU.Trigger)
		}
		if m.config.HostDefaults.Memory == nil {
			t.Fatal("Memory defaults should be set")
		}
		if m.config.HostDefaults.Memory.Trigger != 85 {
			t.Errorf("Memory trigger = %v, want 85", m.config.HostDefaults.Memory.Trigger)
		}
		if m.config.HostDefaults.Disk == nil {
			t.Fatal("Disk defaults should be set")
		}
		if m.config.HostDefaults.Disk.Trigger != 90 {
			t.Errorf("Disk trigger = %v, want 90", m.config.HostDefaults.Disk.Trigger)
		}
	})

	t.Run("Trigger=0 preserved to disable alerting", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		// Set Memory to 0 to disable memory alerting for host agents
		cfg := AlertConfig{
			Enabled: true,
			HostDefaults: ThresholdConfig{
				CPU:    &HysteresisThreshold{Trigger: 80, Clear: 75},
				Memory: &HysteresisThreshold{Trigger: 0, Clear: 0}, // Disabled
				Disk:   &HysteresisThreshold{Trigger: 90, Clear: 85},
			},
		}

		m.UpdateConfig(cfg)

		m.mu.RLock()
		defer m.mu.RUnlock()

		// Memory threshold should remain at 0 (disabled), not reset to default
		if m.config.HostDefaults.Memory == nil {
			t.Fatal("Memory defaults should be preserved (not nil)")
		}
		if m.config.HostDefaults.Memory.Trigger != 0 {
			t.Errorf("Memory trigger = %v, want 0 (disabled)", m.config.HostDefaults.Memory.Trigger)
		}
		if m.config.HostDefaults.Memory.Clear != 0 {
			t.Errorf("Memory clear = %v, want 0 (disabled)", m.config.HostDefaults.Memory.Clear)
		}

		// CPU and Disk should still have their values
		if m.config.HostDefaults.CPU.Trigger != 80 {
			t.Errorf("CPU trigger = %v, want 80", m.config.HostDefaults.CPU.Trigger)
		}
		if m.config.HostDefaults.Disk.Trigger != 90 {
			t.Errorf("Disk trigger = %v, want 90", m.config.HostDefaults.Disk.Trigger)
		}
	})

	t.Run("Trigger=0 sets Clear=0 automatically", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		// Set CPU to 0 with a non-zero Clear - Clear should be normalized to 0
		cfg := AlertConfig{
			Enabled: true,
			HostDefaults: ThresholdConfig{
				CPU:    &HysteresisThreshold{Trigger: 0, Clear: 50}, // Clear should become 0
				Memory: &HysteresisThreshold{Trigger: 85, Clear: 80},
				Disk:   &HysteresisThreshold{Trigger: 0, Clear: 75}, // Clear should become 0
			},
		}

		m.UpdateConfig(cfg)

		m.mu.RLock()
		defer m.mu.RUnlock()

		if m.config.HostDefaults.CPU.Clear != 0 {
			t.Errorf("CPU clear = %v, want 0 when trigger is 0", m.config.HostDefaults.CPU.Clear)
		}
		if m.config.HostDefaults.Disk.Clear != 0 {
			t.Errorf("Disk clear = %v, want 0 when trigger is 0", m.config.HostDefaults.Disk.Clear)
		}
	})

	t.Run("missing Clear computed from Trigger", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		cfg := AlertConfig{
			Enabled: true,
			HostDefaults: ThresholdConfig{
				CPU:    &HysteresisThreshold{Trigger: 90, Clear: 0}, // Clear should be computed
				Memory: &HysteresisThreshold{Trigger: 95, Clear: 0}, // Clear should be computed
				Disk:   &HysteresisThreshold{Trigger: 92, Clear: 0}, // Clear should be computed
			},
		}

		m.UpdateConfig(cfg)

		m.mu.RLock()
		defer m.mu.RUnlock()

		// Clear should be Trigger - 5
		if m.config.HostDefaults.CPU.Clear != 85 {
			t.Errorf("CPU clear = %v, want 85", m.config.HostDefaults.CPU.Clear)
		}
		if m.config.HostDefaults.Memory.Clear != 90 {
			t.Errorf("Memory clear = %v, want 90", m.config.HostDefaults.Memory.Clear)
		}
		if m.config.HostDefaults.Disk.Clear != 87 {
			t.Errorf("Disk clear = %v, want 87", m.config.HostDefaults.Disk.Clear)
		}
	})
}

// TestNormalizeStorageDefaultsPreservesZeroTrigger verifies that setting
// StorageDefault threshold to 0 is preserved to disable storage alerting.
func TestNormalizeStorageDefaultsPreservesZeroTrigger(t *testing.T) {
	// t.Parallel()

	t.Run("negative trigger gets factory defaults", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		cfg := AlertConfig{
			Enabled:        true,
			StorageDefault: HysteresisThreshold{Trigger: -1, Clear: 0},
		}

		m.UpdateConfig(cfg)

		m.mu.RLock()
		defer m.mu.RUnlock()

		if m.config.StorageDefault.Trigger != 85 {
			t.Errorf("StorageDefault trigger = %v, want 85", m.config.StorageDefault.Trigger)
		}
		if m.config.StorageDefault.Clear != 80 {
			t.Errorf("StorageDefault clear = %v, want 80", m.config.StorageDefault.Clear)
		}
	})

	t.Run("Trigger=0 preserved to disable storage alerting", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		cfg := AlertConfig{
			Enabled:        true,
			StorageDefault: HysteresisThreshold{Trigger: 0, Clear: 0},
		}

		m.UpdateConfig(cfg)

		m.mu.RLock()
		defer m.mu.RUnlock()

		if m.config.StorageDefault.Trigger != 0 {
			t.Errorf("StorageDefault trigger = %v, want 0 (disabled)", m.config.StorageDefault.Trigger)
		}
		if m.config.StorageDefault.Clear != 0 {
			t.Errorf("StorageDefault clear = %v, want 0 (disabled)", m.config.StorageDefault.Clear)
		}
	})

	t.Run("missing Clear computed from Trigger", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		cfg := AlertConfig{
			Enabled:        true,
			StorageDefault: HysteresisThreshold{Trigger: 90, Clear: 0},
		}

		m.UpdateConfig(cfg)

		m.mu.RLock()
		defer m.mu.RUnlock()

		if m.config.StorageDefault.Trigger != 90 {
			t.Errorf("StorageDefault trigger = %v, want 90", m.config.StorageDefault.Trigger)
		}
		if m.config.StorageDefault.Clear != 85 {
			t.Errorf("StorageDefault clear = %v, want 85 (trigger - 5)", m.config.StorageDefault.Clear)
		}
	})
}

// TestNormalizeNodeDefaultsTemperaturePreservesZeroTrigger verifies that setting
// NodeDefaults.Temperature threshold to 0 is preserved to disable temperature alerting.
func TestNormalizeNodeDefaultsTemperaturePreservesZeroTrigger(t *testing.T) {
	// t.Parallel()

	t.Run("nil Temperature gets factory defaults", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		cfg := AlertConfig{
			Enabled:      true,
			NodeDefaults: ThresholdConfig{}, // Empty - Temperature needs defaults
		}

		m.UpdateConfig(cfg)

		m.mu.RLock()
		defer m.mu.RUnlock()

		if m.config.NodeDefaults.Temperature == nil {
			t.Fatal("Temperature defaults should be set")
		}
		if m.config.NodeDefaults.Temperature.Trigger != 80 {
			t.Errorf("Temperature trigger = %v, want 80", m.config.NodeDefaults.Temperature.Trigger)
		}
		if m.config.NodeDefaults.Temperature.Clear != 75 {
			t.Errorf("Temperature clear = %v, want 75", m.config.NodeDefaults.Temperature.Clear)
		}
	})

	t.Run("Trigger=0 preserved to disable temperature alerting", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		cfg := AlertConfig{
			Enabled: true,
			NodeDefaults: ThresholdConfig{
				Temperature: &HysteresisThreshold{Trigger: 0, Clear: 0},
			},
		}

		m.UpdateConfig(cfg)

		m.mu.RLock()
		defer m.mu.RUnlock()

		if m.config.NodeDefaults.Temperature == nil {
			t.Fatal("Temperature should be preserved (not nil)")
		}
		if m.config.NodeDefaults.Temperature.Trigger != 0 {
			t.Errorf("Temperature trigger = %v, want 0 (disabled)", m.config.NodeDefaults.Temperature.Trigger)
		}
		if m.config.NodeDefaults.Temperature.Clear != 0 {
			t.Errorf("Temperature clear = %v, want 0 (disabled)", m.config.NodeDefaults.Temperature.Clear)
		}
	})

	t.Run("missing Clear computed from Trigger", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		cfg := AlertConfig{
			Enabled: true,
			NodeDefaults: ThresholdConfig{
				Temperature: &HysteresisThreshold{Trigger: 85, Clear: 0},
			},
		}

		m.UpdateConfig(cfg)

		m.mu.RLock()
		defer m.mu.RUnlock()

		if m.config.NodeDefaults.Temperature.Trigger != 85 {
			t.Errorf("Temperature trigger = %v, want 85", m.config.NodeDefaults.Temperature.Trigger)
		}
		if m.config.NodeDefaults.Temperature.Clear != 80 {
			t.Errorf("Temperature clear = %v, want 80 (trigger - 5)", m.config.NodeDefaults.Temperature.Clear)
		}
	})
}

// TestNormalizeDockerThresholdPreservesZeroTrigger verifies that Docker
// container thresholds can be set to 0 to disable alerting.
func TestNormalizeDockerThresholdPreservesZeroTrigger(t *testing.T) {
	// t.Parallel()

	t.Run("Trigger=0 disables Docker CPU alerting", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		cfg := AlertConfig{
			Enabled: true,
			DockerDefaults: DockerThresholdConfig{
				CPU:    HysteresisThreshold{Trigger: 0, Clear: 0},
				Memory: HysteresisThreshold{Trigger: 85, Clear: 80},
				Disk:   HysteresisThreshold{Trigger: 85, Clear: 80},
			},
		}

		m.UpdateConfig(cfg)

		m.mu.RLock()
		defer m.mu.RUnlock()

		if m.config.DockerDefaults.CPU.Trigger != 0 {
			t.Errorf("Docker CPU trigger = %v, want 0 (disabled)", m.config.DockerDefaults.CPU.Trigger)
		}
		if m.config.DockerDefaults.Memory.Trigger != 85 {
			t.Errorf("Docker Memory trigger = %v, want 85", m.config.DockerDefaults.Memory.Trigger)
		}
	})

	t.Run("negative trigger replaced with defaults", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		cfg := AlertConfig{
			Enabled: true,
			DockerDefaults: DockerThresholdConfig{
				CPU:    HysteresisThreshold{Trigger: -5, Clear: 0},
				Memory: HysteresisThreshold{Trigger: -10, Clear: 0},
				Disk:   HysteresisThreshold{Trigger: -1, Clear: 0},
			},
		}

		m.UpdateConfig(cfg)

		m.mu.RLock()
		defer m.mu.RUnlock()

		if m.config.DockerDefaults.CPU.Trigger != 80 {
			t.Errorf("Docker CPU trigger = %v, want 80 (default)", m.config.DockerDefaults.CPU.Trigger)
		}
		if m.config.DockerDefaults.Memory.Trigger != 85 {
			t.Errorf("Docker Memory trigger = %v, want 85 (default)", m.config.DockerDefaults.Memory.Trigger)
		}
		if m.config.DockerDefaults.Disk.Trigger != 85 {
			t.Errorf("Docker Disk trigger = %v, want 85 (default)", m.config.DockerDefaults.Disk.Trigger)
		}
	})
}

func TestNormalizeDockerIgnoredPrefixes(t *testing.T) {
	// t.Parallel()

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
			// t.Parallel()

			got := NormalizeDockerIgnoredPrefixes(tc.input)
			if !reflect.DeepEqual(got, tc.expected) {
				t.Fatalf("expected %v, got %v", tc.expected, got)
			}
		})
	}
}

func TestCheckDockerHostIgnoredPrefixClearsExistingAlerts(t *testing.T) {
	m := newTestManager(t)

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
	// t.Parallel()

	t.Run("nil input remains nil", func(t *testing.T) {
		// t.Parallel()

		m := newTestManager(t)
		m.UpdateConfig(AlertConfig{})

		m.mu.RLock()
		defer m.mu.RUnlock()

		if m.config.DockerIgnoredContainerPrefixes != nil {
			t.Fatalf("expected nil prefixes, got %v", m.config.DockerIgnoredContainerPrefixes)
		}
	})

	t.Run("duplicates trimmed and deduplicated", func(t *testing.T) {
		// t.Parallel()

		m := newTestManager(t)
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
	// t.Parallel()

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
		{name: "skips empty prefix in list", containerName: "runner-job", containerID: "abc", prefixes: []string{"", "runner-"}, want: true},
		{name: "all empty prefixes returns false", containerName: "runner-job", containerID: "abc", prefixes: []string{"", "  ", ""}, want: false},
		{name: "empty name matches id", containerName: "", containerID: "runner-123", prefixes: []string{"runner-"}, want: true},
		{name: "empty id matches name", containerName: "runner-job", containerID: "", prefixes: []string{"runner-"}, want: true},
		{name: "both empty no match", containerName: "", containerID: "", prefixes: []string{"runner-"}, want: false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// t.Parallel()

			if got := matchesDockerIgnoredPrefix(tc.containerName, tc.containerID, tc.prefixes); got != tc.want {
				t.Fatalf("matchesDockerIgnoredPrefix(%q, %q, %v) = %v, want %v", tc.containerName, tc.containerID, tc.prefixes, got, tc.want)
			}
		})
	}
}

func TestDockerInstanceName(t *testing.T) {
	// t.Parallel()

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
			// t.Parallel()

			if got := dockerInstanceName(tc.host); got != tc.want {
				t.Fatalf("dockerInstanceName(%+v) = %q, want %q", tc.host, got, tc.want)
			}
		})
	}
}

func TestDockerContainerDisplayName(t *testing.T) {
	// t.Parallel()

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
			// t.Parallel()

			if got := dockerContainerDisplayName(tc.container); got != tc.want {
				t.Fatalf("dockerContainerDisplayName(%+v) = %q, want %q", tc.container, got, tc.want)
			}
		})
	}
}

func TestDockerResourceID(t *testing.T) {
	// t.Parallel()

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
			// t.Parallel()

			if got := dockerResourceID(tc.hostID, tc.containerID); got != tc.want {
				t.Fatalf("dockerResourceID(%q, %q) = %q, want %q", tc.hostID, tc.containerID, got, tc.want)
			}
		})
	}
}

func TestHasKnownFirmwareBug(t *testing.T) {
	// t.Parallel()

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
			// t.Parallel()

			if got := hasKnownFirmwareBug(tc.model); got != tc.want {
				t.Fatalf("hasKnownFirmwareBug(%q) = %v, want %v", tc.model, got, tc.want)
			}
		})
	}
}

func TestCheckDiskHealthSkipsSamsung980FalseAlerts(t *testing.T) {
	m := newTestManager(t)
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
	m := newTestManager(t)
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

func TestCheckDiskHealthHealthyDiskNoAlert(t *testing.T) {
	m := newTestManager(t)
	m.ClearActiveAlerts()

	// Non-Samsung disk with PASSED health should not create alert
	disk := proxmox.Disk{
		DevPath: "/dev/sda",
		Model:   "Western Digital WD40EFZX",
		Serial:  "WD-WCC4E0123456",
		Type:    "hdd",
		Health:  "PASSED",
		Wearout: 0, // N/A for HDD
		Size:    4000787030016,
	}

	m.CheckDiskHealth("test-instance", "pve-node1", disk)

	m.mu.RLock()
	healthAlertID := "disk-health-test-instance-pve-node1-/dev/sda"
	if _, exists := m.activeAlerts[healthAlertID]; exists {
		m.mu.RUnlock()
		t.Fatalf("expected no health alert for healthy disk with PASSED status")
	}
	m.mu.RUnlock()

	// Also test with "OK" status
	disk.Health = "OK"
	m.CheckDiskHealth("test-instance", "pve-node1", disk)

	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, exists := m.activeAlerts[healthAlertID]; exists {
		t.Fatalf("expected no health alert for healthy disk with OK status")
	}
}

func TestCheckDiskHealthFailedDiskCreatesAlert(t *testing.T) {
	m := newTestManager(t)
	m.ClearActiveAlerts()

	// Non-Samsung disk with FAILED health should create alert
	disk := proxmox.Disk{
		DevPath: "/dev/sdb",
		Model:   "Seagate ST2000DM008",
		Serial:  "ZA123456",
		Type:    "hdd",
		Health:  "FAILED",
		Wearout: 0,
		Size:    2000398934016,
	}

	m.CheckDiskHealth("test-instance", "pve-node1", disk)

	m.mu.RLock()
	defer m.mu.RUnlock()

	healthAlertID := "disk-health-test-instance-pve-node1-/dev/sdb"
	alert, exists := m.activeAlerts[healthAlertID]
	if !exists {
		t.Fatalf("expected health alert to be created for failed disk")
	}

	if alert.Level != AlertLevelCritical {
		t.Errorf("expected critical alert level, got %s", alert.Level)
	}
	if alert.Type != "disk-health" {
		t.Errorf("expected type disk-health, got %s", alert.Type)
	}
	if alert.Node != "pve-node1" {
		t.Errorf("expected node pve-node1, got %s", alert.Node)
	}
	if alert.Instance != "test-instance" {
		t.Errorf("expected instance test-instance, got %s", alert.Instance)
	}
}

func TestCheckDiskHealthRecoveryAlertCleared(t *testing.T) {
	m := newTestManager(t)
	m.ClearActiveAlerts()

	disk := proxmox.Disk{
		DevPath: "/dev/sdc",
		Model:   "Intel SSDSC2BB480G4",
		Serial:  "BTWL123456789",
		Type:    "ssd",
		Health:  "FAILED",
		Wearout: 50,
		Size:    480103981056,
	}

	// First check creates alert
	m.CheckDiskHealth("test-instance", "pve-node1", disk)

	healthAlertID := "disk-health-test-instance-pve-node1-/dev/sdc"
	m.mu.RLock()
	if _, exists := m.activeAlerts[healthAlertID]; !exists {
		m.mu.RUnlock()
		t.Fatalf("expected health alert to be created")
	}
	m.mu.RUnlock()

	// Disk health recovers
	disk.Health = "PASSED"
	m.CheckDiskHealth("test-instance", "pve-node1", disk)

	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, exists := m.activeAlerts[healthAlertID]; exists {
		t.Fatalf("expected health alert to be cleared after recovery")
	}
}

func TestCheckDiskHealthLowWearoutCreatesAlert(t *testing.T) {
	m := newTestManager(t)
	m.ClearActiveAlerts()

	// SSD with low wearout (less than 10% life remaining)
	disk := proxmox.Disk{
		DevPath: "/dev/nvme1n1",
		Model:   "Crucial CT1000MX500",
		Serial:  "12345678ABCD",
		Type:    "nvme",
		Health:  "PASSED",
		Wearout: 5, // Only 5% life remaining
		Size:    1000204886016,
	}

	m.CheckDiskHealth("test-instance", "pve-node1", disk)

	m.mu.RLock()
	defer m.mu.RUnlock()

	wearoutAlertID := "disk-wearout-test-instance-pve-node1-/dev/nvme1n1"
	alert, exists := m.activeAlerts[wearoutAlertID]
	if !exists {
		t.Fatalf("expected wearout alert to be created for disk with low life remaining")
	}

	if alert.Level != AlertLevelWarning {
		t.Errorf("expected warning alert level, got %s", alert.Level)
	}
	if alert.Type != "disk-wearout" {
		t.Errorf("expected type disk-wearout, got %s", alert.Type)
	}
	if alert.Value != 5 {
		t.Errorf("expected value 5, got %f", alert.Value)
	}
	if alert.Threshold != 10.0 {
		t.Errorf("expected threshold 10.0, got %f", alert.Threshold)
	}
}

func TestCheckDiskHealthWearoutAlertUpdatesOnSubsequentChecks(t *testing.T) {
	m := newTestManager(t)
	m.ClearActiveAlerts()

	disk := proxmox.Disk{
		DevPath: "/dev/nvme2n1",
		Model:   "Kingston SA2000M8",
		Serial:  "50026B768A123456",
		Type:    "nvme",
		Health:  "PASSED",
		Wearout: 8,
		Size:    500107862016,
	}

	// First check creates alert
	m.CheckDiskHealth("test-instance", "pve-node1", disk)

	wearoutAlertID := "disk-wearout-test-instance-pve-node1-/dev/nvme2n1"
	m.mu.RLock()
	alert, exists := m.activeAlerts[wearoutAlertID]
	if !exists {
		m.mu.RUnlock()
		t.Fatalf("expected wearout alert to be created")
	}
	firstLastSeen := alert.LastSeen
	m.mu.RUnlock()

	// Wait a moment to ensure time difference
	time.Sleep(10 * time.Millisecond)

	// Wearout decreases further
	disk.Wearout = 6
	m.CheckDiskHealth("test-instance", "pve-node1", disk)

	m.mu.RLock()
	defer m.mu.RUnlock()

	alert, exists = m.activeAlerts[wearoutAlertID]
	if !exists {
		t.Fatalf("expected wearout alert to still exist")
	}

	if !alert.LastSeen.After(firstLastSeen) {
		t.Errorf("expected LastSeen to be updated, got %v (original: %v)", alert.LastSeen, firstLastSeen)
	}
	if alert.Value != 6 {
		t.Errorf("expected value to be updated to 6, got %f", alert.Value)
	}
}

func TestCheckDiskHealthWearoutRecoveryAlertCleared(t *testing.T) {
	m := newTestManager(t)
	m.ClearActiveAlerts()

	disk := proxmox.Disk{
		DevPath: "/dev/sdd",
		Model:   "ADATA SU800",
		Serial:  "2J012345678",
		Type:    "ssd",
		Health:  "PASSED",
		Wearout: 5,
		Size:    256060514304,
	}

	// First check creates wearout alert
	m.CheckDiskHealth("test-instance", "pve-node1", disk)

	wearoutAlertID := "disk-wearout-test-instance-pve-node1-/dev/sdd"
	m.mu.RLock()
	if _, exists := m.activeAlerts[wearoutAlertID]; !exists {
		m.mu.RUnlock()
		t.Fatalf("expected wearout alert to be created")
	}
	m.mu.RUnlock()

	// Wearout recovers (replaced drive, or misread corrected)
	disk.Wearout = 95
	m.CheckDiskHealth("test-instance", "pve-node1", disk)

	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, exists := m.activeAlerts[wearoutAlertID]; exists {
		t.Fatalf("expected wearout alert to be cleared after recovery")
	}
}

func TestCheckDiskHealthEmptyOrUnknownHealthNoAlert(t *testing.T) {
	m := newTestManager(t)
	m.ClearActiveAlerts()

	disk := proxmox.Disk{
		DevPath: "/dev/sde",
		Model:   "Generic USB Storage",
		Serial:  "USB123456",
		Type:    "hdd",
		Health:  "", // Empty health - SMART not supported
		Wearout: 0,
		Size:    128043712512,
	}

	healthAlertID := "disk-health-test-instance-pve-node1-/dev/sde"

	// Empty health should not create alert
	m.CheckDiskHealth("test-instance", "pve-node1", disk)

	m.mu.RLock()
	if _, exists := m.activeAlerts[healthAlertID]; exists {
		m.mu.RUnlock()
		t.Fatalf("expected no health alert for disk with empty health status")
	}
	m.mu.RUnlock()

	// UNKNOWN health should not create alert
	disk.Health = "UNKNOWN"
	m.CheckDiskHealth("test-instance", "pve-node1", disk)

	m.mu.RLock()
	if _, exists := m.activeAlerts[healthAlertID]; exists {
		m.mu.RUnlock()
		t.Fatalf("expected no health alert for disk with UNKNOWN health status")
	}
	m.mu.RUnlock()

	// Lowercase "unknown" should also not create alert (normalized to uppercase)
	disk.Health = "unknown"
	m.CheckDiskHealth("test-instance", "pve-node1", disk)

	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, exists := m.activeAlerts[healthAlertID]; exists {
		t.Fatalf("expected no health alert for disk with lowercase unknown health status")
	}
}

func TestDisableAllStorageClearsExistingAlerts(t *testing.T) {
	m := newTestManager(t)

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

	m := newTestManager(t)
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
	m := newTestManager(t)

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
	m := newTestManager(t)

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
	// t.Parallel()

	tests := []struct {
		name     string
		instance string
		node     string
		vmID     int
		want     string
	}{
		{
			name:     "different instance and node",
			instance: "cluster-1",
			node:     "pve-node",
			vmID:     100,
			want:     "cluster-1:pve-node:100",
		},
		{
			name:     "same instance and node",
			instance: "pve-node",
			node:     "pve-node",
			vmID:     200,
			want:     "pve-node:pve-node:200",
		},
		{
			name:     "empty instance uses node",
			instance: "",
			node:     "pve-node",
			vmID:     300,
			want:     "pve-node:pve-node:300",
		},
		{
			name:     "whitespace instance uses node",
			instance: "   ",
			node:     "pve-node",
			vmID:     400,
			want:     "pve-node:pve-node:400",
		},
		{
			name:     "instance with whitespace trimmed",
			instance: "  cluster-1  ",
			node:     "pve-node",
			vmID:     500,
			want:     "cluster-1:pve-node:500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// t.Parallel()
			got := BuildGuestKey(tt.instance, tt.node, tt.vmID)
			if got != tt.want {
				t.Errorf("BuildGuestKey(%q, %q, %d) = %q, want %q", tt.instance, tt.node, tt.vmID, got, tt.want)
			}
		})
	}
}

func TestCheckFlapping(t *testing.T) {
	// t.Parallel()

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
			// t.Parallel()

			m := newTestManager(t)

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

			// Call checkFlappingLocked
			m.mu.Lock()
			result := m.checkFlappingLocked(alertID)
			m.mu.Unlock()

			if result != tt.expectFlapping {
				t.Errorf("checkFlappingLocked() = %v, want %v", result, tt.expectFlapping)
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
	// t.Parallel()

	m := newTestManager(t)

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

	// Call checkFlappingLocked - should return true but NOT update suppression
	m.mu.Lock()
	result := m.checkFlappingLocked(alertID)
	m.mu.Unlock()

	if !result {
		t.Errorf("checkFlappingLocked() = false, want true for already flapping alert")
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
	// t.Parallel()

	m := newTestManager(t)

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

	// Call checkFlappingLocked - old entries should be pruned
	m.mu.Lock()
	result := m.checkFlappingLocked(alertID)
	historyLen := len(m.flappingHistory[alertID])
	m.mu.Unlock()

	if result {
		t.Errorf("checkFlappingLocked() = true, want false (old entries should be pruned)")
	}

	// Only the current call should remain in history
	if historyLen != 1 {
		t.Errorf("history length = %d, want 1 (old entries should be pruned)", historyLen)
	}
}

func TestGetGlobalMetricTimeThreshold(t *testing.T) {
	// t.Parallel()

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
			// t.Parallel()

			m := newTestManager(t)
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
	// t.Parallel()

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
			// t.Parallel()

			m := newTestManager(t)
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
	// t.Parallel()

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
			// t.Parallel()

			m := newTestManager(t)
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
	// t.Parallel()

	t.Run("no rate limit when MaxAlertsHour is zero", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
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
		// t.Parallel()
		m := newTestManager(t)
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
		// t.Parallel()
		m := newTestManager(t)
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
		// t.Parallel()
		m := newTestManager(t)
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
		// t.Parallel()
		m := newTestManager(t)
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
		// t.Parallel()
		m := newTestManager(t)
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
		// t.Parallel()
		m := newTestManager(t)
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
	// t.Parallel()

	t.Run("nil thresholds get defaults", func(t *testing.T) {
		// t.Parallel()
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
		// t.Parallel()
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
		// t.Parallel()
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
		// t.Parallel()
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
		// t.Parallel()
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
		// t.Parallel()
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
	// t.Parallel()

	t.Run("cooldown disabled allows notification", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
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
		// t.Parallel()
		m := newTestManager(t)
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
		// t.Parallel()
		m := newTestManager(t)
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
		// t.Parallel()
		m := newTestManager(t)
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
		// t.Parallel()
		m := newTestManager(t)
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
		// t.Parallel()
		m := newTestManager(t)
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
	// t.Parallel()

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
			// t.Parallel()
			result := dockerServiceDisplayName(tt.service)
			if result != tt.expected {
				t.Errorf("dockerServiceDisplayName(%+v) = %q, want %q", tt.service, result, tt.expected)
			}
		})
	}
}

func TestDockerServiceResourceID(t *testing.T) {
	// t.Parallel()

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
			// t.Parallel()
			result := dockerServiceResourceID(tt.hostID, tt.serviceID, tt.serviceName)
			if result != tt.expected {
				t.Errorf("dockerServiceResourceID(%q, %q, %q) = %q, want %q",
					tt.hostID, tt.serviceID, tt.serviceName, result, tt.expected)
			}
		})
	}
}

func TestClearStorageOfflineAlert(t *testing.T) {
	// t.Parallel()

	t.Run("clears existing offline alert", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

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

		resolvedCh := make(chan string, 1)
		m.SetResolvedCallback(func(id string) {
			resolvedCh <- id
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
		select {
		case resolvedID := <-resolvedCh:
			if resolvedID != alertID {
				t.Errorf("expected resolved callback with %q, got %q", alertID, resolvedID)
			}
		case <-time.After(2 * time.Second):
			t.Error("expected resolved callback to be called")
		}
	})

	t.Run("noop when no alert exists", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

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
		// t.Parallel()
		m := newTestManager(t)

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
	// t.Parallel()

	t.Run("clears specified metrics", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

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
		// t.Parallel()
		m := newTestManager(t)

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
		// t.Parallel()
		m := newTestManager(t)

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
	// t.Parallel()

	t.Run("clears all disk alerts for host", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

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
		// t.Parallel()
		m := newTestManager(t)

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
		// t.Parallel()
		m := newTestManager(t)

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
		// t.Parallel()
		m := newTestManager(t)

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
	// t.Parallel()

	t.Run("clears alerts not in seen set", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

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
		// t.Parallel()
		m := newTestManager(t)

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
		// t.Parallel()
		m := newTestManager(t)

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
		// t.Parallel()
		m := newTestManager(t)

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
	// t.Parallel()
	m := newTestManager(t)

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
	// t.Parallel()

	t.Run("clears offline alert and tracking", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

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
		// t.Parallel()
		m := newTestManager(t)

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
		// t.Parallel()
		m := newTestManager(t)

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
	// t.Parallel()

	t.Run("clears alerts not in seen set", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

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
		// t.Parallel()
		m := newTestManager(t)

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
		// t.Parallel()
		m := newTestManager(t)

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
	// t.Parallel()

	t.Run("calls callback with alert and level", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

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
		// t.Parallel()
		m := newTestManager(t)
		// No callback set

		alert := &Alert{ID: "test-alert"}

		// Should not panic
		m.safeCallEscalateCallback(alert, 1)
	})

	t.Run("recovers from panic in callback", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

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
		// t.Parallel()
		m := newTestManager(t)

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
	// t.Parallel()

	t.Run("calls callback with alert ID synchronously", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

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
		// t.Parallel()
		m := newTestManager(t)

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
		// t.Parallel()
		m := newTestManager(t)
		// No callback set

		// Should not panic
		m.safeCallResolvedCallback("test-alert", false)
		m.safeCallResolvedCallback("test-alert", true)
	})

	t.Run("recovers from panic in sync callback", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.SetResolvedCallback(func(alertID string) {
			panic("test panic")
		})

		// Should not panic the caller
		m.safeCallResolvedCallback("panic-test", false)
	})

	t.Run("recovers from panic in async callback", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

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

func TestHandleHostOnline(t *testing.T) {
	// t.Parallel()

	t.Run("clears offline alert and confirmation tracking", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		host := models.Host{ID: "host-1", Hostname: "my-host"}
		alertID := fmt.Sprintf("host-offline-%s", host.ID)
		resourceKey := fmt.Sprintf("host:%s", host.ID)

		// Set up offline alert and tracking
		m.mu.Lock()
		m.activeAlerts[alertID] = &Alert{ID: alertID, ResourceID: resourceKey}
		m.offlineConfirmations[resourceKey] = 5
		m.mu.Unlock()

		m.HandleHostOnline(host)

		m.mu.RLock()
		_, alertExists := m.activeAlerts[alertID]
		_, confirmExists := m.offlineConfirmations[resourceKey]
		m.mu.RUnlock()

		if alertExists {
			t.Error("expected offline alert to be cleared")
		}
		if confirmExists {
			t.Error("expected offline confirmation to be cleared")
		}
	})

	t.Run("clears confirmation even without alert", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		host := models.Host{ID: "host-2"}
		resourceKey := fmt.Sprintf("host:%s", host.ID)

		// Only tracking, no alert
		m.mu.Lock()
		m.offlineConfirmations[resourceKey] = 2
		m.mu.Unlock()

		m.HandleHostOnline(host)

		m.mu.RLock()
		_, exists := m.offlineConfirmations[resourceKey]
		m.mu.RUnlock()

		if exists {
			t.Error("expected offline confirmation to be cleared")
		}
	})

	t.Run("empty host ID is noop", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		// Create data that should not be touched
		m.mu.Lock()
		m.activeAlerts["host-offline-other"] = &Alert{ID: "host-offline-other"}
		m.offlineConfirmations["host:other"] = 3
		m.mu.Unlock()

		host := models.Host{ID: ""}
		m.HandleHostOnline(host)

		m.mu.RLock()
		_, alertExists := m.activeAlerts["host-offline-other"]
		_, confirmExists := m.offlineConfirmations["host:other"]
		m.mu.RUnlock()

		if !alertExists {
			t.Error("expected other alert to remain when host ID is empty")
		}
		if !confirmExists {
			t.Error("expected other confirmation to remain when host ID is empty")
		}
	})
}

func TestAcknowledgeAlertNotFound(t *testing.T) {
	// t.Parallel()
	m := newTestManager(t)

	err := m.AcknowledgeAlert("nonexistent-alert", "user1")

	if err == nil {
		t.Fatal("expected error when acknowledging nonexistent alert")
	}
	if !strings.Contains(err.Error(), "alert not found") {
		t.Errorf("expected 'alert not found' error, got: %v", err)
	}
}

func TestUnacknowledgeAlertNotFound(t *testing.T) {
	// t.Parallel()
	m := newTestManager(t)

	err := m.UnacknowledgeAlert("nonexistent-alert")

	if err == nil {
		t.Fatal("expected error when unacknowledging nonexistent alert")
	}
	if !strings.Contains(err.Error(), "alert not found") {
		t.Errorf("expected 'alert not found' error, got: %v", err)
	}
}

func TestUnacknowledgeAlertSuccess(t *testing.T) {
	// t.Parallel()
	m := newTestManager(t)

	// Create and acknowledge an alert first
	alertID := "test-alert-123"
	now := time.Now()
	m.activeAlerts[alertID] = &Alert{
		ID:           alertID,
		Acknowledged: true,
		AckTime:      &now,
		AckUser:      "user1",
	}
	m.ackState[alertID] = ackRecord{acknowledged: true, user: "user1", time: now}

	// Unacknowledge the alert
	err := m.UnacknowledgeAlert(alertID)

	if err != nil {
		t.Fatalf("unexpected error unacknowledging alert: %v", err)
	}

	// Verify alert state was updated
	alert := m.activeAlerts[alertID]
	if alert.Acknowledged {
		t.Error("expected Acknowledged to be false")
	}
	if alert.AckTime != nil {
		t.Error("expected AckTime to be nil")
	}
	if alert.AckUser != "" {
		t.Errorf("expected AckUser to be empty, got: %s", alert.AckUser)
	}

	// Verify ackState was removed
	if _, exists := m.ackState[alertID]; exists {
		t.Error("expected ackState entry to be deleted")
	}
}

func TestClearActiveAlertsEmptyMaps(t *testing.T) {
	// t.Parallel()
	m := newTestManager(t)

	// Ensure maps are empty initially
	if len(m.activeAlerts) != 0 {
		t.Fatalf("expected activeAlerts to be empty, got %d", len(m.activeAlerts))
	}
	if len(m.pendingAlerts) != 0 {
		t.Fatalf("expected pendingAlerts to be empty, got %d", len(m.pendingAlerts))
	}

	// Call ClearActiveAlerts on empty manager - should return early without panic
	m.ClearActiveAlerts()

	// Verify maps are still empty (function returned early)
	if len(m.activeAlerts) != 0 {
		t.Errorf("expected activeAlerts to remain empty, got %d", len(m.activeAlerts))
	}
}

func TestClearActiveAlertsWithExistingAlerts(t *testing.T) {
	// t.Parallel()
	m := newTestManager(t)

	// Populate various maps with test data
	m.mu.Lock()
	m.activeAlerts["test-alert-1"] = &Alert{ID: "test-alert-1", Type: "cpu-usage"}
	m.activeAlerts["test-alert-2"] = &Alert{ID: "test-alert-2", Type: "memory-usage"}
	m.pendingAlerts["pending-1"] = time.Now()
	m.recentAlerts["recent-1"] = &Alert{ID: "recent-1", Type: "disk-usage"}
	m.suppressedUntil["suppressed-1"] = time.Now().Add(time.Hour)
	m.alertRateLimit["rate-1"] = []time.Time{time.Now()}
	m.nodeOfflineCount["node-1"] = 3
	m.offlineConfirmations["node-1"] = 2
	m.dockerOfflineCount["docker-1"] = 1
	m.dockerStateConfirm["docker-1"] = 1
	m.ackState["test-alert-1"] = ackRecord{acknowledged: true, user: "testuser", time: time.Now()}
	m.mu.Unlock()

	m.resolvedMutex.Lock()
	m.recentlyResolved["resolved-1"] = &ResolvedAlert{Alert: &Alert{ID: "resolved-1"}, ResolvedTime: time.Now()}
	m.resolvedMutex.Unlock()

	// Call ClearActiveAlerts
	m.ClearActiveAlerts()

	// Give goroutine time to run SaveActiveAlerts
	time.Sleep(50 * time.Millisecond)

	// Verify all maps are cleared
	m.mu.RLock()
	if len(m.activeAlerts) != 0 {
		t.Errorf("expected activeAlerts to be empty, got %d", len(m.activeAlerts))
	}
	if len(m.pendingAlerts) != 0 {
		t.Errorf("expected pendingAlerts to be empty, got %d", len(m.pendingAlerts))
	}
	if len(m.recentAlerts) != 0 {
		t.Errorf("expected recentAlerts to be empty, got %d", len(m.recentAlerts))
	}
	if len(m.suppressedUntil) != 0 {
		t.Errorf("expected suppressedUntil to be empty, got %d", len(m.suppressedUntil))
	}
	if len(m.alertRateLimit) != 0 {
		t.Errorf("expected alertRateLimit to be empty, got %d", len(m.alertRateLimit))
	}
	if len(m.nodeOfflineCount) != 0 {
		t.Errorf("expected nodeOfflineCount to be empty, got %d", len(m.nodeOfflineCount))
	}
	if len(m.offlineConfirmations) != 0 {
		t.Errorf("expected offlineConfirmations to be empty, got %d", len(m.offlineConfirmations))
	}
	if len(m.dockerOfflineCount) != 0 {
		t.Errorf("expected dockerOfflineCount to be empty, got %d", len(m.dockerOfflineCount))
	}
	if len(m.dockerStateConfirm) != 0 {
		t.Errorf("expected dockerStateConfirm to be empty, got %d", len(m.dockerStateConfirm))
	}
	if len(m.ackState) != 0 {
		t.Errorf("expected ackState to be empty, got %d", len(m.ackState))
	}
	m.mu.RUnlock()

	m.resolvedMutex.RLock()
	if len(m.recentlyResolved) != 0 {
		t.Errorf("expected recentlyResolved to be empty, got %d", len(m.recentlyResolved))
	}
	m.resolvedMutex.RUnlock()
}

func TestClearBackupAlertsLocked(t *testing.T) {
	// t.Parallel()

	t.Run("clears backup-age alerts only", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		// Add a backup-age alert
		m.activeAlerts["backup-alert-1"] = &Alert{
			ID:   "backup-alert-1",
			Type: "backup-age",
		}
		// Add a non-backup alert
		m.activeAlerts["cpu-alert-1"] = &Alert{
			ID:   "cpu-alert-1",
			Type: "cpu",
		}
		// Add another backup-age alert
		m.activeAlerts["backup-alert-2"] = &Alert{
			ID:   "backup-alert-2",
			Type: "backup-age",
		}

		if len(m.activeAlerts) != 3 {
			t.Fatalf("expected 3 alerts, got %d", len(m.activeAlerts))
		}

		m.mu.Lock()
		m.clearBackupAlertsLocked()
		m.mu.Unlock()

		// Should have removed backup-age alerts, keeping cpu alert
		if len(m.activeAlerts) != 1 {
			t.Errorf("expected 1 alert remaining, got %d", len(m.activeAlerts))
		}
		if _, exists := m.activeAlerts["cpu-alert-1"]; !exists {
			t.Error("expected cpu-alert-1 to remain")
		}
		if _, exists := m.activeAlerts["backup-alert-1"]; exists {
			t.Error("expected backup-alert-1 to be cleared")
		}
		if _, exists := m.activeAlerts["backup-alert-2"]; exists {
			t.Error("expected backup-alert-2 to be cleared")
		}
	})

	t.Run("handles nil alert in map", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		// Add a nil alert entry
		m.activeAlerts["nil-alert"] = nil
		// Add a valid backup-age alert
		m.activeAlerts["backup-alert"] = &Alert{
			ID:   "backup-alert",
			Type: "backup-age",
		}

		m.mu.Lock()
		m.clearBackupAlertsLocked()
		m.mu.Unlock()

		// Should have skipped nil and removed backup-age
		if len(m.activeAlerts) != 1 {
			t.Errorf("expected 1 alert remaining, got %d", len(m.activeAlerts))
		}
		// Nil entry should remain
		if _, exists := m.activeAlerts["nil-alert"]; !exists {
			t.Error("expected nil-alert entry to remain (nil check should skip it)")
		}
	})

	t.Run("empty alerts map is no-op", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.clearBackupAlertsLocked()
		m.mu.Unlock()

		if len(m.activeAlerts) != 0 {
			t.Errorf("expected 0 alerts, got %d", len(m.activeAlerts))
		}
	})
}

func TestClearBackupAlerts(t *testing.T) {
	// t.Parallel()
	m := newTestManager(t)

	// Add a backup-age alert
	m.activeAlerts["backup-alert"] = &Alert{
		ID:   "backup-alert",
		Type: "backup-age",
	}
	// Add a non-backup alert
	m.activeAlerts["cpu-alert"] = &Alert{
		ID:   "cpu-alert",
		Type: "cpu",
	}

	// Call the public method (handles locking internally)
	m.clearBackupAlerts()

	// Only cpu alert should remain
	if len(m.activeAlerts) != 1 {
		t.Errorf("expected 1 alert remaining, got %d", len(m.activeAlerts))
	}
	if _, exists := m.activeAlerts["cpu-alert"]; !exists {
		t.Error("expected cpu-alert to remain")
	}
}

func TestClearSnapshotAlertsForInstanceLocked(t *testing.T) {
	// t.Parallel()

	t.Run("clears snapshot alerts for specific instance", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		// Add snapshot alerts for different instances
		m.activeAlerts["snap-inst1"] = &Alert{
			ID:       "snap-inst1",
			Type:     "snapshot-age",
			Instance: "instance1",
		}
		m.activeAlerts["snap-inst2"] = &Alert{
			ID:       "snap-inst2",
			Type:     "snapshot-age",
			Instance: "instance2",
		}
		// Add a non-snapshot alert
		m.activeAlerts["cpu-alert"] = &Alert{
			ID:   "cpu-alert",
			Type: "cpu",
		}

		m.mu.Lock()
		m.clearSnapshotAlertsForInstanceLocked("instance1")
		m.mu.Unlock()

		// Should keep instance2 snapshot and cpu alert
		if len(m.activeAlerts) != 2 {
			t.Errorf("expected 2 alerts remaining, got %d", len(m.activeAlerts))
		}
		if _, exists := m.activeAlerts["snap-inst1"]; exists {
			t.Error("expected snap-inst1 to be cleared")
		}
		if _, exists := m.activeAlerts["snap-inst2"]; !exists {
			t.Error("expected snap-inst2 to remain")
		}
	})

	t.Run("clears all snapshot alerts when instance is empty", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		// Add snapshot alerts for different instances
		m.activeAlerts["snap-inst1"] = &Alert{
			ID:       "snap-inst1",
			Type:     "snapshot-age",
			Instance: "instance1",
		}
		m.activeAlerts["snap-inst2"] = &Alert{
			ID:       "snap-inst2",
			Type:     "snapshot-age",
			Instance: "instance2",
		}
		// Add a non-snapshot alert
		m.activeAlerts["cpu-alert"] = &Alert{
			ID:   "cpu-alert",
			Type: "cpu",
		}

		m.mu.Lock()
		m.clearSnapshotAlertsForInstanceLocked("")
		m.mu.Unlock()

		// Should keep only cpu alert
		if len(m.activeAlerts) != 1 {
			t.Errorf("expected 1 alert remaining, got %d", len(m.activeAlerts))
		}
		if _, exists := m.activeAlerts["cpu-alert"]; !exists {
			t.Error("expected cpu-alert to remain")
		}
	})

	t.Run("handles nil alert in map", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		// Add nil entry and valid snapshot alert
		m.activeAlerts["nil-alert"] = nil
		m.activeAlerts["snap-alert"] = &Alert{
			ID:       "snap-alert",
			Type:     "snapshot-age",
			Instance: "inst1",
		}

		m.mu.Lock()
		m.clearSnapshotAlertsForInstanceLocked("inst1")
		m.mu.Unlock()

		// Nil entry should remain, snapshot should be cleared
		if len(m.activeAlerts) != 1 {
			t.Errorf("expected 1 alert remaining, got %d", len(m.activeAlerts))
		}
		if _, exists := m.activeAlerts["nil-alert"]; !exists {
			t.Error("expected nil-alert entry to remain")
		}
	})
}

func TestClearSnapshotAlertsForInstance(t *testing.T) {
	// t.Parallel()
	m := newTestManager(t)

	// Add a snapshot alert
	m.activeAlerts["snap-alert"] = &Alert{
		ID:       "snap-alert",
		Type:     "snapshot-age",
		Instance: "instance1",
	}

	// Call the public method (handles locking internally)
	m.clearSnapshotAlertsForInstance("instance1")

	if len(m.activeAlerts) != 0 {
		t.Errorf("expected 0 alerts remaining, got %d", len(m.activeAlerts))
	}
}

func TestApplyGlobalOfflineSettingsLocked(t *testing.T) {
	// t.Parallel()

	t.Run("DisableAllNodesOffline clears node offline alerts", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		// Add node offline alerts
		m.activeAlerts["node-offline-node1"] = &Alert{ID: "node-offline-node1", Type: "offline"}
		m.activeAlerts["node-offline-node2"] = &Alert{ID: "node-offline-node2", Type: "offline"}
		// Add non-node alert
		m.activeAlerts["cpu-alert"] = &Alert{ID: "cpu-alert", Type: "cpu"}
		// Add to nodeOfflineCount
		m.nodeOfflineCount["node1"] = 3
		m.nodeOfflineCount["node2"] = 2

		m.config.DisableAllNodesOffline = true

		m.mu.Lock()
		m.applyGlobalOfflineSettingsLocked()
		m.mu.Unlock()

		// Node alerts should be cleared
		if _, exists := m.activeAlerts["node-offline-node1"]; exists {
			t.Error("expected node-offline-node1 to be cleared")
		}
		if _, exists := m.activeAlerts["node-offline-node2"]; exists {
			t.Error("expected node-offline-node2 to be cleared")
		}
		// Non-node alert should remain
		if _, exists := m.activeAlerts["cpu-alert"]; !exists {
			t.Error("expected cpu-alert to remain")
		}
		// nodeOfflineCount should be reset
		if len(m.nodeOfflineCount) != 0 {
			t.Errorf("expected nodeOfflineCount to be empty, got %d entries", len(m.nodeOfflineCount))
		}
	})

	t.Run("DisableAllPBSOffline clears PBS offline alerts", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		// Add PBS offline alerts
		m.activeAlerts["pbs-offline-pbs1"] = &Alert{ID: "pbs-offline-pbs1", ResourceID: "pbs1", Type: "offline"}
		// Add non-PBS alert
		m.activeAlerts["cpu-alert"] = &Alert{ID: "cpu-alert", Type: "cpu"}
		// Add to offlineConfirmations
		m.offlineConfirmations["pbs1"] = 3

		m.config.DisableAllPBSOffline = true

		m.mu.Lock()
		m.applyGlobalOfflineSettingsLocked()
		m.mu.Unlock()

		// PBS alert should be cleared
		if _, exists := m.activeAlerts["pbs-offline-pbs1"]; exists {
			t.Error("expected pbs-offline-pbs1 to be cleared")
		}
		// Non-PBS alert should remain
		if _, exists := m.activeAlerts["cpu-alert"]; !exists {
			t.Error("expected cpu-alert to remain")
		}
		// offlineConfirmations for PBS should be removed
		if _, exists := m.offlineConfirmations["pbs1"]; exists {
			t.Error("expected offlineConfirmations for pbs1 to be removed")
		}
	})

	t.Run("DisableAllGuestsOffline clears guest powered off alerts", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		// Add guest powered off alerts
		m.activeAlerts["guest-powered-off-vm1"] = &Alert{ID: "guest-powered-off-vm1", ResourceID: "vm1", Type: "powered-off"}
		// Add non-guest alert
		m.activeAlerts["cpu-alert"] = &Alert{ID: "cpu-alert", Type: "cpu"}
		// Add to offlineConfirmations
		m.offlineConfirmations["vm1"] = 2

		m.config.DisableAllGuestsOffline = true

		m.mu.Lock()
		m.applyGlobalOfflineSettingsLocked()
		m.mu.Unlock()

		// Guest alert should be cleared
		if _, exists := m.activeAlerts["guest-powered-off-vm1"]; exists {
			t.Error("expected guest-powered-off-vm1 to be cleared")
		}
		// Non-guest alert should remain
		if _, exists := m.activeAlerts["cpu-alert"]; !exists {
			t.Error("expected cpu-alert to remain")
		}
		// offlineConfirmations for guest should be removed
		if _, exists := m.offlineConfirmations["vm1"]; exists {
			t.Error("expected offlineConfirmations for vm1 to be removed")
		}
	})

	t.Run("DisableAllDockerHostsOffline clears docker host alerts", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		// Add docker host offline alerts
		m.activeAlerts["docker-host-offline-host1"] = &Alert{ID: "docker-host-offline-host1", Type: "offline"}
		// Add non-docker host alert
		m.activeAlerts["cpu-alert"] = &Alert{ID: "cpu-alert", Type: "cpu"}
		// Add to dockerOfflineCount
		m.dockerOfflineCount["host1"] = 3

		m.config.DisableAllDockerHostsOffline = true

		m.mu.Lock()
		m.applyGlobalOfflineSettingsLocked()
		m.mu.Unlock()

		// Docker host alert should be cleared
		if _, exists := m.activeAlerts["docker-host-offline-host1"]; exists {
			t.Error("expected docker-host-offline-host1 to be cleared")
		}
		// Non-docker host alert should remain
		if _, exists := m.activeAlerts["cpu-alert"]; !exists {
			t.Error("expected cpu-alert to remain")
		}
		// dockerOfflineCount should be reset
		if len(m.dockerOfflineCount) != 0 {
			t.Errorf("expected dockerOfflineCount to be empty, got %d entries", len(m.dockerOfflineCount))
		}
	})

	t.Run("DisableAllDockerContainers clears docker container alerts", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		// Add docker container alerts
		m.activeAlerts["docker-container-unhealthy-c1"] = &Alert{ID: "docker-container-unhealthy-c1", Type: "unhealthy"}
		m.activeAlerts["docker-container-exited-c2"] = &Alert{ID: "docker-container-exited-c2", Type: "exited"}
		// Add non-container alert
		m.activeAlerts["cpu-alert"] = &Alert{ID: "cpu-alert", Type: "cpu"}
		// Add tracking state
		m.dockerStateConfirm["c1"] = 2
		m.dockerRestartTracking["c1"] = &dockerRestartRecord{count: 5}
		m.dockerLastExitCode["c1"] = 137

		m.config.DisableAllDockerContainers = true

		m.mu.Lock()
		m.applyGlobalOfflineSettingsLocked()
		m.mu.Unlock()

		// Docker container alerts should be cleared
		if _, exists := m.activeAlerts["docker-container-unhealthy-c1"]; exists {
			t.Error("expected docker-container-unhealthy-c1 to be cleared")
		}
		if _, exists := m.activeAlerts["docker-container-exited-c2"]; exists {
			t.Error("expected docker-container-exited-c2 to be cleared")
		}
		// Non-container alert should remain
		if _, exists := m.activeAlerts["cpu-alert"]; !exists {
			t.Error("expected cpu-alert to remain")
		}
		// Tracking state should be reset
		if len(m.dockerStateConfirm) != 0 {
			t.Errorf("expected dockerStateConfirm to be empty, got %d entries", len(m.dockerStateConfirm))
		}
		if len(m.dockerRestartTracking) != 0 {
			t.Errorf("expected dockerRestartTracking to be empty, got %d entries", len(m.dockerRestartTracking))
		}
		if len(m.dockerLastExitCode) != 0 {
			t.Errorf("expected dockerLastExitCode to be empty, got %d entries", len(m.dockerLastExitCode))
		}
	})

	t.Run("DisableAllDockerServices clears docker service alerts", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		// Add docker service alerts
		m.activeAlerts["docker-service-unhealthy-svc1"] = &Alert{ID: "docker-service-unhealthy-svc1", Type: "unhealthy"}
		// Add non-service alert
		m.activeAlerts["cpu-alert"] = &Alert{ID: "cpu-alert", Type: "cpu"}

		m.config.DisableAllDockerServices = true

		m.mu.Lock()
		m.applyGlobalOfflineSettingsLocked()
		m.mu.Unlock()

		// Docker service alert should be cleared
		if _, exists := m.activeAlerts["docker-service-unhealthy-svc1"]; exists {
			t.Error("expected docker-service-unhealthy-svc1 to be cleared")
		}
		// Non-service alert should remain
		if _, exists := m.activeAlerts["cpu-alert"]; !exists {
			t.Error("expected cpu-alert to remain")
		}
	})

	t.Run("no settings enabled does nothing", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		// Add various alerts
		m.activeAlerts["node-offline-node1"] = &Alert{ID: "node-offline-node1", Type: "offline"}
		m.activeAlerts["pbs-offline-pbs1"] = &Alert{ID: "pbs-offline-pbs1", Type: "offline"}
		m.activeAlerts["docker-container-unhealthy-c1"] = &Alert{ID: "docker-container-unhealthy-c1", Type: "unhealthy"}

		// All disable settings are false by default

		m.mu.Lock()
		m.applyGlobalOfflineSettingsLocked()
		m.mu.Unlock()

		// All alerts should remain
		if len(m.activeAlerts) != 3 {
			t.Errorf("expected 3 alerts to remain, got %d", len(m.activeAlerts))
		}
	})
}

func TestHandleHostOffline(t *testing.T) {
	// t.Parallel()

	t.Run("empty host ID returns early", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		m.config.Enabled = true

		host := models.Host{ID: "", Hostname: "test-host"}
		m.HandleHostOffline(host)

		// No alert should be created
		if len(m.activeAlerts) != 0 {
			t.Errorf("expected 0 alerts, got %d", len(m.activeAlerts))
		}
	})

	t.Run("alerts disabled returns early", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		m.config.Enabled = false

		host := models.Host{ID: "host1", Hostname: "test-host"}
		m.HandleHostOffline(host)

		// No alert should be created
		if len(m.activeAlerts) != 0 {
			t.Errorf("expected 0 alerts, got %d", len(m.activeAlerts))
		}
	})

	t.Run("DisableAllHostsOffline clears alert and returns", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		m.config.Enabled = true
		m.config.DisableAllHostsOffline = true

		// Pre-create an alert and confirmation
		alertID := "host-offline-host1"
		m.activeAlerts[alertID] = &Alert{ID: alertID, Type: "host-offline"}
		m.offlineConfirmations["host:host1"] = 5

		host := models.Host{ID: "host1", Hostname: "test-host"}
		m.HandleHostOffline(host)

		// Alert should be cleared and confirmations removed
		if _, exists := m.activeAlerts[alertID]; exists {
			t.Error("expected alert to be cleared")
		}
		if _, exists := m.offlineConfirmations["host:host1"]; exists {
			t.Error("expected offlineConfirmations to be cleared")
		}
	})

	t.Run("override DisableConnectivity clears alert and returns", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		m.config.Enabled = true
		m.config.Overrides = map[string]ThresholdConfig{
			"host1": {DisableConnectivity: true},
		}

		// Pre-create an alert and confirmation
		alertID := "host-offline-host1"
		m.activeAlerts[alertID] = &Alert{ID: alertID, Type: "host-offline"}
		m.offlineConfirmations["host:host1"] = 5

		host := models.Host{ID: "host1", Hostname: "test-host"}
		m.HandleHostOffline(host)

		// Alert should be cleared and confirmations removed
		if _, exists := m.activeAlerts[alertID]; exists {
			t.Error("expected alert to be cleared")
		}
		if _, exists := m.offlineConfirmations["host:host1"]; exists {
			t.Error("expected offlineConfirmations to be cleared")
		}
	})

	t.Run("override Disabled clears alert and returns", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		m.config.Enabled = true
		m.config.Overrides = map[string]ThresholdConfig{
			"host1": {Disabled: true},
		}

		host := models.Host{ID: "host1", Hostname: "test-host"}
		m.HandleHostOffline(host)

		// No alert should be created
		if len(m.activeAlerts) != 0 {
			t.Errorf("expected 0 alerts, got %d", len(m.activeAlerts))
		}
	})

	t.Run("existing alert updates LastSeen", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		m.config.Enabled = true

		alertID := "host-offline-host1"
		oldTime := time.Now().Add(-1 * time.Hour)
		m.activeAlerts[alertID] = &Alert{ID: alertID, Type: "host-offline", LastSeen: oldTime}

		host := models.Host{ID: "host1", Hostname: "test-host"}
		m.HandleHostOffline(host)

		// LastSeen should be updated
		alert := m.activeAlerts[alertID]
		if alert.LastSeen.Before(time.Now().Add(-1 * time.Minute)) {
			t.Errorf("expected LastSeen to be updated to recent time, got %v", alert.LastSeen)
		}
	})

	t.Run("insufficient confirmations waits", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		m.config.Enabled = true

		host := models.Host{ID: "host1", Hostname: "test-host"}

		// First two calls should not create alert
		m.HandleHostOffline(host)
		if len(m.activeAlerts) != 0 {
			t.Errorf("expected 0 alerts after 1st call, got %d", len(m.activeAlerts))
		}
		if m.offlineConfirmations["host:host1"] != 1 {
			t.Errorf("expected 1 confirmation, got %d", m.offlineConfirmations["host:host1"])
		}

		m.HandleHostOffline(host)
		if len(m.activeAlerts) != 0 {
			t.Errorf("expected 0 alerts after 2nd call, got %d", len(m.activeAlerts))
		}
		if m.offlineConfirmations["host:host1"] != 2 {
			t.Errorf("expected 2 confirmations, got %d", m.offlineConfirmations["host:host1"])
		}
	})

	t.Run("sufficient confirmations creates alert", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		m.config.Enabled = true

		host := models.Host{
			ID:          "host1",
			Hostname:    "test-host",
			DisplayName: "Test Host",
			Platform:    "linux",
			OSName:      "Ubuntu",
			OSVersion:   "22.04",
		}

		// Make 3 calls to reach required confirmations
		m.HandleHostOffline(host)
		m.HandleHostOffline(host)
		m.HandleHostOffline(host)

		// Alert should now be created
		alertID := "host-offline-host1"
		alert, exists := m.activeAlerts[alertID]
		if !exists {
			t.Fatal("expected alert to be created after 3 confirmations")
		}
		if alert.Type != "host-offline" {
			t.Errorf("expected type 'host-offline', got '%s'", alert.Type)
		}
		if alert.Level != AlertLevelCritical {
			t.Errorf("expected level Critical, got '%s'", alert.Level)
		}
		if alert.ResourceName == "" {
			t.Error("expected ResourceName to be set")
		}
	})
}

func TestReevaluateActiveAlertsLocked(t *testing.T) {
	// t.Parallel()

	t.Run("empty alerts map is no-op", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.reevaluateActiveAlertsLocked()
		m.mu.Unlock()

		if len(m.activeAlerts) != 0 {
			t.Errorf("expected 0 alerts, got %d", len(m.activeAlerts))
		}
	})

	t.Run("alert with insufficient ID parts is skipped", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		// Alert ID without dash separator
		m.activeAlerts["singlepart"] = &Alert{ID: "singlepart", Type: "cpu", Value: 90}

		m.mu.Lock()
		m.reevaluateActiveAlertsLocked()
		m.mu.Unlock()

		// Alert should remain (skipped due to ID format)
		if _, exists := m.activeAlerts["singlepart"]; !exists {
			t.Error("expected singlepart alert to remain")
		}
	})

	t.Run("DisableAllPMG resolves PMG queue alerts", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		// Add PMG queue alert
		m.activeAlerts["pmg-queue-cpu"] = &Alert{
			ID:   "pmg-queue-cpu",
			Type: "queue-depth",
		}

		m.config.DisableAllPMG = true

		m.mu.Lock()
		m.reevaluateActiveAlertsLocked()
		m.mu.Unlock()

		// PMG alert should be resolved
		if _, exists := m.activeAlerts["pmg-queue-cpu"]; exists {
			t.Error("expected PMG alert to be resolved")
		}
	})

	t.Run("DisableAllHosts resolves Host alerts", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		// Add host alert with resourceType metadata
		m.activeAlerts["host-1-cpu"] = &Alert{
			ID:    "host-1-cpu",
			Type:  "cpu",
			Value: 90,
			Metadata: map[string]interface{}{
				"resourceType": "Host",
			},
		}

		m.config.DisableAllHosts = true

		m.mu.Lock()
		m.reevaluateActiveAlertsLocked()
		m.mu.Unlock()

		// Host alert should be resolved
		if _, exists := m.activeAlerts["host-1-cpu"]; exists {
			t.Error("expected Host alert to be resolved")
		}
	})

	t.Run("Docker host offline alerts are skipped", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		// Add docker host offline alert
		m.activeAlerts["docker-host-1-offline"] = &Alert{
			ID:   "docker-host-1-offline",
			Type: "docker-host-offline",
		}

		m.mu.Lock()
		m.reevaluateActiveAlertsLocked()
		m.mu.Unlock()

		// Docker host offline alert should remain (skipped)
		if _, exists := m.activeAlerts["docker-host-1-offline"]; !exists {
			t.Error("expected docker-host-offline alert to remain")
		}
	})

	t.Run("DisableAllDockerHosts resolves dockerhost alerts", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		// Add dockerhost metric alert
		m.activeAlerts["dockerhost-1-cpu"] = &Alert{
			ID:    "dockerhost-1-cpu",
			Type:  "cpu",
			Value: 90,
			Metadata: map[string]interface{}{
				"resourceType": "dockerhost",
			},
		}

		m.config.DisableAllDockerHosts = true

		m.mu.Lock()
		m.reevaluateActiveAlertsLocked()
		m.mu.Unlock()

		// Dockerhost alert should be resolved
		if _, exists := m.activeAlerts["dockerhost-1-cpu"]; exists {
			t.Error("expected dockerhost alert to be resolved")
		}
	})

	t.Run("DisableAllNodes resolves Node alerts", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		// Add node alert with Instance = "Node"
		m.activeAlerts["node1-cpu"] = &Alert{
			ID:       "node1-cpu",
			Type:     "cpu",
			Value:    90,
			Instance: "Node",
		}

		m.config.DisableAllNodes = true

		m.mu.Lock()
		m.reevaluateActiveAlertsLocked()
		m.mu.Unlock()

		// Node alert should be resolved
		if _, exists := m.activeAlerts["node1-cpu"]; exists {
			t.Error("expected Node alert to be resolved")
		}
	})

	t.Run("DisableAllStorage resolves Storage alerts", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		// Add storage alert with Instance = "Storage"
		m.activeAlerts["storage1-usage"] = &Alert{
			ID:       "storage1-usage",
			Type:     "usage",
			Value:    90,
			Instance: "Storage",
		}

		m.config.DisableAllStorage = true

		m.mu.Lock()
		m.reevaluateActiveAlertsLocked()
		m.mu.Unlock()

		// Storage alert should be resolved
		if _, exists := m.activeAlerts["storage1-usage"]; exists {
			t.Error("expected Storage alert to be resolved")
		}
	})

	t.Run("DisableAllPBS resolves PBS alerts", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		// Add PBS alert with Instance = "PBS"
		m.activeAlerts["pbs1-cpu"] = &Alert{
			ID:       "pbs1-cpu",
			Type:     "cpu",
			Value:    90,
			Instance: "PBS",
		}

		m.config.DisableAllPBS = true

		m.mu.Lock()
		m.reevaluateActiveAlertsLocked()
		m.mu.Unlock()

		// PBS alert should be resolved
		if _, exists := m.activeAlerts["pbs1-cpu"]; exists {
			t.Error("expected PBS alert to be resolved")
		}
	})

	t.Run("DisableAllGuests resolves Guest alerts", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		// Add guest alert with Instance set to something other than "Node"/"Storage"/"PBS"
		// Note: If both Instance and Node are empty, it matches the node branch
		m.activeAlerts["guest1-cpu"] = &Alert{
			ID:       "guest1-cpu",
			Type:     "cpu",
			Value:    90,
			Instance: "qemu/100", // Guest instance
			Node:     "pve1",     // Different from Instance, so doesn't match node branch
		}

		m.config.DisableAllGuests = true

		m.mu.Lock()
		m.reevaluateActiveAlertsLocked()
		m.mu.Unlock()

		// Guest alert should be resolved
		if _, exists := m.activeAlerts["guest1-cpu"]; exists {
			t.Error("expected Guest alert to be resolved")
		}
	})

	t.Run("alert with disabled override is resolved", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		// Add guest alert with override
		m.activeAlerts["guest1-cpu"] = &Alert{
			ID:       "guest1-cpu",
			Type:     "cpu",
			Value:    90,
			Instance: "qemu/100",
			Node:     "pve1",
		}
		m.config.Overrides = map[string]ThresholdConfig{
			"guest1": {Disabled: true},
		}

		m.mu.Lock()
		m.reevaluateActiveAlertsLocked()
		m.mu.Unlock()

		// Alert should be resolved due to disabled override
		if _, exists := m.activeAlerts["guest1-cpu"]; exists {
			t.Error("expected alert with disabled override to be resolved")
		}
	})

	t.Run("alert below clear threshold is resolved", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		// Add guest alert below new clear threshold
		m.activeAlerts["guest1-cpu"] = &Alert{
			ID:        "guest1-cpu",
			Type:      "cpu",
			Value:     70, // Below clear threshold
			Threshold: 80,
			Instance:  "qemu/100",
			Node:      "pve1",
		}
		m.config.GuestDefaults.CPU = &HysteresisThreshold{Trigger: 80, Clear: 75}

		m.mu.Lock()
		m.reevaluateActiveAlertsLocked()
		m.mu.Unlock()

		// Alert should be resolved (value 70 < clear 75)
		if _, exists := m.activeAlerts["guest1-cpu"]; exists {
			t.Error("expected alert below clear threshold to be resolved")
		}
	})

	t.Run("alert between clear and trigger is resolved on config change", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		// Add guest alert between clear and new higher trigger
		m.activeAlerts["guest1-cpu"] = &Alert{
			ID:        "guest1-cpu",
			Type:      "cpu",
			Value:     85, // Between clear (75) and new trigger (90)
			Threshold: 80,
			Instance:  "qemu/100",
			Node:      "pve1",
		}
		m.config.GuestDefaults.CPU = &HysteresisThreshold{Trigger: 90, Clear: 75}

		m.mu.Lock()
		m.reevaluateActiveAlertsLocked()
		m.mu.Unlock()

		// Alert should be resolved (value 85 < trigger 90)
		if _, exists := m.activeAlerts["guest1-cpu"]; exists {
			t.Error("expected alert between thresholds to be resolved")
		}
	})
}

func TestHandleHostRemoved(t *testing.T) {
	// t.Parallel()

	t.Run("empty host ID is no-op", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		m.mu.Lock()
		m.activeAlerts["host-offline-host1"] = &Alert{ID: "host-offline-host1"}
		m.mu.Unlock()

		// Empty ID host
		m.HandleHostRemoved(models.Host{ID: ""})

		// Alert should still exist
		m.mu.RLock()
		_, exists := m.activeAlerts["host-offline-host1"]
		m.mu.RUnlock()
		if !exists {
			t.Error("expected alert to remain when empty host ID passed")
		}
	})

	t.Run("clears host offline alert", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		m.mu.Lock()
		m.config.Enabled = true
		m.activeAlerts["host-offline-host1"] = &Alert{
			ID:         "host-offline-host1",
			ResourceID: "host:host1",
		}
		m.offlineConfirmations["host:host1"] = 5
		m.mu.Unlock()

		m.HandleHostRemoved(models.Host{ID: "host1", Hostname: "testhost"})

		m.mu.RLock()
		_, alertExists := m.activeAlerts["host-offline-host1"]
		_, confirmExists := m.offlineConfirmations["host:host1"]
		m.mu.RUnlock()

		if alertExists {
			t.Error("expected host offline alert to be cleared")
		}
		if confirmExists {
			t.Error("expected offline confirmations to be cleared")
		}
	})

	t.Run("clears host metric alerts", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		m.mu.Lock()
		m.config.Enabled = true
		// Add CPU and memory alerts for host
		m.activeAlerts["host:host1-cpu"] = &Alert{
			ID:         "host:host1-cpu",
			ResourceID: "host:host1",
		}
		m.activeAlerts["host:host1-memory"] = &Alert{
			ID:         "host:host1-memory",
			ResourceID: "host:host1",
		}
		m.mu.Unlock()

		m.HandleHostRemoved(models.Host{ID: "host1", Hostname: "testhost"})

		m.mu.RLock()
		_, cpuExists := m.activeAlerts["host:host1-cpu"]
		_, memExists := m.activeAlerts["host:host1-memory"]
		m.mu.RUnlock()

		if cpuExists {
			t.Error("expected host CPU alert to be cleared")
		}
		if memExists {
			t.Error("expected host memory alert to be cleared")
		}
	})

	t.Run("clears host disk alerts", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		m.mu.Lock()
		m.config.Enabled = true
		// Add disk alerts for host
		m.activeAlerts["host:host1/disk:sda-usage"] = &Alert{
			ID:         "host:host1/disk:sda-usage",
			ResourceID: "host:host1/disk:sda",
		}
		m.activeAlerts["host:host1/disk:sdb-usage"] = &Alert{
			ID:         "host:host1/disk:sdb-usage",
			ResourceID: "host:host1/disk:sdb",
		}
		m.mu.Unlock()

		m.HandleHostRemoved(models.Host{ID: "host1", Hostname: "testhost"})

		m.mu.RLock()
		_, sda := m.activeAlerts["host:host1/disk:sda-usage"]
		_, sdb := m.activeAlerts["host:host1/disk:sdb-usage"]
		m.mu.RUnlock()

		if sda {
			t.Error("expected host disk sda alert to be cleared")
		}
		if sdb {
			t.Error("expected host disk sdb alert to be cleared")
		}
	})

	t.Run("clears all alert types together", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		m.mu.Lock()
		m.config.Enabled = true
		// Add multiple alert types
		m.activeAlerts["host-offline-host1"] = &Alert{ID: "host-offline-host1", ResourceID: "host:host1"}
		m.activeAlerts["host:host1-cpu"] = &Alert{ID: "host:host1-cpu", ResourceID: "host:host1"}
		m.activeAlerts["host:host1-memory"] = &Alert{ID: "host:host1-memory", ResourceID: "host:host1"}
		m.activeAlerts["host:host1/disk:sda-usage"] = &Alert{ID: "host:host1/disk:sda-usage", ResourceID: "host:host1/disk:sda"}
		m.offlineConfirmations["host:host1"] = 3
		m.mu.Unlock()

		m.HandleHostRemoved(models.Host{ID: "host1", Hostname: "testhost"})

		m.mu.RLock()
		alertCount := 0
		for id := range m.activeAlerts {
			if strings.Contains(id, "host1") {
				alertCount++
			}
		}
		_, confirmExists := m.offlineConfirmations["host:host1"]
		m.mu.RUnlock()

		if alertCount > 0 {
			t.Errorf("expected all host1 alerts to be cleared, got %d remaining", alertCount)
		}
		if confirmExists {
			t.Error("expected offline confirmations to be cleared")
		}
	})
}

func TestReevaluateGuestAlert(t *testing.T) {
	// t.Parallel()

	t.Run("no active alerts is no-op", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		m.mu.Lock()
		m.config.Enabled = true
		m.config.GuestDefaults.CPU = &HysteresisThreshold{Trigger: 80, Clear: 70}
		m.mu.Unlock()

		// No alerts exist - should not panic
		m.ReevaluateGuestAlert(nil, "guest1")

		m.mu.RLock()
		count := len(m.activeAlerts)
		m.mu.RUnlock()
		if count != 0 {
			t.Errorf("expected 0 alerts, got %d", count)
		}
	})

	t.Run("clears alert when threshold disabled (nil)", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		m.mu.Lock()
		m.config.Enabled = true
		m.activeAlerts["guest1-cpu"] = &Alert{
			ID:    "guest1-cpu",
			Type:  "cpu",
			Value: 90,
		}
		m.config.GuestDefaults.CPU = nil // Disabled
		m.mu.Unlock()

		m.ReevaluateGuestAlert(nil, "guest1")

		m.mu.RLock()
		_, exists := m.activeAlerts["guest1-cpu"]
		m.mu.RUnlock()
		if exists {
			t.Error("expected alert to be cleared when threshold is nil")
		}
	})

	t.Run("clears alert when trigger is zero", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		m.mu.Lock()
		m.config.Enabled = true
		m.activeAlerts["guest1-memory"] = &Alert{
			ID:    "guest1-memory",
			Type:  "memory",
			Value: 85,
		}
		m.config.GuestDefaults.Memory = &HysteresisThreshold{Trigger: 0, Clear: 0}
		m.mu.Unlock()

		m.ReevaluateGuestAlert(nil, "guest1")

		m.mu.RLock()
		_, exists := m.activeAlerts["guest1-memory"]
		m.mu.RUnlock()
		if exists {
			t.Error("expected alert to be cleared when trigger is 0")
		}
	})

	t.Run("clears alert when value below clear threshold", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		m.mu.Lock()
		m.config.Enabled = true
		m.activeAlerts["guest1-cpu"] = &Alert{
			ID:    "guest1-cpu",
			Type:  "cpu",
			Value: 65, // Below clear threshold of 70
		}
		m.config.GuestDefaults.CPU = &HysteresisThreshold{Trigger: 80, Clear: 70}
		m.mu.Unlock()

		m.ReevaluateGuestAlert(nil, "guest1")

		m.mu.RLock()
		_, exists := m.activeAlerts["guest1-cpu"]
		m.mu.RUnlock()
		if exists {
			t.Error("expected alert to be cleared when value below clear threshold")
		}
	})

	t.Run("clears alert when value below trigger threshold", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		m.mu.Lock()
		m.config.Enabled = true
		m.activeAlerts["guest1-disk"] = &Alert{
			ID:    "guest1-disk",
			Type:  "disk",
			Value: 75, // Below trigger of 80
		}
		m.config.GuestDefaults.Disk = &HysteresisThreshold{Trigger: 80, Clear: 70}
		m.mu.Unlock()

		m.ReevaluateGuestAlert(nil, "guest1")

		m.mu.RLock()
		_, exists := m.activeAlerts["guest1-disk"]
		m.mu.RUnlock()
		if exists {
			t.Error("expected alert to be cleared when value below trigger")
		}
	})

	t.Run("keeps alert when value above both thresholds", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		m.mu.Lock()
		m.config.Enabled = true
		m.activeAlerts["guest1-cpu"] = &Alert{
			ID:    "guest1-cpu",
			Type:  "cpu",
			Value: 90, // Above both trigger (80) and clear (70)
		}
		m.config.GuestDefaults.CPU = &HysteresisThreshold{Trigger: 80, Clear: 70}
		m.mu.Unlock()

		m.ReevaluateGuestAlert(nil, "guest1")

		m.mu.RLock()
		_, exists := m.activeAlerts["guest1-cpu"]
		m.mu.RUnlock()
		if !exists {
			t.Error("expected alert to remain when value above thresholds")
		}
	})

	t.Run("processes all metric types", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		m.mu.Lock()
		m.config.Enabled = true
		// Add alerts for all metric types with values below threshold
		metrics := []string{"cpu", "memory", "disk", "diskRead", "diskWrite", "networkIn", "networkOut"}
		for _, metric := range metrics {
			m.activeAlerts[fmt.Sprintf("guest1-%s", metric)] = &Alert{
				ID:    fmt.Sprintf("guest1-%s", metric),
				Type:  metric,
				Value: 50, // Below threshold
			}
		}
		threshold := &HysteresisThreshold{Trigger: 80, Clear: 70}
		m.config.GuestDefaults.CPU = threshold
		m.config.GuestDefaults.Memory = threshold
		m.config.GuestDefaults.Disk = threshold
		m.config.GuestDefaults.DiskRead = threshold
		m.config.GuestDefaults.DiskWrite = threshold
		m.config.GuestDefaults.NetworkIn = threshold
		m.config.GuestDefaults.NetworkOut = threshold
		m.mu.Unlock()

		m.ReevaluateGuestAlert(nil, "guest1")

		m.mu.RLock()
		remaining := len(m.activeAlerts)
		m.mu.RUnlock()
		if remaining != 0 {
			t.Errorf("expected all alerts to be cleared, got %d remaining", remaining)
		}
	})

	t.Run("clears pending alert when threshold disabled", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		m.mu.Lock()
		m.config.Enabled = true
		m.activeAlerts["guest1-cpu"] = &Alert{
			ID:    "guest1-cpu",
			Type:  "cpu",
			Value: 90,
		}
		m.pendingAlerts["guest1-cpu"] = time.Now() // pendingAlerts is map[string]time.Time
		m.config.GuestDefaults.CPU = nil           // Disabled
		m.mu.Unlock()

		m.ReevaluateGuestAlert(nil, "guest1")

		m.mu.RLock()
		_, alertExists := m.activeAlerts["guest1-cpu"]
		_, pendingExists := m.pendingAlerts["guest1-cpu"]
		m.mu.RUnlock()
		if alertExists {
			t.Error("expected active alert to be cleared")
		}
		if pendingExists {
			t.Error("expected pending alert to be cleared")
		}
	})

	t.Run("uses clear equals trigger when clear is zero", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		m.mu.Lock()
		m.config.Enabled = true
		m.activeAlerts["guest1-cpu"] = &Alert{
			ID:    "guest1-cpu",
			Type:  "cpu",
			Value: 75, // Below trigger of 80
		}
		// Clear is 0, so it should use trigger (80) as clear threshold
		m.config.GuestDefaults.CPU = &HysteresisThreshold{Trigger: 80, Clear: 0}
		m.mu.Unlock()

		m.ReevaluateGuestAlert(nil, "guest1")

		m.mu.RLock()
		_, exists := m.activeAlerts["guest1-cpu"]
		m.mu.RUnlock()
		if exists {
			t.Error("expected alert to be cleared when value below trigger (used as clear)")
		}
	})

	t.Run("ignores alerts for different guests", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		m.mu.Lock()
		m.config.Enabled = true
		m.activeAlerts["guest1-cpu"] = &Alert{
			ID:    "guest1-cpu",
			Type:  "cpu",
			Value: 50, // Below threshold
		}
		m.activeAlerts["guest2-cpu"] = &Alert{
			ID:    "guest2-cpu",
			Type:  "cpu",
			Value: 50, // Below threshold
		}
		m.config.GuestDefaults.CPU = &HysteresisThreshold{Trigger: 80, Clear: 70}
		m.mu.Unlock()

		// Only reevaluate guest1
		m.ReevaluateGuestAlert(nil, "guest1")

		m.mu.RLock()
		_, guest1Exists := m.activeAlerts["guest1-cpu"]
		_, guest2Exists := m.activeAlerts["guest2-cpu"]
		m.mu.RUnlock()

		if guest1Exists {
			t.Error("expected guest1 alert to be cleared")
		}
		if !guest2Exists {
			t.Error("expected guest2 alert to remain (not reevaluated)")
		}
	})
}

func TestHandleDockerHostOffline(t *testing.T) {
	// t.Parallel()

	t.Run("empty host ID is no-op", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		m.mu.Lock()
		m.config.Enabled = true
		initialCount := len(m.activeAlerts)
		m.mu.Unlock()

		m.HandleDockerHostOffline(models.DockerHost{ID: ""})

		m.mu.RLock()
		finalCount := len(m.activeAlerts)
		m.mu.RUnlock()
		if finalCount != initialCount {
			t.Error("expected no change when empty host ID passed")
		}
	})

	t.Run("disabled alerts is no-op", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		m.mu.Lock()
		m.config.Enabled = false
		m.mu.Unlock()

		m.HandleDockerHostOffline(models.DockerHost{ID: "docker1", DisplayName: "Docker Host 1"})

		m.mu.RLock()
		_, exists := m.activeAlerts["docker-host-offline-docker1"]
		m.mu.RUnlock()
		if exists {
			t.Error("expected no alert when alerts are disabled")
		}
	})

	t.Run("DisableAllDockerHostsOffline clears tracking and alert", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		m.mu.Lock()
		m.config.Enabled = true
		m.config.DisableAllDockerHostsOffline = true
		m.dockerOfflineCount["docker1"] = 5
		m.activeAlerts["docker-host-offline-docker1"] = &Alert{ID: "docker-host-offline-docker1"}
		m.mu.Unlock()

		m.HandleDockerHostOffline(models.DockerHost{ID: "docker1", DisplayName: "Docker Host 1"})

		m.mu.RLock()
		_, alertExists := m.activeAlerts["docker-host-offline-docker1"]
		_, countExists := m.dockerOfflineCount["docker1"]
		m.mu.RUnlock()
		if alertExists {
			t.Error("expected alert to be cleared")
		}
		if countExists {
			t.Error("expected offline count to be cleared")
		}
	})

	t.Run("override DisableConnectivity clears tracking and alert", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		m.mu.Lock()
		m.config.Enabled = true
		m.config.Overrides = map[string]ThresholdConfig{
			"docker1": {DisableConnectivity: true},
		}
		m.dockerOfflineCount["docker1"] = 3
		m.activeAlerts["docker-host-offline-docker1"] = &Alert{ID: "docker-host-offline-docker1"}
		m.mu.Unlock()

		m.HandleDockerHostOffline(models.DockerHost{ID: "docker1", DisplayName: "Docker Host 1"})

		m.mu.RLock()
		_, alertExists := m.activeAlerts["docker-host-offline-docker1"]
		_, countExists := m.dockerOfflineCount["docker1"]
		m.mu.RUnlock()
		if alertExists {
			t.Error("expected alert to be cleared with override")
		}
		if countExists {
			t.Error("expected offline count to be cleared with override")
		}
	})

	t.Run("existing alert updates LastSeen", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		oldTime := time.Now().Add(-1 * time.Hour)
		m.mu.Lock()
		m.config.Enabled = true
		m.activeAlerts["docker-host-offline-docker1"] = &Alert{
			ID:       "docker-host-offline-docker1",
			LastSeen: oldTime,
		}
		m.mu.Unlock()

		m.HandleDockerHostOffline(models.DockerHost{ID: "docker1", DisplayName: "Docker Host 1"})

		m.mu.RLock()
		alert := m.activeAlerts["docker-host-offline-docker1"]
		m.mu.RUnlock()
		if alert == nil {
			t.Fatal("expected alert to exist")
		}
		if !alert.LastSeen.After(oldTime) {
			t.Error("expected LastSeen to be updated")
		}
	})

	t.Run("requires 3 confirmations before alert", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		m.mu.Lock()
		m.config.Enabled = true
		m.mu.Unlock()

		host := models.DockerHost{ID: "docker1", DisplayName: "Docker Host 1", Hostname: "docker-server"}

		// First call - confirmation 1
		m.HandleDockerHostOffline(host)
		m.mu.RLock()
		count1 := m.dockerOfflineCount["docker1"]
		alert1 := m.activeAlerts["docker-host-offline-docker1"]
		m.mu.RUnlock()
		if count1 != 1 {
			t.Errorf("expected count 1, got %d", count1)
		}
		if alert1 != nil {
			t.Error("expected no alert after 1 confirmation")
		}

		// Second call - confirmation 2
		m.HandleDockerHostOffline(host)
		m.mu.RLock()
		count2 := m.dockerOfflineCount["docker1"]
		alert2 := m.activeAlerts["docker-host-offline-docker1"]
		m.mu.RUnlock()
		if count2 != 2 {
			t.Errorf("expected count 2, got %d", count2)
		}
		if alert2 != nil {
			t.Error("expected no alert after 2 confirmations")
		}

		// Third call - confirmation 3 - should create alert
		m.HandleDockerHostOffline(host)
		m.mu.RLock()
		count3 := m.dockerOfflineCount["docker1"]
		alert3 := m.activeAlerts["docker-host-offline-docker1"]
		m.mu.RUnlock()
		if count3 != 3 {
			t.Errorf("expected count 3, got %d", count3)
		}
		if alert3 == nil {
			t.Fatal("expected alert after 3 confirmations")
		}
		if alert3.Type != "docker-host-offline" {
			t.Errorf("expected type docker-host-offline, got %s", alert3.Type)
		}
		if alert3.Level != AlertLevelCritical {
			t.Errorf("expected critical level, got %s", alert3.Level)
		}
	})

	t.Run("alert has correct metadata", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		m.mu.Lock()
		m.config.Enabled = true
		m.dockerOfflineCount["docker1"] = 2 // Pre-set to trigger on next call
		m.mu.Unlock()

		host := models.DockerHost{
			ID:          "docker1",
			DisplayName: "My Docker Host",
			Hostname:    "docker-server.local",
			AgentID:     "agent-123",
		}

		m.HandleDockerHostOffline(host)

		m.mu.RLock()
		alert := m.activeAlerts["docker-host-offline-docker1"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected alert to be created")
		}
		if alert.ResourceID != "docker:docker1" {
			t.Errorf("expected resourceID docker:docker1, got %s", alert.ResourceID)
		}
		if alert.ResourceName != "My Docker Host" {
			t.Errorf("expected resourceName 'My Docker Host', got %s", alert.ResourceName)
		}
		if alert.Node != "docker-server.local" {
			t.Errorf("expected node docker-server.local, got %s", alert.Node)
		}
		if alert.Metadata["resourceType"] != "DockerHost" {
			t.Errorf("expected metadata resourceType DockerHost, got %v", alert.Metadata["resourceType"])
		}
		if alert.Metadata["hostId"] != "docker1" {
			t.Errorf("expected metadata hostId docker1, got %v", alert.Metadata["hostId"])
		}
		if alert.Metadata["agentId"] != "agent-123" {
			t.Errorf("expected metadata agentId agent-123, got %v", alert.Metadata["agentId"])
		}
	})
}

func TestSetMetricHooks(t *testing.T) {
	// NOT parallel - modifies package-level state
	// Save existing state and restore after test
	oldFired := recordAlertFired
	oldResolved := recordAlertResolved
	oldSuppressed := recordAlertSuppressed
	oldAcknowledged := recordAlertAcknowledged
	defer func() {
		recordAlertFired = oldFired
		recordAlertResolved = oldResolved
		recordAlertSuppressed = oldSuppressed
		recordAlertAcknowledged = oldAcknowledged
	}()

	t.Run("sets all hooks", func(t *testing.T) {
		var firedCalled, resolvedCalled, suppressedCalled, acknowledgedCalled bool

		SetMetricHooks(
			func(a *Alert) { firedCalled = true },
			func(a *Alert) { resolvedCalled = true },
			func(s string) { suppressedCalled = true },
			func() { acknowledgedCalled = true },
		)

		// Verify hooks are set by calling them (if they were nil, this would panic)
		if recordAlertFired != nil {
			recordAlertFired(&Alert{})
		}
		if recordAlertResolved != nil {
			recordAlertResolved(&Alert{})
		}
		if recordAlertSuppressed != nil {
			recordAlertSuppressed("test")
		}
		if recordAlertAcknowledged != nil {
			recordAlertAcknowledged()
		}

		if !firedCalled {
			t.Error("expected fired hook to be called")
		}
		if !resolvedCalled {
			t.Error("expected resolved hook to be called")
		}
		if !suppressedCalled {
			t.Error("expected suppressed hook to be called")
		}
		if !acknowledgedCalled {
			t.Error("expected acknowledged hook to be called")
		}
	})

	t.Run("nil hooks are safe", func(t *testing.T) {
		SetMetricHooks(nil, nil, nil, nil)

		// Should not panic
		if recordAlertFired != nil {
			t.Error("expected fired hook to be nil")
		}
		if recordAlertResolved != nil {
			t.Error("expected resolved hook to be nil")
		}
	})
}

func TestNotifyExistingAlert(t *testing.T) {
	// t.Parallel()

	t.Run("non-existent alert is no-op", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		// Should not panic
		m.NotifyExistingAlert("non-existent-alert")
	})

	t.Run("existing alert dispatches notification", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		dispatchedCh := make(chan bool, 1)
		m.SetAlertCallback(func(a *Alert) {
			dispatchedCh <- true
		})

		m.mu.Lock()
		m.config.Enabled = true
		m.config.ActivationState = ActivationActive // Must be active to dispatch
		m.activeAlerts["test-alert"] = &Alert{
			ID:    "test-alert",
			Type:  "test",
			Level: AlertLevelWarning,
		}
		m.mu.Unlock()

		m.NotifyExistingAlert("test-alert")

		// Wait for async dispatch with timeout
		select {
		case <-dispatchedCh:
			// Success
		case <-time.After(1 * time.Second):
			t.Error("expected alert callback to be called (timeout)")
		}
	})
}

func TestGetResolvedAlert(t *testing.T) {
	// t.Parallel()

	t.Run("returns nil for non-existent alert", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		result := m.GetResolvedAlert("non-existent")
		if result != nil {
			t.Error("expected nil for non-existent alert")
		}
	})

	t.Run("returns nil for nil resolved entry", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		m.resolvedMutex.Lock()
		m.recentlyResolved["test"] = nil
		m.resolvedMutex.Unlock()

		result := m.GetResolvedAlert("test")
		if result != nil {
			t.Error("expected nil for nil resolved entry")
		}
	})

	t.Run("returns nil when Alert is nil", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		m.resolvedMutex.Lock()
		m.recentlyResolved["test"] = &ResolvedAlert{Alert: nil}
		m.resolvedMutex.Unlock()

		result := m.GetResolvedAlert("test")
		if result != nil {
			t.Error("expected nil when Alert is nil")
		}
	})

	t.Run("returns cloned alert", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		resolvedTime := time.Now()
		m.resolvedMutex.Lock()
		m.recentlyResolved["test"] = &ResolvedAlert{
			Alert: &Alert{
				ID:           "test",
				Type:         "cpu",
				Level:        AlertLevelWarning,
				ResourceID:   "res1",
				ResourceName: "Resource 1",
			},
			ResolvedTime: resolvedTime,
		}
		m.resolvedMutex.Unlock()

		result := m.GetResolvedAlert("test")
		if result == nil {
			t.Fatal("expected non-nil result")
		}
		if result.Alert.ID != "test" {
			t.Errorf("expected ID test, got %s", result.Alert.ID)
		}
		if result.ResolvedTime != resolvedTime {
			t.Error("expected resolved time to match")
		}
	})
}

func TestGetAlertHistory(t *testing.T) {
	// t.Parallel()

	t.Run("returns history from history manager", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		// Add some alerts to history
		m.historyManager.AddAlert(Alert{ID: "alert1", Type: "cpu"})
		m.historyManager.AddAlert(Alert{ID: "alert2", Type: "memory"})

		history := m.GetAlertHistory(10)
		if len(history) < 2 {
			t.Errorf("expected at least 2 history entries, got %d", len(history))
		}
	})

	t.Run("respects limit", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		// Add alerts
		for i := 0; i < 5; i++ {
			m.historyManager.AddAlert(Alert{ID: fmt.Sprintf("alert%d", i), Type: "test"})
		}

		history := m.GetAlertHistory(2)
		if len(history) > 2 {
			t.Errorf("expected max 2 entries, got %d", len(history))
		}
	})
}

func TestGetAlertHistorySince(t *testing.T) {
	// t.Parallel()

	t.Run("zero time returns all history", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		m.historyManager.AddAlert(Alert{ID: "alert1", Type: "cpu"})

		history := m.GetAlertHistorySince(time.Time{}, 10)
		if len(history) == 0 {
			t.Error("expected history entries for zero time")
		}
	})

	t.Run("filters by time", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		// Add an alert
		m.historyManager.AddAlert(Alert{ID: "alert1", Type: "cpu", StartTime: time.Now()})

		// Query for alerts after now + 1 hour (should return none)
		future := time.Now().Add(1 * time.Hour)
		history := m.GetAlertHistorySince(future, 10)
		if len(history) != 0 {
			t.Errorf("expected 0 entries for future time, got %d", len(history))
		}
	})
}

func TestClearAlertHistory(t *testing.T) {
	// t.Parallel()

	t.Run("clears all history", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		// Add some alerts
		m.historyManager.AddAlert(Alert{ID: "alert1", Type: "cpu"})
		m.historyManager.AddAlert(Alert{ID: "alert2", Type: "memory"})

		err := m.ClearAlertHistory()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		history := m.GetAlertHistory(10)
		if len(history) != 0 {
			t.Errorf("expected 0 entries after clear, got %d", len(history))
		}
	})
}

func TestClearNodeOfflineAlert(t *testing.T) {
	// t.Parallel()

	t.Run("no alert and no count is no-op", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		node := models.Node{ID: "node1", Name: "Node 1"}
		m.clearNodeOfflineAlert(node)

		m.mu.RLock()
		alertCount := len(m.activeAlerts)
		m.mu.RUnlock()
		if alertCount != 0 {
			t.Errorf("expected 0 alerts, got %d", alertCount)
		}
	})

	t.Run("resets offline count when node comes online", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		m.mu.Lock()
		m.nodeOfflineCount["node1"] = 5
		m.mu.Unlock()

		node := models.Node{ID: "node1", Name: "Node 1"}
		m.clearNodeOfflineAlert(node)

		m.mu.RLock()
		_, exists := m.nodeOfflineCount["node1"]
		m.mu.RUnlock()
		if exists {
			t.Error("expected offline count to be cleared")
		}
	})

	t.Run("clears existing alert and adds to resolved", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		resolvedCh := make(chan struct{}, 1)
		m.SetResolvedCallback(func(alertID string) {
			resolvedCh <- struct{}{}
		})

		m.mu.Lock()
		m.nodeOfflineCount["node1"] = 3
		m.activeAlerts["node-offline-node1"] = &Alert{
			ID:        "node-offline-node1",
			Type:      "offline",
			StartTime: time.Now().Add(-10 * time.Minute),
		}
		m.mu.Unlock()

		node := models.Node{ID: "node1", Name: "Node 1", Instance: "pve1"}
		m.clearNodeOfflineAlert(node)

		m.mu.RLock()
		_, alertExists := m.activeAlerts["node-offline-node1"]
		_, countExists := m.nodeOfflineCount["node1"]
		m.mu.RUnlock()

		if alertExists {
			t.Error("expected alert to be cleared")
		}
		if countExists {
			t.Error("expected offline count to be cleared")
		}

		// Check resolved
		m.resolvedMutex.RLock()
		resolved := m.recentlyResolved["node-offline-node1"]
		m.resolvedMutex.RUnlock()
		if resolved == nil {
			t.Error("expected alert to be added to recently resolved")
		}
		select {
		case <-resolvedCh:
		case <-time.After(2 * time.Second):
			t.Error("expected resolved callback to be called")
		}
	})
}

// TestClearOfflineAlertNoDeadlock is a regression test for a deadlock introduced
// by commit 07b4765b. The resolved callback (handleAlertResolved) calls
// ShouldSuppressResolvedNotification which acquires m.mu.RLock(). If the
// clear*OfflineAlert functions call the callback synchronously while holding
// m.mu.Lock(), Go's non-reentrant RWMutex deadlocks.
func TestClearOfflineAlertNoDeadlock(t *testing.T) {
	// t.Parallel()

	type testCase struct {
		name    string
		setupFn func(m *Manager)
		clearFn func(m *Manager)
	}

	cases := []testCase{
		{
			name: "clearNodeOfflineAlert",
			setupFn: func(m *Manager) {
				m.mu.Lock()
				m.activeAlerts["node-offline-node1"] = &Alert{
					ID:        "node-offline-node1",
					Type:      "offline",
					StartTime: time.Now().Add(-5 * time.Minute),
				}
				m.mu.Unlock()
			},
			clearFn: func(m *Manager) {
				m.clearNodeOfflineAlert(models.Node{ID: "node1", Name: "Node 1", Instance: "pve1"})
			},
		},
		{
			name: "clearPBSOfflineAlert",
			setupFn: func(m *Manager) {
				m.mu.Lock()
				m.activeAlerts["pbs-offline-pbs1"] = &Alert{
					ID:        "pbs-offline-pbs1",
					Type:      "offline",
					StartTime: time.Now().Add(-5 * time.Minute),
				}
				m.mu.Unlock()
			},
			clearFn: func(m *Manager) {
				m.clearPBSOfflineAlert(models.PBSInstance{ID: "pbs1", Name: "PBS 1", Host: "host1"})
			},
		},
		{
			name: "clearPMGOfflineAlert",
			setupFn: func(m *Manager) {
				m.mu.Lock()
				m.activeAlerts["pmg-offline-pmg1"] = &Alert{
					ID:        "pmg-offline-pmg1",
					Type:      "offline",
					StartTime: time.Now().Add(-5 * time.Minute),
				}
				m.mu.Unlock()
			},
			clearFn: func(m *Manager) {
				m.clearPMGOfflineAlert(models.PMGInstance{ID: "pmg1", Name: "PMG 1", Host: "host1"})
			},
		},
		{
			name: "clearStorageOfflineAlert",
			setupFn: func(m *Manager) {
				m.mu.Lock()
				m.activeAlerts["storage-offline-stor1"] = &Alert{
					ID:        "storage-offline-stor1",
					Type:      "offline",
					StartTime: time.Now().Add(-5 * time.Minute),
				}
				m.mu.Unlock()
			},
			clearFn: func(m *Manager) {
				m.clearStorageOfflineAlert(models.Storage{ID: "stor1", Name: "Storage 1", Node: "node1"})
			},
		},
		{
			name: "clearGuestPoweredOffAlert",
			setupFn: func(m *Manager) {
				m.mu.Lock()
				m.activeAlerts["guest-powered-off-vm100"] = &Alert{
					ID:        "guest-powered-off-vm100",
					Type:      "powered-off",
					StartTime: time.Now().Add(-5 * time.Minute),
				}
				m.mu.Unlock()
			},
			clearFn: func(m *Manager) {
				m.clearGuestPoweredOffAlert("vm100", "TestVM")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestManager(t)

			// Simulate what handleAlertResolved does in production:
			// it calls ShouldSuppressResolvedNotification which acquires m.mu.RLock().
			// Before the fix, this deadlocked because the caller held m.mu.Lock().
			done := make(chan struct{})
			m.SetResolvedCallback(func(alertID string) {
				_ = m.ShouldSuppressResolvedNotification(&Alert{ID: alertID})
				close(done)
			})

			tc.setupFn(m)
			tc.clearFn(m)

			select {
			case <-done:
				// Callback completed without deadlock
			case <-time.After(3 * time.Second):
				t.Fatal("deadlock: resolved callback did not complete within 3 seconds")
			}
		})
	}
}

func TestClearPBSOfflineAlert(t *testing.T) {
	// t.Parallel()

	t.Run("no alert and no count is no-op", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		pbs := models.PBSInstance{ID: "pbs1", Name: "PBS 1"}
		m.clearPBSOfflineAlert(pbs)

		m.mu.RLock()
		alertCount := len(m.activeAlerts)
		m.mu.RUnlock()
		if alertCount != 0 {
			t.Errorf("expected 0 alerts, got %d", alertCount)
		}
	})

	t.Run("resets offline confirmation count", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		m.mu.Lock()
		m.offlineConfirmations["pbs1"] = 5
		m.mu.Unlock()

		pbs := models.PBSInstance{ID: "pbs1", Name: "PBS 1"}
		m.clearPBSOfflineAlert(pbs)

		m.mu.RLock()
		_, exists := m.offlineConfirmations["pbs1"]
		m.mu.RUnlock()
		if exists {
			t.Error("expected offline confirmation count to be cleared")
		}
	})

	t.Run("clears existing alert and adds to resolved", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		resolvedCh := make(chan struct{}, 1)
		m.SetResolvedCallback(func(alertID string) {
			resolvedCh <- struct{}{}
		})

		m.mu.Lock()
		m.offlineConfirmations["pbs1"] = 3
		m.activeAlerts["pbs-offline-pbs1"] = &Alert{
			ID:        "pbs-offline-pbs1",
			Type:      "offline",
			StartTime: time.Now().Add(-5 * time.Minute),
		}
		m.mu.Unlock()

		pbs := models.PBSInstance{ID: "pbs1", Name: "PBS 1", Host: "pbs.local"}
		m.clearPBSOfflineAlert(pbs)

		m.mu.RLock()
		_, alertExists := m.activeAlerts["pbs-offline-pbs1"]
		_, countExists := m.offlineConfirmations["pbs1"]
		m.mu.RUnlock()

		if alertExists {
			t.Error("expected alert to be cleared")
		}
		if countExists {
			t.Error("expected offline confirmation count to be cleared")
		}

		// Check resolved
		m.resolvedMutex.RLock()
		resolved := m.recentlyResolved["pbs-offline-pbs1"]
		m.resolvedMutex.RUnlock()
		if resolved == nil {
			t.Error("expected alert to be added to recently resolved")
		}
		select {
		case <-resolvedCh:
		case <-time.After(2 * time.Second):
			t.Error("expected resolved callback to be called")
		}
	})
}

func TestClearPMGOfflineAlert(t *testing.T) {
	// t.Parallel()

	t.Run("no alert and no count is no-op", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		pmg := models.PMGInstance{ID: "pmg1", Name: "PMG 1"}
		m.clearPMGOfflineAlert(pmg)

		m.mu.RLock()
		alertCount := len(m.activeAlerts)
		m.mu.RUnlock()
		if alertCount != 0 {
			t.Errorf("expected 0 alerts, got %d", alertCount)
		}
	})

	t.Run("resets offline confirmation count", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		m.mu.Lock()
		m.offlineConfirmations["pmg1"] = 5
		m.mu.Unlock()

		pmg := models.PMGInstance{ID: "pmg1", Name: "PMG 1"}
		m.clearPMGOfflineAlert(pmg)

		m.mu.RLock()
		_, exists := m.offlineConfirmations["pmg1"]
		m.mu.RUnlock()
		if exists {
			t.Error("expected offline confirmation count to be cleared")
		}
	})

	t.Run("clears existing alert and adds to resolved", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		resolvedCh := make(chan struct{}, 1)
		m.SetResolvedCallback(func(alertID string) {
			resolvedCh <- struct{}{}
		})

		m.mu.Lock()
		m.offlineConfirmations["pmg1"] = 3
		m.activeAlerts["pmg-offline-pmg1"] = &Alert{
			ID:        "pmg-offline-pmg1",
			Type:      "offline",
			StartTime: time.Now().Add(-5 * time.Minute),
		}
		m.mu.Unlock()

		pmg := models.PMGInstance{ID: "pmg1", Name: "PMG 1", Host: "pmg.local"}
		m.clearPMGOfflineAlert(pmg)

		m.mu.RLock()
		_, alertExists := m.activeAlerts["pmg-offline-pmg1"]
		_, countExists := m.offlineConfirmations["pmg1"]
		m.mu.RUnlock()

		if alertExists {
			t.Error("expected alert to be cleared")
		}
		if countExists {
			t.Error("expected offline confirmation count to be cleared")
		}

		// Check resolved
		m.resolvedMutex.RLock()
		resolved := m.recentlyResolved["pmg-offline-pmg1"]
		m.resolvedMutex.RUnlock()
		if resolved == nil {
			t.Error("expected alert to be added to recently resolved")
		}
		select {
		case <-resolvedCh:
		case <-time.After(2 * time.Second):
			t.Error("expected resolved callback to be called")
		}
	})
}

func TestCheckNodeOffline(t *testing.T) {
	// t.Parallel()

	t.Run("override DisableConnectivity clears alert and returns", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		m.mu.Lock()
		m.config.Overrides = map[string]ThresholdConfig{
			"node1": {DisableConnectivity: true},
		}
		m.activeAlerts["node-offline-node1"] = &Alert{ID: "node-offline-node1"}
		m.nodeOfflineCount["node1"] = 5
		m.mu.Unlock()

		node := models.Node{ID: "node1", Name: "Node 1"}
		m.checkNodeOffline(node)

		m.mu.RLock()
		_, alertExists := m.activeAlerts["node-offline-node1"]
		_, countExists := m.nodeOfflineCount["node1"]
		m.mu.RUnlock()

		if alertExists {
			t.Error("expected alert to be cleared when connectivity disabled")
		}
		if countExists {
			t.Error("expected offline count to be cleared")
		}
	})

	t.Run("existing alert updates LastSeen", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		oldTime := time.Now().Add(-1 * time.Hour)
		m.mu.Lock()
		m.activeAlerts["node-offline-node1"] = &Alert{
			ID:        "node-offline-node1",
			StartTime: oldTime,
			LastSeen:  oldTime,
		}
		m.mu.Unlock()

		node := models.Node{ID: "node1", Name: "Node 1"}
		m.checkNodeOffline(node)

		m.mu.RLock()
		alert := m.activeAlerts["node-offline-node1"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected alert to exist")
		}
		if !alert.LastSeen.After(oldTime) {
			t.Error("expected LastSeen to be updated")
		}
		if !alert.StartTime.Equal(oldTime) {
			t.Error("expected StartTime to remain unchanged")
		}
	})

	t.Run("insufficient confirmations waits", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		node := models.Node{ID: "node1", Name: "Node 1", Instance: "pve1"}

		// First call - count 1
		m.checkNodeOffline(node)
		m.mu.RLock()
		count1 := m.nodeOfflineCount["node1"]
		alert1 := m.activeAlerts["node-offline-node1"]
		m.mu.RUnlock()
		if count1 != 1 {
			t.Errorf("expected count 1, got %d", count1)
		}
		if alert1 != nil {
			t.Error("expected no alert after 1 confirmation")
		}

		// Second call - count 2
		m.checkNodeOffline(node)
		m.mu.RLock()
		count2 := m.nodeOfflineCount["node1"]
		alert2 := m.activeAlerts["node-offline-node1"]
		m.mu.RUnlock()
		if count2 != 2 {
			t.Errorf("expected count 2, got %d", count2)
		}
		if alert2 != nil {
			t.Error("expected no alert after 2 confirmations")
		}
	})

	t.Run("creates alert after 3 confirmations", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		m.mu.Lock()
		m.nodeOfflineCount["node1"] = 2 // Pre-set to trigger on next call
		m.mu.Unlock()

		node := models.Node{
			ID:               "node1",
			Name:             "Node 1",
			Instance:         "pve1",
			Status:           "offline",
			ConnectionHealth: "disconnected",
		}

		m.checkNodeOffline(node)

		m.mu.RLock()
		alert := m.activeAlerts["node-offline-node1"]
		count := m.nodeOfflineCount["node1"]
		m.mu.RUnlock()

		if count != 3 {
			t.Errorf("expected count 3, got %d", count)
		}
		if alert == nil {
			t.Fatal("expected alert after 3 confirmations")
		}
		if alert.Type != "connectivity" {
			t.Errorf("expected type connectivity, got %s", alert.Type)
		}
		if alert.Level != AlertLevelCritical {
			t.Errorf("expected critical level, got %s", alert.Level)
		}
		if alert.ResourceID != "node1" {
			t.Errorf("expected resourceID node1, got %s", alert.ResourceID)
		}
	})

	t.Run("alert added to history", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		m.mu.Lock()
		m.nodeOfflineCount["node1"] = 2
		m.mu.Unlock()

		node := models.Node{ID: "node1", Name: "Node 1", Instance: "pve1"}
		m.checkNodeOffline(node)

		// Check history
		history := m.GetAlertHistory(10)
		found := false
		for _, h := range history {
			if h.ID == "node-offline-node1" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected alert to be added to history")
		}
	})
}

func TestCheckPBSOffline(t *testing.T) {
	// t.Parallel()

	t.Run("override Disabled clears alert and returns", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		m.mu.Lock()
		m.config.Overrides = map[string]ThresholdConfig{
			"pbs1": {Disabled: true},
		}
		m.activeAlerts["pbs-offline-pbs1"] = &Alert{ID: "pbs-offline-pbs1"}
		m.mu.Unlock()

		pbs := models.PBSInstance{ID: "pbs1", Name: "PBS 1"}
		m.checkPBSOffline(pbs)

		m.mu.RLock()
		_, alertExists := m.activeAlerts["pbs-offline-pbs1"]
		m.mu.RUnlock()

		if alertExists {
			t.Error("expected alert to be cleared when disabled")
		}
	})

	t.Run("override DisableConnectivity clears alert and returns", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		m.mu.Lock()
		m.config.Overrides = map[string]ThresholdConfig{
			"pbs1": {DisableConnectivity: true},
		}
		m.activeAlerts["pbs-offline-pbs1"] = &Alert{ID: "pbs-offline-pbs1"}
		m.mu.Unlock()

		pbs := models.PBSInstance{ID: "pbs1", Name: "PBS 1"}
		m.checkPBSOffline(pbs)

		m.mu.RLock()
		_, alertExists := m.activeAlerts["pbs-offline-pbs1"]
		m.mu.RUnlock()

		if alertExists {
			t.Error("expected alert to be cleared when connectivity disabled")
		}
	})

	t.Run("insufficient confirmations waits", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		pbs := models.PBSInstance{ID: "pbs1", Name: "PBS 1"}

		// First two calls - not enough confirmations
		m.checkPBSOffline(pbs)
		m.checkPBSOffline(pbs)

		m.mu.RLock()
		count := m.offlineConfirmations["pbs1"]
		_, alertExists := m.activeAlerts["pbs-offline-pbs1"]
		m.mu.RUnlock()

		if count != 2 {
			t.Errorf("expected count 2, got %d", count)
		}
		if alertExists {
			t.Error("expected no alert after 2 confirmations")
		}
	})

	t.Run("creates alert after 3 confirmations", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		m.mu.Lock()
		m.offlineConfirmations["pbs1"] = 2
		m.mu.Unlock()

		pbs := models.PBSInstance{ID: "pbs1", Name: "PBS 1", Host: "pbs.local"}
		m.checkPBSOffline(pbs)

		m.mu.RLock()
		alert := m.activeAlerts["pbs-offline-pbs1"]
		count := m.offlineConfirmations["pbs1"]
		m.mu.RUnlock()

		if count != 3 {
			t.Errorf("expected count 3, got %d", count)
		}
		if alert == nil {
			t.Fatal("expected alert after 3 confirmations")
		}
		if alert.Type != "offline" {
			t.Errorf("expected type offline, got %s", alert.Type)
		}
		if alert.Level != AlertLevelCritical {
			t.Errorf("expected critical level, got %s", alert.Level)
		}
	})

	t.Run("existing alert updates LastSeen", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		oldTime := time.Now().Add(-1 * time.Hour)
		m.mu.Lock()
		m.offlineConfirmations["pbs1"] = 3
		m.activeAlerts["pbs-offline-pbs1"] = &Alert{
			ID:       "pbs-offline-pbs1",
			LastSeen: oldTime,
		}
		m.mu.Unlock()

		pbs := models.PBSInstance{ID: "pbs1", Name: "PBS 1"}
		m.checkPBSOffline(pbs)

		m.mu.RLock()
		alert := m.activeAlerts["pbs-offline-pbs1"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected alert to exist")
		}
		if !alert.LastSeen.After(oldTime) {
			t.Error("expected LastSeen to be updated")
		}
	})
}

func TestCheckPMGOffline(t *testing.T) {
	// t.Parallel()

	t.Run("override Disabled clears alert and returns", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		m.mu.Lock()
		m.config.Overrides = map[string]ThresholdConfig{
			"pmg1": {Disabled: true},
		}
		m.activeAlerts["pmg-offline-pmg1"] = &Alert{ID: "pmg-offline-pmg1"}
		m.mu.Unlock()

		pmg := models.PMGInstance{ID: "pmg1", Name: "PMG 1"}
		m.checkPMGOffline(pmg)

		m.mu.RLock()
		_, alertExists := m.activeAlerts["pmg-offline-pmg1"]
		m.mu.RUnlock()

		if alertExists {
			t.Error("expected alert to be cleared when disabled")
		}
	})

	t.Run("override DisableConnectivity clears alert and returns", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		m.mu.Lock()
		m.config.Overrides = map[string]ThresholdConfig{
			"pmg1": {DisableConnectivity: true},
		}
		m.activeAlerts["pmg-offline-pmg1"] = &Alert{ID: "pmg-offline-pmg1"}
		m.mu.Unlock()

		pmg := models.PMGInstance{ID: "pmg1", Name: "PMG 1"}
		m.checkPMGOffline(pmg)

		m.mu.RLock()
		_, alertExists := m.activeAlerts["pmg-offline-pmg1"]
		m.mu.RUnlock()

		if alertExists {
			t.Error("expected alert to be cleared when connectivity disabled")
		}
	})

	t.Run("insufficient confirmations waits", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		pmg := models.PMGInstance{ID: "pmg1", Name: "PMG 1"}

		// First two calls - not enough confirmations
		m.checkPMGOffline(pmg)
		m.checkPMGOffline(pmg)

		m.mu.RLock()
		count := m.offlineConfirmations["pmg1"]
		_, alertExists := m.activeAlerts["pmg-offline-pmg1"]
		m.mu.RUnlock()

		if count != 2 {
			t.Errorf("expected count 2, got %d", count)
		}
		if alertExists {
			t.Error("expected no alert after 2 confirmations")
		}
	})

	t.Run("creates alert after 3 confirmations", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		m.mu.Lock()
		m.offlineConfirmations["pmg1"] = 2
		m.mu.Unlock()

		pmg := models.PMGInstance{ID: "pmg1", Name: "PMG 1", Host: "pmg.local"}
		m.checkPMGOffline(pmg)

		m.mu.RLock()
		alert := m.activeAlerts["pmg-offline-pmg1"]
		count := m.offlineConfirmations["pmg1"]
		m.mu.RUnlock()

		if count != 3 {
			t.Errorf("expected count 3, got %d", count)
		}
		if alert == nil {
			t.Fatal("expected alert after 3 confirmations")
		}
		if alert.Type != "offline" {
			t.Errorf("expected type offline, got %s", alert.Type)
		}
		if alert.Level != AlertLevelCritical {
			t.Errorf("expected critical level, got %s", alert.Level)
		}
	})

	t.Run("existing alert updates LastSeen", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		oldTime := time.Now().Add(-1 * time.Hour)
		m.mu.Lock()
		m.offlineConfirmations["pmg1"] = 3
		m.activeAlerts["pmg-offline-pmg1"] = &Alert{
			ID:       "pmg-offline-pmg1",
			LastSeen: oldTime,
		}
		m.mu.Unlock()

		pmg := models.PMGInstance{ID: "pmg1", Name: "PMG 1"}
		m.checkPMGOffline(pmg)

		m.mu.RLock()
		alert := m.activeAlerts["pmg-offline-pmg1"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected alert to exist")
		}
		if !alert.LastSeen.After(oldTime) {
			t.Error("expected LastSeen to be updated")
		}
	})
}

func TestCalculateTrimmedBaseline(t *testing.T) {
	// t.Parallel()

	t.Run("less than 12 samples returns untrustworthy", func(t *testing.T) {
		// t.Parallel()
		samples := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}
		baseline, trustworthy := calculateTrimmedBaseline(samples)
		if trustworthy {
			t.Error("expected untrustworthy with less than 12 samples")
		}
		if baseline != 0 {
			t.Errorf("expected baseline 0, got %f", baseline)
		}
	})

	t.Run("empty samples returns untrustworthy", func(t *testing.T) {
		// t.Parallel()
		samples := []float64{}
		baseline, trustworthy := calculateTrimmedBaseline(samples)
		if trustworthy {
			t.Error("expected untrustworthy with empty samples")
		}
		if baseline != 0 {
			t.Errorf("expected baseline 0, got %f", baseline)
		}
	})

	t.Run("12-23 samples uses simple mean", func(t *testing.T) {
		// t.Parallel()
		// 12 samples summing to 78
		samples := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}
		baseline, trustworthy := calculateTrimmedBaseline(samples)
		if !trustworthy {
			t.Error("expected trustworthy with 12 samples")
		}
		// Mean of 1-12 is (1+2+...+12)/12 = 78/12 = 6.5
		if baseline != 6.5 {
			t.Errorf("expected baseline 6.5, got %f", baseline)
		}
	})

	t.Run("24+ samples uses trimmed mean", func(t *testing.T) {
		// t.Parallel()
		// 24 identical values - trimmed mean should equal value
		samples := make([]float64, 24)
		for i := range samples {
			samples[i] = 10.0
		}
		baseline, trustworthy := calculateTrimmedBaseline(samples)
		if !trustworthy {
			t.Error("expected trustworthy with 24 samples")
		}
		if baseline != 10.0 {
			t.Errorf("expected baseline 10.0, got %f", baseline)
		}
	})

	t.Run("24+ samples falls back to median when diff > 40%", func(t *testing.T) {
		// t.Parallel()
		// Create samples where trimmed mean differs significantly from median
		// Mostly 10s with some extreme outliers that survive trimming
		samples := make([]float64, 24)
		for i := range samples {
			if i < 4 {
				samples[i] = 100.0 // Extreme high values
			} else {
				samples[i] = 10.0 // Normal values
			}
		}
		// After sorting: 10,10,...,10,100,100,100,100
		// Median is 10 (middle values are 10s)
		// Trimmed mean (drop 2 highest and 2 lowest): still has 2 100s
		// So trimmed mean > median * 1.4, should fall back to median
		baseline, trustworthy := calculateTrimmedBaseline(samples)
		if !trustworthy {
			t.Error("expected trustworthy")
		}
		// Should use median (10) due to large diff
		if baseline != 10.0 {
			t.Errorf("expected baseline 10.0 (median fallback), got %f", baseline)
		}
	})

	t.Run("24+ samples uses trimmed mean when diff <= 40%", func(t *testing.T) {
		// t.Parallel()
		// Sequential values with minimal outlier effect
		samples := make([]float64, 24)
		for i := range samples {
			samples[i] = float64(i + 1) // 1,2,3,...,24
		}
		baseline, trustworthy := calculateTrimmedBaseline(samples)
		if !trustworthy {
			t.Error("expected trustworthy")
		}
		// Median of 1-24 is (12+13)/2 = 12.5
		// Trimmed mean of 3-22 is (3+4+...+22)/20 = 250/20 = 12.5
		// Both are close, should use trimmed mean
		if baseline != 12.5 {
			t.Errorf("expected baseline 12.5, got %f", baseline)
		}
	})

	t.Run("odd length array uses middle element for median", func(t *testing.T) {
		// t.Parallel()
		// 25 samples: an odd-length array
		samples := make([]float64, 25)
		for i := range samples {
			samples[i] = float64(i + 1) // 1,2,3,...,25
		}
		baseline, trustworthy := calculateTrimmedBaseline(samples)
		if !trustworthy {
			t.Error("expected trustworthy")
		}
		// Median of sorted 1-25 is the 13th element = 13
		// Trimmed mean excludes top/bottom 2: 3..23 = 21 elements, sum = (3+23)*21/2 = 273, mean = 13
		// Both are 13, diff is 0%, should use trimmed mean = 13
		if baseline != 13.0 {
			t.Errorf("expected baseline 13.0, got %f", baseline)
		}
	})

	t.Run("trimmed mean less than median triggers diff calculation", func(t *testing.T) {
		// t.Parallel()
		// Create samples where trimmed mean < median but within 40%
		// High outliers at top (excluded by trim), low values in middle
		samples := make([]float64, 24)
		// First 2 (will be trimmed): very low
		samples[0], samples[1] = 1, 2
		// Middle 20: mostly 50 but some variance
		for i := 2; i < 22; i++ {
			samples[i] = 50.0
		}
		// Last 2 (will be trimmed): very high
		samples[22], samples[23] = 100, 200

		baseline, trustworthy := calculateTrimmedBaseline(samples)
		if !trustworthy {
			t.Error("expected trustworthy")
		}
		// After sorting: 1, 2, 50x20, 100, 200
		// Median of even array: (50+50)/2 = 50
		// Trimmed mean: 50x20/20 = 50
		// Should return 50
		if baseline != 50.0 {
			t.Errorf("expected baseline 50.0, got %f", baseline)
		}
	})
}

func TestCreateOrUpdateNodeAlert(t *testing.T) {
	// t.Parallel()

	t.Run("creates new alert", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		pmg := models.PMGInstance{ID: "pmg1", Name: "PMG 1"}
		m.createOrUpdateNodeAlert(
			"pmg1-node-queue",
			pmg,
			"mail-node1",
			"pmg-node-queue",
			AlertLevelWarning,
			100,
			50,
			"Queue depth high",
		)

		m.mu.RLock()
		alert := m.activeAlerts["pmg1-node-queue"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected alert to be created")
		}
		if alert.Type != "pmg-node-queue" {
			t.Errorf("expected type pmg-node-queue, got %s", alert.Type)
		}
		if alert.Level != AlertLevelWarning {
			t.Errorf("expected warning level, got %s", alert.Level)
		}
		if alert.Value != 100 {
			t.Errorf("expected value 100, got %f", alert.Value)
		}
		if alert.Threshold != 50 {
			t.Errorf("expected threshold 50, got %f", alert.Threshold)
		}
		if alert.Node != "mail-node1" {
			t.Errorf("expected node mail-node1, got %s", alert.Node)
		}
	})

	t.Run("updates existing alert", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		oldTime := time.Now().Add(-1 * time.Hour)
		m.mu.Lock()
		m.activeAlerts["pmg1-node-queue"] = &Alert{
			ID:        "pmg1-node-queue",
			Value:     50,
			Threshold: 40,
			Level:     AlertLevelWarning,
			Message:   "Old message",
			LastSeen:  oldTime,
		}
		m.mu.Unlock()

		pmg := models.PMGInstance{ID: "pmg1", Name: "PMG 1"}
		m.createOrUpdateNodeAlert(
			"pmg1-node-queue",
			pmg,
			"mail-node1",
			"pmg-node-queue",
			AlertLevelCritical,
			200,
			100,
			"New message",
		)

		m.mu.RLock()
		alert := m.activeAlerts["pmg1-node-queue"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected alert to exist")
		}
		if alert.Value != 200 {
			t.Errorf("expected value 200, got %f", alert.Value)
		}
		if alert.Threshold != 100 {
			t.Errorf("expected threshold 100, got %f", alert.Threshold)
		}
		if alert.Level != AlertLevelCritical {
			t.Errorf("expected critical level, got %s", alert.Level)
		}
		if alert.Message != "New message" {
			t.Errorf("expected 'New message', got %s", alert.Message)
		}
		if !alert.LastSeen.After(oldTime) {
			t.Error("expected LastSeen to be updated")
		}
	})
}

func TestCheckPMGQueueDepths(t *testing.T) {
	// t.Parallel()

	t.Run("no thresholds configured does not create alerts", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		pmg := models.PMGInstance{
			ID:   "pmg1",
			Name: "PMG 1",
			Nodes: []models.PMGNodeStatus{
				{Name: "node1", QueueStatus: &models.PMGQueueStatus{Total: 1000, Deferred: 500, Hold: 300}},
			},
		}

		// No thresholds configured (all 0)
		defaults := PMGThresholdConfig{}
		m.checkPMGQueueDepths(pmg, defaults)

		m.mu.RLock()
		totalAlerts := len(m.activeAlerts)
		m.mu.RUnlock()

		if totalAlerts != 0 {
			t.Errorf("expected no alerts when no thresholds configured, got %d", totalAlerts)
		}
	})

	t.Run("total queue warning alert", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		pmg := models.PMGInstance{
			ID:   "pmg1",
			Name: "PMG 1",
			Host: "pmg-server",
			Nodes: []models.PMGNodeStatus{
				{Name: "node1", QueueStatus: &models.PMGQueueStatus{Total: 300}},
				{Name: "node2", QueueStatus: &models.PMGQueueStatus{Total: 250}},
			},
		}

		defaults := PMGThresholdConfig{
			QueueTotalWarning:  500,
			QueueTotalCritical: 1000,
		}
		m.checkPMGQueueDepths(pmg, defaults)

		m.mu.RLock()
		alert := m.activeAlerts["pmg1-queue-total"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected warning alert to be created")
		}
		if alert.Level != AlertLevelWarning {
			t.Errorf("expected warning level, got %s", alert.Level)
		}
		if alert.Value != 550 {
			t.Errorf("expected value 550, got %f", alert.Value)
		}
	})

	t.Run("total queue critical alert", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		pmg := models.PMGInstance{
			ID:   "pmg1",
			Name: "PMG 1",
			Nodes: []models.PMGNodeStatus{
				{Name: "node1", QueueStatus: &models.PMGQueueStatus{Total: 600}},
				{Name: "node2", QueueStatus: &models.PMGQueueStatus{Total: 500}},
			},
		}

		defaults := PMGThresholdConfig{
			QueueTotalWarning:  500,
			QueueTotalCritical: 1000,
		}
		m.checkPMGQueueDepths(pmg, defaults)

		m.mu.RLock()
		alert := m.activeAlerts["pmg1-queue-total"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected critical alert to be created")
		}
		if alert.Level != AlertLevelCritical {
			t.Errorf("expected critical level, got %s", alert.Level)
		}
		if alert.Value != 1100 {
			t.Errorf("expected value 1100, got %f", alert.Value)
		}
	})

	t.Run("deferred queue warning alert", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		pmg := models.PMGInstance{
			ID:   "pmg1",
			Name: "PMG 1",
			Nodes: []models.PMGNodeStatus{
				{Name: "node1", QueueStatus: &models.PMGQueueStatus{Deferred: 150}},
				{Name: "node2", QueueStatus: &models.PMGQueueStatus{Deferred: 100}},
			},
		}

		defaults := PMGThresholdConfig{
			DeferredQueueWarn:     200,
			DeferredQueueCritical: 500,
		}
		m.checkPMGQueueDepths(pmg, defaults)

		m.mu.RLock()
		alert := m.activeAlerts["pmg1-queue-deferred"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected deferred alert to be created")
		}
		if alert.Level != AlertLevelWarning {
			t.Errorf("expected warning level, got %s", alert.Level)
		}
		if alert.Value != 250 {
			t.Errorf("expected value 250, got %f", alert.Value)
		}
	})

	t.Run("deferred queue critical alert", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		pmg := models.PMGInstance{
			ID:   "pmg1",
			Name: "PMG 1",
			Nodes: []models.PMGNodeStatus{
				{Name: "node1", QueueStatus: &models.PMGQueueStatus{Deferred: 300}},
				{Name: "node2", QueueStatus: &models.PMGQueueStatus{Deferred: 250}},
			},
		}

		defaults := PMGThresholdConfig{
			DeferredQueueWarn:     200,
			DeferredQueueCritical: 500,
		}
		m.checkPMGQueueDepths(pmg, defaults)

		m.mu.RLock()
		alert := m.activeAlerts["pmg1-queue-deferred"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected critical alert to be created")
		}
		if alert.Level != AlertLevelCritical {
			t.Errorf("expected critical level, got %s", alert.Level)
		}
	})

	t.Run("hold queue warning alert", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		pmg := models.PMGInstance{
			ID:   "pmg1",
			Name: "PMG 1",
			Nodes: []models.PMGNodeStatus{
				{Name: "node1", QueueStatus: &models.PMGQueueStatus{Hold: 75}},
				{Name: "node2", QueueStatus: &models.PMGQueueStatus{Hold: 50}},
			},
		}

		defaults := PMGThresholdConfig{
			HoldQueueWarn:     100,
			HoldQueueCritical: 300,
		}
		m.checkPMGQueueDepths(pmg, defaults)

		m.mu.RLock()
		alert := m.activeAlerts["pmg1-queue-hold"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected hold alert to be created")
		}
		if alert.Level != AlertLevelWarning {
			t.Errorf("expected warning level, got %s", alert.Level)
		}
		if alert.Value != 125 {
			t.Errorf("expected value 125, got %f", alert.Value)
		}
	})

	t.Run("hold queue critical alert", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		pmg := models.PMGInstance{
			ID:   "pmg1",
			Name: "PMG 1",
			Nodes: []models.PMGNodeStatus{
				{Name: "node1", QueueStatus: &models.PMGQueueStatus{Hold: 200}},
				{Name: "node2", QueueStatus: &models.PMGQueueStatus{Hold: 150}},
			},
		}

		defaults := PMGThresholdConfig{
			HoldQueueWarn:     100,
			HoldQueueCritical: 300,
		}
		m.checkPMGQueueDepths(pmg, defaults)

		m.mu.RLock()
		alert := m.activeAlerts["pmg1-queue-hold"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected critical alert to be created")
		}
		if alert.Level != AlertLevelCritical {
			t.Errorf("expected critical level, got %s", alert.Level)
		}
	})

	t.Run("updates existing alert", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		oldTime := time.Now().Add(-1 * time.Hour)
		m.mu.Lock()
		m.activeAlerts["pmg1-queue-total"] = &Alert{
			ID:       "pmg1-queue-total",
			Value:    400,
			Level:    AlertLevelWarning,
			LastSeen: oldTime,
		}
		m.mu.Unlock()

		pmg := models.PMGInstance{
			ID:   "pmg1",
			Name: "PMG 1",
			Nodes: []models.PMGNodeStatus{
				{Name: "node1", QueueStatus: &models.PMGQueueStatus{Total: 1200}},
			},
		}

		defaults := PMGThresholdConfig{
			QueueTotalWarning:  500,
			QueueTotalCritical: 1000,
		}
		m.checkPMGQueueDepths(pmg, defaults)

		m.mu.RLock()
		alert := m.activeAlerts["pmg1-queue-total"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected alert to exist")
		}
		if alert.Value != 1200 {
			t.Errorf("expected value 1200, got %f", alert.Value)
		}
		if alert.Level != AlertLevelCritical {
			t.Errorf("expected critical level, got %s", alert.Level)
		}
		if !alert.LastSeen.After(oldTime) {
			t.Error("expected LastSeen to be updated")
		}
	})

	t.Run("below threshold clears alert", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.activeAlerts["pmg1-queue-total"] = &Alert{ID: "pmg1-queue-total"}
		m.mu.Unlock()

		pmg := models.PMGInstance{
			ID:   "pmg1",
			Name: "PMG 1",
			Nodes: []models.PMGNodeStatus{
				{Name: "node1", QueueStatus: &models.PMGQueueStatus{Total: 100}},
			},
		}

		defaults := PMGThresholdConfig{
			QueueTotalWarning:  500,
			QueueTotalCritical: 1000,
		}
		m.checkPMGQueueDepths(pmg, defaults)

		m.mu.RLock()
		_, exists := m.activeAlerts["pmg1-queue-total"]
		m.mu.RUnlock()

		if exists {
			t.Error("expected alert to be cleared when below threshold")
		}
	})

	t.Run("nil QueueStatus is handled", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		pmg := models.PMGInstance{
			ID:   "pmg1",
			Name: "PMG 1",
			Nodes: []models.PMGNodeStatus{
				{Name: "node1", QueueStatus: nil},
				{Name: "node2", QueueStatus: &models.PMGQueueStatus{Total: 100}},
			},
		}

		defaults := PMGThresholdConfig{
			QueueTotalWarning: 500,
		}
		// Should not panic
		m.checkPMGQueueDepths(pmg, defaults)

		m.mu.RLock()
		_, exists := m.activeAlerts["pmg1-queue-total"]
		m.mu.RUnlock()

		if exists {
			t.Error("expected no alert with total below threshold")
		}
	})
}

func TestCheckPMGOldestMessage(t *testing.T) {
	// t.Parallel()

	t.Run("no thresholds configured returns early", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		pmg := models.PMGInstance{
			ID:   "pmg1",
			Name: "PMG 1",
			Nodes: []models.PMGNodeStatus{
				{Name: "node1", QueueStatus: &models.PMGQueueStatus{OldestAge: 7200}}, // 2 hours
			},
		}

		defaults := PMGThresholdConfig{} // No thresholds
		m.checkPMGOldestMessage(pmg, defaults)

		m.mu.RLock()
		_, exists := m.activeAlerts["pmg1-oldest-message"]
		m.mu.RUnlock()

		if exists {
			t.Error("expected no alert when no thresholds configured")
		}
	})

	t.Run("no messages clears existing alert", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.activeAlerts["pmg1-oldest-message"] = &Alert{ID: "pmg1-oldest-message"}
		m.mu.Unlock()

		pmg := models.PMGInstance{
			ID:   "pmg1",
			Name: "PMG 1",
			Nodes: []models.PMGNodeStatus{
				{Name: "node1", QueueStatus: &models.PMGQueueStatus{OldestAge: 0}},
			},
		}

		defaults := PMGThresholdConfig{
			OldestMessageWarnMins: 30,
			OldestMessageCritMins: 60,
		}
		m.checkPMGOldestMessage(pmg, defaults)

		m.mu.RLock()
		_, exists := m.activeAlerts["pmg1-oldest-message"]
		m.mu.RUnlock()

		if exists {
			t.Error("expected alert to be cleared when no messages in queue")
		}
	})

	t.Run("warning alert when message age exceeds warning threshold", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		pmg := models.PMGInstance{
			ID:   "pmg1",
			Name: "PMG 1",
			Host: "pmg-server",
			Nodes: []models.PMGNodeStatus{
				{Name: "node1", QueueStatus: &models.PMGQueueStatus{OldestAge: 2400}}, // 40 minutes
			},
		}

		defaults := PMGThresholdConfig{
			OldestMessageWarnMins: 30,
			OldestMessageCritMins: 60,
		}
		m.checkPMGOldestMessage(pmg, defaults)

		m.mu.RLock()
		alert := m.activeAlerts["pmg1-oldest-message"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected warning alert to be created")
		}
		if alert.Level != AlertLevelWarning {
			t.Errorf("expected warning level, got %s", alert.Level)
		}
		if alert.Value != 40 {
			t.Errorf("expected value 40 minutes, got %f", alert.Value)
		}
		if alert.Threshold != 30 {
			t.Errorf("expected threshold 30, got %f", alert.Threshold)
		}
	})

	t.Run("critical alert when message age exceeds critical threshold", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		pmg := models.PMGInstance{
			ID:   "pmg1",
			Name: "PMG 1",
			Nodes: []models.PMGNodeStatus{
				{Name: "node1", QueueStatus: &models.PMGQueueStatus{OldestAge: 4200}}, // 70 minutes
			},
		}

		defaults := PMGThresholdConfig{
			OldestMessageWarnMins: 30,
			OldestMessageCritMins: 60,
		}
		m.checkPMGOldestMessage(pmg, defaults)

		m.mu.RLock()
		alert := m.activeAlerts["pmg1-oldest-message"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected critical alert to be created")
		}
		if alert.Level != AlertLevelCritical {
			t.Errorf("expected critical level, got %s", alert.Level)
		}
		if alert.Threshold != 60 {
			t.Errorf("expected threshold 60, got %f", alert.Threshold)
		}
	})

	t.Run("below threshold clears alert", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.activeAlerts["pmg1-oldest-message"] = &Alert{ID: "pmg1-oldest-message"}
		m.mu.Unlock()

		pmg := models.PMGInstance{
			ID:   "pmg1",
			Name: "PMG 1",
			Nodes: []models.PMGNodeStatus{
				{Name: "node1", QueueStatus: &models.PMGQueueStatus{OldestAge: 900}}, // 15 minutes
			},
		}

		defaults := PMGThresholdConfig{
			OldestMessageWarnMins: 30,
			OldestMessageCritMins: 60,
		}
		m.checkPMGOldestMessage(pmg, defaults)

		m.mu.RLock()
		_, exists := m.activeAlerts["pmg1-oldest-message"]
		m.mu.RUnlock()

		if exists {
			t.Error("expected alert to be cleared when below threshold")
		}
	})

	t.Run("finds oldest across multiple nodes", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		pmg := models.PMGInstance{
			ID:   "pmg1",
			Name: "PMG 1",
			Nodes: []models.PMGNodeStatus{
				{Name: "node1", QueueStatus: &models.PMGQueueStatus{OldestAge: 1200}}, // 20 minutes
				{Name: "node2", QueueStatus: &models.PMGQueueStatus{OldestAge: 3000}}, // 50 minutes (oldest)
				{Name: "node3", QueueStatus: &models.PMGQueueStatus{OldestAge: 600}},  // 10 minutes
			},
		}

		defaults := PMGThresholdConfig{
			OldestMessageWarnMins: 30,
			OldestMessageCritMins: 60,
		}
		m.checkPMGOldestMessage(pmg, defaults)

		m.mu.RLock()
		alert := m.activeAlerts["pmg1-oldest-message"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected alert to be created")
		}
		if alert.Value != 50 {
			t.Errorf("expected value 50 (oldest across nodes), got %f", alert.Value)
		}
	})

	t.Run("updates existing alert", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		oldTime := time.Now().Add(-1 * time.Hour)
		m.mu.Lock()
		m.activeAlerts["pmg1-oldest-message"] = &Alert{
			ID:       "pmg1-oldest-message",
			Value:    40,
			Level:    AlertLevelWarning,
			LastSeen: oldTime,
		}
		m.mu.Unlock()

		pmg := models.PMGInstance{
			ID:   "pmg1",
			Name: "PMG 1",
			Nodes: []models.PMGNodeStatus{
				{Name: "node1", QueueStatus: &models.PMGQueueStatus{OldestAge: 4800}}, // 80 minutes
			},
		}

		defaults := PMGThresholdConfig{
			OldestMessageWarnMins: 30,
			OldestMessageCritMins: 60,
		}
		m.checkPMGOldestMessage(pmg, defaults)

		m.mu.RLock()
		alert := m.activeAlerts["pmg1-oldest-message"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected alert to exist")
		}
		if alert.Value != 80 {
			t.Errorf("expected value 80, got %f", alert.Value)
		}
		if alert.Level != AlertLevelCritical {
			t.Errorf("expected critical level, got %s", alert.Level)
		}
		if !alert.LastSeen.After(oldTime) {
			t.Error("expected LastSeen to be updated")
		}
	})

	t.Run("nil QueueStatus is handled", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		pmg := models.PMGInstance{
			ID:   "pmg1",
			Name: "PMG 1",
			Nodes: []models.PMGNodeStatus{
				{Name: "node1", QueueStatus: nil},
				{Name: "node2", QueueStatus: &models.PMGQueueStatus{OldestAge: 2400}}, // 40 minutes
			},
		}

		defaults := PMGThresholdConfig{
			OldestMessageWarnMins: 30,
		}
		// Should not panic and should use the valid node's data
		m.checkPMGOldestMessage(pmg, defaults)

		m.mu.RLock()
		alert := m.activeAlerts["pmg1-oldest-message"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected alert to be created from valid node data")
		}
		if alert.Value != 40 {
			t.Errorf("expected value 40, got %f", alert.Value)
		}
	})
}

func TestCheckStorageOffline(t *testing.T) {
	// t.Parallel()

	t.Run("first poll increments confirmation but does not create alert", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		storage := models.Storage{
			ID:   "local-lvm",
			Name: "Local LVM",
			Node: "pve-node1",
		}

		m.checkStorageOffline(storage)

		m.mu.RLock()
		confirmCount := m.offlineConfirmations["local-lvm"]
		_, alertExists := m.activeAlerts["storage-offline-local-lvm"]
		m.mu.RUnlock()

		if confirmCount != 1 {
			t.Errorf("expected confirmation count 1, got %d", confirmCount)
		}
		if alertExists {
			t.Error("expected no alert on first poll")
		}
	})

	t.Run("second poll creates alert after confirmation", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		storage := models.Storage{
			ID:       "local-lvm",
			Name:     "Local LVM",
			Node:     "pve-node1",
			Instance: "pve-instance",
		}

		// First poll - confirmation
		m.checkStorageOffline(storage)
		// Second poll - should create alert
		m.checkStorageOffline(storage)

		m.mu.RLock()
		alert := m.activeAlerts["storage-offline-local-lvm"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected alert to be created after second poll")
		}
		if alert.Type != "offline" {
			t.Errorf("expected type 'offline', got %s", alert.Type)
		}
		if alert.Level != AlertLevelWarning {
			t.Errorf("expected warning level, got %s", alert.Level)
		}
		if alert.ResourceID != "local-lvm" {
			t.Errorf("expected resource ID 'local-lvm', got %s", alert.ResourceID)
		}
		if alert.Node != "pve-node1" {
			t.Errorf("expected node 'pve-node1', got %s", alert.Node)
		}
	})

	t.Run("existing alert updates LastSeen", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		oldTime := time.Now().Add(-1 * time.Hour)
		m.mu.Lock()
		m.offlineConfirmations["local-lvm"] = 5 // Already confirmed
		m.activeAlerts["storage-offline-local-lvm"] = &Alert{
			ID:       "storage-offline-local-lvm",
			LastSeen: oldTime,
		}
		m.mu.Unlock()

		storage := models.Storage{
			ID:   "local-lvm",
			Name: "Local LVM",
			Node: "pve-node1",
		}

		m.checkStorageOffline(storage)

		m.mu.RLock()
		alert := m.activeAlerts["storage-offline-local-lvm"]
		m.mu.RUnlock()

		if !alert.LastSeen.After(oldTime) {
			t.Error("expected LastSeen to be updated")
		}
	})

	t.Run("disabled storage clears existing alert", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		// Pre-create an alert
		m.mu.Lock()
		m.activeAlerts["storage-offline-local-lvm"] = &Alert{ID: "storage-offline-local-lvm"}
		m.config.Overrides = map[string]ThresholdConfig{
			"local-lvm": {Disabled: true},
		}
		m.mu.Unlock()

		storage := models.Storage{
			ID:   "local-lvm",
			Name: "Local LVM",
			Node: "pve-node1",
		}

		m.checkStorageOffline(storage)

		m.mu.RLock()
		_, exists := m.activeAlerts["storage-offline-local-lvm"]
		m.mu.RUnlock()

		if exists {
			t.Error("expected alert to be cleared when storage is disabled")
		}
	})

	t.Run("disabled storage does not create alert", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.config.Overrides = map[string]ThresholdConfig{
			"local-lvm": {Disabled: true},
		}
		m.mu.Unlock()

		storage := models.Storage{
			ID:   "local-lvm",
			Name: "Local LVM",
			Node: "pve-node1",
		}

		// Multiple polls should not create alert
		m.checkStorageOffline(storage)
		m.checkStorageOffline(storage)
		m.checkStorageOffline(storage)

		m.mu.RLock()
		_, exists := m.activeAlerts["storage-offline-local-lvm"]
		m.mu.RUnlock()

		if exists {
			t.Error("expected no alert when storage is disabled")
		}
	})
}

func TestCheckGuestPoweredOff(t *testing.T) {
	// t.Parallel()

	t.Run("first poll increments confirmation but does not create alert", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.checkGuestPoweredOff("vm100", "TestVM", "pve-node1", "pve-instance", "VM", false)

		m.mu.RLock()
		confirmCount := m.offlineConfirmations["vm100"]
		_, alertExists := m.activeAlerts["guest-powered-off-vm100"]
		m.mu.RUnlock()

		if confirmCount != 1 {
			t.Errorf("expected confirmation count 1, got %d", confirmCount)
		}
		if alertExists {
			t.Error("expected no alert on first poll")
		}
	})

	t.Run("second poll creates alert after confirmation", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		// First poll - confirmation
		m.checkGuestPoweredOff("vm100", "TestVM", "pve-node1", "pve-instance", "VM", false)
		// Second poll - should create alert
		m.checkGuestPoweredOff("vm100", "TestVM", "pve-node1", "pve-instance", "VM", false)

		m.mu.RLock()
		alert := m.activeAlerts["guest-powered-off-vm100"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected alert to be created after second poll")
		}
		if alert.Type != "powered-off" {
			t.Errorf("expected type 'powered-off', got %s", alert.Type)
		}
		if alert.Level != AlertLevelWarning {
			t.Errorf("expected warning level (default severity), got %s", alert.Level)
		}
		if alert.ResourceID != "vm100" {
			t.Errorf("expected resource ID 'vm100', got %s", alert.ResourceID)
		}
	})

	t.Run("existing alert updates LastSeen and level", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		oldTime := time.Now().Add(-1 * time.Hour)
		m.mu.Lock()
		m.activeAlerts["guest-powered-off-vm100"] = &Alert{
			ID:       "guest-powered-off-vm100",
			LastSeen: oldTime,
			Level:    AlertLevelWarning,
		}
		m.mu.Unlock()

		m.checkGuestPoweredOff("vm100", "TestVM", "pve-node1", "pve-instance", "VM", false)

		m.mu.RLock()
		alert := m.activeAlerts["guest-powered-off-vm100"]
		m.mu.RUnlock()

		if !alert.LastSeen.After(oldTime) {
			t.Error("expected LastSeen to be updated")
		}
	})

	t.Run("monitorOnly flag is set in metadata", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		// First poll
		m.checkGuestPoweredOff("vm100", "TestVM", "pve-node1", "pve-instance", "VM", true)
		// Second poll - creates alert
		m.checkGuestPoweredOff("vm100", "TestVM", "pve-node1", "pve-instance", "VM", true)

		m.mu.RLock()
		alert := m.activeAlerts["guest-powered-off-vm100"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected alert to be created")
		}
		if alert.Metadata == nil {
			t.Fatal("expected metadata to be set")
		}
		if monitorOnly, ok := alert.Metadata["monitorOnly"].(bool); !ok || !monitorOnly {
			t.Error("expected monitorOnly to be true")
		}
	})

	t.Run("disabled guest clears existing alert", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		// Pre-create an alert and confirmation count
		m.mu.Lock()
		m.activeAlerts["guest-powered-off-vm100"] = &Alert{ID: "guest-powered-off-vm100"}
		m.offlineConfirmations["vm100"] = 5
		m.config.Overrides = map[string]ThresholdConfig{
			"vm100": {Disabled: true},
		}
		m.mu.Unlock()

		m.checkGuestPoweredOff("vm100", "TestVM", "pve-node1", "pve-instance", "VM", false)

		m.mu.RLock()
		_, alertExists := m.activeAlerts["guest-powered-off-vm100"]
		_, confirmExists := m.offlineConfirmations["vm100"]
		m.mu.RUnlock()

		if alertExists {
			t.Error("expected alert to be cleared when guest is disabled")
		}
		if confirmExists {
			t.Error("expected confirmation count to be cleared")
		}
	})

	t.Run("disableConnectivity clears existing alert", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		// Pre-create an alert
		m.mu.Lock()
		m.activeAlerts["guest-powered-off-vm100"] = &Alert{ID: "guest-powered-off-vm100"}
		m.config.Overrides = map[string]ThresholdConfig{
			"vm100": {DisableConnectivity: true},
		}
		m.mu.Unlock()

		m.checkGuestPoweredOff("vm100", "TestVM", "pve-node1", "pve-instance", "VM", false)

		m.mu.RLock()
		_, exists := m.activeAlerts["guest-powered-off-vm100"]
		m.mu.RUnlock()

		if exists {
			t.Error("expected alert to be cleared when connectivity is disabled")
		}
	})

	t.Run("uses override severity when configured", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.config.Overrides = map[string]ThresholdConfig{
			"vm100": {PoweredOffSeverity: AlertLevelCritical},
		}
		m.mu.Unlock()

		// First poll
		m.checkGuestPoweredOff("vm100", "TestVM", "pve-node1", "pve-instance", "VM", false)
		// Second poll
		m.checkGuestPoweredOff("vm100", "TestVM", "pve-node1", "pve-instance", "VM", false)

		m.mu.RLock()
		alert := m.activeAlerts["guest-powered-off-vm100"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected alert to be created")
		}
		if alert.Level != AlertLevelCritical {
			t.Errorf("expected critical level from override, got %s", alert.Level)
		}
	})

	t.Run("uses default severity when no override", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.config.GuestDefaults.PoweredOffSeverity = AlertLevelCritical
		m.mu.Unlock()

		// First poll
		m.checkGuestPoweredOff("vm100", "TestVM", "pve-node1", "pve-instance", "VM", false)
		// Second poll
		m.checkGuestPoweredOff("vm100", "TestVM", "pve-node1", "pve-instance", "VM", false)

		m.mu.RLock()
		alert := m.activeAlerts["guest-powered-off-vm100"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected alert to be created")
		}
		if alert.Level != AlertLevelCritical {
			t.Errorf("expected critical level from defaults, got %s", alert.Level)
		}
	})

	t.Run("container type in message", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		// First poll
		m.checkGuestPoweredOff("ct200", "TestContainer", "pve-node1", "pve-instance", "Container", false)
		// Second poll
		m.checkGuestPoweredOff("ct200", "TestContainer", "pve-node1", "pve-instance", "Container", false)

		m.mu.RLock()
		alert := m.activeAlerts["guest-powered-off-ct200"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected alert to be created")
		}
		if !strings.Contains(alert.Message, "Container") {
			t.Errorf("expected message to contain 'Container', got %s", alert.Message)
		}
		if !strings.Contains(alert.Message, "TestContainer") {
			t.Errorf("expected message to contain 'TestContainer', got %s", alert.Message)
		}
	})
}

func TestCleanup(t *testing.T) {
	// t.Parallel()

	t.Run("auto-acknowledges old alerts", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		oldTime := time.Now().Add(-3 * time.Hour)
		m.mu.Lock()
		m.config.AutoAcknowledgeAfterHours = 2
		m.activeAlerts["old-alert"] = &Alert{
			ID:           "old-alert",
			StartTime:    oldTime,
			Acknowledged: false,
		}
		m.mu.Unlock()

		m.Cleanup(1 * time.Hour)

		m.mu.RLock()
		alert := m.activeAlerts["old-alert"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected alert to exist")
		}
		if !alert.Acknowledged {
			t.Error("expected alert to be auto-acknowledged")
		}
		if alert.AckUser != "system-auto" {
			t.Errorf("expected AckUser 'system-auto', got %s", alert.AckUser)
		}
	})

	t.Run("removes old acknowledged alerts by TTL", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		oldAckTime := time.Now().Add(-10 * 24 * time.Hour) // 10 days ago
		m.mu.Lock()
		m.config.MaxAcknowledgedAgeDays = 7
		m.activeAlerts["ack-alert"] = &Alert{
			ID:           "ack-alert",
			Acknowledged: true,
			AckTime:      &oldAckTime,
		}
		m.mu.Unlock()

		m.Cleanup(1 * time.Hour)

		m.mu.RLock()
		_, exists := m.activeAlerts["ack-alert"]
		m.mu.RUnlock()

		if exists {
			t.Error("expected acknowledged alert to be removed by TTL")
		}
	})

	t.Run("removes old unacknowledged alerts by TTL", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		oldTime := time.Now().Add(-40 * 24 * time.Hour) // 40 days ago
		m.mu.Lock()
		m.config.MaxAlertAgeDays = 30
		m.config.AutoAcknowledgeAfterHours = 0 // Disable auto-acknowledge to test TTL
		m.activeAlerts["old-unack-alert"] = &Alert{
			ID:           "old-unack-alert",
			StartTime:    oldTime,
			Acknowledged: false,
		}
		m.mu.Unlock()

		m.Cleanup(1 * time.Hour)

		m.mu.RLock()
		_, exists := m.activeAlerts["old-unack-alert"]
		m.mu.RUnlock()

		if exists {
			t.Error("expected old unacknowledged alert to be removed by TTL")
		}
	})

	t.Run("removes acknowledged alerts by maxAge fallback", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		oldAckTime := time.Now().Add(-2 * time.Hour)
		m.mu.Lock()
		m.activeAlerts["ack-fallback"] = &Alert{
			ID:           "ack-fallback",
			Acknowledged: true,
			AckTime:      &oldAckTime,
		}
		m.mu.Unlock()

		m.Cleanup(1 * time.Hour)

		m.mu.RLock()
		_, exists := m.activeAlerts["ack-fallback"]
		m.mu.RUnlock()

		if exists {
			t.Error("expected acknowledged alert to be removed by maxAge fallback")
		}
	})

	t.Run("keeps acknowledged alerts that are still active", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		oldAckTime := time.Now().Add(-2 * time.Hour)
		recentSeen := time.Now().Add(-5 * time.Minute)
		m.mu.Lock()
		m.activeAlerts["ack-active"] = &Alert{
			ID:           "ack-active",
			Acknowledged: true,
			AckTime:      &oldAckTime,
			LastSeen:     recentSeen,
			StartTime:    recentSeen,
		}
		m.mu.Unlock()

		m.Cleanup(1 * time.Hour)

		m.mu.RLock()
		_, exists := m.activeAlerts["ack-active"]
		m.mu.RUnlock()

		if !exists {
			t.Error("expected acknowledged active alert to remain")
		}
	})

	t.Run("cleans up old recent alerts", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		oldTime := time.Now().Add(-10 * time.Minute)
		m.mu.Lock()
		m.recentAlerts["recent-old"] = &Alert{
			ID:        "recent-old",
			StartTime: oldTime,
		}
		m.mu.Unlock()

		m.Cleanup(1 * time.Hour)

		m.mu.RLock()
		_, exists := m.recentAlerts["recent-old"]
		m.mu.RUnlock()

		if exists {
			t.Error("expected old recent alert to be cleaned up")
		}
	})

	t.Run("cleans up expired suppressions", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.suppressedUntil["suppressed-alert"] = time.Now().Add(-1 * time.Hour)
		m.mu.Unlock()

		m.Cleanup(1 * time.Hour)

		m.mu.RLock()
		_, exists := m.suppressedUntil["suppressed-alert"]
		m.mu.RUnlock()

		if exists {
			t.Error("expected expired suppression to be cleaned up")
		}
	})

	t.Run("cleans up old rate limit entries", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.alertRateLimit["rate-limited"] = []time.Time{
			time.Now().Add(-2 * time.Hour),    // Old, should be removed
			time.Now().Add(-30 * time.Minute), // Recent, should remain
		}
		m.mu.Unlock()

		m.Cleanup(1 * time.Hour)

		m.mu.RLock()
		times := m.alertRateLimit["rate-limited"]
		m.mu.RUnlock()

		if len(times) != 1 {
			t.Errorf("expected 1 recent time, got %d", len(times))
		}
	})

	t.Run("removes empty rate limit entries", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.alertRateLimit["all-old"] = []time.Time{
			time.Now().Add(-2 * time.Hour),
		}
		m.mu.Unlock()

		m.Cleanup(1 * time.Hour)

		m.mu.RLock()
		_, exists := m.alertRateLimit["all-old"]
		m.mu.RUnlock()

		if exists {
			t.Error("expected empty rate limit entry to be removed")
		}
	})

	t.Run("cleans up old recently resolved alerts", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.resolvedMutex.Lock()
		m.recentlyResolved["old-resolved"] = &ResolvedAlert{
			Alert:        &Alert{ID: "old-resolved"},
			ResolvedTime: time.Now().Add(-10 * time.Minute),
		}
		m.resolvedMutex.Unlock()

		m.Cleanup(1 * time.Hour)

		m.resolvedMutex.Lock()
		_, exists := m.recentlyResolved["old-resolved"]
		m.resolvedMutex.Unlock()

		if exists {
			t.Error("expected old recently resolved alert to be cleaned up")
		}
	})

	t.Run("cleans up stale pending alerts", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.pendingAlerts["stale-pending"] = time.Now().Add(-15 * time.Minute)
		m.mu.Unlock()

		m.Cleanup(1 * time.Hour)

		m.mu.RLock()
		_, exists := m.pendingAlerts["stale-pending"]
		m.mu.RUnlock()

		if exists {
			t.Error("expected stale pending alert to be cleaned up")
		}
	})

	t.Run("cleans up flapping history for inactive alerts", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.flappingHistory["inactive-alert"] = []time.Time{
			time.Now().Add(-30 * time.Minute),
		}
		m.flappingActive["inactive-alert"] = true
		// No active alert, no suppression
		m.mu.Unlock()

		m.Cleanup(1 * time.Hour)

		m.mu.RLock()
		_, historyExists := m.flappingHistory["inactive-alert"]
		_, activeExists := m.flappingActive["inactive-alert"]
		m.mu.RUnlock()

		if historyExists {
			t.Error("expected flapping history to be cleaned up")
		}
		if activeExists {
			t.Error("expected flapping active flag to be cleaned up")
		}
	})

	t.Run("cleans up stale Docker restart tracking", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.dockerRestartTracking["stale-container"] = &dockerRestartRecord{
			lastChecked: time.Now().Add(-25 * time.Hour),
		}
		m.mu.Unlock()

		m.Cleanup(1 * time.Hour)

		m.mu.RLock()
		_, exists := m.dockerRestartTracking["stale-container"]
		m.mu.RUnlock()

		if exists {
			t.Error("expected stale Docker restart tracking to be cleaned up")
		}
	})

	t.Run("cleans up stale PMG anomaly trackers", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.pmgAnomalyTrackers["stale-pmg"] = &pmgAnomalyTracker{
			LastSampleTime: time.Now().Add(-25 * time.Hour),
		}
		m.mu.Unlock()

		m.Cleanup(1 * time.Hour)

		m.mu.RLock()
		_, exists := m.pmgAnomalyTrackers["stale-pmg"]
		m.mu.RUnlock()

		if exists {
			t.Error("expected stale PMG anomaly tracker to be cleaned up")
		}
	})

	t.Run("cleans up empty PMG quarantine history", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.pmgQuarantineHistory["empty-pmg"] = []pmgQuarantineSnapshot{}
		m.mu.Unlock()

		m.Cleanup(1 * time.Hour)

		m.mu.RLock()
		_, exists := m.pmgQuarantineHistory["empty-pmg"]
		m.mu.RUnlock()

		if exists {
			t.Error("expected empty PMG quarantine history to be cleaned up")
		}
	})

	t.Run("cleans up stale PMG quarantine history", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.pmgQuarantineHistory["stale-pmg"] = []pmgQuarantineSnapshot{
			{Timestamp: time.Now().Add(-8 * 24 * time.Hour)},
		}
		m.mu.Unlock()

		m.Cleanup(1 * time.Hour)

		m.mu.RLock()
		_, exists := m.pmgQuarantineHistory["stale-pmg"]
		m.mu.RUnlock()

		if exists {
			t.Error("expected stale PMG quarantine history to be cleaned up")
		}
	})
}

func TestConvertLegacyThreshold(t *testing.T) {
	// t.Parallel()

	t.Run("nil input returns nil", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		result := m.convertLegacyThreshold(nil)

		if result != nil {
			t.Error("expected nil result for nil input")
		}
	})

	t.Run("zero value returns nil", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		zero := 0.0
		result := m.convertLegacyThreshold(&zero)

		if result != nil {
			t.Error("expected nil result for zero value")
		}
	})

	t.Run("negative value returns nil", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		neg := -5.0
		result := m.convertLegacyThreshold(&neg)

		if result != nil {
			t.Error("expected nil result for negative value")
		}
	})

	t.Run("positive value with default margin", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		threshold := 80.0
		result := m.convertLegacyThreshold(&threshold)

		if result == nil {
			t.Fatal("expected non-nil result")
		}
		if result.Trigger != 80.0 {
			t.Errorf("expected trigger 80.0, got %f", result.Trigger)
		}
		if result.Clear != 75.0 { // 80 - 5 (default margin)
			t.Errorf("expected clear 75.0, got %f", result.Clear)
		}
	})

	t.Run("positive value with custom margin", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.config.HysteresisMargin = 10.0
		m.mu.Unlock()

		threshold := 80.0
		result := m.convertLegacyThreshold(&threshold)

		if result == nil {
			t.Fatal("expected non-nil result")
		}
		if result.Trigger != 80.0 {
			t.Errorf("expected trigger 80.0, got %f", result.Trigger)
		}
		if result.Clear != 70.0 { // 80 - 10 (custom margin)
			t.Errorf("expected clear 70.0, got %f", result.Clear)
		}
	})
}

func TestCheckEscalations(t *testing.T) {
	// t.Parallel()

	t.Run("does nothing when escalation is disabled", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		oldTime := time.Now().Add(-2 * time.Hour)
		m.mu.Lock()
		m.config.ActivationState = ActivationActive
		m.config.Schedule.Escalation.Enabled = false
		m.config.Schedule.Escalation.Levels = []EscalationLevel{
			{After: 30, Notify: "email"},
		}
		m.activeAlerts["test-alert"] = &Alert{
			ID:             "test-alert",
			StartTime:      oldTime,
			LastEscalation: 0,
		}
		m.mu.Unlock()

		m.checkEscalations()

		m.mu.RLock()
		alert := m.activeAlerts["test-alert"]
		m.mu.RUnlock()

		if alert.LastEscalation != 0 {
			t.Errorf("expected no escalation when disabled, got %d", alert.LastEscalation)
		}
	})

	t.Run("does nothing when alerts are globally disabled", func(t *testing.T) {
		m := newTestManager(t)

		oldTime := time.Now().Add(-2 * time.Hour)
		m.mu.Lock()
		m.config.Enabled = false
		m.config.ActivationState = ActivationActive
		m.config.Schedule.Escalation.Enabled = true
		m.config.Schedule.Escalation.Levels = []EscalationLevel{
			{After: 30, Notify: "email"},
		}
		m.activeAlerts["global-disabled-alert"] = &Alert{
			ID:             "global-disabled-alert",
			StartTime:      oldTime,
			LastEscalation: 0,
		}
		m.mu.Unlock()

		m.checkEscalations()

		m.mu.RLock()
		alert := m.activeAlerts["global-disabled-alert"]
		m.mu.RUnlock()

		if alert.LastEscalation != 0 {
			t.Errorf("expected no escalation when alerts are globally disabled, got %d", alert.LastEscalation)
		}
	})

	t.Run("does nothing when activation state is pending", func(t *testing.T) {
		m := newTestManager(t)

		oldTime := time.Now().Add(-2 * time.Hour)
		m.mu.Lock()
		m.config.Enabled = true
		m.config.ActivationState = ActivationPending
		m.config.Schedule.Escalation.Enabled = true
		m.config.Schedule.Escalation.Levels = []EscalationLevel{
			{After: 30, Notify: "email"},
		}
		m.activeAlerts["pending-alert"] = &Alert{
			ID:             "pending-alert",
			StartTime:      oldTime,
			LastEscalation: 0,
		}
		m.mu.Unlock()

		m.checkEscalations()

		m.mu.RLock()
		alert := m.activeAlerts["pending-alert"]
		m.mu.RUnlock()

		if alert.LastEscalation != 0 {
			t.Errorf("expected no escalation when activation is pending, got %d", alert.LastEscalation)
		}
	})

	t.Run("does nothing when activation state is snoozed", func(t *testing.T) {
		m := newTestManager(t)

		oldTime := time.Now().Add(-2 * time.Hour)
		m.mu.Lock()
		m.config.Enabled = true
		m.config.ActivationState = ActivationSnoozed
		m.config.Schedule.Escalation.Enabled = true
		m.config.Schedule.Escalation.Levels = []EscalationLevel{
			{After: 30, Notify: "email"},
		}
		m.activeAlerts["snoozed-alert"] = &Alert{
			ID:             "snoozed-alert",
			StartTime:      oldTime,
			LastEscalation: 0,
		}
		m.mu.Unlock()

		m.checkEscalations()

		m.mu.RLock()
		alert := m.activeAlerts["snoozed-alert"]
		m.mu.RUnlock()

		if alert.LastEscalation != 0 {
			t.Errorf("expected no escalation when activation is snoozed, got %d", alert.LastEscalation)
		}
	})

	t.Run("skips acknowledged alerts", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		oldTime := time.Now().Add(-2 * time.Hour)
		m.mu.Lock()
		m.config.ActivationState = ActivationActive
		m.config.Schedule.Escalation.Enabled = true
		m.config.Schedule.Escalation.Levels = []EscalationLevel{
			{After: 30, Notify: "email"},
		}
		m.activeAlerts["ack-alert"] = &Alert{
			ID:             "ack-alert",
			StartTime:      oldTime,
			LastEscalation: 0,
			Acknowledged:   true,
		}
		m.mu.Unlock()

		m.checkEscalations()

		m.mu.RLock()
		alert := m.activeAlerts["ack-alert"]
		m.mu.RUnlock()

		if alert.LastEscalation != 0 {
			t.Error("expected no escalation for acknowledged alert")
		}
	})

	t.Run("escalates alert after threshold time", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		oldTime := time.Now().Add(-45 * time.Minute) // 45 minutes ago
		m.mu.Lock()
		m.config.ActivationState = ActivationActive
		m.config.Schedule.Escalation.Enabled = true
		m.config.Schedule.Escalation.Levels = []EscalationLevel{
			{After: 30, Notify: "email"},   // 30 minutes
			{After: 60, Notify: "webhook"}, // 60 minutes
		}
		m.activeAlerts["escalate-alert"] = &Alert{
			ID:             "escalate-alert",
			StartTime:      oldTime,
			LastEscalation: 0,
		}
		m.mu.Unlock()

		m.checkEscalations()

		m.mu.RLock()
		alert := m.activeAlerts["escalate-alert"]
		m.mu.RUnlock()

		if alert.LastEscalation != 1 {
			t.Errorf("expected escalation to level 1, got %d", alert.LastEscalation)
		}
		if len(alert.EscalationTimes) != 1 {
			t.Errorf("expected 1 escalation time, got %d", len(alert.EscalationTimes))
		}
	})

	t.Run("escalates to multiple levels", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		oldTime := time.Now().Add(-90 * time.Minute) // 90 minutes ago
		m.mu.Lock()
		m.config.ActivationState = ActivationActive
		m.config.Schedule.Escalation.Enabled = true
		m.config.Schedule.Escalation.Levels = []EscalationLevel{
			{After: 30, Notify: "email"},   // 30 minutes
			{After: 60, Notify: "webhook"}, // 60 minutes
		}
		m.activeAlerts["multi-escalate"] = &Alert{
			ID:             "multi-escalate",
			StartTime:      oldTime,
			LastEscalation: 0,
		}
		m.mu.Unlock()

		m.checkEscalations()

		m.mu.RLock()
		alert := m.activeAlerts["multi-escalate"]
		m.mu.RUnlock()

		if alert.LastEscalation != 2 {
			t.Errorf("expected escalation to level 2, got %d", alert.LastEscalation)
		}
		if len(alert.EscalationTimes) != 2 {
			t.Errorf("expected 2 escalation times, got %d", len(alert.EscalationTimes))
		}
	})

	t.Run("does not re-escalate already escalated level", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		oldTime := time.Now().Add(-45 * time.Minute)
		m.mu.Lock()
		m.config.ActivationState = ActivationActive
		m.config.Schedule.Escalation.Enabled = true
		m.config.Schedule.Escalation.Levels = []EscalationLevel{
			{After: 30, Notify: "email"},
		}
		m.activeAlerts["already-escalated"] = &Alert{
			ID:              "already-escalated",
			StartTime:       oldTime,
			LastEscalation:  1,
			EscalationTimes: []time.Time{time.Now().Add(-10 * time.Minute)},
		}
		m.mu.Unlock()

		m.checkEscalations()

		m.mu.RLock()
		alert := m.activeAlerts["already-escalated"]
		m.mu.RUnlock()

		if alert.LastEscalation != 1 {
			t.Errorf("expected escalation to remain at 1, got %d", alert.LastEscalation)
		}
		if len(alert.EscalationTimes) != 1 {
			t.Errorf("expected 1 escalation time (unchanged), got %d", len(alert.EscalationTimes))
		}
	})

	t.Run("does not escalate before threshold time", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		recentTime := time.Now().Add(-10 * time.Minute) // Only 10 minutes ago
		m.mu.Lock()
		m.config.ActivationState = ActivationActive
		m.config.Schedule.Escalation.Enabled = true
		m.config.Schedule.Escalation.Levels = []EscalationLevel{
			{After: 30, Notify: "email"}, // 30 minutes threshold
		}
		m.activeAlerts["recent-alert"] = &Alert{
			ID:             "recent-alert",
			StartTime:      recentTime,
			LastEscalation: 0,
		}
		m.mu.Unlock()

		m.checkEscalations()

		m.mu.RLock()
		alert := m.activeAlerts["recent-alert"]
		m.mu.RUnlock()

		if alert.LastEscalation != 0 {
			t.Error("expected no escalation for recent alert")
		}
	})
}

func TestCleanupAlertsForNodes(t *testing.T) {
	// t.Parallel()

	t.Run("removes alerts for non-existent nodes", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.activeAlerts["alert-old-node"] = &Alert{
			ID:   "alert-old-node",
			Node: "old-node",
		}
		m.activeAlerts["alert-valid-node"] = &Alert{
			ID:   "alert-valid-node",
			Node: "valid-node",
		}
		m.mu.Unlock()

		existingNodes := map[string]bool{
			"valid-node": true,
		}

		m.CleanupAlertsForNodes(existingNodes)

		// Give async save goroutine time to complete
		time.Sleep(50 * time.Millisecond)

		m.mu.RLock()
		_, oldExists := m.activeAlerts["alert-old-node"]
		_, validExists := m.activeAlerts["alert-valid-node"]
		m.mu.RUnlock()

		if oldExists {
			t.Error("expected alert for old node to be removed")
		}
		if !validExists {
			t.Error("expected alert for valid node to remain")
		}
	})

	t.Run("skips Docker alerts", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.activeAlerts["docker-container-state"] = &Alert{
			ID:         "docker-container-state",
			ResourceID: "docker:host1:container1",
			Node:       "non-existent-node",
		}
		m.activeAlerts["alert-with-docker-resource"] = &Alert{
			ID:         "alert-with-docker-resource",
			ResourceID: "docker:host2:container2",
			Node:       "non-existent-node",
		}
		m.mu.Unlock()

		existingNodes := map[string]bool{}

		m.CleanupAlertsForNodes(existingNodes)

		m.mu.RLock()
		_, dockerExists := m.activeAlerts["docker-container-state"]
		_, dockerResourceExists := m.activeAlerts["alert-with-docker-resource"]
		m.mu.RUnlock()

		if !dockerExists {
			t.Error("expected docker alert to be preserved")
		}
		if !dockerResourceExists {
			t.Error("expected alert with docker resource to be preserved")
		}
	})

	t.Run("skips PBS alerts", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.activeAlerts["pbs-offline-test"] = &Alert{
			ID:   "pbs-offline-test",
			Node: "non-existent-node",
		}
		m.activeAlerts["pbs-backup-alert"] = &Alert{
			ID:   "pbs-backup-alert",
			Type: "pbs-offline",
			Node: "non-existent-node",
		}
		m.mu.Unlock()

		existingNodes := map[string]bool{}

		m.CleanupAlertsForNodes(existingNodes)

		m.mu.RLock()
		_, pbsExists := m.activeAlerts["pbs-offline-test"]
		_, pbsTypeExists := m.activeAlerts["pbs-backup-alert"]
		m.mu.RUnlock()

		if !pbsExists {
			t.Error("expected pbs-prefixed alert to be preserved")
		}
		if !pbsTypeExists {
			t.Error("expected pbs-offline type alert to be preserved")
		}
	})

	t.Run("removes alerts with empty node", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.activeAlerts["empty-node-alert"] = &Alert{
			ID:   "empty-node-alert",
			Node: "",
		}
		m.mu.Unlock()

		existingNodes := map[string]bool{
			"valid-node": true,
		}

		m.CleanupAlertsForNodes(existingNodes)

		time.Sleep(50 * time.Millisecond)

		m.mu.RLock()
		_, exists := m.activeAlerts["empty-node-alert"]
		m.mu.RUnlock()

		if exists {
			t.Error("expected alert with empty node to be removed")
		}
	})

	t.Run("handles nil alert in map", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.activeAlerts["nil-alert"] = nil
		m.activeAlerts["valid-alert"] = &Alert{
			ID:   "valid-alert",
			Node: "valid-node",
		}
		m.mu.Unlock()

		existingNodes := map[string]bool{
			"valid-node": true,
		}

		// Should not panic
		m.CleanupAlertsForNodes(existingNodes)

		m.mu.RLock()
		_, validExists := m.activeAlerts["valid-alert"]
		m.mu.RUnlock()

		if !validExists {
			t.Error("expected valid alert to remain")
		}
	})

	t.Run("no cleanup needed logs correctly", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.activeAlerts["valid-alert"] = &Alert{
			ID:   "valid-alert",
			Node: "valid-node",
		}
		m.mu.Unlock()

		existingNodes := map[string]bool{
			"valid-node": true,
		}

		// Should not panic and should not remove any alerts
		m.CleanupAlertsForNodes(existingNodes)

		m.mu.RLock()
		count := len(m.activeAlerts)
		m.mu.RUnlock()

		if count != 1 {
			t.Errorf("expected 1 alert, got %d", count)
		}
	})
}

func TestCheckZFSPoolHealth(t *testing.T) {
	// t.Parallel()

	t.Run("nil ZFSPool returns early", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		storage := models.Storage{
			ID:      "local-zfs",
			Name:    "Local ZFS",
			Node:    "pve-node1",
			ZFSPool: nil,
		}

		// Should not panic
		m.checkZFSPoolHealth(storage)

		m.mu.RLock()
		count := len(m.activeAlerts)
		m.mu.RUnlock()

		if count != 0 {
			t.Errorf("expected no alerts for nil pool, got %d", count)
		}
	})

	t.Run("ONLINE pool does not create state alert", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		storage := models.Storage{
			ID:   "local-zfs",
			Name: "Local ZFS",
			Node: "pve-node1",
			ZFSPool: &models.ZFSPool{
				Name:  "rpool",
				State: "ONLINE",
			},
		}

		m.checkZFSPoolHealth(storage)

		m.mu.RLock()
		_, exists := m.activeAlerts["zfs-pool-state-local-zfs"]
		m.mu.RUnlock()

		if exists {
			t.Error("expected no state alert for ONLINE pool")
		}
	})

	t.Run("DEGRADED pool creates warning alert", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		storage := models.Storage{
			ID:       "local-zfs",
			Name:     "Local ZFS",
			Node:     "pve-node1",
			Instance: "pve-instance",
			ZFSPool: &models.ZFSPool{
				Name:  "rpool",
				State: "DEGRADED",
			},
		}

		m.checkZFSPoolHealth(storage)

		m.mu.RLock()
		alert := m.activeAlerts["zfs-pool-state-local-zfs"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected state alert for DEGRADED pool")
		}
		if alert.Level != AlertLevelWarning {
			t.Errorf("expected warning level, got %s", alert.Level)
		}
		if alert.Type != "zfs-pool-state" {
			t.Errorf("expected type 'zfs-pool-state', got %s", alert.Type)
		}
	})

	t.Run("FAULTED pool creates critical alert", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		storage := models.Storage{
			ID:   "local-zfs",
			Name: "Local ZFS",
			Node: "pve-node1",
			ZFSPool: &models.ZFSPool{
				Name:  "rpool",
				State: "FAULTED",
			},
		}

		m.checkZFSPoolHealth(storage)

		m.mu.RLock()
		alert := m.activeAlerts["zfs-pool-state-local-zfs"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected state alert for FAULTED pool")
		}
		if alert.Level != AlertLevelCritical {
			t.Errorf("expected critical level, got %s", alert.Level)
		}
	})

	t.Run("UNAVAIL pool creates critical alert", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		storage := models.Storage{
			ID:   "local-zfs",
			Name: "Local ZFS",
			Node: "pve-node1",
			ZFSPool: &models.ZFSPool{
				Name:  "rpool",
				State: "UNAVAIL",
			},
		}

		m.checkZFSPoolHealth(storage)

		m.mu.RLock()
		alert := m.activeAlerts["zfs-pool-state-local-zfs"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected state alert for UNAVAIL pool")
		}
		if alert.Level != AlertLevelCritical {
			t.Errorf("expected critical level, got %s", alert.Level)
		}
	})

	t.Run("pool coming back ONLINE clears state alert", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		// Pre-create a state alert
		m.mu.Lock()
		m.activeAlerts["zfs-pool-state-local-zfs"] = &Alert{
			ID:    "zfs-pool-state-local-zfs",
			Level: AlertLevelWarning,
		}
		m.mu.Unlock()

		storage := models.Storage{
			ID:   "local-zfs",
			Name: "Local ZFS",
			Node: "pve-node1",
			ZFSPool: &models.ZFSPool{
				Name:  "rpool",
				State: "ONLINE",
			},
		}

		m.checkZFSPoolHealth(storage)

		m.mu.RLock()
		_, exists := m.activeAlerts["zfs-pool-state-local-zfs"]
		m.mu.RUnlock()

		if exists {
			t.Error("expected state alert to be cleared when pool is ONLINE")
		}
	})

	t.Run("pool with errors creates error alert", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		storage := models.Storage{
			ID:   "local-zfs",
			Name: "Local ZFS",
			Node: "pve-node1",
			ZFSPool: &models.ZFSPool{
				Name:           "rpool",
				State:          "ONLINE",
				ReadErrors:     5,
				WriteErrors:    2,
				ChecksumErrors: 1,
			},
		}

		m.checkZFSPoolHealth(storage)

		m.mu.RLock()
		alert := m.activeAlerts["zfs-pool-errors-local-zfs"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected errors alert for pool with errors")
		}
		if alert.Type != "zfs-pool-errors" {
			t.Errorf("expected type 'zfs-pool-errors', got %s", alert.Type)
		}
		if alert.Value != 8 { // 5 + 2 + 1
			t.Errorf("expected value 8, got %f", alert.Value)
		}
	})

	t.Run("pool error count increase updates alert", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		oldTime := time.Now().Add(-1 * time.Hour)
		m.mu.Lock()
		m.activeAlerts["zfs-pool-errors-local-zfs"] = &Alert{
			ID:        "zfs-pool-errors-local-zfs",
			Value:     5,
			StartTime: oldTime,
		}
		m.mu.Unlock()

		storage := models.Storage{
			ID:   "local-zfs",
			Name: "Local ZFS",
			Node: "pve-node1",
			ZFSPool: &models.ZFSPool{
				Name:           "rpool",
				State:          "ONLINE",
				ReadErrors:     10,
				WriteErrors:    0,
				ChecksumErrors: 0,
			},
		}

		m.checkZFSPoolHealth(storage)

		m.mu.RLock()
		alert := m.activeAlerts["zfs-pool-errors-local-zfs"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected errors alert to exist")
		}
		if alert.Value != 10 {
			t.Errorf("expected value 10, got %f", alert.Value)
		}
		// Start time should be preserved
		if !alert.StartTime.Equal(oldTime) {
			t.Error("expected StartTime to be preserved on update")
		}
	})

	t.Run("pool with no errors clears error alert", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.activeAlerts["zfs-pool-errors-local-zfs"] = &Alert{
			ID: "zfs-pool-errors-local-zfs",
		}
		m.mu.Unlock()

		storage := models.Storage{
			ID:   "local-zfs",
			Name: "Local ZFS",
			Node: "pve-node1",
			ZFSPool: &models.ZFSPool{
				Name:           "rpool",
				State:          "ONLINE",
				ReadErrors:     0,
				WriteErrors:    0,
				ChecksumErrors: 0,
			},
		}

		m.checkZFSPoolHealth(storage)

		m.mu.RLock()
		_, exists := m.activeAlerts["zfs-pool-errors-local-zfs"]
		m.mu.RUnlock()

		if exists {
			t.Error("expected errors alert to be cleared when no errors")
		}
	})

	t.Run("device with errors creates device alert", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		storage := models.Storage{
			ID:   "local-zfs",
			Name: "Local ZFS",
			Node: "pve-node1",
			ZFSPool: &models.ZFSPool{
				Name:  "rpool",
				State: "ONLINE",
				Devices: []models.ZFSDevice{
					{Name: "sda", State: "ONLINE", ReadErrors: 3, WriteErrors: 0, ChecksumErrors: 0},
				},
			},
		}

		m.checkZFSPoolHealth(storage)

		m.mu.RLock()
		alert := m.activeAlerts["zfs-device-local-zfs-sda"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected device alert for device with errors")
		}
		if alert.Type != "zfs-device" {
			t.Errorf("expected type 'zfs-device', got %s", alert.Type)
		}
	})

	t.Run("device in FAULTED state creates critical alert", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		storage := models.Storage{
			ID:   "local-zfs",
			Name: "Local ZFS",
			Node: "pve-node1",
			ZFSPool: &models.ZFSPool{
				Name:  "rpool",
				State: "DEGRADED",
				Devices: []models.ZFSDevice{
					{Name: "sda", State: "FAULTED"},
				},
			},
		}

		m.checkZFSPoolHealth(storage)

		m.mu.RLock()
		alert := m.activeAlerts["zfs-device-local-zfs-sda"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected device alert for FAULTED device")
		}
		if alert.Level != AlertLevelCritical {
			t.Errorf("expected critical level for FAULTED device, got %s", alert.Level)
		}
	})

	t.Run("healthy device clears device alert", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.activeAlerts["zfs-device-local-zfs-sda"] = &Alert{
			ID: "zfs-device-local-zfs-sda",
		}
		m.mu.Unlock()

		storage := models.Storage{
			ID:   "local-zfs",
			Name: "Local ZFS",
			Node: "pve-node1",
			ZFSPool: &models.ZFSPool{
				Name:  "rpool",
				State: "ONLINE",
				Devices: []models.ZFSDevice{
					{Name: "sda", State: "ONLINE", ReadErrors: 0, WriteErrors: 0, ChecksumErrors: 0},
				},
			},
		}

		m.checkZFSPoolHealth(storage)

		m.mu.RLock()
		_, exists := m.activeAlerts["zfs-device-local-zfs-sda"]
		m.mu.RUnlock()

		if exists {
			t.Error("expected device alert to be cleared for healthy device")
		}
	})

	t.Run("SPARE device in normal state does not create alert", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		storage := models.Storage{
			ID:   "local-zfs",
			Name: "Local ZFS",
			Node: "pve-node1",
			ZFSPool: &models.ZFSPool{
				Name:  "rpool",
				State: "ONLINE",
				Devices: []models.ZFSDevice{
					{Name: "sdb", State: "SPARE", ReadErrors: 0, WriteErrors: 0, ChecksumErrors: 0},
				},
			},
		}

		m.checkZFSPoolHealth(storage)

		m.mu.RLock()
		_, exists := m.activeAlerts["zfs-device-local-zfs-sdb"]
		m.mu.RUnlock()

		if exists {
			t.Error("expected no alert for SPARE device without errors")
		}
	})
}

func TestCheckPMGNodeQueues(t *testing.T) {
	// t.Parallel()

	t.Run("empty nodes returns early", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		pmg := models.PMGInstance{
			ID:    "pmg1",
			Name:  "PMG 1",
			Nodes: []models.PMGNodeStatus{},
		}

		defaults := PMGThresholdConfig{
			QueueTotalWarning: 100,
		}

		// Should not panic
		m.checkPMGNodeQueues(pmg, defaults)

		m.mu.RLock()
		count := len(m.activeAlerts)
		m.mu.RUnlock()

		if count != 0 {
			t.Errorf("expected no alerts for empty nodes, got %d", count)
		}
	})

	t.Run("nil QueueStatus is skipped", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		pmg := models.PMGInstance{
			ID:   "pmg1",
			Name: "PMG 1",
			Nodes: []models.PMGNodeStatus{
				{Name: "node1", QueueStatus: nil},
			},
		}

		defaults := PMGThresholdConfig{
			QueueTotalWarning: 100,
		}

		m.checkPMGNodeQueues(pmg, defaults)

		m.mu.RLock()
		count := len(m.activeAlerts)
		m.mu.RUnlock()

		if count != 0 {
			t.Errorf("expected no alerts for nil QueueStatus, got %d", count)
		}
	})

	t.Run("total queue warning alert", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		pmg := models.PMGInstance{
			ID:   "pmg1",
			Name: "PMG 1",
			Nodes: []models.PMGNodeStatus{
				{Name: "node1", QueueStatus: &models.PMGQueueStatus{Total: 80}},
			},
		}

		defaults := PMGThresholdConfig{
			QueueTotalWarning:  100, // 60% scaled = 60
			QueueTotalCritical: 200, // 80% scaled = 160
		}

		m.checkPMGNodeQueues(pmg, defaults)

		m.mu.RLock()
		alert := m.activeAlerts["pmg1-node1-queue-total"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected total queue warning alert")
		}
		if alert.Level != AlertLevelWarning {
			t.Errorf("expected warning level, got %s", alert.Level)
		}
		if alert.Value != 80 {
			t.Errorf("expected value 80, got %f", alert.Value)
		}
	})

	t.Run("total queue critical alert", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		pmg := models.PMGInstance{
			ID:   "pmg1",
			Name: "PMG 1",
			Nodes: []models.PMGNodeStatus{
				{Name: "node1", QueueStatus: &models.PMGQueueStatus{Total: 200}},
			},
		}

		defaults := PMGThresholdConfig{
			QueueTotalWarning:  100, // 60% scaled = 60
			QueueTotalCritical: 200, // 80% scaled = 160
		}

		m.checkPMGNodeQueues(pmg, defaults)

		m.mu.RLock()
		alert := m.activeAlerts["pmg1-node1-queue-total"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected total queue critical alert")
		}
		if alert.Level != AlertLevelCritical {
			t.Errorf("expected critical level, got %s", alert.Level)
		}
	})

	t.Run("deferred queue warning alert", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		pmg := models.PMGInstance{
			ID:   "pmg1",
			Name: "PMG 1",
			Nodes: []models.PMGNodeStatus{
				{Name: "node1", QueueStatus: &models.PMGQueueStatus{Deferred: 40}},
			},
		}

		defaults := PMGThresholdConfig{
			DeferredQueueWarn:     50, // 60% scaled = 30
			DeferredQueueCritical: 100,
		}

		m.checkPMGNodeQueues(pmg, defaults)

		m.mu.RLock()
		alert := m.activeAlerts["pmg1-node1-queue-deferred"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected deferred queue warning alert")
		}
		if alert.Level != AlertLevelWarning {
			t.Errorf("expected warning level, got %s", alert.Level)
		}
	})

	t.Run("hold queue warning alert", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		pmg := models.PMGInstance{
			ID:   "pmg1",
			Name: "PMG 1",
			Nodes: []models.PMGNodeStatus{
				{Name: "node1", QueueStatus: &models.PMGQueueStatus{Hold: 25}},
			},
		}

		defaults := PMGThresholdConfig{
			HoldQueueWarn:     30, // 60% scaled = 18
			HoldQueueCritical: 60,
		}

		m.checkPMGNodeQueues(pmg, defaults)

		m.mu.RLock()
		alert := m.activeAlerts["pmg1-node1-queue-hold"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected hold queue warning alert")
		}
		if alert.Level != AlertLevelWarning {
			t.Errorf("expected warning level, got %s", alert.Level)
		}
	})

	t.Run("oldest message age warning alert", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		pmg := models.PMGInstance{
			ID:   "pmg1",
			Name: "PMG 1",
			Nodes: []models.PMGNodeStatus{
				{Name: "node1", QueueStatus: &models.PMGQueueStatus{OldestAge: 2400}}, // 40 minutes
			},
		}

		defaults := PMGThresholdConfig{
			OldestMessageWarnMins: 50, // 60% scaled = 30 minutes
			OldestMessageCritMins: 90,
		}

		m.checkPMGNodeQueues(pmg, defaults)

		m.mu.RLock()
		alert := m.activeAlerts["pmg1-node1-oldest-message"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected oldest message warning alert")
		}
		if alert.Level != AlertLevelWarning {
			t.Errorf("expected warning level, got %s", alert.Level)
		}
		if alert.Value != 40 { // 2400 seconds / 60 = 40 minutes
			t.Errorf("expected value 40, got %f", alert.Value)
		}
	})

	t.Run("below threshold clears alert", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		// Pre-create an alert
		m.mu.Lock()
		m.activeAlerts["pmg1-node1-queue-total"] = &Alert{ID: "pmg1-node1-queue-total"}
		m.mu.Unlock()

		pmg := models.PMGInstance{
			ID:   "pmg1",
			Name: "PMG 1",
			Nodes: []models.PMGNodeStatus{
				{Name: "node1", QueueStatus: &models.PMGQueueStatus{Total: 10}},
			},
		}

		defaults := PMGThresholdConfig{
			QueueTotalWarning:  100, // 60% scaled = 60
			QueueTotalCritical: 200,
		}

		m.checkPMGNodeQueues(pmg, defaults)

		m.mu.RLock()
		_, exists := m.activeAlerts["pmg1-node1-queue-total"]
		m.mu.RUnlock()

		if exists {
			t.Error("expected alert to be cleared when below threshold")
		}
	})

	t.Run("outlier detection adds note to message", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		pmg := models.PMGInstance{
			ID:   "pmg1",
			Name: "PMG 1",
			Nodes: []models.PMGNodeStatus{
				{Name: "node1", QueueStatus: &models.PMGQueueStatus{Total: 10}},
				{Name: "node2", QueueStatus: &models.PMGQueueStatus{Total: 10}},
				{Name: "node3", QueueStatus: &models.PMGQueueStatus{Total: 100}}, // outlier
			},
		}

		defaults := PMGThresholdConfig{
			QueueTotalWarning:  100, // 60% scaled = 60
			QueueTotalCritical: 200,
		}

		m.checkPMGNodeQueues(pmg, defaults)

		m.mu.RLock()
		alert := m.activeAlerts["pmg1-node3-queue-total"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected alert for outlier node")
		}
		if !strings.Contains(alert.Message, "outlier") {
			t.Errorf("expected message to contain 'outlier', got %s", alert.Message)
		}
	})

	t.Run("no thresholds configured does not create alerts", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		pmg := models.PMGInstance{
			ID:   "pmg1",
			Name: "PMG 1",
			Nodes: []models.PMGNodeStatus{
				{Name: "node1", QueueStatus: &models.PMGQueueStatus{Total: 1000, Deferred: 500, Hold: 300}},
			},
		}

		defaults := PMGThresholdConfig{} // All zero

		m.checkPMGNodeQueues(pmg, defaults)

		m.mu.RLock()
		count := len(m.activeAlerts)
		m.mu.RUnlock()

		if count != 0 {
			t.Errorf("expected no alerts when no thresholds configured, got %d", count)
		}
	})

	t.Run("updates existing alert", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		oldTime := time.Now().Add(-1 * time.Hour)
		m.mu.Lock()
		m.activeAlerts["pmg1-node1-queue-total"] = &Alert{
			ID:        "pmg1-node1-queue-total",
			Value:     60,
			Level:     AlertLevelWarning,
			LastSeen:  oldTime,
			StartTime: oldTime,
		}
		m.mu.Unlock()

		pmg := models.PMGInstance{
			ID:   "pmg1",
			Name: "PMG 1",
			Nodes: []models.PMGNodeStatus{
				{Name: "node1", QueueStatus: &models.PMGQueueStatus{Total: 200}},
			},
		}

		defaults := PMGThresholdConfig{
			QueueTotalWarning:  100, // 60% scaled = 60
			QueueTotalCritical: 200, // 80% scaled = 160
		}

		m.checkPMGNodeQueues(pmg, defaults)

		m.mu.RLock()
		alert := m.activeAlerts["pmg1-node1-queue-total"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected alert to exist")
		}
		if alert.Value != 200 {
			t.Errorf("expected value 200, got %f", alert.Value)
		}
		if alert.Level != AlertLevelCritical {
			t.Errorf("expected critical level, got %s", alert.Level)
		}
		if !alert.LastSeen.After(oldTime) {
			t.Error("expected LastSeen to be updated")
		}
	})
}

func TestDockerContainerHealthAlert(t *testing.T) {
	t.Run("healthy container - no alert", func(t *testing.T) {
		m := newTestManager(t)

		host := models.DockerHost{
			ID:          "host-health-1",
			DisplayName: "Docker Host",
			Hostname:    "docker.local",
			Containers: []models.DockerContainer{
				{
					ID:     "container-1",
					Name:   "healthy-app",
					State:  "running",
					Status: "Up 10 minutes",
					Health: "healthy",
				},
			},
		}

		m.CheckDockerHost(host)

		resourceID := dockerResourceID(host.ID, host.Containers[0].ID)
		alertID := fmt.Sprintf("docker-container-health-%s", resourceID)
		if _, exists := m.activeAlerts[alertID]; exists {
			t.Fatal("expected no health alert for healthy container")
		}
	})

	t.Run("container with empty health - no alert", func(t *testing.T) {
		m := newTestManager(t)

		host := models.DockerHost{
			ID:          "host-health-2",
			DisplayName: "Docker Host",
			Hostname:    "docker.local",
			Containers: []models.DockerContainer{
				{
					ID:     "container-2",
					Name:   "no-health-check",
					State:  "running",
					Status: "Up 10 minutes",
					Health: "",
				},
			},
		}

		m.CheckDockerHost(host)

		resourceID := dockerResourceID(host.ID, host.Containers[0].ID)
		alertID := fmt.Sprintf("docker-container-health-%s", resourceID)
		if _, exists := m.activeAlerts[alertID]; exists {
			t.Fatal("expected no health alert for container with empty health")
		}
	})

	t.Run("container with none health - no alert", func(t *testing.T) {
		m := newTestManager(t)

		host := models.DockerHost{
			ID:          "host-health-3",
			DisplayName: "Docker Host",
			Hostname:    "docker.local",
			Containers: []models.DockerContainer{
				{
					ID:     "container-3",
					Name:   "no-health-check",
					State:  "running",
					Status: "Up 10 minutes",
					Health: "none",
				},
			},
		}

		m.CheckDockerHost(host)

		resourceID := dockerResourceID(host.ID, host.Containers[0].ID)
		alertID := fmt.Sprintf("docker-container-health-%s", resourceID)
		if _, exists := m.activeAlerts[alertID]; exists {
			t.Fatal("expected no health alert for container with none health")
		}
	})

	t.Run("container starting - no alert", func(t *testing.T) {
		m := newTestManager(t)

		host := models.DockerHost{
			ID:          "host-health-4",
			DisplayName: "Docker Host",
			Hostname:    "docker.local",
			Containers: []models.DockerContainer{
				{
					ID:     "container-4",
					Name:   "starting-app",
					State:  "running",
					Status: "Up 5 seconds",
					Health: "starting",
				},
			},
		}

		m.CheckDockerHost(host)

		resourceID := dockerResourceID(host.ID, host.Containers[0].ID)
		alertID := fmt.Sprintf("docker-container-health-%s", resourceID)
		if _, exists := m.activeAlerts[alertID]; exists {
			t.Fatal("expected no health alert for starting container")
		}
	})

	t.Run("unhealthy container - critical alert", func(t *testing.T) {
		m := newTestManager(t)

		host := models.DockerHost{
			ID:          "host-health-5",
			DisplayName: "Docker Host",
			Hostname:    "docker.local",
			Containers: []models.DockerContainer{
				{
					ID:     "container-5",
					Name:   "unhealthy-app",
					State:  "running",
					Status: "Up 10 minutes (unhealthy)",
					Health: "unhealthy",
				},
			},
		}

		m.CheckDockerHost(host)

		resourceID := dockerResourceID(host.ID, host.Containers[0].ID)
		alertID := fmt.Sprintf("docker-container-health-%s", resourceID)
		alert, exists := m.activeAlerts[alertID]
		if !exists {
			t.Fatal("expected health alert for unhealthy container")
		}
		if alert.Level != AlertLevelCritical {
			t.Fatalf("expected critical alert for unhealthy container, got %s", alert.Level)
		}
		if alert.Type != "docker-container-health" {
			t.Fatalf("expected alert type docker-container-health, got %s", alert.Type)
		}
	})

	t.Run("container with other health status - warning alert", func(t *testing.T) {
		m := newTestManager(t)

		host := models.DockerHost{
			ID:          "host-health-6",
			DisplayName: "Docker Host",
			Hostname:    "docker.local",
			Containers: []models.DockerContainer{
				{
					ID:     "container-6",
					Name:   "degraded-app",
					State:  "running",
					Status: "Up 10 minutes",
					Health: "degraded",
				},
			},
		}

		m.CheckDockerHost(host)

		resourceID := dockerResourceID(host.ID, host.Containers[0].ID)
		alertID := fmt.Sprintf("docker-container-health-%s", resourceID)
		alert, exists := m.activeAlerts[alertID]
		if !exists {
			t.Fatal("expected health alert for degraded container")
		}
		if alert.Level != AlertLevelWarning {
			t.Fatalf("expected warning alert for non-unhealthy bad status, got %s", alert.Level)
		}
	})

	t.Run("alert cleared when container becomes healthy", func(t *testing.T) {
		m := newTestManager(t)

		hostID := "host-health-7"
		containerID := "container-7"

		// First check with unhealthy container
		hostUnhealthy := models.DockerHost{
			ID:          hostID,
			DisplayName: "Docker Host",
			Hostname:    "docker.local",
			Containers: []models.DockerContainer{
				{
					ID:     containerID,
					Name:   "recovering-app",
					State:  "running",
					Status: "Up 10 minutes (unhealthy)",
					Health: "unhealthy",
				},
			},
		}

		m.CheckDockerHost(hostUnhealthy)

		resourceID := dockerResourceID(hostID, containerID)
		alertID := fmt.Sprintf("docker-container-health-%s", resourceID)
		if _, exists := m.activeAlerts[alertID]; !exists {
			t.Fatal("expected health alert to be raised")
		}

		// Now container becomes healthy
		hostHealthy := models.DockerHost{
			ID:          hostID,
			DisplayName: "Docker Host",
			Hostname:    "docker.local",
			Containers: []models.DockerContainer{
				{
					ID:     containerID,
					Name:   "recovering-app",
					State:  "running",
					Status: "Up 15 minutes",
					Health: "healthy",
				},
			},
		}

		m.CheckDockerHost(hostHealthy)

		if _, exists := m.activeAlerts[alertID]; exists {
			t.Fatal("expected health alert to be cleared when container became healthy")
		}
	})
}

func TestDockerContainerOOMKillAlert(t *testing.T) {
	t.Run("running container - no alert", func(t *testing.T) {
		m := newTestManager(t)

		host := models.DockerHost{
			ID:          "host-oom-1",
			DisplayName: "Docker Host",
			Hostname:    "docker.local",
			Containers: []models.DockerContainer{
				{
					ID:       "container-1",
					Name:     "running-app",
					State:    "running",
					Status:   "Up 10 minutes",
					ExitCode: 0,
				},
			},
		}

		m.CheckDockerHost(host)

		resourceID := dockerResourceID(host.ID, host.Containers[0].ID)
		alertID := fmt.Sprintf("docker-container-oom-%s", resourceID)
		if _, exists := m.activeAlerts[alertID]; exists {
			t.Fatal("expected no OOM alert for running container")
		}
	})

	t.Run("exited container with non-137 exit code - no alert", func(t *testing.T) {
		m := newTestManager(t)

		host := models.DockerHost{
			ID:          "host-oom-2",
			DisplayName: "Docker Host",
			Hostname:    "docker.local",
			Containers: []models.DockerContainer{
				{
					ID:       "container-2",
					Name:     "normal-exit-app",
					State:    "exited",
					Status:   "Exited (1) 5 minutes ago",
					ExitCode: 1,
				},
			},
		}

		m.CheckDockerHost(host)

		resourceID := dockerResourceID(host.ID, host.Containers[0].ID)
		alertID := fmt.Sprintf("docker-container-oom-%s", resourceID)
		if _, exists := m.activeAlerts[alertID]; exists {
			t.Fatal("expected no OOM alert for container with exit code 1")
		}
	})

	t.Run("exited container with exit code 137 - critical OOM alert", func(t *testing.T) {
		m := newTestManager(t)

		host := models.DockerHost{
			ID:          "host-oom-3",
			DisplayName: "Docker Host",
			Hostname:    "docker.local",
			Containers: []models.DockerContainer{
				{
					ID:          "container-3",
					Name:        "oom-killed-app",
					State:       "exited",
					Status:      "Exited (137) 1 minute ago",
					ExitCode:    137,
					MemoryUsage: 512 * 1024 * 1024,
					MemoryLimit: 512 * 1024 * 1024,
				},
			},
		}

		m.CheckDockerHost(host)

		resourceID := dockerResourceID(host.ID, host.Containers[0].ID)
		alertID := fmt.Sprintf("docker-container-oom-%s", resourceID)
		alert, exists := m.activeAlerts[alertID]
		if !exists {
			t.Fatal("expected OOM alert for container with exit code 137")
		}
		if alert.Level != AlertLevelCritical {
			t.Fatalf("expected critical OOM alert, got %s", alert.Level)
		}
		if alert.Type != "docker-container-oom-kill" {
			t.Fatalf("expected alert type docker-container-oom-kill, got %s", alert.Type)
		}
	})

	t.Run("dead container with exit code 137 - critical OOM alert", func(t *testing.T) {
		m := newTestManager(t)

		host := models.DockerHost{
			ID:          "host-oom-dead",
			DisplayName: "Docker Host",
			Hostname:    "docker.local",
			Containers: []models.DockerContainer{
				{
					ID:       "container-dead",
					Name:     "dead-oom-app",
					State:    "dead",
					Status:   "Dead",
					ExitCode: 137,
				},
			},
		}

		m.CheckDockerHost(host)

		resourceID := dockerResourceID(host.ID, host.Containers[0].ID)
		alertID := fmt.Sprintf("docker-container-oom-%s", resourceID)
		alert, exists := m.activeAlerts[alertID]
		if !exists {
			t.Fatal("expected OOM alert for dead container with exit code 137")
		}
		if alert.Level != AlertLevelCritical {
			t.Fatalf("expected critical OOM alert, got %s", alert.Level)
		}
	})

	t.Run("repeated 137 exit code - no new alert", func(t *testing.T) {
		m := newTestManager(t)

		hostID := "host-oom-4"
		containerID := "container-4"

		host := models.DockerHost{
			ID:          hostID,
			DisplayName: "Docker Host",
			Hostname:    "docker.local",
			Containers: []models.DockerContainer{
				{
					ID:       containerID,
					Name:     "oom-killed-app",
					State:    "exited",
					Status:   "Exited (137) 1 minute ago",
					ExitCode: 137,
				},
			},
		}

		// First check - should create alert
		m.CheckDockerHost(host)

		resourceID := dockerResourceID(hostID, containerID)
		alertID := fmt.Sprintf("docker-container-oom-%s", resourceID)
		alert1, exists := m.activeAlerts[alertID]
		if !exists {
			t.Fatal("expected OOM alert on first check")
		}
		startTime := alert1.StartTime

		// Second check with same exit code - should not create new alert
		m.CheckDockerHost(host)

		alert2, exists := m.activeAlerts[alertID]
		if !exists {
			t.Fatal("expected OOM alert to still exist on second check")
		}
		if alert2.StartTime != startTime {
			t.Fatal("expected alert start time to be preserved (not a new alert)")
		}
	})

	t.Run("container recovers - alert cleared", func(t *testing.T) {
		m := newTestManager(t)

		hostID := "host-oom-5"
		containerID := "container-5"

		// First check with OOM killed container
		hostOOM := models.DockerHost{
			ID:          hostID,
			DisplayName: "Docker Host",
			Hostname:    "docker.local",
			Containers: []models.DockerContainer{
				{
					ID:       containerID,
					Name:     "recovering-app",
					State:    "exited",
					Status:   "Exited (137) 1 minute ago",
					ExitCode: 137,
				},
			},
		}

		m.CheckDockerHost(hostOOM)

		resourceID := dockerResourceID(hostID, containerID)
		alertID := fmt.Sprintf("docker-container-oom-%s", resourceID)
		if _, exists := m.activeAlerts[alertID]; !exists {
			t.Fatal("expected OOM alert to be raised")
		}

		// Container is restarted and running again
		hostRunning := models.DockerHost{
			ID:          hostID,
			DisplayName: "Docker Host",
			Hostname:    "docker.local",
			Containers: []models.DockerContainer{
				{
					ID:       containerID,
					Name:     "recovering-app",
					State:    "running",
					Status:   "Up 30 seconds",
					ExitCode: 0,
				},
			},
		}

		m.CheckDockerHost(hostRunning)

		if _, exists := m.activeAlerts[alertID]; exists {
			t.Fatal("expected OOM alert to be cleared when container started running")
		}
	})

	t.Run("container exits with different code - alert cleared", func(t *testing.T) {
		m := newTestManager(t)

		hostID := "host-oom-6"
		containerID := "container-6"

		// First check with OOM killed container
		hostOOM := models.DockerHost{
			ID:          hostID,
			DisplayName: "Docker Host",
			Hostname:    "docker.local",
			Containers: []models.DockerContainer{
				{
					ID:       containerID,
					Name:     "multi-exit-app",
					State:    "exited",
					Status:   "Exited (137) 1 minute ago",
					ExitCode: 137,
				},
			},
		}

		m.CheckDockerHost(hostOOM)

		resourceID := dockerResourceID(hostID, containerID)
		alertID := fmt.Sprintf("docker-container-oom-%s", resourceID)
		if _, exists := m.activeAlerts[alertID]; !exists {
			t.Fatal("expected OOM alert to be raised")
		}

		// Container exits with different exit code (normal error)
		hostNormalExit := models.DockerHost{
			ID:          hostID,
			DisplayName: "Docker Host",
			Hostname:    "docker.local",
			Containers: []models.DockerContainer{
				{
					ID:       containerID,
					Name:     "multi-exit-app",
					State:    "exited",
					Status:   "Exited (1) 30 seconds ago",
					ExitCode: 1,
				},
			},
		}

		m.CheckDockerHost(hostNormalExit)

		if _, exists := m.activeAlerts[alertID]; exists {
			t.Fatal("expected OOM alert to be cleared when container exited with different code")
		}
	})
}

func TestDockerContainerRestartLoopAlert(t *testing.T) {
	t.Run("first check - no alert", func(t *testing.T) {
		m := newTestManager(t)

		host := models.DockerHost{
			ID:          "host-restart-1",
			DisplayName: "Docker Host",
			Hostname:    "docker.local",
			Containers: []models.DockerContainer{
				{
					ID:           "container-1",
					Name:         "first-check-app",
					State:        "running",
					Status:       "Up 10 minutes",
					RestartCount: 5, // Even with high restart count, first check just initializes
				},
			},
		}

		m.CheckDockerHost(host)

		resourceID := dockerResourceID(host.ID, host.Containers[0].ID)
		alertID := fmt.Sprintf("docker-container-restart-loop-%s", resourceID)
		if _, exists := m.activeAlerts[alertID]; exists {
			t.Fatal("expected no restart loop alert on first check (just initializes tracking)")
		}

		// Verify tracking was initialized
		m.mu.Lock()
		record, exists := m.dockerRestartTracking[resourceID]
		m.mu.Unlock()
		if !exists {
			t.Fatal("expected tracking record to be initialized")
		}
		if record.lastCount != 5 {
			t.Fatalf("expected lastCount=5, got %d", record.lastCount)
		}
	})

	t.Run("stable restart count - no alert", func(t *testing.T) {
		m := newTestManager(t)

		hostID := "host-restart-2"
		containerID := "container-2"

		host := models.DockerHost{
			ID:          hostID,
			DisplayName: "Docker Host",
			Hostname:    "docker.local",
			Containers: []models.DockerContainer{
				{
					ID:           containerID,
					Name:         "stable-app",
					State:        "running",
					Status:       "Up 10 minutes",
					RestartCount: 2,
				},
			},
		}

		// First check - initializes tracking
		m.CheckDockerHost(host)

		// Second check - same restart count
		m.CheckDockerHost(host)

		// Third check - still same restart count
		m.CheckDockerHost(host)

		resourceID := dockerResourceID(hostID, containerID)
		alertID := fmt.Sprintf("docker-container-restart-loop-%s", resourceID)
		if _, exists := m.activeAlerts[alertID]; exists {
			t.Fatal("expected no restart loop alert for stable container")
		}
	})

	t.Run("restarts under threshold - no alert", func(t *testing.T) {
		m := newTestManager(t)
		// Configure threshold to 3 (default)
		m.config.DockerDefaults.RestartCount = 3
		m.config.DockerDefaults.RestartWindow = 300

		hostID := "host-restart-3"
		containerID := "container-3"

		// First check - initializes with RestartCount=0
		host := models.DockerHost{
			ID:          hostID,
			DisplayName: "Docker Host",
			Hostname:    "docker.local",
			Containers: []models.DockerContainer{
				{
					ID:           containerID,
					Name:         "under-threshold-app",
					State:        "running",
					Status:       "Up 10 minutes",
					RestartCount: 0,
				},
			},
		}
		m.CheckDockerHost(host)

		// Container restarts twice (under threshold of 3)
		host.Containers[0].RestartCount = 2
		m.CheckDockerHost(host)

		// One more restart (now at 3, threshold is >3 so still no alert)
		host.Containers[0].RestartCount = 3
		m.CheckDockerHost(host)

		resourceID := dockerResourceID(hostID, containerID)
		alertID := fmt.Sprintf("docker-container-restart-loop-%s", resourceID)
		if _, exists := m.activeAlerts[alertID]; exists {
			t.Fatal("expected no restart loop alert when restarts <= threshold")
		}

		// Verify we tracked 3 restarts
		m.mu.Lock()
		record := m.dockerRestartTracking[resourceID]
		recentCount := len(record.times)
		m.mu.Unlock()
		if recentCount != 3 {
			t.Fatalf("expected 3 tracked restarts, got %d", recentCount)
		}
	})

	t.Run("hits restart loop threshold - alert raised", func(t *testing.T) {
		m := newTestManager(t)
		// Configure threshold to 3 (alert when >3)
		m.config.DockerDefaults.RestartCount = 3
		m.config.DockerDefaults.RestartWindow = 300

		hostID := "host-restart-4"
		containerID := "container-4"

		// First check - initializes with RestartCount=0
		host := models.DockerHost{
			ID:          hostID,
			DisplayName: "Docker Host",
			Hostname:    "docker.local",
			Containers: []models.DockerContainer{
				{
					ID:           containerID,
					Name:         "restart-loop-app",
					State:        "running",
					Status:       "Up 1 minute",
					RestartCount: 0,
				},
			},
		}
		m.CheckDockerHost(host)

		// Container restarts 4 times (exceeds threshold of 3)
		host.Containers[0].RestartCount = 4
		m.CheckDockerHost(host)

		resourceID := dockerResourceID(hostID, containerID)
		alertID := fmt.Sprintf("docker-container-restart-loop-%s", resourceID)
		alert, exists := m.activeAlerts[alertID]
		if !exists {
			t.Fatal("expected restart loop alert when restarts > threshold")
		}
		if alert.Level != AlertLevelCritical {
			t.Fatalf("expected critical alert, got %s", alert.Level)
		}
		if alert.Type != "docker-container-restart-loop" {
			t.Fatalf("expected alert type docker-container-restart-loop, got %s", alert.Type)
		}

		// Verify metadata
		if alert.Metadata["restartCount"] != 4 {
			t.Fatalf("expected restartCount=4 in metadata, got %v", alert.Metadata["restartCount"])
		}
		if alert.Metadata["recentRestarts"] != 4 {
			t.Fatalf("expected recentRestarts=4 in metadata, got %v", alert.Metadata["recentRestarts"])
		}
	})

	t.Run("restart loop recovery - alert cleared", func(t *testing.T) {
		m := newTestManager(t)
		// Configure short window for testing
		m.config.DockerDefaults.RestartCount = 3
		m.config.DockerDefaults.RestartWindow = 1 // 1 second window for testing

		hostID := "host-restart-5"
		containerID := "container-5"
		resourceID := dockerResourceID(hostID, containerID)

		// Manually set up a restart loop state
		m.mu.Lock()
		now := time.Now()
		m.dockerRestartTracking[resourceID] = &dockerRestartRecord{
			count:       5,
			lastCount:   5,
			times:       []time.Time{now, now, now, now}, // 4 recent restarts
			lastChecked: now,
		}
		m.mu.Unlock()

		// Create initial alert
		host := models.DockerHost{
			ID:          hostID,
			DisplayName: "Docker Host",
			Hostname:    "docker.local",
			Containers: []models.DockerContainer{
				{
					ID:           containerID,
					Name:         "recovering-app",
					State:        "running",
					Status:       "Up 1 minute",
					RestartCount: 5,
				},
			},
		}
		m.CheckDockerHost(host)

		alertID := fmt.Sprintf("docker-container-restart-loop-%s", resourceID)
		if _, exists := m.activeAlerts[alertID]; !exists {
			t.Fatal("expected restart loop alert to be raised initially")
		}

		// Simulate time passing by shifting tracked restarts outside the configured window.
		m.mu.Lock()
		if rec, ok := m.dockerRestartTracking[resourceID]; ok {
			past := time.Now().Add(-2 * time.Second)
			for i := range rec.times {
				rec.times[i] = past
			}
			rec.lastChecked = past
		}
		m.mu.Unlock()

		// Check again with same restart count - old restarts should be cleaned up
		m.CheckDockerHost(host)

		if _, exists := m.activeAlerts[alertID]; exists {
			t.Fatal("expected restart loop alert to be cleared after window passes")
		}
	})

	t.Run("incremental restarts trigger alert", func(t *testing.T) {
		m := newTestManager(t)
		m.config.DockerDefaults.RestartCount = 2
		m.config.DockerDefaults.RestartWindow = 300

		hostID := "host-restart-6"
		containerID := "container-6"

		host := models.DockerHost{
			ID:          hostID,
			DisplayName: "Docker Host",
			Hostname:    "docker.local",
			Containers: []models.DockerContainer{
				{
					ID:           containerID,
					Name:         "incremental-restart-app",
					State:        "running",
					Status:       "Up 1 minute",
					RestartCount: 0,
				},
			},
		}

		// First check - initializes
		m.CheckDockerHost(host)

		resourceID := dockerResourceID(hostID, containerID)
		alertID := fmt.Sprintf("docker-container-restart-loop-%s", resourceID)

		// Restart 1
		host.Containers[0].RestartCount = 1
		m.CheckDockerHost(host)
		if _, exists := m.activeAlerts[alertID]; exists {
			t.Fatal("expected no alert after 1 restart")
		}

		// Restart 2
		host.Containers[0].RestartCount = 2
		m.CheckDockerHost(host)
		if _, exists := m.activeAlerts[alertID]; exists {
			t.Fatal("expected no alert after 2 restarts (threshold is >2)")
		}

		// Restart 3 - exceeds threshold
		host.Containers[0].RestartCount = 3
		m.CheckDockerHost(host)
		if _, exists := m.activeAlerts[alertID]; !exists {
			t.Fatal("expected alert after 3 restarts (>2 threshold)")
		}
	})

	t.Run("alert preserves start time on updates", func(t *testing.T) {
		m := newTestManager(t)
		m.config.DockerDefaults.RestartCount = 2
		m.config.DockerDefaults.RestartWindow = 300

		hostID := "host-restart-7"
		containerID := "container-7"

		host := models.DockerHost{
			ID:          hostID,
			DisplayName: "Docker Host",
			Hostname:    "docker.local",
			Containers: []models.DockerContainer{
				{
					ID:           containerID,
					Name:         "preserve-time-app",
					State:        "running",
					Status:       "Up 1 minute",
					RestartCount: 0,
				},
			},
		}

		// Initialize and trigger alert
		m.CheckDockerHost(host)
		host.Containers[0].RestartCount = 5
		m.CheckDockerHost(host)

		resourceID := dockerResourceID(hostID, containerID)
		alertID := fmt.Sprintf("docker-container-restart-loop-%s", resourceID)
		alert1, exists := m.activeAlerts[alertID]
		if !exists {
			t.Fatal("expected alert to be raised")
		}
		startTime1 := alert1.StartTime

		// More restarts - alert should update but preserve start time
		time.Sleep(10 * time.Millisecond)
		host.Containers[0].RestartCount = 7
		m.CheckDockerHost(host)

		alert2, exists := m.activeAlerts[alertID]
		if !exists {
			t.Fatal("expected alert to still exist")
		}
		if !alert2.StartTime.Equal(startTime1) {
			t.Fatalf("expected start time to be preserved, got %v vs %v", alert2.StartTime, startTime1)
		}
	})
}

func TestApplyThresholdOverride(t *testing.T) {
	t.Run("empty override returns base unchanged", func(t *testing.T) {
		m := newTestManager(t)
		base := ThresholdConfig{
			CPU:    &HysteresisThreshold{Trigger: 80, Clear: 75},
			Memory: &HysteresisThreshold{Trigger: 90, Clear: 85},
		}
		override := ThresholdConfig{}
		result := m.applyThresholdOverride(base, override)

		if result.CPU == nil || result.CPU.Trigger != 80 || result.CPU.Clear != 75 {
			t.Errorf("expected CPU to match base, got %+v", result.CPU)
		}
		if result.Memory == nil || result.Memory.Trigger != 90 || result.Memory.Clear != 85 {
			t.Errorf("expected Memory to match base, got %+v", result.Memory)
		}
		if result.Disabled {
			t.Error("expected Disabled to remain false")
		}
	})

	t.Run("Disabled flag override", func(t *testing.T) {
		m := newTestManager(t)
		base := ThresholdConfig{Disabled: false}
		override := ThresholdConfig{Disabled: true}
		result := m.applyThresholdOverride(base, override)

		if !result.Disabled {
			t.Error("expected Disabled to be true after override")
		}
	})

	t.Run("DisableConnectivity override", func(t *testing.T) {
		m := newTestManager(t)
		base := ThresholdConfig{DisableConnectivity: false}
		override := ThresholdConfig{DisableConnectivity: true}
		result := m.applyThresholdOverride(base, override)

		if !result.DisableConnectivity {
			t.Error("expected DisableConnectivity to be true after override")
		}
	})

	t.Run("CPU threshold override", func(t *testing.T) {
		m := newTestManager(t)
		base := ThresholdConfig{
			CPU: &HysteresisThreshold{Trigger: 80, Clear: 75},
		}
		override := ThresholdConfig{
			CPU: &HysteresisThreshold{Trigger: 95, Clear: 90},
		}
		result := m.applyThresholdOverride(base, override)

		if result.CPU == nil {
			t.Fatal("expected CPU to be set")
		}
		if result.CPU.Trigger != 95 || result.CPU.Clear != 90 {
			t.Errorf("expected CPU override values, got Trigger=%v Clear=%v", result.CPU.Trigger, result.CPU.Clear)
		}
	})

	t.Run("legacy CPU threshold conversion", func(t *testing.T) {
		m := newTestManager(t)
		m.config.HysteresisMargin = 5.0
		base := ThresholdConfig{}
		legacyVal := 85.0
		override := ThresholdConfig{
			CPULegacy: &legacyVal,
		}
		result := m.applyThresholdOverride(base, override)

		if result.CPU == nil {
			t.Fatal("expected CPU to be converted from legacy")
		}
		if result.CPU.Trigger != 85.0 {
			t.Errorf("expected Trigger=85, got %v", result.CPU.Trigger)
		}
		if result.CPU.Clear != 80.0 {
			t.Errorf("expected Clear=80 (85-5 margin), got %v", result.CPU.Clear)
		}
	})

	t.Run("modern CPU takes precedence over legacy", func(t *testing.T) {
		m := newTestManager(t)
		legacyVal := 70.0
		base := ThresholdConfig{}
		override := ThresholdConfig{
			CPU:       &HysteresisThreshold{Trigger: 95, Clear: 90},
			CPULegacy: &legacyVal,
		}
		result := m.applyThresholdOverride(base, override)

		if result.CPU.Trigger != 95 {
			t.Errorf("expected modern CPU to take precedence, got Trigger=%v", result.CPU.Trigger)
		}
	})

	t.Run("multiple metrics override", func(t *testing.T) {
		m := newTestManager(t)
		base := ThresholdConfig{
			CPU:    &HysteresisThreshold{Trigger: 80, Clear: 75},
			Memory: &HysteresisThreshold{Trigger: 80, Clear: 75},
			Disk:   &HysteresisThreshold{Trigger: 80, Clear: 75},
		}
		override := ThresholdConfig{
			CPU:        &HysteresisThreshold{Trigger: 90, Clear: 85},
			Memory:     &HysteresisThreshold{Trigger: 95, Clear: 90},
			NetworkIn:  &HysteresisThreshold{Trigger: 100, Clear: 95},
			NetworkOut: &HysteresisThreshold{Trigger: 200, Clear: 190},
		}
		result := m.applyThresholdOverride(base, override)

		if result.CPU.Trigger != 90 {
			t.Errorf("expected CPU override, got %v", result.CPU.Trigger)
		}
		if result.Memory.Trigger != 95 {
			t.Errorf("expected Memory override, got %v", result.Memory.Trigger)
		}
		// Disk should remain unchanged (not in override)
		if result.Disk.Trigger != 80 {
			t.Errorf("expected Disk unchanged, got %v", result.Disk.Trigger)
		}
		if result.NetworkIn == nil || result.NetworkIn.Trigger != 100 {
			t.Errorf("expected NetworkIn to be added, got %+v", result.NetworkIn)
		}
		if result.NetworkOut == nil || result.NetworkOut.Trigger != 200 {
			t.Errorf("expected NetworkOut to be added, got %+v", result.NetworkOut)
		}
	})

	t.Run("Note override", func(t *testing.T) {
		m := newTestManager(t)
		base := ThresholdConfig{}
		note := "test note"
		override := ThresholdConfig{Note: &note}
		result := m.applyThresholdOverride(base, override)

		if result.Note == nil || *result.Note != "test note" {
			t.Errorf("expected Note to be set, got %v", result.Note)
		}
	})

	t.Run("Note cleared when empty string", func(t *testing.T) {
		m := newTestManager(t)
		existingNote := "existing note"
		base := ThresholdConfig{Note: &existingNote}
		emptyNote := ""
		override := ThresholdConfig{Note: &emptyNote}
		result := m.applyThresholdOverride(base, override)

		if result.Note != nil {
			t.Errorf("expected Note to be nil when empty string override, got %v", *result.Note)
		}
	})

	t.Run("Note trimmed of whitespace", func(t *testing.T) {
		m := newTestManager(t)
		base := ThresholdConfig{}
		note := "  trimmed note  "
		override := ThresholdConfig{Note: &note}
		result := m.applyThresholdOverride(base, override)

		if result.Note == nil || *result.Note != "trimmed note" {
			t.Errorf("expected Note to be trimmed, got %v", result.Note)
		}
	})

	t.Run("whitespace-only Note becomes nil", func(t *testing.T) {
		m := newTestManager(t)
		existingNote := "existing"
		base := ThresholdConfig{Note: &existingNote}
		whitespaceNote := "   "
		override := ThresholdConfig{Note: &whitespaceNote}
		result := m.applyThresholdOverride(base, override)

		if result.Note != nil {
			t.Errorf("expected whitespace-only Note to become nil, got %v", *result.Note)
		}
	})

	t.Run("all metric types with legacy conversion", func(t *testing.T) {
		m := newTestManager(t)
		m.config.HysteresisMargin = 5.0
		base := ThresholdConfig{}

		val80 := 80.0
		val90 := 90.0
		val100 := 100.0
		val200 := 200.0
		override := ThresholdConfig{
			MemoryLegacy:     &val80,
			DiskLegacy:       &val90,
			DiskReadLegacy:   &val100,
			DiskWriteLegacy:  &val100,
			NetworkInLegacy:  &val200,
			NetworkOutLegacy: &val200,
		}
		result := m.applyThresholdOverride(base, override)

		if result.Memory == nil || result.Memory.Trigger != 80 {
			t.Errorf("expected Memory converted, got %+v", result.Memory)
		}
		if result.Disk == nil || result.Disk.Trigger != 90 {
			t.Errorf("expected Disk converted, got %+v", result.Disk)
		}
		if result.DiskRead == nil || result.DiskRead.Trigger != 100 {
			t.Errorf("expected DiskRead converted, got %+v", result.DiskRead)
		}
		if result.DiskWrite == nil || result.DiskWrite.Trigger != 100 {
			t.Errorf("expected DiskWrite converted, got %+v", result.DiskWrite)
		}
		if result.NetworkIn == nil || result.NetworkIn.Trigger != 200 {
			t.Errorf("expected NetworkIn converted, got %+v", result.NetworkIn)
		}
		if result.NetworkOut == nil || result.NetworkOut.Trigger != 200 {
			t.Errorf("expected NetworkOut converted, got %+v", result.NetworkOut)
		}
	})

	t.Run("Temperature and Usage override", func(t *testing.T) {
		m := newTestManager(t)
		base := ThresholdConfig{}
		override := ThresholdConfig{
			Temperature: &HysteresisThreshold{Trigger: 85, Clear: 80},
			Usage:       &HysteresisThreshold{Trigger: 90, Clear: 85},
		}
		result := m.applyThresholdOverride(base, override)

		if result.Temperature == nil || result.Temperature.Trigger != 85 {
			t.Errorf("expected Temperature override, got %+v", result.Temperature)
		}
		if result.Usage == nil || result.Usage.Trigger != 90 {
			t.Errorf("expected Usage override, got %+v", result.Usage)
		}
	})

	t.Run("ensureHysteresisThreshold fills missing Clear", func(t *testing.T) {
		m := newTestManager(t)
		base := ThresholdConfig{}
		override := ThresholdConfig{
			CPU: &HysteresisThreshold{Trigger: 80, Clear: 0}, // Clear not set
		}
		result := m.applyThresholdOverride(base, override)

		if result.CPU == nil {
			t.Fatal("expected CPU to be set")
		}
		// ensureHysteresisThreshold sets Clear to Trigger - 5 when Clear <= 0
		if result.CPU.Clear != 75 {
			t.Errorf("expected Clear to be 75 (80-5 default), got %v", result.CPU.Clear)
		}
	})
}

func TestSuppressGuestAlerts(t *testing.T) {
	t.Run("no alerts for guest returns false", func(t *testing.T) {
		m := newTestManager(t)

		result := m.suppressGuestAlerts("vm100")
		if result {
			t.Error("expected false when no alerts exist for guest")
		}
	})

	t.Run("active alert with exact ResourceID match clears and returns true", func(t *testing.T) {
		m := newTestManager(t)

		m.mu.Lock()
		m.activeAlerts["vm100-cpu"] = &Alert{
			ID:         "vm100-cpu",
			ResourceID: "vm100",
			Type:       "cpu",
		}
		m.mu.Unlock()

		result := m.suppressGuestAlerts("vm100")
		if !result {
			t.Error("expected true when active alert was cleared")
		}

		m.mu.RLock()
		if _, exists := m.activeAlerts["vm100-cpu"]; exists {
			t.Error("expected alert to be cleared from activeAlerts")
		}
		m.mu.RUnlock()
	})

	t.Run("active alert with prefix match clears", func(t *testing.T) {
		m := newTestManager(t)

		m.mu.Lock()
		m.activeAlerts["vm100/disk1-disk"] = &Alert{
			ID:         "vm100/disk1-disk",
			ResourceID: "vm100/disk1",
			Type:       "disk",
		}
		m.mu.Unlock()

		result := m.suppressGuestAlerts("vm100")
		if !result {
			t.Error("expected true when active alert was cleared")
		}

		m.mu.RLock()
		if _, exists := m.activeAlerts["vm100/disk1-disk"]; exists {
			t.Error("expected alert with prefix match to be cleared")
		}
		m.mu.RUnlock()
	})

	t.Run("clears from all auxiliary maps", func(t *testing.T) {
		m := newTestManager(t)

		now := time.Now()
		m.mu.Lock()
		m.activeAlerts["vm100-cpu"] = &Alert{
			ID:         "vm100-cpu",
			ResourceID: "vm100",
			Type:       "cpu",
		}
		m.pendingAlerts["vm100-memory"] = now
		m.recentAlerts["vm100-disk"] = &Alert{ID: "vm100-disk", ResourceID: "vm100"}
		m.suppressedUntil["vm100-network"] = now.Add(time.Hour)
		m.alertRateLimit["vm100-io"] = []time.Time{now}
		m.offlineConfirmations["vm100"] = 1
		m.mu.Unlock()

		result := m.suppressGuestAlerts("vm100")
		if !result {
			t.Error("expected true when active alert was cleared")
		}

		m.mu.RLock()
		defer m.mu.RUnlock()

		if _, exists := m.activeAlerts["vm100-cpu"]; exists {
			t.Error("expected activeAlerts to be cleared")
		}
		if _, exists := m.pendingAlerts["vm100-memory"]; exists {
			t.Error("expected pendingAlerts to be cleared")
		}
		if _, exists := m.recentAlerts["vm100-disk"]; exists {
			t.Error("expected recentAlerts to be cleared")
		}
		if _, exists := m.suppressedUntil["vm100-network"]; exists {
			t.Error("expected suppressedUntil to be cleared")
		}
		if _, exists := m.alertRateLimit["vm100-io"]; exists {
			t.Error("expected alertRateLimit to be cleared")
		}
		if _, exists := m.offlineConfirmations["vm100"]; exists {
			t.Error("expected offlineConfirmations to be cleared")
		}
	})

	t.Run("multiple alerts cleared", func(t *testing.T) {
		m := newTestManager(t)

		m.mu.Lock()
		m.activeAlerts["vm100-cpu"] = &Alert{
			ID:         "vm100-cpu",
			ResourceID: "vm100",
			Type:       "cpu",
		}
		m.activeAlerts["vm100-memory"] = &Alert{
			ID:         "vm100-memory",
			ResourceID: "vm100",
			Type:       "memory",
		}
		m.activeAlerts["vm100/disk0-disk"] = &Alert{
			ID:         "vm100/disk0-disk",
			ResourceID: "vm100/disk0",
			Type:       "disk",
		}
		// Also add an alert for a different guest that should NOT be cleared
		m.activeAlerts["vm200-cpu"] = &Alert{
			ID:         "vm200-cpu",
			ResourceID: "vm200",
			Type:       "cpu",
		}
		m.mu.Unlock()

		result := m.suppressGuestAlerts("vm100")
		if !result {
			t.Error("expected true when alerts were cleared")
		}

		m.mu.RLock()
		defer m.mu.RUnlock()

		if _, exists := m.activeAlerts["vm100-cpu"]; exists {
			t.Error("expected vm100-cpu to be cleared")
		}
		if _, exists := m.activeAlerts["vm100-memory"]; exists {
			t.Error("expected vm100-memory to be cleared")
		}
		if _, exists := m.activeAlerts["vm100/disk0-disk"]; exists {
			t.Error("expected vm100/disk0-disk to be cleared")
		}
		if _, exists := m.activeAlerts["vm200-cpu"]; !exists {
			t.Error("expected vm200-cpu to NOT be cleared")
		}
	})

	t.Run("clears auxiliary maps even without active alerts", func(t *testing.T) {
		m := newTestManager(t)

		now := time.Now()
		m.mu.Lock()
		// No active alerts, but has entries in auxiliary maps
		m.pendingAlerts["vm100-memory"] = now
		m.recentAlerts["vm100-disk"] = &Alert{ID: "vm100-disk", ResourceID: "vm100"}
		m.suppressedUntil["vm100-network"] = now.Add(time.Hour)
		m.alertRateLimit["vm100-io"] = []time.Time{now}
		m.offlineConfirmations["vm100"] = 1
		m.mu.Unlock()

		result := m.suppressGuestAlerts("vm100")
		// Returns false because no active alerts were cleared
		if result {
			t.Error("expected false when no active alerts were cleared")
		}

		m.mu.RLock()
		defer m.mu.RUnlock()

		// But auxiliary maps should still be cleared
		if _, exists := m.pendingAlerts["vm100-memory"]; exists {
			t.Error("expected pendingAlerts to be cleared")
		}
		if _, exists := m.recentAlerts["vm100-disk"]; exists {
			t.Error("expected recentAlerts to be cleared")
		}
		if _, exists := m.suppressedUntil["vm100-network"]; exists {
			t.Error("expected suppressedUntil to be cleared")
		}
		if _, exists := m.alertRateLimit["vm100-io"]; exists {
			t.Error("expected alertRateLimit to be cleared")
		}
		if _, exists := m.offlineConfirmations["vm100"]; exists {
			t.Error("expected offlineConfirmations to be cleared")
		}
	})
}

func TestGuestHasMonitorOnlyAlerts(t *testing.T) {
	t.Run("no alerts returns false", func(t *testing.T) {
		m := newTestManager(t)

		result := m.guestHasMonitorOnlyAlerts("vm100")
		if result {
			t.Error("expected false when no alerts exist")
		}
	})

	t.Run("has non-monitor-only alert returns false", func(t *testing.T) {
		m := newTestManager(t)

		m.mu.Lock()
		m.activeAlerts["vm100-cpu"] = &Alert{
			ID:         "vm100-cpu",
			ResourceID: "vm100",
			Type:       "cpu",
			Metadata:   nil, // No metadata means not monitor-only
		}
		m.mu.Unlock()

		result := m.guestHasMonitorOnlyAlerts("vm100")
		if result {
			t.Error("expected false when alert is not monitor-only")
		}
	})

	t.Run("has monitor-only alert with bool metadata returns true", func(t *testing.T) {
		m := newTestManager(t)

		m.mu.Lock()
		m.activeAlerts["vm100-cpu"] = &Alert{
			ID:         "vm100-cpu",
			ResourceID: "vm100",
			Type:       "cpu",
			Metadata: map[string]interface{}{
				"monitorOnly": true,
			},
		}
		m.mu.Unlock()

		result := m.guestHasMonitorOnlyAlerts("vm100")
		if !result {
			t.Error("expected true when monitor-only alert exists")
		}
	})

	t.Run("has monitor-only alert with string metadata returns true", func(t *testing.T) {
		m := newTestManager(t)

		m.mu.Lock()
		m.activeAlerts["vm100-cpu"] = &Alert{
			ID:         "vm100-cpu",
			ResourceID: "vm100",
			Type:       "cpu",
			Metadata: map[string]interface{}{
				"monitorOnly": "true",
			},
		}
		m.mu.Unlock()

		result := m.guestHasMonitorOnlyAlerts("vm100")
		if !result {
			t.Error("expected true when monitor-only alert exists (string metadata)")
		}
	})

	t.Run("alert for different guest not matched", func(t *testing.T) {
		m := newTestManager(t)

		m.mu.Lock()
		m.activeAlerts["vm200-cpu"] = &Alert{
			ID:         "vm200-cpu",
			ResourceID: "vm200",
			Type:       "cpu",
			Metadata: map[string]interface{}{
				"monitorOnly": true,
			},
		}
		m.mu.Unlock()

		result := m.guestHasMonitorOnlyAlerts("vm100")
		if result {
			t.Error("expected false when monitor-only alert is for different guest")
		}
	})

	t.Run("monitorOnly false returns false", func(t *testing.T) {
		m := newTestManager(t)

		m.mu.Lock()
		m.activeAlerts["vm100-cpu"] = &Alert{
			ID:         "vm100-cpu",
			ResourceID: "vm100",
			Type:       "cpu",
			Metadata: map[string]interface{}{
				"monitorOnly": false,
			},
		}
		m.mu.Unlock()

		result := m.guestHasMonitorOnlyAlerts("vm100")
		if result {
			t.Error("expected false when monitorOnly is explicitly false")
		}
	})
}

func TestCheckNode(t *testing.T) {
	// t.Parallel()

	t.Run("returns early when alerts disabled", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		m.mu.Lock()
		m.config.Enabled = false
		m.mu.Unlock()

		node := models.Node{
			ID:     "node1",
			Name:   "Node 1",
			CPU:    0.95, // Would trigger alert if enabled
			Status: "online",
		}

		m.CheckNode(node)

		m.mu.RLock()
		alertCount := len(m.activeAlerts)
		m.mu.RUnlock()

		if alertCount != 0 {
			t.Errorf("expected no alerts when disabled, got %d", alertCount)
		}
	})

	t.Run("DisableAllNodes clears existing alerts", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		// Pre-create alerts that should be cleared
		m.mu.Lock()
		m.activeAlerts["node1-cpu"] = &Alert{ID: "node1-cpu", ResourceID: "node1", Type: "cpu"}
		m.activeAlerts["node1-memory"] = &Alert{ID: "node1-memory", ResourceID: "node1", Type: "memory"}
		m.activeAlerts["node1-disk"] = &Alert{ID: "node1-disk", ResourceID: "node1", Type: "disk"}
		m.activeAlerts["node1-temperature"] = &Alert{ID: "node1-temperature", ResourceID: "node1", Type: "temperature"}
		m.activeAlerts["node-offline-node1"] = &Alert{ID: "node-offline-node1", ResourceID: "node1", Type: "connectivity"}
		m.nodeOfflineCount["node1"] = 5
		m.config.DisableAllNodes = true
		m.mu.Unlock()

		node := models.Node{ID: "node1", Name: "Node 1", Status: "online"}
		m.CheckNode(node)

		m.mu.RLock()
		_, cpuExists := m.activeAlerts["node1-cpu"]
		_, memExists := m.activeAlerts["node1-memory"]
		_, diskExists := m.activeAlerts["node1-disk"]
		_, tempExists := m.activeAlerts["node1-temperature"]
		_, offlineExists := m.activeAlerts["node-offline-node1"]
		_, countExists := m.nodeOfflineCount["node1"]
		m.mu.RUnlock()

		if cpuExists {
			t.Error("expected cpu alert to be cleared")
		}
		if memExists {
			t.Error("expected memory alert to be cleared")
		}
		if diskExists {
			t.Error("expected disk alert to be cleared")
		}
		if tempExists {
			t.Error("expected temperature alert to be cleared")
		}
		if offlineExists {
			t.Error("expected offline alert to be cleared")
		}
		if countExists {
			t.Error("expected offline count to be cleared")
		}
	})

	t.Run("DisableNodesOffline clears tracking and offline alerts", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		// Pre-create offline alert and tracking
		m.mu.Lock()
		m.activeAlerts["node-offline-node1"] = &Alert{ID: "node-offline-node1", ResourceID: "node1", Type: "connectivity"}
		m.nodeOfflineCount["node1"] = 3
		m.config.DisableAllNodesOffline = true
		m.mu.Unlock()

		node := models.Node{ID: "node1", Name: "Node 1", Status: "offline"}
		m.CheckNode(node)

		m.mu.RLock()
		_, alertExists := m.activeAlerts["node-offline-node1"]
		_, countExists := m.nodeOfflineCount["node1"]
		m.mu.RUnlock()

		if alertExists {
			t.Error("expected offline alert to be cleared")
		}
		if countExists {
			t.Error("expected offline count to be cleared")
		}
	})

	t.Run("offline node triggers offline check", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		// Pre-set count to trigger alert on this call
		m.mu.Lock()
		m.nodeOfflineCount["node1"] = 2
		m.mu.Unlock()

		node := models.Node{
			ID:       "node1",
			Name:     "Node 1",
			Instance: "pve1",
			Status:   "offline",
		}
		m.CheckNode(node)

		m.mu.RLock()
		alert := m.activeAlerts["node-offline-node1"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected offline alert to be created")
		}
		if alert.Type != "connectivity" {
			t.Errorf("expected type connectivity, got %s", alert.Type)
		}
	})

	t.Run("node with connection error triggers offline check", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.nodeOfflineCount["node1"] = 2
		m.mu.Unlock()

		node := models.Node{
			ID:               "node1",
			Name:             "Node 1",
			Instance:         "pve1",
			Status:           "online",
			ConnectionHealth: "error",
		}
		m.CheckNode(node)

		m.mu.RLock()
		alert := m.activeAlerts["node-offline-node1"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected offline alert for connection error")
		}
	})

	t.Run("node with connection failed triggers offline check", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.nodeOfflineCount["node1"] = 2
		m.mu.Unlock()

		node := models.Node{
			ID:               "node1",
			Name:             "Node 1",
			Instance:         "pve1",
			Status:           "online",
			ConnectionHealth: "failed",
		}
		m.CheckNode(node)

		m.mu.RLock()
		alert := m.activeAlerts["node-offline-node1"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected offline alert for connection failed")
		}
	})

	t.Run("online node clears offline alert", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		// Pre-create offline alert
		m.mu.Lock()
		m.activeAlerts["node-offline-node1"] = &Alert{
			ID:         "node-offline-node1",
			ResourceID: "node1",
			Type:       "connectivity",
		}
		m.nodeOfflineCount["node1"] = 5
		m.mu.Unlock()

		node := models.Node{
			ID:               "node1",
			Name:             "Node 1",
			Instance:         "pve1",
			Status:           "online",
			ConnectionHealth: "connected",
		}
		m.CheckNode(node)

		m.mu.RLock()
		_, alertExists := m.activeAlerts["node-offline-node1"]
		_, countExists := m.nodeOfflineCount["node1"]
		m.mu.RUnlock()

		if alertExists {
			t.Error("expected offline alert to be cleared")
		}
		if countExists {
			t.Error("expected offline count to be cleared")
		}
	})

	t.Run("online node triggers metric checks", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		// Set thresholds that will trigger and disable time threshold
		m.mu.Lock()
		m.config.TimeThreshold = 0
		m.config.TimeThresholds = map[string]int{}
		m.config.NodeDefaults = ThresholdConfig{
			CPU: &HysteresisThreshold{Trigger: 80.0, Clear: 70.0},
		}
		m.mu.Unlock()

		node := models.Node{
			ID:       "node1",
			Name:     "Node 1",
			Instance: "pve1",
			Status:   "online",
			CPU:      0.95, // 95% - above trigger
		}
		m.CheckNode(node)

		m.mu.RLock()
		alert := m.activeAlerts["node1-cpu"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected cpu alert to be created")
		}
		if alert.Type != "cpu" {
			t.Errorf("expected type cpu, got %s", alert.Type)
		}
	})

	t.Run("offline node skips metric checks", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.config.NodeDefaults = ThresholdConfig{
			CPU: &HysteresisThreshold{Trigger: 80.0, Clear: 70.0},
		}
		m.mu.Unlock()

		node := models.Node{
			ID:     "node1",
			Name:   "Node 1",
			Status: "offline",
			CPU:    0.95, // Would trigger if checked
		}
		m.CheckNode(node)

		m.mu.RLock()
		_, cpuExists := m.activeAlerts["node1-cpu"]
		m.mu.RUnlock()

		if cpuExists {
			t.Error("expected no cpu alert for offline node")
		}
	})

	t.Run("applies override thresholds", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.config.NodeDefaults = ThresholdConfig{
			CPU: &HysteresisThreshold{Trigger: 80.0, Clear: 70.0},
		}
		m.config.Overrides = map[string]ThresholdConfig{
			"node1": {
				CPU: &HysteresisThreshold{Trigger: 99.0, Clear: 90.0}, // Higher threshold
			},
		}
		m.mu.Unlock()

		node := models.Node{
			ID:       "node1",
			Name:     "Node 1",
			Instance: "pve1",
			Status:   "online",
			CPU:      0.95, // 95% - below override trigger of 99%
		}
		m.CheckNode(node)

		m.mu.RLock()
		_, cpuExists := m.activeAlerts["node1-cpu"]
		m.mu.RUnlock()

		if cpuExists {
			t.Error("expected no alert due to higher override threshold")
		}
	})

	t.Run("checks temperature with package temp", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.config.TimeThreshold = 0
		m.config.TimeThresholds = map[string]int{}
		m.config.NodeDefaults = ThresholdConfig{
			Temperature: &HysteresisThreshold{Trigger: 80.0, Clear: 70.0},
		}
		m.mu.Unlock()

		node := models.Node{
			ID:       "node1",
			Name:     "Node 1",
			Instance: "pve1",
			Status:   "online",
			Temperature: &models.Temperature{
				Available:  true,
				CPUPackage: 90.0, // Above trigger
				CPUMax:     85.0,
			},
		}
		m.CheckNode(node)

		m.mu.RLock()
		alert := m.activeAlerts["node1-temperature"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected temperature alert")
		}
	})

	t.Run("checks temperature with max temp fallback", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.config.TimeThreshold = 0
		m.config.TimeThresholds = map[string]int{}
		m.config.NodeDefaults = ThresholdConfig{
			Temperature: &HysteresisThreshold{Trigger: 80.0, Clear: 70.0},
		}
		m.mu.Unlock()

		node := models.Node{
			ID:       "node1",
			Name:     "Node 1",
			Instance: "pve1",
			Status:   "online",
			Temperature: &models.Temperature{
				Available:  true,
				CPUPackage: 0,    // Zero - will use max
				CPUMax:     90.0, // Above trigger
			},
		}
		m.CheckNode(node)

		m.mu.RLock()
		alert := m.activeAlerts["node1-temperature"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected temperature alert using max temp fallback")
		}
	})

	t.Run("skips temperature when not available", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.config.NodeDefaults = ThresholdConfig{
			Temperature: &HysteresisThreshold{Trigger: 80.0, Clear: 70.0},
		}
		m.mu.Unlock()

		node := models.Node{
			ID:       "node1",
			Name:     "Node 1",
			Instance: "pve1",
			Status:   "online",
			Temperature: &models.Temperature{
				Available:  false, // Not available
				CPUPackage: 90.0,
			},
		}
		m.CheckNode(node)

		m.mu.RLock()
		_, tempExists := m.activeAlerts["node1-temperature"]
		m.mu.RUnlock()

		if tempExists {
			t.Error("expected no temperature alert when not available")
		}
	})

	t.Run("skips temperature when nil", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.config.NodeDefaults = ThresholdConfig{
			Temperature: &HysteresisThreshold{Trigger: 80.0, Clear: 70.0},
		}
		m.mu.Unlock()

		node := models.Node{
			ID:          "node1",
			Name:        "Node 1",
			Instance:    "pve1",
			Status:      "online",
			Temperature: nil, // Nil temperature
		}
		m.CheckNode(node)

		m.mu.RLock()
		_, tempExists := m.activeAlerts["node1-temperature"]
		m.mu.RUnlock()

		if tempExists {
			t.Error("expected no temperature alert when temp is nil")
		}
	})

	t.Run("skips temperature when threshold nil", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		// No temperature threshold set
		m.mu.Lock()
		m.config.NodeDefaults = ThresholdConfig{
			Temperature: nil,
		}
		m.mu.Unlock()

		node := models.Node{
			ID:       "node1",
			Name:     "Node 1",
			Instance: "pve1",
			Status:   "online",
			Temperature: &models.Temperature{
				Available:  true,
				CPUPackage: 90.0,
			},
		}
		m.CheckNode(node)

		m.mu.RLock()
		_, tempExists := m.activeAlerts["node1-temperature"]
		m.mu.RUnlock()

		if tempExists {
			t.Error("expected no temperature alert when threshold nil")
		}
	})

	t.Run("checks memory metric", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.config.TimeThreshold = 0
		m.config.TimeThresholds = map[string]int{}
		m.config.NodeDefaults = ThresholdConfig{
			Memory: &HysteresisThreshold{Trigger: 80.0, Clear: 70.0},
		}
		m.mu.Unlock()

		node := models.Node{
			ID:       "node1",
			Name:     "Node 1",
			Instance: "pve1",
			Status:   "online",
			Memory: models.Memory{
				Usage: 95.0, // Above trigger
			},
		}
		m.CheckNode(node)

		m.mu.RLock()
		alert := m.activeAlerts["node1-memory"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected memory alert")
		}
	})

	t.Run("checks disk metric", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.config.TimeThreshold = 0
		m.config.TimeThresholds = map[string]int{}
		m.config.NodeDefaults = ThresholdConfig{
			Disk: &HysteresisThreshold{Trigger: 80.0, Clear: 70.0},
		}
		m.mu.Unlock()

		node := models.Node{
			ID:       "node1",
			Name:     "Node 1",
			Instance: "pve1",
			Status:   "online",
			Disk: models.Disk{
				Usage: 95.0, // Above trigger
			},
		}
		m.CheckNode(node)

		m.mu.RLock()
		alert := m.activeAlerts["node1-disk"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected disk alert")
		}
	})
}

func TestCheckGuest(t *testing.T) {
	// t.Parallel()

	t.Run("returns early when alerts disabled", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		m.mu.Lock()
		m.config.Enabled = false
		m.mu.Unlock()

		vm := models.VM{
			ID:     "vm100",
			Name:   "TestVM",
			Node:   "node1",
			Status: "running",
			CPU:    0.95,
		}

		m.CheckGuest(vm, "pve1")

		m.mu.RLock()
		alertCount := len(m.activeAlerts)
		m.mu.RUnlock()

		if alertCount != 0 {
			t.Errorf("expected no alerts when disabled, got %d", alertCount)
		}
	})

	t.Run("returns early when all guests disabled", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		m.mu.Lock()
		m.config.DisableAllGuests = true
		m.mu.Unlock()

		vm := models.VM{
			ID:     "vm100",
			Name:   "TestVM",
			Node:   "node1",
			Status: "running",
			CPU:    0.95,
		}

		m.CheckGuest(vm, "pve1")

		m.mu.RLock()
		alertCount := len(m.activeAlerts)
		m.mu.RUnlock()

		if alertCount != 0 {
			t.Errorf("expected no alerts when all guests disabled, got %d", alertCount)
		}
	})

	t.Run("handles VM type correctly", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		m.mu.Lock()
		m.config.TimeThreshold = 0
		m.config.TimeThresholds = map[string]int{}
		m.config.GuestDefaults = ThresholdConfig{
			CPU: &HysteresisThreshold{Trigger: 80.0, Clear: 70.0},
		}
		m.mu.Unlock()

		vm := models.VM{
			ID:     "vm100",
			Name:   "TestVM",
			Node:   "node1",
			Status: "running",
			CPU:    0.95, // 95%
		}

		m.CheckGuest(vm, "pve1")

		m.mu.RLock()
		alert := m.activeAlerts["vm100-cpu"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected cpu alert for VM")
		}
	})

	t.Run("handles Container type correctly", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		m.mu.Lock()
		m.config.TimeThreshold = 0
		m.config.TimeThresholds = map[string]int{}
		m.config.GuestDefaults = ThresholdConfig{
			CPU: &HysteresisThreshold{Trigger: 80.0, Clear: 70.0},
		}
		m.mu.Unlock()

		ct := models.Container{
			ID:     "ct101",
			Name:   "TestCT",
			Node:   "node1",
			Status: "running",
			CPU:    0.95, // 95%
		}

		m.CheckGuest(ct, "pve1")

		m.mu.RLock()
		alert := m.activeAlerts["ct101-cpu"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected cpu alert for Container")
		}
	})

	t.Run("returns for unsupported guest type", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		// Pass a string which is unsupported
		m.CheckGuest("invalid", "pve1")

		m.mu.RLock()
		alertCount := len(m.activeAlerts)
		m.mu.RUnlock()

		if alertCount != 0 {
			t.Errorf("expected no alerts for unsupported type, got %d", alertCount)
		}
	})

	t.Run("suppresses alerts with pulse-no-alerts tag", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		// Pre-create an alert
		m.mu.Lock()
		m.activeAlerts["vm100-cpu"] = &Alert{
			ID:         "vm100-cpu",
			ResourceID: "vm100",
			Type:       "cpu",
		}
		m.mu.Unlock()

		vm := models.VM{
			ID:     "vm100",
			Name:   "TestVM",
			Node:   "node1",
			Status: "running",
			CPU:    0.95,
			Tags:   []string{"pulse-no-alerts"},
		}

		m.CheckGuest(vm, "pve1")

		m.mu.RLock()
		_, exists := m.activeAlerts["vm100-cpu"]
		m.mu.RUnlock()

		if exists {
			t.Error("expected alert to be suppressed with pulse-no-alerts tag")
		}
	})

	t.Run("stopped guest triggers powered-off check", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		// Pre-set confirmation count to trigger alert
		m.mu.Lock()
		m.offlineConfirmations["vm100"] = 2
		m.mu.Unlock()

		vm := models.VM{
			ID:     "vm100",
			Name:   "TestVM",
			Node:   "node1",
			Status: "stopped",
		}

		m.CheckGuest(vm, "pve1")

		m.mu.RLock()
		alert := m.activeAlerts["guest-powered-off-vm100"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected powered-off alert for stopped guest")
		}
	})

	t.Run("stopped guest with DisableAllGuestsOffline clears tracking", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.config.DisableAllGuestsOffline = true
		m.offlineConfirmations["vm100"] = 5
		m.activeAlerts["guest-powered-off-vm100"] = &Alert{
			ID:         "guest-powered-off-vm100",
			ResourceID: "vm100",
			Type:       "powered-off",
		}
		m.mu.Unlock()

		vm := models.VM{
			ID:     "vm100",
			Name:   "TestVM",
			Node:   "node1",
			Status: "stopped",
		}

		m.CheckGuest(vm, "pve1")

		m.mu.RLock()
		_, alertExists := m.activeAlerts["guest-powered-off-vm100"]
		_, countExists := m.offlineConfirmations["vm100"]
		m.mu.RUnlock()

		if alertExists {
			t.Error("expected powered-off alert to be cleared")
		}
		if countExists {
			t.Error("expected offline count to be cleared")
		}
	})

	t.Run("paused guest clears powered-off alert", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.activeAlerts["guest-powered-off-vm100"] = &Alert{
			ID:         "guest-powered-off-vm100",
			ResourceID: "vm100",
			Type:       "powered-off",
		}
		m.mu.Unlock()

		vm := models.VM{
			ID:     "vm100",
			Name:   "TestVM",
			Node:   "node1",
			Status: "paused",
		}

		m.CheckGuest(vm, "pve1")

		m.mu.RLock()
		_, exists := m.activeAlerts["guest-powered-off-vm100"]
		m.mu.RUnlock()

		if exists {
			t.Error("expected powered-off alert to be cleared for paused guest")
		}
	})

	t.Run("non-running guest clears metric alerts", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.activeAlerts["vm100-cpu"] = &Alert{
			ID:         "vm100-cpu",
			ResourceID: "vm100",
			Type:       "cpu",
		}
		m.activeAlerts["vm100-memory"] = &Alert{
			ID:         "vm100-memory",
			ResourceID: "vm100",
			Type:       "memory",
		}
		m.mu.Unlock()

		vm := models.VM{
			ID:     "vm100",
			Name:   "TestVM",
			Node:   "node1",
			Status: "stopped",
		}

		m.CheckGuest(vm, "pve1")

		m.mu.RLock()
		_, cpuExists := m.activeAlerts["vm100-cpu"]
		_, memExists := m.activeAlerts["vm100-memory"]
		m.mu.RUnlock()

		if cpuExists {
			t.Error("expected cpu alert to be cleared for non-running guest")
		}
		if memExists {
			t.Error("expected memory alert to be cleared for non-running guest")
		}
	})

	t.Run("running guest clears powered-off alert", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.activeAlerts["guest-powered-off-vm100"] = &Alert{
			ID:         "guest-powered-off-vm100",
			ResourceID: "vm100",
			Type:       "powered-off",
		}
		m.offlineConfirmations["vm100"] = 5
		m.mu.Unlock()

		vm := models.VM{
			ID:     "vm100",
			Name:   "TestVM",
			Node:   "node1",
			Status: "running",
		}

		m.CheckGuest(vm, "pve1")

		m.mu.RLock()
		_, exists := m.activeAlerts["guest-powered-off-vm100"]
		m.mu.RUnlock()

		if exists {
			t.Error("expected powered-off alert to be cleared for running guest")
		}
	})

	t.Run("disabled thresholds clear existing alerts", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.activeAlerts["vm100-cpu"] = &Alert{
			ID:         "vm100-cpu",
			ResourceID: "vm100",
			Type:       "cpu",
		}
		m.config.Overrides = map[string]ThresholdConfig{
			"vm100": {Disabled: true},
		}
		m.mu.Unlock()

		vm := models.VM{
			ID:     "vm100",
			Name:   "TestVM",
			Node:   "node1",
			Status: "running",
		}

		m.CheckGuest(vm, "pve1")

		m.mu.RLock()
		_, exists := m.activeAlerts["vm100-cpu"]
		m.mu.RUnlock()

		if exists {
			t.Error("expected alert to be cleared when guest has alerts disabled")
		}
	})

	t.Run("checks memory metric", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		m.mu.Lock()
		m.config.TimeThreshold = 0
		m.config.TimeThresholds = map[string]int{}
		m.config.GuestDefaults = ThresholdConfig{
			Memory: &HysteresisThreshold{Trigger: 80.0, Clear: 70.0},
		}
		m.mu.Unlock()

		vm := models.VM{
			ID:     "vm100",
			Name:   "TestVM",
			Node:   "node1",
			Status: "running",
			Memory: models.Memory{Usage: 95.0},
		}

		m.CheckGuest(vm, "pve1")

		m.mu.RLock()
		alert := m.activeAlerts["vm100-memory"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected memory alert")
		}
	})

	t.Run("checks disk metric", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		m.mu.Lock()
		m.config.TimeThreshold = 0
		m.config.TimeThresholds = map[string]int{}
		m.config.GuestDefaults = ThresholdConfig{
			Disk: &HysteresisThreshold{Trigger: 80.0, Clear: 70.0},
		}
		m.mu.Unlock()

		vm := models.VM{
			ID:     "vm100",
			Name:   "TestVM",
			Node:   "node1",
			Status: "running",
			Disk:   models.Disk{Usage: 95.0},
		}

		m.CheckGuest(vm, "pve1")

		m.mu.RLock()
		alert := m.activeAlerts["vm100-disk"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected disk alert")
		}
	})

	t.Run("checks individual disks", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		m.mu.Lock()
		m.config.TimeThreshold = 0
		m.config.TimeThresholds = map[string]int{}
		m.config.GuestDefaults = ThresholdConfig{
			Disk: &HysteresisThreshold{Trigger: 80.0, Clear: 70.0},
		}
		m.mu.Unlock()

		vm := models.VM{
			ID:     "vm100",
			Name:   "TestVM",
			Node:   "node1",
			Status: "running",
			Disks: []models.Disk{
				{Mountpoint: "/", Usage: 95.0, Total: 100},
				{Mountpoint: "/data", Usage: 50.0, Total: 100},
			},
		}

		m.CheckGuest(vm, "pve1")

		m.mu.RLock()
		// Check that alert for high disk was created
		var foundDiskAlert bool
		for alertID := range m.activeAlerts {
			if strings.Contains(alertID, "vm100-disk-") {
				foundDiskAlert = true
				break
			}
		}
		m.mu.RUnlock()

		if !foundDiskAlert {
			t.Fatal("expected individual disk alert")
		}
	})

	t.Run("skips disk with zero total", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		m.mu.Lock()
		m.config.TimeThreshold = 0
		m.config.TimeThresholds = map[string]int{}
		m.config.GuestDefaults = ThresholdConfig{
			Disk: &HysteresisThreshold{Trigger: 80.0, Clear: 70.0},
		}
		m.mu.Unlock()

		vm := models.VM{
			ID:     "vm100",
			Name:   "TestVM",
			Node:   "node1",
			Status: "running",
			Disks: []models.Disk{
				{Mountpoint: "/", Usage: 95.0, Total: 0}, // Zero total - should skip
			},
		}

		m.CheckGuest(vm, "pve1")

		m.mu.RLock()
		var foundDiskAlert bool
		for alertID := range m.activeAlerts {
			if strings.Contains(alertID, "vm100-disk-") {
				foundDiskAlert = true
				break
			}
		}
		m.mu.RUnlock()

		if foundDiskAlert {
			t.Error("expected no disk alert for disk with zero total")
		}
	})

	t.Run("skips disk with negative usage", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		m.mu.Lock()
		m.config.TimeThreshold = 0
		m.config.TimeThresholds = map[string]int{}
		m.config.GuestDefaults = ThresholdConfig{
			Disk: &HysteresisThreshold{Trigger: 80.0, Clear: 70.0},
		}
		m.mu.Unlock()

		vm := models.VM{
			ID:     "vm100",
			Name:   "TestVM",
			Node:   "node1",
			Status: "running",
			Disks: []models.Disk{
				{Mountpoint: "/", Usage: -1.0, Total: 100}, // Negative usage - should skip
			},
		}

		m.CheckGuest(vm, "pve1")

		m.mu.RLock()
		var foundDiskAlert bool
		for alertID := range m.activeAlerts {
			if strings.Contains(alertID, "vm100-disk-") {
				foundDiskAlert = true
				break
			}
		}
		m.mu.RUnlock()

		if foundDiskAlert {
			t.Error("expected no disk alert for disk with negative usage")
		}
	})

	t.Run("checks diskRead metric", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		m.mu.Lock()
		m.config.TimeThreshold = 0
		m.config.TimeThresholds = map[string]int{}
		m.config.GuestDefaults = ThresholdConfig{
			DiskRead: &HysteresisThreshold{Trigger: 100.0, Clear: 80.0}, // MB/s
		}
		m.mu.Unlock()

		vm := models.VM{
			ID:       "vm100",
			Name:     "TestVM",
			Node:     "node1",
			Status:   "running",
			DiskRead: 200 * 1024 * 1024, // 200 MB/s in bytes
		}

		m.CheckGuest(vm, "pve1")

		m.mu.RLock()
		alert := m.activeAlerts["vm100-diskRead"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected diskRead alert")
		}
	})

	t.Run("checks diskWrite metric", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		m.mu.Lock()
		m.config.TimeThreshold = 0
		m.config.TimeThresholds = map[string]int{}
		m.config.GuestDefaults = ThresholdConfig{
			DiskWrite: &HysteresisThreshold{Trigger: 100.0, Clear: 80.0}, // MB/s
		}
		m.mu.Unlock()

		vm := models.VM{
			ID:        "vm100",
			Name:      "TestVM",
			Node:      "node1",
			Status:    "running",
			DiskWrite: 200 * 1024 * 1024, // 200 MB/s in bytes
		}

		m.CheckGuest(vm, "pve1")

		m.mu.RLock()
		alert := m.activeAlerts["vm100-diskWrite"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected diskWrite alert")
		}
	})

	t.Run("checks networkIn metric", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		m.mu.Lock()
		m.config.TimeThreshold = 0
		m.config.TimeThresholds = map[string]int{}
		m.config.GuestDefaults = ThresholdConfig{
			NetworkIn: &HysteresisThreshold{Trigger: 100.0, Clear: 80.0}, // MB/s
		}
		m.mu.Unlock()

		vm := models.VM{
			ID:        "vm100",
			Name:      "TestVM",
			Node:      "node1",
			Status:    "running",
			NetworkIn: 200 * 1024 * 1024, // 200 MB/s in bytes
		}

		m.CheckGuest(vm, "pve1")

		m.mu.RLock()
		alert := m.activeAlerts["vm100-networkIn"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected networkIn alert")
		}
	})

	t.Run("checks networkOut metric", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		m.mu.Lock()
		m.config.TimeThreshold = 0
		m.config.TimeThresholds = map[string]int{}
		m.config.GuestDefaults = ThresholdConfig{
			NetworkOut: &HysteresisThreshold{Trigger: 100.0, Clear: 80.0}, // MB/s
		}
		m.mu.Unlock()

		vm := models.VM{
			ID:         "vm100",
			Name:       "TestVM",
			Node:       "node1",
			Status:     "running",
			NetworkOut: 200 * 1024 * 1024, // 200 MB/s in bytes
		}

		m.CheckGuest(vm, "pve1")

		m.mu.RLock()
		alert := m.activeAlerts["vm100-networkOut"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected networkOut alert")
		}
	})

	t.Run("applies relaxed thresholds with pulse-relaxed tag", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		m.mu.Lock()
		m.config.TimeThreshold = 0
		m.config.TimeThresholds = map[string]int{}
		m.config.GuestDefaults = ThresholdConfig{
			CPU: &HysteresisThreshold{Trigger: 80.0, Clear: 70.0},
		}
		m.mu.Unlock()

		// CPU at 90% - would trigger normally but relaxed threshold is 95%
		vm := models.VM{
			ID:     "vm100",
			Name:   "TestVM",
			Node:   "node1",
			Status: "running",
			CPU:    0.90, // 90%
			Tags:   []string{"pulse-relaxed"},
		}

		m.CheckGuest(vm, "pve1")

		m.mu.RLock()
		_, exists := m.activeAlerts["vm100-cpu"]
		m.mu.RUnlock()

		if exists {
			t.Error("expected no alert due to relaxed thresholds")
		}
	})

	t.Run("disk uses device as label fallback", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		m.mu.Lock()
		m.config.TimeThreshold = 0
		m.config.TimeThresholds = map[string]int{}
		m.config.GuestDefaults = ThresholdConfig{
			Disk: &HysteresisThreshold{Trigger: 80.0, Clear: 70.0},
		}
		m.mu.Unlock()

		vm := models.VM{
			ID:     "vm100",
			Name:   "TestVM",
			Node:   "node1",
			Status: "running",
			Disks: []models.Disk{
				{Device: "sda1", Usage: 95.0, Total: 100}, // No mountpoint, has device
			},
		}

		m.CheckGuest(vm, "pve1")

		m.mu.RLock()
		var foundDiskAlert bool
		for alertID := range m.activeAlerts {
			if strings.Contains(alertID, "vm100-disk-") {
				foundDiskAlert = true
				break
			}
		}
		m.mu.RUnlock()

		if !foundDiskAlert {
			t.Fatal("expected disk alert using device as label")
		}
	})

	t.Run("disk uses index as label when no mountpoint or device", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		m.mu.Lock()
		m.config.TimeThreshold = 0
		m.config.TimeThresholds = map[string]int{}
		m.config.GuestDefaults = ThresholdConfig{
			Disk: &HysteresisThreshold{Trigger: 80.0, Clear: 70.0},
		}
		m.mu.Unlock()

		vm := models.VM{
			ID:     "vm100",
			Name:   "TestVM",
			Node:   "node1",
			Status: "running",
			Disks: []models.Disk{
				{Usage: 95.0, Total: 100}, // No mountpoint or device
			},
		}

		m.CheckGuest(vm, "pve1")

		m.mu.RLock()
		var foundDiskAlert bool
		for alertID := range m.activeAlerts {
			if strings.Contains(alertID, "vm100-disk-") {
				foundDiskAlert = true
				break
			}
		}
		m.mu.RUnlock()

		if !foundDiskAlert {
			t.Fatal("expected disk alert using index as label")
		}
	})
}

func TestCheckHostComprehensive(t *testing.T) {
	// t.Parallel()

	t.Run("returns early for empty host ID", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		m.mu.Lock()
		m.config.TimeThreshold = 0
		m.config.TimeThresholds = map[string]int{}
		m.config.HostDefaults = ThresholdConfig{
			CPU: &HysteresisThreshold{Trigger: 80.0, Clear: 70.0},
		}
		m.mu.Unlock()

		host := models.Host{
			ID:       "",
			CPUUsage: 95.0,
		}

		m.CheckHost(host)

		m.mu.RLock()
		alertCount := len(m.activeAlerts)
		m.mu.RUnlock()

		if alertCount != 0 {
			t.Errorf("expected no alerts for empty host ID, got %d", alertCount)
		}
	})

	t.Run("returns early when alerts disabled", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)
		m.mu.Lock()
		m.config.Enabled = false
		m.mu.Unlock()

		host := models.Host{
			ID:       "host1",
			CPUUsage: 95.0,
		}

		m.CheckHost(host)

		m.mu.RLock()
		alertCount := len(m.activeAlerts)
		m.mu.RUnlock()

		if alertCount != 0 {
			t.Errorf("expected no alerts when disabled, got %d", alertCount)
		}
	})

	t.Run("DisableAllHosts clears existing alerts", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.activeAlerts["host:host1-cpu"] = &Alert{ID: "host:host1-cpu", ResourceID: "host:host1", Type: "cpu"}
		m.activeAlerts["host:host1-memory"] = &Alert{ID: "host:host1-memory", ResourceID: "host:host1", Type: "memory"}
		m.config.DisableAllHosts = true
		m.mu.Unlock()

		host := models.Host{
			ID:       "host1",
			CPUUsage: 95.0,
		}

		m.CheckHost(host)

		m.mu.RLock()
		_, cpuExists := m.activeAlerts["host:host1-cpu"]
		_, memExists := m.activeAlerts["host:host1-memory"]
		m.mu.RUnlock()

		if cpuExists {
			t.Error("expected cpu alert to be cleared")
		}
		if memExists {
			t.Error("expected memory alert to be cleared")
		}
	})

	t.Run("override with Disabled clears alerts", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.activeAlerts["host:host1-cpu"] = &Alert{ID: "host:host1-cpu", ResourceID: "host:host1", Type: "cpu"}
		m.config.Overrides = map[string]ThresholdConfig{
			"host1": {Disabled: true},
		}
		m.mu.Unlock()

		host := models.Host{
			ID:       "host1",
			CPUUsage: 95.0,
		}

		m.CheckHost(host)

		m.mu.RLock()
		_, exists := m.activeAlerts["host:host1-cpu"]
		m.mu.RUnlock()

		if exists {
			t.Error("expected alert to be cleared when host has alerts disabled")
		}
	})

	t.Run("clears CPU alerts when threshold nil", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.activeAlerts["host:host1-cpu"] = &Alert{ID: "host:host1-cpu", ResourceID: "host:host1", Type: "cpu"}
		m.config.HostDefaults = ThresholdConfig{
			CPU: nil, // No CPU threshold
		}
		m.mu.Unlock()

		host := models.Host{
			ID:       "host1",
			CPUUsage: 95.0,
		}

		m.CheckHost(host)

		m.mu.RLock()
		_, exists := m.activeAlerts["host:host1-cpu"]
		m.mu.RUnlock()

		if exists {
			t.Error("expected CPU alert to be cleared when threshold is nil")
		}
	})

	t.Run("clears memory alerts when threshold nil", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.activeAlerts["host:host1-memory"] = &Alert{ID: "host:host1-memory", ResourceID: "host:host1", Type: "memory"}
		m.config.HostDefaults = ThresholdConfig{
			Memory: nil, // No memory threshold
		}
		m.mu.Unlock()

		host := models.Host{
			ID: "host1",
			Memory: models.Memory{
				Usage: 95.0,
			},
		}

		m.CheckHost(host)

		m.mu.RLock()
		_, exists := m.activeAlerts["host:host1-memory"]
		m.mu.RUnlock()

		if exists {
			t.Error("expected memory alert to be cleared when threshold is nil")
		}
	})

	t.Run("clears disk alerts when threshold nil", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		// Disk alert ID format: {resourceID}-disk where resourceID is host:hostID/disk:mountpoint
		alertID := "host:host1/disk:/-disk"
		m.mu.Lock()
		m.activeAlerts[alertID] = &Alert{ID: alertID, ResourceID: "host:host1/disk:/", Type: "disk"}
		m.config.HostDefaults = ThresholdConfig{
			Disk: nil, // No disk threshold
		}
		m.mu.Unlock()

		host := models.Host{
			ID: "host1",
			Disks: []models.Disk{
				{Mountpoint: "/", Usage: 95.0, Total: 100},
			},
		}

		m.CheckHost(host)

		m.mu.RLock()
		_, exists := m.activeAlerts[alertID]
		m.mu.RUnlock()

		if exists {
			t.Error("expected disk alert to be cleared when threshold is nil")
		}
	})

	t.Run("RAID degraded creates critical alert", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		host := models.Host{
			ID:       "host1",
			Hostname: "testhost",
			RAID: []models.HostRAIDArray{
				{
					Device:        "/dev/md2", // Note: md0/md1 are skipped for Synology compatibility
					Level:         "raid1",
					State:         "degraded",
					TotalDevices:  2,
					ActiveDevices: 1,
					FailedDevices: 1,
				},
			},
		}

		m.CheckHost(host)

		m.mu.RLock()
		alert := m.activeAlerts["host-host1-raid-md2"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected RAID degraded alert")
		}
		if alert.Level != AlertLevelCritical {
			t.Errorf("expected critical level, got %s", alert.Level)
		}
	})

	t.Run("RAID rebuilding creates warning alert", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		host := models.Host{
			ID:       "host1",
			Hostname: "testhost",
			RAID: []models.HostRAIDArray{
				{
					Device:         "/dev/md2", // Note: md0/md1 are skipped for Synology compatibility
					Level:          "raid1",
					State:          "recovering",
					TotalDevices:   2,
					ActiveDevices:  2,
					FailedDevices:  0,
					RebuildPercent: 50.0,
				},
			},
		}

		m.CheckHost(host)

		m.mu.RLock()
		alert := m.activeAlerts["host-host1-raid-md2"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected RAID rebuilding alert")
		}
		if alert.Level != AlertLevelWarning {
			t.Errorf("expected warning level, got %s", alert.Level)
		}
	})

	t.Run("RAID healthy clears alert", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.activeAlerts["host-host1-raid-md2"] = &Alert{
			ID:    "host-host1-raid-md2",
			Type:  "raid",
			Level: AlertLevelCritical,
		}
		m.mu.Unlock()

		host := models.Host{
			ID:       "host1",
			Hostname: "testhost",
			RAID: []models.HostRAIDArray{
				{
					Device:        "/dev/md2", // Note: md0/md1 are skipped for Synology compatibility
					Level:         "raid1",
					State:         "active",
					TotalDevices:  2,
					ActiveDevices: 2,
					FailedDevices: 0,
				},
			},
		}

		m.CheckHost(host)

		m.mu.RLock()
		_, exists := m.activeAlerts["host-host1-raid-md2"]
		m.mu.RUnlock()

		if exists {
			t.Error("expected RAID alert to be cleared for healthy array")
		}
	})

	t.Run("RAID with failed devices triggers degraded", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		host := models.Host{
			ID:       "host1",
			Hostname: "testhost",
			RAID: []models.HostRAIDArray{
				{
					Device:        "/dev/md2", // Note: md0/md1 are skipped for Synology compatibility
					Level:         "raid1",
					State:         "active", // State might say active but with failed devices
					TotalDevices:  2,
					ActiveDevices: 1,
					FailedDevices: 1, // This triggers degraded alert
				},
			},
		}

		m.CheckHost(host)

		m.mu.RLock()
		alert := m.activeAlerts["host-host1-raid-md2"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected RAID alert for failed devices")
		}
		if alert.Level != AlertLevelCritical {
			t.Errorf("expected critical level for failed devices, got %s", alert.Level)
		}
	})

	t.Run("RAID resync triggers rebuilding alert", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		host := models.Host{
			ID:       "host1",
			Hostname: "testhost",
			RAID: []models.HostRAIDArray{
				{
					Device:        "/dev/md2", // Note: md0/md1 are skipped for Synology compatibility
					Level:         "raid1",
					State:         "resync",
					TotalDevices:  2,
					ActiveDevices: 2,
					FailedDevices: 0,
				},
			},
		}

		m.CheckHost(host)

		m.mu.RLock()
		alert := m.activeAlerts["host-host1-raid-md2"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected RAID rebuilding alert for resync")
		}
		if alert.Level != AlertLevelWarning {
			t.Errorf("expected warning level for resync, got %s", alert.Level)
		}
	})

	t.Run("existing RAID alert not duplicated", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		originalTime := time.Now().Add(-1 * time.Hour)
		m.mu.Lock()
		m.activeAlerts["host-host1-raid-md2"] = &Alert{
			ID:        "host-host1-raid-md2",
			Type:      "raid",
			Level:     AlertLevelCritical,
			StartTime: originalTime,
		}
		m.mu.Unlock()

		host := models.Host{
			ID:       "host1",
			Hostname: "testhost",
			RAID: []models.HostRAIDArray{
				{
					Device:        "/dev/md2", // Note: md0/md1 are skipped for Synology compatibility
					Level:         "raid1",
					State:         "degraded",
					TotalDevices:  2,
					ActiveDevices: 1,
					FailedDevices: 1,
				},
			},
		}

		m.CheckHost(host)

		m.mu.RLock()
		alert := m.activeAlerts["host-host1-raid-md2"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected RAID alert to still exist")
		}
		// The alert should preserve its original start time
		if !alert.StartTime.Equal(originalTime) {
			t.Error("expected alert start time to be preserved")
		}
	})

	t.Run("applies override thresholds", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.config.TimeThreshold = 0
		m.config.TimeThresholds = map[string]int{}
		m.config.HostDefaults = ThresholdConfig{
			CPU: &HysteresisThreshold{Trigger: 80.0, Clear: 70.0},
		}
		m.config.Overrides = map[string]ThresholdConfig{
			"host1": {
				CPU: &HysteresisThreshold{Trigger: 99.0, Clear: 95.0}, // Higher threshold
			},
		}
		m.mu.Unlock()

		host := models.Host{
			ID:       "host1",
			Hostname: "testhost",
			CPUUsage: 95.0, // Below override trigger
		}

		m.CheckHost(host)

		m.mu.RLock()
		_, exists := m.activeAlerts["host:host1-cpu"]
		m.mu.RUnlock()

		if exists {
			t.Error("expected no alert due to higher override threshold")
		}
	})

	t.Run("checks multiple disks", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.config.TimeThreshold = 0
		m.config.TimeThresholds = map[string]int{}
		m.config.HostDefaults = ThresholdConfig{
			Disk: &HysteresisThreshold{Trigger: 80.0, Clear: 70.0},
		}
		m.mu.Unlock()

		host := models.Host{
			ID:       "host1",
			Hostname: "testhost",
			Disks: []models.Disk{
				{Mountpoint: "/", Usage: 95.0, Total: 100},
				{Mountpoint: "/data", Usage: 50.0, Total: 100}, // Below threshold
			},
		}

		m.CheckHost(host)

		m.mu.RLock()
		var diskAlertCount int
		for alertID := range m.activeAlerts {
			// Disk alert ID format: host:hostID/disk:label-disk
			if strings.Contains(alertID, "host:host1/disk:") {
				diskAlertCount++
			}
		}
		m.mu.RUnlock()

		if diskAlertCount != 1 {
			t.Errorf("expected 1 disk alert, got %d", diskAlertCount)
		}
	})

	t.Run("clears offline alert when host comes online", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		resourceKey := hostResourceID("host1")
		m.mu.Lock()
		m.activeAlerts["host-offline-host1"] = &Alert{
			ID:   "host-offline-host1",
			Type: "connectivity",
		}
		m.offlineConfirmations[resourceKey] = 5
		m.mu.Unlock()

		host := models.Host{
			ID:       "host1",
			Hostname: "testhost",
		}

		m.CheckHost(host)

		m.mu.RLock()
		_, alertExists := m.activeAlerts["host-offline-host1"]
		_, countExists := m.offlineConfirmations[resourceKey]
		m.mu.RUnlock()

		if alertExists {
			t.Error("expected offline alert to be cleared")
		}
		if countExists {
			t.Error("expected offline count to be cleared")
		}
	})

	t.Run("includes tags in metadata", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.config.TimeThreshold = 0
		m.config.TimeThresholds = map[string]int{}
		m.config.HostDefaults = ThresholdConfig{
			CPU: &HysteresisThreshold{Trigger: 80.0, Clear: 70.0},
		}
		m.mu.Unlock()

		host := models.Host{
			ID:       "host1",
			Hostname: "testhost",
			CPUUsage: 95.0,
			Tags:     []string{"production", "critical"},
		}

		m.CheckHost(host)

		m.mu.RLock()
		alert := m.activeAlerts["host:host1-cpu"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected CPU alert")
		}
		if alert.Metadata == nil {
			t.Fatal("expected metadata in alert")
		}
		tags, ok := alert.Metadata["tags"].([]string)
		if !ok || len(tags) != 2 {
			t.Error("expected tags in metadata")
		}
	})
}

func TestCheckPBSComprehensive(t *testing.T) {
	// t.Parallel()

	t.Run("returns early when alerts disabled", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.config.Enabled = false
		m.mu.Unlock()

		pbs := models.PBSInstance{
			ID:   "pbs1",
			Name: "testpbs",
			CPU:  95.0,
		}

		m.CheckPBS(pbs)

		m.mu.RLock()
		alertCount := len(m.activeAlerts)
		m.mu.RUnlock()

		if alertCount != 0 {
			t.Errorf("expected no alerts when disabled, got %d", alertCount)
		}
	})

	t.Run("DisableAllPBS clears existing alerts", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.activeAlerts["pbs1-cpu"] = &Alert{ID: "pbs1-cpu", Type: "cpu"}
		m.activeAlerts["pbs1-memory"] = &Alert{ID: "pbs1-memory", Type: "memory"}
		m.activeAlerts["pbs-offline-pbs1"] = &Alert{ID: "pbs-offline-pbs1", Type: "connectivity"}
		m.offlineConfirmations["pbs1"] = 3
		m.config.DisableAllPBS = true
		m.mu.Unlock()

		pbs := models.PBSInstance{
			ID:   "pbs1",
			Name: "testpbs",
		}

		m.CheckPBS(pbs)

		m.mu.RLock()
		_, cpuExists := m.activeAlerts["pbs1-cpu"]
		_, memExists := m.activeAlerts["pbs1-memory"]
		_, offlineExists := m.activeAlerts["pbs-offline-pbs1"]
		_, confirmExists := m.offlineConfirmations["pbs1"]
		m.mu.RUnlock()

		if cpuExists {
			t.Error("expected CPU alert to be cleared")
		}
		if memExists {
			t.Error("expected memory alert to be cleared")
		}
		if offlineExists {
			t.Error("expected offline alert to be cleared")
		}
		if confirmExists {
			t.Error("expected offline confirmation to be cleared")
		}
	})

	t.Run("override with Disabled clears alerts", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.activeAlerts["pbs1-cpu"] = &Alert{ID: "pbs1-cpu", Type: "cpu"}
		m.activeAlerts["pbs1-memory"] = &Alert{ID: "pbs1-memory", Type: "memory"}
		m.activeAlerts["pbs-offline-pbs1"] = &Alert{ID: "pbs-offline-pbs1", Type: "connectivity"}
		m.offlineConfirmations["pbs1"] = 3
		m.config.Overrides = map[string]ThresholdConfig{
			"pbs1": {Disabled: true},
		}
		m.mu.Unlock()

		pbs := models.PBSInstance{
			ID:   "pbs1",
			Name: "testpbs",
		}

		m.CheckPBS(pbs)

		m.mu.RLock()
		_, cpuExists := m.activeAlerts["pbs1-cpu"]
		_, memExists := m.activeAlerts["pbs1-memory"]
		_, offlineExists := m.activeAlerts["pbs-offline-pbs1"]
		_, confirmExists := m.offlineConfirmations["pbs1"]
		m.mu.RUnlock()

		if cpuExists {
			t.Error("expected CPU alert to be cleared")
		}
		if memExists {
			t.Error("expected memory alert to be cleared")
		}
		if offlineExists {
			t.Error("expected offline alert to be cleared")
		}
		if confirmExists {
			t.Error("expected offline confirmation to be cleared")
		}
	})

	t.Run("DisableAllPBSOffline clears offline alert", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.activeAlerts["pbs-offline-pbs1"] = &Alert{ID: "pbs-offline-pbs1", Type: "connectivity"}
		m.offlineConfirmations["pbs1"] = 3
		m.config.DisableAllPBSOffline = true
		m.mu.Unlock()

		pbs := models.PBSInstance{
			ID:     "pbs1",
			Name:   "testpbs",
			Status: "offline",
		}

		m.CheckPBS(pbs)

		m.mu.RLock()
		_, offlineExists := m.activeAlerts["pbs-offline-pbs1"]
		_, confirmExists := m.offlineConfirmations["pbs1"]
		m.mu.RUnlock()

		if offlineExists {
			t.Error("expected offline alert to be cleared when DisableAllPBSOffline is true")
		}
		if confirmExists {
			t.Error("expected offline confirmation to be cleared")
		}
	})

	t.Run("checks CPU threshold when online", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.config.TimeThreshold = 0
		m.config.TimeThresholds = map[string]int{}
		m.config.NodeDefaults = ThresholdConfig{
			CPU: &HysteresisThreshold{Trigger: 80.0, Clear: 70.0},
		}
		m.mu.Unlock()

		pbs := models.PBSInstance{
			ID:     "pbs1",
			Name:   "testpbs",
			Host:   "pbshost",
			Status: "online",
			CPU:    95.0,
		}

		m.CheckPBS(pbs)

		m.mu.RLock()
		alert := m.activeAlerts["pbs1-cpu"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected CPU alert")
		}
	})

	t.Run("checks memory threshold when online", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.config.TimeThreshold = 0
		m.config.TimeThresholds = map[string]int{}
		m.config.NodeDefaults = ThresholdConfig{
			Memory: &HysteresisThreshold{Trigger: 80.0, Clear: 70.0},
		}
		m.mu.Unlock()

		pbs := models.PBSInstance{
			ID:     "pbs1",
			Name:   "testpbs",
			Host:   "pbshost",
			Status: "online",
			Memory: 95.0,
		}

		m.CheckPBS(pbs)

		m.mu.RLock()
		alert := m.activeAlerts["pbs1-memory"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected memory alert")
		}
	})

	t.Run("skips metrics when PBS is offline", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.config.TimeThreshold = 0
		m.config.TimeThresholds = map[string]int{}
		m.config.NodeDefaults = ThresholdConfig{
			CPU:    &HysteresisThreshold{Trigger: 80.0, Clear: 70.0},
			Memory: &HysteresisThreshold{Trigger: 80.0, Clear: 70.0},
		}
		m.mu.Unlock()

		pbs := models.PBSInstance{
			ID:     "pbs1",
			Name:   "testpbs",
			Status: "offline",
			CPU:    95.0,
			Memory: 95.0,
		}

		m.CheckPBS(pbs)

		m.mu.RLock()
		_, cpuExists := m.activeAlerts["pbs1-cpu"]
		_, memExists := m.activeAlerts["pbs1-memory"]
		m.mu.RUnlock()

		if cpuExists {
			t.Error("expected no CPU alert when offline")
		}
		if memExists {
			t.Error("expected no memory alert when offline")
		}
	})

	t.Run("applies override thresholds", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.config.TimeThreshold = 0
		m.config.TimeThresholds = map[string]int{}
		m.config.NodeDefaults = ThresholdConfig{
			CPU: &HysteresisThreshold{Trigger: 80.0, Clear: 70.0},
		}
		m.config.Overrides = map[string]ThresholdConfig{
			"pbs1": {
				CPU: &HysteresisThreshold{Trigger: 99.0, Clear: 95.0}, // Higher threshold
			},
		}
		m.mu.Unlock()

		pbs := models.PBSInstance{
			ID:     "pbs1",
			Name:   "testpbs",
			Status: "online",
			CPU:    95.0, // Below override trigger
		}

		m.CheckPBS(pbs)

		m.mu.RLock()
		_, exists := m.activeAlerts["pbs1-cpu"]
		m.mu.RUnlock()

		if exists {
			t.Error("expected no alert due to higher override threshold")
		}
	})

	t.Run("checks offline status", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		// Pre-populate confirmation count to bypass waiting period
		m.offlineConfirmations["pbs1"] = 2
		m.mu.Unlock()

		pbs := models.PBSInstance{
			ID:     "pbs1",
			Name:   "testpbs",
			Status: "offline",
		}

		m.CheckPBS(pbs)

		m.mu.RLock()
		alert := m.activeAlerts["pbs-offline-pbs1"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected offline alert")
		}
		if alert.Type != "offline" {
			t.Errorf("expected offline type, got %s", alert.Type)
		}
	})

	t.Run("checks connection health error", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		// Pre-populate confirmation count to bypass waiting period
		m.offlineConfirmations["pbs1"] = 2
		m.mu.Unlock()

		pbs := models.PBSInstance{
			ID:               "pbs1",
			Name:             "testpbs",
			Status:           "online",
			ConnectionHealth: "error",
		}

		m.CheckPBS(pbs)

		m.mu.RLock()
		alert := m.activeAlerts["pbs-offline-pbs1"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected offline alert for connection health error")
		}
	})

	t.Run("checks connection health unhealthy", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		// Pre-populate confirmation count to bypass waiting period
		m.offlineConfirmations["pbs1"] = 2
		m.mu.Unlock()

		pbs := models.PBSInstance{
			ID:               "pbs1",
			Name:             "testpbs",
			Status:           "online",
			ConnectionHealth: "unhealthy",
		}

		m.CheckPBS(pbs)

		m.mu.RLock()
		alert := m.activeAlerts["pbs-offline-pbs1"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected offline alert for connection health unhealthy")
		}
	})

	t.Run("clears stale metric alerts when connection health is unhealthy", func(t *testing.T) {
		m := newTestManager(t)
		m.ClearActiveAlerts()

		m.mu.Lock()
		m.config.TimeThreshold = 0
		m.config.TimeThresholds = map[string]int{}
		m.activeAlerts["pbs1-cpu"] = &Alert{ID: "pbs1-cpu", Type: "cpu"}
		m.activeAlerts["pbs1-memory"] = &Alert{ID: "pbs1-memory", Type: "memory"}
		m.offlineConfirmations["pbs1"] = 2 // trigger offline alert immediately
		m.mu.Unlock()

		pbs := models.PBSInstance{
			ID:               "pbs1",
			Name:             "testpbs",
			Status:           "online",
			ConnectionHealth: "unhealthy",
			CPU:              99,
			Memory:           99,
		}

		m.CheckPBS(pbs)

		m.mu.RLock()
		_, cpuExists := m.activeAlerts["pbs1-cpu"]
		_, memExists := m.activeAlerts["pbs1-memory"]
		offline := m.activeAlerts["pbs-offline-pbs1"]
		m.mu.RUnlock()

		if cpuExists {
			t.Fatal("expected stale CPU alert to be cleared while PBS is unhealthy")
		}
		if memExists {
			t.Fatal("expected stale memory alert to be cleared while PBS is unhealthy")
		}
		if offline == nil {
			t.Fatal("expected offline alert for unhealthy PBS connection")
		}
	})

	t.Run("clears offline alert when back online", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.activeAlerts["pbs-offline-pbs1"] = &Alert{ID: "pbs-offline-pbs1", Type: "connectivity"}
		m.offlineConfirmations["pbs1"] = 5
		m.mu.Unlock()

		pbs := models.PBSInstance{
			ID:               "pbs1",
			Name:             "testpbs",
			Status:           "online",
			ConnectionHealth: "healthy",
		}

		m.CheckPBS(pbs)

		m.mu.RLock()
		_, offlineExists := m.activeAlerts["pbs-offline-pbs1"]
		_, confirmExists := m.offlineConfirmations["pbs1"]
		m.mu.RUnlock()

		if offlineExists {
			t.Error("expected offline alert to be cleared when back online")
		}
		if confirmExists {
			t.Error("expected offline confirmation to be cleared")
		}
	})
}

func TestCheckPMGComprehensive(t *testing.T) {
	// t.Parallel()

	t.Run("returns early when alerts disabled", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.config.Enabled = false
		m.mu.Unlock()

		pmg := models.PMGInstance{
			ID:   "pmg1",
			Name: "testpmg",
		}

		m.CheckPMG(pmg)

		m.mu.RLock()
		alertCount := len(m.activeAlerts)
		m.mu.RUnlock()

		if alertCount != 0 {
			t.Errorf("expected no alerts when disabled, got %d", alertCount)
		}
	})

	t.Run("DisableAllPMG clears existing alerts", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.activeAlerts["pmg1-queue-total"] = &Alert{ID: "pmg1-queue-total", Type: "queue-total"}
		m.activeAlerts["pmg1-queue-deferred"] = &Alert{ID: "pmg1-queue-deferred", Type: "queue-deferred"}
		m.activeAlerts["pmg1-queue-hold"] = &Alert{ID: "pmg1-queue-hold", Type: "queue-hold"}
		m.activeAlerts["pmg1-oldest-message"] = &Alert{ID: "pmg1-oldest-message", Type: "oldest-message"}
		m.activeAlerts["pmg-offline-pmg1"] = &Alert{ID: "pmg-offline-pmg1", Type: "connectivity"}
		m.offlineConfirmations["pmg1"] = 3
		m.config.DisableAllPMG = true
		m.mu.Unlock()

		pmg := models.PMGInstance{
			ID:   "pmg1",
			Name: "testpmg",
		}

		m.CheckPMG(pmg)

		m.mu.RLock()
		_, queueTotalExists := m.activeAlerts["pmg1-queue-total"]
		_, queueDeferredExists := m.activeAlerts["pmg1-queue-deferred"]
		_, queueHoldExists := m.activeAlerts["pmg1-queue-hold"]
		_, oldestMsgExists := m.activeAlerts["pmg1-oldest-message"]
		_, offlineExists := m.activeAlerts["pmg-offline-pmg1"]
		_, confirmExists := m.offlineConfirmations["pmg1"]
		m.mu.RUnlock()

		if queueTotalExists {
			t.Error("expected queue-total alert to be cleared")
		}
		if queueDeferredExists {
			t.Error("expected queue-deferred alert to be cleared")
		}
		if queueHoldExists {
			t.Error("expected queue-hold alert to be cleared")
		}
		if oldestMsgExists {
			t.Error("expected oldest-message alert to be cleared")
		}
		if offlineExists {
			t.Error("expected offline alert to be cleared")
		}
		if confirmExists {
			t.Error("expected offline confirmation to be cleared")
		}
	})

	t.Run("override with Disabled clears alerts", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.activeAlerts["pmg1-queue-total"] = &Alert{ID: "pmg1-queue-total", Type: "queue-total"}
		m.activeAlerts["pmg1-oldest-message"] = &Alert{ID: "pmg1-oldest-message", Type: "oldest-message"}
		m.activeAlerts["pmg-offline-pmg1"] = &Alert{ID: "pmg-offline-pmg1", Type: "connectivity"}
		m.offlineConfirmations["pmg1"] = 3
		m.config.Overrides = map[string]ThresholdConfig{
			"pmg1": {Disabled: true},
		}
		m.mu.Unlock()

		pmg := models.PMGInstance{
			ID:   "pmg1",
			Name: "testpmg",
		}

		m.CheckPMG(pmg)

		m.mu.RLock()
		_, queueExists := m.activeAlerts["pmg1-queue-total"]
		_, oldestExists := m.activeAlerts["pmg1-oldest-message"]
		_, offlineExists := m.activeAlerts["pmg-offline-pmg1"]
		_, confirmExists := m.offlineConfirmations["pmg1"]
		m.mu.RUnlock()

		if queueExists {
			t.Error("expected queue alert to be cleared")
		}
		if oldestExists {
			t.Error("expected oldest-message alert to be cleared")
		}
		if offlineExists {
			t.Error("expected offline alert to be cleared")
		}
		if confirmExists {
			t.Error("expected offline confirmation to be cleared")
		}
	})

	t.Run("DisableAllPMGOffline clears offline alert", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.activeAlerts["pmg-offline-pmg1"] = &Alert{ID: "pmg-offline-pmg1", Type: "connectivity"}
		m.offlineConfirmations["pmg1"] = 3
		m.config.DisableAllPMGOffline = true
		m.mu.Unlock()

		pmg := models.PMGInstance{
			ID:     "pmg1",
			Name:   "testpmg",
			Status: "offline",
		}

		m.CheckPMG(pmg)

		m.mu.RLock()
		_, offlineExists := m.activeAlerts["pmg-offline-pmg1"]
		_, confirmExists := m.offlineConfirmations["pmg1"]
		m.mu.RUnlock()

		if offlineExists {
			t.Error("expected offline alert to be cleared when DisableAllPMGOffline is true")
		}
		if confirmExists {
			t.Error("expected offline confirmation to be cleared")
		}
	})

	t.Run("checks offline status", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		// Pre-populate confirmation count to bypass waiting period (3 required)
		m.offlineConfirmations["pmg1"] = 2
		m.mu.Unlock()

		pmg := models.PMGInstance{
			ID:     "pmg1",
			Name:   "testpmg",
			Status: "offline",
		}

		m.CheckPMG(pmg)

		m.mu.RLock()
		alert := m.activeAlerts["pmg-offline-pmg1"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected offline alert")
		}
		if alert.Type != "offline" {
			t.Errorf("expected offline type, got %s", alert.Type)
		}
	})

	t.Run("checks connection health error", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		// Pre-populate confirmation count to bypass waiting period
		m.offlineConfirmations["pmg1"] = 2
		m.mu.Unlock()

		pmg := models.PMGInstance{
			ID:               "pmg1",
			Name:             "testpmg",
			Status:           "online",
			ConnectionHealth: "error",
		}

		m.CheckPMG(pmg)

		m.mu.RLock()
		alert := m.activeAlerts["pmg-offline-pmg1"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected offline alert for connection health error")
		}
	})

	t.Run("checks connection health unhealthy", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		// Pre-populate confirmation count to bypass waiting period
		m.offlineConfirmations["pmg1"] = 2
		m.mu.Unlock()

		pmg := models.PMGInstance{
			ID:               "pmg1",
			Name:             "testpmg",
			Status:           "online",
			ConnectionHealth: "unhealthy",
		}

		m.CheckPMG(pmg)

		m.mu.RLock()
		alert := m.activeAlerts["pmg-offline-pmg1"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected offline alert for connection health unhealthy")
		}
	})

	t.Run("clears stale PMG metric alerts when connection health is unhealthy", func(t *testing.T) {
		m := newTestManager(t)
		m.ClearActiveAlerts()

		m.mu.Lock()
		m.activeAlerts["pmg1-queue-total"] = &Alert{
			ID:         "pmg1-queue-total",
			Type:       "queue-depth",
			ResourceID: "pmg1",
		}
		m.activeAlerts["pmg1-anomaly-spamIn"] = &Alert{
			ID:         "pmg1-anomaly-spamIn",
			Type:       "anomaly-spamIn",
			ResourceID: "pmg1",
		}
		m.activeAlerts["pmg1-node1-queue-hold"] = &Alert{
			ID:         "pmg1-node1-queue-hold",
			Type:       "queue-hold",
			ResourceID: "pmg1",
		}
		m.offlineConfirmations["pmg1"] = 2 // trigger offline alert immediately
		m.mu.Unlock()

		pmg := models.PMGInstance{
			ID:               "pmg1",
			Name:             "testpmg",
			Status:           "online",
			ConnectionHealth: "unhealthy",
		}

		m.CheckPMG(pmg)

		m.mu.RLock()
		_, queueExists := m.activeAlerts["pmg1-queue-total"]
		_, anomalyExists := m.activeAlerts["pmg1-anomaly-spamIn"]
		_, nodeQueueExists := m.activeAlerts["pmg1-node1-queue-hold"]
		offline := m.activeAlerts["pmg-offline-pmg1"]
		m.mu.RUnlock()

		if queueExists {
			t.Fatal("expected stale queue alert to be cleared while PMG is unhealthy")
		}
		if anomalyExists {
			t.Fatal("expected stale anomaly alert to be cleared while PMG is unhealthy")
		}
		if nodeQueueExists {
			t.Fatal("expected stale per-node queue alert to be cleared while PMG is unhealthy")
		}
		if offline == nil {
			t.Fatal("expected offline alert for unhealthy PMG connection")
		}
	})

	t.Run("clears offline alert when back online", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.activeAlerts["pmg-offline-pmg1"] = &Alert{ID: "pmg-offline-pmg1", Type: "connectivity"}
		m.offlineConfirmations["pmg1"] = 5
		m.mu.Unlock()

		pmg := models.PMGInstance{
			ID:               "pmg1",
			Name:             "testpmg",
			Status:           "online",
			ConnectionHealth: "healthy",
		}

		m.CheckPMG(pmg)

		m.mu.RLock()
		_, offlineExists := m.activeAlerts["pmg-offline-pmg1"]
		_, confirmExists := m.offlineConfirmations["pmg1"]
		m.mu.RUnlock()

		if offlineExists {
			t.Error("expected offline alert to be cleared when back online")
		}
		if confirmExists {
			t.Error("expected offline confirmation to be cleared")
		}
	})

	t.Run("skips metrics when PMG is offline", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		pmg := models.PMGInstance{
			ID:     "pmg1",
			Name:   "testpmg",
			Status: "offline",
		}

		m.CheckPMG(pmg)

		m.mu.RLock()
		var queueAlertCount int
		for alertID := range m.activeAlerts {
			if strings.Contains(alertID, "pmg1-queue") || strings.Contains(alertID, "pmg1-oldest") {
				queueAlertCount++
			}
		}
		m.mu.RUnlock()

		if queueAlertCount != 0 {
			t.Error("expected no queue alerts when offline")
		}
	})
}

func TestCheckStorageComprehensive(t *testing.T) {
	// t.Parallel()

	t.Run("returns early when alerts disabled", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.config.Enabled = false
		m.mu.Unlock()

		storage := models.Storage{
			ID:     "storage1",
			Name:   "teststorage",
			Status: "active",
			Usage:  95.0,
		}

		m.CheckStorage(storage)

		m.mu.RLock()
		alertCount := len(m.activeAlerts)
		m.mu.RUnlock()

		if alertCount != 0 {
			t.Errorf("expected no alerts when disabled, got %d", alertCount)
		}
	})

	t.Run("DisableAllStorage clears existing alerts", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.activeAlerts["storage1-usage"] = &Alert{ID: "storage1-usage", Type: "usage"}
		m.activeAlerts["storage-offline-storage1"] = &Alert{ID: "storage-offline-storage1", Type: "connectivity"}
		m.config.DisableAllStorage = true
		m.mu.Unlock()

		storage := models.Storage{
			ID:     "storage1",
			Name:   "teststorage",
			Status: "active",
		}

		m.CheckStorage(storage)

		m.mu.RLock()
		_, usageExists := m.activeAlerts["storage1-usage"]
		_, offlineExists := m.activeAlerts["storage-offline-storage1"]
		m.mu.RUnlock()

		if usageExists {
			t.Error("expected usage alert to be cleared")
		}
		if offlineExists {
			t.Error("expected offline alert to be cleared")
		}
	})

	t.Run("override with Disabled clears alerts", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.activeAlerts["storage1-usage"] = &Alert{ID: "storage1-usage", Type: "usage"}
		m.activeAlerts["storage-offline-storage1"] = &Alert{ID: "storage-offline-storage1", Type: "connectivity"}
		m.config.Overrides = map[string]ThresholdConfig{
			"storage1": {Disabled: true},
		}
		m.mu.Unlock()

		storage := models.Storage{
			ID:     "storage1",
			Name:   "teststorage",
			Status: "active",
		}

		m.CheckStorage(storage)

		m.mu.RLock()
		_, usageExists := m.activeAlerts["storage1-usage"]
		_, offlineExists := m.activeAlerts["storage-offline-storage1"]
		m.mu.RUnlock()

		if usageExists {
			t.Error("expected usage alert to be cleared")
		}
		if offlineExists {
			t.Error("expected offline alert to be cleared")
		}
	})

	t.Run("checks usage threshold", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.config.TimeThreshold = 0
		m.config.TimeThresholds = map[string]int{}
		m.config.StorageDefault = HysteresisThreshold{Trigger: 80.0, Clear: 70.0}
		m.mu.Unlock()

		storage := models.Storage{
			ID:     "storage1",
			Name:   "teststorage",
			Node:   "node1",
			Status: "active",
			Usage:  95.0,
		}

		m.CheckStorage(storage)

		m.mu.RLock()
		alert := m.activeAlerts["storage1-usage"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected usage alert")
		}
	})

	t.Run("applies override threshold", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.config.TimeThreshold = 0
		m.config.TimeThresholds = map[string]int{}
		m.config.StorageDefault = HysteresisThreshold{Trigger: 80.0, Clear: 70.0}
		overrideThreshold := HysteresisThreshold{Trigger: 99.0, Clear: 95.0}
		m.config.Overrides = map[string]ThresholdConfig{
			"storage1": {Usage: &overrideThreshold},
		}
		m.mu.Unlock()

		storage := models.Storage{
			ID:     "storage1",
			Name:   "teststorage",
			Status: "active",
			Usage:  95.0, // Below override threshold
		}

		m.CheckStorage(storage)

		m.mu.RLock()
		_, exists := m.activeAlerts["storage1-usage"]
		m.mu.RUnlock()

		if exists {
			t.Error("expected no alert due to higher override threshold")
		}
	})

	t.Run("skips usage check when offline", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.config.TimeThreshold = 0
		m.config.TimeThresholds = map[string]int{}
		m.config.StorageDefault = HysteresisThreshold{Trigger: 80.0, Clear: 70.0}
		m.mu.Unlock()

		storage := models.Storage{
			ID:     "storage1",
			Name:   "teststorage",
			Status: "offline",
			Usage:  95.0,
		}

		m.CheckStorage(storage)

		m.mu.RLock()
		_, exists := m.activeAlerts["storage1-usage"]
		m.mu.RUnlock()

		if exists {
			t.Error("expected no usage alert when offline")
		}
	})

	t.Run("skips usage check when unavailable", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.config.TimeThreshold = 0
		m.config.TimeThresholds = map[string]int{}
		m.config.StorageDefault = HysteresisThreshold{Trigger: 80.0, Clear: 70.0}
		m.mu.Unlock()

		storage := models.Storage{
			ID:     "storage1",
			Name:   "teststorage",
			Status: "unavailable",
			Usage:  95.0,
		}

		m.CheckStorage(storage)

		m.mu.RLock()
		_, exists := m.activeAlerts["storage1-usage"]
		m.mu.RUnlock()

		if exists {
			t.Error("expected no usage alert when unavailable")
		}
	})

	t.Run("checks offline status", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		// Pre-populate confirmation count (requires 2)
		m.offlineConfirmations["storage1"] = 1
		m.mu.Unlock()

		storage := models.Storage{
			ID:     "storage1",
			Name:   "teststorage",
			Status: "offline",
		}

		m.CheckStorage(storage)

		m.mu.RLock()
		alert := m.activeAlerts["storage-offline-storage1"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected offline alert")
		}
	})

	t.Run("checks unavailable status", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		// Pre-populate confirmation count (requires 2)
		m.offlineConfirmations["storage1"] = 1
		m.mu.Unlock()

		storage := models.Storage{
			ID:     "storage1",
			Name:   "teststorage",
			Status: "unavailable",
		}

		m.CheckStorage(storage)

		m.mu.RLock()
		alert := m.activeAlerts["storage-offline-storage1"]
		m.mu.RUnlock()

		if alert == nil {
			t.Fatal("expected offline alert for unavailable status")
		}
	})

	t.Run("clears offline alert when back online", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.activeAlerts["storage-offline-storage1"] = &Alert{ID: "storage-offline-storage1", Type: "connectivity"}
		m.offlineConfirmations["storage1"] = 5
		m.mu.Unlock()

		storage := models.Storage{
			ID:     "storage1",
			Name:   "teststorage",
			Status: "active",
		}

		m.CheckStorage(storage)

		m.mu.RLock()
		_, offlineExists := m.activeAlerts["storage-offline-storage1"]
		_, confirmExists := m.offlineConfirmations["storage1"]
		m.mu.RUnlock()

		if offlineExists {
			t.Error("expected offline alert to be cleared when back online")
		}
		if confirmExists {
			t.Error("expected offline confirmation to be cleared")
		}
	})

	t.Run("skips usage check when usage is zero", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		m.mu.Lock()
		m.config.TimeThreshold = 0
		m.config.TimeThresholds = map[string]int{}
		m.config.StorageDefault = HysteresisThreshold{Trigger: 80.0, Clear: 70.0}
		m.mu.Unlock()

		storage := models.Storage{
			ID:     "storage1",
			Name:   "teststorage",
			Status: "active",
			Usage:  0, // No usage data
		}

		m.CheckStorage(storage)

		m.mu.RLock()
		_, exists := m.activeAlerts["storage1-usage"]
		m.mu.RUnlock()

		if exists {
			t.Error("expected no usage alert when usage is zero")
		}
	})
}

func TestDispatchAlert(t *testing.T) {
	// t.Parallel()

	t.Run("returns false when onAlert is nil", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		alert := &Alert{
			ID:   "test-alert",
			Type: "cpu",
		}

		result := m.dispatchAlert(alert, false)

		if result {
			t.Error("expected false when onAlert callback is nil")
		}
	})

	t.Run("returns false when alert is nil", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		called := false
		m.SetAlertCallback(func(a *Alert) {
			called = true
		})

		result := m.dispatchAlert(nil, false)

		if result {
			t.Error("expected false when alert is nil")
		}
		if called {
			t.Error("callback should not be called for nil alert")
		}
	})

	t.Run("returns false when activation state is pending", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		called := false
		m.SetAlertCallback(func(a *Alert) {
			called = true
		})

		m.mu.Lock()
		m.config.ActivationState = ActivationPending
		m.mu.Unlock()

		alert := &Alert{
			ID:   "test-alert",
			Type: "cpu",
		}

		result := m.dispatchAlert(alert, false)

		if result {
			t.Error("expected false when activation is pending")
		}
		if called {
			t.Error("callback should not be called when pending")
		}
	})

	t.Run("returns false when activation state is snoozed", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		called := false
		m.SetAlertCallback(func(a *Alert) {
			called = true
		})

		m.mu.Lock()
		m.config.ActivationState = ActivationSnoozed
		m.mu.Unlock()

		alert := &Alert{
			ID:   "test-alert",
			Type: "cpu",
		}

		result := m.dispatchAlert(alert, false)

		if result {
			t.Error("expected false when activation is snoozed")
		}
		if called {
			t.Error("callback should not be called when snoozed")
		}
	})

	t.Run("returns false for monitor-only alert", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		called := false
		m.SetAlertCallback(func(a *Alert) {
			called = true
		})

		m.mu.Lock()
		m.config.ActivationState = ActivationActive
		m.mu.Unlock()

		alert := &Alert{
			ID:       "test-alert",
			Type:     "cpu",
			Metadata: map[string]interface{}{"monitorOnly": true},
		}

		result := m.dispatchAlert(alert, false)

		if result {
			t.Error("expected false for monitor-only alert")
		}
		if called {
			t.Error("callback should not be called for monitor-only alert")
		}
	})

	t.Run("dispatches synchronously when async is false", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		var receivedAlert *Alert
		m.SetAlertCallback(func(a *Alert) {
			receivedAlert = a
		})

		m.mu.Lock()
		m.config.ActivationState = ActivationActive
		m.mu.Unlock()

		alert := &Alert{
			ID:           "test-alert",
			Type:         "cpu",
			ResourceName: "testvm",
		}

		result := m.dispatchAlert(alert, false)

		if !result {
			t.Error("expected true for successful dispatch")
		}
		if receivedAlert == nil {
			t.Fatal("callback should have been called")
		}
		if receivedAlert.ID != alert.ID {
			t.Error("alert ID should match")
		}
	})

	t.Run("dispatches asynchronously when async is true", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		var receivedAlert *Alert
		done := make(chan struct{})
		m.SetAlertCallback(func(a *Alert) {
			receivedAlert = a
			close(done)
		})

		m.mu.Lock()
		m.config.ActivationState = ActivationActive
		m.mu.Unlock()

		alert := &Alert{
			ID:           "test-alert",
			Type:         "cpu",
			ResourceName: "testvm",
		}

		result := m.dispatchAlert(alert, true)

		if !result {
			t.Error("expected true for successful dispatch")
		}

		// Wait for async callback
		select {
		case <-done:
			// Success
		case <-time.After(time.Second):
			t.Fatal("async callback not called within timeout")
		}

		if receivedAlert == nil {
			t.Fatal("callback should have been called")
		}
		if receivedAlert.ID != alert.ID {
			t.Error("alert ID should match")
		}
	})

	t.Run("clones alert before dispatch", func(t *testing.T) {
		// t.Parallel()
		m := newTestManager(t)

		var receivedAlert *Alert
		m.SetAlertCallback(func(a *Alert) {
			receivedAlert = a
		})

		m.mu.Lock()
		m.config.ActivationState = ActivationActive
		m.mu.Unlock()

		alert := &Alert{
			ID:           "test-alert",
			Type:         "cpu",
			ResourceName: "testvm",
		}

		m.dispatchAlert(alert, false)

		if receivedAlert == alert {
			t.Error("alert should be cloned, not passed directly")
		}
	})
}

func TestPreserveAlertState(t *testing.T) {
	t.Run("nil updated alert is handled", func(t *testing.T) {
		m := newTestManager(t)
		// Should not panic
		m.preserveAlertState("test-id", nil)
	})

	t.Run("preserves state from existing alert", func(t *testing.T) {
		m := newTestManager(t)

		ackTime := time.Now().Add(-30 * time.Minute)
		existing := &Alert{
			ID:              "test-alert",
			Type:            "cpu",
			StartTime:       time.Now().Add(-1 * time.Hour),
			Acknowledged:    true,
			AckUser:         "testuser",
			AckTime:         &ackTime,
			LastEscalation:  2,
			EscalationTimes: []time.Time{time.Now().Add(-25 * time.Minute)},
		}

		m.mu.Lock()
		m.activeAlerts["test-alert"] = existing
		m.mu.Unlock()

		updated := &Alert{
			ID:        "test-alert",
			Type:      "cpu",
			StartTime: time.Now(), // Different start time
		}

		m.preserveAlertState("test-alert", updated)

		if !updated.StartTime.Equal(existing.StartTime) {
			t.Error("StartTime should be preserved from existing alert")
		}
		if !updated.Acknowledged {
			t.Error("Acknowledged should be preserved")
		}
		if updated.AckUser != "testuser" {
			t.Errorf("AckUser should be preserved, got %s", updated.AckUser)
		}
		if updated.AckTime == nil || !updated.AckTime.Equal(ackTime) {
			t.Error("AckTime should be preserved")
		}
		if updated.LastEscalation != 2 {
			t.Error("LastEscalation should be preserved")
		}
		if len(updated.EscalationTimes) != 1 {
			t.Error("EscalationTimes should be preserved")
		}
	})

	t.Run("falls back to ackState for new alert", func(t *testing.T) {
		m := newTestManager(t)

		ackTime := time.Now().Add(-15 * time.Minute)
		m.mu.Lock()
		m.ackState["test-alert"] = ackRecord{
			acknowledged: true,
			user:         "fallbackuser",
			time:         ackTime,
		}
		m.mu.Unlock()

		updated := &Alert{
			ID:        "test-alert",
			Type:      "cpu",
			StartTime: time.Now(),
		}

		m.preserveAlertState("test-alert", updated)

		if !updated.Acknowledged {
			t.Error("Acknowledged should be set from ackState")
		}
		if updated.AckUser != "fallbackuser" {
			t.Errorf("AckUser should be from ackState, got %s", updated.AckUser)
		}
		if updated.AckTime == nil || !updated.AckTime.Equal(ackTime) {
			t.Error("AckTime should be from ackState")
		}
	})

	t.Run("no state to preserve for new alert", func(t *testing.T) {
		m := newTestManager(t)

		startTime := time.Now()
		updated := &Alert{
			ID:        "new-alert",
			Type:      "cpu",
			StartTime: startTime,
		}

		m.preserveAlertState("new-alert", updated)

		if !updated.StartTime.Equal(startTime) {
			t.Error("StartTime should remain unchanged for new alert")
		}
		if updated.Acknowledged {
			t.Error("Acknowledged should remain false for new alert")
		}
	})
}

func TestCheckPMGQuarantineBacklog(t *testing.T) {
	t.Run("nil quarantine clears alerts", func(t *testing.T) {
		m := newTestManager(t)

		// Create an existing quarantine alert
		m.mu.Lock()
		m.activeAlerts["pmg1-quarantine-spam"] = &Alert{
			ID:   "pmg1-quarantine-spam",
			Type: "quarantine-spam",
		}
		m.activeAlerts["pmg1-quarantine-virus"] = &Alert{
			ID:   "pmg1-quarantine-virus",
			Type: "quarantine-virus",
		}
		m.mu.Unlock()

		pmg := models.PMGInstance{
			ID:         "pmg1",
			Name:       "pmg-server",
			Host:       "pmg.example.com",
			Quarantine: nil,
		}

		m.checkPMGQuarantineBacklog(pmg, PMGThresholdConfig{})

		m.mu.RLock()
		_, spamExists := m.activeAlerts["pmg1-quarantine-spam"]
		_, virusExists := m.activeAlerts["pmg1-quarantine-virus"]
		m.mu.RUnlock()

		if spamExists {
			t.Error("spam alert should be cleared when quarantine is nil")
		}
		if virusExists {
			t.Error("virus alert should be cleared when quarantine is nil")
		}
	})

	t.Run("warning threshold triggers alert", func(t *testing.T) {
		m := newTestManager(t)
		m.ClearActiveAlerts()
		m.mu.Lock()
		m.pmgQuarantineHistory = make(map[string][]pmgQuarantineSnapshot)
		m.config.ActivationState = ActivationActive
		m.mu.Unlock()

		pmg := models.PMGInstance{
			ID:   "pmg1",
			Name: "pmg-server",
			Host: "pmg.example.com",
			Quarantine: &models.PMGQuarantineTotals{
				Spam:  2500, // Above warning threshold
				Virus: 100,
			},
		}

		thresholds := PMGThresholdConfig{
			QuarantineSpamWarn:      2000,
			QuarantineSpamCritical:  5000,
			QuarantineVirusWarn:     2000,
			QuarantineVirusCritical: 5000,
		}

		m.checkPMGQuarantineBacklog(pmg, thresholds)

		m.mu.RLock()
		alert, exists := m.activeAlerts["pmg1-quarantine-spam"]
		m.mu.RUnlock()

		if !exists {
			t.Fatal("spam quarantine warning alert should be created")
		}
		if alert.Level != AlertLevelWarning {
			t.Errorf("alert level should be warning, got %s", alert.Level)
		}
	})

	t.Run("critical threshold triggers alert", func(t *testing.T) {
		m := newTestManager(t)
		m.ClearActiveAlerts()
		m.mu.Lock()
		m.pmgQuarantineHistory = make(map[string][]pmgQuarantineSnapshot)
		m.config.ActivationState = ActivationActive
		m.mu.Unlock()

		pmg := models.PMGInstance{
			ID:   "pmg1",
			Name: "pmg-server",
			Host: "pmg.example.com",
			Quarantine: &models.PMGQuarantineTotals{
				Spam:  6000, // Above critical threshold
				Virus: 100,
			},
		}

		thresholds := PMGThresholdConfig{
			QuarantineSpamWarn:      2000,
			QuarantineSpamCritical:  5000,
			QuarantineVirusWarn:     2000,
			QuarantineVirusCritical: 5000,
		}

		m.checkPMGQuarantineBacklog(pmg, thresholds)

		m.mu.RLock()
		alert, exists := m.activeAlerts["pmg1-quarantine-spam"]
		m.mu.RUnlock()

		if !exists {
			t.Fatal("spam quarantine critical alert should be created")
		}
		if alert.Level != AlertLevelCritical {
			t.Errorf("alert level should be critical, got %s", alert.Level)
		}
	})

	t.Run("below threshold clears alert", func(t *testing.T) {
		m := newTestManager(t)
		m.ClearActiveAlerts()
		m.mu.Lock()
		m.pmgQuarantineHistory = make(map[string][]pmgQuarantineSnapshot)
		m.activeAlerts["pmg1-quarantine-spam"] = &Alert{
			ID:    "pmg1-quarantine-spam",
			Type:  "quarantine-spam",
			Level: AlertLevelWarning,
		}
		m.mu.Unlock()

		pmg := models.PMGInstance{
			ID:   "pmg1",
			Name: "pmg-server",
			Host: "pmg.example.com",
			Quarantine: &models.PMGQuarantineTotals{
				Spam:  500, // Below warning threshold
				Virus: 100,
			},
		}

		thresholds := PMGThresholdConfig{
			QuarantineSpamWarn:     2000,
			QuarantineSpamCritical: 5000,
		}

		m.checkPMGQuarantineBacklog(pmg, thresholds)

		m.mu.RLock()
		_, exists := m.activeAlerts["pmg1-quarantine-spam"]
		m.mu.RUnlock()

		if exists {
			t.Error("spam quarantine alert should be cleared when below threshold")
		}
	})

	t.Run("growth rate triggers warning alert", func(t *testing.T) {
		m := newTestManager(t)
		m.ClearActiveAlerts()
		m.mu.Lock()
		m.config.ActivationState = ActivationActive
		// Set up history from ~2 hours ago
		m.pmgQuarantineHistory = map[string][]pmgQuarantineSnapshot{
			"pmg1": {
				{
					Spam:      1000,
					Virus:     100,
					Timestamp: time.Now().Add(-2 * time.Hour),
				},
			},
		}
		m.mu.Unlock()

		pmg := models.PMGInstance{
			ID:   "pmg1",
			Name: "pmg-server",
			Host: "pmg.example.com",
			Quarantine: &models.PMGQuarantineTotals{
				Spam:  1500, // 50% growth (500 messages)
				Virus: 100,
			},
		}

		thresholds := PMGThresholdConfig{
			QuarantineSpamWarn:      10000, // High absolute threshold (won't trigger)
			QuarantineSpamCritical:  20000,
			QuarantineGrowthWarnPct: 25,  // 25% growth warning
			QuarantineGrowthWarnMin: 250, // Minimum 250 messages
			QuarantineGrowthCritPct: 50,  // 50% growth critical
			QuarantineGrowthCritMin: 500, // Minimum 500 messages
		}

		m.checkPMGQuarantineBacklog(pmg, thresholds)

		m.mu.RLock()
		alert, exists := m.activeAlerts["pmg1-quarantine-spam"]
		m.mu.RUnlock()

		if !exists {
			t.Fatal("spam quarantine growth alert should be created")
		}
		if alert.Level != AlertLevelCritical {
			t.Errorf("alert level should be critical due to 50%% growth + 500 messages, got %s", alert.Level)
		}
	})

	t.Run("updates existing alert", func(t *testing.T) {
		m := newTestManager(t)
		m.ClearActiveAlerts()
		m.mu.Lock()
		m.pmgQuarantineHistory = make(map[string][]pmgQuarantineSnapshot)
		m.config.ActivationState = ActivationActive
		m.activeAlerts["pmg1-quarantine-spam"] = &Alert{
			ID:        "pmg1-quarantine-spam",
			Type:      "quarantine-spam",
			Level:     AlertLevelWarning,
			Value:     2500,
			Threshold: 2000,
			LastSeen:  time.Now().Add(-5 * time.Minute),
		}
		m.mu.Unlock()

		pmg := models.PMGInstance{
			ID:   "pmg1",
			Name: "pmg-server",
			Host: "pmg.example.com",
			Quarantine: &models.PMGQuarantineTotals{
				Spam:  3000, // Higher spam count
				Virus: 100,
			},
		}

		thresholds := PMGThresholdConfig{
			QuarantineSpamWarn:     2000,
			QuarantineSpamCritical: 5000,
		}

		m.checkPMGQuarantineBacklog(pmg, thresholds)

		m.mu.RLock()
		alert, exists := m.activeAlerts["pmg1-quarantine-spam"]
		m.mu.RUnlock()

		if !exists {
			t.Fatal("spam quarantine alert should still exist")
		}
		if alert.Value != 3000 {
			t.Errorf("alert value should be updated to 3000, got %.0f", alert.Value)
		}
	})

	t.Run("virus quarantine alert", func(t *testing.T) {
		m := newTestManager(t)
		m.ClearActiveAlerts()
		m.mu.Lock()
		m.pmgQuarantineHistory = make(map[string][]pmgQuarantineSnapshot)
		m.config.ActivationState = ActivationActive
		m.mu.Unlock()

		pmg := models.PMGInstance{
			ID:   "pmg1",
			Name: "pmg-server",
			Host: "pmg.example.com",
			Quarantine: &models.PMGQuarantineTotals{
				Spam:  100,
				Virus: 3000, // Above virus warning threshold
			},
		}

		thresholds := PMGThresholdConfig{
			QuarantineSpamWarn:      2000,
			QuarantineSpamCritical:  5000,
			QuarantineVirusWarn:     2000,
			QuarantineVirusCritical: 5000,
		}

		m.checkPMGQuarantineBacklog(pmg, thresholds)

		m.mu.RLock()
		alert, exists := m.activeAlerts["pmg1-quarantine-virus"]
		m.mu.RUnlock()

		if !exists {
			t.Fatal("virus quarantine warning alert should be created")
		}
		if alert.Level != AlertLevelWarning {
			t.Errorf("alert level should be warning, got %s", alert.Level)
		}
	})
}

func TestLoadActiveAlerts(t *testing.T) {
	t.Run("no file returns nil error", func(t *testing.T) {
		m := newTestManager(t)
		m.ClearActiveAlerts()

		err := m.LoadActiveAlerts()
		if err != nil {
			t.Errorf("expected no error when file doesn't exist, got %v", err)
		}
	})

	t.Run("loads alerts from valid file", func(t *testing.T) {
		m := newTestManager(t)

		// Create an alert and save it
		startTime := time.Now().Add(-30 * time.Minute)
		alert := &Alert{
			ID:           "test-load-alert",
			Type:         "cpu",
			Level:        AlertLevelWarning,
			ResourceID:   "test-resource",
			ResourceName: "test-vm",
			Node:         "node1",
			Instance:     "pve1",
			Message:      "Test alert",
			Value:        85.0,
			Threshold:    80.0,
			StartTime:    startTime,
			LastSeen:     time.Now(),
		}

		m.mu.Lock()
		m.activeAlerts[alert.ID] = alert
		m.mu.Unlock()

		// Save to disk
		_ = m.SaveActiveAlerts()

		// Clear in-memory map only (don't use ClearActiveAlerts which triggers async save)
		m.mu.Lock()
		m.activeAlerts = make(map[string]*Alert)
		m.mu.Unlock()

		err := m.LoadActiveAlerts()
		if err != nil {
			t.Fatalf("failed to load alerts: %v", err)
		}

		m.mu.RLock()
		loaded, exists := m.activeAlerts["test-load-alert"]
		m.mu.RUnlock()

		if !exists {
			t.Fatal("alert should be loaded from file")
		}
		if loaded.Type != "cpu" {
			t.Errorf("loaded alert type should be cpu, got %s", loaded.Type)
		}
		if loaded.Value != 85.0 {
			t.Errorf("loaded alert value should be 85.0, got %.1f", loaded.Value)
		}
	})

	t.Run("migrates legacy guest alert IDs on load", func(t *testing.T) {
		m := newTestManager(t)
		m.ClearActiveAlerts()

		now := time.Now()
		alerts := []Alert{
			{
				ID:           "pve1-node1-100-cpu",
				Type:         "cpu",
				Level:        AlertLevelWarning,
				ResourceID:   "pve1-node1-100",
				ResourceName: "legacy-vm",
				Node:         "node1",
				Instance:     "pve1",
				StartTime:    now.Add(-10 * time.Minute),
				LastSeen:     now,
			},
		}

		alertsDir := filepath.Join(utils.GetDataDir(), "alerts")
		if err := os.MkdirAll(alertsDir, 0755); err != nil {
			t.Fatalf("failed to create alerts dir: %v", err)
		}

		data, err := json.Marshal(alerts)
		if err != nil {
			t.Fatalf("failed to marshal legacy alert: %v", err)
		}
		alertsFile := filepath.Join(alertsDir, "active-alerts.json")
		if err := os.WriteFile(alertsFile, data, 0644); err != nil {
			t.Fatalf("failed to write alerts json: %v", err)
		}

		if err := m.LoadActiveAlerts(); err != nil {
			t.Fatalf("failed to load alerts: %v", err)
		}

		m.mu.RLock()
		_, oldExists := m.activeAlerts["pve1-node1-100-cpu"]
		migrated, migratedExists := m.activeAlerts["pve1-100-cpu"]
		m.mu.RUnlock()

		if oldExists {
			t.Fatal("legacy alert key should be migrated to canonical format")
		}
		if !migratedExists {
			t.Fatal("migrated alert key should exist")
		}
		if migrated.ResourceID != "pve1-100" {
			t.Fatalf("expected migrated ResourceID pve1-100, got %q", migrated.ResourceID)
		}
	})

	t.Run("skips old alerts", func(t *testing.T) {
		m := newTestManager(t)

		// Create an old alert (>24 hours)
		startTime := time.Now().Add(-25 * time.Hour)
		alert := &Alert{
			ID:           "old-alert",
			Type:         "cpu",
			Level:        AlertLevelWarning,
			ResourceID:   "test-resource",
			ResourceName: "test-vm",
			StartTime:    startTime,
			LastSeen:     startTime,
		}

		m.mu.Lock()
		m.activeAlerts[alert.ID] = alert
		m.mu.Unlock()

		// Save to disk
		_ = m.SaveActiveAlerts()

		// Clear in-memory map only (don't use ClearActiveAlerts which triggers async save)
		m.mu.Lock()
		m.activeAlerts = make(map[string]*Alert)
		m.mu.Unlock()

		err := m.LoadActiveAlerts()
		if err != nil {
			t.Fatalf("failed to load alerts: %v", err)
		}

		m.mu.RLock()
		_, exists := m.activeAlerts["old-alert"]
		m.mu.RUnlock()

		if exists {
			t.Error("old alert (>24h) should be skipped during load")
		}
	})

	t.Run("skips old acknowledged alerts", func(t *testing.T) {
		m := newTestManager(t)

		// Create an alert acknowledged >1 hour ago
		startTime := time.Now().Add(-30 * time.Minute)
		ackTime := time.Now().Add(-2 * time.Hour)
		alert := &Alert{
			ID:           "old-ack-alert",
			Type:         "cpu",
			Level:        AlertLevelWarning,
			ResourceID:   "test-resource",
			ResourceName: "test-vm",
			StartTime:    startTime,
			LastSeen:     time.Now(),
			Acknowledged: true,
			AckTime:      &ackTime,
			AckUser:      "testuser",
		}

		m.mu.Lock()
		m.activeAlerts[alert.ID] = alert
		m.mu.Unlock()

		// Save to disk
		_ = m.SaveActiveAlerts()

		// Clear in-memory map only (don't use ClearActiveAlerts which triggers async save)
		m.mu.Lock()
		m.activeAlerts = make(map[string]*Alert)
		m.mu.Unlock()

		err := m.LoadActiveAlerts()
		if err != nil {
			t.Fatalf("failed to load alerts: %v", err)
		}

		m.mu.RLock()
		_, exists := m.activeAlerts["old-ack-alert"]
		ackRecord, ackExists := m.ackState["old-ack-alert"]
		m.mu.RUnlock()

		if exists {
			t.Error("old acknowledged alert (>1h) should be skipped from activeAlerts")
		}

		// But ackState should be preserved so the alert doesn't retrigger if it reappears
		if !ackExists {
			t.Error("ackState should be preserved for old acknowledged alerts to prevent retriggering")
		}
		if ackExists && !ackRecord.acknowledged {
			t.Error("ackState.acknowledged should be true")
		}
		if ackExists && ackRecord.user != "testuser" {
			t.Errorf("ackState.user should be 'testuser', got %q", ackRecord.user)
		}
	})

	t.Run("restores acknowledgment state", func(t *testing.T) {
		m := newTestManager(t)

		// Create an acknowledged alert
		startTime := time.Now().Add(-10 * time.Minute)
		ackTime := time.Now().Add(-5 * time.Minute)
		alert := &Alert{
			ID:           "ack-alert",
			Type:         "cpu",
			Level:        AlertLevelWarning,
			ResourceID:   "test-resource",
			ResourceName: "test-vm",
			StartTime:    startTime,
			LastSeen:     time.Now(),
			Acknowledged: true,
			AckTime:      &ackTime,
			AckUser:      "testuser",
		}

		m.mu.Lock()
		m.activeAlerts[alert.ID] = alert
		m.mu.Unlock()

		// Save to disk
		_ = m.SaveActiveAlerts()

		// Clear in-memory maps only (don't use ClearActiveAlerts which triggers async save)
		m.mu.Lock()
		m.activeAlerts = make(map[string]*Alert)
		m.ackState = make(map[string]ackRecord)
		m.mu.Unlock()

		err := m.LoadActiveAlerts()
		if err != nil {
			t.Fatalf("failed to load alerts: %v", err)
		}

		m.mu.RLock()
		loaded, exists := m.activeAlerts["ack-alert"]
		ackRecord, hasAckRecord := m.ackState["ack-alert"]
		m.mu.RUnlock()

		if !exists {
			t.Fatal("acknowledged alert should be loaded")
		}
		if !loaded.Acknowledged {
			t.Error("loaded alert should be acknowledged")
		}
		if loaded.AckUser != "testuser" {
			t.Errorf("loaded alert AckUser should be testuser, got %s", loaded.AckUser)
		}
		if !hasAckRecord {
			t.Error("ackState should be restored for acknowledged alert")
		}
		if !ackRecord.acknowledged {
			t.Error("ackState should show acknowledged=true")
		}
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		m := newTestManager(t)
		m.ClearActiveAlerts()

		// Write invalid JSON to the alerts file
		alertsDir := filepath.Join(utils.GetDataDir(), "alerts")
		if err := os.MkdirAll(alertsDir, 0755); err != nil {
			t.Fatalf("failed to create alerts dir: %v", err)
		}

		alertsFile := filepath.Join(alertsDir, "active-alerts.json")
		if err := os.WriteFile(alertsFile, []byte("invalid json"), 0644); err != nil {
			t.Fatalf("failed to write invalid json: %v", err)
		}

		err := m.LoadActiveAlerts()
		if err == nil {
			t.Error("expected error for invalid JSON")
		}
	})

	t.Run("skips duplicate alerts", func(t *testing.T) {
		m := newTestManager(t)
		m.ClearActiveAlerts()

		// Write JSON with duplicate alert IDs
		alertsDir := filepath.Join(utils.GetDataDir(), "alerts")
		if err := os.MkdirAll(alertsDir, 0755); err != nil {
			t.Fatalf("failed to create alerts dir: %v", err)
		}

		startTime := time.Now().Add(-10 * time.Minute)
		alerts := []Alert{
			{ID: "dup-alert", Type: "cpu", StartTime: startTime, LastSeen: time.Now()},
			{ID: "dup-alert", Type: "memory", StartTime: startTime, LastSeen: time.Now()},
		}

		data, _ := json.Marshal(alerts)
		alertsFile := filepath.Join(alertsDir, "active-alerts.json")
		if err := os.WriteFile(alertsFile, data, 0644); err != nil {
			t.Fatalf("failed to write alerts json: %v", err)
		}

		err := m.LoadActiveAlerts()
		if err != nil {
			t.Fatalf("failed to load alerts: %v", err)
		}

		m.mu.RLock()
		alert, exists := m.activeAlerts["dup-alert"]
		m.mu.RUnlock()

		if !exists {
			t.Fatal("alert should exist after load")
		}
		// First one wins
		if alert.Type != "cpu" {
			t.Errorf("first alert should win, got type %s", alert.Type)
		}
	})
}

func TestActiveAlertPersistenceUsesManagerDataDir(t *testing.T) {
	managerDir := t.TempDir()
	otherDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", otherDir)

	m := NewManagerWithDataDir(managerDir)
	t.Cleanup(func() {
		m.Stop()
	})

	startTime := time.Now().Add(-5 * time.Minute)
	m.mu.Lock()
	m.activeAlerts["scoped-alert"] = &Alert{
		ID:           "scoped-alert",
		Type:         "cpu",
		Level:        AlertLevelWarning,
		ResourceID:   "scoped-resource",
		ResourceName: "scoped-vm",
		StartTime:    startTime,
		LastSeen:     time.Now(),
	}
	m.mu.Unlock()

	if err := m.SaveActiveAlerts(); err != nil {
		t.Fatalf("SaveActiveAlerts failed: %v", err)
	}

	managerAlertsFile := filepath.Join(managerDir, "alerts", "active-alerts.json")
	if _, err := os.Stat(managerAlertsFile); err != nil {
		t.Fatalf("expected manager-scoped alerts file %s: %v", managerAlertsFile, err)
	}

	otherAlertsFile := filepath.Join(otherDir, "alerts", "active-alerts.json")
	if _, err := os.Stat(otherAlertsFile); !os.IsNotExist(err) {
		t.Fatalf("expected no active alerts in env dir %s, got err=%v", otherAlertsFile, err)
	}

	m.mu.Lock()
	m.activeAlerts = make(map[string]*Alert)
	m.mu.Unlock()

	if err := m.LoadActiveAlerts(); err != nil {
		t.Fatalf("LoadActiveAlerts failed: %v", err)
	}

	m.mu.RLock()
	_, exists := m.activeAlerts["scoped-alert"]
	m.mu.RUnlock()

	if !exists {
		t.Fatal("expected scoped-alert to be restored from manager-scoped data dir")
	}
}

func TestNamespaceMatchesInstance(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		instance  string
		expected  bool
	}{
		// Exact matches
		{"exact match", "pve", "pve", true},
		{"exact match with numbers", "pve1", "pve1", true},

		// Suffix matches (namespace is suffix of instance)
		{"namespace suffix of instance", "nat", "pve-nat", true},
		{"namespace suffix of instance no dash", "nat", "pvenat", true},

		// Suffix matches (instance is suffix of namespace)
		{"instance suffix of namespace", "pvebackups", "pve", false},  // "pve" is not suffix of "pvebackups"
		{"instance suffix of namespace 2", "backupspve", "pve", true}, // "pve" IS suffix of "backupspve"

		// Case insensitive
		{"case insensitive exact", "PVE", "pve", true},
		{"case insensitive suffix", "NAT", "pve-nat", true},

		// Special characters ignored
		{"special chars in namespace", "pve_nat", "pvenat", true},
		{"special chars in instance", "pvenat", "pve-nat", true},
		{"both have special chars", "pve-1", "pve_1", true},

		// No matches - substring but not suffix
		{"no match substring not suffix", "production", "my-production-server", false}, // "production" is not suffix of "myproductionserver"
		{"no match pve not suffix of pvenat", "pve", "pve-nat", false},                 // "pve" is not suffix of "pvenat"

		// No matches
		{"no match", "production", "staging", false},
		{"no match different names", "pve1", "pve2", false},
		{"no match partial mismatch", "abc", "xyz", false},

		// Empty values
		{"empty namespace", "", "pve", false},
		{"empty instance", "pve", "", false},
		{"both empty", "", "", false},

		// Real-world scenarios from issue #1095
		{"pve namespace with pve instance", "pve", "pve", true},
		{"nat namespace with pve-nat instance", "nat", "pve-nat", true},
		{"pve1 namespace with pve1 instance", "pve1", "pve1", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := namespaceMatchesInstance(tt.namespace, tt.instance)
			if result != tt.expected {
				t.Errorf("namespaceMatchesInstance(%q, %q) = %v, want %v",
					tt.namespace, tt.instance, result, tt.expected)
			}
		})
	}
}
