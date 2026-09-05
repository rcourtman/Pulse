package monitoring

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
)

func TestPollPBSInstanceDoesNotQueryExcludedDatastoreDetails(t *testing.T) {
	var mu sync.Mutex
	var requestedPaths []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestedPaths = append(requestedPaths, r.URL.Path)
		mu.Unlock()

		switch r.URL.Path {
		case "/api2/json/version":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"version": "3.4"},
			})
		case "/api2/json/nodes/localhost/status":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{}})
		case "/api2/json/admin/datastore":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]string{
					{"store": "internal"},
					{"store": "exthdd1500gb"},
				},
			})
		case "/api2/json/admin/datastore/internal/rrd":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
		case "/api2/json/admin/datastore/internal/status":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"total": 100.0,
					"used":  25.0,
					"avail": 75.0,
				},
			})
		case "/api2/json/admin/datastore/internal/gc":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{}})
		case "/api2/json/admin/datastore/internal/namespace":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
		default:
			http.Error(w, "unexpected request", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	client, err := pbs.NewClient(pbs.ClientConfig{
		Host:       server.URL,
		TokenName:  "root@pam!pulse-token",
		TokenValue: "secret",
	})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	monitor := &Monitor{
		config: &config.Config{
			PBSInstances: []config.PBSInstance{{
				Name:              "pbs-excludes",
				Host:              server.URL,
				MonitorDatastores: true,
				ExcludeDatastores: []string{"ext*"},
			}},
		},
		state:           models.NewState(),
		authFailures:    make(map[string]int),
		lastAuthAttempt: make(map[string]time.Time),
		pollStatusMap:   make(map[string]*pollStatus),
		circuitBreakers: make(map[string]*circuitBreaker),
	}

	monitor.pollPBSInstance(context.Background(), "pbs-excludes", client)

	snapshot := monitor.state.GetSnapshot()
	if len(snapshot.PBSInstances) != 1 {
		t.Fatalf("PBS instances = %+v, want one", snapshot.PBSInstances)
	}
	if snapshot.PBSInstances[0].ID != PBSMonitorResourceID("pbs-excludes") {
		t.Fatalf("PBS runtime identity = %q, want canonical monitor identity", snapshot.PBSInstances[0].ID)
	}
	datastores := snapshot.PBSInstances[0].Datastores
	if len(datastores) != 1 || datastores[0].Name != "internal" {
		t.Fatalf("datastores = %+v, want only internal", datastores)
	}

	mu.Lock()
	defer mu.Unlock()
	for _, path := range requestedPaths {
		if path == "/api2/json/nodes" {
			t.Fatalf("API-token poll queried the superuser-only node-name endpoint: %s", path)
		}
		if strings.Contains(path, "exthdd1500gb") {
			t.Fatalf("excluded datastore received a detail request: %s", path)
		}
	}
}

// The hostname a PBS node reports about itself (GET /nodes) is
// machine-identity evidence for connected-system grouping: it merges the
// host agent running on the PBS machine into the PBS connection even when
// the connection was configured by IP or DNS alias. The fetch is optional
// collection — capture it when available, never fail the poll on it.
func TestPollPBSInstanceCapturesReportedNodeName(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/version":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"version": "3.4"},
			})
		case "/api2/json/nodes":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{{"node": "pbs01"}},
			})
		case "/api2/json/nodes/localhost/status":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{}})
		default:
			http.Error(w, "unexpected request", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	client, err := pbs.NewClient(pbs.ClientConfig{Host: server.URL, User: "root@pam"})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	monitor := &Monitor{
		config: &config.Config{
			PBSInstances: []config.PBSInstance{{
				Name: "pbs-nodename",
				Host: server.URL,
			}},
		},
		state:           models.NewState(),
		authFailures:    make(map[string]int),
		lastAuthAttempt: make(map[string]time.Time),
		pollStatusMap:   make(map[string]*pollStatus),
		circuitBreakers: make(map[string]*circuitBreaker),
	}

	monitor.pollPBSInstance(context.Background(), "pbs-nodename", client)

	snapshot := monitor.state.GetSnapshot()
	if len(snapshot.PBSInstances) != 1 {
		t.Fatalf("PBS instances = %+v, want one", snapshot.PBSInstances)
	}
	instance := snapshot.PBSInstances[0]
	if instance.NodeName != "pbs01" {
		t.Fatalf("NodeName = %q, want %q", instance.NodeName, "pbs01")
	}
	if instance.Status != "online" || instance.ConnectionHealth != "healthy" {
		t.Fatalf("status = %q health = %q, want online/healthy", instance.Status, instance.ConnectionHealth)
	}
}

