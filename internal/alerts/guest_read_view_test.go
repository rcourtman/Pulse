package alerts

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestGuestReadViewPreservesMetrics(t *testing.T) {
	vm := models.VM{
		ID: "site:node:123", VMID: 123, Instance: "site", Node: "node", Name: "database", Status: "running",
		CPU: .375, Memory: models.Memory{Usage: 82}, Disk: models.Disk{Usage: 63},
		DiskRead: 11, DiskWrite: 22, NetworkIn: 33, NetworkOut: 44,
		IORateValidity: models.IORateValidity{Explicit: true, DiskRead: true, NetworkOut: true},
		Disks:          []models.Disk{{Mountpoint: "/data", Usage: 63}}, Tags: []string{"production"},
	}
	want := guestMetrics{
		ID: vm.ID, Name: "database", Node: "node", Status: "running", CPU: 37.5, MemUsage: 82, DiskUsage: 63,
		DiskRead: 11, DiskWrite: 22, NetworkIn: 33, NetworkOut: 44,
		IORateValidity: models.IORateValidity{Explicit: true, DiskRead: true, NetworkOut: true},
	}
	for _, state := range []struct {
		name                      string
		memoryUnavailable         bool
		memoryState, vmDiskReason string
		diskUsage                 float64
	}{
		{name: "legacy", diskUsage: 63},
		{name: "current", memoryState: "current", diskUsage: 63},
		{name: "retained-memory", memoryState: "last-known", diskUsage: 63},
		{name: "unavailable-memory", memoryUnavailable: true, memoryState: "current", diskUsage: 63},
		{name: "unknown-memory-state", memoryState: "future", diskUsage: 63},
		{name: "retained-vm-disk", vmDiskReason: "prev-vm-locked", diskUsage: 63},
		{name: "unavailable-disk", diskUsage: -1},
	} {
		t.Run(state.name, func(t *testing.T) {
			vm.Memory.UsageUnavailable, vm.Memory.Observation.State = state.memoryUnavailable, state.memoryState
			vm.DiskStatusReason, vm.Disk.Usage = state.vmDiskReason, state.diskUsage
			ct := guestRuleContainer(vm)
			snapshot := guestSnapshotFromVM(vm)
			for _, input := range []struct {
				name  string
				guest any
				isLXC bool
			}{{"vm", vm, false}, {"vm-pointer", &vm, false}, {"lxc", ct, true}, {"lxc-pointer", &ct, true}, {"snapshot", snapshot, false}, {"snapshot-pointer", &snapshot, false}} {
				t.Run(input.name, func(t *testing.T) {
					expected := want
					expected.MemoryUnavailable = state.memoryUnavailable || (state.memoryState != "" && state.memoryState != "current")
					expected.DiskUsage = state.diskUsage
					expected.DiskUnavailable = state.diskUsage < 0 || (!input.isLXC && state.vmDiskReason != "")
					got, ok := extractGuestMetrics(input.guest)
					if !ok || !reflect.DeepEqual(got, expected) {
						t.Fatalf("metrics = %+v (supported %v), want %+v", got, ok, expected)
					}
				})
			}
		})
	}
}

func TestGuestSnapshotModelCollectionsStayOwned(t *testing.T) {
	for _, kind := range []string{"vm", "vm-pointer", "lxc", "lxc-pointer", "vm-constructor", "lxc-constructor"} {
		t.Run(kind, func(t *testing.T) {
			vm := guestRuleVM(2)
			ct := guestRuleContainer(vm)
			var snapshot guestSnapshot
			switch kind {
			case "vm":
				snapshot, _ = extractGuestSnapshot(vm)
			case "vm-pointer":
				snapshot, _ = extractGuestSnapshot(&vm)
			case "lxc":
				snapshot, _ = extractGuestSnapshot(ct)
			case "lxc-pointer":
				snapshot, _ = extractGuestSnapshot(&ct)
			case "vm-constructor":
				snapshot = guestSnapshotFromVM(vm)
			case "lxc-constructor":
				snapshot = guestSnapshotFromContainer(ct)
			}
			snapshot.Disks[0].Mountpoint, snapshot.Tags[0] = "/snapshot-only", "snapshot-only"
			if vm.Disks[0].Mountpoint == "/snapshot-only" || vm.Tags[0] == "snapshot-only" {
				t.Fatal("owned snapshot changed the model collections")
			}
			vm.Disks[1].Mountpoint, vm.Tags[1] = "/model-only", "model-only"
			if snapshot.Disks[1].Mountpoint == "/model-only" || snapshot.Tags[1] == "model-only" {
				t.Fatal("model changed an owned snapshot's collections")
			}
		})
	}
}

