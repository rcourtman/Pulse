// Package license handles Pulse Pro license validation and feature gating.
package license

import "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"

// Feature constants represent gated features in Pulse Pro.
// These are embedded in license JWTs and checked at runtime.
const (
	FeatureAIPatrol          = licensing.FeatureAIPatrol
	FeatureAIAlerts          = licensing.FeatureAIAlerts
	FeatureAIAutoFix         = licensing.FeatureAIAutoFix
	FeatureKubernetesAI      = licensing.FeatureKubernetesAI
	FeatureAgentProfiles     = licensing.FeatureAgentProfiles
	FeatureUpdateAlerts      = licensing.FeatureUpdateAlerts
	FeatureRBAC              = licensing.FeatureRBAC
	FeatureAuditLogging      = licensing.FeatureAuditLogging
	FeatureSSO               = licensing.FeatureSSO
	FeatureAdvancedSSO       = licensing.FeatureAdvancedSSO
	FeatureAdvancedReporting = licensing.FeatureAdvancedReporting
	FeatureLongTermMetrics   = licensing.FeatureLongTermMetrics
	FeatureRelay             = licensing.FeatureRelay
	FeatureMultiUser         = licensing.FeatureMultiUser
	FeatureWhiteLabel        = licensing.FeatureWhiteLabel
	FeatureMultiTenant       = licensing.FeatureMultiTenant
	FeatureUnlimited         = licensing.FeatureUnlimited
)

// Tier represents a license tier.
type Tier = licensing.Tier

const (
	TierFree       = licensing.TierFree
	TierPro        = licensing.TierPro
	TierProAnnual  = licensing.TierProAnnual
	TierLifetime   = licensing.TierLifetime
	TierCloud      = licensing.TierCloud
	TierMSP        = licensing.TierMSP
	TierEnterprise = licensing.TierEnterprise
)

// TierFeatures maps each tier to its included features.
var TierFeatures = licensing.TierFeatures

// DeriveCapabilitiesFromTier derives effective capabilities from tier and explicit features.
func DeriveCapabilitiesFromTier(tier Tier, explicitFeatures []string) []string {
	return licensing.DeriveCapabilitiesFromTier(tier, explicitFeatures)
}

// DeriveEntitlements derives capabilities and limits from tier and legacy claim fields.
func DeriveEntitlements(tier Tier, features []string, maxNodes int, maxGuests int) (capabilities []string, limits map[string]int64) {
	return licensing.DeriveEntitlements(tier, features, maxNodes, maxGuests)
}

// TierHasFeature checks if a tier includes a specific feature.
func TierHasFeature(tier Tier, feature string) bool {
	return licensing.TierHasFeature(tier, feature)
}

// GetTierDisplayName returns a human-readable name for the tier.
func GetTierDisplayName(tier Tier) string {
	return licensing.GetTierDisplayName(tier)
}

// GetFeatureDisplayName returns a human-readable name for a feature.
func GetFeatureDisplayName(feature string) string {
	return licensing.GetFeatureDisplayName(feature)
}
