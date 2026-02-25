package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

// EntitlementPayload is the normalized entitlement response for frontend consumption.
// Frontend should use this instead of inferring capabilities from tier names.
type EntitlementPayload = entitlementPayloadModel

// LimitStatus represents a quantitative limit with current usage state.
type LimitStatus = limitStatusModel

// UpgradeReason provides context for why a user should upgrade.
type UpgradeReason = upgradeReasonModel

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

	// Build payload from current license status (which is evaluator-aware when no JWT is present).
	status := svc.Status()
	usage := h.entitlementUsageSnapshot(r.Context())
	trialEndsAtUnix := trialEndsAtUnixFromService(svc)

	// Onboarding overflow: +1 host for 14 days on free tier.
	overflowGrantedAt := h.ensureOnboardingOverflow(r.Context(), status.Tier)
	now := time.Now()
	if bonus := pkglicensing.OverflowBonus(status.Tier, overflowGrantedAt, now); bonus > 0 {
		status.MaxNodes += bonus
	}

	payload := buildEntitlementPayloadWithUsage(status, svc.SubscriptionState(), usage, trialEndsAtUnix)

	// Surface overflow days remaining for frontend messaging.
	if days := pkglicensing.OverflowDaysRemaining(status.Tier, overflowGrantedAt, now); days > 0 {
		payload.OverflowDaysRemaining = &days
	}

	if eval := svc.Evaluator(); eval != nil {
		if pv := strings.TrimSpace(eval.PlanVersion()); pv != "" {
			payload.PlanVersion = pv
		}
	}
	payload.TrialEligible, payload.TrialEligibilityReason = h.trialStartEligibility(r.Context(), svc)
	payload.HostedMode = h != nil && h.hostedMode

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(payload)
}

func trialEndsAtUnixFromService(svc *licenseService) *int64 {
	if svc == nil {
		return nil
	}
	eval := svc.Evaluator()
	if eval == nil {
		return nil
	}
	return eval.TrialEndsAt()
}

type entitlementUsageSnapshot = entitlementUsageSnapshotModel

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
func buildEntitlementPayload(status *licenseStatus, subscriptionState string) EntitlementPayload {
	return buildEntitlementPayloadFromLicensing(status, subscriptionState)
}

// buildEntitlementPayloadWithUsage constructs the normalized payload from LicenseStatus and observed usage.
func buildEntitlementPayloadWithUsage(
	status *licenseStatus,
	subscriptionState string,
	usage entitlementUsageSnapshot,
	trialEndsAtUnix *int64,
) EntitlementPayload {
	return buildEntitlementPayloadWithUsageFromLicensing(status, subscriptionState, usage, trialEndsAtUnix)
}

// limitState returns the over-limit UX state string.
// Exported for testing.
func limitState(current, limit int64) string {
	return limitStateFromLicensing(current, limit)
}

// ensureOnboardingOverflow lazy-initializes the overflow grant for free-tier workspaces.
// Returns the OverflowGrantedAt timestamp (may be nil for non-free tiers or missing billing store).
func (h *LicenseHandlers) ensureOnboardingOverflow(ctx context.Context, tier pkglicensing.Tier) *int64 {
	if tier != pkglicensing.TierFree || h == nil || h.mtPersistence == nil {
		return nil
	}

	orgID := GetOrgID(ctx)
	if orgID == "" {
		orgID = "default"
	}

	billingStore := config.NewFileBillingStore(h.mtPersistence.BaseDataDir())
	existing, err := billingStore.GetBillingState(orgID)
	if err != nil {
		return nil
	}

	now := time.Now().Unix()

	if existing == nil {
		// No billing state yet. Re-read to handle the case where a concurrent
		// request (e.g. trial start) created billing state between our reads.
		fresh, freshErr := billingStore.GetBillingState(orgID)
		if freshErr != nil {
			return nil
		}
		if fresh != nil {
			// Another request created billing state first — add overflow to it.
			if fresh.OverflowGrantedAt != nil {
				return fresh.OverflowGrantedAt
			}
			fresh.OverflowGrantedAt = &now
			if saveErr := billingStore.SaveBillingState(orgID, fresh); saveErr != nil {
				return nil
			}
			return &now
		}

		// Still nil — create minimal state with only the overflow grant.
		// Use empty subscription state (not trial) to avoid accidentally changing
		// the org's subscription lifecycle.
		state := &pkglicensing.BillingState{
			Capabilities:      []string{},
			Limits:            map[string]int64{},
			MetersEnabled:     []string{},
			OverflowGrantedAt: &now,
		}
		if saveErr := billingStore.SaveBillingState(orgID, state); saveErr != nil {
			return nil
		}
		return &now
	}

	// Already granted — return existing timestamp (set-once).
	if existing.OverflowGrantedAt != nil {
		return existing.OverflowGrantedAt
	}

	// First access with existing billing state but no overflow yet — grant now.
	// Re-read to minimize the race window with concurrent billing state writers
	// (e.g. trial start). We only touch OverflowGrantedAt, preserving all other fields.
	fresh, freshErr := billingStore.GetBillingState(orgID)
	if freshErr != nil || fresh == nil {
		return nil
	}
	if fresh.OverflowGrantedAt != nil {
		// Another request won the race — use their timestamp.
		return fresh.OverflowGrantedAt
	}
	fresh.OverflowGrantedAt = &now
	if saveErr := billingStore.SaveBillingState(orgID, fresh); saveErr != nil {
		return nil
	}
	return &now
}

// overflowGrantedAtForContext returns the OverflowGrantedAt timestamp for the
// current org, reading from the evaluator first (hosted path), then falling
// back to billing state on disk (self-hosted path). Does NOT lazy-initialize.
func (h *LicenseHandlers) overflowGrantedAtForContext(ctx context.Context) *int64 {
	if h == nil || h.mtPersistence == nil {
		return nil
	}

	// Hosted path: evaluator already has OverflowGrantedAt cached.
	svc, _, err := h.getTenantComponents(ctx)
	if err == nil && svc != nil {
		if eval := svc.Evaluator(); eval != nil {
			return eval.OverflowGrantedAt()
		}
	}

	// Self-hosted path: read from billing state directly.
	orgID := GetOrgID(ctx)
	if orgID == "" {
		orgID = "default"
	}
	billingStore := config.NewFileBillingStore(h.mtPersistence.BaseDataDir())
	existing, readErr := billingStore.GetBillingState(orgID)
	if readErr != nil || existing == nil {
		return nil
	}
	return existing.OverflowGrantedAt
}

func (h *LicenseHandlers) trialStartEligibility(ctx context.Context, svc *licenseService) (eligible bool, reason string) {
	if h == nil || h.mtPersistence == nil {
		return false, "unavailable"
	}

	orgID := GetOrgID(ctx)
	if orgID == "" {
		orgID = "default"
	}

	billingStore := config.NewFileBillingStore(h.mtPersistence.BaseDataDir())
	existing, err := billingStore.GetBillingState(orgID)
	if err != nil {
		return false, "unavailable"
	}

	hasActiveLicense := svc != nil && svc.Current() != nil && svc.IsValid()
	decision := evaluateTrialStartEligibilityFromLicensing(hasActiveLicense, existing)
	if decision.Allowed {
		return true, ""
	}
	return false, string(decision.Reason)
}