func TestGuestReadViewEmptyAndUnsupportedInputs(t *testing.T) {
	for _, input := range []any{nil, (*models.VM)(nil), (*models.Container)(nil), (*guestSnapshot)(nil), struct{}{}} {
		view, ok := extractGuestReadView(input)
		if ok || !reflect.DeepEqual(view, emptyGuestSnapshot()) {
			t.Fatalf("unsupported %T returned %+v, %v", input, view, ok)
		}
	}
	for _, input := range []any{models.VM{}, &models.VM{}, models.Container{}, &models.Container{}, guestSnapshot{}, &guestSnapshot{}} {
		view, ok := extractGuestReadView(input)
		if !ok || view.Disks == nil || view.Tags == nil {
			t.Fatalf("empty %T lost supported/normalized collections", input)
		}
	}
}

func TestGuestReadPolicyUsesCurrentTagsWithoutMutatingThem(t *testing.T) {
	vm := guestRuleVM(2)
	ct := guestRuleContainer(vm)
	m := &Manager{config: AlertConfig{GuestDefaults: ThresholdConfig{CPU: &HysteresisThreshold{Trigger: 80, Clear: 75}}}}
	for _, input := range []any{&vm, &ct} {
		for _, state := range []struct {
			tag  string
			want float64
		}{{"production", 80}, {" PULSE-RELAXED ", 95}, {"production", 80}} {
			vm.Tags[0] = state.tag
			beforeDisks := append([]models.Disk(nil), vm.Disks...)
			beforeTags := append([]string(nil), vm.Tags...)
			if got := m.getGuestThresholds(input, vm.ID); got.CPU.Trigger != state.want {
				t.Fatalf("%T CPU trigger = %v, want %v", input, got.CPU.Trigger, state.want)
			}
			if !reflect.DeepEqual(vm.Disks, beforeDisks) || !reflect.DeepEqual(vm.Tags, beforeTags) {
				t.Fatal("policy read mutated borrowed collections")
			}
			if m.config.GuestDefaults.CPU.Trigger != 80 {
				t.Fatal("relaxation mutated saved defaults")
			}
		}
	}
}

var guestReadMetricsResult guestMetrics
var guestReadTagsResult pulseTagSettings

func TestGuestPolicyProjectionDoesNotAllocateInventory(t *testing.T) {
	// The policy needs scalar readings and tag values, not an owned inventory.
	// Measure these projections directly: total threshold-policy allocations
	// include unrelated identity formatting and threshold copies.
	small := guestRuleVM(0)
	large := guestRuleVM(128)
	for i := 0; i < 32; i++ {
		large.Tags = append(large.Tags, fmt.Sprintf("tag-%d", i))
	}
	smallCT, largeCT := guestRuleContainer(small), guestRuleContainer(large)
	for _, input := range []struct {
		name         string
		small, large any
	}{{"vm-value", small, large}, {"vm-pointer", &small, &large}, {"lxc-value", smallCT, largeCT}, {"lxc-pointer", &smallCT, &largeCT}} {
		t.Run(input.name, func(t *testing.T) {
			for _, guest := range []any{input.small, input.large} {
				if got := testing.AllocsPerRun(50, func() { guestReadMetricsResult, _ = extractGuestMetrics(guest) }); got != 0 {
					t.Fatalf("scalar projection allocated %v times", got)
				}
				if got := testing.AllocsPerRun(50, func() {
					view, _ := extractGuestReadView(guest)
					guestReadTagsResult = parsePulseTags(view.Tags)
				}); got != 0 {
					t.Fatalf("lowercase-tag projection allocated %v times", got)
				}
			}
		})
	}
}
