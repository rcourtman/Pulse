package alerts

import (
	"testing"
	"time"

	alertspecs "github.com/rcourtman/pulse-go-rewrite/internal/alerts/specs"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
)

func boolPtr(v bool) *bool {
	return &v
}

func TestApplyThresholdOverrideIncludesLifecycleFields(t *testing.T) {
	m := newTestManager(t)

	base := ThresholdConfig{
		PoweredOffSeverity: AlertLevelWarning,
		Backup: &BackupAlertConfig{
			Enabled:       true,
			WarningDays:   3,
			CriticalDays:  7,
			FreshHours:    24,
			StaleHours:    72,
			AlertOrphaned: boolPtr(true),
			IgnoreVMIDs:   []string{"100"},
		},
		Snapshot: &SnapshotAlertConfig{
			Enabled:      true,
			WarningDays:  7,
			CriticalDays: 14,
		},
	}

	override := ThresholdConfig{
		PoweredOffSeverity: AlertLevelCritical,
		Backup: &BackupAlertConfig{
			Enabled:       false,
			WarningDays:   10,
			CriticalDays:  20,
			FreshHours:    12,
			StaleHours:    36,
			AlertOrphaned: boolPtr(false),
			IgnoreVMIDs:   []string{"200", "201"},
		},
		Snapshot: &SnapshotAlertConfig{
			Enabled:         false,
			WarningDays:     30,
			CriticalDays:    60,
			WarningSizeGiB:  10,
			CriticalSizeGiB: 20,
		},
	}

	got := m.applyThresholdOverride(base, override)

	if got.PoweredOffSeverity != AlertLevelCritical {
		t.Fatalf("PoweredOffSeverity = %q, want %q", got.PoweredOffSeverity, AlertLevelCritical)
	}
	if got.Backup == nil || got.Backup.Enabled {
		t.Fatalf("Backup override not applied: %+v", got.Backup)
	}
	if got.Backup.AlertOrphaned == nil || *got.Backup.AlertOrphaned {
		t.Fatalf("Backup AlertOrphaned override not applied: %+v", got.Backup)
	}
	if len(got.Backup.IgnoreVMIDs) != 2 || got.Backup.IgnoreVMIDs[0] != "200" {
		t.Fatalf("Backup IgnoreVMIDs override not applied: %+v", got.Backup.IgnoreVMIDs)
	}
	if got.Snapshot == nil || got.Snapshot.Enabled {
		t.Fatalf("Snapshot override not applied: %+v", got.Snapshot)
	}
}

func TestCheckGuestStoppedUsesResolvedThresholdsForPoweredOff(t *testing.T) {
	m := newTestManager(t)
	guestID := BuildGuestKey("pve1", "node1", 100)

	m.mu.Lock()
	m.config.Enabled = true
	m.config.CustomRules = []CustomAlertRule{
		{
			Name:     "disable-powered-off",
			Enabled:  true,
			Priority: 10,
			FilterConditions: FilterStack{
				LogicalOperator: "AND",
			},
			Thresholds: ThresholdConfig{
				DisableConnectivity: true,
			},
		},
	}
	m.mu.Unlock()

	vm := models.VM{
		ID:       guestID,
		VMID:     100,
		Name:     "app01",
		Node:     "node1",
		Instance: "pve1",
		Status:   "stopped",
	}

	m.CheckGuest(vm, "pve1")
	m.CheckGuest(vm, "pve1")

	m.mu.RLock()
	_, alertExists := m.activeAlerts["guest-powered-off-"+guestID]
	confirmationExists := testCoreHasIncident(m, guestID, canonicalPoweredStateSpecID(guestID))
	m.mu.RUnlock()

	if alertExists {
		t.Fatalf("expected no powered-off alert for %q when resolved thresholds disable connectivity", guestID)
	}
	if confirmationExists {
		t.Fatalf("expected no powered-off confirmations for %q when resolved thresholds disable connectivity", guestID)
	}
}

func TestCheckUnifiedResourceUsesCanonicalGuestOverrideKey(t *testing.T) {
	m := newTestManager(t)
	resourceID := BuildGuestKey("pve1", "node1", 100)

	m.mu.Lock()
	m.config.Enabled = true
	m.config.TimeThresholds = map[string]int{}
	m.config.GuestDefaults = ThresholdConfig{
		CPU: &HysteresisThreshold{Trigger: 80, Clear: 75},
	}
	m.config.Overrides = map[string]ThresholdConfig{
		resourceID: {
			CPU: &HysteresisThreshold{Trigger: 60, Clear: 55},
		},
	}
	m.mu.Unlock()

	m.CheckUnifiedResource(&UnifiedResourceInput{
		ID:       resourceID,
		Type:     "vm",
		Name:     "app01",
		Node:     "node1",
		Instance: "pve1",
		CPU:      &UnifiedResourceMetric{Percent: 65},
	})

	exists := testHasActiveAlert(t, m, canonicalMetricStateID(resourceID, "cpu"))

	if !exists {
		t.Fatalf("expected canonical resource ID %q to be used for override lookup and alert IDs", resourceID)
	}
}

func TestCheckUnifiedResourceUsesStableClusteredGuestOverrideKey(t *testing.T) {
	m := newTestManager(t)
	resourceID := BuildGuestKey("pve1", "node2", 101)

	m.mu.Lock()
	m.config.Enabled = true
	m.config.TimeThresholds = map[string]int{}
	m.config.GuestDefaults = ThresholdConfig{
		CPU: &HysteresisThreshold{Trigger: 80, Clear: 75},
	}
	m.config.Overrides = map[string]ThresholdConfig{
		stableGuestOverrideKey("pve1", 101): {
			CPU: &HysteresisThreshold{Trigger: 60, Clear: 55},
		},
	}
	m.mu.Unlock()

	m.CheckUnifiedResource(&UnifiedResourceInput{
		ID:       resourceID,
		Type:     "vm",
		Name:     "app02",
		Node:     "node2",
		Instance: "pve1",
		CPU:      &UnifiedResourceMetric{Percent: 65},
	})

	exists := testHasActiveAlert(t, m, canonicalMetricStateID(resourceID, "cpu"))
	if !exists {
		t.Fatalf("expected stable clustered guest override to resolve for canonical resource ID %q", resourceID)
	}
}

