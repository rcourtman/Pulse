package truenas

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"
)

// The real pool.dataset.query response has never carried a "mounted" field on
// any TrueNAS version (CORE 13 and SCALE both strip it from the property
// allowlist), so its absence must not read as "unmounted" — that rendered
// every dataset Offline (#1573). "locked" is the one unmounted-like state the
// API does report.
func TestGetDatasetsRESTDefaultsMountedWhenFieldAbsent(t *testing.T) {
	server := newMockServer(t, map[string]apiResponse{
		"/api/v2.0/pool/dataset": {
			body: `[
				{"id":"tank/media","name":"tank/media","pool":"tank","used":{"parsed":1},"available":{"parsed":2},"readonly":{"parsed":false},"mountpoint":"/mnt/tank/media"},
				{"id":"tank/vault","name":"tank/vault","pool":"tank","used":{"parsed":1},"available":{"parsed":2},"readonly":{"parsed":false},"mountpoint":"/mnt/tank/vault","locked":true},
				{"id":"tank/legacy","name":"tank/legacy","pool":"tank","used":{"parsed":1},"available":{"parsed":2},"readonly":{"parsed":false},"mountpoint":"/mnt/tank/legacy","mounted":false}
			]`,
		},
	}, nil)
	t.Cleanup(server.Close)

	client := mustClientForServer(t, server.URL, ClientConfig{APIKey: "api-key"})
	datasets, err := client.getDatasetsREST(context.Background())
	if err != nil {
		t.Fatalf("getDatasetsREST() error = %v", err)
	}
	if len(datasets) != 3 {
		t.Fatalf("expected 3 datasets, got %d", len(datasets))
	}
	if !datasets[0].Mounted {
		t.Fatalf("dataset without mounted field must default to mounted: %+v", datasets[0])
	}
	if datasets[1].Mounted {
		t.Fatalf("locked dataset must not count as mounted: %+v", datasets[1])
	}
	if datasets[2].Mounted {
		t.Fatalf("explicit mounted=false must be honored: %+v", datasets[2])
	}
}

func TestDatasetsFromRPCMapsDefaultMountedWhenFieldAbsent(t *testing.T) {
	server := newMockServer(t, map[string]apiResponse{}, nil)
	t.Cleanup(server.Close)
	client := mustClientForServer(t, server.URL, ClientConfig{APIKey: "api-key"})
	_ = client

	items := []map[string]any{
		{"id": "tank/media", "name": "tank/media", "pool": "tank", "used": map[string]any{"parsed": 1}, "available": map[string]any{"parsed": 2}},
		{"id": "tank/vault", "name": "tank/vault", "pool": "tank", "locked": true},
		{"id": "tank/legacy", "name": "tank/legacy", "pool": "tank", "mounted": false},
	}
	// Mirror getDatasetsRPC's mounted derivation on raw maps.
	mounted := func(item map[string]any) bool {
		return readBoolAnyDefault(item, true, "mounted") && !readBoolAny(item, "locked")
	}
	if !mounted(items[0]) {
		t.Fatal("dataset without mounted field must default to mounted")
	}
	if mounted(items[1]) {
		t.Fatal("locked dataset must not count as mounted")
	}
	if mounted(items[2]) {
		t.Fatal("explicit mounted=false must be honored")
	}
}

// disk.temperatures takes parameters, so REST v2.0 serves it as POST with a
// body keyed by parameter name on every TrueNAS version; the old GET always
// failed, which left disk temperatures blank wherever the JSON-RPC endpoint
// does not exist (CORE 13, SCALE < 25.04) (#1573).
func TestDiskTemperaturesFallBackToRESTPost(t *testing.T) {
	sawPost := false
	server := newMockServer(t, map[string]apiResponse{
		"/api/v2.0/disk/temperatures": {
			body: `{"ada0":33,"ada1":null}`,
		},
	}, func(t *testing.T, request *http.Request) {
		if request.URL.Path == "/api/v2.0/disk/temperatures" {
			if request.Method != http.MethodPost {
				t.Fatalf("disk/temperatures must be requested via POST, got %s", request.Method)
			}
			sawPost = true
		}
	})
	t.Cleanup(server.Close)

	client := mustClientForServer(t, server.URL, ClientConfig{APIKey: "api-key"})
	temperatures, err := client.getDiskTemperaturesWithFallback(context.Background(), []string{"ada0", "ada1"})
	if err != nil {
		t.Fatalf("getDiskTemperaturesWithFallback() error = %v", err)
	}
	if !sawPost {
		t.Fatal("expected a POST request to /disk/temperatures")
	}
	if len(temperatures) != 1 || temperatures["ada0"] != 33 {
		t.Fatalf("unexpected temperatures: %#v", temperatures)
	}
}

