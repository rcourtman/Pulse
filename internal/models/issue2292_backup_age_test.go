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
					if guest.VMID == 112 {
						got[guest.Instance] = guest.LastBackup
					}
				}
			} else {
				for _, guest := range snapshot.Containers {
					if guest.VMID == 112 {
						got[guest.Instance] = guest.LastBackup
					}
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
					{VMID: 113, Name: "other-b", Instance: "cluster-b", Node: "pve"},
				})
			} else {
				state.UpdateContainers([]Container{
					{VMID: 112, Name: "guest-a", Instance: "cluster-a", Node: "pve"},
					{VMID: 112, Name: "guest-b", Instance: "cluster-b", Node: "pve"},
					{VMID: 113, Name: "other-b", Instance: "cluster-b", Node: "pve"},
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
				// B is visible on a different PBS instance. That is not proof
				// that pbs-main's tied snapshot belongs to A rather than B.
				{ID: "other-b", VMID: "113", BackupType: backupType, BackupTime: recentB,
					Instance: "pbs-other"},
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
					if guest.VMID == 112 {
						got[guest.Instance] = guest.LastBackup
					}
				}
			} else {
				for _, guest := range snapshot.Containers {
					if guest.VMID == 112 {
						got[guest.Instance] = guest.LastBackup
					}
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
					if guest.VMID != 112 {
						continue
					}
					if guest.BackupInProgress != (guest.Instance == "cluster-b") {
						t.Errorf("%s BackupInProgress = %v, want only cluster-b running", guest.Instance, guest.BackupInProgress)
					}
				}
			} else {
				for _, guest := range snapshot.Containers {
					if guest.VMID != 112 {
						continue
					}
					if guest.BackupInProgress != (guest.Instance == "cluster-b") {
						t.Errorf("%s BackupInProgress = %v, want only cluster-b running", guest.Instance, guest.BackupInProgress)
					}
				}
			}
		})
	}
}

// The reported notes template changed from "pulse" to "pdm21.21008.pulse".
// Root has no namespace placement, so a colliding CT ID needs the complete
// node/VMID/name comment to identify which PVE connection owns the newer row.
func TestSyncGuestBackupTimesNodeVMIDNameCommentForUniqueCT(t *testing.T) {
	now := time.Now()
	recent := now.Add(-10 * time.Hour)
	state := NewState()
	state.UpdateContainers([]Container{
		{VMID: 21008, Name: "pulse", Instance: "cluster-a", Node: "pdm21"},
	})
	state.mu.Lock()
	state.PBSBackups = []PBSBackup{
		{ID: "old", VMID: "21008", BackupType: "ct", BackupTime: now.Add(-29 * 24 * time.Hour),
			Instance: "pbs-main", Comment: "pulse"},
		{ID: "recent", VMID: "21008", BackupType: "ct", BackupTime: recent,
			Instance: "pbs-main", Comment: "pdm21.21008.pulse"},
	}
	state.mu.Unlock()

	state.SyncGuestBackupTimes()
	if got := state.GetSnapshot().Containers[0].LastBackup; !got.Equal(recent) {
		t.Errorf("unique CT LastBackup = %v, want newer Root snapshot %v", got, recent)
	}
}

func TestSyncGuestBackupTimesNodeVMIDNameCommentForCollidingCT(t *testing.T) {
	now := time.Now()
	old := now.Add(-29 * 24 * time.Hour)
	recent := now.Add(-10 * time.Hour)
	other := now.Add(-2 * time.Hour)

	state := NewState()
	state.UpdateContainers([]Container{
		{VMID: 21008, Name: "pulse", Instance: "cluster-a", Node: "pdm21"},
		{VMID: 21008, Name: "other", Instance: "cluster-b", Node: "pdm22"},
	})
	state.mu.Lock()
	state.PBSBackups = []PBSBackup{
		{ID: "old", VMID: "21008", BackupType: "ct", BackupTime: old,
			Instance: "pbs-main", Comment: "pulse"},
		{ID: "recent", VMID: "21008", BackupType: "ct", BackupTime: recent,
			Instance: "pbs-main", Comment: "pdm21.21008.pulse"},
		// An unmarked newer root snapshot is still unsafe to assign to either CT.
		{ID: "unknown", VMID: "21008", BackupType: "ct", BackupTime: other,
			Instance: "pbs-unattributed"},
		{ID: "running", VMID: "21008", BackupType: "ct", BackupTime: now,
			Instance: "pbs-main", Comment: "pdm21.21008.pulse", InProgress: true},
	}
	state.mu.Unlock()

	state.SyncGuestBackupTimes()
	for _, guest := range state.GetSnapshot().Containers {
		switch guest.Instance {
		case "cluster-a":
			if !guest.LastBackup.Equal(recent) || !guest.BackupInProgress {
				t.Errorf("cluster-a LastBackup/running = %v/%v, want %v/true", guest.LastBackup, guest.BackupInProgress, recent)
			}
		case "cluster-b":
			if !guest.LastBackup.IsZero() || guest.BackupInProgress {
				t.Errorf("cluster-b inherited another CT's backup: %v/%v", guest.LastBackup, guest.BackupInProgress)
			}
		}
	}
}

// A matching node/VMID/name comment is still insufficient when two distinct
// connections host an indistinguishable CT. The #1639 collision guard wins.
func TestSyncGuestBackupTimesNodeVMIDNameCommentTieStaysUnattributed(t *testing.T) {
	now := time.Now()
	state := NewState()
	state.UpdateContainers([]Container{
		{VMID: 21008, Name: "pulse", Instance: "cluster-a", Node: "pdm21"},
		{VMID: 21008, Name: "pulse", Instance: "cluster-b", Node: "pdm21"},
	})
	state.mu.Lock()
	state.PBSBackups = []PBSBackup{
		{ID: "tied", VMID: "21008", BackupType: "ct", BackupTime: now.Add(-10 * time.Hour),
			Instance: "pbs-main", Comment: "pdm21.21008.pulse"},
		{ID: "running-tied", VMID: "21008", BackupType: "ct", BackupTime: now,
			Instance: "pbs-main", Comment: "pdm21.21008.pulse", InProgress: true},
	}
	state.mu.Unlock()

	state.SyncGuestBackupTimes()
	for _, guest := range state.GetSnapshot().Containers {
		if !guest.LastBackup.IsZero() || guest.BackupInProgress {
			t.Errorf("%s inherited tied CT backup: %v/%v", guest.Instance, guest.LastBackup, guest.BackupInProgress)
		}
	}
}
