package mock

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestBuildFixtureStateIncludesDockerHosts(t *testing.T) {
	cfg := DefaultConfig
	cfg.DockerHostCount = 2
	cfg.DockerContainersPerHost = 5

	data := buildFixtureState(cfg)

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
		if len(host.Images) == 0 {
			t.Fatalf("docker host %s has no image inventory", host.Hostname)
		}
		if len(host.Volumes) == 0 {
			t.Fatalf("docker host %s has no volume inventory", host.Hostname)
		}
		if len(host.Networks) == 0 {
			t.Fatalf("docker host %s has no network inventory", host.Hostname)
		}
		if host.StorageUsage == nil || host.StorageUsage.Images.TotalCount == 0 || host.StorageUsage.Volumes.TotalCount == 0 {
			t.Fatalf("docker host %s has no engine storage usage inventory: %+v", host.Hostname, host.StorageUsage)
		}
	}
}

func TestComputeGuestCountsHandlesZeroBaselines(t *testing.T) {
	cfg := MockConfig{
		VMsPerNode:  0,
		LXCsPerNode: 0,
	}

	roles := []string{"vm-heavy", "container-heavy", "light", "mixed"}
	for _, role := range roles {
		vmCount, lxcCount := computeGuestCounts(cfg, role)
		if vmCount < 0 || lxcCount < 0 {
			t.Fatalf("expected non-negative counts for role %q, got vm=%d lxc=%d", role, vmCount, lxcCount)
		}
	}
}

func TestBuildFixtureStateWithZeroGuestBaselinesDoesNotPanic(t *testing.T) {
	cfg := DefaultConfig
	cfg.NodeCount = 4
	cfg.VMsPerNode = 0
	cfg.LXCsPerNode = 0
	cfg.DockerHostCount = 0
	cfg.GenericHostCount = 0
	cfg.K8sClusterCount = 0

	for i := 0; i < 20; i++ {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("buildFixtureState panicked on iteration %d: %v", i, r)
				}
			}()
			_ = buildFixtureState(cfg)
		}()
	}
}

func TestBuildFixtureStateIncludesSwarmServices(t *testing.T) {
	cfg := DefaultConfig
	cfg.DockerHostCount = 4
	cfg.DockerContainersPerHost = 6
	cfg.RandomMetrics = false

	data := buildFixtureState(cfg)

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
		if len(host.Secrets) == 0 {
			t.Fatalf("expected secrets for service host %s", host.ID)
		}
		if len(host.Configs) == 0 {
			t.Fatalf("expected configs for service host %s", host.ID)
		}
		if len(host.Nodes) == 0 {
			t.Fatalf("expected swarm nodes for service host %s", host.ID)
		}
		found = true
		break
	}

	if !found {
		t.Fatalf("expected at least one docker host with swarm services")
	}
}

func TestBuildFixtureStateIncludesHostAgents(t *testing.T) {
	cfg := DefaultConfig
	cfg.GenericHostCount = 5
	cfg.K8sClusterCount = 0
	cfg.RandomMetrics = false

	data := buildFixtureState(cfg)

	expectedMin := cfg.GenericHostCount + cfg.NodeCount
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
		if host.Memory.Total > 0 {
			if host.Memory.Cache <= 0 {
				t.Fatalf("host agent %s should report reclaimable cache so the memory split is exercisable: %+v", host.ID, host.Memory)
			}
			if sum := host.Memory.Used + host.Memory.Cache + host.Memory.Free; sum > host.Memory.Total {
				t.Fatalf("host agent %s memory used+cache+free %d exceeds total %d", host.ID, sum, host.Memory.Total)
			}
		}
	}
}

func TestBuildFixtureStatePreservesStandaloneHostAgentsWhenLinkingNodes(t *testing.T) {
	cfg := DefaultConfig
	cfg.NodeCount = 3
	cfg.GenericHostCount = 4
	cfg.K8sClusterCount = 0
	cfg.RandomMetrics = false

	data := buildFixtureState(cfg)

	standalone := 0
	proxmoxLinked := 0
	for _, host := range data.Hosts {
		if host.LinkedNodeID != "" {
			proxmoxLinked++
		} else {
			standalone++
		}
	}

	if standalone != cfg.GenericHostCount {
		t.Fatalf("expected %d standalone agent hosts, got %d", cfg.GenericHostCount, standalone)
	}
	if proxmoxLinked != cfg.NodeCount {
		t.Fatalf("expected %d Proxmox-linked agent hosts, got %d", cfg.NodeCount, proxmoxLinked)
	}
}

