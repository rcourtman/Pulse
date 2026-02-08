package api

import (
	"encoding/json"
	"math"
	"net/http"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/license"
	evaluator "github.com/rcourtman/pulse-go-rewrite/internal/license/entitlements"
	"github.com/rcourtman/pulse-go-rewrite/internal/license/subscription"
)

// Keep evaluator package imported ahead of MON-08 service wiring.
var _ = evaluator.NewEvaluator

// EntitlementPayload is the normalized entitlement response for frontend consumption.
// Frontend should use this instead of inferring capabilities from tier names.
type EntitlementPayload struct {
	// Capabilities lists all granted capability keys.
	Capabilities []string `json:"capabilities"`

	// Limits lists quantitative limits with current usage.
	Limits []LimitStatus `json:"limits"`

	// SubscriptionState is the current subscription lifecycle state.
	SubscriptionState string `json:"subscription_state"`

	// UpgradeReasons provides user-actionable upgrade prompts.
	UpgradeReasons []UpgradeReason `json:"upgrade_reasons"`

	// PlanVersion preserves grandfathered terms.
	PlanVersion string `json:"plan_version,omitempty"`

	// Tier is the marketing tier name (for display only, never gate on this).
	Tier string `json:"tier"`

	// TrialExpiresAt is the trial expiration Unix timestamp when in trial state.
	TrialExpiresAt *int64 `json:"trial_expires_at,omitempty"`

	// TrialDaysRemaining is the number of whole or partial days remaining in trial.
	TrialDaysRemaining *int `json:"trial_days_remaining,omitempty"`
}

// LimitStatus represents a quantitative limit with current usage state.
type LimitStatus struct {
	// Key is the limit identifier (e.g., "max_nodes").
	Key string `json:"key"`

	// Limit is the maximum allowed value (0 = unlimited).
	Limit int64 `json:"limit"`

	// Current is the observed current usage.
	Current int64 `json:"current"`

	// State describes the over-limit UX state.
	// Values: "ok", "warning", "enforced"
	State string `json:"state"`
}

// UpgradeReason provides context for why a user should upgrade.
type UpgradeReason struct {
	// Key is the capability or limit this reason relates to.
	Key string `json:"key"`

	// Reason is a user-facing description of why upgrading helps.
	Reason string `json:"reason"`

	// ActionURL is where the user can go to upgrade.
	ActionURL string `json:"action_url,omitempty"`
}

// HandleEntitlements returns the normalized entitlement payload for the current tenant.
// This is the primary endpoint the frontend should use for feature gating decisions.
func (h *LicenseHandlers) HandleEntitlements(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc, _, err := h.getTenantComponents(r.Context())
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Build payload from current license status (evaluator wiring comes in MON-08)
	status := svc.Status()
	payload := buildEntitlementPayload(status, svc.SubscriptionState())

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(payload)
}

// buildEntitlementPayload constructs the normalized payload from LicenseStatus.
// This provides backward compatibility before the evaluator is wired in.
func buildEntitlementPayload(status *license.LicenseStatus, subscriptionState string) EntitlementPayload {
	if status == nil {
		return EntitlementPayload{
			Capabilities:      []string{},
			Limits:            []LimitStatus{},
			SubscriptionState: string(license.SubStateExpired),
			UpgradeReasons:    []UpgradeReason{},
			Tier:              string(license.TierFree),
		}
	}

	payload := EntitlementPayload{
		Capabilities:   append([]string(nil), status.Features...),
		Limits:         []LimitStatus{},
		Tier:           string(status.Tier),
		UpgradeReasons: []UpgradeReason{},
	}

	if payload.Capabilities == nil {
		payload.Capabilities = []string{}
	}

	// Use provided subscription state when present; otherwise derive from status.
	if subscriptionState == "" {
		subState := license.SubStateActive
		if !status.Valid {
			subState = license.SubStateExpired
		} else if status.InGracePeriod {
			subState = license.SubStateGrace
		}
		subscriptionState = string(subState)
	}
	payload.SubscriptionState = string(subscription.GetBehavior(license.SubscriptionState(subscriptionState)).State)

	if payload.SubscriptionState == string(license.SubStateTrial) && status.ExpiresAt != nil {
		if expiresAt, err := time.Parse(time.RFC3339, *status.ExpiresAt); err == nil {
			expiresAtUnix := expiresAt.Unix()
			payload.TrialExpiresAt = &expiresAtUnix

			daysRemaining := int(math.Ceil(float64(expiresAtUnix-time.Now().Unix()) / 86400.0))
			if daysRemaining < 0 {
				daysRemaining = 0
			}
			payload.TrialDaysRemaining = &daysRemaining
		}
	}

	// Build limits.
	if status.MaxNodes > 0 {
		payload.Limits = append(payload.Limits, LimitStatus{
			Key:     "max_nodes",
			Limit:   int64(status.MaxNodes),
			Current: 0, // Actual count will be wired when evaluator is integrated.
			State:   limitState(0, int64(status.MaxNodes)),
		})
	}
	if status.MaxGuests > 0 {
		payload.Limits = append(payload.Limits, LimitStatus{
			Key:     "max_guests",
			Limit:   int64(status.MaxGuests),
			Current: 0,
			State:   limitState(0, int64(status.MaxGuests)),
		})
	}

	// Generate upgrade reasons for free tier.
	if status.Tier == license.TierFree {
		payload.UpgradeReasons = append(payload.UpgradeReasons, UpgradeReason{
			Key:    "ai_autofix",
			Reason: "Upgrade to Pro to enable automatic remediation with Pulse Patrol.",
		})
		payload.UpgradeReasons = append(payload.UpgradeReasons, UpgradeReason{
			Key:    "rbac",
			Reason: "Upgrade to Pro to enable role-based access control.",
		})
	}

	return payload
}

// limitState returns the over-limit UX state string.
// Exported for testing.
func limitState(current, limit int64) string {
	if limit <= 0 {
		return "ok" // unlimited
	}
	if current >= limit {
		return "enforced"
	}
	// 90% threshold for warning.
	if current*10 >= limit*9 {
		return "warning"
	}
	return "ok"
}
