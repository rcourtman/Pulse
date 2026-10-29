package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
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

// The setup wizard leaves the Pulse UI authenticated with the wildcard admin
// token, so its own node and finding fetches must not read as external agent
// use. Requests without a same-origin fetch marker still count.
func TestExternalAgentActivityIgnoresFirstPartyBrowserRequests(t *testing.T) {
	persistence := config.NewConfigPersistence(t.TempDir())
	router := &Router{persistence: persistence}
	adminToken := &config.APITokenRecord{Scopes: []string{config.ScopeWildcard}}

	for _, site := range []string{"same-origin", "Same-Origin"} {
		uiReq := httptest.NewRequest(http.MethodGet, "/api/config/nodes", nil)
		uiReq.Header.Set("Sec-Fetch-Site", site)
		attachAPITokenRecord(uiReq, adminToken)
		router.recordExternalAgentCapabilityActivity(uiReq, agentcapabilities.ListNodesCapabilityName)
	}

	for _, site := range []string{"", "cross-site", "same-site", "none"} {
		req := httptest.NewRequest(http.MethodGet, "/api/ai/patrol/findings", nil)
		if site != "" {
			req.Header.Set("Sec-Fetch-Site", site)
		}
		attachAPITokenRecord(req, adminToken)
		router.recordExternalAgentCapabilityActivity(req, agentcapabilities.ListFindingsCapabilityName)
	}

	history, err := persistence.LoadExternalAgentActivityHistory()
	if err != nil {
		t.Fatalf("LoadExternalAgentActivityHistory: %v", err)
	}
	if len(history.Events) != 4 {
		t.Fatalf("expected only the four non-first-party requests to count, got %d: %+v", len(history.Events), history.Events)
	}
	for _, event := range history.Events {
		if event.Activity != config.ExternalAgentActivityFindingList {
			t.Fatalf("first-party UI request was counted: %+v", event)
		}
	}
}
