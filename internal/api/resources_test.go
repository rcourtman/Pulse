package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

type resourceStateProvider struct {
	snapshot models.StateSnapshot
}

func (s resourceStateProvider) GetState() models.StateSnapshot {
	return s.snapshot
}

type mockSupplementalRecordsProvider struct {
	records []unified.IngestRecord
}

func (m mockSupplementalRecordsProvider) GetCurrentRecords() []unified.IngestRecord {
	out := make([]unified.IngestRecord, len(m.records))
	copy(out, m.records)
	return out
}

func TestResourceListMergesLinkedHost(t *testing.T) {
	now := time.Now().UTC()
	node := models.Node{
		ID:                "instance-pve1",
		Name:              "pve1",
		Instance:          "instance",
		Host:              "https://pve1:8006",
		Status:            "online",
		CPU:               0.15,
		Memory:            models.Memory{Total: 1024, Used: 512, Free: 512, Usage: 0.5},
		Disk:              models.Disk{Total: 2048, Used: 1024, Free: 1024, Usage: 0.5},
		LastSeen:          now,
		LinkedHostAgentID: "host-1",
	}
	host := models.Host{
		ID:           "host-1",
		Hostname:     "pve1",
		Status:       "online",
		Memory:       models.Memory{Total: 2048, Used: 1024, Free: 1024, Usage: 0.5},
		LastSeen:     now,
		LinkedNodeID: node.ID,
	}

	snapshot := models.StateSnapshot{
		Nodes: []models.Node{node},
		Hosts: []models.Host{host},
	}

	cfg := &config.Config{DataPath: t.TempDir()}
	h := NewResourceHandlers(cfg)
	h.SetStateProvider(resourceStateProvider{snapshot: snapshot})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/resources?type=host", nil)
	h.HandleListResources(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp ResourcesResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resp.Data))
	}
	resource := resp.Data[0]
	if !containsSource(resource.Sources, unified.SourceProxmox) || !containsSource(resource.Sources, unified.SourceAgent) {
		t.Fatalf("expected merged sources, got %+v", resource.Sources)
	}
	if resource.DiscoveryTarget == nil {
		t.Fatalf("expected discovery target on merged host")
	}
	if resource.DiscoveryTarget.ResourceType != "host" {
		t.Fatalf("discovery target resourceType = %q, want host", resource.DiscoveryTarget.ResourceType)
	}
	if resource.DiscoveryTarget.HostID != "host-1" || resource.DiscoveryTarget.ResourceID != "host-1" {
		t.Fatalf("discovery target = %+v, want host-1/host-1", resource.DiscoveryTarget)
	}
}

