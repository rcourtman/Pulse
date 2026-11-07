package models

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"
)

func pmgPollingStateFixture() *State {
	s := NewState()
	at := time.Date(2026, 11, 7, 0, 0, 0, 0, time.UTC)
	s.PMGBackups = []PMGBackup{
		{ID: "same", Instance: "site", Node: "one", Filename: "one.tgz", BackupTime: at, Size: 10},
		{ID: "same", Instance: "peer", Node: "one", Filename: "peer.tgz", BackupTime: at.Add(time.Hour), Size: 20},
		{ID: "second", Instance: "site", Node: "two", Filename: "two.tgz", BackupTime: at.Add(-time.Hour), Size: 30},
		{ID: "empty", Instance: "", Filename: "default.tgz"},
		{ID: "space", Instance: " site"},
		{ID: "case", Instance: "SITE"},
	}
	s.Backups.PMG = []PMGBackup{{ID: "stale-aggregate", Instance: "site"}}
	return s
}

func TestPMGBackupPollingSnapshotParityAndOwnership(t *testing.T) {
	for _, s := range []*State{{}, NewState(), pmgPollingStateFixture()} {
		full := s.GetSnapshot().PMGBackups
		for _, instance := range []string{"site", "peer", "", " site", "SITE", "absent"} {
			want := []PMGBackup{}
			for _, row := range full {
				if row.Instance == instance {
					want = append(want, row)
				}
			}
			got := s.GetPMGBackupsForInstance(instance)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("%q differs from full snapshot: got %+v; want %+v", instance, got, want)
			}
			gotJSON, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			wantJSON, err := json.Marshal(want)
			if err != nil || string(gotJSON) != string(wantJSON) {
				t.Fatalf("serialized values or order changed for %q", instance)
			}
		}
	}
	s := pmgPollingStateFixture()
	before := s.GetSnapshot().PMGBackups
	rows := s.GetPMGBackupsForInstance("site")
	rows[0].ID = "reader-only"
	rows[0].BackupTime = time.Time{}
	rows[1] = PMGBackup{}
	rows = append(rows, PMGBackup{ID: "reader-only"})
	if !reflect.DeepEqual(s.GetSnapshot().PMGBackups, before) {
		t.Fatal("caller mutated primary backup evidence")
	}
	s.UpdatePMGBackups("site", []PMGBackup{{ID: "replacement", Instance: "site"}})
	if len(rows) != 3 || rows[0].ID != "reader-only" {
		t.Fatal("source replacement mutated a retained read")
	}
}

func TestPMGBackupPollingSnapshotConcurrentReplacement(t *testing.T) {
	s := NewState()
	s.UpdatePMGBackups("site", []PMGBackup{{ID: "0", Instance: "site", Filename: "0"}, {ID: "0", Instance: "site", Filename: "0"}})
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 1; i <= 100; i++ {
			id := fmt.Sprint(i)
			s.UpdatePMGBackups("site", []PMGBackup{{ID: id, Instance: "site", Filename: id}, {ID: id, Instance: "site", Filename: id}})
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			rows := s.GetPMGBackupsForInstance("site")
			if len(rows) != 2 || rows[0].ID != rows[1].ID || rows[0].Filename != rows[0].ID || rows[1].Filename != rows[0].ID {
				t.Error("read mixed source generations")
				return
			}
			rows[0].ID = "reader-only"
		}
	}()
	wg.Wait()
}

var pmgBackupPollingSnapshotSink []PMGBackup

func TestPMGBackupPollingSnapshotDoesNotCopyOtherInstances(t *testing.T) {
	small, large := pmgPollingStateFixture(), pmgPollingStateFixture()
	for i := 0; i < 1000; i++ {
		large.PMGBackups = append(large.PMGBackups, PMGBackup{ID: fmt.Sprint(i), Instance: "other"})
	}
	measure := func(s *State) float64 {
		return testing.AllocsPerRun(5, func() { pmgBackupPollingSnapshotSink = s.GetPMGBackupsForInstance("site") })
	}
	base, grown := measure(small), measure(large)
	t.Logf("narrow PMG snapshot allocations: small=%.0f other-instance-1000=%.0f", base, grown)
	if grown > base {
		t.Fatalf("narrow snapshot copied other instances: %.0f -> %.0f", base, grown)
	}
}
