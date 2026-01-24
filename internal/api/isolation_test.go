package api

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStateIsolation_Permanent verifies that tenant context is respected by router helper.
func TestStateIsolation_Permanent(t *testing.T) {
	// Setup config with MT enabled
	cfg := &config.Config{
		MultiTenantEnabled: true,
	}

	// Create router with nil monitors initially
	router := &Router{
		config: cfg,
	}

	_ = router // Suppress unused

	// Simulate request with Tenant Context
	req := httptest.NewRequest("GET", "/api/state", nil)
	ctx := context.WithValue(req.Context(), OrgIDContextKey, "tenant-1")
	req = req.WithContext(ctx)

	// Verify GetOrgID helper correctly extracts it
	assert.Equal(t, "tenant-1", GetOrgID(req.Context()))
}

func TestGetTenantMonitor_Permanent(t *testing.T) {
	cfg := &config.Config{MultiTenantEnabled: true}
	defaultMon := &monitoring.Monitor{}

	router := &Router{
		config:  cfg,
		monitor: defaultMon,
	}

	// Case 1: Default Context -> Default Monitor
	req := httptest.NewRequest("GET", "/", nil)
	mon := router.getTenantMonitor(req.Context())
	assert.Equal(t, defaultMon, mon)

	// Case 2: Tenant Context but NO MT Monitor -> Default Fallback
	ctx := context.WithValue(req.Context(), OrgIDContextKey, "tenant-1")
	mon = router.getTenantMonitor(ctx)
	assert.Equal(t, defaultMon, mon, "Should fallback to default monitor if mtMonitor is nil/failed")
}

func TestResourceIsolation_Permanent(t *testing.T) {
	handlers := NewResourceHandlers()

	// Create snapshots with nodes
	// Note: FromNode uses n.ID as the Resource ID directly
	defaultState := models.StateSnapshot{
		Nodes: []models.Node{{ID: "node-1", Name: "proxmox-default"}},
	}

	tenantAState := models.StateSnapshot{
		Nodes: []models.Node{{ID: "node-2", Name: "proxmox-tenant-a"}},
	}

	// Populate Default
	handlers.PopulateFromSnapshot(defaultState)

	// Populate Tenant A
	handlers.PopulateFromSnapshotForTenant("tenant-a", tenantAState)

	// Verify Default Store
	defaultStore := handlers.Store()

	node, ok := defaultStore.Get("node-1")
	require.True(t, ok, "Default node 'node-1' should be present in default store")
	assert.Equal(t, "proxmox-default", node.Name)

	_, ok = defaultStore.Get("node-2")
	assert.False(t, ok, "Tenant A node 'node-2' should not be in default store")

	// Verify Tenant A Store
	storeA := handlers.getStoreForTenant("tenant-a")
	node, ok = storeA.Get("node-2")
	require.True(t, ok, "Tenant A node 'node-2' should be present in Tenant A store")
	assert.Equal(t, "proxmox-tenant-a", node.Name)

	_, ok = storeA.Get("node-1")
	assert.False(t, ok, "Default node 'node-1' should not be in Tenant A store")
}
