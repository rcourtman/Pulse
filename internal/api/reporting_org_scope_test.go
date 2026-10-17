package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
	"github.com/rcourtman/pulse-go-rewrite/pkg/reporting"
)

// Org A owns node-a and org B owns node-b. Org A's metrics store also holds
// points under node-b, so a report that reads the wrong organization's store
// shows org A's value (11) instead of org B's (77).
const (
	orgScopeValueA = "11.00"
	orgScopeValueB = "77.00"
)

func newReportingOrgScopeFixture(t *testing.T) *ReportingHandlers {
	t.Helper()
	now := time.Now()
	stores := map[string]*metrics.Store{}
	for orgID, series := range map[string]map[string]float64{
		"org-a": {"node-a": 11, "node-b": 11},
		"org-b": {"node-b": 77},
	} {
		store, err := metrics.NewStore(metrics.DefaultConfig(t.TempDir()))
		if err != nil {
			t.Fatalf("metrics store for %s: %v", orgID, err)
		}
		t.Cleanup(func() { store.Close() })
		// Every tier holds the points, so whichever tier a report window
		// selects finds them.
		for resourceID, value := range series {
			for _, tier := range []metrics.Tier{metrics.TierRaw, metrics.TierMinute, metrics.TierHourly, metrics.TierDaily} {
				for i := 0; i < 6; i++ {
					store.WriteWithTier("node", resourceID, "cpu", value, now.Add(time.Duration(-50+i*5)*time.Minute), tier)
				}
			}
		}
		store.Flush()
		stores[orgID] = store
	}

	original := reporting.GetEngine()
	reporting.SetEngine(reporting.NewReportEngine(reporting.EngineConfig{
		MetricsStoreForOrg: func(orgID string) (*metrics.Store, error) {
			store, ok := stores[orgID]
			if !ok {
				return nil, fmt.Errorf("organization %q is not provisioned", orgID)
			}
			return store, nil
		},
	}))
	t.Cleanup(func() { reporting.SetEngine(original) })

	stateA := models.NewState()
	stateA.Nodes = []models.Node{{ID: "node-a", Name: "pve-a", Status: "online"}}
	stateB := models.NewState()
	stateB.Nodes = []models.Node{{ID: "node-b", Name: "pve-b", Status: "online"}}

	mtm := monitoring.NewMultiTenantMonitor(&config.Config{}, nil, nil)
	setUnexportedField(t, mtm, "monitors", map[string]*monitoring.Monitor{
		"org-a": newReportingMonitorForTest(t, stateA, nil),
		"org-b": newReportingMonitorForTest(t, stateB, nil),
	})
	return NewReportingHandlers(mtm, nil)
}

func orgScopeContext(orgID string) context.Context {
	return context.WithValue(context.Background(), OrgIDContextKey, orgID)
}

func orgScopeUnifiedNodeID(t *testing.T, handlers *ReportingHandlers, orgID string) string {
	t.Helper()
	snapshot, ok := handlers.getReportingEnrichmentSnapshot(context.Background(), orgID)
	if !ok {
		t.Fatalf("no inventory snapshot for %s", orgID)
	}
	for _, resource := range snapshot.Resources {
		if reporting.CanonicalResourceType(string(resource.Type)) != "" && resource.Name != "" {
			return resource.ID
		}
	}
	t.Fatalf("no unified resource in %s inventory: %+v", orgID, snapshot.Resources)
	return ""
}

func TestReportingSingleReportReadsOnlyTheRequestOrganizationsMetrics(t *testing.T) {
	handlers := newReportingOrgScopeFixture(t)
	unifiedA := orgScopeUnifiedNodeID(t, handlers, "org-a")

	generate := func(orgID, method, resourceID string) *httptest.ResponseRecorder {
		var req *http.Request
		if method == http.MethodGet {
			req = httptest.NewRequest(http.MethodGet, "/api/admin/reports/generate?format=csv&resourceType=node&metricType=cpu&resourceId="+resourceID, nil)
		} else {
			body := fmt.Sprintf(`{"format":"csv","resourceType":"node","metricType":"cpu","resourceId":%q}`, resourceID)
			req = httptest.NewRequest(http.MethodPost, "/api/admin/reports/generate", strings.NewReader(body))
		}
		rec := httptest.NewRecorder()
		handlers.HandleGenerateReport(rec, req.WithContext(orgScopeContext(orgID)))
		return rec
	}

	for _, method := range []string{http.MethodGet, http.MethodPost} {
		// Org A's resource, by legacy and unified ID, is not reportable from org B.
		for _, foreignID := range []string{"node-a", unifiedA} {
			rec := generate("org-b", method, foreignID)
			if rec.Code != http.StatusNotFound || strings.Contains(rec.Body.String(), orgScopeValueA) {
				t.Fatalf("%s org B report on org A's %s = %d %q, want 404 without org A's metrics", method, foreignID, rec.Code, rec.Body.String())
			}
		}

		// Org B's own resource reads org B's store, although org A's store
		// holds points under the same ID.
		rec := generate("org-b", method, "node-b")
		if rec.Code != http.StatusOK {
			t.Fatalf("%s org B report on node-b = %d %q", method, rec.Code, rec.Body.String())
		}
		if body := rec.Body.String(); !strings.Contains(body, orgScopeValueB) || strings.Contains(body, orgScopeValueA) {
			t.Fatalf("%s org B report on node-b must carry only org B's metrics:\n%s", method, body)
		}

		// Org A still reports on its own resource.
		rec = generate("org-a", method, "node-a")
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), orgScopeValueA) {
			t.Fatalf("%s org A report on node-a = %d %q", method, rec.Code, rec.Body.String())
		}
	}
}

func TestReportingMultiReportReadsOnlyTheRequestOrganizationsMetrics(t *testing.T) {
	handlers := newReportingOrgScopeFixture(t)
	unifiedA := orgScopeUnifiedNodeID(t, handlers, "org-a")

	generate := func(orgID string, resourceIDs ...string) *httptest.ResponseRecorder {
		resources := make([]multiReportResourceEntry, 0, len(resourceIDs))
		for _, id := range resourceIDs {
			resources = append(resources, multiReportResourceEntry{ResourceType: "node", ResourceID: id})
		}
		body, err := json.Marshal(multiReportRequestBody{Resources: resources, Format: "csv", MetricType: "cpu"})
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPost, "/api/admin/reports/generate-multi", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		handlers.HandleGenerateMultiReport(rec, req.WithContext(orgScopeContext(orgID)))
		return rec
	}

	for _, foreignID := range []string{"node-a", unifiedA} {
		rec := generate("org-b", "node-b", foreignID)
		if rec.Code != http.StatusNotFound || strings.Contains(rec.Body.String(), orgScopeValueA) {
			t.Fatalf("org B fleet report including org A's %s = %d %q, want 404 without org A's metrics", foreignID, rec.Code, rec.Body.String())
		}
	}

	rec := generate("org-b", "node-b")
	if rec.Code != http.StatusOK {
		t.Fatalf("org B fleet report = %d %q", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); !strings.Contains(body, orgScopeValueB) || strings.Contains(body, orgScopeValueA) {
		t.Fatalf("org B fleet report must carry only org B's metrics:\n%s", body)
	}
}
