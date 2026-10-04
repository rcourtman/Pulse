package alerts

import (
	"bytes"
	"encoding/json"
	"sort"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/eventlog"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func guestObservationManager(t *testing.T) *Manager {
	t.Helper()
	m := newEventLogManager(t)
	cfg := characterizationBaseConfig()
	cfg.GuestDefaults.DiskWrite = &HysteresisThreshold{Trigger: 5, Clear: 4}
	m.UpdateConfig(cfg)
	disableTestTimeThresholds(m)
	return m
}

func guestObservationVM() models.VM {
	return models.VM{
		ID: "site:node:105", VMID: 105, Name: "database", Node: "node", Instance: "site", Type: "qemu", Status: "running",
		Memory: models.Memory{Total: 1000, Used: 960, Free: 40, Usage: 96,
			Observation: models.MemoryObservation{State: "current", Source: "guest-agent-meminfo", ObservedAt: time.Now().Add(-time.Second)}},
		Disk: models.Disk{Total: 1000, Used: 980, Free: 20, Usage: 98},
		Disks: []models.Disk{{Total: 1000, Used: 980, Free: 20, Usage: 98, Mountpoint: "/", Device: "/dev/vda"},
			{Total: 1000, Used: 980, Free: 20, Usage: 98, Mountpoint: "/data", Device: "/dev/vdb"}},
		IORateValidity: models.IORateValidity{Explicit: true},
	}
}