func TestPoolDiskMembersFromTopology(t *testing.T) {
	topology := map[string]any{
		"data": []any{
			map[string]any{
				"type":   "RAIDZ1",
				"status": "DEGRADED",
				"children": []any{
					map[string]any{
						"type": "DISK", "status": "ONLINE",
						"device": "ada0p2", "disk": "ada0",
						"children": []any{},
					},
					map[string]any{
						"type": "DISK", "status": "UNAVAIL",
						"device": "", "disk": "",
						"unavail_disk": map[string]any{"devname": "ada1"},
						"children":     []any{},
					},
				},
			},
		},
		"spare": []any{
			map[string]any{"type": "DISK", "status": "AVAIL", "device": "da9p1", "disk": "da9", "children": []any{}},
		},
		"log": []any{
			map[string]any{"type": "DISK", "status": "ONLINE", "path": "/dev/nvme0n1p3", "children": []any{}},
		},
	}

	members := poolDiskMembersFromTopology(topology)
	if len(members) != 4 {
		t.Fatalf("expected 4 members, got %#v", members)
	}
	byDisk := map[string]PoolDiskMember{}
	for _, member := range members {
		key := member.Disk
		if key == "" {
			key = member.Device
		}
		byDisk[key] = member
	}
	if byDisk["ada0"].Status != "ONLINE" {
		t.Fatalf("expected ada0 ONLINE, got %#v", byDisk["ada0"])
	}
	if byDisk["ada1"].Status != "UNAVAIL" {
		t.Fatalf("expected unavail_disk member ada1 UNAVAIL, got %#v", byDisk["ada1"])
	}
	if byDisk["da9"].Status != "AVAIL" {
		t.Fatalf("expected spare da9 AVAIL, got %#v", byDisk["da9"])
	}
	if byDisk["nvme0n1"].Device != "nvme0n1p3" || byDisk["nvme0n1"].Status != "ONLINE" {
		t.Fatalf("expected path-only nvme0n1 ONLINE, got %#v", byDisk["nvme0n1"])
	}
}

func TestParseBootPoolStateSupportsCOREGroupsShape(t *testing.T) {
	item := map[string]any{
		"id":      "freenas-boot",
		"name":    "freenas-boot",
		"status":  "ONLINE",
		"healthy": true,
		"properties": map[string]any{
			"size":      map[string]any{"parsed": int64(255060803584)},
			"allocated": map[string]any{"parsed": int64(16224641024)},
			"free":      map[string]any{"parsed": int64(238836162560)},
		},
		"groups": map[string]any{
			"data": []any{
				map[string]any{
					"type":   "MIRROR",
					"status": "ONLINE",
					"children": []any{
						map[string]any{"type": "DISK", "status": "ONLINE", "path": "/dev/ada4p2", "children": []any{}},
						map[string]any{"type": "DISK", "status": "ONLINE", "path": "/dev/ada5p2", "children": []any{}},
					},
				},
			},
		},
	}

	pool, ok := parseBootPoolState(item)
	if !ok {
		t.Fatal("expected CORE boot.get_state payload to parse")
	}
	if !pool.IsBoot || pool.Name != "freenas-boot" || pool.Status != "ONLINE" {
		t.Fatalf("unexpected boot pool identity/state: %+v", pool)
	}
	if pool.TotalBytes != 255060803584 || pool.UsedBytes != 16224641024 || pool.FreeBytes != 238836162560 {
		t.Fatalf("unexpected boot pool capacity: %+v", pool)
	}
	if len(pool.DiskMembers) != 2 || pool.DiskMembers[0].Disk != "ada4" || pool.DiskMembers[1].Disk != "ada5" {
		t.Fatalf("unexpected boot pool members: %+v", pool.DiskMembers)
	}
}

