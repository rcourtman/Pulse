package alerts

import (
	"fmt"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// configureDiskTypeHostManager builds an alert manager configured with a
// per-type NVMe override (92/87) and a global agent disk default (90/85),
// with immediate alerting (no time threshold, no suppression).
func configureDiskTypeHostManager(t *testing.T) *Manager {
	t.Helper()

	m := newTestManager(t)
	cfg := AlertConfig{
		Enabled:         true,
		ActivationState: ActivationActive,
		AgentDefaults: ThresholdConfig{
			Disk: &HysteresisThreshold{Trigger: 90, Clear: 85},
		},
		DiskFillByType: map[string]HysteresisThreshold{
			"nvme": {Trigger: 92, Clear: 87},
			"sata": {Trigger: 90, Clear: 85},
			"hdd":  {Trigger: 85, Clear: 80},
		},
		Overrides:         map[string]ThresholdConfig{},
		TimeThresholds:    map[string]int{},
		SuppressionWindow: 0,
		MinimumDelta:      0,
	}
	m.UpdateConfig(cfg)

	m.mu.Lock()
	m.config.TimeThresholds = map[string]int{}
	m.config.MetricTimeThresholds = nil
	m.config.SuppressionWindow = 0
	m.config.MinimumDelta = 0
	m.mu.Unlock()

	m.ClearActiveAlerts()
	return m
}

func configureDiskTempTypeHostManager(t *testing.T) *Manager {
	t.Helper()

	m := newTestManager(t)
	cfg := AlertConfig{
		Enabled:         true,
		ActivationState: ActivationActive,
		AgentDefaults: ThresholdConfig{
			DiskTemperature: &HysteresisThreshold{Trigger: 55, Clear: 50},
		},
		DiskTempByType: map[string]HysteresisThreshold{
			"nvme": {Trigger: 70, Clear: 65},
			"sas":  {Trigger: 65, Clear: 60},
			"sata": {Trigger: 55, Clear: 50},
		},
		Overrides:         map[string]ThresholdConfig{},
		TimeThresholds:    map[string]int{},
		SuppressionWindow: 0,
		MinimumDelta:      0,
	}
	m.UpdateConfig(cfg)

	m.mu.Lock()
	m.config.TimeThresholds = map[string]int{}
	m.config.MetricTimeThresholds = nil
	m.config.SuppressionWindow = 0
	m.config.MinimumDelta = 0
	m.mu.Unlock()

	m.ClearActiveAlerts()
	return m
}

func hostWithSMARTDiskTemp(id, diskType string, temperature int) models.Host {
	return models.Host{
		ID:          id,
		DisplayName: id,
		Hostname:    id,
		Status:      "online",
		Sensors: models.HostSensorSummary{
			SMART: []models.HostDiskSMART{
				{
					Device:      "/dev/" + id,
					Model:       "test-disk",
					Type:        diskType,
					Temperature: temperature,
				},
			},
		},
		IntervalSeconds: 30,
		LastSeen:        time.Now(),
	}
}

func hostDiskTempAlertID(host models.Host) string {
	disk := host.Sensors.SMART[0]
	resourceID := fmt.Sprintf("%s/disk_temp:%s", hostResourceID(host.ID), sanitizeHostComponent(disk.Device))
	return canonicalMetricStateID(resourceID, "diskTemperature")
}

func TestHostDiskFillUsesPerTypeThresholdForNVMe(t *testing.T) {
	m := configureDiskTypeHostManager(t)

	hostBase := models.Host{
		ID:       "host-nvme",
		Hostname: "host-nvme",
		Status:   "online",
	}

	// 91% fill on an NVMe device must NOT alert: nvme trigger is 92.
	hostBelow := hostBase
	hostBelow.Disks = []models.Disk{{
		Mountpoint: "/",
		Device:     "/dev/nvme0n1p1",
		Usage:      91.0,
		Total:      1000,
		Used:       910,
		Free:       90,
	}}
	m.CheckHost(hostBelow)

	diskResourceID, _ := hostDiskResourceID(hostBelow, hostBelow.Disks[0])
	trackingKey := canonicalMetricStateID(diskResourceID, "disk")
	if _, exists := testLookupActiveAlert(t, m, trackingKey); exists {
		t.Fatalf("expected no alert for nvme disk at 91%% (nvme trigger 92), got active: %v", alertKeys(m))
	}
	m.mu.RLock()
	if _, pending := m.pendingAlerts[trackingKey]; pending {
		m.mu.RUnlock()
		t.Fatalf("expected no pending alert for nvme disk at 91%%, but pendingAlerts has %q", trackingKey)
	}
	m.mu.RUnlock()

	// 93% fill on the same NVMe device must alert at the per-type threshold.
	hostAbove := hostBase
	hostAbove.Disks = []models.Disk{{
		Mountpoint: "/",
		Device:     "/dev/nvme0n1p1",
		Usage:      93.0,
		Total:      1000,
		Used:       930,
		Free:       70,
	}}
	m.CheckHost(hostAbove)

	if _, exists := testLookupActiveAlert(t, m, trackingKey); !exists {
		t.Fatalf("expected alert for nvme disk at 93%% (nvme trigger 92), active: %v", alertKeys(m))
	}
}

func TestHostDiskFillFallsBackToGlobalForUnknownDevice(t *testing.T) {
	m := configureDiskTypeHostManager(t)

	host := models.Host{
		ID:       "host-sata",
		Hostname: "host-sata",
		Status:   "online",
		Disks: []models.Disk{{
			Mountpoint: "/",
			Device:     "/dev/sda1",
			Usage:      91.0,
			Total:      1000,
			Used:       910,
			Free:       90,
		}},
	}

	m.CheckHost(host)

	diskResourceID, _ := hostDiskResourceID(host, host.Disks[0])
	trackingKey := canonicalMetricStateID(diskResourceID, "disk")
	if _, exists := testLookupActiveAlert(t, m, trackingKey); !exists {
		t.Fatalf("expected alert for /dev/sda1 at 91%% (global trigger 90, sata key dormant), active: %v", alertKeys(m))
	}
}

func TestHostDiskFillPerTypeThresholdDoesNotOverrideDisabledGlobalDefault(t *testing.T) {
	m := configureDiskTypeHostManager(t)

	m.mu.Lock()
	m.config.AgentDefaults.Disk = &HysteresisThreshold{Trigger: 0, Clear: 0}
	m.mu.Unlock()

	host := models.Host{
		ID:       "host-disabled-nvme",
		Hostname: "host-disabled-nvme",
		Status:   "online",
		Disks: []models.Disk{{
			Mountpoint: "/",
			Device:     "/dev/nvme0n1p1",
			Usage:      93.0,
			Total:      1000,
			Used:       930,
			Free:       70,
		}},
	}

	m.CheckHost(host)

	diskResourceID, _ := hostDiskResourceID(host, host.Disks[0])
	trackingKey := canonicalMetricStateID(diskResourceID, "disk")
	if _, exists := testLookupActiveAlert(t, m, trackingKey); exists {
		t.Fatalf("expected no alert when global agent disk threshold is disabled, active: %v", alertKeys(m))
	}
	m.mu.RLock()
	if _, pending := m.pendingAlerts[trackingKey]; pending {
		m.mu.RUnlock()
		t.Fatalf("expected no pending alert when global agent disk threshold is disabled, but pendingAlerts has %q", trackingKey)
	}
	m.mu.RUnlock()
}

func TestStorageTypeBranchNotRegressed(t *testing.T) {
	m := newTestManager(t)
	configureUnifiedEvalManager(t, m, unifiedEvalBaseConfig())

	m.CheckUnifiedResource(&UnifiedResourceInput{
		ID:   "storage-1",
		Type: "storage",
		Name: "storage-1",
		Disk: &UnifiedResourceMetric{Percent: 92},
	})

	assertAlertPresent(t, m, canonicalMetricStateID("storage-1", "usage"))
}

func TestHostDiskTempUsesNVMeThreshold(t *testing.T) {
	m := configureDiskTempTypeHostManager(t)

	hostBelow := hostWithSMARTDiskTemp("host-temp-nvme", "nvme", 62)
	m.CheckHost(hostBelow)

	trackingKey := hostDiskTempAlertID(hostBelow)
	if _, exists := testLookupActiveAlert(t, m, trackingKey); exists {
		t.Fatalf("expected no alert for nvme disk at 62C (nvme trigger 70), got active: %v", alertKeys(m))
	}
	m.mu.RLock()
	if _, pending := m.pendingAlerts[trackingKey]; pending {
		m.mu.RUnlock()
		t.Fatalf("expected no pending alert for nvme disk at 62C, but pendingAlerts has %q", trackingKey)
	}
	m.mu.RUnlock()

	hostAbove := hostWithSMARTDiskTemp("host-temp-nvme", "nvme", 71)
	m.CheckHost(hostAbove)

	if _, exists := testLookupActiveAlert(t, m, trackingKey); !exists {
		t.Fatalf("expected alert for nvme disk at 71C (nvme trigger 70), active: %v", alertKeys(m))
	}
}

func TestHostDiskTempUsesSASThreshold(t *testing.T) {
	m := configureDiskTempTypeHostManager(t)

	hostBelow := hostWithSMARTDiskTemp("host-temp-sas", "sas", 64)
	m.CheckHost(hostBelow)

	trackingKey := hostDiskTempAlertID(hostBelow)
	if _, exists := testLookupActiveAlert(t, m, trackingKey); exists {
		t.Fatalf("expected no alert for sas disk at 64C (sas trigger 65), got active: %v", alertKeys(m))
	}

	hostAbove := hostWithSMARTDiskTemp("host-temp-sas", "sas", 66)
	m.CheckHost(hostAbove)

	if _, exists := testLookupActiveAlert(t, m, trackingKey); !exists {
		t.Fatalf("expected alert for sas disk at 66C (sas trigger 65), active: %v", alertKeys(m))
	}
}

func TestHostDiskTempUsesSATAThreshold(t *testing.T) {
	m := configureDiskTempTypeHostManager(t)

	hostBelow := hostWithSMARTDiskTemp("host-temp-sata", "sata", 54)
	m.CheckHost(hostBelow)

	trackingKey := hostDiskTempAlertID(hostBelow)
	if _, exists := testLookupActiveAlert(t, m, trackingKey); exists {
		t.Fatalf("expected no alert for sata disk at 54C (sata trigger 55), got active: %v", alertKeys(m))
	}

	hostAbove := hostWithSMARTDiskTemp("host-temp-sata", "sata", 56)
	m.CheckHost(hostAbove)

	if _, exists := testLookupActiveAlert(t, m, trackingKey); !exists {
		t.Fatalf("expected alert for sata disk at 56C (sata trigger 55), active: %v", alertKeys(m))
	}
}

func TestHostDiskTempPerTypeThresholdDoesNotOverrideDisabledGlobalDefault(t *testing.T) {
	m := configureDiskTempTypeHostManager(t)

	m.mu.Lock()
	m.config.AgentDefaults.DiskTemperature = &HysteresisThreshold{Trigger: 0, Clear: 0}
	m.mu.Unlock()

	host := hostWithSMARTDiskTemp("host-temp-disabled-nvme", "nvme", 71)
	m.CheckHost(host)

	trackingKey := hostDiskTempAlertID(host)
	if _, exists := testLookupActiveAlert(t, m, trackingKey); exists {
		t.Fatalf("expected no alert when global agent disk temperature threshold is disabled, active: %v", alertKeys(m))
	}
	m.mu.RLock()
	if _, pending := m.pendingAlerts[trackingKey]; pending {
		m.mu.RUnlock()
		t.Fatalf("expected no pending alert when global agent disk temperature threshold is disabled, but pendingAlerts has %q", trackingKey)
	}
	m.mu.RUnlock()
}
