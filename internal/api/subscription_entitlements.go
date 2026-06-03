package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// EntitlementPayload is the normalized entitlement response for frontend consumption.
// Frontend should use this instead of inferring capabilities from tier names.
type EntitlementPayload = entitlementPayloadModel

// RuntimeCapabilitiesPayload is the canonical non-commercial runtime capability
// response for feature gating and operational limit checks.
type RuntimeCapabilitiesPayload = runtimeCapabilitiesPayloadModel

// CommercialPosturePayload is the canonical non-billing commercial response
// for upgrade posture and monitored-system migration guidance.
type CommercialPosturePayload = commercialPosturePayloadModel

// LimitStatus represents a quantitative limit with current usage state.
type LimitStatus = limitStatusModel

// UpgradeReason provides context for why a user should upgrade.
type UpgradeReason = upgradeReasonModel

func (h *LicenseHandlers) buildCommercialEntitlementPayload(
	ctx context.Context,
) (EntitlementPayload, error) {
	svc, _, err := h.getTenantComponents(ctx)
	if err != nil {
		return EntitlementPayload{}, err
	}

	status := svc.Status()
	usage := h.entitlementUsageSnapshot(ctx)
	trialEndsAtUnix := trialEndsAtUnixFromService(svc)

	overflowGrantedAt := h.ensureOnboardingOverflow(ctx, status.Tier)
	now := time.Now()

	payload := buildEntitlementPayloadWithUsage(
		status,
		h.payloadSubscriptionStateForService(svc),
		usage,
		trialEndsAtUnix,
	)

	// Surface overflow days remaining for frontend messaging.
	if days := overflowDaysRemainingFromLicensing(status.Tier, overflowGrantedAt, now); days > 0 {
		payload.OverflowDaysRemaining = &days
	}

	if eval := svc.Evaluator(); eval != nil {
		if pv := strings.TrimSpace(eval.PlanVersion()); pv != "" {
			payload.PlanVersion = pv
		}
	}
	existing := h.billingStateForContext(ctx)
	if existing != nil {
		payload.CommercialMigration = cloneCommercialMigrationStatusFromLicensing(existing.CommercialMigration)
	}
	payload.HostedMode = h != nil && h.hostedMode
	payload.Runtime = cloneRuntimeIdentityFromLicensing(h.currentRuntimeIdentity())
	return payload, nil
}

// HandleEntitlements returns the normalized entitlement payload for the current tenant.
// This is the commercial entitlement endpoint for billing, activation, and upgrade presentation.
func (h *LicenseHandlers) HandleEntitlements(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", nil)
		return
	}

	payload, err := h.buildCommercialEntitlementPayload(r.Context())
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "internal_error", "Internal server error", nil)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(payload)
}

// HandleCommercialPosture returns the canonical non-billing commercial posture payload.
func (h *LicenseHandlers) HandleCommercialPosture(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", nil)
		return
	}

	payload, err := h.buildCommercialEntitlementPayload(r.Context())
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "internal_error", "Internal server error", nil)
		return
	}

	posture := commercialPosturePayloadFromEntitlementPayload(payload)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(posture)
}

// HandleRuntimeCapabilities returns the canonical non-commercial runtime capability payload.
func (h *LicenseHandlers) HandleRuntimeCapabilities(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", nil)
		return
	}

	svc, _, err := h.getTenantComponents(r.Context())
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "internal_error", "Internal server error", nil)
		return
	}

	status := svc.Status()
	usage := h.entitlementUsageSnapshot(r.Context())

	payload := buildRuntimeCapabilitiesPayloadWithUsage(
		status,
		h.payloadSubscriptionStateForService(svc),
		usage,
	)
	payload.HostedMode = h != nil && h.hostedMode
	runtimeIdentity := h.currentRuntimeIdentity()
	payload.Runtime = cloneRuntimeIdentityFromLicensing(runtimeIdentity)
	payload.Capabilities, payload.BlockedCapabilities = filterCapabilitiesForRuntimeIdentityFromLicensing(
		payload.Capabilities,
		runtimeIdentity,
	)
	if h != nil && h.cfg != nil && h.cfg.DemoMode {
		payload = sanitizeRuntimeCapabilitiesPayloadForPublicDemo(payload)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(payload)
}

