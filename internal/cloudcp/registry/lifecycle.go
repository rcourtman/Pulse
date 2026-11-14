package registry

import (
	"context"
	"fmt"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/lifecycle"
)

// AcquireRuntimeLifecycle coordinates with the separate rollout command using
// the registry's storage directory, not a process-local mutex or a DB lease.
func (r *TenantRegistry) AcquireRuntimeLifecycle(ctx context.Context, tenantID string) (context.Context, *lifecycle.Lease, error) {
	if r == nil {
		return ctx, nil, fmt.Errorf("tenant registry is required")
	}
	return lifecycle.Acquire(ctx, r.dir, tenantID)
}
