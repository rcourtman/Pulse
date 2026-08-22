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
