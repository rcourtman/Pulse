package alerts

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func configureOverrideNormalizationTestManager(t *testing.T, m *Manager, cfg AlertConfig) {
	t.Helper()

	m.UpdateConfig(cfg)

	// Force immediate alerting behavior for deterministic threshold tests.
	m.mu.Lock()
	m.config.TimeThreshold = 0
	m.config.TimeThresholds = map[string]int{}
	m.config.MetricTimeThresholds = nil
	m.config.SuppressionWindow = 0
	m.config.MinimumDelta = 0
	m.mu.Unlock()

	m.ClearActiveAlerts()
}

// TestOverrideResolutionByResourceType verifies that per-resource overrides
// resolve correctly for each resource type when using CheckUnifiedResource.
func TestOverrideResolutionByResourceType(t *testing.T) {
	cases := []struct {
		name         string
		resourceID   string
		resourceType string
		override     ThresholdConfig
		inputMetric  string
		trigger      float64
	}{
		{
			name:         "VM override by ID",
			resourceID:   "qemu-100",
			resourceType: "vm",
			override: ThresholdConfig{
				CPU: &HysteresisThreshold{Trigger: 50, Clear: 45},
			},
			inputMetric: "cpu",
			trigger:     50,
		},
		{
			name:         "Node override by ID",
			resourceID:   "node/pve-1",
			resourceType: "node",
			override: ThresholdConfig{
				CPU: &HysteresisThreshold{Trigger: 60, Clear: 55},
			},
			inputMetric: "cpu",
			trigger:     60,
		},
		{
			name:         "Storage override by ID",
			resourceID:   "local-lvm",
			resourceType: "storage",
			override: ThresholdConfig{
				Usage: &HysteresisThreshold{Trigger: 70, Clear: 65},
			},
			inputMetric: "usage",
			trigger:     70,
		},
		{
			name:         "PBS override by ID",
			resourceID:   "pbs-1",
			resourceType: "pbs",
			override: ThresholdConfig{
				CPU: &HysteresisThreshold{Trigger: 55, Clear: 50},
			},
			inputMetric: "cpu",
			trigger:     55,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mgr := newTestManager(t)
			cfg := AlertConfig{
				Enabled:         true,
				ActivationState: ActivationActive,
				GuestDefaults: ThresholdConfig{
					CPU: &HysteresisThreshold{Trigger: 80, Clear: 75},
				},
				NodeDefaults: ThresholdConfig{
					CPU: &HysteresisThreshold{Trigger: 80, Clear: 75},
				},
				PBSDefaults: ThresholdConfig{
					CPU: &HysteresisThreshold{Trigger: 80, Clear: 75},
				},
				StorageDefault: HysteresisThreshold{Trigger: 85, Clear: 80},
				Overrides: map[string]ThresholdConfig{
					tc.resourceID: tc.override,
				},
			}
			configureOverrideNormalizationTestManager(t, mgr, cfg)

			value := tc.trigger + 5 // Between override trigger and default trigger.
			input := &UnifiedResourceInput{
				ID:   tc.resourceID,
				Type: tc.resourceType,
				Name: tc.name,
				Node: "test-node",
			}
			switch tc.inputMetric {
			case "cpu":
				input.CPU = &UnifiedResourceMetric{Percent: value}
			case "usage":
				input.Disk = &UnifiedResourceMetric{Percent: value}
			default:
				t.Fatalf("unsupported test metric %q", tc.inputMetric)
			}

			mgr.CheckUnifiedResource(input)

			alerts := mgr.GetActiveAlerts()
			if len(alerts) == 0 {
				t.Fatalf("expected alert with override trigger %.0f and value %.0f", tc.trigger, value)
			}
		})
	}
}

// TestOverrideDisabledSuppressesAlerts verifies that setting Disabled=true
// in an override prevents alerts for that resource.
func TestOverrideDisabledSuppressesAlerts(t *testing.T) {
	mgr := newTestManager(t)
	cfg := AlertConfig{
		Enabled:         true,
		ActivationState: ActivationActive,
		GuestDefaults: ThresholdConfig{
			CPU: &HysteresisThreshold{Trigger: 80, Clear: 75},
		},
		Overrides: map[string]ThresholdConfig{
			"qemu-200": {Disabled: true},
		},
	}
	configureOverrideNormalizationTestManager(t, mgr, cfg)

	mgr.CheckUnifiedResource(&UnifiedResourceInput{
		ID:   "qemu-200",
		Type: "vm",
		Name: "suppressed-vm",
		Node: "test-node",
		CPU:  &UnifiedResourceMetric{Percent: 95},
	})

	alerts := mgr.GetActiveAlerts()
	if len(alerts) != 0 {
		t.Fatalf("expected no alerts for disabled override, got %d", len(alerts))
	}
}

