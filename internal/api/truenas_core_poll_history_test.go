package api

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/truenas"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
)

const coreHistoryFixtureDisks = 20

// Keep the actual CORE REST client and move only the collection clock to
// three logical minute-spaced polls. The fixture's rate timestamps are
// generated from the same logical clock; the test need not wait three minutes.
type coreHistoryFixtureClock struct {
	client      *truenas.Client
	collectedAt time.Time
}

func (f *coreHistoryFixtureClock) Fetch(ctx context.Context) (*truenas.FixtureSnapshot, error) {
	snapshot, err := f.client.FetchSnapshot(ctx)
	if err == nil {
		snapshot.CollectedAt = f.collectedAt
	}
	return snapshot, err
}

type coreWindowHistory struct {
	ResourceID string                          `json:"resourceId"`
	Range      string                          `json:"range"`
	Start      int64                           `json:"start"`
	End        int64                           `json:"end"`
	Source     string                          `json:"source"`
	Metrics    map[string][]hostIOHistoryPoint `json:"metrics"`
}

type coreHistoryRun struct {
	anchor        time.Time
	methods       map[string]int
	requestCounts <-chan string
	store         *metrics.Store
	inventory     []unifiedresources.Resource
	request       func(string) *httptest.ResponseRecorder
}

func coreHistoryTarget(i int) string { return fmt.Sprintf("core-window-disk-%02d", i) }

func coreHistoryPath(i int, span string) string {
	return "/api/metrics-store/history?resourceType=disk&resourceId=" + url.QueryEscape(coreHistoryTarget(i)) + "&range=" + span + "&maxPoints=240"
}

var coreHistoryReads = [2][6]float64{{3000, 0, 24000, 3000, 500, 56000}, {3000, 43000, 2500, 2000, 37000, 1000}}
var coreHistoryWrites = [2][6]float64{{1700, 10000, 0, 7000, 6000, 4500}, {1700, 250, 20000, 1800, 1000, 27000}}