func (h *LicenseHandlers) payloadSubscriptionStateForService(svc *licenseService) string {
	if svc == nil {
		return ""
	}
	if svc.Current() == nil && svc.Evaluator() == nil && h != nil && !h.hostedMode {
		// Self-hosted community installs have no commercial lifecycle source at
		// all. Report the runtime posture as active community instead of an
		// expired paid subscription.
		return string(subscriptionStateActiveValue)
	}
	return svc.SubscriptionState()
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

// entitlementUsageSnapshot returns best-effort runtime usage counts for
// entitlement payloads and billing/support context.
func (h *LicenseHandlers) entitlementUsageSnapshot(ctx context.Context) entitlementUsageSnapshot {
	usage := entitlementUsageSnapshot{}
	if h == nil {
		return usage
	}

	orgID := GetOrgID(ctx)
	if orgID == "" {
		orgID = "default"
	}

	// Count canonical top-level monitored systems from monitor state.
	var monitorResolved bool
	if h.mtMonitor != nil {
		if monitor, err := h.mtMonitor.GetMonitor(orgID); err == nil && monitor != nil {
			state := monitor.MonitoredSystemUsage()
			if state.Available {
				usage.MonitoredSystems = int64(state.Count)
				usage.MonitoredSystemsAvailable = true
				usage.LegacyConnections = legacyConnectionCountsFromReadState(state.ReadState)
				monitorResolved = true
			} else if usage.MonitoredSystemsUnavailableReason == "" {
				usage.MonitoredSystemsUnavailableReason = state.UnavailableReason
			}
		}
	}
	if !monitorResolved && orgID == "default" && h.monitor != nil {
		state := h.monitor.MonitoredSystemUsage()
		if state.Available {
			usage.MonitoredSystems = int64(state.Count)
			usage.MonitoredSystemsAvailable = true
			usage.LegacyConnections = legacyConnectionCountsFromReadState(state.ReadState)
		} else if usage.MonitoredSystemsUnavailableReason == "" {
			usage.MonitoredSystemsUnavailableReason = state.UnavailableReason
		}
	}

	// Guest metadata for guest limit tracking.
	if h.mtPersistence != nil {
		if persistence, err := h.mtPersistence.GetPersistence(orgID); err == nil && persistence != nil {
			if guestStore := persistence.GetGuestMetadataStore(); guestStore != nil {
				usage.Guests = int64(len(guestStore.GetAll()))
			}
		}
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

func buildRuntimeCapabilitiesPayloadWithUsage(
	status *licenseStatus,
	subscriptionState string,
	usage entitlementUsageSnapshot,
) RuntimeCapabilitiesPayload {
	return buildRuntimeCapabilitiesPayloadWithUsageFromLicensing(status, subscriptionState, usage)
}

func buildCommercialPosturePayloadWithUsage(
	status *licenseStatus,
	subscriptionState string,
	usage entitlementUsageSnapshot,
	trialEndsAtUnix *int64,
) CommercialPosturePayload {
	return buildCommercialPosturePayloadWithUsageFromLicensing(
		status,
		subscriptionState,
		usage,
		trialEndsAtUnix,
	)
}

func commercialPosturePayloadFromEntitlementPayload(
	payload EntitlementPayload,
) CommercialPosturePayload {
	return commercialPosturePayloadFromEntitlementPayloadFromLicensing(payload)
}

// limitState returns the over-limit UX state string.
// Exported for testing.
func limitState(current, limit int64) string {
	return limitStateFromLicensing(current, limit)
}

// ensureOnboardingOverflow lazy-initializes the overflow grant for free-tier workspaces.
// Returns the OverflowGrantedAt timestamp (may be nil for non-free tiers or missing billing store).
func (h *LicenseHandlers) ensureOnboardingOverflow(ctx context.Context, tier licenseTier) *int64 {
	if tier != licenseTierFreeValue || h == nil || h.mtPersistence == nil {
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
		state := &billingState{
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

func (h *LicenseHandlers) billingStateForContext(ctx context.Context) *billingState {
	if h == nil || h.mtPersistence == nil {
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
	return existing
}
