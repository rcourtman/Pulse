package models

import (
	"fmt"
	"reflect"
	"testing"
	"time"
)

func TestUpdatePBSBackupsPublishesDetachedWinningRows(t *testing.T) {
	s := NewState()
	at := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	peer := PBSBackup{
		ID: "peer", Instance: "other", Datastore: "archive", BackupType: "ct", VMID: "101",
		BackupTime: at.Add(time.Hour), Files: []string{"peer.conf.blob"},
	}
	s.UpdatePBSBackups("other", []PBSBackup{
		peer, {ID: "shared-id", Instance: "other", BackupTime: at.Add(4 * time.Hour)},
	})
	s.UpdatePBSBackups("active", []PBSBackup{{ID: "removed", Instance: "active", BackupTime: at.Add(5 * time.Hour)}})
	before := s.GetSnapshot()

	winner := PBSBackup{
		ID: "replacement", Instance: "active", Datastore: "archive", Namespace: "test/ns",
		BackupType: "vm", VMID: "101", BackupTime: at.Add(2 * time.Hour),
		InProgress: true, WriteActivityObserved: true, WriteActive: true, CarriedOver: true,
		VerificationRaw: map[string]any{"state": "fixture"},
		Files:           []string{"qemu-server.conf.blob"},
	}
	newShared := PBSBackup{ID: "shared-id", Instance: "active", BackupTime: at}
	input := []PBSBackup{
		{ID: winner.ID, Instance: "active", BackupTime: at.Add(3 * time.Hour)},
		newShared, winner,
	}
	want := []PBSBackup{clonePBSBackup(winner), clonePBSBackup(peer), newShared.NormalizeCollections()}

	s.UpdatePBSBackups("active", input)
	// Both nested fields copied by clonePBSBackup still belong to the caller.
	input[2].Files[0] = "input-mutated"
	input[2].VerificationRaw.(map[string]any)["state"] = "input-mutated"
	input[2].VMID = "102"

	snapshot := s.GetSnapshot()
	if !reflect.DeepEqual(snapshot.PBSBackups, want) || !reflect.DeepEqual(snapshot.Backups.PBS, want) {
		t.Fatalf("publication must keep newest-first winners and peer evidence: got %+v, want %+v", snapshot.PBSBackups, want)
	}
	if len(before.PBSBackups) != 3 || before.PBSBackups[0].ID != "removed" {
		t.Fatalf("later publication rewrote the previous reader's rows: %+v", before.PBSBackups)
	}

	s.UpdatePBSBackups("active", nil)
	if got := s.GetPBSBackups(); !reflect.DeepEqual(got, want[1:2]) {
		t.Fatalf("empty read should remove only the refreshed instance's rows: got %+v", got)
	}
	s.UpdatePBSBackups("other", nil)
	if got := s.GetPBSBackups(); got == nil || len(got) != 0 {
		t.Fatalf("empty backup reads must keep an empty collection: got %+v", got)
	}
}

func BenchmarkUpdatePBSBackups(b *testing.B) {
	for _, tc := range []struct {
		name              string
		incoming, sibling int
	}{
		{"single", 1, 0},
		// 35 fast-store and 17 archive-store groups with 50 snapshots
		// each on both PBS instances, as in the Proxmox memory watch.
		{"watch-estate", 2600, 2600},
	} {
		b.Run(tc.name, func(b *testing.B) {
			at := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
			rows := func(instance string, count int) []PBSBackup {
				out := make([]PBSBackup, count)
				for i := range out {
					out[i] = PBSBackup{
						ID: fmt.Sprintf("%s-%d", instance, i), Instance: instance,
						BackupTime:      at.Add(time.Duration(i) * time.Minute),
						Files:           []string{"index.json.blob", "config.blob", "disk.fidx", "client.log.blob"},
						VerificationRaw: map[string]any{"state": "ok", "upid": "fixture"},
					}
				}
				return out
			}
			s := NewState()
			s.UpdatePBSBackups("sibling", rows("sibling", tc.sibling))
			incoming := rows("active", tc.incoming)
			s.UpdatePBSBackups("active", incoming)
			b.ReportAllocs()
			for b.Loop() {
				s.UpdatePBSBackups("active", incoming)
			}
		})
	}
}
