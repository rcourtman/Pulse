package mock

import (
	"strings"
	"testing"
	"time"
)

func TestBuildFixtureGraphAppliesCuratedDemoScenarioAcrossEstate(t *testing.T) {
	cfg := DefaultConfig
	cfg.RandomMetrics = false

	now := time.Date(2026, time.April, 1, 12, 0, 0, 0, time.UTC)
	graph := buildFixtureGraph(cfg, now)

	for _, want := range []string{"checkout-web-01", "orders-api-01"} {
		if !vmNameExists(graph, want) {
			t.Fatalf("expected curated vm %q in canonical demo graph", want)
		}
	}
	for _, want := range []string{"auth-service-01", "backup-orchestrator-01"} {
		if !containerNameExists(graph, want) {
			t.Fatalf("expected curated system container %q in canonical demo graph", want)
		}
	}
	for _, want := range []string{"edge-apps-01", "ops-services-01"} {
		if !dockerHostNameExists(graph, want) {
			t.Fatalf("expected curated docker host %q in canonical demo graph", want)
		}
	}
	for _, want := range []string{"customer-portal", "backup-coordinator"} {
		if !dockerContainerNameExists(graph, want) {
			t.Fatalf("expected curated app container %q in canonical demo graph", want)
		}
	}
	for _, want := range []string{"client-files", "edge-dns"} {
		if !trueNASAppNameExists(graph, want) {
			t.Fatalf("expected curated TrueNAS app %q in canonical demo graph", want)
		}
	}
	for _, want := range []string{"warehouse-api-01", "etl-batch-01"} {
		if !vmwareVMNameExists(graph, want) {
			t.Fatalf("expected curated VMware VM %q in canonical demo graph", want)
		}
	}
	if !nodeDisplayNameExists(graph, "West Production A") {
		t.Fatal("expected curated infrastructure node display name in canonical demo graph")
	}
	if !kubernetesNodeExists(graph, "prod-euw1-k8s-01") {
		t.Fatal("expected curated kubernetes node in canonical demo graph")
	}
	for _, want := range []string{"checkout-web", "payments-worker"} {
		if !kubernetesDeploymentExists(graph, want) {
			t.Fatalf("expected curated kubernetes deployment %q in canonical demo graph", want)
		}
	}
	for _, want := range []string{"shared-backup-fabric", "west-a-service-pool"} {
		if !storageNameExists(graph, want) {
			t.Fatalf("expected curated storage name %q in canonical demo graph", want)
		}
	}
	if storageNameExists(graph, "service-pool") {
		t.Fatal("expected demo storage naming to remove generic service-pool alias")
	}
	if !pbsInstanceExists(graph, "backup-vault") {
		t.Fatal("expected curated PBS instance in canonical demo graph")
	}
	if !allPBSDatastoresOnline(graph) {
		t.Fatal("expected curated PBS datastores to normalize onto online status")
	}
	if !pmgInstanceExists(graph, "mail-gateway-eu") {
		t.Fatal("expected curated PMG instance in canonical demo graph")
	}
	if leak := firstLegacyMockClusterLeak(graph); leak != "" {
		t.Fatalf("expected demo scenario to replace legacy mock-cluster labels, found %s", leak)
	}
}