func TestGetBootPoolFallsBackToCORERESTShape(t *testing.T) {
	sawREST := false
	server := newMockServer(t, map[string]apiResponse{
		"/api/v2.0/boot/get_state": {
			body: `{
				"id":"freenas-boot",
				"name":"freenas-boot",
				"status":"ONLINE",
				"healthy":true,
				"properties":{
					"size":{"parsed":255060803584},
					"allocated":{"parsed":16224641024},
					"free":{"parsed":238836162560}
				},
				"groups":{"data":[{"type":"MIRROR","status":"ONLINE","children":[
					{"type":"DISK","status":"ONLINE","path":"/dev/ada4p2","children":[]},
					{"type":"DISK","status":"ONLINE","path":"/dev/ada5p2","children":[]}
				]}]}
			}`,
		},
	}, func(t *testing.T, request *http.Request) {
		if request.URL.Path == "/api/v2.0/boot/get_state" {
			sawREST = true
			if request.Method != http.MethodGet {
				t.Fatalf("boot/get_state must be requested via GET, got %s", request.Method)
			}
		}
	})
	t.Cleanup(server.Close)

	client := mustClientForServer(t, server.URL, ClientConfig{APIKey: "api-key"})
	pool, err := client.GetBootPool(context.Background())
	if err != nil {
		t.Fatalf("GetBootPool() error = %v", err)
	}
	if !sawREST {
		t.Fatal("expected REST boot/get_state fallback")
	}
	if pool == nil || !pool.IsBoot || pool.Status != "ONLINE" || len(pool.DiskMembers) != 2 {
		t.Fatalf("unexpected boot pool: %+v", pool)
	}
}

func TestWholeDiskFromDevicePreservesWholeDiskNames(t *testing.T) {
	tests := map[string]string{
		"ada4p2":    "ada4",
		"nvme0n1p3": "nvme0n1",
		"sda3":      "sda",
		"ada4":      "ada4",
		"da9":       "da9",
		"nvme0n1":   "nvme0n1",
	}
	for device, want := range tests {
		if got := wholeDiskFromDevice(device); got != want {
			t.Fatalf("wholeDiskFromDevice(%q) = %q, want %q", device, got, want)
		}
	}
}

func TestMergeBootPoolDoesNotDuplicatePoolQueryIdentity(t *testing.T) {
	pools := []Pool{{
		ID:         "freenas-boot",
		Name:       "freenas-boot",
		Status:     "",
		TotalBytes: 100,
	}}
	boot := Pool{
		ID:          "freenas-boot",
		Name:        "freenas-boot",
		Status:      "ONLINE",
		TotalBytes:  100,
		UsedBytes:   40,
		FreeBytes:   60,
		IsBoot:      true,
		DiskMembers: []PoolDiskMember{{Disk: "ada4", Device: "ada4p2", Status: "ONLINE"}},
	}

	merged := mergeBootPool(pools, boot)
	if len(merged) != 1 {
		t.Fatalf("expected one connection-local pool identity, got %+v", merged)
	}
	if !merged[0].IsBoot || merged[0].Status != "ONLINE" || len(merged[0].DiskMembers) != 1 {
		t.Fatalf("expected boot state to enrich pool.query record, got %+v", merged[0])
	}
}

func TestEnrichDisksFromPoolTopology(t *testing.T) {
	pools := []Pool{{
		Name: "tank",
		DiskMembers: []PoolDiskMember{
			{Disk: "ada0", Device: "ada0p2", Status: "ONLINE"},
			{Disk: "ada1", Device: "ada1p2", Status: "FAULTED"},
		},
	}}
	disks := []Disk{
		{ID: "{serial}A", Name: "ada0"},
		{ID: "{serial}B", Name: "ada1"},
		{ID: "{serial}C", Name: "ada2"},
		{ID: "{serial}D", Name: "ada3", Pool: "other", Status: "ONLINE"},
	}

	enrichDisksFromPoolTopology(pools, disks)

	if disks[0].Pool != "tank" || disks[0].Status != "ONLINE" {
		t.Fatalf("expected ada0 in tank/ONLINE, got %+v", disks[0])
	}
	if disks[1].Pool != "tank" || disks[1].Status != "FAULTED" {
		t.Fatalf("expected ada1 in tank/FAULTED, got %+v", disks[1])
	}
	if disks[2].Pool != "" || disks[2].Status != "" {
		t.Fatalf("unassigned disk must stay unassigned, got %+v", disks[2])
	}
	if disks[3].Pool != "other" || disks[3].Status != "ONLINE" {
		t.Fatalf("explicit disk fields must not be overwritten, got %+v", disks[3])
	}
}

