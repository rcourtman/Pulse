package mock

import (
	"testing"
	"time"
)

func TestBuildFixtureGraphIncludesDiscoveryFixtures(t *testing.T) {
	cfg := DefaultConfig
	cfg.DockerHostCount = 3
	cfg.DockerContainersPerHost = 6
	cfg.VMsPerNode = 3
	cfg.LXCsPerNode = 3

	graph := buildFixtureGraph(cfg, time.Date(2026, time.April, 10, 12, 0, 0, 0, time.UTC))

	if len(graph.DiscoveryFixtures) == 0 {
		t.Fatal("expected discovery fixtures")
	}

	var dockerFixture *DiscoveryFixture
	var vmFixture *DiscoveryFixture
	var agentFixture *DiscoveryFixture
	for _, discovery := range graph.DiscoveryFixtures {
		if discovery == nil {
			t.Fatal("discovery fixture must not be nil")
		}
		if discovery.ID == "" || discovery.TargetID == "" || discovery.ResourceID == "" {
			t.Fatalf("discovery fixture missing identity: %+v", discovery)
		}
		if discovery.ServiceName == "" || discovery.ServiceVersion == "" {
			t.Fatalf("discovery fixture missing service details: %+v", discovery)
		}
		if discovery.CLIAccessVersion != mockCLIAccessVersion {
			t.Fatalf("discovery fixture CLI version = %d, want %d", discovery.CLIAccessVersion, mockCLIAccessVersion)
		}
		switch discovery.ResourceType {
		case discoveryResourceTypeDocker:
			if dockerFixture == nil {
				dockerFixture = discovery
			}
		case discoveryResourceTypeVM:
			if vmFixture == nil {
				vmFixture = discovery
			}
		case discoveryResourceTypeAgent:
			if agentFixture == nil {
				agentFixture = discovery
			}
		}
	}

	if dockerFixture == nil {
		t.Fatal("expected at least one Docker discovery fixture")
	}
	if dockerFixture.SuggestedURL == "" {
		t.Fatalf("expected Docker discovery fixture to include suggested URL: %+v", dockerFixture)
	}
	if len(dockerFixture.DockerMounts) == 0 {
		t.Fatalf("expected Docker discovery fixture to include bind mount context: %+v", dockerFixture)
	}
	if vmFixture == nil {
		t.Fatal("expected at least one VM discovery fixture")
	}
	if agentFixture == nil {
		t.Fatal("expected at least one agent discovery fixture")
	}
}

func TestCurrentDiscoveryFixtureLookupReturnsDefensiveCopies(t *testing.T) {
	previous := IsMockEnabled()
	if err := SetEnabled(true); err != nil {
		t.Fatalf("enable mock mode: %v", err)
	}
	t.Cleanup(func() {
		if err := SetEnabled(previous); err != nil {
			t.Fatalf("restore mock mode: %v", err)
		}
	})

	fixtures := CurrentDiscoveryFixtures()
	if len(fixtures) == 0 {
		t.Fatal("expected current discovery fixtures")
	}

	first := fixtures[0]
	first.ServiceName = "mutated"
	if len(first.ConfigPaths) > 0 {
		first.ConfigPaths[0] = "/mutated"
	}

	again := CurrentDiscoveryFixtureByResource(first.ResourceType, first.TargetID, first.ResourceID)
	if again == nil {
		t.Fatalf("expected fixture lookup for %s", first.ID)
	}
	if again.ServiceName == "mutated" {
		t.Fatal("fixture lookup returned shared pointer")
	}
	if len(again.ConfigPaths) > 0 && again.ConfigPaths[0] == "/mutated" {
		t.Fatal("fixture lookup returned shared slice")
	}
}
