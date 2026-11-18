package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
)

// Drive the real PBS poll, retained failure record and canonical datastore
// incident from the same responses the Backup Servers table consumes.
func TestPBSConfiguredJobInventoryRetainsFailureUntilReadable(t *testing.T) {
	const (
		failed = iota
		unread
		recovered
		removed
	)
	replies := []string{
		`{"data":[{"id":"copy-offsite","store":"offsite","remote":"primary","last-run-state":"connection error","last-run-upid":"UPID:sync:1","last-run-endtime":1794960000}]}`,
		`{"data":[],"errors":{"status":"incomplete listing"}}`,
		`{"data":[{"id":"copy-offsite","store":"offsite","remote":"primary","last-run-state":"OK","last-run-upid":"UPID:sync:2","last-run-endtime":1794960300}]}`,
		`{"data":[]}`,
	}
	var mode atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet {
			t.Errorf("job inventory made a %s request", req.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		var response string
		switch req.URL.Path {
		case "/api2/json/version":
			response = `{"data":{"version":"4.2"}}`
		case "/api2/json/nodes/localhost/status":
			response = `{"data":{"cpu":0.15,"memory":{"used":4096,"total":8192},"uptime":120}}`
		case "/api2/json/admin/datastore":
			response = `{"data":[{"store":"offsite"},{"store":"onsite"}]}`
		case "/api2/json/admin/datastore/offsite/status", "/api2/json/admin/datastore/onsite/status":
			response = `{"data":{"total":2199023255552,"used":659706976666,"avail":1539316278886}}`
		case "/api2/json/admin/datastore/offsite/gc", "/api2/json/admin/datastore/onsite/gc":
			response = `{"data":{}}`
		case "/api2/json/admin/datastore/offsite/namespace", "/api2/json/admin/datastore/onsite/namespace",
			"/api2/json/admin/datastore/offsite/rrd", "/api2/json/admin/datastore/onsite/rrd",
			"/api2/json/nodes/localhost/tasks":
			response = `{"data":[]}`
		case "/api2/json/admin/sync":
			response = replies[mode.Load()]
		case "/api2/json/admin/verify":
			response = `{"data":[{"id":"verify-offsite","store":"offsite","last-run-state":"OK"},{"id":"verify-onsite","store":"onsite","last-run-state":"OK"}]}`
		default:
			t.Errorf("unexpected PBS request %q", req.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = fmt.Fprint(w, response)
	}))
	t.Cleanup(server.Close)

	const name = "backup-vault"
	host := server.URL
	instance := config.PBSInstance{
		Name: name, Host: host, MonitorDatastores: true,
		MonitorSyncJobs: true, MonitorVerifyJobs: true,
	}
	client, err := pbs.NewClient(pbs.ClientConfig{
		Host: host, TokenName: "fixture@pbs!read", TokenValue: "fixture-only", Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	newMonitor := func() *Monitor {
		m := newPBSHealthAuthorityMonitor([]config.PBSInstance{instance})
		m.pbsJobFailureStore = config.NewPBSJobFailureStore(dir, nil)
		return m
	}
	m := newMonitor()
	registry := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(unifiedresources.NewMemoryStore()))
	manager := alerts.NewManagerWithDataDir(t.TempDir())
	t.Cleanup(manager.Stop)
	alertConfig := manager.GetConfig()
	alertConfig.Enabled = true
	alertConfig.ActivationState = alerts.ActivationActive
	alertConfig.Schedule.QuietHours.Enabled = false
	manager.UpdateConfig(alertConfig)
	var originalObservedAt time.Time
	var originalIncidentID string
	poll := func(label string, phase int32, want string, wantFailure, wantAlert, export bool) {
		t.Helper()
		mode.Store(phase)
		m.pollPBSInstance(context.Background(), name, client)
		snapshot := m.state.GetSnapshot()
		result := pbsInstanceByName(t, snapshot, name)
		if result.ConnectionHealth != "healthy" || len(result.Datastores) != 2 {
			t.Fatalf("%s: unrelated provider reads changed: connection %s, datastores %d", label, result.ConnectionHealth, len(result.Datastores))
		}
		if got := postureOf(result.JobHealthEvidence, "sync", "copy-offsite"); got != want {
			t.Errorf("%s: sync posture = %s, want %s", label, got, want)
		}
		for _, id := range []string{"verify-onsite", "verify-offsite"} {
			if got := postureOf(result.JobHealthEvidence, "verify", id); got != "healthy/last-run-success" {
				t.Errorf("%s: readable sibling %s = %s", label, id, got)
			}
		}
		partial := false
		for _, fact := range result.JobHealthEvidence {
			switch fact.ID {
			case "sync:partial":
				partial = fact.EvidenceScope == pbs.JobEvidenceScopePartialRead &&
					fact.Confidence == pbs.JobEvidenceConfidencePartialError &&
					!strings.Contains(fact.Error, "incomplete listing")
			case "copy-offsite":
				if label == "failed" {
					originalObservedAt = fact.Freshness.ObservedAt
				}
				if phase == unread {
					// The persisted failure carries the run clock, not the
					// in-memory observation time. A restart cannot renew either.
					wantObservation := originalObservedAt
					if label == "restart-unread" {
						wantObservation = time.Time{}
					}
					if !fact.Freshness.ObservedAt.Equal(wantObservation) ||
						fact.LastRunEndtime != 1794960000 || fact.LastRunUPID != "UPID:sync:1" {
						t.Errorf("%s: unread listing renewed or rewrote the original run: %+v", label, fact)
					}
				}
			}
		}
		if partial != (phase == unread) {
			t.Errorf("%s: family-scoped partial evidence = %v", label, partial)
		}

		// Publish a new generation, the same path the monitor uses for
		// independent snapshot observations (not an additive resource merge).
		registry.PopulateFromSnapshot(snapshot)
		resources := registry.GetAll()
		manager.SyncUnifiedResourceIncidents(resources)
		var active []alerts.Alert
		for _, alert := range manager.GetActiveAlerts() {
			if alert.Type == "backup-storage-incident" {
				active = append(active, alert)
			}
		}
		if (wantAlert && len(active) != 1) || (!wantAlert && len(active) != 0) ||
			(len(active) == 1 && active[0].ResourceName != "offsite") {
			t.Errorf("%s: active offsite job alert = %+v, want active = %t", label, active, wantAlert)
		}
		var incidents []unifiedresources.ResourceIncident
		var servers []unifiedresources.Resource
		for _, row := range resources {
			if row.Type == unifiedresources.ResourceTypePBS {
				servers = append(servers, row)
			}
			for _, incident := range row.Incidents {
				if incident.Code == unifiedresources.PBSJobFailedIncidentCode {
					incidents = append(incidents, incident)
					if row.Name != "offsite" {
						t.Errorf("%s: failed sync was attributed to %q", label, row.Name)
					}
				}
			}
		}
		if (wantFailure && len(incidents) != 1) || (!wantFailure && len(incidents) != 0) {
			t.Errorf("%s: canonical PBS job incidents = %d, want failure = %t", label, len(incidents), wantFailure)
		}
		if len(incidents) == 1 {
			if label == "failed" {
				originalIncidentID = incidents[0].NativeID
			}
			if incidents[0].NativeID != originalIncidentID || incidents[0].StartedAt.Unix() != 1794960000 {
				t.Errorf("%s: canonical incident lost its original identity/clock: %+v", label, incidents[0])
			}
		}
		persisted := config.NewPBSJobFailureStore(dir, nil).Get(result.ID, host)
		if (wantFailure && len(persisted) != 1) || (!wantFailure && len(persisted) != 0) {
			t.Errorf("%s: reopened record has %d failures, want failure = %t", label, len(persisted), wantFailure)
		}
		if export && os.Getenv("PULSE_PBS_JOB_INVENTORY_FRAMES") == "1" {
			if len(servers) != 1 || servers[0].PBS == nil {
				t.Fatalf("%s: missing canonical PBS data", label)
			}
			data, err := json.Marshal(servers)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("PBS_JOB_FRAME %s %s", label, data)
		}
	}

	const failing = "critical/last-run-failed"
	const success = "healthy/last-run-success"
	poll("failed", failed, failing, true, true, true)
	poll("unread", unread, failing, true, true, true)
	poll("still-unread", unread, failing, true, true, false)
	m = newMonitor()
	poll("restart-unread", unread, failing, true, true, false)
	poll("recovered", recovered, success, false, true, true)
	poll("confirmed-success", recovered, success, false, false, false)
	poll("failed-again", failed, failing, true, true, false)
	poll("removed", removed, "absent", false, true, true)
	poll("confirmed-removal", removed, "absent", false, false, false)
}
