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

type resourceV2StateProvider struct {
	snapshot models.StateSnapshot
}

func (s resourceV2StateProvider) GetState() models.StateSnapshot {
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

func TestResourceV2ListMergesLinkedHost(t *testing.T) {
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
	h := NewResourceV2Handlers(cfg)
	h.SetStateProvider(resourceV2StateProvider{snapshot: snapshot})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v2/resources?type=host", nil)
	h.HandleListResources(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp ResourcesV2Response
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

func TestResourceV2ListDoesNotMergeOneSidedLinkedHost(t *testing.T) {
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
	h := NewResourceV2Handlers(cfg)
	h.SetStateProvider(resourceV2StateProvider{snapshot: snapshot})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v2/resources?type=host", nil)
	h.HandleListResources(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp ResourcesV2Response
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

func TestResourceV2GetResource(t *testing.T) {
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
	h := NewResourceV2Handlers(cfg)
	h.SetStateProvider(resourceV2StateProvider{snapshot: snapshot})

	listRec := httptest.NewRecorder()
	listReq := httptest.NewRequest(http.MethodGet, "/api/v2/resources?type=host", nil)
	h.HandleListResources(listRec, listReq)

	var listResp ResourcesV2Response
	if err := json.NewDecoder(listRec.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listResp.Data) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(listResp.Data))
	}

	resourceID := listResp.Data[0].ID
	getRec := httptest.NewRecorder()
	getReq := httptest.NewRequest(http.MethodGet, "/api/v2/resources/"+resourceID, nil)
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

func TestResourceV2LinkMergesResources(t *testing.T) {
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
	h := NewResourceV2Handlers(cfg)
	h.SetStateProvider(resourceV2StateProvider{snapshot: snapshot})

	listRec := httptest.NewRecorder()
	listReq := httptest.NewRequest(http.MethodGet, "/api/v2/resources?type=host", nil)
	h.HandleListResources(listRec, listReq)

	var listResp ResourcesV2Response
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
	linkReq := httptest.NewRequest(http.MethodPost, "/api/v2/resources/"+primaryID+"/link", bytes.NewReader(payloadBytes))
	h.HandleLink(linkRec, linkReq)
	if linkRec.Code != http.StatusOK {
		t.Fatalf("link status = %d, body=%s", linkRec.Code, linkRec.Body.String())
	}

	listRec2 := httptest.NewRecorder()
	listReq2 := httptest.NewRequest(http.MethodGet, "/api/v2/resources?type=host", nil)
	h.HandleListResources(listRec2, listReq2)

	var listResp2 ResourcesV2Response
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

func TestResourceV2ReportMergeCreatesExclusions(t *testing.T) {
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
	h := NewResourceV2Handlers(cfg)
	h.SetStateProvider(resourceV2StateProvider{snapshot: snapshot})

	listRec := httptest.NewRecorder()
	listReq := httptest.NewRequest(http.MethodGet, "/api/v2/resources?type=host", nil)
	h.HandleListResources(listRec, listReq)

	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, body=%s", listRec.Code, listRec.Body.String())
	}

	var listResp ResourcesV2Response
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
	reportReq := httptest.NewRequest(http.MethodPost, "/api/v2/resources/"+resourceID+"/report-merge", bytes.NewReader(reportBytes))
	h.HandleReportMerge(reportRec, reportReq)
	if reportRec.Code != http.StatusOK {
		t.Fatalf("report-merge status = %d, body=%s", reportRec.Code, reportRec.Body.String())
	}

	listRec2 := httptest.NewRecorder()
	listReq2 := httptest.NewRequest(http.MethodGet, "/api/v2/resources?type=host", nil)
	h.HandleListResources(listRec2, listReq2)

	if listRec2.Code != http.StatusOK {
		t.Fatalf("list status = %d, body=%s", listRec2.Code, listRec2.Body.String())
	}

	var listResp2 ResourcesV2Response
	if err := json.NewDecoder(listRec2.Body).Decode(&listResp2); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listResp2.Data) != 2 {
		t.Fatalf("expected 2 resources after report-merge, got %d", len(listResp2.Data))
	}
}

func TestResourceV2ListIncludesKubernetesPods(t *testing.T) {
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
	h := NewResourceV2Handlers(cfg)
	h.SetStateProvider(resourceV2StateProvider{snapshot: snapshot})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v2/resources?type=pod", nil)
	h.HandleListResources(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp ResourcesV2Response
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

func TestResourceV2TypeAliasKubernetesIncludesKubernetesResources(t *testing.T) {
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
	h := NewResourceV2Handlers(cfg)
	h.SetStateProvider(resourceV2StateProvider{snapshot: snapshot})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v2/resources?type=k8s", nil)
	h.HandleListResources(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp ResourcesV2Response
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(resp.Data) != 3 {
		t.Fatalf("expected 3 kubernetes resources for k8s alias, got %d", len(resp.Data))
	}
}

func TestResourceV2ListIncludesPBSAndPMG(t *testing.T) {
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
				MailStats: &models.PMGMailStats{
					BytesIn:  1_500_000,
					BytesOut: 900_000,
				},
			},
		},
	}

	cfg := &config.Config{DataPath: t.TempDir()}
	h := NewResourceV2Handlers(cfg)
	h.SetStateProvider(resourceV2StateProvider{snapshot: snapshot})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v2/resources?type=pbs,pmg", nil)
	h.HandleListResources(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp ResourcesV2Response
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Data) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resp.Data))
	}

	var gotPBS, gotPMG bool
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
			if resource.PMG == nil {
				t.Fatalf("expected PMG payload, got nil")
			}
			if resource.DiscoveryTarget == nil || resource.DiscoveryTarget.ResourceType != "host" {
				t.Fatalf("expected host discovery target for PMG, got %+v", resource.DiscoveryTarget)
			}
		}
	}

	if !gotPBS || !gotPMG {
		t.Fatalf("expected both PBS and PMG resources, got %+v", resp.Data)
	}
}

