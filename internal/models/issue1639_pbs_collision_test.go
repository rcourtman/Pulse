package models

import (
	"testing"
	"time"
)

// Issue #1639: two PVE clusters with the same VMID, root-namespace PBS
// snapshots, and no comment left both guests with zero LastBackup because
// the ambiguity guard disabled the VMID-only fallback and the scorer never
// consulted the snapshots' provenance (owner token, datastore, PBS
// instance) or the PVE-side storage listings.

// TestIssue1639PBSCollisionResolvedBySubmissionSource verifies that when
// each cluster's submission source is learnable from its other,
// non-colliding guests, root-namespace comment-less snapshots for a
// colliding VMID are attributed to the correct cluster.
func TestIssue1639PBSCollisionResolvedBySubmissionSource(t *testing.T) {
	state := NewState()

	now := time.Now()
	backupTimeA := now.Add(-2 * time.Hour)
	backupTimeB := now.Add(-3 * time.Hour)
	uniqueTimeA := now.Add(-26 * time.Hour)
	uniqueTimeB := now.Add(-27 * time.Hour)

	state.UpdateVMs([]VM{
		// VMID 173 exists on both clusters — the reported collision.
		{VMID: 173, Name: "web-a", Instance: "cluster-a", Node: "pve-a1"},
		{VMID: 173, Name: "web-b", Instance: "cluster-b", Node: "pve-b1"},
		// Each cluster also has a unique VMID whose snapshots teach the
		// learner that cluster's owner token and datastore.
		{VMID: 100, Name: "db-a", Instance: "cluster-a", Node: "pve-a1"},
		{VMID: 150, Name: "db-b", Instance: "cluster-b", Node: "pve-b1"},
	})

	state.mu.Lock()
	state.PBSBackups = []PBSBackup{
		// Teaching snapshots: unique VMIDs, root namespace, no comment.
		{ID: "a-100", VMID: "100", BackupType: "vm", BackupTime: uniqueTimeA,
			Instance: "pbs-main", Datastore: "store-a", Owner: "cluster-a@pbs!token"},
		{ID: "b-150", VMID: "150", BackupType: "vm", BackupTime: uniqueTimeB,
			Instance: "pbs-main", Datastore: "store-b", Owner: "cluster-b@pbs!token"},
		// Colliding VMID with no namespace/comment evidence, distinct
		// owner and datastore per cluster.
		{ID: "a-173", VMID: "173", BackupType: "vm", BackupTime: backupTimeA,
			Instance: "pbs-main", Datastore: "store-a", Owner: "cluster-a@pbs!token"},
		{ID: "b-173", VMID: "173", BackupType: "vm", BackupTime: backupTimeB,
			Instance: "pbs-main", Datastore: "store-b", Owner: "cluster-b@pbs!token"},
	}
	state.mu.Unlock()

	state.SyncGuestBackupTimes()
	snapshot := state.GetSnapshot()

	got := map[string]time.Time{}
	for _, vm := range snapshot.VMs {
		if vm.VMID == 173 {
			got[vm.Instance] = vm.LastBackup
		}
	}
	if !got["cluster-a"].Equal(backupTimeA) {
		t.Errorf("cluster-a VM 173 LastBackup = %v, want its own snapshot %v", got["cluster-a"], backupTimeA)
	}
	if !got["cluster-b"].Equal(backupTimeB) {
		t.Errorf("cluster-b VM 173 LastBackup = %v, want its own snapshot %v", got["cluster-b"], backupTimeB)
	}
}

