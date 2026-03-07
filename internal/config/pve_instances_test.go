package config

import "testing"

func TestConsolidatePVEInstancesMergesDuplicateClusters(t *testing.T) {
	instances, changed := ConsolidatePVEInstances([]PVEInstance{
		{Name: "c1", ClusterName: "cluster-A", IsCluster: true, ClusterEndpoints: []ClusterEndpoint{{NodeName: "n1"}}},
		{Name: "c2", ClusterName: "cluster-A", IsCluster: true, ClusterEndpoints: []ClusterEndpoint{{NodeName: "n2"}}},
		{Name: "c3", ClusterName: "cluster-B", IsCluster: true},
	})

	if !changed {
		t.Fatalf("expected consolidation change")
	}
	if len(instances) != 2 {
		t.Fatalf("expected 2 instances after consolidation, got %d", len(instances))
	}
	if len(instances[0].ClusterEndpoints) != 2 {
		t.Fatalf("expected 2 endpoints in primary cluster, got %d", len(instances[0].ClusterEndpoints))
	}
}

func TestConsolidatePVEInstancesRemovesStandaloneCoveredByClusterEndpoint(t *testing.T) {
	instances, changed := ConsolidatePVEInstances([]PVEInstance{
		{
			Name:        "homelab",
			ClusterName: "cluster-A",
			IsCluster:   true,
			ClusterEndpoints: []ClusterEndpoint{
				{NodeName: "minipc", Host: "https://10.0.0.5:8006"},
			},
		},
		{
			Name:        "minipc-standalone",
			Host:        "10.0.0.5",
			GuestURL:    "https://minipc.example",
			Fingerprint: "fp-standalone",
		},
	})

	if !changed {
		t.Fatalf("expected consolidation change")
	}
	if len(instances) != 1 {
		t.Fatalf("expected 1 instance after consolidation, got %d", len(instances))
	}
	if got := instances[0].ClusterEndpoints[0].GuestURL; got != "https://minipc.example" {
		t.Fatalf("GuestURL = %q, want https://minipc.example", got)
	}
	if got := instances[0].ClusterEndpoints[0].Fingerprint; got != "fp-standalone" {
		t.Fatalf("Fingerprint = %q, want fp-standalone", got)
	}
}

func TestConsolidatePVEInstancesKeepsStandaloneWithoutExplicitOverlap(t *testing.T) {
	instances, changed := ConsolidatePVEInstances([]PVEInstance{
		{
			Name:        "homelab",
			ClusterName: "cluster-A",
			IsCluster:   true,
			ClusterEndpoints: []ClusterEndpoint{
				{NodeName: "minipc", Host: "https://minipc.local:8006"},
			},
		},
		{
			Name: "minipc-standalone",
			Host: "https://10.0.0.5:8006",
		},
	})

	if changed {
		t.Fatalf("expected no consolidation change without explicit endpoint overlap")
	}
	if len(instances) != 2 {
		t.Fatalf("expected both instances to remain, got %d", len(instances))
	}
}