func startCOREHistoryFixture(t *testing.T, anchor time.Time, compatibilityDuplicate bool) coreHistoryRun {
	t.Helper()
	t.Setenv("PULSE_DEV", "true")
	previous := truenas.IsFeatureEnabled()
	truenas.SetFeatureEnabled(true)
	t.Cleanup(func() { truenas.SetFeatureEnabled(previous) })

	// Three logical minute-apart polls without sleeping. The fixture serves
	// 10s observations as explicitly timestamped rows; a coarse response
	// step keeps the three tail readings in the real client's live suffix.
	firstTail := anchor.Add(-2 * time.Minute).Unix()
	methods := make(map[string]int)
	requestCounts := make(chan string, 128)
	var poll atomic.Int64
	names := make([]string, coreHistoryFixtureDisks)
	for i := range names {
		names[i] = fmt.Sprintf("ada%d", i)
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case requestCounts <- r.Method + " " + r.URL.Path:
		default:
			t.Error("fixture received unexpected extra calls")
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v2.0/system/info":
			fmt.Fprint(w, `{"hostname":"core-window","version":"TrueNAS-13.0-U6.1","cores":4,"physmem":17179869184,"system_serial":"core-window-machine"}`)
		case "/api/v2.0/disk":
			var inventory []map[string]any
			for i, device := range names {
				pool := "tank"
				if i < 2 {
					pool = "boot-pool"
				}
				inventory = append(inventory, map[string]any{"identifier": "{" + device + "}", "name": device, "serial": coreHistoryTarget(i), "model": fmt.Sprintf("Fixture disk %02d", i), "pool": pool, "size": 1073741824000, "type": "HDD"})
			}
			_ = json.NewEncoder(w).Encode(inventory)
		case "/api/v2.0/disk/temperatures":
			_ = json.NewEncoder(w).Encode(map[string]int{"ada0": 32, "ada1": 36})
		case "/api/v2.0/alert/list":
			fmt.Fprint(w, "[]")
		case "/api/v2.0/reporting/graphs":
			_ = json.NewEncoder(w).Encode([]map[string]any{{"name": "disk", "identifiers": names}})
		case "/api/v2.0/reporting/get_data":
			var query struct {
				Graphs    []struct{ Name, Identifier string }
				Reporting struct{ Start, End int64 } `json:"reporting_query"`
			}
			if json.NewDecoder(r.Body).Decode(&query) != nil || query.Reporting.End-query.Reporting.Start != 300 {
				t.Error("CORE reporting did not use the existing 300-second request")
				http.Error(w, "bad fixture request", 400)
				return
			}
			last := firstTail + 60*poll.Load()
			replies := []map[string]any{}
			for _, graph := range query.Graphs {
				if graph.Name != "disk" {
					continue
				}
				var i int
				_, _ = fmt.Sscanf(graph.Identifier, "ada%d", &i)
				rows := make([][]any, 0, 31)
				for at := last - 300; at <= last; at += 10 {
					slot := (at - firstTail) / 10
					idx := (slot%6 + 6) % 6
					read := coreHistoryReads[i%2][idx] + float64(i/2)*1000
					write := any(coreHistoryWrites[i%2][idx] + float64(i/2)*500)
					if i == coreHistoryFixtureDisks-1 && idx == 3 {
						write = nil
					}
					rows = append(rows, []any{at, read, write})
				}
				if compatibilityDuplicate && i == 0 {
					// The first reply is timestamped compatibility data. The later
					// native reply has exactly the same tail, but different earlier
					// readings. Matching values alone cannot establish provenance.
					replies = append(replies, map[string]any{
						"name": " DISK ", "identifier": " " + graph.Identifier + " ", "step": 0,
						"legend": []string{"disk_octets_read", "disk_octets_write"}, "data": rows[len(rows)-1:],
					})
				}
				replies = append(replies, map[string]any{"name": "disk", "identifier": graph.Identifier, "start": last - 300, "end": last, "step": 150, "legend": []string{"disk_octets_read", "disk_octets_write"}, "data": rows})
			}
			_ = json.NewEncoder(w).Encode(replies)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	client, err := truenas.NewClient(truenas.ClientConfig{
		Host: server.URL, APIKey: "offline-fixture-only",
		Fingerprint: fmt.Sprintf("%x", sha256.Sum256(server.Certificate().Raw)), InsecureSkipVerify: true,
		SkipReplication: true, SkipDatasets: true, SkipPools: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	clock := &coreHistoryFixtureClock{client: client}
	provider := truenas.NewLiveProviderForConnection(clock, "core-window")
	t.Cleanup(client.Close)
	// Feed the exact accepted provider and its records through the monitored
	// poller's own org scope, without running a timer or issuing another poll.
	poller := monitoring.NewTrueNASPoller(nil, time.Minute, nil)
	setUnexportedField(t, poller, "providersByOrg", map[string]map[string]*truenas.Provider{"default": {"core-window": provider}})
	cached := map[string][]unifiedresources.IngestRecord{}
	setUnexportedField(t, poller, "cachedRecordsByOrg", map[string]map[string][]unifiedresources.IngestRecord{"default": cached})

	store, err := metrics.NewStore(metrics.DefaultConfig(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.WaitForMaintenance(10 * time.Second); err != nil {
		t.Fatal(err)
	}
	// Recorded earlier coverage uses the same canonical History keys; leave
	// the 30m window entirely to native polling and keep the older data equal
	// within each pair so it cannot fake the newly distinct trajectories.
	var older []metrics.WriteMetric
	for _, id := range []string{coreHistoryTarget(0), coreHistoryTarget(1)} {
		for i := 4; i < 96; i++ {
			for _, direction := range []string{"diskread", "diskwrite"} {
				older = append(older, metrics.WriteMetric{ResourceType: "disk", ResourceID: id, MetricType: direction, Value: 1500 + float64(i%11)*250, Timestamp: anchor.Add(-time.Duration(i) * 15 * time.Minute), Tier: metrics.TierMinute})
			}
		}
	}
	store.WriteBatchSync(older)
	monitor := &monitoring.Monitor{}
	setUnexportedField(t, monitor, "state", models.NewState())
	setUnexportedField(t, monitor, "metricsHistory", monitoring.NewMetricsHistory(1024, 24*time.Hour))
	setUnexportedField(t, monitor, "metricsStore", store)
	adapter := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))
	for i := int64(0); i < 3; i++ {
		poll.Store(i)
		clock.collectedAt = time.Unix(firstTail+60*i+1, 0).UTC()
		if err := provider.Refresh(context.Background()); err != nil {
			t.Fatal(err)
		}
		cached["core-window"] = provider.Records()
		monitor.SetResourceStoreAndProviders(adapter, map[unifiedresources.DataSource]monitoring.MonitorSupplementalRecordsProvider{unifiedresources.SourceTrueNAS: poller})
		for rebuild := 0; rebuild < 6; rebuild++ {
			monitor.SetSupplementalRecordsProvider(unifiedresources.SourceTrueNAS, poller)
		}
	}
	store.Flush()
	for len(requestCounts) > 0 {
		methods[<-requestCounts]++
	}

	const tokenValue = "core-history-fixture.1234567890123456789"
	token := newTokenRecord(t, tokenValue, []string{config.ScopeMonitoringRead}, nil)
	cfg := newTestConfigWithTokens(t, token)
	router := newTestRouter(t, cfg, monitor, nil, nil, nil, "test")
	t.Cleanup(router.ShutdownBackgroundWorkers)
	t.Cleanup(router.ShutdownResourceStores)
	t.Cleanup(router.ShutdownRBAC)
	router.setMonitorSupplementalRecordsProvider(unifiedresources.SourceTrueNAS, poller)
	request := func(path string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r.Header.Set("X-API-Token", tokenValue)
		w := httptest.NewRecorder()
		router.Handler().ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body.String())
		}
		return w
	}
	inventory := request("/api/resources?type=physical_disk,agent&source=truenas&limit=100")
	var rsrc ResourcesResponse
	if err := json.Unmarshal(inventory.Body.Bytes(), &rsrc); err != nil {
		t.Fatal(err)
	}
	if len(rsrc.Data) != coreHistoryFixtureDisks+1 {
		t.Fatalf("incomplete canonical inventory: %d resources", len(rsrc.Data))
	}
	for _, res := range rsrc.Data {
		if res.Type == unifiedresources.ResourceTypeAgent {
			if res.Metrics == nil || res.Metrics.DiskRead == nil || res.Metrics.DiskWrite == nil ||
				res.Metrics.DiskRead.Value != 150000 || res.Metrics.DiskWrite.Value != 79000 {
				t.Fatalf("aligned host totals changed: %+v", res.Metrics)
			}
		} else if res.PhysicalDisk == nil || res.PhysicalDisk.IO != nil || res.MetricsTarget == nil || res.MetricsTarget.ResourceType != "disk" {
			t.Fatalf("physical disk lost its target or invented counters: %+v", res)
		}
	}
	return coreHistoryRun{anchor: anchor, methods: methods, requestCounts: requestCounts, store: store, inventory: rsrc.Data, request: request}
}

func (r coreHistoryRun) readDisk(t *testing.T, index int, span string) coreWindowHistory {
	t.Helper()
	w := r.request(coreHistoryPath(index, span))
	var history coreWindowHistory
	if err := json.Unmarshal(w.Body.Bytes(), &history); err != nil || (history.ResourceID != coreHistoryTarget(index) || history.Source != "store") {
		t.Fatalf("wrong history target: %s / %v", w.Body, err)
	}
	return history
}

// The I/O candidate must survive native decoding, canonical target selection
// and persisted History. The two selected disks end each poll with the same
// live reading. No new provider call is needed to see an earlier difference.
func TestCOREHistoryKeepsIntraPollIOAndDayCoverage(t *testing.T) {
	r := startCOREHistoryFixture(t, time.Now().UTC().Truncate(time.Minute).Add(-time.Minute), false)
	for _, span := range []string{"30m", "1h", "24h"} {
		a, b := r.readDisk(t, 0, span), r.readDisk(t, 1, span)
		for _, kind := range []string{"diskread", "diskwrite"} {
			x, y := a.Metrics[kind], b.Metrics[kind]
			if len(x) == 0 || len(y) == 0 || reflect.DeepEqual(x, y) {
				t.Errorf("%s/%s: distinct intra-poll readings collapsed to identical disk History", span, kind)
				continue
			}
			if span == "24h" {
				for i, series := range [][]hostIOHistoryPoint{x, y} {
					if len(series) < 93 || series[0].Timestamp > r.anchor.Add(-23*time.Hour).UnixMilli() {
						t.Errorf("disk %d %s day coverage lost: %d buckets, first %d", i, kind, len(series), series[0].Timestamp)
					}
				}
			}
		}
		for i, history := range []coreWindowHistory{a, b} {
			// The fixture's two displayed disks always report both rates in
			// each row. Sampling must neither invent nulls nor strand a
			// valid companion in a new, one-direction display bucket.
			read, write := history.Metrics["diskread"], history.Metrics["diskwrite"]
			if len(read) != len(write) {
				t.Errorf("%s disk %d reported paired native rates but History dropped one direction", span, i)
			} else {
				for j := range read {
					if read[j].Timestamp != write[j].Timestamp {
						t.Errorf("%s disk %d split a native History frame", span, i)
					}
				}
			}
			temp := history.Metrics["smart_temp"]
			if len(temp) == 0 || temp[len(temp)-1].Value != 32+float64(4*i) || len(history.Metrics["disk"]) != 0 {
				t.Errorf("%s disk %d lost temperature or invented busy", span, i)
			}
		}
	}
	if r.methods["POST /api/v2.0/reporting/get_data"] != 3 || r.methods["GET /api/v2.0/reporting/graphs"] != 3 || len(r.requestCounts) != 0 {
		t.Errorf("History or rebuild added provider calls: %v plus %d", r.methods, len(r.requestCounts))
	}
	count := 0
	for i := 0; i < coreHistoryFixtureDisks; i++ {
		for _, kind := range []string{"diskread", "diskwrite"} {
			points, err := r.store.Query("disk", coreHistoryTarget(i), kind, r.anchor.Add(-30*time.Minute), time.Now(), 0)
			if err != nil {
				t.Fatal(err)
			}
			if len(points) != 4 {
				t.Errorf("%s %s: %d raw I/O points, want 3 tails and one earlier observation", coreHistoryTarget(i), kind, len(points))
			}
			count += len(points)
			for _, p := range points {
				if p.Timestamp.Before(r.anchor.Add(-2 * time.Minute)) {
					t.Errorf("backfilled a pre-inventory observation: %+v", p)
				}
			}
		}
	}
	t.Logf("CORE_HISTORY accepted I/O rows=%d / 3 poll ticks / %d disks", count, coreHistoryFixtureDisks)
}

func TestCOREHistoryKeepsFirstGraphAuthority(t *testing.T) {
	r := startCOREHistoryFixture(t, time.Now().UTC().Truncate(time.Minute).Add(-time.Minute), true)
	id := coreHistoryTarget(0)
	var live *unifiedresources.ResourceMetrics
	for _, resource := range r.inventory {
		if resource.MetricsTarget != nil && resource.MetricsTarget.ResourceID == id {
			live = resource.Metrics
			break
		}
	}
	// The resource DTO exposes the live values, not the internal observation
	// timestamp. Verify the exact native instants in the persisted rows below.
	if live == nil || live.DiskRead == nil || live.DiskWrite == nil {
		t.Fatal("compatibility reply's live rates missing")
	}
	if live.DiskRead.Value != 3000 || live.DiskWrite.Value != 1700 {
		t.Fatalf("compatibility reply's live rates changed: read %g, write %g", live.DiskRead.Value, live.DiskWrite.Value)
	}
	for _, direction := range []struct {
		metric string
		value  float64
	}{{"diskread", 3000}, {"diskwrite", 1700}} {
		points, err := r.store.Query("disk", id, direction.metric, r.anchor.Add(-30*time.Minute), time.Now(), 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(points) != 3 {
			t.Fatalf("%s: want only the three first-graph tails, got %+v", direction.metric, points)
		}
		for i, point := range points {
			wantAt := r.anchor.Add(time.Duration(i-2) * time.Minute)
			if !point.Timestamp.Equal(wantAt) || point.Value != direction.value {
				t.Fatalf("%s: unexpected observation %+v (wanted %g at %s)", direction.metric, point, direction.value, wantAt)
			}
		}
	}
	// The preceding compatibility graph for one disk does not disable the
	// ordinary native frame from the first graph of a different disk.
	native := r.readDisk(t, 1, "1h")
	seenEarlier := false
	for _, point := range native.Metrics["diskread"] {
		if point.Max > 3000 {
			seenEarlier = true
		}
	}
	if !seenEarlier || r.methods["POST /api/v2.0/reporting/get_data"] != 3 || len(r.requestCounts) != 0 {
		t.Fatalf("lost valid sibling History or added provider calls: earlier %t requests %v plus %d", seenEarlier, r.methods, len(r.requestCounts))
	}
}
