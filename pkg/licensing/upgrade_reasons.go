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
var UpgradeReasonMatrix = []ReasonEntry{
	{
		Feature:   FeatureAIAutoFix,
		Reason:    "Upgrade to Pro to enable automatic remediation — Pulse Patrol will fix issues it finds without manual intervention.",
		ActionURL: UpgradeURLForFeature(FeatureAIAutoFix),
		Priority:  1,
	},
	{
		Feature:   FeatureLongTermMetrics,
		Reason:    "Upgrade to Pro for 90 days of historical metrics — spot trends, plan capacity, and investigate past incidents.",
		ActionURL: UpgradeURLForFeature(FeatureLongTermMetrics),
		Priority:  2,
	},
	{
		Feature:   FeatureRelay,
		Reason:    "Upgrade to Pro to access your infrastructure remotely via the Pulse mobile app with end-to-end encryption.",
		ActionURL: UpgradeURLForFeature(FeatureRelay),
		Priority:  3,
	},
	{
		Feature:   FeatureRBAC,
		Reason:    "Upgrade to Pro to control who can view, manage, and modify your infrastructure with fine-grained roles.",
		ActionURL: UpgradeURLForFeature(FeatureRBAC),
		Priority:  4,
	},
	{
		Feature:   FeatureAIAlerts,
		Reason:    "Upgrade to Pro for AI-powered alert analysis — get root cause insights when alerts fire.",
		ActionURL: UpgradeURLForFeature(FeatureAIAlerts),
		Priority:  5,
	},
	{
		Feature:   FeatureKubernetesAI,
		Reason:    "Upgrade to Pro for AI-powered Kubernetes insights — analyze pod health, resource pressure, and cluster issues.",
		ActionURL: UpgradeURLForFeature(FeatureKubernetesAI),
		Priority:  6,
	},
	{
		Feature:   FeatureAgentProfiles,
		Reason:    "Upgrade to Pro to manage agent configurations centrally — deploy consistent settings across your fleet.",
		ActionURL: UpgradeURLForFeature(FeatureAgentProfiles),
		Priority:  7,
	},
	{
		Feature:   FeatureAdvancedSSO,
		Reason:    "Upgrade to Pro for SAML SSO, multi-provider support, and automatic role mapping.",
		ActionURL: UpgradeURLForFeature(FeatureAdvancedSSO),
		Priority:  8,
	},
	{
		Feature:   FeatureAuditLogging,
		Reason:    "Upgrade to Pro for tamper-evident audit logs — track every action for compliance and incident response.",
		ActionURL: UpgradeURLForFeature(FeatureAuditLogging),
		Priority:  9,
	},
	{
		Feature:   FeatureAdvancedReporting,
		Reason:    "Upgrade to Pro for automated infrastructure reports — generate PDF/CSV summaries for stakeholders.",
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
