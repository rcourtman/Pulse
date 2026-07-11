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

func TestBuildConnectedInfrastructure_UsesSharedDisplayNameFallback(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	items := buildConnectedInfrastructure([]unifiedresources.Resource{
		{
			ID:       "tower-resource",
			LastSeen: now,
			Status:   unifiedresources.StatusOnline,
			Canonical: &unifiedresources.CanonicalIdentity{
				DisplayName: "Canonical Tower",
			},
			Agent: &unifiedresources.AgentData{
				AgentID:      "tower-agent",
				AgentVersion: "1.2.3",
				Hostname:     "tower.local",
				Platform:     "linux",
			},
			Identity: unifiedresources.ResourceIdentity{
				Hostnames: []string{"tower.local"},
			},
		},
	}, models.StateSnapshot{})

	if len(items) != 1 {
		t.Fatalf("expected 1 connected infrastructure item, got %d", len(items))
	}

	item := items[0]
	if item.Name != "Canonical Tower" {
		t.Fatalf("expected canonical display name to drive name, got %q", item.Name)
	}
	if item.DisplayName != "Canonical Tower" {
		t.Fatalf("expected canonical display name to drive displayName, got %q", item.DisplayName)
	}
}

func TestBuildConnectedInfrastructure_UsesSharedTopLevelSystemGrouping(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	items := buildConnectedInfrastructure([]unifiedresources.Resource{
		{
			ID:       "tower-agent",
			Name:     "Tower Agent",
			LastSeen: now,
			Status:   unifiedresources.StatusOnline,
			Agent: &unifiedresources.AgentData{
				AgentID:      "tower-agent",
				AgentVersion: "1.2.3",
				Hostname:     "tower.local",
			},
			Identity: unifiedresources.ResourceIdentity{
				Hostnames: []string{"tower.local"},
			},
		},
		{
			ID:       "tower-pbs",
			Name:     "Tower PBS",
			LastSeen: now,
			Status:   unifiedresources.StatusOnline,
			PBS: &unifiedresources.PBSData{
				InstanceID: "pbs-1",
				Hostname:   "tower.local",
				HostURL:    "https://tower.local:8007",
				Version:    "3.4.1",
			},
		},
	}, models.StateSnapshot{})

	if len(items) != 1 {
		t.Fatalf("expected one connected infrastructure item after shared grouping, got %d", len(items))
	}

	item := items[0]
	if len(item.Surfaces) != 2 {
		t.Fatalf("expected host telemetry and PBS surfaces on one item, got %#v", item.Surfaces)
	}
	if item.ID == "resource:tower-pbs" {
		t.Fatalf("expected PBS surface to attach to the shared top-level system group, got %q", item.ID)
	}
}

