package alerts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/eventlog"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func invalidFilesystemRows() []struct {
	name       string
	invalidate func(*models.Disk)
} {
	return []struct {
		name       string
		invalidate func(*models.Disk)
	}{
		{"negative-usage", func(d *models.Disk) { d.Usage = -1 }},
		{"zero-total", func(d *models.Disk) { d.Total = 0 }},
		{"negative-total", func(d *models.Disk) { d.Total = -1 }},
		{"over-100", func(d *models.Disk) { d.Usage = 101 }},
		{"nan", func(d *models.Disk) { d.Usage = math.NaN() }},
		{"positive-infinity", func(d *models.Disk) { d.Usage = math.Inf(1) }},
		{"negative-infinity", func(d *models.Disk) { d.Usage = math.Inf(-1) }},
	}
}

func checkInvalidFilesystemGuest(m *Manager, vm models.VM, kind string) {
	ct := models.Container{ID: vm.ID, VMID: vm.VMID, Name: vm.Name, Node: vm.Node, Instance: vm.Instance, Type: "lxc", Status: vm.Status, Memory: vm.Memory, Disk: vm.Disk, Disks: vm.Disks, IORateValidity: vm.IORateValidity}
	switch kind {
	case "vm":
		m.CheckGuest(vm, "site")
	case "vm-pointer":
		m.CheckGuest(&vm, "site")
	case "lxc":
		m.CheckGuest(ct, "site")
	case "lxc-pointer":
		m.CheckGuest(&ct, "site")
	}
}

// A row still in the inventory is not removed just because its measurement
// is unusable. Run the real evaluator, public snapshots and event consumer.
func TestGuestFilesystemInvalidRowLifecycle(t *testing.T) {
	for _, kind := range []string{"vm", "vm-pointer", "lxc", "lxc-pointer"} {
		for _, invalid := range invalidFilesystemRows() {
			t.Run(kind+"/"+invalid.name, func(t *testing.T) {
				m, elapsed := filesystemRecoveryManager(t, 0, false)
				vm, peer := filesystemRecoveryVM(105), filesystemRecoveryVM(106)
				checkInvalidFilesystemGuest(m, vm, kind)
				checkInvalidFilesystemGuest(m, peer, kind)
				id := filesystemRecoveryID(vm)
				original := filesystemRecoveryActiveAlert(t, m, id)
				if err := m.AcknowledgeAlert(id, "fixture operator"); err != nil {
					t.Fatal(err)
				}
				before := guestObservationBytes(t, m)
				invalid.invalidate(&vm.Disks[1])
				vm.Disk.Usage = 20 // A healthy rollup is not this filesystem's recovery.
				for poll := 0; poll < 3; poll++ {
					elapsed.Add(int64(time.Minute))
					checkInvalidFilesystemGuest(m, vm, kind)
				}
				if !bytes.Equal(before, guestObservationBytes(t, m)) {
					t.Fatal("invalid row changed incident/occurrence/acknowledgement or its peer")
				}
				if len(queryAlertEvents(t, m, eventlog.Filter{Types: []string{eventlog.TypeResolved}})) != 0 {
					t.Fatal("invalid row published recovery")
				}
				vm = filesystemRecoveryVM(105)
				setBackupFilesystemUsage(&vm, 20)
				checkInvalidFilesystemGuest(m, vm, kind)
				checkInvalidFilesystemGuest(m, vm, kind)
				requireFilesystemRecovery(t, m, original)
				if len(m.GetActiveAlerts()) != 1 || m.GetActiveAlerts()[0].ResourceID == original.ResourceID {
					t.Fatal("fresh recovery crossed guest identity")
				}
				if m.ShadowDivergences() != 0 {
					t.Fatal("shadow/core diverged")
				}
			})
		}
	}
}

func TestGuestFilesystemInvalidRowDuplicateCannotRecover(t *testing.T) {
	for _, invalid := range invalidFilesystemRows() {
		for _, invalidFirst := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/first-%t", invalid.name, invalidFirst), func(t *testing.T) {
				m, _ := filesystemRecoveryManager(t, 0, false)
				vm := filesystemRecoveryVM(105)
				m.CheckGuest(vm, "site")
				original := filesystemRecoveryActiveAlert(t, m, filesystemRecoveryID(vm))
				low, gap := vm.Disks[1], vm.Disks[1]
				low.Usage, low.Used, low.Free = 20, 200, 800
				invalid.invalidate(&gap)
				vm.Disks = []models.Disk{low, gap}
				if invalidFirst {
					vm.Disks[0], vm.Disks[1] = vm.Disks[1], vm.Disks[0]
				}
				m.CheckGuest(vm, "site")
				held := filesystemRecoveryActiveAlert(t, m, original.ID)
				if held.Value != original.Value || held.StartTime != original.StartTime {
					t.Fatal("healthy alias hid invalid evidence")
				}
				vm.Disks = []models.Disk{low, low}
				m.CheckGuest(vm, "site")
				requireFilesystemRecovery(t, m, original)
			})
		}
	}
}

