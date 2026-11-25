package monitoring

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

type stubHAClient struct {
	stubPVEClient
	entries []proxmox.HAStatusEntry
	err     error
	calls   int
}

func (s *stubHAClient) GetClusterHAStatus(ctx context.Context) ([]proxmox.HAStatusEntry, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	return s.entries, nil
}

func TestPollPVEClusterHAStates(t *testing.T) {
	t.Run("skips standalone single-node instances", func(t *testing.T) {
		client := &stubHAClient{}
		got := pollPVEClusterHAStates(
			context.Background(),
			"standalone",
			&config.PVEInstance{Name: "standalone", IsCluster: false},
			client,
			[]proxmox.Node{{Node: "pve1"}},
			nil,
		)
		if client.calls != 0 || got != nil {
			t.Fatalf("expected no HA status call for standalone node; calls=%d got=%v", client.calls, got)
		}
	})

	t.Run("extracts LRM maintenance state on clustered instances", func(t *testing.T) {
		client := &stubHAClient{
			entries: []proxmox.HAStatusEntry{
				{ID: "lrm:pve1", Type: "lrm", Node: "pve1", Status: "pve1 (active, Wed Nov 25 20:00:00 2026)"},
				{ID: "lrm:pve2", Type: "lrm", Node: "pve2", State: "maintenance", Status: "pve2 (maintenance mode, Wed Nov 25 20:00:00 2026)"},
			},
		}
		got := pollPVEClusterHAStates(
			context.Background(),
			"cluster-a",
			&config.PVEInstance{Name: "cluster-a", IsCluster: true},
			client,
			[]proxmox.Node{{Node: "pve1"}, {Node: "pve2"}},
			nil,
		)
		if client.calls != 1 {
			t.Fatalf("calls = %d, want 1", client.calls)
		}
		if got["pve1"].maintenance || got["pve1"].state != "active" {
			t.Fatalf("pve1 state = %+v, want active/false", got["pve1"])
		}
		if !got["pve2"].maintenance || got["pve2"].state != "maintenance" {
			t.Fatalf("pve2 state = %+v, want maintenance/true", got["pve2"])
		}
	})

	t.Run("carries forward previous HA maintenance state on transient poll error", func(t *testing.T) {
		client := &stubHAClient{err: errors.New("temporary 595")}
		prev := []models.Node{
			{Name: "pve2", HAMaintenance: true, HAState: "maintenance"},
		}
		got := pollPVEClusterHAStates(
			context.Background(),
			"cluster-a",
			&config.PVEInstance{Name: "cluster-a", IsCluster: true},
			client,
			[]proxmox.Node{{Node: "pve1"}, {Node: "pve2"}},
			prev,
		)
		if !got["pve2"].maintenance || got["pve2"].state != "maintenance" {
			t.Fatalf("expected pve2 HA maintenance state carried forward, got %+v", got["pve2"])
		}
	})
}

func TestOperatorIntentResolverProjectsProxmoxHAMaintenanceForNodeOnly(t *testing.T) {
	registry := unifiedresources.NewRegistry(nil)
	now := time.Now().UTC()

	node := models.Node{
		ID:              "cluster-a-pve2",
		Name:            "pve2",
		Instance:        "cluster-a",
		Status:          "online",
		IsClusterMember: true,
		ClusterName:     "cluster-a",
		HAMaintenance:   true,
		HAState:         "maintenance",
		LastSeen:        now,
	}
	vm := models.VM{
		ID:       "cluster-a:pve2:101",
		VMID:     101,
		Name:     "app-vm",
		Node:     "pve2",
		Instance: "cluster-a",
		Status:   "running",
		Type:     "qemu",
		LastSeen: now,
	}
	registry.IngestSnapshot(models.StateSnapshot{
		Nodes:      []models.Node{node},
		VMs:        []models.VM{vm},
		LastUpdate: now,
	})

	adapter := unifiedresources.NewMonitorAdapter(registry)
	am := alerts.NewManager()
	defer am.Stop()

	mon := &Monitor{
		alertManager:  am,
		state:         models.NewState(),
		resourceStore: adapter,
	}
	mon.installOperatorIntentResolver(adapter)

	nodeCanonicalID, ok := adapter.ResolveCanonicalResourceID(node.ID)
	if !ok {
		t.Fatalf("failed to resolve canonical ID for node %q", node.ID)
	}
	vmCanonicalID, ok := adapter.ResolveCanonicalResourceID(vm.ID)
	if !ok {
		t.Fatalf("failed to resolve canonical ID for vm %q", vm.ID)
	}

	// Verify node gets read-only Proxmox HA maintenance suppression without mutating persisted operator state.
	if _, persisted, err := adapter.GetResourceOperatorState(nodeCanonicalID); err != nil || persisted {
		t.Fatalf("expected no persisted operator state row, got persisted=%v err=%v", persisted, err)
	}

	// End-to-end AlertManager check: going offline during HA maintenance must not raise a node-offline alert.
	offlineNode := node
	offlineNode.Status = "offline"
	offlineNode.ConnectionHealth = "offline"
	am.CheckNode(offlineNode)
	if activeAlerts := am.GetActiveAlerts(); len(activeAlerts) != 0 {
		t.Fatalf("expected 0 active alerts while node is in Proxmox HA maintenance, got %+v", activeAlerts)
	}

	// Verify ProviderMaintenanceCoverageAt returns active node-scoped coverage for the node and false for the VM.
	cov, active := unifiedresources.ProviderMaintenanceCoverageAt(now, adapter, nodeCanonicalID)
	if !active || cov.Reason != unifiedresources.ProxmoxHAMaintenanceReason || cov.Scope != unifiedresources.MaintenanceScopeResource || cov.Inherited {
		t.Fatalf("unexpected node HA maintenance coverage: active=%v cov=%+v", active, cov)
	}
	if _, vmActive := unifiedresources.ProviderMaintenanceCoverageAt(now, adapter, vmCanonicalID); vmActive {
		t.Fatalf("expected VM not to inherit node HA maintenance coverage")
	}

	// Exiting HA maintenance clears the facet cleanly on the next snapshot.
	node.HAMaintenance = false
	node.HAState = "active"
	registry.IngestSnapshot(models.StateSnapshot{
		Nodes:      []models.Node{node},
		VMs:        []models.VM{vm},
		LastUpdate: now.Add(10 * time.Second),
	})
	if _, stillActive := unifiedresources.ProviderMaintenanceCoverageAt(now.Add(10*time.Second), adapter, nodeCanonicalID); stillActive {
		t.Fatalf("expected HA maintenance coverage to stop after Proxmox exits maintenance")
	}
}