func TestParsePoolStatePreservesScanTopologyErrorsAndNativeMissingEvidence(t *testing.T) {
	item := map[string]any{
		"id":            float64(7),
		"guid":          "pool-guid-7",
		"name":          "tank",
		"status":        "DEGRADED",
		"status_code":   "FEAT_DISABLED",
		"status_detail": "One or more devices is unavailable",
		"scan": map[string]any{
			"function":         "RESILVER",
			"state":            "SCANNING",
			"percentage":       42.5,
			"errors":           float64(0),
			"bytes_examined":   float64(1024),
			"bytes_to_process": float64(4096),
			"total_secs_left":  float64(120),
			"start_time":       "2026-07-24T09:00:00Z",
		},
		"topology": map[string]any{
			"data": []any{
				map[string]any{
					"guid":   "mirror-guid",
					"name":   "mirror-0",
					"type":   "MIRROR",
					"status": "DEGRADED",
					"stats": map[string]any{
						"read_errors":     float64(1),
						"write_errors":    float64(2),
						"checksum_errors": float64(3),
					},
					"children": []any{
						map[string]any{
							"guid":   "disk-guid-0",
							"type":   "DISK",
							"disk":   "sda",
							"path":   "/dev/sda2",
							"status": "ONLINE",
						},
						map[string]any{
							"guid":   "disk-guid-1",
							"type":   "UNAVAIL_DISK",
							"path":   "/dev/disk/by-partuuid/missing-member",
							"status": "UNAVAIL",
							"unavail_disk": map[string]any{
								"devname": "sdb",
							},
						},
					},
				},
			},
			"spare": []any{
				map[string]any{
					"guid":   "spare-guid",
					"type":   "DISK",
					"disk":   "sdc",
					"path":   "/dev/sdc",
					"status": "AVAIL",
				},
			},
		},
	}

	pool, ok := parsePoolState(item, false)
	if !ok {
		t.Fatal("expected pool")
	}
	if pool.GUID != "pool-guid-7" || pool.StatusCode != "FEAT_DISABLED" || pool.StatusDetail == "" {
		t.Fatalf("native pool identity/status evidence lost: %+v", pool)
	}
	if pool.Scan == nil || pool.Scan.Function != "RESILVER" || pool.Scan.State != "SCANNING" || pool.Scan.Percentage != 42.5 || pool.Scan.TotalSecondsRemaining != 120 {
		t.Fatalf("structured scan evidence = %+v", pool.Scan)
	}
	if pool.ReadErrors != 1 || pool.WriteErrors != 2 || pool.ChecksumErrors != 3 {
		t.Fatalf("pool error totals = read %d write %d checksum %d", pool.ReadErrors, pool.WriteErrors, pool.ChecksumErrors)
	}
	if len(pool.VDevs) != 4 {
		t.Fatalf("vdevs = %+v", pool.VDevs)
	}
	if len(pool.DiskMembers) != 3 {
		t.Fatalf("disk members = %+v", pool.DiskMembers)
	}
	missing := pool.DiskMembers[1]
	if missing.Disk != "sdb" || !missing.Missing || missing.Role != "data" || missing.Path != "/dev/disk/by-partuuid/missing-member" {
		t.Fatalf("missing member evidence = %+v", missing)
	}
	spare := pool.DiskMembers[2]
	if spare.Disk != "sdc" || spare.Status != "AVAIL" || spare.Role != "spare" {
		t.Fatalf("spare member evidence = %+v", spare)
	}

	disks := enrichDisksFromPoolTopology([]Pool{pool}, []Disk{{ID: "sda", Name: "sda"}})
	if len(disks) != 2 {
		t.Fatalf("expected only explicit unavailable member to synthesize, got %+v", disks)
	}
	if disks[1].Name != "sdb" || disks[1].Pool != "tank" || disks[1].Status != "UNAVAIL" {
		t.Fatalf("synthetic unavailable disk = %+v", disks[1])
	}
}

