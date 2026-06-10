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

// Regression test for #1441: when the whole instance stops answering (host
// shut down), the poll error path must still run the offline grace policy
// instead of freezing the last online snapshot in state forever.
func TestPollPVEInstanceMarksNodesOfflineWhenUnreachable(t *testing.T) {
	m := &Monitor{
		config: &config.Config{
			PVEInstances: []config.PVEInstance{
				// Non-default port so the portless fallback path stays out of the way.
				{Name: "pve-test", Host: "https://localhost:9999"},
			},
		},
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
	defer m.alertManager.Stop()
	defer m.notificationMgr.Stop()

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
