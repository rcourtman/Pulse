package monitoring

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

type unreachablePVEClient struct {
	stubPVEClient
}

func (c *unreachablePVEClient) GetNodes(ctx context.Context) ([]proxmox.Node, error) {
	return nil, fmt.Errorf("dial tcp: i/o timeout")
}

// A host that is already down when Pulse starts has no node state, so the
// poll error path must synthesize an offline entry from the instance config
// instead of leaving the configured instance invisible until its first
// successful poll.
func TestPollPVEInstanceSynthesizesOfflineNodeWhenNeverSeen(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())

	mon := newTestPVEMonitor("test")
	defer mon.alertManager.Stop()
	defer mon.notificationMgr.Stop()

	mon.pollPVEInstance(context.Background(), "test", &unreachablePVEClient{})

	snapshot := mon.state.GetSnapshot()
	if len(snapshot.Nodes) != 1 {
		t.Fatalf("expected one synthesized node in state, got %d", len(snapshot.Nodes))
	}

	node := snapshot.Nodes[0]
	if node.Status != "offline" {
		t.Fatalf("expected synthesized node offline, got %q", node.Status)
	}
	if node.ConnectionHealth != "error" {
		t.Fatalf("expected synthesized node connection health error, got %q", node.ConnectionHealth)
	}
	if node.Name != "test" {
		t.Fatalf("expected synthesized node named after instance, got %q", node.Name)
	}
	if node.Host != "https://pve" {
		t.Fatalf("expected synthesized node to carry configured host, got %q", node.Host)
	}
}

// A node seen online recently rides out an unreachable poll inside the grace
// window with its prior status and degraded connection health.
func TestPollPVEInstanceKeepsRecentNodesThroughUnreachablePoll(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())

	client := &stubPVEClient{
		nodes: []proxmox.Node{
			{
				Node:   "node1",
				Status: "online",
				CPU:    0.10,
				MaxCPU: 8,
				Mem:    4 * 1024 * 1024 * 1024,
				MaxMem: 8 * 1024 * 1024 * 1024,
				Uptime: 7200,
			},
		},
		nodeStatus: &proxmox.NodeStatus{
			Memory: &proxmox.MemoryStatus{
				Total: 8 * 1024 * 1024 * 1024,
				Used:  4 * 1024 * 1024 * 1024,
				Free:  4 * 1024 * 1024 * 1024,
			},
		},
	}

	mon := newTestPVEMonitor("test")
	defer mon.alertManager.Stop()
	defer mon.notificationMgr.Stop()

	mon.pollPVEInstance(context.Background(), "test", client)

	mon.pollPVEInstance(context.Background(), "test", &unreachablePVEClient{})

	snapshot := mon.state.GetSnapshot()
	if len(snapshot.Nodes) != 1 {
		t.Fatalf("expected one node in state, got %d", len(snapshot.Nodes))
	}

	node := snapshot.Nodes[0]
	if node.Status != "online" {
		t.Fatalf("expected recent node to remain online during grace window, got %q", node.Status)
	}
	if node.ConnectionHealth != "degraded" {
		t.Fatalf("expected recent node connection health degraded, got %q", node.ConnectionHealth)
	}
}

// Once the grace window lapses, an unreachable instance's nodes flip to
// offline instead of freezing at their last online snapshot forever.
func TestPollPVEInstanceMarksStaleNodesOfflineWhenUnreachable(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())

	client := &stubPVEClient{
		nodes: []proxmox.Node{
			{
				Node:   "node1",
				Status: "online",
				CPU:    0.10,
				MaxCPU: 8,
				Mem:    4 * 1024 * 1024 * 1024,
				MaxMem: 8 * 1024 * 1024 * 1024,
				Uptime: 7200,
			},
		},
		nodeStatus: &proxmox.NodeStatus{
			Memory: &proxmox.MemoryStatus{
				Total: 8 * 1024 * 1024 * 1024,
				Used:  4 * 1024 * 1024 * 1024,
				Free:  4 * 1024 * 1024 * 1024,
			},
		},
	}

	mon := newTestPVEMonitor("test")
	defer mon.alertManager.Stop()
	defer mon.notificationMgr.Stop()

	mon.pollPVEInstance(context.Background(), "test", client)

	first := mon.state.GetSnapshot()
	if len(first.Nodes) != 1 {
		t.Fatalf("expected one node after first poll, got %d", len(first.Nodes))
	}

	staleNode := first.Nodes[0]
	staleNode.LastSeen = time.Now().Add(-nodeOfflineGracePeriod - 2*time.Second)
	mon.state.UpdateNodesForInstance("test", []models.Node{staleNode})

	mon.pollPVEInstance(context.Background(), "test", &unreachablePVEClient{})

	second := mon.state.GetSnapshot()
	if len(second.Nodes) != 1 {
		t.Fatalf("expected one node after unreachable poll, got %d", len(second.Nodes))
	}

	node := second.Nodes[0]
	if node.Status != "offline" {
		t.Fatalf("expected stale node to be marked offline, got %q", node.Status)
	}
	if node.ConnectionHealth != "error" {
		t.Fatalf("expected stale node connection health error, got %q", node.ConnectionHealth)
	}
	if node.Uptime != 0 {
		t.Fatalf("expected stale node uptime cleared, got %d", node.Uptime)
	}
}