func TestFixtureGraphUpdateMetricsPreservesCuratedDemoScenario(t *testing.T) {
	cfg := DefaultConfig
	cfg.RandomMetrics = false

	start := time.Date(2026, time.April, 1, 12, 0, 0, 0, time.UTC)
	later := start.Add(45 * time.Minute)

	graph := buildFixtureGraph(cfg, start)
	graph.UpdateMetrics(cfg, later)

	if !vmNameExists(graph, "checkout-web-01") {
		t.Fatal("expected curated vm naming to survive metric refresh")
	}
	if !dockerContainerNameExists(graph, "customer-portal") {
		t.Fatal("expected curated docker container naming to survive metric refresh")
	}
	if !trueNASAppNameExists(graph, "client-files") {
		t.Fatal("expected curated TrueNAS app naming to survive metric refresh")
	}
	if !vmwareVMNameExists(graph, "warehouse-api-01") {
		t.Fatal("expected curated VMware naming to survive metric refresh")
	}
	if !kubernetesNodeExists(graph, "prod-euw1-k8s-01") {
		t.Fatal("expected curated kubernetes node naming to survive metric refresh")
	}
	if !storageNameExists(graph, "west-a-service-pool") {
		t.Fatal("expected curated storage naming to survive metric refresh")
	}
	if !allPBSDatastoresOnline(graph) {
		t.Fatal("expected curated PBS datastore status to survive metric refresh")
	}
	if leak := firstLegacyMockClusterLeak(graph); leak != "" {
		t.Fatalf("expected metric refresh to preserve cluster aliasing, found %s", leak)
	}
}

func TestFixtureGraphUpdateMetricsRestoresStableDemoInfrastructurePosture(t *testing.T) {
	cfg := DefaultConfig
	cfg.RandomMetrics = true

	start := time.Date(2026, time.April, 1, 12, 0, 0, 0, time.UTC)
	later := start.Add(45 * time.Minute)

	graph := buildFixtureGraph(cfg, start)

	for i := range graph.State.Hosts {
		graph.State.Hosts[i].Status = "offline"
		graph.State.Hosts[i].LastSeen = time.Time{}
	}
	for i := range graph.State.DockerHosts {
		graph.State.DockerHosts[i].Status = "offline"
	}
	for i := range graph.State.KubernetesClusters {
		graph.State.KubernetesClusters[i].Status = "offline"
		for j := range graph.State.KubernetesClusters[i].Nodes {
			graph.State.KubernetesClusters[i].Nodes[j].Ready = false
			graph.State.KubernetesClusters[i].Nodes[j].Unschedulable = true
		}
	}
	for i := range graph.State.PBSInstances {
		graph.State.PBSInstances[i].Status = "offline"
		graph.State.PBSInstances[i].ConnectionHealth = "error"
		graph.State.PBSInstances[i].LastSeen = time.Time{}
	}
	for i := range graph.State.PMGInstances {
		graph.State.PMGInstances[i].Status = "offline"
		graph.State.PMGInstances[i].ConnectionHealth = "error"
		graph.State.PMGInstances[i].LastSeen = time.Time{}
	}
	graph.State.ConnectionHealth = map[string]bool{
		"docker-stale":     false,
		"kubernetes-stale": false,
		"host-stale":       false,
		"pbs-stale":        false,
		"pmg-stale":        false,
		"pve-stale":        false,
	}

	graph.UpdateMetrics(cfg, later)

	if !allHostsOnline(graph) {
		t.Fatal("expected demo host agents to restore online status after metric refresh")
	}
	if !allDockerHostsOnline(graph) {
		t.Fatal("expected curated docker hosts to restore online status after metric refresh")
	}
	if !allKubernetesClustersOnline(graph) {
		t.Fatal("expected curated kubernetes clusters to restore online status after metric refresh")
	}
	if !allKubernetesNodesReady(graph) {
		t.Fatal("expected curated kubernetes nodes to restore ready status after metric refresh")
	}
	if !allPBSInstancesOnlineHealthy(graph) {
		t.Fatal("expected curated PBS posture to restore healthy online status after metric refresh")
	}
	if !allPMGInstancesOnlineHealthy(graph) {
		t.Fatal("expected curated PMG posture to restore healthy online status after metric refresh")
	}
	if !connectionHealthMatchesCuratedDemoState(graph) {
		t.Fatal("expected connection-health map to match the final curated demo state after metric refresh")
	}
	if _, ok := graph.State.ConnectionHealth["docker-stale"]; ok {
		t.Fatal("expected stale docker connection-health keys to be cleared")
	}
	if _, ok := graph.State.ConnectionHealth["kubernetes-stale"]; ok {
		t.Fatal("expected stale kubernetes connection-health keys to be cleared")
	}
	if _, ok := graph.State.ConnectionHealth["host-stale"]; ok {
		t.Fatal("expected stale host-agent connection-health keys to be cleared")
	}
	if _, ok := graph.State.ConnectionHealth["pbs-stale"]; ok {
		t.Fatal("expected stale PBS connection-health keys to be cleared")
	}
	if _, ok := graph.State.ConnectionHealth["pmg-stale"]; ok {
		t.Fatal("expected stale PMG connection-health keys to be cleared")
	}
	if _, ok := graph.State.ConnectionHealth["pve-stale"]; ok {
		t.Fatal("expected stale PVE connection-health keys to be cleared")
	}
}