func TestResourceListDoesNotMergeOneSidedLinkedHost(t *testing.T) {
	now := time.Now().UTC()
	node := models.Node{
		ID:                "instance-pve1",
		Name:              "pve1",
		Instance:          "instance",
		Host:              "https://pve1:8006",
		Status:            "online",
		CPU:               0.15,
		Memory:            models.Memory{Total: 1024, Used: 512, Free: 512, Usage: 0.5},
		Disk:              models.Disk{Total: 2048, Used: 1024, Free: 1024, Usage: 0.5},
		LastSeen:          now,
		LinkedHostAgentID: "host-1",
	}
	host := models.Host{
		ID:       "host-1",
		Hostname: "pve1",
		Status:   "online",
		Memory:   models.Memory{Total: 2048, Used: 1024, Free: 1024, Usage: 0.5},
		LastSeen: now,
		// Intentionally not setting LinkedNodeID to ensure one-sided links are ignored.
	}

	snapshot := models.StateSnapshot{
		Nodes: []models.Node{node},
		Hosts: []models.Host{host},
	}

	cfg := &config.Config{DataPath: t.TempDir()}
	h := NewResourceHandlers(cfg)
	h.SetStateProvider(resourceStateProvider{snapshot: snapshot})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/resources?type=host", nil)
	h.HandleListResources(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp ResourcesResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(resp.Data) != 2 {
		t.Fatalf("expected 2 resources without reciprocal link, got %d", len(resp.Data))
	}

	var agentTarget, nodeTarget *unified.DiscoveryTarget
	for _, resource := range resp.Data {
		if containsSource(resource.Sources, unified.SourceAgent) {
			agentTarget = resource.DiscoveryTarget
		}
		if containsSource(resource.Sources, unified.SourceProxmox) {
			nodeTarget = resource.DiscoveryTarget
		}
	}
	if agentTarget == nil {
		t.Fatalf("expected discovery target for agent host")
	}
	if agentTarget.HostID != "host-1" || agentTarget.ResourceID != "host-1" {
		t.Fatalf("agent discovery target = %+v, want host-1/host-1", agentTarget)
	}
	if nodeTarget == nil {
		t.Fatalf("expected discovery target for proxmox node host")
	}
	if nodeTarget.HostID != "pve1" || nodeTarget.ResourceID != "pve1" {
		t.Fatalf("node discovery target = %+v, want pve1/pve1", nodeTarget)
	}
}

func TestResourceGetResource(t *testing.T) {
	now := time.Now().UTC()
	host := models.Host{
		ID:       "host-1",
		Hostname: "pve1",
		Status:   "online",
		Memory:   models.Memory{Total: 2048, Used: 1024, Free: 1024, Usage: 0.5},
		LastSeen: now,
	}
	snapshot := models.StateSnapshot{Hosts: []models.Host{host}}

	cfg := &config.Config{DataPath: t.TempDir()}
	h := NewResourceHandlers(cfg)
	h.SetStateProvider(resourceStateProvider{snapshot: snapshot})

	listRec := httptest.NewRecorder()
	listReq := httptest.NewRequest(http.MethodGet, "/api/resources?type=host", nil)
	h.HandleListResources(listRec, listReq)

	var listResp ResourcesResponse
	if err := json.NewDecoder(listRec.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listResp.Data) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(listResp.Data))
	}

	resourceID := listResp.Data[0].ID
	getRec := httptest.NewRecorder()
	getReq := httptest.NewRequest(http.MethodGet, "/api/resources/"+resourceID, nil)
	h.HandleGetResource(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", getRec.Code, getRec.Body.String())
	}
	var resource unified.Resource
	if err := json.NewDecoder(getRec.Body).Decode(&resource); err != nil {
		t.Fatalf("decode resource: %v", err)
	}
	if resource.ID != resourceID {
		t.Fatalf("resource id = %q, want %q", resource.ID, resourceID)
	}
	if resource.DiscoveryTarget == nil {
		t.Fatalf("expected discovery target on get resource")
	}
	if resource.DiscoveryTarget.HostID != "host-1" || resource.DiscoveryTarget.ResourceID != "host-1" {
		t.Fatalf("discovery target = %+v, want host-1/host-1", resource.DiscoveryTarget)
	}
}

func containsSource(sources []unified.DataSource, target unified.DataSource) bool {
	for _, source := range sources {
		if source == target {
			return true
		}
	}
	return false
}

func TestResourceLinkMergesResources(t *testing.T) {
	now := time.Now().UTC()
	host := models.Host{
		ID:       "host-1",
		Hostname: "alpha",
		Status:   "online",
		Memory:   models.Memory{Total: 2048, Used: 1024, Free: 1024, Usage: 0.5},
		LastSeen: now,
	}
	dockerHost := models.DockerHost{
		ID:       "docker-1",
		Hostname: "beta",
		Status:   "online",
		CPUs:     4,
		Memory:   models.Memory{Total: 4096, Used: 1024, Free: 3072, Usage: 0.25},
		LastSeen: now,
	}

	snapshot := models.StateSnapshot{Hosts: []models.Host{host}, DockerHosts: []models.DockerHost{dockerHost}}

	cfg := &config.Config{DataPath: t.TempDir()}
	h := NewResourceHandlers(cfg)
	h.SetStateProvider(resourceStateProvider{snapshot: snapshot})

	listRec := httptest.NewRecorder()
	listReq := httptest.NewRequest(http.MethodGet, "/api/resources?type=host", nil)
	h.HandleListResources(listRec, listReq)

	var listResp ResourcesResponse
	if err := json.NewDecoder(listRec.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listResp.Data) != 2 {
		t.Fatalf("expected 2 resources before link, got %d", len(listResp.Data))
	}
	primaryID := listResp.Data[0].ID
	secondaryID := listResp.Data[1].ID

	linkPayload := map[string]string{"targetId": secondaryID, "reason": "manual merge"}
	payloadBytes, _ := json.Marshal(linkPayload)
	linkRec := httptest.NewRecorder()
	linkReq := httptest.NewRequest(http.MethodPost, "/api/resources/"+primaryID+"/link", bytes.NewReader(payloadBytes))
	h.HandleLink(linkRec, linkReq)
	if linkRec.Code != http.StatusOK {
		t.Fatalf("link status = %d, body=%s", linkRec.Code, linkRec.Body.String())
	}

	listRec2 := httptest.NewRecorder()
	listReq2 := httptest.NewRequest(http.MethodGet, "/api/resources?type=host", nil)
	h.HandleListResources(listRec2, listReq2)

	var listResp2 ResourcesResponse
	if err := json.NewDecoder(listRec2.Body).Decode(&listResp2); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listResp2.Data) != 1 {
		t.Fatalf("expected 1 resource after link, got %d", len(listResp2.Data))
	}
	resource := listResp2.Data[0]
	if !containsSource(resource.Sources, unified.SourceAgent) || !containsSource(resource.Sources, unified.SourceDocker) {
		t.Fatalf("expected merged sources, got %+v", resource.Sources)
	}
}