// TestIssue1639PBSCollisionResolvedByDistinctPBSInstance verifies the
// weakest source component: each cluster pushes to its own PBS instance,
// same datastore name, no owner reported.
func TestIssue1639PBSCollisionResolvedByDistinctPBSInstance(t *testing.T) {
	state := NewState()

	now := time.Now()
	backupTimeA := now.Add(-2 * time.Hour)
	backupTimeB := now.Add(-3 * time.Hour)

	state.UpdateVMs([]VM{
		{VMID: 173, Name: "web-a", Instance: "cluster-a", Node: "pve-a1"},
		{VMID: 173, Name: "web-b", Instance: "cluster-b", Node: "pve-b1"},
		{VMID: 100, Name: "db-a", Instance: "cluster-a", Node: "pve-a1"},
		{VMID: 150, Name: "db-b", Instance: "cluster-b", Node: "pve-b1"},
	})

	state.mu.Lock()
	state.PBSBackups = []PBSBackup{
		{ID: "a-100", VMID: "100", BackupType: "vm", BackupTime: now.Add(-26 * time.Hour), Instance: "pbs-one", Datastore: "backups"},
		{ID: "b-150", VMID: "150", BackupType: "vm", BackupTime: now.Add(-27 * time.Hour), Instance: "pbs-two", Datastore: "backups"},
		{ID: "a-173", VMID: "173", BackupType: "vm", BackupTime: backupTimeA, Instance: "pbs-one", Datastore: "backups"},
		{ID: "b-173", VMID: "173", BackupType: "vm", BackupTime: backupTimeB, Instance: "pbs-two", Datastore: "backups"},
	}
	state.mu.Unlock()

	state.SyncGuestBackupTimes()
	snapshot := state.GetSnapshot()

	for _, vm := range snapshot.VMs {
		switch {
		case vm.VMID == 173 && vm.Instance == "cluster-a":
			if !vm.LastBackup.Equal(backupTimeA) {
				t.Errorf("cluster-a VM 173 LastBackup = %v, want %v", vm.LastBackup, backupTimeA)
			}
		case vm.VMID == 173 && vm.Instance == "cluster-b":
			if !vm.LastBackup.Equal(backupTimeB) {
				t.Errorf("cluster-b VM 173 LastBackup = %v, want %v", vm.LastBackup, backupTimeB)
			}
		}
	}
}

// TestIssue1639PBSCollisionSharedSourceStaysDropped verifies the guard is
// not weakened: when both clusters push through the same owner, datastore,
// and PBS instance and no PVE-side evidence exists, the snapshots stay
// unattributed rather than being guessed onto a cluster.
func TestIssue1639PBSCollisionSharedSourceStaysDropped(t *testing.T) {
	state := NewState()

	now := time.Now()

	state.UpdateVMs([]VM{
		{VMID: 173, Name: "web-a", Instance: "cluster-a", Node: "pve-a1"},
		{VMID: 173, Name: "web-b", Instance: "cluster-b", Node: "pve-b1"},
		{VMID: 100, Name: "db-a", Instance: "cluster-a", Node: "pve-a1"},
		{VMID: 150, Name: "db-b", Instance: "cluster-b", Node: "pve-b1"},
	})

	state.mu.Lock()
	state.PBSBackups = []PBSBackup{
		// Both clusters share one submission source, so nothing about the
		// colliding snapshots discriminates between them.
		{ID: "a-100", VMID: "100", BackupType: "vm", BackupTime: now.Add(-26 * time.Hour), Instance: "pbs-main", Datastore: "backups", Owner: "shared@pbs!token"},
		{ID: "b-150", VMID: "150", BackupType: "vm", BackupTime: now.Add(-27 * time.Hour), Instance: "pbs-main", Datastore: "backups", Owner: "shared@pbs!token"},
		{ID: "x-173", VMID: "173", BackupType: "vm", BackupTime: now.Add(-2 * time.Hour), Instance: "pbs-main", Datastore: "backups", Owner: "shared@pbs!token"},
	}
	state.mu.Unlock()

	state.SyncGuestBackupTimes()
	snapshot := state.GetSnapshot()

	for _, vm := range snapshot.VMs {
		if vm.VMID == 173 && !vm.LastBackup.IsZero() {
			t.Errorf("VM 173 on %s should stay unattributed with a shared submission source, got %v", vm.Instance, vm.LastBackup)
		}
	}
}

