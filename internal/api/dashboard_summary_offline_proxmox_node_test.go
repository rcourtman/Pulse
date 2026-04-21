package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestDashboardSummaryKeepsOfflineProxmoxNodeVisible(t *testing.T) {
	now := time.Now().UTC()

	h := NewResourceHandlers(&config.Config{DataPath: t.TempDir()})
	h.SetStateProvider(resourceUnifiedSeedProvider{
		snapshot: models.StateSnapshot{LastUpdate: now},
		resources: []unified.Resource{
			{
				ID:        "offline-pve",
				Type:      unified.ResourceTypeAgent,
				Name:      "PVE Offline",
				Status:    unified.StatusOffline,
				LastSeen:  now,
				UpdatedAt: now,
				Sources:   []unified.DataSource{unified.SourceProxmox},
				Canonical: &unified.CanonicalIdentity{
					DisplayName: "PVE Offline",
					Hostname:    "pve-offline.lab.local",
					PlatformID:  "offline-pve",
				},
				Proxmox: &unified.ProxmoxData{
					SourceID:         "offline-pve",
					NodeName:         "pve-offline",
					Instance:         "homelab",
					ConnectionHealth: "error",
					PVEVersion:       "8.3-1",
				},
			},
		},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/resources/dashboard-summary", nil)
	h.HandleDashboardSummary(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp DashboardOverviewSummaryResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Health.TotalResources != 1 {
		t.Fatalf("health.totalResources = %d, want 1", resp.Health.TotalResources)
	}
	if resp.Infrastructure.Total != 1 {
		t.Fatalf("infrastructure.total = %d, want 1", resp.Infrastructure.Total)
	}
	if resp.Infrastructure.ByStatus["offline"] != 1 {
		t.Fatalf("infrastructure.byStatus.offline = %d, want 1", resp.Infrastructure.ByStatus["offline"])
	}
	if resp.Infrastructure.ByType["agent"] != 1 {
		t.Fatalf("infrastructure.byType.agent = %d, want 1", resp.Infrastructure.ByType["agent"])
	}
	if len(resp.ProblemResources) != 1 {
		t.Fatalf("problemResources len = %d, want 1", len(resp.ProblemResources))
	}
	if resp.ProblemResources[0].ID != "offline-pve" {
		t.Fatalf("problemResources[0].id = %q, want offline-pve", resp.ProblemResources[0].ID)
	}
	if got := resp.ProblemResources[0].Problems; len(got) != 1 || got[0] != "Offline" {
		t.Fatalf("problemResources[0].problems = %+v, want [Offline]", got)
	}
}