func TestMonitor_PollPBSInstance_AuthFailure(t *testing.T) {
	// Setup mock server that returns 401
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	// Setup client
	client, err := pbs.NewClient(pbs.ClientConfig{
		Host:       server.URL,
		TokenName:  "root@pam!token",
		TokenValue: "secret",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Setup monitor
	m := &Monitor{
		config: &config.Config{
			PBSInstances: []config.PBSInstance{
				{Name: "pbs-auth-fail", Host: server.URL, MonitorDatastores: true},
			},
		},
		state:           models.NewState(),
		authFailures:    make(map[string]int),
		lastAuthAttempt: make(map[string]time.Time),
		pollStatusMap:   make(map[string]*pollStatus),
		circuitBreakers: make(map[string]*circuitBreaker),
		// We need connectionHealth map initialized if SetConnectionHealth uses it?
		// models.NewState() handles it.
	}

	// Execute
	ctx := context.Background()
	m.pollPBSInstance(ctx, "pbs-auth-fail", client)

	// Verify
	// status should be offline
	// recordAuthFailure should have been called?
	// Monitor stores auth failures in memory map `authFailures`.
	// We can check `m.state.ConnectionHealth` for "pbs-pbs-auth-fail".

	// Verify manually using snapshot
	snapshot := m.state.GetSnapshot()
	if snapshot.ConnectionHealth["pbs-pbs-auth-fail"] {
		t.Error("Expected connection health to be false")
	}
	if len(snapshot.PBSInstances) != 1 {
		t.Fatalf("expected failed PBS poll to refresh one dashboard projection, got %+v", snapshot.PBSInstances)
	}
	if instance := snapshot.PBSInstances[0]; instance.Status != "offline" || instance.ConnectionHealth != "error" {
		t.Fatalf("failed PBS dashboard projection = %+v, want offline/error", instance)
	}

	// Regression: pollStatusMap must record the failure. A defer-arg bug
	// previously captured pollErr at register-time (always nil), so failed
	// polls were recorded as success and the connections aggregator reported
	// broken instances as healthy.
	if status := m.pollStatusMap["pbs::pbs-auth-fail"]; status == nil {
		t.Fatal("expected pollStatusMap entry for pbs::pbs-auth-fail, got nil")
	} else {
		if !status.LastSuccess.IsZero() {
			t.Errorf("expected LastSuccess to remain zero on auth failure, got %v", status.LastSuccess)
		}
		if status.ConsecutiveFailures == 0 {
			t.Error("expected ConsecutiveFailures > 0 after auth failure, got 0")
		}
	}

	// We can't easily check authFailures map as it is private and no getter (except checking if it backs off?)
}

func TestMonitor_PollPBSInstance_DatastoreDetails(t *testing.T) {
	// Setup mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/version") {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]interface{}{"version": "2.0"},
			})
			return
		}
		if strings.Contains(r.URL.Path, "/nodes/localhost/status") {
			// Fail node status
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if strings.Contains(r.URL.Path, "/admin/datastore") && strings.HasSuffix(r.URL.Path, "/admin/datastore") {
			// GetDatastores list
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": []map[string]interface{}{
					{"store": "ds1", "comment": "comment1"}, // GetDatastores list returns small subset of fields
					{"store": "ds2", "comment": "comment2"},
					{"store": "ds-error", "comment": "status failure"},
				},
			})
			return
		}

		if strings.Contains(r.URL.Path, "/status") {
			// Datastore Status
			var data map[string]interface{}
			if strings.Contains(r.URL.Path, "ds1") {
				data = map[string]interface{}{"total": 100.0, "used": 50.0, "avail": 50.0}
			} else if strings.Contains(r.URL.Path, "ds2") {
				data = map[string]interface{}{"total-space": 200.0, "used-space": 100.0, "avail-space": 100.0, "deduplication-factor": 1.5, "status": "read_only"}
			} else if strings.Contains(r.URL.Path, "ds-error") {
				http.Error(w, "datastore unavailable", http.StatusInternalServerError)
				return
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"data": data})
			return
		}

		if strings.Contains(r.URL.Path, "/rrd") {
			// RRD
			json.NewEncoder(w).Encode(map[string]interface{}{"data": []interface{}{}})
			return
		}

		if strings.Contains(r.URL.Path, "/namespace") {
			// ListNamespaces
			if strings.Contains(r.URL.Path, "ds1") {
				// DS 1: Fail namespaces
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			if strings.Contains(r.URL.Path, "ds2") {
				// DS 2: Varied namespaces
				json.NewEncoder(w).Encode(map[string]interface{}{
					"data": []map[string]interface{}{
						{"ns": "ns1"},
						{"path": "ns2"}, // alternate field
						{"name": "ns3"}, // alternate field
					},
				})
				return
			}
		}

		// Catch-all success for rrd/status calls from client.GetDatastores (it calls internal methods)
		// Wait, client.GetDatastores calls /api2/json/admin/datastore
		// client.ListNamespaces calls /api2/json/admin/datastore/{store}/namespace?
		// No, client.ListNamespaces: req to /admin/datastore/%s/namespace

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"data": []interface{}{}})
	}))
	defer server.Close()

	client, err := pbs.NewClient(pbs.ClientConfig{Host: server.URL, TokenName: "root@pam!token", TokenValue: "val"})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	m := &Monitor{
		config: &config.Config{
			PBSInstances: []config.PBSInstance{
				{Name: "pbs-details", Host: server.URL, MonitorDatastores: true},
			},
		},
		state:           models.NewState(),
		authFailures:    make(map[string]int),
		lastAuthAttempt: make(map[string]time.Time),
		pollStatusMap:   make(map[string]*pollStatus),
		circuitBreakers: make(map[string]*circuitBreaker),
	}

	m.pollPBSInstance(context.Background(), "pbs-details", client)

	// Verify State
	snapshot := m.state.GetSnapshot()
	var inst *models.PBSInstance
	for _, i := range snapshot.PBSInstances {
		if i.Name == "pbs-details" {
			copy := i
			inst = &copy
			break
		}
	}

	if inst == nil {
		t.Fatal("Instance not found")
	}

	if len(inst.Datastores) != 3 {
		t.Errorf("Expected 3 datastores, got %d", len(inst.Datastores))
	}

	// Check DS2 size calculation and status propagation.
	var ds2 *models.PBSDatastore
	var dsError *models.PBSDatastore
	for _, ds := range inst.Datastores {
		if ds.Name == "ds2" {
			copy := ds
			ds2 = &copy
		}
		if ds.Name == "ds-error" {
			copy := ds
			dsError = &copy
		}
	}
	if ds2 != nil {
		if ds2.Total != 200 {
			t.Errorf("Expected DS2 total 200, got %d", ds2.Total)
		}
		if ds2.Status != "read_only" {
			t.Errorf("Expected DS2 status read_only, got %q", ds2.Status)
		}
		if len(ds2.Namespaces) != 4 {
			t.Errorf("Expected 4 namespaces for DS2, got %d", len(ds2.Namespaces))
		}
	} else {
		t.Error("DS2 not found")
	}
	if dsError == nil {
		t.Error("ds-error not found")
	} else {
		if dsError.Status != "unavailable" {
			t.Errorf("Expected ds-error status unavailable, got %q", dsError.Status)
		}
		if !strings.Contains(dsError.Error, "Failed to get status") {
			t.Errorf("Expected ds-error to preserve status error, got %q", dsError.Error)
		}
	}
}

