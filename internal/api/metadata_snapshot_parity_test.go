package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
)

// Exercise the unchanged real HTTP producers through their new monitor reads.
// Compare every output field with the preceding whole-state projection, then
// retain safe fixture bytes for consuming-surface browser verification.
func TestMetadataSnapshotHTTPParity(t *testing.T) {
	previous := mock.IsMockEnabled()
	if err := mock.SetEnabled(false); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := mock.SetEnabled(previous); err != nil {
			t.Error(err)
		}
	})
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	next := now.Add(time.Hour)
	state := models.NewState()
	state.PVEBackups = models.PVEBackups{
		BackupTasks: []models.BackupTask{{ID: "task-100", Node: "node-a", Instance: "pve", Type: "qemu", VMID: 100,
			Status: "OK", StartTime: now, EndTime: now.Add(time.Minute), ObservedAt: now.Add(time.Minute), Size: 1024}},
		StorageBackups: []models.StorageBackup{{ID: "archive-100", Node: "node-a", Instance: "pve", Type: "qemu", VMID: 100,
			Time: now, CTime: now.Unix(), Size: 1073741824, Format: "vma", Storage: "local", Protected: true, Volid: "local:backup/vzdump-qemu-100.vma.zst"}},
		GuestSnapshots: []models.GuestSnapshot{{ID: "snapshot-100", Name: "before-maintenance", Node: "node-a", Instance: "pve",
			Type: "qemu", VMID: 100, Time: now, Description: "Local checkpoint", VMState: true}},
	}
	state.PBSBackups = []models.PBSBackup{
		{ID: "pbs/main/team/vm/100", Instance: "pbs", Datastore: "main", Namespace: "team", BackupType: "vm", VMID: "100",
			BackupTime: now, Size: 2147483648, Protected: true, Verified: true, Owner: "fixture@pbs", Files: []string{"index.json.blob"}},
		{ID: "pbs/main/team/vm/101", Instance: "pbs", Datastore: "main", Namespace: "team", BackupType: "vm", VMID: "101",
			BackupTime: now, InProgress: true, WriteActivityObserved: true, WriteActive: false, Files: []string{"qemu-server.conf.blob"}},
	}
	state.ReplicationJobs = []models.ReplicationJob{{ID: "pve/100-0", Instance: "pve", JobID: "100-0", GuestID: 100,
		GuestName: "Recovery VM", GuestType: "qemu", SourceNode: "node-a", TargetNode: "node-b", SourceStorage: "local-zfs", TargetStorage: "local-zfs",
		Enabled: true, LastSyncStatus: "OK", LastSyncTime: &now, NextSyncTime: &next, DurationSeconds: 30, Schedule: "hourly", LastPolled: now}}
	monitor := &monitoring.Monitor{}
	setUnexportedField(t, monitor, "state", state)
	router := &Router{monitor: monitor}
	full := monitor.GetState()
	frames := make(map[string]json.RawMessage)
	for _, path := range []string{
		"/api/backups/pve", "/api/backups/pve?vmid=100", "/api/backups/pve?vmid=999",
		"/api/backups/pbs", "/api/backups/pbs?vmid=101", "/api/backups/pbs?namespace=missing",
		"/api/replication/jobs", "/api/replication/jobs?guest=100", "/api/replication/jobs?platform=unknown",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		var want any
		switch req.URL.Path {
		case "/api/backups/pve":
			router.handleListPVEBackups(rec, req)
			data := filterPVEBackups(full.PVEBackups, req)
			want = PVEBackupsResponse{Data: data, Meta: PVEBackupsMeta{len(data.BackupTasks), len(data.StorageBackups), len(data.GuestSnapshots)}}
		case "/api/backups/pbs":
			router.handleListPBSBackups(rec, req)
			data := filterPBSBackups(full.PBSBackups, req)
			want = PBSBackupsResponse{Data: PBSBackupsPayload{data}, Meta: PBSBackupsMeta{len(data)}}
		case "/api/replication/jobs":
			router.handleListReplicationJobs(rec, req)
			data := filterReplicationJobs(full.ReplicationJobs, req)
			want = ReplicationJobsResponse{Data: data, Meta: ReplicationJobsMeta{len(data)}}
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("%s returned %d", path, rec.Code)
		}
		var gotJSON, wantJSON any
		if err := json.Unmarshal(rec.Body.Bytes(), &gotJSON); err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(want)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(encoded, &wantJSON); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(gotJSON, wantJSON) {
			t.Fatalf("%s response changed", path)
		}
		frames[path] = append(json.RawMessage(nil), rec.Body.Bytes()...)
	}
	encoded, err := json.Marshal(frames)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("METADATA_HTTP_FRAMES=%s", encoded)
}