func TestGenerateVMStoppedPreservesConfiguredCapacity(t *testing.T) {
	cfg := DefaultConfig
	cfg.StoppedPercent = 1

	vm := generateVM("node-01", "mock-cluster", 1001, cfg)

	if vm.Status != "stopped" {
		t.Fatalf("expected stopped VM, got %q", vm.Status)
	}
	if vm.Memory.Total <= 0 {
		t.Fatalf("expected stopped VM to keep configured memory total, got %d", vm.Memory.Total)
	}
	if vm.Memory.Used != 0 || vm.Memory.Usage != 0 {
		t.Fatalf("expected stopped VM memory usage to be zero, got used=%d usage=%f", vm.Memory.Used, vm.Memory.Usage)
	}
	if vm.Memory.Free != vm.Memory.Total {
		t.Fatalf("expected stopped VM free memory %d to equal total %d", vm.Memory.Free, vm.Memory.Total)
	}
}

func TestGenerateContainerStoppedPreservesConfiguredCapacity(t *testing.T) {
	cfg := DefaultConfig
	cfg.StoppedPercent = 1

	ct := generateContainer("node-01", "mock-cluster", 2001, cfg)

	if ct.Status != "stopped" {
		t.Fatalf("expected stopped container, got %q", ct.Status)
	}
	if ct.Memory.Total <= 0 {
		t.Fatalf("expected stopped container to keep configured memory total, got %d", ct.Memory.Total)
	}
	if ct.Memory.Used != 0 || ct.Memory.Usage != 0 {
		t.Fatalf("expected stopped container memory usage to be zero, got used=%d usage=%f", ct.Memory.Used, ct.Memory.Usage)
	}
	if ct.Memory.Free != ct.Memory.Total {
		t.Fatalf("expected stopped container free memory %d to equal total %d", ct.Memory.Free, ct.Memory.Total)
	}
}

func TestNormalizeMockBlendWeight_ComposesAcrossUpdateInterval(t *testing.T) {
	perMinuteWeight := 0.22
	perTickWeight := normalizeMockBlendWeight(perMinuteWeight, defaultMockUpdateInterval, time.Minute)
	compounded := 1 - math.Pow(1-perTickWeight, float64(time.Minute/defaultMockUpdateInterval))

	if perTickWeight >= perMinuteWeight {
		t.Fatalf("expected per-tick weight %.6f to be less than per-minute weight %.6f", perTickWeight, perMinuteWeight)
	}
	if math.Abs(compounded-perMinuteWeight) > 0.01 {
		t.Fatalf("expected compounded weight %.6f to stay close to %.6f", compounded, perMinuteWeight)
	}
}

func TestBuildFixtureStateLinksAllNodesToHostAgents(t *testing.T) {
	cfg := DefaultConfig
	cfg.NodeCount = 7
	cfg.GenericHostCount = 2
	cfg.RandomMetrics = false

	data := buildFixtureState(cfg)

	if len(data.Hosts) < cfg.NodeCount {
		t.Fatalf("expected enough hosts to link all nodes, got hosts=%d nodes=%d", len(data.Hosts), cfg.NodeCount)
	}

	hostsByID := make(map[string]models.Host, len(data.Hosts))
	for _, host := range data.Hosts {
		hostsByID[host.ID] = host
	}

	for _, node := range data.Nodes {
		if node.LinkedAgentID == "" {
			t.Fatalf("node %s is missing linked host agent id", node.ID)
		}
		host, ok := hostsByID[node.LinkedAgentID]
		if !ok {
			t.Fatalf("node %s linked host %s not found", node.ID, node.LinkedAgentID)
		}
		if host.LinkedNodeID != node.ID {
			t.Fatalf("host %s linkedNodeID=%q, want %q", host.ID, host.LinkedNodeID, node.ID)
		}
	}
}

