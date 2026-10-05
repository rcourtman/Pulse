package truenas

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// These are declared protocol controls, not an alternative native CORE
// capacity source. #2510's absent pool bytes must still remain unavailable.
func TestTrueNASPoolCapacitySnapshot(t *testing.T) {
	enableTrueNASFeatureFlag(t)
	for _, tc := range []struct {
		name   string
		fields string
		known  bool
		used   int64
	}{
		{"flat", `,"size":1000,"allocated":400,"free":600`, true, 400},
		{"empty", `,"size":1000,"allocated":0,"free":1000`, true, 0},
		{"full", `,"size":1000,"allocated":1000,"free":0`, true, 1000},
		{"numeric strings", `,"size":"1000","allocated":"400","free":"600"`, true, 400},
		{"derived free from pool bytes", `,"size":1000,"allocated":400`, true, 400},
		{"no capacity", ``, false, 0},
		{"size alone", `,"size":1000`, false, 0},
		{"null usage", `,"size":1000,"allocated":null,"free":600`, false, 0},
		{"unknown usage", `,"size":1000,"allocated":"unknown","free":600`, false, 0},
		{"object usage", `,"size":1000,"allocated":{"unexpected":400},"free":600`, false, 0},
		{"fractional usage", `,"size":1000,"allocated":0.5,"free":999.5`, false, 0},
		{"negative usage", `,"size":1000,"allocated":-1,"free":1001`, false, 0},
		{"over capacity", `,"size":1000,"allocated":1001,"free":0`, false, 0},
		{"contradictory free", `,"size":1000,"allocated":400,"free":999`, false, 0},
		{"unknown free", `,"size":1000,"allocated":400,"free":"unknown"`, false, 0},
		{"overflow", `,"size":9223372036854775808,"allocated":400,"free":600`, false, 0},
		{"nested hypothesis not a REST source", `,"properties":{"size":{"parsed":1000},"allocated":{"parsed":400},"free":{"parsed":600}}`, false, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			responses := defaultAPIResponses()
			responses["/api/v2.0/system/info"] = apiResponse{body: `{"hostname":"core","version":"TrueNAS-13.0-U6.8","cores":4,"physmem":4096}`}
			responses["/api/v2.0/pool"] = apiResponse{body: `[{"id":1,"guid":"pool-guid","name":"tank","status":"DEGRADED","scan":{"function":"SCRUB","state":"FINISHED"},"topology":{"data":[{"type":"DISK","disk":"sda","status":"DEGRADED"}]}` + tc.fields + `}]`}
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/current" {
					http.NotFound(w, r)
					return
				}
				response, ok := responses[r.URL.Path]
				if !ok {
					http.NotFound(w, r)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(response.body))
			}))
			t.Cleanup(server.Close)
			client := protocolFixtureClient(t, server.URL, ClientConfig{APIKey: "synthetic-key"})
			t.Cleanup(client.Close)
			snapshot, err := client.FetchSnapshot(context.Background())
			if err != nil {
				t.Fatalf("capacity metadata discarded the whole snapshot: %v", err)
			}
			if len(snapshot.Pools) != 1 || len(snapshot.Datasets) != 1 || len(snapshot.Disks) != 2 || len(snapshot.Alerts) != 1 {
				t.Fatal("capacity metadata lost unrelated inventory")
			}
			pool := snapshot.Pools[0]
			if pool.GUID != "pool-guid" || pool.Status != "DEGRADED" || pool.Scan == nil || pool.Scan.Function != "SCRUB" || len(pool.DiskMembers) != 1 {
				t.Fatalf("capacity metadata lost pool health/identity/topology/scan: %+v", pool)
			}
			if tc.known {
				if pool.TotalBytes != 1000 || pool.UsedBytes != tc.used || pool.FreeBytes != 1000-tc.used {
					t.Fatalf("capacity = %d/%d/%d", pool.TotalBytes, pool.UsedBytes, pool.FreeBytes)
				}
			} else if pool.TotalBytes != 0 || pool.UsedBytes != 0 || pool.FreeBytes != 0 {
				t.Fatalf("incomplete/invalid capacity became a usable subtotal: %+v", pool)
			}
			registry := unifiedresources.NewRegistry(unifiedresources.NewMemoryStore())
			registry.IngestRecords(unifiedresources.SourceTrueNAS, NewProvider(*snapshot).Records())
			foundPool, foundDataset, foundHost := false, false, false
			for _, resource := range registry.List() {
				if resource.Type == unifiedresources.ResourceTypeAgent {
					foundHost = true
					if (resource.Metrics.Disk != nil) != tc.known {
						t.Fatal("host capacity availability disagrees with its only pool")
					}
				}
				if resource.Storage == nil {
					continue
				}
				if resource.Storage.Type == "zfs-pool" {
					foundPool = true
					metric := resource.Metrics.Disk
					if (metric != nil) != tc.known {
						t.Fatalf("unknown capacity became a measured zero: %+v", metric)
					}
					if tc.known && (metric.Used == nil || *metric.Used != tc.used || metric.Total == nil || *metric.Total != 1000 || metric.Percent != float64(tc.used)/10) {
						t.Fatalf("pool bytes changed at ingestion: %+v", metric)
					}
				} else if resource.Storage.Type == "zfs-dataset" {
					foundDataset = true
					if resource.Metrics.Disk == nil || *resource.Metrics.Disk.Used != 12345 || *resource.Metrics.Disk.Total != 12900 {
						t.Fatal("pool availability changed independent dataset capacity")
					}
				}
			}
			if !foundPool || !foundDataset || !foundHost {
				t.Fatal("snapshot/provider/registry path lost resources")
			}
		})
	}
}

