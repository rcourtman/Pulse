package alerts

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"reflect"
	"sync"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// Keep the pre-projection evaluator independent of the new stack helper: it
// extracts each condition's guest separately and folds the collected results.
func legacyGuestRuleStack(m *Manager, guest any, stack FilterStack) bool {
	if len(stack.Filters) == 0 {
		return true
	}
	results := make([]bool, len(stack.Filters))
	for i, filter := range stack.Filters {
		results[i] = m.evaluateFilterCondition(guest, filter)
	}
	if stack.LogicalOperator == "AND" {
		for _, result := range results {
			if !result {
				return false
			}
		}
		return true
	}
	for _, result := range results {
		if result {
			return true
		}
	}
	return false
}

func legacyGuestRuleThresholds(m *Manager, base ThresholdConfig, guest any) ThresholdConfig {
	if guest == nil {
		return base
	}
	var applicable *CustomAlertRule
	highestPriority := -1
	for i := range m.config.CustomRules {
		rule := &m.config.CustomRules[i]
		if rule.Enabled && legacyGuestRuleStack(m, guest, rule.FilterConditions) && rule.Priority > highestPriority {
			applicable, highestPriority = rule, rule.Priority
		}
	}
	if applicable == nil {
		return base
	}
	return m.applyThresholdOverride(base, applicable.Thresholds)
}

func guestRuleVM(disks int) models.VM {
	vm := models.VM{
		ID: "site:node:101", VMID: 101, Instance: "site", Node: "node", Name: "web-DB", Status: "running", CPU: .75,
		Memory: models.Memory{Usage: 85}, Disk: models.Disk{Usage: 70}, Tags: []string{"production", "database"},
		DiskRead: 20 << 20, DiskWrite: 10 << 20, NetworkIn: 5 << 20, NetworkOut: 2 << 20,
	}
	for i := 0; i < disks; i++ {
		vm.Disks = append(vm.Disks, models.Disk{Mountpoint: fmt.Sprintf("/volume/%d", i), Usage: 70})
	}
	return vm
}

func guestRuleContainer(vm models.VM) models.Container {
	return models.Container{
		ID: vm.ID, VMID: vm.VMID, Instance: vm.Instance, Node: vm.Node, Name: vm.Name, Status: vm.Status, CPU: vm.CPU,
		Memory: vm.Memory, Disk: vm.Disk, Disks: vm.Disks, Tags: vm.Tags, IORateValidity: vm.IORateValidity,
		DiskRead: vm.DiskRead, DiskWrite: vm.DiskWrite, NetworkIn: vm.NetworkIn, NetworkOut: vm.NetworkOut,
	}
}

func guestRuleInputs() []struct {
	name  string
	guest any
} {
	vm := guestRuleVM(8)
	ct := guestRuleContainer(vm)
	snapshot := guestSnapshotFromVM(vm)
	stale := vm
	stale.Memory.Observation.State, stale.DiskStatusReason = "last-known", "prev-vm-locked"
	unavailable := vm
	unavailable.Memory.UsageUnavailable, unavailable.Disk.Usage = true, -1
	invalidRates := vm
	invalidRates.IORateValidity = models.IORateValidity{Explicit: true}
	validZero := vm
	validZero.DiskRead, validZero.DiskWrite, validZero.NetworkIn, validZero.NetworkOut = 0, 0, 0, 0
	validZero.IORateValidity = models.IORateValidity{Explicit: true, DiskRead: true, DiskWrite: true, NetworkIn: true, NetworkOut: true}
	relaxed := vm
	relaxed.Tags = append([]string{"pulse-relaxed"}, vm.Tags...)
	return []struct {
		name  string
		guest any
	}{
		{"vm", vm}, {"vm-pointer", &vm}, {"lxc", ct}, {"lxc-pointer", &ct},
		{"snapshot", snapshot}, {"snapshot-pointer", &snapshot},
		{"retained", stale}, {"unavailable", unavailable}, {"invalid-rates", invalidRates}, {"valid-zero", validZero},
		{"relaxed", relaxed}, {"nil", nil}, {"nil-vm", (*models.VM)(nil)}, {"nil-lxc", (*models.Container)(nil)},
		{"nil-snapshot", (*guestSnapshot)(nil)}, {"unsupported", struct{}{}},
	}
}

