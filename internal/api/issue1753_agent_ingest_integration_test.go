package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

// Issue #1753 crossed three runtime boundaries: report authentication, host to
// provider-node linking, and canonical presentation. Keep those boundaries in
// one regression test so two valid agents from independent sites cannot be
// mistaken for an auth failure or collapsed merely because both PVE machines
// report the same native short hostname.
func TestIssue1753SameNameProxmoxAgentsAuthenticateAndStayDistinctEndToEnd(t *testing.T) {
	const (
		stagingToken    = "issue-1753-staging-agent-token.12345678"
		productionToken = "issue-1753-production-agent-token.12345678"
	)
	stagingRecord := newTokenRecord(t, stagingToken, []string{config.ScopeAgentReport}, nil)
	productionRecord := newTokenRecord(t, productionToken, []string{config.ScopeAgentReport}, nil)

	dataPath := t.TempDir()
	cfg := &config.Config{
		DataPath:   dataPath,
		ConfigPath: dataPath,
		APITokens:  []config.APITokenRecord{stagingRecord, productionRecord},
	}
	monitor, err := monitoring.New(cfg)
	if err != nil {
		t.Fatalf("monitoring.New: %v", err)
	}
	t.Cleanup(monitor.Stop)

	now := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	monitorState(t, monitor).UpdateNodes([]models.Node{
		{
			ID: "staging-pve", NodeIdentity: "staging-pve", Name: "pve",
			DisplayName: "Staging", Instance: "staging", Host: "https://pve.staging.example:8006",
			Status: "online", LastSeen: now,
			NetworkInterfaces: []models.HostNetworkInterface{{Name: "vmbr0", Addresses: []string{"192.0.2.11/24"}}},
		},
		{
			ID: "production-pve", NodeIdentity: "production-pve", Name: "pve",
			DisplayName: "Production", Instance: "production", Host: "https://pve.production.example:8006",
			Status: "online", LastSeen: now,
			NetworkInterfaces: []models.HostNetworkInterface{{Name: "vmbr0", Addresses: []string{"198.51.100.21/24"}}},
		},
	})

	router := NewRouter(cfg, monitor, nil, nil, func() error { return nil }, "6.4.2")
	t.Cleanup(router.shutdownBackgroundWorkers)

	postReport := func(rawToken, agentID, machineID, address string, at time.Time) {
		t.Helper()
		report := agentshost.Report{
			Agent: agentshost.AgentInfo{ID: agentID, Version: "6.4.2", Type: "unified"},
			Host: agentshost.HostInfo{
				ID: machineID, MachineID: machineID, Hostname: "pve",
				Platform: "linux", OSName: "Proxmox VE",
			},
			Network:   []agentshost.NetworkInterface{{Name: "vmbr0", Addresses: []string{address}}},
			Timestamp: at,
		}
		body, err := json.Marshal(report)
		if err != nil {
			t.Fatalf("marshal report: %v", err)
		}
		req := httptest.NewRequest(http.MethodPost, "/api/agents/agent/report", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+rawToken)
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("agent %q report status = %d, want 200: %s", agentID, rec.Code, rec.Body.String())
		}
	}

	postReport(stagingToken, "agent-staging", "machine-staging", "192.0.2.11/24", now.Add(time.Second))
	postReport(productionToken, "agent-production", "machine-production", "198.51.100.21/24", now.Add(2*time.Second))

	snapshot := monitor.GetLiveStateSnapshot()
	if len(snapshot.Hosts) != 2 || len(snapshot.Nodes) != 2 {
		t.Fatalf("live topology = %d hosts / %d nodes, want 2 / 2: %+v", len(snapshot.Hosts), len(snapshot.Nodes), snapshot)
	}
	linkedByInstance := make(map[string]string, len(snapshot.Nodes))
	for _, node := range snapshot.Nodes {
		linkedByInstance[node.Instance] = node.LinkedAgentID
	}
	if linkedByInstance["staging"] != "machine-staging" || linkedByInstance["production"] != "machine-production" {
		t.Fatalf("provider links = %+v, want each site linked to its own authenticated agent", linkedByInstance)
	}

	resources, _ := monitor.UnifiedResourceSnapshot()
	byInstance := make(map[string]unifiedresources.Resource)
	for _, resource := range resources {
		if resource.Proxmox != nil && resource.Proxmox.NodeName != "" {
			byInstance[resource.Proxmox.Instance] = resource
		}
	}
	if len(byInstance) != 2 {
		t.Fatalf("presentation provider rows = %d, want 2: %+v", len(byInstance), resources)
	}
	for instance, want := range map[string]struct {
		agentID string
		name    string
	}{
		"staging":    {agentID: "machine-staging", name: "Staging"},
		"production": {agentID: "machine-production", name: "Production"},
	} {
		resource := byInstance[instance]
		if resource.Agent == nil || resource.Agent.AgentID != want.agentID || resource.Name != want.name {
			t.Fatalf("%s presentation row = %+v, want agent %q and name %q", instance, resource, want.agentID, want.name)
		}
	}
}
