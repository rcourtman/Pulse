package alerts

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/reducer"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/diskinventory"
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

func TestCheckHostClearsDiskTemperatureAlertsWhenThresholdTurnsOff(t *testing.T) {
	cases := []struct {
		name    string
		turnOff func(m *Manager, hostID string)
	}{
		{
			name: "agent default disabled",
			turnOff: func(m *Manager, hostID string) {
				m.config.AgentDefaults.DiskTemperature = &HysteresisThreshold{Trigger: 0, Clear: 0}
			},
		},
		{
			name: "host override disables disk temperature",
			turnOff: func(m *Manager, hostID string) {
				m.config.Overrides[hostID] = ThresholdConfig{DiskTemperature: &HysteresisThreshold{Trigger: 0, Clear: 0}}
			},
		},
		{
			name: "host override disables the agent",
			turnOff: func(m *Manager, hostID string) {
				m.config.Overrides[hostID] = ThresholdConfig{Disabled: true}
			},
		},
		{
			name: "all agent alerts disabled",
			turnOff: func(m *Manager, hostID string) {
				m.config.DisableAllAgents = true
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := configureDiskTempTypeHostManager(t)
			host := hostWithSMARTDiskTemp("host-temp-off", "sata", 60)
			alertID := hostDiskTempAlertID(host)

			m.CheckHost(host)
			alert, exists := testLookupActiveAlert(t, m, alertID)
			if !exists {
				t.Fatalf("expected disk temperature alert at 60C (sata trigger 55), active: %v", alertKeys(m))
			}
			if alert.Type != "diskTemperature" || !strings.HasPrefix(alert.ResourceID, hostDiskTemperatureResourcePrefix(host.ID)) {
				t.Fatalf("disk temperature alert type/resource = %q/%q", alert.Type, alert.ResourceID)
			}

			// Change config directly so only CheckHost can resolve the alert.
			m.mu.Lock()
			tc.turnOff(m, host.ID)
			m.mu.Unlock()

			m.CheckHost(host)
			if _, exists := testLookupActiveAlert(t, m, alertID); exists {
				t.Fatalf("disk temperature alert stayed active after its threshold was turned off, active: %v", alertKeys(m))
			}
		})
	}
}

func TestHostDiskTemperatureAlertsClearWhenAgentLeaves(t *testing.T) {
	t.Run("agent removed", func(t *testing.T) {
		m := configureDiskTempTypeHostManager(t)
		host := hostWithSMARTDiskTemp("host-temp-removed", "sata", 60)
		m.CheckHost(host)
		if _, exists := testLookupActiveAlert(t, m, hostDiskTempAlertID(host)); !exists {
			t.Fatalf("expected disk temperature alert, active: %v", alertKeys(m))
		}

		m.HandleHostRemoved(host)
		if _, exists := testLookupActiveAlert(t, m, hostDiskTempAlertID(host)); exists {
			t.Fatalf("disk temperature alert outlived its removed agent, active: %v", alertKeys(m))
		}
	})

	t.Run("agent confirmed offline", func(t *testing.T) {
		m := configureDiskTempTypeHostManager(t)
		host := hostWithSMARTDiskTemp("host-temp-offline", "sata", 60)
		m.CheckHost(host)
		if _, exists := testLookupActiveAlert(t, m, hostDiskTempAlertID(host)); !exists {
			t.Fatalf("expected disk temperature alert, active: %v", alertKeys(m))
		}

		for i := 0; i < 3; i++ {
			m.HandleHostOffline(host)
		}
		if _, exists := testLookupActiveAlert(t, m, canonicalConnectivityStateID(hostResourceID(host.ID))); !exists {
			t.Fatalf("expected confirmed offline alert, active: %v", alertKeys(m))
		}
		if _, exists := testLookupActiveAlert(t, m, hostDiskTempAlertID(host)); exists {
			t.Fatalf("disk temperature alert stayed beside the offline alert, active: %v", alertKeys(m))
		}
	})
}