// TestIssue1639PBSCollisionResolvedByPVEStorageConfirmation verifies the
// storage-view path: even with identical submission sources (mirrored
// clusters pushing with one token), each PVE connection's own pbs-type
// storage listing attributes its snapshots — as long as the two listings are
// disjoint, i.e. each cluster mounts only its own datastore, so seeing a
// snapshot there really does mean authoring it.
func TestIssue1639PBSCollisionResolvedByPVEStorageConfirmation(t *testing.T) {
	state := NewState()

	now := time.Now()
	backupTimeA := now.Add(-2 * time.Hour).Truncate(time.Second)
	backupTimeB := now.Add(-3 * time.Hour).Truncate(time.Second)

	state.UpdateVMs([]VM{
		{VMID: 173, Name: "web", Instance: "cluster-a", Node: "pve-a1"},
		{VMID: 173, Name: "web", Instance: "cluster-b", Node: "pve-b1"},
	})

	state.mu.Lock()
	state.PBSBackups = []PBSBackup{
		{ID: "a-173", VMID: "173", BackupType: "vm", BackupTime: backupTimeA, Instance: "pbs-main", Datastore: "backups", Owner: "shared@pbs!token"},
		{ID: "b-173", VMID: "173", BackupType: "vm", BackupTime: backupTimeB, Instance: "pbs-main", Datastore: "backups", Owner: "shared@pbs!token"},
	}
	state.mu.Unlock()

	state.UpdatePBSGuestConfirmationsForInstance("cluster-a", []PBSGuestConfirmation{
		{Storage: "pbs-a", BackupType: "vm", VMID: 173, Time: backupTimeA.Unix()},
	})
	state.UpdatePBSGuestConfirmationsForInstance("cluster-b", []PBSGuestConfirmation{
		{Storage: "pbs-b", BackupType: "vm", VMID: 173, Time: backupTimeB.Unix()},
	})

	state.SyncGuestBackupTimes()
	snapshot := state.GetSnapshot()

	for _, vm := range snapshot.VMs {
		switch vm.Instance {
		case "cluster-a":
			if !vm.LastBackup.Equal(backupTimeA) {
				t.Errorf("cluster-a VM 173 LastBackup = %v, want confirmed snapshot %v", vm.LastBackup, backupTimeA)
			}
		case "cluster-b":
			if !vm.LastBackup.Equal(backupTimeB) {
				t.Errorf("cluster-b VM 173 LastBackup = %v, want confirmed snapshot %v", vm.LastBackup, backupTimeB)
			}
		}
	}
}

// TestIssue1639PBSCollisionForeignSourceRejected verifies negative
// attribution: a snapshot whose source decisively belongs to another
// cluster must not reach this cluster's guest even though the guest has no
// snapshot of its own.
func TestIssue1639PBSCollisionForeignSourceRejected(t *testing.T) {
	state := NewState()

	now := time.Now()

	state.UpdateVMs([]VM{
		{VMID: 173, Name: "web-a", Instance: "cluster-a", Node: "pve-a1"},
		{VMID: 173, Name: "web-b", Instance: "cluster-b", Node: "pve-b1"},
		{VMID: 100, Name: "db-a", Instance: "cluster-a", Node: "pve-a1"},
		{VMID: 150, Name: "db-b", Instance: "cluster-b", Node: "pve-b1"},
	})

	state.mu.Lock()
	state.PBSBackups = []PBSBackup{
		// Both clusters are visible with a source of their own, so the
		// singleton owner token really is cluster-a's. The only 173 snapshot
		// belongs to cluster-a and cluster-b must stay at zero.
		{ID: "a-100", VMID: "100", BackupType: "vm", BackupTime: now.Add(-26 * time.Hour), Instance: "pbs-main", Datastore: "store-a", Owner: "cluster-a@pbs!token"},
		{ID: "b-150", VMID: "150", BackupType: "vm", BackupTime: now.Add(-27 * time.Hour), Instance: "pbs-main", Datastore: "store-b", Owner: "cluster-b@pbs!token"},
		{ID: "a-173", VMID: "173", BackupType: "vm", BackupTime: now.Add(-2 * time.Hour), Instance: "pbs-main", Datastore: "store-a", Owner: "cluster-a@pbs!token"},
	}
	state.mu.Unlock()

	state.SyncGuestBackupTimes()
	snapshot := state.GetSnapshot()

	for _, vm := range snapshot.VMs {
		if vm.VMID != 173 {
			continue
		}
		switch vm.Instance {
		case "cluster-a":
			if vm.LastBackup.IsZero() {
				t.Error("cluster-a VM 173 should get its own snapshot")
			}
		case "cluster-b":
			if !vm.LastBackup.IsZero() {
				t.Errorf("cluster-b VM 173 must not inherit cluster-a's snapshot, got %v", vm.LastBackup)
			}
		}
	}
}

