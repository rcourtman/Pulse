package hostagent

import (
	"context"
	"testing"
)

// Ceph Quincy+ (including Squid on PVE 9) reports monmap.num_mons and
// mgrmap.num_standbys instead of the mons/standbys arrays the legacy schema
// used. Issue #1626: the parser only read the arrays, so MON count came back 0
// and MGR count came back 1 on a 3-node cluster with an active + standby mgr.
const issue1626SquidStatusJSON = `{
  "fsid": "9d4c2f0a-1626-4f6e-9b7a-squid0000001",
  "health": {"status": "HEALTH_OK", "checks": {}, "mutes": []},
  "election_epoch": 148,
  "quorum": [0, 1, 2],
  "quorum_names": ["pve1", "pve2", "pve3"],
  "quorum_age": 4161,
  "monmap": {
    "epoch": 3,
    "min_mon_release_name": "squid",
    "num_mons": 3
  },
  "mgrmap": {
    "available": true,
    "num_standbys": 1,
    "modules": ["balancer", "crash", "devicehealth", "orchestrator"],
    "services": {}
  },
  "osdmap": {"epoch": 214, "num_osds": 6, "num_up_osds": 6, "num_in_osds": 6, "num_remapped_pgs": 0},
  "pgmap": {
    "num_pgs": 129,
    "bytes_total": 12002349744128,
    "bytes_used": 3000587436032,
    "bytes_avail": 9001762308096,
    "data_bytes": 999862272000
  }
}`

func TestIssue1626ParseCephStatusSquidSchema(t *testing.T) {
	status, err := parseCephStatus([]byte(issue1626SquidStatusJSON))
	if err != nil {
		t.Fatalf("parseCephStatus returned error: %v", err)
	}

	if status.MonMap.NumMons != 3 {
		t.Errorf("MonMap.NumMons = %d, want 3", status.MonMap.NumMons)
	}
	if status.MgrMap.NumMgrs != 2 {
		t.Errorf("MgrMap.NumMgrs = %d, want 2 (active + num_standbys)", status.MgrMap.NumMgrs)
	}
	if status.MgrMap.Standbys != 1 {
		t.Errorf("MgrMap.Standbys = %d, want 1", status.MgrMap.Standbys)
	}
	if !status.MgrMap.Available {
		t.Error("MgrMap.Available = false, want true")
	}

	var monService, mgrService *CephServiceInfo
	for i := range status.Services {
		switch status.Services[i].Type {
		case "mon":
			monService = &status.Services[i]
		case "mgr":
			mgrService = &status.Services[i]
		}
	}
	if monService == nil || monService.Running != 3 || monService.Total != 3 {
		t.Errorf("mon service = %+v, want Running 3 / Total 3", monService)
	}
	if mgrService == nil || mgrService.Running != 1 || mgrService.Total != 2 {
		t.Errorf("mgr service = %+v, want Running 1 / Total 2", mgrService)
	}
}

func TestIssue1626CollectCephSquidSchema(t *testing.T) {
	withLookPath(t, func(file string) (string, error) { return fakeCephBinary, nil })
	withCommandRunner(t, func(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
		if len(args) > 0 && args[0] == "status" {
			return []byte(issue1626SquidStatusJSON), nil, nil
		}
		return []byte(`{"stats":{},"pools":[]}`), nil, nil
	})

	status, err := CollectCeph(context.Background())
	if err != nil {
		t.Fatalf("CollectCeph returned error: %v", err)
	}
	if status == nil {
		t.Fatal("CollectCeph returned nil status")
	}
	if status.MonMap.NumMons != 3 {
		t.Errorf("MonMap.NumMons = %d, want 3", status.MonMap.NumMons)
	}
	if status.MgrMap.NumMgrs != 2 {
		t.Errorf("MgrMap.NumMgrs = %d, want 2", status.MgrMap.NumMgrs)
	}
}

// Legacy (pre-Quincy) payloads with mons/standbys arrays must keep working.
func TestIssue1626ParseCephStatusLegacySchemaUnchanged(t *testing.T) {
	legacy := []byte(`{
	  "fsid": "legacy-fsid",
	  "health": {"status": "HEALTH_OK", "checks": {}},
	  "monmap": {"epoch": 7, "mons": [
	    {"name": "a", "rank": 0, "addr": "10.0.0.1"},
	    {"name": "b", "rank": 1, "addr": "10.0.0.2"},
	    {"name": "c", "rank": 2, "addr": "10.0.0.3"}
	  ]},
	  "mgrmap": {"available": true, "active_name": "mgr-a", "standbys": [{"name": "mgr-b"}]},
	  "osdmap": {"epoch": 3, "num_osds": 3, "num_up_osds": 3, "num_in_osds": 3},
	  "pgmap": {"num_pgs": 64, "bytes_total": 1000, "bytes_used": 250, "bytes_avail": 750}
	}`)

	status, err := parseCephStatus(legacy)
	if err != nil {
		t.Fatalf("parseCephStatus returned error: %v", err)
	}
	if status.MonMap.NumMons != 3 || len(status.MonMap.Monitors) != 3 {
		t.Errorf("legacy MonMap = %+v, want 3 monitors", status.MonMap)
	}
	if status.MgrMap.NumMgrs != 2 || status.MgrMap.ActiveMgr != "mgr-a" || status.MgrMap.Standbys != 1 {
		t.Errorf("legacy MgrMap = %+v, want 2 mgrs with active mgr-a", status.MgrMap)
	}
}
