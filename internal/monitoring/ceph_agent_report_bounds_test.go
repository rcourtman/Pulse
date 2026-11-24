package monitoring

import (
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

func agentCephReport(hostname, fsid string, pools int) agentshost.Report {
	ceph := &agentshost.CephCluster{FSID: fsid, Health: agentshost.CephHealth{Status: "HEALTH_OK"}}
	for i := range pools {
		ceph.Pools = append(ceph.Pools, agentshost.CephPool{ID: i, Name: fmt.Sprintf("pool-%d", i)})
	}
	return agentshost.Report{
		Agent:     agentshost.AgentInfo{ID: hostname, Version: "6.0.0", Type: "unified", IntervalSeconds: 30},
		Host:      agentshost.HostInfo{ID: hostname, Hostname: hostname, Platform: "linux"},
		Ceph:      ceph,
		Timestamp: time.Now().UTC(),
	}
}

// Every Ceph pool of a host report becomes a storage resource, so the report
// must not choose how many there are.
func TestApplyHostReportBoundsCephPools(t *testing.T) {
	monitor := newTestMonitor(t)

	host, err := monitor.ApplyHostReport(agentCephReport("lab", "lab-fsid", maxAgentCephPools+1), nil)
	if err != nil {
		t.Fatalf("ApplyHostReport: %v", err)
	}
	if host.Ceph == nil || len(host.Ceph.Pools) != maxAgentCephPools {
		t.Fatalf("host keeps %d Ceph pools, want %d", len(host.Ceph.Pools), maxAgentCephPools)
	}
	clusters := monitor.state.GetSnapshot().CephClusters
	if len(clusters) != 1 || len(clusters[0].Pools) != maxAgentCephPools {
		t.Fatalf("Ceph clusters = %d with %d pools, want 1 with %d", len(clusters), len(clusters[0].Pools), maxAgentCephPools)
	}
	if last := clusters[0].Pools[maxAgentCephPools-1].Name; last != fmt.Sprintf("pool-%d", maxAgentCephPools-1) {
		t.Fatalf("last kept pool = %q, want the report's first %d pools in order", last, maxAgentCephPools)
	}
}

// A host belongs to one Ceph cluster. Each cluster ID it names replaces the
// record it wrote before instead of adding a cluster and its pools to state
// for good, and the record goes with the host when the host is removed.
func TestApplyHostReportCephClusterReplacesTheHostsEarlierCluster(t *testing.T) {
	monitor := newTestMonitor(t)
	monitor.state.UpdateCephClustersForInstance("pve", []models.CephCluster{{ID: "pve-ceph", FSID: "pve-fsid", Name: "pve Ceph"}})

	assertClusters := func(want ...string) {
		t.Helper()
		var got []string
		for _, cluster := range monitor.state.GetSnapshot().CephClusters {
			got = append(got, cluster.Source+"="+cluster.ID)
		}
		sort.Strings(got)
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("Ceph clusters = %v, want %v", got, want)
		}
	}
	apply := func(hostname, fsid string) models.Host {
		t.Helper()
		host, err := monitor.ApplyHostReport(agentCephReport(hostname, fsid, 2), nil)
		if err != nil {
			t.Fatalf("ApplyHostReport(%s, %s): %v", hostname, fsid, err)
		}
		return host
	}

	apply("ceph1", "ceph-fsid")
	lab := apply("lab", "lab-fsid-1")
	assertClusters("host-agent=ceph-fsid", "host-agent=lab-fsid-1", "proxmox-api=pve-ceph")

	apply("lab", "lab-fsid-2")
	apply("lab", "LAB-FSID-2")
	apply("lab", "")
	assertClusters("host-agent=agent-ceph-"+lab.ID, "host-agent=ceph-fsid", "proxmox-api=pve-ceph")

	// Joining a cluster the Proxmox API reports leaves that record in place.
	apply("lab", "PVE-FSID")
	assertClusters("host-agent=ceph-fsid", "proxmox-api=pve-ceph")
	apply("lab", "lab-fsid-3")
	assertClusters("host-agent=ceph-fsid", "host-agent=lab-fsid-3", "proxmox-api=pve-ceph")

	// A cluster another host agent still reports stays when one of them leaves it.
	apply("lab", "ceph-fsid")
	assertClusters("host-agent=ceph-fsid", "proxmox-api=pve-ceph")
	apply("lab", "lab-fsid-4")
	assertClusters("host-agent=ceph-fsid", "host-agent=lab-fsid-4", "proxmox-api=pve-ceph")

	if _, err := monitor.RemoveHostAgent(lab.ID); err != nil {
		t.Fatalf("RemoveHostAgent: %v", err)
	}
	assertClusters("host-agent=ceph-fsid", "proxmox-api=pve-ceph")
}
