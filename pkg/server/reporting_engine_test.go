package server

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/pkg/reporting"
)

func TestOrgScopedReportEngineResolvesTheRequestOrganizationsStore(t *testing.T) {
	baseDir := t.TempDir()
	mtm := monitoring.NewMultiTenantMonitor(&config.Config{DataPath: baseDir}, config.NewMultiTenantPersistence(baseDir), nil)
	t.Cleanup(mtm.Stop)
	monitor, err := mtm.GetMonitor("default")
	if err != nil {
		t.Fatalf("GetMonitor(default): %v", err)
	}
	store := monitor.GetMetricsStore()
	if store == nil {
		t.Fatal("default monitor has no metrics store")
	}
	now := time.Now()
	store.Write("node", "node-1", "cpu", 42, now.Add(-10*time.Minute))
	store.Flush()

	engine := newOrgScopedReportEngine(func() *monitoring.MultiTenantMonitor { return mtm })
	request := func(orgID string) reporting.MetricReportRequest {
		return reporting.MetricReportRequest{
			OrgID:        orgID,
			ResourceType: "node",
			ResourceID:   "node-1",
			Start:        now.Add(-time.Hour),
			End:          now.Add(time.Minute),
		}
	}

	evidence, err := engine.MetricEvidenceFor(context.Background(), request("default"))
	if err != nil {
		t.Fatalf("default organization evidence: %v", err)
	}
	if stats := evidence.Metrics["cpu"]; stats.RetainedPoints == 0 || stats.Latest != 42 {
		t.Fatalf("default organization evidence = %+v, want its own store's cpu reading", evidence.Metrics)
	}

	if _, err := engine.MetricEvidenceFor(context.Background(), request("")); !errors.Is(err, reporting.ErrReportOrgRequired) {
		t.Fatalf("request without organization: got %v, want ErrReportOrgRequired", err)
	}
	if _, err := engine.MetricEvidenceFor(context.Background(), request("org-not-provisioned")); err == nil {
		t.Fatal("an unprovisioned organization must not fall back to the default store")
	}

	stopped := newOrgScopedReportEngine(func() *monitoring.MultiTenantMonitor { return nil })
	if _, err := stopped.MetricEvidenceFor(context.Background(), request("default")); err == nil {
		t.Fatal("an engine without a running monitor must refuse requests")
	}
}
