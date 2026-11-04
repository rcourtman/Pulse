package monitoring

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func refusedBroadcastContainer(id, detail string) unifiedresources.Resource {
	return unifiedresources.Resource{
		ID:     id,
		Type:   unifiedresources.ResourceTypeAppContainer,
		Name:   id,
		Status: unifiedresources.StatusOnline,
		Capabilities: []unifiedresources.ResourceCapability{
			{Name: "stop", Type: unifiedresources.CapabilityTypeCommon},
		},
		ActionReadiness: []unifiedresources.ResourceActionReadiness{{
			Name:       "stop",
			Available:  false,
			ReasonCode: "command_agent_disconnected",
			Reason:     "Docker / Podman command agent is not connected.",
			Detail:     detail,
		}},
	}
}

func TestBroadcastRowsReferenceOneRefusalEntryAcrossHosts(t *testing.T) {
	healthy := refusedBroadcastContainer("container-healthy", "")
	healthy.ActionReadiness = nil

	rows, catalogs := convertPresentationResourcesForBroadcast([]unifiedresources.Resource{
		refusedBroadcastContainer("container-a", "no live command session for agent a"),
		refusedBroadcastContainer("container-b", "no live command session for agent b"),
		healthy,
	})

	byID := make(map[string]models.ResourceFrontend, len(rows))
	for _, row := range rows {
		byID[row.ID] = row
	}
	ref := byID["container-a"].ActionReadinessRef
	if ref == "" || byID["container-b"].ActionReadinessRef != ref {
		t.Fatalf("refused rows should share one ref, got %q and %q", ref, byID["container-b"].ActionReadinessRef)
	}
	if len(catalogs.actionReadiness) != 1 {
		t.Fatalf("catalog entries = %d, want 1: %v", len(catalogs.actionReadiness), catalogs.actionReadiness)
	}
	entry := string(catalogs.actionReadiness[ref])
	if !strings.Contains(entry, "command_agent_disconnected") || strings.Contains(entry, "no live command session") {
		t.Fatalf("catalog entry = %s, want the refusal without host detail", entry)
	}
	if byID["container-healthy"].ActionReadinessRef != "" {
		t.Fatalf("a row with nothing refused carries ref %q", byID["container-healthy"].ActionReadinessRef)
	}
	// The capability set stays whole on a refused row, so a disconnect moves
	// one ref per row and not two.
	if byID["container-a"].CapabilitiesRef == "" || byID["container-a"].CapabilitiesRef != byID["container-healthy"].CapabilitiesRef {
		t.Fatalf("capability refs differ: refused %q, healthy %q", byID["container-a"].CapabilitiesRef, byID["container-healthy"].CapabilitiesRef)
	}

	encoded, err := json.Marshal(byID["container-a"])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"actionReadinessRef":"`+ref+`"`) {
		t.Fatalf("wire row = %s, want actionReadinessRef", encoded)
	}
	if strings.Contains(string(encoded), `"actionReadiness":`) {
		t.Fatalf("wire row = %s, refusals must ride the catalog", encoded)
	}
}

func TestBroadcastWithNoRefusalsHasNoCatalog(t *testing.T) {
	healthy := refusedBroadcastContainer("container-healthy", "")
	healthy.ActionReadiness = nil
	_, catalogs := convertPresentationResourcesForBroadcast([]unifiedresources.Resource{healthy})
	if catalogs.actionReadiness != nil {
		t.Fatalf("catalog = %v, want nil so the field stays off the wire", catalogs.actionReadiness)
	}
}

func TestBroadcastStateRunsTheRefusalProjectionOnItsOwnRows(t *testing.T) {
	now := time.Now().UTC()
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestResources([]unifiedresources.Resource{{
		ID:       "container-1",
		Type:     unifiedresources.ResourceTypeAppContainer,
		Name:     "web",
		Status:   unifiedresources.StatusOnline,
		LastSeen: now,
		Sources:  []unifiedresources.DataSource{unifiedresources.SourceDocker},
		Docker:   &unifiedresources.DockerData{ContainerID: "abc", Runtime: "docker"},
		Capabilities: []unifiedresources.ResourceCapability{
			{Name: "stop", Type: unifiedresources.CapabilityTypeCommon},
		},
	}})
	store := &broadcastProjectionCountingStore{MonitorAdapter: unifiedresources.NewMonitorAdapter(registry)}
	m := &Monitor{resourceStore: store, orgID: "tenant-a"}
	snapshot := models.EmptyStateSnapshot()
	build := func() models.StateFrontend {
		return m.buildBroadcastFrontendStateFromSnapshotWithClock(snapshot, m.mockModeFence.begin(), func() time.Time { return now })
	}
	containerRow := func(state models.StateFrontend) models.ResourceFrontend {
		for _, row := range state.Resources {
			if row.Type == "app-container" {
				return row
			}
		}
		t.Fatalf("no container row: %+v", state.Resources)
		return models.ResourceFrontend{}
	}

	if row := containerRow(build()); row.ActionReadinessRef != "" {
		t.Fatalf("ref %q without a projection installed", row.ActionReadinessRef)
	}

	refused := true
	var sawOrg string
	m.SetBroadcastActionAvailability(func(orgID string, resources []unifiedresources.Resource) {
		sawOrg = orgID
		if !refused {
			return
		}
		for i := range resources {
			if resources[i].ID == "container-1" {
				resources[i].ActionReadiness = []unifiedresources.ResourceActionReadiness{{Name: "stop", ReasonCode: "command_agent_disconnected"}}
			}
		}
	})

	state := build()
	row := containerRow(state)
	if sawOrg != "tenant-a" {
		t.Fatalf("projection saw org %q, want the monitor's", sawOrg)
	}
	if row.ActionReadinessRef == "" || len(state.ActionReadinessCatalog[row.ActionReadinessRef]) == 0 {
		t.Fatalf("row ref %q, catalog %v", row.ActionReadinessRef, state.ActionReadinessCatalog)
	}

	// The projection decorates the broadcast's own rows, never the store's.
	for _, resource := range store.GetAll() {
		if resource.ActionReadiness != nil {
			t.Fatalf("store row %s gained broadcast readiness: %#v", resource.ID, resource.ActionReadiness)
		}
	}

	refused = false
	state = build()
	if row := containerRow(state); row.ActionReadinessRef != "" || state.ActionReadinessCatalog != nil {
		t.Fatalf("refusal outlived the check: ref %q, catalog %v", row.ActionReadinessRef, state.ActionReadinessCatalog)
	}

	m.SetBroadcastActionAvailability(nil)
	if row := containerRow(build()); row.ActionReadinessRef != "" {
		t.Fatalf("ref %q after clearing the projection", row.ActionReadinessRef)
	}
}