// TestOverrideKeyStabilityAcrossUnifiedPath verifies that the same override key
// works for both typed Check* calls and CheckUnifiedResource.
func TestOverrideKeyStabilityAcrossUnifiedPath(t *testing.T) {
	typedMgr := NewManager()
	unifiedMgr := NewManager()

	cfg := AlertConfig{
		Enabled:         true,
		ActivationState: ActivationActive,
		NodeDefaults: ThresholdConfig{
			CPU: &HysteresisThreshold{Trigger: 80, Clear: 75},
		},
		Overrides: map[string]ThresholdConfig{
			"node/pve-1": {
				CPU: &HysteresisThreshold{Trigger: 50, Clear: 45},
			},
		},
	}
	configureOverrideNormalizationTestManager(t, typedMgr, cfg)
	configureOverrideNormalizationTestManager(t, unifiedMgr, cfg)

	typedMgr.CheckNode(models.Node{
		ID:       "node/pve-1",
		Name:     "pve-1",
		Instance: "pve-1",
		Status:   "online",
		CPU:      0.60,
	})

	unifiedMgr.CheckUnifiedResource(&UnifiedResourceInput{
		ID:   "node/pve-1",
		Type: "node",
		Name: "pve-1",
		Node: "pve-1",
		CPU:  &UnifiedResourceMetric{Percent: 60},
	})

	typedAlerts := typedMgr.GetActiveAlerts()
	unifiedAlerts := unifiedMgr.GetActiveAlerts()

	if len(typedAlerts) != 1 {
		t.Fatalf("typed path: expected 1 alert, got %d", len(typedAlerts))
	}
	if len(unifiedAlerts) != 1 {
		t.Fatalf("unified path: expected 1 alert, got %d", len(unifiedAlerts))
	}
}

// TestBackupSnapshotOverridesUntouched verifies that backup and snapshot
// override configurations are not affected by override normalization.
func TestBackupSnapshotOverridesUntouched(t *testing.T) {
	mgr := newTestManager(t)
	backupCfg := &BackupAlertConfig{
		Enabled:      true,
		WarningDays:  2,
		CriticalDays: 4,
		FreshHours:   24,
		StaleHours:   72,
	}
	snapshotCfg := &SnapshotAlertConfig{
		Enabled:         true,
		WarningDays:     7,
		CriticalDays:    14,
		WarningSizeGiB:  20,
		CriticalSizeGiB: 40,
	}

	cfg := AlertConfig{
		Enabled:         true,
		ActivationState: ActivationActive,
		GuestDefaults: ThresholdConfig{
			CPU: &HysteresisThreshold{Trigger: 80, Clear: 75},
		},
		Overrides: map[string]ThresholdConfig{
			"qemu-100": {
				CPU:      &HysteresisThreshold{Trigger: 50, Clear: 45},
				Backup:   backupCfg,
				Snapshot: snapshotCfg,
				Disabled: false,
			},
		},
	}
	configureOverrideNormalizationTestManager(t, mgr, cfg)

	mgr.mu.RLock()
	override, exists := mgr.config.Overrides["qemu-100"]
	mgr.mu.RUnlock()
	if !exists {
		t.Fatalf("expected override for qemu-100")
	}
	if override.CPU == nil || override.CPU.Trigger != 50 {
		t.Fatalf("expected CPU trigger 50, got %v", override.CPU)
	}
	if override.Backup == nil || override.Backup.WarningDays != backupCfg.WarningDays || override.Backup.CriticalDays != backupCfg.CriticalDays {
		t.Fatalf("expected backup override to remain intact, got %#v", override.Backup)
	}
	if override.Snapshot == nil || override.Snapshot.WarningDays != snapshotCfg.WarningDays || override.Snapshot.CriticalDays != snapshotCfg.CriticalDays {
		t.Fatalf("expected snapshot override to remain intact, got %#v", override.Snapshot)
	}
}