// TestIssue1639PBSCollisionInvisibleClusterKeepsSourceInconclusive is the
// asymmetric case the first #1639 fix got wrong. Clusters only become known
// to the submission-source learner through snapshots that were already
// attributable, so cluster-b — sharing cluster-a's owner token and owning no
// uniquely-attributable guest — is invisible. The shared token then looks
// like cluster-a's alone and cluster-a is handed cluster-b's snapshot.
// Without further evidence both guests must stay unattributed.
func TestIssue1639PBSCollisionInvisibleClusterKeepsSourceInconclusive(t *testing.T) {
	state := NewState()

	now := time.Now()
	backupTimeB := now.Add(-2 * time.Hour).Truncate(time.Second)

	state.UpdateVMs([]VM{
		{VMID: 173, Name: "web-a", Instance: "cluster-a", Node: "pve-a1"},
		{VMID: 173, Name: "web-b", Instance: "cluster-b", Node: "pve-b1"},
		// Only cluster-a owns a guest whose snapshots are attributable on
		// their own; cluster-b's single guest is the colliding one.
		{VMID: 100, Name: "db-a", Instance: "cluster-a", Node: "pve-a1"},
	})

	state.mu.Lock()
	state.PBSBackups = []PBSBackup{
		{ID: "a-100", VMID: "100", BackupType: "vm", BackupTime: now.Add(-26 * time.Hour),
			Instance: "pbs-main", Datastore: "backups", Owner: "shared@pbs!token"},
		// Authored by cluster-b, indistinguishable from cluster-a's own
		// submissions because the token and datastore are shared.
		{ID: "b-173", VMID: "173", BackupType: "vm", BackupTime: backupTimeB,
			Instance: "pbs-main", Datastore: "backups", Owner: "shared@pbs!token"},
	}
	state.mu.Unlock()

	state.SyncGuestBackupTimes()

	for _, vm := range state.GetSnapshot().VMs {
		if vm.VMID != 173 {
			continue
		}
		if !vm.LastBackup.IsZero() {
			t.Errorf("VM 173 on %s = %v, want zero while cluster-b has no attributable evidence", vm.Instance, vm.LastBackup)
		}
	}
}

