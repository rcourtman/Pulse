package licensing

import "sort"

type SelfHostedFeatureRole string

const (
	SelfHostedFeatureRoleHidden            SelfHostedFeatureRole = "hidden"
	SelfHostedFeatureRoleIncluded          SelfHostedFeatureRole = "included"
	SelfHostedFeatureRolePrimaryPillar     SelfHostedFeatureRole = "primary_pillar"
	SelfHostedFeatureRoleIncludedExtra     SelfHostedFeatureRole = "included_extra"
	SelfHostedFeatureRoleCompatibilityOnly SelfHostedFeatureRole = "compatibility_only"
)

type SelfHostedFeatureRoles struct {
	Community SelfHostedFeatureRole
	Relay     SelfHostedFeatureRole
	Pro       SelfHostedFeatureRole
}

type FeatureMetadata struct {
	Key                   string
	DisplayName           string
	ComparisonName        string
	ShowInComparisonTable bool
	DisplayableInPlanUI   bool
	SelfHostedRoles       SelfHostedFeatureRoles
	UpgradeReason         string
	UpgradePriority       int
}

var orderedFeatureMetadataKeys = []string{
	FeatureUpdateAlerts,
	FeatureSSO,
	FeatureAIPatrol,
	FeatureRelay,
	FeatureMobileApp,
	FeaturePushNotifications,
	FeatureAIAlerts,
	FeatureAIAutoFix,
	FeatureLongTermMetrics,
	FeatureAdvancedSSO,
	FeatureRBAC,
	FeatureAuditLogging,
	FeatureAdvancedReporting,
	FeatureAgentProfiles,
	FeatureKubernetesAI,
	FeatureMultiUser,
	FeatureWhiteLabel,
	FeatureMultiTenant,
	FeatureUnlimited,
	FeatureDemoFixtures,
}

