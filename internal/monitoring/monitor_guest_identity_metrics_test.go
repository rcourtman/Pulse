package monitoring

import (
	"context"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

type stubClusterStatusPVEClient struct {
	PVEClientInterface
	statuses []proxmox.ClusterStatus
	err      error
}

func (s *stubClusterStatusPVEClient) GetClusterStatus(ctx context.Context) ([]proxmox.ClusterStatus, error) {
	return s.statuses, s.err
}

func TestGetGuestMetricsSplicesLiveMigratedSiblingNodeHistory(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "homelab-pve1", Name: "pve1", Instance: "homelab", ClusterName: "homelab", IsClusterMember: true, Status: "online", LastSeen: now},
			{ID: "homelab-pve2", Name: "pve2", Instance: "homelab", ClusterName: "homelab", IsClusterMember: true, Status: "online", LastSeen: now},
		},
		VMs: []models.VM{
			{
				ID:       "homelab:pve2:100",
				VMID:     100,
				GuestUID: "uid-same-guest",
				Name:     "app-vm",
				Node:     "pve2",
				Instance: "homelab",
				Status:   "running",
				LastSeen: now,
			},
		},
	})

	history := NewMetricsHistory(64, time.Hour)
	history.AddGuestMetric("homelab:pve1:100", "cpu", 21, now.Add(-20*time.Minute))
	history.AddGuestMetric("homelab:pve1:100", "cpu", 24, now.Add(-15*time.Minute))
	history.AddGuestMetric("homelab:pve2:100", "cpu", 35, now.Add(-5*time.Minute))
	history.AddGuestMetric("homelab:pve2:100", "cpu", 40, now.Add(-1*time.Minute))

	m := &Monitor{
		metricsHistory: history,
		resourceStore:  unifiedresources.NewMonitorAdapter(registry),
	}

	got := m.GetGuestMetrics("homelab:pve2:100", time.Hour)
	cpu := got["cpu"]
	if len(cpu) != 4 {
		t.Fatalf("expected 4 spliced CPU points across pve1->pve2 migration, got %d (%+v)", len(cpu), cpu)
	}
	wantValues := []float64{21, 24, 35, 40}
	for i, want := range wantValues {
		if cpu[i].Value != want {
			t.Fatalf("cpu[%d].Value = %v, want %v", i, cpu[i].Value, want)
		}
	}
}

func TestGetGuestMetricsWithholdsSiblingHistoryAfterVMIDReuse(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	registry := unifiedresources.NewRegistry(nil)
	nodes := []models.Node{
		{ID: "homelab-pve1", Name: "pve1", Instance: "homelab", ClusterName: "homelab", IsClusterMember: true, Status: "online", LastSeen: now},
		{ID: "homelab-pve2", Name: "pve2", Instance: "homelab", ClusterName: "homelab", IsClusterMember: true, Status: "online", LastSeen: now},
	}

	// Original guest on pve1 with UID 1.
	registry.IngestSnapshot(models.StateSnapshot{
		Nodes: nodes,
		VMs: []models.VM{
			{ID: "homelab:pve1:100", VMID: 100, GuestUID: "uid-old", Name: "old-vm", Node: "pve1", Instance: "homelab", Status: "running", LastSeen: now.Add(-20 * time.Minute)},
		},
	})
	// Deleted and replaced on pve2 with UID 2 on same VMID 100.
	registry.IngestSnapshot(models.StateSnapshot{Nodes: nodes})
	registry.IngestSnapshot(models.StateSnapshot{
		Nodes: nodes,
		VMs: []models.VM{
			{ID: "homelab:pve2:100", VMID: 100, GuestUID: "uid-new", Name: "new-vm", Node: "pve2", Instance: "homelab", Status: "running", LastSeen: now},
		},
	})

	history := NewMetricsHistory(64, time.Hour)
	history.AddGuestMetric("homelab:pve1:100", "cpu", 90, now.Add(-20*time.Minute))
	history.AddGuestMetric("homelab:pve2:100", "cpu", 12, now.Add(-2*time.Minute))

	m := &Monitor{
		metricsHistory: history,
		resourceStore:  unifiedresources.NewMonitorAdapter(registry),
	}

	got := m.GetGuestMetrics("homelab:pve2:100", time.Hour)
	cpu := got["cpu"]
	if len(cpu) != 1 || cpu[0].Value != 12 {
		t.Fatalf("replacement guest after VMID reuse must not inherit old guest's sibling metrics, got %+v", cpu)
	}
}

func TestReconcilePVENodeInventoryStampsClusterIDAndGuardsChange(t *testing.T) {
	now := time.Now().UTC()
	cfg := &config.PVEInstance{
		Name:        "lab",
		Host:        "https://10.0.0.10:8006",
		IsCluster:   true,
		ClusterName: "lab",
	}
	m := &Monitor{
		config:              &config.Config{PVEInstances: []config.PVEInstance{*cfg}},
		pveMembershipMisses: make(map[string]int),
	}
	current := []models.Node{
		{ID: "lab-pve1", Name: "pve1", Instance: "lab", ClusterName: "lab", IsClusterMember: true, Status: "online", LastSeen: now},
	}

	client := &stubClusterStatusPVEClient{
		statuses: []proxmox.ClusterStatus{
			{Type: "cluster", Name: "lab", Quorate: 1, ClusterID: "cid-lab-1"},
			{Type: "node", Name: "pve1", Online: 1, Nodeid: 1, IP: "10.0.0.10"},
		},
	}
	reconciled := m.reconcilePVENodeInventory(context.Background(), "lab", cfg, client, current, nil)
	if len(reconciled) != 1 || reconciled[0].ClusterID != "cid-lab-1" {
		t.Fatalf("expected ClusterID cid-lab-1 stamped on reconciled node, got %+v", reconciled)
	}

	// If the endpoint suddenly responds with a different cluster-id, retain prior nodes.
	conflictClient := &stubClusterStatusPVEClient{
		statuses: []proxmox.ClusterStatus{
			{Type: "cluster", Name: "lab", Quorate: 1, ClusterID: "cid-other-cluster"},
			{Type: "node", Name: "pve2", Online: 1, Nodeid: 1, IP: "10.0.0.20"},
		},
	}
	guarded := m.reconcilePVENodeInventory(context.Background(), "lab", cfg, conflictClient, nil, reconciled)
	if len(guarded) != 1 || guarded[0].Name != "pve1" || guarded[0].ClusterID != "cid-lab-1" {
		t.Fatalf("expected cluster-id mismatch guard to retain pve1/cid-lab-1, got %+v", guarded)
	}
}
