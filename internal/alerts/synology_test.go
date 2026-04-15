package alerts

import (
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestSynologyRAIDSuppression(t *testing.T) {
	m := newTestManager(t)
	m.ClearActiveAlerts()
	m.mu.Lock()
	m.config.TimeThresholds = map[string]int{}
	m.mu.Unlock()

	host := models.Host{
		ID:          "syno-1",
		DisplayName: "Synology NAS",
		Hostname:    "synology",
		Status:      "online",
		LastSeen:    time.Now(),
		RAID: []models.HostRAIDArray{
			{
				Device:        "/dev/md0", // Suppressed
				Level:         "raid1",
				State:         "degraded", // Should NOT alert
				FailedDevices: 1,
			},
			{
				Device:         "/dev/md1", // Suppressed
				Level:          "raid1",
				State:          "resyncing", // Should NOT alert
				RebuildPercent: 50.0,
			},
			{
				Device:        "/dev/md2", // Not suppressed
				Level:         "raid5",
				State:         "degraded", // SHOULD alert
				FailedDevices: 1,
			},
		},
	}

	m.CheckHost(host)

	alerts := m.GetActiveAlerts()
	var md0Found, md1Found, md2Found bool

	for _, a := range alerts {
		if strings.Contains(a.ID, "md0") {
			md0Found = true
		}
		if strings.Contains(a.ID, "md1") {
			md1Found = true
		}
		if strings.Contains(a.ID, "md2") {
			md2Found = true
		}
	}

	if md0Found {
		t.Error("expected md0 alert to be suppressed")
	}
	if md1Found {
		t.Error("expected md1 alert to be suppressed")
	}
	if !md2Found {
		t.Error("expected md2 alert to be created")
	}
}

func TestSynologyRAIDClearing(t *testing.T) {
	m := newTestManager(t)
	m.ClearActiveAlerts()
	m.mu.Lock()
	m.config.TimeThresholds = map[string]int{}
	m.mu.Unlock()

	// Manually inject an alert for md0
	alertID := buildCanonicalStateID("agent:syno-1/raid:md0", "agent:syno-1/raid:md0-health")
	m.mu.Lock()
	m.activeAlerts[alertID] = &Alert{
		ID:           alertID,
		ResourceID:   "agent:syno-1/raid:md0",
		ResourceName: "Synology NAS - /dev/md0 (raid1)",
		Message:      "RAID array degraded",
	}
	m.mu.Unlock()

	host := models.Host{
		ID:          "syno-1",
		DisplayName: "Synology NAS",
		Hostname:    "synology",
		Status:      "online",
		LastSeen:    time.Now(),
		RAID: []models.HostRAIDArray{
			{
				Device:        "/dev/md0", // Suppressed
				Level:         "raid1",
				State:         "degraded", // Should trigger clearing logic
				FailedDevices: 1,
			},
		},
	}

	m.CheckHost(host)

	m.mu.RLock()
	_, exists := testLookupActiveAlert(t, m, alertID)
	m.mu.RUnlock()

	if exists {
		t.Error("expected md0 alert to be filtered and cleared")
	}
}

func TestQNAPRAIDSuppression(t *testing.T) {
	m := newTestManager(t)
	m.ClearActiveAlerts()
	m.mu.Lock()
	m.config.TimeThresholds = map[string]int{}
	m.mu.Unlock()

	host := models.Host{
		ID:          "qnap-1",
		DisplayName: "QNAP NAS",
		Hostname:    "qnap",
		OSName:      "QNAP QTS",
		Status:      "online",
		LastSeen:    time.Now(),
		RAID: []models.HostRAIDArray{
			{
				Device:        "/dev/md9",
				Level:         "raid1",
				State:         "degraded",
				FailedDevices: 1,
			},
			{
				Device:         "/dev/md13",
				Level:          "raid1",
				State:          "resyncing",
				RebuildPercent: 50.0,
			},
			{
				Device:        "/dev/md2",
				Level:         "raid5",
				State:         "degraded",
				FailedDevices: 1,
			},
		},
	}

	m.CheckHost(host)

	alerts := m.GetActiveAlerts()
	var md9Found, md13Found, md2Found bool

	for _, a := range alerts {
		if strings.Contains(a.ID, "md9") {
			md9Found = true
		}
		if strings.Contains(a.ID, "md13") {
			md13Found = true
		}
		if strings.Contains(a.ID, "md2") {
			md2Found = true
		}
	}

	if md9Found {
		t.Error("expected md9 alert to be suppressed")
	}
	if md13Found {
		t.Error("expected md13 alert to be suppressed")
	}
	if !md2Found {
		t.Error("expected md2 alert to be created")
	}
}

func TestVendorManagedFilteredRAIDStateStillClearsAlerts(t *testing.T) {
	testCases := []struct {
		name        string
		host        models.Host
		alertDevice string
	}{
		{
			name: "synology clears stale md0",
			host: models.Host{
				ID:          "syno-filtered",
				DisplayName: "Synology NAS",
				Hostname:    "synology",
				OSName:      "Synology DSM",
				Status:      "online",
				LastSeen:    time.Now(),
				RAID: []models.HostRAIDArray{
					{Device: "/dev/md2", Level: "raid5", State: "clean"},
				},
			},
			alertDevice: "md0",
		},
		{
			name: "qnap clears stale md9",
			host: models.Host{
				ID:          "qnap-filtered",
				DisplayName: "QNAP NAS",
				Hostname:    "qnap",
				OSName:      "QNAP QTS",
				Status:      "online",
				LastSeen:    time.Now(),
				RAID: []models.HostRAIDArray{
					{Device: "/dev/md2", Level: "raid5", State: "clean"},
				},
			},
			alertDevice: "md9",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestManager(t)
			m.ClearActiveAlerts()
			m.mu.Lock()
			m.config.TimeThresholds = map[string]int{}
			alertID := buildCanonicalStateID(
				"agent:"+tc.host.ID+"/raid:"+tc.alertDevice,
				"agent:"+tc.host.ID+"/raid:"+tc.alertDevice+"-health",
			)
			m.activeAlerts[alertID] = &Alert{
				ID:           alertID,
				ResourceID:   "agent:" + tc.host.ID + "/raid:" + tc.alertDevice,
				ResourceName: tc.host.DisplayName + " - /dev/" + tc.alertDevice + " (raid1)",
				Message:      "RAID array degraded",
			}
			m.mu.Unlock()

			m.CheckHost(tc.host)

			m.mu.RLock()
			_, exists := testLookupActiveAlert(t, m, alertID)
			m.mu.RUnlock()
			if exists {
				t.Fatalf("expected stale %s alert to be cleared", tc.alertDevice)
			}
		})
	}
}

