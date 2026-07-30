package alerts

import (
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestHostCustomSensorAlertLifecycle(t *testing.T) {
	manager := newTestManager(t)
	manager.ClearActiveAlerts()
	value := 23.0
	eventAt := time.Now().UTC().Add(-2 * time.Hour)
	host := models.Host{
		ID:          "custom-sensor-host",
		Hostname:    "edge-1",
		DisplayName: "Edge 1",
		Sensors: models.HostSensorSummary{
			Custom: []models.HostCustomSensorMetric{{
				ID:           "queue_depth",
				Name:         "Queue depth",
				Group:        "Main server",
				Subgroup:     "Backup",
				Kind:         "timestamp",
				Unit:         "items",
				Value:        &value,
				Status:       "critical",
				ObservedAt:   time.Now().UTC(),
				EventAt:      &eventAt,
				AlertOnError: true,
			}},
		},
	}

	manager.CheckHost(host)
	active := manager.GetActiveAlerts()
	if !hasAlertType(active, "custom-sensor") {
		t.Fatal("expected custom sensor alert")
	}
	for _, alert := range active {
		if alert.Type != "custom-sensor" {
			continue
		}
		if alert.Metadata["customSensorGroup"] != "Main server" ||
			alert.Metadata["customSensorSubgroup"] != "Backup" ||
			alert.Metadata["customSensorKind"] != "timestamp" ||
			alert.Metadata["customSensorEventAt"] != eventAt.Format(time.RFC3339) {
			t.Fatalf("custom sensor alert metadata = %#v", alert.Metadata)
		}
	}

	host.Sensors.Custom[0].Status = "ok"
	manager.CheckHost(host)
	if hasAlertType(manager.GetActiveAlerts(), "custom-sensor") {
		t.Fatal("healthy custom sensor did not resolve its alert")
	}

	host.Sensors.Custom[0].Status = "error"
	host.Sensors.Custom[0].Error = "probe timed out"
	manager.CheckHost(host)
	if !hasAlertType(manager.GetActiveAlerts(), "custom-sensor") {
		t.Fatal("alertOnError custom sensor did not alert")
	}

	host.Sensors.Custom = nil
	manager.CheckHost(host)
	if hasAlertType(manager.GetActiveAlerts(), "custom-sensor") {
		t.Fatal("removed custom sensor did not clear its alert")
	}
}

func TestHostCustomSensorErrorCanBeReportOnly(t *testing.T) {
	manager := newTestManager(t)
	manager.ClearActiveAlerts()
	host := models.Host{
		ID:       "custom-sensor-report-only",
		Hostname: "edge-2",
		Sensors: models.HostSensorSummary{
			Custom: []models.HostCustomSensorMetric{{
				ID:         "optional_probe",
				Name:       "Optional probe",
				Status:     "error",
				ObservedAt: time.Now().UTC(),
				Error:      "not installed",
			}},
		},
	}

	manager.CheckHost(host)
	if hasAlertType(manager.GetActiveAlerts(), "custom-sensor") {
		t.Fatal("alertOnError=false custom sensor unexpectedly alerted")
	}
}

func TestHandleHostOfflineExpiresUnraidOperationAlertBeforeConnectivityConfirmation(t *testing.T) {
	manager := newTestManager(t)
	manager.ClearActiveAlerts()
	host := models.Host{
		ID:          "unraid-host",
		Hostname:    "tower",
		DisplayName: "Tower",
		Platform:    "unraid",
		Unraid: &models.HostUnraidStorage{
			ArrayStarted: true,
			ArrayState:   "STARTED",
			SyncAction:   "check",
			SyncProgress: 40,
			Disks: []models.HostUnraidDisk{
				{Name: "parity", Role: "parity", Status: "online", Device: "/dev/sda"},
				{Name: "disk1", Role: "data", Status: "online", Device: "/dev/sdb"},
			},
		},
	}

	manager.CheckHost(host)
	if !hasAlertType(manager.GetActiveAlerts(), "storage-topology") {
		t.Fatal("expected active Unraid operation alert")
	}

	manager.HandleHostOffline(host)
	alerts := manager.GetActiveAlerts()
	if hasAlertType(alerts, "storage-topology") {
		t.Fatal("expired Unraid operation alert remained active")
	}
	if hasAlertType(alerts, "host-offline") {
		t.Fatal("connectivity alert should still require its confirmation window")
	}
}

func TestHandleHostTelemetryExpiredPreservesStaticStorageRisk(t *testing.T) {
	t.Run("Unraid no-parity risk survives transient check expiry", func(t *testing.T) {
		manager := newTestManager(t)
		manager.ClearActiveAlerts()
		host := models.Host{
			ID:          "unraid-no-parity",
			Hostname:    "tower",
			DisplayName: "Tower",
			Platform:    "unraid",
			Unraid: &models.HostUnraidStorage{
				ArrayStarted: true,
				ArrayState:   "STARTED",
				SyncAction:   "check",
				SyncProgress: 40,
				Disks: []models.HostUnraidDisk{
					{Name: "disk1", Role: "data", Status: "online", Device: "/dev/sdb"},
				},
			},
		}

		manager.CheckHost(host)
		manager.HandleHostTelemetryExpired(host)

		var storageAlert *Alert
		activeAlerts := manager.GetActiveAlerts()
		for i := range activeAlerts {
			alert := activeAlerts[i]
			if alert.Type == "storage-topology" {
				storageAlert = &alert
				break
			}
		}
		if storageAlert == nil {
			t.Fatal("static no-parity risk was cleared with transient operation state")
		}
		if strings.Contains(strings.ToLower(storageAlert.Message), "check") {
			t.Fatalf("expired operation remained in static alert: %q", storageAlert.Message)
		}
	})

	t.Run("degraded RAID risk survives transient rebuild expiry", func(t *testing.T) {
		manager := newTestManager(t)
		manager.ClearActiveAlerts()
		host := models.Host{
			ID:          "raid-degraded",
			Hostname:    "storage-host",
			DisplayName: "Storage Host",
			Platform:    "linux",
			RAID: []models.HostRAIDArray{{
				Device:         "/dev/md0",
				Level:          "raid1",
				State:          "degraded",
				TotalDevices:   2,
				ActiveDevices:  1,
				WorkingDevices: 1,
				FailedDevices:  1,
				Operation:      "recovery",
				RebuildPercent: 40,
				RebuildSpeed:   "100M/sec",
			}},
		}

		manager.CheckHost(host)
		manager.HandleHostTelemetryExpired(host)
		if !hasAlertType(manager.GetActiveAlerts(), "raid") {
			t.Fatal("static degraded RAID risk was cleared with transient rebuild state")
		}
	})
}

func hasAlertType(alerts []Alert, alertType string) bool {
	for _, alert := range alerts {
		if alert.Type == alertType {
			return true
		}
	}
	return false
}
