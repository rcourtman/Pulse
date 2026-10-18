package alerts

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/eventlog"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func filesystemRecoveryManager(t *testing.T, grace int, explicit bool) (*Manager, *atomic.Int64) {
	t.Helper()
	m := newEventLogManager(t)
	cfg := characterizationBaseConfig()
	cfg.GuestDefaults.Disk = &HysteresisThreshold{Trigger: 90, Clear: 80}
	cfg.MetricTimeThresholds = map[string]map[string]int{"all": {"disk": grace}}
	m.UpdateConfig(cfg)
	elapsed := &atomic.Int64{}
	origin := time.Now().UTC()
	m.now = func() time.Time { return origin.Add(time.Duration(elapsed.Load())) }
	m.intentClock = func() time.Duration { return time.Duration(elapsed.Load()) }
	if explicit {
		doc := NewAlertIntentPolicyDocument()
		doc.Defaults[MetricAlertIntentSignal("disk")] = AlertIntentRule{GraceSeconds: intPointer(grace)}
		if err := m.LoadIntentPolicies(doc); err != nil {
			t.Fatal(err)
		}
	}
	return m, elapsed
}

func filesystemRecoveryVM(id int) models.VM {
	return models.VM{
		ID: fmt.Sprintf("site:node:%d", id), VMID: id, Name: "backup-worker", Node: "node", Instance: "site", Status: "running", Type: "qemu",
		Memory: models.Memory{Total: 1000, Used: 100, Free: 900, Usage: 10},
		Disk:   models.Disk{Total: 1000, Used: 950, Free: 50, Usage: 95},
		Disks: []models.Disk{
			{Mountpoint: "/", Device: "/dev/vda1", Total: 1000, Used: 200, Free: 800, Usage: 20},
			{Mountpoint: "/backup", Device: "/dev/vdb1", Total: 1000, Used: 950, Free: 50, Usage: 95},
		},
		IORateValidity: models.IORateValidity{Explicit: true},
	}
}

func filesystemRecoveryID(vm models.VM) string {
	_, _, key := guestDiskIdentity(vm.Disks[1], 1)
	return canonicalMetricStateID(vm.ID+"-disk-"+key, "disk")
}

func setBackupFilesystemUsage(vm *models.VM, usage float64) {
	vm.Disk.Usage = usage
	for i := range vm.Disks {
		if vm.Disks[i].Mountpoint == "/backup" {
			vm.Disks[i].Usage = usage
			vm.Disks[i].Used = int64(usage * 10)
			vm.Disks[i].Free = vm.Disks[i].Total - vm.Disks[i].Used
		}
	}
}

func requireFilesystemRecovery(t *testing.T, m *Manager, original *Alert) {
	t.Helper()
	m.mu.Lock()
	active := testHasActiveAlert(t, m, original.ID)
	m.mu.Unlock()
	if active {
		t.Fatal("fresh below-clear usage left the original filesystem incident active")
	}
	fired := queryAlertEvents(t, m, eventlog.Filter{AlertID: original.ID, Types: []string{eventlog.TypeFired}})
	resolved := queryAlertEvents(t, m, eventlog.Filter{AlertID: original.ID, Types: []string{eventlog.TypeResolved}})
	if len(fired) != 1 || len(resolved) != 1 {
		t.Fatalf("expected one firing and one recovery for the same filesystem: fired=%+v resolved=%+v", fired, resolved)
	}
	var firedSnapshot, resolvedSnapshot Alert
	if err := json.Unmarshal(fired[0].Snapshot, &firedSnapshot); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(resolved[0].Snapshot, &resolvedSnapshot); err != nil {
		t.Fatal(err)
	}
	if !firedSnapshot.StartTime.Equal(original.StartTime) || !resolvedSnapshot.StartTime.Equal(original.StartTime) || resolved[0].ResourceID != original.ResourceID || resolvedSnapshot.Resolution != nil {
		t.Fatalf("recovery lost the original occurrence/filesystem: fired=%+v resolved=%+v", fired[0], resolved[0])
	}
}

