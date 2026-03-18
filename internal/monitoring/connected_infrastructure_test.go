package monitoring

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	unifiedresources "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestBuildConnectedInfrastructure_AppliesIgnoreStateToActiveSurfaces(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	items := buildConnectedInfrastructure([]unifiedresources.Resource{
		{
			ID:       "tower-resource",
			Name:     "Tower",
			LastSeen: now,
			Status:   unifiedresources.StatusOnline,
			Agent: &unifiedresources.AgentData{
				AgentID:      "tower-agent",
				AgentVersion: "1.2.3",
				Hostname:     "tower.local",
				Platform:     "linux",
			},
			Docker: &unifiedresources.DockerData{
				HostSourceID: "tower-docker",
				AgentID:      "tower-agent",
			},
			PBS: &unifiedresources.PBSData{
				Version: "3.4.1",
			},
			Identity: unifiedresources.ResourceIdentity{
				Hostnames: []string{"tower.local"},
			},
		},
	}, models.StateSnapshot{
		RemovedDockerHosts: []models.RemovedDockerHost{
			{ID: "tower-docker", RemovedAt: now},
		},
	})

	if len(items) != 2 {
		t.Fatalf("expected 2 connected infrastructure items, got %d", len(items))
	}

	var active *models.ConnectedInfrastructureItemFrontend
	var ignored *models.ConnectedInfrastructureItemFrontend
	for i := range items {
		item := &items[i]
		switch item.Status {
		case "active":
			active = item
		case "ignored":
			ignored = item
		}
	}

	if active == nil {
		t.Fatal("expected active connected infrastructure item")
	}
	if ignored == nil {
		t.Fatal("expected ignored connected infrastructure item")
	}

	if len(active.Surfaces) != 2 {
		t.Fatalf("expected host-managed active surfaces to remain without docker, got %#v", active.Surfaces)
	}
	for _, surface := range active.Surfaces {
		if surface.Kind == "docker" {
			t.Fatalf("expected docker surface to be removed from active item, got %#v", active.Surfaces)
		}
	}

	if len(ignored.Surfaces) != 1 || ignored.Surfaces[0].Kind != "docker" {
		t.Fatalf("expected ignored docker surface, got %#v", ignored.Surfaces)
	}
}
