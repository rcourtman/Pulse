package api

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/license"
	"github.com/rcourtman/pulse-go-rewrite/internal/license/conversion"
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

	// HostedMode indicates that this server is running in Pulse hosted mode.
	// It is used by the frontend to gate hosted-control-plane-only UI.
	HostedMode bool `json:"hosted_mode"`
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
	usage := h.entitlementUsageSnapshot(r.Context())
	trialEndsAtUnix := trialEndsAtUnixFromService(svc)
	payload := buildEntitlementPayloadWithUsage(status, svc.SubscriptionState(), usage, trialEndsAtUnix)
	payload.HostedMode = h != nil && h.hostedMode

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(payload)
}

func trialEndsAtUnixFromService(svc *license.Service) *int64 {
	if svc == nil {
		return nil
	}
	eval := svc.Evaluator()
	if eval == nil {
		return nil
	}
	return eval.TrialEndsAt()
}

type entitlementUsageSnapshot struct {
	Nodes  int64
	Guests int64
}

// entitlementUsageSnapshot returns best-effort runtime usage counts for limits.
func (h *LicenseHandlers) entitlementUsageSnapshot(ctx context.Context) entitlementUsageSnapshot {
	usage := entitlementUsageSnapshot{}
	if h == nil || h.mtPersistence == nil {
		return usage
	}

	orgID := GetOrgID(ctx)
	if orgID == "" {
		orgID = "default"
	}

	persistence, err := h.mtPersistence.GetPersistence(orgID)
	if err != nil || persistence == nil {
		return usage
	}

	if nodesConfig, err := persistence.LoadNodesConfig(); err == nil && nodesConfig != nil {
		usage.Nodes = int64(len(nodesConfig.PVEInstances) + len(nodesConfig.PBSInstances) + len(nodesConfig.PMGInstances))
	}

	// Guest metadata is currently the most broadly available tenant-level guest index.
	if guestStore := persistence.GetGuestMetadataStore(); guestStore != nil {
		usage.Guests = int64(len(guestStore.GetAll()))
	}

	return usage
}

// buildEntitlementPayload constructs the normalized payload from LicenseStatus.
// This provides backward compatibility before the evaluator is wired in.
func buildEntitlementPayload(status *license.LicenseStatus, subscriptionState string) EntitlementPayload {
	return buildEntitlementPayloadWithUsage(status, subscriptionState, entitlementUsageSnapshot{}, nil)
}

// buildEntitlementPayloadWithUsage constructs the normalized payload from LicenseStatus and observed usage.
func buildEntitlementPayloadWithUsage(
	status *license.LicenseStatus,
	subscriptionState string,
	usage entitlementUsageSnapshot,
	trialEndsAtUnix *int64,
) EntitlementPayload {
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

	if payload.SubscriptionState == string(license.SubStateTrial) {
		// Prefer billing-state trial timestamps (hosted/self-hosted trial) over license ExpiresAt.
		if trialEndsAtUnix != nil {
			expiresAtUnix := *trialEndsAtUnix
			payload.TrialExpiresAt = &expiresAtUnix

			daysRemaining := int(math.Ceil(float64(expiresAtUnix-time.Now().Unix()) / 86400.0))
			if daysRemaining < 0 {
				daysRemaining = 0
			}
			payload.TrialDaysRemaining = &daysRemaining
		} else if status.ExpiresAt != nil {
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
	}

	// Build limits.
	if status.MaxNodes > 0 {
		payload.Limits = append(payload.Limits, LimitStatus{
			Key:     "max_nodes",
			Limit:   int64(status.MaxNodes),
			Current: usage.Nodes,
			State:   limitState(usage.Nodes, int64(status.MaxNodes)),
		})
	}
	if status.MaxGuests > 0 {
		payload.Limits = append(payload.Limits, LimitStatus{
			Key:     "max_guests",
			Limit:   int64(status.MaxGuests),
			Current: usage.Guests,
			State:   limitState(usage.Guests, int64(status.MaxGuests)),
		})
	}

	reasons := conversion.GenerateUpgradeReasons(payload.Capabilities)
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
