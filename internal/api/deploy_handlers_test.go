package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/deploy"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
)

// newTestDeployHandlers creates a DeployHandlers with minimal dependencies for testing.
func newTestDeployHandlers(t *testing.T, nodes []models.Node, hosts []models.Host) *DeployHandlers {
	t.Helper()

	// Open a temp deploy store.
	dbPath := t.TempDir() + "/deploy.db"
	store, err := deploy.Open(dbPath)
	if err != nil {
		t.Fatalf("open deploy store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	// Create a monitor with state.
	monitor, state := newDeployTestMonitor(t)
	for _, n := range nodes {
		state.Nodes = append(state.Nodes, n)
	}
	for _, h := range hosts {
		state.UpsertHost(h)
	}

	execServer := agentexec.NewServer(func(string, string) bool { return true })
	reservation := deploy.NewReservationManager()

	return NewDeployHandlers(store, monitor, execServer, reservation, func(_ *http.Request) string {
		return "http://10.0.0.1:7655"
	})
}

// newDeployTestMonitor creates a minimal monitor for deploy handler tests.
func newDeployTestMonitor(t *testing.T) (*monitoring.Monitor, *models.State) {
	t.Helper()
	monitor := &monitoring.Monitor{}
	state := models.NewState()
	setUnexportedField(t, monitor, "state", state)
	return monitor, state
}

func TestHandleCandidatesMethod(t *testing.T) {
	h := newTestDeployHandlers(t, nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/clusters/lab/agent-deploy/candidates", nil)
	rec := httptest.NewRecorder()

	h.HandleCandidates(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}

func TestHandleCandidatesMissingClusterID(t *testing.T) {
	h := newTestDeployHandlers(t, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/clusters//agent-deploy/candidates", nil)
	rec := httptest.NewRecorder()

	h.HandleCandidates(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestHandleCandidatesEmptyCluster(t *testing.T) {
	h := newTestDeployHandlers(t, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/clusters/nonexistent/agent-deploy/candidates", nil)
	rec := httptest.NewRecorder()

	h.HandleCandidates(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d. Body: %s", rec.Code, rec.Body.String())
	}

	var resp candidatesResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Nodes) != 0 {
		t.Errorf("expected 0 nodes, got %d", len(resp.Nodes))
	}
}

func TestHandleCandidatesWithCluster(t *testing.T) {
	nodes := []models.Node{
		{
			ID: "node_pve-a", Name: "pve-a", Host: "https://10.0.0.1:8006",
			IsClusterMember: true, ClusterName: "lab",
			LinkedHostAgentID: "host-a",
		},
		{
			ID: "node_pve-b", Name: "pve-b", Host: "https://10.0.0.2:8006",
			IsClusterMember: true, ClusterName: "lab",
		},
		{
			ID: "node_pve-c", Name: "pve-c", Host: "https://10.0.0.3:8006",
			IsClusterMember: true, ClusterName: "lab",
		},
		{
			ID: "node_standalone", Name: "standalone", Host: "https://10.0.0.4:8006",
			IsClusterMember: false,
		},
	}

	h := newTestDeployHandlers(t, nodes, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/clusters/lab/agent-deploy/candidates", nil)
	rec := httptest.NewRecorder()

	h.HandleCandidates(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d. Body: %s", rec.Code, rec.Body.String())
	}

	var resp candidatesResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.ClusterName != "lab" {
		t.Errorf("expected clusterName 'lab', got %q", resp.ClusterName)
	}
	if len(resp.Nodes) != 3 {
		t.Fatalf("expected 3 cluster nodes, got %d", len(resp.Nodes))
	}

	// pve-a has agent — not deployable.
	var foundA, foundB bool
	for _, n := range resp.Nodes {
		switch n.Name {
		case "pve-a":
			foundA = true
			if n.Deployable {
				t.Error("pve-a should not be deployable (has agent)")
			}
			if n.Reason != "already_agent" {
				t.Errorf("pve-a reason: want 'already_agent', got %q", n.Reason)
			}
		case "pve-b":
			foundB = true
			if !n.Deployable {
				t.Error("pve-b should be deployable")
			}
			if n.IP != "10.0.0.2" {
				t.Errorf("pve-b IP: want '10.0.0.2', got %q", n.IP)
			}
		}
	}
	if !foundA || !foundB {
		t.Error("missing expected nodes in response")
	}

	// Standalone node should be excluded.
	for _, n := range resp.Nodes {
		if n.Name == "standalone" {
			t.Error("standalone node should not appear in cluster candidates")
		}
	}
}

func TestHandleCreatePreflightMissingBody(t *testing.T) {
	h := newTestDeployHandlers(t, nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/clusters/lab/agent-deploy/preflights", strings.NewReader(""))
	rec := httptest.NewRecorder()

	h.HandleCreatePreflight(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d. Body: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleCreatePreflightMissingSourceAgent(t *testing.T) {
	h := newTestDeployHandlers(t, nil, nil)
	body := `{"targetNodeIds":["node_b"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/clusters/lab/agent-deploy/preflights", strings.NewReader(body))
	rec := httptest.NewRecorder()

	h.HandleCreatePreflight(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d. Body: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleCreatePreflightSourceOffline(t *testing.T) {
	h := newTestDeployHandlers(t, nil, nil)
	body := `{"sourceAgentId":"agent-1","targetNodeIds":["node_b"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/clusters/lab/agent-deploy/preflights", strings.NewReader(body))
	rec := httptest.NewRecorder()

	h.HandleCreatePreflight(rec, req)

	// Source agent is not connected — expect 409.
	if rec.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d. Body: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleCreatePreflightTooManyTargets(t *testing.T) {
	h := newTestDeployHandlers(t, nil, nil)
	// Build 101 target node IDs.
	ids := make([]string, 101)
	for i := range ids {
		ids[i] = `"node_` + string(rune('a'+i%26)) + `"`
	}
	body := `{"sourceAgentId":"agent-1","targetNodeIds":[` + strings.Join(ids, ",") + `]}`
	req := httptest.NewRequest(http.MethodPost, "/api/clusters/lab/agent-deploy/preflights", strings.NewReader(body))
	rec := httptest.NewRecorder()

	h.HandleCreatePreflight(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d. Body: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleGetPreflightNotFound(t *testing.T) {
	h := newTestDeployHandlers(t, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/agent-deploy/preflights/nonexistent", nil)
	rec := httptest.NewRecorder()

	h.HandleGetPreflight(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d. Body: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleGetPreflightFound(t *testing.T) {
	h := newTestDeployHandlers(t, nil, nil)

	// Create a job directly in the store.
	now := time.Now().UTC()
	job := &deploy.Job{
		ID:            "pf_test1",
		ClusterID:     "lab",
		ClusterName:   "lab",
		SourceAgentID: "agent-1",
		SourceNodeID:  "node_a",
		OrgID:         "default",
		Status:        deploy.JobRunning,
		MaxParallel:   2,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := h.store.CreateJob(context.Background(), job); err != nil {
		t.Fatalf("create job: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/agent-deploy/preflights/pf_test1", nil)
	rec := httptest.NewRecorder()

	h.HandleGetPreflight(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d. Body: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		ID      string `json:"id"`
		Status  string `json:"status"`
		Targets []any  `json:"targets"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.ID != "pf_test1" {
		t.Errorf("expected id 'pf_test1', got %q", resp.ID)
	}
	if resp.Status != "running" {
		t.Errorf("expected status 'running', got %q", resp.Status)
	}
}

func TestHandlePreflightEventsNotFound(t *testing.T) {
	h := newTestDeployHandlers(t, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/agent-deploy/preflights/nonexistent/events", nil)
	rec := httptest.NewRecorder()

	h.HandlePreflightEvents(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d. Body: %s", rec.Code, rec.Body.String())
	}
}

func TestExtractClusterID(t *testing.T) {
	tests := []struct {
		path, prefix, suffix string
		want                 string
	}{
		{"/api/clusters/lab/agent-deploy/candidates", "/api/clusters/", "/agent-deploy/candidates", "lab"},
		{"/api/clusters/my-cluster/agent-deploy/preflights", "/api/clusters/", "/agent-deploy/preflights", "my-cluster"},
		{"/api/clusters//agent-deploy/candidates", "/api/clusters/", "/agent-deploy/candidates", ""},
	}
	for _, tt := range tests {
		got := extractClusterID(tt.path, tt.prefix, tt.suffix)
		if got != tt.want {
			t.Errorf("extractClusterID(%q, %q, %q) = %q, want %q", tt.path, tt.prefix, tt.suffix, got, tt.want)
		}
	}
}

func TestExtractPathSuffix(t *testing.T) {
	tests := []struct {
		path, prefix, want string
	}{
		{"/api/agent-deploy/preflights/pf_123", "/api/agent-deploy/preflights/", "pf_123"},
		{"/api/agent-deploy/preflights/pf_123/events", "/api/agent-deploy/preflights/", "pf_123"},
		{"/api/agent-deploy/preflights/", "/api/agent-deploy/preflights/", ""},
	}
	for _, tt := range tests {
		got := extractPathSuffix(tt.path, tt.prefix)
		if got != tt.want {
			t.Errorf("extractPathSuffix(%q, %q) = %q, want %q", tt.path, tt.prefix, got, tt.want)
		}
	}
}

func TestNodeIP(t *testing.T) {
	tests := []struct {
		host string
		want string
	}{
		{"https://10.0.0.2:8006", "10.0.0.2"},
		{"https://pve-b.lab.local:8006", "pve-b.lab.local"},
		{"http://192.168.1.1:8006", "192.168.1.1"},
		{"", ""},
	}
	for _, tt := range tests {
		node := models.Node{Host: tt.host}
		got := nodeIP(node)
		if got != tt.want {
			t.Errorf("nodeIP(Host=%q) = %q, want %q", tt.host, got, tt.want)
		}
	}
}

func TestIsDeployJobTerminal(t *testing.T) {
	terminal := []deploy.JobStatus{
		deploy.JobSucceeded, deploy.JobPartialSuccess, deploy.JobFailed, deploy.JobCanceled,
	}
	nonTerminal := []deploy.JobStatus{
		deploy.JobQueued, deploy.JobWaitingSource, deploy.JobRunning, deploy.JobCanceling,
	}
	for _, s := range terminal {
		if !isDeployJobTerminal(s) {
			t.Errorf("expected %q to be terminal", s)
		}
	}
	for _, s := range nonTerminal {
		if isDeployJobTerminal(s) {
			t.Errorf("expected %q to NOT be terminal", s)
		}
	}
}

func TestSSESubscriptionLifecycle(t *testing.T) {
	h := newTestDeployHandlers(t, nil, nil)

	// Add 2 clients.
	ch1 := h.addSSEClient("job-1", "client-1")
	ch2 := h.addSSEClient("job-1", "client-2")

	if ch1 == nil || ch2 == nil {
		t.Fatal("expected non-nil channels")
	}

	// Broadcast event.
	evt := &deploy.Event{
		ID:      "evt-1",
		JobID:   "job-1",
		Type:    deploy.EventPreflightResult,
		Message: "test event",
	}
	h.broadcastSSE("job-1", evt)

	// Both should receive.
	select {
	case data := <-ch1:
		if !strings.Contains(string(data), "test event") {
			t.Errorf("ch1 received unexpected data: %s", data)
		}
	default:
		t.Error("ch1 should have received data")
	}
	select {
	case data := <-ch2:
		if !strings.Contains(string(data), "test event") {
			t.Errorf("ch2 received unexpected data: %s", data)
		}
	default:
		t.Error("ch2 should have received data")
	}

	// Remove client-1.
	h.removeSSEClient("job-1", "client-1")

	// Close all for job.
	h.closeSSESub("job-1")

	// ch2 should be closed.
	_, ok := <-ch2
	if ok {
		t.Error("expected ch2 to be closed after closeSSESub")
	}
}
