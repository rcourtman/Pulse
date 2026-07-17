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
	}

	members := poolDiskMembersFromTopology(topology)
	if len(members) != 3 {
		t.Fatalf("expected 3 members, got %#v", members)
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
