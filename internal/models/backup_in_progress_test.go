package models

import (
	"testing"
	"time"
)

// An in-flight PBS snapshot (no manifest yet) was previously counted as a
// completed backup: the guest's LastBackup jumped to "now" the moment vzdump
// started, and the badge showed a healthy backup that did not exist.
// These tests pin the corrected behaviour: in-progress evidence feeds
// BackupInProgress only, and LastBackup tracks completed backups.

func TestSyncGuestBackupTimesIgnoresInProgressPBSSnapshot(t *testing.T) {
	state := NewState()
	now := time.Now()
	completedTime := now.Add(-48 * time.Hour)
	inFlightTime := now.Add(-5 * time.Minute)

	state.UpdateVMs([]VM{
		{VMID: 117, Name: "win11-pvepc", Instance: "pve-pc", Node: "pve-pc"},
	})

	state.mu.Lock()
	state.PBSBackups = []PBSBackup{
		{ID: "done-117", VMID: "117", BackupType: "vm", BackupTime: completedTime,
			Instance: "verdeclose", Datastore: "main", Namespace: "pve-pc"},
		{ID: "flight-117", VMID: "117", BackupType: "vm", BackupTime: inFlightTime,
			Instance: "verdeclose", Datastore: "main", Namespace: "pve-pc", InProgress: true},
	}
	state.mu.Unlock()

	state.SyncGuestBackupTimes()
	snapshot := state.GetSnapshot()

	if len(snapshot.VMs) != 1 {
		t.Fatalf("expected 1 VM, got %d", len(snapshot.VMs))
	}
	vm := snapshot.VMs[0]
	if !vm.LastBackup.Equal(completedTime) {
		t.Errorf("LastBackup = %v, want the completed snapshot %v (in-flight one must not advance it)",
			vm.LastBackup, completedTime)
	}
	if !vm.BackupInProgress {
		t.Error("BackupInProgress = false, want true while an in-flight snapshot exists")
	}
}

func TestSyncGuestBackupTimesInProgressOnlyNeverSetsLastBackup(t *testing.T) {
	state := NewState()

	state.UpdateVMs([]VM{
		{VMID: 117, Name: "win11-pvepc", Instance: "pve-pc", Node: "pve-pc"},
	})

	state.mu.Lock()
	state.PBSBackups = []PBSBackup{
		{ID: "flight-117", VMID: "117", BackupType: "vm", BackupTime: time.Now(),
			Instance: "verdeclose", Datastore: "main", Namespace: "pve-pc", InProgress: true},
	}
	state.mu.Unlock()

	state.SyncGuestBackupTimes()
	snapshot := state.GetSnapshot()

	vm := snapshot.VMs[0]
	if !vm.LastBackup.IsZero() {
		t.Errorf("LastBackup = %v, want zero: the guest has no completed backup", vm.LastBackup)
	}
	if !vm.BackupInProgress {
		t.Error("BackupInProgress = false, want true")
	}
}

func TestSyncGuestBackupTimesTerminalIncompletePBSSnapshotIsNotRunningOrComplete(t *testing.T) {
	state := NewState()
	state.UpdateVMs([]VM{
		{VMID: 117, Name: "win11-pvepc", Instance: "pve-pc", Node: "pve-pc"},
	})

	state.mu.Lock()
	state.PBSBackups = []PBSBackup{{
		ID:                    "failed-sync-117",
		VMID:                  "117",
		BackupType:            "vm",
		BackupTime:            time.Now().Add(-time.Hour),
		Instance:              "offsite",
		Datastore:             "main",
		Namespace:             "pve-pc",
		InProgress:            true,
		WriteActivityObserved: true,
		WriteActive:           false,
	}}
	state.mu.Unlock()

	state.SyncGuestBackupTimes()
	vm := state.GetSnapshot().VMs[0]
	if vm.BackupInProgress {
		t.Error("BackupInProgress = true for an incomplete snapshot after its PBS task terminated")
	}
	if !vm.LastBackup.IsZero() {
		t.Errorf("LastBackup = %v, want zero because a failed sync artifact is not complete", vm.LastBackup)
	}
}

func TestSyncGuestBackupTimesRunningVzdumpTaskSetsBackupInProgress(t *testing.T) {
	state := NewState()
	now := time.Now()

	state.UpdateContainers([]Container{
		{VMID: 105, Name: "media", Instance: "homelab", Node: "minipc"},
	})

	state.mu.Lock()
	state.PVEBackups.BackupTasks = []BackupTask{
		{ID: "homelab-upid1", Instance: "homelab", Node: "minipc", Type: "vzdump",
			VMID: 105, Status: "running", StartTime: now.Add(-10 * time.Minute),
			ObservedAt: now},
	}
	state.mu.Unlock()

	state.SyncGuestBackupTimes()
	snapshot := state.GetSnapshot()

	ct := snapshot.Containers[0]
	if !ct.BackupInProgress {
		t.Error("BackupInProgress = false, want true while a vzdump task is running")
	}
	if !ct.LastBackup.IsZero() {
		t.Errorf("LastBackup = %v, want zero", ct.LastBackup)
	}
}

func TestSyncGuestBackupTimesIgnoresStaleRunningTask(t *testing.T) {
	state := NewState()
	now := time.Now()

	state.UpdateVMs([]VM{
		{VMID: 200, Name: "old", Instance: "homelab", Node: "delly"},
	})

	state.mu.Lock()
	state.PVEBackups.BackupTasks = []BackupTask{
		// Unfinished task last observed two days ago: the instance stopped
		// refreshing, so this must not pin the guest as backing up.
		{ID: "homelab-upid2", Instance: "homelab", Node: "delly", Type: "vzdump",
			VMID: 200, Status: "running", StartTime: now.Add(-49 * time.Hour),
			ObservedAt: now.Add(-48 * time.Hour)},
	}
	state.mu.Unlock()

	state.SyncGuestBackupTimes()
	snapshot := state.GetSnapshot()

	if snapshot.VMs[0].BackupInProgress {
		t.Error("BackupInProgress = true, want false for a stale running task")
	}
}

