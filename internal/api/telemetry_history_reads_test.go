package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/testutil"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
	"github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
)

func TestMetricsHistoryTelemetryRequiresServedExtendedData(t *testing.T) {
	testutil.SetMockMode(t, false)
	for _, tc := range []struct {
		name, query string
		age         time.Duration
		licensed    bool
		want        int
	}{
		{"old single metric", "&metric=cpu&range=30d", 8 * 24 * time.Hour, true, 1},
		{"old all metrics", "&range=90d", 8 * 24 * time.Hour, true, 1},
		{"recent data in long range", "&metric=cpu&range=30d", 24 * time.Hour, true, 0},
		{"denied range", "&metric=cpu&range=30d", 8 * 24 * time.Hour, false, 0},
		{"empty long range", "&metric=missing&range=30d", 8 * 24 * time.Hour, true, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			monitor, _, _ := newTestMonitor(t)
			store, err := metrics.NewStore(metrics.DefaultConfig(t.TempDir()))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = store.Close() })
			store.WriteBatchSync([]metrics.WriteMetric{{ResourceType: "vm", ResourceID: "private-resource", MetricType: "cpu", Value: 42, Timestamp: time.Now().Add(-tc.age), Tier: metrics.TierDaily}})
			setUnexportedField(t, monitor, "metricsStore", store)
			persistence := config.NewConfigPersistence(t.TempDir())
			handlers := NewLicenseHandlers(config.NewMultiTenantPersistence(t.TempDir()), false)
			if tc.licensed {
				setMetricsHistoryLicenseTier(t, handlers, pkglicensing.TierPro)
			}
			router := &Router{monitor: monitor, persistence: persistence, licenseHandlers: handlers}
			rec := httptest.NewRecorder()
			router.handleMetricsHistory(rec, httptest.NewRequest(http.MethodGet, "/api/metrics-store/history?resourceType=vm&resourceId=private-resource"+tc.query, nil))
			if tc.licensed && rec.Code != http.StatusOK {
				t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
			}
			now := time.Now().UTC()
			if got := persistence.CountSuccessfulExtendedHistoryReadsSince(now.AddDate(0, 0, -30), now); got != tc.want {
				t.Fatalf("reads = %d, want %d; response %s", got, tc.want, rec.Body.String())
			}
		})
	}
}
