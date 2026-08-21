package resourceapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/api/apicontext"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	unifiedresources "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestPruneResourceForListResponseDoesNotMutateSharedPMG(t *testing.T) {
	sharedPMG := &unifiedresources.PMGData{
		InstanceID:      "pmg-1",
		RelayDomains:    []unifiedresources.PMGRelayDomainMeta{{Domain: "example.com"}},
		DomainStatsAsOf: time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC),
	}
	resource := unifiedresources.Resource{ID: "pmg-resource", PMG: sharedPMG}

	pruneResourceForListResponse(&resource)

	if resource.PMG == sharedPMG {
		t.Fatal("expected prune to clone the PMG struct before clearing it")
	}
	if resource.PMG.RelayDomains != nil || !resource.PMG.DomainStatsAsOf.IsZero() {
		t.Fatal("expected the pruned copy to be cleared")
	}
	if len(sharedPMG.RelayDomains) != 1 || sharedPMG.DomainStatsAsOf.IsZero() {
		t.Fatal("expected the shared PMG struct to remain untouched")
	}
}

func TestSharedResourceListsCachedPerGenerationAndImmuneToRequestDecoration(t *testing.T) {
	now := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	h := NewQueryService(&config.Config{DataPath: t.TempDir()})
	h.SetStateProvider(resourceUnifiedSeedProvider{
		snapshot: models.StateSnapshot{LastUpdate: now},
		resources: []unifiedresources.Resource{
			{
				ID:       "vm-1",
				Type:     unifiedresources.ResourceTypeVM,
				Name:     "worker",
				Status:   unifiedresources.StatusOnline,
				LastSeen: now,
				Sources:  []unifiedresources.DataSource{unifiedresources.SourceProxmox},
			},
			{
				ID:       "pmg-1",
				Type:     unifiedresources.ResourceTypePMG,
				Name:     "mailgw",
				Status:   unifiedresources.StatusOnline,
				LastSeen: now,
				Sources:  []unifiedresources.DataSource{unifiedresources.SourceProxmox},
				PMG: &unifiedresources.PMGData{
					InstanceID:   "pmg-1",
					RelayDomains: []unifiedresources.PMGRelayDomainMeta{{Domain: "example.com"}},
				},
			},
		},
	})

	orgID := ""
	first, _, err := h.sharedPresentationResources(orgID)
	if err != nil {
		t.Fatalf("sharedPresentationResources: %v", err)
	}
	if len(first) == 0 {
		t.Fatal("expected seeded resources")
	}
	second, _, err := h.sharedPresentationResources(orgID)
	if err != nil {
		t.Fatalf("sharedPresentationResources: %v", err)
	}
	if &first[0] != &second[0] {
		t.Fatal("expected the cached shared list for an unchanged registry generation")
	}

	// A full list request prunes PMG detail in its response; the shared cache
	// must keep the untouched data for later consumers.
	req := httptest.NewRequest(http.MethodGet, "/api/resources", nil)
	req = req.WithContext(context.WithValue(req.Context(), apicontext.OrgIDContextKey, orgID))
	rec := httptest.NewRecorder()
	h.HandleListResources(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}

	after, _, err := h.sharedPresentationResources(orgID)
	if err != nil {
		t.Fatalf("sharedPresentationResources: %v", err)
	}
	if &first[0] != &after[0] {
		t.Fatal("expected the request to serve from the cached list, not rebuild it")
	}
	foundPMG := false
	for i := range after {
		if after[i].PMG == nil {
			continue
		}
		foundPMG = true
		if len(after[i].PMG.RelayDomains) != 1 {
			t.Fatal("expected the shared cache to retain PMG relay domains after a list request pruned its response copy")
		}
	}
	if !foundPMG {
		t.Fatal("expected a PMG-bearing resource in the shared list")
	}

	rawFirst, _, err := h.sharedRawResources(orgID)
	if err != nil {
		t.Fatalf("sharedRawResources: %v", err)
	}
	rawSecond, _, err := h.sharedRawResources(orgID)
	if err != nil {
		t.Fatalf("sharedRawResources: %v", err)
	}
	if len(rawFirst) == 0 || &rawFirst[0] != &rawSecond[0] {
		t.Fatal("expected the cached shared raw list for an unchanged registry generation")
	}
}