func TestResourceV2ListIncludesStorageMetadata(t *testing.T) {
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
	h := NewResourceV2Handlers(cfg)
	h.SetStateProvider(resourceV2StateProvider{snapshot: snapshot})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v2/resources?type=storage", nil)
	h.HandleListResources(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp ResourcesV2Response
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

func TestResourceV2ListIncludesTrueNASFromSupplementalProvider(t *testing.T) {
	now := time.Now().UTC()

	cfg := &config.Config{DataPath: t.TempDir()}
	h := NewResourceV2Handlers(cfg)
	h.SetStateProvider(resourceV2StateProvider{snapshot: models.StateSnapshot{LastUpdate: now}})
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
	req := httptest.NewRequest(http.MethodGet, "/api/v2/resources?source=truenas", nil)
	h.HandleListResources(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp ResourcesV2Response
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

func TestResourceV2ListWithoutSupplementalProvider(t *testing.T) {
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
	h := NewResourceV2Handlers(cfg)
	h.SetStateProvider(resourceV2StateProvider{snapshot: snapshot})

	truenasRec := httptest.NewRecorder()
	truenasReq := httptest.NewRequest(http.MethodGet, "/api/v2/resources?source=truenas", nil)
	h.HandleListResources(truenasRec, truenasReq)

	if truenasRec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", truenasRec.Code, truenasRec.Body.String())
	}

	var truenasResp ResourcesV2Response
	if err := json.NewDecoder(truenasRec.Body).Decode(&truenasResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(truenasResp.Data) != 0 {
		t.Fatalf("expected 0 truenas resources without supplemental provider, got %d", len(truenasResp.Data))
	}

	agentRec := httptest.NewRecorder()
	agentReq := httptest.NewRequest(http.MethodGet, "/api/v2/resources?source=agent", nil)
	h.HandleListResources(agentRec, agentReq)

	if agentRec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", agentRec.Code, agentRec.Body.String())
	}

	var agentResp ResourcesV2Response
	if err := json.NewDecoder(agentRec.Body).Decode(&agentResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(agentResp.Data) != 1 {
		t.Fatalf("expected 1 agent resource, got %d", len(agentResp.Data))
	}
}
