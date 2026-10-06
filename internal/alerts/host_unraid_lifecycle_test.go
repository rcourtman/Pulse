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

func TestHostCustomSensorEscalationDelivery(t *testing.T) {
	for _, policy := range []string{"ready", "acknowledged", "snoozed", "rate-limited", "flapping", "inactive"} {
		t.Run(policy, func(t *testing.T) {
			m := newTestManager(t)
			cfg := m.GetConfig()
			cfg.Enabled = true
			cfg.ActivationState = ActivationActive
			cfg.FlappingEnabled = false
			cfg.Schedule.MaxAlertsHour = 0
			m.UpdateConfig(cfg)
			var delivered []AlertLevel
			m.SetAlertCallback(func(a *Alert) {
				if a.Type == "custom-sensor" {
					delivered = append(delivered, a.Level)
				}
			})
			host := models.Host{ID: "sensor-host", Hostname: "sensor-host", Sensors: models.HostSensorSummary{
				Custom: []models.HostCustomSensorMetric{{ID: "probe", Name: "Probe", Status: "warning", ObservedAt: time.Now()}},
			}}
			m.CheckHost(host)
			if len(delivered) != 1 || delivered[0] != AlertLevelWarning {
				t.Fatalf("initial delivery = %v", delivered)
			}
			var id string
			for _, a := range m.GetActiveAlerts() {
				if a.Type == "custom-sensor" {
					id = a.ID
				}
			}
			if id == "" {
				t.Fatal("missing custom sensor incident")
			}
			switch policy {
			case "acknowledged":
				if err := m.AcknowledgeAlert(id, "tester"); err != nil {
					t.Fatal(err)
				}
			case "snoozed":
				if err := m.SnoozeAlert(id, "tester", time.Now().Add(time.Hour)); err != nil {
					t.Fatal(err)
				}
			case "rate-limited":
				a := testRequireActiveAlert(t, m, id)
				m.mu.Lock()
				m.config.Schedule.MaxAlertsHour = 1
				m.alertRateLimit[canonicalTrackingKeyForAlert(a)] = []time.Time{time.Now()}
				m.mu.Unlock()
			case "flapping":
				a := testRequireActiveAlert(t, m, id)
				m.mu.Lock()
				m.config.FlappingEnabled = true
				m.suppressedUntil[canonicalTrackingKeyForAlert(a)] = time.Now().Add(time.Hour)
				m.mu.Unlock()
			case "inactive":
				cfg.ActivationState = ActivationPending
				m.UpdateConfig(cfg)
			}
			// Repeated warning observations must not resend.
			m.CheckHost(host)
			host.Sensors.Custom[0].Status = "critical"
			m.CheckHost(host)
			want := 1
			if policy == "ready" {
				want = 2
			}
			if len(delivered) != want {
				t.Fatalf("warning->critical deliveries = %v, want %d callbacks", delivered, want)
			}
			if policy == "ready" && delivered[1] != AlertLevelCritical {
				t.Fatalf("escalation = %v", delivered)
			}
			active := m.GetActiveAlerts()
			found := false
			for _, a := range active {
				if a.Type == "custom-sensor" {
					found = true
					if a.ID != id || a.Level != AlertLevelCritical {
						t.Fatalf("updated incident = %+v", a)
					}
					if policy == "acknowledged" && !a.Acknowledged {
						t.Fatal("acknowledgement lost")
					}
				}
			}
			if !found {
				t.Fatal("critical incident missing")
			}
			m.CheckHost(host)
			host.Sensors.Custom[0].Status = "warning"
			m.CheckHost(host)
			m.CheckHost(host)
			if len(delivered) != want {
				t.Fatalf("unchanged/downgrade noise: %v", delivered)
			}
		})
	}
}