func TestPBSJobHealthEvidenceKeepsFreshnessSeparateFromConfidence(t *testing.T) {
	observedAt := time.Unix(1700007200, 0).UTC()
	facts := []pbs.JobHealthEvidence{
		{
			ID:             "sync-remote-a",
			Family:         "sync",
			Store:          "fast",
			LastRunState:   "OK",
			LastRunUPID:    "UPID:sync:1",
			LastRunEndtime: 1700000000,
			NextRun:        1700003600,
			UPID:           "UPID:sync:1",
			TaskStatus:     "OK",
			Confidence:     "direct-task-match",
			EvidenceSource: pbs.JobEvidenceSourcePBSJobConfig,
			EvidenceScope:  pbs.JobEvidenceScopeConfiguredJob,
		},
	}

	evidence := pbsJobHealthEvidenceFromFacts(facts, observedAt)
	if len(evidence) != 1 {
		t.Fatalf("expected 1 evidence item, got %d", len(evidence))
	}
	got := evidence[0]
	if got.Confidence != "direct-task-match" {
		t.Fatalf("confidence = %q, want direct-task-match", got.Confidence)
	}
	if got.EvidenceSource != pbs.JobEvidenceSourcePBSJobConfig || got.EvidenceScope != pbs.JobEvidenceScopeConfiguredJob {
		t.Fatalf("evidence source/scope = %q/%q, want configured PBS job", got.EvidenceSource, got.EvidenceScope)
	}
	if got.Freshness.State != "overdue" {
		t.Fatalf("freshness state = %q, want overdue", got.Freshness.State)
	}
	if got.Posture != "warning" || got.PostureReason != "job-overdue" {
		t.Fatalf("posture = %q/%q, want warning/job-overdue", got.Posture, got.PostureReason)
	}
	if got.LastRunState != "OK" || got.LastRunUPID != "UPID:sync:1" || got.LastRunEndtime != 1700000000 || got.NextRun != 1700003600 {
		t.Fatalf("raw PBS last-run fields were not preserved: %+v", got)
	}
}