func TestResourceReportMergeCreatesExclusions(t *testing.T) {
	now := time.Now().UTC()
	sharedInterfaces := []models.HostNetworkInterface{
		{
			Name:      "eth0",
			MAC:       "aa:bb:cc:dd:ee:ff",
			Addresses: []string{"10.0.0.5"},
		},
	}
	host := models.Host{
		ID:                "host-1",
		Hostname:          "alpha",
		Status:            "online",
		Memory:            models.Memory{Total: 2048, Used: 1024, Free: 1024, Usage: 0.5},
		LastSeen:          now,
		NetworkInterfaces: sharedInterfaces,
	}
	dockerHost := models.DockerHost{
		ID:                "docker-1",
		Hostname:          "alpha",
		Status:            "online",
		CPUs:              4,
		Memory:            models.Memory{Total: 4096, Used: 2048, Free: 2048, Usage: 0.5},
		LastSeen:          now,
		NetworkInterfaces: sharedInterfaces,
	}

	snapshot := models.StateSnapshot{Hosts: []models.Host{host}, DockerHosts: []models.DockerHost{dockerHost}}

	cfg := &config.Config{DataPath: t.TempDir()}
	h := NewResourceHandlers(cfg)
	h.SetStateProvider(resourceStateProvider{snapshot: snapshot})

	listRec := httptest.NewRecorder()
	listReq := httptest.NewRequest(http.MethodGet, "/api/resources?type=host", nil)
	h.HandleListResources(listRec, listReq)

	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, body=%s", listRec.Code, listRec.Body.String())
	}

	var listResp ResourcesResponse
	if err := json.NewDecoder(listRec.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listResp.Data) != 1 {
		t.Fatalf("expected 1 merged resource, got %d", len(listResp.Data))
	}
	resourceID := listResp.Data[0].ID

	reportPayload := map[string]any{
		"sources": []string{"agent", "docker"},
		"notes":   "incorrect merge",
	}
	reportBytes, _ := json.Marshal(reportPayload)
	reportRec := httptest.NewRecorder()
	reportReq := httptest.NewRequest(http.MethodPost, "/api/resources/"+resourceID+"/report-merge", bytes.NewReader(reportBytes))
	h.HandleReportMerge(reportRec, reportReq)
	if reportRec.Code != http.StatusOK {
		t.Fatalf("report-merge status = %d, body=%s", reportRec.Code, reportRec.Body.String())
	}

	listRec2 := httptest.NewRecorder()
	listReq2 := httptest.NewRequest(http.MethodGet, "/api/resources?type=host", nil)
	h.HandleListResources(listRec2, listReq2)

	if listRec2.Code != http.StatusOK {
		t.Fatalf("list status = %d, body=%s", listRec2.Code, listRec2.Body.String())
	}

	var listResp2 ResourcesResponse
	if err := json.NewDecoder(listRec2.Body).Decode(&listResp2); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listResp2.Data) != 2 {
		t.Fatalf("expected 2 resources after report-merge, got %d", len(listResp2.Data))
	}
}