func TestCheckNodeDisabledOverrideClearsExistingAlerts(t *testing.T) {
	m := newTestManager(t)
	node := models.Node{
		ID:       "node/pve-1",
		Name:     "pve-1",
		Instance: "pve1",
		Status:   "offline",
	}

	m.mu.Lock()
	m.config.Enabled = true
	m.config.Overrides = map[string]ThresholdConfig{
		node.ID: {Disabled: true},
	}
	cpuState, cpuAlert := testNewCanonicalAlert(node.ID, canonicalMetricSpecID(node.ID, "cpu"), string(alertspecs.AlertSpecKindMetricThreshold), "cpu")
	offlineState, offlineAlert := testNewCanonicalAlert(node.ID, canonicalConnectivitySpecID(node.ID), string(alertspecs.AlertSpecKindConnectivity), "offline")
	m.setActiveAlertNoLock(cpuState, cpuAlert)
	m.setActiveAlertNoLock(offlineState, offlineAlert)
	m.mu.Unlock()

	m.CheckNode(node)

	m.mu.RLock()
	_, cpuExists := m.activeAlerts[cpuState]
	_, offlineExists := m.activeAlerts[offlineState]
	countExists := testCoreHasIncident(m, node.ID, canonicalConnectivitySpecID(node.ID))
	m.mu.RUnlock()

	if cpuExists {
		t.Fatalf("expected CPU alert to be cleared for disabled node override")
	}
	if offlineExists {
		t.Fatalf("expected offline alert to be cleared for disabled node override")
	}
	if countExists {
		t.Fatalf("expected node offline tracking to be cleared for disabled node override")
	}
}

func testActiveAlertIDsOfType(m *Manager, alertType string) []string {
	var ids []string
	for _, alert := range m.GetActiveAlerts() {
		if alert.Type == alertType {
			ids = append(ids, alert.ID)
		}
	}
	return ids
}

func testNodeWithHostAgent() (models.Node, models.Host) {
	node := models.Node{
		ID:       "homelab-delly2",
		Name:     "delly2",
		Instance: "homelab",
		Status:   "online",
		CPU:      0.10,
		Memory:   models.Memory{Total: 100, Used: 40, Free: 60, Usage: 40},
		Disk:     models.Disk{Usage: 30},
	}
	host := models.Host{
		ID:           "agent-delly2",
		Hostname:     "delly2",
		LinkedNodeID: node.ID,
		CPUUsage:     10,
		Memory:       models.Memory{Total: 100, Used: 40, Free: 60, Usage: 40},
	}
	return node, host
}

// A host agent on a Proxmox node takes over CPU, memory and disk alerts, but it
// has no CPU temperature metric, so the node must keep raising temperature.
func TestCheckNodeKeepsTemperatureAlertWhenHostAgentMonitorsNode(t *testing.T) {
	m := newTestManager(t)
	m.mu.Lock()
	m.config.Enabled = true
	m.config.TimeThresholds = map[string]int{}
	m.config.NodeDefaults.Temperature = &HysteresisThreshold{Trigger: 80, Clear: 75}
	m.mu.Unlock()

	node, host := testNodeWithHostAgent()
	m.CheckHost(host)
	if !m.hasHostAgentForNode(node.ID) {
		t.Fatalf("expected CheckHost to link %q to node %q for deduplication", host.ID, node.ID)
	}

	node.Temperature = &models.Temperature{Available: true, CPUPackage: 90}
	m.CheckNode(node)
	m.CheckHost(host)

	tempAlertID := canonicalMetricStateID(node.ID, "temperature")
	if got := testActiveAlertIDsOfType(m, "temperature"); len(got) != 1 || got[0] != tempAlertID {
		t.Fatalf("expected exactly one temperature alert %q, got %v", tempAlertID, got)
	}

	node.Temperature = &models.Temperature{Available: true, CPUPackage: 60}
	m.CheckNode(node)
	if got := testActiveAlertIDsOfType(m, "temperature"); len(got) != 0 {
		t.Fatalf("expected node temperature alert to recover while the agent is registered, got %v", got)
	}
}

// A node alert that was open when the host agent registered must be handed
// over, not frozen until the agent goes offline.
func TestCheckNodeReleasesOpenMetricAlertWhenHostAgentRegisters(t *testing.T) {
	m := newTestManager(t)
	m.mu.Lock()
	m.config.Enabled = true
	m.config.TimeThresholds = map[string]int{}
	m.config.NodeDefaults.Memory = &HysteresisThreshold{Trigger: 85, Clear: 80}
	m.config.AgentDefaults.Memory = &HysteresisThreshold{Trigger: 85, Clear: 80}
	m.mu.Unlock()

	node, host := testNodeWithHostAgent()
	node.Memory = models.Memory{Total: 100, Used: 95, Free: 5, Usage: 95}
	host.Memory = node.Memory

	m.CheckNode(node)
	nodeAlertID := canonicalMetricStateID(node.ID, "memory")
	if !testHasActiveAlert(t, m, nodeAlertID) {
		t.Fatalf("expected node memory alert %q before the agent registers", nodeAlertID)
	}

	m.CheckHost(host)
	m.CheckNode(node)

	if testHasActiveAlert(t, m, nodeAlertID) {
		t.Fatalf("expected node memory alert %q to be released once the host agent owns memory", nodeAlertID)
	}
	m.mu.RLock()
	nodeIncident := testCoreHasIncident(m, node.ID, canonicalMetricSpecID(node.ID, "memory"))
	m.mu.RUnlock()
	if nodeIncident {
		t.Fatalf("expected no node memory incident left in the reducer core after handover")
	}
	agentAlertID := canonicalMetricStateID(hostResourceID(host.ID), "memory")
	if got := testActiveAlertIDsOfType(m, "memory"); len(got) != 1 || got[0] != agentAlertID {
		t.Fatalf("expected exactly one memory alert %q, got %v", agentAlertID, got)
	}
}

// Missing temperature telemetry is not a reading: it must not resolve an open
// node temperature alert, while a disabled threshold still clears it.
func TestCheckNodeMissingTemperatureDoesNotResolveOpenAlert(t *testing.T) {
	m := newTestManager(t)
	m.mu.Lock()
	m.config.Enabled = true
	m.config.TimeThresholds = map[string]int{}
	m.config.NodeDefaults.Temperature = &HysteresisThreshold{Trigger: 80, Clear: 75}
	m.mu.Unlock()

	node, _ := testNodeWithHostAgent()
	node.Temperature = &models.Temperature{Available: true, CPUPackage: 90}
	m.CheckNode(node)
	tempAlertID := canonicalMetricStateID(node.ID, "temperature")
	if !testHasActiveAlert(t, m, tempAlertID) {
		t.Fatalf("expected node temperature alert %q", tempAlertID)
	}

	for _, missing := range []*models.Temperature{nil, {Available: false}, {Available: true}} {
		node.Temperature = missing
		m.CheckNode(node)
		if !testHasActiveAlert(t, m, tempAlertID) {
			t.Fatalf("expected temperature alert to stay open without a reading (temperature %+v)", missing)
		}
	}

	m.mu.Lock()
	m.config.NodeDefaults.Temperature = &HysteresisThreshold{Trigger: 0, Clear: 0}
	m.mu.Unlock()
	m.CheckNode(node)
	if testHasActiveAlert(t, m, tempAlertID) {
		t.Fatalf("expected disabled temperature threshold to clear %q without a reading", tempAlertID)
	}
}