var featureMetadataCatalog = map[string]FeatureMetadata{
	FeatureUpdateAlerts: {
		Key:                   FeatureUpdateAlerts,
		DisplayName:           "Update Alerts (Container/Package Updates)",
		ComparisonName:        "Update Alerts",
		ShowInComparisonTable: true,
		DisplayableInPlanUI:   true,
		SelfHostedRoles: SelfHostedFeatureRoles{
			Community: SelfHostedFeatureRoleIncluded,
			Relay:     SelfHostedFeatureRoleIncluded,
			Pro:       SelfHostedFeatureRoleIncluded,
		},
	},
	FeatureSSO: {
		Key:                   FeatureSSO,
		DisplayName:           "Basic SSO (OIDC)",
		ComparisonName:        "Basic SSO (OIDC)",
		ShowInComparisonTable: true,
		DisplayableInPlanUI:   true,
		SelfHostedRoles: SelfHostedFeatureRoles{
			Community: SelfHostedFeatureRoleIncluded,
			Relay:     SelfHostedFeatureRoleIncluded,
			Pro:       SelfHostedFeatureRoleIncluded,
		},
	},
	FeatureAIPatrol: {
		Key:                   FeatureAIPatrol,
		DisplayName:           "Pulse Patrol (Background Health Checks)",
		ComparisonName:        "Pulse Patrol",
		ShowInComparisonTable: true,
		DisplayableInPlanUI:   true,
		SelfHostedRoles: SelfHostedFeatureRoles{
			Community: SelfHostedFeatureRoleIncluded,
			Relay:     SelfHostedFeatureRoleIncluded,
			Pro:       SelfHostedFeatureRoleIncluded,
		},
	},
	FeatureRelay: {
		Key:                   FeatureRelay,
		DisplayName:           "Pulse Relay (Remote Access)",
		ComparisonName:        "Pulse Relay (Remote Access)",
		ShowInComparisonTable: true,
		DisplayableInPlanUI:   true,
		SelfHostedRoles: SelfHostedFeatureRoles{
			Community: SelfHostedFeatureRoleHidden,
			Relay:     SelfHostedFeatureRolePrimaryPillar,
			Pro:       SelfHostedFeatureRoleIncluded,
		},
		UpgradeReason:   "Get Relay so Pulse stays reachable securely from anywhere instead of only on the local dashboard.",
		UpgradePriority: 1,
	},
	FeatureMobileApp: {
		Key:                   FeatureMobileApp,
		DisplayName:           "Mobile App Access",
		ComparisonName:        "Mobile App Access",
		ShowInComparisonTable: true,
		DisplayableInPlanUI:   true,
		SelfHostedRoles: SelfHostedFeatureRoles{
			Community: SelfHostedFeatureRoleHidden,
			Relay:     SelfHostedFeatureRolePrimaryPillar,
			Pro:       SelfHostedFeatureRoleIncluded,
		},
		UpgradeReason:   "Get Relay so you can check Pulse from your phone when you are away from the dashboard.",
		UpgradePriority: 1,
	},
	FeaturePushNotifications: {
		Key:                   FeaturePushNotifications,
		DisplayName:           "Push Notifications",
		ComparisonName:        "Push Notifications",
		ShowInComparisonTable: true,
		DisplayableInPlanUI:   true,
		SelfHostedRoles: SelfHostedFeatureRoles{
			Community: SelfHostedFeatureRoleHidden,
			Relay:     SelfHostedFeatureRolePrimaryPillar,
			Pro:       SelfHostedFeatureRoleIncluded,
		},
		UpgradeReason:   "Get Relay so important alerts reach you immediately on mobile instead of waiting for you to reopen Pulse.",
		UpgradePriority: 1,
	},
	FeatureLongTermMetrics: {
		Key:                   FeatureLongTermMetrics,
		DisplayName:           "Extended Metric History",
		ComparisonName:        "Extended Metric History",
		ShowInComparisonTable: false,
		DisplayableInPlanUI:   true,
		SelfHostedRoles: SelfHostedFeatureRoles{
			Community: SelfHostedFeatureRoleHidden,
			Relay:     SelfHostedFeatureRolePrimaryPillar,
			Pro:       SelfHostedFeatureRolePrimaryPillar,
		},
		UpgradeReason:   "Get Relay for 14 days of history, or Pro for 90 days, so you can see what changed before and after an incident.",
		UpgradePriority: 2,
	},
	FeatureAIAlerts: {
		Key:                   FeatureAIAlerts,
		DisplayName:           "Alert Analysis",
		ComparisonName:        "Pulse Alert Analysis",
		ShowInComparisonTable: true,
		DisplayableInPlanUI:   true,
		SelfHostedRoles: SelfHostedFeatureRoles{
			Community: SelfHostedFeatureRoleHidden,
			Relay:     SelfHostedFeatureRoleHidden,
			Pro:       SelfHostedFeatureRolePrimaryPillar,
		},
		UpgradeReason:   "Upgrade to Pro so alerts arrive with root-cause analysis instead of a stack of symptoms.",
		UpgradePriority: 4,
	},
	FeatureAIAutoFix: {
		Key:                   FeatureAIAutoFix,
		DisplayName:           "Pulse Patrol Auto-Fix",
		ComparisonName:        "Patrol Auto-Fix",
		ShowInComparisonTable: true,
		DisplayableInPlanUI:   true,
		SelfHostedRoles: SelfHostedFeatureRoles{
			Community: SelfHostedFeatureRoleHidden,
			Relay:     SelfHostedFeatureRoleHidden,
			Pro:       SelfHostedFeatureRolePrimaryPillar,
		},
		UpgradeReason:   "Upgrade to Pro so Pulse can move from finding issues to applying safe remediation with your approval or in autonomous mode.",
		UpgradePriority: 3,
	},
	FeatureKubernetesAI: {
		Key:                   FeatureKubernetesAI,
		DisplayName:           "Kubernetes AI Analysis (Compatibility)",
		ComparisonName:        "Kubernetes AI Analysis (Compatibility)",
		ShowInComparisonTable: false,
		DisplayableInPlanUI:   false,
		SelfHostedRoles: SelfHostedFeatureRoles{
			Community: SelfHostedFeatureRoleHidden,
			Relay:     SelfHostedFeatureRoleHidden,
			Pro:       SelfHostedFeatureRoleCompatibilityOnly,
		},
	},
	FeatureAgentProfiles: {
		Key:                   FeatureAgentProfiles,
		DisplayName:           "Centralized Agent Profiles",
		ComparisonName:        "Centralized Agent Profiles",
		ShowInComparisonTable: true,
		DisplayableInPlanUI:   true,
		SelfHostedRoles: SelfHostedFeatureRoles{
			Community: SelfHostedFeatureRoleHidden,
			Relay:     SelfHostedFeatureRoleHidden,
			Pro:       SelfHostedFeatureRoleIncludedExtra,
		},
		UpgradeReason:   "Upgrade to Pro to standardize agent behavior across systems without reconfiguring every install by hand.",
		UpgradePriority: 6,
	},
	FeatureRBAC: {
		Key:                   FeatureRBAC,
		DisplayName:           "Role-Based Access Control (RBAC)",
		ComparisonName:        "Role-Based Access Control (RBAC)",
		ShowInComparisonTable: true,
		DisplayableInPlanUI:   true,
		SelfHostedRoles: SelfHostedFeatureRoles{
			Community: SelfHostedFeatureRoleHidden,
			Relay:     SelfHostedFeatureRoleHidden,
			Pro:       SelfHostedFeatureRoleIncludedExtra,
		},
		UpgradeReason:   "Upgrade to Pro when more than one operator needs safe access boundaries around infrastructure changes.",
		UpgradePriority: 5,
	},
	FeatureAuditLogging: {
		Key:                   FeatureAuditLogging,
		DisplayName:           "Audit Logging",
		ComparisonName:        "Audit Logging",
		ShowInComparisonTable: true,
		DisplayableInPlanUI:   true,
		SelfHostedRoles: SelfHostedFeatureRoles{
			Community: SelfHostedFeatureRoleHidden,
			Relay:     SelfHostedFeatureRoleHidden,
			Pro:       SelfHostedFeatureRoleIncludedExtra,
		},
		UpgradeReason:   "Upgrade to Pro to keep a trustworthy action trail for incident review, accountability, and compliance.",
		UpgradePriority: 8,
	},
	FeatureAdvancedSSO: {
		Key:                   FeatureAdvancedSSO,
		DisplayName:           "Advanced SSO (SAML/Multi-Provider)",
		ComparisonName:        "Advanced SSO (SAML/Multi-Provider)",
		ShowInComparisonTable: true,
		DisplayableInPlanUI:   true,
		SelfHostedRoles: SelfHostedFeatureRoles{
			Community: SelfHostedFeatureRoleHidden,
			Relay:     SelfHostedFeatureRoleHidden,
			Pro:       SelfHostedFeatureRoleIncludedExtra,
		},
		UpgradeReason:   "Upgrade to Pro to connect your identity provider and keep operator access aligned with your existing auth controls.",
		UpgradePriority: 7,
	},
	FeatureAdvancedReporting: {
		Key:                   FeatureAdvancedReporting,
		DisplayName:           "PDF/CSV Reporting",
		ComparisonName:        "PDF/CSV Reporting",
		ShowInComparisonTable: true,
		DisplayableInPlanUI:   true,
		SelfHostedRoles: SelfHostedFeatureRoles{
			Community: SelfHostedFeatureRoleHidden,
			Relay:     SelfHostedFeatureRoleHidden,
			Pro:       SelfHostedFeatureRoleIncludedExtra,
		},
		UpgradeReason:   "Upgrade to Pro to turn live infrastructure state into shareable reports without manual screenshot work.",
		UpgradePriority: 9,
	},
	FeatureMultiUser: {
		Key:                 FeatureMultiUser,
		DisplayName:         "Multi-User Mode",
		ComparisonName:      "Multi-User Mode",
		DisplayableInPlanUI: false,
		SelfHostedRoles: SelfHostedFeatureRoles{
			Community: SelfHostedFeatureRoleHidden,
			Relay:     SelfHostedFeatureRoleHidden,
			Pro:       SelfHostedFeatureRoleHidden,
		},
	},
	FeatureWhiteLabel: {
		Key:                 FeatureWhiteLabel,
		DisplayName:         "White-Label Branding",
		ComparisonName:      "White-Label Branding",
		DisplayableInPlanUI: false,
		SelfHostedRoles: SelfHostedFeatureRoles{
			Community: SelfHostedFeatureRoleHidden,
			Relay:     SelfHostedFeatureRoleHidden,
			Pro:       SelfHostedFeatureRoleHidden,
		},
	},
	FeatureMultiTenant: {
		Key:                 FeatureMultiTenant,
		DisplayName:         "Multi-Tenant Mode",
		ComparisonName:      "Multi-Tenant Mode",
		DisplayableInPlanUI: true,
		SelfHostedRoles: SelfHostedFeatureRoles{
			Community: SelfHostedFeatureRoleHidden,
			Relay:     SelfHostedFeatureRoleHidden,
			Pro:       SelfHostedFeatureRoleHidden,
		},
	},
	FeatureUnlimited: {
		Key:                 FeatureUnlimited,
		DisplayName:         "Unlimited Instances",
		ComparisonName:      "Unlimited Instances",
		DisplayableInPlanUI: false,
		SelfHostedRoles: SelfHostedFeatureRoles{
			Community: SelfHostedFeatureRoleHidden,
			Relay:     SelfHostedFeatureRoleHidden,
			Pro:       SelfHostedFeatureRoleHidden,
		},
	},
	FeatureDemoFixtures: {
		Key:                 FeatureDemoFixtures,
		DisplayName:         "Demo Fixtures (Internal)",
		ComparisonName:      "Demo Fixtures (Internal)",
		DisplayableInPlanUI: false,
		SelfHostedRoles: SelfHostedFeatureRoles{
			Community: SelfHostedFeatureRoleHidden,
			Relay:     SelfHostedFeatureRoleHidden,
			Pro:       SelfHostedFeatureRoleHidden,
		},
	},
}

