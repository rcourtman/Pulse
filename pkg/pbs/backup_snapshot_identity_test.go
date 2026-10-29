package pbs

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func TestBackupSnapshotsRequireRequestedUniqueIdentity(t *testing.T) {
	for _, group := range []struct{ kind, id string }{{"vm", "101"}, {"ct", "101"}, {"host", "host-A.example"}} {
		row := func(kind, id string, at int64) BackupSnapshot {
			return BackupSnapshot{BackupType: kind, BackupID: id, BackupTime: at, Size: 1024, Comment: "private-payload-marker"}
		}
		valid := row(group.kind, group.id, 1793293200)
		wrongKind := "vm"
		if group.kind == wrongKind {
			wrongKind = "ct"
		}
		for _, tc := range []struct {
			name  string
			rows  []BackupSnapshot
			valid bool
		}{
			{"empty", []BackupSnapshot{}, true},
			{"matching", []BackupSnapshot{valid}, true},
			{"distinct-times-in-response-order", []BackupSnapshot{valid, row(group.kind, group.id, valid.BackupTime-3600)}, true},
			{"wrong-kind", []BackupSnapshot{row(wrongKind, group.id, valid.BackupTime)}, false},
			{"wrong-id", []BackupSnapshot{row(group.kind, group.id+"0", valid.BackupTime)}, false},
			{"missing-kind", []BackupSnapshot{row("", group.id, valid.BackupTime)}, false},
			{"missing-id", []BackupSnapshot{row(group.kind, "", valid.BackupTime)}, false},
			{"no-kind-normalisation", []BackupSnapshot{row(strings.ToUpper(group.kind), group.id, valid.BackupTime)}, false},
			{"no-id-normalisation", []BackupSnapshot{row(group.kind, " "+group.id, valid.BackupTime)}, false},
			{"mixed-valid-and-invalid", []BackupSnapshot{valid, row(wrongKind, group.id, valid.BackupTime-3600)}, false},
			{"identical-repeat", []BackupSnapshot{valid, valid}, false},
			{"conflicting-repeat", []BackupSnapshot{valid, {BackupType: group.kind, BackupID: group.id, BackupTime: valid.BackupTime, Size: 999}}, false},
		} {
			t.Run(group.kind+"/"+tc.name, func(t *testing.T) {
				reads := 0
				client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
					reads++
					q := r.URL.Query()
					if r.Method != http.MethodGet || r.URL.Path != "/api2/json/admin/datastore/archive/snapshots" || q.Get("ns") != "tenant/one" || q.Get("backup-type") != group.kind || q.Get("backup-id") != group.id {
						t.Errorf("request scope/method changed: %s %s", r.Method, r.URL)
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"data": tc.rows})
				})
				defer server.Close()
				got, err := client.ListBackupSnapshots(context.Background(), "archive", "tenant/one", group.kind, group.id)
				if tc.valid {
					if err != nil || !reflect.DeepEqual(got, tc.rows) {
						t.Fatalf("valid response changed: rows=%+v err=%v", got, err)
					}
				} else {
					if err == nil || len(got) != 0 {
						t.Fatalf("conflicting group admitted: rows=%+v err=%v", got, err)
					}
					if err != nil && strings.Contains(err.Error(), "private-payload-marker") {
						t.Error("error exposed snapshot payload")
					}
				}
				if reads != 1 {
					t.Errorf("admission caused %d requests, want one", reads)
				}
			})
		}
	}
}

func TestBackupSnapshotIdentityAdmissionCoversEntireResponse(t *testing.T) {
	rows := make([]BackupSnapshot, 101)
	for i := range rows {
		rows[i] = BackupSnapshot{BackupType: "vm", BackupID: "101", BackupTime: int64(1793293200 - i)}
	}
	rows[len(rows)-1].BackupID = "102"
	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": rows})
	})
	defer server.Close()
	got, err := client.ListBackupSnapshots(context.Background(), "archive", "", "vm", "101")
	if err == nil || len(got) != 0 {
		t.Fatalf("invalid row beyond retained group cap admitted: count=%d err=%v", len(got), err)
	}
}
