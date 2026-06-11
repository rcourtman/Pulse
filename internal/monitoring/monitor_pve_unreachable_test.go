package monitoring

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

func TestPreserveOrExpireNodes(t *testing.T) {
	m := &Monitor{
		state:          models.NewState(),
		nodeLastOnline: make(map[string]time.Time),
	}

	t.Run("within grace keeps node online with degraded health", func(t *testing.T) {
		preserved := m.preserveOrExpireNodes([]models.Node{{
			ID:               "inst-node1",
			Name:             "node1",
			Status:           "online",
			ConnectionHealth: "error",
			Uptime:           1234,
			LastSeen:         time.Now().Add(-10 * time.Second),
		}})
		if len(preserved) != 1 {
			t.Fatalf("preserved = %d nodes, want 1", len(preserved))
		}
		if preserved[0].Status != "online" {
			t.Fatalf("Status = %q, want online within grace", preserved[0].Status)
		}
		if preserved[0].ConnectionHealth != "degraded" {
			t.Fatalf("ConnectionHealth = %q, want degraded within grace", preserved[0].ConnectionHealth)
		}
	})

	t.Run("offline node with stamped LastSeen is not resurrected to online", func(t *testing.T) {
		// Simulates the unified-registry round-trip: ingest stamps zero
		// LastSeen with the ingest time, so a synthesized offline
		// placeholder comes back from prev state looking "recently seen"
		// even though no poll ever reached the node. Grace must not flip
		// it to online — that loop made never-seen hosts permanently
		// report online/degraded.
		preserved := m.preserveOrExpireNodes([]models.Node{{
			ID:               "inst-placeholder",
			Name:             "placeholder",
			Status:           "offline",
			ConnectionHealth: "error",
			LastSeen:         time.Now().Add(-5 * time.Second),
		}})
		if len(preserved) != 1 {
			t.Fatalf("preserved = %d nodes, want 1", len(preserved))
		}
		if preserved[0].Status != "offline" {
			t.Fatalf("Status = %q, want offline (no online sighting to justify grace)", preserved[0].Status)
		}
		if preserved[0].ConnectionHealth != "error" {
			t.Fatalf("ConnectionHealth = %q, want error", preserved[0].ConnectionHealth)
		}
	})

	t.Run("past grace marks node offline", func(t *testing.T) {
		preserved := m.preserveOrExpireNodes([]models.Node{{
			ID:               "inst-node1",
			Name:             "node1",
			Status:           "online",
			ConnectionHealth: "healthy",
			Uptime:           1234,
			LastSeen:         time.Now().Add(-2 * nodeOfflineGracePeriod),
		}})
		if len(preserved) != 1 {
			t.Fatalf("preserved = %d nodes, want 1", len(preserved))
		}
		if preserved[0].Status != "offline" {
			t.Fatalf("Status = %q, want offline past grace", preserved[0].Status)
		}
		if preserved[0].ConnectionHealth != "error" {
			t.Fatalf("ConnectionHealth = %q, want error past grace", preserved[0].ConnectionHealth)
		}
		if preserved[0].Uptime != 0 {
			t.Fatalf("Uptime = %d, want 0 past grace", preserved[0].Uptime)
		}
	})
}

type unreachablePVEClient struct {
	mockPVEClientExtended
}

func (c *unreachablePVEClient) GetNodes(ctx context.Context) ([]proxmox.Node, error) {
	return nil, fmt.Errorf("connection refused")
}

func newUnreachableTestMonitor(t *testing.T, cfg *config.Config) *Monitor {
	t.Helper()
	m := &Monitor{
		config:                  cfg,
		state:                   models.NewState(),
		pveClients:              make(map[string]PVEClientInterface),
		nodeLastOnline:          make(map[string]time.Time),
		nodeSnapshots:           make(map[string]NodeMemorySnapshot),
		guestSnapshots:          make(map[string]GuestMemorySnapshot),
		metricsHistory:          NewMetricsHistory(32, time.Hour),
		lastClusterCheck:        make(map[string]time.Time),
		lastPhysicalDiskPoll:    make(map[string]time.Time),
		lastPVEBackupPoll:       make(map[string]time.Time),
		authFailures:            make(map[string]int),
		lastAuthAttempt:         make(map[string]time.Time),
		pollStatusMap:           make(map[string]*pollStatus),
		nodePendingUpdatesCache: make(map[string]pendingUpdatesCache),
		instanceInfoCache:       make(map[string]*instanceInfo),
		lastOutcome:             make(map[string]taskOutcome),
		failureCounts:           make(map[string]int),
		alertManager:            alerts.NewManager(),
		notificationMgr:         notifications.NewNotificationManager(""),
	}
	t.Cleanup(m.alertManager.Stop)
	t.Cleanup(m.notificationMgr.Stop)
	return m
}