func guestRuleConditions() []FilterCondition {
	return []FilterCondition{
		{Type: "metric", Field: "CPU", Operator: ">", Value: json.Number("60")},
		{Type: "metric", Field: "memory", Operator: ">=", Value: float32(85)},
		{Type: "metric", Field: "disk", Operator: "<", Value: 80},
		{Type: "metric", Field: "diskRead", Operator: "==", Value: int64(20)},
		{Type: "metric", Field: "diskWrite", Operator: "<=", Value: uint(10)},
		{Type: "metric", Field: "networkIn", Operator: "=", Value: 5.0},
		{Type: "metric", Field: "networkOut", Operator: ">=", Value: 0.0},
		{Type: "metric", Field: "memory", Operator: "<", Value: 90.0},
		{Type: "metric", Field: "disk", Operator: ">", Value: -2.0},
		{Type: "metric", Field: "diskRead", Operator: "==", Value: 0.0},
		{Type: "metric", Field: "cpu", Operator: ">", Value: "60"},
		{Type: "metric", Field: "unknown", Operator: ">", Value: 0.0},
		{Type: "metric", Field: "cpu", Operator: "!=", Value: 0.0},
		{Type: "text", Field: "NAME", Value: "DB"}, {Type: "text", Field: "node", Value: "NODE"},
		{Type: "text", Field: "vmid", Value: "101"}, {Type: "text", Field: "unknown", Value: "web"},
		{Type: "raw", RawText: "RUNNING"}, {Type: "raw", RawText: "SITE"}, {Type: "raw"}, {Type: "unknown"},
	}
}

func TestGuestRuleStackMatchesLegacy(t *testing.T) {
	m := &Manager{}
	conditions := guestRuleConditions()
	for _, input := range guestRuleInputs() {
		t.Run(input.name, func(t *testing.T) {
			for _, operator := range []string{"AND", "OR", "and", ""} {
				stacks := []FilterStack{{LogicalOperator: operator}, {LogicalOperator: operator, Filters: conditions}}
				for i, condition := range conditions {
					stacks = append(stacks, FilterStack{LogicalOperator: operator, Filters: []FilterCondition{condition}},
						FilterStack{LogicalOperator: operator, Filters: []FilterCondition{condition, conditions[(i+1)%len(conditions)]}})
				}
				for i, stack := range stacks {
					want := legacyGuestRuleStack(m, input.guest, stack)
					if got := m.evaluateFilterStack(input.guest, stack); got != want {
						t.Fatalf("operator %q stack %d = %v, legacy %v", operator, i, got, want)
					}
				}
			}
		})
	}
}

type guestRuleCountedText struct{ calls *int }

func (v guestRuleCountedText) String() string {
	(*v.calls)++
	return "web"
}

func TestGuestRuleStackStillEvaluatesEveryCondition(t *testing.T) {
	for _, operator := range []string{"AND", "OR", ""} {
		t.Run(operator, func(t *testing.T) {
			calls := 0
			firstValue := 0.0 // OR is already true; AND is already false.
			if operator == "AND" {
				firstValue = 100
			}
			stack := FilterStack{LogicalOperator: operator, Filters: []FilterCondition{
				{Type: "metric", Field: "cpu", Operator: ">", Value: firstValue},
				{Type: "text", Field: "name", Value: guestRuleCountedText{calls: &calls}},
			}}
			m := &Manager{config: AlertConfig{CustomRules: []CustomAlertRule{{Enabled: true, Priority: 1, FilterConditions: stack}}}}
			m.customRuleThresholdsNoLock(ThresholdConfig{}, guestRuleVM(1))
			if calls != 1 {
				t.Fatalf("evaluated later text condition %d times, want once", calls)
			}
		})
	}
}

