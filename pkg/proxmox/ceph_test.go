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
