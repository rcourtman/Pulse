// Package licensing defines shared Pulse feature and tier contracts.
//
// This package exists so private extension modules can depend on canonical
// licensing metadata without importing internal packages.
package licensing

import (
	"sort"
	"strings"
	"time"
)

// Feature constants represent gated features in Pulse.
// These are embedded in license JWTs and checked at runtime.
const (
	// Free tier features
	FeatureUpdateAlerts = "update_alerts" // Alerts for pending container/package updates
	FeatureSSO          = "sso"           // Core OIDC/SAML SSO authentication
	FeatureAdvancedSSO  = "advanced_sso"  // Compatibility key for SAML and multi-provider SSO; included in Community
	FeatureAIPatrol     = "ai_patrol"     // Background AI health monitoring (BYOK, free with own key)

	// Relay tier features (everything in Free, plus:)
	FeatureRelay             = "relay"              // Relay remote access
	FeatureMobileApp         = "mobile_app"         // Mobile app access
	FeaturePushNotifications = "push_notifications" // Push notifications
	FeatureLongTermMetrics   = "long_term_metrics"  // Extended historical metrics (14d Relay, 90d Pro)

	// Pro tier features (everything in Relay, plus:)
	FeatureAIAlerts          = "ai_alerts"          // AI analysis when alerts fire
	FeatureAIAutoFix         = "ai_autofix"         // Governed Patrol fixes
	FeatureKubernetesAI      = "kubernetes_ai"      // Legacy Kubernetes analysis compatibility gate (NOT basic monitoring)
	FeatureAgentProfiles     = "agent_profiles"     // Centralized agent configuration profiles
	FeatureRBAC              = "rbac"               // Role-Based Access Control
	FeatureAuditLogging      = "audit_logging"      // Persistent audit logs with signing
	FeatureAdvancedReporting = "advanced_reporting" // PDF/CSV reporting engine

	// MSP/Enterprise tier features
	FeatureMultiUser   = "multi_user"   // Multi-user (likely merged with RBAC)
	FeatureWhiteLabel  = "white_label"  // Custom report branding
	FeatureMultiTenant = "multi_tenant" // Multi-tenant organizations
	FeatureUnlimited   = "unlimited"    // Compatibility capability marker for MSP/enterprise deals

	// Internal-only runtime capabilities. These must never be added to public
	// tier defaults or public pricing contracts.
	FeatureDemoFixtures = "demo_fixtures" // Allows release builds in DEMO_MODE to render mock fixture data
)

// Tier represents a license tier.
type Tier string

const (
	TierFree       Tier = "free"
	TierRelay      Tier = "relay"
	TierPro        Tier = "pro"
	TierProPlus    Tier = "pro_plus"
	TierProAnnual  Tier = "pro_annual" // Legacy: same features as TierPro
	TierLifetime   Tier = "lifetime"   // Legacy: same features as TierPro
	TierCloud      Tier = "cloud"
	TierMSP        Tier = "msp"
	TierEnterprise Tier = "enterprise"
)

