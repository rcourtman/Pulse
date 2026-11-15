package registry

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

// WorkspaceLimitPolicy is server configuration, not a request or a fallback
// inferred from a tenant's saved plan. Only configured MSP control planes may
// admit an MSP account without a Stripe mapping.
type WorkspaceLimitPolicy struct {
	ProviderHostedMSP        bool
	ProviderMSPPlanVersion   string
	ProviderMSPLicenseLapsed func() bool
}

type WorkspaceAdmission struct {
	PlanVersion          string
	SubscriptionState    pkglicensing.SubscriptionState
	Limit                int
	StripeCustomerID     string
	StripeSubscriptionID string
}

// WorkspaceAdmissionError is a definite policy refusal. Storage failures stay
// ordinary errors so the HTTP caller does not misreport them as subscription
// withdrawal or ask the user to buy a replacement plan.
type WorkspaceAdmissionError struct {
	Reason  string
	Message string
}

func (e *WorkspaceAdmissionError) Error() string { return e.Message }

type workspaceAdmissionReader interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func (r *TenantRegistry) ResolveWorkspaceAdmission(ctx context.Context, accountID string, policy WorkspaceLimitPolicy) (*WorkspaceAdmission, error) {
	return resolveWorkspaceAdmission(ctx, r.db, accountID, policy)
}

func resolveWorkspaceAdmission(ctx context.Context, reader workspaceAdmissionReader, accountID string, policy WorkspaceLimitPolicy) (*WorkspaceAdmission, error) {
	var kind string
	var billingAccount, plan, state, customer, subscription sql.NullString
	err := reader.QueryRowContext(ctx, `
		SELECT a.kind, sa.account_id, sa.plan_version, sa.subscription_state,
		       sa.stripe_customer_id, sa.stripe_subscription_id
		FROM accounts a LEFT JOIN stripe_accounts sa ON sa.account_id = a.id
		WHERE a.id = ?`, strings.TrimSpace(accountID)).Scan(&kind, &billingAccount, &plan, &state, &customer, &subscription)
	if err == sql.ErrNoRows {
		return nil, &WorkspaceAdmissionError{"account_not_found", "account not found"}
	}
	if err != nil {
		return nil, fmt.Errorf("load workspace admission: %w", err)
	}

	admission := &WorkspaceAdmission{}
	if billingAccount.Valid {
		// A current billing record takes precedence even on an MSP control plane.
		// Its suspension or incomplete metadata must never become local authority.
		admission.PlanVersion = pkglicensing.CanonicalizePlanVersion(plan.String)
		admission.SubscriptionState = pkglicensing.ParseStoredSubscriptionState(state.String)
		admission.StripeCustomerID = strings.TrimSpace(customer.String)
		admission.StripeSubscriptionID = strings.TrimSpace(subscription.String)
		if !pkglicensing.ShouldGrantPaidCapabilities(admission.SubscriptionState) {
			reason, message := "subscription_inactive", "cannot create workspaces on an inactive subscription"
			if admission.SubscriptionState == pkglicensing.SubStateCanceled {
				reason, message = "subscription_canceled", "cannot create workspaces on a canceled subscription"
			}
			return nil, &WorkspaceAdmissionError{reason, message}
		}
		if admission.PlanVersion == "" {
			return nil, &WorkspaceAdmissionError{"billing_plan_missing", "subscription plan is unavailable; no workspace was added"}
		}
	} else {
		if !policy.ProviderHostedMSP || AccountKind(kind) != AccountKindMSP {
			return nil, &WorkspaceAdmissionError{"no_billing_record", "account has no active subscription"}
		}
		if policy.ProviderMSPLicenseLapsed != nil && policy.ProviderMSPLicenseLapsed() {
			return nil, &WorkspaceAdmissionError{"provider_msp_license_lapsed", "Your Pulse MSP plan has ended, so no new clients can be added. Buy a plan from Plan in the portal to add clients again."}
		}
		admission.PlanVersion = pkglicensing.CanonicalizePlanVersion(policy.ProviderMSPPlanVersion)
		if admission.PlanVersion == "" {
			return nil, &WorkspaceAdmissionError{"provider_msp_plan_missing", "provider MSP plan is not configured"}
		}
		if _, known := pkglicensing.WorkspaceLimitForPlan(admission.PlanVersion); !known || !strings.HasPrefix(admission.PlanVersion, "msp_") {
			return nil, &WorkspaceAdmissionError{"provider_msp_plan_unknown", "provider MSP plan has no workspace limit"}
		}
		admission.SubscriptionState = pkglicensing.SubStateActive
	}
	admission.Limit, _ = pkglicensing.WorkspaceLimitForPlan(admission.PlanVersion)
	return admission, nil
}
