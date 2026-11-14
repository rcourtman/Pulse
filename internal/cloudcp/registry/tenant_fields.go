package registry

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

var (
	ErrTenantNotFound      = errors.New("tenant not found")
	ErrTenantStateConflict = errors.New("tenant state does not allow this change")
)

// IsTenantDeletionState identifies rows whose runtime must not be reactivated
// by account edits or later billing events. Billing/account metadata may remain.
func IsTenantDeletionState(state TenantState) bool {
	return state == TenantStateDeleting || state == TenantStateDeleted
}

const accountTenantSelect = `SELECT id, account_id, email, display_name, state,
 stripe_customer_id, stripe_subscription_id, stripe_price_id, plan_version,
 container_id, current_image_digest, desired_image_digest, created_at, updated_at,
 last_health_check, health_check_ok, manual_hold FROM tenants WHERE account_id = ? AND id = ?`

// mutateAccountTenant takes the same short write transaction used by workspace
// creation. It snapshots current fields for the response/audit, never a request's
// pre-body snapshot. The runtime lease remains the caller's responsibility; a
// billing withdrawal need not take that lease and is ordered by this write.
func (r *TenantRegistry) mutateAccountTenant(ctx context.Context, accountID, tenantID string, mutate func(*sql.Conn, *Tenant) error) (*Tenant, *Tenant, error) {
	conn, err := r.db.Conn(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return nil, nil, err
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(context.Background(), "ROLLBACK")
		}
	}()
	previous, err := scanTenant(conn.QueryRowContext(ctx, accountTenantSelect, accountID, tenantID))
	if err != nil {
		return nil, nil, err
	}
	if previous == nil {
		return nil, nil, ErrTenantNotFound
	}
	if err := mutate(conn, previous); err != nil {
		return nil, nil, err
	}
	current, err := scanTenant(conn.QueryRowContext(ctx, accountTenantSelect, accountID, tenantID))
	if err != nil {
		return nil, nil, err
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return nil, nil, err
	}
	committed = true
	return previous, current, nil
}

// PatchTenantForAccount changes only account-editable fields. The entire patch
// is rejected if its state transition is no longer valid; a rename alone can
// never rewrite billing, health, runtime identity or a concurrent withdrawal.
func (r *TenantRegistry) PatchTenantForAccount(ctx context.Context, accountID, tenantID string, displayName *string, state *TenantState, localMSPBilling ...bool) (*Tenant, *Tenant, error) {
	return r.mutateAccountTenant(ctx, accountID, tenantID, func(conn *sql.Conn, current *Tenant) error {
		if IsTenantDeletionState(current.State) {
			return ErrTenantStateConflict
		}
		name, next, hold := current.DisplayName, current.State, current.ManualHold
		if displayName != nil {
			name = strings.TrimSpace(*displayName)
			if name == "" {
				return fmt.Errorf("invalid display name")
			}
		}
		if state != nil {
			if (*state != TenantStateActive && *state != TenantStateSuspended) || (current.State != TenantStateActive && current.State != TenantStateSuspended) {
				return ErrTenantStateConflict
			}
			next = *state
			if next == TenantStateSuspended {
				hold = current.ManualHold || current.State == TenantStateActive
			} else {
				hold = false
			}
			if current.State == TenantStateSuspended && next == TenantStateActive {
				if !current.ManualHold {
					return ErrTenantStateConflict
				}
				allowed, err := canResumeAccountTenant(ctx, conn, current, len(localMSPBilling) > 0 && localMSPBilling[0])
				if err != nil {
					return err
				}
				if !allowed {
					return ErrTenantStateConflict
				}
			}
		}
		_, err := conn.ExecContext(ctx, `UPDATE tenants SET display_name = ?, state = ?, manual_hold = ?, updated_at = ? WHERE account_id = ? AND id = ?`, name, string(next), boolToInt(hold), time.Now().UTC().Unix(), accountID, tenantID)
		return err
	})
}

func canResumeAccountTenant(ctx context.Context, conn *sql.Conn, tenant *Tenant, localMSPBilling bool) (bool, error) {
	var customer, storedState string
	err := conn.QueryRowContext(ctx, `SELECT stripe_customer_id, subscription_state FROM stripe_accounts WHERE account_id = ?`, tenant.AccountID).Scan(&customer, &storedState)
	if errors.Is(err, sql.ErrNoRows) {
		if !localMSPBilling {
			return false, nil
		}
		// Configured MSP control planes have no Stripe identity. Missing billing for
		// a billed tenant is unknown authority, not permission to resume it.
		if tenant.StripeCustomerID != "" || tenant.StripeSubscriptionID != "" || tenant.StripePriceID != "" {
			return false, nil
		}
		var kind AccountKind
		if err := conn.QueryRowContext(ctx, `SELECT kind FROM accounts WHERE id = ?`, tenant.AccountID).Scan(&kind); err != nil {
			return false, err
		}
		return kind == AccountKindMSP, nil
	}
	if err != nil {
		return false, err
	}
	if tenant.StripeCustomerID != "" && tenant.StripeCustomerID != customer {
		return false, nil
	}
	return pkglicensing.ShouldGrantPaidCapabilities(pkglicensing.ParseStoredSubscriptionState(storedState)), nil
}