// The CORE 13 virtual-disk shape from #2466, with synthetic identity. Exercise
// the complete real client path, not just the optional field's decoder.
func TestRESTDiskRotationRateSnapshot(t *testing.T) {
	for _, tc := range []struct {
		name, field, diskType string
		rotational            bool
	}{
		{"unknown", `,"rotationrate":"unknown"`, "UNKNOWN", false},
		{"ssd-sentinel", `,"rotationrate":"ssd"`, "UNKNOWN", false},
		{"integer", `,"rotationrate":7200`, "UNKNOWN", true},
		{"numeric-string", `,"rotationrate":" 7200 "`, "UNKNOWN", true},
		{"zero", `,"rotationrate":0`, "UNKNOWN", false},
		{"null", `,"rotationrate":null`, "UNKNOWN", false},
		{"absent", "", "UNKNOWN", false},
		{"hdd-fallback", `,"rotationrate":"unknown"`, "HDD", true},
		{"ssd-fallback", `,"rotationrate":null`, "SSD", false},
		{"out-of-range", `,"rotationrate":4294967296`, "UNKNOWN", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			routes := alertArgsTransport(defaultAPIResponses())
			routes["/api/v2.0/system/info"] = apiResponse{body: `{"hostname":"core-nas","version":"TrueNAS-13.0-U6.8"}`}
			// A bad optional classifier on one disk must not remove its healthy
			// neighbours, their actual SMART evidence or their temperatures.
			body := fmt.Sprintf(`[{"identifier":"{uuid}synthetic-da0","name":"da0","serial":"","size":34359738368,"model":"QEMU QEMU HARDDISK","transfermode":"Auto","hddstandby":"ALWAYS ON","togglesmart":true,"type":%q,"bus":"VTSCSI"%s},{"identifier":"{disk-1}","name":"sda","serial":"SER-A","size":1000000,"model":"Seagate","type":"HDD","pool":"tank","bus":"SATA","rotationrate":7200,"status":"ONLINE","smart_status":"FAILED"},{"identifier":"{disk-2}","name":"nvme0n1","serial":"SER-B","size":2000000,"model":"Samsung","type":"SSD","pool":"tank","bus":"NVMe","rotationrate":0,"status":"ONLINE"}]`, tc.diskType, tc.field)
			routes["/api/v2.0/disk"] = apiResponse{body: body}
			routes["/api/v2.0/alert/list"] = apiResponse{body: `[{"id":"smart","level":"CRITICAL","formatted":"Disk errors","klass":"SMARTUncorrectedErrorsAlert","args":{"ue":53,"name":"/dev/sda","serial":"SER-A"},"datetime":{"$date":1707400000000}},{"id":"string","level":"WARNING","formatted":"Pool warning","args":"boot-pool","datetime":{"$date":1707400000000}},{"id":"null","level":"INFO","formatted":"Notice","args":null,"datetime":{"$date":1707400000000}}]`}
			routes["/api/v2.0/zfs/snapshot"] = apiResponse{body: `[{"id":"tank/apps@daily","dataset":"tank/apps","snapshot_name":"daily","created_at":1707400000}]`}
			routes["/api/v2.0/replication"] = apiResponse{body: `[{"id":1,"name":"daily-copy","source_datasets":["tank/apps"],"target_dataset":"remote/apps","direction":"PUSH","transport":"SSH","state":"FINISHED"}]`}
			client, err := NewClient(ClientConfig{Host: "http://truenas.invalid", APIKey: "synthetic"})
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			client.httpClient.Transport = routes
			client.mode = TransportLegacyREST
			ctx := context.Background()
			if err := client.TestConnection(ctx); err != nil {
				t.Fatal(err)
			}
			provider := NewLiveProviderForConnection(&APIFetcher{Client: client}, "core-connection")
			for cycle := 0; cycle < 2; cycle++ {
				if err := provider.Refresh(ctx); err != nil {
					t.Fatalf("successful Test Connection must permit complete inventory, cycle %d: %v", cycle, err)
				}
				s := provider.Snapshot()
				if s == nil || s.System.Hostname != "core-nas" || s.CollectedAt.IsZero() || len(s.Pools) != 1 || len(s.Datasets) != 1 || len(s.Disks) != 3 || len(s.Alerts) != 3 || len(s.Services) != 2 || len(s.Apps) != 1 || len(s.VMs) != 1 || len(s.Shares) != 2 || len(s.ZFSSnapshots) != 1 || len(s.ReplicationTasks) != 1 {
					t.Fatalf("incomplete CORE snapshot: %+v", s)
				}
				d := s.Disks[0]
				if d.ID != "{uuid}synthetic-da0" || d.Name != "da0" || d.SizeBytes != 34359738368 || d.Model != "QEMU QEMU HARDDISK" || d.Transport != "vtscsi" || d.Rotational != tc.rotational || d.Serial != "" || d.Pool != "" || d.Status != "" || d.HealthStatusPresent || d.Health != "" || d.Temperature != 0 {
					t.Fatalf("lost identity or fabricated virtual-disk observations: %+v", d)
				}
				if h := s.Disks[1]; h.Serial != "SER-A" || h.Pool != "tank" || h.Transport != "sata" || !h.Rotational || !h.HealthStatusPresent || h.Health != "FAILED" || h.Temperature != 34 {
					t.Fatalf("lost independent HDD evidence: %+v", h)
				}
				if h := s.Disks[2]; h.Serial != "SER-B" || h.Transport != "nvme" || h.Rotational || h.Temperature != 49 {
					t.Fatalf("lost independent SSD evidence: %+v", h)
				}
				if a := s.Alerts[0]; !a.SMARTUncorrectedReported || a.SMARTUncorrectedErrors != 53 || a.DiskName != "/dev/sda" || a.DiskSerial != "SER-A" {
					t.Fatalf("lost object alert evidence: %+v", a)
				}
				for _, a := range s.Alerts[1:] {
					if a.SMARTUncorrectedReported || a.SMARTAvailableSpareReported || a.DiskName != "" || a.DiskSerial != "" || !a.Datetime.Equal(time.UnixMilli(1707400000000).UTC()) {
						t.Fatalf("invented metadata from mixed alert args: %+v", a)
					}
				}
				for _, record := range provider.RecordsFromSnapshot(s) {
					if record.Resource.Name == "da0" {
						meta := record.Resource.PhysicalDisk
						if meta == nil || meta.Health != "UNKNOWN" || meta.DiskType != "vtscsi" || meta.Temperature != 0 || (!tc.rotational && meta.RPM != 0) {
							t.Fatalf("unknown disk must remain unknown in canonical projection: %+v", meta)
						}
					}
				}
			}
			var rpcShape []map[string]any
			if err := json.Unmarshal([]byte(body), &rpcShape); err != nil {
				t.Fatal(err)
			}
			rest, err := client.GetDisks(ctx)
			if err != nil {
				t.Fatal(err)
			}
			rpc, err := client.disksFromMaps(ctx, rpcShape)
			if err != nil || !reflect.DeepEqual(rest, rpc) {
				t.Fatalf("REST/RPC disk evidence differs: REST=%+v RPC=%+v err=%v", rest, rpc, err)
			}
			ids, err := client.listDiskReportingIdentifiers(ctx)
			if err != nil || !reflect.DeepEqual(ids, []string{"da0", "sda", "nvme0n1"}) {
				t.Fatalf("temperature discovery must tolerate the same field: %v, %v", ids, err)
			}
		})
	}
}

