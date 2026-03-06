package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

type tenantResourceStateProvider struct {
	snapshots map[string]models.StateSnapshot
}

func (p tenantResourceStateProvider) UnifiedReadStateForTenant(orgID string) unified.ReadState {
	return SnapshotReadState(p.GetStateForTenant(orgID))
}

func (p tenantResourceStateProvider) GetStateForTenant(orgID string) models.StateSnapshot {
	if p.snapshots == nil {
		return models.StateSnapshot{}
	}
	return p.snapshots[orgID]
}

func (p tenantResourceStateProvider) UnifiedResourceSnapshotForTenant(orgID string) ([]unified.Resource, time.Time) {
	snapshot := p.GetStateForTenant(orgID)
	if snapshot.LastUpdate.IsZero() {
		return nil, time.Time{}
	}

	return []unified.Resource{
		{
			ID:        "agent-tenant-seeded",
			Type:      unified.ResourceTypeAgent,
			Name:      "tenant-seeded",
			Status:    unified.StatusOnline,
			LastSeen:  snapshot.LastUpdate,
			UpdatedAt: snapshot.LastUpdate,
			Sources:   []unified.DataSource{unified.SourceAgent},
			Identity: unified.ResourceIdentity{
				Hostnames: []string{"tenant-seeded"},
			},
		},
	}, snapshot.LastUpdate
}

func TestResourceHandlers_NonDefaultOrgRequiresTenantStateProvider(t *testing.T) {
	now := time.Now().UTC()
	h := NewResourceHandlers(&config.Config{DataPath: t.TempDir()})
	h.SetStateProvider(resourceStateProvider{snapshot: models.StateSnapshot{
		Hosts: []models.Host{{ID: "host-default", Hostname: "default", Status: "online", LastSeen: now}},
	}})

	req := httptest.NewRequest(http.MethodGet, "/api/resources", nil)
	req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, "acme"))
	rec := httptest.NewRecorder()

	h.HandleListResources(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Internal server error") {
		t.Fatalf("expected internal server error body, got %q", rec.Body.String())
	}
}

func TestResourceHandlers_NonDefaultOrgUsesTenantStateProvider(t *testing.T) {
	now := time.Now().UTC()
	h := NewResourceHandlers(&config.Config{DataPath: t.TempDir()})
	h.SetStateProvider(resourceStateProvider{snapshot: models.StateSnapshot{
		Hosts: []models.Host{{ID: "host-default", Hostname: "default", Status: "online", LastSeen: now}},
	}})
	h.SetTenantStateProvider(tenantResourceStateProvider{snapshots: map[string]models.StateSnapshot{
		"acme": {
			Hosts: []models.Host{{ID: "host-tenant", Hostname: "tenant", Status: "online", LastSeen: now}},
		},
	}})

	req := httptest.NewRequest(http.MethodGet, "/api/resources?type=agent", nil)
	req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, "acme"))
	rec := httptest.NewRecorder()

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
	found := false
	for _, h := range resp.Data[0].Identity.Hostnames {
		if h == "tenant" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected tenant hostname in resource identity, got %+v", resp.Data[0].Identity)
	}
}

func TestResourceHandlers_NonDefaultOrgUsesTenantUnifiedSeedProvider(t *testing.T) {
	now := time.Now().UTC()
	h := NewResourceHandlers(&config.Config{DataPath: t.TempDir()})
	h.SetTenantStateProvider(tenantResourceStateProvider{snapshots: map[string]models.StateSnapshot{
		"acme": {LastUpdate: now},
	}})

	req := httptest.NewRequest(http.MethodGet, "/api/resources?type=agent", nil)
	req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, "acme"))
	rec := httptest.NewRecorder()

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
	if got := resp.Data[0].Name; got != "tenant-seeded" {
		t.Fatalf("resource name = %q, want tenant-seeded", got)
	}
}
