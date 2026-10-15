package resourceapi

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/truenas"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// This bridge models the CORE REST schema, not an appliance capture. The
// rejection of query is the reported #2964 response; reporting_query is the
// schema name, distinct from the second positional WebSocket argument. Keep
// this boundary independent of the client's request struct so changing both
// sides to the same wrong spelling cannot make the regression pass.
func TestCORE13RESTReportingQueryResourceJourney(t *testing.T) {
	previous := truenas.IsFeatureEnabled()
	truenas.SetFeatureEnabled(true)
	t.Cleanup(func() { truenas.SetFeatureEnabled(previous) })
	var phase, posts, catalogueCalls, rejectedOldKey atomic.Int64
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v2.0/system/info":
			fmt.Fprint(w, `{"hostname":"core-fixture","version":"TrueNAS-13.0-U6.8","cores":8,"physmem":17179869184}`)
		case "/api/v2.0/pool":
			fmt.Fprint(w, `[{"id":1,"name":"tank","status":"ONLINE","size":1000,"allocated":400,"free":600}]`)
		case "/api/v2.0/disk":
			fmt.Fprint(w, `[{"identifier":"disk-a","name":"da0","serial":"fixture-disk","rotationrate":"unknown","size":1000}]`)
		case "/api/v2.0/pool/dataset", "/api/v2.0/alert/list":
			fmt.Fprint(w, `[]`)
		case "/api/v2.0/reporting/graphs":
			catalogueCalls.Add(1)
			fmt.Fprint(w, `[{"name":"disk","identifiers":["da0"]},{"name":"interface","identifiers":["vtnet0"]}]`)
		case "/api/v2.0/reporting/get_data":
			posts.Add(1)
			var body map[string]json.RawMessage
			if r.Method != http.MethodPost || json.NewDecoder(r.Body).Decode(&body) != nil {
				http.Error(w, "invalid fixture request", http.StatusBadRequest)
				return
			}
			if _, present := body["query"]; present {
				rejectedOldKey.Add(1)
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprint(w, `{"message":"The following attributes are not expected: query"}`)
				return
			}
			var graphs []struct{ Name, Identifier string }
			var window struct {
				Start, End int64
				Aggregate  bool
			}
			if len(body) != 2 || json.Unmarshal(body["graphs"], &graphs) != nil || json.Unmarshal(body["reporting_query"], &window) != nil || !window.Aggregate || window.Start >= window.End {
				http.Error(w, "missing graphs/reporting_query range", http.StatusBadRequest)
				return
			}
			if phase.Load() == 1 {
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprint(w, `{"message":"synthetic reporting validation failure"}`)
				return
			}
			var result []map[string]any
			for _, graph := range graphs {
				var legend []string
				var values []any
				zero := phase.Load() == 3
				if phase.Load() == 2 && graph.Name != "cpu" {
					continue // usable CPU does not establish any sibling reading
				}
				switch graph.Name {
				case "cpu":
					legend, values = []string{"user", "nice", "system", "interrupt", "idle"}, []any{8, 0, 3, 1, 88}
					if zero {
						values = []any{0, 0, 0, 0, 100}
					}
				case "memory":
					legend = []string{"memory-active_value", "memory-inactive_value", "memory-wired_value", "memory-laundry_value", "memory-free_value"}
					values = []any{1 << 30, 2 << 30, 11 << 30, 0, 2 << 30}
					if zero {
						values = []any{0, 0, 0, 0, 16 << 30}
					}
				case "arcsize":
					legend, values = []string{"arcsize_value"}, []any{8 << 30}
					if zero {
						values = []any{0}
					}
				case "disk":
					if graph.Identifier != "da0" {
						t.Error("disk identifier lost")
					}
					legend, values = []string{"disk_octets_read", "disk_octets_write"}, []any{1024, 2048}
					if zero {
						values = []any{0, 0}
					}
				case "interface":
					if graph.Identifier != "vtnet0" {
						t.Error("interface identifier lost")
					}
					legend, values = []string{"rx", "tx", "overlap"}, []any{32, 64, 32}
					if zero {
						values = []any{0, 0, 0}
					}
				case "cputemp":
					legend, values = []string{"temperature"}, []any{42}
				default:
					t.Errorf("unexpected graph %s", graph.Name)
				}
				result = append(result, map[string]any{"name": graph.Name, "identifier": graph.Identifier, "legend": legend, "start": window.End - 10, "end": window.End, "step": 10, "data": []any{values, values}})
			}
			if err := json.NewEncoder(w).Encode(result); err != nil {
				t.Error(err)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	// Pin this fixture certificate through the production VerifyConnection
	// path; no public CA or trust bypass is needed or changed in the product.
	client, err := truenas.NewClient(truenas.ClientConfig{Host: server.URL, APIKey: "synthetic", InsecureSkipVerify: true, Fingerprint: fmt.Sprintf("%x", sha256.Sum256(server.Certificate().Raw))})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	// Establish that the fixture really rejects the old envelope over HTTP.
	old, err := server.Client().Post(server.URL+"/api/v2.0/reporting/get_data", "application/json", strings.NewReader(`{"graphs":[{"name":"cpu"}],"query":{"start":1,"end":2}}`))
	if err != nil {
		t.Fatal(err)
	}
	_ = old.Body.Close()
	if old.StatusCode != 400 || rejectedOldKey.Load() != 1 {
		t.Fatal("fixture does not reproduce the reported query rejection")
	}
	provider := truenas.NewLiveProviderForConnection(&truenas.APIFetcher{Client: client}, "core-rest-fixture")
	for i, name := range []string{"complete", "rejected", "partial", "zero", "recovered"} {
		t.Run(name, func(t *testing.T) {
			phase.Store(int64(i))
			beforePosts, beforeCatalogue := posts.Load(), catalogueCalls.Load()
			if err := provider.Refresh(context.Background()); err != nil {
				t.Fatal(err)
			}
			snapshot := provider.Snapshot()
			if len(snapshot.Disks) != 1 || len(snapshot.Pools) != 1 || snapshot.System.CPUCount != 8 || snapshot.System.MemoryTotalBytes != 16<<30 {
				t.Fatal("optional reporting failure discarded inventory or capacity")
			}
			registry := unified.NewRegistry(nil)
			registry.IngestRecords(unified.SourceTrueNAS, provider.Records())
			h := NewQueryService(&config.Config{DataPath: t.TempDir()})
			h.SetStateProvider(resourceUnifiedSeedProvider{snapshot: models.StateSnapshot{LastUpdate: time.Now()}, resources: registry.List()})
			rec := httptest.NewRecorder()
			h.HandleListResources(rec, httptest.NewRequest(http.MethodGet, "/api/resources?source=truenas&type=agent,storage,physical_disk&limit=100", nil))
			var response ResourcesResponse
			if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &response) != nil {
				t.Fatalf("resource HTTP response %d: %s", rec.Code, rec.Body.String())
			}
			var host *unified.Resource
			for index := range response.Data {
				if response.Data[index].Type == unified.ResourceTypeAgent {
					host = &response.Data[index]
				}
			}
			if host == nil || host.Metrics == nil || host.Metrics.Disk == nil || host.Metrics.Disk.Percent != 40 || host.Agent == nil || host.Agent.Memory == nil {
				t.Fatalf("host/storage capacity lost through HTTP: %s", rec.Body.String())
			}
			cpuKnown, memoryKnown := i != 1, i != 1 && i != 2
			if (host.Metrics.CPU != nil) != cpuKnown || (host.Metrics.Memory != nil) != memoryKnown || host.Agent.Memory.UsageUnavailable == memoryKnown {
				t.Fatalf("missing/partial readings became measured zero: %+v", host.Metrics)
			}
			if cpuKnown {
				want := 12.
				if i == 3 {
					want = 0
				}
				if host.Metrics.CPU.Percent != want {
					t.Fatalf("CPU = %v, want %v", host.Metrics.CPU.Percent, want)
				}
			}
			if memoryKnown {
				want := 37.5
				if i == 3 {
					want = 0
				}
				if host.Metrics.Memory.Percent != want {
					t.Fatalf("ARC-aware memory = %v, want %v", host.Metrics.Memory.Percent, want)
				}
			}
			id, history, historyErr := provider.SystemMetricHistory(context.Background(), time.Hour)
			wantCalls := int64(2) // one successful batch each for live and History
			if i == 1 {
				wantCalls = 14 // existing bounded optional-graph isolation
				if historyErr == nil || snapshot.System.Telemetry.ErrorCategory != "request_rejected" {
					t.Fatal("failed reporting disguised as successful History/telemetry")
				}
			} else if historyErr != nil || id != "core-rest-fixture" || len(history["cpu"]) != 2 {
				t.Fatalf("native CPU History lost: %s %+v %v", id, history, historyErr)
			}
			if posts.Load()-beforePosts != wantCalls || catalogueCalls.Load()-beforeCatalogue != 2 || rejectedOldKey.Load() != 1 {
				t.Fatal("unexpected request count, alias retry, or old query envelope")
			}
			for _, key := range []string{"memory", "diskread", "diskwrite", "netin", "netout"} {
				if (len(history[key]) > 0) != memoryKnown {
					t.Fatalf("History %s fabricated/lost a sibling: %+v", key, history[key])
				}
			}
			proof, err := json.Marshal(map[string]any{"phase": name, "resources": response, "history": history, "telemetry": snapshot.System.Telemetry})
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("CORE_REST_BROWSER %s", proof)
		})
	}
}