func TestResourceListIncludesKubernetesPods(t *testing.T) {
	now := time.Now().UTC()
	snapshot := models.StateSnapshot{
		KubernetesClusters: []models.KubernetesCluster{
			{
				ID:       "cluster-1",
				AgentID:  "agent-1",
				Name:     "prod-k8s",
				Context:  "prod",
				Status:   "online",
				LastSeen: now,
				Version:  "1.31.2",
				Hidden:   false,
				Pods: []models.KubernetesPod{
					{
						UID:       "pod-1",
						Name:      "api-7f8d",
						Namespace: "default",
						NodeName:  "worker-1",
						Phase:     "Running",
						Containers: []models.KubernetesPodContainer{
							{Name: "api", Image: "ghcr.io/acme/api:1.2.3", Ready: true, State: "Running"},
						},
					},
				},
			},
		},
	}

	cfg := &config.Config{DataPath: t.TempDir()}
	h := NewResourceHandlers(cfg)
	h.SetStateProvider(resourceStateProvider{snapshot: snapshot})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/resources?type=pod", nil)
	h.HandleListResources(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp ResourcesResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 kubernetes pod resource, got %d", len(resp.Data))
	}

	resource := resp.Data[0]
	if resource.Type != unified.ResourceTypePod {
		t.Fatalf("resource type = %q, want %q", resource.Type, unified.ResourceTypePod)
	}
	if !containsSource(resource.Sources, unified.SourceK8s) {
		t.Fatalf("expected kubernetes source, got %+v", resource.Sources)
	}
	if resource.Kubernetes == nil || resource.Kubernetes.Namespace != "default" {
		t.Fatalf("expected kubernetes namespace metadata, got %+v", resource.Kubernetes)
	}
	if resource.DiscoveryTarget == nil {
		t.Fatalf("expected discovery target for kubernetes pod")
	}
	if resource.DiscoveryTarget.ResourceType != "k8s" {
		t.Fatalf("discovery target type = %q, want k8s", resource.DiscoveryTarget.ResourceType)
	}
	if resource.DiscoveryTarget.HostID != "agent-1" {
		t.Fatalf("discovery target hostID = %q, want agent-1", resource.DiscoveryTarget.HostID)
	}
	if resource.DiscoveryTarget.ResourceID != "pod-1" {
		t.Fatalf("discovery target resourceID = %q, want pod-1", resource.DiscoveryTarget.ResourceID)
	}
}

