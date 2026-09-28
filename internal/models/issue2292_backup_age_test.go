package models

import (
	"testing"
	"time"
)

// A unique typed VMID permits the root-namespace fallback. Once attributed,
// the badge must reflect the newest completed snapshot, even if an older
// snapshot has a stronger namespace/comment match (#2292).
func TestSyncGuestBackupTimesNewestAttributablePBSBackup(t *testing.T) {
	now := time.Date(2026, time.September, 28, 0, 0, 0, 0, time.UTC)
	old := now.Add(-65 * 24 * time.Hour)
	recent := now.Add(-24 * time.Hour)

	for _, backupType := range []string{"vm", "ct"} {
		t.Run(backupType, func(t *testing.T) {
			state := NewState()
			otherType := "vm"
			if backupType == "vm" {
				otherType = "ct"
				state.UpdateVMs([]VM{{VMID: 112, Name: "guest-a", Instance: "cluster-a", Node: "node-a"}})
			} else {
				state.UpdateContainers([]Container{{VMID: 112, Name: "guest-a", Instance: "cluster-a", Node: "node-a"}})
			}

			state.mu.Lock()
			state.PBSBackups = []PBSBackup{
				{ID: "old", VMID: "112", BackupType: backupType, BackupTime: old,
					Instance: "pbs-main", Namespace: "node-a", Comment: "guest-a"},
				// A unique typed VMID permits root-namespace fallback (score 1).
				{ID: "recent", VMID: "112", BackupType: backupType, BackupTime: recent,
					Instance: "pbs-main"},
				// A later snapshot for the other subject type is not this guest's.
				{ID: "other-type", VMID: "112", BackupType: otherType, BackupTime: now.Add(-time.Hour),
					Instance: "pbs-main", Namespace: "node-a"},
			}
			state.mu.Unlock()

			state.SyncGuestBackupTimes()
			snapshot := state.GetSnapshot()
			var got time.Time
			if backupType == "vm" {
				got = snapshot.VMs[0].LastBackup
			} else {
				got = snapshot.Containers[0].LastBackup
			}
			if !got.Equal(recent) {
				t.Errorf("LastBackup = %v, want newer attributable backup %v", got, recent)
			}
		})
	}
}

// With a colliding typed VMID, the newer snapshot must be distinguishable
// from the other PVE connection; a newer snapshot for that other connection
// must not advance this guest's badge. VM and CT subjects remain separate.
func TestSyncGuestBackupTimesNewestPBSBackupPreservesCollisionGuard(t *testing.T) {
	now := time.Date(2026, time.September, 28, 0, 0, 0, 0, time.UTC)
	oldA := now.Add(-27 * 24 * time.Hour)
	recentA := now.Add(-24 * time.Hour)
	recentB := now.Add(-2 * time.Hour)

	for _, backupType := range []string{"vm", "ct"} {
		t.Run(backupType, func(t *testing.T) {
			state := NewState()
			if backupType == "vm" {
				state.UpdateVMs([]VM{
					{VMID: 112, Name: "guest-a", Instance: "cluster-a", Node: "node-a"},
					{VMID: 112, Name: "guest-b", Instance: "cluster-b", Node: "node-b"},
				})
			} else {
				state.UpdateContainers([]Container{
					{VMID: 112, Name: "guest-a", Instance: "cluster-a", Node: "node-a"},
					{VMID: 112, Name: "guest-b", Instance: "cluster-b", Node: "node-b"},
				})
			}

			state.mu.Lock()
			state.PBSBackups = []PBSBackup{
				{ID: "old-a", VMID: "112", BackupType: backupType, BackupTime: oldA,
					Instance: "pbs-main", Namespace: "node-a", Comment: "guest-a"},
				// The connection namespace is weaker than the actual node's, but
				// still positively identifies A despite the VMID collision.
				{ID: "recent-a", VMID: "112", BackupType: backupType, BackupTime: recentA,
					Instance: "pbs-main", Namespace: "cluster-a"},
				{ID: "recent-b", VMID: "112", BackupType: backupType, BackupTime: recentB,
					Instance: "pbs-main", Namespace: "node-b", Comment: "guest-b"},
				// This newest root-namespace snapshot has no source evidence to
				// distinguish the two PVE connections and must be ignored.
				{ID: "unattributable", VMID: "112", BackupType: backupType, BackupTime: now.Add(-time.Hour),
					Instance: "pbs-unattributed"},
			}
			state.mu.Unlock()

			state.SyncGuestBackupTimes()
			snapshot := state.GetSnapshot()
			got := make(map[string]time.Time)
			if backupType == "vm" {
				for _, guest := range snapshot.VMs {
					got[guest.Instance] = guest.LastBackup
				}
			} else {
				for _, guest := range snapshot.Containers {
					got[guest.Instance] = guest.LastBackup
				}
			}
			if !got["cluster-a"].Equal(recentA) {
				t.Errorf("cluster-a LastBackup = %v, want its newer, weaker match %v", got["cluster-a"], recentA)
			}
			if !got["cluster-b"].Equal(recentB) {
				t.Errorf("cluster-b LastBackup = %v, want its own snapshot %v", got["cluster-b"], recentB)
			}
		})
	}
}

