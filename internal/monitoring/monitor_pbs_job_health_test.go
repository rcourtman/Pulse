package monitoring

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
)

func TestPBSJobHealthRequiresExplicitSuccess(t *testing.T) {
	for _, tc := range []struct {
		name, task, last, freshness, readError, want, reason string
	}{
		{name: "OK", last: "OK", want: "healthy", reason: "last-run-success"},
		{name: "trimmed OK", last: "  oK ", want: "healthy", reason: "last-run-success"},
		{name: "success", last: "success", want: "healthy", reason: "last-run-success"},
		{name: "token failure", last: "ERROR: authentication token rejected", want: "critical", reason: "last-run-failed"},
		{name: "token unknown", last: "authentication token rejected", want: "unknown", reason: "last-run-state-authentication-token-rejected"},
		{name: "not OK", last: "not OK", want: "unknown", reason: "last-run-state-not-ok"},
		{name: "success substring", last: "unsuccessful", want: "unknown", reason: "last-run-state-unsuccessful"},
		{name: "running substring", last: "not running", want: "unknown", reason: "last-run-state-not-running"},
		{name: "running is not completed", last: "running", want: "unknown", reason: "running"},
		{name: "warning with OK", last: "WARNINGS: backup completed OK with skipped chunks", want: "warning", reason: "last-run-warning"},
		{name: "warning with success", last: "WARNINGS: success with skipped chunks", want: "warning", reason: "last-run-warning"},
		{name: "error before warning", last: "ERROR: failure after warnings", want: "critical", reason: "last-run-failed"},
		{name: "error before running", last: "ERROR: previous job still running", want: "critical", reason: "last-run-failed"},
		{name: "ordinary failure", last: "failed", want: "critical", reason: "last-run-failed"},
		{name: "unknown", last: "unknown", want: "unknown", reason: "last-run-state-unknown"},
		{name: "no state", want: "unknown", reason: "no-run-observed"},
		{name: "task wins over config", task: "ERROR: token expired", last: "OK", want: "critical", reason: "last-run-failed"},
		{name: "task running wins over config", task: "running", last: "OK", want: "unknown", reason: "running"},
		{name: "overdue success", last: "OK", freshness: "overdue", want: "warning", reason: "job-overdue"},
		{name: "overdue failure", last: "ERROR: token expired", freshness: "overdue", want: "critical", reason: "last-run-failed"},
		{name: "partial read cannot claim success", last: "OK", readError: "permission denied", want: "unknown", reason: "partial-permission"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fact := pbs.JobHealthEvidence{TaskStatus: tc.task, LastRunState: tc.last, Error: tc.readError, Confidence: "partial-permission"}
			got, reason := pbsJobHealthPosture(fact, models.PBSJobHealthFreshness{State: tc.freshness})
			if got != tc.want || reason != tc.reason {
				t.Fatalf("posture(%q, %q, %q) = %s/%s; want %s/%s", tc.task, tc.last, tc.freshness, got, reason, tc.want, tc.reason)
			}
		})
	}
}

