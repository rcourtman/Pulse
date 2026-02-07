package unifiedresources

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestIngestSnapshotIncludesKubernetesHierarchy(t *testing.T) {
	mock.SetEnabled(true)
	t.Cleanup(func() { mock.SetEnabled(false) })

	now := time.Now().UTC()
	podStart := now.Add(-15 * time.Minute)
	snapshot := models.StateSnapshot{
		KubernetesClusters: []models.KubernetesCluster{
			{
				ID:              "cluster-1",
				AgentID:         "k8s-agent-1",
				Name:            "prod-k8s",
				Context:         "prod",
				Version:         "1.31.2",
				Status:          "online",
				LastSeen:        now,
				IntervalSeconds: 30,
				Nodes: []models.KubernetesNode{
					{
						UID:            "node-uid-1",
						Name:           "worker-1",
						Ready:          true,
						Roles:          []string{"worker"},
						KubeletVersion: "v1.31.2",
					},
				},
				Pods: []models.KubernetesPod{
					{
						UID:       "pod-uid-1",
						Name:      "api-123",
						Namespace: "default",
						NodeName:  "worker-1",
						Phase:     "Running",
						CreatedAt: now.Add(-20 * time.Minute),
						StartTime: &podStart,
						Containers: []models.KubernetesPodContainer{
							{Name: "api", Image: "ghcr.io/acme/api:1.2.3", Ready: true, State: "Running"},
						},
					},
				},
				Deployments: []models.KubernetesDeployment{
					{
						UID:               "deployment-uid-1",
						Name:              "api",
						Namespace:         "default",
						DesiredReplicas:   3,
						AvailableReplicas: 2,
					},
				},
			},
		},
	}

	registry := NewRegistry(NewMemoryStore())
	registry.IngestSnapshot(snapshot)

	resources := registry.List()
	if len(resources) != 4 {
		t.Fatalf("expected 4 kubernetes resources, got %d", len(resources))
	}

	var clusterResource *Resource
	var nodeResource *Resource
	var podResource *Resource
	var deploymentResource *Resource

	for i := range resources {
		resource := resources[i]
		switch resource.Type {
		case ResourceTypeK8sCluster:
			clusterResource = &resource
		case ResourceTypeK8sNode:
			nodeResource = &resource
		case ResourceTypePod:
			podResource = &resource
		case ResourceTypeK8sDeployment:
			deploymentResource = &resource
		}
	}

	if clusterResource == nil {
		t.Fatal("expected kubernetes cluster resource")
	}
	if nodeResource == nil {
		t.Fatal("expected kubernetes node resource")
	}
	if podResource == nil {
		t.Fatal("expected kubernetes pod resource")
	}
	if deploymentResource == nil {
		t.Fatal("expected kubernetes deployment resource")
	}

	if !containsDataSource(clusterResource.Sources, SourceK8s) {
		t.Fatalf("expected cluster source to include kubernetes, got %+v", clusterResource.Sources)
	}
	if nodeResource.ParentID == nil || *nodeResource.ParentID != clusterResource.ID {
		t.Fatalf("expected node parent %q, got %+v", clusterResource.ID, nodeResource.ParentID)
	}
	if podResource.ParentID == nil || *podResource.ParentID != clusterResource.ID {
		t.Fatalf("expected pod parent %q, got %+v", clusterResource.ID, podResource.ParentID)
	}
	if deploymentResource.ParentID == nil || *deploymentResource.ParentID != clusterResource.ID {
		t.Fatalf("expected deployment parent %q, got %+v", clusterResource.ID, deploymentResource.ParentID)
	}

	if podResource.Kubernetes == nil || podResource.Kubernetes.Namespace != "default" {
		t.Fatalf("expected pod namespace metadata, got %+v", podResource.Kubernetes)
	}
	if podResource.Kubernetes.UptimeSeconds <= 0 {
		t.Fatalf("expected pod uptimeSeconds to be populated, got %d", podResource.Kubernetes.UptimeSeconds)
	}
	if podResource.Metrics == nil || podResource.Metrics.CPU == nil || podResource.Metrics.CPU.Value <= 0 {
		t.Fatalf("expected pod cpu metric in mock mode, got %+v", podResource.Metrics)
	}
	if nodeResource.Metrics == nil {
		t.Fatalf("expected node metrics payload to exist, got nil")
	}
	if nodeResource.Metrics.CPU != nil || nodeResource.Metrics.Memory != nil || nodeResource.Metrics.Disk != nil ||
		nodeResource.Metrics.NetIn != nil || nodeResource.Metrics.NetOut != nil ||
		nodeResource.Metrics.DiskRead != nil || nodeResource.Metrics.DiskWrite != nil {
		t.Fatalf("expected node usage metrics to be absent, got %+v", nodeResource.Metrics)
	}
	if nodeResource.Kubernetes == nil {
		t.Fatalf("expected node kubernetes metadata, got nil")
	}
	if nodeResource.Kubernetes.UptimeSeconds != 0 {
		t.Fatalf("expected node uptimeSeconds to be unset, got %d", nodeResource.Kubernetes.UptimeSeconds)
	}
	if nodeResource.Kubernetes.Temperature != nil {
		t.Fatalf("expected node temperature to be unset, got %+v", nodeResource.Kubernetes.Temperature)
	}
	if clusterResource.Metrics == nil {
		t.Fatalf("expected cluster metrics payload to exist, got nil")
	}
	if clusterResource.Metrics.CPU != nil || clusterResource.Metrics.Memory != nil || clusterResource.Metrics.Disk != nil ||
		clusterResource.Metrics.NetIn != nil || clusterResource.Metrics.NetOut != nil ||
		clusterResource.Metrics.DiskRead != nil || clusterResource.Metrics.DiskWrite != nil {
		t.Fatalf("expected cluster usage metrics to be absent, got %+v", clusterResource.Metrics)
	}
	if clusterResource.Kubernetes == nil {
		t.Fatalf("expected cluster kubernetes metadata, got nil")
	}
	if clusterResource.Kubernetes.UptimeSeconds != 0 {
		t.Fatalf("expected cluster uptimeSeconds to be unset, got %d", clusterResource.Kubernetes.UptimeSeconds)
	}
	if clusterResource.Kubernetes.Temperature != nil {
		t.Fatalf("expected cluster temperature to be unset, got %+v", clusterResource.Kubernetes.Temperature)
	}
}

