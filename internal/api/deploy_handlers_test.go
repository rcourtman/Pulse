package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/api/agentbinding"
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

	execServer := agentexec.NewServer(func(string, string, string) bool { return true })

	cfg := &config.Config{
		DataPath: t.TempDir(),
	}

	return NewDeployHandlers(store, monitor, execServer, func(_ *http.Request) string {
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
			LinkedAgentID: "host-a",
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
		got := nodeIP(tt.host)
		if got != tt.want {
			t.Errorf("nodeIP(%q) = %q, want %q", tt.host, got, tt.want)
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
	rec, err := config.NewAPITokenRecord(raw, "test-bootstrap", []string{config.ScopeAgentEnroll})
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
	rec.Metadata[apiTokenMetadataOwnerUserID] = "alice"
	h.persistence = config.NewConfigPersistence(h.config.DataPath)
	if err := h.persistence.SaveAPITokens(h.config.APITokens); err != nil {
		t.Fatalf("persist bootstrap token: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/agents/agent/enroll", enrollJSON(t, "pve-node2"))
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
	runtimeTokenID, _ := resp["runtimeTokenId"].(string)
	if _, invented := resp["agentId"]; invented {
		t.Fatalf("enrollment returned agentId %v; the agent must keep the machine-derived identity its reports carry", resp["agentId"])
	}

	config.Mu.Lock()
	var runtimeRecord *config.APITokenRecord
	for i := range h.config.APITokens {
		if h.config.APITokens[i].ID == runtimeTokenID {
			runtimeRecord = &h.config.APITokens[i]
			break
		}
	}
	config.Mu.Unlock()
	if runtimeRecord == nil {
		t.Fatal("runtime token should be stored")
	}
	if got := runtimeRecord.Metadata[apiTokenMetadataOwnerUserID]; got != "alice" {
		t.Fatalf("runtime token owner_user_id = %q, want alice", got)
	}
	if got := runtimeRecord.Metadata["bound_agent_id"]; got != "" {
		t.Fatalf("runtime token bound_agent_id = %q, want unbound until the agent's first command registration", got)
	}
	if got := runtimeRecord.Metadata["bound_hostname"]; got != "pve-node2" {
		t.Fatalf("runtime token bound_hostname = %q, want pve-node2", got)
	}
	if got := runtimeRecord.Metadata[agentExecBindingVersionKey]; got != agentExecBindingVersion {
		t.Fatalf("runtime token %s = %q, want %q", agentExecBindingVersionKey, got, agentExecBindingVersion)
	}
	if got := runtimeRecord.Metadata[agentbinding.DeployIdentityKey]; got != agentbinding.DeployIdentityAgent {
		t.Fatalf("runtime token %s = %q, want %q so it is never treated as a historical placeholder", agentbinding.DeployIdentityKey, got, agentbinding.DeployIdentityAgent)
	}
	persistedTokens, err := h.persistence.LoadAPITokens()
	if err != nil {
		t.Fatalf("load persisted tokens: %v", err)
	}
	if len(persistedTokens) != 1 || persistedTokens[0].ID != runtimeTokenID {
		t.Fatalf("persisted enrollment transition = %+v, want only runtime token %q", persistedTokens, runtimeTokenID)
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

// A deployed agent reports under its machine-derived identity, Pulse
// acknowledges that identity, and the agent persists it. Its command channel
// must therefore bind to that identity on first registration and keep
// admitting it across restarts. Enrollment once bound the runtime token to an
// invented agent-<hostname>, so every deployed agent lost command execution
// after its first restart.
func TestHandleEnroll_RuntimeTokenBindsTheAgentsOwnIdentityOnFirstCommandRegistration(t *testing.T) {
	h, store := newEnrollTestHandlers(t)
	jobID, targetID := seedEnrollJobAndTarget(t, store, deploy.TargetEnrolling)
	rec := mintTestBootstrapToken(t, h.config, jobID, targetID, "pve-node2")
	h.persistence = config.NewConfigPersistence(h.config.DataPath)

	body, _ := json.Marshal(map[string]any{
		"hostname": "pve-node2", "os": "linux", "arch": "amd64", "agentVersion": "6.0.0", "commandsEnabled": true,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/agents/agent/enroll", bytes.NewBuffer(body))
	attachAPITokenRecord(req, rec)
	rr := httptest.NewRecorder()
	h.HandleEnroll(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("enroll status = %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	runtimeToken, _ := resp["runtimeToken"].(string)
	if runtimeToken == "" {
		t.Fatal("enrollment returned no runtime token")
	}

	// The agent registers first under whatever identity enrollment handed it,
	// falling back to its own, then restarts under the identity Pulse
	// acknowledged for its reports: its machine-derived ID.
	router := &Router{config: h.config, persistence: h.persistence}
	const machineID = "49a2dbb3-3cc8-43b7-ad57-eb1711142454"
	firstID := machineID
	if handed, _ := resp["agentId"].(string); handed != "" {
		firstID = handed
	}
	if _, ok := router.admitAgentExecToken(runtimeToken, firstID, "pve-node2"); !ok {
		t.Fatalf("first command registration as %q was refused", firstID)
	}
	admission, ok := router.admitAgentExecToken(runtimeToken, machineID, "pve-node2")
	if !ok || admission.AgentID != machineID {
		t.Fatalf("registration after restart = %+v ok=%v, want the agent's acknowledged identity", admission, ok)
	}
	if _, ok := router.admitAgentExecToken(runtimeToken, machineID, "pve-node2"); !ok {
		t.Fatal("the bound agent was refused on its next registration")
	}
	if _, ok := router.admitAgentExecToken(runtimeToken, "another-machine", "pve-node2"); ok {
		t.Fatal("a second identity was admitted on a token already bound to the agent")
	}
	persisted, err := h.persistence.LoadAPITokens()
	if err != nil {
		t.Fatalf("load persisted tokens: %v", err)
	}
	if len(persisted) != 1 || persisted[0].Metadata["bound_agent_id"] != machineID {
		t.Fatalf("persisted runtime token = %+v, want bound to %s", persisted, machineID)
	}
}

func TestHandleEnroll_InstallingState(t *testing.T) {
	h, store := newEnrollTestHandlers(t)
	jobID, targetID := seedEnrollJobAndTarget(t, store, deploy.TargetInstalling)
	rec := mintTestBootstrapToken(t, h.config, jobID, targetID, "")

	req := httptest.NewRequest(http.MethodPost, "/api/agents/agent/enroll", enrollJSON(t, "any-host"))
	attachAPITokenRecord(req, rec)

	rr := httptest.NewRecorder()
	h.HandleEnroll(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for installing state, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleEnroll_RollsBackBootstrapConsumptionWhenPersistenceFails(t *testing.T) {
	h, store := newEnrollTestHandlers(t)
	jobID, targetID := seedEnrollJobAndTarget(t, store, deploy.TargetEnrolling)
	bootstrap := mintTestBootstrapToken(t, h.config, jobID, targetID, "pve-node2")

	statePath := filepath.Join(t.TempDir(), "blocked-state")
	h.persistence = config.NewConfigPersistence(statePath)
	if err := os.RemoveAll(statePath); err != nil {
		t.Fatalf("remove persistence directory: %v", err)
	}
	if err := os.WriteFile(statePath, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("create persistence blocker: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/agents/agent/enroll", enrollJSON(t, "pve-node2"))
	attachAPITokenRecord(req, bootstrap)
	rr := httptest.NewRecorder()
	h.HandleEnroll(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d (body=%q)", rr.Code, http.StatusInternalServerError, rr.Body.String())
	}

	config.Mu.Lock()
	tokens := append([]config.APITokenRecord(nil), h.config.APITokens...)
	primaryToken := h.config.APIToken
	config.Mu.Unlock()
	if len(tokens) != 1 || tokens[0].ID != bootstrap.ID {
		t.Fatalf("bootstrap token was not restored exactly: %+v", tokens)
	}
	if primaryToken != bootstrap.Hash {
		t.Fatalf("legacy primary token = %q, want restored bootstrap hash", primaryToken)
	}

	target, err := store.GetTarget(context.Background(), targetID)
	if err != nil {
		t.Fatalf("get target: %v", err)
	}
	if target.Status != deploy.TargetEnrolling {
		t.Fatalf("target status = %q, want %q", target.Status, deploy.TargetEnrolling)
	}
}

func TestHandleEnroll_MissingHostname(t *testing.T) {
	h, _ := newEnrollTestHandlers(t)

	rec := &config.APITokenRecord{
		ID:     "tok-1",
		Scopes: []string{config.ScopeAgentEnroll},
		Metadata: map[string]string{
			deploy.MetaKeyJobID: "j1", deploy.MetaKeyTargetID: "t1",
		},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/agents/agent/enroll", enrollJSON(t, ""))
	attachAPITokenRecord(req, rec)

	rr := httptest.NewRecorder()
	h.HandleEnroll(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleEnroll_NoToken(t *testing.T) {
	h, _ := newEnrollTestHandlers(t)

	req := httptest.NewRequest(http.MethodPost, "/api/agents/agent/enroll", enrollJSON(t, "host"))
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
		Scopes: []string{config.ScopeAgentEnroll},
		// No Metadata — not a deploy bootstrap token.
	}
	req := httptest.NewRequest(http.MethodPost, "/api/agents/agent/enroll", enrollJSON(t, "host"))
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

	req := httptest.NewRequest(http.MethodPost, "/api/agents/agent/enroll", enrollJSON(t, "wrong-host"))
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
	req := httptest.NewRequest(http.MethodPost, "/api/agents/agent/enroll",
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

	req := httptest.NewRequest(http.MethodPost, "/api/agents/agent/enroll", enrollJSON(t, "host"))
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
	req1 := httptest.NewRequest(http.MethodPost, "/api/agents/agent/enroll", enrollJSON(t, "host"))
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

	req := httptest.NewRequest(http.MethodGet, "/api/agents/agent/enroll", nil)
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
	}, "alice")
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
	if !rec.HasScope(config.ScopeAgentEnroll) {
		t.Fatal("token should have agent:enroll scope")
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
	if rec.Metadata[apiTokenMetadataOwnerUserID] != "alice" {
		t.Fatalf("expected owner_user_id=alice, got %q", rec.Metadata[apiTokenMetadataOwnerUserID])
	}
}

func TestMintBootstrapTokenForTarget_RollsBackWhenPersistenceFails(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "blocked-state")
	persistence := config.NewConfigPersistence(statePath)
	if err := os.RemoveAll(statePath); err != nil {
		t.Fatalf("remove persistence directory: %v", err)
	}
	if err := os.WriteFile(statePath, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("create persistence blocker: %v", err)
	}

	existing := config.APITokenRecord{
		ID: "existing", Name: "existing", Hash: "existing-hash",
		CreatedAt: time.Now().UTC(), Scopes: []string{config.ScopeWildcard},
	}
	cfg := &config.Config{APITokens: []config.APITokenRecord{existing}}
	cfg.SortAPITokens()
	h := &DeployHandlers{
		config:      cfg,
		persistence: persistence,
		sseSubs:     make(map[string]*deploySSESub),
	}

	raw, tokenID, err := h.MintBootstrapTokenForTarget(deploy.BootstrapTokenRequest{
		ClusterID: "c1", NodeID: "n1", ExpectedNode: "pve-3",
		JobID: "job-m1", TargetID: "tgt-m1", SourceAgentID: "agent-src",
		OrgID: "test-org", TTL: 15 * time.Minute,
	}, "alice")
	if err == nil {
		t.Fatal("expected persistence failure")
	}
	if raw != "" || tokenID != "" {
		t.Fatalf("failed mint returned credential material: raw=%q id=%q", raw, tokenID)
	}
	if len(cfg.APITokens) != 1 || cfg.APITokens[0].ID != existing.ID {
		t.Fatalf("live tokens were not rolled back: %+v", cfg.APITokens)
	}
	if cfg.APIToken != existing.Hash {
		t.Fatalf("legacy primary token = %q, want %q", cfg.APIToken, existing.Hash)
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
	}, "alice")
	if err == nil {
		t.Fatal("expected error for zero TTL")
	}

	_, _, err = h.MintBootstrapTokenForTarget(deploy.BootstrapTokenRequest{
		ClusterID: "c1", NodeID: "n1", ExpectedNode: "pve-3",
		JobID: "job-1", TargetID: "tgt-1", SourceAgentID: "a1",
		OrgID: "org", TTL: -5 * time.Minute,
	}, "alice")
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

	req := httptest.NewRequest(http.MethodPost, "/api/agents/agent/enroll", enrollJSON(t, "host"))
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
	req1 := httptest.NewRequest(http.MethodPost, "/api/agents/agent/enroll", enrollJSON(t, "host"))
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

	req2 := httptest.NewRequest(http.MethodPost, "/api/agents/agent/enroll", enrollJSON(t, "host"))
	attachAPITokenRecord(req2, rec) // same token record still in context
	rr2 := httptest.NewRecorder()
	h.HandleEnroll(rr2, req2)

	if rr2.Code != http.StatusConflict {
		t.Fatalf("expected 409 for consumed token, got %d: %s", rr2.Code, rr2.Body.String())
	}
}

func TestHandleEnroll_CommandsEnabledAddsAgentExecScope(t *testing.T) {
	h, store := newEnrollTestHandlers(t)
	jobID, targetID := seedEnrollJobAndTarget(t, store, deploy.TargetEnrolling)
	rec := mintTestBootstrapToken(t, h.config, jobID, targetID, "cmd-host")

	body, _ := json.Marshal(map[string]any{
		"hostname": "cmd-host", "os": "linux", "arch": "amd64",
		"agentVersion": "6.0.0", "commandsEnabled": true,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/agents/agent/enroll", bytes.NewBuffer(body))
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
	runtimeTokenID, _ := resp["runtimeTokenId"].(string)
	if runtimeTokenID == "" {
		t.Fatal("expected runtimeTokenId in response")
	}

	// Find the runtime token and verify scopes include agent:exec.
	config.Mu.Lock()
	var found *config.APITokenRecord
	for i := range h.config.APITokens {
		if h.config.APITokens[i].ID == runtimeTokenID {
			found = &h.config.APITokens[i]
			break
		}
	}
	config.Mu.Unlock()

	if found == nil {
		t.Fatal("runtime token not found in config")
	}

	hasExec := false
	hasManage := false
	for _, s := range found.Scopes {
		if s == config.ScopeAgentExec {
			hasExec = true
		}
		if s == config.ScopeAgentManage {
			hasManage = true
		}
	}
	if !hasExec {
		t.Errorf("expected runtime token to have %s scope, got scopes: %v", config.ScopeAgentExec, found.Scopes)
	}
	if hasManage {
		t.Errorf("runtime collector token must not have %s scope, got scopes: %v", config.ScopeAgentManage, found.Scopes)
	}
	if got := found.Metadata["bound_agent_id"]; got != "" {
		t.Errorf("runtime token bound_agent_id = %q, want unbound until the agent's first command registration", got)
	}
	if got := found.Metadata["bound_hostname"]; got != "cmd-host" {
		t.Errorf("runtime token bound_hostname = %q, want %q", got, "cmd-host")
	}
}

func TestHandleEnroll_CommandsDisabledNoAgentExecScope(t *testing.T) {
	h, store := newEnrollTestHandlers(t)
	jobID, targetID := seedEnrollJobAndTarget(t, store, deploy.TargetEnrolling)
	rec := mintTestBootstrapToken(t, h.config, jobID, targetID, "no-cmd-host")

	// commandsEnabled defaults to false (not sent).
	req := httptest.NewRequest(http.MethodPost, "/api/agents/agent/enroll", enrollJSON(t, "no-cmd-host"))
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
	runtimeTokenID, _ := resp["runtimeTokenId"].(string)

	config.Mu.Lock()
	var found *config.APITokenRecord
	for i := range h.config.APITokens {
		if h.config.APITokens[i].ID == runtimeTokenID {
			found = &h.config.APITokens[i]
			break
		}
	}
	config.Mu.Unlock()

	if found == nil {
		t.Fatal("runtime token not found in config")
	}

	// Should have report and config:read — but no management or execution scope.
	for _, s := range found.Scopes {
		if s == config.ScopeAgentExec {
			t.Errorf("runtime token should NOT have %s scope when commandsEnabled is false", config.ScopeAgentExec)
		}
	}

	// Collector credentials must not manage other agent configurations.
	hasManage := false
	for _, s := range found.Scopes {
		if s == config.ScopeAgentManage {
			hasManage = true
		}
	}
	if hasManage {
		t.Errorf("runtime collector token must not have %s scope", config.ScopeAgentManage)
	}
}

// --- Deploy Job tests ---

func TestHandleCreateJob_Success(t *testing.T) {
	nodes := []models.Node{
		{
			ID: "node_pve-a", Name: "pve-a", Host: "https://10.0.0.1:8006",
			IsClusterMember: true, ClusterName: "lab",
			LinkedAgentID: "host-a",
		},
		{
			ID: "node_pve-b", Name: "pve-b", Host: "https://10.0.0.2:8006",
			IsClusterMember: true, ClusterName: "lab",
		},
		{
			ID: "node_pve-c", Name: "pve-c", Host: "https://10.0.0.3:8006",
			IsClusterMember: true, ClusterName: "lab",
		},
	}

	h := newTestDeployHandlers(t, nodes, nil)
	ctx := context.Background()

	// Simulate a connected source agent.
	t.Cleanup(h.execServer.TestRegisterAgent("host-a", "host-a"))

	// Create a succeeded preflight job with ready targets.
	now := time.Now().UTC()
	pfJob := &deploy.Job{
		ID: "pf_test_ok", ClusterID: "lab", ClusterName: "lab",
		SourceAgentID: "host-a", SourceNodeID: "node_pve-a",
		OrgID: "default", Status: deploy.JobSucceeded,
		MaxParallel: 2, CreatedAt: now, UpdatedAt: now,
	}
	if err := h.store.CreateJob(ctx, pfJob); err != nil {
		t.Fatalf("create preflight job: %v", err)
	}
	// Create ready targets.
	for _, info := range []struct{ id, nodeID, name, ip string }{
		{"tgt_pf_b", "node_pve-b", "pve-b", "10.0.0.2"},
		{"tgt_pf_c", "node_pve-c", "pve-c", "10.0.0.3"},
	} {
		if err := h.store.CreateTarget(ctx, &deploy.Target{
			ID: info.id, JobID: "pf_test_ok", NodeID: info.nodeID,
			NodeName: info.name, NodeIP: info.ip, Arch: "amd64",
			Status: deploy.TargetReady, CreatedAt: now, UpdatedAt: now,
		}); err != nil {
			t.Fatalf("create preflight target: %v", err)
		}
	}

	body := `{"sourceAgentId":"host-a","preflightId":"pf_test_ok","targetNodeIds":["node_pve-b","node_pve-c"],"mode":"install"}`
	req := httptest.NewRequest(http.MethodPost, "/api/clusters/lab/agent-deploy/jobs", strings.NewReader(body))
	rec := httptest.NewRecorder()

	h.HandleCreateJob(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp createJobResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.JobID == "" {
		t.Fatal("expected non-empty jobId")
	}
	if len(resp.AcceptedTargets) != 2 {
		t.Fatalf("expected 2 accepted targets, got %d", len(resp.AcceptedTargets))
	}
	if len(resp.SkippedTargets) != 0 {
		t.Fatalf("expected 0 skipped targets, got %d", len(resp.SkippedTargets))
	}
	if resp.EventsURL == "" {
		t.Fatal("expected non-empty eventsUrl")
	}
}

func TestHandleCreateJob_PreflightNotPassed(t *testing.T) {
	h := newTestDeployHandlers(t, nil, nil)
	ctx := context.Background()

	t.Cleanup(h.execServer.TestRegisterAgent("host-a", "host-a"))

	// Preflight with failed status.
	now := time.Now().UTC()
	if err := h.store.CreateJob(ctx, &deploy.Job{
		ID: "pf_failed", ClusterID: "lab", ClusterName: "lab",
		SourceAgentID: "host-a", SourceNodeID: "node_a",
		OrgID: "default", Status: deploy.JobFailed,
		MaxParallel: 2, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create job: %v", err)
	}

	body := `{"sourceAgentId":"host-a","preflightId":"pf_failed","targetNodeIds":["node_b"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/clusters/lab/agent-deploy/jobs", strings.NewReader(body))
	rec := httptest.NewRecorder()

	h.HandleCreateJob(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleCreateJob_SourceAgentOffline(t *testing.T) {
	h := newTestDeployHandlers(t, nil, nil)

	body := `{"sourceAgentId":"offline-agent","preflightId":"pf_1","targetNodeIds":["node_b"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/clusters/lab/agent-deploy/jobs", strings.NewReader(body))
	rec := httptest.NewRecorder()

	h.HandleCreateJob(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleCreateJob_NoPreflightID(t *testing.T) {
	h := newTestDeployHandlers(t, nil, nil)

	t.Cleanup(h.execServer.TestRegisterAgent("host-a", "host-a"))

	body := `{"sourceAgentId":"host-a","targetNodeIds":["node_b"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/clusters/lab/agent-deploy/jobs", strings.NewReader(body))
	rec := httptest.NewRecorder()

	h.HandleCreateJob(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleCreateJob_TargetsNotReady(t *testing.T) {
	h := newTestDeployHandlers(t, nil, nil)
	ctx := context.Background()

	t.Cleanup(h.execServer.TestRegisterAgent("host-a", "host-a"))

	now := time.Now().UTC()
	if err := h.store.CreateJob(ctx, &deploy.Job{
		ID: "pf_partial", ClusterID: "lab", ClusterName: "lab",
		SourceAgentID: "host-a", SourceNodeID: "node_a",
		OrgID: "default", Status: deploy.JobPartialSuccess,
		MaxParallel: 2, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create job: %v", err)
	}

	// One ready, one failed.
	if err := h.store.CreateTarget(ctx, &deploy.Target{
		ID: "tgt_ready", JobID: "pf_partial", NodeID: "node_b",
		NodeName: "pve-b", NodeIP: "10.0.0.2", Arch: "amd64",
		Status: deploy.TargetReady, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create target: %v", err)
	}
	if err := h.store.CreateTarget(ctx, &deploy.Target{
		ID: "tgt_fail", JobID: "pf_partial", NodeID: "node_c",
		NodeName: "pve-c", NodeIP: "10.0.0.3",
		Status: deploy.TargetFailedPermanent, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create target: %v", err)
	}

	// Request both — only ready should be accepted.
	body := `{"sourceAgentId":"host-a","preflightId":"pf_partial","targetNodeIds":["node_b","node_c"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/clusters/lab/agent-deploy/jobs", strings.NewReader(body))
	rec := httptest.NewRecorder()

	h.HandleCreateJob(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp createJobResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.AcceptedTargets) != 1 {
		t.Fatalf("expected 1 accepted target, got %d", len(resp.AcceptedTargets))
	}
	if len(resp.SkippedTargets) != 1 {
		t.Fatalf("expected 1 skipped target, got %d", len(resp.SkippedTargets))
	}
	if resp.SkippedTargets[0].NodeID != "node_c" {
		t.Fatalf("expected skipped nodeId=node_c, got %s", resp.SkippedTargets[0].NodeID)
	}
}

func TestHandleCreateJob_AcceptsAllTargetsWithMonitoredSystemCapsRetired(t *testing.T) {
	setMaxMonitoredSystemsLicenseForTests(t, 4)

	nodes := []models.Node{
		{
			ID: "node_pve-a", Name: "pve-a", Host: "https://10.0.0.1:8006",
			IsClusterMember: true, ClusterName: "lab",
			LinkedAgentID: "host-a",
		},
		{
			ID: "node_pve-b", Name: "pve-b", Host: "https://10.0.0.2:8006",
			IsClusterMember: true, ClusterName: "lab",
		},
		{
			ID: "node_pve-c", Name: "pve-c", Host: "https://10.0.0.3:8006",
			IsClusterMember: true, ClusterName: "lab",
		},
	}
	h := newTestDeployHandlers(t, nodes, nil)
	ctx := context.Background()

	t.Cleanup(h.execServer.TestRegisterAgent("host-a", "host-a"))

	now := time.Now().UTC()
	if err := h.store.CreateJob(ctx, &deploy.Job{
		ID: "pf_license_partial", ClusterID: "lab", ClusterName: "lab",
		SourceAgentID: "host-a", SourceNodeID: "node_pve-a",
		OrgID: "default", Status: deploy.JobSucceeded,
		MaxParallel: 2, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create preflight job: %v", err)
	}
	for _, info := range []struct{ id, nodeID, name, ip string }{
		{"tgt_license_b", "node_pve-b", "pve-b", "10.0.0.2"},
		{"tgt_license_c", "node_pve-c", "pve-c", "10.0.0.3"},
	} {
		if err := h.store.CreateTarget(ctx, &deploy.Target{
			ID: info.id, JobID: "pf_license_partial", NodeID: info.nodeID,
			NodeName: info.name, NodeIP: info.ip, Arch: "amd64",
			Status: deploy.TargetReady, CreatedAt: now, UpdatedAt: now,
		}); err != nil {
			t.Fatalf("create preflight target: %v", err)
		}
	}

	body := `{"sourceAgentId":"host-a","preflightId":"pf_license_partial","targetNodeIds":["node_pve-b","node_pve-c"],"mode":"install"}`
	req := httptest.NewRequest(http.MethodPost, "/api/clusters/lab/agent-deploy/jobs", strings.NewReader(body))
	rec := httptest.NewRecorder()

	h.HandleCreateJob(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp createJobResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.AcceptedTargets) != 2 {
		t.Fatalf("expected 2 accepted targets with monitored-system caps retired, got %d", len(resp.AcceptedTargets))
	}
	if resp.AcceptedTargets[0] != "node_pve-b" || resp.AcceptedTargets[1] != "node_pve-c" {
		t.Fatalf("unexpected accepted targets: %+v", resp.AcceptedTargets)
	}
	if len(resp.SkippedTargets) != 0 {
		t.Fatalf("expected no skipped targets with monitored-system caps retired, got %+v", resp.SkippedTargets)
	}
}

func TestHandleGetJob_Success(t *testing.T) {
	h := newTestDeployHandlers(t, nil, nil)
	ctx := context.Background()
	now := time.Now().UTC()

	if err := h.store.CreateJob(ctx, &deploy.Job{
		ID: "dep_test1", ClusterID: "lab", ClusterName: "lab",
		SourceAgentID: "agent-1", SourceNodeID: "node_a",
		OrgID: "default", Status: deploy.JobRunning,
		MaxParallel: 2, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create job: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/agent-deploy/jobs/dep_test1", nil)
	rec := httptest.NewRecorder()
	h.HandleGetJob(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		ID      string `json:"id"`
		Status  string `json:"status"`
		Targets []any  `json:"targets"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.ID != "dep_test1" {
		t.Fatalf("expected id dep_test1, got %q", resp.ID)
	}
	if resp.Status != "running" {
		t.Fatalf("expected status running, got %q", resp.Status)
	}
}

func TestHandleGetJob_NotFound(t *testing.T) {
	h := newTestDeployHandlers(t, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/agent-deploy/jobs/nonexistent", nil)
	rec := httptest.NewRecorder()
	h.HandleGetJob(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleGetJob_TenantIsolation(t *testing.T) {
	h := newTestDeployHandlers(t, nil, nil)
	ctx := context.Background()
	now := time.Now().UTC()

	// Job belongs to org "other-org".
	if err := h.store.CreateJob(ctx, &deploy.Job{
		ID: "dep_other", ClusterID: "lab", ClusterName: "lab",
		SourceAgentID: "agent-1", SourceNodeID: "node_a",
		OrgID: "other-org", Status: deploy.JobRunning,
		MaxParallel: 2, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create job: %v", err)
	}

	// Request without org header defaults to "default" org → should not see "other-org" job.
	req := httptest.NewRequest(http.MethodGet, "/api/agent-deploy/jobs/dep_other", nil)
	rec := httptest.NewRecorder()
	h.HandleGetJob(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for tenant isolation, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleCancelJob_Success(t *testing.T) {
	h := newTestDeployHandlers(t, nil, nil)
	ctx := context.Background()
	now := time.Now().UTC()

	t.Cleanup(h.execServer.TestRegisterAgent("agent-1", "agent-1"))

	if err := h.store.CreateJob(ctx, &deploy.Job{
		ID: "dep_cancel", ClusterID: "lab", ClusterName: "lab",
		SourceAgentID: "agent-1", SourceNodeID: "node_a",
		OrgID: "default", Status: deploy.JobRunning,
		MaxParallel: 2, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create job: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/agent-deploy/jobs/dep_cancel/cancel", nil)
	rec := httptest.NewRecorder()
	h.HandleCancelJob(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify job is now canceling.
	job, _ := h.store.GetJob(ctx, "dep_cancel")
	if job.Status != deploy.JobCanceling {
		t.Fatalf("expected status canceling, got %q", job.Status)
	}
}

func TestHandleCancelJob_NotRunning(t *testing.T) {
	h := newTestDeployHandlers(t, nil, nil)
	ctx := context.Background()
	now := time.Now().UTC()

	if err := h.store.CreateJob(ctx, &deploy.Job{
		ID: "dep_done", ClusterID: "lab", ClusterName: "lab",
		SourceAgentID: "agent-1", SourceNodeID: "node_a",
		OrgID: "default", Status: deploy.JobSucceeded,
		MaxParallel: 2, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create job: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/agent-deploy/jobs/dep_done/cancel", nil)
	rec := httptest.NewRecorder()
	h.HandleCancelJob(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleRetryJob_Success(t *testing.T) {
	h := newTestDeployHandlers(t, nil, nil)
	ctx := context.Background()
	now := time.Now().UTC()

	t.Cleanup(h.execServer.TestRegisterAgent("agent-1", "agent-1"))

	if err := h.store.CreateJob(ctx, &deploy.Job{
		ID: "dep_retry", ClusterID: "lab", ClusterName: "lab",
		SourceAgentID: "agent-1", SourceNodeID: "node_a",
		OrgID: "default", Status: deploy.JobFailed,
		MaxParallel: 2, RetryMax: 3, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create job: %v", err)
	}

	if err := h.store.CreateTarget(ctx, &deploy.Target{
		ID: "tgt_retry_1", JobID: "dep_retry", NodeID: "node_b",
		NodeName: "pve-b", NodeIP: "10.0.0.2", Arch: "amd64",
		Status: deploy.TargetFailedRetryable, Attempts: 1,
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create target: %v", err)
	}

	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/api/agent-deploy/jobs/dep_retry/retry", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.HandleRetryJob(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify job is now running.
	job, _ := h.store.GetJob(ctx, "dep_retry")
	if job.Status != deploy.JobRunning {
		t.Fatalf("expected status running, got %q", job.Status)
	}
}

func TestHandleRetryJob_AllowsRetryWithMonitoredSystemCapsRetired(t *testing.T) {
	setMaxMonitoredSystemsLicenseForTests(t, 1)

	h := newTestDeployHandlers(t, nil, []models.Host{{ID: "host-existing", Hostname: "existing"}})
	ctx := context.Background()
	now := time.Now().UTC()

	t.Cleanup(h.execServer.TestRegisterAgent("agent-1", "agent-1"))

	if err := h.store.CreateJob(ctx, &deploy.Job{
		ID: "dep_retry_license", ClusterID: "lab", ClusterName: "lab",
		SourceAgentID: "agent-1", SourceNodeID: "node_a",
		OrgID: "default", Status: deploy.JobFailed,
		MaxParallel: 2, RetryMax: 3, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create job: %v", err)
	}

	if err := h.store.CreateTarget(ctx, &deploy.Target{
		ID: "tgt_retry_license_1", JobID: "dep_retry_license", NodeID: "node_b",
		NodeName: "pve-b", NodeIP: "10.0.0.2", Arch: "amd64",
		Status: deploy.TargetFailedRetryable, Attempts: 1,
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create target: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/agent-deploy/jobs/dep_retry_license/retry", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	h.HandleRetryJob(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202 with monitored-system caps retired, got %d: %s", rec.Code, rec.Body.String())
	}

	targets, err := h.store.GetTargetsForJob(ctx, "dep_retry_license")
	if err != nil {
		t.Fatalf("get targets: %v", err)
	}
	if len(targets) != 1 || targets[0].Status != deploy.TargetPending {
		t.Fatalf("expected retry target to be queued, got %+v", targets)
	}
}

func TestHandleRetryJob_NothingToRetry(t *testing.T) {
	h := newTestDeployHandlers(t, nil, nil)
	ctx := context.Background()
	now := time.Now().UTC()

	t.Cleanup(h.execServer.TestRegisterAgent("agent-1", "agent-1"))

	if err := h.store.CreateJob(ctx, &deploy.Job{
		ID: "dep_ok", ClusterID: "lab", ClusterName: "lab",
		SourceAgentID: "agent-1", SourceNodeID: "node_a",
		OrgID: "default", Status: deploy.JobSucceeded,
		MaxParallel: 2, RetryMax: 3, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create job: %v", err)
	}

	if err := h.store.CreateTarget(ctx, &deploy.Target{
		ID: "tgt_ok_1", JobID: "dep_ok", NodeID: "node_b",
		NodeName: "pve-b", NodeIP: "10.0.0.2",
		Status:    deploy.TargetSucceeded,
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create target: %v", err)
	}

	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/api/agent-deploy/jobs/dep_ok/retry", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.HandleRetryJob(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestProcessInstallProgress_InstallTransfer(t *testing.T) {
	h := newTestDeployHandlers(t, nil, nil)
	ctx := context.Background()
	now := time.Now().UTC()

	if err := h.store.CreateJob(ctx, &deploy.Job{
		ID: "dep_prog", ClusterID: "lab", ClusterName: "lab",
		SourceAgentID: "agent-1", SourceNodeID: "node_a",
		OrgID: "default", Status: deploy.JobRunning,
		MaxParallel: 2, RetryMax: 3, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create job: %v", err)
	}
	if err := h.store.CreateTarget(ctx, &deploy.Target{
		ID: "tgt_prog_1", JobID: "dep_prog", NodeID: "node_b",
		NodeName: "pve-b", NodeIP: "10.0.0.2",
		Status:    deploy.TargetPending,
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create target: %v", err)
	}

	// Simulate install_transfer started.
	h.updateTargetFromInstallProgress(ctx, agentexec.DeployProgressPayload{
		TargetID: "tgt_prog_1",
		Phase:    agentexec.DeployPhaseInstallTransfer,
		Status:   agentexec.DeployStepStarted,
	}, 3)

	target, _ := h.store.GetTarget(ctx, "tgt_prog_1")
	if target.Status != deploy.TargetInstalling {
		t.Fatalf("expected status installing, got %q", target.Status)
	}
}

func TestProcessInstallProgress_InstallFailed(t *testing.T) {
	h := newTestDeployHandlers(t, nil, nil)
	ctx := context.Background()
	now := time.Now().UTC()

	if err := h.store.CreateJob(ctx, &deploy.Job{
		ID: "dep_fail", ClusterID: "lab", ClusterName: "lab",
		SourceAgentID: "agent-1", SourceNodeID: "node_a",
		OrgID: "default", Status: deploy.JobRunning,
		MaxParallel: 2, RetryMax: 3, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create job: %v", err)
	}
	if err := h.store.CreateTarget(ctx, &deploy.Target{
		ID: "tgt_fail_1", JobID: "dep_fail", NodeID: "node_b",
		NodeName: "pve-b", NodeIP: "10.0.0.2",
		Status: deploy.TargetInstalling, Attempts: 0,
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create target: %v", err)
	}

	// Simulate install_execute failed.
	h.updateTargetFromInstallProgress(ctx, agentexec.DeployProgressPayload{
		TargetID: "tgt_fail_1",
		Phase:    agentexec.DeployPhaseInstallExecute,
		Status:   agentexec.DeployStepFailed,
		Message:  "install script exited 1",
	}, 3)

	target, _ := h.store.GetTarget(ctx, "tgt_fail_1")
	if target.Status != deploy.TargetFailedRetryable {
		t.Fatalf("expected status failed_retryable, got %q", target.Status)
	}
	if target.Attempts != 1 {
		t.Fatalf("expected attempts=1, got %d", target.Attempts)
	}
}

func TestProcessInstallProgress_InstallFailedPermanent(t *testing.T) {
	h := newTestDeployHandlers(t, nil, nil)
	ctx := context.Background()
	now := time.Now().UTC()

	if err := h.store.CreateJob(ctx, &deploy.Job{
		ID: "dep_failp", ClusterID: "lab", ClusterName: "lab",
		SourceAgentID: "agent-1", SourceNodeID: "node_a",
		OrgID: "default", Status: deploy.JobRunning,
		MaxParallel: 2, RetryMax: 2, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create job: %v", err)
	}
	if err := h.store.CreateTarget(ctx, &deploy.Target{
		ID: "tgt_failp_1", JobID: "dep_failp", NodeID: "node_b",
		NodeName: "pve-b", NodeIP: "10.0.0.2",
		Status: deploy.TargetInstalling, Attempts: 1,
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create target: %v", err)
	}

	// At attempt 1, retryMax is 2, so attempts+1 >= retryMax → permanent.
	h.updateTargetFromInstallProgress(ctx, agentexec.DeployProgressPayload{
		TargetID: "tgt_failp_1",
		Phase:    agentexec.DeployPhaseInstallExecute,
		Status:   agentexec.DeployStepFailed,
		Message:  "install script exited 1",
	}, 2)

	target, _ := h.store.GetTarget(ctx, "tgt_failp_1")
	if target.Status != deploy.TargetFailedPermanent {
		t.Fatalf("expected status failed_permanent, got %q", target.Status)
	}
}

func TestProcessInstallProgress_AgentDisconnect(t *testing.T) {
	h := newTestDeployHandlers(t, nil, nil)
	ctx := context.Background()
	now := time.Now().UTC()

	if err := h.store.CreateJob(ctx, &deploy.Job{
		ID: "dep_disc", ClusterID: "lab", ClusterName: "lab",
		SourceAgentID: "agent-1", SourceNodeID: "node_a",
		OrgID: "default", Status: deploy.JobRunning,
		MaxParallel: 2, RetryMax: 3, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create job: %v", err)
	}

	// Close the channel immediately to simulate disconnect.
	ch := make(chan agentexec.DeployProgressPayload, 1)
	close(ch)

	// Run in foreground for testing.
	h.processInstallProgress("default", "dep_disc", "agent-1", 3, ch)

	// Job should be failed.
	job, _ := h.store.GetJob(ctx, "dep_disc")
	if job.Status != deploy.JobFailed {
		t.Fatalf("expected status failed after disconnect, got %q", job.Status)
	}
}

func TestGetTargetArchFromPreflight(t *testing.T) {
	h := newTestDeployHandlers(t, nil, nil)
	ctx := context.Background()
	now := time.Now().UTC()

	if err := h.store.CreateJob(ctx, &deploy.Job{
		ID: "pf_arch", ClusterID: "lab", ClusterName: "lab",
		SourceAgentID: "agent-1", SourceNodeID: "node_a",
		OrgID: "default", Status: deploy.JobSucceeded,
		MaxParallel: 2, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create job: %v", err)
	}

	// Create target with arch set on it.
	if err := h.store.CreateTarget(ctx, &deploy.Target{
		ID: "tgt_arch_1", JobID: "pf_arch", NodeID: "node_b",
		NodeName: "pve-b", NodeIP: "10.0.0.2", Arch: "arm64",
		Status: deploy.TargetReady, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create target: %v", err)
	}

	got := h.getTargetArchFromPreflight(ctx, "pf_arch", "node_b")
	if got != "arm64" {
		t.Fatalf("expected arm64, got %q", got)
	}

	// Fallback for unknown node.
	got = h.getTargetArchFromPreflight(ctx, "pf_arch", "node_unknown")
	if got != "amd64" {
		t.Fatalf("expected fallback amd64, got %q", got)
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

// clusterDeployAddressFixture models a cluster whose members Pulse discovered by
// name: the API URLs carry hostnames, while the cluster endpoints record the
// addresses Proxmox reports (and, for pve-c, an operator override).
func clusterDeployAddressFixture(t *testing.T) *DeployHandlers {
	t.Helper()
	nodes := []models.Node{
		{
			ID: "node_pve-a", Name: "pve-a", Host: "https://10.0.0.1:8006", Instance: "lab-conn",
			IsClusterMember: true, ClusterName: "lab", LinkedAgentID: "host-a",
		},
		{
			ID: "node_pve-b", Name: "pve-b", Host: "https://pve-b:8006", Instance: "lab-conn",
			IsClusterMember: true, ClusterName: "lab",
		},
		{
			ID: "node_pve-c", Name: "pve-c", Host: "https://pve-c:8006", Instance: "lab-conn",
			IsClusterMember: true, ClusterName: "lab",
		},
		{
			ID: "node_pve-d", Name: "pve-d", Host: "https://pve-d:8006", Instance: "lab-conn",
			IsClusterMember: true, ClusterName: "lab",
		},
		{
			ID: "node_pve-e", Name: "pve-e", Host: "https://10.0.0.5:8006", Instance: "lab-conn",
			IsClusterMember: true, ClusterName: "lab",
		},
	}
	h := newTestDeployHandlers(t, nodes, nil)
	h.config.PVEInstances = []config.PVEInstance{{
		Name:        "lab-conn",
		IsCluster:   true,
		ClusterName: "lab",
		ClusterEndpoints: []config.ClusterEndpoint{
			{NodeName: "pve-a", Host: "https://10.0.0.1:8006", IP: "10.0.0.1"},
			{NodeName: "pve-b", Host: "https://pve-b:8006", IP: "10.0.0.2"},
			{NodeName: "pve-c", Host: "https://pve-c:8006", IP: "10.0.0.3", IPOverride: "10.0.9.3"},
			// pve-d has no recorded address at all.
			{NodeName: "pve-d", Host: "https://pve-d:8006"},
			// pve-e's API URL already holds a literal IP; that wins.
			{NodeName: "pve-e", Host: "https://10.0.0.5:8006", IP: "10.0.0.55"},
		},
	}}
	return h
}

func TestHandleCandidatesResolvesHostnameMembersToClusterAddress(t *testing.T) {
	h := clusterDeployAddressFixture(t)
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

	want := map[string]struct {
		ip         string
		deployable bool
		reason     string
	}{
		"pve-a": {ip: "10.0.0.1", deployable: false, reason: "already_agent"},
		"pve-b": {ip: "10.0.0.2", deployable: true},
		"pve-c": {ip: "10.0.9.3", deployable: true},
		"pve-d": {ip: "", deployable: false, reason: "no_address"},
		"pve-e": {ip: "10.0.0.5", deployable: true},
	}
	if len(resp.Nodes) != len(want) {
		t.Fatalf("expected %d nodes, got %d: %+v", len(want), len(resp.Nodes), resp.Nodes)
	}
	for _, n := range resp.Nodes {
		w, ok := want[n.Name]
		if !ok {
			t.Errorf("unexpected node %q", n.Name)
			continue
		}
		if n.IP != w.ip || n.Deployable != w.deployable || n.Reason != w.reason {
			t.Errorf("%s: got ip=%q deployable=%v reason=%q, want ip=%q deployable=%v reason=%q",
				n.Name, n.IP, n.Deployable, n.Reason, w.ip, w.deployable, w.reason)
		}
	}
}

// candidateNodeID returns the unified node ID the candidates endpoint reports
// for a member name, which is what the UI sends back as a preflight target.
func candidateNodeID(t *testing.T, h *DeployHandlers, name string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	h.HandleCandidates(rec, httptest.NewRequest(http.MethodGet, "/api/clusters/lab/agent-deploy/candidates", nil))
	var resp candidatesResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode candidates: %v", err)
	}
	for _, n := range resp.Nodes {
		if n.Name == name {
			return n.NodeID
		}
	}
	t.Fatalf("candidate %q not found in %+v", name, resp.Nodes)
	return ""
}

func TestHandleCreatePreflightSendsClusterAddressForHostnameMember(t *testing.T) {
	h := clusterDeployAddressFixture(t)
	t.Cleanup(h.execServer.TestRegisterAgent("host-a", "host-a"))

	body := `{"sourceAgentId":"host-a","targetNodeIds":["` + candidateNodeID(t, h, "pve-b") + `"],"maxParallel":1}`
	req := httptest.NewRequest(http.MethodPost, "/api/clusters/lab/agent-deploy/preflights", strings.NewReader(body))
	rec := httptest.NewRecorder()

	h.HandleCreatePreflight(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp createPreflightResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	targets, err := h.store.GetTargetsForJob(context.Background(), resp.PreflightID)
	if err != nil {
		t.Fatalf("get targets: %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets))
	}
	if targets[0].NodeIP != "10.0.0.2" {
		t.Fatalf("target NodeIP = %q, want the cluster-reported 10.0.0.2 rather than the hostname", targets[0].NodeIP)
	}
}

func TestHandleCreatePreflightRejectsMemberWithoutAddress(t *testing.T) {
	h := clusterDeployAddressFixture(t)
	t.Cleanup(h.execServer.TestRegisterAgent("host-a", "host-a"))

	body := `{"sourceAgentId":"host-a","targetNodeIds":["` + candidateNodeID(t, h, "pve-d") + `"],"maxParallel":1}`
	req := httptest.NewRequest(http.MethodPost, "/api/clusters/lab/agent-deploy/preflights", strings.NewReader(body))
	rec := httptest.NewRecorder()

	h.HandleCreatePreflight(rec, req)

	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "no_valid_targets") {
		t.Fatalf("expected 400 no_valid_targets, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestDeployRefusesClusterNameSharedByTwoConnections(t *testing.T) {
	// Two sites whose clusters happen to share a name must not be merged: the
	// deploy routes are keyed on the name, so serving them would let a source
	// agent at one site be pointed at the other site's member addresses.
	nodes := []models.Node{
		{
			ID: "node_a1", Name: "pve-a", Host: "https://10.0.0.1:8006", Instance: "site-a",
			IsClusterMember: true, ClusterName: "lab", LinkedAgentID: "host-a",
		},
		{
			ID: "node_b1", Name: "pve-b", Host: "https://10.9.0.1:8006", Instance: "site-b",
			IsClusterMember: true, ClusterName: "lab",
		},
	}
	h := newTestDeployHandlers(t, nodes, nil)
	t.Cleanup(h.execServer.TestRegisterAgent("host-a", "host-a"))

	rec := httptest.NewRecorder()
	h.HandleCandidates(rec, httptest.NewRequest(http.MethodGet, "/api/clusters/lab/agent-deploy/candidates", nil))
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "ambiguous_cluster") {
		t.Fatalf("candidates: expected 409 ambiguous_cluster, got %d: %s", rec.Code, rec.Body.String())
	}

	body := `{"sourceAgentId":"host-a","targetNodeIds":["anything"],"maxParallel":1}`
	rec = httptest.NewRecorder()
	h.HandleCreatePreflight(rec, httptest.NewRequest(http.MethodPost, "/api/clusters/lab/agent-deploy/preflights", strings.NewReader(body)))
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "ambiguous_cluster") {
		t.Fatalf("preflight: expected 409 ambiguous_cluster, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestDeployTargetIPNeverBorrowsAnotherConnectionsEndpoint(t *testing.T) {
	nodes := []models.Node{
		{
			ID: "node_pve-a", Name: "pve-a", Host: "https://10.0.0.1:8006", Instance: "site-a",
			IsClusterMember: true, ClusterName: "lab", LinkedAgentID: "host-a",
		},
		{
			ID: "node_pve-b", Name: "pve-b", Host: "https://pve-b:8006", Instance: "site-a",
			IsClusterMember: true, ClusterName: "lab",
		},
	}
	h := newTestDeployHandlers(t, nodes, nil)
	// site-b is listed first and has a same-named cluster and member; its
	// address must never be used for site-a's pve-b.
	h.config.PVEInstances = []config.PVEInstance{
		{
			Name: "site-b", IsCluster: true, ClusterName: "lab",
			ClusterEndpoints: []config.ClusterEndpoint{{NodeName: "pve-b", IP: "10.9.0.2"}},
		},
		{
			Name: "site-a", IsCluster: true, ClusterName: "lab",
			ClusterEndpoints: []config.ClusterEndpoint{{NodeName: "pve-b", IP: "10.0.0.2"}},
		},
	}

	rec := httptest.NewRecorder()
	h.HandleCandidates(rec, httptest.NewRequest(http.MethodGet, "/api/clusters/lab/agent-deploy/candidates", nil))
	var resp candidatesResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v (body %s)", err, rec.Body.String())
	}
	for _, n := range resp.Nodes {
		if n.Name == "pve-b" && n.IP != "10.0.0.2" {
			t.Fatalf("pve-b resolved to %q, want its own connection's 10.0.0.2", n.IP)
		}
	}
}

func TestHandleCreateJob_RefusesWhileAnotherInstallIsRunning(t *testing.T) {
	nodes := []models.Node{
		{
			ID: "node_pve-a", Name: "pve-a", Host: "https://10.0.0.1:8006",
			IsClusterMember: true, ClusterName: "lab", LinkedAgentID: "host-a",
		},
		{
			ID: "node_pve-b", Name: "pve-b", Host: "https://10.0.0.2:8006",
			IsClusterMember: true, ClusterName: "lab",
		},
	}
	h := newTestDeployHandlers(t, nodes, nil)
	ctx := context.Background()
	t.Cleanup(h.execServer.TestRegisterAgent("host-a", "host-a"))

	now := time.Now().UTC()
	for _, j := range []*deploy.Job{
		{ID: "pf_ok", ClusterID: "lab", ClusterName: "lab", SourceAgentID: "host-a", SourceNodeID: "node_pve-a",
			OrgID: "default", Status: deploy.JobSucceeded, MaxParallel: 1, CreatedAt: now, UpdatedAt: now},
		// A stale install that never finished must not block anything.
		{ID: "dep_stale", ClusterID: "lab", ClusterName: "lab", SourceAgentID: "host-a", SourceNodeID: "node_pve-a",
			OrgID: "default", Status: deploy.JobRunning, MaxParallel: 1,
			CreatedAt: now.Add(-2 * deployActiveWindow), UpdatedAt: now.Add(-2 * deployActiveWindow)},
	} {
		if err := h.store.CreateJob(ctx, j); err != nil {
			t.Fatalf("create job %s: %v", j.ID, err)
		}
	}
	if err := h.store.CreateTarget(ctx, &deploy.Target{
		ID: "tgt_b", JobID: "pf_ok", NodeID: "node_pve-b", NodeName: "pve-b", NodeIP: "10.0.0.2",
		Arch: "amd64", Status: deploy.TargetReady, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create target: %v", err)
	}
	body := `{"sourceAgentId":"host-a","preflightId":"pf_ok","targetNodeIds":["node_pve-b"]}`

	rec := httptest.NewRecorder()
	h.HandleCreateJob(rec, httptest.NewRequest(http.MethodPost, "/api/clusters/lab/agent-deploy/jobs", strings.NewReader(body)))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("first install: expected 202 despite the stale job, got %d: %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	h.HandleCreateJob(rec, httptest.NewRequest(http.MethodPost, "/api/clusters/lab/agent-deploy/jobs", strings.NewReader(body)))
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "deploy_in_progress") {
		t.Fatalf("overlapping install: expected 409 deploy_in_progress, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleCreateJob_ConcurrentRequestsAdmitOneInstall(t *testing.T) {
	nodes := []models.Node{
		{
			ID: "node_pve-a", Name: "pve-a", Host: "https://10.0.0.1:8006",
			IsClusterMember: true, ClusterName: "lab", LinkedAgentID: "host-a",
		},
		{
			ID: "node_pve-b", Name: "pve-b", Host: "https://10.0.0.2:8006",
			IsClusterMember: true, ClusterName: "lab",
		},
	}
	h := newTestDeployHandlers(t, nodes, nil)
	ctx := context.Background()
	t.Cleanup(h.execServer.TestRegisterAgent("host-a", "host-a"))

	now := time.Now().UTC()
	if err := h.store.CreateJob(ctx, &deploy.Job{
		ID: "pf_ok", ClusterID: "lab", ClusterName: "lab", SourceAgentID: "host-a", SourceNodeID: "node_pve-a",
		OrgID: "default", Status: deploy.JobSucceeded, MaxParallel: 1, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create preflight: %v", err)
	}
	if err := h.store.CreateTarget(ctx, &deploy.Target{
		ID: "tgt_b", JobID: "pf_ok", NodeID: "node_pve-b", NodeName: "pve-b", NodeIP: "10.0.0.2",
		Arch: "amd64", Status: deploy.TargetReady, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create target: %v", err)
	}
	body := `{"sourceAgentId":"host-a","preflightId":"pf_ok","targetNodeIds":["node_pve-b"]}`

	const requests = 8
	codes := make(chan int, requests)
	var wg sync.WaitGroup
	for i := 0; i < requests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rec := httptest.NewRecorder()
			h.HandleCreateJob(rec, httptest.NewRequest(http.MethodPost, "/api/clusters/lab/agent-deploy/jobs", strings.NewReader(body)))
			codes <- rec.Code
		}()
	}
	wg.Wait()
	close(codes)

	accepted, refused := 0, 0
	for code := range codes {
		switch code {
		case http.StatusAccepted:
			accepted++
		case http.StatusConflict:
			refused++
		default:
			t.Errorf("unexpected status %d", code)
		}
	}
	if accepted != 1 || refused != requests-1 {
		t.Fatalf("accepted=%d refused=%d, want exactly one install admitted", accepted, refused)
	}
}
