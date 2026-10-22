package alerts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/eventlog"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/reducer"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func statusObservationGuest(vm models.VM, kind string) any {
	ct := models.Container{ID: vm.ID, VMID: vm.VMID, Name: vm.Name, Node: vm.Node, Instance: vm.Instance, Type: "lxc", Status: vm.Status,
		CPU: vm.CPU, Memory: vm.Memory, Disk: vm.Disk, Disks: vm.Disks, DiskRead: vm.DiskRead, DiskWrite: vm.DiskWrite,
		NetworkIn: vm.NetworkIn, NetworkOut: vm.NetworkOut, IORateValidity: vm.IORateValidity, Tags: vm.Tags, OnBoot: vm.OnBoot}
	switch kind {
	case "vm-pointer":
		return &vm
	case "lxc":
		return ct
	case "lxc-pointer":
		return &ct
	default:
		return vm
	}
}

func TestGuestUnknownStatusPreservesIncidents(t *testing.T) {
	for _, kind := range []string{"vm", "vm-pointer", "lxc", "lxc-pointer"} {
		for _, status := range []string{"", "unknown", "unreachable", "provider-future-state"} {
			t.Run(kind+"/"+status, func(t *testing.T) {
				m := guestObservationManager(t)
				vm := guestObservationVM()
				vm.CPU, vm.DiskWrite = .95, 10*1024*1024
				vm.IORateValidity.DiskWrite = true
				m.CheckGuest(statusObservationGuest(vm, kind), "site")
				if len(m.GetActiveAlerts()) != 5 {
					t.Fatal("expected CPU, memory, disk-write and two filesystem incidents")
				}
				for _, alert := range m.GetActiveAlerts() {
					if err := m.AcknowledgeAlert(alert.ID, "fixture operator"); err != nil {
						t.Fatal(err)
					}
				}
				before := guestObservationBytes(t, m)
				events := len(queryAlertEvents(t, m, eventlog.Filter{}))
				vm.Status, vm.CPU, vm.DiskWrite = status, 0, 0
				vm.Memory.Usage, vm.Disk.Usage, vm.Disks = 0, 0, nil
				for poll := 0; poll < 3; poll++ {
					m.CheckGuest(statusObservationGuest(vm, kind), "site")
				}
				if !bytes.Equal(before, guestObservationBytes(t, m)) || len(m.GetRecentlyResolved()) != 0 || len(queryAlertEvents(t, m, eventlog.Filter{})) != events {
					t.Fatal("unknown status changed an incident, acknowledgement or published recovery")
				}
				vm.Status = "running"
				for poll := 0; poll < 3; poll++ {
					m.CheckGuest(statusObservationGuest(vm, kind), "site")
				}
				if len(m.GetActiveAlerts()) != 0 || len(m.GetRecentlyResolved()) != 5 {
					t.Fatal("ordinary current recovery/removal did not resolve each incident once")
				}
			})
		}
	}
}

func TestGuestUnknownStatusCannotFire(t *testing.T) {
	for _, kind := range []string{"vm", "vm-pointer", "lxc", "lxc-pointer"} {
		m := guestObservationManager(t)
		vm := guestObservationVM()
		vm.Status = "provider-future-state"
		for poll := 0; poll < 3; poll++ {
			m.CheckGuest(statusObservationGuest(vm, kind), "site")
		}
		if len(m.GetActiveAlerts()) != 0 || len(m.GetRecentlyResolved()) != 0 {
			t.Fatalf("%s: unknown state became a breach or recovery", kind)
		}
	}
}

func TestGuestUnknownStatusInterruptsMetricActivation(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		for _, kind := range []string{"vm", "lxc"} {
			t.Run(fmt.Sprintf("%s/explicit-%t", kind, explicit), func(t *testing.T) {
				m, elapsed := continuityManager(t, explicit)
				vm := guestObservationVM()
				vm.Memory.Usage = 85
				for i := range vm.Disks {
					vm.Disks[i].Usage = 85
				}
				m.CheckGuest(statusObservationGuest(vm, kind), "site")
				elapsed.Store(int64(30 * time.Second))
				vm.Status = "unknown"
				m.CheckGuest(statusObservationGuest(vm, kind), "site")
				vm.Status = "running"
				elapsed.Store(int64(90 * time.Second))
				m.CheckGuest(statusObservationGuest(vm, kind), "site")
				elapsed.Store(int64(149 * time.Second))
				m.CheckGuest(statusObservationGuest(vm, kind), "site")
				if len(m.GetActiveAlerts()) != 0 {
					t.Fatal("metric activation spanned unknown guest status")
				}
				elapsed.Store(int64(150 * time.Second))
				m.CheckGuest(statusObservationGuest(vm, kind), "site")
				if len(m.GetActiveAlerts()) != 3 || m.ShadowDivergences() != 0 {
					t.Fatal("fresh continuous metric observations did not fire once")
				}
			})
		}
	}
}

