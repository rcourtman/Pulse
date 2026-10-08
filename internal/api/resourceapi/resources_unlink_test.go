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

// monitorAdapterSeedProvider seeds the resources API from a monitor adapter's
// current generation, as the monitor does in production: REST receives the
// already-linked list and applies the store's decisions on top.
type monitorAdapterSeedProvider struct {
	adapter  *unified.MonitorAdapter
	snapshot models.StateSnapshot
}

func (p *monitorAdapterSeedProvider) ReadSnapshot() models.StateSnapshot { return p.snapshot }

func (p *monitorAdapterSeedProvider) UnifiedResourceSnapshot() ([]unified.Resource, time.Time) {
	return p.adapter.GetAll(), p.adapter.LastRebuiltAt()
}

// Unlinking a manually linked pair splits it on every surface. The link row
// used to survive the unlink, so the agent stayed folded into the VM in the
// monitor's rebuilds and in REST although the API answered "Resources
// unlinked". A later link joins the pair again: each decision replaces the
// previous one for that pair.
func TestResourceUnlinkSplitsManuallyLinkedPair(t *testing.T) {
	now := time.Now().UTC()
	h := NewQueryService(&config.Config{DataPath: t.TempDir()})
	store, err := h.getStore("default")
	if err != nil {
		t.Fatalf("getStore: %v", err)
	}
	snapshot := models.StateSnapshot{
		LastUpdate: now,
		VMs:        []models.VM{{ID: "lab:pve1:101", VMID: 101, Name: "web", Node: "pve1", Instance: "lab", Status: "running", Type: "qemu", LastSeen: now}},
		Hosts:      []models.Host{{ID: "host-app", Hostname: "app-agent", MachineID: "fedcba9876543210", Status: "online", LastSeen: now}},
	}
	adapter := unified.NewMonitorAdapter(unified.NewRegistry(store))
	h.SetStateProvider(&monitorAdapterSeedProvider{adapter: adapter, snapshot: snapshot})
	// Identity pins persist on the first rebuild and steer the second, so a
	// decision only counts once it survives two generations.
	rebuild := func() {
		t.Helper()
		adapter.PopulateFromSnapshot(snapshot)
		adapter.PopulateFromSnapshot(snapshot)
	}
	rebuild()

	var vmID, agentID string
	for _, resource := range adapter.GetAll() {
		switch resource.Type {
		case unified.ResourceTypeVM:
			vmID = resource.ID
		case unified.ResourceTypeAgent:
			agentID = resource.ID
		}
	}
	if vmID == "" || agentID == "" {
		t.Fatalf("fixture did not produce a standalone VM and agent: %+v", adapter.GetAll())
	}

	post := func(action string) {
		t.Helper()
		body, _ := json.Marshal(map[string]string{"targetId": agentID})
		rec := httptest.NewRecorder()
		h.HandleResourceRoutes(rec, httptest.NewRequest(http.MethodPost, "/api/resources/"+vmID+"/"+action, bytes.NewReader(body)))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d body=%s", action, rec.Code, rec.Body.String())
		}
	}
	get := func(id string) (unified.Resource, bool) {
		t.Helper()
		rec := httptest.NewRecorder()
		h.HandleResourceRoutes(rec, httptest.NewRequest(http.MethodGet, "/api/resources/"+id, nil))
		if rec.Code == http.StatusNotFound {
			return unified.Resource{}, false
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("get %s status = %d body=%s", id, rec.Code, rec.Body.String())
		}
		var resource unified.Resource
		if err := json.NewDecoder(rec.Body).Decode(&resource); err != nil {
			t.Fatalf("decode %s: %v", id, err)
		}
		return resource, true
	}
	assertLinked := func(step string, wantLinked bool) {
		t.Helper()
		vm, ok := get(vmID)
		if !ok {
			t.Fatalf("%s: VM %s missing from REST", step, vmID)
		}
		_, agentListed := get(agentID)
		if wantLinked && (vm.Agent == nil || agentListed) {
			t.Fatalf("%s: want agent folded into the VM, VM agent facet=%v agent row listed=%v", step, vm.Agent != nil, agentListed)
		}
		if !wantLinked && (vm.Agent != nil || !agentListed) {
			t.Fatalf("%s: want VM and agent apart, VM agent facet=%v agent row listed=%v", step, vm.Agent != nil, agentListed)
		}
		var monitorVM *unified.Resource
		monitorAgentListed := false
		for _, resource := range adapter.GetAll() {
			resource := resource
			switch resource.ID {
			case vmID:
				monitorVM = &resource
			case agentID:
				monitorAgentListed = true
			}
		}
		if monitorVM == nil || (monitorVM.Agent != nil) != wantLinked || monitorAgentListed == wantLinked {
			t.Fatalf("%s: monitor disagrees with REST: VM present=%v agent listed=%v, want linked=%v", step, monitorVM != nil, monitorAgentListed, wantLinked)
		}
	}

	post("link")
	rebuild()
	assertLinked("after link", true)

	post("unlink")
	rebuild()
	assertLinked("after unlink", false)
	links, err := store.GetLinks()
	if err != nil || len(links) != 0 {
		t.Fatalf("unlink left links %+v (err %v)", links, err)
	}

	post("link")
	rebuild()
	assertLinked("after relink", true)
	exclusions, err := store.GetExclusions()
	if err != nil || len(exclusions) != 0 {
		t.Fatalf("relink left exclusions %+v (err %v)", exclusions, err)
	}
}
