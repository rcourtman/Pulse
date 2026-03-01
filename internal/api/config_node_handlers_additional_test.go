package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// TestHandleGetNodes_MockMode_UsesReadState verifies that the mock-mode branch
// of handleGetNodes uses ReadState typed accessors instead of GetState().
func TestHandleGetNodes_MockMode_UsesReadState(t *testing.T) {
	mock.SetEnabled(true)
	t.Cleanup(func() { mock.SetEnabled(false) })

	// Create a monitor with a resource store populated via ReadState.
	monitor := &monitoring.Monitor{}
	state := models.NewState()
	state.Nodes = []models.Node{
		{ID: "node/pve1", Name: "pve1", Status: "online", Instance: "mock-cluster", Host: "192.168.0.100:8006"},
		{ID: "node/pve2", Name: "pve2", Status: "online", Instance: "mock-cluster", Host: "192.168.0.101:8006"},
		{ID: "node/standalone", Name: "standalone", Status: "online", Instance: "standalone-inst", Host: "192.168.0.150:8006"},
	}
	state.PBSInstances = []models.PBSInstance{
		{ID: "pbs-main", Name: "pbs-main", Host: "192.168.0.10:8007", Status: "online"},
	}
	state.PMGInstances = []models.PMGInstance{
		{ID: "pmg-main", Name: "pmg-main", Host: "192.168.0.20:8006", Status: "online"},
	}
	setUnexportedField(t, monitor, "state", state)
	syncTestResourceStore(t, monitor, state)

	h := &ConfigHandlers{
		legacyMonitor: monitor,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/config/nodes", nil)
	rec := httptest.NewRecorder()

	h.handleGetNodes(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var nodes []NodeResponse
	if err := json.NewDecoder(rec.Body).Decode(&nodes); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Expect: 1 cluster entry + 1 standalone + 1 PBS + 1 PMG = 4
	if len(nodes) != 4 {
		t.Fatalf("expected 4 node responses, got %d: %+v", len(nodes), nodes)
	}

	// Verify cluster entry
	cluster := nodes[0]
	if cluster.Type != "pve" || !cluster.IsCluster {
		t.Errorf("expected first entry to be PVE cluster, got type=%q isCluster=%v", cluster.Type, cluster.IsCluster)
	}
	if len(cluster.ClusterEndpoints) != 2 {
		t.Errorf("expected 2 cluster endpoints, got %d", len(cluster.ClusterEndpoints))
	}

	// Verify standalone
	standalone := nodes[1]
	if standalone.Type != "pve" || standalone.IsCluster {
		t.Errorf("expected standalone PVE, got type=%q isCluster=%v", standalone.Type, standalone.IsCluster)
	}
	if standalone.Name != "standalone" {
		t.Errorf("expected standalone name, got %q", standalone.Name)
	}

	// Verify PBS
	pbsEntry := nodes[2]
	if pbsEntry.Type != "pbs" {
		t.Errorf("expected PBS type, got %q", pbsEntry.Type)
	}
	if pbsEntry.Name != "pbs-main" {
		t.Errorf("expected pbs-main name, got %q", pbsEntry.Name)
	}

	// Verify PMG
	pmgEntry := nodes[3]
	if pmgEntry.Type != "pmg" {
		t.Errorf("expected PMG type, got %q", pmgEntry.Type)
	}
	if pmgEntry.Name != "pmg-main" {
		t.Errorf("expected pmg-main name, got %q", pmgEntry.Name)
	}
}

// TestHandleGetNodes_MockMode_FallbackToGetState verifies the legacy fallback
// when ReadState is not yet populated (e.g., during startup). The handler
// should fall back to GetState() and still return mock data.
func TestHandleGetNodes_MockMode_FallbackToGetState(t *testing.T) {
	mock.SetEnabled(true)
	t.Cleanup(func() { mock.SetEnabled(false) })

	// Monitor with state but no resource store — GetUnifiedReadState() returns nil,
	// so the handler falls back to GetState().
	monitor := &monitoring.Monitor{}
	state := models.NewState()
	state.Nodes = []models.Node{
		{ID: "node/test", Name: "test-node", Status: "online", Instance: "standalone"},
	}
	setUnexportedField(t, monitor, "state", state)

	h := &ConfigHandlers{
		legacyMonitor: monitor,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/config/nodes", nil)
	rec := httptest.NewRecorder()

	h.handleGetNodes(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var nodes []NodeResponse
	if err := json.NewDecoder(rec.Body).Decode(&nodes); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	// Fallback to GetState() should produce at least 1 PVE node entry.
	hasPVE := false
	for _, n := range nodes {
		if n.Type == "pve" {
			hasPVE = true
			break
		}
	}
	if !hasPVE {
		t.Error("expected at least 1 PVE node from GetState() fallback")
	}
}

// Ensure ConfigHandlers.getMonitor falls back to legacyMonitor when no
// multi-tenant monitor is configured (used by mock-mode tests above).
func TestConfigHandlers_getMonitor_Legacy(t *testing.T) {
	monitor := &monitoring.Monitor{}
	h := &ConfigHandlers{legacyMonitor: monitor}

	got := h.getMonitor(context.Background())
	if got != monitor {
		t.Fatal("expected getMonitor to return legacy monitor")
	}
}

// TestHandleGetNodes_MockMode_ReadStateTakesPriority seeds the legacy state
// with one set of node names and the ReadState adapter with different names.
// Because both are populated, the handler must prefer ReadState. We assert the
// ReadState names appear in the response, proving the ReadState branch is taken.
func TestHandleGetNodes_MockMode_ReadStateTakesPriority(t *testing.T) {
	mock.SetEnabled(true)
	t.Cleanup(func() { mock.SetEnabled(false) })

	monitor := &monitoring.Monitor{}

	// Legacy state: node named "legacy-node"
	legacyState := models.NewState()
	legacyState.Nodes = []models.Node{
		{ID: "node/legacy", Name: "legacy-node", Status: "online", Instance: "standalone"},
	}
	setUnexportedField(t, monitor, "state", legacyState)

	// ReadState adapter: node named "readstate-node" (different from legacy)
	readStateSnapshot := models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "node/rs", Name: "readstate-node", Status: "online", Instance: "standalone"},
		},
	}
	adapter := unifiedresources.NewMonitorAdapter(nil)
	adapter.PopulateFromSnapshot(readStateSnapshot)
	setUnexportedField(t, monitor, "resourceStore", monitoring.ResourceStoreInterface(adapter))

	h := &ConfigHandlers{
		legacyMonitor: monitor,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/config/nodes", nil)
	rec := httptest.NewRecorder()

	h.handleGetNodes(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var nodes []NodeResponse
	if err := json.NewDecoder(rec.Body).Decode(&nodes); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Must find "readstate-node", NOT "legacy-node"
	foundReadState := false
	foundLegacy := false
	for _, n := range nodes {
		if n.Name == "readstate-node" {
			foundReadState = true
		}
		if n.Name == "legacy-node" {
			foundLegacy = true
		}
	}
	if !foundReadState {
		t.Error("expected ReadState node name 'readstate-node' in response — ReadState branch not taken")
	}
	if foundLegacy {
		t.Error("found legacy node name 'legacy-node' in response — handler used GetState() instead of ReadState")
	}
}

// Verify the MonitorAdapter satisfies ReadState when populated with PBS/PMG data.
func TestMonitorAdapter_PBSPMGViews(t *testing.T) {
	adapter := unifiedresources.NewMonitorAdapter(nil)
	snapshot := models.StateSnapshot{
		PBSInstances: []models.PBSInstance{
			{ID: "pbs-1", Name: "backup-server", Host: "10.0.0.1:8007", Status: "online"},
		},
		PMGInstances: []models.PMGInstance{
			{ID: "pmg-1", Name: "mail-gateway", Host: "10.0.0.2:8006", Status: "online"},
		},
	}
	adapter.PopulateFromSnapshot(snapshot)

	pbsList := adapter.PBSInstances()
	if len(pbsList) != 1 {
		t.Fatalf("expected 1 PBS instance, got %d", len(pbsList))
	}
	if pbsList[0].Name() != "backup-server" {
		t.Errorf("expected PBS name 'backup-server', got %q", pbsList[0].Name())
	}
	// Hostname is extracted without port
	if pbsList[0].Hostname() != "10.0.0.1" {
		t.Errorf("expected PBS hostname '10.0.0.1', got %q", pbsList[0].Hostname())
	}

	pmgList := adapter.PMGInstances()
	if len(pmgList) != 1 {
		t.Fatalf("expected 1 PMG instance, got %d", len(pmgList))
	}
	if pmgList[0].Name() != "mail-gateway" {
		t.Errorf("expected PMG name 'mail-gateway', got %q", pmgList[0].Name())
	}
	if pmgList[0].Hostname() != "10.0.0.2" {
		t.Errorf("expected PMG hostname '10.0.0.2', got %q", pmgList[0].Hostname())
	}
}