func TestHostDisableClearsRAID(t *testing.T) {
	m := newTestManager(t)
	m.ClearActiveAlerts()
	m.mu.Lock()
	m.config.TimeThresholds = map[string]int{}
	m.mu.Unlock()

	host := models.Host{
		ID:          "host-raid",
		DisplayName: "RAID Host",
		Hostname:    "raid-host",
		Status:      "online",
		LastSeen:    time.Now(),
		RAID: []models.HostRAIDArray{
			{
				Device:        "/dev/md2",
				Level:         "raid5",
				State:         "degraded",
				FailedDevices: 1,
			},
		},
	}

	// 1. Initial check - creates alert
	m.CheckHost(host)

	alertID := buildCanonicalStateID("agent:host-raid/raid:md2", "agent:host-raid/raid:md2-health")
	m.mu.RLock()
	_, exists := testLookupActiveAlert(t, m, alertID)
	m.mu.RUnlock()

	if !exists {
		t.Fatal("expected RAID alert to be created")
	}

	// 2. Disable alerts for this host
	cfg := m.GetConfig()
	cfg.Overrides = map[string]ThresholdConfig{
		host.ID: {
			Disabled: true,
		},
	}
	m.UpdateConfig(cfg)
	m.mu.Lock()
	m.config.TimeThresholds = map[string]int{}
	m.mu.Unlock()

	// 3. Re-check - should clear alerts
	m.CheckHost(host)

	m.mu.RLock()
	_, exists = m.activeAlerts[alertID]
	m.mu.RUnlock()

	if exists {
		t.Error("expected RAID alert to be cleared when host alerts are disabled")
	}
}
