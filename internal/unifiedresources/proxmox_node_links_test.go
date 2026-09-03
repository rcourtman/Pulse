package unifiedresources

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// A host trusted-linked into one cluster must not be inferred as the agent
// view of a same-named node in a different cluster. Refs #1753.
func TestInferLinkedHostsForProxmoxNodesKeepsClustersApart(t *testing.T) {
	stagingNode := models.Node{
		ID:            "staging-pve01",
		Name:          "pve01",
		ClusterName:   "staging",
		Host:          "https://pve01.staging.tld:8006",
		LinkedAgentID: "host-a",
	}
	hostA := &models.Host{
		ID:           "host-a",
		Hostname:     "pve01",
		LinkedNodeID: "staging-pve01",
	}

	t.Run("different cluster blocks the short-name fallback", func(t *testing.T) {
		productionNode := models.Node{
			ID:          "production-pve01",
			Name:        "pve01",
			ClusterName: "production",
			Host:        "https://pve01.production.tld:8006",
		}
		out := inferLinkedHostsForProxmoxNodes(
			[]models.Node{stagingNode, productionNode},
			map[string]*models.Host{"host-a": hostA},
		)
		if out["staging-pve01"] != hostA {
			t.Fatalf("expected staging node to keep its trusted host link, got %+v", out["staging-pve01"])
		}
		if linked, ok := out["production-pve01"]; ok {
			t.Fatalf("expected production node to stay unlinked, got %+v", linked)
		}
	})

	t.Run("control: unknown cluster still falls back on the short name", func(t *testing.T) {
		orphanNode := models.Node{
			ID:   "standalone-pve01",
			Name: "pve01",
			Host: "https://192.0.2.10:8006",
		}
		out := inferLinkedHostsForProxmoxNodes(
			[]models.Node{stagingNode, orphanNode},
			map[string]*models.Host{"host-a": hostA},
		)
		if out["standalone-pve01"] != hostA {
			t.Fatalf("expected cluster-less node to keep the legacy fallback link, got %+v", out["standalone-pve01"])
		}
	})
}

func TestInferLinkedHostsForProxmoxNodesKeepsStandaloneProviderScopesApart(t *testing.T) {
	staging := models.Node{
		ID: "staging-pve", NodeIdentity: "staging-pve", Name: "pve",
		Instance: "staging", Host: "https://pve.staging.example:8006",
		LinkedAgentID: "host-staging",
	}
	production := models.Node{
		ID: "production-pve", NodeIdentity: "production-pve", Name: "pve",
		Instance: "production", Host: "https://pve.production.example:8006",
	}
	host := &models.Host{
		ID: "host-staging", Hostname: "pve", LinkedNodeID: "staging-pve",
	}

	got := inferLinkedHostsForProxmoxNodes(
		[]models.Node{staging, production},
		map[string]*models.Host{host.ID: host},
	)
	if got[staging.ID] == nil || got[staging.ID].ID != host.ID {
		t.Fatalf("trusted staging link was lost: %+v", got)
	}
	if got[production.ID] != nil {
		t.Fatalf("staging host identity leaked into production provider: %+v", got)
	}
}

func TestInferLinkedHostsForProxmoxNodesDoesNotTrustRepeatedClusterLabel(t *testing.T) {
	staging := models.Node{
		ID: "staging-pve", NodeIdentity: "staging-pve", Name: "pve",
		ClusterName: "homelab", Instance: "staging", Host: "https://pve.staging.example:8006",
		LinkedAgentID: "host-staging",
	}
	production := models.Node{
		ID: "production-pve", NodeIdentity: "production-pve", Name: "pve",
		ClusterName: "homelab", Instance: "production", Host: "https://pve.production.example:8006",
	}
	host := &models.Host{
		ID: "host-staging", Hostname: "pve", LinkedNodeID: "staging-pve",
	}

	got := inferLinkedHostsForProxmoxNodes(
		[]models.Node{staging, production},
		map[string]*models.Host{host.ID: host},
	)
	if got[staging.ID] == nil || got[staging.ID].ID != host.ID {
		t.Fatalf("trusted staging link was lost: %+v", got)
	}
	if got[production.ID] != nil {
		t.Fatalf("repeated display label leaked staging host into production provider: %+v", got)
	}
}

func TestProxmoxProviderNodesProveSameMachineAcrossDuplicateConnections(t *testing.T) {
	base := models.Node{
		ID: "site-a-pve", NodeIdentity: "node-pve", Name: "pve",
		Instance: "site-a", Host: "https://pve.example:8006",
	}

	for name, candidate := range map[string]models.Node{
		"node identity": {ID: "site-b-pve", NodeIdentity: "node-pve", Name: "pve", Instance: "site-b", Host: "https://other.example:8006"},
		"endpoint":      {ID: "site-b-pve", Name: "pve", Instance: "site-b", Host: "https://pve.example:8006"},
	} {
		t.Run(name, func(t *testing.T) {
			left := base
			if name == "cluster" {
				left.ClusterName = "prod"
			}
			if !proxmoxProviderNodesProveSameMachine(left, candidate, nil) {
				t.Fatalf("expected provider evidence to prove duplicate views: left=%+v right=%+v", left, candidate)
			}
		})
	}

	distinct := models.Node{
		ID: "site-b-pve", NodeIdentity: "other-pve", Name: "pve",
		Instance: "site-b", Host: "https://pve.other.example:8006",
	}
	if proxmoxProviderNodesProveSameMachine(base, distinct, nil) {
		t.Fatalf("shared native hostname proved distinct standalone providers equal: left=%+v right=%+v", base, distinct)
	}

	leftCluster := base
	leftCluster.NodeIdentity = ""
	leftCluster.ClusterName = "homelab"
	rightCluster := distinct
	rightCluster.NodeIdentity = ""
	rightCluster.ClusterName = "homelab"
	if proxmoxProviderNodesProveSameMachine(leftCluster, rightCluster, nil) {
		t.Fatalf("equal cluster display labels proved distinct provider scopes equal: left=%+v right=%+v", leftCluster, rightCluster)
	}
}