func vmNameExists(graph FixtureGraph, want string) bool {
	for _, vm := range graph.State.VMs {
		if vm.Name == want {
			return true
		}
	}
	return false
}

func containerNameExists(graph FixtureGraph, want string) bool {
	for _, container := range graph.State.Containers {
		if container.Name == want {
			return true
		}
	}
	return false
}

func dockerHostNameExists(graph FixtureGraph, want string) bool {
	for _, host := range graph.State.DockerHosts {
		if host.Hostname == want || host.DisplayName == want {
			return true
		}
	}
	return false
}

func dockerContainerNameExists(graph FixtureGraph, want string) bool {
	for _, host := range graph.State.DockerHosts {
		for _, container := range host.Containers {
			if container.Name == want {
				return true
			}
		}
	}
	return false
}

func trueNASAppNameExists(graph FixtureGraph, want string) bool {
	for _, app := range graph.PlatformFixtures.TrueNAS.Apps {
		if app.Name == want {
			return true
		}
	}
	return false
}

func vmwareVMNameExists(graph FixtureGraph, want string) bool {
	for _, vm := range graph.PlatformFixtures.VMware.VMs {
		if vm.Name == want {
			return true
		}
	}
	return false
}

func nodeDisplayNameExists(graph FixtureGraph, want string) bool {
	for _, node := range graph.State.Nodes {
		if node.DisplayName == want {
			return true
		}
	}
	return false
}

func kubernetesNodeExists(graph FixtureGraph, want string) bool {
	for _, cluster := range graph.State.KubernetesClusters {
		for _, node := range cluster.Nodes {
			if node.Name == want {
				return true
			}
		}
	}
	return false
}

func kubernetesDeploymentExists(graph FixtureGraph, want string) bool {
	for _, cluster := range graph.State.KubernetesClusters {
		for _, deployment := range cluster.Deployments {
			if deployment.Name == want {
				return true
			}
		}
	}
	return false
}

func storageNameExists(graph FixtureGraph, want string) bool {
	for _, storage := range graph.State.Storage {
		if storage.Name == want {
			return true
		}
	}
	return false
}

func pbsInstanceExists(graph FixtureGraph, want string) bool {
	for _, instance := range graph.State.PBSInstances {
		if instance.Name == want {
			return true
		}
	}
	return false
}

func allPBSDatastoresOnline(graph FixtureGraph) bool {
	for _, instance := range graph.State.PBSInstances {
		for _, datastore := range instance.Datastores {
			if datastore.Status != "online" {
				return false
			}
		}
	}
	return true
}

func pmgInstanceExists(graph FixtureGraph, want string) bool {
	for _, instance := range graph.State.PMGInstances {
		if instance.Name == want {
			return true
		}
	}
	return false
}

func allHostsOnline(graph FixtureGraph) bool {
	for _, host := range graph.State.Hosts {
		if host.Status != "online" {
			return false
		}
	}
	return true
}

func allDockerHostsOnline(graph FixtureGraph) bool {
	for _, host := range graph.State.DockerHosts {
		if host.Status != "online" {
			return false
		}
	}
	return true
}

func allKubernetesClustersOnline(graph FixtureGraph) bool {
	for _, cluster := range graph.State.KubernetesClusters {
		if cluster.Status != "online" {
			return false
		}
	}
	return true
}