// PriceIDToPlanVersion maps Stripe price IDs to canonical plan version strings.
// This is the authoritative reverse lookup: given a price ID from a checkout
// session, subscription, or webhook event, callers can resolve the plan version
// without relying on metadata being set. The map covers all known renewing
// recurring prices that Pulse must interpret canonically, including:
//   - v6 Cloud and MSP recurring prices
//   - grandfathered v5 recurring renewals
//   - still-renewing legacy v1 recurring renewals
var PriceIDToPlanVersion = map[string]string{
	// Cloud Starter
	"price_1T5kflBrHBocJIGHUqPv1dzV": "cloud_starter",  // $29/mo
	"price_1T5kfmBrHBocJIGHTS3ymKxM": "cloud_starter",  // $249/yr
	"price_1T5kfnBrHBocJIGHATQJr79D": "cloud_founding", // $19/mo founding

	// Cloud Power
	"price_1T5kg2BrHBocJIGHmkoF0zXY": "cloud_power", // $49/mo
	"price_1T5kg3BrHBocJIGH2EtzKofV": "cloud_power", // $449/yr

	// Cloud Max
	"price_1T5kg4BrHBocJIGHHa8Ecqho": "cloud_max", // $79/mo
	"price_1T5kg5BrHBocJIGH5AIJ4nVc": "cloud_max", // $699/yr

	// MSP Starter
	"price_1T5kgTBrHBocJIGHjOs15LI2": "msp_starter", // $149/mo
	"price_1T5kgUBrHBocJIGHT6PiOn6x": "msp_starter", // $1,490/yr

	// MSP Growth
	"price_1T5kgVBrHBocJIGHulNsCTb1": "msp_growth", // $249/mo
	"price_1T5kgWBrHBocJIGHTuaNjnJ2": "msp_growth", // $2,490/yr

	// MSP Scale
	"price_1T5kgWBrHBocJIGHo40iFeRd": "msp_scale", // $399/mo
	"price_1T5kgXBrHBocJIGHWlOgTyGV": "msp_scale", // $3,990/yr

	// Grandfathered Pulse Pro recurring renewals (v5)
	"price_1ShIsdBrHBocJIGH71yQusLG": "v5_pro_monthly_grandfathered", // $9/mo
	"price_1ShIsnBrHBocJIGHBKkzsZ3T": "v5_pro_annual_grandfathered",  // $79/yr

	// Grandfathered Pulse Pro recurring renewals (legacy v1)
	"price_1SgDxvBrHBocJIGHStaGuiAX": "v5_pro_monthly_grandfathered", // $19/mo
	"price_1SgDxwBrHBocJIGHTKTsIMLc": "v5_pro_annual_grandfathered",  // $190/yr
}

// PlanVersionForPriceID returns the canonical plan version for a Stripe price
// ID. Returns ("", false) if the price ID is not recognized.
func PlanVersionForPriceID(priceID string) (string, bool) {
	v, ok := PriceIDToPlanVersion[priceID]
	return v, ok
}

func IsGrandfatheredRecurringV5PlanVersion(planVersion string) bool {
	switch CanonicalizePlanVersion(planVersion) {
	case "v5_pro_monthly_grandfathered", "v5_pro_annual_grandfathered":
		return true
	default:
		return false
	}
}

// IsSelfHostedCommunityPlanVersion reports whether a persisted billing-state
// plan version denotes the uncapped self-hosted Community/free posture.
func IsSelfHostedCommunityPlanVersion(planVersion string) bool {
	switch strings.ToLower(CanonicalizePlanVersion(planVersion)) {
	case "community", string(TierFree):
		return true
	default:
		return false
	}
}

// IsSelfHostedCoreMonitoringUncappedTier reports whether a tier is part of the
// self-hosted v6 contract where core monitoring volume is not monetized.
func IsSelfHostedCoreMonitoringUncappedTier(tier Tier) bool {
	switch Tier(strings.ToLower(strings.TrimSpace(string(tier)))) {
	case TierFree, TierRelay, TierPro, TierProPlus, TierProAnnual, TierLifetime:
		return true
	default:
		return false
	}
}

func IsSelfHostedCoreMonitoringUncappedPlanVersion(planVersion string) bool {
	normalized := strings.ToLower(CanonicalizePlanVersion(planVersion))
	switch normalized {
	case "community",
		string(TierFree),
		string(TierRelay),
		string(TierPro),
		string(TierProPlus),
		string(TierProAnnual),
		string(TierLifetime),
		"v5_lifetime_grandfathered":
		return true
	default:
		return IsGrandfatheredRecurringV5PlanVersion(normalized)
	}
}

func stripLegacyCommercialCaps(limits map[string]int64) {
	delete(limits, MaxMonitoredSystemsLicenseGateKey)
	delete(limits, "max_guests")
}

func stripSelfHostedCommercialVolumeCaps(limits map[string]int64, planVersion string, tier Tier, uncapped bool) {
	if limits == nil {
		return
	}
	if uncapped ||
		IsSelfHostedCoreMonitoringUncappedPlanVersion(planVersion) ||
		(planVersion == "" && IsSelfHostedCoreMonitoringUncappedTier(tier)) {
		stripLegacyCommercialCaps(limits)
	}
}

// CloudPlanWorkspaceLimits maps cloud plan version strings to the maximum
// number of active workspaces (tenants) the account may create. Individual
// Cloud accounts get exactly 1 workspace; MSP tiers get the client caps from
// the pricing spec.
var CloudPlanWorkspaceLimits = map[string]int{
	// Individual Cloud tiers — one workspace per account
	"cloud_starter":  1,
	"cloud_power":    1,
	"cloud_max":      1,
	"cloud_founding": 1,

	// MSP tiers — client caps from pricing spec
	"msp_starter": 5,  // MSP Starter: up to 5 clients
	"msp_growth":  15, // MSP Growth: up to 15 clients
	"msp_scale":   40, // MSP Scale: up to 40 clients
}