func TestIngestSnapshotLinksKubernetesNodesToHostAgentMetrics(t *testing.T) {
	now := time.Now().UTC()
	snapshot := models.StateSnapshot{
		Hosts: []models.Host{
			{
				ID:            "host-worker-1",
				Hostname:      "worker-1.example.local",
				Status:        "online",
				LastSeen:      now,
				UptimeSeconds: 7200,
				CPUUsage:      42,
				Memory: models.Memory{
					Total: 16 * 1024 * 1024 * 1024,
					Used:  8 * 1024 * 1024 * 1024,
					Usage: 50,
				},
				Disks: []models.Disk{
					{Total: 500 * 1024 * 1024 * 1024, Used: 200 * 1024 * 1024 * 1024, Usage: 40},
				},
				NetInRate:     1200,
				NetOutRate:    2400,
				DiskReadRate:  3600,
				DiskWriteRate: 4800,
				Sensors: models.HostSensorSummary{
					TemperatureCelsius: map[string]float64{"cpu.package": 58.5},
				},
			},
		},
		KubernetesClusters: []models.KubernetesCluster{
			{
				ID:       "cluster-1",
				AgentID:  "k8s-agent-1",
				Name:     "prod-k8s",
				Status:   "online",
				LastSeen: now,
				Nodes: []models.KubernetesNode{
					{
						UID:  "node-uid-1",
						Name: "worker-1",
					},
				},
			},
		},
	}

	registry := NewRegistry(NewMemoryStore())
	registry.IngestSnapshot(snapshot)

	resources := registry.List()
	if len(resources) != 3 {
		t.Fatalf("expected 3 resources (host + cluster + node), got %d", len(resources))
	}

	var clusterResource *Resource
	var nodeResource *Resource
	for i := range resources {
		resource := resources[i]
		switch resource.Type {
		case ResourceTypeK8sCluster:
			clusterResource = &resource
		case ResourceTypeK8sNode:
			nodeResource = &resource
		}
	}

	if nodeResource == nil {
		t.Fatal("expected kubernetes node resource")
	}
	if nodeResource.Metrics == nil || nodeResource.Metrics.CPU == nil || nodeResource.Metrics.CPU.Value <= 0 {
		t.Fatalf("expected kubernetes node cpu metric from linked host, got %+v", nodeResource.Metrics)
	}
	if nodeResource.Metrics.NetIn == nil || nodeResource.Metrics.NetIn.Value != 1200 {
		t.Fatalf("expected kubernetes node netIn metric from linked host, got %+v", nodeResource.Metrics)
	}
	if nodeResource.Kubernetes == nil || nodeResource.Kubernetes.UptimeSeconds != 7200 {
		t.Fatalf("expected kubernetes node uptime from linked host, got %+v", nodeResource.Kubernetes)
	}
	if nodeResource.Kubernetes.Temperature == nil || *nodeResource.Kubernetes.Temperature <= 0 {
		t.Fatalf("expected kubernetes node temperature from linked host, got %+v", nodeResource.Kubernetes)
	}

	if clusterResource == nil {
		t.Fatal("expected kubernetes cluster resource")
	}
	if clusterResource.Metrics == nil || clusterResource.Metrics.CPU == nil || clusterResource.Metrics.CPU.Value <= 0 {
		t.Fatalf("expected kubernetes cluster metrics aggregated from linked hosts, got %+v", clusterResource.Metrics)
	}
}

func TestIngestSnapshotSkipsHiddenKubernetesClusters(t *testing.T) {
	snapshot := models.StateSnapshot{
		KubernetesClusters: []models.KubernetesCluster{
			{
				ID:       "cluster-hidden",
				AgentID:  "k8s-agent-hidden",
				Name:     "hidden",
				Hidden:   true,
				Status:   "online",
				LastSeen: time.Now().UTC(),
				Pods: []models.KubernetesPod{
					{
						UID:       "pod-hidden",
						Name:      "pod-hidden",
						Namespace: "default",
					},
				},
			},
		},
	}

	registry := NewRegistry(NewMemoryStore())
	registry.IngestSnapshot(snapshot)

	if got := len(registry.List()); got != 0 {
		t.Fatalf("expected hidden kubernetes cluster to be skipped, got %d resources", got)
	}
}
