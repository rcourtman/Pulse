package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

// EntitlementPayload is the normalized entitlement response for frontend consumption.
// Frontend should use this instead of inferring capabilities from tier names.
type EntitlementPayload = pkglicensing.EntitlementPayload

// LimitStatus represents a quantitative limit with current usage state.
type LimitStatus = pkglicensing.LimitStatus

// UpgradeReason provides context for why a user should upgrade.
type UpgradeReason = pkglicensing.UpgradeReason

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
	payload := buildEntitlementPayloadWithUsage(status, svc.SubscriptionState(), usage, trialEndsAtUnix)
	if eval := svc.Evaluator(); eval != nil {
		if pv := strings.TrimSpace(eval.PlanVersion()); pv != "" {
			payload.PlanVersion = pv
		}
	}
	payload.HostedMode = h != nil && h.hostedMode

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(payload)
}

func trialEndsAtUnixFromService(svc *pkglicensing.Service) *int64 {
	if svc == nil {
		return nil
	}
	eval := svc.Evaluator()
	if eval == nil {
		return nil
	}
	return eval.TrialEndsAt()
}

type entitlementUsageSnapshot = pkglicensing.EntitlementUsageSnapshot

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
func buildEntitlementPayload(status *pkglicensing.LicenseStatus, subscriptionState string) EntitlementPayload {
	return pkglicensing.BuildEntitlementPayload(status, subscriptionState)
}

// buildEntitlementPayloadWithUsage constructs the normalized payload from LicenseStatus and observed usage.
func buildEntitlementPayloadWithUsage(
	status *pkglicensing.LicenseStatus,
	subscriptionState string,
	usage entitlementUsageSnapshot,
	trialEndsAtUnix *int64,
) EntitlementPayload {
	return pkglicensing.BuildEntitlementPayloadWithUsage(status, subscriptionState, usage, trialEndsAtUnix)
}

// limitState returns the over-limit UX state string.
// Exported for testing.
func limitState(current, limit int64) string {
	return pkglicensing.LimitState(current, limit)
}