// Regression test for #1441: when the whole instance stops answering (host
// shut down), the poll error path must still run the offline grace policy
// instead of freezing the last online snapshot in state forever.
func TestPollPVEInstanceMarksNodesOfflineWhenUnreachable(t *testing.T) {
	m := newUnreachableTestMonitor(t, &config.Config{
		PVEInstances: []config.PVEInstance{
			// Non-default port so the portless fallback path stays out of the way.
			{Name: "pve-test", Host: "https://localhost:9999"},
		},
	})

	// Seed state as if a previous poll saw the node online, longer ago than
	// the offline grace period.
	m.state.UpdateNodesForInstance("pve-test", []models.Node{{
		ID:       "pve-test-node1",
		Name:     "node1",
		Instance: "pve-test",
		Status:   "online",
		Type:     "node",
		Uptime:   4242,
		LastSeen: time.Now().Add(-2 * nodeOfflineGracePeriod),
	}})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	m.pollPVEInstance(ctx, "pve-test", &unreachablePVEClient{})

	nodes := m.state.GetSnapshot().Nodes
	if len(nodes) != 1 {
		t.Fatalf("state nodes = %d, want 1", len(nodes))
	}
	if nodes[0].Status != "offline" {
		t.Fatalf("node Status = %q after unreachable poll past grace, want offline", nodes[0].Status)
	}
	if nodes[0].ConnectionHealth != "error" {
		t.Fatalf("node ConnectionHealth = %q, want error", nodes[0].ConnectionHealth)
	}
}

// Regression test for #1433: an instance that is already down when Pulse
// starts has no previous node state, so the unreachable path must synthesize
// an offline placeholder from config instead of leaving the configured
// instance invisible until its first successful poll.
func TestPollPVEInstanceSynthesizesPlaceholderWhenNeverSeen(t *testing.T) {
	m := newUnreachableTestMonitor(t, &config.Config{
		PVEInstances: []config.PVEInstance{
			// Non-default port so the portless fallback path stays out of the way.
			{Name: "pve-test", Host: "https://localhost:9999"},
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	m.pollPVEInstance(ctx, "pve-test", &unreachablePVEClient{})

	nodes := m.state.GetSnapshot().Nodes
	if len(nodes) != 1 {
		t.Fatalf("state nodes = %d, want 1 synthesized placeholder", len(nodes))
	}
	if nodes[0].ID != "pve-test-pve-test" {
		t.Fatalf("node ID = %q, want pve-test-pve-test", nodes[0].ID)
	}
	if nodes[0].Name != "pve-test" {
		t.Fatalf("node Name = %q, want pve-test", nodes[0].Name)
	}
	if nodes[0].Status != "offline" {
		t.Fatalf("node Status = %q, want offline", nodes[0].Status)
	}
	if nodes[0].ConnectionHealth != "error" {
		t.Fatalf("node ConnectionHealth = %q, want error", nodes[0].ConnectionHealth)
	}
	if nodes[0].Host != "https://localhost:9999" {
		t.Fatalf("node Host = %q, want configured endpoint", nodes[0].Host)
	}
}

func TestPlaceholderNodesForInstanceCluster(t *testing.T) {
	m := &Monitor{
		state: models.NewState(),
		config: &config.Config{
			PVEInstances: []config.PVEInstance{{
				Name:        "cluster-inst",
				Host:        "https://10.0.0.1:8006",
				IsCluster:   true,
				ClusterName: "homelab",
				ClusterEndpoints: []config.ClusterEndpoint{
					{NodeName: "pve1", Host: "https://10.0.0.1:8006"},
					{NodeName: "pve2", Host: "https://10.0.0.2:8006"},
					{NodeName: ""},     // skipped: no node name
					{NodeName: "pve1"}, // skipped: duplicate
				},
			}},
		},
	}

	nodes := m.placeholderNodesForInstance("cluster-inst")
	if len(nodes) != 2 {
		t.Fatalf("placeholders = %d nodes, want 2 (deduped, blank skipped)", len(nodes))
	}
	for i, want := range []string{"pve1", "pve2"} {
		if nodes[i].Name != want {
			t.Fatalf("nodes[%d].Name = %q, want %q", i, nodes[i].Name, want)
		}
		if wantID := "homelab-" + want; nodes[i].ID != wantID {
			t.Fatalf("nodes[%d].ID = %q, want %q (cluster ID convention)", i, nodes[i].ID, wantID)
		}
		if nodes[i].Status != "offline" {
			t.Fatalf("nodes[%d].Status = %q, want offline", i, nodes[i].Status)
		}
		if !nodes[i].IsClusterMember {
			t.Fatalf("nodes[%d].IsClusterMember = false, want true", i)
		}
		if nodes[i].ClusterName != "homelab" {
			t.Fatalf("nodes[%d].ClusterName = %q, want homelab", i, nodes[i].ClusterName)
		}
	}
}
