package resourceapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// Report-merge on a resource an operator link folded together splits it on
// every surface, as unlink does. It used to record exclusions only against
// candidate IDs derived from the merged resource's type, which name neither
// side of the link, so the link row survived and the pair stayed merged while
// the API answered "Merge reported" and the drawer toasted "Resource split
// applied". REST seeds from the monitor's already-linked listing, as in
// production, so the folded row is never in its registry.
func TestResourceReportMergeSplitsOperatorLinkedPair(t *testing.T) {
	now := time.Now().UTC()
	vm := models.VM{ID: "lab:pve1:101", VMID: 101, Name: "web", Node: "pve1", Instance: "lab", Status: "running", Type: "qemu", LastSeen: now}
	agent := models.Host{ID: "host-box", Hostname: "box-agent", MachineID: "fedcba9876543210", Status: "online", LastSeen: now}
	node := models.Node{ID: "lab-pve1", Name: "pve1", Instance: "lab", Host: "https://10.0.0.5:8006", Status: "online", LastSeen: now}
	// The agent and the Docker host take canonical IDs derived from their
	// machine IDs, which no source-specific candidate reproduces, so a
	// same-type link between them fails the same way a cross-type one does.
	docker := models.DockerHost{ID: "docker-box", Hostname: "dock-box", MachineID: "0011223344556677", Status: "online", LastSeen: now}
	// Two linked agents share their only source, which the API used to refuse
	// as "Resource is not merged".
	spare := models.Host{ID: "host-spare", Hostname: "spare-agent", MachineID: "8899aabbccddeeff", Status: "online", LastSeen: now}

	for _, tc := range []struct {
		name     string
		snapshot models.StateSnapshot
		// linkFromAgent links from the agent's side, as the agent's drawer
		// would; otherwise from the other resource's side.
		linkFromAgent bool
	}{
		{"vm-agent", models.StateSnapshot{VMs: []models.VM{vm}, Hosts: []models.Host{agent}}, false},
		{"vm-agent-from-agent", models.StateSnapshot{VMs: []models.VM{vm}, Hosts: []models.Host{agent}}, true},
		{"node-agent", models.StateSnapshot{Nodes: []models.Node{node}, Hosts: []models.Host{agent}}, false},
		{"node-agent-from-agent", models.StateSnapshot{Nodes: []models.Node{node}, Hosts: []models.Host{agent}}, true},
		{"docker-agent", models.StateSnapshot{DockerHosts: []models.DockerHost{docker}, Hosts: []models.Host{agent}}, false},
		{"docker-agent-from-agent", models.StateSnapshot{DockerHosts: []models.DockerHost{docker}, Hosts: []models.Host{agent}}, true},
		{"agent-agent", models.StateSnapshot{Hosts: []models.Host{spare, agent}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.snapshot.LastUpdate = now
			h, store, rebuild := newReportMergeHarness(t, tc.snapshot)
			resources := rebuild()
			if len(resources) != 2 {
				t.Fatalf("fixture produced %d resources, want 2 apart: %+v", len(resources), resources)
			}
			var agentID, otherID string
			for _, resource := range resources {
				if resource.Name == agent.Hostname {
					agentID = resource.ID
				} else {
					otherID = resource.ID
				}
			}
			if agentID == "" || otherID == "" {
				t.Fatalf("fixture lacks a standalone agent: %+v", resources)
			}
			fromID, targetID := otherID, agentID
			if tc.linkFromAgent {
				fromID, targetID = agentID, otherID
			}
			postResourceAction(t, h, fromID, "link", map[string]any{"targetId": targetID})
			linked := rebuild()
			if len(linked) != 1 {
				t.Fatalf("link left %d monitor resources, want one: %+v", len(linked), linked)
			}
			assertRESTResourceCount(t, h, 1)

			// Report as the drawer's Split dialog does: every merged source.
			sources := make([]string, 0, len(linked[0].Sources))
			for _, source := range linked[0].Sources {
				sources = append(sources, string(source))
			}
			postResourceAction(t, h, linked[0].ID, "report-merge", map[string]any{"sources": sources})
			if split := rebuild(); len(split) != 2 {
				t.Fatalf("report-merge left %d monitor resources, want the pair apart: %+v", len(split), split)
			}
			assertRESTResourceCount(t, h, 2)
			if links, err := store.GetLinks(); err != nil || len(links) != 0 {
				t.Fatalf("report-merge left links %+v (err %v)", links, err)
			}
			exclusions, err := store.GetExclusions()
			if err != nil {
				t.Fatal(err)
			}
			excludedPair := false
			for _, exclusion := range exclusions {
				if (exclusion.ResourceA == agentID && exclusion.ResourceB == otherID) ||
					(exclusion.ResourceA == otherID && exclusion.ResourceB == agentID) {
					excludedPair = true
				}
			}
			if !excludedPair {
				t.Fatalf("report-merge did not exclude the linked pair %s/%s: %+v", agentID, otherID, exclusions)
			}
		})
	}
}

