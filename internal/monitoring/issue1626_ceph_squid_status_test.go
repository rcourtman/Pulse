package monitoring

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// Issue #1626: on PVE 9 / Ceph Squid the status payload has no monmap mons
// array, no mgrmap num_mgrs/active_name/standbys, and quorum membership lives
// at the top level. A 3-node cluster with 2 managers rendered as 0 MONs / 1 MGR.
const issue1626SquidStatusJSON = `{
  "fsid": "9d4c2f0a-1626-4f6e-9b7a-squid0000001",
  "health": {"status": "HEALTH_OK", "checks": {}, "mutes": []},
  "election_epoch": 148,
  "quorum": [0, 1, 2],
  "quorum_names": ["pve1", "pve2", "pve3"],
  "quorum_age": 4161,
  "monmap": {
    "epoch": 3,
    "min_mon_release_name": "squid"
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
}`

func TestIssue1626BuildCephClusterModelSquidSchema(t *testing.T) {
	var status proxmox.CephStatus
	if err := json.Unmarshal([]byte(issue1626SquidStatusJSON), &status); err != nil {
		t.Fatalf("unmarshal squid status: %v", err)
	}

	cluster := buildCephClusterModel("pve-squid", &status, nil)

	if cluster.NumMons != 3 {
		t.Errorf("NumMons = %d, want 3", cluster.NumMons)
	}
	if cluster.NumMgrs != 2 {
		t.Errorf("NumMgrs = %d, want 2 (available active + num_standbys)", cluster.NumMgrs)
	}
	if cluster.NumOSDs != 6 || cluster.NumOSDsUp != 6 {
		t.Errorf("OSD counts = %d/%d up, want 6/6", cluster.NumOSDs, cluster.NumOSDsUp)
	}
}

func TestIssue1626ConvertAgentCephSquidCounts(t *testing.T) {
	// Counts as produced by the fixed hostagent parser for the same
	// Squid-shaped payload (see internal/hostagent/issue1626_ceph_squid_status_test.go).
	agentCeph := &agentshost.CephCluster{
		FSID: "9d4c2f0a-1626-4f6e-9b7a-squid0000001",
		Health: agentshost.CephHealth{
			Status: "HEALTH_OK",
		},
		MonMap: agentshost.CephMonitorMap{Epoch: 3, NumMons: 3},
		MgrMap: agentshost.CephManagerMap{Available: true, NumMgrs: 2, Standbys: 1},
		OSDMap: agentshost.CephOSDMap{NumOSDs: 6, NumUp: 6, NumIn: 6},
		PGMap:  agentshost.CephPGMap{NumPGs: 129},
	}

	cluster := convertAgentCephToGlobalCluster(agentCeph, "pve1", "host-1", time.Now())

	if cluster.NumMons != 3 {
		t.Errorf("NumMons = %d, want 3", cluster.NumMons)
	}
	if cluster.NumMgrs != 2 {
		t.Errorf("NumMgrs = %d, want 2", cluster.NumMgrs)
	}
	if cluster.Source != models.CephClusterSourceHostAgent {
		t.Errorf("Source = %q, want %q", cluster.Source, models.CephClusterSourceHostAgent)
	}
}
