package resourceapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

type pageWorkChecker struct {
	checked []string
	refused bool
}

func (c *pageWorkChecker) CheckActionAvailable(_ context.Context, req unified.ActionRequest, _ unified.Resource) unified.ResourceActionReadiness {
	c.checked = append(c.checked, req.ResourceID)
	return unified.ResourceActionReadiness{Name: req.CapabilityName, Available: !c.refused, ReasonCode: "command_agent_disconnected"}
}

type pageWorkProvider struct{ resourceUnifiedSeedProvider }

func (p *pageWorkProvider) ReadSnapshot() models.StateSnapshot { return p.snapshot }

// Returned rows keep current health and action decisions even when an unchanged
// registry generation can reuse its structural counts and governance summary.
func TestResourceListDecoratesOnlyPageWithCurrentHealthAndReadiness(t *testing.T) {
	now := time.Now().UTC()
	provider := &pageWorkProvider{resourceUnifiedSeedProvider{snapshot: models.StateSnapshot{LastUpdate: now}}}
	for i := 0; i < 12; i++ {
		provider.resources = append(provider.resources, unified.Resource{
			ID: fmt.Sprintf("vm-%02d", i), Name: fmt.Sprintf("vm-%02d", i), Type: unified.ResourceTypeVM,
			Status: unified.StatusOnline, LastSeen: now, Sources: []unified.DataSource{unified.SourceProxmox},
			Capabilities: []unified.ResourceCapability{{Name: "start"}},
		})
	}
	checker := &pageWorkChecker{}
	h := NewQueryService(&config.Config{DataPath: t.TempDir()})
	defer h.CloseStores()
	h.SetStateProvider(provider)
	h.SetActionAvailabilityChecker(checker)
	read := func(path string) ResourcesResponse {
		t.Helper()
		rec := httptest.NewRecorder()
		h.HandleListResources(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		var resp ResourcesResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatal(err)
		}
		return resp
	}
	first := read("/api/resources?limit=1&page=2")
	if len(first.Data) != 1 || first.Data[0].ID != "vm-01" {
		t.Fatalf("wrong page: %+v", first.Meta)
	}
	if !reflect.DeepEqual(checker.checked, []string{"vm-01"}) {
		t.Fatalf("projected %v, want only returned row", checker.checked)
	}
	if first.Data[0].Health == nil || first.Data[0].Health.Verdict != unified.HealthOK || len(first.Data[0].Capabilities) != 1 {
		t.Fatal("lost initial health/capability")
	}
	checker.checked = nil
	checker.refused = true
	provider.snapshot.ActiveAlerts = []models.Alert{{ResourceID: "vm-01", Level: "critical", Type: "cpu"}}
	second := read("/api/resources?limit=1&page=2")
	if !reflect.DeepEqual(first.Aggregations, second.Aggregations) {
		t.Fatal("live decoration changed structural aggregation")
	}
	if second.Data[0].Health == nil || second.Data[0].Health.Verdict != unified.HealthCritical || len(second.Data[0].Capabilities) != 0 || len(second.Data[0].ActionReadiness) != 1 {
		t.Fatalf("cached generation hid live change: %+v", second.Data[0])
	}
	if !reflect.DeepEqual(checker.checked, []string{"vm-01"}) {
		t.Fatalf("projected %v, want only returned row", checker.checked)
	}
	checker.checked = nil
	checker.refused = false
	provider.snapshot.ActiveAlerts = nil
	third := read("/api/resources?limit=1&page=2")
	if third.Data[0].Health.Verdict != unified.HealthOK || len(third.Data[0].Capabilities) != 1 || len(third.Data[0].ActionReadiness) != 0 {
		t.Fatal("request decoration mutated the shared resource")
	}
	checker.checked = nil
	empty := read("/api/resources?page=100&limit=1")
	if len(empty.Data) != 0 || len(checker.checked) != 0 || empty.Meta.Total != 12 || empty.Aggregations.Total != 12 {
		t.Fatal("empty page lost counts or projected hidden rows")
	}
}

func TestResourceListAggregationRefreshesWithPresentationGeneration(t *testing.T) {
	now := time.Now().UTC()
	provider := &pageWorkProvider{resourceUnifiedSeedProvider{snapshot: models.StateSnapshot{LastUpdate: now}, resources: []unified.Resource{{ID: "vm-a", Name: "alpha", Type: unified.ResourceTypeVM, Status: unified.StatusOnline, LastSeen: now, Sources: []unified.DataSource{unified.SourceProxmox}, Tags: []string{"public"}}}}}
	h := NewQueryService(&config.Config{DataPath: t.TempDir()})
	defer h.CloseStores()
	h.SetStateProvider(provider)
	read := func() ResourcesResponse {
		t.Helper()
		rec := httptest.NewRecorder()
		h.HandleListResources(rec, httptest.NewRequest(http.MethodGet, "/api/resources?limit=1", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		var response ResourcesResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		return response
	}
	first := read()
	if first.Aggregations.PolicyPosture.SensitivityCounts[unified.ResourceSensitivityPublic] != 1 {
		t.Fatal("missing original policy")
	}
	provider.resources[0].Tags = []string{"restricted"}
	provider.resources[0].Status = unified.StatusOffline
	provider.snapshot.LastUpdate = now.Add(time.Second)
	second := read()
	if second.Aggregations.ByStatus[unified.StatusOffline] != 1 || second.Aggregations.PolicyPosture.SensitivityCounts[unified.ResourceSensitivityRestricted] != 1 || second.Aggregations.PolicyPosture.SensitivityCounts[unified.ResourceSensitivityPublic] != 0 {
		t.Fatal("reused aggregation from previous registry generation")
	}
	// Explicit invalidation matters for links/supplemental records whose change
	// need not advance the monitoring generation marker.
	provider.resources[0].Tags = []string{"sensitive"}
	h.InvalidateCache("")
	third := read()
	if third.Aggregations.PolicyPosture.SensitivityCounts[unified.ResourceSensitivitySensitive] != 1 || third.Aggregations.PolicyPosture.SensitivityCounts[unified.ResourceSensitivityRestricted] != 0 {
		t.Fatal("explicit invalidation retained old aggregation")
	}
	rec := httptest.NewRecorder()
	h.HandleStats(rec, httptest.NewRequest(http.MethodGet, "/api/resources/stats", nil))
	var stats unified.ResourceStats
	if err := json.Unmarshal(rec.Body.Bytes(), &stats); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK || !reflect.DeepEqual(stats, third.Aggregations) {
		t.Fatal("stats endpoint disagrees with list")
	}
}
