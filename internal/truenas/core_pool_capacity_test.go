package truenas

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// CORE 13 pool.query (REST GET /pool) carries no flat size/allocated/free and
// no zpool properties. Its only pool-capacity source is the libzfs vdev
// topology that pool_extend attaches: py-libzfs ZFSVdev.__getstate__ gives
// every vdev a "stats" object whose size/allocated are vdev_stat_t
// vs_space/vs_alloc. OpenZFS maintains those only on top-level vdevs
// (vdev_space_update), and zpool's own size/allocated properties sum the
// normal, special and dedup classes (spa_prop_get_config). Separate log, cache
// and spare vdevs are not pool capacity. Leaf children of a mirror/raidz
// report zero space. See truenas/middleware truenas/13.0-stable
// plugins/pool.py pool_extend and truenas/py-libzfs truenas/13.0-stable
// libzfs.pyx ZFSVdevStats (#2510).

const (
	coreMirrorSize      = int64(3985729650688)
	coreMirrorAllocated = int64(127817449472)
	coreSpecialSize     = int64(495926132736)
	coreSpecialAlloc    = int64(2147483648)
)

func coreVdevStats(size, allocated int64) string {
	return fmt.Sprintf(`{"timestamp":1760650000000000,"read_errors":0,"write_errors":0,"checksum_errors":0,"ops":[0,120,340,0,0,9,0],"bytes":[0,4096,8192,0,0,1024,0],"size":%d,"allocated":%d,"fragmentation":3,"self_healed":0,"configured_ashift":12,"logical_ashift":9,"physical_ashift":12}`, size, allocated)
}

func coreLeaf(name, gptid, guid string) string {
	return fmt.Sprintf(`{"type":"DISK","path":"/dev/gptid/%s","guid":"%s","status":"ONLINE","stats":%s,"children":[],"device":"%sp2","disk":"%s","unavail_disk":null}`, gptid, guid, coreVdevStats(0, 0), name, name)
}

// corePoolRow mirrors a CORE 13.0-U6 pool_extend row: datastore columns,
// zpool status/scan/healthy, transformed topology, autotrim, encryption.
func corePoolRow(topology string) string {
	return `{"id":1,"name":"Intenso","guid":"4421337198572811350","encrypt":0,"encryptkey":"","path":"/mnt/Intenso","status":"ONLINE","scan":{"function":"SCRUB","state":"FINISHED","start_time":{"$date":1760140800000},"end_time":{"$date":1760144400000},"percentage":100.0,"bytes_to_process":127817449472,"bytes_processed":127817449472,"bytes_issued":127817449472,"pause":null,"errors":0,"total_secs_left":null},"topology":` + topology + `,"healthy":true,"status_detail":null,"autotrim":{"value":"off","rawvalue":"off","parsed":"off","source":"DEFAULT"},"is_decrypted":true,"encryptkey_path":null}`
}

func coreTopology(dataStats, specialStats string) string {
	mirror := `{"type":"MIRROR","path":null,"guid":"9182736455463728190","status":"ONLINE","stats":` + dataStats + `,"children":[` +
		coreLeaf("da1", "3f1c0e3a-aa11-11ee-9c1d-0cc47a1b2c3d", "1111") + `,` +
		coreLeaf("da2", "3f2d1f4b-aa11-11ee-9c1d-0cc47a1b2c3d", "2222") + `],"unavail_disk":null}`
	special := `[]`
	if specialStats != "" {
		special = `[{"type":"DISK","path":"/dev/gptid/5a6b7c8d-aa11-11ee-9c1d-0cc47a1b2c3d","guid":"3333","status":"ONLINE","stats":` + specialStats + `,"children":[],"device":"nvd0p1","disk":"nvd0","unavail_disk":null}]`
	}
	// The log, cache and spare vdevs carry their own non-zero space and must
	// not enter pool capacity.
	log := `[{"type":"DISK","path":"/dev/gptid/6b7c8d9e-aa11-11ee-9c1d-0cc47a1b2c3d","guid":"4444","status":"ONLINE","stats":` + coreVdevStats(15032385536, 1048576) + `,"children":[],"device":"nvd1p1","disk":"nvd1","unavail_disk":null}]`
	cache := `[{"type":"DISK","path":"/dev/gptid/7c8d9e0f-aa11-11ee-9c1d-0cc47a1b2c3d","guid":"5555","status":"ONLINE","stats":` + coreVdevStats(250059350016, 0) + `,"children":[],"device":"nvd2p1","disk":"nvd2","unavail_disk":null}]`
	spare := `[{"type":"DISK","path":"/dev/gptid/8d9e0f1a-aa11-11ee-9c1d-0cc47a1b2c3d","guid":"6666","status":"AVAIL","stats":` + coreVdevStats(0, 0) + `,"children":[],"device":"da3p2","disk":"da3","unavail_disk":null}]`
	return `{"data":[` + mirror + `],"log":` + log + `,"cache":` + cache + `,"spare":` + spare + `,"special":` + special + `,"dedup":[]}`
}