// BeginTenantDeletionForAccount withdraws the current row without rewriting any
// of its other fields. Retrying an interrupted deletion is allowed; completed
// deletion is idempotent and never becomes deleting again.
func (r *TenantRegistry) BeginTenantDeletionForAccount(ctx context.Context, accountID, tenantID string) (*Tenant, *Tenant, error) {
	return r.mutateAccountTenant(ctx, accountID, tenantID, func(conn *sql.Conn, current *Tenant) error {
		if IsTenantDeletionState(current.State) {
			return nil
		}
		_, err := conn.ExecContext(ctx, `UPDATE tenants SET state = ?, updated_at = ? WHERE account_id = ? AND id = ?`, string(TenantStateDeleting), time.Now().UTC().Unix(), accountID, tenantID)
		return err
	})
}

// FinalizeTenantDeletionForAccount clears only the runtime being deprovisioned.
// In particular it cannot restore old billing/display/health metadata captured
// before a slow container operation.
func (r *TenantRegistry) FinalizeTenantDeletionForAccount(ctx context.Context, accountID, tenantID string) (*Tenant, *Tenant, error) {
	return r.mutateAccountTenant(ctx, accountID, tenantID, func(conn *sql.Conn, current *Tenant) error {
		if current.State == TenantStateDeleted {
			return nil
		}
		if current.State != TenantStateDeleting {
			return ErrTenantStateConflict
		}
		_, err := conn.ExecContext(ctx, `UPDATE tenants SET state = ?, container_id = '', updated_at = ? WHERE account_id = ? AND id = ?`, string(TenantStateDeleted), time.Now().UTC().Unix(), accountID, tenantID)
		return err
	})
}

// This additive migration only infers a legacy manual hold for local MSP
// clients with no billing identity. Old stored past_due conflated grace with
// paused/unknown billing, so a paid legacy suspension has unknown cause: do not
// manufacture resume authority from it. A fresh granting billing event can
// reconcile it; subsequent deliberate suspensions carry their own hold.
func (r *TenantRegistry) migrateTenantManualHolds() error {
	has, err := r.tenantsHasColumn("manual_hold")
	if err != nil || has {
		return err
	}
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`ALTER TABLE tenants ADD COLUMN manual_hold INTEGER NOT NULL DEFAULT 0`); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE tenants SET manual_hold = 1
 WHERE state = 'suspended' AND stripe_customer_id = '' AND stripe_subscription_id = '' AND stripe_price_id = ''
 AND NOT EXISTS (SELECT 1 FROM stripe_accounts sa WHERE sa.account_id = tenants.account_id)
 AND EXISTS (SELECT 1 FROM accounts a WHERE a.id = tenants.account_id AND a.kind = 'msp')`); err != nil {
		return err
	}
	return tx.Commit()
}

// ApplyTenantBillingGrant writes only billing-owned fields and derives the
// effective lifecycle from the current manual hold. Its caller must hold the
// billing-authority and runtime leases; it cannot recover incomplete/deleted
// runtimes or restore a stale display/runtime snapshot.
func (r *TenantRegistry) ApplyTenantBillingGrant(ctx context.Context, accountID, tenantID, subscriptionID, priceID, planVersion string) (*Tenant, bool, error) {
	applied := false
	_, current, err := r.mutateAccountTenant(ctx, accountID, tenantID, func(conn *sql.Conn, current *Tenant) error {
		switch current.State {
		case TenantStateActive, TenantStateSuspended, TenantStateCanceled:
		default:
			return nil
		}
		next := TenantStateActive
		if current.ManualHold {
			next = TenantStateSuspended
		}
		_, err := conn.ExecContext(ctx, `UPDATE tenants SET state = ?, stripe_subscription_id = ?, stripe_price_id = ?, plan_version = ?, updated_at = ? WHERE account_id = ? AND id = ?`, string(next), subscriptionID, priceID, canonicalizeRegistryPlanVersion(planVersion), time.Now().UTC().Unix(), accountID, tenantID)
		applied = err == nil
		return err
	})
	return current, applied, err
}