// Deduplication only hands the agent the usage metrics it evaluates. With agent
// alerts switched off, or one agent threshold off, the node keeps those alerts
// so the machine is never left unmonitored.
func TestCheckNodeKeepsUsageMetricsTheAgentDoesNotEvaluate(t *testing.T) {
	setup := func(t *testing.T) (*Manager, models.Node, models.Host) {
		m := newTestManager(t)
		m.mu.Lock()
		m.config.Enabled = true
		m.config.TimeThresholds = map[string]int{}
		m.config.NodeDefaults.CPU = &HysteresisThreshold{Trigger: 80, Clear: 75}
		m.config.NodeDefaults.Memory = &HysteresisThreshold{Trigger: 85, Clear: 80}
		m.config.AgentDefaults.CPU = &HysteresisThreshold{Trigger: 80, Clear: 75}
		m.config.AgentDefaults.Memory = &HysteresisThreshold{Trigger: 85, Clear: 80}
		m.mu.Unlock()
		node, host := testNodeWithHostAgent()
		node.CPU = 0.95
		node.Memory = models.Memory{Total: 100, Used: 95, Free: 5, Usage: 95}
		host.CPUUsage = 95
		host.Memory = node.Memory
		return m, node, host
	}
	nodeCPU := func(node models.Node) string { return canonicalMetricStateID(node.ID, "cpu") }
	nodeMemory := func(node models.Node) string { return canonicalMetricStateID(node.ID, "memory") }

	t.Run("agent_alerts_disabled", func(t *testing.T) {
		m, node, host := setup(t)
		m.CheckHost(host)
		m.CheckNode(node)
		if testHasActiveAlert(t, m, nodeCPU(node)) {
			t.Fatalf("expected the agent to own CPU while its alerts are enabled")
		}

		m.mu.Lock()
		m.config.DisableAllAgents = true
		m.mu.Unlock()
		m.CheckHost(host)
		m.CheckNode(node)
		if !testHasActiveAlert(t, m, nodeCPU(node)) || !testHasActiveAlert(t, m, nodeMemory(node)) {
			t.Fatalf("expected the node to alert on CPU and memory while agent alerts are disabled")
		}
	})

	t.Run("agent_cpu_threshold_off", func(t *testing.T) {
		m, node, host := setup(t)
		m.mu.Lock()
		m.config.AgentDefaults.CPU = &HysteresisThreshold{Trigger: 0, Clear: 0}
		m.mu.Unlock()
		m.CheckHost(host)
		m.CheckNode(node)
		if !testHasActiveAlert(t, m, nodeCPU(node)) {
			t.Fatalf("expected the node to keep CPU when the agent CPU threshold is off")
		}
		if testHasActiveAlert(t, m, nodeMemory(node)) {
			t.Fatalf("expected the agent to keep owning memory")
		}
		if got := testActiveAlertIDsOfType(m, "memory"); len(got) != 1 {
			t.Fatalf("expected exactly one memory alert for the machine, got %v", got)
		}
	})
}

// Ownership follows what the linked agents are configured to evaluate, not one
// report's data: a missing agent memory reading must not hand memory back to
// the node, a second linked agent's metrics count, and the node's disk metric
// belongs to the agent only when the agent evaluates that same filesystem.
func TestCheckNodeUsageOwnershipFollowsWhatAgentsEvaluate(t *testing.T) {
	setup := func(t *testing.T) (*Manager, models.Node, models.Host) {
		m := newTestManager(t)
		m.mu.Lock()
		m.config.Enabled = true
		m.config.TimeThresholds = map[string]int{}
		m.config.NodeDefaults.CPU = &HysteresisThreshold{Trigger: 80, Clear: 75}
		m.config.NodeDefaults.Memory = &HysteresisThreshold{Trigger: 85, Clear: 80}
		m.config.NodeDefaults.Disk = &HysteresisThreshold{Trigger: 90, Clear: 85}
		m.config.AgentDefaults.CPU = &HysteresisThreshold{Trigger: 80, Clear: 75}
		m.config.AgentDefaults.Memory = &HysteresisThreshold{Trigger: 85, Clear: 80}
		m.config.AgentDefaults.Disk = &HysteresisThreshold{Trigger: 90, Clear: 85}
		m.mu.Unlock()
		node, host := testNodeWithHostAgent()
		return m, node, host
	}

	t.Run("missing_agent_memory_reading", func(t *testing.T) {
		m, node, host := setup(t)
		node.Memory = models.Memory{Total: 100, Used: 95, Free: 5, Usage: 95}
		host.Memory = node.Memory
		m.CheckHost(host)
		m.CheckNode(node)

		host.Memory = models.Memory{Total: 100, UsageUnavailable: true}
		m.CheckHost(host)
		m.CheckNode(node)
		agentAlertID := canonicalMetricStateID(hostResourceID(host.ID), "memory")
		if got := testActiveAlertIDsOfType(m, "memory"); len(got) != 1 || got[0] != agentAlertID {
			t.Fatalf("expected only the agent memory alert while its reading is missing, got %v", got)
		}
	})

	t.Run("second_linked_agent", func(t *testing.T) {
		m, node, host := setup(t)
		node.CPU = 0.95
		m.mu.Lock()
		m.config.Overrides = map[string]ThresholdConfig{
			"agent-a": {CPU: &HysteresisThreshold{Trigger: 0, Clear: 0}},
		}
		m.mu.Unlock()
		agentA := host
		agentA.ID = "agent-a"
		agentB := host
		agentB.ID = "agent-b"
		agentB.CPUUsage = 95
		m.CheckHost(agentA)
		m.CheckHost(agentB)
		m.CheckNode(node)
		agentBAlertID := canonicalMetricStateID(hostResourceID(agentB.ID), "cpu")
		if got := testActiveAlertIDsOfType(m, "cpu"); len(got) != 1 || got[0] != agentBAlertID {
			t.Fatalf("expected only agent-b's CPU alert when it covers CPU for the node, got %v", got)
		}
	})

	t.Run("summary_disk", func(t *testing.T) {
		m, node, host := setup(t)
		root := models.Disk{Mountpoint: "/", Device: "/dev/sda1", Total: 100, Used: 95, Free: 5, Usage: 95}
		data := models.Disk{Mountpoint: "/data", Device: "/dev/sdb1", Total: 100, Used: 10, Free: 90, Usage: 10}
		host.Disks = []models.Disk{root, data}
		node.Disk = models.Disk{Total: 100, Used: 95, Free: 5, Usage: 95}
		rootResourceID, _ := hostDiskResourceID(host, root)
		nodeDiskAlertID := canonicalMetricStateID(node.ID, "disk")

		m.mu.Lock()
		m.config.Overrides = map[string]ThresholdConfig{rootResourceID: {Disabled: true}}
		m.mu.Unlock()
		m.CheckHost(host)
		m.CheckNode(node)
		if !testHasActiveAlert(t, m, nodeDiskAlertID) {
			t.Fatalf("expected the node to keep its root disk alert while the agent skips that filesystem")
		}

		m.mu.Lock()
		m.config.Overrides = nil
		m.mu.Unlock()
		m.CheckHost(host)
		m.CheckNode(node)
		if testHasActiveAlert(t, m, nodeDiskAlertID) {
			t.Fatalf("expected the node to release its disk alert once the agent evaluates the same filesystem")
		}
		if !testHasActiveAlert(t, m, canonicalMetricStateID(rootResourceID, "disk")) {
			t.Fatalf("expected the agent's root filesystem alert")
		}
	})
}

