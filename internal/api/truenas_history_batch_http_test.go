package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/truenas"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

type batchHTTPHistoryFetcher struct {
	truenas.FixtureFetcher
	onRead func()
	points []truenas.TimeSeriesPoint
	reads  atomic.Int32
}

func (f *batchHTTPHistoryFetcher) SystemMetricHistory(context.Context, time.Duration) (*truenas.SystemMetricHistory, error) {
	f.reads.Add(1)
	if f.onRead != nil {
		f.onRead()
	}
	return &truenas.SystemMetricHistory{CPUPercent: f.points}, nil
}

// Exercise the actual chart producer consumed by Machines. Inventory remains
// the last snapshot during the synthetic connection change; only the obsolete
// native history must withdraw. This is not installed removal or #2077 proof.
func TestTrueNASHistoryBatchHTTPWithdrawal(t *testing.T) {
	old := truenas.IsFeatureEnabled()
	truenas.SetFeatureEnabled(true)
	t.Cleanup(func() { truenas.SetFeatureEnabled(old) })
	oldMulti := IsMultiTenantEnabled()
	SetMultiTenantEnabled(true)
	t.Cleanup(func() { SetMultiTenantEnabled(oldMulti) })
	t.Setenv("PULSE_DEV", "true")
	now := time.Now().UTC().Truncate(time.Second)
	fetchers := map[string]*batchHTTPHistoryFetcher{}
	providers := map[string]*truenas.Provider{}
	records := map[string][]unifiedresources.IngestRecord{}
	for _, id := range []string{"batch-a", "batch-b"} {
		fixture := truenas.DefaultFixtures()
		fixture.System.Hostname = id
		fixture.CollectedAt = now
		fixture.Pools = nil
		fixture.Datasets = nil
		fixture.Disks = nil
		fixture.Alerts = nil
		fixture.Apps = nil
		fixture.VMs = nil
		fixture.Shares = nil
		fixture.Services = nil
		fixture.ZFSSnapshots = nil
		fixture.ReplicationTasks = nil
		points := []truenas.TimeSeriesPoint{{Timestamp: now.Add(-50 * time.Minute), Value: 11}, {Timestamp: now.Add(-25 * time.Minute), Value: 23}, {Timestamp: now.Add(-time.Minute), Value: 0}}
		if id == "batch-b" {
			points[0].Value = 37
			points[1].Value = 51
			points[2].Value = 19
		}
		f := &batchHTTPHistoryFetcher{FixtureFetcher: truenas.FixtureFetcher{Snapshot: fixture}, points: points}
		p := truenas.NewLiveProviderForConnection(f, id)
		if err := p.Refresh(context.Background()); err != nil {
			t.Fatal(err)
		}
		fetchers[id], providers[id], records[id] = f, p, p.Records()
	}
	poller := monitoring.NewTrueNASPoller(nil, time.Minute, nil)
	setUnexportedField(t, poller, "providersByOrg", map[string]map[string]*truenas.Provider{"org-batch": providers})
	setUnexportedField(t, poller, "cachedRecordsByOrg", map[string]map[string][]unifiedresources.IngestRecord{"org-batch": records})
	monitor := &monitoring.Monitor{}
	monitor.SetOrgID("org-batch")
	setUnexportedField(t, monitor, "state", models.NewState())
	setUnexportedField(t, monitor, "metricsHistory", monitoring.NewMetricsHistory(128, time.Hour))
	monitor.SetResourceStoreAndProviders(unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil)), map[unifiedresources.DataSource]monitoring.MonitorSupplementalRecordsProvider{unifiedresources.SourceTrueNAS: poller})
	// No local long-range history: the batch must exercise its native fallback.
	setUnexportedField(t, monitor, "metricsHistory", monitoring.NewMetricsHistory(128, time.Hour))
	const raw = "batch-reader.1234567890123456789"
	token := newTokenRecord(t, raw, []string{config.ScopeMonitoringRead}, nil)
	token.OrgID = "org-batch"
	cfg := newTestConfigWithTokens(t, token)
	cfg.AuthUser, cfg.AuthPass = "fixture-admin", "synthetic-unused-hash"
	if err := os.MkdirAll(filepath.Join(cfg.DataPath, "orgs", "org-batch"), 0700); err != nil {
		t.Fatal(err)
	}
	router := newTestRouter(t, cfg, monitor, nil, nil, nil, "test")
	t.Cleanup(router.ShutdownBackgroundWorkers)
	t.Cleanup(router.ShutdownResourceStores)
	t.Cleanup(router.ShutdownRBAC)
	mtm := monitoring.NewMultiTenantMonitor(cfg, nil, nil)
	setUnexportedField(t, mtm, "monitors", map[string]*monitoring.Monitor{"org-batch": monitor})
	router.mtMonitor = mtm
	router.resourceHandlers.SetTenantStateProvider(NewMultiTenantStateProvider(mtm, monitor))
	router.setMonitorSupplementalRecordsProvider(unifiedresources.SourceTrueNAS, poller)
	request := func(path, credential string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r.Header.Set("X-Pulse-Org-ID", "org-batch")
		if credential != "" {
			r.Header.Set("X-API-Token", credential)
		}
		w := httptest.NewRecorder()
		router.Handler().ServeHTTP(w, r)
		return w
	}
	for _, path := range []string{"/api/charts?range=1h", "/api/charts/infrastructure?range=1h"} {
		if got := request(path, ""); got.Code != 401 {
			t.Fatalf("anonymous chart %d", got.Code)
		}
	}
	if fetchers["batch-a"].reads.Load() != 0 || fetchers["batch-b"].reads.Load() != 0 {
		t.Fatal("anonymous chart reached native history")
	}
	resources := request("/api/resources?type=agent&source=truenas&limit=100", raw)
	if resources.Code != 200 {
		t.Fatal(resources.Body.String())
	}
	t.Logf("BATCH_RESOURCES %s", resources.Body.String())
	type chartReply struct {
		AgentData map[string]map[string][]struct {
			Timestamp int64   `json:"timestamp"`
			Value     float64 `json:"value"`
		} `json:"agentData"`
	}
	decode := func(w *httptest.ResponseRecorder) chartReply {
		var reply chartReply
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &reply) != nil {
			t.Fatalf("charts %d %s", w.Code, w.Body.String())
		}
		return reply
	}
	before := request("/api/charts?range=1h", raw)
	for id, series := range decode(before).AgentData {
		if len(series["cpu"]) != 3 {
			t.Fatalf("baseline native host %s: %+v", id, series)
		}
	}
	t.Logf("BATCH_BEFORE %s", before.Body.String())
	var calls int
	first := ""
	for _, id := range []string{"batch-a", "batch-b"} {
		id := id
		fetchers[id].onRead = func() {
			calls++
			if calls == 1 {
				first = id
			}
			if calls == 2 {
				delete(providers, first)
			}
		}
	}
	during := request("/api/charts/infrastructure?range=1h", raw)
	reply := decode(during)
	if calls != 2 || first == "" {
		t.Fatalf("batch did not read both native hosts: calls %d first %q", calls, first)
	}
	// Provider removal is synchronous inside the later read: no concurrent map
	// access or pointer mutation is needed to model the temporal ownership loss.
	for _, point := range reply.AgentData[first]["cpu"] {
		if point.Timestamp < now.Add(-2*time.Minute).UnixMilli() {
			t.Fatalf("withdrawn earlier native history escaped HTTP: %s", during.Body.String())
		}
	}
	survivor := "batch-a"
	if first == survivor {
		survivor = "batch-b"
	}
	if len(reply.AgentData[survivor]["cpu"]) != 3 {
		t.Fatalf("surviving native host lost history: %s", during.Body.String())
	}
	t.Logf("BATCH_WITHDRAWAL %s", during.Body.String())
	for _, f := range fetchers {
		f.onRead = nil
	}
	providers[first] = truenas.NewLiveProviderForConnection(fetchers[first], first)
	if err := providers[first].Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	after := request("/api/charts?range=1h", raw)
	if len(decode(after).AgentData[first]["cpu"]) != 3 {
		t.Fatal("fresh replacement read did not recover native history")
	}
	t.Logf("BATCH_AFTER %s", after.Body.String())
	t.Logf("BATCH_IDENTITIES withdrawn=%s survivor=%s", first, survivor)
}