func TestPBSJobHealthEvidenceLabelsBackupTasksAsObservedOnly(t *testing.T) {
	observedAt := time.Unix(1700007200, 0).UTC()
	facts := []pbs.JobHealthEvidence{
		{
			ID:             "backup:vm/100",
			Family:         "backup",
			Store:          "fast",
			UPID:           "UPID:backup:1",
			WorkerType:     "backup",
			WorkerID:       "vm/100",
			TaskStatus:     "OK",
			TaskStartTime:  1700000000,
			TaskEndTime:    1700000060,
			Confidence:     pbs.JobEvidenceConfidenceObservedBackupTask,
			EvidenceSource: pbs.JobEvidenceSourcePBSTaskHistory,
			EvidenceScope:  pbs.JobEvidenceScopeObservedTask,
		},
	}

	evidence := pbsJobHealthEvidenceFromFacts(facts, observedAt)
	if len(evidence) != 1 {
		t.Fatalf("expected 1 evidence item, got %d", len(evidence))
	}
	got := evidence[0]
	if got.Family != "backup" || got.Confidence != pbs.JobEvidenceConfidenceObservedBackupTask {
		t.Fatalf("expected observed backup task evidence, got %+v", got)
	}
	if got.EvidenceSource != pbs.JobEvidenceSourcePBSTaskHistory || got.EvidenceScope != pbs.JobEvidenceScopeObservedTask {
		t.Fatalf("evidence source/scope = %q/%q, want PBS task-history observed-task", got.EvidenceSource, got.EvidenceScope)
	}
	if got.Schedule != "" || got.NextRun != 0 || !got.Freshness.NextRun.IsZero() || got.Freshness.State == "scheduled" {
		t.Fatalf("backup task evidence must not become scheduled backup compliance: %+v", got)
	}
}

