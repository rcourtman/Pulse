package unifiedresources

import (
	"sort"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/storagehealth"
)

func strPointer(v string) *string {
	return &v
}

func int64Pointer(v int64) *int64 {
	return &v
}

func testRegistry(resources ...Resource) *ResourceRegistry {
	registry := NewRegistry(nil)
	registry.mu.Lock()
	defer registry.mu.Unlock()

	registry.resources = make(map[string]*Resource, len(resources))
	for i := range resources {
		resource := resources[i]
		registry.resources[resource.ID] = &resource
	}
	return registry
}

func resourceIDs(resources []Resource) []string {
	ids := make([]string, len(resources))
	for i, resource := range resources {
		ids[i] = resource.ID
	}
	sort.Strings(ids)
	return ids
}

func TestMonitorAdapterGetPollingRecommendationsAndSkip(t *testing.T) {
	adapter := NewMonitorAdapter(testRegistry(
		Resource{
			ID:      "agent-1",
			Name:    "ignored",
			Sources: []DataSource{SourceAgent},
			Agent:   &AgentData{Hostname: " Agent-Host "},
		},
		Resource{
			ID:       "hybrid-1",
			Name:     "fallback-name-not-used",
			Sources:  []DataSource{SourceAgent, SourceProxmox},
			Identity: ResourceIdentity{Hostnames: []string{" hybrid-host "}},
		},
		Resource{
			ID:      "hybrid-2",
			Name:    "Agent-Host",
			Sources: []DataSource{SourceAgent, SourceProxmox},
		},
		Resource{
			ID:      "api-1",
			Name:    "api-only",
			Sources: []DataSource{SourceProxmox},
			Proxmox: &ProxmoxData{NodeName: "api-only"},
		},
		Resource{
			ID:      "agent-empty",
			Sources: []DataSource{SourceAgent},
		},
	))

	got := adapter.GetPollingRecommendations()
	if len(got) != 2 {
		t.Fatalf("expected 2 recommendations, got %d", len(got))
	}
	if got["agent-host"] != 0 {
		t.Fatalf("expected agent-host multiplier to be 0, got %.2f", got["agent-host"])
	}
	if got["hybrid-host"] != 0.5 {
		t.Fatalf("expected hybrid-host multiplier to be 0.5, got %.2f", got["hybrid-host"])
	}

	if !adapter.ShouldSkipAPIPolling(" AGENT-HOST ") {
		t.Fatalf("expected ShouldSkipAPIPolling to normalize hostname and skip")
	}
	if adapter.ShouldSkipAPIPolling("hybrid-host") {
		t.Fatalf("expected hybrid host to be reduced polling, not skipped")
	}
	if adapter.ShouldSkipAPIPolling("missing-host") {
		t.Fatalf("expected unknown host not to be skipped")
	}
	if adapter.ShouldSkipAPIPolling("  ") {
		t.Fatalf("expected blank hostname not to be skipped")
	}
}

func TestMonitorSourceType(t *testing.T) {
	tests := []struct {
		name    string
		sources []DataSource
		want    string
	}{
		{name: "none defaults to api", sources: nil, want: "api"},
		{name: "agent source", sources: []DataSource{SourceAgent}, want: "agent"},
		{name: "docker source", sources: []DataSource{SourceDocker}, want: "agent"},
		{name: "k8s source", sources: []DataSource{SourceK8s}, want: "agent"},
		{name: "proxmox source", sources: []DataSource{SourceProxmox}, want: "api"},
		{name: "multiple sources are hybrid", sources: []DataSource{SourceAgent, SourceProxmox}, want: "hybrid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := monitorSourceType(tt.sources); got != tt.want {
				t.Fatalf("monitorSourceType(%v) = %q, want %q", tt.sources, got, tt.want)
			}
		})
	}
}