// A source filter undoes only the links whose folded side brought a named
// source in.
func TestResourceReportMergeSourceFilterSelectsLinks(t *testing.T) {
	now := time.Now().UTC()
	snapshot := models.StateSnapshot{
		LastUpdate: now,
		VMs:        []models.VM{{ID: "lab:pve1:101", VMID: 101, Name: "web", Node: "pve1", Instance: "lab", Status: "running", Type: "qemu", LastSeen: now}},
		Hosts:      []models.Host{{ID: "host-box", Hostname: "box-agent", MachineID: "fedcba9876543210", Status: "online", LastSeen: now}},
	}
	h, store, rebuild := newReportMergeHarness(t, snapshot)
	var vmID, agentID string
	for _, resource := range rebuild() {
		switch resource.Type {
		case unified.ResourceTypeVM:
			vmID = resource.ID
		case unified.ResourceTypeAgent:
			agentID = resource.ID
		}
	}
	postResourceAction(t, h, vmID, "link", map[string]any{"targetId": agentID})
	rebuild()

	// The VM's own source was not folded in by the link.
	postResourceAction(t, h, vmID, "report-merge", map[string]any{"sources": []string{"proxmox"}})
	if links, err := store.GetLinks(); err != nil || len(links) != 1 {
		t.Fatalf("a filter naming only the VM's own source changed the link: %+v (err %v)", links, err)
	}
	postResourceAction(t, h, vmID, "report-merge", map[string]any{"sources": []string{"agent"}})
	if links, err := store.GetLinks(); err != nil || len(links) != 0 {
		t.Fatalf("a filter naming the agent left the link: %+v (err %v)", links, err)
	}
	if split := rebuild(); len(split) != 2 {
		t.Fatalf("report-merge left %d monitor resources, want two: %+v", len(split), split)
	}
}

// Report-merge on a pair identity matching merged keeps it apart through the
// monitor's rebuilds, whose identity pins persist on one generation and steer
// the next.
func TestResourceReportMergeSplitsIdentityMergedPair(t *testing.T) {
	now := time.Now().UTC()
	interfaces := []models.HostNetworkInterface{{Name: "eth0", MAC: "aa:bb:cc:dd:ee:ff", Addresses: []string{"10.0.0.5"}}}
	snapshot := models.StateSnapshot{
		LastUpdate:  now,
		Hosts:       []models.Host{{ID: "host-1", Hostname: "alpha", MachineID: "0123456789abcdef", Status: "online", LastSeen: now, NetworkInterfaces: interfaces}},
		DockerHosts: []models.DockerHost{{ID: "docker-1", Hostname: "alpha", MachineID: "0123456789abcdef", Status: "online", CPUs: 4, LastSeen: now, NetworkInterfaces: interfaces}},
	}
	h, _, rebuild := newReportMergeHarness(t, snapshot)
	merged := rebuild()
	if len(merged) != 1 {
		t.Fatalf("fixture produced %d resources, want one merged: %+v", len(merged), merged)
	}
	sources := make([]string, 0, len(merged[0].Sources))
	for _, source := range merged[0].Sources {
		sources = append(sources, string(source))
	}
	postResourceAction(t, h, merged[0].ID, "report-merge", map[string]any{"sources": sources})
	if split := rebuild(); len(split) != 2 {
		t.Fatalf("report-merge left %d monitor resources, want two: %+v", len(split), split)
	}
	assertRESTResourceCount(t, h, 2)
}

