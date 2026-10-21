package alerts

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// The missing observation is a rule input, not the metric being alerted on.
// Falling through to the default policy would invent a recovery even though
// the current memory observation and original occurrence have not changed.
func TestGuestRuleUnknownFilterPreservesOtherMetricIncident(t *testing.T) {
	for _, kind := range []string{"vm", "lxc"} {
		t.Run(kind, func(t *testing.T) {
			m := guestObservationManager(t)
			cfg := m.GetConfig()
			cfg.GuestDefaults = ThresholdConfig{Memory: &HysteresisThreshold{Trigger: -1}}
			cfg.CustomRules = []CustomAlertRule{{
				Enabled: true, Priority: 10,
				FilterConditions: FilterStack{LogicalOperator: "AND", Filters: []FilterCondition{
					{Type: "metric", Field: "cpu", Operator: ">", Value: 80.0},
				}},
				Thresholds: ThresholdConfig{Memory: &HysteresisThreshold{Trigger: 85, Clear: 80}},
			}}
			m.UpdateConfig(cfg)
			vm := guestObservationVM()
			vm.CPU, vm.Disks = .98, nil
			check := func() {
				if kind == "vm" {
					m.CheckGuest(vm, vm.Instance)
				} else {
					m.CheckGuest(models.Container{ID: vm.ID, VMID: vm.VMID, Name: vm.Name, Node: vm.Node,
						Instance: vm.Instance, Status: vm.Status, CPU: vm.CPU, Memory: vm.Memory, Disk: vm.Disk,
						IORateValidity: vm.IORateValidity}, vm.Instance)
				}
			}
			check()
			active := m.GetActiveAlerts()
			if len(active) != 1 || active[0].Type != "memory" {
				t.Fatalf("known rule must open the memory incident: %+v", active)
			}
			if err := m.AcknowledgeAlert(active[0].ID, "operator"); err != nil {
				t.Fatal(err)
			}
			before := guestObservationBytes(t, m)
			vm.CPU = -1
			for poll := 0; poll < 3; poll++ {
				check()
			}
			if after := guestObservationBytes(t, m); !bytes.Equal(before, after) || len(m.GetRecentlyResolved()) != 0 {
				t.Fatalf("unknown rule input changed the acknowledged incident: before=%s after=%s", before, after)
			}
		})
	}
}

func TestGuestRuleFilterThreeValuedLogic(t *testing.T) {
	m := guestObservationManager(t)
	vm := guestObservationVM()
	vm.CPU = -1
	vm.Memory.Observation.State = "last-known"
	vm.DiskStatusReason, vm.Disk.Usage = "agent-error", -1
	for _, field := range []string{"cpu", "memory", "disk", "diskRead", "diskWrite", "networkIn", "networkOut"} {
		for _, op := range []string{">", "<", ">=", "<=", "=", "=="} {
			t.Run(field+op, func(t *testing.T) {
				unknown := FilterCondition{Type: "metric", Field: field, Operator: op, Value: 0.0}
				match := FilterCondition{Type: "text", Field: "name", Value: "data"}
				miss := FilterCondition{Type: "text", Field: "name", Value: "another-guest"}
				for _, test := range []struct {
					op      string
					filters []FilterCondition
					want    guestFilterResult
				}{
					{"AND", []FilterCondition{unknown}, guestFilterUnknown},
					{"AND", []FilterCondition{unknown, match}, guestFilterUnknown},
					{"AND", []FilterCondition{miss, unknown}, guestFilterNoMatch},
					{"AND", []FilterCondition{unknown, miss}, guestFilterNoMatch},
					{"OR", []FilterCondition{unknown, match}, guestFilterMatch},
					{"OR", []FilterCondition{match, unknown}, guestFilterMatch},
					{"OR", []FilterCondition{unknown, miss}, guestFilterUnknown},
				} {
					stack := FilterStack{LogicalOperator: test.op, Filters: test.filters}
					if got := m.evaluateFilterStackResult(vm, stack); got != test.want {
						t.Fatalf("%+v: got %v, want %v", stack, got, test.want)
					}
					if m.evaluateFilterStack(vm, stack) != (test.want == guestFilterMatch) {
						t.Fatal("boolean consumer accepted an unknown filter")
					}
				}
			})
		}
	}
	for _, filter := range []FilterCondition{
		{Type: "metric", Field: "typo", Operator: ">", Value: 0.0},
		{Type: "metric", Field: "cpu", Operator: "!=", Value: 0.0},
		{Type: "metric", Field: "cpu", Operator: ">", Value: "not a number"},
	} {
		if got := m.evaluateFilterStackResult(vm, FilterStack{Filters: []FilterCondition{filter}}); got != guestFilterNoMatch {
			t.Fatalf("invalid rule became an uncertain policy: %+v", filter)
		}
	}
	// Measured zero remains an ordinary numeric rule input.
	vm.CPU = 0
	if !m.evaluateFilterStack(vm, FilterStack{Filters: []FilterCondition{{Type: "metric", Field: "cpu", Operator: "==", Value: 0.0}}}) {
		t.Fatal("measured idle CPU became unknown")
	}
}

