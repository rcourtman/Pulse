package models

import (
	"reflect"
	"testing"
	"time"
)

func TestGuestObservationsForNodesOwnsOnlyAcceptedScopedTuples(t *testing.T) {
	s := NewState()
	at := time.Date(2026, 10, 14, 10, 0, 0, 0, time.UTC)
	onBoot := true
	vm := VM{ID: "site:member:105", Instance: "site", Node: "member", VMID: 105, CPU: .95, LastSeen: at, OnBoot: &onBoot,
		Memory: Memory{Used: 40, Total: 100, Usage: 40, Observation: MemoryObservation{State: "last-known", Source: "guest-agent-meminfo", ObservedAt: at.Add(-time.Minute)}},
		Disk:   Disk{Used: 20, Total: 100, Usage: 20}, Disks: []Disk{{Used: 20, Total: 100, Mountpoint: "C:\\"}}, IPAddresses: []string{"192.0.2.1"}, Tags: []string{"production"}, NetworkInterfaces: []GuestNetworkInterface{{Name: "Ethernet", Addresses: []string{"192.0.2.1"}}}}
	ct := Container{ID: "site:member:205", Instance: "site", Node: "member", VMID: 205, CPU: .25, LastSeen: at, OnBoot: &onBoot, Memory: vm.Memory, Disk: vm.Disk, Disks: vm.Disks, Tags: vm.Tags, IPAddresses: vm.IPAddresses, NetworkInterfaces: vm.NetworkInterfaces}
	s.UpdateGuestsForInstance("site", []VM{vm, {ID: "site:other:106", Instance: "site", Node: "other", VMID: 106}}, []Container{ct})
	s.UpdateGuestsForInstance("peer", []VM{{ID: "peer:member:105", Instance: "peer", Node: "member", VMID: 105}}, []Container{{ID: "peer:member:205", Instance: "peer", Node: "member", VMID: 205}})
	before := s.GetSnapshot()
	gotVMs, gotCTs := s.GuestObservationsForNodes("site", map[string]struct{}{"member": {}})
	if len(gotVMs) != 1 || len(gotCTs) != 1 || gotVMs[0].CPU != .95 || !reflect.DeepEqual(gotVMs[0].Memory, vm.Memory) || gotCTs[0].Memory.Usage != 40 || !gotVMs[0].LastSeen.Equal(at) {
		t.Fatalf("source tuple or scope lost: %+v %+v", gotVMs, gotCTs)
	}
	gotVMs[0].Disks[0].Used = 99
	gotVMs[0].Tags[0] = "changed"
	gotVMs[0].IPAddresses[0] = "192.0.2.99"
	gotVMs[0].NetworkInterfaces[0].Addresses[0] = "192.0.2.99"
	*gotVMs[0].OnBoot = false
	gotCTs[0].Disks[0].Used = 99
	gotCTs[0].Tags[0] = "changed"
	gotCTs[0].NetworkInterfaces[0].Addresses[0] = "192.0.2.99"
	*gotCTs[0].OnBoot = false
	if !reflect.DeepEqual(before, s.GetSnapshot()) {
		t.Fatal("retained tuples alias accepted source state")
	}
	for _, nodes := range []map[string]struct{}{nil, {"absent": {}}} {
		v, c := s.GuestObservationsForNodes("site", nodes)
		if len(v)+len(c) != 0 {
			t.Fatal("unselected nodes returned observations")
		}
	}
	var missing *State
	v, c := missing.GuestObservationsForNodes("site", map[string]struct{}{"member": {}})
	if len(v)+len(c) != 0 {
		t.Fatal("missing state manufactured observations")
	}
}
