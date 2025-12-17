package ceph

import (
	"testing"
)

func TestParseStatus(t *testing.T) {
	data := []byte(`{
	  "fsid":"fsid-123",
	  "health":{
		"status":"HEALTH_WARN",
		"checks":{
		  "OSD_DOWN":{
			"severity":"HEALTH_WARN",
			"summary":{"message":"1 osd down"},
			"detail":[{"message":"osd.1 is down"}]
		  }
		}
	  },
	  "monmap":{"epoch":7,"mons":[{"name":"a","rank":0,"addr":"10.0.0.1"}]},
	  "mgrmap":{"available":true,"active_name":"mgr-a","standbys":[{"name":"mgr-b"}]},
	  "osdmap":{"epoch":3,"num_osds":3,"num_up_osds":2,"num_in_osds":1},
	  "pgmap":{
		"num_pgs":64,
		"bytes_total":1000,
		"bytes_used":250,
		"bytes_avail":750,
		"data_bytes":200,
		"degraded_ratio":0.1,
		"misplaced_ratio":0.2,
		"read_bytes_sec":1,
		"write_bytes_sec":2,
		"read_op_per_sec":3,
		"write_op_per_sec":4
	  }
	}`)

	status, err := parseStatus(data)
	if err != nil {
		t.Fatalf("parseStatus returned error: %v", err)
	}

	if status.FSID != "fsid-123" {
		t.Fatalf("expected FSID fsid-123, got %q", status.FSID)
	}
	if status.Health.Status != "HEALTH_WARN" {
		t.Fatalf("expected HEALTH_WARN, got %q", status.Health.Status)
	}
	check, ok := status.Health.Checks["OSD_DOWN"]
	if !ok || check.Severity != "HEALTH_WARN" || check.Message != "1 osd down" || len(check.Detail) != 1 {
		t.Fatalf("unexpected parsed health checks: %+v", status.Health.Checks)
	}

	if status.MonMap.NumMons != 1 || len(status.MonMap.Monitors) != 1 {
		t.Fatalf("expected 1 monitor, got %+v", status.MonMap)
	}
	if status.OSDMap.NumOSDs != 3 || status.OSDMap.NumUp != 2 || status.OSDMap.NumIn != 1 {
		t.Fatalf("unexpected OSD map: %+v", status.OSDMap)
	}
	if status.OSDMap.NumDown != 1 || status.OSDMap.NumOut != 2 {
		t.Fatalf("expected computed down/out counts, got %+v", status.OSDMap)
	}

	if status.PGMap.UsagePercent != 25.0 {
		t.Fatalf("expected usage percent 25.0, got %v", status.PGMap.UsagePercent)
	}

	if len(status.Services) != 3 {
		t.Fatalf("expected 3 service summaries, got %d", len(status.Services))
	}
}

func TestParseStatusInvalidJSON(t *testing.T) {
	_, err := parseStatus([]byte(`{not-json}`))
	if err == nil {
		t.Fatalf("expected error for invalid JSON")
	}
}

func TestParseDF(t *testing.T) {
	data := []byte(`{
	  "stats":{"total_bytes":1000,"total_used_bytes":123,"percent_used":0.1234},
	  "pools":[
		{"id":1,"name":"pool-a","stats":{"bytes_used":10,"max_avail":90,"objects":7,"percent_used":0.2}}
	  ]
	}`)

	pools, usagePercent, err := parseDF(data)
	if err != nil {
		t.Fatalf("parseDF returned error: %v", err)
	}
	if usagePercent != 12.34 {
		t.Fatalf("expected percent_used 12.34, got %v", usagePercent)
	}
	if len(pools) != 1 || pools[0].Name != "pool-a" || pools[0].PercentUsed != 20.0 {
		t.Fatalf("unexpected pools parsed: %+v", pools)
	}
}

func TestParseDFInvalidJSON(t *testing.T) {
	_, _, err := parseDF([]byte(`{not-json}`))
	if err == nil {
		t.Fatalf("expected error for invalid JSON")
	}
}

func TestBoolToInt(t *testing.T) {
	if boolToInt(true) != 1 {
		t.Fatalf("expected boolToInt(true)=1")
	}
	if boolToInt(false) != 0 {
		t.Fatalf("expected boolToInt(false)=0")
	}
}
