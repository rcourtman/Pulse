package registry

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

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

// Billing authority must remain revocable even when a crashed data transaction
// left a runtime recovery hold. This separate, non-persistent lease orders
// billing events; it is never held waiting for the runtime lease.
func (r *TenantRegistry) AcquireBillingAuthority(ctx context.Context, customerID string) (context.Context, *lifecycle.Lease, error) {
	if r == nil {
		return ctx, nil, fmt.Errorf("tenant registry is required")
	}
	return lifecycle.Acquire(ctx, filepath.Join(r.dir, "billing-authority"), customerID)
}

// UpdateRuntimeState deliberately cannot write lifecycle or billing authority.
// A stopped-writer transaction can outlive a legitimate cancellation.
func (r *TenantRegistry) UpdateRuntimeState(t *Tenant) error {
	if t == nil {
		return fmt.Errorf("tenant is nil")
	}
	res, err := r.db.Exec(`UPDATE tenants SET container_id = ?, current_image_digest = ?, desired_image_digest = ?, last_health_check = ?, health_check_ok = ?, updated_at = ? WHERE id = ?`,
		t.ContainerID, t.CurrentImageDigest, t.DesiredImageDigest, nullableTimeUnix(t.LastHealthCheck), boolToInt(t.HealthCheckOK), time.Now().UTC().Unix(), t.ID)
	if err != nil {
		return fmt.Errorf("update tenant runtime: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("tenant %q not found", t.ID)
	}
	return nil
}

func (r *TenantRegistry) SetTenantState(id string, state TenantState) error {
	res, err := r.db.Exec(`UPDATE tenants SET state = ?, updated_at = ? WHERE id = ?`, string(state), time.Now().UTC().Unix(), id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("tenant %q not found", id)
	}
	return nil
}

// TransitionTenantState cannot reactivate an intervening withdrawal or replace
// a canceled tenant with a stale provisioning-cleanup decision.
func (r *TenantRegistry) TransitionTenantState(id string, from, to TenantState) error {
	res, err := r.db.Exec(`UPDATE tenants SET state = ?, updated_at = ? WHERE id = ? AND state = ?`, string(to), time.Now().UTC().Unix(), id, string(from))
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("tenant %q state changed before transition", id)
	}
	return nil
}