func TestGuestRulePolicyPriorityAndFinalOverrides(t *testing.T) {
	for _, test := range []struct {
		name                                          string
		unknownPriority                               int
		unknownFirst, disabledRule, override, relaxed bool
		wantKnown                                     bool
	}{
		{name: "higher priority unknown", unknownPriority: 11},
		{name: "lower priority unknown", unknownPriority: 9, wantKnown: true},
		{name: "earlier tied unknown", unknownPriority: 10, unknownFirst: true},
		{name: "later tied unknown", unknownPriority: 10, wantKnown: true},
		{name: "disabled rule", unknownPriority: 11, disabledRule: true, wantKnown: true},
		{name: "negative priority retains historical exclusion", unknownPriority: -1, wantKnown: true},
		{name: "guest override eliminates difference", unknownPriority: 11, override: true, wantKnown: true},
		{name: "relaxed floor eliminates difference", unknownPriority: 11, relaxed: true, wantKnown: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			m := guestObservationManager(t)
			cfg := m.GetConfig()
			known := CustomAlertRule{Enabled: true, Priority: 10, FilterConditions: FilterStack{Filters: []FilterCondition{{Type: "text", Field: "name", Value: "database"}}}, Thresholds: ThresholdConfig{Memory: &HysteresisThreshold{Trigger: 85, Clear: 80}}}
			unknown := CustomAlertRule{Enabled: !test.disabledRule, Priority: test.unknownPriority, FilterConditions: FilterStack{Filters: []FilterCondition{{Type: "metric", Field: "cpu", Operator: ">", Value: 80.0}}}, Thresholds: ThresholdConfig{Memory: &HysteresisThreshold{Trigger: 60, Clear: 55}}}
			cfg.CustomRules = []CustomAlertRule{known, unknown}
			if test.unknownFirst {
				cfg.CustomRules = []CustomAlertRule{unknown, known}
			}
			vm := guestObservationVM()
			vm.CPU = -1
			if test.override {
				cfg.Overrides[vm.ID] = ThresholdConfig{Memory: &HysteresisThreshold{Trigger: 75, Clear: 70}}
			}
			if test.relaxed {
				vm.Tags = []string{"pulse-relaxed"}
			}
			m.UpdateConfig(cfg)
			m.mu.RLock()
			defer m.mu.RUnlock()
			policy := m.guestThresholdPolicyNoLock(vm, vm.ID)
			if got := m.guestSignalPolicyKnownNoLock(policy, "memory"); got != test.wantKnown {
				t.Fatalf("memory policy known = %v, want %v: %+v", got, test.wantKnown, policy)
			}
			for _, signal := range []string{"disk", "diskWrite", "networkIn", "powered-off", "backup-age", "snapshot-age"} {
				if !m.guestSignalPolicyKnownNoLock(policy, signal) {
					t.Fatalf("memory rule fenced independent %s policy", signal)
				}
			}
		})
	}
}

