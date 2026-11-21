package models

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"
)

func metadataStateFixture() *State {
	s := NewState()
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	rate := 64.0
	s.PVEBackups = PVEBackups{
		BackupTasks:    []BackupTask{{ID: "task", Instance: "pve", VMID: 100, Status: "OK"}},
		StorageBackups: []StorageBackup{{ID: "archive", Instance: "pve", VMID: 100}},
		GuestSnapshots: []GuestSnapshot{{ID: "snapshot", Instance: "pve", VMID: 100}},
	}
	s.PBSBackups = []PBSBackup{{ID: "pbs", Instance: "pbs", Namespace: "team", VMID: "100", BackupTime: now,
		Protected: true, InProgress: true, WriteActivityObserved: true, WriteActive: true,
		Files: []string{"index.json.blob"}, VerificationRaw: map[string]interface{}{"state": "ok"}, Owner: "fixture@pbs"}}
	s.PMGBackups = []PMGBackup{{ID: "pmg", Instance: "pmg", BackupTime: now}}
	s.ReplicationJobs = []ReplicationJob{{ID: "replication", LastSyncTime: &now, NextSyncTime: &now, RateLimitMbps: &rate}}
	s.ConnectionHealth = map[string]bool{"pve": true, "pbs": false}
	// The complete snapshot derives backups from the primary collections, not
	// this cache. A scoped optimization must do exactly the same.
	s.Backups = Backups{PBS: []PBSBackup{{ID: "stale-aggregate"}}}
	return s
}

func assertMetadataSnapshotParity(t *testing.T, s *State) {
	t.Helper()
	want := s.GetSnapshot()
	for _, pair := range []struct {
		name      string
		got, want any
	}{
		{"backups", s.GetBackups(), want.Backups},
		{"pve", s.GetPVEBackups(), want.PVEBackups},
		{"pbs", s.GetPBSBackups(), want.PBSBackups},
		{"replication", s.GetReplicationJobs(), want.ReplicationJobs},
		{"health", s.GetConnectionHealth(), want.ConnectionHealth},
	} {
		if !reflect.DeepEqual(pair.got, pair.want) {
			t.Fatalf("%s projection differs from complete snapshot", pair.name)
		}
		gotJSON, err := json.Marshal(pair.got)
		if err != nil {
			t.Fatal(err)
		}
		wantJSON, err := json.Marshal(pair.want)
		if err != nil || !bytes.Equal(gotJSON, wantJSON) {
			t.Fatalf("%s serialization changed", pair.name)
		}
	}
}

func TestMetadataSnapshotsParityAndOwnership(t *testing.T) {
	for _, state := range []*State{{}, NewState(), metadataStateFixture()} {
		assertMetadataSnapshotParity(t, state)
	}
	s := metadataStateFixture()
	before, err := json.Marshal(s.GetSnapshot())
	if err != nil {
		t.Fatal(err)
	}
	all := s.GetBackups()
	all.PVE.BackupTasks[0].Status = "changed"
	all.PVE.StorageBackups[0].ID = "changed"
	all.PVE.GuestSnapshots[0].ID = "changed"
	all.PBS[0].Files[0] = "changed"
	all.PBS[0].VerificationRaw.(map[string]interface{})["state"] = "changed"
	all.PMG[0].ID = "changed"
	pve := s.GetPVEBackups()
	pve.BackupTasks[0].ID = "changed"
	pve.StorageBackups[0].ID = "changed"
	pve.GuestSnapshots[0].ID = "changed"
	pbs := s.GetPBSBackups()
	pbs[0].ID = "changed"
	pbs[0].Files[0] = "changed"
	pbs[0].VerificationRaw.(map[string]interface{})["state"] = "changed"
	jobs := s.GetReplicationJobs()
	jobs[0].ID = "changed"
	*jobs[0].LastSyncTime = time.Time{}
	*jobs[0].NextSyncTime = time.Time{}
	*jobs[0].RateLimitMbps = 0
	health := s.GetConnectionHealth()
	health["pve"] = false
	health["foreign"] = true
	after, err := json.Marshal(s.GetSnapshot())
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("scoped projection shares mutable source fields")
	}
	assertMetadataSnapshotParity(t, s)
}