func TestBuildFixtureStatePopulatesHostIORates(t *testing.T) {
	cfg := DefaultConfig
	cfg.NodeCount = 6
	cfg.GenericHostCount = 1
	cfg.RandomMetrics = true

	data := buildFixtureState(cfg)

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

func TestBuildFixtureStatePopulatesDockerHostIORates(t *testing.T) {
	cfg := DefaultConfig
	cfg.DockerHostCount = 3
	cfg.RandomMetrics = true

	data := buildFixtureState(cfg)

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

func TestBuildFixtureStatePopulatesKubernetesUsageMetrics(t *testing.T) {
	cfg := DefaultConfig
	cfg.K8sClusterCount = 1
	cfg.K8sNodesPerCluster = 4
	cfg.K8sPodsPerCluster = 24
	cfg.RandomMetrics = false

	data := buildFixtureState(cfg)
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

func TestGenerateKubernetesNodesAlwaysRetainsSchedulableReadyNode(t *testing.T) {
	for count := 1; count <= 8; count++ {
		for iteration := 0; iteration < 100; iteration++ {
			nodes := generateKubernetesNodes("fixture-cluster", count)
			hasSchedulableReadyNode := false
			for _, node := range nodes {
				if node.Ready && !node.Unschedulable {
					hasSchedulableReadyNode = true
					break
				}
			}
			if !hasSchedulableReadyNode {
				t.Fatalf("node count %d iteration %d produced no schedulable ready node: %+v", count, iteration, nodes)
			}
		}
	}
}

func TestBuildFixtureStateIncludesKubernetesDeploymentAPIMetadata(t *testing.T) {
	cfg := DefaultConfig
	cfg.K8sClusterCount = 1
	cfg.K8sNodesPerCluster = 3
	cfg.K8sPodsPerCluster = 8
	cfg.K8sDeploymentsPerCluster = 6
	cfg.RandomMetrics = false

	data := buildFixtureState(cfg)
	if len(data.KubernetesClusters) != 1 {
		t.Fatalf("expected exactly one kubernetes cluster, got %d", len(data.KubernetesClusters))
	}

	cluster := data.KubernetesClusters[0]
	if len(cluster.Deployments) == 0 {
		t.Fatal("expected kubernetes deployment inventory")
	}
	for _, deployment := range cluster.Deployments {
		if deployment.UID == "" {
			t.Fatalf("deployment missing uid: %+v", deployment)
		}
		if deployment.CreatedAt.IsZero() {
			t.Fatalf("deployment missing creation metadata: %+v", deployment)
		}
		if deployment.ObservedGeneration == 0 {
			t.Fatalf("deployment missing observed generation: %+v", deployment)
		}
	}
}

func TestBuildFixtureStateCreatesHostEntriesForKubernetesNodes(t *testing.T) {
	cfg := DefaultConfig
	cfg.K8sClusterCount = 1
	cfg.K8sNodesPerCluster = 3
	cfg.K8sPodsPerCluster = 8
	cfg.NodeCount = 0
	cfg.RandomMetrics = false

	data := buildFixtureState(cfg)
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

func TestGenerateNodesBoundsDirectConfiguration(t *testing.T) {
	cfg := DefaultConfig
	cfg.NodeCount = 1 << 30
	if got := len(generateNodes(cfg)); got != maxMockNodeCount {
		t.Fatalf("generateNodes() count = %d, want max %d", got, maxMockNodeCount)
	}

	cfg.NodeCount = -1
	if got := len(generateNodes(cfg)); got != minMockNodeCount {
		t.Fatalf("generateNodes() count = %d, want min %d", got, minMockNodeCount)
	}
}

func TestBuildFixtureStateIncludesKubernetesStorageInventory(t *testing.T) {
	cfg := DefaultConfig
	cfg.K8sClusterCount = 1
	cfg.K8sNodesPerCluster = 3
	cfg.K8sPodsPerCluster = 8
	cfg.RandomMetrics = false

	data := buildFixtureState(cfg)
	if len(data.KubernetesClusters) != 1 {
		t.Fatalf("expected exactly one kubernetes cluster, got %d", len(data.KubernetesClusters))
	}

	cluster := data.KubernetesClusters[0]
	if len(cluster.StorageClasses) == 0 {
		t.Fatal("expected kubernetes storage class inventory")
	}
	if len(cluster.PersistentVolumes) == 0 {
		t.Fatal("expected kubernetes persistent volume inventory")
	}
	if len(cluster.PersistentVolumeClaims) == 0 {
		t.Fatal("expected kubernetes persistent volume claim inventory")
	}
}

func TestBuildFixtureStateIncludesKubernetesNetworkingInventory(t *testing.T) {
	cfg := DefaultConfig
	cfg.K8sClusterCount = 1
	cfg.K8sNodesPerCluster = 3
	cfg.K8sPodsPerCluster = 8
	cfg.RandomMetrics = false

	data := buildFixtureState(cfg)
	if len(data.KubernetesClusters) != 1 {
		t.Fatalf("expected exactly one kubernetes cluster, got %d", len(data.KubernetesClusters))
	}

	cluster := data.KubernetesClusters[0]
	if len(cluster.Services) == 0 {
		t.Fatal("expected kubernetes service inventory")
	}
	foundPublishedService := false
	for _, service := range cluster.Services {
		if service.ServiceType != "NodePort" || len(service.ExternalIPs) == 0 {
			continue
		}
		for _, port := range service.Ports {
			if port.NodePort > 0 {
				foundPublishedService = true
			}
		}
	}
	if !foundPublishedService {
		t.Fatal("expected kubernetes service inventory to include node-port/external service metadata")
	}
	if len(cluster.Ingresses) == 0 {
		t.Fatal("expected kubernetes ingress inventory")
	}
	if len(cluster.EndpointSlices) == 0 {
		t.Fatal("expected kubernetes endpoint slice inventory")
	}
}

func TestMockStateIncludesHostAgents(t *testing.T) {
	mustSetEnabled(t, true)
	t.Cleanup(func() {
		mustSetEnabled(t, false)
	})

	state := CurrentFixtureGraph().State
	if len(state.Hosts) == 0 {
		t.Fatalf("expected hosts in mock state, got %d", len(state.Hosts))
	}

	frontend := state.ToFrontend()
	if frontend.Resources == nil {
		t.Fatal("expected canonical frontend resources slice to be initialized")
	}
	if frontend.ConnectedInfrastructure == nil {
		t.Fatal("expected canonical connectedInfrastructure slice to be initialized")
	}
}

func TestUpdateMetricsMaintainsServiceHealth(t *testing.T) {
	cfg := DefaultConfig
	cfg.DockerHostCount = 3
	cfg.DockerContainersPerHost = 6

	data := buildFixtureState(cfg)
	updateFixtureStateMetrics(&data, cfg)

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

func TestBuildFixtureStateIncludesPMGInstances(t *testing.T) {
	cfg := DefaultConfig

	data := buildFixtureState(cfg)

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

func TestCloneStatePreservesIgnoredInfrastructureEntries(t *testing.T) {
	state := models.StateSnapshot{
		RemovedDockerHosts: []models.RemovedDockerHost{
			{ID: "docker-1", Hostname: "docker.local"},
		},
		RemovedHostAgents: []models.RemovedHostAgent{
			{ID: "agent-1", Hostname: "host.local"},
		},
		RemovedKubernetesClusters: []models.RemovedKubernetesCluster{
			{ID: "cluster-1", Name: "cluster.local"},
		},
	}

	cloned := cloneState(state)

	if len(cloned.RemovedDockerHosts) != 1 {
		t.Fatalf("expected cloned state to include removed docker hosts, got %d", len(cloned.RemovedDockerHosts))
	}
	if len(cloned.RemovedHostAgents) != 1 {
		t.Fatalf("expected cloned state to include removed host agents, got %d", len(cloned.RemovedHostAgents))
	}
	if len(cloned.RemovedKubernetesClusters) != 1 {
		t.Fatalf("expected cloned state to include removed kubernetes clusters, got %d", len(cloned.RemovedKubernetesClusters))
	}

	cloned.RemovedDockerHosts[0].Hostname = "mutated-docker.local"
	cloned.RemovedHostAgents[0].Hostname = "mutated-host.local"
	cloned.RemovedKubernetesClusters[0].Name = "mutated-cluster.local"

	if state.RemovedDockerHosts[0].Hostname == "mutated-docker.local" {
		t.Fatal("expected removed docker hosts to be copied")
	}
	if state.RemovedHostAgents[0].Hostname == "mutated-host.local" {
		t.Fatal("expected removed host agents to be copied")
	}
	if state.RemovedKubernetesClusters[0].Name == "mutated-cluster.local" {
		t.Fatal("expected removed kubernetes clusters to be copied")
	}
}

func TestBuildFixtureStateInitializesIgnoredInfrastructureSlices(t *testing.T) {
	data := buildFixtureState(DefaultConfig)

	if data.RemovedDockerHosts == nil {
		t.Fatal("expected buildFixtureState to initialize RemovedDockerHosts")
	}
	if data.RemovedHostAgents == nil {
		t.Fatal("expected buildFixtureState to initialize RemovedHostAgents")
	}
	if data.RemovedKubernetesClusters == nil {
		t.Fatal("expected buildFixtureState to initialize RemovedKubernetesClusters")
	}
}

// collectFixtureIdentities gathers every identity-bearing ID the fixture
// graph produces, keyed by category. Metric series resource IDs in the mock
// metrics store derive from these, so any per-boot churn here orphans stored
// history and breaks resource continuity across backend restarts.
func collectFixtureIdentities(graph FixtureGraph) map[string][]string {
	ids := map[string][]string{}
	add := func(category, id string) {
		ids[category] = append(ids[category], id)
	}

	state := graph.State
	for _, node := range state.Nodes {
		add("node", node.ID)
	}
	for _, vm := range state.VMs {
		add("vm", vm.ID)
	}
	for _, ct := range state.Containers {
		add("container", ct.ID)
	}
	for _, host := range state.Hosts {
		add("host", host.ID)
		add("host-hostname", host.Hostname)
	}
	for _, dockerHost := range state.DockerHosts {
		add("docker-host", dockerHost.ID)
		for _, c := range dockerHost.Containers {
			add("docker-container", c.ID)
		}
	}
	for _, cluster := range state.KubernetesClusters {
		add("k8s-cluster", cluster.ID)
		for _, node := range cluster.Nodes {
			add("k8s-node", node.UID)
		}
		for _, pod := range cluster.Pods {
			add("k8s-pod", pod.UID)
		}
		for _, deployment := range cluster.Deployments {
			add("k8s-deployment", deployment.UID)
		}
		for _, svc := range cluster.Services {
			add("k8s-service", svc.UID)
		}
	}
	for _, storage := range state.Storage {
		add("storage", storage.ID)
	}
	for _, ceph := range state.CephClusters {
		add("ceph-cluster", ceph.ID)
		add("ceph-fsid", ceph.FSID)
	}
	for _, disk := range state.PhysicalDisks {
		add("physical-disk", disk.ID)
	}

	for category := range ids {
		sort.Strings(ids[category])
	}
	return ids
}

// TestFixtureIdentityStableAcrossBoots simulates two backend boots (the
// package-global rand source is in a different state for each build, exactly
// as two freshly seeded processes would be) and requires every identity set
// to match. Guards the generator's "stable IDs" contract end to end.
func TestFixtureIdentityStableAcrossBoots(t *testing.T) {
	now := time.Date(2026, time.June, 10, 12, 0, 0, 0, time.UTC)

	first := collectFixtureIdentities(buildFixtureGraph(DefaultConfig, now))
	second := collectFixtureIdentities(buildFixtureGraph(DefaultConfig, now))

	categories := map[string]struct{}{}
	for category := range first {
		categories[category] = struct{}{}
	}
	for category := range second {
		categories[category] = struct{}{}
	}

	for category := range categories {
		diff := diffIdentitySets(first[category], second[category])
		if diff != "" {
			t.Errorf("%s identities churned across boots:\n%s", category, diff)
		}
	}
}

func diffIdentitySets(first, second []string) string {
	firstSet := make(map[string]struct{}, len(first))
	for _, id := range first {
		firstSet[id] = struct{}{}
	}
	secondSet := make(map[string]struct{}, len(second))
	for _, id := range second {
		secondSet[id] = struct{}{}
	}

	var lines []string
	for _, id := range first {
		if _, ok := secondSet[id]; !ok {
			lines = append(lines, fmt.Sprintf("  only in first boot:  %s", id))
		}
	}
	for _, id := range second {
		if _, ok := firstSet[id]; !ok {
			lines = append(lines, fmt.Sprintf("  only in second boot: %s", id))
		}
	}

	if len(lines) == 0 {
		return ""
	}
	sort.Strings(lines)
	out := ""
	for _, line := range lines {
		out += line + "\n"
	}
	return out
}