func TestBuildConnectedInfrastructure_KeepsPlatformConnectionsActiveWhenHostTelemetryIgnored(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	items := buildConnectedInfrastructure([]unifiedresources.Resource{
		{
			ID:       "tower-agent",
			Name:     "Tower Agent",
			LastSeen: now,
			Status:   unifiedresources.StatusOnline,
			Agent: &unifiedresources.AgentData{
				AgentID:      "tower-agent",
				AgentVersion: "1.2.3",
				Hostname:     "tower.local",
			},
			Identity: unifiedresources.ResourceIdentity{
				Hostnames: []string{"tower.local"},
			},
		},
		{
			ID:       "tower-pbs",
			Name:     "Tower PBS",
			LastSeen: now,
			Status:   unifiedresources.StatusOnline,
			PBS: &unifiedresources.PBSData{
				InstanceID: "pbs-1",
				Hostname:   "tower.local",
				Version:    "3.4.1",
			},
		},
	}, models.StateSnapshot{
		RemovedHostAgents: []models.RemovedHostAgent{
			{ID: "tower-agent", Hostname: "tower.local", DisplayName: "Tower", RemovedAt: now},
		},
	})

	if len(items) != 2 {
		t.Fatalf("expected active platform item and ignored host item, got %d", len(items))
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

	if len(active.Surfaces) != 1 || active.Surfaces[0].Kind != "pbs" {
		t.Fatalf("expected active platform surface to remain after host ignore, got %#v", active.Surfaces)
	}
	if len(ignored.Surfaces) != 1 || ignored.Surfaces[0].Kind != "agent" {
		t.Fatalf("expected ignored agent surface, got %#v", ignored.Surfaces)
	}
}

func TestBuildConnectedInfrastructure_ProjectsTrueNASSurface(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	items := buildConnectedInfrastructure([]unifiedresources.Resource{
		{
			ID:       "truenas-main",
			Name:     "Tower NAS",
			LastSeen: now,
			Status:   unifiedresources.StatusOnline,
			TrueNAS: &unifiedresources.TrueNASData{
				Hostname: "truenas.local",
				Version:  "25.04.0",
			},
		},
	}, models.StateSnapshot{})

	if len(items) != 1 {
		t.Fatalf("expected one connected infrastructure item, got %d", len(items))
	}

	item := items[0]
	if item.Hostname != "truenas.local" {
		t.Fatalf("expected truenas hostname to drive connected infrastructure hostname, got %q", item.Hostname)
	}
	if item.Version != "25.04.0" {
		t.Fatalf("expected truenas version to drive connected infrastructure version, got %q", item.Version)
	}
	if len(item.Surfaces) != 1 {
		t.Fatalf("expected one truenas surface, got %#v", item.Surfaces)
	}
	if item.Surfaces[0].Kind != "truenas" {
		t.Fatalf("expected truenas surface kind, got %#v", item.Surfaces[0])
	}
}

func TestBuildConnectedInfrastructure_PreservesLinkedGuestIdentity(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	items := buildConnectedInfrastructure([]unifiedresources.Resource{
		{
			ID:       "guest-agent",
			Name:     "debian-go",
			LastSeen: now,
			Status:   unifiedresources.StatusOnline,
			Agent: &unifiedresources.AgentData{
				AgentID:           "guest-agent",
				AgentVersion:      "1.2.3",
				Hostname:          "debian-go.local",
				LinkedVMID:        "101",
				LinkedContainerID: "",
			},
			Identity: unifiedresources.ResourceIdentity{
				Hostnames: []string{"debian-go.local"},
			},
		},
	}, models.StateSnapshot{
		RemovedHostAgents: []models.RemovedHostAgent{
			{
				ID:                "ignored-guest-agent",
				Hostname:          "archive-guest.local",
				DisplayName:       "archive-guest",
				LinkedContainerID: "102",
				RemovedAt:         now,
			},
		},
	})

	if len(items) != 2 {
		t.Fatalf("expected active and ignored guest rows, got %d", len(items))
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

	if active == nil || ignored == nil {
		t.Fatalf("expected both active and ignored connected infrastructure items, got %#v", items)
	}
	if active.LinkedVMID != "101" {
		t.Fatalf("expected active linked VM id to round-trip, got %q", active.LinkedVMID)
	}
	if ignored.LinkedContainerID != "102" {
		t.Fatalf("expected ignored linked container id to round-trip, got %q", ignored.LinkedContainerID)
	}
}

func TestBuildConnectedInfrastructure_IgnoresChildPlatformResources(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	items := buildConnectedInfrastructure([]unifiedresources.Resource{
		{
			ID:       "vm-100",
			Type:     unifiedresources.ResourceTypeVM,
			Name:     "cloudflared",
			LastSeen: now,
			Status:   unifiedresources.StatusOnline,
			Proxmox: &unifiedresources.ProxmoxData{
				SourceID: "100",
				NodeName: "pve-a",
			},
		},
		{
			ID:       "storage-local-zfs",
			Type:     unifiedresources.ResourceTypeStorage,
			Name:     "local-zfs",
			LastSeen: now,
			Status:   unifiedresources.StatusOnline,
			Proxmox: &unifiedresources.ProxmoxData{
				SourceID: "local-zfs",
				NodeName: "pve-a",
			},
		},
		{
			ID:       "pbs-datastore-main",
			Type:     unifiedresources.ResourceTypeStorage,
			Name:     "main",
			LastSeen: now,
			Status:   unifiedresources.StatusOnline,
			PBS: &unifiedresources.PBSData{
				InstanceID: "pbs-main",
				Hostname:   "pbs.local",
			},
		},
	}, models.StateSnapshot{})

	if len(items) != 0 {
		t.Fatalf("expected child platform resources to stay out of connected infrastructure, got %#v", items)
	}
}

func TestBuildConnectedInfrastructure_ResolvesLegacyAgentUpgradePlatform(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	cases := []struct {
		name     string
		platform string
		want     string
	}{
		// Legacy v5 agents report gopsutil's host.Info().Platform verbatim
		// instead of a canonical token (refs #1555).
		{"legacy windows caption", "microsoft windows 11 pro", "windows"},
		{"canonical windows", "windows", "windows"},
		{"darwin", "darwin", "macos"},
		{"freebsd caption", "FreeBSD 14.1-RELEASE", "freebsd"},
		{"linux distro", "ubuntu", "linux"},
		{"empty", "", "linux"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			items := buildConnectedInfrastructure([]unifiedresources.Resource{
				{
					ID:       "host-resource",
					Name:     "Host",
					LastSeen: now,
					Status:   unifiedresources.StatusOnline,
					Agent: &unifiedresources.AgentData{
						AgentID:      "host-agent",
						AgentVersion: "5.1.36",
						Hostname:     "host.local",
						Platform:     tc.platform,
					},
					Identity: unifiedresources.ResourceIdentity{
						Hostnames: []string{"host.local"},
					},
				},
			}, models.StateSnapshot{})

			if len(items) != 1 {
				t.Fatalf("expected 1 connected infrastructure item, got %d", len(items))
			}
			if items[0].UpgradePlatform != tc.want {
				t.Fatalf("expected upgrade platform %q for reported platform %q, got %q", tc.want, tc.platform, items[0].UpgradePlatform)
			}
		})
	}
}