func AllFeatureMetadata() []FeatureMetadata {
	metadata := make([]FeatureMetadata, 0, len(orderedFeatureMetadataKeys))
	for _, key := range orderedFeatureMetadataKeys {
		entry, ok := featureMetadataCatalog[key]
		if !ok {
			continue
		}
		metadata = append(metadata, entry)
	}
	return metadata
}

func GetFeatureMetadata(feature string) (FeatureMetadata, bool) {
	entry, ok := featureMetadataCatalog[feature]
	return entry, ok
}

func GetSelfHostedFeatureRole(feature string, tier Tier) SelfHostedFeatureRole {
	entry, ok := featureMetadataCatalog[feature]
	if !ok {
		return SelfHostedFeatureRoleHidden
	}
	switch tier {
	case TierFree:
		return entry.SelfHostedRoles.Community
	case TierRelay:
		return entry.SelfHostedRoles.Relay
	case TierPro, TierProPlus, TierProAnnual, TierLifetime:
		return entry.SelfHostedRoles.Pro
	default:
		return SelfHostedFeatureRoleHidden
	}
}

func IsCompatibilityOnlyFeature(feature string) bool {
	entry, ok := featureMetadataCatalog[feature]
	if !ok {
		return false
	}
	return entry.SelfHostedRoles.Community == SelfHostedFeatureRoleCompatibilityOnly ||
		entry.SelfHostedRoles.Relay == SelfHostedFeatureRoleCompatibilityOnly ||
		entry.SelfHostedRoles.Pro == SelfHostedFeatureRoleCompatibilityOnly
}

func SelfHostedComparisonFeatures() []FeatureMetadata {
	out := make([]FeatureMetadata, 0)
	for _, entry := range AllFeatureMetadata() {
		if entry.ShowInComparisonTable {
			out = append(out, entry)
		}
	}
	return out
}

func SelfHostedPlanFeaturesForRole(tier Tier, role SelfHostedFeatureRole) []FeatureMetadata {
	out := make([]FeatureMetadata, 0)
	for _, entry := range AllFeatureMetadata() {
		if GetSelfHostedFeatureRole(entry.Key, tier) == role {
			out = append(out, entry)
		}
	}
	return out
}

func GenericUpgradeFeatureMetadata() []FeatureMetadata {
	out := make([]FeatureMetadata, 0)
	for _, entry := range AllFeatureMetadata() {
		if entry.UpgradeReason != "" {
			out = append(out, entry)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].UpgradePriority == out[j].UpgradePriority {
			return out[i].Key < out[j].Key
		}
		return out[i].UpgradePriority < out[j].UpgradePriority
	})
	return out
}
