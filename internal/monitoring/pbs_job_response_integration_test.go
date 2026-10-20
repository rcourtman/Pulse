package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
)

func exportPBSJobResponseObservations(t *testing.T, name string, observations map[string]any) {
	t.Helper()
	if dir := os.Getenv("PULSE_PBS_JOB_RESPONSE_EXPORT"); dir != "" {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		data, err := json.MarshalIndent(observations, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name+".json"), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}

// A missing enabled family must reach the existing drawer as unavailable
// evidence, not disappear while the independent verification remains Healthy.
// No native PBS, guest operation, notification or recovery database is used.
func TestPBSJobResponseGapReachesResources(t *testing.T) {
	var phase atomic.Value
	phase.Store("failed")
	now := time.Now().Truncate(time.Second)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("unexpected mutation: %s", r.Method)
			http.Error(w, "read-only fixture", 405)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		p := phase.Load().(string)
		isSync := r.URL.Path == "/api2/json/config/sync" || r.URL.Path == "/api2/json/nodes/localhost/tasks" && r.URL.Query().Get("typefilter") == "syncjob"
		if isSync {
			switch p {
			case "missing":
				fmt.Fprint(w, `{}`)
				return
			case "null":
				fmt.Fprint(w, `{"data":null}`)
				return
			case "trailing":
				fmt.Fprint(w, `{"data":[]} {"private":"fixture-sensitive-detail"}`)
				return
			case "empty":
				fmt.Fprint(w, `{"data":[]}`)
				return
			}
		}
		status := "ERROR: sync transfer failed"
		if p == "healthy" {
			status = "OK"
		}
		payload := any([]any{})
		switch r.URL.Path {
		case "/api2/json/version":
			payload = map[string]any{"version": "3.4.2"}
		case "/api2/json/nodes/localhost/status":
			payload = map[string]any{"cpu": .1, "memory": map[string]any{"used": 100, "total": 1000}}
		case "/api2/json/config/sync":
			payload = []map[string]any{{"id": "sync-main", "store": "archive", "last-run-state": status, "last-run-upid": "UPID:sync:1", "last-run-endtime": now.Unix()}}
		case "/api2/json/config/verify":
			payload = []map[string]any{{"id": "verify-control", "store": "archive", "last-run-state": "OK", "last-run-upid": "UPID:verify:1", "last-run-endtime": now.Unix()}}
		case "/api2/json/nodes/localhost/tasks":
			worker := r.URL.Query().Get("typefilter")
			if worker == "syncjob" {
				payload = []map[string]any{{"upid": "UPID:sync:1", "worker-type": worker, "worker-id": "sync-main", "status": status, "endtime": now.Unix()}}
			} else if worker == "verificationjob" {
				payload = []map[string]any{{"upid": "UPID:verify:1", "worker-type": worker, "worker-id": "verify-control", "status": "OK", "endtime": now.Unix()}}
			} else {
				t.Errorf("unexpected task family: %s", worker)
			}
		default:
			t.Errorf("unexpected read: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": payload})
	}))
	defer server.Close()
	client, err := pbs.NewClient(pbs.ClientConfig{Host: server.URL, TokenName: "fixture@pbs!reader", TokenValue: "fixture-only"})
	if err != nil {
		t.Fatal(err)
	}
	instance := config.PBSInstance{Name: "pbs-response-evidence", Host: server.URL, MonitorSyncJobs: true, MonitorVerifyJobs: true}
	m := newPBSHealthAuthorityMonitor([]config.PBSInstance{instance})
	observations := map[string]any{}
	for _, p := range []string{"failed", "missing", "null", "trailing", "empty", "healthy", "failed-again"} {
		phase.Store(p)
		m.pollPBSInstance(context.Background(), instance.Name, client)
		snapshot := m.state.GetSnapshot()
		if len(snapshot.PBSInstances) != 1 {
			t.Fatal("PBS instance disappeared")
		}
		raw := snapshot.PBSInstances[0]
		front := m.buildBroadcastFrontendStateFromSnapshot(snapshot, m.mockModeFence.begin())
		observations[p] = map[string]any{"resources": front.Resources, "total": len(front.Resources), "raw": raw}
		if raw.Status != "online" || raw.ConnectionHealth != "healthy" {
			t.Errorf("%s: unavailable job read changed independent connectivity", p)
		}
		partials, controls, syncs := 0, 0, 0
		for _, entry := range raw.JobHealthEvidence {
			if entry.Family == "verify" {
				controls++
				if entry.Posture != "healthy" || entry.ID != "verify-control" {
					t.Errorf("%s: verification control changed: %+v", p, entry)
				}
			}
			if entry.Family == "sync" {
				if entry.Error != "" {
					partials++
					if entry.Posture != "unknown" || entry.Confidence != pbs.JobEvidenceConfidencePartialError || entry.EvidenceScope != pbs.JobEvidenceScopePartialRead {
						t.Errorf("%s: invalid read treated as healthy/permission denial: %+v", p, entry)
					}
				} else {
					syncs++
					want := "critical"
					if p == "healthy" {
						want = "healthy"
					}
					if entry.Posture != want {
						t.Errorf("%s: sync posture=%s want=%s", p, entry.Posture, want)
					}
				}
			}
		}
		wantPartials, wantSyncs := 0, 1
		if p == "missing" || p == "null" || p == "trailing" {
			wantPartials, wantSyncs = 2, 0
		}
		if p == "empty" {
			wantSyncs = 0
		}
		if partials != wantPartials || syncs != wantSyncs || controls != 1 {
			t.Errorf("%s: partial/sync/control=%d/%d/%d want %d/%d/1", p, partials, syncs, controls, wantPartials, wantSyncs)
		}
		wire, _ := json.Marshal(front.Resources)
		if strings.Contains(string(wire), "fixture-sensitive") {
			t.Errorf("%s: malformed body reached served evidence", p)
		}
	}
	exportPBSJobResponseObservations(t, "jobs", observations)
}

