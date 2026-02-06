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
