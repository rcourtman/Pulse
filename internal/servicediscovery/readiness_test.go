package servicediscovery

import (
	"testing"
	"time"

	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestDiscoveryReadinessForTargetStates(t *testing.T) {
	now := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	target := &unified.DiscoveryTarget{
		ResourceType: "system-container",
		AgentID:      "node-a",
		ResourceID:   "101",
		Hostname:     "homeassistant",
	}

	tests := []struct {
		name      string
		discovery *ResourceDiscovery
		maxAge    time.Duration
		want      unified.ResourceDiscoveryReadinessState
	}{
		{
			name: "fresh",
			discovery: &ResourceDiscovery{
				ID:          MakeResourceID(ResourceTypeSystemContainer, "node-a", "101"),
				ServiceName: "Home Assistant",
				Category:    CategoryHomeAuto,
				Confidence:  0.92,
				UpdatedAt:   now.Add(-2 * time.Hour),
				Facts:       []DiscoveryFact{{Key: "service", Value: "homeassistant"}},
				Ports:       []PortInfo{{Port: 8123, Protocol: "tcp"}},
			},
			maxAge: 30 * 24 * time.Hour,
			want:   unified.ResourceDiscoveryReadinessFresh,
		},
		{
			name: "stale",
			discovery: &ResourceDiscovery{
				ID:        MakeResourceID(ResourceTypeSystemContainer, "node-a", "101"),
				UpdatedAt: now.Add(-31 * 24 * time.Hour),
			},
			maxAge: 30 * 24 * time.Hour,
			want:   unified.ResourceDiscoveryReadinessStale,
		},
		{
			name:      "missing",
			discovery: nil,
			maxAge:    30 * 24 * time.Hour,
			want:      unified.ResourceDiscoveryReadinessMissing,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DiscoveryReadinessForTarget(target, tt.discovery, nil, tt.maxAge, now)
			if got.State != tt.want {
				t.Fatalf("state = %q, want %q", got.State, tt.want)
			}
			if got.GeneratedAt.IsZero() {
				t.Fatal("GeneratedAt must be populated")
			}
			if got.ResourceType != "system-container" || got.TargetID != "node-a" || got.ResourceID != "101" {
				t.Fatalf("target fields = %+v", got)
			}
		})
	}
}

func TestDiscoveryReadinessForTargetCountsSafeDiscoveryMetadata(t *testing.T) {
	now := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	target := &unified.DiscoveryTarget{
		ResourceType: "app-container",
		AgentID:      "docker-host",
		ResourceID:   "homeassistant",
	}

	discovery := &ResourceDiscovery{
		ID:           MakeResourceID(ResourceTypeDocker, "docker-host", "homeassistant"),
		ResourceType: ResourceTypeDocker,
		ResourceID:   "homeassistant",
		TargetID:     "docker-host",
		ServiceName:  "Home Assistant",
		Category:     CategoryHomeAuto,
		Confidence:   0.88,
		UpdatedAt:    now.Add(-time.Hour),
		Facts:        []DiscoveryFact{{Key: "service", Value: "homeassistant"}},
		ConfigPaths:  []string{"/config/configuration.yaml"},
		LogPaths:     []string{"/config/home-assistant.log"},
		Ports:        []PortInfo{{Port: 8123, Protocol: "tcp"}},
		DockerMounts: []DockerBindMount{{ContainerName: "homeassistant"}},
	}

	got := DiscoveryReadinessForTarget(target, discovery, nil, 30*24*time.Hour, now)
	if got.State != unified.ResourceDiscoveryReadinessFresh {
		t.Fatalf("state = %q, want fresh", got.State)
	}
	if got.DiscoveryID != MakeResourceID(ResourceTypeDocker, "docker-host", "homeassistant") {
		t.Fatalf("discovery id = %q", got.DiscoveryID)
	}
	if got.FactCount != 5 {
		t.Fatalf("fact count = %d, want 5", got.FactCount)
	}
	if got.ServiceName != "Home Assistant" || got.ServiceCategory != string(CategoryHomeAuto) {
		t.Fatalf("service fields = %+v", got)
	}
}

func TestDiscoveryReadinessForTargetRunningAndUnsupported(t *testing.T) {
	now := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	startedAt := now.Add(-30 * time.Second)
	target := &unified.DiscoveryTarget{
		ResourceType: "pod",
		AgentID:      "cluster-a",
		ResourceID:   "default/homeassistant",
	}

	running := DiscoveryReadinessForTarget(target, nil, &DiscoveryProgress{
		ResourceID: MakeResourceID(ResourceTypeK8s, "cluster-a", "default/homeassistant"),
		Status:     DiscoveryStatusRunning,
		StartedAt:  startedAt,
	}, 30*24*time.Hour, now)
	if running.State != unified.ResourceDiscoveryReadinessRunning {
		t.Fatalf("running state = %q", running.State)
	}
	if running.ObservedAt == nil || !running.ObservedAt.Equal(startedAt) {
		t.Fatalf("running observedAt = %v, want %v", running.ObservedAt, startedAt)
	}

	unsupported := DiscoveryReadinessForTarget(&unified.DiscoveryTarget{
		ResourceType: "ceph",
		AgentID:      "cluster",
		ResourceID:   "fsid",
	}, nil, nil, 30*24*time.Hour, now)
	if unsupported.State != unified.ResourceDiscoveryReadinessUnsupported {
		t.Fatalf("unsupported state = %q", unsupported.State)
	}
}