// A missing reading keeps the incident but must not let a sustained-for or
// recovery delay complete across the gap in evidence.
func TestCheckNodeMissingTemperatureInterruptsTimingRuns(t *testing.T) {
	m := newTestManager(t)
	m.mu.Lock()
	m.config.Enabled = true
	m.config.TimeThresholds = map[string]int{}
	m.config.MetricTimeThresholds = map[string]map[string]int{"node": {"temperature": 300}}
	m.config.NodeDefaults.Temperature = &HysteresisThreshold{Trigger: 80, Clear: 75}
	m.mu.Unlock()

	node, _ := testNodeWithHostAgent()
	tempAlertID := canonicalMetricStateID(node.ID, "temperature")
	specID := canonicalMetricSpecID(node.ID, "temperature")
	hot := &models.Temperature{Available: true, CPUPackage: 85}

	node.Temperature = hot
	m.CheckNode(node)
	m.mu.Lock()
	pending := testCoreIsPending(m, node.ID, specID)
	m.core.ShiftPending(-10 * time.Minute)
	m.mu.Unlock()
	if !pending {
		t.Fatalf("expected a pending temperature run before the reading went missing")
	}

	node.Temperature = nil
	m.CheckNode(node)
	node.Temperature = hot
	m.CheckNode(node)
	if testHasActiveAlert(t, m, tempAlertID) {
		t.Fatalf("expected the sustained-for delay to restart after a missing reading, not fire on the first sample back")
	}

	m.mu.Lock()
	m.core.ShiftPending(-10 * time.Minute)
	m.mu.Unlock()
	m.CheckNode(node)
	if !testHasActiveAlert(t, m, tempAlertID) {
		t.Fatalf("expected the temperature alert to fire after a sustained run")
	}

	node.Temperature = &models.Temperature{Available: true, CPUPackage: 60}
	m.CheckNode(node)
	m.mu.RLock()
	incident, _ := m.core.Incident(node.ID, specID)
	m.mu.RUnlock()
	if incident.RecoverySince.IsZero() {
		t.Fatalf("expected a recovery run to start below the clear threshold")
	}

	node.Temperature = nil
	m.CheckNode(node)
	m.mu.RLock()
	incident, _ = m.core.Incident(node.ID, specID)
	m.mu.RUnlock()
	if !incident.RecoverySince.IsZero() {
		t.Fatalf("expected a missing reading to restart the recovery run")
	}
	if !testHasActiveAlert(t, m, tempAlertID) {
		t.Fatalf("expected the temperature alert to stay open without a reading")
	}
}

// Node alerts keep the PVE instance name in Instance. A config save must judge
// them against node thresholds, not fall through to guest thresholds, which
// have no temperature threshold and used to resolve a live temperature alert.
func TestConfigSaveKeepsNodeTemperatureAlertOverTrigger(t *testing.T) {
	m := newTestManager(t)
	m.mu.Lock()
	m.config.Enabled = true
	m.config.TimeThresholds = map[string]int{}
	m.config.NodeDefaults.Temperature = &HysteresisThreshold{Trigger: 80, Clear: 75}
	m.mu.Unlock()

	node, _ := testNodeWithHostAgent()
	node.Temperature = &models.Temperature{Available: true, CPUPackage: 90}
	m.CheckNode(node)
	tempAlertID := canonicalMetricStateID(node.ID, "temperature")
	if !testHasActiveAlert(t, m, tempAlertID) {
		t.Fatalf("expected node temperature alert %q", tempAlertID)
	}

	m.UpdateConfig(m.GetConfig())
	if !testHasActiveAlert(t, m, tempAlertID) {
		t.Fatalf("expected a config save to keep a node temperature alert still over its trigger")
	}

	config := m.GetConfig()
	config.Overrides = map[string]ThresholdConfig{
		node.ID: {Temperature: &HysteresisThreshold{Trigger: 95, Clear: 92}},
	}
	m.UpdateConfig(config)
	if testHasActiveAlert(t, m, tempAlertID) {
		t.Fatalf("expected a node override raising the trigger above the reading to resolve the alert")
	}
}

func TestReevaluateActiveAlertsUsesSharedAgentOverrideResolution(t *testing.T) {
	m := newTestManager(t)

	m.mu.Lock()
	m.config.Enabled = true
	m.config.AgentDefaults = ThresholdConfig{
		CPU: &HysteresisThreshold{Trigger: 80, Clear: 75},
	}
	m.config.Overrides = map[string]ThresholdConfig{
		"host1": {
			Disabled: true,
		},
	}
	m.activeAlerts["agent:host1-cpu"] = &Alert{
		ID:        "agent:host1-cpu",
		Type:      "cpu",
		Value:     95,
		Threshold: 80,
		Metadata: map[string]interface{}{
			"resourceType": "Agent",
		},
	}
	m.reevaluateActiveAlertsLocked()
	m.mu.Unlock()

	m.mu.RLock()
	_, exists := m.activeAlerts["agent:host1-cpu"]
	m.mu.RUnlock()

	if exists {
		t.Fatalf("expected reevaluation to resolve agent alert using raw host override key")
	}
}

func TestReevaluateActiveAlertsUsesSharedStorageOverrideResolution(t *testing.T) {
	m := newTestManager(t)

	m.mu.Lock()
	m.config.Enabled = true
	m.config.StorageDefault = HysteresisThreshold{Trigger: 85, Clear: 80}
	m.config.Overrides = map[string]ThresholdConfig{
		"storage1": {
			Disabled: true,
		},
	}
	m.activeAlerts["storage1-usage"] = &Alert{
		ID:        "storage1-usage",
		Type:      "usage",
		Value:     95,
		Threshold: 85,
		Instance:  "Storage",
	}
	m.reevaluateActiveAlertsLocked()
	m.mu.Unlock()

	m.mu.RLock()
	_, exists := m.activeAlerts["storage1-usage"]
	m.mu.RUnlock()

	if exists {
		t.Fatalf("expected reevaluation to resolve storage alert when shared override disables storage alerting")
	}
}