func TestPVEBackupCarryForwardReadsKeepOrderedPrimaryRows(t *testing.T) {
	s := metadataStateFixture()
	archives := s.PVEBackups.StorageBackups
	snapshots := s.PVEBackups.GuestSnapshots
	// A single connection still gets an independent bulk copy.
	if got := s.GetPVEStorageBackupsForInstance("pve"); !reflect.DeepEqual(got, archives) {
		t.Fatalf("single-connection archives: got %+v, want %+v", got, archives)
	}
	if got := s.GetPVEGuestSnapshotsForInstance("pve"); !reflect.DeepEqual(got, snapshots) {
		t.Fatalf("single-connection snapshots: got %+v, want %+v", got, snapshots)
	}

	at := time.Date(2026, 11, 20, 7, 30, 0, 0, time.UTC)
	wantArchives := []StorageBackup{
		{ID: "last-local", Instance: "pve", Node: "node-b", StorageNode: "node-a", VMID: 100, Time: at, InProgress: true},
		{ID: "older-shared", Instance: "pve", Node: "node-b", VMID: 100, Time: at.Add(-time.Hour), Verified: true},
	}
	wantSnapshots := []GuestSnapshot{
		{ID: "last-snapshot", Instance: "pve", Node: "node-b", VMID: 100, Time: at, VMState: true},
		{ID: "older-snapshot", Instance: "pve", Node: "node-b", VMID: 100, Time: at.Add(-time.Hour), SizeBytes: 2048},
	}
	s.PVEBackups.StorageBackups = []StorageBackup{wantArchives[0], {ID: "peer-archive", Instance: "PVE", Node: "node-b", VMID: 100}, wantArchives[1]}
	s.PVEBackups.GuestSnapshots = []GuestSnapshot{wantSnapshots[0], {ID: "peer-snapshot", Instance: "PVE", Node: "node-b", VMID: 100}, wantSnapshots[1]}
	s.Backups.PVE = PVEBackups{StorageBackups: archives, GuestSnapshots: snapshots}

	check := func() ([]StorageBackup, []GuestSnapshot) {
		t.Helper()
		gotArchives := s.GetPVEStorageBackupsForInstance("pve")
		gotSnapshots := s.GetPVEGuestSnapshotsForInstance("pve")
		if !reflect.DeepEqual(gotArchives, wantArchives) || !reflect.DeepEqual(gotSnapshots, wantSnapshots) {
			t.Fatalf("scoped reads changed order, provenance or content: archives=%+v, snapshots=%+v", gotArchives, gotSnapshots)
		}
		return gotArchives, gotSnapshots
	}
	gotArchives, gotSnapshots := check()
	gotArchives[0].StorageNode, gotSnapshots[0].Description = "caller-only", "caller-only"
	check()
	if missing := s.GetPVEStorageBackupsForInstance("missing"); missing == nil || len(missing) != 0 {
		t.Fatalf("missing archive connection should return an empty slice: %#v", missing)
	}
	if missing := s.GetPVEGuestSnapshotsForInstance("missing"); missing == nil || len(missing) != 0 {
		t.Fatalf("missing snapshot connection should return an empty slice: %#v", missing)
	}
}

func TestPVEBackupTaskReadKeepsOrderedDetachedRowsForUnreadNodes(t *testing.T) {
	s := metadataStateFixture()
	at := time.Date(2026, 11, 21, 6, 0, 0, 0, time.UTC)
	want := []BackupTask{
		{ID: "child", Instance: "pve", Node: "node-b", VMID: 100, Status: "error", Error: "synthetic error", StartTime: at, ObservedAt: at},
		{ID: "parent", Instance: "pve", Node: "node-a", Status: "running", StartTime: at.Add(-time.Minute), ObservedAt: at},
	}
	s.PVEBackups.BackupTasks = []BackupTask{
		want[0],
		{ID: "peer", Instance: "PVE", Node: "node-b", VMID: 100},
		{ID: "healthy", Instance: "pve", Node: "node-c", VMID: 100},
		{ID: "different-node", Instance: "pve", Node: "NODE-A", VMID: 100},
		want[1],
	}
	s.Backups.PVE.BackupTasks = []BackupTask{{ID: "stale-aggregate", Instance: "pve", Node: "node-b"}}

	unread := map[string]struct{}{"node-a": {}, "node-b": {}}
	got := s.GetPVEBackupTasksForNodes("pve", unread)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want ordered primary rows %+v", got, want)
	}
	got[0].ObservedAt, got[0].Error = time.Time{}, "reader-only"
	if again := s.GetPVEBackupTasksForNodes("pve", unread); !reflect.DeepEqual(again, want) {
		t.Fatalf("scoped reader mutated primary task rows: %+v", again)
	}
	for _, empty := range [][]BackupTask{
		s.GetPVEBackupTasksForNodes("pve", nil),
		s.GetPVEBackupTasksForNodes("missing", unread),
	} {
		if empty == nil || len(empty) != 0 {
			t.Fatalf("unmatched scope should return an empty slice: %#v", empty)
		}
	}
}

