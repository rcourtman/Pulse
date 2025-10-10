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