func TestCheckHostClearsDiskTemperatureAlertWhenDiskLeavesSMARTReport(t *testing.T) {
	const hostID = "host-temp-absent"
	smartDisk := func(device string, temperature int) models.HostDiskSMART {
		return models.HostDiskSMART{Device: device, Model: "test-disk", Type: "sata", Temperature: temperature}
	}
	alertIDFor := func(device string) string {
		return canonicalMetricStateID(hostDiskTemperatureResourceID(hostID, device), "diskTemperature")
	}
	sdaActive := func(m *Manager) bool {
		_, exists := testLookupActiveAlert(t, m, alertIDFor("/dev/sda"))
		return exists
	}
	// sda runs hot (sata trigger 55, clear 50); sdb stays cool.
	hotHost := func(t *testing.T) (*Manager, models.Host) {
		m := configureDiskTempTypeHostManager(t)
		host := hostWithSMARTDiskTemp(hostID, "sata", 60)
		host.Sensors.SMART = []models.HostDiskSMART{smartDisk("/dev/sda", 60), smartDisk("/dev/sdb", 40)}
		m.CheckHost(host)
		if !sdaActive(m) {
			t.Fatalf("expected sda disk temperature alert at 60C, active: %v", alertKeys(m))
		}
		return m, host
	}

	cases := []struct {
		name       string
		next       []models.HostDiskSMART
		wantHeld   bool
		wantRaised string
	}{
		{name: "disk removed", next: []models.HostDiskSMART{smartDisk("/dev/sdb", 40)}},
		{name: "device renamed", next: []models.HostDiskSMART{smartDisk("/dev/sdc", 60), smartDisk("/dev/sdb", 40)}, wantRaised: "/dev/sdc"},
		// No fresh reading says the disk cooled or left, so the alert holds.
		{name: "SMART collection empty", next: nil, wantHeld: true},
		{name: "disk in standby", next: []models.HostDiskSMART{{Device: "/dev/sda", Type: "sata", Temperature: 45, Standby: true}, smartDisk("/dev/sdb", 40)}, wantHeld: true},
		{name: "probe returned no temperature", next: []models.HostDiskSMART{{Device: "/dev/sda", Type: "sata"}, smartDisk("/dev/sdb", 40)}, wantHeld: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, host := hotHost(t)

			host.Sensors.SMART = tc.next
			for report := 1; report <= hostDiskTemperatureAbsenceConfirmations; report++ {
				m.CheckHost(host)
				wantActive := tc.wantHeld || report < hostDiskTemperatureAbsenceConfirmations
				if active := sdaActive(m); active != wantActive {
					t.Fatalf("report %d: sda disk temperature alert active = %v, want %v; active: %v", report, active, wantActive, alertKeys(m))
				}
			}
			if tc.wantRaised != "" {
				if _, exists := testLookupActiveAlert(t, m, alertIDFor(tc.wantRaised)); !exists {
					t.Fatalf("expected %s disk temperature alert, active: %v", tc.wantRaised, alertKeys(m))
				}
			}
			if !tc.wantHeld {
				return
			}

			// A held alert still resolves on the disk's next cool reading.
			host.Sensors.SMART = []models.HostDiskSMART{smartDisk("/dev/sda", 45), smartDisk("/dev/sdb", 40)}
			m.CheckHost(host)
			if sdaActive(m) {
				t.Fatalf("held sda disk temperature alert outlived a 45C reading, active: %v", alertKeys(m))
			}
		})
	}

	t.Run("intermittently dropped disk keeps its alert", func(t *testing.T) {
		m, host := hotHost(t)
		present := host.Sensors.SMART
		dropped := []models.HostDiskSMART{smartDisk("/dev/sdb", 40)}

		// The run of omissions restarts whenever the disk is reported again.
		for _, smart := range [][]models.HostDiskSMART{dropped, dropped, present, dropped, dropped} {
			host.Sensors.SMART = smart
			m.CheckHost(host)
			if !sdaActive(m) {
				t.Fatalf("sda disk temperature alert cleared by an omission run shorter than %d reports, active: %v", hostDiskTemperatureAbsenceConfirmations, alertKeys(m))
			}
		}
	})

	// pendingHost leaves a hot sda inside a 300s alert delay, so sda holds a
	// pending run and no alert.
	pendingHost := func(t *testing.T) (*Manager, models.Host, func() bool) {
		m := configureDiskTempTypeHostManager(t)
		m.mu.Lock()
		m.config.TimeThresholds = map[string]int{"agent": 300}
		m.mu.Unlock()
		sdaResourceID := hostDiskTemperatureResourceID(hostID, "/dev/sda")
		sdaPending := func() bool {
			m.mu.RLock()
			defer m.mu.RUnlock()
			for _, resourceID := range m.core.PendingResourceIDs() {
				if resourceID == sdaResourceID {
					return true
				}
			}
			return false
		}

		host := hostWithSMARTDiskTemp(hostID, "sata", 60)
		host.Sensors.SMART = []models.HostDiskSMART{smartDisk("/dev/sda", 60), smartDisk("/dev/sdb", 40)}
		m.CheckHost(host)
		if sdaActive(m) || !sdaPending() {
			t.Fatalf("expected a pending sda run inside the 300s delay, active: %v", alertKeys(m))
		}
		return m, host, sdaPending
	}

	t.Run("departed disk drops its pending run", func(t *testing.T) {
		m, host, sdaPending := pendingHost(t)

		host.Sensors.SMART = []models.HostDiskSMART{smartDisk("/dev/sdb", 40)}
		for report := 1; report <= hostDiskTemperatureAbsenceConfirmations; report++ {
			m.CheckHost(host)
			wantPending := false // A single omission interrupts pending evidence, not firing alerts.
			if pending := sdaPending(); pending != wantPending {
				t.Fatalf("report %d: pending sda run = %v, want %v", report, pending, wantPending)
			}
		}
	})

	t.Run("threshold off drops pending runs", func(t *testing.T) {
		m, host, sdaPending := pendingHost(t)

		m.mu.Lock()
		m.config.AgentDefaults.DiskTemperature = &HysteresisThreshold{Trigger: 0, Clear: 0}
		m.mu.Unlock()
		m.CheckHost(host)
		if sdaPending() {
			t.Fatal("pending sda run survived its threshold being turned off; re-enabling would inherit its old start time")
		}
	})

	t.Run("removed host leaves no pending run or absence count", func(t *testing.T) {
		m, host, sdaPending := pendingHost(t)

		host.Sensors.SMART = []models.HostDiskSMART{smartDisk("/dev/sdb", 40)}
		m.CheckHost(host)
		m.HandleHostRemoved(host)
		if sdaPending() {
			t.Fatal("pending sda run outlived its removed host")
		}
		m.mu.RLock()
		defer m.mu.RUnlock()
		if len(m.hostDiskTempAbsences) != 0 {
			t.Fatalf("absence counts outlived their removed host: %v", m.hostDiskTempAbsences)
		}
	})
}

