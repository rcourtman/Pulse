package truenas

import (
	"context"
	"net/http"
	"testing"
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