func TestResourceListFiltersKubernetesNamespace(t *testing.T) {
	now := time.Now().UTC()
	snapshot := models.StateSnapshot{
		KubernetesClusters: []models.KubernetesCluster{
			{
				ID:       "cluster-1",
				AgentID:  "agent-1",
				Name:     "prod-k8s",
				Context:  "prod",
				Status:   "online",
				LastSeen: now,
				Version:  "1.31.2",
				Hidden:   false,
				Pods: []models.KubernetesPod{
					{UID: "pod-1", Name: "api-1", Namespace: "default", Phase: "Running"},
					{UID: "pod-2", Name: "api-2", Namespace: "kube-system", Phase: "Running"},
				},
				Deployments: []models.KubernetesDeployment{
					{UID: "dep-1", Name: "web", Namespace: "default", DesiredReplicas: 3, ReadyReplicas: 3},
					{UID: "dep-2", Name: "dns", Namespace: "kube-system", DesiredReplicas: 2, ReadyReplicas: 2},
				},
			},
		},
	}

	cfg := &config.Config{DataPath: t.TempDir()}
	h := NewResourceHandlers(cfg)
	h.SetStateProvider(resourceStateProvider{snapshot: snapshot})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/resources?type=k8s&namespace=default", nil)
	h.HandleListResources(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp ResourcesResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(resp.Data) != 2 {
		t.Fatalf("expected 2 kubernetes resources for namespace=default, got %d", len(resp.Data))
	}

	for _, resource := range resp.Data {
		if resource.Kubernetes == nil {
			t.Fatalf("expected kubernetes payload, got nil: %+v", resource)
		}
		if resource.Kubernetes.Namespace != "default" {
			t.Fatalf("expected namespace default, got %q (resource=%+v)", resource.Kubernetes.Namespace, resource)
		}
	}
}

func TestK8sNamespacesEndpointAggregatesPodsAndDeployments(t *testing.T) {
	now := time.Now().UTC()
	snapshot := models.StateSnapshot{
		KubernetesClusters: []models.KubernetesCluster{
			{
				ID:       "cluster-1",
				AgentID:  "agent-1",
				Name:     "prod-k8s",
				Context:  "prod",
				Status:   "online",
				LastSeen: now,
				Version:  "1.31.2",
				Hidden:   false,
				Pods: []models.KubernetesPod{
					{UID: "pod-1", Name: "api-1", Namespace: "default", Phase: "Running"},
					{UID: "pod-2", Name: "api-2", Namespace: "default", Phase: "Pending"},
					{UID: "pod-3", Name: "dns-1", Namespace: "kube-system", Phase: "Running"},
				},
				Deployments: []models.KubernetesDeployment{
					{UID: "dep-1", Name: "web", Namespace: "default", DesiredReplicas: 3, ReadyReplicas: 3, AvailableReplicas: 3},
					{UID: "dep-2", Name: "dns", Namespace: "kube-system", DesiredReplicas: 2, ReadyReplicas: 1, AvailableReplicas: 1},
				},
			},
		},
	}

	cfg := &config.Config{DataPath: t.TempDir()}
	h := NewResourceHandlers(cfg)
	h.SetStateProvider(resourceStateProvider{snapshot: snapshot})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/resources/k8s/namespaces?cluster=prod-k8s", nil)
	h.HandleK8sNamespaces(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Cluster string `json:"cluster"`
		Data    []struct {
			Namespace string `json:"namespace"`
			Pods      struct {
				Total   int `json:"total"`
				Online  int `json:"online"`
				Warning int `json:"warning"`
				Offline int `json:"offline"`
				Unknown int `json:"unknown"`
			} `json:"pods"`
			Deployments struct {
				Total   int `json:"total"`
				Online  int `json:"online"`
				Warning int `json:"warning"`
				Offline int `json:"offline"`
				Unknown int `json:"unknown"`
			} `json:"deployments"`
		} `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Cluster != "prod-k8s" {
		t.Fatalf("cluster = %q, want prod-k8s", resp.Cluster)
	}

	byNS := make(map[string]struct {
		Namespace string `json:"namespace"`
		Pods      struct {
			Total   int `json:"total"`
			Online  int `json:"online"`
			Warning int `json:"warning"`
			Offline int `json:"offline"`
			Unknown int `json:"unknown"`
		} `json:"pods"`
		Deployments struct {
			Total   int `json:"total"`
			Online  int `json:"online"`
			Warning int `json:"warning"`
			Offline int `json:"offline"`
			Unknown int `json:"unknown"`
		} `json:"deployments"`
	})
	for _, row := range resp.Data {
		byNS[row.Namespace] = row
	}
	if len(byNS) != 2 {
		t.Fatalf("expected 2 namespaces, got %d (%+v)", len(byNS), resp.Data)
	}

	// default: 2 pods (one running=online, one pending=warning), 1 deployment (ready=online)
	defaultRow, ok := byNS["default"]
	if !ok {
		t.Fatalf("expected default namespace row")
	}
	if defaultRow.Pods.Total != 2 || defaultRow.Pods.Online != 1 || defaultRow.Pods.Warning != 1 {
		t.Fatalf("default pods = %+v, want total=2 online=1 warning=1", defaultRow.Pods)
	}
	if defaultRow.Deployments.Total != 1 || defaultRow.Deployments.Online != 1 {
		t.Fatalf("default deployments = %+v, want total=1 online=1", defaultRow.Deployments)
	}

	kubeSystemRow, ok := byNS["kube-system"]
	if !ok {
		t.Fatalf("expected kube-system namespace row")
	}
	if kubeSystemRow.Pods.Total != 1 || kubeSystemRow.Pods.Online != 1 {
		t.Fatalf("kube-system pods = %+v, want total=1 online=1", kubeSystemRow.Pods)
	}
	if kubeSystemRow.Deployments.Total != 1 || kubeSystemRow.Deployments.Warning != 1 {
		t.Fatalf("kube-system deployments = %+v, want total=1 warning=1", kubeSystemRow.Deployments)
	}
}

func TestResourceTypeAliasKubernetesIncludesKubernetesResources(t *testing.T) {
	now := time.Now().UTC()
	snapshot := models.StateSnapshot{
		KubernetesClusters: []models.KubernetesCluster{
			{
				ID:       "cluster-1",
				AgentID:  "agent-1",
				Name:     "prod-k8s",
				Status:   "online",
				LastSeen: now,
				Nodes: []models.KubernetesNode{
					{
						UID:   "node-1",
						Name:  "worker-1",
						Ready: true,
					},
				},
				Pods: []models.KubernetesPod{
					{
						UID:       "pod-1",
						Name:      "api-7f8d",
						Namespace: "default",
						Phase:     "Running",
					},
				},
			},
		},
	}

	cfg := &config.Config{DataPath: t.TempDir()}
	h := NewResourceHandlers(cfg)
	h.SetStateProvider(resourceStateProvider{snapshot: snapshot})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/resources?type=k8s", nil)
	h.HandleListResources(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp ResourcesResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(resp.Data) != 3 {
		t.Fatalf("expected 3 kubernetes resources for k8s alias, got %d", len(resp.Data))
	}
}

func TestResourceListIncludesDockerSwarmServicesAndFiltersByCluster(t *testing.T) {
	now := time.Now().UTC()

	service := models.DockerService{
		ID:           "svc-1",
		Name:         "web",
		Stack:        "edge",
		Image:        "nginx:1.27",
		Mode:         "replicated",
		DesiredTasks: 3,
		RunningTasks: 2,
		EndpointPorts: []models.DockerServicePort{
			{Protocol: "tcp", TargetPort: 80, PublishedPort: 8080, PublishMode: "ingress"},
		},
	}

	// Two Swarm nodes reporting the same service; unified ingest should de-dupe services per swarm cluster.
	host1 := models.DockerHost{
		ID:               "docker-1",
		AgentID:          "agent-1",
		Hostname:         "swarm-1",
		DisplayName:      "swarm-1",
		Status:           "online",
		CPUs:             4,
		TotalMemoryBytes: 8 * 1024 * 1024 * 1024,
		Memory:           models.Memory{Total: 8 * 1024 * 1024 * 1024, Used: 2 * 1024 * 1024 * 1024, Free: 6 * 1024 * 1024 * 1024, Usage: 0.25},
		LastSeen:         now,
		IntervalSeconds:  5,
		Swarm: &models.DockerSwarmInfo{
			ClusterID:   "cluster-1",
			ClusterName: "prod-swarm",
			NodeID:      "node-1",
			NodeRole:    "manager",
		},
		Services: []models.DockerService{service},
	}
	host2 := models.DockerHost{
		ID:               "docker-2",
		AgentID:          "agent-2",
		Hostname:         "swarm-2",
		DisplayName:      "swarm-2",
		Status:           "online",
		CPUs:             4,
		TotalMemoryBytes: 8 * 1024 * 1024 * 1024,
		Memory:           models.Memory{Total: 8 * 1024 * 1024 * 1024, Used: 1 * 1024 * 1024 * 1024, Free: 7 * 1024 * 1024 * 1024, Usage: 0.125},
		LastSeen:         now,
		IntervalSeconds:  5,
		Swarm: &models.DockerSwarmInfo{
			ClusterID:   "cluster-1",
			ClusterName: "prod-swarm",
			NodeID:      "node-2",
			NodeRole:    "worker",
		},
		Services: []models.DockerService{service},
	}

	snapshot := models.StateSnapshot{
		DockerHosts: []models.DockerHost{host1, host2},
	}

	cfg := &config.Config{DataPath: t.TempDir()}
	h := NewResourceHandlers(cfg)
	h.SetStateProvider(resourceStateProvider{snapshot: snapshot})

	// Unfiltered-by-cluster: expect the service to show up exactly once.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/resources?type=docker_service", nil)
	h.HandleListResources(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp ResourcesResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 docker service resource (de-duped across swarm nodes), got %d", len(resp.Data))
	}

	r := resp.Data[0]
	if r.Type != unified.ResourceTypeDockerService {
		t.Fatalf("resource type = %q, want %q", r.Type, unified.ResourceTypeDockerService)
	}
	if r.Docker == nil {
		t.Fatalf("expected docker payload on docker-service resource")
	}
	if r.Docker.ServiceID != "svc-1" || r.Name != "web" {
		t.Fatalf("unexpected service identity: name=%q serviceId=%q", r.Name, r.Docker.ServiceID)
	}
	if r.Identity.ClusterName != "prod-swarm" {
		t.Fatalf("identity.clusterName = %q, want prod-swarm", r.Identity.ClusterName)
	}

	// Cluster filter should also return the service.
	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/api/resources?type=docker_service&cluster=prod-swarm", nil)
	h.HandleListResources(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("cluster status = %d, body=%s", rec2.Code, rec2.Body.String())
	}
	var resp2 ResourcesResponse
	if err := json.NewDecoder(rec2.Body).Decode(&resp2); err != nil {
		t.Fatalf("decode cluster response: %v", err)
	}
	if len(resp2.Data) != 1 {
		t.Fatalf("expected 1 docker service resource for cluster filter, got %d", len(resp2.Data))
	}
}

func TestResourceListIncludesPBSAndPMG(t *testing.T) {
	now := time.Now().UTC()
	snapshot := models.StateSnapshot{
		PBSInstances: []models.PBSInstance{
			{
				ID:               "pbs-1",
				Name:             "pbs-main",
				Host:             "https://pbs.example.com:8007",
				Status:           "online",
				CPU:              14.2,
				Memory:           35.0,
				MemoryUsed:       4 * 1024 * 1024 * 1024,
				MemoryTotal:      12 * 1024 * 1024 * 1024,
				Uptime:           7200,
				ConnectionHealth: "connected",
				LastSeen:         now,
			},
		},
		PMGInstances: []models.PMGInstance{
			{
				ID:               "pmg-1",
				Name:             "pmg-main",
				Host:             "https://pmg.example.com:8006",
				Status:           "online",
				ConnectionHealth: "connected",
				LastSeen:         now,
				LastUpdated:      now,
				RelayDomains: []models.PMGRelayDomain{
					{Domain: "example.com", Comment: "primary relay"},
				},
				DomainStats: []models.PMGDomainStat{
					{Domain: "example.com", MailCount: 100, SpamCount: 5, VirusCount: 1, Bytes: 1234},
				},
				DomainStatsAsOf: now,
				MailStats: &models.PMGMailStats{
					BytesIn:  1_500_000,
					BytesOut: 900_000,
				},
			},
		},
	}

	cfg := &config.Config{DataPath: t.TempDir()}
	h := NewResourceHandlers(cfg)
	h.SetStateProvider(resourceStateProvider{snapshot: snapshot})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/resources?type=pbs,pmg", nil)
	h.HandleListResources(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp ResourcesResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Data) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resp.Data))
	}

	var gotPBS, gotPMG bool
	var pmgResourceID string
	for _, resource := range resp.Data {
		switch resource.Type {
		case unified.ResourceTypePBS:
			gotPBS = true
			if resource.PBS == nil {
				t.Fatalf("expected PBS payload, got nil")
			}
			if resource.DiscoveryTarget == nil || resource.DiscoveryTarget.ResourceType != "host" {
				t.Fatalf("expected host discovery target for PBS, got %+v", resource.DiscoveryTarget)
			}
		case unified.ResourceTypePMG:
			gotPMG = true
			pmgResourceID = resource.ID
			if resource.PMG == nil {
				t.Fatalf("expected PMG payload, got nil")
			}
			// List response should be summary-only (heavy fields pruned).
			if len(resource.PMG.RelayDomains) > 0 {
				t.Fatalf("expected relayDomains pruned from list response, got %+v", resource.PMG.RelayDomains)
			}
			if len(resource.PMG.DomainStats) > 0 {
				t.Fatalf("expected domainStats pruned from list response, got %+v", resource.PMG.DomainStats)
			}
			if resource.DiscoveryTarget == nil || resource.DiscoveryTarget.ResourceType != "host" {
				t.Fatalf("expected host discovery target for PMG, got %+v", resource.DiscoveryTarget)
			}
		}
	}

	if !gotPBS || !gotPMG {
		t.Fatalf("expected both PBS and PMG resources, got %+v", resp.Data)
	}

	// Detail response should include the heavy fields.
	if pmgResourceID == "" {
		t.Fatalf("expected pmg resource id to be set")
	}
	getRec := httptest.NewRecorder()
	getReq := httptest.NewRequest(http.MethodGet, "/api/resources/"+pmgResourceID, nil)
	h.HandleGetResource(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get status = %d, body=%s", getRec.Code, getRec.Body.String())
	}
	var pmgResource unified.Resource
	if err := json.NewDecoder(getRec.Body).Decode(&pmgResource); err != nil {
		t.Fatalf("decode pmg get response: %v", err)
	}
	if pmgResource.PMG == nil {
		t.Fatalf("expected PMG payload on get response, got nil")
	}
	if len(pmgResource.PMG.RelayDomains) == 0 {
		t.Fatalf("expected relayDomains on get response, got empty")
	}
	if len(pmgResource.PMG.DomainStats) == 0 {
		t.Fatalf("expected domainStats on get response, got empty")
	}
}

func TestResourceListIncludesStorageMetadata(t *testing.T) {
	snapshot := models.StateSnapshot{
		Storage: []models.Storage{
			{
				ID:       "storage-1",
				Name:     "ceph-rbd",
				Node:     "pve-1",
				Instance: "cluster-a",
				Type:     "rbd",
				Content:  "images,backup",
				Shared:   true,
				Status:   "available",
				Enabled:  true,
				Active:   true,
				Total:    1000,
				Used:     250,
				Free:     750,
				Usage:    25,
			},
		},
	}

	cfg := &config.Config{DataPath: t.TempDir()}
	h := NewResourceHandlers(cfg)
	h.SetStateProvider(resourceStateProvider{snapshot: snapshot})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/resources?type=storage", nil)
	h.HandleListResources(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp ResourcesResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 storage resource, got %d", len(resp.Data))
	}

	resource := resp.Data[0]
	if resource.Storage == nil {
		t.Fatalf("expected storage metadata payload")
	}
	if got, want := resource.Storage.Type, "rbd"; got != want {
		t.Fatalf("storage.type = %q, want %q", got, want)
	}
	if got, want := resource.Storage.Content, "images,backup"; got != want {
		t.Fatalf("storage.content = %q, want %q", got, want)
	}
	if got, want := resource.Storage.ContentTypes, []string{"images", "backup"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("storage.contentTypes = %v, want %v", got, want)
	}
	if !resource.Storage.Shared {
		t.Fatalf("expected storage.shared=true")
	}
	if !resource.Storage.IsCeph {
		t.Fatalf("expected storage.isCeph=true")
	}
	if resource.Storage.IsZFS {
		t.Fatalf("expected storage.isZfs=false")
	}
	if resource.Proxmox == nil || resource.Proxmox.NodeName != "pve-1" || resource.Proxmox.Instance != "cluster-a" {
		t.Fatalf("expected proxmox node/instance metadata to remain populated, got %+v", resource.Proxmox)
	}
}

func TestResourceListIncludesTrueNASFromSupplementalProvider(t *testing.T) {
	now := time.Now().UTC()

	cfg := &config.Config{DataPath: t.TempDir()}
	h := NewResourceHandlers(cfg)
	h.SetStateProvider(resourceStateProvider{snapshot: models.StateSnapshot{LastUpdate: now}})
	h.SetSupplementalRecordsProvider(unified.SourceTrueNAS, mockSupplementalRecordsProvider{
		records: []unified.IngestRecord{
			{
				SourceID: "system:truenas-main",
				Resource: unified.Resource{
					Type:      unified.ResourceTypeHost,
					Name:      "truenas-main",
					Status:    unified.StatusOnline,
					LastSeen:  now,
					UpdatedAt: now,
				},
				Identity: unified.ResourceIdentity{
					MachineID: "tn-main",
					Hostnames: []string{"truenas-main"},
				},
			},
		},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/resources?source=truenas", nil)
	h.HandleListResources(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp ResourcesResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 truenas resource, got %d", len(resp.Data))
	}

	resource := resp.Data[0]
	if resource.Type != unified.ResourceTypeHost {
		t.Fatalf("resource type = %q, want %q", resource.Type, unified.ResourceTypeHost)
	}
	if !containsSource(resource.Sources, unified.SourceTrueNAS) {
		t.Fatalf("expected truenas source, got %+v", resource.Sources)
	}
}

func TestResourceListWithoutSupplementalProvider(t *testing.T) {
	now := time.Now().UTC()
	snapshot := models.StateSnapshot{
		LastUpdate: now,
		Hosts: []models.Host{
			{
				ID:       "host-1",
				Hostname: "agent-host",
				Status:   "online",
				LastSeen: now,
			},
		},
	}

	cfg := &config.Config{DataPath: t.TempDir()}
	h := NewResourceHandlers(cfg)
	h.SetStateProvider(resourceStateProvider{snapshot: snapshot})

	truenasRec := httptest.NewRecorder()
	truenasReq := httptest.NewRequest(http.MethodGet, "/api/resources?source=truenas", nil)
	h.HandleListResources(truenasRec, truenasReq)

	if truenasRec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", truenasRec.Code, truenasRec.Body.String())
	}

	var truenasResp ResourcesResponse
	if err := json.NewDecoder(truenasRec.Body).Decode(&truenasResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(truenasResp.Data) != 0 {
		t.Fatalf("expected 0 truenas resources without supplemental provider, got %d", len(truenasResp.Data))
	}

	agentRec := httptest.NewRecorder()
	agentReq := httptest.NewRequest(http.MethodGet, "/api/resources?source=agent", nil)
	h.HandleListResources(agentRec, agentReq)

	if agentRec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", agentRec.Code, agentRec.Body.String())
	}

	var agentResp ResourcesResponse
	if err := json.NewDecoder(agentRec.Body).Decode(&agentResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(agentResp.Data) != 1 {
		t.Fatalf("expected 1 agent resource, got %d", len(agentResp.Data))
	}
}
