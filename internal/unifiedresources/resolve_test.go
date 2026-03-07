package unifiedresources

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestResolveResource_NilReadState(t *testing.T) {
	loc := ResolveResource(nil, "anything")
	if loc.Found {
		t.Fatal("expected not found for nil ReadState")
	}
}

func TestResolveResource_Node(t *testing.T) {
	rr := NewRegistry(nil)
	rr.IngestSnapshot(models.StateSnapshot{
		Nodes: []models.Node{{ID: "n1", Name: "pve-node"}},
	})
	loc := ResolveResource(rr, "pve-node")
	if !loc.Found {
		t.Fatal("expected node to be found")
	}
	if loc.ResourceType != "node" {
		t.Fatalf("expected node type, got %q", loc.ResourceType)
	}
	if loc.TargetHost != "pve-node" {
		t.Fatalf("expected target_host pve-node, got %q", loc.TargetHost)
	}
}

func TestResolveResource_VM(t *testing.T) {
	rr := NewRegistry(nil)
	rr.IngestSnapshot(models.StateSnapshot{
		VMs: []models.VM{{ID: "vm-1", Name: "alpha", VMID: 101, Node: "node1"}},
	})
	loc := ResolveResource(rr, "alpha")
	if !loc.Found || loc.ResourceType != "vm" {
		t.Fatalf("expected vm, got found=%v type=%q", loc.Found, loc.ResourceType)
	}
	if loc.VMID != 101 || loc.Node != "node1" {
		t.Fatalf("expected VMID=101 Node=node1, got VMID=%d Node=%q", loc.VMID, loc.Node)
	}
}

func TestResolveResource_Container(t *testing.T) {
	rr := NewRegistry(nil)
	rr.IngestSnapshot(models.StateSnapshot{
		Containers: []models.Container{{ID: "lxc-1", Name: "beta", VMID: 201, Node: "node1", Type: "lxc"}},
	})
	loc := ResolveResource(rr, "beta")
	if !loc.Found || loc.ResourceType != "system-container" {
		t.Fatalf("expected system-container, got found=%v type=%q", loc.Found, loc.ResourceType)
	}
	if loc.VMID != 201 {
		t.Fatalf("expected VMID=201, got %d", loc.VMID)
	}
}

func TestResolveResource_DockerContainer(t *testing.T) {
	rr := NewRegistry(nil)
	rr.IngestSnapshot(models.StateSnapshot{
		Containers: []models.Container{{ID: "lxc-1", Name: "dock1", VMID: 100, Node: "node1", Type: "lxc"}},
		DockerHosts: []models.DockerHost{{
			ID:       "dock1",
			Hostname: "dock1",
			Containers: []models.DockerContainer{{
				ID:    "cid1",
				Name:  "homepage",
				State: "running",
			}},
		}},
	})
	loc := ResolveResource(rr, "homepage")
	if !loc.Found || loc.ResourceType != "app-container" {
		t.Fatalf("expected app-container, got found=%v type=%q", loc.Found, loc.ResourceType)
	}
	if loc.DockerHostName != "dock1" {
		t.Fatalf("expected docker host dock1, got %q", loc.DockerHostName)
	}
	if loc.DockerHostType != "system-container" {
		t.Fatalf("expected docker host type system-container, got %q", loc.DockerHostType)
	}
	if loc.DockerHostVMID != 100 {
		t.Fatalf("expected docker host VMID 100, got %d", loc.DockerHostVMID)
	}
	// TargetHost must be rewritten to the LXC name for command routing.
	if loc.TargetHost != "dock1" {
		t.Fatalf("expected target_host rewritten to LXC name dock1, got %q", loc.TargetHost)
	}
}

func TestResolveResource_DockerContainerTargetHostRewrite(t *testing.T) {
	// Docker container lookup must rewrite TargetHost to the backing LXC name.
	// Use Docker host ID matching (not hostname) to verify the rewrite path.
	rr := NewRegistry(nil)
	rr.IngestSnapshot(models.StateSnapshot{
		Containers: []models.Container{{ID: "lxc-1", Name: "docker-host-lxc", VMID: 100, Node: "node1", Type: "lxc"}},
		DockerHosts: []models.DockerHost{{
			ID:       "docker-host-lxc",
			Hostname: "docker-host-lxc",
			Containers: []models.DockerContainer{{
				ID:    "cid1",
				Name:  "myapp",
				State: "running",
			}},
		}},
	})
	loc := ResolveResource(rr, "myapp")
	if !loc.Found || loc.ResourceType != "app-container" {
		t.Fatalf("expected app-container, got found=%v type=%q", loc.Found, loc.ResourceType)
	}
	if loc.TargetHost != "docker-host-lxc" {
		t.Fatalf("expected target_host rewritten to LXC name, got %q", loc.TargetHost)
	}
	if loc.DockerHostType != "system-container" {
		t.Fatalf("expected system-container, got %q", loc.DockerHostType)
	}

	// Docker HOST lookup must NOT rewrite TargetHost.
	locHost := ResolveResource(rr, "docker-host-lxc")
	if !locHost.Found || locHost.ResourceType != "system-container" {
		// Should match as system-container first (earlier in resolution order)
		t.Fatalf("expected system-container, got found=%v type=%q", locHost.Found, locHost.ResourceType)
	}
}

