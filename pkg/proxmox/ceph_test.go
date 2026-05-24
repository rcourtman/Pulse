package proxmox

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetCephStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/cluster/ceph/status" {
			http.NotFound(w, r)
			return
		}
		resp := map[string]interface{}{
			"data": map[string]interface{}{
				"fsid": "fsid-1",
				"health": map[string]interface{}{
					"status": "HEALTH_OK",
					"summary": []map[string]interface{}{
						{"severity": "info", "summary": "ok"},
					},
					"checks": map[string]interface{}{},
				},
				"servicemap": map[string]interface{}{
					"services": map[string]interface{}{},
				},
				"monmap": map[string]interface{}{
					"num_mons": 3,
				},
				"mgrmap": map[string]interface{}{
					"num_mgrs":    2,
					"active_name": "mgr.node1",
					"standbys":    []string{"mgr.node2"},
				},
				"osdmap": map[string]interface{}{
					"num_osds":    1,
					"num_up_osds": 1,
					"num_in_osds": 1,
				},
				"pgmap": map[string]interface{}{
					"num_pgs": 5,
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := &Client{baseURL: server.URL, httpClient: server.Client()}
	status, err := client.GetCephStatus(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.FSID != "fsid-1" || status.Health.Status != "HEALTH_OK" {
		t.Fatalf("unexpected status: %+v", status)
	}
	if status.MonMap.NumMons != 3 {
		t.Fatalf("MonMap.NumMons = %d, want 3", status.MonMap.NumMons)
	}
	if status.MgrMap.NumMgrs != 2 {
		t.Fatalf("MgrMap.NumMgrs = %d, want 2", status.MgrMap.NumMgrs)
	}
	if status.MgrMap.ActiveName != "mgr.node1" {
		t.Fatalf("MgrMap.ActiveName = %q, want %q", status.MgrMap.ActiveName, "mgr.node1")
	}
}

func TestGetCephStatusHandlesStandbyObjects(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/cluster/ceph/status" {
			http.NotFound(w, r)
			return
		}
		resp := map[string]interface{}{
			"data": map[string]interface{}{
				"fsid": "fsid-1",
				"health": map[string]interface{}{
					"status":  "HEALTH_OK",
					"summary": []map[string]interface{}{},
					"checks":  map[string]interface{}{},
				},
				"servicemap": map[string]interface{}{
					"services": map[string]interface{}{},
				},
				"monmap": map[string]interface{}{
					"num_mons": 3,
				},
				"mgrmap": map[string]interface{}{
					"available":   true,
					"num_mgrs":    2,
					"active_name": "mgr.node1",
					"standbys": []map[string]interface{}{
						{"name": "mgr.node2"},
						{"name": "mgr.node3"},
					},
				},
				"osdmap": map[string]interface{}{
					"num_osds":    1,
					"num_up_osds": 1,
					"num_in_osds": 1,
				},
				"pgmap": map[string]interface{}{
					"num_pgs": 5,
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := &Client{baseURL: server.URL, httpClient: server.Client()}
	status, err := client.GetCephStatus(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(status.MgrMap.Standbys) != 2 {
		t.Fatalf("MgrMap.Standbys length = %d, want 2", len(status.MgrMap.Standbys))
	}
	if status.MgrMap.Standbys[0] != "mgr.node2" || status.MgrMap.Standbys[1] != "mgr.node3" {
		t.Fatalf("MgrMap.Standbys = %#v, want [mgr.node2 mgr.node3]", status.MgrMap.Standbys)
	}
}

func TestGetCephStatusCountsMonitorsFromDetailedMonMap(t *testing.T) {
	tests := []struct {
		name   string
		monmap map[string]interface{}
		want   int
	}{
		{
			name: "mons array without num_mons",
			monmap: map[string]interface{}{
				"mons": []map[string]interface{}{
					{"name": "mon-a", "rank": 0, "addr": "10.0.0.1:6789/0"},
					{"name": "mon-b", "rank": 1, "addr": "10.0.0.2:6789/0"},
					{"name": "mon-c", "rank": 2, "addr": "10.0.0.3:6789/0"},
				},
			},
			want: 3,
		},
		{
			name: "quorum names without num_mons",
			monmap: map[string]interface{}{
				"quorum_names": []string{"mon-a", "mon-b", "mon-c"},
			},
			want: 3,
		},
		{
			name: "quorum ranks without num_mons",
			monmap: map[string]interface{}{
				"quorum": []int{0, 1, 2},
			},
			want: 3,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/cluster/ceph/status" {
					http.NotFound(w, r)
					return
				}
				resp := map[string]interface{}{
					"data": map[string]interface{}{
						"health": map[string]interface{}{
							"status":  "HEALTH_OK",
							"summary": []map[string]interface{}{},
							"checks":  map[string]interface{}{},
						},
						"servicemap": map[string]interface{}{
							"services": map[string]interface{}{},
						},
						"monmap": tc.monmap,
						"mgrmap": map[string]interface{}{},
						"osdmap": map[string]interface{}{},
						"pgmap":  map[string]interface{}{},
					},
				}
				_ = json.NewEncoder(w).Encode(resp)
			}))
			defer server.Close()

			client := &Client{baseURL: server.URL, httpClient: server.Client()}
			status, err := client.GetCephStatus(context.Background())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if status.MonMap.NumMons != tc.want {
				t.Fatalf("MonMap.NumMons = %d, want %d", status.MonMap.NumMons, tc.want)
			}
		})
	}
}

func TestGetCephDF(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/cluster/ceph/df" {
			http.NotFound(w, r)
			return
		}
		resp := map[string]interface{}{
			"data": map[string]interface{}{
				"data": map[string]interface{}{
					"stats": map[string]interface{}{
						"total_bytes":          100,
						"total_used_bytes":     40,
						"total_avail_bytes":    60,
						"total_used_raw_bytes": 45,
						"percent_used":         40.0,
					},
					"pools": []map[string]interface{}{
						{
							"id":   1,
							"name": "pool1",
							"stats": map[string]interface{}{
								"bytes_used":   10,
								"kb_used":      20,
								"max_avail":    30,
								"objects":      40,
								"percent_used": 10.0,
								"dirty":        0,
								"stored_raw":   50,
							},
						},
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := &Client{baseURL: server.URL, httpClient: server.Client()}
	df, err := client.GetCephDF(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if df.Data.Stats.TotalBytes != 100 || len(df.Data.Pools) != 1 {
		t.Fatalf("unexpected df: %+v", df)
	}
}