// TestPBSAndPMGPollSkipDisabledInstances asserts that the PBS and PMG poll
// entry points short-circuit when their resolved instance config carries
// `Disabled: true`. This is a source-level guardrail for the discovery
// provider surface: the unified connections ledger surfaces `Disabled` as
// `paused`, and the PBS/PMG pollers must not drive live API calls or
// surface ingest while that flag is set, across restarts or reloads.
func TestPBSAndPMGPollSkipDisabledInstances(t *testing.T) {
	data, err := os.ReadFile("monitor_pbs_pmg.go")
	if err != nil {
		t.Fatalf("failed to read monitor_pbs_pmg.go: %v", err)
	}
	source := string(data)

	// Both PBS and PMG poll flows must explicitly guard on Disabled.
	if count := strings.Count(source, "if instanceCfg.Disabled {"); count < 2 {
		t.Fatalf("monitor_pbs_pmg.go must contain the Disabled-skip guard in both PBS and PMG poll entry points; found %d", count)
	}

	// The guards must short-circuit the poll with an early return so no
	// downstream API client is constructed for a paused instance.
	for _, snippet := range []string{
		"Skipping PBS poll: instance is paused",
		"Skipping PMG poll: instance is paused",
	} {
		if !strings.Contains(source, snippet) {
			t.Fatalf("monitor_pbs_pmg.go must emit debug-log %q when skipping a disabled instance so operators can correlate paused ledger rows with runtime behavior", snippet)
		}
	}
}

// Exercise real HTTP polling and one manager through missing metrics and recovery.
func TestPBSMetricAvailabilityAlertLifecycle(t *testing.T) {
	fixture := newPBSHealthTestServer(t)
	instance := config.PBSInstance{Name: "pbs-lifecycle", Host: fixture.server.URL, MonitorDatastores: true}
	monitor := newPBSHealthAuthorityMonitor([]config.PBSInstance{instance})
	client := newPBSHealthTestClient(t, instance.Host)
	manager := alerts.NewManagerWithDataDir(t.TempDir())
	defer manager.Stop()
	monitor.alertManager = manager
	manager.UpdateConfig(alerts.AlertConfig{Enabled: true, ActivationState: alerts.ActivationActive,
		TimeThresholds: map[string]int{"pbs": 0}, PBSDefaults: alerts.ThresholdConfig{
			Memory: &alerts.HysteresisThreshold{Trigger: 40, Clear: 30},
		}})
	fired, resolved := make(chan string, 8), make(chan string, 8)
	manager.SetAlertCallback(func(a *alerts.Alert) { fired <- a.ID })
	manager.SetResolvedCallback(func(id string) { resolved <- id })
	poll := func() models.PBSInstance {
		monitor.pollPBSInstance(context.Background(), instance.Name, client)
		return pbsInstanceByName(t, monitor.state.GetSnapshot(), instance.Name)
	}
	poll()
	if len(manager.GetActiveAlerts()) != 1 {
		t.Fatal("high memory failed to activate")
	}
	select {
	case <-fired:
	case <-time.After(time.Second):
		t.Fatal("missing alert dispatch")
	}
	for _, mode := range []pbsHealthTestMode{pbsHealthTestNodeDenied, pbsHealthTestNodeGatewayFailure} {
		fixture.setMode(mode)
		missing := poll()
		if missing.Status != "online" || !missing.NodeMetricsUnavailable {
			t.Fatalf("bad missing projection: %+v", missing)
		}
		for range 5 {
			poll()
		}
		if len(manager.GetActiveAlerts()) != 1 {
			t.Fatal("endpoint failure resolved memory alert")
		}
		if len(manager.GetRecentlyResolved()) != 0 {
			t.Fatal("false resolved history")
		}
		select {
		case id := <-resolved:
			t.Fatalf("false recovery dispatch %s", id)
		case <-time.After(50 * time.Millisecond):
		}
	}
	fixture.setMode(pbsHealthTestLowMemory)
	for range 5 {
		poll()
	}
	if len(manager.GetActiveAlerts()) != 0 {
		t.Fatal("valid low memory failed to recover")
	}
	select {
	case <-resolved:
	case <-time.After(time.Second):
		t.Fatal("missing recovery dispatch")
	}
	if len(manager.GetRecentlyResolved()) != 1 {
		t.Fatal("missing resolved history")
	}
}
