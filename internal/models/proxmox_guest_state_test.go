package models

import (
	"cmp"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestUpdateGuestsForInstancePreservesScopedBackupEvidence(t *testing.T) {
	at := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	previousVM := VM{ID: "site:old:102", VMID: 102, Instance: "site", Node: "old", LastBackup: at, BackupInProgress: true}
	previousCT := Container{ID: "site:old:102", VMID: 102, Instance: "site", Node: "old", LastBackup: at.Add(-time.Hour)}
	peerVM := VM{ID: "peer:old:102", VMID: 102, Instance: "peer", Node: "old", LastBackup: at.Add(-2 * time.Hour)}
	peerCT := Container{ID: "peer:old:102", VMID: 102, Instance: "peer", Node: "old", LastBackup: at.Add(-3 * time.Hour), BackupInProgress: true}

	s := NewState()
	s.UpdateGuestsForInstance("site", []VM{
		previousVM,
		{ID: "site:old:103", VMID: 103, Instance: "site", Node: "old", LastBackup: at, BackupInProgress: true},
		{ID: "site:old:108", VMID: 108, Instance: "site", Node: "old"},
	}, []Container{
		previousCT,
		{ID: "site:old:110", VMID: 110, Instance: "site", Node: "old"},
	})
	s.UpdateGuestsForInstance("peer", []VM{peerVM}, []Container{peerCT})

	movedVM := VM{ID: "site:new:102", VMID: 102, Instance: "site", Node: "new", Name: "vm-current", Status: "running", LastSeen: at,
		Disks: []Disk{{Used: 20, Total: 100}}, NetworkInterfaces: []GuestNetworkInterface{{Name: "eth0", Addresses: []string{"192.0.2.2"}}}}
	movedCT := Container{ID: "site:new:102", VMID: 102, Instance: "site", Node: "new", Name: "ct-current", Status: "running", LastSeen: at,
		Disks: []Disk{{Used: 30, Total: 100}}, NetworkInterfaces: []GuestNetworkInterface{{Name: "eth0", Addresses: []string{"192.0.2.3"}}}}
	observedVM := VM{ID: "site:old:103", VMID: 103, Instance: "site", Node: "old", LastBackup: at.Add(time.Minute)}
	newCT := Container{ID: "site:new:101", VMID: 101, Instance: "site", Node: "new"}
	s.UpdateGuestsForInstance("site", []VM{
		observedVM,
		{ID: movedVM.ID, VMID: movedVM.VMID, Instance: "site", Node: "new", Name: "superseded"},
		movedVM,
	}, []Container{movedCT, newCT})

	wantVM, wantCT := movedVM, movedCT
	wantVM.LastBackup, wantVM.BackupInProgress = previousVM.LastBackup, previousVM.BackupInProgress
	wantCT.LastBackup, wantCT.BackupInProgress = previousCT.LastBackup, previousCT.BackupInProgress
	published := s.GetSnapshot()
	require.True(t, slices.IsSortedFunc(published.VMs, func(a, b VM) int { return cmp.Compare(a.VMID, b.VMID) }))
	require.True(t, slices.IsSortedFunc(published.Containers, func(a, b Container) int { return cmp.Compare(a.VMID, b.VMID) }))
	require.ElementsMatch(t, []VM{wantVM.NormalizeCollections(), observedVM.NormalizeCollections(), peerVM.NormalizeCollections()}, published.VMs)
	require.ElementsMatch(t, []Container{wantCT.NormalizeCollections(), newCT.NormalizeCollections(), peerCT.NormalizeCollections()}, published.Containers)

	// Caller-owned disk and address slices must not alias the published state.
	movedVM.Disks[0].Used = 99
	movedVM.NetworkInterfaces[0].Addresses[0] = "192.0.2.99"
	movedCT.Disks[0].Used = 99
	movedCT.NetworkInterfaces[0].Addresses[0] = "192.0.2.99"
	require.Equal(t, published, s.GetSnapshot())

	s.UpdateGuestsForInstance("site", nil, nil)
	remaining := s.GetSnapshot()
	require.Equal(t, []VM{peerVM.NormalizeCollections()}, remaining.VMs)
	require.Equal(t, []Container{peerCT.NormalizeCollections()}, remaining.Containers)
}