func TestReevaluateActiveAlertsUsesStorageMetadataForCephPoolAlias(t *testing.T) {
	m := newTestManager(t)
	resourceID := "pve5-ceph-pool-data_replication"

	m.mu.Lock()
	m.config.Enabled = true
	m.config.StorageDefault = HysteresisThreshold{Trigger: 95, Clear: 90}
	m.config.Overrides = map[string]ThresholdConfig{
		"agent:pve5-ceph-pool-data_replication": {
			Usage: &HysteresisThreshold{Trigger: 50, Clear: 45},
		},
	}
	state, alert := testNewCanonicalAlert(resourceID, canonicalMetricSpecID(resourceID, "usage"), string(alertspecs.AlertSpecKindMetricThreshold), "usage")
	alert.Value = 60
	alert.Threshold = 50
	alert.ResourceName = "data_replication"
	alert.Node = "pve5"
	alert.Instance = "pve5"
	alert.Metadata = map[string]interface{}{
		"resourceType": "Storage",
	}
	m.setActiveAlertNoLock(state, alert)
	m.reevaluateActiveAlertsLocked()
	m.mu.Unlock()

	m.mu.RLock()
	_, exists := m.activeAlerts[state]
	m.mu.RUnlock()

	if !exists {
		t.Fatalf("expected Ceph pool storage alert to remain active under agent-prefixed alias override")
	}
}

func TestStorageThresholdResolutionUsesAliasIDs(t *testing.T) {
	m := newTestManager(t)

	m.mu.Lock()
	m.config.StorageDefault = HysteresisThreshold{Trigger: 95, Clear: 90}
	m.config.Overrides = map[string]ThresholdConfig{
		"agent:pve5-ceph-pool-data_replication": {
			Usage: &HysteresisThreshold{Trigger: 50, Clear: 45},
		},
	}
	thresholds := m.resolveStorageThresholdsNoLock(models.Storage{
		ID:       "pve5-ceph-pool-data_replication",
		AliasIDs: []string{"agent:pve5-ceph-pool-data_replication"},
	})
	m.mu.Unlock()

	if thresholds.Usage == nil {
		t.Fatalf("expected usage threshold to be resolved")
	}
	if thresholds.Usage.Trigger != 50 || thresholds.Usage.Clear != 45 {
		t.Fatalf("usage threshold = %#v, want alias override trigger 50 clear 45", thresholds.Usage)
	}
}

func TestCheckStorageOfflineUsesSharedThresholdResolution(t *testing.T) {
	m := newTestManager(t)
	storage := models.Storage{
		ID:     "storage1",
		Name:   "tank",
		Status: "offline",
	}

	m.mu.Lock()
	m.config.Overrides = map[string]ThresholdConfig{
		storage.ID: {
			DisableConnectivity: true,
		},
	}
	m.activeAlerts["storage-offline-"+storage.ID] = &Alert{ID: "storage-offline-" + storage.ID}
	m.mu.Unlock()

	m.checkStorageOffline(storage)

	m.mu.RLock()
	_, alertExists := m.activeAlerts["storage-offline-"+storage.ID]
	confirmExists := testCoreHasIncident(m, storage.ID, canonicalConnectivitySpecID(storage.ID))
	m.mu.RUnlock()

	if alertExists {
		t.Fatalf("expected storage offline alert to clear when shared thresholds disable connectivity")
	}
	if confirmExists {
		t.Fatalf("expected storage offline confirmations to clear when shared thresholds disable connectivity")
	}
}

func TestCheckSnapshotsUsesGuestContextForCustomRules(t *testing.T) {
	m := newTestManager(t)
	m.ClearActiveAlerts()

	cfg := AlertConfig{
		Enabled: true,
		SnapshotDefaults: SnapshotAlertConfig{
			Enabled:      true,
			WarningDays:  7,
			CriticalDays: 14,
		},
		CustomRules: []CustomAlertRule{
			{
				Name:     "db-snapshots",
				Enabled:  true,
				Priority: 10,
				FilterConditions: FilterStack{
					LogicalOperator: "AND",
					Filters: []FilterCondition{
						{Type: "text", Field: "name", Value: "db"},
					},
				},
				Thresholds: ThresholdConfig{
					Snapshot: &SnapshotAlertConfig{
						Enabled:      true,
						WarningDays:  15,
						CriticalDays: 20,
					},
				},
			},
		},
	}
	m.UpdateConfig(cfg)
	m.mu.Lock()
	m.config.TimeThresholds = map[string]int{}
	m.mu.Unlock()

	now := time.Now()
	snapshots := []models.GuestSnapshot{
		{
			ID:       "inst-node-100-weekly",
			Name:     "weekly",
			Node:     "node",
			Instance: "inst",
			Type:     "qemu",
			VMID:     100,
			Time:     now.Add(-10 * 24 * time.Hour),
		},
		{
			ID:       "inst-node-101-weekly",
			Name:     "weekly",
			Node:     "node",
			Instance: "inst",
			Type:     "qemu",
			VMID:     101,
			Time:     now.Add(-10 * 24 * time.Hour),
		},
	}
	guestLookups := map[string]GuestLookup{
		BuildGuestKey("inst", "node", 100): {Name: "db-server"},
		BuildGuestKey("inst", "node", 101): {Name: "web-server"},
	}

	m.CheckSnapshotsForInstance("inst", snapshots, guestLookups)

	m.mu.RLock()
	_, dbExists := testLookupActiveAlert(t, m, "snapshot-age-inst-node-100-weekly")
	_, webExists := testLookupActiveAlert(t, m, "snapshot-age-inst-node-101-weekly")
	m.mu.RUnlock()

	if dbExists {
		t.Fatalf("expected db snapshot alert to be suppressed by custom rule thresholds")
	}
	if !webExists {
		t.Fatalf("expected non-matching snapshot alert to use default thresholds")
	}
}

