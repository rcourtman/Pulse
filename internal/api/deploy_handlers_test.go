package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/deploy"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
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

	cfg := &config.Config{
		DataPath: t.TempDir(),
	}

	return NewDeployHandlers(store, monitor, execServer, reservation, func(_ *http.Request) string {
		return "http://10.0.0.1:7655"
	}, cfg, nil)
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

// --- Enrollment tests ---

// newEnrollTestHandlers creates a DeployHandlers with a real deploy store and config
// for enrollment testing. Returns the handlers and the store for seeding data.
func newEnrollTestHandlers(t *testing.T) (*DeployHandlers, *deploy.Store) {
	t.Helper()

	dir := t.TempDir()
	store, err := deploy.Open(filepath.Join(dir, "deploy.db"))
	if err != nil {
		t.Fatalf("open deploy store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	cfg := &config.Config{DataPath: dir}

	return &DeployHandlers{
		store:   store,
		config:  cfg,
		sseSubs: make(map[string]*deploySSESub),
	}, store
}

// seedEnrollJobAndTarget creates a job + target for enrollment tests.
func seedEnrollJobAndTarget(t *testing.T, store *deploy.Store, status deploy.TargetStatus) (jobID, targetID string) {
	t.Helper()
	now := time.Now().UTC()
	jobID = "job_enroll_1"
	targetID = "tgt_enroll_1"

	if err := store.CreateJob(context.Background(), &deploy.Job{
		ID: jobID, ClusterID: "c1", ClusterName: "lab",
		SourceAgentID: "agent-src", SourceNodeID: "node-src",
		OrgID: "default", Status: deploy.JobRunning,
		MaxParallel: 2, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create job: %v", err)
	}
	if err := store.CreateTarget(context.Background(), &deploy.Target{
		ID: targetID, JobID: jobID, NodeID: "node-tgt",
		NodeName: "pve-node2", NodeIP: "10.0.0.2",
		Status: status, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create target: %v", err)
	}
	return jobID, targetID
}

func mintTestBootstrapToken(t *testing.T, cfg *config.Config, jobID, targetID, expectedNode string) *config.APITokenRecord {
	t.Helper()
	raw, err := auth.GenerateAPIToken()
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	rec, err := config.NewAPITokenRecord(raw, "test-bootstrap", []string{config.ScopeHostEnroll})
	if err != nil {
		t.Fatalf("new token record: %v", err)
	}
	rec.Metadata = map[string]string{
		deploy.MetaKeyJobID:        jobID,
		deploy.MetaKeyTargetID:     targetID,
		deploy.MetaKeyExpectedNode: expectedNode,
	}
	rec.OrgID = "default"

	config.Mu.Lock()
	cfg.UpsertAPIToken(*rec)
	config.Mu.Unlock()
	return rec
}

func enrollJSON(t *testing.T, hostname string) *bytes.Buffer {
	t.Helper()
	b, _ := json.Marshal(map[string]any{
		"hostname": hostname, "os": "linux", "arch": "amd64", "agentVersion": "6.0.0",
	})
	return bytes.NewBuffer(b)
}

func enrollJSONWithProxmox(t *testing.T, hostname, pveNodeName string) *bytes.Buffer {
	t.Helper()
	b, _ := json.Marshal(map[string]any{
		"hostname": hostname, "os": "linux", "arch": "amd64", "agentVersion": "6.0.0",
		"proxmox": map[string]string{"nodeName": pveNodeName},
	})
	return bytes.NewBuffer(b)
}

func TestHandleEnroll_Success(t *testing.T) {
	h, store := newEnrollTestHandlers(t)
	jobID, targetID := seedEnrollJobAndTarget(t, store, deploy.TargetEnrolling)
	rec := mintTestBootstrapToken(t, h.config, jobID, targetID, "pve-node2")

	req := httptest.NewRequest(http.MethodPost, "/api/agents/host/enroll", enrollJSON(t, "pve-node2"))
	attachAPITokenRecord(req, rec)

	rr := httptest.NewRecorder()
	h.HandleEnroll(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["runtimeToken"] == nil || resp["runtimeToken"] == "" {
		t.Fatal("expected runtimeToken in response")
	}
	if resp["runtimeTokenId"] == nil || resp["runtimeTokenId"] == "" {
		t.Fatal("expected runtimeTokenId in response")
	}
	if resp["hostId"] != "host-pve-node2" {
		t.Fatalf("expected hostId=host-pve-node2, got %v", resp["hostId"])
	}

	// Target should now be verifying.
	target, err := store.GetTarget(context.Background(), targetID)
	if err != nil {
		t.Fatalf("get target: %v", err)
	}
	if target.Status != deploy.TargetVerifying {
		t.Fatalf("expected status %q, got %q", deploy.TargetVerifying, target.Status)
	}

	// Bootstrap token should be consumed.
	config.Mu.Lock()
	for _, tok := range h.config.APITokens {
		if tok.ID == rec.ID {
			config.Mu.Unlock()
			t.Fatal("bootstrap token should have been removed")
		}
	}
	config.Mu.Unlock()
}

func TestHandleEnroll_InstallingState(t *testing.T) {
	h, store := newEnrollTestHandlers(t)
	jobID, targetID := seedEnrollJobAndTarget(t, store, deploy.TargetInstalling)
	rec := mintTestBootstrapToken(t, h.config, jobID, targetID, "")

	req := httptest.NewRequest(http.MethodPost, "/api/agents/host/enroll", enrollJSON(t, "any-host"))
	attachAPITokenRecord(req, rec)

	rr := httptest.NewRecorder()
	h.HandleEnroll(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for installing state, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleEnroll_MissingHostname(t *testing.T) {
	h, _ := newEnrollTestHandlers(t)

	rec := &config.APITokenRecord{
		ID:     "tok-1",
		Scopes: []string{config.ScopeHostEnroll},
		Metadata: map[string]string{
			deploy.MetaKeyJobID: "j1", deploy.MetaKeyTargetID: "t1",
		},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/agents/host/enroll", enrollJSON(t, ""))
	attachAPITokenRecord(req, rec)

	rr := httptest.NewRecorder()
	h.HandleEnroll(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleEnroll_NoToken(t *testing.T) {
	h, _ := newEnrollTestHandlers(t)

	req := httptest.NewRequest(http.MethodPost, "/api/agents/host/enroll", enrollJSON(t, "host"))
	rr := httptest.NewRecorder()
	h.HandleEnroll(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleEnroll_NotBootstrapToken(t *testing.T) {
	h, _ := newEnrollTestHandlers(t)

	rec := &config.APITokenRecord{
		ID:     "tok-no-meta",
		Scopes: []string{config.ScopeHostEnroll},
		// No Metadata — not a deploy bootstrap token.
	}
	req := httptest.NewRequest(http.MethodPost, "/api/agents/host/enroll", enrollJSON(t, "host"))
	attachAPITokenRecord(req, rec)

	rr := httptest.NewRecorder()
	h.HandleEnroll(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleEnroll_BindingMismatch(t *testing.T) {
	h, store := newEnrollTestHandlers(t)
	jobID, targetID := seedEnrollJobAndTarget(t, store, deploy.TargetEnrolling)
	rec := mintTestBootstrapToken(t, h.config, jobID, targetID, "expected-host")

	req := httptest.NewRequest(http.MethodPost, "/api/agents/host/enroll", enrollJSON(t, "wrong-host"))
	attachAPITokenRecord(req, rec)

	rr := httptest.NewRecorder()
	h.HandleEnroll(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleEnroll_ProxmoxNodeNameMatch(t *testing.T) {
	h, store := newEnrollTestHandlers(t)
	jobID, targetID := seedEnrollJobAndTarget(t, store, deploy.TargetEnrolling)
	rec := mintTestBootstrapToken(t, h.config, jobID, targetID, "pve-node2")

	// OS hostname differs but proxmox.nodeName matches.
	req := httptest.NewRequest(http.MethodPost, "/api/agents/host/enroll",
		enrollJSONWithProxmox(t, "os-hostname", "pve-node2"))
	attachAPITokenRecord(req, rec)

	rr := httptest.NewRecorder()
	h.HandleEnroll(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for proxmox match, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleEnroll_InvalidTargetState(t *testing.T) {
	h, store := newEnrollTestHandlers(t)
	jobID, targetID := seedEnrollJobAndTarget(t, store, deploy.TargetReady)
	rec := mintTestBootstrapToken(t, h.config, jobID, targetID, "")

	req := httptest.NewRequest(http.MethodPost, "/api/agents/host/enroll", enrollJSON(t, "host"))
	attachAPITokenRecord(req, rec)

	rr := httptest.NewRecorder()
	h.HandleEnroll(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleEnroll_TokenConsumed(t *testing.T) {
	h, store := newEnrollTestHandlers(t)
	jobID, targetID := seedEnrollJobAndTarget(t, store, deploy.TargetEnrolling)
	rec := mintTestBootstrapToken(t, h.config, jobID, targetID, "")

	// First call succeeds.
	req1 := httptest.NewRequest(http.MethodPost, "/api/agents/host/enroll", enrollJSON(t, "host"))
	attachAPITokenRecord(req1, rec)
	rr1 := httptest.NewRecorder()
	h.HandleEnroll(rr1, req1)
	if rr1.Code != http.StatusOK {
		t.Fatalf("first enroll expected 200, got %d", rr1.Code)
	}

	// Bootstrap token should be gone.
	config.Mu.Lock()
	found := false
	for _, tok := range h.config.APITokens {
		if tok.ID == rec.ID {
			found = true
		}
	}
	config.Mu.Unlock()
	if found {
		t.Fatal("bootstrap token should have been removed after first enroll")
	}
}

func TestHandleEnroll_MethodNotAllowed(t *testing.T) {
	h, _ := newEnrollTestHandlers(t)

	req := httptest.NewRequest(http.MethodGet, "/api/agents/host/enroll", nil)
	rr := httptest.NewRecorder()
	h.HandleEnroll(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
}

func TestMintBootstrapTokenForTarget(t *testing.T) {
	cfg := &config.Config{DataPath: t.TempDir()}
	h := &DeployHandlers{
		config:  cfg,
		sseSubs: make(map[string]*deploySSESub),
	}

	raw, tokenID, err := h.MintBootstrapTokenForTarget(deploy.BootstrapTokenRequest{
		ClusterID: "c1", NodeID: "n1", ExpectedNode: "pve-3",
		JobID: "job-m1", TargetID: "tgt-m1", SourceAgentID: "agent-src",
		OrgID: "test-org", TTL: 15 * time.Minute,
	})
	if err != nil {
		t.Fatalf("MintBootstrapTokenForTarget: %v", err)
	}
	if raw == "" || tokenID == "" {
		t.Fatal("expected non-empty raw token and ID")
	}

	config.Mu.Lock()
	rec, valid := cfg.ValidateAPIToken(raw)
	config.Mu.Unlock()

	if !valid || rec == nil {
		t.Fatal("minted token should be valid")
	}
	if !rec.HasScope(config.ScopeHostEnroll) {
		t.Fatal("token should have host-agent:enroll scope")
	}
	if rec.OrgID != "test-org" {
		t.Fatalf("expected orgID=test-org, got %q", rec.OrgID)
	}
	if rec.ExpiresAt == nil {
		t.Fatal("token should have expiry set")
	}
	if rec.Metadata[deploy.MetaKeyJobID] != "job-m1" {
		t.Fatalf("expected jobID=job-m1, got %q", rec.Metadata[deploy.MetaKeyJobID])
	}
	if rec.Metadata[deploy.MetaKeyExpectedNode] != "pve-3" {
		t.Fatalf("expected expectedNode=pve-3, got %q", rec.Metadata[deploy.MetaKeyExpectedNode])
	}
}

func TestMintBootstrapTokenForTarget_InvalidTTL(t *testing.T) {
	cfg := &config.Config{DataPath: t.TempDir()}
	h := &DeployHandlers{
		config:  cfg,
		sseSubs: make(map[string]*deploySSESub),
	}

	_, _, err := h.MintBootstrapTokenForTarget(deploy.BootstrapTokenRequest{
		ClusterID: "c1", NodeID: "n1", ExpectedNode: "pve-3",
		JobID: "job-1", TargetID: "tgt-1", SourceAgentID: "a1",
		OrgID: "org", TTL: 0,
	})
	if err == nil {
		t.Fatal("expected error for zero TTL")
	}

	_, _, err = h.MintBootstrapTokenForTarget(deploy.BootstrapTokenRequest{
		ClusterID: "c1", NodeID: "n1", ExpectedNode: "pve-3",
		JobID: "job-1", TargetID: "tgt-1", SourceAgentID: "a1",
		OrgID: "org", TTL: -5 * time.Minute,
	})
	if err == nil {
		t.Fatal("expected error for negative TTL")
	}
}

func TestHandleEnroll_JobTargetMismatch(t *testing.T) {
	h, store := newEnrollTestHandlers(t)
	// Create a job and target with jobID "job_enroll_1".
	_, targetID := seedEnrollJobAndTarget(t, store, deploy.TargetEnrolling)

	// Create a token bound to a DIFFERENT job.
	rec := mintTestBootstrapToken(t, h.config, "different_job_id", targetID, "")

	req := httptest.NewRequest(http.MethodPost, "/api/agents/host/enroll", enrollJSON(t, "host"))
	attachAPITokenRecord(req, rec)

	rr := httptest.NewRecorder()
	h.HandleEnroll(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for job/target mismatch, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleEnroll_TokenAlreadyConsumed(t *testing.T) {
	h, store := newEnrollTestHandlers(t)
	jobID, targetID := seedEnrollJobAndTarget(t, store, deploy.TargetEnrolling)
	rec := mintTestBootstrapToken(t, h.config, jobID, targetID, "")

	// First call succeeds.
	req1 := httptest.NewRequest(http.MethodPost, "/api/agents/host/enroll", enrollJSON(t, "host"))
	attachAPITokenRecord(req1, rec)
	rr1 := httptest.NewRecorder()
	h.HandleEnroll(rr1, req1)
	if rr1.Code != http.StatusOK {
		t.Fatalf("first enroll expected 200, got %d", rr1.Code)
	}

	// Simulate second request that already passed RequireAuth (has the record in context)
	// but the token was consumed by the first request.
	// Reset target to enrolling so we can test the token-consumed path specifically.
	_ = store.UpdateTargetStatus(context.Background(), targetID, deploy.TargetEnrolling, "")

	req2 := httptest.NewRequest(http.MethodPost, "/api/agents/host/enroll", enrollJSON(t, "host"))
	attachAPITokenRecord(req2, rec) // same token record still in context
	rr2 := httptest.NewRecorder()
	h.HandleEnroll(rr2, req2)

	if rr2.Code != http.StatusConflict {
		t.Fatalf("expected 409 for consumed token, got %d: %s", rr2.Code, rr2.Body.String())
	}
}

func TestGetTarget(t *testing.T) {
	store, err := deploy.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	now := time.Now().UTC()

	// Non-existent target returns nil, nil.
	target, err := store.GetTarget(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if target != nil {
		t.Fatal("expected nil target")
	}

	// Seed and retrieve.
	_ = store.CreateJob(ctx, &deploy.Job{
		ID: "j1", ClusterID: "c1", ClusterName: "lab",
		SourceAgentID: "a1", SourceNodeID: "n1",
		OrgID: "default", Status: deploy.JobRunning,
		MaxParallel: 1, CreatedAt: now, UpdatedAt: now,
	})
	_ = store.CreateTarget(ctx, &deploy.Target{
		ID: "tgt-1", JobID: "j1", NodeID: "n1", NodeName: "pve-1",
		NodeIP: "10.0.0.1", Arch: "amd64", Status: deploy.TargetEnrolling,
		CreatedAt: now, UpdatedAt: now,
	})

	target, err = store.GetTarget(ctx, "tgt-1")
	if err != nil {
		t.Fatalf("GetTarget: %v", err)
	}
	if target == nil {
		t.Fatal("expected non-nil target")
	}
	if target.NodeName != "pve-1" {
		t.Fatalf("got NodeName=%q, want pve-1", target.NodeName)
	}
	if target.Arch != "amd64" {
		t.Fatalf("got Arch=%q, want amd64", target.Arch)
	}
}