func TestPBSBackupCarryForwardReadsKeepOrderedDetachedPrimaryRows(t *testing.T) {
	s := metadataStateFixture()
	got := s.GetPBSBackupsForInstance("pbs")
	if want := s.GetPBSBackups(); !reflect.DeepEqual(got, want) {
		t.Fatalf("single-connection rows: got %+v, want %+v", got, want)
	}
	s.PBSBackups = append(s.PBSBackups,
		PBSBackup{ID: "peer", Instance: "PBS", Namespace: "team", VMID: "100"},
		PBSBackup{ID: "older", Instance: "pbs", Namespace: "team", VMID: "100", VerificationRaw: "ok", CarriedOver: true},
		PBSBackup{ID: "empty-name", Instance: ""},
	)
	s.Backups.PBS = []PBSBackup{{ID: "stale-aggregate", Instance: "pbs"}}
	before := s.GetPBSBackups()
	for _, instance := range []string{"pbs", "PBS", "", "missing"} {
		want := []PBSBackup{}
		for _, row := range before {
			if row.Instance == instance {
				want = append(want, row)
			}
		}
		got := s.GetPBSBackupsForInstance(instance)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%q: got %+v, want ordered primary rows %+v", instance, got, want)
		}
	}

	got = s.GetPBSBackupsForInstance("pbs")
	got[0].Files[0] = "reader-only"
	got[0].VerificationRaw.(map[string]interface{})["state"] = "reader-only"
	got[1] = PBSBackup{}
	if after := s.GetPBSBackups(); !reflect.DeepEqual(after, before) {
		t.Fatal("scoped caller changed stored backup evidence")
	}
}

func TestMetadataSnapshotsConcurrentTransactions(t *testing.T) {
	s := metadataStateFixture()
	var workers sync.WaitGroup
	workers.Add(2)
	go func() {
		defer workers.Done()
		for i := 0; i < 100; i++ {
			id := fmt.Sprint(i)
			s.mu.Lock()
			s.PVEBackups.BackupTasks[0].Instance = id
			s.PBSBackups[0].Instance = id
			s.PMGBackups[0].Instance = id
			s.PBSBackups[0].Files[0] = id
			s.PBSBackups[0].VerificationRaw.(map[string]interface{})["state"] = id
			s.ReplicationJobs[0].ID = id
			s.ConnectionHealth["pve"] = i%2 == 0
			s.mu.Unlock()
		}
	}()
	go func() {
		defer workers.Done()
		for i := 0; i < 100; i++ {
			all := s.GetBackups()
			// The initial fixture has different source instances; every writer
			// transaction after it uses one generation across all three sources.
			id := all.PVE.BackupTasks[0].Instance
			if id != "pve" && (all.PBS[0].Instance != id || all.PMG[0].Instance != id) {
				t.Error("aggregate projection mixed writer generations")
				return
			}
			all.PBS[0].Files[0] = "reader"
			s.GetPVEBackups().BackupTasks[0].ID = "reader"
			s.GetPBSBackups()[0].Files[0] = "reader"
			s.GetReplicationJobs()[0].ID = "reader"
			s.GetConnectionHealth()["pve"] = false
		}
	}()
	workers.Wait()
	assertMetadataSnapshotParity(t, s)
}

func TestSyncBackupsDoesNotDeepCloneUnchangedPBSInventory(t *testing.T) {
	s := NewState()
	const pbsCount = 500
	pbsBackups := make([]PBSBackup, pbsCount)
	for i := range pbsBackups {
		pbsBackups[i] = PBSBackup{
			ID:       fmt.Sprintf("pbs-%d", i),
			Instance: "pbs-main",
			VMID:     fmt.Sprint(100 + i%35),
			Files:    []string{"index.json.blob", "drive-scsi0.img.fidx"},
		}
	}
	s.UpdatePBSBackups("pbs-main", pbsBackups)

	if len(s.Backups.PBS) != pbsCount || &s.Backups.PBS[0] != &s.PBSBackups[0] {
		t.Fatal("State.Backups.PBS retained a duplicate copy instead of mirroring State.PBSBackups")
	}

	snaps := []GuestSnapshot{{ID: "snap-1", Instance: "pve1", VMID: 100, Name: "pre-upgrade"}}
	allocs := testing.AllocsPerRun(20, func() {
		s.UpdateGuestSnapshotsForInstance("pve1", snaps)
	})
	// Deep-cloning 500 PBS backups on every snapshot update allocated >500 times
	// (one slice per backup's Files plus the PBSBackup slice itself).
	if allocs > 50 {
		t.Fatalf("UpdateGuestSnapshotsForInstance allocated %.1f times with %d PBS backups present, want <= 50", allocs, pbsCount)
	}
}
