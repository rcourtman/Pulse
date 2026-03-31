package mock

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
)

func TestCurrentFixtureGraphReturnsDefensiveCopies(t *testing.T) {
	previous := IsMockEnabled()
	SetEnabled(true)
	t.Cleanup(func() { SetEnabled(previous) })

	graph := CurrentFixtureGraph()
	if len(graph.State.Nodes) == 0 {
		t.Fatal("expected legacy snapshot nodes in canonical mock graph")
	}
	if len(graph.PlatformFixtures.VMware.Hosts) == 0 {
		t.Fatal("expected VMware fixtures in canonical mock graph")
	}

	originalNodeName := graph.State.Nodes[0].Name
	originalHostName := graph.PlatformFixtures.VMware.Hosts[0].Name

	graph.State.Nodes[0].Name = "mutated-node"
	graph.PlatformFixtures.VMware.Hosts[0].Name = "mutated-host"
	if len(graph.AlertHistory) > 0 {
		graph.AlertHistory[0].ID = "mutated-alert"
	}

	current := CurrentFixtureGraph()
	if current.State.Nodes[0].Name != originalNodeName {
		t.Fatalf("expected canonical graph to protect state snapshot, got %q", current.State.Nodes[0].Name)
	}
	if current.PlatformFixtures.VMware.Hosts[0].Name != originalHostName {
		t.Fatalf("expected canonical graph to protect platform fixtures, got %q", current.PlatformFixtures.VMware.Hosts[0].Name)
	}
	if len(graph.AlertHistory) > 0 && len(current.AlertHistory) > 0 && current.AlertHistory[0].ID == "mutated-alert" {
		t.Fatal("expected canonical graph to protect alert history")
	}
}

func TestFixtureGraphRecoveryPointsDeriveSubjectsFromCurrentGraph(t *testing.T) {
	previous := IsMockEnabled()
	SetEnabled(true)
	t.Cleanup(func() { SetEnabled(previous) })

	graph := CurrentFixtureGraph()
	if len(graph.State.KubernetesClusters) == 0 {
		t.Fatal("expected Kubernetes clusters in canonical mock graph")
	}
	if len(graph.State.VMs) == 0 && len(graph.State.Containers) == 0 {
		t.Fatal("expected Proxmox guests in canonical mock graph")
	}
	if len(graph.PlatformFixtures.TrueNAS.Datasets) == 0 {
		t.Fatal("expected TrueNAS datasets in canonical mock graph")
	}

	clusterNames := make(map[string]struct{}, len(graph.State.KubernetesClusters))
	for _, cluster := range graph.State.KubernetesClusters {
		if name := firstNonEmptyTrimmed(cluster.DisplayName, cluster.CustomDisplayName, cluster.Name); name != "" {
			clusterNames[name] = struct{}{}
		}
	}

	guestNames := make(map[string]struct{}, len(graph.State.VMs)+len(graph.State.Containers))
	for _, guest := range graph.State.VMs {
		if guest.Name != "" {
			guestNames[guest.Name] = struct{}{}
		}
	}
	for _, guest := range graph.State.Containers {
		if guest.Name != "" {
			guestNames[guest.Name] = struct{}{}
		}
	}

	datasetNames := make(map[string]struct{}, len(graph.PlatformFixtures.TrueNAS.Datasets))
	for _, dataset := range graph.PlatformFixtures.TrueNAS.Datasets {
		if dataset.Name != "" {
			datasetNames[dataset.Name] = struct{}{}
		}
	}

	points := CurrentFixtureGraph().RecoveryPoints()
	if len(points) == 0 {
		t.Fatal("expected mock recovery points")
	}

	foundKubernetes := false
	foundProxmox := false
	foundTrueNAS := false

	for _, point := range points {
		if point.SubjectRef == nil {
			continue
		}
		switch point.Provider {
		case recovery.ProviderKubernetes:
			if point.SubjectRef.Type == "k8s-cluster" {
				if _, ok := clusterNames[point.SubjectRef.Name]; ok {
					foundKubernetes = true
				}
			}
		case recovery.ProviderProxmoxPVE:
			if _, ok := guestNames[point.SubjectRef.Name]; ok {
				foundProxmox = true
			}
		case recovery.ProviderTrueNAS:
			if _, ok := datasetNames[point.SubjectRef.Name]; ok {
				foundTrueNAS = true
			}
		}
	}

	if !foundKubernetes {
		t.Fatal("expected recovery points to derive Kubernetes subjects from canonical mock graph")
	}
	if !foundProxmox {
		t.Fatal("expected recovery points to derive Proxmox subjects from canonical mock graph")
	}
	if !foundTrueNAS {
		t.Fatal("expected recovery points to derive TrueNAS subjects from canonical mock graph")
	}
}

func TestFixtureGraphRecoveryPointsKeepTrueNASArtifactsFreshForDemoMode(t *testing.T) {
	previous := IsMockEnabled()
	SetEnabled(true)
	t.Cleanup(func() { SetEnabled(previous) })

	points := CurrentFixtureGraph().RecoveryPoints()
	if len(points) == 0 {
		t.Fatal("expected mock recovery points")
	}

	var newest time.Time
	for _, point := range points {
		if point.Provider != recovery.ProviderTrueNAS || point.CompletedAt == nil {
			continue
		}
		if point.CompletedAt.After(newest) {
			newest = *point.CompletedAt
		}
	}

	if newest.IsZero() {
		t.Fatal("expected completed TrueNAS recovery points")
	}
	if newest.Before(time.Now().UTC().Add(-24 * time.Hour)) {
		t.Fatalf("expected demo TrueNAS recovery artifacts within the last 24h, got newest %s", newest.UTC().Format(time.RFC3339))
	}
}