func TestConfigSaveResolvesDiskTemperatureAlertAgainstItsDiskTypeThreshold(t *testing.T) {
	m := configureDiskTempTypeHostManager(t)

	m.mu.Lock()
	m.config.AgentDefaults.DiskTemperature = &HysteresisThreshold{Trigger: 75, Clear: 70}
	m.mu.Unlock()

	// 72C is above the nvme trigger (70) but below the agent default (75).
	host := hostWithSMARTDiskTemp("host-temp-save", "nvme", 72)
	alertID := hostDiskTempAlertID(host)
	m.CheckHost(host)
	if _, exists := testLookupActiveAlert(t, m, alertID); !exists {
		t.Fatalf("expected nvme disk temperature alert at 72C, active: %v", alertKeys(m))
	}

	// An unrelated save keeps the alert CheckHost would raise again.
	cfg := m.GetConfig()
	cfg.AgentDefaults.CPU = &HysteresisThreshold{Trigger: 85, Clear: 80}
	m.UpdateConfig(cfg)
	if _, exists := testLookupActiveAlert(t, m, alertID); !exists {
		t.Fatalf("unrelated config save resolved a disk temperature alert above its nvme trigger, active: %v", alertKeys(m))
	}

	// Turning disk temperature off resolves it on save, before any report.
	cfg = m.GetConfig()
	cfg.AgentDefaults.DiskTemperature = &HysteresisThreshold{Trigger: 0, Clear: 0}
	m.UpdateConfig(cfg)
	if _, exists := testLookupActiveAlert(t, m, alertID); exists {
		t.Fatalf("disk temperature alert stayed active after the threshold was turned off, active: %v", alertKeys(m))
	}
}

func TestConfigSaveKeepsDiskTemperatureAlertWithoutDiskType(t *testing.T) {
	m := configureDiskTempTypeHostManager(t)

	m.mu.Lock()
	m.config.AgentDefaults.DiskTemperature = &HysteresisThreshold{Trigger: 75, Clear: 70}
	m.mu.Unlock()

	host := hostWithSMARTDiskTemp("host-temp-legacy", "nvme", 72)
	alertID := hostDiskTempAlertID(host)
	m.CheckHost(host)
	alert, exists := testLookupActiveAlert(t, m, alertID)
	if !exists {
		t.Fatalf("expected nvme disk temperature alert at 72C, active: %v", alertKeys(m))
	}
	// Restored from a release that did not record the disk type.
	m.mu.Lock()
	delete(alert.Metadata, "diskType")
	m.mu.Unlock()

	// 72C is above the nvme trigger (70) but below the agent default (75).
	cfg := m.GetConfig()
	cfg.AgentDefaults.CPU = &HysteresisThreshold{Trigger: 85, Clear: 80}
	m.UpdateConfig(cfg)
	if _, exists := testLookupActiveAlert(t, m, alertID); !exists {
		t.Fatalf("config save resolved a disk temperature alert of unknown disk type that its nvme trigger still fires, active: %v", alertKeys(m))
	}

	cfg = m.GetConfig()
	cfg.AgentDefaults.DiskTemperature = &HysteresisThreshold{Trigger: 0, Clear: 0}
	m.UpdateConfig(cfg)
	if _, exists := testLookupActiveAlert(t, m, alertID); exists {
		t.Fatalf("disk temperature alert of unknown disk type stayed active after the threshold was turned off, active: %v", alertKeys(m))
	}
}

