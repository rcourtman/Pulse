package alerts

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/eventlog"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func duplicateFilesystemVM() models.VM {
	vm := filesystemRecoveryVM(105)
	high := vm.Disks[1]
	low := high
	low.Usage, low.Used, low.Free = 20, 200, 800
	vm.Disks = []models.Disk{low, high}
	return vm
}

func TestGuestFilesystemDuplicateIdentityLifecycle(t *testing.T) {
	for _, grace := range []int{0, 60} {
		t.Run(fmt.Sprintf("grace-%d", grace), func(t *testing.T) {
			m, elapsed := filesystemRecoveryManager(t, grace, true)
			vm, peer := duplicateFilesystemVM(), filesystemRecoveryVM(106)
			peer.Disks = peer.Disks[1:]
			id := canonicalMetricStateID(vm.ID+"-disk-backup-dev-vdb1", "disk")
			// Same display name, but a separate guest remains independent.
			before, _ := json.Marshal(vm.Disks)
			m.CheckGuest(vm, "site")
			m.CheckGuest(peer, "site")
			if grace > 0 {
				if len(m.GetActiveAlerts()) != 0 {
					t.Fatal("duplicate observations completed activation grace")
				}
				elapsed.Store(int64(time.Duration(grace-1) * time.Second))
				vm.Disks[0], vm.Disks[1] = vm.Disks[1], vm.Disks[0]
				m.CheckGuest(vm, "site")
				if len(m.GetActiveAlerts()) != 0 {
					t.Fatal("reordering completed activation early")
				}
				elapsed.Store(int64(time.Duration(grace) * time.Second))
				m.CheckGuest(vm, "site")
				m.CheckGuest(peer, "site")
			}
			original := filesystemRecoveryActiveAlert(t, m, id)
			if len(m.GetActiveAlerts()) != 2 || original.Value != 95 {
				t.Fatal("one observation per filesystem/guest required")
			}
			if err := m.AcknowledgeAlert(id, "fixture operator"); err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 4; i++ {
				elapsed.Add(int64(time.Second))
				vm.Disks[0], vm.Disks[1] = vm.Disks[1], vm.Disks[0]
				m.CheckGuest(vm, "site")
				held := filesystemRecoveryActiveAlert(t, m, id)
				if held.StartTime != original.StartTime || !held.Acknowledged || held.Value != 95 || held.Metadata["usedBytes"] != int64(950) {
					t.Fatal("low alias changed occurrence/acknowledgement/high tuple")
				}
			}
			if got := queryAlertEvents(t, m, eventlog.Filter{AlertID: id, Types: []string{eventlog.TypeFired}}); len(got) != 1 {
				t.Fatal("duplicate row created another firing")
			}
			if got := queryAlertEvents(t, m, eventlog.Filter{AlertID: id, Types: []string{eventlog.TypeResolved}}); len(got) != 0 {
				t.Fatal("low duplicate invented recovery")
			}
			// Restore original order before checking the caller-owned inventory.
			if grace > 0 {
				vm.Disks[0], vm.Disks[1] = vm.Disks[1], vm.Disks[0]
			}
			after, _ := json.Marshal(vm.Disks)
			if string(before) != string(after) {
				t.Fatal("alert selection mutated caller-owned rows")
			}
			// Retained duplicate data still does not establish a fresh recovery.
			vm.DiskStatusReason = "prev-vm-locked"
			setBackupFilesystemUsage(&vm, 0)
			m.CheckGuest(vm, "site")
			if !filesystemRecoveryActiveAlert(t, m, id).Acknowledged {
				t.Fatal("retained reading lost acknowledgement")
			}
			vm.DiskStatusReason = ""
			m.CheckGuest(vm, "site")
			m.CheckGuest(vm, "site")
			requireFilesystemRecovery(t, m, original)
			if len(m.GetActiveAlerts()) != 1 || m.GetActiveAlerts()[0].ResourceID == original.ResourceID {
				t.Fatal("recovery affected same-named peer")
			}
			if m.ShadowDivergences() != 0 {
				t.Fatal("shadow/core diverged")
			}
		})
	}
}

