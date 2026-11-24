package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
	agentsk8s "github.com/rcourtman/pulse-go-rewrite/pkg/agents/kubernetes"
)

func newKubernetesAgentHandlers(t *testing.T, cfg *config.Config) (*KubernetesAgentHandlers, *monitoring.Monitor) {
	t.Helper()

	if cfg == nil {
		cfg = &config.Config{DataPath: t.TempDir()}
	}
	if cfg.DataPath == "" {
		cfg.DataPath = t.TempDir()
	}

	monitor, err := monitoring.New(cfg)
	if err != nil {
		t.Fatalf("monitoring.New: %v", err)
	}
	t.Cleanup(func() { monitor.Stop() })

	hub := websocket.NewHub(nil)
	handler := NewKubernetesAgentHandlers(nil, monitor, hub)
	return handler, monitor
}

func seedKubernetesCluster(t *testing.T, monitor *monitoring.Monitor) string {
	t.Helper()

	report := agentsk8s.Report{
		Agent: agentsk8s.AgentInfo{
			ID:              "agent-1",
			Version:         "1.0.0",
			IntervalSeconds: 30,
		},
		Cluster: agentsk8s.ClusterInfo{
			ID:      "cluster-1",
			Name:    "cluster-1",
			Version: "1.28.0",
		},
		Nodes: []agentsk8s.Node{
			{Name: "node-1", Ready: true},
		},
		Pods: []agentsk8s.Pod{
			{Name: "pod-1", Namespace: "default", Phase: "Running"},
		},
		Timestamp: time.Now().UTC(),
	}

	cluster, err := monitor.ApplyKubernetesReport(report, nil)
	if err != nil {
		t.Fatalf("ApplyKubernetesReport: %v", err)
	}
	if cluster.ID == "" {
		t.Fatalf("expected cluster ID to be set")
	}
	return cluster.ID
}

func TestKubernetesAgentHandlers_SetMonitorGetMonitor(t *testing.T) {
	handler, monitor := newKubernetesAgentHandlers(t, nil)

	other, err := monitoring.New(&config.Config{DataPath: t.TempDir()})
	if err != nil {
		t.Fatalf("monitoring.New: %v", err)
	}
	t.Cleanup(func() { other.Stop() })

	handler.SetMonitor(other)
	if got := handler.getMonitor(context.Background()); got != other {
		t.Fatalf("getMonitor = %v, want %v", got, other)
	}

	handler.SetMonitor(monitor)
	if got := handler.getMonitor(context.Background()); got != monitor {
		t.Fatalf("getMonitor = %v, want %v", got, monitor)
	}
}