// newReportMergeHarness serves the resources API from a monitor adapter on the
// same SQLite store, rebuilt three times per step: identity pins persist on one
// rebuild and steer the next.
func newReportMergeHarness(t *testing.T, snapshot models.StateSnapshot) (*QueryService, unified.ResourceStore, func() []unified.Resource) {
	t.Helper()
	h := NewQueryService(&config.Config{DataPath: t.TempDir()})
	store, err := h.getStore("default")
	if err != nil {
		t.Fatalf("getStore: %v", err)
	}
	adapter := unified.NewMonitorAdapter(unified.NewRegistry(store))
	h.SetStateProvider(&monitorAdapterSeedProvider{adapter: adapter, snapshot: snapshot})
	rebuild := func() []unified.Resource {
		t.Helper()
		for i := 0; i < 3; i++ {
			adapter.PopulateFromSnapshot(snapshot)
		}
		return adapter.GetAll()
	}
	return h, store, rebuild
}

func postResourceAction(t *testing.T, h *QueryService, resourceID, action string, payload map[string]any) {
	t.Helper()
	body, _ := json.Marshal(payload)
	rec := httptest.NewRecorder()
	h.HandleResourceRoutes(rec, httptest.NewRequest(http.MethodPost, "/api/resources/"+resourceID+"/"+action, bytes.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("%s %s status = %d body=%s", action, resourceID, rec.Code, rec.Body.String())
	}
}

func assertRESTResourceCount(t *testing.T, h *QueryService, want int) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.HandleListResources(rec, httptest.NewRequest(http.MethodGet, "/api/resources", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp ResourcesResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(resp.Data) != want {
		ids := make([]string, 0, len(resp.Data))
		for _, resource := range resp.Data {
			ids = append(ids, resource.ID+"("+string(resource.Type)+")")
		}
		t.Fatalf("REST lists %d resources %v, want %d", len(resp.Data), ids, want)
	}
}

// Canonical-ID succession re-keys link rows with UPDATE OR IGNORE but moves
// primary_id unconditionally, so a row whose re-key collided with the
// successor's own row keeps the retired endpoint and gains the successor as
// its primary. That row used to merge the successor through a pair it does
// not name, so reporting (or unlinking) the merged pair left it behind and
// the next rebuild merged the pair again.
func TestResourceReportMergeSplitsPairBehindASuccessionShadowedRow(t *testing.T) {
	now := time.Now().UTC()
	snapshot := models.StateSnapshot{
		LastUpdate: now,
		VMs:        []models.VM{{ID: "lab:pve1:101", VMID: 101, Name: "web", Node: "pve1", Instance: "lab", Status: "running", Type: "qemu", LastSeen: now}},
		Hosts:      []models.Host{{ID: "host-box", Hostname: "box-agent", MachineID: "fedcba9876543210", Status: "online", LastSeen: now}},
	}
	h, store, rebuild := newReportMergeHarness(t, snapshot)
	var vmID, agentID string
	for _, resource := range rebuild() {
		switch resource.Type {
		case unified.ResourceTypeVM:
			vmID = resource.ID
		case unified.ResourceTypeAgent:
			agentID = resource.ID
		}
	}
	if err := store.AddLink(unified.ResourceLink{ResourceA: "agent-retired0000000", ResourceB: vmID, PrimaryID: agentID, CreatedAt: now.Add(-time.Hour)}); err != nil {
		t.Fatal(err)
	}
	postResourceAction(t, h, vmID, "link", map[string]any{"targetId": agentID})
	linked := rebuild()
	if len(linked) != 1 {
		t.Fatalf("link left %d monitor resources, want one: %+v", len(linked), linked)
	}

	postResourceAction(t, h, linked[0].ID, "report-merge", map[string]any{})
	if split := rebuild(); len(split) != 2 {
		t.Fatalf("report-merge left %d monitor resources, want the pair apart: %+v", len(split), split)
	}
	assertRESTResourceCount(t, h, 2)
}
