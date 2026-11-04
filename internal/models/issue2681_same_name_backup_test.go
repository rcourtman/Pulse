package models

import (
	"testing"
	"time"
)

// The reporter's estate (#2681): a home cluster that backs up to PBS, and a
// standalone host added later that reuses a node name, a VMID and a guest name.
func issue2681State(comment, namespace string, backupTime time.Time) *State {
	state := NewState()
	state.UpdateVMs([]VM{
		{VMID: 100, Name: "docker1", Instance: "home", Node: "pmx1"},
		{VMID: 101, Name: "web", Instance: "home", Node: "pmx2"},
		{VMID: 100, Name: "docker1", Instance: "hetzner", Node: "pmx1"},
	})
	state.mu.Lock()
	state.PBSBackups = []PBSBackup{
		{ID: "home-100", VMID: "100", BackupType: "vm", BackupTime: backupTime,
			Instance: "pbs-main", Datastore: "backups", Owner: "home@pbs!token",
			Comment: comment, Namespace: namespace},
		{ID: "home-101", VMID: "101", BackupType: "vm", BackupTime: backupTime,
			Instance: "pbs-main", Datastore: "backups", Owner: "home@pbs!token"},
	}
	state.mu.Unlock()
	return state
}

func issue2681LastBackups(state *State) (home, hetzner time.Time) {
	for _, vm := range state.GetSnapshot().VMs {
		if vm.VMID != 100 {
			continue
		}
		if vm.Instance == "home" {
			home = vm.LastBackup
		} else {
			hetzner = vm.LastBackup
		}
	}
	return home, hetzner
}

// An exclusive storage view says which connection wrote a snapshot, so it
// settles a colliding guest even when the snapshot's labels fit both.
func TestIssue2681ExclusiveStorageViewSettlesCollidingGuest(t *testing.T) {
	backupTime := time.Now().Add(-2 * time.Hour).Truncate(time.Second)
	for _, tc := range []struct {
		name, comment, namespace string
	}{
		{name: "no evidence on the snapshot"},
		// The default vzdump notes template: both guests are called docker1.
		{name: "guest name matches both", comment: "docker1"},
		// A namespace per node: both connections have a node called pmx1.
		{name: "namespace matches both", namespace: "pmx1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := issue2681State(tc.comment, tc.namespace, backupTime)
			state.UpdatePBSGuestConfirmationsForInstance("home", []PBSGuestConfirmation{
				{Storage: "pbs", BackupType: "vm", VMID: 100, Time: backupTime.Unix()},
				{Storage: "pbs", BackupType: "vm", VMID: 101, Time: backupTime.Unix()},
			})
			state.SyncGuestBackupTimes()
			home, hetzner := issue2681LastBackups(state)
			if !home.Equal(backupTime) {
				t.Errorf("home VM 100 LastBackup = %v, want %v", home, backupTime)
			}
			if !hetzner.IsZero() {
				t.Errorf("hetzner VM 100 LastBackup = %v, want none", hetzner)
			}
		})
	}
}

// Labels that fit both guests stay unattributed without that view: a guest on
// another connection is not ruled out just because no evidence names it.
func TestIssue2681TiedLabelsWithoutStorageViewStayUnattributed(t *testing.T) {
	backupTime := time.Now().Add(-2 * time.Hour).Truncate(time.Second)
	state := issue2681State("docker1", "", backupTime)
	state.UpdateStorage([]Storage{
		{ID: "home-pbs", Name: "pbs", Instance: "home", Type: "pbs", Content: "backup"},
		{ID: "hetzner-local", Name: "local", Instance: "hetzner", Node: "pmx1", Type: "dir", Content: "backup"},
	})
	state.SyncGuestBackupTimes()
	home, hetzner := issue2681LastBackups(state)
	if !home.IsZero() || !hetzner.IsZero() {
		t.Errorf("LastBackup home=%v hetzner=%v, want neither without authorship evidence", home, hetzner)
	}
}