// UnknownPlanDefaultWorkspaceLimit is the safe-default workspace limit applied
// when a plan version is not recognized. Fail-closed: unknown plans get the
// smallest MSP tier limit.
const UnknownPlanDefaultWorkspaceLimit = 1

// WorkspaceLimitForPlan returns the maximum active workspace count for a given
// cloud plan version and whether the plan was recognized. If unrecognized,
// returns a safe default (1) and known=false.
func WorkspaceLimitForPlan(planVersion string) (limit int, known bool) {
	planVersion = CanonicalizePlanVersion(planVersion)
	if l, ok := CloudPlanWorkspaceLimits[planVersion]; ok {
		return l, true
	}
	return UnknownPlanDefaultWorkspaceLimit, false
}

// TierHistoryDays defines the maximum metrics history retention per tier.
var TierHistoryDays = map[Tier]int{
	TierFree:       7,
	TierRelay:      14,
	TierPro:        90,
	TierProPlus:    90,
	TierProAnnual:  90,
	TierLifetime:   90,
	TierCloud:      90,
	TierMSP:        90,
	TierEnterprise: 90,
}

// freeFeatures are the base capabilities available to all users.
var freeFeatures = []string{
	FeatureUpdateAlerts,
	FeatureSSO,
	FeatureAdvancedSSO,
	FeatureAIPatrol, // Patrol is free with BYOK; governed fixes require Pro.
}

// relayFeatures adds remote access and mobile on top of free.
var relayFeatures = appendFeatures(freeFeatures,
	FeatureRelay,
	FeatureMobileApp,
	FeaturePushNotifications,
	FeatureLongTermMetrics, // 14 days (vs 7 for free)
)

// proFeatures adds AI automation, fleet management, and compliance on top of relay.
var proFeatures = appendFeatures(relayFeatures,
	FeatureAIAlerts,
	FeatureAIAutoFix,
	FeatureKubernetesAI,
	FeatureAgentProfiles,
	FeatureRBAC,
	FeatureAuditLogging,
	FeatureAdvancedReporting,
)

// mspFeatures adds multi-tenant policy on top of pro.
var mspFeatures = appendFeatures(proFeatures,
	FeatureUnlimited,
	FeatureMultiTenant,
)

// enterpriseFeatures adds white-label and multi-user on top of MSP.
var enterpriseFeatures = appendFeatures(mspFeatures,
	FeatureMultiUser,
	FeatureWhiteLabel,
)

// appendFeatures returns a new slice with extra features appended (no mutation).
func appendFeatures(base []string, extra ...string) []string {
	result := make([]string, len(base), len(base)+len(extra))
	copy(result, base)
	return append(result, extra...)
}

// TierFeatures maps each tier to its included features.
var TierFeatures = map[Tier][]string{
	TierFree:       freeFeatures,
	TierRelay:      relayFeatures,
	TierPro:        proFeatures,
	TierProPlus:    proFeatures, // Legacy compatibility tier; same runtime features as Pro
	TierProAnnual:  proFeatures, // Legacy: same features as Pro
	TierLifetime:   proFeatures, // Legacy: same features as Pro
	TierCloud:      proFeatures, // Cloud includes all Pro features + managed hosting
	TierMSP:        mspFeatures,
	TierEnterprise: enterpriseFeatures,
}

// DeriveCapabilitiesFromTier derives effective capabilities from tier and explicit features.
func DeriveCapabilitiesFromTier(tier Tier, explicitFeatures []string) []string {
	featureSet := make(map[string]struct{})
	for _, feature := range TierFeatures[tier] {
		featureSet[feature] = struct{}{}
	}
	for _, feature := range explicitFeatures {
		featureSet[feature] = struct{}{}
	}

	capabilities := make([]string, 0, len(featureSet))
	for feature := range featureSet {
		capabilities = append(capabilities, feature)
	}
	sort.Strings(capabilities)
	return capabilities
}