func allKubernetesNodesReady(graph FixtureGraph) bool {
	for _, cluster := range graph.State.KubernetesClusters {
		for _, node := range cluster.Nodes {
			if !node.Ready || node.Unschedulable {
				return false
			}
		}
	}
	return true
}

func allPBSInstancesOnlineHealthy(graph FixtureGraph) bool {
	for _, instance := range graph.State.PBSInstances {
		if instance.Status != "online" || instance.ConnectionHealth != "healthy" {
			return false
		}
	}
	return true
}

func allPMGInstancesOnlineHealthy(graph FixtureGraph) bool {
	for _, instance := range graph.State.PMGInstances {
		if instance.Status != "online" || instance.ConnectionHealth != "healthy" {
			return false
		}
	}
	return true
}

func connectionHealthMatchesCuratedDemoState(graph FixtureGraph) bool {
	for _, node := range graph.State.Nodes {
		key := "pve-" + strings.TrimSpace(node.Name)
		expected := !strings.EqualFold(strings.TrimSpace(node.Status), "offline")
		healthy, ok := graph.State.ConnectionHealth[key]
		if key == "pve-" || !ok || healthy != expected {
			return false
		}
	}
	for _, host := range graph.State.DockerHosts {
		key := "docker-" + strings.TrimSpace(host.ID)
		expected := !strings.EqualFold(strings.TrimSpace(host.Status), "offline")
		healthy, ok := graph.State.ConnectionHealth[key]
		if key == "docker-" || !ok || healthy != expected {
			return false
		}
	}
	for _, cluster := range graph.State.KubernetesClusters {
		key := "kubernetes-" + strings.TrimSpace(cluster.ID)
		expected := !strings.EqualFold(strings.TrimSpace(cluster.Status), "offline")
		healthy, ok := graph.State.ConnectionHealth[key]
		if key == "kubernetes-" || !ok || healthy != expected {
			return false
		}
	}
	for _, host := range graph.State.Hosts {
		key := "host-" + strings.TrimSpace(host.ID)
		expected := !strings.EqualFold(strings.TrimSpace(host.Status), "offline")
		healthy, ok := graph.State.ConnectionHealth[key]
		if key == "host-" || !ok || healthy != expected {
			return false
		}
	}
	for _, instance := range graph.State.PBSInstances {
		key := "pbs-" + strings.TrimSpace(instance.Name)
		expected := !strings.EqualFold(strings.TrimSpace(instance.Status), "offline")
		healthy, ok := graph.State.ConnectionHealth[key]
		if key == "pbs-" || !ok || healthy != expected {
			return false
		}
	}
	for _, instance := range graph.State.PMGInstances {
		key := "pmg-" + strings.TrimSpace(instance.Name)
		expected := !strings.EqualFold(strings.TrimSpace(instance.Status), "offline")
		healthy, ok := graph.State.ConnectionHealth[key]
		if key == "pmg-" || !ok || healthy != expected {
			return false
		}
	}
	return true
}

func firstLegacyMockClusterLeak(graph FixtureGraph) string {
	for _, node := range graph.State.Nodes {
		if strings.EqualFold(node.Instance, "mock-cluster") || strings.EqualFold(node.ClusterName, "mock-cluster") {
			return "node:" + firstNonEmptyTrimmed(node.DisplayName, node.Name, node.ID)
		}
	}
	for _, vm := range graph.State.VMs {
		if strings.EqualFold(vm.Instance, "mock-cluster") {
			return "vm:" + firstNonEmptyTrimmed(vm.Name, vm.ID)
		}
	}
	for _, container := range graph.State.Containers {
		if strings.EqualFold(container.Instance, "mock-cluster") {
			return "container:" + firstNonEmptyTrimmed(container.Name, container.ID)
		}
	}
	for _, storage := range graph.State.Storage {
		if strings.EqualFold(storage.Instance, "mock-cluster") {
			return "storage:" + firstNonEmptyTrimmed(storage.Name, storage.ID)
		}
	}
	return ""
}