// Issue #2068: a temporary backup file fills one filesystem, then is removed.
// Drive the real guest evaluator and event reader, not just reducer signals.
// Repeated guest names and reversed disk order must not clear a peer or turn
// the healthy root/guest rollup into recovery of the full backup filesystem.
func TestGuestFilesystemTemporaryUseRecovery(t *testing.T) {
	for _, policy := range []struct {
		name     string
		grace    int
		explicit bool
	}{{"instant", 0, false}, {"legacy-activation-delay", 60, false}, {"explicit-activation-delay", 60, true}} {
		for _, acknowledged := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/acknowledged-%t", policy.name, acknowledged), func(t *testing.T) {
				m, elapsed := filesystemRecoveryManager(t, policy.grace, policy.explicit)
				vm, peer := filesystemRecoveryVM(105), filesystemRecoveryVM(106)
				id, peerID := filesystemRecoveryID(vm), filesystemRecoveryID(peer)
				m.CheckGuest(vm, "site")
				m.CheckGuest(peer, "site")
				if policy.grace > 0 {
					if len(m.GetActiveAlerts()) != 0 {
						t.Fatal("activation grace did not hold temporary high usage")
					}
					elapsed.Store(int64(time.Duration(policy.grace-1) * time.Second))
					m.CheckGuest(vm, "site")
					m.CheckGuest(peer, "site")
					if len(m.GetActiveAlerts()) != 0 {
						t.Fatal("activation grace completed before its boundary")
					}
					elapsed.Store(int64(time.Duration(policy.grace) * time.Second))
					m.CheckGuest(vm, "site")
					m.CheckGuest(peer, "site")
				}
				if len(m.GetActiveAlerts()) != 2 {
					t.Fatalf("wanted exactly the two backup filesystems, not roots/rollups: %+v", m.GetActiveAlerts())
				}
				original, peerOriginal := activeAlert(t, m, id), activeAlert(t, m, peerID)
				if acknowledged {
					if err := m.AcknowledgeAlert(id, "test-operator"); err != nil {
						t.Fatal(err)
					}
				}
				vm.Disks[0], vm.Disks[1] = vm.Disks[1], vm.Disks[0]
				setBackupFilesystemUsage(&vm, 85) // Below trigger, but above clear.
				elapsed.Add(int64(time.Minute))
				m.CheckGuest(vm, "site")
				held := activeAlert(t, m, id)
				if held.Value != original.Value || held.Message != original.Message || held.LastSeen != original.LastSeen || held.StartTime != original.StartTime || held.Acknowledged != acknowledged {
					t.Fatalf("hysteresis changed the breach/occurrence/ack: before=%+v after=%+v", original, held)
				}
				if held.MetricStatus == nil || held.MetricStatus.Phase != models.MetricAlertPhaseLatched || held.MetricStatus.Value != 85 {
					t.Fatalf("current hysteresis hold is not explained: %+v", held.MetricStatus)
				}
				setBackupFilesystemUsage(&vm, 80) // Clear is inclusive, without acknowledgement or a second activation delay.
				elapsed.Add(int64(time.Second))
				m.CheckGuest(vm, "site")
				m.CheckGuest(vm, "site")
				requireFilesystemRecovery(t, m, original)
				if got := activeAlert(t, m, peerID); got.StartTime != peerOriginal.StartTime || got.Value != peerOriginal.Value {
					t.Fatal("same-named peer was changed by another guest's recovery")
				}
				if len(m.GetActiveAlerts()) != 1 {
					t.Fatal("recovery left a duplicate guest-wide alert or lost the peer")
				}
			})
		}
	}
}

