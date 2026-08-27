package alerts

// Characterization for the Phase 3 policy resolution
// (docs/ALERT_ENGINE_EVOLUTION.md): effectiveAlertPolicyNoLock must answer
// exactly what the scattered legacy reads answered — the per-type
// resolveResourceThresholds / getGuestThresholds paths and the DisableAll*
// booleans — across the config surface. Any divergence is a bug in the
// fold, never an improvement.

import (
	"reflect"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func testPolicyConfigMatrix() AlertConfig {
	trig := func(t, c float64) *HysteresisThreshold {
		return &HysteresisThreshold{Trigger: t, Clear: c}
	}
	return AlertConfig{
		Enabled:        true,
		GuestDefaults:  ThresholdConfig{CPU: trig(80, 75), Memory: trig(85, 80)},
		NodeDefaults:   ThresholdConfig{CPU: trig(85, 80), Temperature: trig(75, 70)},
		AgentDefaults:  ThresholdConfig{CPU: trig(90, 85), DiskTemperature: trig(60, 55)},
		PBSDefaults:    ThresholdConfig{CPU: trig(70, 65)},
		StorageDefault: HysteresisThreshold{Trigger: 88, Clear: 83},
		KubernetesDefaults: ThresholdConfig{
			CPU: trig(75, 70), Memory: trig(80, 75),
		},
		TrueNASDefaults:     ThresholdConfig{CPU: trig(65, 60), Usage: trig(82, 77)},
		TrueNASDiskDefaults: ThresholdConfig{DiskTemperature: trig(55, 50)},
		VMwareDefaults:      ThresholdConfig{CPU: trig(78, 73), Usage: trig(84, 79)},
		Overrides: map[string]ThresholdConfig{
			"node-2":       {CPU: trig(95, 90)},
			"pbs-quiet":    {Disabled: true},
			"storage-full": {Usage: trig(97, 94)},
			"storage-alias": {
				Usage: trig(93, 91),
			},
			"vm-override": {Memory: trig(99, 97), DisableConnectivity: true},
		},
		CustomRules: []CustomAlertRule{
			{
				Name:     "named-guests",
				Enabled:  true,
				Priority: 10,
				FilterConditions: FilterStack{
					LogicalOperator: "AND",
					Filters: []FilterCondition{
						{Type: "text", Field: "name", Value: "web"},
					},
				},
				Thresholds: ThresholdConfig{CPU: trig(60, 55)},
			},
			{
				Name:     "disabled-rule",
				Enabled:  false,
				Priority: 99,
				FilterConditions: FilterStack{
					LogicalOperator: "AND",
					Filters: []FilterCondition{
						{Type: "text", Field: "name", Value: "web"},
					},
				},
				Thresholds: ThresholdConfig{CPU: trig(10, 5)},
			},
		},
		DisableAllNodes:              true,
		DisableAllGuestsOffline:      true,
		DisableAllPBS:                true,
		DisableAllPBSOffline:         true,
		DisableAllDockerHostsOffline: true,
		DisableAllTrueNAS:            true,
	}
}

func TestEffectiveAlertPolicyMatchesLegacyThresholdResolution(t *testing.T) {
	m := newTestManager(t)
	m.mu.Lock()
	m.config = testPolicyConfigMatrix()
	m.mu.Unlock()

	cases := []struct {
		name  string
		query alertPolicyQuery
	}{
		{"node default", alertPolicyQuery{TypeKey: "node", ResourceID: "node-1"}},
		{"node override", alertPolicyQuery{TypeKey: "node", ResourceID: "node-2"}},
		{"agent", alertPolicyQuery{TypeKey: "agent", ResourceID: "agent:host-1"}},
		{"pbs disabled override", alertPolicyQuery{TypeKey: "pbs", ResourceID: "pbs-quiet"}},
		{"storage default", alertPolicyQuery{TypeKey: "storage", ResourceID: "storage-empty"}},
		{"storage override", alertPolicyQuery{TypeKey: "storage", ResourceID: "storage-full"}},
		{"k8s node", alertPolicyQuery{TypeKey: "k8s-node", ResourceID: "k8s-node-1"}},
		{"truenas pool", alertPolicyQuery{TypeKey: "truenas-pool", ResourceID: "pool-1"}},
		{"vmware datastore", alertPolicyQuery{TypeKey: "vmware-datastore", ResourceID: "ds-1"}},
		{"vm without live guest", alertPolicyQuery{TypeKey: "vm", ResourceID: "vm-override"}},
		{"unknown type", alertPolicyQuery{TypeKey: "mystery", ResourceID: "x"}},
	}

	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			legacy := m.resolveResourceThresholds(tc.query.TypeKey, tc.query.ResourceID)
			policy := m.effectiveAlertPolicyNoLock(tc.query)
			if !reflect.DeepEqual(policy.Thresholds, legacy) {
				t.Fatalf("thresholds diverge from legacy resolution:\n policy: %+v\n legacy: %+v", policy.Thresholds, legacy)
			}
		})
	}
}