func TestConfigSaveKeepsAlertTheEvaluatorKeepsWithoutRecoveryBand(t *testing.T) {
	// Normalization keeps a 70/75 entry, and the evaluator treats a clear
	// level at or above the trigger as no recovery band, firing from 70C.
	for _, reading := range []int{72, 70} {
		t.Run(fmt.Sprintf("%dC", reading), func(t *testing.T) {
			m := configureDiskTempTypeHostManager(t)
			m.mu.Lock()
			m.config.DiskTempByType["nvme"] = HysteresisThreshold{Trigger: 70, Clear: 75}
			m.mu.Unlock()

			host := hostWithSMARTDiskTemp("host-temp-no-band", "nvme", reading)
			alertID := hostDiskTempAlertID(host)
			m.CheckHost(host)
			if _, exists := testLookupActiveAlert(t, m, alertID); !exists {
				t.Fatalf("expected nvme disk temperature alert at %dC, active: %v", reading, alertKeys(m))
			}

			cfg := m.GetConfig()
			cfg.AgentDefaults.CPU = &HysteresisThreshold{Trigger: 85, Clear: 80}
			m.UpdateConfig(cfg)
			if _, exists := testLookupActiveAlert(t, m, alertID); !exists {
				t.Fatalf("config save resolved an alert the evaluator keeps firing at %dC, active: %v", reading, alertKeys(m))
			}

			m.CheckHost(host)
			if _, exists := testLookupActiveAlert(t, m, alertID); !exists {
				t.Fatalf("expected the next report to keep the alert, active: %v", alertKeys(m))
			}
		})
	}
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

// Disabling between two reports is a real policy interval even when there is
// no firing alert for UpdateConfig to visit. Linked overrides keep precedence.
func TestDiskTemperaturePendingConfigDisablement(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		for _, scope := range []string{"global", "agent-default", "all-agents", "host", "host-disabled", "node", "vm", "container", "type", "unrelated"} {
			t.Run(fmt.Sprintf("%s/explicit=%v", scope, explicit), func(t *testing.T) {
				m, elapsed := continuityManager(t, explicit)
				host := hostWithSMARTDiskTemp("pending-temp", "custom", 85)
				host.LinkedNodeID, host.LinkedVMID, host.LinkedContainerID = "site:node", "site:node:101", "site:node:102"
				id := hostDiskTemperatureResourceID(host.ID, host.Sensors.SMART[0].Device)
				m.CheckHost(host)
				if incident, ok := continuityIncident(m, id, "diskTemperature"); !ok || incident.State != reducer.StatePending {
					t.Fatalf("no initial pending run: %+v %v", incident, ok)
				}
				cfg := m.GetConfig()
				switch scope {
				case "global":
					cfg.Enabled = false
				case "agent-default":
					cfg.AgentDefaults.DiskTemperature = &HysteresisThreshold{Trigger: 0}
				case "host-disabled":
					cfg.Overrides[host.ID] = ThresholdConfig{Disabled: true}
				case "all-agents":
					cfg.DisableAllAgents = true
				case "host":
					cfg.Overrides[host.ID] = ThresholdConfig{DiskTemperature: &HysteresisThreshold{Trigger: 0}}
				case "node":
					cfg.Overrides[host.LinkedNodeID] = ThresholdConfig{DiskTemperature: &HysteresisThreshold{Trigger: 0}}
				case "vm":
					host.LinkedNodeID = "" // The linked node override must not mask the VM.
					m.CheckHost(host)
					cfg.Overrides[host.LinkedVMID] = ThresholdConfig{DiskTemperature: &HysteresisThreshold{Trigger: 0}}
				case "container":
					host.LinkedNodeID, host.LinkedVMID = "", ""
					m.CheckHost(host)
					cfg.Overrides[host.LinkedContainerID] = ThresholdConfig{DiskTemperature: &HysteresisThreshold{Trigger: 0}}
				case "type":
					cfg.DiskTempByType = map[string]HysteresisThreshold{"custom": {Trigger: 0}}
				case "unrelated":
					cfg.Schedule.QuietHours.Timezone = "UTC"
				}
				m.UpdateConfig(cfg)
				_, pending := continuityIncident(m, id, "diskTemperature")
				if pending != (scope == "unrelated") {
					t.Fatalf("after save pending=%v; unrelated saves alone must preserve evidence", pending)
				}
				if scope == "unrelated" {
					return
				}
				cfg.Enabled, cfg.DisableAllAgents = true, false
				cfg.AgentDefaults.DiskTemperature = &HysteresisThreshold{Trigger: 80, Clear: 70}
				cfg.Overrides = map[string]ThresholdConfig{}
				cfg.DiskTempByType = map[string]HysteresisThreshold{"custom": {Trigger: 80, Clear: 70}}
				m.UpdateConfig(cfg)
				elapsed.Store(int64(2 * time.Minute))
				m.CheckHost(host)
				if len(m.GetActiveAlerts()) != 0 {
					t.Fatal("re-enable inherited the pre-disable start")
				}
				elapsed.Store(int64(2*time.Minute + 59*time.Second))
				m.CheckHost(host)
				if len(m.GetActiveAlerts()) != 0 {
					t.Fatal("fresh grace fired early")
				}
				elapsed.Store(int64(3 * time.Minute))
				m.CheckHost(host)
				if len(m.GetActiveAlerts()) != 1 {
					t.Fatal("fresh continuous breach never fired")
				}
				m.mu.RLock()
				defer m.mu.RUnlock()
				if len(m.hostDiskTempPendingContexts) != 0 {
					t.Fatal("context outlived the pending run")
				}
			})
		}
	}
}