// boot.get_state on CORE is zfs.pool.query: capacity lives in properties.
const coreBootState = `{"name":"boot-pool","id":"boot-pool","guid":"7766554433221100","hostname":"core","status":"ONLINE","healthy":true,"warning":false,"status_detail":null,"error_count":0,"properties":{"size":{"value":"15.5G","rawvalue":"16642998272","parsed":16642998272,"source":"NONE"},"allocated":{"value":"2.31G","rawvalue":"2480377856","parsed":2480377856,"source":"NONE"},"free":{"value":"13.2G","rawvalue":"14162620416","parsed":14162620416,"source":"NONE"}},"groups":{"data":[{"type":"disk","path":"/dev/da0p2","guid":"7777","status":"ONLINE","stats":{"size":16642998272,"allocated":2480377856},"children":[]}],"log":[],"cache":[],"spare":[],"special":[],"dedup":[]},"scan":null}`

func coreCapacityServer(t *testing.T, poolBody string) *httptest.Server {
	t.Helper()
	responses := defaultAPIResponses()
	responses["/api/v2.0/system/info"] = apiResponse{body: `{"hostname":"core","version":"TrueNAS-13.0-U6.8","cores":4,"physmem":4096}`}
	responses["/api/v2.0/pool"] = apiResponse{body: poolBody}
	responses["/api/v2.0/boot/get_state"] = apiResponse{body: coreBootState}
	responses["/api/v2.0/pool/dataset"] = apiResponse{body: `[{"id":"Intenso","name":"Intenso","pool":"Intenso","used":{"rawvalue":"119000000000","parsed":119000000000},"available":{"rawvalue":"2780000000000","parsed":2780000000000},"mountpoint":"/mnt/Intenso","readonly":{"rawvalue":"off","parsed":false}}]`}
	responses["/api/v2.0/disk"] = apiResponse{body: `[{"identifier":"{serial}DA1","name":"da1","serial":"DA1","size":4000787030016,"model":"QEMU HARDDISK","type":"HDD","pool":"Intenso","bus":"SCSI","rotationrate":"unknown","status":"ONLINE"},{"identifier":"{serial}DA2","name":"da2","serial":"DA2","size":4000787030016,"model":"QEMU HARDDISK","type":"HDD","pool":"Intenso","bus":"SCSI","rotationrate":"unknown","status":"ONLINE"}]`}
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
	return server
}

func TestTrueNASCOREPoolCapacityFromTopologyStats(t *testing.T) {
	enableTrueNASFeatureFlag(t)
	wantTotal := coreMirrorSize + coreSpecialSize
	wantUsed := coreMirrorAllocated + coreSpecialAlloc
	server := coreCapacityServer(t, `[`+corePoolRow(coreTopology(coreVdevStats(coreMirrorSize, coreMirrorAllocated), coreVdevStats(coreSpecialSize, coreSpecialAlloc)))+`]`)
	client := protocolFixtureClient(t, server.URL, ClientConfig{APIKey: "synthetic-key"})

	snapshot, err := client.FetchSnapshot(context.Background())
	if err != nil {
		t.Fatalf("FetchSnapshot: %v", err)
	}
	var data, boot *Pool
	for i := range snapshot.Pools {
		switch snapshot.Pools[i].Name {
		case "Intenso":
			data = &snapshot.Pools[i]
		case "boot-pool":
			boot = &snapshot.Pools[i]
		}
	}
	if data == nil || boot == nil || len(snapshot.Pools) != 2 {
		t.Fatalf("pools = %+v", snapshot.Pools)
	}
	if data.TotalBytes != wantTotal || data.UsedBytes != wantUsed || data.FreeBytes != wantTotal-wantUsed {
		t.Fatalf("CORE data-pool capacity = %d/%d/%d, want %d/%d/%d", data.TotalBytes, data.UsedBytes, data.FreeBytes, wantTotal, wantUsed, wantTotal-wantUsed)
	}
	if data.GUID != "4421337198572811350" || data.Status != "ONLINE" || data.Scan == nil || data.Scan.Function != "SCRUB" {
		t.Fatalf("capacity source lost pool identity/health/scan: %+v", data)
	}
	if len(data.DiskMembers) != 6 || len(data.VDevs) != 7 {
		t.Fatalf("capacity source lost topology: members=%d vdevs=%d", len(data.DiskMembers), len(data.VDevs))
	}
	if !boot.IsBoot || boot.TotalBytes != 16642998272 || boot.UsedBytes != 2480377856 {
		t.Fatalf("boot-pool capacity changed: %+v", boot)
	}

	registry := unifiedresources.NewRegistry(unifiedresources.NewMemoryStore())
	registry.IngestRecords(unifiedresources.SourceTrueNAS, NewProvider(*snapshot).Records())
	foundPool, foundHost := false, false
	for _, resource := range registry.List() {
		if resource.Type == unifiedresources.ResourceTypeAgent {
			foundHost = true
			metric := resource.Metrics.Disk
			if metric == nil || metric.Total == nil || *metric.Total != wantTotal+16642998272 {
				t.Fatalf("host storage subtotal = %+v", metric)
			}
		}
		if resource.Storage == nil || resource.Storage.Type != "zfs-pool" || resource.Name != "Intenso" {
			continue
		}
		foundPool = true
		metric := resource.Metrics.Disk
		if metric == nil || metric.Total == nil || *metric.Total != wantTotal || metric.Used == nil || *metric.Used != wantUsed {
			t.Fatalf("CORE pool capacity lost at ingestion: %+v", metric)
		}
	}
	if !foundPool || !foundHost {
		t.Fatal("snapshot/provider/registry path lost the CORE pool or host")
	}
}