func TestGuestUnknownStatusPreservesPoweredOff(t *testing.T) {
	for _, kind := range []string{"vm", "vm-pointer", "lxc", "lxc-pointer"} {
		t.Run(kind, func(t *testing.T) {
			m := guestObservationManager(t)
			vm := guestObservationVM()
			vm.Status = "stopped"
			m.CheckGuest(statusObservationGuest(vm, kind), "site")
			m.CheckGuest(statusObservationGuest(vm, kind), "site")
			active := m.GetActiveAlerts()
			if len(active) != 1 || active[0].Type != "powered-off" {
				t.Fatal("expected powered-off incident")
			}
			if err := m.AcknowledgeAlert(active[0].ID, "fixture operator"); err != nil {
				t.Fatal(err)
			}
			before := guestObservationBytes(t, m)
			vm.Status = "unknown"
			for poll := 0; poll < 3; poll++ {
				m.CheckGuest(statusObservationGuest(vm, kind), "site")
			}
			if !bytes.Equal(before, guestObservationBytes(t, m)) || len(m.GetRecentlyResolved()) != 0 {
				t.Fatal("unknown status announced that a stopped guest is running")
			}
			vm.Status = "running"
			vm.Memory.Usage, vm.Disk.Usage, vm.Disks = 0, 0, nil
			m.CheckGuest(statusObservationGuest(vm, kind), "site")
			if len(m.GetActiveAlerts()) != 0 || len(m.GetRecentlyResolved()) != 1 {
				t.Fatal("a real running observation did not clear powered-off")
			}
		})
	}
}

func TestGuestUnknownStatusInterruptsPoweredOffActivation(t *testing.T) {
	m := guestObservationManager(t)
	vm := guestObservationVM()
	vm.Status = "stopped"
	m.CheckGuest(vm, "site")
	vm.Status = "unknown"
	m.CheckGuest(vm, "site")
	vm.Status = "stopped"
	m.CheckGuest(vm, "site")
	if len(m.GetActiveAlerts()) != 0 {
		t.Fatal("stopped confirmations spanned unknown status")
	}
	m.CheckGuest(vm, "site")
	if len(m.GetActiveAlerts()) != 1 {
		t.Fatal("consecutive stopped observations could not fire")
	}
}

func TestGuestConfirmedNonRunningPolicyUnchanged(t *testing.T) {
	for _, kind := range []string{"vm", "lxc"} {
		for _, status := range []string{"stopped", "paused", "suspended"} {
			t.Run(kind+"/"+status, func(t *testing.T) {
				m := guestObservationManager(t)
				vm := guestObservationVM()
				m.CheckGuest(statusObservationGuest(vm, kind), "site")
				vm.Status = status
				m.CheckGuest(statusObservationGuest(vm, kind), "site")
				m.CheckGuest(statusObservationGuest(vm, kind), "site")
				want := 0
				if status == "stopped" {
					want = 1
				}
				if len(m.GetActiveAlerts()) != want || len(m.GetRecentlyResolved()) != 3 {
					t.Fatal("explicit non-running metric/powered-state policy changed")
				}
			})
		}
	}
}

func TestGuestUnknownStatusInterruptsRecovery(t *testing.T) {
	for _, kind := range []string{"vm", "lxc"} {
		t.Run(kind, func(t *testing.T) {
			m, elapsed := continuityManager(t, false)
			vm := guestObservationVM()
			vm.Disks, vm.Disk.Usage = nil, 0
			m.CheckGuest(statusObservationGuest(vm, kind), "site")
			if len(m.GetActiveAlerts()) != 1 {
				t.Fatal("critical memory did not fire")
			}
			elapsed.Store(int64(10 * time.Second))
			vm.Memory.Usage = 10
			m.CheckGuest(statusObservationGuest(vm, kind), "site")
			incident, _ := continuityIncident(m, vm.ID, "memory")
			if incident.RecoverySince.IsZero() {
				t.Fatal("current healthy memory did not begin recovery")
			}
			before := guestObservationBytes(t, m)
			elapsed.Store(int64(2 * time.Minute))
			vm.Status = "unknown"
			m.CheckGuest(statusObservationGuest(vm, kind), "site")
			incident, _ = continuityIncident(m, vm.ID, "memory")
			if !incident.RecoverySince.IsZero() || !bytes.Equal(before, guestObservationBytes(t, m)) {
				t.Fatal("unknown status kept recovery timing or changed the open incident")
			}
			vm.Status = "running"
			m.CheckGuest(statusObservationGuest(vm, kind), "site")
			elapsed.Store(int64(2*time.Minute + 59*time.Second))
			m.CheckGuest(statusObservationGuest(vm, kind), "site")
			if len(m.GetActiveAlerts()) != 1 {
				t.Fatal("recovery completed across unknown status")
			}
			elapsed.Store(int64(3 * time.Minute))
			m.CheckGuest(statusObservationGuest(vm, kind), "site")
			if len(m.GetActiveAlerts()) != 0 || len(m.GetRecentlyResolved()) != 1 || m.ShadowDivergences() != 0 {
				t.Fatal("continuous healthy memory failed to recover once")
			}
		})
	}
}

