package proxmox

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Issue #1626: PVE 9 / Ceph Squid payloads omit mgrmap num_mgrs/active_name
// and the monmap mons array. The Squid mgrmap only carries available +
// num_standbys, and quorum membership lives at the top level of the payload.
const issue1626SquidAPIPayload = `{
  "data": {
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
      "modules": ["balancer", "crash", "devicehealth"],
      "services": {}
    },
    "servicemap": {"services": {}},
    "osdmap": {"num_osds": 6, "num_up_osds": 6, "num_in_osds": 6},
    "pgmap": {"num_pgs": 129, "bytes_total": 12002349744128, "bytes_used": 3000587436032, "bytes_avail": 9001762308096}
  }
}`

func TestIssue1626GetCephStatusSquidSchema(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/cluster/ceph/status" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(issue1626SquidAPIPayload))
	}))
	defer server.Close()

	client := &Client{baseURL: server.URL, httpClient: server.Client()}
	status, err := client.GetCephStatus(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if status.MonMap.NumMons != 3 {
		t.Errorf("MonMap.NumMons = %d, want 3", status.MonMap.NumMons)
	}
	if !status.MgrMap.Available {
		t.Error("MgrMap.Available = false, want true")
	}
	if status.MgrMap.NumStandbys != 1 {
		t.Errorf("MgrMap.NumStandbys = %d, want 1", status.MgrMap.NumStandbys)
	}
	if len(status.QuorumNames) != 3 {
		t.Errorf("QuorumNames = %v, want 3 entries", status.QuorumNames)
	}
	if len(status.Quorum) != 3 {
		t.Errorf("Quorum = %v, want 3 entries", status.Quorum)
	}
}

// Legacy payloads with a standbys array must still populate NumStandbys.
func TestIssue1626CephMgrMapLegacyStandbysArray(t *testing.T) {
	var mgrMap CephMgrMap
	if err := mgrMap.UnmarshalJSON([]byte(`{"available": true, "active_name": "mgr-a", "standbys": ["mgr-b", "mgr-c"]}`)); err != nil {
		t.Fatalf("UnmarshalJSON returned error: %v", err)
	}
	if mgrMap.NumStandbys != 2 {
		t.Errorf("NumStandbys = %d, want 2 (derived from standbys array)", mgrMap.NumStandbys)
	}
	if len(mgrMap.Standbys) != 2 {
		t.Errorf("Standbys = %v, want 2 entries", mgrMap.Standbys)
	}
}