func TestCheckBackupsUsesGuestContextForCustomRules(t *testing.T) {
	m := newTestManager(t)
	m.ClearActiveAlerts()

	cfg := AlertConfig{
		Enabled: true,
		BackupDefaults: BackupAlertConfig{
			Enabled:      true,
			WarningDays:  7,
			CriticalDays: 14,
		},
		CustomRules: []CustomAlertRule{
			{
				Name:     "db-backups",
				Enabled:  true,
				Priority: 10,
				FilterConditions: FilterStack{
					LogicalOperator: "AND",
					Filters: []FilterCondition{
						{Type: "text", Field: "name", Value: "db"},
					},
				},
				Thresholds: ThresholdConfig{
					Backup: &BackupAlertConfig{
						Enabled:      true,
						WarningDays:  15,
						CriticalDays: 20,
					},
				},
			},
		},
	}
	m.UpdateConfig(cfg)

	now := time.Now()
	rollups := []recovery.ProtectionRollup{
		{
			RollupID: "db-rollup",
			SubjectRef: &recovery.ExternalRef{
				Type:      "proxmox-vm",
				Namespace: "inst",
				Name:      "db-server",
				ID:        BuildGuestKey("inst", "node", 100),
				Class:     "node",
			},
			LastSuccessAt: ptrTime(now.Add(-10 * 24 * time.Hour)),
			LastOutcome:   recovery.OutcomeSuccess,
			Providers:     []recovery.Provider{recovery.ProviderProxmoxPVE},
		},
		{
			RollupID: "web-rollup",
			SubjectRef: &recovery.ExternalRef{
				Type:      "proxmox-vm",
				Namespace: "inst",
				Name:      "web-server",
				ID:        BuildGuestKey("inst", "node", 101),
				Class:     "node",
			},
			LastSuccessAt: ptrTime(now.Add(-10 * 24 * time.Hour)),
			LastOutcome:   recovery.OutcomeSuccess,
			Providers:     []recovery.Provider{recovery.ProviderProxmoxPVE},
		},
	}

	guest100 := GuestLookup{
		ResourceID: BuildGuestKey("inst", "node", 100),
		Name:       "db-server",
		Instance:   "inst",
		Node:       "node",
		Type:       "qemu",
		VMID:       100,
	}
	guest101 := GuestLookup{
		ResourceID: BuildGuestKey("inst", "node", 101),
		Name:       "web-server",
		Instance:   "inst",
		Node:       "node",
		Type:       "qemu",
		VMID:       101,
	}
	guestsByKey := map[string]GuestLookup{
		guest100.ResourceID: guest100,
		guest101.ResourceID: guest101,
	}
	guestsByVMID := map[string][]GuestLookup{
		"100": {guest100},
		"101": {guest101},
	}

	m.CheckBackups(rollups, guestsByKey, guestsByVMID)

	m.mu.RLock()
	_, dbExists := testLookupActiveAlert(t, m, "backup-age-"+sanitizeAlertKey(guest100.ResourceID))
	_, webExists := testLookupActiveAlert(t, m, "backup-age-"+sanitizeAlertKey(guest101.ResourceID))
	m.mu.RUnlock()

	if dbExists {
		t.Fatalf("expected db backup alert to be suppressed by custom rule thresholds")
	}
	if !webExists {
		t.Fatalf("expected non-matching backup alert to use default thresholds")
	}
}

func TestReevaluateActiveAlertsUsesGuestContextForMetricCustomRules(t *testing.T) {
	m := newTestManager(t)
	resourceID := BuildGuestKey("pve1", "node1", 100)

	m.mu.Lock()
	m.config.Enabled = true
	m.config.GuestDefaults = ThresholdConfig{
		CPU: &HysteresisThreshold{Trigger: 80, Clear: 75},
	}
	m.config.CustomRules = []CustomAlertRule{
		{
			Name:     "db-metrics",
			Enabled:  true,
			Priority: 10,
			FilterConditions: FilterStack{
				LogicalOperator: "AND",
				Filters: []FilterCondition{
					{Type: "text", Field: "name", Value: "db"},
				},
			},
			Thresholds: ThresholdConfig{
				CPU: &HysteresisThreshold{Trigger: 95, Clear: 90},
			},
		},
	}
	state, alert := testNewCanonicalAlert(resourceID, canonicalMetricSpecID(resourceID, "cpu"), string(alertspecs.AlertSpecKindMetricThreshold), "cpu")
	alert.Value = 90
	alert.Threshold = 80
	alert.ResourceName = "db-server"
	alert.Node = "node1"
	alert.Instance = "pve1"
	alert.Metadata = map[string]interface{}{
		"resourceType": "vm",
	}
	m.setActiveAlertNoLock(state, alert)
	m.reevaluateActiveAlertsLocked()
	m.mu.Unlock()

	m.mu.RLock()
	_, exists := m.activeAlerts[state]
	m.mu.RUnlock()

	if exists {
		t.Fatalf("expected guest metric alert to resolve when custom rule raises the trigger")
	}
}

func TestReevaluateActiveAlertsUsesGuestContextForBackupCustomRules(t *testing.T) {
	m := newTestManager(t)
	resourceID := BuildGuestKey("pve1", "node1", 100)

	m.mu.Lock()
	m.config.Enabled = true
	m.config.BackupDefaults = BackupAlertConfig{
		Enabled:      true,
		WarningDays:  7,
		CriticalDays: 14,
	}
	m.config.CustomRules = []CustomAlertRule{
		{
			Name:     "db-backup-reeval",
			Enabled:  true,
			Priority: 10,
			FilterConditions: FilterStack{
				LogicalOperator: "AND",
				Filters: []FilterCondition{
					{Type: "text", Field: "name", Value: "db"},
				},
			},
			Thresholds: ThresholdConfig{
				Backup: &BackupAlertConfig{
					Enabled:      true,
					WarningDays:  15,
					CriticalDays: 20,
				},
			},
		},
	}
	state, alert := testNewCanonicalAlert(resourceID, resourceID+"-backup-age", string(alertspecs.AlertSpecKindPostureThreshold), "backup-age")
	alert.Value = 10
	alert.Threshold = 7
	alert.ResourceName = "db-server backup"
	alert.Node = "node1"
	alert.Instance = "pve1"
	alert.Metadata = map[string]interface{}{
		"ageDays":       10.0,
		"guestName":     "db-server",
		"guestType":     "qemu",
		"guestInstance": "pve1",
		"guestNode":     "node1",
		"guestVmid":     100,
		"orphaned":      false,
	}
	m.setActiveAlertNoLock(state, alert)
	m.reevaluateActiveAlertsLocked()
	m.mu.Unlock()

	m.mu.RLock()
	_, exists := m.activeAlerts[state]
	m.mu.RUnlock()

	if exists {
		t.Fatalf("expected guest backup alert to resolve when custom rule raises backup thresholds")
	}
}