func TestGuestUnknownStatusPreservesIndependentEvidenceAndPolicy(t *testing.T) {
	for _, policy := range []string{"threshold-off", "filesystem-off", "offline-off", "pulse-no-alerts"} {
		t.Run(policy, func(t *testing.T) {
			m := guestObservationManager(t)
			vm := guestObservationVM()
			if policy == "offline-off" {
				vm.Status = "stopped"
			}
			m.CheckGuest(vm, "site")
			m.CheckGuest(vm, "site")
			vm.Status = "unknown"
			cfg := m.GetConfig()
			want := 0
			switch policy {
			case "threshold-off":
				cfg.GuestDefaults.Memory, cfg.GuestDefaults.Disk = &HysteresisThreshold{Trigger: -1}, &HysteresisThreshold{Trigger: -1}
			case "filesystem-off":
				_, key, _ := guestDiskIdentity(vm.Disks[0], 0)
				cfg.Overrides[guestDiskOverrideKey(vm, vm.ID, key)] = ThresholdConfig{Disabled: true}
				want = 2
			case "offline-off":
				cfg.DisableAllGuestsOffline = true
			case "pulse-no-alerts":
				vm.Tags = []string{"pulse-no-alerts"}
			}
			m.UpdateConfig(cfg)
			m.CheckGuest(vm, "site")
			if len(m.GetActiveAlerts()) != want {
				t.Fatal("unknown status blocked explicit operator disablement")
			}
		})
	}
	// A guest status gap does not retire the independent backup observation
	// with the same resource ID, nor pending work on another VM or cluster.
	m, _ := continuityManager(t, true)
	vm := guestObservationVM()
	m.CheckGuest(vm, "site")
	peer := vm
	peer.ID, peer.VMID = "site:node:106", 106
	m.CheckGuest(peer, "site")
	otherCluster := vm
	otherCluster.ID, otherCluster.Instance = "other:node:105", "other"
	m.CheckGuest(otherCluster, "other")
	m.mu.Lock()
	m.core.ApplyDiscrete(reducer.DiscreteSignal{ResourceID: vm.ID, Key: "independent-backup", Matched: true, ObservedAt: m.policyNow()}, reducer.DiscreteRule{Confirmations: 2})
	m.mu.Unlock()
	vm.Status = "unknown"
	m.CheckGuest(vm, "site")
	m.mu.RLock()
	_, backupPending := m.core.Incident(vm.ID, "independent-backup")
	m.mu.RUnlock()
	if !backupPending {
		t.Fatal("guest status discarded independent backup evidence")
	}
	if _, exists := continuityIncident(m, peer.ID, "memory"); !exists {
		t.Fatal("guest status interrupted another VM")
	}
	if _, exists := continuityIncident(m, otherCluster.ID, "memory"); !exists {
		t.Fatal("guest status crossed instance identity")
	}
	if _, exists := continuityIncident(m, vm.ID, "memory"); exists {
		t.Fatal("unknown status retained this guest's activation")
	}
}

// The browser consumes these exact public projections, not independently
// authored alert JSON. No guest command, backup, SMTP or native estate is used.
func TestGuestStatusObservationConsumerFrames(t *testing.T) {
	m := guestObservationManager(t)
	vm := guestObservationVM()
	vm.Memory.Usage = 0
	vm.Disks = vm.Disks[:1]
	frames := make(map[string]any)
	capture := func(phase string) {
		frames[phase] = map[string]any{"active": m.GetActiveAlerts(), "resolved": m.GetRecentlyResolved()}
	}
	m.CheckGuest(vm, "site")
	capture("firing")
	before := guestObservationBytes(t, m)
	vm.Status, vm.Disks, vm.Memory.Usage, vm.Disk.Usage = "unknown", nil, 0, 0
	m.CheckGuest(vm, "site")
	if !bytes.Equal(before, guestObservationBytes(t, m)) {
		t.Fatal("unknown status removed the browser's incident")
	}
	capture("unknown")
	for _, alert := range m.GetActiveAlerts() {
		if err := m.AcknowledgeAlert(alert.ID, "fixture operator"); err != nil {
			t.Fatal(err)
		}
	}
	capture("acknowledged")
	vm.Status = "provider-future-state"
	m.CheckGuest(vm, "site")
	capture("future")
	vm.Status = "running"
	m.CheckGuest(vm, "site")
	m.CheckGuest(vm, "site")
	capture("recovered")
	vm.Status = "stopped"
	m.CheckGuest(vm, "site")
	m.CheckGuest(vm, "site")
	capture("powered-off")
	before = guestObservationBytes(t, m)
	vm.Status = "unknown"
	m.CheckGuest(vm, "site")
	if !bytes.Equal(before, guestObservationBytes(t, m)) {
		t.Fatal("unknown status removed powered-off from the browser")
	}
	capture("powered-unknown")
	vm.Status = "running"
	m.CheckGuest(vm, "site")
	capture("running")
	if os.Getenv("PULSE_GUEST_STATUS_FRAMES") == "1" {
		encoded, err := json.Marshal(frames)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Printf("GUEST_STATUS_FRAMES %s\n", encoded)
	}
}