func TestTrueNASPoolCapacityRPCExactBytes(t *testing.T) {
	const size = int64(9007199254740993) // Above JSON float64's exact integer range.
	for _, method := range []string{"pool.query", "boot.get_state"} {
		t.Run(method, func(t *testing.T) {
			server := newMockServerWithRPC(t, nil, nil, func(t *testing.T, conn *websocket.Conn) {
				auth := readRPCRequest(t, conn)
				writeRPCResult(t, conn, auth.ID, true)
				req := readRPCRequest(t, conn)
				if req.Method != method {
					t.Fatalf("method = %q, want %q", req.Method, method)
				}
				row := fmt.Sprintf(`{"name":"tank","status":"ONLINE","size":%d,"allocated":1,"free":%d}`, size, size-1)
				if method == "pool.query" {
					row = "[" + row + "]"
				}
				writeRPCResult(t, conn, req.ID, json.RawMessage(row))
			})
			t.Cleanup(server.Close)
			client := mustClientForServer(t, server.URL, ClientConfig{APIKey: "synthetic-key"})
			t.Cleanup(client.Close)
			var pool Pool
			if method == "pool.query" {
				pools, err := client.GetPools(context.Background())
				if err != nil || len(pools) != 1 {
					t.Fatalf("GetPools: %v", err)
				}
				pool = pools[0]
			} else {
				boot, err := client.GetBootPool(context.Background())
				if err != nil || boot == nil {
					t.Fatalf("GetBootPool: %v", err)
				}
				pool = *boot
			}
			if pool.TotalBytes != size || pool.UsedBytes != 1 || pool.FreeBytes != size-1 {
				t.Fatalf("native integer bytes were rounded: %+v", pool)
			}
		})
	}
}

func TestTrueNASPoolCapacityAggregate(t *testing.T) {
	for _, tc := range []struct {
		name  string
		pools []Pool
		total int64
		used  int64
	}{
		{"complete", []Pool{{TotalBytes: 1000, UsedBytes: 400}, {TotalBytes: 1000, UsedBytes: 0}}, 2000, 400},
		{"unknown data with known boot", []Pool{{Name: "tank"}, {Name: "boot", IsBoot: true, TotalBytes: 1000, UsedBytes: 400}}, 0, 0},
		{"over capacity", []Pool{{TotalBytes: 1000, UsedBytes: 1001}}, 0, 0},
		{"negative used", []Pool{{TotalBytes: 1000, UsedBytes: -1}}, 0, 0},
		{"total overflow", []Pool{{TotalBytes: math.MaxInt64, UsedBytes: 1}, {TotalBytes: 1, UsedBytes: 0}}, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			total, used := aggregatePoolUsage(tc.pools)
			if total != tc.total || used != tc.used {
				t.Fatalf("host subtotal %d/%d, want %d/%d", total, used, tc.total, tc.used)
			}
		})
	}
}

func TestTrueNASPoolCapacitySourcesDoNotMix(t *testing.T) {
	properties := map[string]any{
		"size": map[string]any{"parsed": int64(2000)}, "allocated": map[string]any{"parsed": int64(900)}, "free": map[string]any{"parsed": int64(1100)},
	}
	for _, tc := range []struct {
		name string
		flat map[string]any
		want [3]int64
	}{
		{"property-only boot shape", nil, [3]int64{2000, 900, 1100}},
		{"flat idle does not inherit property usage", map[string]any{"size": 1000, "allocated": 0, "free": 1000}, [3]int64{1000, 0, 1000}},
		{"flat full does not inherit property free", map[string]any{"size": 1000, "allocated": 1000, "free": 0}, [3]int64{1000, 1000, 0}},
		{"partial flat does not join a second source", map[string]any{"size": 1000}, [3]int64{}},
		{"invalid flat does not try a second source", map[string]any{"size": "unknown", "allocated": 400}, [3]int64{}},
		{"null free is not absent", map[string]any{"size": 1000, "allocated": 400, "free": nil}, [3]int64{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			item := map[string]any{"name": "tank", "status": "ONLINE", "properties": properties}
			for key, value := range tc.flat {
				item[key] = value
			}
			pool, ok := parseBootPoolState(item)
			if !ok || !pool.IsBoot || [3]int64{pool.TotalBytes, pool.UsedBytes, pool.FreeBytes} != tc.want {
				t.Fatalf("mixed or fabricated capacity: %+v", pool)
			}
		})
	}
}
