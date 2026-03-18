package api

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/stretchr/testify/assert"
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

	// Case 2: Tenant Context but NO MT Monitor -> fail closed
	ctx := context.WithValue(req.Context(), OrgIDContextKey, "tenant-1")
	mon = router.getTenantMonitor(ctx)
	assert.Nil(t, mon, "Should fail closed for non-default org when tenant monitor is unavailable")
}
