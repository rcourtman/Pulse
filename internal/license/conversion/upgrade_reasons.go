package conversion

import (
	"sort"

	"github.com/rcourtman/pulse-go-rewrite/internal/license"
)

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
		Feature:   license.FeatureAIAutoFix,
		Reason:    "Upgrade to Pro to enable automatic remediation — Pulse Patrol will fix issues it finds without manual intervention.",
		ActionURL: "https://pulserelay.pro/pricing?utm_source=pulse&utm_medium=app&utm_campaign=upgrade&feature=ai_autofix",
		Priority:  1,
	},
	{
		Feature:   license.FeatureLongTermMetrics,
		Reason:    "Upgrade to Pro for 90 days of historical metrics — spot trends, plan capacity, and investigate past incidents.",
		ActionURL: "https://pulserelay.pro/pricing?utm_source=pulse&utm_medium=app&utm_campaign=upgrade&feature=long_term_metrics",
		Priority:  2,
	},
	{
		Feature:   license.FeatureRelay,
		Reason:    "Upgrade to Pro to access your infrastructure remotely via the Pulse mobile app with end-to-end encryption.",
		ActionURL: "https://pulserelay.pro/pricing?utm_source=pulse&utm_medium=app&utm_campaign=upgrade&feature=relay",
		Priority:  3,
	},
	{
		Feature:   license.FeatureRBAC,
		Reason:    "Upgrade to Pro to control who can view, manage, and modify your infrastructure with fine-grained roles.",
		ActionURL: "https://pulserelay.pro/pricing?utm_source=pulse&utm_medium=app&utm_campaign=upgrade&feature=rbac",
		Priority:  4,
	},
	{
		Feature:   license.FeatureAIAlerts,
		Reason:    "Upgrade to Pro for AI-powered alert analysis — get root cause insights when alerts fire.",
		ActionURL: "https://pulserelay.pro/pricing?utm_source=pulse&utm_medium=app&utm_campaign=upgrade&feature=ai_alerts",
		Priority:  5,
	},
	{
		Feature:   license.FeatureKubernetesAI,
		Reason:    "Upgrade to Pro for AI-powered Kubernetes insights — analyze pod health, resource pressure, and cluster issues.",
		ActionURL: "https://pulserelay.pro/pricing?utm_source=pulse&utm_medium=app&utm_campaign=upgrade&feature=kubernetes_ai",
		Priority:  6,
	},
	{
		Feature:   license.FeatureAgentProfiles,
		Reason:    "Upgrade to Pro to manage agent configurations centrally — deploy consistent settings across your fleet.",
		ActionURL: "https://pulserelay.pro/pricing?utm_source=pulse&utm_medium=app&utm_campaign=upgrade&feature=agent_profiles",
		Priority:  7,
	},
	{
		Feature:   license.FeatureAdvancedSSO,
		Reason:    "Upgrade to Pro for SAML SSO, multi-provider support, and automatic role mapping.",
		ActionURL: "https://pulserelay.pro/pricing?utm_source=pulse&utm_medium=app&utm_campaign=upgrade&feature=advanced_sso",
		Priority:  8,
	},
	{
		Feature:   license.FeatureAuditLogging,
		Reason:    "Upgrade to Pro for tamper-evident audit logs — track every action for compliance and incident response.",
		ActionURL: "https://pulserelay.pro/pricing?utm_source=pulse&utm_medium=app&utm_campaign=upgrade&feature=audit_logging",
		Priority:  9,
	},
	{
		Feature:   license.FeatureAdvancedReporting,
		Reason:    "Upgrade to Pro for automated infrastructure reports — generate PDF/CSV summaries for stakeholders.",
		ActionURL: "https://pulserelay.pro/pricing?utm_source=pulse&utm_medium=app&utm_campaign=upgrade&feature=advanced_reporting",
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
