package alerts

import (
	"bytes"
	"testing"

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
