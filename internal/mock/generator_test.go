package mock

import (
	"strings"
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

	expectedMin := cfg.GenericHostCount
	if cfg.NodeCount > expectedMin {
		expectedMin = cfg.NodeCount
	}
	if len(data.Hosts) < expectedMin {
		t.Fatalf("expected at least %d host agents, got %d", expectedMin, len(data.Hosts))
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

func TestGenerateMockDataLinksAllNodesToHostAgents(t *testing.T) {
	cfg := DefaultConfig
	cfg.NodeCount = 7
	cfg.GenericHostCount = 2
	cfg.RandomMetrics = false

	data := GenerateMockData(cfg)

	if len(data.Hosts) < cfg.NodeCount {
		t.Fatalf("expected enough hosts to link all nodes, got hosts=%d nodes=%d", len(data.Hosts), cfg.NodeCount)
	}

	hostsByID := make(map[string]models.Host, len(data.Hosts))
	for _, host := range data.Hosts {
		hostsByID[host.ID] = host
	}

	for _, node := range data.Nodes {
		if node.LinkedHostAgentID == "" {
			t.Fatalf("node %s is missing linked host agent id", node.ID)
		}
		host, ok := hostsByID[node.LinkedHostAgentID]
		if !ok {
			t.Fatalf("node %s linked host %s not found", node.ID, node.LinkedHostAgentID)
		}
		if host.LinkedNodeID != node.ID {
			t.Fatalf("host %s linkedNodeID=%q, want %q", host.ID, host.LinkedNodeID, node.ID)
		}
	}
}

func TestGenerateMockDataPopulatesHostIORates(t *testing.T) {
	cfg := DefaultConfig
	cfg.NodeCount = 6
	cfg.GenericHostCount = 1
	cfg.RandomMetrics = true

	data := GenerateMockData(cfg)

	for _, host := range data.Hosts {
		if host.Status == "offline" {
			if host.NetInRate != 0 || host.NetOutRate != 0 || host.DiskReadRate != 0 || host.DiskWriteRate != 0 {
				t.Fatalf("offline host %s should have zero I/O rates", host.ID)
			}
			continue
		}
		if host.NetInRate <= 0 || host.NetOutRate <= 0 {
			t.Fatalf("host %s missing network rates: in=%f out=%f", host.ID, host.NetInRate, host.NetOutRate)
		}
		if host.DiskReadRate <= 0 || host.DiskWriteRate <= 0 {
			t.Fatalf("host %s missing disk rates: read=%f write=%f", host.ID, host.DiskReadRate, host.DiskWriteRate)
		}
	}
}

func TestGenerateMockDataPopulatesDockerHostIORates(t *testing.T) {
	cfg := DefaultConfig
	cfg.DockerHostCount = 3
	cfg.RandomMetrics = true

	data := GenerateMockData(cfg)

	if len(data.DockerHosts) == 0 {
		t.Fatal("expected docker hosts in mock data")
	}

	for _, host := range data.DockerHosts {
		if host.Status == "offline" {
			if host.NetInRate != 0 || host.NetOutRate != 0 || host.DiskReadRate != 0 || host.DiskWriteRate != 0 {
				t.Fatalf("offline docker host %s should have zero I/O rates", host.ID)
			}
			if host.Temperature != nil {
				t.Fatalf("offline docker host %s should not report temperature", host.ID)
			}
			continue
		}
		if host.NetInRate <= 0 || host.NetOutRate <= 0 {
			t.Fatalf("docker host %s missing network rates: in=%f out=%f", host.ID, host.NetInRate, host.NetOutRate)
		}
		if host.DiskReadRate <= 0 || host.DiskWriteRate <= 0 {
			t.Fatalf("docker host %s missing disk rates: read=%f write=%f", host.ID, host.DiskReadRate, host.DiskWriteRate)
		}
		if host.Temperature == nil {
			t.Fatalf("docker host %s missing temperature", host.ID)
		}
	}
}

func TestGenerateMockDataPopulatesKubernetesUsageMetrics(t *testing.T) {
	cfg := DefaultConfig
	cfg.K8sClusterCount = 1
	cfg.K8sNodesPerCluster = 4
	cfg.K8sPodsPerCluster = 24
	cfg.RandomMetrics = false

	data := GenerateMockData(cfg)
	if len(data.KubernetesClusters) != 1 {
		t.Fatalf("expected exactly one kubernetes cluster, got %d", len(data.KubernetesClusters))
	}

	cluster := data.KubernetesClusters[0]
	readyNodeNames := make(map[string]struct{}, len(cluster.Nodes))
	for _, node := range cluster.Nodes {
		if node.Ready {
			readyNodeNames[strings.TrimSpace(node.Name)] = struct{}{}
		}
	}

	runningPodsWithUsage := 0
	for _, pod := range cluster.Pods {
		if !strings.EqualFold(pod.Phase, "running") {
			continue
		}
		if _, ok := readyNodeNames[strings.TrimSpace(pod.NodeName)]; !ok {
			continue
		}
		if pod.UsageCPUMilliCores <= 0 || pod.UsageMemoryBytes <= 0 {
			t.Fatalf("running pod %s missing cpu/memory usage: %+v", pod.Name, pod)
		}
		if pod.NetInRate <= 0 || pod.NetOutRate <= 0 {
			t.Fatalf("running pod %s missing network rates: in=%f out=%f", pod.Name, pod.NetInRate, pod.NetOutRate)
		}
		if pod.EphemeralStorageCapacityBytes <= 0 || pod.EphemeralStorageUsedBytes <= 0 {
			t.Fatalf("running pod %s missing ephemeral storage usage: %+v", pod.Name, pod)
		}
		if pod.DiskUsagePercent <= 0 {
			t.Fatalf("running pod %s missing disk usage percent: %+v", pod.Name, pod)
		}
		runningPodsWithUsage++
	}
	if runningPodsWithUsage == 0 {
		t.Fatal("expected at least one running kubernetes pod with usage metrics")
	}

	readyNodesWithUsage := 0
	for _, node := range cluster.Nodes {
		if !node.Ready {
			continue
		}
		if node.UsageCPUMilliCores <= 0 || node.UsageMemoryBytes <= 0 {
			t.Fatalf("ready node %s missing usage metrics: %+v", node.Name, node)
		}
		readyNodesWithUsage++
	}
	if readyNodesWithUsage == 0 {
		t.Fatal("expected at least one ready kubernetes node with usage metrics")
	}
}

func TestGenerateMockDataCreatesHostEntriesForKubernetesNodes(t *testing.T) {
	cfg := DefaultConfig
	cfg.K8sClusterCount = 1
	cfg.K8sNodesPerCluster = 3
	cfg.K8sPodsPerCluster = 8
	cfg.NodeCount = 0
	cfg.RandomMetrics = false

	data := GenerateMockData(cfg)
	if len(data.KubernetesClusters) == 0 {
		t.Fatal("expected kubernetes clusters in mock data")
	}

	hostsByName := make(map[string]models.Host, len(data.Hosts))
	for _, host := range data.Hosts {
		hostsByName[strings.ToLower(strings.TrimSpace(host.Hostname))] = host
	}

	for _, node := range data.KubernetesClusters[0].Nodes {
		host, ok := hostsByName[strings.ToLower(strings.TrimSpace(node.Name))]
		if !ok {
			t.Fatalf("expected host entry for kubernetes node %s", node.Name)
		}
		if host.NetInRate <= 0 || host.NetOutRate <= 0 {
			t.Fatalf("kubernetes node host %s missing network rates: in=%f out=%f", host.Hostname, host.NetInRate, host.NetOutRate)
		}
		if host.DiskReadRate <= 0 || host.DiskWriteRate <= 0 {
			t.Fatalf("kubernetes node host %s missing disk rates: read=%f write=%f", host.Hostname, host.DiskReadRate, host.DiskWriteRate)
		}
		if len(host.Sensors.TemperatureCelsius) == 0 {
			t.Fatalf("kubernetes node host %s missing temperature sensors", host.Hostname)
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