func TestGuestRulePolicyMatchesLegacy(t *testing.T) {
	rng := rand.New(rand.NewSource(4101))
	conditions := guestRuleConditions()
	for _, input := range guestRuleInputs() {
		t.Run(input.name, func(t *testing.T) {
			for trial := 0; trial < 64; trial++ {
				m := &Manager{config: AlertConfig{GuestDefaults: ThresholdConfig{CPU: &HysteresisThreshold{Trigger: 80, Clear: 75}},
					Overrides: map[string]ThresholdConfig{"guest:site:101": {Memory: &HysteresisThreshold{Trigger: 90, Clear: 85}}}}}
				for i := 0; i < trial%12; i++ {
					rule := CustomAlertRule{Enabled: rng.Intn(4) != 0, Priority: rng.Intn(7) - 3,
						Thresholds: ThresholdConfig{CPU: &HysteresisThreshold{Trigger: float64(40 + i), Clear: float64(35 + i)}}}
					rule.FilterConditions.LogicalOperator = []string{"AND", "OR", "", "and"}[rng.Intn(4)]
					for f := 0; f < i%5; f++ {
						rule.FilterConditions.Filters = append(rule.FilterConditions.Filters, conditions[rng.Intn(len(conditions))])
					}
					m.config.CustomRules = append(m.config.CustomRules, rule)
				}
				before, _ := json.Marshal(m.config)
				want := legacyGuestRuleThresholds(m, m.defaultThresholdsForResourceType("vm"), input.guest)
				want = m.resolveGuestThresholdOverride(want, input.guest, "site:node:101")
				if guest, ok := extractGuestSnapshot(input.guest); ok && parsePulseTags(guest.Tags).Relaxed {
					want = applyRelaxedGuestThresholds(want)
				}
				got := m.getGuestThresholds(input.guest, "site:node:101")
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("trial %d: thresholds differ from legacy\ngot %+v\nwant %+v", trial, got, want)
				}
				after, _ := json.Marshal(m.config)
				if string(before) != string(after) {
					t.Fatalf("trial %d changed stored configuration", trial)
				}
			}
		})
	}
}

func TestGuestRulePolicyPriorityAndEmptyInputs(t *testing.T) {
	rule := func(priority int, enabled bool, trigger float64, filters ...FilterCondition) CustomAlertRule {
		return CustomAlertRule{Enabled: enabled, Priority: priority, FilterConditions: FilterStack{LogicalOperator: "AND", Filters: filters},
			Thresholds: ThresholdConfig{CPU: &HysteresisThreshold{Trigger: trigger, Clear: trigger - 5}}}
	}
	match := FilterCondition{Type: "text", Field: "name", Value: "web"}
	miss := FilterCondition{Type: "text", Field: "name", Value: "absent"}
	for _, tc := range []struct {
		name  string
		guest any
		rules []CustomAlertRule
		want  float64
	}{
		{"nil-skips-empty-rule", nil, []CustomAlertRule{rule(1, true, 50)}, 80},
		{"typed-nil-empty-rule", (*models.VM)(nil), []CustomAlertRule{rule(1, true, 50)}, 50},
		{"unsupported-empty-rule", struct{}{}, []CustomAlertRule{rule(1, true, 50)}, 50},
		{"typed-nil-filtered-rule", (*models.Container)(nil), []CustomAlertRule{rule(1, true, 50, match)}, 80},
		{"first-equal-priority", guestRuleVM(1), []CustomAlertRule{rule(3, true, 50, match), rule(3, true, 60, match)}, 50},
		{"negative-priority", guestRuleVM(1), []CustomAlertRule{rule(-1, true, 50, match), rule(-2, true, 60)}, 80},
		{"zero-priority", guestRuleVM(1), []CustomAlertRule{rule(0, true, 50, match)}, 50},
		{"highest-matching", guestRuleVM(1), []CustomAlertRule{rule(3, true, 50, match), rule(4, true, 60, match)}, 60},
		{"disabled-and-missing", guestRuleVM(1), []CustomAlertRule{rule(3, true, 50, match), rule(4, false, 60), rule(5, true, 70, miss)}, 50},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := &Manager{config: AlertConfig{CustomRules: tc.rules}}
			base := ThresholdConfig{CPU: &HysteresisThreshold{Trigger: 80, Clear: 75}}
			got := m.customRuleThresholdsNoLock(base, tc.guest)
			if got.CPU == nil || got.CPU.Trigger != tc.want {
				t.Fatalf("CPU = %+v, want %v", got.CPU, tc.want)
			}
		})
	}
}