func TestResolveResource_DockerHost(t *testing.T) {
	rr := NewRegistry(nil)
	rr.IngestSnapshot(models.StateSnapshot{
		DockerHosts: []models.DockerHost{{
			ID:       "standalone1",
			Hostname: "standalone1",
		}},
	})
	loc := ResolveResource(rr, "standalone1")
	if !loc.Found || loc.ResourceType != "docker-host" {
		t.Fatalf("expected docker-host, got found=%v type=%q", loc.Found, loc.ResourceType)
	}
	if loc.DockerHostType != "standalone" {
		t.Fatalf("expected standalone, got %q", loc.DockerHostType)
	}
}

func TestResolveResource_Host(t *testing.T) {
	rr := NewRegistry(nil)
	rr.IngestSnapshot(models.StateSnapshot{
		Hosts: []models.Host{{ID: "host1", Hostname: "myserver", Platform: "linux"}},
	})
	loc := ResolveResource(rr, "myserver")
	if !loc.Found || loc.ResourceType != "agent" {
		t.Fatalf("expected agent, got found=%v type=%q", loc.Found, loc.ResourceType)
	}
	if loc.Platform != "linux" {
		t.Fatalf("expected linux platform, got %q", loc.Platform)
	}
	if loc.TargetID != "host1" {
		t.Fatalf("expected TargetID=host1 (canonical target ID), got %q", loc.TargetID)
	}

	// Lookup by agent/source ID should also work.
	loc2 := ResolveResource(rr, "host1")
	if !loc2.Found || loc2.ResourceType != "agent" {
		t.Fatalf("expected agent lookup by agent ID, got found=%v type=%q", loc2.Found, loc2.ResourceType)
	}
}

func TestResolveResource_K8sCluster(t *testing.T) {
	rr := NewRegistry(nil)
	rr.IngestSnapshot(models.StateSnapshot{
		KubernetesClusters: []models.KubernetesCluster{{
			ID:      "k8s1",
			Name:    "prod",
			AgentID: "agent-1",
		}},
	})
	loc := ResolveResource(rr, "prod")
	if !loc.Found || loc.ResourceType != "k8s-cluster" {
		t.Fatalf("expected k8s-cluster, got found=%v type=%q", loc.Found, loc.ResourceType)
	}
	if loc.K8sAgentID != "agent-1" {
		t.Fatalf("expected agent-1, got %q", loc.K8sAgentID)
	}
}

func TestResolveResource_K8sPod(t *testing.T) {
	rr := NewRegistry(nil)
	rr.IngestSnapshot(models.StateSnapshot{
		KubernetesClusters: []models.KubernetesCluster{{
			ID:      "k8s1",
			Name:    "prod",
			AgentID: "agent-1",
			Pods: []models.KubernetesPod{{
				Name:      "nginx-abc",
				Namespace: "default",
			}},
		}},
	})
	loc := ResolveResource(rr, "nginx-abc")
	if !loc.Found || loc.ResourceType != "k8s-pod" {
		t.Fatalf("expected k8s-pod, got found=%v type=%q", loc.Found, loc.ResourceType)
	}
	if loc.K8sNamespace != "default" {
		t.Fatalf("expected namespace default, got %q", loc.K8sNamespace)
	}
	if loc.K8sClusterName != "prod" {
		t.Fatalf("expected cluster prod, got %q", loc.K8sClusterName)
	}
}

func TestResolveResource_K8sDeployment(t *testing.T) {
	rr := NewRegistry(nil)
	rr.IngestSnapshot(models.StateSnapshot{
		KubernetesClusters: []models.KubernetesCluster{{
			ID:      "k8s1",
			Name:    "prod",
			AgentID: "agent-1",
			Deployments: []models.KubernetesDeployment{{
				Name:      "web-deploy",
				Namespace: "production",
			}},
		}},
	})
	loc := ResolveResource(rr, "web-deploy")
	if !loc.Found || loc.ResourceType != "k8s-deployment" {
		t.Fatalf("expected k8s-deployment, got found=%v type=%q", loc.Found, loc.ResourceType)
	}
	if loc.K8sNamespace != "production" {
		t.Fatalf("expected namespace production, got %q", loc.K8sNamespace)
	}
}

func TestResolveResource_NotFound(t *testing.T) {
	rr := NewRegistry(nil)
	rr.IngestSnapshot(models.StateSnapshot{})
	loc := ResolveResource(rr, "nonexistent")
	if loc.Found {
		t.Fatal("expected not found")
	}
	if loc.Name != "nonexistent" {
		t.Fatalf("expected name preserved, got %q", loc.Name)
	}
}
