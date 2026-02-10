package registry

import (
	"crypto/rand"
	"fmt"
	"strings"
	"time"
)

// TenantState represents the lifecycle state of a tenant.
type TenantState string

const (
	TenantStateProvisioning TenantState = "provisioning"
	TenantStateActive       TenantState = "active"
	TenantStateSuspended    TenantState = "suspended"
	TenantStateCanceled     TenantState = "canceled"
	TenantStateDeleted      TenantState = "deleted"
)

// Tenant represents a Cloud tenant record in the registry.
type Tenant struct {
	ID                   string      `json:"id"`
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