func TestDiskTemperaturePendingRestartContextIsNotInvented(t *testing.T) {
	m, elapsed := continuityManager(t, true)
	host := hostWithSMARTDiskTemp("restored-temp", "sata", 85)
	id := hostDiskTemperatureResourceID(host.ID, host.Sensors.SMART[0].Device)
	m.CheckHost(host)
	m.mu.Lock()
	// Persisted intent carries grace but not host link/type context. Reducer
	// pending incidents are not restored; this is the actual restart shape.
	m.core.Reset()
	m.hostDiskTempPendingContexts = nil
	key := canonicalMetricStateID(id, "diskTemperature")
	_, pending := m.intentPending[key]
	m.mu.Unlock()
	if !pending {
		t.Fatal("explicit intent control did not retain grace")
	}
	m.UpdateConfig(m.GetConfig())
	m.mu.RLock()
	_, pending = m.intentPending[key]
	m.mu.RUnlock()
	if pending {
		t.Fatal("save retained grace whose link/type enablement is unknown")
	}
	elapsed.Store(int64(2 * time.Minute))
	m.CheckHost(host)
	if len(m.GetActiveAlerts()) != 0 {
		t.Fatal("restored grace fired without fresh evidence")
	}
}

func TestDiskTemperatureCurrentCollectionAndOverrideControls(t *testing.T) {
	for _, provenance := range []string{"legacy", "available", "other-field-only"} {
		t.Run(provenance, func(t *testing.T) {
			m, elapsed := continuityManager(t, false)
			host := hostWithSMARTDiskTemp("current-temp", "sata", 85)
			switch provenance {
			case "available":
				host.Sensors.SMART[0].Collection = &diskinventory.CollectionStatus{Temperature: diskinventory.Available("smartctl")}
			case "other-field-only":
				host.Sensors.SMART[0].Collection = &diskinventory.CollectionStatus{Serial: diskinventory.Available("smartctl")}
			}
			m.CheckHost(host)
			elapsed.Store(int64(time.Minute))
			m.CheckHost(host)
			if len(m.GetActiveAlerts()) != 1 {
				t.Fatal("continuous accepted temperature did not fire")
			}
		})
	}
	t.Run("host override wins over disabled type", func(t *testing.T) {
		m, elapsed := continuityManager(t, false)
		host := hostWithSMARTDiskTemp("override-temp", "custom", 85)
		cfg := m.GetConfig()
		cfg.DiskTempByType = map[string]HysteresisThreshold{"custom": {Trigger: 0}}
		cfg.Overrides[host.ID] = ThresholdConfig{DiskTemperature: &HysteresisThreshold{Trigger: 80, Clear: 70}}
		m.UpdateConfig(cfg)
		m.CheckHost(host)
		m.UpdateConfig(m.GetConfig())
		elapsed.Store(int64(time.Minute))
		m.CheckHost(host)
		if len(m.GetActiveAlerts()) != 1 {
			t.Fatal("save discarded an enabled explicit override")
		}
	})
}

func TestDiskTemperaturePendingContextCleanup(t *testing.T) {
	for _, removal := range []bool{false, true} {
		t.Run(fmt.Sprintf("removed=%v", removal), func(t *testing.T) {
			m, _ := continuityManager(t, false)
			host := hostWithSMARTDiskTemp("pending-cleanup", "custom", 85)
			m.CheckHost(host)
			m.mu.RLock()
			tracked := len(m.hostDiskTempPendingContexts)
			m.mu.RUnlock()
			if tracked != 1 || len(m.GetActiveAlerts()) != 0 {
				t.Fatal("control must have pending context and no active alert")
			}
			if removal {
				m.HandleHostRemoved(host)
			} else {
				m.ClearActiveAlerts()
			}
			m.mu.RLock()
			defer m.mu.RUnlock()
			if len(m.hostDiskTempPendingContexts) != 0 || m.hostDiskTemperaturePendingNoLock(hostDiskTemperatureResourceID(host.ID, host.Sensors.SMART[0].Device)) {
				t.Fatal("pending context/state outlived its host or explicit clear")
			}
		})
	}
}

// Feed only synthetic telemetry through the real CheckHost, config and callback
// paths. No collector, smartctl process, native host or notification provider.
type smartLifecycleFixture struct {
	m                *Manager
	host             models.Host
	healthID, wearID string
	initial          map[string]Alert
	fires            atomic.Int32
	recoveries       chan *ResolvedAlert
}