// TestIssue1639PBSCollisionInvisibleClusterResolvedByOwnStorageView is the
// same topology as above with the one piece of evidence that settles it:
// cluster-b's own pbs-type storage lists the snapshot, and no other
// connection's listing overlaps that storage. The snapshot goes to
// cluster-b, and cluster-a still does not get it.
func TestIssue1639PBSCollisionInvisibleClusterResolvedByOwnStorageView(t *testing.T) {
	state := NewState()

	now := time.Now()
	uniqueTimeA := now.Add(-26 * time.Hour).Truncate(time.Second)
	backupTimeB := now.Add(-2 * time.Hour).Truncate(time.Second)

	state.UpdateVMs([]VM{
		{VMID: 173, Name: "web-a", Instance: "cluster-a", Node: "pve-a1"},
		{VMID: 173, Name: "web-b", Instance: "cluster-b", Node: "pve-b1"},
		{VMID: 100, Name: "db-a", Instance: "cluster-a", Node: "pve-a1"},
	})

	state.mu.Lock()
	state.PBSBackups = []PBSBackup{
		{ID: "a-100", VMID: "100", BackupType: "vm", BackupTime: uniqueTimeA,
			Instance: "pbs-main", Datastore: "backups", Owner: "shared@pbs!token"},
		{ID: "b-173", VMID: "173", BackupType: "vm", BackupTime: backupTimeB,
			Instance: "pbs-main", Datastore: "backups", Owner: "shared@pbs!token"},
	}
	state.mu.Unlock()

	state.UpdatePBSGuestConfirmationsForInstance("cluster-a", []PBSGuestConfirmation{
		{Storage: "pbs-a", BackupType: "vm", VMID: 100, Time: uniqueTimeA.Unix()},
	})
	state.UpdatePBSGuestConfirmationsForInstance("cluster-b", []PBSGuestConfirmation{
		{Storage: "pbs-b", BackupType: "vm", VMID: 173, Time: backupTimeB.Unix()},
	})

	state.SyncGuestBackupTimes()

	for _, vm := range state.GetSnapshot().VMs {
		if vm.VMID != 173 {
			continue
		}
		switch vm.Instance {
		case "cluster-a":
			if !vm.LastBackup.IsZero() {
				t.Errorf("cluster-a VM 173 = %v, want zero (the snapshot is cluster-b's)", vm.LastBackup)
			}
		case "cluster-b":
			if !vm.LastBackup.Equal(backupTimeB) {
				t.Errorf("cluster-b VM 173 = %v, want its own confirmed snapshot %v", vm.LastBackup, backupTimeB)
			}
		}
	}
}

// TestIssue1639PBSStorageConfirmationFromSharedViewIsNotAuthorship covers the
// second half of the confirmation weakening: a connection whose storage view
// also lists snapshots another connection lists is looking at a shared,
// synced, or offsite datastore. Seeing a snapshot there proves visibility,
// not authorship, so it must not attribute a colliding VMID on its own.
func TestIssue1639PBSStorageConfirmationFromSharedViewIsNotAuthorship(t *testing.T) {
	state := NewState()

	now := time.Now()
	sharedTime := now.Add(-25 * time.Hour).Truncate(time.Second)
	disputedTime := now.Add(-2 * time.Hour).Truncate(time.Second)

	state.UpdateVMs([]VM{
		{VMID: 173, Name: "web-a", Instance: "cluster-a", Node: "pve-a1"},
		{VMID: 173, Name: "web-b", Instance: "cluster-b", Node: "pve-b1"},
		{VMID: 200, Name: "shared-guest", Instance: "cluster-b", Node: "pve-b1"},
	})

	state.mu.Lock()
	state.PBSBackups = []PBSBackup{
		{ID: "b-200", VMID: "200", BackupType: "vm", BackupTime: sharedTime,
			Instance: "pbs-main", Datastore: "backups", Owner: "shared@pbs!token"},
		{ID: "x-173", VMID: "173", BackupType: "vm", BackupTime: disputedTime,
			Instance: "pbs-main", Datastore: "backups", Owner: "shared@pbs!token"},
	}
	state.mu.Unlock()

	// Both connections mount the same datastore: their listings overlap on
	// VM 200, which is cluster-b's guest. cluster-a additionally lists the
	// disputed 173 snapshot, but its view has already proven it can see
	// other clusters' snapshots.
	state.UpdatePBSGuestConfirmationsForInstance("cluster-a", []PBSGuestConfirmation{
		{Storage: "pbs-shared", BackupType: "vm", VMID: 200, Time: sharedTime.Unix()},
		{Storage: "pbs-shared", BackupType: "vm", VMID: 173, Time: disputedTime.Unix()},
	})
	state.UpdatePBSGuestConfirmationsForInstance("cluster-b", []PBSGuestConfirmation{
		{Storage: "pbs-shared", BackupType: "vm", VMID: 200, Time: sharedTime.Unix()},
	})

	state.SyncGuestBackupTimes()

	for _, vm := range state.GetSnapshot().VMs {
		if vm.VMID != 173 {
			continue
		}
		if !vm.LastBackup.IsZero() {
			t.Errorf("VM 173 on %s = %v, want zero: a shared storage view is visibility, not authorship", vm.Instance, vm.LastBackup)
		}
	}
}
