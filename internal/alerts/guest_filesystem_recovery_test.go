package alerts

import (
	"bytes"
	"encoding/json"
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

func filesystemRecoveryActiveAlert(t *testing.T, m *Manager, id string) *Alert {
	t.Helper()
	for _, alert := range m.GetActiveAlerts() {
		if alert.ID == id {
			return alert.Clone()
		}
	}
	t.Fatalf("expected active filesystem alert %q", id)
	return nil
}

func requireFilesystemRecovery(t *testing.T, m *Manager, original *Alert) {
	t.Helper()
	// The lookup helper owns its lock; an outer write lock would deadlock it.
	if testHasActiveAlert(t, m, original.ID) {
		t.Fatal("fresh at-or-below-clear usage left the original filesystem incident active")
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
				original, peerOriginal := filesystemRecoveryActiveAlert(t, m, id), filesystemRecoveryActiveAlert(t, m, peerID)
				if acknowledged {
					if err := m.AcknowledgeAlert(id, "test-operator"); err != nil {
						t.Fatal(err)
					}
				}
				vm.Disks[0], vm.Disks[1] = vm.Disks[1], vm.Disks[0]
				setBackupFilesystemUsage(&vm, 85) // Below trigger, but above clear.
				elapsed.Add(int64(time.Minute))
				m.CheckGuest(vm, "site")
				held := filesystemRecoveryActiveAlert(t, m, id)
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
				if got := filesystemRecoveryActiveAlert(t, m, peerID); got.StartTime != peerOriginal.StartTime || got.Value != peerOriginal.Value {
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
			original := filesystemRecoveryActiveAlert(t, m, id)
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

func TestGuestFilesystemTemporaryUseRejectsCapacityWindow(t *testing.T) {
	m, _ := filesystemRecoveryManager(t, 0, false)
	cfg := m.GetConfig()
	cfg.MetricEvaluationWindows = map[string]map[string]int{"all": {"disk": 300}, "vm": {"disk": 300}}
	m.UpdateConfig(cfg)
	for resourceType, metrics := range m.GetConfig().MetricEvaluationWindows {
		if _, present := metrics["disk"]; present {
			t.Fatalf("unsupported capacity window survived configuration normalization for %s", resourceType)
		}
	}
	vm := filesystemRecoveryVM(105)
	id := filesystemRecoveryID(vm)
	var diskHistoryCalls atomic.Int32
	m.SetMetricWindowProvider(func(r MetricWindowRequest) ([]MetricWindowPoint, error) {
		if r.Metric != "disk" {
			return nil, nil
		}
		diskHistoryCalls.Add(1)
		// If the rejected rule were admitted, this old high history would
		// incorrectly hold the incident after the temporary file is gone.
		return []MetricWindowPoint{{r.Start, 95}, {r.Start.Add(2 * time.Minute), 95}, {r.Start.Add(4 * time.Minute), 95}}, nil
	})
	m.CheckGuest(vm, "site")
	original := filesystemRecoveryActiveAlert(t, m, id)
	if original.MetricStatus == nil || original.MetricStatus.EvaluationWindowSeconds != 0 || original.MetricStatus.RawValue != nil {
		t.Fatalf("ordinary disk capacity was presented as an average: %+v", original.MetricStatus)
	}
	setBackupFilesystemUsage(&vm, 20)
	m.CheckGuest(vm, "site")
	m.CheckGuest(vm, "site")
	requireFilesystemRecovery(t, m, original)
	if diskHistoryCalls.Load() != 0 {
		t.Fatal("ordinary filesystem capacity consulted a rejected rolling-window provider")
	}
}