func TestGuestFilesystemInvalidRowInterruptsActivation(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		for _, invalid := range invalidFilesystemRows() {
			t.Run(fmt.Sprintf("%s/explicit-%t", invalid.name, explicit), func(t *testing.T) {
				m, elapsed := filesystemRecoveryManager(t, 60, explicit)
				vm := filesystemRecoveryVM(105)
				m.CheckGuest(vm, "site")
				elapsed.Store(int64(30 * time.Second))
				invalid.invalidate(&vm.Disks[1])
				m.CheckGuest(vm, "site")
				vm = filesystemRecoveryVM(105)
				elapsed.Store(int64(90 * time.Second))
				m.CheckGuest(vm, "site")
				elapsed.Store(int64(149 * time.Second))
				m.CheckGuest(vm, "site")
				if len(m.GetActiveAlerts()) != 0 {
					t.Fatal("activation completed across a missing observation")
				}
				elapsed.Store(int64(150 * time.Second))
				m.CheckGuest(vm, "site")
				if len(m.GetActiveAlerts()) != 1 {
					t.Fatal("new continuous observations could not complete activation")
				}
			})
		}
	}
}

func TestGuestFilesystemInvalidRowIndependentCleanupAndPolicy(t *testing.T) {
	for _, end := range []string{"removed", "disabled", "threshold-off"} {
		for _, invalid := range invalidFilesystemRows() {
			t.Run(end+"/"+invalid.name, func(t *testing.T) {
				m, _ := filesystemRecoveryManager(t, 0, false)
				vm := filesystemRecoveryVM(105)
				vm.Disks[0].Usage, vm.Disks[0].Used, vm.Disks[0].Free = 95, 950, 50
				m.CheckGuest(vm, "site")
				backup := filesystemRecoveryActiveAlert(t, m, filesystemRecoveryID(vm))
				vm.Disks[0].Usage, vm.Disks[0].Used, vm.Disks[0].Free = 20, 200, 800
				invalid.invalidate(&vm.Disks[1])
				m.CheckGuest(vm, "site")
				filesystemRecoveryActiveAlert(t, m, backup.ID)
				if len(m.GetActiveAlerts()) != 1 {
					t.Fatal("invalid row blocked independent sibling recovery")
				}
				switch end {
				case "removed":
					vm.Disks = vm.Disks[:1]
				default:
					cfg := m.GetConfig()
					_, key, _ := guestDiskIdentity(vm.Disks[1], 1)
					override := ThresholdConfig{Disabled: true}
					if end == "threshold-off" {
						override = ThresholdConfig{Disk: &HysteresisThreshold{Trigger: -1}}
					}
					cfg.Overrides[guestDiskOverrideKey(vm, vm.ID, key)] = override
					m.UpdateConfig(cfg)
				}
				m.CheckGuest(vm, "site")
				if len(m.GetActiveAlerts()) != 0 {
					t.Fatal("invalid observation blocked authoritative removal or explicit disablement")
				}
			})
		}
	}
}

func TestGuestFilesystemInvalidRowAggregateFallback(t *testing.T) {
	for _, invalid := range invalidFilesystemRows()[:3] {
		t.Run(invalid.name, func(t *testing.T) {
			m, _ := filesystemRecoveryManager(t, 0, false)
			vm := filesystemRecoveryVM(105)
			vm.Disks = vm.Disks[1:]
			invalid.invalidate(&vm.Disks[0])
			m.CheckGuest(vm, "site")
			active := m.GetActiveAlerts()
			if len(active) != 1 || active[0].ResourceID != vm.ID {
				t.Fatal("existing independently usable aggregate fallback changed")
			}
		})
	}
}

// Export only exact, finite public Alert JSON. Invalid input never becomes
// a zero, NaN, recovery, or replacement alert in the consuming Overview.
func TestGuestFilesystemInvalidRowConsumerProjection(t *testing.T) {
	m, elapsed := filesystemRecoveryManager(t, 0, false)
	vm, peer := filesystemRecoveryVM(105), filesystemRecoveryVM(106)
	vm.Disks[0].Usage, vm.Disks[0].Used, vm.Disks[0].Free = 95, 950, 50
	phases := make(map[string]any)
	emit := func(phase string) {
		phases[phase] = map[string]any{"active": m.GetActiveAlerts(), "history": m.GetAlertHistory(20)}
	}
	m.CheckGuest(vm, "site")
	m.CheckGuest(peer, "site")
	id := filesystemRecoveryID(vm)
	emit("firing")
	if err := m.AcknowledgeAlert(id, "fixture operator"); err != nil {
		t.Fatal(err)
	}
	emit("acknowledged")
	elapsed.Add(int64(time.Minute))
	vm.Disks[1].Usage, vm.Disk.Usage = -1, 20
	m.CheckGuest(vm, "site")
	emit("invalid-usage")
	elapsed.Add(int64(time.Minute))
	vm.Disks[1].Usage, vm.Disks[1].Total = 0, 0
	vm.Disks[0].Usage, vm.Disks[0].Used, vm.Disks[0].Free = 20, 200, 800
	m.CheckGuest(vm, "site")
	emit("invalid-capacity")
	elapsed.Add(int64(time.Minute))
	vm.Node, vm.ID = "moved", "site:moved:105"
	m.CheckGuest(vm, "site")
	emit("moved-invalid")
	elapsed.Add(int64(time.Minute))
	vm = filesystemRecoveryVM(105)
	vm.Node, vm.ID = "moved", "site:moved:105"
	setBackupFilesystemUsage(&vm, 20)
	m.CheckGuest(vm, "site")
	emit("recovered")
	if len(m.GetActiveAlerts()) != 1 {
		t.Fatal("fixture did not isolate final recovery")
	}
	wire, err := json.Marshal(phases)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("GUEST_INVALID_FILESYSTEM_PROJECTIONS %s", wire)
}