func TestRESTDiskRotationRatePreservesErrorsAndTopology(t *testing.T) {
	routes := alertArgsTransport(defaultAPIResponses())
	routes["/api/v2.0/pool"] = apiResponse{body: `[{"id":1,"name":"tank","status":"DEGRADED","topology":{"data":[{"type":"DISK","disk":"da0","status":"FAULTED","path":"/dev/da0p2","stats":{"read_errors":2,"write_errors":3,"checksum_errors":4}}]}}]`}
	routes["/api/v2.0/disk"] = apiResponse{body: `[{"identifier":"{uuid}synthetic","name":"da0","model":"QEMU QEMU HARDDISK","size":34359738368,"type":"UNKNOWN","bus":"VTSCSI","rotationrate":"unknown"}]`}
	client, err := NewClient(ClientConfig{Host: "http://truenas.invalid", APIKey: "synthetic"})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	client.httpClient.Transport = routes
	client.mode = TransportLegacyREST
	s, err := client.FetchSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Disks) != 1 || s.Disks[0].Pool != "tank" || s.Disks[0].Status != "FAULTED" || healthFromDisk(s.Disks[0]) != "FAILED" || len(s.Pools[0].DiskMembers) != 1 || s.Pools[0].DiskMembers[0].ReadErrors != 2 {
		t.Fatalf("tolerant rotation rate lost real topology/health: %+v", s)
	}
	for _, tc := range []struct {
		name     string
		response apiResponse
	}{
		{"malformed-json", apiResponse{body: `[{"rotationrate":`}},
		{"wrong-inventory-shape", apiResponse{body: `{"rotationrate":"unknown"}`}},
		{"invalid-required-field", apiResponse{body: `[{"name":"da0","rotationrate":"unknown","size":"not-a-size"}]`}},
		{"authentication", apiResponse{status: 401, body: `{}`}},
		{"server", apiResponse{status: 503, body: `{}`}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			routes["/api/v2.0/disk"] = tc.response
			if result, err := client.FetchSnapshot(context.Background()); err == nil || result != nil || !strings.Contains(err.Error(), "fetch truenas disks:") {
				t.Fatalf("disk response failure must remain a failed snapshot: %+v, %v", result, err)
			}
		})
	}
}