func newSMARTLifecycleFixture(t *testing.T) *smartLifecycleFixture {
	t.Helper()
	m := newEventLogManager(t)
	cfg := m.GetConfig()
	cfg.FlappingEnabled = false
	cfg.Schedule.Cooldown = 0
	cfg.Schedule.MaxAlertsHour = 0
	cfg.AgentDefaults.Disk = &HysteresisThreshold{Trigger: 80, Clear: 70}
	cfg.AgentDefaults.DiskTemperature = &HysteresisThreshold{Trigger: 80, Clear: 70}
	cfg.DiskTempByType = map[string]HysteresisThreshold{"sata": {Trigger: 80, Clear: 70}}
	m.UpdateConfig(cfg)
	m.mu.Lock()
	m.config.TimeThresholds = map[string]int{}
	m.config.MetricTimeThresholds = map[string]map[string]int{"all": {"disk": 0, "diskTemperature": 0}}
	m.mu.Unlock()
	used, pending := 96, int64(2)
	host := models.Host{ID: "smart-lifecycle", Hostname: "storage", Status: "online",
		Disks: []models.Disk{{Mountpoint: "/data", Device: "/dev/sda1", Total: 1000, Used: 950, Usage: 95}},
		Sensors: models.HostSensorSummary{SMART: []models.HostDiskSMART{
			{Device: "/dev/sda", Type: "sata", Health: "FAILED", Temperature: 95, Attributes: &models.SMARTAttributes{PercentageUsed: &used, PendingSectors: &pending}},
			{Device: "/dev/sdb", Type: "sata", Health: "PASSED", Temperature: 20},
		}},
	}
	resourceID, _ := hostSMARTDiskResourceID(host, host.Sensors.SMART[0])
	f := &smartLifecycleFixture{m: m, host: host,
		healthID: buildCanonicalStateID(resourceID, resourceID+"-disk-health"),
		wearID:   buildCanonicalStateID(resourceID, resourceID+"-disk-wearout"),
		initial:  make(map[string]Alert), recoveries: make(chan *ResolvedAlert, 32),
	}
	m.SetAlertCallback(func(a *Alert) {
		if a.Type == "disk-health" || a.Type == "disk-wearout" {
			f.fires.Add(1)
		}
	})
	m.SetResolvedAlertCallback(func(r *ResolvedAlert) {
		if r.Alert.Type == "disk-health" || r.Alert.Type == "disk-wearout" {
			f.recoveries <- r
		}
	})
	m.CheckHost(host)
	for _, id := range []string{f.healthID, f.wearID} {
		f.initial[id] = *testRequireActiveAlert(t, m, id)
	}
	if f.fires.Load() != 2 {
		t.Fatalf("initial risk callbacks = %d, want 2", f.fires.Load())
	}
	return f
}

func (f *smartLifecycleFixture) held(t *testing.T) {
	t.Helper()
	for _, id := range []string{f.healthID, f.wearID} {
		a := testRequireActiveAlert(t, f.m, id)
		if !a.StartTime.Equal(f.initial[id].StartTime) {
			t.Fatalf("%s was replaced by a new occurrence", id)
		}
	}
	if f.fires.Load() != 2 {
		t.Fatalf("missing report refired risk notifications: %d", f.fires.Load())
	}
	for _, e := range queryAlertEvents(t, f.m, eventlog.Filter{Types: []string{eventlog.TypeResolved}}) {
		if e.AlertID == f.healthID || e.AlertID == f.wearID {
			t.Fatalf("missing evidence published recovery: %+v", e)
		}
	}
}

func (f *smartLifecycleFixture) resolved(t *testing.T, ids ...string) {
	t.Helper()
	want := make(map[string]bool)
	for _, id := range ids {
		if testHasActiveAlert(t, f.m, id) {
			t.Fatalf("risk alert remains active: %s", id)
		}
		want[id] = true
	}
	for range ids {
		select {
		case r := <-f.recoveries:
			if !want[r.Alert.ID] || !r.Alert.StartTime.Equal(f.initial[r.Alert.ID].StartTime) {
				t.Fatalf("wrong recovery occurrence: %+v", r)
			}
			delete(want, r.Alert.ID)
		case <-time.After(3 * time.Second):
			t.Fatal("ordinary recovery callback did not complete")
		}
	}
	for _, id := range ids {
		events := queryAlertEvents(t, f.m, eventlog.Filter{AlertID: id, Types: []string{eventlog.TypeResolved}})
		if len(events) != 1 {
			t.Fatalf("%s has %d resolution events, want 1", id, len(events))
		}
	}
}

func TestHostSMARTRiskMissingReportsPreserveOccurrenceAndCallbacks(t *testing.T) {
	f := newSMARTLifecycleFixture(t)
	if err := f.m.AcknowledgeAlert(f.healthID, "operator"); err != nil {
		t.Fatal(err)
	}
	present := f.host.Sensors.SMART
	// Both risk families on one disk must count once per report, not twice.
	for _, list := range [][]models.HostDiskSMART{nil, {present[1]}, {present[1]}, nil, {present[1]}, present, nil, present} {
		f.host.Sensors.SMART = list
		f.m.CheckHost(f.host)
		f.held(t)
		if !testRequireActiveAlert(t, f.m, f.healthID).Acknowledged {
			t.Fatal("acknowledgement lost during observation gap")
		}
	}
	// Standby and unavailable health/wear fields are presence, not recovery.
	f.host.Sensors.SMART = []models.HostDiskSMART{{Device: "/dev/sda", Standby: true}, present[1]}
	f.m.CheckHost(f.host)
	f.held(t)
	f.host.Sensors.SMART[0].Standby = false
	f.m.CheckHost(f.host)
	f.held(t)
	used, pending := 10, int64(0)
	f.host.Sensors.SMART[0] = models.HostDiskSMART{Device: "/dev/sda", Health: "PASSED", Temperature: 20, Attributes: &models.SMARTAttributes{PercentageUsed: &used, PendingSectors: &pending}}
	f.m.CheckHost(f.host)
	f.resolved(t, f.healthID, f.wearID)
	if f.fires.Load() != 2 {
		t.Fatalf("healthy recovery refired: %d", f.fires.Load())
	}
	t.Log("empty/partial/standby/unknown reports preserved occurrence and acknowledgement; fresh complete evidence recovered once")
}

