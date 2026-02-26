package licensing

import (
	"math"
	"time"
)

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

	// HostedMode indicates that this server is running in Pulse hosted mode.
	// It is used by the frontend to gate hosted-control-plane-only UI.
	HostedMode bool `json:"hosted_mode"`

	// Valid mirrors the effective license validity for display surfaces.
	Valid bool `json:"valid"`

	// LicensedEmail is the activated license email when available.
	LicensedEmail string `json:"licensed_email,omitempty"`

	// ExpiresAt is the RFC3339 expiration timestamp when available.
	ExpiresAt *string `json:"expires_at,omitempty"`

	// IsLifetime indicates a lifetime entitlement with no expiration.
	IsLifetime bool `json:"is_lifetime"`

	// DaysRemaining is the number of days left until expiration.
	DaysRemaining int `json:"days_remaining"`

	// InGracePeriod indicates whether the entitlement is currently in grace.
	InGracePeriod bool `json:"in_grace_period,omitempty"`

	// GracePeriodEnd is the RFC3339 grace period end timestamp when available.
	GracePeriodEnd *string `json:"grace_period_end,omitempty"`

	// TrialEligible indicates whether this org can start a self-serve trial right now.
	TrialEligible bool `json:"trial_eligible"`

	// TrialEligibilityReason is set when trial start is denied.
	TrialEligibilityReason string `json:"trial_eligibility_reason,omitempty"`

	// MaxHistoryDays is the maximum metrics history retention in days for the current tier.
	MaxHistoryDays int `json:"max_history_days"`

	// OverflowDaysRemaining is set when the onboarding overflow (+1 host) is active.
	// Indicates the number of days remaining in the 14-day overflow window.
	OverflowDaysRemaining *int `json:"overflow_days_remaining,omitempty"`
}

// LimitStatus represents a quantitative limit with current usage state.
type LimitStatus struct {
	// Key is the limit identifier (e.g., "max_agents").
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

type EntitlementUsageSnapshot struct {
	Nodes  int64
	Guests int64
}

// BuildEntitlementPayload constructs the normalized payload from LicenseStatus.
func BuildEntitlementPayload(status *LicenseStatus, subscriptionState string) EntitlementPayload {
	return BuildEntitlementPayloadWithUsage(status, subscriptionState, EntitlementUsageSnapshot{}, nil)
}

// BuildEntitlementPayloadWithUsage constructs the normalized payload from LicenseStatus and observed usage.
func BuildEntitlementPayloadWithUsage(
	status *LicenseStatus,
	subscriptionState string,
	usage EntitlementUsageSnapshot,
	trialEndsAtUnix *int64,
) EntitlementPayload {
	if status == nil {
		return EntitlementPayload{
			Capabilities:      []string{},
			Limits:            []LimitStatus{},
			SubscriptionState: string(SubStateExpired),
			UpgradeReasons:    []UpgradeReason{},
			Tier:              string(TierFree),
			MaxHistoryDays:    TierHistoryDays[TierFree],
		}
	}

	maxHistDays := TierHistoryDays[status.Tier]
	if maxHistDays == 0 {
		maxHistDays = TierHistoryDays[TierFree]
	}

	payload := EntitlementPayload{
		Capabilities:   append([]string(nil), status.Features...),
		Limits:         []LimitStatus{},
		Tier:           string(status.Tier),
		UpgradeReasons: []UpgradeReason{},
		Valid:          status.Valid,
		LicensedEmail:  status.Email,
		ExpiresAt:      status.ExpiresAt,
		IsLifetime:     status.IsLifetime,
		DaysRemaining:  status.DaysRemaining,
		InGracePeriod:  status.InGracePeriod,
		GracePeriodEnd: status.GracePeriodEnd,
		MaxHistoryDays: maxHistDays,
	}

	if payload.Capabilities == nil {
		payload.Capabilities = []string{}
	}

	// Use provided subscription state when present; otherwise derive from status.
	if subscriptionState == "" {
		subState := SubStateActive
		if !status.Valid {
			subState = SubStateExpired
		} else if status.InGracePeriod {
			subState = SubStateGrace
		}
		subscriptionState = string(subState)
	}
	payload.SubscriptionState = string(GetBehavior(SubscriptionState(subscriptionState)).State)

	if payload.SubscriptionState == string(SubStateTrial) {
		applyTrialWindow(&payload, status, trialEndsAtUnix, time.Now().Unix())
	}

	// When subscription state doesn't grant paid features, cap history to free tier.
	if !subscriptionStateHasPaidFeatures(SubscriptionState(payload.SubscriptionState)) {
		payload.MaxHistoryDays = TierHistoryDays[TierFree]
	}

	// Build limits.
	if status.MaxAgents > 0 {
		payload.Limits = append(payload.Limits, LimitStatus{
			Key:     MaxAgentsLicenseGateKey,
			Limit:   int64(status.MaxAgents),
			Current: usage.Nodes,
			State:   LimitState(usage.Nodes, int64(status.MaxAgents)),
		})
	}
	if status.MaxGuests > 0 {
		payload.Limits = append(payload.Limits, LimitStatus{
			Key:     "max_guests",
			Limit:   int64(status.MaxGuests),
			Current: usage.Guests,
			State:   LimitState(usage.Guests, int64(status.MaxGuests)),
		})
	}

	reasons := GenerateUpgradeReasons(payload.Capabilities)
	payload.UpgradeReasons = make([]UpgradeReason, 0, len(reasons))
	for _, reason := range reasons {
		payload.UpgradeReasons = append(payload.UpgradeReasons, UpgradeReason{
			Key:       reason.Feature,
			Reason:    reason.Reason,
			ActionURL: reason.ActionURL,
		})
	}

	return payload
}

func applyTrialWindow(payload *EntitlementPayload, status *LicenseStatus, trialEndsAtUnix *int64, nowUnix int64) {
	if payload == nil || status == nil {
		return
	}
	// Prefer billing-state trial timestamps (hosted/self-hosted trial) over license ExpiresAt.
	if trialEndsAtUnix != nil {
		expiresAtUnix := *trialEndsAtUnix
		payload.TrialExpiresAt = &expiresAtUnix
		daysRemaining := remainingTrialDays(expiresAtUnix, nowUnix)
		payload.TrialDaysRemaining = &daysRemaining
		return
	}
	if status.ExpiresAt == nil {
		return
	}
	expiresAt, err := time.Parse(time.RFC3339, *status.ExpiresAt)
	if err != nil {
		return
	}
	expiresAtUnix := expiresAt.Unix()
	payload.TrialExpiresAt = &expiresAtUnix
	daysRemaining := remainingTrialDays(expiresAtUnix, nowUnix)
	payload.TrialDaysRemaining = &daysRemaining
}

func remainingTrialDays(expiresAtUnix, nowUnix int64) int {
	daysRemaining := int(math.Ceil(float64(expiresAtUnix-nowUnix) / 86400.0))
	if daysRemaining < 0 {
		daysRemaining = 0
	}
	return daysRemaining
}

// LimitState returns the over-limit UX state string.
func LimitState(current, limit int64) string {
	if limit <= 0 {
		return "ok" // unlimited
	}
	if current >= limit {
		return "enforced"
	}
	// For small limits (≤10, but >1), warn at N-1 so users get notice before hitting the wall.
	// For larger limits, use 90% threshold.
	if limit > 1 && limit <= 10 {
		if current >= limit-1 {
			return "warning"
		}
	} else if current*10 >= limit*9 {
		return "warning"
	}
	return "ok"
}
