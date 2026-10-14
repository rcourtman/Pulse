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
		StorageBackups: []StorageBackup{{ID: "archive", Instance: "pve", VMID: "100"}},
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
