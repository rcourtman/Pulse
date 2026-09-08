package api

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// cleanupTestRouter gives the fixture ownership of constructor-started workers.
// Register immediately after construction, before request assertions can fail.
func cleanupTestRouter(t *testing.T, router *Router) {
	t.Helper()
	t.Cleanup(func() {
		if router.agentExecServer != nil {
			router.agentExecServer.Shutdown()
		}
		router.shutdownBackgroundWorkers()
		router.ShutdownResourceStores()
		router.ShutdownRBAC()
	})
}

func TestRouterFixtureCleanupCancelsLifecycle(t *testing.T) {
	var router *Router
	workerDone := make(chan struct{})
	t.Run("fixture", func(t *testing.T) {
		cfg := &config.Config{DataPath: t.TempDir()}
		router = NewRouter(cfg, nil, nil, nil, nil, "test")
		cleanupTestRouter(t, router)
		router.startLifecycleWorker(func() {
			<-router.lifecycleCtx.Done()
			close(workerDone)
		})
	})
	// Also clean up after a regression so the proof itself cannot leak.
	defer router.shutdownBackgroundWorkers()
	defer router.ShutdownResourceStores()
	defer router.ShutdownRBAC()
	select {
	case <-router.lifecycleCtx.Done():
	default:
		t.Fatal("router lifecycle remains active after fixture cleanup")
	}
	select {
	case <-workerDone:
	default:
		t.Fatal("router lifecycle worker was not joined by fixture cleanup")
	}
}