func TestSyncGuestBackupTimesIgnoresInProgressStorageBackup(t *testing.T) {
	state := NewState()
	now := time.Now()
	completedTime := now.Add(-24 * time.Hour)

	state.UpdateVMs([]VM{
		{VMID: 117, Name: "win11-pvepc", Instance: "pve-pc", Node: "pve-pc"},
	})

	state.mu.Lock()
	state.PVEBackups.StorageBackups = []StorageBackup{
		{ID: "pve-pc-old", Instance: "pve-pc", Node: "pve-pc", Type: "qemu",
			VMID: 117, Time: completedTime, Size: 42 << 30},
		// Partial archive a running vzdump is still writing.
		{ID: "pve-pc-partial", Instance: "pve-pc", Node: "pve-pc", Type: "qemu",
			VMID: 117, Time: now.Add(-3 * time.Minute), Size: 7 << 30, InProgress: true},
	}
	state.mu.Unlock()

	state.SyncGuestBackupTimes()
	snapshot := state.GetSnapshot()

	vm := snapshot.VMs[0]
	if !vm.LastBackup.Equal(completedTime) {
		t.Errorf("LastBackup = %v, want completed archive time %v", vm.LastBackup, completedTime)
	}
}

// Guest polls know nothing about backups; the instance merge must carry the
// backup-running flag forward alongside LastBackup so it does not flap off
// between backup sync passes.
func TestUpdateVMsForInstancePreservesBackupInProgress(t *testing.T) {
	state := NewState()

	state.UpdateVMs([]VM{
		{ID: "pve-pc-117", VMID: 117, Name: "win11-pvepc", Instance: "pve-pc", Node: "pve-pc",
			LastBackup: time.Now().Add(-24 * time.Hour), BackupInProgress: true},
	})

	// Fresh poll data: no backup knowledge at all.
	state.UpdateVMsForInstance("pve-pc", []VM{
		{ID: "pve-pc-117", VMID: 117, Name: "win11-pvepc", Instance: "pve-pc", Node: "pve-pc"},
	})

	snapshot := state.GetSnapshot()
	vm := snapshot.VMs[0]
	if vm.LastBackup.IsZero() {
		t.Error("LastBackup lost across instance update")
	}
	if !vm.BackupInProgress {
		t.Error("BackupInProgress lost across instance update")
	}
}

// A PBS-to-PBS sync interrupted mid-snapshot leaves a manifest-less copy of
// an already-completed backup on the target datastore. That partial copy
// carries the original backup time, so it is never newer than its completed
// twin and must not report the guest as backing up until the sync is
// repaired (#1815).
func TestSyncGuestBackupTimesIgnoresPartialSyncCopyOfCompletedBackup(t *testing.T) {
	state := NewState()
	now := time.Now()
	backupTime := now.Add(-2 * time.Hour)

	state.UpdateVMs([]VM{
		{VMID: 117, Name: "win11-pvepc", Instance: "pve-pc", Node: "pve-pc"},
	})

	state.mu.Lock()
	state.PBSBackups = []PBSBackup{
		// Completed snapshot on the primary PBS.
		{ID: "done-117", VMID: "117", BackupType: "vm", BackupTime: backupTime,
			Instance: "verdeclose", Datastore: "main", Namespace: "pve-pc"},
		// Partial copy of the same snapshot on the sync target.
		{ID: "partial-117", VMID: "117", BackupType: "vm", BackupTime: backupTime,
			Instance: "offsite", Datastore: "sync", Namespace: "pve-pc", InProgress: true},
	}
	state.mu.Unlock()

	state.SyncGuestBackupTimes()
	snapshot := state.GetSnapshot()

	vm := snapshot.VMs[0]
	if !vm.LastBackup.Equal(backupTime) {
		t.Errorf("LastBackup = %v, want %v", vm.LastBackup, backupTime)
	}
	if vm.BackupInProgress {
		t.Error("BackupInProgress = true, want false for a partial sync copy of a completed backup")
	}
}

// A manifest-less snapshot with no completed twin left must age out the same
// way a stale running task does: whatever was writing it is long gone.
func TestSyncGuestBackupTimesIgnoresAbandonedInFlightSnapshot(t *testing.T) {
	state := NewState()
	now := time.Now()

	state.UpdateVMs([]VM{
		{VMID: 117, Name: "win11-pvepc", Instance: "pve-pc", Node: "pve-pc"},
	})

	state.mu.Lock()
	state.PBSBackups = []PBSBackup{
		{ID: "flight-117", VMID: "117", BackupType: "vm", BackupTime: now.Add(-72 * time.Hour),
			Instance: "verdeclose", Datastore: "main", Namespace: "pve-pc", InProgress: true},
	}
	state.mu.Unlock()

	state.SyncGuestBackupTimes()
	snapshot := state.GetSnapshot()

	if snapshot.VMs[0].BackupInProgress {
		t.Error("BackupInProgress = true, want false for an abandoned in-flight snapshot")
	}
	if !snapshot.VMs[0].LastBackup.IsZero() {
		t.Errorf("LastBackup = %v, want zero", snapshot.VMs[0].LastBackup)
	}
}
