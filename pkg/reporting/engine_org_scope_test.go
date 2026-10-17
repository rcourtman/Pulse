package reporting

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
)

func newOrgScopeTestStore(t *testing.T, cpu float64, now time.Time) *metrics.Store {
	t.Helper()
	store, err := metrics.NewStore(metrics.StoreConfig{
		DBPath:          filepath.Join(t.TempDir(), "metrics.db"),
		WriteBufferSize: 10,
		FlushInterval:   100 * time.Millisecond,
		RetentionRaw:    24 * time.Hour,
		RetentionMinute: 7 * 24 * time.Hour,
		RetentionHourly: 30 * 24 * time.Hour,
		RetentionDaily:  90 * 24 * time.Hour,
	})
	if err != nil {
		t.Fatalf("create metrics store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	// Both organizations record the same resource ID, so only the store a
	// query lands on decides which value comes back.
	for i := 0; i < 6; i++ {
		store.Write("node", "shared-node", "cpu", cpu, now.Add(time.Duration(-30+i*5)*time.Minute))
	}
	store.Flush()
	return store
}

func newOrgScopedTestEngine(t *testing.T, now time.Time) *ReportEngine {
	t.Helper()
	stores := map[string]*metrics.Store{
		"org-a": newOrgScopeTestStore(t, 11, now),
		"org-b": newOrgScopeTestStore(t, 77, now),
	}
	return NewReportEngine(EngineConfig{
		MetricsStoreForOrg: func(orgID string) (*metrics.Store, error) {
			store, ok := stores[orgID]
			if !ok {
				return nil, fmt.Errorf("organization %q is not provisioned", orgID)
			}
			return store, nil
		},
	})
}

func orgScopeTestRequest(orgID string, now time.Time) MetricReportRequest {
	return MetricReportRequest{
		OrgID:        orgID,
		ResourceType: "node",
		ResourceID:   "shared-node",
		MetricType:   "cpu",
		Start:        now.Add(-time.Hour),
		End:          now.Add(time.Minute),
		Format:       FormatCSV,
	}
}

func TestReportEngineRefusesRequestsWithoutAnOrganization(t *testing.T) {
	now := time.Now()
	engine := newOrgScopedTestEngine(t, now)

	req := orgScopeTestRequest("", now)
	if _, _, err := engine.Generate(req); !errors.Is(err, ErrReportOrgRequired) {
		t.Fatalf("Generate without org: got %v, want ErrReportOrgRequired", err)
	}
	if _, err := engine.NarrativeFor(req); !errors.Is(err, ErrReportOrgRequired) {
		t.Fatalf("NarrativeFor without org: got %v, want ErrReportOrgRequired", err)
	}
	if _, err := engine.MetricEvidenceFor(context.Background(), req); !errors.Is(err, ErrReportOrgRequired) {
		t.Fatalf("MetricEvidenceFor without org: got %v, want ErrReportOrgRequired", err)
	}
	multi := MultiReportRequest{
		Resources: []MetricReportRequest{orgScopeTestRequest("org-a", now)},
		Start:     req.Start,
		End:       req.End,
		Format:    FormatCSV,
	}
	if _, _, err := engine.GenerateMulti(multi); !errors.Is(err, ErrReportOrgRequired) {
		t.Fatalf("GenerateMulti without org: got %v, want ErrReportOrgRequired", err)
	}
	if _, err := engine.FleetNarrativeFor(multi); !errors.Is(err, ErrReportOrgRequired) {
		t.Fatalf("FleetNarrativeFor without org: got %v, want ErrReportOrgRequired", err)
	}
	if _, _, err := engine.Generate(orgScopeTestRequest("org-unknown", now)); err == nil {
		t.Fatal("Generate for an unprovisioned organization should fail")
	}
}

func TestReportEngineMultiReportUsesTheRequestOrganizationForEveryResource(t *testing.T) {
	now := time.Now()
	engine := newOrgScopedTestEngine(t, now)

	// A resource entry naming another organization must not redirect its
	// query: the multi-report's organization applies to all of them.
	resource := orgScopeTestRequest("org-a", now)
	data, _, err := engine.GenerateMulti(MultiReportRequest{
		OrgID:      "org-b",
		Resources:  []MetricReportRequest{resource},
		Start:      resource.Start,
		End:        resource.End,
		MetricType: "cpu",
		Format:     FormatCSV,
	})
	if err != nil {
		t.Fatalf("GenerateMulti: %v", err)
	}
	csv := string(data)
	if !strings.Contains(csv, "77.00") || strings.Contains(csv, "11.00") {
		t.Fatalf("multi-report for org-b should carry only org-b's values, got:\n%s", csv)
	}
}
