package unifiedresources

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestIngestSnapshotIncludesKubernetesHierarchy(t *testing.T) {
	enableMockMode(t)

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
				Namespaces: []models.KubernetesNamespace{{UID: "namespace-uid-1", Name: "default", Phase: "Active"}},
				Services: []models.KubernetesService{{
					UID: "service-uid-1", Name: "api", Namespace: "default", ServiceType: "ClusterIP", ClusterIP: "10.96.0.10",
				}},
				StatefulSets: []models.KubernetesStatefulSet{{
					UID: "statefulset-uid-1", Name: "db", Namespace: "default", DesiredReplicas: 3, ReadyReplicas: 2,
				}},
				DaemonSets: []models.KubernetesDaemonSet{{
					UID: "daemonset-uid-1", Name: "node-agent", Namespace: "kube-system", DesiredNumberScheduled: 1, NumberReady: 1,
				}},
				Jobs:     []models.KubernetesJob{{UID: "job-uid-1", Name: "backup", Namespace: "default", DesiredCompletions: 1, Succeeded: 1}},
				CronJobs: []models.KubernetesCronJob{{UID: "cronjob-uid-1", Name: "nightly", Namespace: "default", Schedule: "0 1 * * *"}},
				Ingresses: []models.KubernetesIngress{{
					UID: "ingress-uid-1", Name: "api", Namespace: "default", Hosts: []string{"api.example.test"},
				}},
				PersistentVolumes: []models.KubernetesPersistentVolume{{
					UID: "pv-uid-1", Name: "pv-api", Phase: "Bound", ClaimNamespace: "default", ClaimName: "api-data",
				}},
				PersistentVolumeClaims: []models.KubernetesPersistentVolumeClaim{{
					UID: "pvc-uid-1", Name: "api-data", Namespace: "default", Phase: "Bound", VolumeName: "pv-api",
				}},
				Events: []models.KubernetesEvent{{UID: "event-uid-1", Name: "api.1", Namespace: "default", EventType: "Warning", Reason: "BackOff", Count: 2}},
			},
		},
	}

	registry := NewRegistry(NewMemoryStore())
	registry.IngestSnapshot(snapshot)

	resources := registry.List()
	if len(resources) != 14 {
		t.Fatalf("expected 14 kubernetes resources, got %d", len(resources))
	}

	var clusterResource *Resource
	var nodeResource *Resource
	var podResource *Resource
	var deploymentResource *Resource
	seenTypes := make(map[ResourceType]int)

	for i := range resources {
		resource := resources[i]
		seenTypes[resource.Type]++
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
	for _, resourceType := range []ResourceType{
		ResourceTypeK8sNamespace,
		ResourceTypeK8sService,
		ResourceTypeK8sStatefulSet,
		ResourceTypeK8sDaemonSet,
		ResourceTypeK8sJob,
		ResourceTypeK8sCronJob,
		ResourceTypeK8sIngress,
		ResourceTypeK8sPV,
		ResourceTypeK8sPVC,
		ResourceTypeK8sEvent,
	} {
		if seenTypes[resourceType] != 1 {
			t.Fatalf("expected one %s resource, got counts %#v", resourceType, seenTypes)
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
	if podResource.Kubernetes.MetricCapabilities == nil {
		t.Fatalf("expected kubernetes metric capabilities on pod resource, got nil")
	}
	if podResource.Kubernetes.MetricCapabilities.NodeCPUMemory {
		t.Fatalf("expected node CPU/memory capability to be false without linked hosts or node usage metrics, got %+v", podResource.Kubernetes.MetricCapabilities)
	}
	if podResource.Kubernetes.MetricCapabilities.NodeTelemetry {
		t.Fatalf("expected node telemetry capability to be false without linked host agent, got %+v", podResource.Kubernetes.MetricCapabilities)
	}
	if podResource.Kubernetes.MetricCapabilities.PodDiskIO {
		t.Fatalf("expected pod disk I/O capability to remain false, got %+v", podResource.Kubernetes.MetricCapabilities)
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
	if clusterResource.Kubernetes.MetricCapabilities == nil {
		t.Fatalf("expected cluster kubernetes metric capabilities, got nil")
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
	if len(resources) != 2 {
		t.Fatalf("expected 2 resources (cluster + merged agent), got %d", len(resources))
	}

	var clusterResource *Resource
	var agentResource *Resource
	for i := range resources {
		resource := resources[i]
		switch resource.Type {
		case ResourceTypeK8sCluster:
			clusterResource = &resource
		case ResourceTypeAgent:
			agentResource = &resource
		}
	}

	if agentResource == nil {
		t.Fatal("expected linked kubernetes node to merge into agent resource")
	}
	if agentResource.Metrics == nil || agentResource.Metrics.CPU == nil || agentResource.Metrics.CPU.Value <= 0 {
		t.Fatalf("expected merged agent cpu metric from linked host, got %+v", agentResource.Metrics)
	}
	if agentResource.Metrics.NetIn == nil || agentResource.Metrics.NetIn.Value != 1200 {
		t.Fatalf("expected merged agent netIn metric from linked host, got %+v", agentResource.Metrics)
	}
	if agentResource.Kubernetes == nil || agentResource.Kubernetes.UptimeSeconds != 7200 {
		t.Fatalf("expected kubernetes node payload to retain linked host uptime, got %+v", agentResource.Kubernetes)
	}
	if agentResource.Kubernetes.Temperature == nil || *agentResource.Kubernetes.Temperature <= 0 {
		t.Fatalf("expected kubernetes node payload to retain linked host temperature, got %+v", agentResource.Kubernetes)
	}
	if agentResource.Kubernetes.MetricCapabilities == nil {
		t.Fatalf("expected kubernetes metric capabilities on node resource, got nil")
	}
	if !agentResource.Kubernetes.MetricCapabilities.NodeCPUMemory {
		t.Fatalf("expected node CPU/memory capability from linked host, got %+v", agentResource.Kubernetes.MetricCapabilities)
	}
	if !agentResource.Kubernetes.MetricCapabilities.NodeTelemetry {
		t.Fatalf("expected node telemetry capability from linked host, got %+v", agentResource.Kubernetes.MetricCapabilities)
	}
	if !hasDataSource(agentResource.Sources, SourceAgent) || !hasDataSource(agentResource.Sources, SourceK8s) {
		t.Fatalf("expected merged agent sources to include agent+kubernetes, got %+v", agentResource.Sources)
	}

	if clusterResource == nil {
		t.Fatal("expected kubernetes cluster resource")
	}
	if clusterResource.Metrics == nil || clusterResource.Metrics.CPU == nil || clusterResource.Metrics.CPU.Value <= 0 {
		t.Fatalf("expected kubernetes cluster metrics aggregated from linked hosts, got %+v", clusterResource.Metrics)
	}
}

func TestIngestSnapshotKubernetesCapabilitiesFromK8sMetrics(t *testing.T) {
	now := time.Now().UTC()
	snapshot := models.StateSnapshot{
		KubernetesClusters: []models.KubernetesCluster{
			{
				ID:       "cluster-k8s-only",
				AgentID:  "k8s-agent-only",
				Name:     "k8s-only",
				Status:   "online",
				LastSeen: now,
				Nodes: []models.KubernetesNode{
					{
						UID:                "node-uid-1",
						Name:               "worker-1",
						UsageCPUMilliCores: 720,
						UsageCPUPercent:    36,
						UsageMemoryBytes:   6 * 1024 * 1024 * 1024,
					},
				},
				Pods: []models.KubernetesPod{
					{
						UID:                           "pod-uid-1",
						Name:                          "api-1",
						Namespace:                     "default",
						NodeName:                      "worker-1",
						Phase:                         "Running",
						UsageCPUMilliCores:            450,
						UsageCPUPercent:               22,
						UsageMemoryBytes:              850 * 1024 * 1024,
						UsageMemoryPercent:            17,
						NetworkRxBytes:                250000,
						NetworkTxBytes:                190000,
						NetInRate:                     1300,
						NetOutRate:                    900,
						EphemeralStorageUsedBytes:     3 * 1024 * 1024 * 1024,
						EphemeralStorageCapacityBytes: 12 * 1024 * 1024 * 1024,
						DiskUsagePercent:              25,
					},
				},
			},
		},
	}

	registry := NewRegistry(NewMemoryStore())
	registry.IngestSnapshot(snapshot)

	resources := registry.List()
	var clusterResource *Resource
	for i := range resources {
		resource := resources[i]
		if resource.Type == ResourceTypeK8sCluster {
			clusterResource = &resource
			break
		}
	}
	if clusterResource == nil {
		t.Fatal("expected kubernetes cluster resource")
	}
	if clusterResource.Kubernetes == nil || clusterResource.Kubernetes.MetricCapabilities == nil {
		t.Fatalf("expected kubernetes metric capabilities on cluster resource, got %+v", clusterResource.Kubernetes)
	}

	cap := clusterResource.Kubernetes.MetricCapabilities
	if !cap.NodeCPUMemory {
		t.Fatalf("expected node CPU/memory capability from k8s usage metrics, got %+v", cap)
	}
	if cap.NodeTelemetry {
		t.Fatalf("expected node telemetry capability to be false without linked host agent, got %+v", cap)
	}
	if !cap.PodCPUMemory {
		t.Fatalf("expected pod CPU/memory capability from k8s usage metrics, got %+v", cap)
	}
	if !cap.PodNetwork {
		t.Fatalf("expected pod network capability from pod rates/bytes, got %+v", cap)
	}
	if !cap.PodEphemeralDisk {
		t.Fatalf("expected pod ephemeral disk capability from summary metrics, got %+v", cap)
	}
	if cap.PodDiskIO {
		t.Fatalf("expected pod disk I/O capability to remain false, got %+v", cap)
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