// DeriveEntitlements derives capabilities and non-monitoring quantitative limits.
// The monitored-system parameter is retained for old callers but ignored:
// monitored-system volume is no longer a license entitlement.
func DeriveEntitlements(tier Tier, features []string, _ int, maxGuests int) (capabilities []string, limits map[string]int64) {
	capabilities = DeriveCapabilitiesFromTier(tier, features)

	limits = make(map[string]int64)
	if maxGuests > 0 {
		limits["max_guests"] = int64(maxGuests)
	}

	return capabilities, limits
}

// OnboardingOverflowDuration is the window during which free-tier workspaces
// receive +1 host slot after initial setup.
const OnboardingOverflowDuration = 14 * 24 * time.Hour

// OverflowBonus returns the number of bonus host slots granted by the
// onboarding overflow. Returns 1 if the tier is free, overflowGrantedAt
// is set, and the current time is within 14 days of the grant. Otherwise 0.
func OverflowBonus(tier Tier, overflowGrantedAt *int64, now time.Time) int {
	if tier != TierFree || overflowGrantedAt == nil {
		return 0
	}
	grantedAt := time.Unix(*overflowGrantedAt, 0)
	elapsed := now.Sub(grantedAt)
	if elapsed < 0 {
		// Future timestamp — treat as not yet granted.
		return 0
	}
	if elapsed < OnboardingOverflowDuration {
		return 1
	}
	return 0
}

// OverflowDaysRemaining returns the number of days remaining in the overflow
// window. Returns 0 if overflow is not active.
func OverflowDaysRemaining(tier Tier, overflowGrantedAt *int64, now time.Time) int {
	if OverflowBonus(tier, overflowGrantedAt, now) == 0 {
		return 0
	}
	grantedAt := time.Unix(*overflowGrantedAt, 0)
	expiresAt := grantedAt.Add(OnboardingOverflowDuration)
	remaining := expiresAt.Sub(now)
	days := int(remaining.Hours()/24) + 1 // ceiling: partial day counts as 1
	if days < 0 {
		return 0
	}
	return days
}

// TierHasFeature checks if a tier includes a specific feature.
func TierHasFeature(tier Tier, feature string) bool {
	features, ok := TierFeatures[tier]
	if !ok {
		return false
	}
	for _, f := range features {
		if f == feature {
			return true
		}
	}
	return false
}

// CapabilityVisibleInPublicPayload reports whether a capability key belongs in
// browser-facing entitlement and runtime payload contracts.
func CapabilityVisibleInPublicPayload(feature string) bool {
	switch feature {
	case FeatureDemoFixtures:
		return false
	default:
		return true
	}
}

// FilterPublicCapabilities strips internal-only capability keys from public API
// payload contracts while preserving caller order for visible features.
func FilterPublicCapabilities(features []string) []string {
	if len(features) == 0 {
		return []string{}
	}

	filtered := make([]string, 0, len(features))
	for _, feature := range features {
		if CapabilityVisibleInPublicPayload(feature) {
			filtered = append(filtered, feature)
		}
	}
	return filtered
}

// GetTierDisplayName returns a human-readable name for the tier.
func GetTierDisplayName(tier Tier) string {
	switch tier {
	case TierFree:
		return "Community"
	case TierRelay:
		return "Relay"
	case TierPro:
		return "Pro"
	case TierProPlus:
		return "Pro+"
	case TierProAnnual:
		return "Pro (Annual)"
	case TierLifetime:
		return "Pro (Lifetime)"
	case TierCloud:
		return "Cloud"
	case TierMSP:
		return "MSP"
	case TierEnterprise:
		return "Enterprise"
	default:
		return "Unknown"
	}
}

// GetFeatureMinTierName returns the display name of the lowest tier that includes the given feature.
// This is used for user-facing messages like "requires Pulse Relay or above".
// The tier ordering is: Free < Relay < Pro < MSP < Enterprise.
func GetFeatureMinTierName(feature string) string {
	orderedTiers := []Tier{TierFree, TierRelay, TierPro, TierMSP, TierEnterprise}
	for _, tier := range orderedTiers {
		if TierHasFeature(tier, feature) {
			return GetTierDisplayName(tier)
		}
	}
	return "Pro" // fallback
}

// GetFeatureDisplayName returns a human-readable name for a feature.
func GetFeatureDisplayName(feature string) string {
	if entry, ok := GetFeatureMetadata(feature); ok {
		return entry.DisplayName
	}
	return feature
}