func TestGuestRuleUnknownCannotFireAndKeepsIndependentMetrics(t *testing.T) {
	for _, disabled := range []bool{false, true} {
		t.Run(fmt.Sprint("possible-disabled-", disabled), func(t *testing.T) {
			m := guestObservationManager(t)
			cfg := m.GetConfig()
			cfg.GuestDefaults = ThresholdConfig{Memory: &HysteresisThreshold{Trigger: 85, Clear: 80}, DiskWrite: &HysteresisThreshold{Trigger: 5, Clear: 4}}
			cfg.CustomRules = []CustomAlertRule{{Enabled: true, Priority: 10,
				FilterConditions: FilterStack{Filters: []FilterCondition{{Type: "metric", Field: "cpu", Operator: ">", Value: 80.0}}},
				Thresholds:       ThresholdConfig{Disabled: disabled, Memory: &HysteresisThreshold{Trigger: -1}},
			}}
			m.UpdateConfig(cfg)
			vm := guestObservationVM()
			vm.CPU, vm.DiskWrite, vm.IORateValidity.DiskWrite = -1, 10*1024*1024, true
			for i := 0; i < 3; i++ {
				m.CheckGuest(vm, "site")
			}
			active := m.GetActiveAlerts()
			if disabled {
				if len(active) != 0 {
					t.Fatal("possible full disablement still fired an incident")
				}
			} else if len(active) != 1 || active[0].Type != "diskWrite" {
				t.Fatalf("uncertain memory fired or independent I/O was blocked: %+v", active)
			}
		})
	}
}

func TestGuestRuleUnknownInterruptsPendingAndRecovery(t *testing.T) {
	m := guestObservationManager(t)
	cfg := m.GetConfig()
	cfg.GuestDefaults = ThresholdConfig{Memory: &HysteresisThreshold{Trigger: -1}}
	cfg.CustomRules = []CustomAlertRule{{Enabled: true, Priority: 10,
		FilterConditions: FilterStack{Filters: []FilterCondition{{Type: "metric", Field: "cpu", Operator: ">", Value: 80.0}}},
		Thresholds:       ThresholdConfig{Memory: &HysteresisThreshold{Trigger: 85, Clear: 80}},
	}}
	cfg.MetricTimeThresholds = map[string]map[string]int{"guest": {"memory": 60}}
	m.UpdateConfig(cfg)
	vm := guestObservationVM()
	vm.CPU = .98
	m.CheckGuest(vm, "site")
	m.mu.Lock()
	m.core.ShiftPending(-2 * time.Minute)
	m.mu.Unlock()
	vm.CPU = -1
	m.CheckGuest(vm, "site")
	vm.CPU = .98
	m.CheckGuest(vm, "site")
	if len(m.GetActiveAlerts()) != 0 {
		t.Fatal("missing rule input completed an activation delay")
	}
	m.mu.Lock()
	m.core.ShiftPending(-2 * time.Minute)
	m.mu.Unlock()
	m.CheckGuest(vm, "site")
	if len(m.GetActiveAlerts()) != 1 {
		t.Fatal("known continuous rule did not complete activation")
	}
	vm.Memory.Usage = 10
	m.CheckGuest(vm, "site") // First current recovery confirmation.
	before := guestObservationBytes(t, m)
	vm.CPU = -1
	for i := 0; i < 3; i++ {
		m.CheckGuest(vm, "site")
	}
	if !bytes.Equal(before, guestObservationBytes(t, m)) {
		t.Fatal("unknown rule input completed recovery")
	}
	vm.CPU = .98
	m.CheckGuest(vm, "site")
	if len(m.GetActiveAlerts()) != 1 {
		t.Fatal("recovery confirmation spanned a policy gap")
	}
	m.CheckGuest(vm, "site")
	if len(m.GetActiveAlerts()) != 0 || len(m.GetRecentlyResolved()) != 1 {
		t.Fatal("known current recovery did not resolve once")
	}
}

