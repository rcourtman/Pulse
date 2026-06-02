package models

import "testing"

// #1341: the same physical Ceph cluster can be reported by both the Proxmox API
// poller and a Pulse host-agent. DedupeCephClusters must collapse them to one
// deterministic identity so the UI and alert evaluation never disagree on the
// pool ID.
func TestDedupeCephClustersPrefersApiSourceForSharedFSID(t *testing.T) {
	api := CephCluster{
		ID:       "pve5-ceph-fsid-1341",
		Instance: "pve5",
		Name:     "Ceph",
		FSID:     "ceph-fsid-1341",
		NumMons:  3,
		NumMgrs:  2,
		Pools:    []CephPool{{ID: 2, Name: "data_replication", PercentUsed: 61.1}},
	}
	agent := CephCluster{
		ID:       "ceph-fsid-1341",
		Instance: "agent:pve5",
		Name:     "pve5 Ceph",
		FSID:     "ceph-fsid-1341",
		NumMons:  3,
		NumMgrs:  2,
		Pools:    []CephPool{{ID: 2, Name: "data_replication", PercentUsed: 61.1}},
	}

	for _, order := range [][]CephCluster{{api, agent}, {agent, api}} {
		deduped := DedupeCephClusters(order)
		if len(deduped) != 1 {
			t.Fatalf("expected 1 deduped cluster, got %d", len(deduped))
		}
		if deduped[0].Instance != "pve5" {
			t.Errorf("expected non-agent (API) cluster to win, got instance %q", deduped[0].Instance)
		}
	}
}

// When only a host-agent reports the cluster, it must survive dedup so its
// pools still drive alerts.
func TestDedupeCephClustersKeepsAgentOnlyCluster(t *testing.T) {
	agent := CephCluster{
		ID:       "ceph-fsid-solo",
		Instance: "agent:pve5",
		FSID:     "ceph-fsid-solo",
		Pools:    []CephPool{{ID: 1, Name: "data_replication"}},
	}
	deduped := DedupeCephClusters([]CephCluster{agent})
	if len(deduped) != 1 || deduped[0].Instance != "agent:pve5" {
		t.Fatalf("expected the agent-only cluster to survive, got %+v", deduped)
	}
}

// Distinct clusters (different FSIDs) must all be preserved.
func TestDedupeCephClustersKeepsDistinctFSIDs(t *testing.T) {
	a := CephCluster{ID: "a", Instance: "pve1", FSID: "fsid-a", Name: "A"}
	b := CephCluster{ID: "b", Instance: "pve2", FSID: "fsid-b", Name: "B"}
	deduped := DedupeCephClusters([]CephCluster{a, b})
	if len(deduped) != 2 {
		t.Fatalf("expected 2 distinct clusters, got %d", len(deduped))
	}
}

// Among same-class (both agent) sources, the more complete report wins, with a
// deterministic tie-break so the winner never oscillates between snapshots.
func TestDedupeCephClustersStableAmongSameClass(t *testing.T) {
	sparse := CephCluster{ID: "z", Instance: "agent:b", FSID: "fsid-x", NumMons: 1}
	rich := CephCluster{ID: "a", Instance: "agent:a", FSID: "fsid-x", NumMons: 3, NumMgrs: 2, Pools: []CephPool{{Name: "p"}}}

	first := DedupeCephClusters([]CephCluster{sparse, rich})
	second := DedupeCephClusters([]CephCluster{rich, sparse})
	if len(first) != 1 || len(second) != 1 {
		t.Fatalf("expected single winner, got %d/%d", len(first), len(second))
	}
	if first[0].ID != rich.ID || second[0].ID != rich.ID {
		t.Errorf("expected the more complete report to win deterministically, got %q/%q", first[0].ID, second[0].ID)
	}
}