func TestTrueNASCOREPoolTopologyCapacityFailsClosed(t *testing.T) {
	enableTrueNASFeatureFlag(t)
	good := coreVdevStats(coreMirrorSize, coreMirrorAllocated)
	const huge = int64(9007199254740993) // Above float64's exact integer range.
	for _, tc := range []struct {
		name  string
		row   string
		total int64
		used  int64
	}{
		{"data only", corePoolRow(coreTopology(good, "")), coreMirrorSize, coreMirrorAllocated},
		{"exact bytes above 2^53", corePoolRow(coreTopology(coreVdevStats(huge, 1), "")), huge, 1},
		{"flat bytes win over topology", strings.Replace(corePoolRow(coreTopology(good, "")), `"id":1,`, `"id":1,"size":1000,"allocated":400,"free":600,`, 1), 1000, 400},
		{"null flat bytes stay unknown", strings.Replace(corePoolRow(coreTopology(good, "")), `"id":1,`, `"id":1,"size":null,"allocated":null,"free":null,`, 1), 0, 0},
		{"top-level vdev without stats", corePoolRow(coreTopology(`null`, "")), 0, 0},
		{"one counted vdev without stats", corePoolRow(coreTopology(good, `{"read_errors":0}`)), 0, 0},
		{"unknown size", corePoolRow(coreTopology(`{"size":"unknown","allocated":1}`, "")), 0, 0},
		{"fractional size", corePoolRow(coreTopology(`{"size":1000.5,"allocated":1}`, "")), 0, 0},
		{"allocated above size", corePoolRow(coreTopology(coreVdevStats(1000, 1001), "")), 0, 0},
		{"zero size", corePoolRow(coreTopology(coreVdevStats(0, 0), "")), 0, 0},
		{"overflowing sum", corePoolRow(coreTopology(coreVdevStats(9223372036854775000, 0), coreVdevStats(9223372036854775000, 0))), 0, 0},
		{"offline pool without topology", strings.Replace(corePoolRow(`null`), `"status":"ONLINE"`, `"status":"OFFLINE"`, 1), 0, 0},
		{"log and cache alone are not a pool", corePoolRow(`{"data":[],"log":[{"type":"DISK","stats":` + good + `}],"cache":[{"type":"DISK","stats":` + good + `}]}`), 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := coreCapacityServer(t, `[`+tc.row+`]`)
			client := protocolFixtureClient(t, server.URL, ClientConfig{APIKey: "synthetic-key"})
			pools, err := client.GetPools(context.Background())
			if err != nil || len(pools) != 1 {
				t.Fatalf("GetPools = %+v, %v", pools, err)
			}
			pool := pools[0]
			if pool.TotalBytes != tc.total || pool.UsedBytes != tc.used || pool.FreeBytes != tc.total-tc.used {
				t.Fatalf("capacity = %d/%d/%d, want %d/%d/%d", pool.TotalBytes, pool.UsedBytes, pool.FreeBytes, tc.total, tc.used, tc.total-tc.used)
			}
		})
	}
}

// The zfs.pool.query shape (boot.get_state) keeps reading its properties;
// topology is consulted only when neither flat bytes nor properties exist.
func TestTrueNASPoolStateTopologyCapacityIsLastSource(t *testing.T) {
	groups := map[string]any{"data": []any{map[string]any{"type": "disk", "stats": map[string]any{"size": int64(5000), "allocated": int64(100)}}}}
	properties := map[string]any{"size": map[string]any{"parsed": int64(2000)}, "allocated": map[string]any{"parsed": int64(900)}}
	pool, ok := parseBootPoolState(map[string]any{"name": "boot-pool", "groups": groups, "properties": properties})
	if !ok || pool.TotalBytes != 2000 || pool.UsedBytes != 900 {
		t.Fatalf("properties lost to topology: %+v", pool)
	}
	pool, ok = parsePoolState(map[string]any{"name": "tank", "topology": groups}, false)
	if !ok || pool.TotalBytes != 5000 || pool.UsedBytes != 100 || pool.FreeBytes != 4900 {
		t.Fatalf("topology-only pool state = %+v", pool)
	}
}
