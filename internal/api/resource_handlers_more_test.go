package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	unifiedresources "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

type stubResourceStateProvider struct {
	snapshot models.StateSnapshot
}

func (s stubResourceStateProvider) GetState() models.StateSnapshot {
	return s.snapshot
}

type stubTenantStateProvider struct {
	snapshot models.StateSnapshot
}

func (s stubTenantStateProvider) GetStateForTenant(_ string) models.StateSnapshot {
	return s.snapshot
}

func decodeResourcesResponse(t *testing.T, rec *httptest.ResponseRecorder) ResourcesResponse {
	t.Helper()
	var resp ResourcesResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp
}

func TestResourceHandlers_GetStoreStats(t *testing.T) {
	handlers := NewResourceHandlers()
	handlers.Store().Upsert(unifiedresources.LegacyResource{ID: "vm-1", Type: unifiedresources.LegacyResourceTypeVM})
	handlers.getStoreForTenant("tenant-a").Upsert(unifiedresources.LegacyResource{ID: "node-1", Type: unifiedresources.LegacyResourceTypeNode})

	stats := handlers.GetStoreStats()
	if stats["default"].TotalResources != 1 {
		t.Fatalf("expected default total 1, got %d", stats["default"].TotalResources)
	}
	if stats["tenant-a"].TotalResources != 1 {
		t.Fatalf("expected tenant total 1, got %d", stats["tenant-a"].TotalResources)
	}
}

