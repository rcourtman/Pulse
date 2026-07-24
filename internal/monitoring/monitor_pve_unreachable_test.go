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

type membershipPVEClient struct {
	stubPVEClient
	statuses  []proxmox.ClusterStatus
	statusErr error
}

func (c *membershipPVEClient) GetClusterStatus(context.Context) ([]proxmox.ClusterStatus, error) {
	return c.statuses, c.statusErr
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

func TestReconcilePVENodeInventoryPreservesCanonicalMembership(t *testing.T) {
	lastSeen := time.Now().Add(-2 * time.Hour).UTC()
	cfg := &config.Config{
		PVEInstances: []config.PVEInstance{{
			Name:        "cluster-api",
			Host:        "https://pve-a:8006",
			IsCluster:   true,
			ClusterName: "production",
			ClusterEndpoints: []config.ClusterEndpoint{
				{NodeID: "node/a", NodeName: "pve-a", Host: "https://pve-a:8006", Online: true, LastSeen: lastSeen},
				{NodeID: "node/b", NodeName: "pve-b", Host: "https://pve-b:8006", Online: true, LastSeen: lastSeen},
			},
		}},
	}
	m := newUnreachableTestMonitor(t, cfg)
	instanceCfg := &cfg.PVEInstances[0]
	client := &membershipPVEClient{statuses: []proxmox.ClusterStatus{
		{Type: "cluster", Name: "production", Quorate: 1},
		{Type: "node", ID: "node/a", Name: "pve-a", Online: 1},
		{Type: "node", ID: "node/b", Name: "pve-b", Online: 0},
	}}
	previous := []models.Node{
		{
			ID:               "production-pve-a",
			Name:             "pve-a",
			Instance:         "cluster-api",
			ClusterName:      "production",
			Status:           "online",
			ConnectionHealth: "healthy",
			LastSeen:         lastSeen,
		},
		{
			ID:               "production-pve-b",
			Name:             "pve-b",
			Instance:         "cluster-api",
			ClusterName:      "production",
			Host:             "https://pve-b:8006",
			Status:           "online",
			ConnectionHealth: "healthy",
			CPU:              0.8,
			Uptime:           3600,
			LastSeen:         lastSeen,
			LinkedAgentID:    "agent-pve-b",
		},
	}
	current := []models.Node{previous[0]}

	got := m.reconcilePVENodeInventory(
		context.Background(),
		"cluster-api",
		instanceCfg,
		client,
		current,
		previous,
	)
	if len(got) != 2 {
		t.Fatalf("reconciled nodes = %d, want 2: %+v", len(got), got)
	}
	byName := pveNodeByName(got)
	offline := byName["pve-b"]
	if offline.ID != "production-pve-b" {
		t.Fatalf("offline node ID = %q, want stable canonical ID", offline.ID)
	}
	if offline.Status != "offline" || offline.ConnectionHealth != "error" {
		t.Fatalf("offline node state = (%q, %q), want offline/error", offline.Status, offline.ConnectionHealth)
	}
	if offline.CPU != 0 || offline.Uptime != 0 {
		t.Fatalf("offline node live metrics = (cpu=%v uptime=%d), want zeroed", offline.CPU, offline.Uptime)
	}
	if !offline.LastSeen.Equal(lastSeen) || offline.LinkedAgentID != "agent-pve-b" {
		t.Fatalf("offline node lost last-known identity evidence: %+v", offline)
	}
	if len(instanceCfg.ClusterEndpoints) != 2 {
		t.Fatalf("cluster endpoints = %d, want 2 canonical members", len(instanceCfg.ClusterEndpoints))
	}
}

func TestReconcilePVENodeInventoryTreatsPartialAndNonQuorateReadsAsUncertain(t *testing.T) {
	tests := []struct {
		name      string
		statuses  []proxmox.ClusterStatus
		statusErr error
	}{
		{
			name:      "membership API unavailable",
			statusErr: fmt.Errorf("temporary connection failure"),
		},
		{
			name: "membership response incomplete",
			statuses: []proxmox.ClusterStatus{
				{Type: "cluster", Name: "production", Quorate: 1},
			},
		},
		{
			name: "cluster has no quorum",
			statuses: []proxmox.ClusterStatus{
				{Type: "cluster", Name: "production", Quorate: 0},
				{Type: "node", Name: "pve-a", Online: 1},
			},
		},
		{
			name: "cluster identity changed",
			statuses: []proxmox.ClusterStatus{
				{Type: "cluster", Name: "different-cluster", Quorate: 1},
				{Type: "node", Name: "pve-a", Online: 1},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{PVEInstances: []config.PVEInstance{{
				Name:        "cluster-api",
				IsCluster:   true,
				ClusterName: "production",
				ClusterEndpoints: []config.ClusterEndpoint{
					{NodeName: "pve-a"},
					{NodeName: "pve-b"},
				},
			}}}
			m := newUnreachableTestMonitor(t, cfg)
			previous := []models.Node{
				{ID: "production-pve-a", Name: "pve-a", Instance: "cluster-api", Status: "online", LastSeen: time.Now()},
				{ID: "production-pve-b", Name: "pve-b", Instance: "cluster-api", Status: "offline"},
			}
			got := m.reconcilePVENodeInventory(
				context.Background(),
				"cluster-api",
				&cfg.PVEInstances[0],
				&membershipPVEClient{statuses: tt.statuses, statusErr: tt.statusErr},
				previous[:1],
				previous,
			)
			if len(got) != 2 {
				t.Fatalf("uncertain membership returned %d nodes, want last-known union: %+v", len(got), got)
			}
			if got[1].Name != "pve-b" || got[1].Status != "offline" {
				t.Fatalf("unobserved member = %+v, want pve-b retained offline", got[1])
			}
			if len(cfg.PVEInstances[0].ClusterEndpoints) != 2 {
				t.Fatalf("uncertain membership rewrote endpoints: %+v", cfg.PVEInstances[0].ClusterEndpoints)
			}
		})
	}
}

func TestReconcilePVENodeInventoryAddsNewAuthoritativeMemberImmediately(t *testing.T) {
	cfg := &config.Config{PVEInstances: []config.PVEInstance{{
		Name:        "cluster-api",
		Host:        "https://pve-a:8006",
		IsCluster:   true,
		ClusterName: "production",
		ClusterEndpoints: []config.ClusterEndpoint{
			{NodeName: "pve-a", Host: "https://pve-a:8006"},
		},
	}}}
	m := newUnreachableTestMonitor(t, cfg)
	previous := []models.Node{{
		ID:          "production-pve-a",
		Name:        "pve-a",
		Instance:    "cluster-api",
		ClusterName: "production",
		Status:      "online",
		LastSeen:    time.Now(),
	}}
	client := &membershipPVEClient{statuses: []proxmox.ClusterStatus{
		{Type: "cluster", Name: "production", Quorate: 1},
		{Type: "node", ID: "node/a", Name: "pve-a", Online: 1},
		{Type: "node", ID: "node/c", Name: "pve-c", IP: "10.0.0.3", Online: 1},
	}}

	got := m.reconcilePVENodeInventory(
		context.Background(),
		"cluster-api",
		&cfg.PVEInstances[0],
		client,
		previous,
		previous,
	)
	if len(got) != 2 {
		t.Fatalf("membership addition returned %d nodes, want 2: %+v", len(got), got)
	}
	added := pveNodeByName(got)["pve-c"]
	if added.ID != "production-pve-c" || added.Status != "unknown" || added.ConnectionHealth != "degraded" {
		t.Fatalf("new member = %+v, want immediate stable unknown/degraded placeholder", added)
	}
	if len(cfg.PVEInstances[0].ClusterEndpoints) != 2 {
		t.Fatalf("new member was not persisted to endpoints: %+v", cfg.PVEInstances[0].ClusterEndpoints)
	}
}

func TestPVENodeIdentityScopeSeparatesSameNameClusters(t *testing.T) {
	cfg := &config.Config{PVEInstances: []config.PVEInstance{
		{Name: "site-a", IsCluster: true, ClusterName: "production"},
		{Name: "site-b", IsCluster: true, ClusterName: "production"},
	}}
	m := &Monitor{config: cfg, state: models.NewState()}

	a := m.placeholderNodeForInstance("site-a", &cfg.PVEInstances[0], "pve-1")
	b := m.placeholderNodeForInstance("site-b", &cfg.PVEInstances[1], "pve-1")
	if a.ID != "site-a-pve-1" || b.ID != "site-b-pve-1" || a.ID == b.ID {
		t.Fatalf("same-name cluster node IDs collided: a=%q b=%q", a.ID, b.ID)
	}
}