// An unreadable live-writer view is not evidence that an incomplete snapshot
// stopped running. Exercise real ordinary polling, cached incomplete artifacts,
// guest backup times and served Resources without backup/freeze/restore commands.
func TestPBSWriterResponseGapCannotClearGuestActivity(t *testing.T) {
	var phase atomic.Value
	var writerReads, snapshotReads atomic.Int32
	phase.Store("active")
	completed := time.Now().Add(-48 * time.Hour).Truncate(time.Second)
	inFlight := time.Now().Add(-5 * time.Minute).Truncate(time.Second)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("unexpected mutation: %s", r.Method)
			http.Error(w, "read-only fixture", 405)
			return
		}
		if r.Header.Get("Authorization") != "PBSAPIToken=fixture@pbs!reader:fixture-only" {
			t.Error("ordinary polling changed credentials")
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/groups"):
			fmt.Fprintf(w, `{"data":[{"backup-type":"vm","backup-id":"117","last-backup":%d,"backup-count":2},{"backup-type":"vm","backup-id":"118","last-backup":%d,"backup-count":1}]}`, inFlight.Unix(), completed.Unix())
		case strings.HasSuffix(r.URL.Path, "/snapshots"):
			snapshotReads.Add(1)
			id := r.URL.Query().Get("backup-id")
			rows := []map[string]any{{"backup-type": "vm", "backup-id": id, "backup-time": completed.Unix(), "size": 1024, "files": []string{"index.json.blob"}, "verification": map[string]any{"state": "ok"}}}
			if id == "117" {
				rows = append(rows, map[string]any{"backup-type": "vm", "backup-id": id, "backup-time": inFlight.Unix(), "size": 0, "files": []string{}, "verification": map[string]any{"state": "none"}})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": rows})
		case r.URL.Path == "/api2/json/nodes/localhost/tasks":
			writerReads.Add(1)
			switch phase.Load().(string) {
			case "missing":
				fmt.Fprint(w, `{}`)
				return
			case "null":
				fmt.Fprint(w, `{"data":null}`)
				return
			case "trailing":
				fmt.Fprint(w, `{"data":[]} {"private":"fixture-sensitive-detail"}`)
				return
			}
			rows := []map[string]any{}
			if phase.Load().(string) != "empty" && r.URL.Query().Get("typefilter") == "backup" {
				row := map[string]any{"worker-type": "backup", "worker-id": "archive:vm/117", "starttime": inFlight.Unix()}
				switch phase.Load().(string) {
				case "type-conflict":
					row["worker-type"], row["worker_type"] = "verificationjob", "backup"
				case "identity-conflict":
					row["worker-id"], row["worker_id"] = "archive:vm/118", "archive:vm/117"
				case "invalid-type":
					row["worker-type"], row["worker_type"] = nil, "backup"
				case "invalid-identity":
					row["worker-id"] = 117
				case "equivalent-aliases":
					row["worker_type"], row["worker_id"] = " BACKUP ", " archive:vm/117 "
				}
				rows = append(rows, row)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": rows})
		default:
			t.Errorf("unexpected read: %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client, err := pbs.NewClient(pbs.ClientConfig{Host: server.URL, TokenName: "fixture@pbs!reader", TokenValue: "fixture-only"})
	if err != nil {
		t.Fatal(err)
	}
	m := &Monitor{state: models.NewState()}
	m.state.UpdateVMs([]models.VM{{ID: makeGuestID("cluster", "pve-a", 117), VMID: 117, Name: "backup-guest", Instance: "cluster", Node: "pve-a", Status: "running"}, {ID: makeGuestID("cluster", "pve-a", 118), VMID: 118, Name: "completed-control", Instance: "cluster", Node: "pve-a", Status: "running"}})
	observations := map[string]any{}
	for _, p := range []string{"active", "missing", "null", "trailing", "type-conflict", "identity-conflict", "invalid-type", "invalid-identity", "empty", "equivalent-aliases", "active-again"} {
		phase.Store(p)
		beforeWriters, beforeSnapshots := writerReads.Load(), snapshotReads.Load()
		m.pollPBSBackups(context.Background(), "pbs-response-evidence", client, []models.PBSDatastore{{Name: "archive"}})
		snapshot := m.state.GetSnapshot()
		front := m.buildBroadcastFrontendStateFromSnapshot(snapshot, m.mockModeFence.begin())
		observations[p] = map[string]any{"resources": front.Resources, "total": len(front.Resources), "backups": snapshot.PBSBackups, "vms": snapshot.VMs, "writerReads": writerReads.Load() - beforeWriters, "snapshotReads": snapshotReads.Load() - beforeSnapshots}
		if len(snapshot.PBSBackups) != 3 || len(snapshot.VMs) != 2 {
			t.Fatal("snapshot or guest inventory changed")
		}
		wantObserved := p == "active" || p == "empty" || p == "equivalent-aliases" || p == "active-again"
		wantReads := int32(1)
		if wantObserved {
			wantReads = 2
		}
		if writerReads.Load()-beforeWriters != wantReads {
			t.Errorf("%s: unexpected writer retry/read count", p)
		}
		if p != "active" && snapshotReads.Load() != beforeSnapshots {
			t.Errorf("%s: unchanged snapshot cache was refetched", p)
		}
		incompleteCount := 0
		for _, backup := range snapshot.PBSBackups {
			if backup.InProgress {
				incompleteCount++
				wantActive := p == "active" || p == "equivalent-aliases" || p == "active-again"
				if backup.WriteActivityObserved != wantObserved || backup.WriteActive != wantActive {
					t.Errorf("%s: writer observed/active=%t/%t want %t/%t", p, backup.WriteActivityObserved, backup.WriteActive, wantObserved, wantActive)
				}
			}
		}
		if incompleteCount != 1 {
			t.Fatal("fixture did not exercise one incomplete snapshot")
		}
		for _, vm := range snapshot.VMs {
			wantRunning := vm.VMID == 117 && p != "empty"
			if vm.BackupInProgress != wantRunning || !vm.LastBackup.Equal(completed) {
				t.Errorf("%s: VM%d activity=%t last=%v want %t/%v", p, vm.VMID, vm.BackupInProgress, vm.LastBackup, wantRunning, completed)
			}
		}
	}
	exportPBSJobResponseObservations(t, "writers", observations)
}
