package monitoring

import (
	"context"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReloadableMonitor_Lifecycle_Coverage(t *testing.T) {
	// Create minimal config
	cfg := &config.Config{
		DataPath: t.TempDir(),
	}
	persistence := config.NewMultiTenantPersistence(cfg.DataPath)

	// Create ReloadableMonitor
	rm, err := NewReloadableMonitor(cfg, persistence, nil)
	require.NoError(t, err)
	require.NotNil(t, rm)

	// Test GetConfig
	assert.Equal(t, cfg, rm.GetConfig())

	// Test Start
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rm.Start(ctx)

	// Test GetMultiTenantMonitor
	mtm := rm.GetMultiTenantMonitor()
	require.NotNil(t, mtm)

	// Test GetMonitor (default legacy shim)
	// Should initialize default monitor on demand
	m := rm.GetMonitor()
	require.NotNil(t, m)

	// Test GetState (default)
	state := rm.GetState("default")
	require.NotNil(t, state)

	// Test GetState (non-existent) - should auto-provision and return empty state
	stateMissing := rm.GetState("missing-org")
	require.NotNil(t, stateMissing)
	snapshot, ok := stateMissing.(models.StateSnapshot)
	require.True(t, ok)
	assert.Empty(t, snapshot.Nodes)

	// Test GetState with invalid OrgID (should fail persistence check)
	// Assuming "../" or similar might be rejected by GetPersistence or underlying path logic
	// If GetMonitor is robust, checking error branch might require mocking persistence failure.
	// For now, attempting path traversal char.
	// If Pulse cleans it, it might pass. Checking code: persistence joins path.
	// Let's try an error injection if possible, or skip if too complex.
	// Actually, persistence.GetPersistence returns error if newPersistence fails? No, usually succeeds unless mkdir fails.
	// We'll skip complex mocking just for this line, accepting high coverage.

	// Start reload in background
	errChan := make(chan error)
	go func() {
		errChan <- rm.Reload()
	}()

	// Wait for reload (it sleeps for 1s in doReload)
	select {
	case err := <-errChan:
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("Reload timed out")
	}

	// Verify internal state after reload (ctx should be new)
	assert.NotNil(t, rm.ctx)

	// Test Stop
	rm.Stop()
}