func TestEffectiveAlertPolicyMatchesGuestResolutionWithCustomRules(t *testing.T) {
	m := newTestManager(t)
	m.mu.Lock()
	m.config = testPolicyConfigMatrix()
	m.mu.Unlock()

	guests := []struct {
		name  string
		guest models.VM
		id    string
	}{
		{"custom rule matches", models.VM{Name: "web-1", Node: "pve1", Instance: "pve1", VMID: 100}, "pve1-100"},
		{"custom rule misses", models.VM{Name: "db-1", Node: "pve1", Instance: "pve1", VMID: 101}, "pve1-101"},
		{"override wins over rule", models.VM{Name: "web-2", Node: "pve1", Instance: "pve1", VMID: 102}, "vm-override"},
	}

	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, tc := range guests {
		t.Run(tc.name, func(t *testing.T) {
			legacy := m.getGuestThresholds(tc.guest, tc.id)
			policy := m.effectiveAlertPolicyNoLock(alertPolicyQuery{TypeKey: "vm", ResourceID: tc.id, Guest: tc.guest})
			if !reflect.DeepEqual(policy.Thresholds, legacy) {
				t.Fatalf("guest thresholds diverge from legacy resolution:\n policy: %+v\n legacy: %+v", policy.Thresholds, legacy)
			}
		})
	}
}

func TestEffectiveAlertPolicyMatchesStorageAliasResolution(t *testing.T) {
	m := newTestManager(t)
	m.mu.Lock()
	m.config = testPolicyConfigMatrix()
	m.mu.Unlock()

	m.mu.RLock()
	defer m.mu.RUnlock()
	legacy := m.resolveStorageThresholdOverride(m.defaultThresholdsForResourceType("storage"), "storage-unknown", []string{"storage-alias"})
	policy := m.effectiveAlertPolicyNoLock(alertPolicyQuery{TypeKey: "storage", ResourceID: "storage-unknown", StorageAliases: []string{"storage-alias"}})
	if !reflect.DeepEqual(policy.Thresholds, legacy) {
		t.Fatalf("alias storage thresholds diverge:\n policy: %+v\n legacy: %+v", policy.Thresholds, legacy)
	}
}

func TestEffectiveAlertPolicySwitchesMatchLegacyBooleans(t *testing.T) {
	m := newTestManager(t)
	m.mu.Lock()
	m.config = testPolicyConfigMatrix()
	m.mu.Unlock()

	m.mu.RLock()
	defer m.mu.RUnlock()
	cases := []struct {
		typeKey     string
		wantAll     bool
		wantOffline bool
	}{
		{"node", m.config.DisableAllNodes, m.config.DisableAllNodesOffline},
		{"vm", m.config.DisableAllGuests, m.config.DisableAllGuestsOffline},
		{"system-container", m.config.DisableAllGuests, m.config.DisableAllGuestsOffline},
		{"app-container", m.config.DisableAllDockerContainers, false},
		{"docker-host", m.config.DisableAllDockerHosts, m.config.DisableAllDockerHostsOffline},
		{"docker-service", m.config.DisableAllDockerServices, false},
		{"agent", m.config.DisableAllAgents, m.config.DisableAllAgentsOffline},
		{"storage", m.config.DisableAllStorage, false},
		{"pbs", m.config.DisableAllPBS, m.config.DisableAllPBSOffline},
		{"pmg", m.config.DisableAllPMG, m.config.DisableAllPMGOffline},
		{"k8s-node", m.config.DisableAllKubernetes, false},
		{"truenas-pool", m.config.DisableAllTrueNAS, false},
		{"vmware-vm", m.config.DisableAllVMware, false},
		{"mystery", false, false},
	}
	for _, tc := range cases {
		policy := m.effectiveAlertPolicyNoLock(alertPolicyQuery{TypeKey: tc.typeKey, ResourceID: "r"})
		if policy.AllDisabled != tc.wantAll || policy.OfflineDisabled != tc.wantOffline {
			t.Errorf("%s: switches = (all=%v offline=%v), want (all=%v offline=%v)",
				tc.typeKey, policy.AllDisabled, policy.OfflineDisabled, tc.wantAll, tc.wantOffline)
		}
	}
}