func TestGuestRulePolicyDoesNotCacheLiveReadings(t *testing.T) {
	vm := guestRuleVM(1)
	m := &Manager{config: AlertConfig{CustomRules: []CustomAlertRule{{Enabled: true, Priority: 1,
		FilterConditions: FilterStack{LogicalOperator: "AND", Filters: []FilterCondition{{Type: "metric", Field: "memory", Operator: ">", Value: 80.0}}},
		Thresholds:       ThresholdConfig{CPU: &HysteresisThreshold{Trigger: 60, Clear: 55}}}}}}
	base := ThresholdConfig{CPU: &HysteresisThreshold{Trigger: 90, Clear: 85}}
	for _, state := range []struct {
		usage   float64
		fresh   string
		enabled bool
		want    float64
	}{{85, "current", true, 60}, {85, "last-known", true, 90}, {85, "current", true, 60}, {10, "current", true, 90}, {85, "current", false, 90}} {
		vm.Memory.Usage, vm.Memory.Observation.State = state.usage, state.fresh
		m.config.CustomRules[0].Enabled = state.enabled
		if got := m.customRuleThresholdsNoLock(base, &vm); got.CPU.Trigger != state.want {
			t.Fatalf("state %+v: CPU trigger %v", state, got.CPU.Trigger)
		}
	}
}

func TestGuestRulePolicyConcurrentReaders(t *testing.T) {
	m := &Manager{config: AlertConfig{GuestDefaults: ThresholdConfig{CPU: &HysteresisThreshold{Trigger: 90, Clear: 85}},
		CustomRules: []CustomAlertRule{{Enabled: true, Priority: 1,
			FilterConditions: FilterStack{LogicalOperator: "AND", Filters: []FilterCondition{{Type: "metric", Field: "cpu", Operator: ">", Value: 50.0}}},
			Thresholds:       ThresholdConfig{CPU: &HysteresisThreshold{Trigger: 80, Clear: 75}}}}}}
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			vm := guestRuleVM(2)
			if i == 0 {
				vm.CPU = .1
			}
			for j := 0; j < 100; j++ {
				m.mu.RLock()
				got := m.getGuestThresholds(&vm, vm.ID)
				want := 90.0
				if m.config.CustomRules[0].Enabled && i != 0 {
					want = 80
				}
				if got.CPU.Trigger != want {
					t.Errorf("CPU trigger %v", got.CPU.Trigger)
				}
				m.mu.RUnlock()
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 100; j++ {
			m.mu.Lock()
			m.config.CustomRules[0].Enabled = j%2 == 0
			m.mu.Unlock()
		}
	}()
	wg.Wait()
}

var guestRuleBenchmarkResult ThresholdConfig

// Benchmark the actual threshold-policy read, not only the stack helper.
func BenchmarkGuestRulePolicy(b *testing.B) {
	for _, kind := range []string{"vm-value", "lxc-pointer", "guest-snapshot"} {
		for _, size := range []struct {
			name                  string
			rules, filters, disks int
		}{{"no-rules", 0, 0, 8}, {"one-condition", 1, 1, 8}, {"eight-rules", 8, 4, 8}, {"many-rules", 32, 8, 8}, {"no-inventory", 32, 8, 0}} {
			b.Run(kind+"/"+size.name, func(b *testing.B) {
				vm := guestRuleVM(size.disks)
				var guest any = vm
				if kind == "lxc-pointer" {
					ct := guestRuleContainer(vm)
					guest = &ct
				} else if kind == "guest-snapshot" {
					guest = guestSnapshotFromVM(vm)
				}
				m := &Manager{config: AlertConfig{GuestDefaults: ThresholdConfig{CPU: &HysteresisThreshold{Trigger: 90, Clear: 85}}}}
				for i := 0; i < size.rules; i++ {
					rule := CustomAlertRule{Enabled: true, Priority: i, FilterConditions: FilterStack{LogicalOperator: "AND"},
						Thresholds: ThresholdConfig{CPU: &HysteresisThreshold{Trigger: 80, Clear: 75}}}
					for f := 0; f < size.filters; f++ {
						rule.FilterConditions.Filters = append(rule.FilterConditions.Filters, FilterCondition{Type: "metric", Field: "cpu", Operator: ">", Value: 1.0})
					}
					m.config.CustomRules = append(m.config.CustomRules, rule)
				}
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					m.mu.RLock()
					guestRuleBenchmarkResult = m.getGuestThresholds(guest, vm.ID)
					m.mu.RUnlock()
				}
			})
		}
	}
}