func guestObservationBytes(t *testing.T, m *Manager) []byte {
	t.Helper()
	active := m.GetActiveAlerts()
	sort.Slice(active, func(i, j int) bool { return active[i].ID < active[j].ID })
	data, err := json.Marshal(active)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestGuestAlertRetainedObservationsCannotFire(t *testing.T) {
	for _, state := range []string{"last-known", "unavailable", "unknown-version"} {
		for _, kind := range []string{"vm", "vm-pointer", "lxc", "lxc-pointer"} {
			t.Run(kind+"/"+state, func(t *testing.T) {
				m := guestObservationManager(t)
				vm := guestObservationVM()
				vm.Memory.Observation.State = state
				vm.DiskStatusReason, vm.GuestAgentStatus = "prev-vm-locked", "deferred"
				var guest any = vm
				switch kind {
				case "vm-pointer":
					guest = &vm
				case "lxc", "lxc-pointer":
					ct := models.Container{ID: vm.ID, VMID: vm.VMID, Name: vm.Name, Node: vm.Node, Instance: vm.Instance, Status: vm.Status, Memory: vm.Memory,
						Disk: models.Disk{Usage: 10}, IORateValidity: vm.IORateValidity}
					guest = ct
					if kind == "lxc-pointer" {
						guest = &ct
					}
				}
				for poll := 0; poll < 4; poll++ {
					m.CheckGuest(guest, "site")
				}
				if got := m.GetActiveAlerts(); len(got) != 0 {
					t.Fatalf("retained observations opened alerts: %+v", got)
				}
				if len(queryAlertEvents(t, m, eventlog.Filter{})) != 0 {
					t.Fatal("retained observations emitted lifecycle/delivery evidence")
				}
			})
		}
	}
}

func TestGuestAlertUnavailableDiskInventoryIsNotRecovery(t *testing.T) {
	for _, reason := range []string{"prev-vm-locked", "prev-lock-unverified", "agent-timeout", "agent-not-running", "permission-denied", "agent-error", "no-filesystems"} {
		t.Run(reason, func(t *testing.T) {
			m := guestObservationManager(t)
			vm := guestObservationVM()
			m.CheckGuest(vm, "site")
			active := m.GetActiveAlerts()
			if len(active) != 3 {
				t.Fatalf("current memory and two filesystems must fire: %+v", active)
			}
			for _, a := range active {
				if err := m.AcknowledgeAlert(a.ID, "operator"); err != nil {
					t.Fatal(err)
				}
			}
			before := guestObservationBytes(t, m)
			events := len(queryAlertEvents(t, m, eventlog.Filter{}))
			vm.Memory.Usage, vm.Memory.Used, vm.Memory.Free = 10, 100, 900
			vm.Memory.Observation.State = "last-known"
			vm.Disk.Usage, vm.DiskStatusReason = 10, reason
			for i := range vm.Disks {
				vm.Disks[i].Usage = 10
			}
			m.CheckGuest(vm, "site")
			vm.Disks = nil // Failed/expired retained inventory is not removal.
			vm.Disk.Usage = -1
			for poll := 0; poll < 4; poll++ {
				m.CheckGuest(vm, "site")
			}
			if after := guestObservationBytes(t, m); !bytes.Equal(before, after) {
				t.Fatalf("unknown observation changed incident values/identity/ack/age: before=%s after=%s", before, after)
			}
			if len(queryAlertEvents(t, m, eventlog.Filter{})) != events {
				t.Fatal("unknown observation emitted lifecycle/delivery evidence")
			}
			vm.Memory.Observation.State = "current"
			vm.Disk.Usage, vm.DiskStatusReason = 10, ""
			vm.Disks = []models.Disk{{Total: 1000, Used: 100, Free: 900, Usage: 10, Mountpoint: "/", Device: "/dev/vda"}}
			m.CheckGuest(vm, "site") // Fresh recovery plus authoritative removal of /data.
			m.CheckGuest(vm, "site")
			if len(m.GetActiveAlerts()) != 0 || len(queryAlertEvents(t, m, eventlog.Filter{Types: []string{eventlog.TypeResolved}})) != 3 {
				t.Fatal("fresh recovery/removal must resolve each original incident exactly once")
			}
		})
	}
}

func TestGuestAlertRetainedAggregateAndPendingCannotAdvance(t *testing.T) {
	m := guestObservationManager(t)
	vm := guestObservationVM()
	vm.Disks = nil
	vm.Memory.Usage = 90
	m.mu.Lock()
	m.config.MetricTimeThresholds = map[string]map[string]int{"guest": {"memory": 60, "disk": 60}}
	m.mu.Unlock()
	m.CheckGuest(vm, "site")
	if len(m.GetActiveAlerts()) != 0 {
		t.Fatal("fresh warning observations should begin their delay, not fire")
	}
	m.mu.Lock()
	m.core.ShiftPending(-2 * time.Minute)
	m.mu.Unlock()
	vm.Memory.Observation.State, vm.DiskStatusReason = "last-known", "prev-agent-cooldown"
	for poll := 0; poll < 4; poll++ {
		m.CheckGuest(vm, "site")
	}
	if len(m.GetActiveAlerts()) != 0 {
		t.Fatal("retained memory/disk promoted a delayed observation to an incident")
	}
	vm.Memory.Observation.State, vm.DiskStatusReason = "current", ""
	m.CheckGuest(vm, "site")
	if len(m.GetActiveAlerts()) != 2 {
		t.Fatal("fresh evidence must resume the existing delay policy")
	}
}

func TestGuestAlertFreshIndependentMetricsAndDisablement(t *testing.T) {
	for _, state := range []string{"", "current"} {
		t.Run("legacy-or-current/"+state, func(t *testing.T) {
			m := guestObservationManager(t)
			vm := guestObservationVM()
			vm.Memory.Observation.State = state
			// Guest-agent state alone cannot suppress independent current data.
			vm.GuestAgentStatus, vm.Lock = "deferred", "backup"
			vm.Memory.Observation.Source = "agent"
			m.CheckGuest(vm, "site")
			if len(m.GetActiveAlerts()) != 3 {
				t.Fatal("legacy/current independent memory and filesystems were suppressed")
			}
			vm.Memory.Observation.State, vm.DiskStatusReason = "last-known", "prev-vm-locked"
			cfg := m.GetConfig()
			cfg.GuestDefaults.Memory = &HysteresisThreshold{Trigger: -1}
			cfg.GuestDefaults.Disk = &HysteresisThreshold{Trigger: -1}
			m.UpdateConfig(cfg)
			m.CheckGuest(vm, "site")
			if len(m.GetActiveAlerts()) != 0 {
				t.Fatal("explicit rule disablement must remain independent of telemetry")
			}
		})
	}
	m := guestObservationManager(t)
	vm := guestObservationVM()
	vm.Memory.Observation.State, vm.DiskStatusReason = "last-known", "prev-vm-locked"
	vm.CPU, vm.DiskWrite = .9, 10*1024*1024
	vm.IORateValidity.DiskWrite = true
	m.CheckGuest(vm, "site")
	active := m.GetActiveAlerts()
	if len(active) != 2 {
		t.Fatalf("current CPU/I/O should still fire, retained memory/disk should not: %+v", active)
	}
	for _, a := range active {
		if a.Type != "cpu" && a.Type != "diskWrite" {
			t.Fatalf("unexpected retained metric alert: %+v", a)
		}
	}
}

func TestGuestFiltersDoNotUseRetainedObservations(t *testing.T) {
	m := guestObservationManager(t)
	vm := guestObservationVM()
	for _, field := range []string{"memory", "disk"} {
		for _, op := range []string{">", "<", "=="} {
			condition := FilterCondition{Type: "metric", Field: field, Operator: op, Value: 96.0}
			vm.Memory.Observation.State, vm.DiskStatusReason = "last-known", "prev-vm-locked"
			if m.evaluateVMCondition(vm, condition) || m.evaluateFilterCondition(&vm, condition) {
				t.Fatalf("retained %s matched %s", field, op)
			}
		}
	}
	vm.Memory.Observation.State, vm.DiskStatusReason = "current", ""
	for _, field := range []string{"memory", "disk"} {
		if !m.evaluateVMCondition(vm, FilterCondition{Type: "metric", Field: field, Operator: ">", Value: 90.0}) {
			t.Fatalf("fresh %s could not match", field)
		}
	}
	vm.Memory = models.UnavailableMemory(1000)
	if m.evaluateVMCondition(vm, FilterCondition{Type: "metric", Field: "memory", Operator: "<", Value: 90.0}) {
		t.Fatal("unavailable memory was treated as a measured zero in a filter")
	}
}
