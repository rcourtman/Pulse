package alerts

import (
	"sort"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func unifiedEvalBaseConfig() AlertConfig {
	return AlertConfig{
		Enabled:         true,
		ActivationState: ActivationActive,
		GuestDefaults: ThresholdConfig{
			CPU:    &HysteresisThreshold{Trigger: 80, Clear: 75},
			Memory: &HysteresisThreshold{Trigger: 85, Clear: 80},
			Disk:   &HysteresisThreshold{Trigger: 90, Clear: 85},
		},
		NodeDefaults: ThresholdConfig{
			CPU:    &HysteresisThreshold{Trigger: 80, Clear: 75},
			Memory: &HysteresisThreshold{Trigger: 85, Clear: 80},
			Disk:   &HysteresisThreshold{Trigger: 90, Clear: 85},
		},
		AgentDefaults: ThresholdConfig{
			CPU:    &HysteresisThreshold{Trigger: 80, Clear: 75},
			Memory: &HysteresisThreshold{Trigger: 85, Clear: 80},
			Disk:   &HysteresisThreshold{Trigger: 90, Clear: 85},
		},
		PBSDefaults: ThresholdConfig{
			CPU:    &HysteresisThreshold{Trigger: 80, Clear: 75},
			Memory: &HysteresisThreshold{Trigger: 85, Clear: 80},
		},
		StorageDefault: HysteresisThreshold{Trigger: 85, Clear: 80},
		Overrides:      map[string]ThresholdConfig{},

		// Keep these explicit to make test intent obvious; final values are forced in configureUnifiedEvalManager.
		TimeThresholds:    map[string]int{},
		SuppressionWindow: 0,
		MinimumDelta:      0,
	}
}

func configureUnifiedEvalManager(t *testing.T, m *Manager, cfg AlertConfig) {
	t.Helper()

	m.UpdateConfig(cfg)

	// UpdateConfig normalizes zero values back to defaults; force immediate alerting in tests.
	m.mu.Lock()
	m.config.TimeThresholds = map[string]int{}
	m.config.MetricTimeThresholds = nil
	m.config.SuppressionWindow = 0
	m.config.MinimumDelta = 0
	m.mu.Unlock()

	m.ClearActiveAlerts()
}

func alertKeys(m *Manager) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	keys := make([]string, 0, len(m.activeAlerts))
	for id := range m.activeAlerts {
		keys = append(keys, id)
	}
	sort.Strings(keys)
	return keys
}

func assertAlertPresent(t *testing.T, m *Manager, alertID string) {
	t.Helper()

	m.mu.RLock()
	_, exists := m.activeAlerts[alertID]
	m.mu.RUnlock()
	if !exists {
		t.Fatalf("expected alert %q to exist, active alerts: %v", alertID, alertKeys(m))
	}
}

func assertAlertMissing(t *testing.T, m *Manager, alertID string) {
	t.Helper()

	m.mu.RLock()
	_, exists := m.activeAlerts[alertID]
	m.mu.RUnlock()
	if exists {
		t.Fatalf("expected alert %q to be absent, active alerts: %v", alertID, alertKeys(m))
	}
}

func TestCheckUnifiedResourceMajorFamilies(t *testing.T) {
	m := newTestManager(t)
	configureUnifiedEvalManager(t, m, unifiedEvalBaseConfig())

	tests := []struct {
		name    string
		alertID string
		input   *UnifiedResourceInput
	}{
		{
			name:    "VM CPU above threshold creates alert",
			alertID: "vm-101-cpu",
			input: &UnifiedResourceInput{
				ID:   "vm-101",
				Type: "vm",
				Name: "vm-101",
				CPU:  &UnifiedResourceMetric{Percent: 90},
			},
		},
		{
			name:    "System container CPU above threshold creates alert",
			alertID: "lxc-200-cpu",
			input: &UnifiedResourceInput{
				ID:   "lxc-200",
				Type: "system-container",
				Name: "worker-ct",
				CPU:  &UnifiedResourceMetric{Percent: 91},
			},
		},
		{
			name:    "Node memory above threshold creates alert",
			alertID: "node-a-memory",
			input: &UnifiedResourceInput{
				ID:     "node-a",
				Type:   "node",
				Name:   "node-a",
				Memory: &UnifiedResourceMetric{Percent: 90},
			},
		},
		{
			name:    "Agent disk above threshold creates alert",
			alertID: "host-1-disk",
			input: &UnifiedResourceInput{
				ID:   "host-1",
				Type: "agent",
				Name: "host-1",
				Disk: &UnifiedResourceMetric{Percent: 95},
			},
		},
		{
			name:    "Storage usage above threshold creates alert",
			alertID: "storage-1-usage",
			input: &UnifiedResourceInput{
				ID:   "storage-1",
				Type: "storage",
				Name: "storage-1",
				Disk: &UnifiedResourceMetric{Percent: 92},
			},
		},
		{
			name:    "PBS CPU above threshold creates alert",
			alertID: "pbs-1-cpu",
			input: &UnifiedResourceInput{
				ID:   "pbs-1",
				Type: "pbs",
				Name: "pbs-1",
				CPU:  &UnifiedResourceMetric{Percent: 88},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m.ClearActiveAlerts()
			m.CheckUnifiedResource(tt.input)
			assertAlertPresent(t, m, tt.alertID)
		})
	}
}