// A view settles a tie. It does not hand a guest a snapshot whose labels fit
// another guest better, and that guest keeps it (#2292).
func TestIssue2681ExclusiveStorageViewDoesNotOutvoteStrongerMatch(t *testing.T) {
	backupTime := time.Now().Add(-2 * time.Hour).Truncate(time.Second)
	state := NewState()
	state.UpdateVMs([]VM{
		{VMID: 100, Name: "web-a", Instance: "home", Node: "pve"},
		{VMID: 100, Name: "web-b", Instance: "hetzner", Node: "pve"},
	})
	state.mu.Lock()
	state.PBSBackups = []PBSBackup{{
		ID: "b-100", VMID: "100", BackupType: "vm", BackupTime: backupTime,
		Instance: "pbs-main", Datastore: "backups", Namespace: "pve", Comment: "web-b",
	}}
	state.mu.Unlock()
	state.UpdatePBSGuestConfirmationsForInstance("home", []PBSGuestConfirmation{
		{Storage: "pbs", BackupType: "vm", VMID: 100, Time: backupTime.Unix()},
	})
	state.SyncGuestBackupTimes()
	home, hetzner := issue2681LastBackups(state)
	if !home.IsZero() {
		t.Errorf("home LastBackup = %v, want none: the labels fit the other guest better", home)
	}
	if !hetzner.Equal(backupTime) {
		t.Errorf("hetzner LastBackup = %v, want %v", hetzner, backupTime)
	}
}

// The reporter's names: connection "Hetzner pmx1" ends in the node name both
// installations use. A namespace named after a connection gives its guest the
// badge, and one that fits both guests equally gives it to neither.
func TestIssue2681ConnectionNamespaceSettlesSharedNodeName(t *testing.T) {
	backupTime := time.Now().Add(-2 * time.Hour).Truncate(time.Second)
	for _, tc := range []struct{ namespace, wantOwner string }{
		{namespace: "Hetzner pmx1", wantOwner: "Hetzner pmx1"},
		{namespace: "hetzner-pmx1", wantOwner: "Hetzner pmx1"},
		{namespace: "Home", wantOwner: "Home"},
		{namespace: "pmx1"},
		{namespace: "site-pmx1"},
		{namespace: "hetzner"},
	} {
		t.Run(tc.namespace, func(t *testing.T) {
			state := NewState()
			state.UpdateVMs([]VM{
				{VMID: 100, Name: "docker1", Instance: "Home", Node: "pmx1"},
				{VMID: 100, Name: "docker1", Instance: "Hetzner pmx1", Node: "pmx1"},
			})
			state.mu.Lock()
			state.PBSBackups = []PBSBackup{{
				ID: "b-100", VMID: "100", BackupType: "vm", BackupTime: backupTime,
				Instance: "pbs-main", Datastore: "backups", Namespace: tc.namespace,
			}}
			state.mu.Unlock()
			state.SyncGuestBackupTimes()
			for _, vm := range state.GetSnapshot().VMs {
				if vm.Instance == tc.wantOwner {
					if !vm.LastBackup.Equal(backupTime) {
						t.Errorf("%s LastBackup = %v, want %v", vm.Instance, vm.LastBackup, backupTime)
					}
				} else if !vm.LastBackup.IsZero() {
					t.Errorf("%s LastBackup = %v, want none", vm.Instance, vm.LastBackup)
				}
			}
		})
	}
}

func TestPBSExclusiveConfirmersOmitsOverlappingViews(t *testing.T) {
	state := NewState()
	state.UpdatePBSGuestConfirmationsForInstance("home", []PBSGuestConfirmation{
		{Storage: "pbs", BackupType: "vm", VMID: 100, Time: 10},
		{Storage: "shared", BackupType: "vm", VMID: 200, Time: 20},
	})
	state.UpdatePBSGuestConfirmationsForInstance("hetzner", []PBSGuestConfirmation{
		{Storage: "shared", BackupType: "vm", VMID: 200, Time: 20},
	})
	got := state.PBSExclusiveConfirmers()
	if got[PBSSnapshotRef{BackupType: "vm", VMID: 100, Time: 10}] != "home" {
		t.Errorf("confirmers = %+v, want home for its own view", got)
	}
	if confirmer, ok := got[PBSSnapshotRef{BackupType: "vm", VMID: 200, Time: 20}]; ok {
		t.Errorf("shared snapshot confirmed by %q, want no confirmer", confirmer)
	}
}
