package mock

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestGenerateMockDataIncludesDockerHosts(t *testing.T) {
	cfg := DefaultConfig
	cfg.DockerHostCount = 2
	cfg.DockerContainersPerHost = 5

	data := GenerateMockData(cfg)

	if len(data.DockerHosts) != cfg.DockerHostCount {
		t.Fatalf("expected %d docker hosts, got %d", cfg.DockerHostCount, len(data.DockerHosts))
	}

	for _, host := range data.DockerHosts {
		if host.ID == "" {
			t.Fatalf("docker host missing id: %+v", host)
		}
		if len(host.Containers) == 0 {
			t.Fatalf("docker host %s has no containers", host.Hostname)
		}
	}
}

func TestGenerateMockDataIncludesSwarmServices(t *testing.T) {
	cfg := DefaultConfig
	cfg.DockerHostCount = 4
	cfg.DockerContainersPerHost = 6
	cfg.RandomMetrics = false

	data := GenerateMockData(cfg)

	found := false
	for _, host := range data.DockerHosts {
		if len(host.Services) == 0 {
			continue
		}
		if host.Swarm == nil {
			t.Fatalf("expected swarm metadata for host %s", host.ID)
		}
		if len(host.Tasks) == 0 {
			t.Fatalf("expected tasks for service host %s", host.ID)
		}
		found = true
		break
	}

	if !found {
		t.Fatalf("expected at least one docker host with swarm services")
	}
}

func TestGenerateMockDataIncludesHostAgents(t *testing.T) {
	cfg := DefaultConfig
	cfg.GenericHostCount = 5
	cfg.RandomMetrics = false

	data := GenerateMockData(cfg)

	if len(data.Hosts) != cfg.GenericHostCount {
		t.Fatalf("expected %d host agents, got %d", cfg.GenericHostCount, len(data.Hosts))
	}

	for _, host := range data.Hosts {
		if host.ID == "" {
			t.Fatalf("host agent missing id: %+v", host)
		}
		if host.Hostname == "" {
			t.Fatalf("host agent missing hostname: %+v", host)
		}
		if host.Status == "" {
			t.Fatalf("host agent missing status: %+v", host)
		}
	}
}

func TestMockStateIncludesHostAgents(t *testing.T) {
	SetEnabled(true)
	t.Cleanup(func() {
		SetEnabled(false)
	})

	state := GetMockState()
	if len(state.Hosts) == 0 {
		t.Fatalf("expected hosts in mock state, got %d", len(state.Hosts))
	}

	frontend := state.ToFrontend()
	if len(frontend.Hosts) == 0 {
		t.Fatalf("expected hosts in frontend state, got %d", len(frontend.Hosts))
	}
}

func TestUpdateMetricsMaintainsServiceHealth(t *testing.T) {
	cfg := DefaultConfig
	cfg.DockerHostCount = 3
	cfg.DockerContainersPerHost = 6

	data := GenerateMockData(cfg)
	UpdateMetrics(&data, cfg)

	for _, host := range data.DockerHosts {
		if len(host.Services) == 0 {
			continue
		}
		if host.Swarm == nil {
			t.Fatalf("expected swarm metadata for host %s after update", host.ID)
		}

		for _, svc := range host.Services {
			if svc.DesiredTasks < 0 {
				t.Fatalf("service %s has negative desired tasks", svc.Name)
			}
			if svc.RunningTasks < 0 {
				t.Fatalf("service %s has negative running tasks", svc.Name)
			}
			if svc.RunningTasks > svc.DesiredTasks && svc.DesiredTasks > 0 {
				t.Fatalf("service %s has running (%d) > desired (%d)", svc.Name, svc.RunningTasks, svc.DesiredTasks)
			}
		}
	}
}

func TestGenerateMockDataIncludesPMGInstances(t *testing.T) {
	cfg := DefaultConfig

	data := GenerateMockData(cfg)

	if len(data.PMGInstances) == 0 {
		t.Fatalf("expected PMG instances in mock data")
	}

	for _, inst := range data.PMGInstances {
		if inst.Name == "" {
			t.Fatalf("PMG instance missing name: %+v", inst)
		}
		if inst.Status == "" {
			t.Fatalf("PMG instance missing status: %+v", inst)
		}
	}
}

func TestCloneStateCopiesPMGInstances(t *testing.T) {
	state := models.StateSnapshot{
		PMGInstances: []models.PMGInstance{
			{ID: "pmg-test", Name: "pmg-test", Status: "online"},
		},
	}

	cloned := cloneState(state)

	if len(cloned.PMGInstances) != 1 {
		t.Fatalf("expected cloned state to include PMG instances, got %d", len(cloned.PMGInstances))
	}

	cloned.PMGInstances[0].Name = "modified"
	if state.PMGInstances[0].Name == "modified" {
		t.Fatal("expected PMG instances to be deep-copied")
	}
}