// Read-only synthetic PBS HTTP -> ordinary poll -> published raw state ->
// canonical Resources projection. It sends no guest/backup/restore commands.
func TestPBSPollJobHealthStatusProjection(t *testing.T) {
	var status atomic.Value
	status.Store("OK")
	var mutations atomic.Int64
	now := time.Now().UTC().Truncate(time.Second)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			mutations.Add(1)
			http.Error(w, "mutation not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		payload := any([]any{})
		switch r.URL.Path {
		case "/api2/json/version":
			payload = map[string]any{"version": "3.4.2"}
		case "/api2/json/nodes/localhost/status":
			payload = map[string]any{"cpu": 0.1, "memory": map[string]any{"used": 100, "total": 1000}}
		case "/api2/json/config/sync":
			payload = []map[string]any{{"id": "sync-main", "store": "backups", "schedule": "hourly", "last-run-state": status.Load().(string), "last-run-upid": "UPID:sync:1", "last-run-endtime": now.Unix(), "next-run": now.Add(time.Hour).Unix()}}
		case "/api2/json/config/verify":
			payload = []map[string]any{{"id": "verify-control", "store": "backups", "last-run-state": "OK", "last-run-upid": "UPID:verify:1", "last-run-endtime": now.Unix()}}
		case "/api2/json/nodes/localhost/tasks":
			switch r.URL.Query().Get("typefilter") {
			case "syncjob":
				payload = []map[string]any{{"upid": "UPID:sync:1", "worker-type": "syncjob", "worker-id": "sync-main", "status": status.Load().(string), "endtime": now.Unix()}}
			case "verificationjob":
				payload = []map[string]any{{"upid": "UPID:verify:1", "worker-type": "verificationjob", "worker-id": "verify-control", "status": "OK", "endtime": now.Unix()}}
			default:
				t.Errorf("unexpected task query: %s", r.URL.RawQuery)
			}
		default:
			t.Errorf("unexpected read: %s", r.URL.Path)
			http.Error(w, "unexpected path", http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": payload})
	}))
	defer server.Close()
	instance := config.PBSInstance{Name: "pbs-job-evidence", Host: server.URL, MonitorSyncJobs: true, MonitorVerifyJobs: true}
	monitor := newPBSHealthAuthorityMonitor([]config.PBSInstance{instance})
	client, err := pbs.NewClient(pbs.ClientConfig{Host: server.URL, TokenName: "fixture@pbs!monitor", TokenValue: "synthetic-only"})
	if err != nil {
		t.Fatal(err)
	}
	adapter := unifiedresources.NewMonitorAdapter(nil)
	exports := make(map[string]any)
	var identity string
	for _, phase := range []struct{ name, status, posture, reason string }{
		{"failed", "ERROR: authentication token rejected", "critical", "last-run-failed"},
		{"warning", "WARNINGS: backup completed OK with skipped chunks", "warning", "last-run-warning"},
		{"running", "running", "unknown", "running"},
		{"unknown", "not OK", "unknown", "last-run-state-not-ok"},
		{"recovered", "OK", "healthy", "last-run-success"},
		{"failed-again", "ERROR: authentication token rejected", "critical", "last-run-failed"},
	} {
		status.Store(phase.status)
		monitor.pollPBSInstance(context.Background(), instance.Name, client)
		snapshot := monitor.state.GetSnapshot()
		if len(snapshot.PBSInstances) != 1 {
			t.Fatalf("missing PBS instance: %+v", snapshot.PBSInstances)
		}
		raw := snapshot.PBSInstances[0]
		if len(raw.JobHealthEvidence) != 2 || raw.Status != "online" || raw.ConnectionHealth != "healthy" {
			t.Fatalf("job failure changed transport health or evidence count: %+v", raw)
		}
		for _, evidence := range raw.JobHealthEvidence {
			switch evidence.ID {
			case "sync-main":
				if evidence.Posture != phase.posture || evidence.PostureReason != phase.reason || evidence.TaskStatus != phase.status || evidence.LastRunState != phase.status || evidence.UPID != "UPID:sync:1" || evidence.Confidence != pbs.JobEvidenceConfidenceDirectTaskMatch || evidence.EvidenceSource != pbs.JobEvidenceSourcePBSJobConfig || evidence.EvidenceScope != pbs.JobEvidenceScopeConfiguredJob || evidence.LastRunEndtime != now.Unix() {
					t.Errorf("%s: raw evidence = %+v, want %s/%s with unchanged source facts", phase.name, evidence, phase.posture, phase.reason)
				}
			case "verify-control":
				if evidence.Posture != "healthy" || evidence.PostureReason != "last-run-success" || evidence.TaskStatus != "OK" {
					t.Errorf("%s: independent successful verification changed: %+v", phase.name, evidence)
				}
			default:
				t.Errorf("unexpected evidence: %+v", evidence)
			}
		}
		adapter.PopulateFromSnapshot(snapshot)
		resources := adapter.GetAll()
		if len(resources) != 1 || resources[0].PBS == nil {
			t.Fatalf("unexpected resources: %+v", resources)
		}
		resource := resources[0]
		if identity != "" && resource.ID != identity {
			t.Fatal("resource identity changed with job status")
		}
		identity = resource.ID
		projected, _ := json.Marshal(resource.PBS.JobHealthEvidence)
		original, _ := json.Marshal(raw.JobHealthEvidence)
		if string(projected) != string(original) {
			t.Fatal("Resources projection changed PBS evidence")
		}
		if resource.PBS.JobHealthEvidenceCount != 2 {
			t.Fatal("Resources projection lost evidence count")
		}
		exports[phase.name] = map[string]any{"resources": resources, "total": len(resources)}
	}
	if mutations.Load() != 0 {
		t.Fatal("fixture received a mutation")
	}
	if dir := os.Getenv("PULSE_PBS_JOB_HEALTH_EXPORT"); dir != "" {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		data, err := json.MarshalIndent(exports, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "producer.json"), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