func TestHostSMARTRiskDepartureRequiresFreshConsecutiveReports(t *testing.T) {
	for _, gap := range []string{"empty", "expiry", "present"} {
		t.Run(gap, func(t *testing.T) {
			f := newSMARTLifecycleFixture(t)
			present := f.host.Sensors.SMART
			f.host.Sensors.SMART = []models.HostDiskSMART{present[1]}
			for i := 0; i < 2; i++ {
				f.m.CheckHost(f.host)
				f.held(t)
			}
			switch gap {
			case "empty":
				f.host.Sensors.SMART = nil
				f.m.CheckHost(f.host)
			case "expiry":
				f.m.HandleHostTelemetryExpired(f.host)
			case "present":
				f.host.Sensors.SMART = present
				f.m.CheckHost(f.host)
			}
			f.held(t)
			f.host.Sensors.SMART = []models.HostDiskSMART{present[1]}
			for i := 1; i <= 3; i++ {
				f.m.CheckHost(f.host)
				if i < 3 {
					f.held(t)
				}
			}
			f.resolved(t, f.healthID, f.wearID)
			if f.fires.Load() != 2 {
				t.Fatalf("departure refired %d callbacks", f.fires.Load())
			}
		})
	}
}

func TestHostSMARTRiskRulesDisableBetweenReports(t *testing.T) {
	f := newSMARTLifecycleFixture(t)
	cfg := f.m.GetConfig()
	off := 0
	cfg.AgentDefaults.SMARTHealthFailure = &off
	f.m.UpdateConfig(cfg)
	f.held(t) // Pending sectors remain enabled; disabling only one cause is not recovery.
	zero := int64(0)
	cfg.AgentDefaults.SMARTPending = &zero
	f.m.UpdateConfig(cfg)
	f.resolved(t, f.healthID)
	if !testHasActiveAlert(t, f.m, f.wearID) {
		t.Fatal("health policy disabled unrelated wearout")
	}
	// A missing report cannot resurrect it. Re-enable before another report;
	// only a newly observed failing disk may start another occurrence.
	f.host.Sensors.SMART = nil
	f.m.CheckHost(f.host)
	if testHasActiveAlert(t, f.m, f.healthID) {
		t.Fatal("missing report raised disabled health")
	}
	on, one := 1, int64(1)
	cfg.AgentDefaults.SMARTHealthFailure = &on
	cfg.AgentDefaults.SMARTPending = &one
	f.m.UpdateConfig(cfg)
	f.m.CheckHost(f.host)
	if testHasActiveAlert(t, f.m, f.healthID) {
		t.Fatal("re-enable raised health without evidence")
	}
	used, pending := 96, int64(2)
	f.host.Sensors.SMART = []models.HostDiskSMART{{Device: "/dev/sda", Health: "FAILED", Attributes: &models.SMARTAttributes{PercentageUsed: &used, PendingSectors: &pending}}}
	f.m.CheckHost(f.host)
	a := testRequireActiveAlert(t, f.m, f.healthID)
	if a.StartTime.Equal(f.initial[f.healthID].StartTime) || f.fires.Load() != 3 {
		t.Fatal("fresh bad evidence did not start exactly one new health occurrence")
	}
}

func TestHostSMARTRiskEmptyReportHonoursRuleOffAndLegacyUnknowns(t *testing.T) {
	for _, metadata := range []string{"current", "legacy", "unknown"} {
		t.Run(metadata, func(t *testing.T) {
			f := newSMARTLifecycleFixture(t)
			off, zero := 0, int64(0)
			f.m.mu.Lock()
			f.m.config.AgentDefaults.SMARTHealthFailure = &off
			f.m.config.AgentDefaults.SMARTPending = &zero
			if metadata != "current" {
				a, _ := f.m.getActiveAlertNoLock(f.healthID)
				if metadata == "legacy" {
					delete(a.Metadata, "riskCodes")
				} else {
					a.Metadata["riskCodes"] = []interface{}{"future-code"}
				}
			}
			f.m.mu.Unlock()
			f.host.Sensors.SMART = nil
			f.m.CheckHost(f.host)
			if metadata == "current" {
				f.resolved(t, f.healthID)
			} else {
				f.held(t)
			}
			if !testHasActiveAlert(t, f.m, f.wearID) {
				t.Fatal("wearout was disabled with health")
			}
		})
	}
}

