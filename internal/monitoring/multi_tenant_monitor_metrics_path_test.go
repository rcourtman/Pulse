package monitoring

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
)

// PULSE_METRICS_DB_PATH names a single database file. Only the default org may
// use it; every other org must keep its own metrics.db under its data
// directory so organizations never read each other's series.
func TestMultiTenantMonitorMetricsDBPathOverrideStaysOnDefaultOrg(t *testing.T) {
	mtp, defaultPersistence := newTestTenantPersistence(t)
	overridePath := filepath.Join(t.TempDir(), "metrics-tmpfs", "metrics.db")
	baseCfg := &config.Config{
		DataPath:      defaultPersistence.GetConfigDir(),
		MetricsDBPath: overridePath,
	}
	mtm := NewMultiTenantMonitor(baseCfg, mtp, nil)
	t.Cleanup(mtm.Stop)

	orgIDs := []string{"org-a", "org-b"}
	for _, orgID := range orgIDs {
		if err := mtp.SaveOrganization(&models.Organization{ID: orgID, DisplayName: orgID}); err != nil {
			t.Fatalf("SaveOrganization(%s) error = %v", orgID, err)
		}
	}

	stores := make(map[string]*metrics.Store)
	for _, orgID := range append([]string{"default"}, orgIDs...) {
		monitor, err := mtm.GetMonitor(orgID)
		if err != nil {
			t.Fatalf("GetMonitor(%s) error = %v", orgID, err)
		}
		store := monitor.GetMetricsStore()
		if store == nil {
			t.Fatalf("GetMonitor(%s) has no metrics store", orgID)
		}
		stores[orgID] = store
	}

	if baseCfg.MetricsDBPath != overridePath {
		t.Fatalf("base config MetricsDBPath = %q, want %q", baseCfg.MetricsDBPath, overridePath)
	}
	if _, err := os.Stat(overridePath); err != nil {
		t.Fatalf("default org should open the override path %q: %v", overridePath, err)
	}
	for _, orgID := range orgIDs {
		orgPersistence, err := mtp.GetPersistence(orgID)
		if err != nil {
			t.Fatalf("GetPersistence(%s) error = %v", orgID, err)
		}
		orgDBPath := filepath.Join(orgPersistence.GetConfigDir(), "metrics.db")
		if _, err := os.Stat(orgDBPath); err != nil {
			t.Fatalf("%s should open its own metrics database at %q: %v", orgID, orgDBPath, err)
		}
	}
	if stores["org-a"] == stores["org-b"] || stores["org-a"] == stores["default"] {
		t.Fatal("expected a distinct metrics store per organization")
	}

	now := time.Now().Truncate(time.Second)
	for _, orgID := range []string{"default", "org-a"} {
		stores[orgID].WriteBatchSync([]metrics.WriteMetric{{
			ResourceType: "vm",
			ResourceID:   "vm-" + orgID,
			MetricType:   "cpu",
			Value:        42,
			Timestamp:    now,
			Tier:         metrics.TierRaw,
		}})
	}

	for _, tc := range []struct {
		orgID      string
		resourceID string
		wantPoints bool
	}{
		{orgID: "default", resourceID: "vm-default", wantPoints: true},
		{orgID: "default", resourceID: "vm-org-a", wantPoints: false},
		{orgID: "org-a", resourceID: "vm-org-a", wantPoints: true},
		{orgID: "org-a", resourceID: "vm-default", wantPoints: false},
		{orgID: "org-b", resourceID: "vm-org-a", wantPoints: false},
		{orgID: "org-b", resourceID: "vm-default", wantPoints: false},
	} {
		points, err := stores[tc.orgID].Query("vm", tc.resourceID, "cpu", now.Add(-time.Minute), now.Add(time.Minute), 0)
		if err != nil {
			t.Fatalf("%s Query(%s) error = %v", tc.orgID, tc.resourceID, err)
		}
		if got := len(points) > 0; got != tc.wantPoints {
			t.Fatalf("%s store points for %s = %d, want present=%v", tc.orgID, tc.resourceID, len(points), tc.wantPoints)
		}
	}
}
