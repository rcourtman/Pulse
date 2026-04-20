package licensing

import "sort"

// ReasonEntry defines an actionable upgrade prompt tied to a missing feature.
type ReasonEntry struct {
	Feature   string // Feature key constant (e.g., "ai_autofix")
	Reason    string // User-facing description
	ActionURL string // Parameterized upgrade URL with UTM
	Priority  int    // Sort order (lower = more important)
}

// UpgradeReasonMatrix is the canonical feature-to-upgrade-reason mapping.
// Relay features use "Upgrade to Relay" messaging; Pro features use "Upgrade to Pro".
var UpgradeReasonMatrix = []ReasonEntry{
	{
		Feature:   FeatureRelay,
		Reason:    "Get Relay so Pulse stays reachable securely from anywhere instead of only on the local dashboard.",
		ActionURL: UpgradeURLForFeature(FeatureRelay),
		Priority:  1,
	},
	{
		Feature:   FeatureMobileApp,
		Reason:    "Get Relay so you can check Pulse from your phone when you are away from the dashboard.",
		ActionURL: UpgradeURLForFeature(FeatureMobileApp),
		Priority:  1,
	},
	{
		Feature:   FeaturePushNotifications,
		Reason:    "Get Relay so important alerts reach you immediately on mobile instead of waiting for you to reopen Pulse.",
		ActionURL: UpgradeURLForFeature(FeaturePushNotifications),
		Priority:  1,
	},
	{
		Feature:   FeatureLongTermMetrics,
		Reason:    "Get Relay for 14 days of history, or Pro for 90 days, so you can see what changed before and after an incident.",
		ActionURL: UpgradeURLForFeature(FeatureLongTermMetrics),
		Priority:  2,
	},
	{
		Feature:   FeatureAIAutoFix,
		Reason:    "Upgrade to Pro so Pulse can move from finding issues to applying safe remediation with your approval or in autonomous mode.",
		ActionURL: UpgradeURLForFeature(FeatureAIAutoFix),
		Priority:  3,
	},
	{
		Feature:   FeatureAIAlerts,
		Reason:    "Upgrade to Pro so alerts arrive with root-cause analysis instead of a stack of symptoms.",
		ActionURL: UpgradeURLForFeature(FeatureAIAlerts),
		Priority:  4,
	},
	{
		Feature:   FeatureKubernetesAI,
		Reason:    "Upgrade to Pro so Pulse can explain cluster pressure, failing pods, and likely causes without manual Kubernetes triage.",
		ActionURL: UpgradeURLForFeature(FeatureKubernetesAI),
		Priority:  5,
	},
	{
		Feature:   FeatureRBAC,
		Reason:    "Upgrade to Pro when more than one operator needs safe access boundaries around infrastructure changes.",
		ActionURL: UpgradeURLForFeature(FeatureRBAC),
		Priority:  6,
	},
	{
		Feature:   FeatureAgentProfiles,
		Reason:    "Upgrade to Pro to standardize agent behavior across systems without reconfiguring every install by hand.",
		ActionURL: UpgradeURLForFeature(FeatureAgentProfiles),
		Priority:  7,
	},
	{
		Feature:   FeatureAdvancedSSO,
		Reason:    "Upgrade to Pro to connect your identity provider and keep operator access aligned with your existing auth controls.",
		ActionURL: UpgradeURLForFeature(FeatureAdvancedSSO),
		Priority:  8,
	},
	{
		Feature:   FeatureAuditLogging,
		Reason:    "Upgrade to Pro to keep a trustworthy action trail for incident review, accountability, and compliance.",
		ActionURL: UpgradeURLForFeature(FeatureAuditLogging),
		Priority:  9,
	},
	{
		Feature:   FeatureAdvancedReporting,
		Reason:    "Upgrade to Pro to turn live infrastructure state into shareable reports without manual screenshot work.",
		ActionURL: UpgradeURLForFeature(FeatureAdvancedReporting),
		Priority:  10,
	},
}

// GenerateUpgradeReasons returns upgrade reasons for features missing in capabilities.
func GenerateUpgradeReasons(capabilities []string) []ReasonEntry {
	capSet := make(map[string]struct{}, len(capabilities))
	for _, capability := range capabilities {
		capSet[capability] = struct{}{}
	}

	reasons := make([]ReasonEntry, 0, len(UpgradeReasonMatrix))
	for _, entry := range UpgradeReasonMatrix {
		if _, hasFeature := capSet[entry.Feature]; hasFeature {
			continue
		}
		reasons = append(reasons, entry)
	}

	sort.SliceStable(reasons, func(i, j int) bool {
		if reasons[i].Priority == reasons[j].Priority {
			return reasons[i].Feature < reasons[j].Feature
		}
		return reasons[i].Priority < reasons[j].Priority
	})

	return reasons
}