func TestHandleGetResources_FiltersAndModes(t *testing.T) {
	handlers := NewResourceHandlers()
	handlers.Store().Upsert(unifiedresources.LegacyResource{
		ID:           "vm-1",
		Type:         unifiedresources.LegacyResourceTypeVM,
		Status:       unifiedresources.LegacyStatusRunning,
		PlatformType: unifiedresources.LegacyPlatformProxmoxPVE,
		ParentID:     "node-1",
		Alerts:       []unifiedresources.LegacyAlert{{ID: "alert-1"}},
	})
	handlers.Store().Upsert(unifiedresources.LegacyResource{
		ID:           "node-1",
		Type:         unifiedresources.LegacyResourceTypeNode,
		Status:       unifiedresources.LegacyStatusOnline,
		PlatformType: unifiedresources.LegacyPlatformProxmoxPVE,
	})
	handlers.Store().Upsert(unifiedresources.LegacyResource{
		ID:           "ct-1",
		Type:         unifiedresources.LegacyResourceTypeContainer,
		Status:       unifiedresources.LegacyStatusStopped,
		PlatformType: unifiedresources.LegacyPlatformProxmoxPVE,
	})
	handlers.Store().Upsert(unifiedresources.LegacyResource{
		ID:           "dh-1",
		Type:         unifiedresources.LegacyResourceTypeDockerHost,
		Status:       unifiedresources.LegacyStatusOnline,
		PlatformType: unifiedresources.LegacyPlatformDocker,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/resources", nil)
	rec := httptest.NewRecorder()
	handlers.HandleGetResources(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	resp := decodeResourcesResponse(t, rec)
	if resp.Count != 4 {
		t.Fatalf("expected count 4, got %d", resp.Count)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/resources?type=vm,container", nil)
	rec = httptest.NewRecorder()
	handlers.HandleGetResources(rec, req)
	resp = decodeResourcesResponse(t, rec)
	if resp.Count != 2 {
		t.Fatalf("expected type count 2, got %d", resp.Count)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/resources?platform=docker", nil)
	rec = httptest.NewRecorder()
	handlers.HandleGetResources(rec, req)
	resp = decodeResourcesResponse(t, rec)
	if resp.Count != 1 || resp.Resources[0].ID != "dh-1" {
		t.Fatalf("expected docker host, got %#v", resp.Resources)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/resources?status=running", nil)
	rec = httptest.NewRecorder()
	handlers.HandleGetResources(rec, req)
	resp = decodeResourcesResponse(t, rec)
	if resp.Count != 1 || resp.Resources[0].ID != "vm-1" {
		t.Fatalf("expected running vm, got %#v", resp.Resources)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/resources?parent=node-1", nil)
	rec = httptest.NewRecorder()
	handlers.HandleGetResources(rec, req)
	resp = decodeResourcesResponse(t, rec)
	if resp.Count != 1 || resp.Resources[0].ID != "vm-1" {
		t.Fatalf("expected parent filter vm, got %#v", resp.Resources)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/resources?alerts=true", nil)
	rec = httptest.NewRecorder()
	handlers.HandleGetResources(rec, req)
	resp = decodeResourcesResponse(t, rec)
	if resp.Count != 1 || resp.Resources[0].ID != "vm-1" {
		t.Fatalf("expected alerts filter vm, got %#v", resp.Resources)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/resources?infrastructure=true", nil)
	rec = httptest.NewRecorder()
	handlers.HandleGetResources(rec, req)
	resp = decodeResourcesResponse(t, rec)
	if resp.Count != 2 {
		t.Fatalf("expected infrastructure count 2, got %d", resp.Count)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/resources?workloads=true", nil)
	rec = httptest.NewRecorder()
	handlers.HandleGetResources(rec, req)
	resp = decodeResourcesResponse(t, rec)
	if resp.Count != 2 {
		t.Fatalf("expected workloads count 2, got %d", resp.Count)
	}
}

func TestHandleGetResources_MethodNotAllowed(t *testing.T) {
	handlers := NewResourceHandlers()
	req := httptest.NewRequest(http.MethodPost, "/api/resources", nil)
	rec := httptest.NewRecorder()

	handlers.HandleGetResources(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHandleGetResource_MethodNotAllowedAndMissingID(t *testing.T) {
	handlers := NewResourceHandlers()

	req := httptest.NewRequest(http.MethodPost, "/api/resources/vm-1", nil)
	rec := httptest.NewRecorder()
	handlers.HandleGetResource(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/resources/", nil)
	rec = httptest.NewRecorder()
	handlers.HandleGetResource(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestHandleGetResourceStats_MethodNotAllowed(t *testing.T) {
	handlers := NewResourceHandlers()
	req := httptest.NewRequest(http.MethodPost, "/api/resources/stats", nil)
	rec := httptest.NewRecorder()

	handlers.HandleGetResourceStats(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHandleGetResources_StateProviderAndTenantProvider(t *testing.T) {
	handlers := NewResourceHandlers()

	snapshot := models.StateSnapshot{
		Nodes: []models.Node{{ID: "node-1", Name: "node-1"}},
	}
	handlers.SetStateProvider(stubResourceStateProvider{snapshot: snapshot})

	req := httptest.NewRequest(http.MethodGet, "/api/resources", nil)
	rec := httptest.NewRecorder()
	handlers.HandleGetResources(rec, req)
	resp := decodeResourcesResponse(t, rec)
	if resp.Count != 1 {
		t.Fatalf("expected count 1 from state provider, got %d", resp.Count)
	}

	tenantSnapshot := models.StateSnapshot{
		Nodes: []models.Node{{ID: "node-tenant", Name: "node-tenant"}},
	}
	handlers.SetTenantStateProvider(stubTenantStateProvider{snapshot: tenantSnapshot})
	req = httptest.NewRequest(http.MethodGet, "/api/resources", nil)
	ctx := context.WithValue(req.Context(), OrgIDContextKey, "tenant-1")
	req = req.WithContext(ctx)
	rec = httptest.NewRecorder()
	handlers.HandleGetResources(rec, req)
	resp = decodeResourcesResponse(t, rec)
	if resp.Count != 1 || resp.Resources[0].ID != "node-tenant" {
		t.Fatalf("expected tenant resource, got %#v", resp.Resources)
	}
}

func TestHandleGetResources_RespectsTenantContextIsolation(t *testing.T) {
	handlers := NewResourceHandlers()

	handlers.PopulateFromSnapshotForTenant("org-a", models.StateSnapshot{
		Nodes: []models.Node{{ID: "node-a", Name: "node-a"}},
	})
	handlers.PopulateFromSnapshotForTenant("org-b", models.StateSnapshot{
		Nodes: []models.Node{{ID: "node-b", Name: "node-b"}},
	})

	reqA := httptest.NewRequest(http.MethodGet, "/api/resources", nil)
	reqA = reqA.WithContext(context.WithValue(reqA.Context(), OrgIDContextKey, "org-a"))
	recA := httptest.NewRecorder()
	handlers.HandleGetResources(recA, reqA)
	if recA.Code != http.StatusOK {
		t.Fatalf("expected 200 for org-a resources, got %d", recA.Code)
	}
	respA := decodeResourcesResponse(t, recA)
	if respA.Count != 1 || respA.Resources[0].ID != "node-a" {
		t.Fatalf("expected org-a resources only, got %#v", respA.Resources)
	}

	reqB := httptest.NewRequest(http.MethodGet, "/api/resources", nil)
	reqB = reqB.WithContext(context.WithValue(reqB.Context(), OrgIDContextKey, "org-b"))
	recB := httptest.NewRecorder()
	handlers.HandleGetResources(recB, reqB)
	if recB.Code != http.StatusOK {
		t.Fatalf("expected 200 for org-b resources, got %d", recB.Code)
	}
	respB := decodeResourcesResponse(t, recB)
	if respB.Count != 1 || respB.Resources[0].ID != "node-b" {
		t.Fatalf("expected org-b resources only, got %#v", respB.Resources)
	}
}

func TestPopulateFromSnapshotForTenant(t *testing.T) {
	handlers := NewResourceHandlers()

	now := time.Now()
	snapshot := models.StateSnapshot{
		Nodes: []models.Node{{
			ID:     "node-1",
			Name:   "node-1",
			CPU:    0.2,
			Memory: models.Memory{Total: 100, Used: 50, Free: 50, Usage: 50},
			Disk:   models.Disk{Total: 100, Used: 10, Free: 90, Usage: 10},
		}},
		VMs: []models.VM{{
			ID:     "vm-1",
			Name:   "vm-1",
			Status: "running",
			Memory: models.Memory{Total: 100, Used: 10, Free: 90, Usage: 10},
			Disk:   models.Disk{Total: 100, Used: 10, Free: 90, Usage: 10},
		}},
		Containers: []models.Container{{
			ID:     "ct-1",
			Name:   "ct-1",
			Status: "running",
			Memory: models.Memory{Total: 100, Used: 10, Free: 90, Usage: 10},
			Disk:   models.Disk{Total: 100, Used: 10, Free: 90, Usage: 10},
		}},
		Hosts: []models.Host{{
			ID:       "host-1",
			Hostname: "host-1",
			Status:   "online",
			Memory:   models.Memory{Total: 100, Used: 20, Free: 80, Usage: 20},
		}},
		DockerHosts: []models.DockerHost{{
			ID:               "dh-1",
			Hostname:         "dh-1",
			Status:           "online",
			CPUUsage:         5,
			Memory:           models.Memory{Total: 100, Used: 20, Free: 80, Usage: 20},
			TotalMemoryBytes: 100,
			LastSeen:         now,
			IntervalSeconds:  60,
			Containers: []models.DockerContainer{{
				ID:            "dc-1",
				Name:          "dc-1",
				State:         "running",
				Status:        "running",
				CPUPercent:    1,
				MemoryLimit:   100,
				MemoryUsage:   20,
				MemoryPercent: 20,
				UptimeSeconds: 10,
				CreatedAt:     now,
			}},
		}},
		PBSInstances: []models.PBSInstance{{
			ID:               "pbs-1",
			Name:             "pbs-1",
			Host:             "pbs-host",
			Status:           "online",
			CPU:              2,
			Memory:           10,
			MemoryUsed:       10,
			MemoryTotal:      100,
			Uptime:           10,
			ConnectionHealth: "ok",
			LastSeen:         now,
		}},
		Storage: []models.Storage{{
			ID:       "storage-1",
			Name:     "storage-1",
			Instance: "inst-1",
			Node:     "node-1",
			Status:   "online",
			Total:    100,
			Used:     50,
			Free:     50,
			Usage:    50,
			Content:  "images",
			Enabled:  true,
			Active:   true,
		}},
	}

	handlers.PopulateFromSnapshotForTenant("tenant-1", snapshot)

	store := handlers.getStoreForTenant("tenant-1")
	stats := store.GetStats()
	if stats.TotalResources != 8 {
		t.Fatalf("expected 8 resources, got %d", stats.TotalResources)
	}
}

func TestParsePlatformTypesAndStatuses(t *testing.T) {
	platforms := parsePlatformTypes(" docker , proxmox-pve , ")
	if len(platforms) != 2 {
		t.Fatalf("expected 2 platforms, got %d", len(platforms))
	}
	if platforms[0] != unifiedresources.LegacyPlatformDocker || platforms[1] != unifiedresources.LegacyPlatformProxmoxPVE {
		t.Fatalf("unexpected platforms: %#v", platforms)
	}

	statuses := parseStatuses("running, stopped ,")
	if len(statuses) != 2 {
		t.Fatalf("expected 2 statuses, got %d", len(statuses))
	}
	if statuses[0] != unifiedresources.LegacyStatusRunning || statuses[1] != unifiedresources.LegacyStatusStopped {
		t.Fatalf("unexpected statuses: %#v", statuses)
	}
}
