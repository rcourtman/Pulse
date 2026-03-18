package api

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
)

// newMultiTenantRouter creates a test router with a minimal multi-tenant monitor
// so that non-default org requests pass the tenant monitor guard middleware.
// Tests that exercise non-default org behavior must use this instead of NewRouter(cfg, nil, nil, ...).
func newMultiTenantRouter(t *testing.T, cfg *config.Config) *Router {
	t.Helper()
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")
	mtm := monitoring.NewMultiTenantMonitor(cfg, router.multiTenant, nil)
	t.Cleanup(mtm.Stop)
	router.mtMonitor = mtm
	return router
}