// An agent reports things besides its own machine: filesystems, disks,
// arrays and custom sensors. Their alerts target a child resource, and the
// alert card's monitoring policy writes to that child, so they must not
// carry the machine's "agent" type: the card would offer the machine's
// retirement copy ("Agent removal remains available from Machines") for a
// policy that never touches the machine. platformType keeps every one of
// them linked to the agent's page.
func TestHostChildAlertsNameTheirOwnResourceType(t *testing.T) {
	m := newTestManager(t)
	m.ClearActiveAlerts()
	m.mu.Lock()
	m.config.TimeThresholds = map[string]int{}
	m.config.AgentDefaults.CPU = &HysteresisThreshold{Trigger: 80, Clear: 70}
	m.config.AgentDefaults.Disk = &HysteresisThreshold{Trigger: 80, Clear: 70}
	m.config.AgentDefaults.DiskTemperature = &HysteresisThreshold{Trigger: 50, Clear: 45}
	m.mu.Unlock()

	pending := int64(2)
	sensorValue := 23.0
	linuxHost := models.Host{
		ID:          "child-linux",
		Hostname:    "storage-host",
		DisplayName: "Storage Host",
		Platform:    "linux",
		CPUUsage:    95,
		Disks: []models.Disk{{
			Mountpoint: "/srv",
			Device:     "/dev/sdc1",
			Usage:      92,
			Total:      100,
			Used:       92,
			Free:       8,
		}},
		Sensors: models.HostSensorSummary{
			SMART: []models.HostDiskSMART{{
				Device:      "/dev/sda",
				Model:       "IronWolf",
				Serial:      "SERIAL-CHILD-1",
				Health:      "PASSED",
				Temperature: 55,
				Attributes:  &models.SMARTAttributes{PendingSectors: &pending},
			}},
			Custom: []models.HostCustomSensorMetric{{
				ID:         "queue_depth",
				Name:       "Queue depth",
				Value:      &sensorValue,
				Status:     "critical",
				ObservedAt: time.Now().UTC(),
			}},
		},
		RAID: []models.HostRAIDArray{{
			Device:         "/dev/md0",
			Level:          "raid1",
			State:          "degraded",
			TotalDevices:   2,
			ActiveDevices:  1,
			WorkingDevices: 1,
			FailedDevices:  1,
		}},
	}
	unraidHost := models.Host{
		ID:          "child-unraid",
		Hostname:    "tower",
		DisplayName: "Tower",
		Platform:    "unraid",
		Unraid: &models.HostUnraidStorage{
			ArrayStarted: true,
			ArrayState:   "STARTED",
			Disks: []models.HostUnraidDisk{
				{Name: "disk1", Role: "data", Status: "online", Device: "/dev/sdb"},
			},
		},
	}
	m.CheckHost(linuxHost)
	m.CheckHost(unraidHost)

	want := map[string]string{
		"cpu":              "agent",
		"disk":             "agent-disk",
		"diskTemperature":  "agent-disk",
		"disk-health":      "agent-disk",
		"raid":             "agent-storage",
		"storage-topology": "agent-storage",
		"custom-sensor":    "agent-sensor",
	}
	seen := make(map[string]bool, len(want))
	for _, alert := range m.GetActiveAlerts() {
		wantType, ok := want[alert.Type]
		if !ok {
			continue
		}
		seen[alert.Type] = true
		if got := alert.Metadata["resourceType"]; got != wantType {
			t.Errorf("%s alert resourceType = %v, want %s", alert.Type, got, wantType)
		}
		if got := alert.Metadata["platformType"]; got != "agent" {
			t.Errorf("%s alert platformType = %v, want agent", alert.Type, got)
		}
		hostID, _ := alert.Metadata["hostId"].(string)
		if wantType != "agent" && alert.ResourceID == hostResourceID(hostID) {
			t.Errorf("%s alert targets the machine itself (%s), so it is not a child alert", alert.Type, alert.ResourceID)
		}
	}
	for alertType := range want {
		if !seen[alertType] {
			t.Errorf("expected an active %s alert", alertType)
		}
	}
}