func TestKubernetesAgentHandlers_HandleReport(t *testing.T) {
	handler, _ := newKubernetesAgentHandlers(t, nil)

	report := agentsk8s.Report{
		Agent: agentsk8s.AgentInfo{
			ID:              "agent-2",
			Version:         "1.0.0",
			IntervalSeconds: 30,
		},
		Cluster: agentsk8s.ClusterInfo{
			ID:      "cluster-2",
			Name:    "cluster-2",
			Version: "1.28.0",
		},
		Timestamp: time.Now().UTC(),
	}
	body, _ := json.Marshal(report)

	req := httptest.NewRequest(http.MethodPost, "/api/agents/kubernetes/report", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.HandleReport(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
}

// TestKubernetesAgentHandlers_HandleReport_AllowsNewMonitoredSystemWithCapsRetired
// verifies that a new Kubernetes cluster is accepted without monitored-system caps.
func TestKubernetesAgentHandlers_HandleReport_AllowsNewMonitoredSystemWithCapsRetired(t *testing.T) {
	setMaxMonitoredSystemsLicenseForTests(t, 1)

	handler, monitor := newKubernetesAgentHandlers(t, nil)
	existingClusterID := seedKubernetesCluster(t, monitor)
	if existingClusterID == "" {
		t.Fatalf("expected seeded cluster ID")
	}

	// New cluster should be accepted because monitored-system caps are retired.
	newReport := agentsk8s.Report{
		Agent: agentsk8s.AgentInfo{
			ID:              "agent-2",
			Version:         "1.0.0",
			IntervalSeconds: 30,
		},
		Cluster: agentsk8s.ClusterInfo{
			ID:      "cluster-2",
			Name:    "cluster-2",
			Version: "1.28.0",
		},
		Timestamp: time.Now().UTC(),
	}
	newBody, _ := json.Marshal(newReport)
	newReq := httptest.NewRequest(http.MethodPost, "/api/agents/kubernetes/report", bytes.NewReader(newBody))
	newRec := httptest.NewRecorder()
	handler.HandleReport(newRec, newReq)
	if newRec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", newRec.Code, newRec.Body.String())
	}
}

func TestKubernetesAgentHandlers_HandleClusterActions(t *testing.T) {
	handler, _ := newKubernetesAgentHandlers(t, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/agents/kubernetes/clusters/cluster-1/allow-reenroll", nil)
	rec := httptest.NewRecorder()

	handler.HandleClusterActions(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
}

func TestKubernetesAgentHandlers_HandleDeleteCluster(t *testing.T) {
	handler, monitor := newKubernetesAgentHandlers(t, nil)
	clusterID := seedKubernetesCluster(t, monitor)

	req := httptest.NewRequest(http.MethodDelete, "/api/agents/kubernetes/clusters/"+clusterID, nil)
	rec := httptest.NewRecorder()

	handler.HandleDeleteCluster(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
}

func TestKubernetesAgentHandlers_HandleAllowReenroll(t *testing.T) {
	handler, _ := newKubernetesAgentHandlers(t, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/agents/kubernetes/clusters/cluster-1/allow-reenroll", nil)
	rec := httptest.NewRecorder()

	handler.HandleAllowReenroll(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
}

func TestKubernetesAgentHandlers_HandleUnhideCluster(t *testing.T) {
	handler, monitor := newKubernetesAgentHandlers(t, nil)
	clusterID := seedKubernetesCluster(t, monitor)

	req := httptest.NewRequest(http.MethodPut, "/api/agents/kubernetes/clusters/"+clusterID+"/unhide", nil)
	rec := httptest.NewRecorder()

	handler.HandleUnhideCluster(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
}

func TestKubernetesAgentHandlers_HandleMarkPendingUninstall(t *testing.T) {
	handler, monitor := newKubernetesAgentHandlers(t, nil)
	clusterID := seedKubernetesCluster(t, monitor)

	req := httptest.NewRequest(http.MethodPut, "/api/agents/kubernetes/clusters/"+clusterID+"/pending-uninstall", nil)
	rec := httptest.NewRecorder()

	handler.HandleMarkPendingUninstall(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
}

func TestKubernetesAgentHandlers_HandleSetCustomDisplayName(t *testing.T) {
	handler, monitor := newKubernetesAgentHandlers(t, nil)
	clusterID := seedKubernetesCluster(t, monitor)

	body := []byte(`{"displayName":"Custom Cluster"}`)
	req := httptest.NewRequest(http.MethodPut, "/api/agents/kubernetes/clusters/"+clusterID+"/display-name", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.HandleSetCustomDisplayName(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
}

type recordingKubernetesRecoveryIngestor struct {
	clusterIDs []string
}

func (i *recordingKubernetesRecoveryIngestor) IngestKubernetesReport(_ context.Context, _ string, report agentsk8s.Report) error {
	i.clusterIDs = append(i.clusterIDs, report.Cluster.ID)
	return nil
}

// A cluster ID is only what a report states, so a report from a token that does
// not own the cluster record must land on a record of its own, and its recovery
// artifacts must follow it there.
func TestKubernetesAgentHandlers_HandleReport_ForeignTokenGetsOwnCluster(t *testing.T) {
	handler, monitor := newKubernetesAgentHandlers(t, nil)
	ingestor := &recordingKubernetesRecoveryIngestor{}
	handler.SetRecoveryIngestor(ingestor)

	send := func(token *config.APITokenRecord, agentID string, nodeReady bool) string {
		t.Helper()
		body, _ := json.Marshal(agentsk8s.Report{
			Agent:     agentsk8s.AgentInfo{ID: agentID, Version: "1.0.0", IntervalSeconds: 30},
			Cluster:   agentsk8s.ClusterInfo{ID: "shared-cluster-id", Name: "in-cluster"},
			Nodes:     []agentsk8s.Node{{UID: "n1", Name: "node-1", Ready: nodeReady}},
			Recovery:  &agentsk8s.RecoveryReport{},
			Timestamp: time.Now().UTC(),
		})
		req := httptest.NewRequest(http.MethodPost, "/api/agents/kubernetes/report", bytes.NewReader(body))
		attachAPITokenRecord(req, token)
		rec := httptest.NewRecorder()
		handler.HandleReport(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
		}
		var resp struct {
			ClusterID string `json:"clusterId"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		return resp.ClusterID
	}

	ownerToken := &config.APITokenRecord{ID: "token-owner", Scopes: []string{config.ScopeKubernetesReport}}
	foreignToken := &config.APITokenRecord{ID: "token-laptop", Scopes: []string{config.ScopeKubernetesReport}}

	if got := send(ownerToken, "prod-agent", false); got != "shared-cluster-id" {
		t.Fatalf("owner clusterId = %q, want the reported ID", got)
	}
	foreignID := send(foreignToken, "laptop", true)
	if foreignID == "" || foreignID == "shared-cluster-id" {
		t.Fatalf("foreign token clusterId = %q, want a record of its own", foreignID)
	}

	if len(ingestor.clusterIDs) != 2 || ingestor.clusterIDs[0] != "shared-cluster-id" || ingestor.clusterIDs[1] != foreignID {
		t.Fatalf("recovery ingest cluster IDs = %v, want [shared-cluster-id %s]", ingestor.clusterIDs, foreignID)
	}

	for _, cluster := range monitor.GetState().KubernetesClusters {
		if cluster.ID != "shared-cluster-id" {
			continue
		}
		if cluster.TokenID != "token-owner" || cluster.AgentID != "prod-agent" {
			t.Fatalf("owner record taken over: token=%q agent=%q", cluster.TokenID, cluster.AgentID)
		}
		if len(cluster.Nodes) != 1 || cluster.Nodes[0].Ready {
			t.Fatalf("owner record nodes replaced: %+v", cluster.Nodes)
		}
		return
	}
	t.Fatal("owner record missing")
}