func TestReevaluateActiveAlertsUsesGuestContextForPoweredOffCustomRules(t *testing.T) {
	m := newTestManager(t)
	resourceID := BuildGuestKey("pve1", "node1", 100)

	m.mu.Lock()
	m.config.Enabled = true
	m.config.CustomRules = []CustomAlertRule{
		{
			Name:     "db-no-powered-off",
			Enabled:  true,
			Priority: 10,
			FilterConditions: FilterStack{
				LogicalOperator: "AND",
				Filters: []FilterCondition{
					{Type: "text", Field: "name", Value: "db"},
				},
			},
			Thresholds: ThresholdConfig{
				DisableConnectivity: true,
			},
		},
	}
	state, alert := testNewCanonicalAlert(resourceID, canonicalPoweredStateSpecID(resourceID), string(alertspecs.AlertSpecKindPoweredState), "powered-off")
	alert.ResourceName = "db-server"
	alert.Node = "node1"
	alert.Instance = "pve1"
	alert.Metadata = map[string]interface{}{
		"resourceType": "vm",
	}
	m.setActiveAlertNoLock(state, alert)
	m.reevaluateActiveAlertsLocked()
	m.mu.Unlock()

	m.mu.RLock()
	_, exists := m.activeAlerts[state]
	m.mu.RUnlock()

	if exists {
		t.Fatalf("expected guest powered-off alert to resolve when custom rule disables connectivity")
	}
}

func TestReevaluateActiveAlertsUsesPBSResourceTypeMetadata(t *testing.T) {
	m := newTestManager(t)

	m.mu.Lock()
	m.config.Enabled = true
	m.config.PBSDefaults = ThresholdConfig{
		CPU:    &HysteresisThreshold{Trigger: 99, Clear: 94},
		Memory: &HysteresisThreshold{Trigger: 0, Clear: 0},
	}
	state, alert := testNewCanonicalAlert("pbs-1", canonicalMetricSpecID("pbs-1", "memory"), string(alertspecs.AlertSpecKindMetricThreshold), "memory")
	alert.Value = 90.8
	alert.Threshold = 85
	alert.ResourceName = "pbs"
	alert.Node = "pbs.local"
	alert.Instance = "pbs"
	alert.Metadata = map[string]interface{}{
		"resourceType": "PBS",
	}
	m.setActiveAlertNoLock(state, alert)
	m.reevaluateActiveAlertsLocked()
	m.mu.Unlock()

	m.mu.RLock()
	_, exists := m.activeAlerts[state]
	m.mu.RUnlock()

	if exists {
		t.Fatalf("expected PBS metric alert to resolve when PBS memory threshold is disabled")
	}
}

func TestReevaluateActiveAlertsUsesKubernetesResourceTypeMetadata(t *testing.T) {
	m := newTestManager(t)

	resourceID := "k8s:prod/ns:default/pod:api-7d9f"
	m.mu.Lock()
	m.config.Enabled = true
	m.config.KubernetesDefaults = ThresholdConfig{
		CPU: &HysteresisThreshold{Trigger: 0, Clear: 0},
	}
	state, alert := testNewCanonicalAlert(resourceID, canonicalMetricSpecID(resourceID, "cpu"), string(alertspecs.AlertSpecKindMetricThreshold), "cpu")
	alert.Value = 88
	alert.Threshold = 80
	alert.ResourceName = "api-7d9f"
	alert.Node = "worker-1"
	alert.Instance = "prod"
	alert.Metadata = map[string]interface{}{
		"resourceType": "Kubernetes Pod",
	}
	m.setActiveAlertNoLock(state, alert)
	m.reevaluateActiveAlertsLocked()
	m.mu.Unlock()

	m.mu.RLock()
	_, exists := m.activeAlerts[state]
	m.mu.RUnlock()

	if exists {
		t.Fatalf("expected Kubernetes metric alert to resolve when Kubernetes CPU threshold is disabled")
	}
}

func TestReevaluateActiveAlertsUsesTrueNASResourceTypeMetadata(t *testing.T) {
	m := newTestManager(t)

	resourceID := "storage:truenas-main/pool:tank"
	m.mu.Lock()
	m.config.Enabled = true
	m.config.StorageDefault = HysteresisThreshold{Trigger: 95, Clear: 90}
	m.config.TrueNASDefaults = ThresholdConfig{
		Usage: &HysteresisThreshold{Trigger: 0, Clear: 0},
	}
	state, alert := testNewCanonicalAlert(resourceID, canonicalMetricSpecID(resourceID, "usage"), string(alertspecs.AlertSpecKindMetricThreshold), "usage")
	alert.Value = 90
	alert.Threshold = 85
	alert.ResourceName = "tank"
	alert.Node = "truenas-main"
	alert.Instance = "TrueNAS"
	alert.Metadata = map[string]interface{}{
		"resourceType": "TrueNAS Pool",
	}
	m.setActiveAlertNoLock(state, alert)
	m.reevaluateActiveAlertsLocked()
	m.mu.Unlock()

	m.mu.RLock()
	_, exists := m.activeAlerts[state]
	m.mu.RUnlock()

	if exists {
		t.Fatalf("expected TrueNAS metric alert to resolve when TrueNAS usage threshold is disabled")
	}
}

func TestReevaluateActiveAlertsUsesVMwareResourceTypeMetadata(t *testing.T) {
	m := newTestManager(t)

	resourceID := "vmware:vc-1:datastore:datastore-301"
	m.mu.Lock()
	m.config.Enabled = true
	m.config.StorageDefault = HysteresisThreshold{Trigger: 95, Clear: 90}
	m.config.VMwareDefaults = ThresholdConfig{
		Usage: &HysteresisThreshold{Trigger: 0, Clear: 0},
	}
	state, alert := testNewCanonicalAlert(resourceID, canonicalMetricSpecID(resourceID, "usage"), string(alertspecs.AlertSpecKindMetricThreshold), "usage")
	alert.Value = 90
	alert.Threshold = 85
	alert.ResourceName = "nvme-primary"
	alert.Node = "Lab Datacenter"
	alert.Instance = "Lab vCenter"
	alert.Metadata = map[string]interface{}{
		"resourceType": "vSphere Datastore",
	}
	m.setActiveAlertNoLock(state, alert)
	m.reevaluateActiveAlertsLocked()
	m.mu.Unlock()

	m.mu.RLock()
	_, exists := m.activeAlerts[state]
	m.mu.RUnlock()

	if exists {
		t.Fatalf("expected vSphere metric alert to resolve when vSphere usage threshold is disabled")
	}
}