// Two independent clusters can have the same VMID and node label. In that
// case, a node namespace alone is positive for both guests, but a matching
// guest name makes the backup specific to one. Recency must be considered
// only after deciding which guest each snapshot can actually identify.
func TestSyncGuestBackupTimesNewestPBSBackupSameNodeCollision(t *testing.T) {
	now := time.Date(2026, time.September, 28, 0, 0, 0, 0, time.UTC)
	oldA := now.Add(-65 * 24 * time.Hour)
	recentA := now.Add(-24 * time.Hour)
	recentB := now.Add(-2 * time.Hour)

	for _, backupType := range []string{"vm", "ct"} {
		t.Run(backupType, func(t *testing.T) {
			state := NewState()
			if backupType == "vm" {
				state.UpdateVMs([]VM{
					{VMID: 112, Name: "guest-a", Instance: "cluster-a", Node: "pve"},
					{VMID: 112, Name: "guest-b", Instance: "cluster-b", Node: "pve"},
				})
			} else {
				state.UpdateContainers([]Container{
					{VMID: 112, Name: "guest-a", Instance: "cluster-a", Node: "pve"},
					{VMID: 112, Name: "guest-b", Instance: "cluster-b", Node: "pve"},
				})
			}

			state.mu.Lock()
			state.PBSBackups = []PBSBackup{
				{ID: "old-a", VMID: "112", BackupType: backupType, BackupTime: oldA,
					Instance: "pbs-main", Namespace: "pve", Comment: "guest-a"},
				// Weaker than old-a, but only cluster-a matches this namespace.
				{ID: "recent-a", VMID: "112", BackupType: backupType, BackupTime: recentA,
					Instance: "pbs-main", Namespace: "cluster-a"},
				// Both guests match node pve, but guest-b is the stronger match.
				{ID: "recent-b", VMID: "112", BackupType: backupType, BackupTime: recentB,
					Instance: "pbs-main", Namespace: "pve", Comment: "guest-b"},
				// Neither guest owns an otherwise indistinguishable newer copy.
				{ID: "shared-node", VMID: "112", BackupType: backupType, BackupTime: now.Add(-time.Hour),
					Instance: "pbs-main", Namespace: "pve"},
				// The same attribution rule also governs a live, incomplete PBS
				// snapshot: only B may show a running backup.
				{ID: "running-b", VMID: "112", BackupType: backupType, BackupTime: time.Now(),
					Instance: "pbs-main", Namespace: "pve", Comment: "guest-b", InProgress: true},
				{ID: "running-shared", VMID: "112", BackupType: backupType, BackupTime: time.Now(),
					Instance: "pbs-main", Namespace: "pve", InProgress: true},
			}
			state.mu.Unlock()

			state.SyncGuestBackupTimes()
			snapshot := state.GetSnapshot()
			got := make(map[string]time.Time)
			if backupType == "vm" {
				for _, guest := range snapshot.VMs {
					got[guest.Instance] = guest.LastBackup
				}
			} else {
				for _, guest := range snapshot.Containers {
					got[guest.Instance] = guest.LastBackup
				}
			}
			if !got["cluster-a"].Equal(recentA) {
				t.Errorf("cluster-a LastBackup = %v, want its newer uniquely attributable backup %v", got["cluster-a"], recentA)
			}
			if !got["cluster-b"].Equal(recentB) {
				t.Errorf("cluster-b LastBackup = %v, want its own backup %v", got["cluster-b"], recentB)
			}
			if backupType == "vm" {
				for _, guest := range snapshot.VMs {
					if guest.BackupInProgress != (guest.Instance == "cluster-b") {
						t.Errorf("%s BackupInProgress = %v, want only cluster-b running", guest.Instance, guest.BackupInProgress)
					}
				}
			} else {
				for _, guest := range snapshot.Containers {
					if guest.BackupInProgress != (guest.Instance == "cluster-b") {
						t.Errorf("%s BackupInProgress = %v, want only cluster-b running", guest.Instance, guest.BackupInProgress)
					}
				}
			}
		})
	}
}