func TestCheckUnifiedResourceRejectsLegacyGuestTypeAlias(t *testing.T) {
	m := newTestManager(t)
	configureUnifiedEvalManager(t, m, unifiedEvalBaseConfig())

	m.CheckUnifiedResource(&UnifiedResourceInput{
		ID:   "legacy-ct-200",
		Type: "lxc",
		Name: "legacy-ct",
		CPU:  &UnifiedResourceMetric{Percent: 95},
	})

	assertAlertMissing(t, m, "legacy-ct-200-cpu")
}

func TestCheckUnifiedResourceOverrideLowerThresholdCreatesAlert(t *testing.T) {
	m := newTestManager(t)
	cfg := unifiedEvalBaseConfig()
	cfg.Overrides["vm-override"] = ThresholdConfig{
		CPU: &HysteresisThreshold{Trigger: 60, Clear: 55},
	}
	configureUnifiedEvalManager(t, m, cfg)

	m.CheckUnifiedResource(&UnifiedResourceInput{
		ID:   "vm-override",
		Type: "vm",
		Name: "vm-override",
		CPU:  &UnifiedResourceMetric{Percent: 65},
	})

	assertAlertPresent(t, m, "vm-override-cpu")
}

func TestCheckUnifiedResourceNilInputNoPanic(t *testing.T) {
	m := newTestManager(t)
	configureUnifiedEvalManager(t, m, unifiedEvalBaseConfig())

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("CheckUnifiedResource(nil) panicked: %v", r)
		}
	}()

	m.CheckUnifiedResource(nil)
}

func TestCheckUnifiedResourceDisabledThresholdsNoAlert(t *testing.T) {
	m := newTestManager(t)
	cfg := unifiedEvalBaseConfig()
	cfg.GuestDefaults.Disabled = true
	configureUnifiedEvalManager(t, m, cfg)

	m.CheckUnifiedResource(&UnifiedResourceInput{
		ID:   "vm-disabled",
		Type: "vm",
		Name: "vm-disabled",
		CPU:  &UnifiedResourceMetric{Percent: 95},
	})

	assertAlertMissing(t, m, "vm-disabled-cpu")
}

func TestCheckUnifiedResourceAnnotatesMetricAlertsWithCanonicalSpecMetadata(t *testing.T) {
	m := newTestManager(t)
	configureUnifiedEvalManager(t, m, unifiedEvalBaseConfig())

	m.CheckUnifiedResource(&UnifiedResourceInput{
		ID:   "vm-annotated",
		Type: "vm",
		Name: "vm-annotated",
		CPU:  &UnifiedResourceMetric{Percent: 90},
	})

	m.mu.RLock()
	alert := m.activeAlerts["vm-annotated-cpu"]
	m.mu.RUnlock()
	if alert == nil {
		t.Fatal("expected vm-annotated-cpu alert")
	}
	if got := alert.Metadata["canonicalAlertKind"]; got != "metric-threshold" {
		t.Fatalf("canonicalAlertKind = %v, want metric-threshold", got)
	}
	if got := alert.Metadata["canonicalSpecID"]; got != "vm-annotated-cpu" {
		t.Fatalf("canonicalSpecID = %v, want vm-annotated-cpu", got)
	}
}

func TestCheckGuestPerDiskAnnotatesCanonicalSpecMetadata(t *testing.T) {
	m := newTestManager(t)
	configureUnifiedEvalManager(t, m, unifiedEvalBaseConfig())

	guestID := BuildGuestKey("pve1", "node1", 101)
	m.CheckGuest(models.VM{
		ID:       guestID,
		VMID:     101,
		Name:     "app01",
		Node:     "node1",
		Instance: "pve1",
		Status:   "running",
		CPU:      0.20,
		Memory:   models.Memory{Usage: 40},
		Disk:     models.Disk{Usage: 40},
		Disks: []models.Disk{
			{
				Mountpoint: "/",
				Device:     "scsi0",
				Usage:      95,
				Total:      100,
				Used:       95,
				Free:       5,
			},
		},
	}, "pve1")

	alertID := guestID + "-disk-scsi0-disk"
	m.mu.RLock()
	alert := m.activeAlerts[alertID]
	m.mu.RUnlock()
	if alert == nil {
		t.Fatalf("expected guest disk alert %q", alertID)
	}
	if got := alert.Metadata["canonicalAlertKind"]; got != "metric-threshold" {
		t.Fatalf("canonicalAlertKind = %v, want metric-threshold", got)
	}
	if got := alert.Metadata["canonicalSpecID"]; got != alertID {
		t.Fatalf("canonicalSpecID = %v, want %s", got, alertID)
	}
}

func TestCheckNodeTemperatureAnnotatesCanonicalSpecMetadata(t *testing.T) {
	m := newTestManager(t)
	configureUnifiedEvalManager(t, m, unifiedEvalBaseConfig())

	m.CheckNode(models.Node{
		ID:       "node/pve-1",
		Name:     "pve-1",
		Instance: "pve-1",
		Status:   "online",
		CPU:      0.20,
		Memory:   models.Memory{Usage: 40},
		Disk:     models.Disk{Usage: 40},
		Temperature: &models.Temperature{
			Available:  true,
			CPUPackage: 90,
		},
	})

	m.mu.RLock()
	alert := m.activeAlerts["node/pve-1-temperature"]
	m.mu.RUnlock()
	if alert == nil {
		t.Fatal("expected node temperature alert")
	}
	if got := alert.Metadata["canonicalAlertKind"]; got != "metric-threshold" {
		t.Fatalf("canonicalAlertKind = %v, want metric-threshold", got)
	}
	if got := alert.Metadata["canonicalSpecID"]; got != "node/pve-1-temperature" {
		t.Fatalf("canonicalSpecID = %v, want node/pve-1-temperature", got)
	}
}