func TestGuestFilesystemDuplicateIdentityPolicyAndInvalidObservation(t *testing.T) {
	for _, value := range []float64{101, math.NaN(), math.Inf(1)} {
		for _, invalidFirst := range []bool{false, true} {
			t.Run(fmt.Sprintf("invalid-%v/first-%t", value, invalidFirst), func(t *testing.T) {
				m, _ := filesystemRecoveryManager(t, 0, false)
				vm := duplicateFilesystemVM()
				m.CheckGuest(vm, "site")
				low := vm.Disks[0]
				invalid := low
				invalid.Usage = value
				vm.Disks = []models.Disk{low, invalid}
				if invalidFirst {
					vm.Disks[0], vm.Disks[1] = vm.Disks[1], vm.Disks[0]
				}
				m.CheckGuest(vm, "site")
				held := filesystemRecoveryActiveAlert(t, m, original.ID)
				if held.StartTime != original.StartTime || held.Value != original.Value {
					t.Fatal("invalid alias became recovery")
				}
				_, key, _ := guestDiskIdentity(low, 0)
				cfg := m.GetConfig()
				cfg.Overrides[guestDiskOverrideKey(vm, vm.ID, key)] = ThresholdConfig{Disabled: true}
				m.UpdateConfig(cfg)
				m.CheckGuest(vm, "site")
				if len(m.GetActiveAlerts()) != 0 {
					t.Fatal("invalid alias prevented explicit disablement")
				}
			})
		}
	}
	m, _ := filesystemRecoveryManager(t, 0, false)
	vm := duplicateFilesystemVM()
	// Same mount, distinct devices keep separate existing identities and policies.
	vm.Disks[0].Device = "/dev/vdc1"
	_, key, _ := guestDiskIdentity(vm.Disks[1], 1)
	cfg := m.GetConfig()
	cfg.Overrides[guestDiskOverrideKey(vm, vm.ID, key)] = ThresholdConfig{Disk: &HysteresisThreshold{Trigger: 99, Clear: 90}}
	m.UpdateConfig(cfg)
	m.CheckGuest(vm, "site")
	if len(m.GetActiveAlerts()) != 0 {
		t.Fatal("filesystem override was ignored")
	}
	vm.Disks[0].Usage, vm.Disks[0].Used, vm.Disks[0].Free = 96, 960, 40
	m.CheckGuest(vm, "site")
	active := m.GetActiveAlerts()
	if len(active) != 1 || active[0].Metadata["device"] != "/dev/vdc1" {
		t.Fatal("distinct identity inherited another disk policy")
	}
}

// Export exact public Alert JSON for the unchanged production Overview consumer.
func TestGuestFilesystemDuplicateIdentityConsumerProjection(t *testing.T) {
	m, elapsed := filesystemRecoveryManager(t, 0, false)
	vm, peer := duplicateFilesystemVM(), filesystemRecoveryVM(106)
	peer.Disks = peer.Disks[1:]
	phases := make(map[string]any)
	emit := func(name string) {
		phases[name] = map[string]any{"active": m.GetActiveAlerts(), "history": m.GetAlertHistory(20)}
	}
	m.CheckGuest(vm, "site")
	m.CheckGuest(peer, "site")
	emit("firing")
	var own *Alert
	for _, a := range m.GetActiveAlerts() {
		if a.Metadata["guestResourceId"] == vm.ID {
			own = a.Clone()
		}
	}
	if own == nil {
		t.Fatal("fixture did not fire primary")
	}
	elapsed.Add(int64(time.Second))
	vm.Disks[0], vm.Disks[1] = vm.Disks[1], vm.Disks[0]
	m.CheckGuest(vm, "site")
	emit("reordered")
	if err := m.AcknowledgeAlert(own.ID, "fixture operator"); err != nil {
		t.Fatal(err)
	}
	emit("acknowledged")
	elapsed.Add(int64(time.Second))
	vm.Disks[0], vm.Disks[1] = vm.Disks[1], vm.Disks[0]
	m.CheckGuest(vm, "site")
	emit("low-first")
	setBackupFilesystemUsage(&vm, 0)
	m.CheckGuest(vm, "site")
	emit("recovered")
	if len(m.GetActiveAlerts()) != 1 || reflect.DeepEqual(phases["low-first"], phases["recovered"]) {
		t.Fatal("fixture never recovered")
	}
	wire, err := json.Marshal(phases)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("GUEST_DUPLICATE_PROJECTIONS %s", wire)
}