func TestHostSMARTRiskFilesystemAndHostLifecycleStayIndependent(t *testing.T) {
	for _, action := range []string{"filesystem-absent", "node-link", "removed", "host-disabled", "agents-disabled", "host-isolation"} {
		t.Run(action, func(t *testing.T) {
			f := newSMARTLifecycleFixture(t)
			switch action {
			case "filesystem-absent":
				fsID, _ := hostDiskResourceID(f.host, f.host.Disks[0])
				id := canonicalMetricStateID(fsID, "disk")
				testRequireActiveAlert(t, f.m, id)
				f.host.Disks = nil
				f.host.Sensors.SMART = nil
				f.m.CheckHost(f.host)
				if testHasActiveAlert(t, f.m, id) {
					t.Fatal("filesystem absence incorrectly held usage")
				}
				f.held(t)
			case "host-isolation":
				other := f.host
				other.ID = "smart-lifecycle-other"
				f.m.CheckHost(other)
				other.Sensors.SMART = []models.HostDiskSMART{other.Sensors.SMART[1]}
				for i := 0; i < 3; i++ {
					f.m.CheckHost(other)
				}
				f.held(t)
			default:
				present := f.host.Sensors.SMART
				f.host.Sensors.SMART = []models.HostDiskSMART{present[1]}
				f.m.CheckHost(f.host)
				f.held(t)
				switch action {
				case "node-link":
					f.host.LinkedNodeID = "pve-node"
					f.host.Sensors.SMART = nil
					f.m.CheckHost(f.host)
				case "removed":
					f.m.HandleHostRemoved(f.host)
				case "host-disabled":
					cfg := f.m.GetConfig()
					cfg.Overrides["agent:"+f.host.ID] = ThresholdConfig{Disabled: true}
					f.m.UpdateConfig(cfg)
				case "agents-disabled":
					cfg := f.m.GetConfig()
					cfg.DisableAllAgents = true
					f.m.UpdateConfig(cfg)
				}
				f.resolved(t, f.healthID, f.wearID)
				// Returning ownership gets a new occurrence and a fresh departure run.
				f.host.LinkedNodeID = ""
				cfg := f.m.GetConfig()
				cfg.DisableAllAgents = false
				delete(cfg.Overrides, "agent:"+f.host.ID)
				f.m.UpdateConfig(cfg)
				f.host.Sensors.SMART = present
				f.m.CheckHost(f.host)
				f.host.Sensors.SMART = []models.HostDiskSMART{present[1]}
				for i := 0; i < 2; i++ {
					f.m.CheckHost(f.host)
					testRequireActiveAlert(t, f.m, f.healthID)
					testRequireActiveAlert(t, f.m, f.wearID)
				}
			}
		})
	}
}

func TestHostSMARTRiskFilesystemIdentityCannotSubstituteInventory(t *testing.T) {
	for _, direction := range []string{"SMART-only", "filesystem-only"} {
		t.Run(direction, func(t *testing.T) {
			f := newSMARTLifecycleFixture(t)
			f.host.Disks[0].Mountpoint = "/sda"
			f.m.CheckHost(f.host)
			diskResourceID, _ := hostDiskResourceID(f.host, f.host.Disks[0])
			riskResourceID, _ := hostSMARTDiskResourceID(f.host, f.host.Sensors.SMART[0])
			if diskResourceID != riskResourceID {
				t.Fatal("control must share the disk resource ID")
			}
			usageID := canonicalMetricStateID(diskResourceID, "disk")
			testRequireActiveAlert(t, f.m, usageID)
			if direction == "SMART-only" {
				f.host.Disks = nil
				f.m.CheckHost(f.host)
				if testHasActiveAlert(t, f.m, usageID) {
					t.Fatal("SMART identity held missing filesystem usage")
				}
				f.held(t)
			} else {
				f.host.Sensors.SMART = []models.HostDiskSMART{f.host.Sensors.SMART[1]}
				for i := 1; i <= 3; i++ {
					f.m.CheckHost(f.host)
					testRequireActiveAlert(t, f.m, usageID)
					if i < 3 {
						f.held(t)
					}
				}
				f.resolved(t, f.healthID, f.wearID)
			}
		})
	}
}

func TestHostSMARTRiskRestartAndGlobalPauseRestartDeparture(t *testing.T) {
	for _, gap := range []string{"restart", "global-pause"} {
		t.Run(gap, func(t *testing.T) {
			f := newSMARTLifecycleFixture(t)
			if err := f.m.AcknowledgeAlert(f.healthID, "operator"); err != nil {
				t.Fatal(err)
			}
			f.host.Sensors.SMART = []models.HostDiskSMART{f.host.Sensors.SMART[1]}
			for i := 0; i < 2; i++ {
				f.m.CheckHost(f.host)
				f.held(t)
			}
			cfg := f.m.GetConfig()
			if gap == "restart" {
				dir := filepath.Dir(f.m.alertsDir)
				f.m.Stop() // Actual checkpoint and ordinary persisted-alert restoration.
				f.m = NewManagerWithDataDir(dir)
				t.Cleanup(f.m.Stop)
				f.m.UpdateConfig(cfg)
				f.m.SetAlertCallback(func(a *Alert) {
					if a.Type == "disk-health" || a.Type == "disk-wearout" {
						f.fires.Add(1)
					}
				})
				f.m.SetResolvedAlertCallback(func(r *ResolvedAlert) {
					if r.Alert.Type == "disk-health" || r.Alert.Type == "disk-wearout" {
						f.recoveries <- r
					}
				})
			} else {
				cfg.Enabled = false
				f.m.UpdateConfig(cfg)
				cfg.Enabled = true
				f.m.UpdateConfig(cfg)
			}
			for i := 1; i <= 3; i++ {
				f.m.CheckHost(f.host)
				if i < 3 {
					f.held(t)
					if !testRequireActiveAlert(t, f.m, f.healthID).Acknowledged {
						t.Fatal("gap lost persisted acknowledgement")
					}
				}
			}
			f.resolved(t, f.healthID, f.wearID)
		})
	}
}