func TestGuestFilesystemDisableOverrideExcludesMountAndAggregate(t *testing.T) {
	m := newTestManager(t)
	guestID := BuildGuestKey("pve1", "node1", 101)

	m.mu.Lock()
	m.config.TimeThresholds = map[string]int{}
	m.config.GuestDefaults = ThresholdConfig{
		Disk: &HysteresisThreshold{Trigger: 80, Clear: 75},
	}
	m.config.Overrides = map[string]ThresholdConfig{
		"guest-disk:pve1:node1:101/disk:boot-dev-vda1": {Disabled: true},
	}
	m.mu.Unlock()

	m.CheckGuest(models.VM{
		ID:       guestID,
		VMID:     101,
		Name:     "flatcar",
		Node:     "node1",
		Instance: "pve1",
		Status:   "running",
		Disk:     models.Disk{Usage: 97},
		Disks: []models.Disk{
			{Mountpoint: "/boot", Device: "/dev/vda1", Usage: 97, Total: 100, Used: 97, Free: 3},
			{Mountpoint: "/", Device: "/dev/vda2", Usage: 50, Total: 100, Used: 50, Free: 50},
		},
	}, "pve1")

	bootAlertID := canonicalMetricStateID(guestID+"-disk-boot-dev-vda1", "disk")
	rootAlertID := canonicalMetricStateID(guestID+"-disk-dev-vda2", "disk")
	aggregateAlertID := canonicalMetricStateID(guestID, "disk")

	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, exists := testLookupActiveAlert(t, m, bootAlertID); exists {
		t.Fatalf("disabled /boot filesystem created alert %q", bootAlertID)
	}
	if _, exists := testLookupActiveAlert(t, m, rootAlertID); exists {
		t.Fatalf("healthy root filesystem created alert %q", rootAlertID)
	}
	if _, exists := testLookupActiveAlert(t, m, aggregateAlertID); exists {
		t.Fatalf("disabled /boot filesystem leaked into aggregate alert %q", aggregateAlertID)
	}
}

func TestGuestFilesystemThresholdOverrideOwnsMountEvaluation(t *testing.T) {
	m := newTestManager(t)
	guestID := BuildGuestKey("pve1", "node1", 101)

	m.mu.Lock()
	m.config.TimeThresholds = map[string]int{}
	m.config.GuestDefaults = ThresholdConfig{
		Disk: &HysteresisThreshold{Trigger: 80, Clear: 75},
	}
	m.config.Overrides = map[string]ThresholdConfig{
		"guest-disk:guest:pve1:101/disk:boot-dev-vda1": {
			Disk: &HysteresisThreshold{Trigger: 99, Clear: 95},
		},
	}
	aggregateAlertID := canonicalMetricStateID(guestID, "disk")
	m.activeAlerts[aggregateAlertID] = &Alert{
		ID:         aggregateAlertID,
		ResourceID: guestID,
		Type:       "disk",
		Value:      97,
	}
	m.mu.Unlock()

	vm := models.VM{
		ID:       guestID,
		VMID:     101,
		Name:     "flatcar",
		Node:     "node1",
		Instance: "pve1",
		Status:   "running",
		Disks: []models.Disk{
			{Mountpoint: "/boot", Device: "/dev/vda1", Usage: 97, Total: 100, Used: 97, Free: 3},
		},
	}
	m.CheckGuest(vm, "pve1")

	bootAlertID := canonicalMetricStateID(guestID+"-disk-boot-dev-vda1", "disk")
	m.mu.RLock()
	_, existsAt97 := testLookupActiveAlert(t, m, bootAlertID)
	_, aggregateExists := testLookupActiveAlert(t, m, aggregateAlertID)
	m.mu.RUnlock()
	if existsAt97 {
		t.Fatalf("/boot alert fired below its 99%% override")
	}
	if aggregateExists {
		t.Fatalf("dedicated filesystem override left stale aggregate alert %q", aggregateAlertID)
	}

	vm.Disks[0].Usage = 100
	vm.Disks[0].Used = 100
	vm.Disks[0].Free = 0
	m.CheckGuest(vm, "pve1")

	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, exists := testLookupActiveAlert(t, m, bootAlertID); !exists {
		t.Fatalf("/boot alert did not fire above its 99%% override")
	}
}

func TestGuestFilesystemEvidenceSuppressesDuplicateAggregateAlert(t *testing.T) {
	m := newTestManager(t)
	guestID := BuildGuestKey("pve1", "node1", 109)
	m.mu.Lock()
	m.config.TimeThresholds = map[string]int{}
	m.config.GuestDefaults = ThresholdConfig{Disk: &HysteresisThreshold{Trigger: 80, Clear: 75}}
	m.mu.Unlock()

	m.CheckGuest(models.VM{
		ID: guestID, VMID: 109, Name: "pulse-dev", Node: "node1", Instance: "pve1", Status: "running",
		Disk:  models.Disk{Usage: 96.5},
		Disks: []models.Disk{{Mountpoint: "/", Device: "/dev/vda1", Usage: 96.9, Total: 100, Used: 97, Free: 3}},
	}, "pve1")

	aggregateID := canonicalMetricStateID(guestID, "disk")
	filesystemID := canonicalMetricStateID(guestID+"-disk-dev-vda1", "disk")
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, exists := testLookupActiveAlert(t, m, aggregateID); exists {
		t.Fatalf("aggregate alert %q duplicated filesystem evidence", aggregateID)
	}
	if _, exists := testLookupActiveAlert(t, m, filesystemID); !exists {
		t.Fatalf("actionable filesystem alert %q was not created", filesystemID)
	}
}

// Regression: checkMetric stores canonical-identity alerts under the
// canonical state key, so hysteresis resolution must not remove only the
// unregistered legacy "<resourceID>-<metric>" ID.
func TestCheckMetricResolveRemovesCanonicallyKeyedAlert(t *testing.T) {
	manager := NewManagerWithDataDir(t.TempDir(), WithoutPersistedAlertRestore())
	t.Cleanup(manager.Stop)

	manager.mu.Lock()
	manager.config.Enabled = true
	manager.config.TimeThresholds = map[string]int{}
	manager.mu.Unlock()

	threshold := &HysteresisThreshold{Trigger: 80, Clear: 75}
	resourceID := "guest-123/disk:root"

	manager.checkMetric(resourceID, "test-vm", "node1", "qemu/123", "VM", "disk", 85, threshold, nil)

	manager.mu.Lock()
	activeAfterFire := len(manager.activeAlerts)
	manager.mu.Unlock()
	if activeAfterFire != 1 {
		t.Fatalf("active alerts after fire = %d, want 1", activeAfterFire)
	}

	manager.checkMetric(resourceID, "test-vm", "node1", "qemu/123", "VM", "disk", 70, threshold, nil)

	manager.mu.Lock()
	activeAfterResolve := len(manager.activeAlerts)
	manager.mu.Unlock()
	if activeAfterResolve != 0 {
		t.Fatalf("active alerts after resolve = %d, want 0 (stale alert left behind)", activeAfterResolve)
	}

	if resolved := manager.GetResolvedAlert(buildCanonicalStateID(resourceID, "metric-threshold:disk")); resolved == nil {
		t.Fatal("expected a recently-resolved entry for the cleared alert")
	}
}
