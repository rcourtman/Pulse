package registry

import (
	"crypto/rand"
	"fmt"
	"strings"
	"time"
)

type AccountKind string

const (
	AccountKindIndividual AccountKind = "individual"
	AccountKindMSP        AccountKind = "msp"
)

// Account represents an account record in the control plane registry.
type Account struct {
	ID          string      `json:"id"`
	Kind        AccountKind `json:"kind"`
	DisplayName string      `json:"display_name"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

// User represents a user record in the control plane registry.
type User struct {
	ID          string     `json:"id"`
	Email       string     `json:"email"`
	CreatedAt   time.Time  `json:"created_at"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
}

type MemberRole string

const (
	MemberRoleOwner    MemberRole = "owner"
	MemberRoleAdmin    MemberRole = "admin"
	MemberRoleTech     MemberRole = "tech"
	MemberRoleReadOnly MemberRole = "read_only"
)

// AccountMembership represents a mapping between an account and a user.
type AccountMembership struct {
	AccountID string     `json:"account_id"`
	UserID    string     `json:"user_id"`
	Role      MemberRole `json:"role"`
	CreatedAt time.Time  `json:"created_at"`
}

// TenantState represents the lifecycle state of a tenant.
type TenantState string

const (
	TenantStateProvisioning TenantState = "provisioning"
	TenantStateActive       TenantState = "active"
	TenantStateSuspended    TenantState = "suspended"
	TenantStateCanceled     TenantState = "canceled"
	TenantStateDeleting     TenantState = "deleting"
	TenantStateDeleted      TenantState = "deleted"
	TenantStateFailed       TenantState = "failed"
)

// Tenant represents a Cloud tenant record in the registry.
type Tenant struct {
	ID                   string      `json:"id"`
	AccountID            string      `json:"account_id"`
	Email                string      `json:"email"`
	DisplayName          string      `json:"display_name"`
	State                TenantState `json:"state"`
	StripeCustomerID     string      `json:"stripe_customer_id"`
	StripeSubscriptionID string      `json:"stripe_subscription_id"`
	StripePriceID        string      `json:"stripe_price_id"`
	PlanVersion          string      `json:"plan_version"`
	ContainerID          string      `json:"container_id"`
	CurrentImageDigest   string      `json:"current_image_digest"`
	DesiredImageDigest   string      `json:"desired_image_digest"`
	CreatedAt            time.Time   `json:"created_at"`
	UpdatedAt            time.Time   `json:"updated_at"`
	LastHealthCheck      *time.Time  `json:"last_health_check,omitempty"`
	HealthCheckOK        bool        `json:"health_check_ok"`
}

// StripeAccount maps a control-plane account to a single Stripe customer +
// subscription for consolidated (MSP-style) billing.
type StripeAccount struct {
	AccountID                 string `json:"account_id"`
	StripeCustomerID          string `json:"stripe_customer_id"`
	StripeSubscriptionID      string `json:"stripe_subscription_id"`
	StripeSubItemWorkspacesID string `json:"stripe_sub_item_workspaces_id"`
	PlanVersion               string `json:"plan_version"`
	SubscriptionState         string `json:"subscription_state"` // trial, active, past_due, canceled
	TrialEndsAt               *int64 `json:"trial_ends_at"`
	CurrentPeriodEnd          *int64 `json:"current_period_end"`
	UpdatedAt                 int64  `json:"updated_at"`
}

// crockfordBase32 is the Crockford base32 alphabet (excludes I, L, O, U).
const crockfordBase32 = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// GenerateTenantID returns a tenant ID of the form "t-" followed by 10 random
// Crockford base32 characters (50 bits of entropy).
func GenerateTenantID() (string, error) {
	b := make([]byte, 10)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate tenant id: %w", err)
	}
	var sb strings.Builder
	sb.WriteString("t-")
	for _, v := range b {
		sb.WriteByte(crockfordBase32[int(v)%len(crockfordBase32)])
	}
	return sb.String(), nil
}

// GenerateAccountID returns an account ID of the form "a_" followed by 10 random
// Crockford base32 characters (50 bits of entropy).
func GenerateAccountID() (string, error) {
	b := make([]byte, 10)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate account id: %w", err)
	}
	var sb strings.Builder
	sb.WriteString("a_")
	for _, v := range b {
		sb.WriteByte(crockfordBase32[int(v)%len(crockfordBase32)])
	}
	return sb.String(), nil
}

// GenerateUserID returns a user ID of the form "u_" followed by 10 random
// Crockford base32 characters (50 bits of entropy).
func GenerateUserID() (string, error) {
	b := make([]byte, 10)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate user id: %w", err)
	}
	var sb strings.Builder
	sb.WriteString("u_")
	for _, v := range b {
		sb.WriteByte(crockfordBase32[int(v)%len(crockfordBase32)])
	}
	return sb.String(), nil
}