func TestMonitorHostnamePriority(t *testing.T) {
	tests := []struct {
		name     string
		resource Resource
		want     string
	}{
		{
			name: "agent hostname preferred",
			resource: Resource{
				Agent:   &AgentData{Hostname: " agent-host "},
				Docker:  &DockerData{Hostname: "docker-host"},
				Proxmox: &ProxmoxData{NodeName: "node-host"},
				Identity: ResourceIdentity{
					Hostnames: []string{"identity-host"},
				},
			},
			want: "agent-host",
		},
		{
			name: "docker hostname fallback",
			resource: Resource{
				Agent:   &AgentData{Hostname: " "},
				Docker:  &DockerData{Hostname: " docker-host "},
				Proxmox: &ProxmoxData{NodeName: "node-host"},
			},
			want: "docker-host",
		},
		{
			name: "proxmox hostname fallback",
			resource: Resource{
				Agent:   &AgentData{Hostname: ""},
				Docker:  &DockerData{Hostname: " "},
				Proxmox: &ProxmoxData{NodeName: " node-host "},
			},
			want: "node-host",
		},
		{
			name: "identity hostname fallback",
			resource: Resource{
				Identity: ResourceIdentity{Hostnames: []string{" ", " identity-host ", "other"}},
			},
			want: "identity-host",
		},
		{
			name:     "empty resource",
			resource: Resource{},
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := monitorHostname(tt.resource); got != tt.want {
				t.Fatalf("monitorHostname() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMonitorAdapterPopulateFromSnapshot(t *testing.T) {
	now := time.Date(2026, 2, 12, 12, 0, 0, 0, time.UTC)
	snapshot := models.StateSnapshot{
		Hosts: []models.Host{
			{
				ID:       "host-1",
				Hostname: "host-1.local",
				Status:   "online",
				LastSeen: now,
			},
		},
		ActiveAlerts: []models.Alert{
			{ID: "alert-1", ResourceID: "host-1"},
		},
	}

	adapter := NewMonitorAdapter(NewRegistry(nil))
	adapter.PopulateFromSnapshot(snapshot)

	if len(adapter.GetAll()) == 0 {
		t.Fatalf("expected PopulateFromSnapshot to ingest resources")
	}
	if len(adapter.activeAlerts) != 1 {
		t.Fatalf("expected 1 active alert, got %d", len(adapter.activeAlerts))
	}
	if adapter.activeAlerts[0].ID != "alert-1" {
		t.Fatalf("expected copied alert ID alert-1, got %q", adapter.activeAlerts[0].ID)
	}

	snapshot.ActiveAlerts[0].ID = "mutated"
	if adapter.activeAlerts[0].ID != "alert-1" {
		t.Fatalf("expected active alerts to be copied from snapshot")
	}

	nilRegistryAdapter := &MonitorAdapter{}
	nilRegistryAdapter.PopulateFromSnapshot(snapshot)
	if len(nilRegistryAdapter.activeAlerts) != 0 {
		t.Fatalf("expected nil-registry adapter to ignore snapshot")
	}
}

func TestMonitorAdapterPopulateFromSnapshotReplacesPreviousRegistryState(t *testing.T) {
	now := time.Date(2026, 3, 7, 12, 0, 0, 0, time.UTC)
	adapter := NewMonitorAdapter(NewRegistry(nil))

	first := models.StateSnapshot{
		Hosts: []models.Host{
			{
				ID:        "host-1",
				Hostname:  "minipc.local",
				MachineID: "machine-1",
				Status:    "online",
				LastSeen:  now,
			},
		},
	}
	second := models.StateSnapshot{
		Hosts: []models.Host{
			{
				ID:        "host-2",
				Hostname:  "delly.local",
				MachineID: "machine-2",
				Status:    "online",
				LastSeen:  now.Add(time.Minute),
			},
		},
	}

	adapter.PopulateFromSnapshot(first)
	if resources := adapter.GetAll(); len(resources) != 1 || resources[0].Name != "minipc.local" {
		t.Fatalf("expected first snapshot only, got %#v", resources)
	}

	adapter.PopulateFromSnapshot(second)
	resources := adapter.GetAll()
	if len(resources) != 1 {
		t.Fatalf("expected previous snapshot resources to be replaced, got %#v", resources)
	}
	if resources[0].Name != "delly.local" {
		t.Fatalf("expected only second snapshot resource to remain, got %#v", resources)
	}
}

func TestMonitorAdapterUnifiedResourceFreshnessTracksRegistryUpdates(t *testing.T) {
	now := time.Date(2026, 3, 7, 12, 0, 0, 0, time.UTC)
	adapter := NewMonitorAdapter(NewRegistry(nil))

	adapter.PopulateFromSnapshot(models.StateSnapshot{
		Hosts: []models.Host{
			{ID: "host-1", Hostname: "minipc.local", Status: "online", LastSeen: now},
		},
	})
	firstFreshness := adapter.UnifiedResourceFreshness()
	if firstFreshness.IsZero() {
		t.Fatal("expected non-zero freshness after snapshot populate")
	}

	time.Sleep(5 * time.Millisecond)
	adapter.PopulateSupplementalRecords(DataSource("xcp"), []IngestRecord{
		{
			SourceID: "xcp-host-1",
			Resource: Resource{
				Type:     ResourceTypeAgent,
				Name:     "xcp-host-1",
				Status:   StatusOnline,
				LastSeen: now.Add(time.Minute),
			},
			Identity: ResourceIdentity{Hostnames: []string{"xcp-host-1"}},
		},
	})
	secondFreshness := adapter.UnifiedResourceFreshness()
	if !secondFreshness.After(firstFreshness) {
		t.Fatalf("expected supplemental ingest to advance freshness: first=%v second=%v", firstFreshness, secondFreshness)
	}
}

func TestMonitorAdapterPopulateSupplementalRecords(t *testing.T) {
	now := time.Date(2026, 2, 21, 10, 0, 0, 0, time.UTC)
	adapter := NewMonitorAdapter(NewRegistry(nil))

	customSource := DataSource("xcp")
	adapter.PopulateSupplementalRecords(customSource, []IngestRecord{
		{
			SourceID: "xcp-host-1",
			Resource: Resource{
				Type:     ResourceTypeAgent,
				Name:     "xcp-host-1",
				Status:   StatusOnline,
				LastSeen: now,
			},
			Identity: ResourceIdentity{Hostnames: []string{"xcp-host-1"}},
		},
	})

	resources := adapter.GetAll()
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource after supplemental ingest, got %d", len(resources))
	}
	if resources[0].Name != "xcp-host-1" {
		t.Fatalf("expected resource name xcp-host-1, got %q", resources[0].Name)
	}
	if len(resources[0].Sources) != 1 || resources[0].Sources[0] != customSource {
		t.Fatalf("expected source %q, got %#v", customSource, resources[0].Sources)
	}

	// Nil/empty inputs should be ignored safely.
	adapter.PopulateSupplementalRecords("", []IngestRecord{{SourceID: "ignored"}})
	adapter.PopulateSupplementalRecords(customSource, nil)
	nilRegistryAdapter := &MonitorAdapter{}
	nilRegistryAdapter.PopulateSupplementalRecords(customSource, []IngestRecord{{SourceID: "ignored"}})
}

func TestUnifiedAIAdapterClassificationAndStats(t *testing.T) {
	registry := testRegistry(
		Resource{ID: "host-1", Type: ResourceTypeAgent, Status: StatusOnline, Sources: []DataSource{SourceAgent}},
		Resource{ID: "cluster-1", Type: ResourceTypeK8sCluster, Status: StatusOnline, Sources: []DataSource{SourceK8s}},
		Resource{ID: "node-1", Type: ResourceTypeK8sNode, Status: StatusWarning, Sources: []DataSource{SourceK8s}},
		Resource{ID: "vm-1", Type: ResourceTypeVM, Status: StatusOnline, Sources: []DataSource{SourceProxmox}},
		Resource{ID: "lxc-1", Type: ResourceTypeSystemContainer, Status: StatusOnline, Sources: []DataSource{SourceProxmox}},
		Resource{ID: "ct-1", Type: ResourceTypeAppContainer, Status: StatusOnline, Sources: []DataSource{SourceDocker}},
		Resource{ID: "pod-1", Type: ResourceTypePod, Status: StatusOffline, Sources: []DataSource{SourceK8s}},
		Resource{ID: "dep-1", Type: ResourceTypeK8sDeployment, Status: StatusOnline, Sources: []DataSource{SourceK8s}},
		Resource{ID: "storage-1", Type: ResourceTypeStorage, Status: StatusOnline, Sources: []DataSource{SourceProxmox}},
	)
	adapter := NewUnifiedAIAdapter(registry)

	infrastructure := adapter.GetInfrastructure()
	if len(infrastructure) != 3 {
		t.Fatalf("expected 3 infrastructure resources, got %d", len(infrastructure))
	}
	workloads := adapter.GetWorkloads()
	if len(workloads) != 5 {
		t.Fatalf("expected 5 workload resources, got %d", len(workloads))
	}
	containers := adapter.GetByType(ResourceTypeAppContainer)
	if len(containers) != 1 || containers[0].ID != "ct-1" {
		t.Fatalf("expected container lookup to return ct-1, got %#v", containers)
	}

	stats := adapter.GetStats()
	if stats.Total != 9 {
		t.Fatalf("expected stats total 9, got %d", stats.Total)
	}
	if stats.ByType[ResourceTypeAgent] != 1 || stats.ByType[ResourceTypeVM] != 1 {
		t.Fatalf("unexpected ByType stats: %#v", stats.ByType)
	}
	if stats.ByStatus[StatusOffline] != 1 {
		t.Fatalf("expected one offline resource, got %#v", stats.ByStatus)
	}

	var nilAdapter *UnifiedAIAdapter
	nilStats := nilAdapter.GetStats()
	if nilStats.Total != 0 || nilStats.ByType == nil || nilStats.ByStatus == nil || nilStats.BySource == nil {
		t.Fatalf("expected nil adapter stats to return initialized empty maps")
	}
}

func TestDockerNativeInventoryAdapters(t *testing.T) {
	now := time.Date(2026, 5, 24, 8, 0, 0, 0, time.UTC)
	host := models.DockerHost{ID: "docker-host-1", Hostname: "edge", Runtime: "podman", RuntimeVersion: "5.0.0", LastSeen: now}

	imageResource, imageIdentity := resourceFromDockerImage(models.DockerImage{
		ID: "sha256:image1", RepoTags: []string{"repo/app:latest"}, SizeBytes: 1024, Labels: map[string]string{"tier": "web"},
	}, host)
	if imageResource.Type != ResourceTypeDockerImage || imageResource.Docker == nil || imageResource.Docker.ImageID != "sha256:image1" || imageIdentity.Hostnames[0] != "repo/app:latest" {
		t.Fatalf("unexpected image resource: resource=%+v identity=%+v", imageResource, imageIdentity)
	}

	volumeResource, _ := resourceFromDockerVolume(models.DockerVolume{Name: "app-data", Driver: "local", SizeBytes: 2048, RefCount: 3}, host)
	if volumeResource.Type != ResourceTypeDockerVolume || volumeResource.Docker == nil || volumeResource.Docker.VolumeName != "app-data" || volumeResource.Docker.SizeBytes != 2048 {
		t.Fatalf("unexpected volume resource: %+v", volumeResource)
	}

	networkResource, _ := resourceFromDockerNetwork(models.DockerNetwork{
		ID: "net1", Name: "app-net", Driver: "bridge", EnableIPv4: true, Subnets: []models.DockerNetworkSubnet{{Subnet: "10.88.0.0/24", Gateway: "10.88.0.1"}},
	}, host)
	if networkResource.Type != ResourceTypeDockerNetwork || networkResource.Docker == nil || networkResource.Docker.NetworkID != "net1" || networkResource.Docker.Subnets[0].Gateway != "10.88.0.1" {
		t.Fatalf("unexpected network resource: %+v", networkResource)
	}

	taskResource, taskIdentity := resourceFromDockerTask(models.DockerTask{
		ID: "task-1", ServiceName: "api", ServiceID: "svc-1", Slot: 2, DesiredState: "running", CurrentState: "running", NodeName: "edge",
	}, host)
	if taskResource.Type != ResourceTypeDockerTask || taskResource.Name != "api.2" || taskResource.Status != StatusOnline || taskIdentity.Hostnames[0] != "api.2" {
		t.Fatalf("unexpected task resource: resource=%+v identity=%+v", taskResource, taskIdentity)
	}

	nodeResource, nodeIdentity := resourceFromDockerSwarmNode(models.DockerNode{
		ID:                  "node-1",
		Hostname:            "manager-1",
		Role:                "manager",
		Availability:        "active",
		State:               "ready",
		ManagerReachability: "reachable",
		Leader:              true,
		EngineVersion:       "27.5.1",
		NanoCPUs:            4_000_000_000,
		MemoryBytes:         16 * 1024 * 1024 * 1024,
		Labels:              map[string]string{"zone": "rack-a"},
	}, host)
	if nodeResource.Type != ResourceTypeDockerSwarmNode || nodeResource.Docker == nil || nodeResource.Docker.NodeID != "node-1" || nodeResource.Docker.NodeRole != "manager" || nodeIdentity.Hostnames[0] != "manager-1" {
		t.Fatalf("unexpected swarm node resource: resource=%+v identity=%+v", nodeResource, nodeIdentity)
	}
}

func TestKubernetesNativeInventoryAdapters(t *testing.T) {
	now := time.Date(2026, 5, 24, 8, 0, 0, 0, time.UTC)
	allowExpansion := true
	automountToken := true
	cluster := models.KubernetesCluster{ID: "cluster-1", Name: "prod", DisplayName: "Production", LastSeen: now}

	service, serviceIdentity := resourceFromKubernetesService(cluster, models.KubernetesService{
		UID: "svc-1", Name: "checkout", Namespace: "services", ServiceType: "ClusterIP", ClusterIP: "10.96.0.10", Ports: []models.KubernetesServicePort{{Name: "http", Protocol: "TCP", Port: 80, TargetPort: "8080"}},
	}, nil)
	if service.Type != ResourceTypeK8sService || service.Kubernetes == nil || service.Kubernetes.ServiceType != "ClusterIP" || serviceIdentity.ClusterName != "Production" {
		t.Fatalf("unexpected service resource: resource=%+v identity=%+v", service, serviceIdentity)
	}

	replicaSet, _ := resourceFromKubernetesReplicaSet(cluster, models.KubernetesReplicaSet{Name: "checkout-6d4", Namespace: "services", DesiredReplicas: 3, ReadyReplicas: 2, OwnerKind: "Deployment", OwnerName: "checkout"}, nil)
	if replicaSet.Type != ResourceTypeK8sReplicaSet || replicaSet.Status != StatusWarning || replicaSet.Kubernetes == nil || replicaSet.Kubernetes.OwnerName != "checkout" {
		t.Fatalf("unexpected replicaset resource: %+v", replicaSet)
	}

	statefulSet, _ := resourceFromKubernetesStatefulSet(cluster, models.KubernetesStatefulSet{Name: "db", Namespace: "services", DesiredReplicas: 3, ReadyReplicas: 2}, nil)
	if statefulSet.Type != ResourceTypeK8sStatefulSet || statefulSet.Status != StatusWarning {
		t.Fatalf("unexpected statefulset resource: %+v", statefulSet)
	}

	daemonSet, _ := resourceFromKubernetesDaemonSet(cluster, models.KubernetesDaemonSet{Name: "node-agent", Namespace: "kube-system", DesiredNumberScheduled: 3, NumberReady: 3}, nil)
	if daemonSet.Type != ResourceTypeK8sDaemonSet || daemonSet.Status != StatusOnline {
		t.Fatalf("unexpected daemonset resource: %+v", daemonSet)
	}

	job, _ := resourceFromKubernetesJob(cluster, models.KubernetesJob{Name: "backup", Namespace: "services", DesiredCompletions: 1, Succeeded: 1}, nil)
	if job.Type != ResourceTypeK8sJob || job.Status != StatusOnline {
		t.Fatalf("unexpected job resource: %+v", job)
	}

	cronJob, _ := resourceFromKubernetesCronJob(cluster, models.KubernetesCronJob{Name: "nightly", Namespace: "services", Schedule: "0 1 * * *"}, nil)
	if cronJob.Type != ResourceTypeK8sCronJob || cronJob.Kubernetes == nil || cronJob.Kubernetes.Schedule != "0 1 * * *" {
		t.Fatalf("unexpected cronjob resource: %+v", cronJob)
	}

	ingress, _ := resourceFromKubernetesIngress(cluster, models.KubernetesIngress{Name: "checkout", Namespace: "services", Hosts: []string{"checkout.example.test"}}, nil)
	if ingress.Type != ResourceTypeK8sIngress || ingress.Kubernetes == nil || ingress.Kubernetes.Hosts[0] != "checkout.example.test" {
		t.Fatalf("unexpected ingress resource: %+v", ingress)
	}

	endpointSlice, _ := resourceFromKubernetesEndpointSlice(cluster, models.KubernetesEndpointSlice{Name: "checkout-abc", Namespace: "services", AddressType: "IPv4", ServiceName: "checkout", EndpointCount: 3, ReadyEndpointCount: 2, Ports: []models.KubernetesEndpointPort{{Name: "http", Protocol: "TCP", Port: 80}}}, nil)
	if endpointSlice.Type != ResourceTypeK8sEndpointSlice || endpointSlice.Status != StatusOnline || endpointSlice.Kubernetes == nil || endpointSlice.Kubernetes.ServiceName != "checkout" || endpointSlice.Kubernetes.EndpointPorts[0].Port != 80 {
		t.Fatalf("unexpected endpoint slice resource: %+v", endpointSlice)
	}

	networkPolicy, _ := resourceFromKubernetesNetworkPolicy(cluster, models.KubernetesNetworkPolicy{Name: "checkout-deny", Namespace: "services", PolicyTypes: []string{"Ingress", "Egress"}, IngressRuleCount: 1, EgressRuleCount: 2}, nil)
	if networkPolicy.Type != ResourceTypeK8sNetworkPolicy || networkPolicy.Kubernetes == nil || len(networkPolicy.Kubernetes.PolicyTypes) != 2 || networkPolicy.Kubernetes.EgressRuleCount != 2 {
		t.Fatalf("unexpected network policy resource: %+v", networkPolicy)
	}

	pv, _ := resourceFromKubernetesPersistentVolume(cluster, models.KubernetesPersistentVolume{Name: "pv-checkout", Phase: "Bound", CapacityBytes: 10_000, ClaimName: "checkout-data"}, nil)
	if pv.Type != ResourceTypeK8sPV || pv.Kubernetes == nil || pv.Kubernetes.ClaimName != "checkout-data" {
		t.Fatalf("unexpected pv resource: %+v", pv)
	}

	pvc, _ := resourceFromKubernetesPersistentVolumeClaim(cluster, models.KubernetesPersistentVolumeClaim{Name: "checkout-data", Namespace: "services", Phase: "Bound", VolumeName: "pv-checkout"}, nil)
	if pvc.Type != ResourceTypeK8sPVC || pvc.Kubernetes == nil || pvc.Kubernetes.VolumeName != "pv-checkout" {
		t.Fatalf("unexpected pvc resource: %+v", pvc)
	}

	storageClass, _ := resourceFromKubernetesStorageClass(cluster, models.KubernetesStorageClass{Name: "fast", Provisioner: "csi.example.test", ReclaimPolicy: "Delete", VolumeBindingMode: "WaitForFirstConsumer", AllowVolumeExpansion: &allowExpansion, ParameterKeys: []string{"type"}}, nil)
	if storageClass.Type != ResourceTypeK8sStorageClass || storageClass.Kubernetes == nil || storageClass.Kubernetes.Provisioner != "csi.example.test" || storageClass.Kubernetes.AllowVolumeExpansion == nil || !*storageClass.Kubernetes.AllowVolumeExpansion {
		t.Fatalf("unexpected storage class resource: %+v", storageClass)
	}

	configMap, _ := resourceFromKubernetesConfigMap(cluster, models.KubernetesConfigMap{Name: "checkout-config", Namespace: "services", DataKeys: []string{"app.yaml"}, BinaryDataKeys: []string{"logo.png"}, Immutable: true}, nil)
	if configMap.Type != ResourceTypeK8sConfigMap || configMap.Kubernetes == nil || configMap.Kubernetes.DataKeys[0] != "app.yaml" || !configMap.Kubernetes.Immutable {
		t.Fatalf("unexpected configmap resource: %+v", configMap)
	}

	secret, _ := resourceFromKubernetesSecret(cluster, models.KubernetesSecret{Name: "checkout-secret", Namespace: "services", Type: "Opaque", DataKeys: []string{"token"}, Immutable: true}, nil)
	if secret.Type != ResourceTypeK8sSecret || secret.Kubernetes == nil || secret.Kubernetes.SecretType != "Opaque" || secret.Kubernetes.DataKeys[0] != "token" || !secret.Kubernetes.Immutable {
		t.Fatalf("unexpected secret resource: %+v", secret)
	}

	serviceAccount, _ := resourceFromKubernetesServiceAccount(cluster, models.KubernetesServiceAccount{Name: "checkout", Namespace: "services", AutomountServiceAccountToken: &automountToken, SecretCount: 1, ImagePullSecrets: []string{"pull-secret"}}, nil)
	if serviceAccount.Type != ResourceTypeK8sServiceAccount || serviceAccount.Kubernetes == nil || serviceAccount.Kubernetes.SecretCount != 1 || serviceAccount.Kubernetes.AutomountServiceAccountToken == nil || !*serviceAccount.Kubernetes.AutomountServiceAccountToken {
		t.Fatalf("unexpected serviceaccount resource: %+v", serviceAccount)
	}

	resourceQuota, _ := resourceFromKubernetesResourceQuota(cluster, models.KubernetesResourceQuota{Name: "services-quota", Namespace: "services", Hard: map[string]string{"pods": "10"}, Used: map[string]string{"pods": "4"}}, nil)
	if resourceQuota.Type != ResourceTypeK8sResourceQuota || resourceQuota.Kubernetes == nil || resourceQuota.Kubernetes.Hard["pods"] != "10" || resourceQuota.Kubernetes.Used["pods"] != "4" {
		t.Fatalf("unexpected resource quota resource: %+v", resourceQuota)
	}

	limitRange, _ := resourceFromKubernetesLimitRange(cluster, models.KubernetesLimitRange{Name: "services-limits", Namespace: "services", LimitTypes: []string{"Container"}}, nil)
	if limitRange.Type != ResourceTypeK8sLimitRange || limitRange.Kubernetes == nil || limitRange.Kubernetes.LimitTypes[0] != "Container" {
		t.Fatalf("unexpected limit range resource: %+v", limitRange)
	}

	pdb, _ := resourceFromKubernetesPodDisruptionBudget(cluster, models.KubernetesPodDisruptionBudget{Name: "checkout-pdb", Namespace: "services", MinAvailable: "1", DesiredHealthy: 1, CurrentHealthy: 1, DisruptionsAllowed: 1, ExpectedPods: 2}, nil)
	if pdb.Type != ResourceTypeK8sPDB || pdb.Status != StatusOnline || pdb.Kubernetes == nil || pdb.Kubernetes.MinAvailable != "1" || pdb.Kubernetes.ExpectedPods != 2 {
		t.Fatalf("unexpected pdb resource: %+v", pdb)
	}

	hpa, _ := resourceFromKubernetesHorizontalPodAutoscaler(cluster, models.KubernetesHorizontalPodAutoscaler{Name: "checkout-hpa", Namespace: "services", TargetKind: "Deployment", TargetName: "checkout", MinReplicas: 2, MaxReplicas: 10, CurrentReplicas: 3, DesiredReplicas: 4, MetricTypes: []string{"Resource:cpu"}}, nil)
	if hpa.Type != ResourceTypeK8sHPA || hpa.Status != StatusOnline || hpa.Kubernetes == nil || hpa.Kubernetes.TargetName != "checkout" || hpa.Kubernetes.MetricTypes[0] != "Resource:cpu" {
		t.Fatalf("unexpected hpa resource: %+v", hpa)
	}

	event, _ := resourceFromKubernetesEvent(cluster, models.KubernetesEvent{Name: "checkout.1", Namespace: "services", EventType: "Warning", Reason: "BackOff", Count: 2}, nil)
	if event.Type != ResourceTypeK8sEvent || event.Status != StatusWarning || event.Kubernetes == nil || event.Kubernetes.Reason != "BackOff" {
		t.Fatalf("unexpected event resource: %+v", event)
	}
}

func TestUnifiedAIAdapterGetTopByMetric(t *testing.T) {
	registry := testRegistry(
		Resource{
			ID:   "vm-z",
			Type: ResourceTypeVM,
			Name: "Zulu",
			Metrics: &ResourceMetrics{
				CPU: &MetricValue{Percent: 90},
				Memory: &MetricValue{
					Used:  int64Pointer(8),
					Total: int64Pointer(10),
				},
			},
		},
		Resource{
			ID:   "vm-a",
			Type: ResourceTypeVM,
			Name: "alpha",
			Metrics: &ResourceMetrics{
				CPU:    &MetricValue{Value: 90},
				Memory: &MetricValue{Percent: 60},
				Disk:   &MetricValue{Percent: 70},
			},
		},
		Resource{
			ID:   "vm-b",
			Type: ResourceTypeVM,
			Name: "beta",
			Metrics: &ResourceMetrics{
				CPU: &MetricValue{
					Used:  int64Pointer(30),
					Total: int64Pointer(60),
				},
				Memory: &MetricValue{Percent: 40},
			},
		},
		Resource{
			ID:   "host-1",
			Type: ResourceTypeAgent,
			Name: "host",
			Metrics: &ResourceMetrics{
				CPU: &MetricValue{Percent: 99},
			},
		},
		Resource{
			ID:      "vm-zero",
			Type:    ResourceTypeVM,
			Name:    "zero",
			Metrics: &ResourceMetrics{CPU: &MetricValue{}},
		},
	)
	adapter := NewUnifiedAIAdapter(registry)

	topCPU := adapter.GetTopByCPU(0, []ResourceType{ResourceTypeVM})
	if len(topCPU) != 3 {
		t.Fatalf("expected 3 VMs with positive CPU metrics, got %d", len(topCPU))
	}
	if topCPU[0].ID != "vm-a" || topCPU[1].ID != "vm-z" || topCPU[2].ID != "vm-b" {
		t.Fatalf("unexpected CPU ranking order: %#v", resourceIDs(topCPU))
	}

	topCPULimit := adapter.GetTopByCPU(1, []ResourceType{ResourceTypeVM})
	if len(topCPULimit) != 1 || topCPULimit[0].ID != "vm-a" {
		t.Fatalf("expected limited CPU ranking to include vm-a, got %#v", topCPULimit)
	}

	topMemory := adapter.GetTopByMemory(0, nil)
	if len(topMemory) != 3 {
		t.Fatalf("expected 3 resources with positive memory metrics, got %d", len(topMemory))
	}
	if topMemory[0].ID != "vm-z" {
		t.Fatalf("expected vm-z to lead memory ranking, got %q", topMemory[0].ID)
	}

	topDisk := adapter.GetTopByDisk(0, []ResourceType{ResourceTypeVM})
	if len(topDisk) != 1 || topDisk[0].ID != "vm-a" {
		t.Fatalf("expected disk ranking to include vm-a only, got %#v", topDisk)
	}
}

func TestUnifiedAIAdapterIncludesVMwareCanonicalReadSurface(t *testing.T) {
	registry := testRegistry(
		Resource{
			ID:      "vmware-host-1",
			Type:    ResourceTypeAgent,
			Name:    "esxi-01.lab.local",
			Status:  StatusWarning,
			Sources: []DataSource{SourceVMware},
			VMware: &VMwareData{
				ConnectionID:       "vc-1",
				EntityType:         "host",
				ManagedObjectID:    "host-101",
				OverallStatus:      "yellow",
				ActiveAlarmCount:   1,
				ActiveAlarmSummary: "Host connection degraded (yellow)",
			},
			Incidents: []ResourceIncident{{
				Provider: "vmware",
				Code:     "vmware_alarm_state",
				Severity: storagehealth.RiskWarning,
				Summary:  "Host host-101 has VMware alarm Host connection degraded (yellow)",
			}},
		},
		Resource{
			ID:      "vmware-vm-1",
			Type:    ResourceTypeVM,
			Name:    "app-01",
			Status:  StatusWarning,
			Sources: []DataSource{SourceVMware},
			VMware: &VMwareData{
				ConnectionID:      "vc-1",
				EntityType:        "vm",
				ManagedObjectID:   "vm-201",
				OverallStatus:     "red",
				SnapshotCount:     2,
				RecentTaskSummary: "Create snapshot (success)",
			},
		},
		Resource{
			ID:      "vmware-ds-1",
			Type:    ResourceTypeStorage,
			Name:    "nvme-primary",
			Status:  StatusWarning,
			Sources: []DataSource{SourceVMware},
			VMware: &VMwareData{
				ConnectionID:      "vc-1",
				EntityType:        "datastore",
				ManagedObjectID:   "datastore-11",
				OverallStatus:     "yellow",
				RecentTaskSummary: "Refresh datastore (queued)",
			},
		},
	)

	adapter := NewUnifiedAIAdapter(registry)
	infrastructure := adapter.GetInfrastructure()
	if len(infrastructure) != 1 || infrastructure[0].ID != "vmware-host-1" {
		t.Fatalf("expected VMware host on infrastructure read path, got %#v", infrastructure)
	}
	workloads := adapter.GetWorkloads()
	if len(workloads) != 1 || workloads[0].ID != "vmware-vm-1" {
		t.Fatalf("expected VMware VM on workload read path, got %#v", workloads)
	}
	all := adapter.GetAll()
	if len(all) != 3 {
		t.Fatalf("expected all canonical VMware resources, got %d", len(all))
	}
	if all[0].VMware == nil && all[1].VMware == nil && all[2].VMware == nil {
		t.Fatal("expected VMware metadata on assistant read surface")
	}
}

func TestUnifiedAIAdapterGetRelated(t *testing.T) {
	registry := testRegistry(
		Resource{ID: "parent", Type: ResourceTypeAgent, Name: "parent-host"},
		Resource{ID: "child", Type: ResourceTypeAppContainer, Name: "child", ParentID: strPointer("parent")},
		Resource{ID: "sibling", Type: ResourceTypeVM, Name: "sibling", ParentID: strPointer("parent")},
		Resource{ID: "grandchild", Type: ResourceTypeSystemContainer, Name: "grandchild", ParentID: strPointer("child")},
		Resource{ID: "other", Type: ResourceTypeStorage, Name: "other"},
	)
	adapter := NewUnifiedAIAdapter(registry)

	childRelated := adapter.GetRelated("child")
	if len(childRelated["parent"]) != 1 || childRelated["parent"][0].ID != "parent" {
		t.Fatalf("expected parent relation for child, got %#v", childRelated["parent"])
	}
	if len(childRelated["siblings"]) != 1 || childRelated["siblings"][0].ID != "sibling" {
		t.Fatalf("expected one sibling for child, got %#v", childRelated["siblings"])
	}
	if len(childRelated["children"]) != 1 || childRelated["children"][0].ID != "grandchild" {
		t.Fatalf("expected one grandchild relation, got %#v", childRelated["children"])
	}

	parentRelated := adapter.GetRelated("parent")
	if len(parentRelated["parent"]) != 0 {
		t.Fatalf("expected parent resource to have no parent relation")
	}
	childIDs := resourceIDs(parentRelated["children"])
	if len(childIDs) != 2 || childIDs[0] != "child" || childIDs[1] != "sibling" {
		t.Fatalf("expected parent children [child sibling], got %#v", childIDs)
	}

	if got := adapter.GetRelated("missing"); len(got) != 0 {
		t.Fatalf("expected missing resource relations to be empty, got %#v", got)
	}
	if got := adapter.GetRelated("  "); len(got) != 0 {
		t.Fatalf("expected blank resource ID relations to be empty, got %#v", got)
	}
}

func TestUnifiedAIAdapterFindContainerHost(t *testing.T) {
	registry := testRegistry(
		Resource{
			ID:    "host-a",
			Type:  ResourceTypeAgent,
			Name:  "host-a-name",
			Agent: &AgentData{Hostname: "agent-host-a"},
		},
		Resource{
			ID:       "ctr-a",
			Type:     ResourceTypeAppContainer,
			Name:     "web-app",
			ParentID: strPointer("host-a"),
			Docker:   &DockerData{ContainerID: "abc123"},
		},
		Resource{
			ID:       "host-b",
			Type:     ResourceTypeAgent,
			Name:     "host-b-name",
			Identity: ResourceIdentity{Hostnames: []string{"identity-host-b"}},
		},
		Resource{
			ID:       "vm-b",
			Type:     ResourceTypeVM,
			Name:     "db-vm",
			ParentID: strPointer("host-b"),
		},
		Resource{
			ID:   "host-c",
			Type: ResourceTypeAgent,
			Name: "parent-c-name",
		},
		Resource{
			ID:       "lxc-c",
			Type:     ResourceTypeSystemContainer,
			Name:     "cache-lxc",
			ParentID: strPointer("host-c"),
		},
		Resource{
			ID:   "host-d",
			Type: ResourceTypeAgent,
		},
		Resource{
			ID:       "ctr-d",
			Type:     ResourceTypeAppContainer,
			Name:     "orphan-ish",
			ParentID: strPointer("host-d"),
		},
		Resource{
			ID:       "ctr-missing",
			Type:     ResourceTypeAppContainer,
			Name:     "missing-parent",
			ParentID: strPointer("missing-parent-id"),
		},
		Resource{
			ID:       "pod-e",
			Type:     ResourceTypePod,
			Name:     "pod-e",
			ParentID: strPointer("host-a"),
		},
	)
	adapter := NewUnifiedAIAdapter(registry)

	tests := []struct {
		name  string
		query string
		want  string
	}{
		{name: "exact container name", query: "web-app", want: "agent-host-a"},
		{name: "exact container id", query: "abc123", want: "agent-host-a"},
		{name: "partial container id", query: "bc12", want: "agent-host-a"},
		{name: "vm fallback to identity hostname", query: "db-vm", want: "identity-host-b"},
		{name: "lxc fallback to parent name", query: "cache-lxc", want: "parent-c-name"},
		{name: "fallback to parent id", query: "orphan-ish", want: "host-d"},
		{name: "missing parent", query: "missing-parent", want: ""},
		{name: "non-workload type ignored", query: "pod-e", want: ""},
		{name: "unknown query", query: "does-not-exist", want: ""},
		{name: "blank query", query: "  ", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := adapter.FindContainerHost(tt.query); got != tt.want {
				t.Fatalf("FindContainerHost(%q) = %q, want %q", tt.query, got, tt.want)
			}
		})
	}
}

func TestMetricPercent(t *testing.T) {
	tests := []struct {
		name   string
		metric *MetricValue
		want   float64
	}{
		{name: "nil", metric: nil, want: 0},
		{name: "percent has priority", metric: &MetricValue{Percent: 75, Value: 10}, want: 75},
		{name: "value fallback", metric: &MetricValue{Value: 55}, want: 55},
		{name: "used total fallback", metric: &MetricValue{Used: int64Pointer(25), Total: int64Pointer(50)}, want: 50},
		{name: "invalid total", metric: &MetricValue{Used: int64Pointer(25), Total: int64Pointer(0)}, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := metricPercent(tt.metric); got != tt.want {
				t.Fatalf("metricPercent(%#v) = %v, want %v", tt.metric, got, tt.want)
			}
		})
	}
}
