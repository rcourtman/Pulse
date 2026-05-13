package alerts

import (
	"testing"

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
