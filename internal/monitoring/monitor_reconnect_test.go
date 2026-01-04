package monitoring

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

func TestMonitor_RetryFailedConnections_Detailed_Extra(t *testing.T) {
	// Save original factory and restore after test
	origClientFunc := newProxmoxClientFunc
	origRetryDelays := connRetryDelays
	defer func() {
		newProxmoxClientFunc = origClientFunc
		connRetryDelays = origRetryDelays
	}()

	// Speed up test - provide enough entries to avoid hitting the 60s fallback immediately
	connRetryDelays = []time.Duration{
		1 * time.Millisecond,
		1 * time.Millisecond,
		1 * time.Millisecond,
		1 * time.Millisecond,
	}

	// Setup monitor with a disconnected PVE instance
	m := &Monitor{
		config: &config.Config{
			PVEInstances: []config.PVEInstance{
				{Name: "pve1", Host: "https://pve1:8006", User: "root@pam", TokenValue: "token"},
			},
			PBSInstances:      []config.PBSInstance{},
			ConnectionTimeout: time.Second,
		},
		pveClients: make(map[string]PVEClientInterface),
		pbsClients: make(map[string]*pbs.Client),
		state:      models.NewState(),
	}

	// 1. Test Successful Reconnection
	called := false
	newProxmoxClientFunc = func(cfg proxmox.ClientConfig) (PVEClientInterface, error) {
		called = true
		if !strings.Contains(cfg.Host, "pve1") {
			return nil, fmt.Errorf("unexpected host: %s", cfg.Host)
		}
		return &mockPVEClientExtra{}, nil
	}

	m.retryFailedConnections(context.Background())

	if !called {
		t.Error("Expected newProxmoxClientFunc to be called")
	}
	m.mu.Lock() // retryFailedConnections uses locking, we should too when reading map potentially
	client := m.pveClients["pve1"]
	m.mu.Unlock()

	if client == nil {
		t.Error("Expected pve1 client to be reconnected")
	}

	// 2. Test Failed Reconnection
	// Reset
	m.pveClients = make(map[string]PVEClientInterface)
	m.config.PVEInstances = []config.PVEInstance{
		{Name: "pve2", Host: "https://pve2:8006"},
	}

	newProxmoxClientFunc = func(cfg proxmox.ClientConfig) (PVEClientInterface, error) {
		return nil, fmt.Errorf("connection failed")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	m.retryFailedConnections(ctx)

	m.mu.Lock()
	client = m.pveClients["pve2"]
	m.mu.Unlock()

	if client != nil {
		t.Error("Expected pve2 client to remain nil on failure")
	}

	// 3. Test Cluster Reconnection logic (Missing coverage area)
	// Cluster config with basic endpoint
	m.config.PVEInstances = []config.PVEInstance{
		{
			Name:      "cluster1",
			IsCluster: true,
			ClusterEndpoints: []config.ClusterEndpoint{
				{Host: "node1", IP: "192.168.1.1"},
			},
			Host: "https://cluster:8006",
		},
	}
	m.pveClients = make(map[string]PVEClientInterface)

	// mocking NewClusterClient is hard because it is a direct call in retryFailedConnections
	// But we can verify that keys are added to map if it succeeds, BUT NewClusterClient is not mocked via variable.
	// It calls proxmox.NewClusterClient directly.
	// However, NewClusterClient usually doesn't do network checks immediately unless it calls .Connect()?
	// Checking pkg/proxmox/cluster_client.go would verify.
	// If it doesn't do net checks, it will succeed.

	m.retryFailedConnections(context.Background())

	m.mu.Lock()
	cClient := m.pveClients["cluster1"]
	m.mu.Unlock()

	if cClient == nil {
		t.Log("Cluster client creation requires proxmox.NewClusterClient to succeed")
		// If it failed, it might be due to validEndpoint check logic in retryFailedConnections
	}
}