// Child alerts keep the agent policy path. A configuration save that turns
// off storage or guest alerts must leave them alone: CheckHost only consults
// the agent switches, so resolving them here would re-raise and re-notify
// on the next report.
func TestHostChildAlertsIgnoreOtherPlatformSwitchesOnConfigSave(t *testing.T) {
	m := newTestManager(t)
	m.ClearActiveAlerts()
	m.mu.Lock()
	m.config.TimeThresholds = map[string]int{}
	m.mu.Unlock()

	pending := int64(2)
	sensorValue := 23.0
	host := models.Host{
		ID:       "child-policy",
		Hostname: "storage-host",
		Platform: "linux",
		Sensors: models.HostSensorSummary{
			SMART: []models.HostDiskSMART{{
				Device:     "/dev/sda",
				Serial:     "SERIAL-CHILD-2",
				Health:     "PASSED",
				Attributes: &models.SMARTAttributes{PendingSectors: &pending},
			}},
			Custom: []models.HostCustomSensorMetric{{
				ID:         "queue_depth",
				Name:       "Queue depth",
				Value:      &sensorValue,
				Status:     "critical",
				ObservedAt: time.Now().UTC(),
			}},
		},
		RAID: []models.HostRAIDArray{{
			Device:         "/dev/md0",
			Level:          "raid1",
			State:          "degraded",
			TotalDevices:   2,
			ActiveDevices:  1,
			WorkingDevices: 1,
			FailedDevices:  1,
		}},
	}
	m.CheckHost(host)
	childTypes := []string{"disk-health", "raid", "custom-sensor"}
	for _, alertType := range childTypes {
		if !hasAlertType(m.GetActiveAlerts(), alertType) {
			t.Fatalf("expected an active %s alert before the config save", alertType)
		}
	}

	m.mu.Lock()
	m.config.DisableAllStorage = true
	m.config.DisableAllGuests = true
	m.reevaluateActiveAlertsLocked()
	m.mu.Unlock()

	for _, alertType := range childTypes {
		if !hasAlertType(m.GetActiveAlerts(), alertType) {
			t.Errorf("%s alert was resolved by the storage or guest switch", alertType)
		}
	}

	m.mu.Lock()
	m.config.DisableAllAgents = true
	m.reevaluateActiveAlertsLocked()
	m.mu.Unlock()

	for _, alertType := range childTypes {
		if hasAlertType(m.GetActiveAlerts(), alertType) {
			t.Errorf("%s alert survived disabling agent alerts", alertType)
		}
	}
}

// The Proxmox node sweep removes alerts whose node is not a Proxmox node
// unless it recognises them as agent alerts, by an agent: resource id or the
// agent type. Child alerts no longer carry the agent type, so they must keep
// the agent: id their canonical spec gives them.
func TestHostChildAlertsSurviveProxmoxNodeCleanup(t *testing.T) {
	m := newTestManager(t)
	m.ClearActiveAlerts()
	pending := int64(2)
	sensorValue := 23.0
	host := models.Host{
		ID:       "child-sweep",
		Hostname: "storage-host",
		Platform: "linux",
		Sensors: models.HostSensorSummary{
			SMART: []models.HostDiskSMART{{
				Device:     "/dev/sda",
				Serial:     "SERIAL-CHILD-3",
				Health:     "PASSED",
				Attributes: &models.SMARTAttributes{PendingSectors: &pending},
			}},
			Custom: []models.HostCustomSensorMetric{{
				ID:         "queue_depth",
				Name:       "Queue depth",
				Value:      &sensorValue,
				Status:     "critical",
				ObservedAt: time.Now().UTC(),
			}},
		},
		RAID: []models.HostRAIDArray{{
			Device:         "/dev/md0",
			Level:          "raid1",
			State:          "degraded",
			TotalDevices:   2,
			ActiveDevices:  1,
			WorkingDevices: 1,
			FailedDevices:  1,
		}},
	}
	m.CheckHost(host)
	childTypes := []string{"disk-health", "raid", "custom-sensor"}
	for _, alertType := range childTypes {
		if !hasAlertType(m.GetActiveAlerts(), alertType) {
			t.Fatalf("expected an active %s alert", alertType)
		}
	}

	m.CleanupAlertsForNodes(map[string]bool{"pve1": true})

	for _, alertType := range childTypes {
		if !hasAlertType(m.GetActiveAlerts(), alertType) {
			t.Errorf("Proxmox node cleanup removed an agent %s alert", alertType)
		}
	}
}
