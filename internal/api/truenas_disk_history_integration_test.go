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

	"github.com/gorilla/websocket"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/truenas"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
)

// Inventory is a declared fixture; History traverses the real authenticated
// TrueNAS client, provider, tenant poller, monitor and Pulse route. It does not
// claim an appliance installation, rendered browser or reporter retest.
type nativeDiskHistoryAPIFetcher struct {
	truenas.FixtureFetcher
	truenas.APIFetcher
}

func (f *nativeDiskHistoryAPIFetcher) Fetch(ctx context.Context) (*truenas.FixtureSnapshot, error) {
	return f.FixtureFetcher.Fetch(ctx)
}

func TestTrueNASDiskHistoryAuthenticatedRoute(t *testing.T) {
	previous := truenas.IsFeatureEnabled()
	truenas.SetFeatureEnabled(true)
	t.Cleanup(func() { truenas.SetFeatureEnabled(previous) })
	previousMulti := IsMultiTenantEnabled()
	SetMultiTenantEnabled(true)
	t.Cleanup(func() { SetMultiTenantEnabled(previousMulti) })
	t.Setenv("PULSE_DEV", "true")
	var reads, auths atomic.Int32
	var nativeEnd atomic.Int64
	upgrader := websocket.Upgrader{}
	native := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/current" {
			t.Errorf("unexpected REST/transport fallback %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()
		for {
			var request struct {
				ID     any               `json:"id"`
				Method string            `json:"method"`
				Params []json.RawMessage `json:"params"`
			}
			if err := conn.ReadJSON(&request); err != nil {
				return
			}
			var result any
			switch request.Method {
			case "auth.login_with_api_key":
				var key string
				if len(request.Params) != 1 || json.Unmarshal(request.Params[0], &key) != nil || key != "native-fixture-key" {
					t.Error("native authentication contract changed")
					return
				}
				auths.Add(1)
				result = true
			case "reporting.get_data", "reporting.netdata_get_data":
				reads.Add(1)
				if len(request.Params) != 2 {
					t.Error("native history parameters changed")
					return
				}
				var graphs []struct {
					Name       string `json:"name"`
					Identifier string `json:"identifier"`
				}
				var query struct {
					Start     int64 `json:"start"`
					End       int64 `json:"end"`
					Aggregate bool  `json:"aggregate"`
				}
				if json.Unmarshal(request.Params[0], &graphs) != nil || len(graphs) != 1 || graphs[0].Name != "disktemp" || graphs[0].Identifier != "sdb" || json.Unmarshal(request.Params[1], &query) != nil || !query.Aggregate || query.End-query.Start != 3600 {
					t.Error("native disk identity/time window changed")
					return
				}
				result = []any{}
				if request.Method == "reporting.netdata_get_data" {
					nativeEnd.Store(query.End)
					result = []any{map[string]any{
						"name": "disktemp", "identifier": "sdb", "legend": []string{"temperature"},
						"start": query.End - 3540, "end": query.End - 60, "step": 1740,
						"data":         [][]any{{31.25}, {33.5}, {32.75}},
						"aggregations": map[string]any{"min": []float64{999}, "mean": []float64{999}, "max": []float64{999}},
					}}
				}
			default:
				t.Errorf("unexpected native method %q", request.Method)
				return
			}
			if err := conn.WriteJSON(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": result}); err != nil {
				t.Error(err)
				return
			}
		}
	}))
	t.Cleanup(native.Close)
	nativeClient, err := truenas.NewClient(truenas.ClientConfig{Host: native.URL, APIKey: "native-fixture-key", UseHTTPS: true, InsecureSkipVerify: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(nativeClient.Close)
	fixture := truenas.DefaultFixtures()
	fixture.Disks = []truenas.Disk{{Name: "sdb", Serial: "disk-serial", Temperature: 34}}
	provider := truenas.NewLiveProviderForConnection(&nativeDiskHistoryAPIFetcher{FixtureFetcher: truenas.FixtureFetcher{Snapshot: fixture}, APIFetcher: truenas.APIFetcher{Client: nativeClient}}, "native-connection")
	if err := provider.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	poller := monitoring.NewTrueNASPoller(nil, time.Minute, nil)
	setUnexportedField(t, poller, "providersByOrg", map[string]map[string]*truenas.Provider{"org-a": {"native-connection": provider}})
	setUnexportedField(t, poller, "cachedRecordsByOrg", map[string]map[string][]unifiedresources.IngestRecord{"org-a": {"native-connection": provider.Records()}})
	monitor := &monitoring.Monitor{}
	monitor.SetOrgID("org-a")
	setUnexportedField(t, monitor, "state", models.NewState())
	setUnexportedField(t, monitor, "metricsHistory", monitoring.NewMetricsHistory(20, time.Hour))
	monitor.SetSupplementalRecordsProvider(unifiedresources.SourceTrueNAS, poller)
	registry := unifiedresources.NewRegistry(nil)
	monitor.SetResourceStore(unifiedresources.NewMonitorAdapter(registry))

	const token = "disk-history-read-fixture.12345678"
	readToken := newTokenRecord(t, token, []string{config.ScopeMonitoringRead}, nil)
	readToken.OrgID = "org-a"
	const wrongScope = "disk-history-settings-fixture.12345678"
	scopeToken := newTokenRecord(t, wrongScope, []string{config.ScopeSettingsRead}, nil)
	scopeToken.OrgID = "org-a"
	cfg := newTestConfigWithTokens(t, readToken, scopeToken)
	cfg.AuthUser = "fixture-admin"
	cfg.AuthPass = "fixture-password-hash-not-used-by-token"
	for _, org := range []string{"org-a", "org-b"} {
		if err := os.MkdirAll(filepath.Join(cfg.DataPath, "orgs", org), 0700); err != nil {
			t.Fatal(err)
		}
	}
	router := NewRouter(cfg, monitor, nil, nil, nil, "test")
	t.Cleanup(router.ShutdownBackgroundWorkers)
	t.Cleanup(router.ShutdownResourceStores)
	t.Cleanup(router.ShutdownRBAC)
	mtm := monitoring.NewMultiTenantMonitor(cfg, nil, nil)
	setUnexportedField(t, mtm, "monitors", map[string]*monitoring.Monitor{"org-a": monitor, "org-b": {}})
	router.mtMonitor = mtm
	// Bind the declared native provider after the constructor installs its
	// runtime owners, using the same setter/tenant initializer as production.
	router.setMonitorSupplementalRecordsProvider(unifiedresources.SourceTrueNAS, poller)
	maxPoints := ""
	request := func(raw, org, metric string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/metrics-store/history?resourceType=disk&resourceId=disk-serial&metric="+metric+"&range=1h&maxPoints="+maxPoints, nil)
		req.Header.Set("X-Pulse-Org-ID", org)
		if raw != "" {
			req.Header.Set("X-API-Token", raw)
		}
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		return rec
	}
	for _, test := range []struct {
		raw, org string
		want     int
	}{{"", "org-a", http.StatusUnauthorized}, {wrongScope, "org-a", http.StatusForbidden}, {token, "org-b", http.StatusForbidden}} {
		rec := request(test.raw, test.org, "smart_temp")
		if rec.Code != test.want || reads.Load() != 0 {
			t.Fatalf("auth/scope/tenant refusal lost: status %d want %d native reads %d body %s", rec.Code, test.want, reads.Load(), rec.Body.String())
		}
	}
	rec := request(token, "org-a", "smart_temp")
	if rec.Code != http.StatusOK {
		t.Fatalf("authenticated History status %d: %s", rec.Code, rec.Body.String())
	}
	var got metricsHistoryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.ResourceType != "disk" || got.ResourceId != "disk-serial" || got.Metric != "smart_temp" || len(got.Points) != 3 || reads.Load() != 2 || auths.Load() != 1 {
		t.Fatalf("client/provider/poller/route did not deliver native disk history: %+v, reads %d auths %d", got, reads.Load(), auths.Load())
	}
	end := nativeEnd.Load()
	for i, value := range []float64{31.25, 33.5, 32.75} {
		if got.Points[i].Timestamp != time.Unix(end-3540+int64(i)*1740, 0).UnixMilli() || got.Points[i].Value != value {
			t.Fatalf("native timestamp/Celsius sample %d changed: %+v", i, got.Points[i])
		}
	}
	type point struct {
		Timestamp int64   `json:"timestamp"`
		Value     float64 `json:"value"`
		Min       float64 `json:"min"`
		Max       float64 `json:"max"`
	}
	readAll := func() map[string][]point {
		t.Helper()
		rec := request(token, "org-a", "")
		var all struct {
			ResourceType string             `json:"resourceType"`
			ResourceID   string             `json:"resourceId"`
			Metrics      map[string][]point `json:"metrics"`
		}
		if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &all) != nil || all.ResourceType != "disk" || all.ResourceID != "disk-serial" {
			t.Fatalf("all-metric drawer contract lost: status %d body %s", rec.Code, rec.Body.String())
		}
		if maxPoints == "240" {
			t.Logf("BOUND_DRAWER_HISTORY_RESPONSE %s", rec.Body.String())
		}
		return all.Metrics
	}
	assertNative := func(points []point, wantReads int32) {
		t.Helper()
		if len(points) != 3 || reads.Load() != wantReads {
			t.Fatalf("all-metric drawer suppressed native History: %+v, reads %d want %d", points, reads.Load(), wantReads)
		}
		for i, value := range []float64{31.25, 33.5, 32.75} {
			wantTime := time.Unix(nativeEnd.Load()-3540+int64(i)*1740, 0).UnixMilli()
			if points[i].Value != value || points[i].Min != value || points[i].Max != value || points[i].Timestamp != wantTime {
				t.Fatalf("native all-metric timestamp/Celsius/bounds changed: %+v", points[i])
			}
		}
	}
	assertNative(readAll()["smart_temp"], 4)

	// The real drawer asks for all metrics. An unrelated I/O row, then a
	// single shallow temperature row, must not bypass the native reader.
	store, err := metrics.NewStore(metrics.DefaultConfig(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	setUnexportedField(t, monitor, "metricsStore", store)
	// Match the existing persistent-history timestamp precision, as the other
	// store-backed route fixtures do. Native sample/bucket assertions below
	// still require the exact independently supplied timestamps.
	ioTime := time.Now().UTC().Truncate(time.Second).Add(-20 * time.Second)
	store.WriteBatchSync([]metrics.WriteMetric{{ResourceType: "disk", ResourceID: "disk-serial", MetricType: "diskread", Tier: metrics.TierRaw, Value: 1024, Timestamp: ioTime}})
	all := readAll()
	assertNative(all["smart_temp"], 6)
	if len(all["diskread"]) != 1 || all["diskread"][0].Value != 1024 || all["diskread"][0].Min != 1024 || all["diskread"][0].Max != 1024 || all["diskread"][0].Timestamp != ioTime.UnixMilli() {
		t.Fatalf("native supplement changed independent stored I/O: %+v", all["diskread"])
	}
	store.WriteBatchSync([]metrics.WriteMetric{{ResourceType: "disk", ResourceID: "disk-serial", MetricType: "smart_temp", Tier: metrics.TierRaw, Value: 44, Timestamp: ioTime}})
	assertNative(readAll()["smart_temp"], 8)
	rec = request(token, "org-a", "smart_temp")
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &got) != nil || len(got.Points) != 3 || reads.Load() != 10 {
		t.Fatalf("single-metric sparse store hid native History: status %d points %+v reads %d", rec.Code, got.Points, reads.Load())
	}
	for i, value := range []float64{31.25, 33.5, 32.75} {
		if got.Points[i].Value != value {
			t.Fatalf("sparse stored/native samples were spliced: %+v", got.Points)
		}
	}
	// The actual browser query requests 240 points. Preserve the existing
	// 15-second bucket centres and per-observation values/bounds, not a fake
	// aggregate temperature. Retain the real fixture API response for rendering.
	maxPoints = "240"
	all = readAll()
	if len(all["smart_temp"]) != 3 || reads.Load() != 12 {
		t.Fatalf("browser query hid native history: %+v, reads %d", all, reads.Load())
	}
	for i, value := range []float64{31.25, 33.5, 32.75} {
		ts := nativeEnd.Load() - 3540 + int64(i)*1740
		wantTime := ((ts/15)*15 + 7) * 1000
		if all["smart_temp"][i].Value != value || all["smart_temp"][i].Min != value || all["smart_temp"][i].Max != value || all["smart_temp"][i].Timestamp != wantTime {
			t.Fatalf("issued browser aggregation changed native sample: %+v", all["smart_temp"][i])
		}
	}
	maxPoints = ""
	// Adequate local coverage keeps the native session quiet and its original
	// independent values intact. Ordinary store errors still fail closed.
	now := time.Now().UTC()
	for _, offset := range []time.Duration{-55 * time.Minute, -30 * time.Minute, -time.Minute} {
		store.WriteBatchSync([]metrics.WriteMetric{{ResourceType: "disk", ResourceID: "disk-serial", MetricType: "smart_temp", Tier: metrics.TierRaw, Value: 35, Timestamp: now.Add(offset)}})
	}
	all = readAll()
	if len(all["smart_temp"]) != 4 || all["smart_temp"][0].Value != 35 || reads.Load() != 12 {
		t.Fatalf("complete store needlessly invoked/replaced native history: %+v, reads %d", all["smart_temp"], reads.Load())
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	rec = request(token, "org-a", "")
	if rec.Code != http.StatusInternalServerError || reads.Load() != 12 {
		t.Fatalf("failed store read silently became native success: status %d reads %d", rec.Code, reads.Load())
	}
	if removed := cfg.RemoveAPIToken(readToken.ID); removed == nil {
		t.Fatal("fixture token missing")
	}
	cfg.SortAPITokens()
	rec = request(token, "org-a", "smart_temp")
	if rec.Code != http.StatusUnauthorized || reads.Load() != 12 {
		t.Fatalf("revoked token still reached History: status %d reads %d", rec.Code, reads.Load())
	}
}