func TestGuestRuleConfigSaveAndFilesystemOverride(t *testing.T) {
	m := guestObservationManager(t)
	cfg := m.GetConfig()
	cfg.GuestDefaults = ThresholdConfig{Memory: &HysteresisThreshold{Trigger: -1}, Disk: &HysteresisThreshold{Trigger: -1}}
	cfg.CustomRules = []CustomAlertRule{{Enabled: true, Priority: 10,
		FilterConditions: FilterStack{Filters: []FilterCondition{{Type: "metric", Field: "cpu", Operator: ">", Value: 80.0}}},
		Thresholds:       ThresholdConfig{Memory: &HysteresisThreshold{Trigger: 85, Clear: 80}, Disk: &HysteresisThreshold{Trigger: 85, Clear: 80}},
	}}
	m.UpdateConfig(cfg)
	vm := guestObservationVM()
	vm.CPU = .98
	m.CheckGuest(vm, "site")
	if len(m.GetActiveAlerts()) != 3 {
		t.Fatal("known custom policy did not open memory and both filesystems")
	}
	for _, alert := range m.GetActiveAlerts() {
		if err := m.AcknowledgeAlert(alert.ID, "operator"); err != nil {
			t.Fatal(err)
		}
	}
	before := guestObservationBytes(t, m)
	m.UpdateConfig(m.GetConfig())
	vm.CPU = -1
	m.ReevaluateGuestAlert(vm, vm.ID)
	m.CheckGuest(vm, "site")
	if !bytes.Equal(before, guestObservationBytes(t, m)) {
		t.Fatal("save or unknown reevaluation changed an original incident")
	}
	_, diskKey, _ := guestDiskIdentity(vm.Disks[0], 0)
	cfg = m.GetConfig()
	cfg.Overrides[guestDiskOverrideKey(vm, vm.ID, diskKey)] = ThresholdConfig{Disabled: true}
	m.UpdateConfig(cfg)
	m.CheckGuest(vm, "site")
	if len(m.GetActiveAlerts()) != 2 {
		t.Fatal("explicit filesystem disablement was lost or crossed identities")
	}
	cfg = m.GetConfig()
	cfg.Overrides[vm.ID] = ThresholdConfig{Disabled: true}
	m.UpdateConfig(cfg)
	m.CheckGuest(vm, "site")
	if len(m.GetActiveAlerts()) != 0 {
		t.Fatal("explicit guest disablement was fenced by an unknown rule")
	}
}

func TestGuestRuleEveryUnavailableInputPreservesCrossMetricAlert(t *testing.T) {
	for _, field := range []string{"cpu", "memory", "disk", "diskRead", "diskWrite", "networkIn", "networkOut"} {
		for _, kind := range []string{"vm", "lxc"} {
			t.Run(kind+"/"+field, func(t *testing.T) {
				m := guestObservationManager(t)
				cfg := m.GetConfig()
				cfg.GuestDefaults = ThresholdConfig{}
				threshold := &HysteresisThreshold{Trigger: 85, Clear: 80}
				target := ThresholdConfig{CPU: threshold}
				if field == "cpu" {
					target = ThresholdConfig{Memory: threshold}
				}
				cfg.CustomRules = []CustomAlertRule{{Enabled: true, Priority: 1, FilterConditions: FilterStack{Filters: []FilterCondition{{Type: "metric", Field: field, Operator: ">", Value: 1.0}}}, Thresholds: target}}
				m.UpdateConfig(cfg)
				vm := guestObservationVM()
				vm.CPU, vm.Disks = .98, nil
				vm.DiskRead, vm.DiskWrite, vm.NetworkIn, vm.NetworkOut = 10*1024*1024, 10*1024*1024, 10*1024*1024, 10*1024*1024
				vm.IORateValidity = models.IORateValidity{Explicit: true, DiskRead: true, DiskWrite: true, NetworkIn: true, NetworkOut: true}
				check := func() { m.CheckGuest(statusObservationGuest(vm, kind), "site") }
				check()
				if len(m.GetActiveAlerts()) != 1 {
					t.Fatal("known cross-metric rule failed")
				}
				for _, alert := range m.GetActiveAlerts() {
					if err := m.AcknowledgeAlert(alert.ID, "operator"); err != nil {
						t.Fatal(err)
					}
				}
				before := guestObservationBytes(t, m)
				switch field {
				case "cpu":
					vm.CPU = -1
				case "memory":
					vm.Memory.Observation.State = "last-known"
				case "disk":
					vm.Disk.Usage = -1
				case "diskRead":
					vm.IORateValidity.DiskRead = false
				case "diskWrite":
					vm.IORateValidity.DiskWrite = false
				case "networkIn":
					vm.IORateValidity.NetworkIn = false
				case "networkOut":
					vm.IORateValidity.NetworkOut = false
				}
				for i := 0; i < 3; i++ {
					check()
				}
				if !bytes.Equal(before, guestObservationBytes(t, m)) || len(m.GetRecentlyResolved()) != 0 {
					t.Fatal("unavailable filter input changed another metric's occurrence")
				}
			})
		}
	}
}
