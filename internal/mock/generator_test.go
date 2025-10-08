package mock

import "testing"

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