func TestGuestFilesystemTemporaryUseUnavailableRecovery(t *testing.T) {
	for _, gap := range []string{"retained-low", "retained-high", "missing-inventory", "negative-current"} {
		t.Run(gap, func(t *testing.T) {
			m, elapsed := filesystemRecoveryManager(t, 0, false)
			vm := filesystemRecoveryVM(105)
			id := filesystemRecoveryID(vm)
			m.CheckGuest(vm, "site")
			original := activeAlert(t, m, id)
			before := guestObservationBytes(t, m)
			setBackupFilesystemUsage(&vm, 20)
			vm.DiskStatusReason = "agent-timeout"
			switch gap {
			case "retained-high":
				setBackupFilesystemUsage(&vm, 95)
			case "missing-inventory":
				vm.Disks = nil
			case "negative-current":
				vm.DiskStatusReason = ""
				vm.Disk.Usage = -1
			}
			for poll := 0; poll < 3; poll++ {
				elapsed.Add(int64(time.Minute))
				m.CheckGuest(vm, "site")
			}
			if !bytes.Equal(before, guestObservationBytes(t, m)) {
				t.Fatal("unavailable readings changed the open filesystem incident")
			}
			if got := queryAlertEvents(t, m, eventlog.Filter{AlertID: id, Types: []string{eventlog.TypeResolved}}); len(got) != 0 {
				t.Fatal("unavailable readings invented a recovery")
			}
			vm = filesystemRecoveryVM(105)
			setBackupFilesystemUsage(&vm, 20)
			m.CheckGuest(vm, "site")
			m.CheckGuest(vm, "site")
			requireFilesystemRecovery(t, m, original)
		})
	}
}

func TestGuestFilesystemTemporaryUseRollingRecovery(t *testing.T) {
	m, elapsed := filesystemRecoveryManager(t, 0, false)
	cfg := m.GetConfig()
	cfg.MetricEvaluationWindows = map[string]map[string]int{"all": {"disk": 300}}
	m.UpdateConfig(cfg)
	vm := filesystemRecoveryVM(105)
	id := filesystemRecoveryID(vm)
	_, _, key := guestDiskIdentity(vm.Disks[1], 1)
	resourceID := vm.ID + "-disk-" + key
	historyValue, historyState := 95.0, "complete"
	m.SetMetricWindowProvider(func(r MetricWindowRequest) ([]MetricWindowPoint, error) {
		value := 20.0 // The healthy root must not supply the backup series.
		if r.ResourceID == resourceID {
			value = historyValue
			switch historyState {
			case "empty":
				return nil, nil
			case "error":
				return nil, errors.New("synthetic history unavailable")
			case "partial":
				return []MetricWindowPoint{{r.Start, value}}, nil
			}
		}
		return []MetricWindowPoint{{r.Start, value}, {r.Start.Add(2 * time.Minute), value}, {r.Start.Add(4 * time.Minute), value}}, nil
	})
	m.CheckGuest(vm, "site")
	original := activeAlert(t, m, id)
	setBackupFilesystemUsage(&vm, 20)
	elapsed.Add(int64(time.Minute))
	m.CheckGuest(vm, "site")
	held := activeAlert(t, m, id)
	if held.MetricStatus == nil || held.MetricStatus.RawValue == nil || *held.MetricStatus.RawValue != 20 || held.MetricStatus.Value <= 80 {
		t.Fatalf("low current usage must still respect the high rolling average: %+v", held.MetricStatus)
	}
	before := guestObservationBytes(t, m)
	for _, state := range []string{"empty", "error", "partial"} {
		historyState = state
		elapsed.Add(int64(time.Minute))
		m.CheckGuest(vm, "site")
		if !bytes.Equal(before, guestObservationBytes(t, m)) {
			t.Fatalf("%s history invented an evaluated recovery", state)
		}
	}
	historyValue, historyState = 20, "complete"
	elapsed.Add(int64(time.Minute))
	m.CheckGuest(vm, "site")
	m.CheckGuest(vm, "site")
	requireFilesystemRecovery(t, m, original)
}
