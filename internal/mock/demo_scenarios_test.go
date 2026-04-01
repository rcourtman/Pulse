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
	for _, want := range []string{"shared-backup-fabric", "service-pool"} {
		if !storageNameExists(graph, want) {
			t.Fatalf("expected curated storage name %q in canonical demo graph", want)
		}
	}
	if !pbsInstanceExists(graph, "backup-vault") {
		t.Fatal("expected curated PBS instance in canonical demo graph")
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
	if !kubernetesNodeExists(graph, "prod-euw1-k8s-01") {
		t.Fatal("expected curated kubernetes node naming to survive metric refresh")
	}
	if !storageNameExists(graph, "shared-backup-fabric") {
		t.Fatal("expected curated storage naming to survive metric refresh")
	}
	if leak := firstLegacyMockClusterLeak(graph); leak != "" {
		t.Fatalf("expected metric refresh to preserve cluster aliasing, found %s", leak)
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

func pmgInstanceExists(graph FixtureGraph, want string) bool {
	for _, instance := range graph.State.PMGInstances {
		if instance.Name == want {
			return true
		}
	}
	return false
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
